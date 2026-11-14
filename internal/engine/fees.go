package engine

import (
	"math"
	"sort"

	"highwaycoins/internal/api"
)

const (
	// maxScheduleSteps is the server's limit on fee steps.
	maxScheduleSteps = 48
	minFeeCents      = 100
	lateSteps        = 4
	lastSteps        = 8
	// stepGridMin aligns fee steps of different gigs, so a level 1 run (one step per
	// schedule, raised by the engine) wakes for many gigs at once.
	stepGridMin = 5
)

func lerp(a, b, t float64) float64 { return a + (b-a)*math.Max(0, math.Min(1, t)) }

func feeOf(base int, mult float64) int {
	return max(minFeeCents, int(math.Round(float64(base)*mult)))
}

// discoveryMult is the ascending clock that looks for the cheapest driver: FeeStart at
// the post, FeeStep more every StepMin, up to FeeMid.
func (p Params) discoveryMult(sincePost int) float64 {
	if p.StepMin <= 0 || p.FeeStep <= 0 {
		return p.FeeMid
	}
	return math.Min(p.FeeMid, p.FeeStart+p.FeeStep*float64(sincePost/p.StepMin))
}

// discoveryEnd is how long after the post the discovery clock reaches FeeMid.
func (p Params) discoveryEnd() int {
	if p.StepMin <= 0 || p.FeeStep <= 0 || p.FeeMid <= p.FeeStart {
		return 0
	}
	return int(math.Ceil((p.FeeMid-p.FeeStart)/p.FeeStep-1e-9)) * p.StepMin
}

// holdMult is the slow rise between the discovery clock and the deadline pressure: from
// FeeMid when the clock stops to HoldMax when the pressure starts. Zero when switched
// off, before the clock stops, or when the pressure starts before it does.
func (p Params) holdMult(sincePost, untilLast int) float64 {
	start := p.discoveryEnd()
	span := sincePost + untilLast - p.LeadMin - start
	if p.HoldMax <= p.FeeMid || sincePost < start || span <= 0 {
		return 0
	}
	return lerp(p.FeeMid, p.HoldMax, float64(sincePost-start)/float64(span))
}

// urgencyMult is the pressure of the deadline: nothing until LeadMin before the last
// on-time pickup, then from FeeMid up to FeeMax at FinalMin before it.
func (p Params) urgencyMult(untilLast int) float64 {
	switch {
	case untilLast >= p.LeadMin:
		return 0
	case untilLast <= p.FinalMin:
		return p.FeeMax
	}
	return lerp(p.FeeMid, p.FeeMax, float64(p.LeadMin-untilLast)/float64(max(1, p.LeadMin-p.FinalMin)))
}

// Schedule builds the fee steps of a gig posted (or planned again) at minute now, with
// lastPickup the last minute a driver can collect the car inside the gig's windows. The
// fee is the higher of the discovery clock and the deadline pressure.
func (p Params) Schedule(now, lastPickup, base int) []api.FeeStep {
	if lastPickup-now <= p.FinalMin {
		return []api.FeeStep{{At: now, FeeCents: feeOf(base, p.FeeMax)}}
	}
	final := lastPickup - p.FinalMin
	times := []int{now}
	if p.StepMin > 0 && p.FeeStep > 0 {
		for k := 1; p.FeeStart+p.FeeStep*float64(k-1) < p.FeeMid; k++ {
			times = append(times, onGrid(now+k*p.StepMin))
		}
	}
	if hold, lead := now+p.discoveryEnd(), lastPickup-p.LeadMin; p.HoldMax > p.FeeMid && lead > hold {
		for k := 1; k <= p.HoldSteps; k++ {
			times = append(times, onGrid(hold+(lead-hold)*k/p.HoldSteps))
		}
	}
	for k := 0; k <= p.UrgentSteps; k++ {
		t := lastPickup - p.LeadMin + (p.LeadMin-p.FinalMin)*k/max(1, p.UrgentSteps)
		times = append(times, onGrid(t))
	}
	sort.Ints(times)
	steps := make([]api.FeeStep, 0, len(times))
	for _, t := range times {
		if t < now || t > final+stepGridMin {
			continue
		}
		mult := math.Max(math.Max(p.discoveryMult(t-now), p.holdMult(t-now, lastPickup-t)), p.urgencyMult(lastPickup-t))
		steps = appendStep(steps, t, feeOf(base, mult))
	}
	return steps
}

