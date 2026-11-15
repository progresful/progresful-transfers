package engine

import (
	"testing"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

func postsOf(sent []Pending) (singles, chains, withdraws int) {
	for _, p := range sent {
		switch {
		case p.Action.Type == api.ActWithdraw:
			withdraws++
		case p.Action.Type == api.ActPost && len(p.Action.Stops) == 4:
			chains++
		case p.Action.Type == api.ActPost:
			singles++
		}
	}
	return singles, chains, withdraws
}

// Two cars won in the same step: the first one's gig has not left for the app yet, so the
// second finds no open gig to pair with. With ChainBatch the unsent gig counts as a partner
// and is dropped from the outbox: the app only ever sees the chain.
func TestCarsWonInOneStepAreChained(t *testing.T) {
	tests := []struct {
		name                     string
		batch                    int
		singles, chains, batched int
	}{
		{"off: both go alone", 0, 2, 0, 0},
		{"on: one chain, the single post is never sent", 1, 0, 1, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := chainEngine(t)
			e.P.ChainBatch = tc.batch
			deadline := cal.At(0, 20*cal.Hour)
			first := chainOrder(e, "R1", "a", "b", e.now, deadline)
			second := chainOrder(e, "R2", "b", "a", e.now, deadline)
			e.dispatch(first)
			e.dispatch(second)
			sent := e.Drain()
			singles, chains, withdraws := postsOf(sent)
			if singles != tc.singles || chains != tc.chains || withdraws != 0 {
				t.Fatalf("sent %d single posts, %d chains, %d withdraws; want %d, %d, 0", singles, chains, withdraws, tc.singles, tc.chains)
			}
			if e.stats.ChainsBatched != tc.batched || e.stats.Posts != len(sent) {
				t.Errorf("batched %d (want %d), posts counted %d (sent %d)", e.stats.ChainsBatched, tc.batched, e.stats.Posts, len(sent))
			}
			if tc.chains == 0 {
				return
			}
			e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g1"}})
			chain := e.gigs["g1"]
			if chain == nil || chain.status != gigOpen || chain.mode != modeChain || first.legs[0].gig != chain || second.legs[0].gig != chain {
				t.Fatalf("the chain is not in the books: %+v", chain)
			}
			if first.chainTries != 1 || second.chainTries != 1 {
				t.Errorf("both orders have had one attempt, got %d and %d", first.chainTries, second.chainTries)
			}
			e.adoptOrphans()
			if more := e.Drain(); len(more) != 0 {
				t.Errorf("both cars are in the chain: nothing more to send, got %+v", more)
			}
		})
	}
}

// A gig that is already in the app is still withdrawn when it becomes a partner: only a
// post that has not been sent can be taken back from the outbox.
func TestBatchChainStillWithdrawsAnOpenPartner(t *testing.T) {
	e := chainEngine(t)
	e.P.ChainBatch = 1
	deadline := cal.At(0, 20*cal.Hour)
	first := chainOrder(e, "R1", "a", "b", e.now, deadline)
	e.dispatch(first)
	e.Resolve(e.Drain(), []api.ActionResult{{OK: true, ID: "g1"}})
	second := chainOrder(e, "R2", "b", "a", e.now, deadline)
	e.dispatch(second)
	sent := e.Drain()
	if singles, chains, withdraws := postsOf(sent); singles != 0 || chains != 1 || withdraws != 1 || sent[0].Action.GigID != "g1" {
		t.Fatalf("want withdraw g1 + one chain, got %+v", sent)
	}
	if e.stats.ChainsBatched != 0 {
		t.Errorf("an open partner is not a batch pairing, counted %d", e.stats.ChainsBatched)
	}
}

// bookedChain sets up a chain that a driver booked 20 minutes after it was posted.
func bookedChain(t *testing.T, recoverMin int, deadline int) (*Engine, *gigBook) {
	t.Helper()
	e := chainEngine(t)
	e.P.ChainRecoverMin = recoverMin
	first := chainOrder(e, "R1", "a", "b", e.now, deadline)
	e.dispatch(first)
	e.Resolve(e.Drain(), []api.ActionResult{{OK: true, ID: "g1"}})
	second := chainOrder(e, "R2", "b", "a", e.now, deadline)
	e.dispatch(second)
	e.Resolve(e.Drain(), []api.ActionResult{{OK: true, ID: "g1"}, {OK: true, ID: "g2"}})
	chain := e.gigs["g2"]
	if chain == nil || chain.mode != modeChain {
		t.Fatalf("no chain to book: %+v", chain)
	}
	e.SetNow(e.now + 20)
	e.Handle([]api.Event{{Kind: api.KindGigBooked, At: e.now, GigID: "g2", DriverID: "d1", FeeCents: 5000}})
	if sent := e.Drain(); len(sent) != 0 || chain.status != gigBooked {
		t.Fatalf("a booked chain needs nothing, got %+v (status %s)", sent, chain.status)
	}
	return e, chain
}

