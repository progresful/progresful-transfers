package engine

import (
	"testing"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

func chainOrder(e *Engine, id, from, to string, ready, deadline int) *orderBook {
	h := &api.OpeningHours{Opens: 7 * cal.Hour, Closes: 20 * cal.Hour, Weekdays: []int{0, 1, 2, 3, 4}}
	w := api.TimeWindow{From: ready, To: deadline}
	leg := api.Leg{Pickup: api.StopPlace{PlaceID: from, Window: w, Hours: h}, Delivery: api.StopPlace{PlaceID: to, Window: w, Hours: h}}
	req := &api.PriceRequest{ID: id, DealerID: "D01", Legs: []api.Leg{leg}, ReferenceFeeCents: 3000}
	order := api.Order{ID: id, DealerID: "D01", Option: api.NextDay, PriceCents: 6000, Deadline: deadline, Legs: []api.Leg{leg}, DeliveredAt: []*int{nil},
		LastPickupAt: deadline - e.Map.Minutes(from, to)}
	ob := &orderBook{order: order, req: req, legs: make([]legBook, 1)}
	e.orders[id] = ob
	return ob
}

func chainEngine(t *testing.T) *Engine {
	p := DefaultParamsFor(8)
	p.Chains = 1
	e := New(8, p, testMap(t))
	e.now, e.endsAt = cal.At(0, 10*cal.Hour), 14280
	return e
}

func TestPlanChain(t *testing.T) {
	e := chainEngine(t)
	deadline := cal.At(0, 20*cal.Hour)
	out := chainOrder(e, "R1", "a", "b", e.now, deadline)  // a -> b
	back := chainOrder(e, "R2", "b", "a", e.now, deadline) // b -> a: starts where the first ends
	plan, ok := e.planChain(out, back)
	if !ok {
		t.Fatal("a return trip with a whole day of room should chain")
	}
	if plan.first != out || plan.second != back || plan.startBy <= e.now {
		t.Errorf("plan = %+v", plan)
	}
	// The second car must still be collected in time after the first drive and the hop.
	if latest := e.lastPickupOf(back, []int{0}); plan.startBy+e.driveMin(&out.order.Legs[0])+plan.hop > latest {
		t.Errorf("start by %s leaves the second car past its last pickup %s", cal.Format(plan.startBy), cal.Format(latest))
	}
	tight := chainOrder(e, "R3", "b", "a", e.now, e.now+90) // 90 minutes to its deadline: no room
	if _, ok := e.planChain(out, tight); ok {
		t.Error("a second car with no room must not be chained")
	}
	private := chainOrder(e, "R4", "b", "a", e.now, deadline)
	private.order.Legs[0].Pickup.PrivateAddress = true
	if e.chainable(private) {
		t.Error("a private pickup has a booked slot: not for chains")
	}
}

// A new order is chained with an open gig's car: the old gig is withdrawn, one two-car gig
// is posted, and when the attempt ends unbooked both cars are posted again alone.
func TestChainAttemptAndSplit(t *testing.T) {
	e := chainEngine(t)
	deadline := cal.At(0, 20*cal.Hour)
	first := chainOrder(e, "R1", "a", "b", e.now, deadline)
	e.dispatch(first)
	sent := e.Drain()
	if len(sent) != 1 || sent[0].Action.Type != api.ActPost {
		t.Fatalf("the first order is posted alone, got %d actions", len(sent))
	}
	e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g1"}})

	second := chainOrder(e, "R2", "b", "a", e.now, deadline)
	e.dispatch(second)
	sent = e.Drain()
	if len(sent) != 2 || sent[0].Action.Type != api.ActWithdraw || sent[0].Action.GigID != "g1" || sent[1].Action.Type != api.ActPost || len(sent[1].Action.Stops) != 4 {
		t.Fatalf("want withdraw g1 + one four-stop post, got %+v", sent)
	}
	stops := sent[1].Action.Stops
	// Either car may go first (the trip is symmetric here); each must be collected and
	// delivered before the other is touched.
	if stops[0].Kind != stopPickup || stops[1].Kind != stopDelivery || stops[0].OrderID != stops[1].OrderID ||
		stops[2].Kind != stopPickup || stops[3].Kind != stopDelivery || stops[2].OrderID != stops[3].OrderID || stops[0].OrderID == stops[2].OrderID {
		t.Errorf("chain stops out of order: %+v", stops)
	}
	e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g1"}, {OK: true, ID: "g2"}})
	chain := e.gigs["g2"]
	if chain == nil || chain.status != gigOpen || chain.mode != modeChain || first.legs[0].gig != chain || second.legs[0].gig != chain {
		t.Fatalf("chain gig not in the books: %+v", chain)
	}
	if wake := e.NextWake(); wake != chain.chainUntil+1 {
		t.Errorf("the engine should wake when the attempt ends (%d), got %d", chain.chainUntil+1, wake)
	}

	// Nobody books it: at the end of the attempt both cars go alone, and are not chained again.
	e.SetNow(chain.chainUntil + 1)
	e.Handle(nil)
	sent = e.Drain()
	posts := 0
	for _, p := range sent {
		if p.Action.Type == api.ActPost {
			posts++
			if len(p.Action.Stops) != 2 {
				t.Errorf("after the split each car has its own gig, got %d stops", len(p.Action.Stops))
			}
		}
	}
	if posts != 2 || sent[0].Action.Type != api.ActWithdraw || sent[0].Action.GigID != "g2" {
		t.Errorf("want withdraw g2 + two single posts, got %+v", sent)
	}
}
