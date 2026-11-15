package engine

import "math"

// The belief model of the market (Params.MarketModel 2).
//
// The competitor's price for a request is the list price times a world level, a day
// factor, a dealer part and a part for each end of the lane (HANDBOOK.md, "Dealers compare
// prices"). The model keeps a Gaussian belief (mean and variance) about each part and
// updates all of them with every answer to a quote, by assumed-density filtering: the
// less a part is known, the more of the surprise it takes. So the first answers of a
// morning move the day factor a lot and the last ones hardly at all, which a fixed
// learning rate cannot do. The chance to win a quote is logistic in the log ratio, with a
// width that grows with what is still unknown about the request's level.
//
// Measured on 132,126 real quote outcomes of 164 practice weeks (42 seeds), predicting
// each answer before learning from it: log-loss 0.558 against 0.654 for the running-rate
// model, and well calibrated (quotes given 75% won 70%, against 61%).

// belief is what is known about one part of the price level (log ratio).
type belief struct {
	M float64 `json:"m"`
	V float64 `json:"v"`
}

// beliefBook holds the beliefs of one world.
type beliefBook struct {
	scale       float64 // width of the dealer's own choice, when the level is known exactly
	world       belief
	day         belief
	dayIndex    int
	dealers     map[string]*belief
	pickups     map[string]*belief
	deliveries  map[string]*belief
	priorDay    float64 // prior deviation of a day's factor
	priorDealer float64
	priorPlace  float64
	drift       float64 // deviation added to dealers and places each morning: the competitor moves
	fresh       bool    // answers are judged against the current level (Params.BeliefFresh)
	partial     float64 // share of dealers that take a menu without the fastest option (Params.PartialShare)
	war         *warWatch
}

func newBeliefBook(p Params) *beliefBook {
	var war *warWatch
	if p.War > 0 && p.War < 1 {
		war = newWarWatch(p.War, p.BeliefDay)
	}
	return &beliefBook{
		fresh:       p.BeliefFresh == 1 || war != nil,
		partial:     p.PartialShare,
		war:         war,
		scale:       p.BeliefScale,
		world:       belief{M: math.Log(p.MarketStart), V: p.BeliefWorld * p.BeliefWorld},
		day:         belief{V: p.BeliefDay * p.BeliefDay},
		dayIndex:    -1,
		dealers:     map[string]*belief{},
		pickups:     map[string]*belief{},
		deliveries:  map[string]*belief{},
		priorDay:    p.BeliefDay,
		priorDealer: p.BeliefDealer,
		priorPlace:  p.BeliefPlace,
		drift:       p.BeliefDrift,
	}
}

// roll starts a new day: its factor is unknown again, and the competitor may have moved
// its prices for a dealer or a place overnight.
func (b *beliefBook) roll(day int) {
	if day == b.dayIndex {
		return
	}
	first := b.dayIndex < 0
	if b.war != nil {
		b.war.roll(b.dayIndex, day)
	}
	b.dayIndex = day
	b.day = belief{V: b.priorDay * b.priorDay}
	if first {
		return
	}
	moved := b.drift * b.drift
	for _, group := range []map[string]*belief{b.dealers, b.pickups, b.deliveries} {
		for _, part := range group {
			part.V += moved
		}
	}
}

func partOf(group map[string]*belief, key string, prior float64) *belief {
	part := group[key]
	if part == nil {
		part = &belief{V: prior * prior}
		group[key] = part
	}
	return part
}

// parts lists the beliefs that make up a request's level. withDay leaves out the day
// factor for an answer to a quote of an earlier day.
func (b *beliefBook) parts(dealer, pickup, delivery string, withDay bool) []*belief {
	parts := []*belief{&b.world, partOf(b.dealers, dealer, b.priorDealer), partOf(b.pickups, pickup, b.priorPlace), partOf(b.deliveries, delivery, b.priorPlace)}
	if withDay {
		parts = append(parts, &b.day)
	}
	return parts
}

// placeShift is the part of a lane's price level that holds in every world.
func placeShift(pickup, delivery string) float64 {
	return placePickupEffect[pickup] + placeDeliveryEffect[delivery]
}

// widthFor is the width of the win curve when the level is known with variance v: the
// logistic of the dealer's choice widened by the uncertainty (matching variances).
func (b *beliefBook) widthFor(v float64) float64 {
	return math.Sqrt(b.scale*b.scale + 3*v/(math.Pi*math.Pi))
}

