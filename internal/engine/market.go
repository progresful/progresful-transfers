package engine

import (
	"math"

	"highwaycoins/internal/cal"
)

const (
	// maxTrust is the cap of a dealer's count of on-time orders (level 8).
	maxTrust = 25
	// trustStep is how much each counted order raises the prices the dealer accepts.
	trustStep = 0.01
	// levelTrust is the level at which dealers count on-time orders.
	levelTrust = 8
	// dayLocalWeight is how much faster today's dealer and region parts learn than the global one.
	dayLocalWeight = 1.5
	// ratioGrid is how many candidate prices the quoter compares for one request.
	ratioGrid = 48
	// requestDayStart and requestDayEnd bound the hours in which requests arrive.
	requestDayStart = 8 * cal.Hour
	requestDayEnd   = 18 * cal.Hour
	requestDays     = 5
	// marketModelBeliefs selects the belief model of the price level (beliefs.go).
	marketModelBeliefs = 2
)

// dealerBook is what the engine has learned about one dealer.
type dealerBook struct {
	offset   float64 // learned shift of the dealer's accepted price level (log ratio)
	requests int
	quotes   int
	wins     int
	trust    int // orders delivered on time, halved by a late one (counts at level 8)
}

// dayBook holds what the market does today only: the competitor's day factor, a price
// war (everywhere, 20% down) or an outage (one region on a dearer backup). None is
// announced; each shows in the day's wins and losses, and is gone the next morning.
type dayBook struct {
	day    int
	global float64
	dealer map[string]float64
	place  map[string]float64 // by pickup place; a quote teaches the places around it too
}

// market is the engine's running estimate of what dealers accept: the probability that
// a quote at ratio r (price over list price) wins is logistic in log r around a level
// made of a world part, a dealer part and the dealer's trust.
type market struct {
	mu      float64 // world level: log of the ratio at which a quote wins half the time
	scale   float64 // width of the logistic in log-ratio
	rate    float64 // learning rate of the world level
	dRate   float64 // learning rate of a dealer's offset
	trustOn bool
	seen    int
	dealers map[string]*dealerBook
	// Today's fast-moving part, and how fast it learns (0: off).
	today   dayBook
	dayRate float64
	// around lists the places that share a region with a place (itself included).
	around func(place string) []string
	// dest is the learned shift of the level by delivery place: the part of a lane's price
	// that is new in each world. destRate is how fast it learns (0: off).
	dest     map[string]*destBook
	destRate float64
	// beliefs is the belief model of the level (Params.MarketModel 2); nil keeps the
	// running rates above.
	beliefs *beliefBook
}

// destBook is what the week's quotes have shown about one delivery place.
type destBook struct {
	offset float64
	quotes int
}

// destShiftCap bounds a delivery place's learned shift (log ratio): a few unlucky quotes
// must not price a place out.
const destShiftCap = 0.25

// destShift is the learned shift of the level for cars going to a place.
func (m *market) destShift(place string) float64 {
	if b := m.dest[place]; b != nil {
		return b.offset
	}
	return 0
}

// learnDest moves a delivery place's shift by a quote's surprise; the steps shrink as
// quotes for the place come in.
func (m *market) learnDest(place string, surprise float64) {
	if m.destRate <= 0 || place == "" {
		return
	}
	b := m.dest[place]
	if b == nil {
		b = &destBook{}
		m.dest[place] = b
	}
	b.quotes++
	b.offset += surprise * math.Max(m.destRate, 3*m.destRate/math.Sqrt(float64(b.quotes)))
	b.offset = math.Max(-destShiftCap, math.Min(destShiftCap, b.offset))
}

func newMarket(p Params, level int) *market {
	var beliefs *beliefBook
	if p.MarketModel == marketModelBeliefs {
		beliefs = newBeliefBook(p)
	}
	return &market{
		beliefs:  beliefs,
		mu:       math.Log(p.MarketStart),
		scale:    p.MarketScale,
		rate:     p.MarketRate,
		dRate:    p.DealerRate,
		trustOn:  level >= levelTrust && p.TrustBoost > 0,
		dealers:  map[string]*dealerBook{},
		today:    dayBook{day: -1, dealer: map[string]float64{}, place: map[string]float64{}},
		dayRate:  p.DayRate,
		around:   func(place string) []string { return []string{place} },
		dest:     map[string]*destBook{},
		destRate: p.DestRate,
	}
}

func (m *market) dealer(id string) *dealerBook {
	d := m.dealers[id]
	if d == nil {
		d = &dealerBook{}
		m.dealers[id] = d
	}
	return d
}

