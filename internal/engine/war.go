package engine

import "math"

// The competitor's price war. Every week, on one working day, the competitor quotes 20%
// under its usual prices everywhere from 08:00 to 18:00, and nobody announces it. The
// belief model alone takes it for an unusually cheap day and needs the whole morning to
// follow: measured on 44 war days, the level was 7% down at 08:30, 16% at 09:30 and 22%
// only at 10:30, and until then one quote in five was won instead of one in three.
//
// The watch treats the day as one of two cases. It keeps a second belief about today's
// factor, the one that holds if today is the war day, and the odds between the two. Each
// answer moves the odds by how much likelier it is in the war case. Before any answer the
// chance is one in the working days that can still be the war day, so a week with four
// calm days makes Friday (nearly) sure. The quote is the price that pays best over both
// cases, and once the war is the likelier case its losses teach the war day's factor, not
// the prices of the dealers and places that hold all week.

const (
	// warSure is the chance at the end of a day from which it counts as the week's war day.
	warSure = 0.9
	// warCalm is the chance at the end of a day under which the day counts as a calm one,
	// when at least warMinAnswers answers were seen on it.
	warCalm       = 0.1
	warMinAnswers = 15
	// warPriorMax is the most a day is taken for the war day before any answer: a war day
	// that the watch missed must not make the last day a sure one.
	warPriorMax = 0.8
	// warLearnBar is the chance from which the week's prices learn from the war case.
	warLearnBar = 0.5
	// warStep bounds how far one answer moves the odds.
	warStep = 20.0
	warOdds = 1e4
)

// warWatch follows the answers of one day.
type warWatch struct {
	shift    float64 // log of the war's prices over the usual ones
	prior    float64 // prior deviation of a day's factor
	day      belief  // today's factor if today is the war day
	odds     float64 // odds that today is the war day
	over     bool    // the week's war day was found on an earlier day
	calmDays int     // watched working days without a war
	answers  int     // answers taken today
}

func newWarWatch(war, priorDay float64) *warWatch {
	return &warWatch{shift: math.Log(war), prior: priorDay}
}

// roll ends day prev (-1: there was none) and starts day: a day that ended as a sure war
// was the week's only one, and a watched day that ended calm cannot have been it.
func (w *warWatch) roll(prev, day int) {
	if prev >= 0 && prev < requestDays && !w.over {
		switch chance := w.chance(); {
		case chance >= warSure:
			w.over = true
		case chance <= warCalm && w.answers >= warMinAnswers:
			w.calmDays++
		}
	}
	w.day = belief{M: w.shift, V: w.prior * w.prior}
	w.answers, w.odds = 0, 0
	if w.over || day >= requestDays {
		return
	}
	chance := math.Min(warPriorMax, 1/float64(max(1, requestDays-w.calmDays)))
	w.odds = chance / (1 - chance)
}

// chance is the chance that today is the war day.
func (w *warWatch) chance() float64 { return w.odds / (1 + w.odds) }

// open reports whether today can still be the war day.
func (w *warWatch) open() bool { return w.odds > 0 }

// move takes one answer, likelier times as likely on a war day as on a usual one.
func (w *warWatch) move(likelier float64) {
	w.answers++
	step := math.Max(1/warStep, math.Min(warStep, likelier))
	w.odds = math.Max(1/warOdds, math.Min(warOdds, w.odds*step))
}

// caseStep is one answer taken under one case of the day: how likely the answer was, and
// what it does to the beliefs.
type caseStep struct {
	likely      float64
	gradient    float64
	information float64
	shrink      float64
	day         belief // the day's factor after the answer
}

// caseStep takes the answer to a quote under one belief about the day. base and v are the
// level and the variance of everything but the day's factor.
func (b *beliefBook) caseStep(q pendingQuote, base, v float64, day belief, won bool) caseStep {
	total := v + day.V
	width := b.widthFor(total)
	chance := sigmoid((base + day.M - math.Log(q.Ratio)) / width)
	y, likely := 0.0, 1-chance
	if won {
		y, likely = 1, chance
	}
	damp := b.lossDamp(q, chance, won)
	gradient := damp * (y - chance) / width
	information := damp * chance * (1 - chance) / (width * width)
	shrink := 1 + information*total
	return caseStep{likely: likely, gradient: gradient, information: information, shrink: shrink,
		day: belief{M: day.M + day.V*gradient/shrink, V: day.V - day.V*day.V*information/shrink}}
}

// warCase is the chance that today is the war day, and how far the level of that case is
// from the usual one (log ratio). Both are 0 without a watch, or when today cannot be it.
func (b *beliefBook) warCase() (chance, shift float64) {
	if b.war == nil || !b.war.open() {
		return 0, 0
	}
	return b.war.chance(), b.war.day.M - b.day.M
}

// learnBothCases takes the answer to a quote of today while the day can still be the war
// day. Each case keeps its own belief about the day and learns from the answer; the odds
// move by how well each case foresaw it; the parts that hold all week learn from the
// likelier case only.
func (b *beliefBook) learnBothCases(q pendingQuote, won bool) {
	persistent := b.parts(q.Dealer, q.Place, q.Dest, false)
	base, v := placeShift(q.Place, q.Dest)+q.Level-q.Base, 0.0
	for _, part := range persistent {
		base += part.M
		v += part.V
	}
	usual := b.caseStep(q, base, v, b.day, won)
	war := b.caseStep(q, base, v, b.war.day, won)
	b.war.move(war.likely / math.Max(usual.likely, 1e-9))
	chosen := usual
	if b.war.chance() >= warLearnBar {
		chosen = war
	}
	for _, part := range persistent {
		part.M += part.V * chosen.gradient / chosen.shrink
		part.V -= part.V * part.V * chosen.information / chosen.shrink
	}
	b.day, b.war.day = usual.day, war.day
}
