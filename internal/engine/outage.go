package engine

import (
	"math"

	"highwaycoins/internal/cal"
	"highwaycoins/internal/geo"
)

// The competitor's outage. Every week, on one working day, its systems are down in one
// region from 08:00 to 18:00: a town and the places within 80 km of it in a straight
// line. The dealers there turn to a backup that costs about 30% more. Nobody announces
// it, and busy regions go down more often: in most weeks it is the capital region, with
// 60 to 100 of the day's requests.
//
// Without the watch the engine won 85-99% of its quotes there that day (against 20-45%
// elsewhere) and its prices followed by 6%: each win only says that the price was not too
// high. The watch treats it as what it is, one of two cases. For every town it keeps the
// odds that its region is down today, and each answer to a quote from that region moves
// them by how much likelier the answer is with the backup's prices than without. A run of
// wins at the usual price makes an outage likely within the hour; one lost quote ends it.
// The quote is then the price that pays best over both cases.

const (
	outageRadiusKm = 80.0
	// outageSure is the chance above which a day's outage counts as found: the week has no other.
	outageSure = 0.9
	// outageStep bounds how far one answer moves the odds, so one odd dealer decides nothing.
	outageStep = 20.0
	outageOdds = 1e4
	// outageLearnBar is the chance of an outage from which an answer no longer teaches the
	// price model: it would take the backup's prices for the competitor's.
	outageLearnBar = 0.5
)

// outageWatch follows the answers of one day, region by region.
type outageWatch struct {
	near   map[string][]string // place -> the towns whose region holds it
	prior  float64             // odds of an outage in one region on one day
	backup float64             // log of the backup's price over the competitor's
	day    int
	odds   map[string]float64 // town -> odds that its region is down today
	over   bool               // this week's outage was found on an earlier day
}

func newOutageWatch(m *geo.Map, prior, backup float64) *outageWatch {
	w := &outageWatch{near: map[string][]string{}, prior: prior, backup: math.Log(backup), day: -1, odds: map[string]float64{}}
	for _, p := range m.Places() {
		for _, town := range m.Places() {
			if m.AirKm(town.ID, p.ID) <= outageRadiusKm {
				w.near[p.ID] = append(w.near[p.ID], town.ID)
			}
		}
	}
	return w
}

// open reports whether minute now is inside the hours of an outage.
func outageHours(now int) bool {
	minute := cal.MinuteOfDay(now)
	return cal.DayOf(now) < requestDays && minute >= requestDayStart && minute < requestDayEnd
}

// roll starts a new day: yesterday's outage, if it was found, was the week's only one.
func (w *outageWatch) roll(day int) {
	if day == w.day {
		return
	}
	for _, odds := range w.odds {
		if odds/(1+odds) >= outageSure {
			w.over = true
		}
	}
	w.day, w.odds = day, map[string]float64{}
}

func (w *outageWatch) oddsOf(town string) float64 {
	if odds, ok := w.odds[town]; ok {
		return odds
	}
	return w.prior
}

// chance is the chance that the region of a pickup place is down at minute now.
func (w *outageWatch) chance(place string, now int) float64 {
	w.roll(cal.DayOf(now))
	if w.over || !outageHours(now) {
		return 0
	}
	best := 0.0
	for _, town := range w.near[place] {
		best = math.Max(best, w.oddsOf(town))
	}
	return best / (1 + best)
}

// learn takes the answer to a quote made at minute at for a car from place: level and
// width are the win curve the quote was priced against, without an outage.
func (w *outageWatch) learn(place string, at int, level, width, ratio float64, won bool) {
	if day := cal.DayOf(at); day > w.day {
		w.roll(day)
	}
	if cal.DayOf(at) != w.day || w.over || !outageHours(at) {
		return // the answer to a quote of an earlier day, or of an hour without outages
	}
	usual, down := winChance(level, ratio, width), winChance(level+w.backup, ratio, width)
	if !won {
		usual, down = 1-usual, 1-down
	}
	step := math.Max(1/outageStep, math.Min(outageStep, down/math.Max(usual, 1e-9)))
	for _, town := range w.near[place] {
		w.odds[town] = math.Max(1/outageOdds, math.Min(outageOdds, w.oddsOf(town)*step))
	}
}

// outageRatio is the price over list price that pays best when the dealer is on the
// backup with the given chance, and the chance to win at it. It compares the best price
// of each case over both cases, so a small chance leaves the quote where it was.
func outageRatio(chance, backup, level, width, costRatio, bonusRatio, floorRatio float64) (float64, float64) {
	shift := math.Log(backup)
	win := func(r float64) float64 {
		return (1-chance)*winChance(level, r, width) + chance*winChance(level+shift, r, width)
	}
	best, _ := bestRatioAt(level, width, costRatio, bonusRatio, floorRatio)
	high, _ := bestRatioAt(level+shift, width, costRatio, bonusRatio, floorRatio)
	if (high-costRatio+bonusRatio)*win(high) > (best-costRatio+bonusRatio)*win(best) {
		best = high
	}
	return best, win(best)
}
