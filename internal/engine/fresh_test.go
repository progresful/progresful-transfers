package engine

import (
	"testing"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

// bookedSingle posts one car, lets a driver book it and moves the clock on.
func bookedSingle(t *testing.T, freshMin, heldMin, deadline int) (*Engine, *gigBook, int) {
	t.Helper()
	e := chainEngine(t)
	e.P.DropFreshMin = freshMin
	ob := chainOrder(e, "R1", "a", "b", e.now, deadline)
	e.dispatch(ob)
	sent := e.Drain()
	if len(sent) != 1 {
		t.Fatalf("want one post, got %d", len(sent))
	}
	opening := sent[0].Action.Schedule[0].FeeCents
	e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g1"}})
	e.SetNow(e.now + 30)
	e.Handle([]api.Event{{Kind: api.KindGigBooked, At: e.now, GigID: "g1", DriverID: "d1", FeeCents: 2700}})
	e.Drain()
	e.SetNow(e.now + heldMin)
	return e, e.gigs["g1"], opening
}

// A driver gives a single car back hours later. Left alone the gig shows the fee its
// clock reached meanwhile; with DropFreshMin and room to spare it is posted again and
// opens at the lowest fee, as the first time.
func TestDroppedSingleStartsAFreshClock(t *testing.T) {
	tests := []struct {
		name     string
		freshMin int
		deadline int
		fresh    bool
	}{
		{"off: the gig stays as it is", 0, cal.At(2, 20*cal.Hour), false},
		{"room to spare: posted again", 360, cal.At(2, 20*cal.Hour), true},
		{"little room: the gig stays", 360, cal.At(0, 20*cal.Hour), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, g, opening := bookedSingle(t, tc.freshMin, 240, tc.deadline)
			e.Handle([]api.Event{{Kind: api.KindGigDropped, At: e.now, GigID: "g1", DriverID: "d1"}})
			sent := e.Drain()
			singles, _, withdraws := postsOf(sent)
			if !tc.fresh {
				if withdraws != 0 || singles != 0 || g.status != gigOpen || e.stats.FreshClocks != 0 {
					t.Fatalf("the gig should stay open: %d withdraws, %d posts, status %s", withdraws, singles, g.status)
				}
				return
			}
			if withdraws != 1 || singles != 1 || sent[0].Action.GigID != "g1" || e.stats.FreshClocks != 1 {
				t.Fatalf("want withdraw g1 + one new post, got %+v", sent)
			}
			if got := sent[1].Action.Schedule[0].FeeCents; got != opening {
				t.Errorf("the new gig opens at %d, the first one opened at %d", got, opening)
			}
		})
	}
}

// A car that is posted at night with room keeps its opening fee until 07:00; with little
// room, or in the daytime, the clock starts at once.
func TestNightPostWaitsForTheMorning(t *testing.T) {
	tests := []struct {
		name     string
		night    int
		day      int
		hour     int
		deadline int
		waits    bool
	}{
		{"off", 0, 0, 23, cal.At(2, 20*cal.Hour), false},
		{"late evening with room", 1, 0, 23, cal.At(2, 20*cal.Hour), true},
		{"after midnight with room", 1, 1, 2, cal.At(2, 20*cal.Hour), true},
		{"at night with little room", 1, 1, 2, cal.At(1, 11*cal.Hour), false},
		{"in the daytime", 1, 1, 10, cal.At(2, 20*cal.Hour), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := chainEngine(t)
			e.P.NightClock = tc.night
			e.now = cal.At(tc.day, tc.hour*cal.Hour)
			ob := chainOrder(e, "R1", "a", "b", e.now, tc.deadline)
			e.dispatch(ob)
			sent := e.Drain()
			if len(sent) != 1 || sent[0].Action.Type != api.ActPost {
				t.Fatalf("want one post, got %+v", sent)
			}
			plan := sent[0].Action.Schedule
			morning := cal.At(1, 7*cal.Hour)
			if plan[0].At != e.now || len(plan) < 2 {
				t.Fatalf("the plan must start now and rise later: %+v", plan)
			}
			if tc.waits && plan[1].At <= morning {
				t.Errorf("the fee should not rise before 07:00: second step at %s", cal.Format(plan[1].At))
			}
			if !tc.waits && plan[1].At > e.now+20 {
				t.Errorf("the clock should start at once: second step at %s", cal.Format(plan[1].At))
			}
		})
	}
}