// level is the log ratio at which a quote for the request wins half the time, without the
// dealer's trust, and the width of the win curve around it.
func (b *beliefBook) level(dealer, pickup, delivery string, day int) (float64, float64) {
	b.roll(day)
	level, v := placeShift(pickup, delivery), 0.0
	for _, part := range b.parts(dealer, pickup, delivery, true) {
		level += part.M
		v += part.V
	}
	return level, b.widthFor(v)
}

// learn updates the beliefs with the answer to a quote that was priced against the given
// level (trust included) at the given ratio.
func (b *beliefBook) learn(q pendingQuote, won bool) {
	today := q.Day == b.dayIndex && q.Base != 0
	if today && b.war != nil && b.war.open() {
		b.learnBothCases(q, won)
		return
	}
	parts := b.parts(q.Dealer, q.Place, q.Dest, q.Day == b.dayIndex)
	v, level := 0.0, q.Level
	if b.fresh && today {
		// The level as it is now: what the model has learned since the quote went out
		// is already in it. The dealer's trust is the part the beliefs do not hold.
		level += placeShift(q.Place, q.Dest) - q.Base
		for _, part := range parts {
			level += part.M
		}
	}
	for _, part := range parts {
		v += part.V
	}
	width := b.widthFor(v)
	chance := sigmoid((level - math.Log(q.Ratio)) / width)
	y := 0.0
	if won {
		y = 1
	}
	damp := b.lossDamp(q, chance, won)
	gradient := damp * (y - chance) / width
	information := damp * chance * (1 - chance) / (width * width)
	shrink := 1 + information*v
	for _, part := range parts {
		part.M += part.V * gradient / shrink
		part.V -= part.V * part.V * information / shrink
	}
}

// lossDamp is how much of a lost quote counts for the price model: all of it, except for
// a quote without the request's fastest option. Only a share of the dealers takes such a
// menu at all, so its loss is partly the wish for speed; with that share s and the chance
// p to win on price alone, the loss says s(1-p)/(1-sp) of what a plain loss says.
func (b *beliefBook) lossDamp(q pendingQuote, chance float64, won bool) float64 {
	if won || !q.Partial || b.partial <= 0 || b.partial >= 1 {
		return 1
	}
	return b.partial * (1 - chance) / (1 - b.partial*chance)
}

// placePickupEffect is the shift of the log price level by pickup place, fitted on the same
// quote outcomes with the belief model in the loop (half of the seeds predicted the other
// half as well as themselves). Cars leaving the capital region are dear, cars leaving a
// remote town cheap.
var placePickupEffect = map[string]float64{
	"espoo": 0.047, "hameenlinna": 0.017, "helsinki": 0.056, "hyvinkaa": 0.073, "joensuu": -0.026,
	"jyvaskyla": -0.009, "kajaani": -0.188, "kemi": -0.099, "kokkola": -0.032, "kotka": -0.016,
	"kouvola": 0.013, "kuopio": 0.006, "lahti": 0.016, "lappeenranta": 0.011, "lempaala": -0.001,
	"mikkeli": -0.001, "oulu": 0.000, "pori": -0.001, "porvoo": 0.031, "raisio": 0.009,
	"rauma": 0.000, "rovaniemi": -0.004, "salo": 0.042, "savonlinna": -0.035, "seinajoki": 0.007,
	"tampere": 0.006, "turku": 0.001, "vaasa": 0.015, "vantaa": 0.074, "ylivieska": -0.012,
}

// placeDeliveryEffect is the shift of the log price level by delivery place: cars going
// to where the drivers live are cheap.
var placeDeliveryEffect = map[string]float64{
	"espoo": -0.094, "hameenlinna": -0.005, "helsinki": -0.100, "hyvinkaa": -0.086, "joensuu": 0.034,
	"jyvaskyla": -0.018, "kajaani": 0.117, "kemi": 0.043, "kokkola": 0.031, "kotka": 0.029,
	"kouvola": 0.037, "kuopio": -0.004, "lahti": 0.005, "lappeenranta": 0.030, "lempaala": -0.025,
	"mikkeli": 0.028, "oulu": -0.012, "pori": 0.030, "porvoo": -0.074, "raisio": -0.036,
	"rauma": 0.028, "rovaniemi": 0.052, "salo": 0.011, "savonlinna": 0.086, "seinajoki": 0.009,
	"tampere": -0.031, "turku": -0.024, "vaasa": 0.010, "vantaa": -0.114, "ylivieska": 0.044,
}
