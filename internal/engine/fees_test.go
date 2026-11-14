package engine

import (
	"testing"

	"highwaycoins/internal/api"
)

func checkValid(t *testing.T, steps []api.FeeStep, now int) {
	t.Helper()
	if len(steps) == 0 || len(steps) > maxScheduleSteps {
		t.Fatalf("schedule has %d steps", len(steps))
	}
	if steps[0].At != now {
		t.Errorf("first step at %d, want now %d", steps[0].At, now)
	}
	for i := 1; i < len(steps); i++ {
		if steps[i].At <= steps[i-1].At || steps[i].FeeCents <= steps[i-1].FeeCents {
			t.Errorf("step %d %+v does not rise after %+v", i, steps[i], steps[i-1])
		}
	}
}

func TestScheduleShapes(t *testing.T) {
	p := DefaultParams()
	const base = 10000
	tests := []struct {
		name               string
		now, lastPickup    int
		wantFirst, wantEnd int
	}{
		{"long horizon: discovery then pressure", 600, 600 + 1440, feeOf(base, p.FeeStart), feeOf(base, p.FeeMax)},
		{"short horizon: starts inside the pressure ramp", 600, 600 + 120, 0, feeOf(base, p.FeeMax)},
		{"no time left: top fee at once", 600, 600 + p.FinalMin, feeOf(base, p.FeeMax), feeOf(base, p.FeeMax)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			steps := p.Schedule(tc.now, tc.lastPickup, base)
			checkValid(t, steps, tc.now)
			if tc.wantFirst != 0 && steps[0].FeeCents != tc.wantFirst {
				t.Errorf("first fee %d, want %d", steps[0].FeeCents, tc.wantFirst)
			}
			if last := steps[len(steps)-1]; last.FeeCents != tc.wantEnd {
				t.Errorf("last fee %d, want %d", last.FeeCents, tc.wantEnd)
			}
			if last := steps[len(steps)-1]; last.At > tc.lastPickup {
				t.Errorf("last step at %d is after the last pickup %d", last.At, tc.lastPickup)
			}
		})
	}
}

func TestScheduleHoldsMidBetweenDiscoveryAndPressure(t *testing.T) {
	p := DefaultParams()
	const base, now, last = 10000, 600, 600 + 2000
	steps := p.Schedule(now, last, base)
	hold := feeAt(steps, last-p.LeadMin-1)
	if hold != feeOf(base, p.FeeMid) {
		t.Errorf("fee before the pressure ramp is %d, want the mid fee %d", hold, feeOf(base, p.FeeMid))
	}
}

func TestLateScheduleRises(t *testing.T) {
	p := DefaultParams()
	const base, now, endsAt = 8000, 1000, 14280
	steps := p.lateSchedule(now, base, endsAt, 0)
	checkValid(t, steps, now)
	if steps[0].FeeCents != feeOf(base, p.LateStart) || steps[len(steps)-1].FeeCents != feeOf(base, p.LateMax) {
		t.Errorf("late ramp runs %d..%d", steps[0].FeeCents, steps[len(steps)-1].FeeCents)
	}
	// With a last-resort fee the plan keeps rising until shortly before the world ends.
	last := feeOf(base, p.LastMult)
	steps = p.lateSchedule(now, base, endsAt, last)
	checkValid(t, steps, now)
	if end := steps[len(steps)-1]; end.FeeCents != last || end.At > endsAt-p.LastLeadMin+stepGridMin {
		t.Errorf("last step %+v, want fee %d by %d", end, last, endsAt-p.LastLeadMin)
	}
	// Close to the end there is no room for the last rise: the plan is the late ramp alone.
	steps = p.lateSchedule(endsAt-300, base, endsAt, last)
	checkValid(t, steps, endsAt-300)
}

func TestFromNowAndFloor(t *testing.T) {
	plan := []api.FeeStep{{At: 100, FeeCents: 500}, {At: 200, FeeCents: 700}, {At: 300, FeeCents: 900}}
	rest := fromNow(plan, 250)
	want := []api.FeeStep{{At: 250, FeeCents: 700}, {At: 300, FeeCents: 900}}
	if !sameSchedule(rest, want) {
		t.Errorf("fromNow = %+v, want %+v", rest, want)
	}
	floored := floorSchedule(plan, 800)
	if !sameSchedule(floored, []api.FeeStep{{At: 100, FeeCents: 800}, {At: 300, FeeCents: 900}}) {
		t.Errorf("floorSchedule = %+v", floored)
	}
	if got := nextStepAfter(plan, 200); got != 300 {
		t.Errorf("nextStepAfter = %d, want 300", got)
	}
	if got := nextStepAfter(plan, 300); got != -1 {
		t.Errorf("nextStepAfter past the end = %d, want -1", got)
	}
}