// With HoldAwake the slow rise of a waiting gig's fee sleeps at night: the fee of 07:00 is
// the fee of 22:00, no step falls in between, and the top of the hold is still reached
// when the deadline's pressure starts.
func TestHoldRiseSleepsAtNight(t *testing.T) {
	now, last, base := cal.At(0, 18*cal.Hour), cal.At(2, 16*cal.Hour), 10000
	plain := DefaultParamsFor(8)
	plain.HoldAwake = 0
	awake := plain
	awake.HoldAwake = 1
	lead := last - plain.LeadMin
	for _, night := range []int{0, 1} {
		evening, morning := cal.At(night, 22*cal.Hour+5), cal.At(night+1, 7*cal.Hour)
		steps := awake.Schedule(now, last, base)
		if a, b := feeAt(steps, evening), feeAt(steps, morning); a != b {
			t.Errorf("night %d: the fee went from %d to %d while no driver looked", night, a, b)
		}
		for _, s := range steps {
			if s.At > evening && s.At <= morning && s.At < lead {
				t.Errorf("night %d: a fee step at %s", night, cal.Format(s.At))
			}
		}
		if a, b := feeAt(plain.Schedule(now, last, base), evening), feeAt(plain.Schedule(now, last, base), morning); a >= b {
			t.Errorf("night %d: the test needs a plain schedule that rises overnight, got %d and %d", night, a, b)
		}
	}
	if got, want := feeAt(awake.Schedule(now, last, base), lead), feeOf(base, plain.HoldMax); got < want-50 || got > want {
		t.Errorf("at the start of the pressure the fee is %d, want the top of the hold %d", got, want)
	}
	if a, b := feeAt(awake.Schedule(now, last, base), last-plain.FinalMin), feeAt(plain.Schedule(now, last, base), last-plain.FinalMin); a != b {
		t.Errorf("the last fee before the deadline is %d, plain %d: the pressure must not change", a, b)
	}
}

func TestAwakeMinutes(t *testing.T) {
	tests := []struct {
		name     string
		from, to int
		want     int
	}{
		{"inside a day", cal.At(0, 9*cal.Hour), cal.At(0, 12*cal.Hour), 180},
		{"across a night", cal.At(0, 21*cal.Hour), cal.At(1, 8*cal.Hour), 120},
		{"all night", cal.At(0, 23*cal.Hour), cal.At(1, 6*cal.Hour), 0},
		{"two days", cal.At(0, 7*cal.Hour), cal.At(1, 22*cal.Hour), 1800},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := awakeMin(tc.from, tc.to); got != tc.want {
				t.Errorf("awake minutes %d, want %d", got, tc.want)
			}
			if tc.want > 0 {
				if at := awakeAt(tc.from, tc.want); awakeMin(tc.from, at) != tc.want || at > tc.to {
					t.Errorf("awakeAt gives %s: %d awake minutes from the start, want %d by %s", cal.Format(at), awakeMin(tc.from, at), tc.want, cal.Format(tc.to))
				}
			}
		})
	}
}

// The driver gives the car back and another takes the same gig in the same minute, while
// the engine has already posted the car again. The world refuses the second gig; with
// RaceSafe that refusal does not mark the car as one that needs a post without windows.
func TestRefusedDuplicatePostLeavesTheCarAlone(t *testing.T) {
	for _, safe := range []int{0, 1} {
		e, g, _ := bookedSingle(t, 360, 240, cal.At(2, 20*cal.Hour))
		e.P.RaceSafe = safe
		ob := e.orders["R1"]
		e.Handle([]api.Event{
			{Kind: api.KindGigDropped, At: e.now, GigID: "g1", DriverID: "d1"},
			{Kind: api.KindGigBooked, At: e.now, GigID: "g1", DriverID: "d2", FeeCents: 3100},
		})
		sent := e.Drain()
		if singles, _, withdraws := postsOf(sent); singles != 1 || withdraws != 1 {
			t.Fatalf("safe %d: want the withdraw and the new post on their way, got %+v", safe, sent)
		}
		e.Resolve(sent, []api.ActionResult{{OK: false, Reason: "Gig g1 is not open"}, {OK: false, Reason: "R1 leg 0 is already in a live gig"}})
		if g.status != gigBooked || ob.legs[0].gig != g {
			t.Fatalf("safe %d: the car belongs to the booked gig, got status %s", safe, g.status)
		}
		if more := e.Drain(); len(more) != 0 {
			t.Errorf("safe %d: nothing more to send, got %+v", safe, more)
		}
		if got, want := ob.legs[0].plain, safe == 0; got != want {
			t.Errorf("safe %d: the car marked for a post without windows: %v, want %v", safe, got, want)
		}
	}
}

