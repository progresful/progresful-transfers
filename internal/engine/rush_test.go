package engine

import (
	"testing"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

// rushEngine is an engine on a Monday morning with one dealer that sends most requests.
func rushEngine(t *testing.T, share float64) *Engine {
	t.Helper()
	p := DefaultParamsFor(8)
	p.Chains = 0
	p.TrustRush = share
	e := New(8, p, testMap(t))
	e.now, e.endsAt = cal.At(0, 10*cal.Hour), 14280
	e.stats.Requests = 40
	e.market.dealer("D01").requests = 20 // half of all requests
	e.market.dealer("D02").requests = 1
	return e
}

func postOf(t *testing.T, e *Engine) api.Action {
	t.Helper()
	for _, p := range e.Drain() {
		if p.Action.Type == api.ActPost {
			return p.Action
		}
	}
	t.Fatal("no gig was posted")
	return api.Action{}
}

func deliveryEnd(a api.Action) int {
	for _, s := range a.Stops {
		if s.Kind == stopDelivery && s.Window != nil {
			return s.Window.To
		}
	}
	return -1
}

// A car of a big dealer that is ready today and due tomorrow is posted to arrive today:
// trust is counted at the delivery, so a day earlier lifts a day of that dealer's prices.
func TestTrustRushPostsABigDealersCarForToday(t *testing.T) {
	tests := []struct {
		name   string
		share  float64
		dealer string
		trust  int
		hour   int
		day    int
		today  bool
	}{
		{"big dealer in the morning", 0.08, "D01", 0, 10, 0, true},
		{"switched off", 0, "D01", 0, 10, 0, false},
		{"small dealer", 0.08, "D02", 0, 10, 0, false},
		{"trust already full", 0.08, "D01", maxTrust, 10, 0, false},
		{"too late in the day", 0.08, "D01", 0, 18, 0, false},
		{"friday: no later request to sell into", 0.08, "D01", 0, 10, 4, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := rushEngine(t, tc.share)
			e.now = cal.At(tc.day, tc.hour*cal.Hour)
			e.market.dealer(tc.dealer).trust = tc.trust
			deadline := cal.At(tc.day+1, 20*cal.Hour)
			if tc.day == 4 {
				deadline = cal.At(7, 20*cal.Hour) // a Friday's car is due on the Monday
			}
			ob := chainOrder(e, "R1", "a", "b", e.now, deadline)
			ob.order.DealerID, ob.req.DealerID = tc.dealer, tc.dealer
			e.dispatch(ob)
			end := deliveryEnd(postOf(t, e))
			g := ob.legs[0].gig
			if g == nil || (g.mode == modeRush) != tc.today {
				t.Fatalf("gig %+v, want a rush gig = %v", g, tc.today)
			}
			if tc.today && (end < 0 || end > cal.At(tc.day, 20*cal.Hour)) {
				t.Errorf("delivery window ends at %s, want it today", cal.Format(end))
			}
			if !tc.today && end >= 0 && end <= cal.At(tc.day, 20*cal.Hour) && deadline > cal.At(tc.day+1, 0) && tc.hour < 18 {
				t.Errorf("delivery window ends at %s, want it by the order's deadline", cal.Format(end))
			}
		})
	}
}

// When nobody can take the car for today any more, the gig is posted again with the
// order's own deadline, and not for today a second time.
func TestTrustRushFallsBackToTheDeadline(t *testing.T) {
	e := rushEngine(t, 0.08)
	deadline := cal.At(1, 20*cal.Hour)
	ob := chainOrder(e, "R1", "a", "b", e.now, deadline)
	e.dispatch(ob)
	postOf(t, e)
	g := ob.legs[0].gig
	if g == nil || g.mode != modeRush || g.bookableUntil <= e.now || g.bookableUntil >= cal.At(0, 20*cal.Hour) {
		t.Fatalf("want a rush gig bookable until this afternoon, got %+v", g)
	}
	g.id, g.status = "g1", gigOpen
	e.gigs["g1"] = g
	if wake := e.NextWake(); wake > g.bookableUntil+1 {
		t.Errorf("the engine wakes at %s, after the rush ends at %s", cal.Format(wake), cal.Format(g.bookableUntil))
	}
	e.SetNow(g.bookableUntil + 1)
	e.Handle(nil)
	var again *api.Action
	withdrawn := false
	for _, p := range e.Drain() {
		a := p.Action
		switch a.Type {
		case api.ActWithdraw:
			withdrawn = withdrawn || a.GigID == "g1"
		case api.ActPost:
			again = &a
		}
	}
	if !withdrawn || again == nil {
		t.Fatalf("after the rush the gig should be withdrawn and posted again: withdrawn %v, posted %v", withdrawn, again != nil)
	}
	if end := deliveryEnd(*again); end <= cal.At(0, 20*cal.Hour) || end > deadline {
		t.Errorf("the second gig's delivery window ends at %s, want it by the order's deadline %s", cal.Format(end), cal.Format(deadline))
	}
}

// The rush pays no deadline surge: the fee creeps to the hold fee's top by the end of
// the rush and no further.
func TestTrustRushFeeStaysUnderTheSurge(t *testing.T) {
	e := rushEngine(t, 0.08)
	ob := chainOrder(e, "R1", "a", "b", e.now, cal.At(1, 20*cal.Hour))
	e.dispatch(ob)
	post := postOf(t, e)
	top := 0
	for _, s := range post.Schedule {
		top = max(top, s.FeeCents)
	}
	if limit := feeOf(3000, e.P.HoldMax); top > limit {
		t.Errorf("the rush gig's fee rises to %d, want at most %d (%.2fx)", top, limit, e.P.HoldMax)
	}
}