// A driver took the chain and gave it back long after its first attempt ran out. Without a
// second attempt the chain is taken apart in the same minute; with ChainRecoverMin it stays
// in the app that long, because one driver already wanted it.
func TestDroppedChainGetsANewAttempt(t *testing.T) {
	deadline := cal.At(1, 20*cal.Hour)
	tests := []struct {
		name       string
		recoverMin int
		event      string
		kept       bool
	}{
		{"off: taken apart at once", 0, api.KindGigDropped, false},
		{"a dropped chain stays an hour", 60, api.KindGigDropped, true},
		{"a no-show chain stays an hour", 60, api.KindNoShow, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, chain := bookedChain(t, tc.recoverMin, deadline)
			e.SetNow(e.now + 200) // long past the first attempt's 90 minutes
			e.Handle([]api.Event{{Kind: tc.event, At: e.now, GigID: "g2", DriverID: "d1"}})
			sent := e.Drain()
			singles, chains, withdraws := postsOf(sent)
			if !tc.kept {
				if withdraws != 1 || singles != 2 || chains != 0 || chain.status != gigWithdrawn {
					t.Fatalf("want the chain withdrawn and two single posts, got %d withdraws, %d singles, %d chains (status %s)", withdraws, singles, chains, chain.status)
				}
				return
			}
			if withdraws != 1 || singles != 0 || chains != 1 || sent[0].Action.GigID != "g2" {
				t.Fatalf("want withdraw g2 + the chain posted again, got %+v", sent)
			}
			// The fee clock starts over: the new chain opens where the first one did.
			if got, was := sent[1].Action.Schedule[0].FeeCents, chain.post.Schedule[0].FeeCents; got != was {
				t.Errorf("the new chain opens at %d, the first one opened at %d", got, was)
			}
			e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g2"}, {OK: true, ID: "g3"}})
			again := e.gigs["g3"]
			if again == nil || again.status != gigOpen || again.mode != modeChain {
				t.Fatalf("the new chain is not in the books: %+v", again)
			}
			if want := e.now + tc.recoverMin; again.chainUntil != want || again.bookableUntil != want {
				t.Errorf("the new attempt ends at %d (bookable until %d), want %d", again.chainUntil, again.bookableUntil, want)
			}
			if e.stats.ChainsRecovered != 1 {
				t.Errorf("recovered chains counted %d, want 1", e.stats.ChainsRecovered)
			}
			if wake := e.NextWake(); wake != again.chainUntil+1 {
				t.Errorf("the engine should wake when the new attempt ends (%d), got %d", again.chainUntil+1, wake)
			}
			// Nobody takes it this time: the cars go alone.
			e.SetNow(again.chainUntil + 1)
			e.Handle(nil)
			if singles, chains, withdraws := postsOf(e.Drain()); withdraws != 1 || singles != 2 || chains != 0 {
				t.Errorf("after the new attempt: want one withdraw and two single posts, got %d, %d and %d chains", withdraws, singles, chains)
			}
		})
	}
}

// A chain given back with too little room left for another attempt is taken apart at once:
// each car must still find a driver of its own.
func TestDroppedChainWithLittleRoomIsTakenApart(t *testing.T) {
	deadline := cal.At(0, 20*cal.Hour)
	e, chain := bookedChain(t, 60, deadline)
	// Move to where less than ChainSlackMin is left before the chain's last start.
	e.SetNow(chain.chainLast - e.P.ChainSlackMin + 10)
	e.Handle([]api.Event{{Kind: api.KindGigDropped, At: e.now, GigID: "g2", DriverID: "d1"}})
	singles, _, withdraws := postsOf(e.Drain())
	if withdraws != 1 || singles != 2 || chain.status != gigWithdrawn || e.stats.ChainsRecovered != 0 {
		t.Fatalf("want the chain taken apart: %d withdraws, %d singles, status %s, recovered %d", withdraws, singles, chain.status, e.stats.ChainsRecovered)
	}
}