// A gig that waited through the night is posted again at 07:00 on a fresh fee clock; one
// with little room, one that is booked and one posted this morning are left alone, and
// the round runs once a day.
func TestMorningRoundStartsWaitingGigsOver(t *testing.T) {
	e := chainEngine(t)
	e.P.MorningFreshMin = 360
	e.now = cal.At(0, 16*cal.Hour)
	e.Handle(nil) // the engine's first day: no round yet, and none later today
	roomy := chainOrder(e, "R1", "a", "b", e.now, cal.At(2, 20*cal.Hour))
	tight := chainOrder(e, "R2", "a", "c", e.now, cal.At(1, 11*cal.Hour))
	taken := chainOrder(e, "R3", "c", "b", e.now, cal.At(2, 20*cal.Hour))
	e.P.Chains = 0 // one gig a car in this test
	for _, ob := range []*orderBook{roomy, tight, taken} {
		e.dispatch(ob)
	}
	sent := e.Drain()
	if len(sent) != 3 {
		t.Fatalf("want three posts, got %d", len(sent))
	}
	opening := sent[0].Action.Schedule[0].FeeCents
	e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g1"}, {OK: true, ID: "g2"}, {OK: true, ID: "g3"}})
	e.Handle([]api.Event{{Kind: api.KindGigBooked, At: e.now, GigID: "g3", DriverID: "d1", FeeCents: 2500}})
	e.Drain()
	if wake := e.NextWake(); wake > cal.At(1, 7*cal.Hour) {
		t.Fatalf("the engine should wake by 07:00 for the round, next wake %s", cal.Format(wake))
	}
	e.SetNow(cal.At(1, 7*cal.Hour))
	e.Handle(nil)
	sent = e.Drain()
	singles, _, withdraws := postsOf(sent)
	if withdraws != 1 || singles != 1 || sent[0].Action.GigID != "g1" || e.stats.MorningFresh != 1 {
		t.Fatalf("want only the roomy gig posted again, got %+v", sent)
	}
	if got := sent[1].Action.Schedule[0].FeeCents; got != opening {
		t.Errorf("the new gig opens at %d, the first one opened at %d", got, opening)
	}
	e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g1"}, {OK: true, ID: "g4"}})
	e.SetNow(cal.At(1, 9*cal.Hour))
	e.Handle(nil)
	if again, _, w := postsOf(e.Drain()); again != 0 || w != 0 {
		t.Errorf("the round runs once a day, got %d posts and %d withdraws", again, w)
	}
}

// The fee clocks that start over are level 8's (e23); the levels below keep the old clock.
func TestFreshClocksByLevel(t *testing.T) {
	for level := 1; level <= 8; level++ {
		p := DefaultParamsFor(level)
		on := level >= 8
		want := Params{}
		if on {
			want.DropFreshMin, want.NightClock, want.ChainRecoverMin, want.ChainNight, want.HoldAwake, want.RaceSafe = 360, 1, 90, 1, 1, 1
		}
		if p.DropFreshMin != want.DropFreshMin || p.NightClock != want.NightClock || p.ChainRecoverMin != want.ChainRecoverMin ||
			p.ChainNight != want.ChainNight || p.HoldAwake != want.HoldAwake || p.RaceSafe != want.RaceSafe {
			t.Errorf("level %d: dropFreshMin %d, nightClock %d, chainRecoverMin %d, chainNight %d, holdAwake %d, raceSafe %d; want them on: %v",
				level, p.DropFreshMin, p.NightClock, p.ChainRecoverMin, p.ChainNight, p.HoldAwake, p.RaceSafe, on)
		}
		// Measured and left off at every level.
		if p.ChainBatch != 0 || p.War != 0 || p.BeliefFresh != 0 || p.PartialCap != 0 || p.PartialShare != 0 || p.StuckFrom != 0 {
			t.Errorf("level %d: a rejected switch is on: %+v", level, []float64{float64(p.ChainBatch), p.War, float64(p.BeliefFresh), p.PartialCap, p.PartialShare, p.StuckFrom})
		}
	}
}