// level is the log ratio at which a quote to this dealer wins half the time.
func (m *market) level(d *dealerBook, trustBoost float64) float64 {
	return m.mu + d.offset + m.trustShift(d, trustBoost)
}

// trustShift is how much a dealer's trust raises the prices it accepts (log ratio).
func (m *market) trustShift(d *dealerBook, trustBoost float64) float64 {
	if !m.trustOn {
		return 0
	}
	return trustBoost * math.Log(1+trustStep*float64(d.trust))
}

// worldRatio is the ratio at which a quote wins half the time on an average lane and day.
func (m *market) worldRatio() float64 {
	if m.beliefs != nil {
		return math.Exp(m.beliefs.world.M)
	}
	return math.Exp(m.mu)
}

// count keeps a dealer's quotes and wins.
func (m *market) count(d *dealerBook, won bool) {
	m.seen++
	d.quotes++
	if won {
		d.wins++
	}
}

// rollDay forgets yesterday's fast part when a new day starts.
func (m *market) rollDay(day int) {
	if day == m.today.day {
		return
	}
	m.today = dayBook{day: day, dealer: map[string]float64{}, place: map[string]float64{}}
}

// todayShift is today's fast part of the level for a dealer's request from a pickup place.
func (m *market) todayShift(dealerID, place string, day int) float64 {
	if m.dayRate <= 0 {
		return 0
	}
	m.rollDay(day)
	return m.today.global + m.today.dealer[dealerID] + m.today.place[place]
}

// learnToday moves today's fast part by a quote's surprise: everywhere a little, and
// more for the dealer and for the region of the pickup.
func (m *market) learnToday(dealerID, place string, day int, surprise float64) {
	if m.dayRate <= 0 || day != m.today.day {
		return // an answer to yesterday's quote says nothing about today
	}
	m.today.global += surprise * m.dayRate
	m.today.dealer[dealerID] += surprise * m.dayRate * dayLocalWeight
	for _, p := range m.around(place) {
		m.today.place[p] += surprise * m.dayRate * dayLocalWeight
	}
}

func sigmoid(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

// winProb is the estimated chance that a quote at the ratio wins against the level.
func (m *market) winProb(level, ratio float64) float64 {
	return winChance(level, ratio, m.scale)
}

// winChance is the chance that a quote at the ratio wins against a level known to the width.
func winChance(level, ratio, width float64) float64 {
	return sigmoid((level - math.Log(ratio)) / width)
}

// learn moves the world level and the dealer's offset towards what a quote's outcome shows,
// and returns the surprise (outcome minus the chance it had). The steps shrink as evidence
// comes in, but never to nothing: the competitor keeps moving.
func (m *market) learn(d *dealerBook, level, ratio float64, won bool) float64 {
	y := 0.0
	if won {
		y = 1
	}
	surprise := y - m.winProb(level, ratio)
	m.count(d, won)
	m.mu += surprise * math.Max(m.rate, 4*m.rate/math.Sqrt(float64(m.seen)))
	d.offset += surprise * math.Max(m.dRate, 2*m.dRate/math.Sqrt(float64(d.quotes)))
	return surprise
}

// bestRatio picks the price ratio with the highest expected gain for a request whose
// cost is costRatio (cost over list price) and which is worth bonusRatio more when won.
// It never goes under floorRatio. The second result is the chance to win at that ratio.
func (m *market) bestRatio(level, costRatio, bonusRatio, floorRatio float64) (float64, float64) {
	return bestRatioAt(level, m.scale, costRatio, bonusRatio, floorRatio)
}

// bestRatioAt is bestRatio for a win curve of the given width around the level.
func bestRatioAt(level, width, costRatio, bonusRatio, floorRatio float64) (float64, float64) {
	lo, hi := math.Exp(level-4*width), math.Exp(level+3*width)
	best, bestGain := math.Max(floorRatio, hi), math.Inf(-1)
	for k := 0; k <= ratioGrid; k++ {
		r := lo + (hi-lo)*float64(k)/ratioGrid
		if r < floorRatio {
			continue
		}
		if gain := (r - costRatio + bonusRatio) * winChance(level, r, width); gain > bestGain {
			best, bestGain = r, gain
		}
	}
	return best, winChance(level, best, width)
}

// requestTimeLeft is the share of the week's request hours (Mon-Fri 08:00-18:00) still to come.
func requestTimeLeft(now int) float64 {
	day, minute := cal.DayOf(now), cal.MinuteOfDay(now)
	if day >= requestDays {
		return 0
	}
	span := float64(requestDayEnd - requestDayStart)
	today := math.Max(0, math.Min(1, float64(requestDayEnd-minute)/span))
	return (float64(requestDays-1-day) + today) / requestDays
}