func onGrid(t int) int { return (t + stepGridMin - 1) / stepGridMin * stepGridMin }

// lateSchedule is the fee plan of a car that can no longer be on time: a slow rise from
// LateStart to LateMax, because the refund is already owed and only its tier is at stake.
// If no driver takes it even then, it keeps rising towards the end of the world, up to
// lastFee: an undelivered car costs far more than any fee.
func (p Params) lateSchedule(now, base, endsAt, lastFee int) []api.FeeStep {
	steps := make([]api.FeeStep, 0, lateSteps+lastSteps+1)
	for k := 0; k <= lateSteps; k++ {
		frac := float64(k) / lateSteps
		at := now
		if k > 0 {
			at = onGrid(now + int(frac*float64(p.LateRampMin)))
		}
		steps = appendStep(steps, at, feeOf(base, lerp(p.LateStart, p.LateMax, frac)))
	}
	from, to := now+p.LateRampMin, endsAt-p.LastLeadMin
	top := feeOf(base, p.LateMax)
	if lastFee <= top || to <= from {
		return steps
	}
	for k := 1; k <= lastSteps; k++ {
		frac := float64(k) / lastSteps
		steps = appendStep(steps, onGrid(from+int(frac*float64(to-from))), top+int(frac*float64(lastFee-top)))
	}
	return steps
}

// appendStep keeps the schedule valid: strictly later times, never a lower fee, at most 48 steps.
func appendStep(steps []api.FeeStep, at, fee int) []api.FeeStep {
	if n := len(steps); n > 0 {
		last := steps[n-1]
		if at <= last.At || fee <= last.FeeCents || n >= maxScheduleSteps {
			return steps
		}
	}
	return append(steps, api.FeeStep{At: at, FeeCents: fee})
}

// feeAt is the fee a schedule shows at minute t: the latest step that has started.
func feeAt(schedule []api.FeeStep, t int) int {
	if len(schedule) == 0 {
		return 0
	}
	fee := schedule[0].FeeCents
	for _, s := range schedule {
		if s.At <= t {
			fee = s.FeeCents
		}
	}
	return fee
}

// fromNow cuts a schedule to the steps that matter from minute now on, opening at the
// fee it shows now. Used to post or reprice with the remainder of a plan.
func fromNow(schedule []api.FeeStep, now int) []api.FeeStep {
	out := []api.FeeStep{{At: now, FeeCents: feeAt(schedule, now)}}
	for _, s := range schedule {
		if s.At > now {
			out = appendStep(out, s.At, s.FeeCents)
		}
	}
	return out
}

// nextStepAfter is the minute of the first step after now, or -1 when the schedule has stopped rising.
func nextStepAfter(schedule []api.FeeStep, now int) int {
	for _, s := range schedule {
		if s.At > now {
			return s.At
		}
	}
	return -1
}

// capSchedule keeps a fee plan under a cap (0: no cap): no gig is worth more than
// delivering its cars is. Steps at the cap collapse into one.
func capSchedule(steps []api.FeeStep, limit int) []api.FeeStep {
	if limit <= 0 {
		return steps
	}
	limit = max(limit, minFeeCents)
	out := make([]api.FeeStep, 0, len(steps))
	for _, s := range steps {
		out = appendStep(out, s.At, min(s.FeeCents, limit))
	}
	return out
}

func floorSchedule(steps []api.FeeStep, floor int) []api.FeeStep {
	out := make([]api.FeeStep, 0, len(steps))
	for _, s := range steps {
		out = appendStep(out, s.At, max(s.FeeCents, floor))
	}
	return out
}

func sameSchedule(a, b []api.FeeStep) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