// Both switches are off unless a level turns them on.
func TestChainExtrasAreSwitches(t *testing.T) {
	p := DefaultParams()
	if p.ChainBatch != 0 || p.ChainRecoverMin != 0 {
		t.Errorf("base defaults: chainBatch %d, chainRecoverMin %d, want both 0", p.ChainBatch, p.ChainRecoverMin)
	}
}

// Two cars whose chain just ended are posted in one step: they are not put back together,
// although each still has a chain attempt left.
func TestSplitCarsAreNotPairedAgainInTheSameStep(t *testing.T) {
	e := chainEngine(t)
	e.P.ChainBatch, e.P.ChainTries = 1, 2
	deadline := cal.At(1, 20*cal.Hour)
	first := chainOrder(e, "R1", "a", "b", e.now, deadline)
	second := chainOrder(e, "R2", "b", "a", e.now, deadline)
	e.dispatch(first)
	e.dispatch(second)
	e.Resolve(e.Drain(), []api.ActionResult{{OK: true, ID: "g1"}})
	chain := e.gigs["g1"]
	if chain == nil || chain.mode != modeChain {
		t.Fatalf("no chain to end: %+v", chain)
	}
	e.SetNow(chain.chainUntil + 1)
	e.Handle(nil)
	if singles, chains, withdraws := postsOf(e.Drain()); withdraws != 1 || singles != 2 || chains != 0 {
		t.Errorf("after the attempt: want one withdraw and two single posts, got %d withdraws, %d singles, %d chains", withdraws, singles, chains)
	}
}

// A chain made at night waits for the drivers: its attempt and its fee clock count from
// 07:00, with an hour more for the slow start of the day.
func TestNightChainCountsFromTheMorning(t *testing.T) {
	tests := []struct {
		name      string
		night     int
		hour      int
		wantUntil func(now int) int
		opensLow  bool
	}{
		{"off: ninety minutes from the post", 0, 23, func(now int) int { return now + 90 }, false},
		{"late evening: from the next morning", 1, 23, func(int) int { return cal.At(1, 7*cal.Hour) + 150 }, true},
		{"after midnight: from this morning", 1, 1, func(int) int { return cal.At(1, 7*cal.Hour) + 150 }, true},
		{"in the daytime nothing changes", 1, 10, func(now int) int { return now + 90 }, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := chainEngine(t)
			e.P.ChainNight = tc.night
			day := 0
			if tc.hour < 7 || tc.hour == 10 {
				day = 1
			}
			e.now = cal.At(day, tc.hour*cal.Hour)
			deadline := cal.At(3, 20*cal.Hour)
			first := chainOrder(e, "R1", "a", "b", e.now, deadline)
			e.dispatch(first)
			e.Resolve(e.Drain(), []api.ActionResult{{OK: true, ID: "g1"}})
			second := chainOrder(e, "R2", "b", "a", e.now, deadline)
			e.dispatch(second)
			sent := e.Drain()
			e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g1"}, {OK: true, ID: "g2"}})
			chain := e.gigs["g2"]
			if chain == nil || chain.mode != modeChain {
				t.Fatalf("no chain: %+v", sent)
			}
			if want := tc.wantUntil(e.now); chain.chainUntil != want || chain.bookableUntil != want {
				t.Errorf("the attempt ends at %s (bookable until %s), want %s", cal.Format(chain.chainUntil), cal.Format(chain.bookableUntil), cal.Format(want))
			}
			plan := chain.post.Schedule
			if plan[0].At != e.now {
				t.Fatalf("the first fee step must start now, got %d", plan[0].At)
			}
			if tc.opensLow {
				// The opening fee holds until the morning; the first rise comes after 07:00.
				if len(plan) < 2 || plan[1].At <= cal.At(1, 7*cal.Hour) {
					t.Errorf("the fee should not rise before the morning: %+v", plan[:min(3, len(plan))])
				}
			} else if len(plan) < 2 || plan[1].At > e.now+20 {
				t.Errorf("in the daytime the fee rises after a quarter of an hour: %+v", plan[:min(3, len(plan))])
			}
		})
	}
}
