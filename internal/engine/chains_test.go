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
	plan, ok := e.planChain([]*orderBook{out, back})
	if !ok {
		t.Fatal("a return trip with a whole day of room should chain")
	}
	if len(plan.orders) != 2 || plan.orders[0] != out || plan.orders[1] != back || plan.startBy <= e.now {
		t.Errorf("plan = %+v", plan)
	}
	// The second car must still be collected in time after the first drive and the hop.
	if latest := e.lastPickupOf(back, []int{0}); plan.startBy+e.driveMin(&out.order.Legs[0])+plan.hop > latest {
		t.Errorf("start by %s leaves the second car past its last pickup %s", cal.Format(plan.startBy), cal.Format(latest))
	}
	tight := chainOrder(e, "R3", "b", "a", e.now, e.now+90) // 90 minutes to its deadline: no room
	if _, ok := e.planChain([]*orderBook{out, tight}); ok {
		t.Error("a second car with no room must not be chained")
	}
	// Three cars: out, back, out again. All have the whole day, so the chain holds; the
	// third car's pickup comes two drives and two hops after the first.
	again := chainOrder(e, "R5", "a", "b", e.now, deadline)
	three, ok := e.planChain([]*orderBook{out, back, again})
	if !ok || len(three.orders) != 3 || three.startBy > plan.startBy {
		t.Errorf("three-car plan = %+v ok=%v; it can start no later than the two-car one (%d)", three, ok, plan.startBy)
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

// After a restart an open chain is still an attempt: it ends on time and its cars go alone,
// instead of waiting in the app until the first car's last pickup.
func TestRejoinKeepsAnOpenChainAnAttempt(t *testing.T) {
	e := chainEngine(t)
	deadline := cal.At(0, 20*cal.Hour)
	out := chainOrder(e, "R1", "a", "b", e.now, deadline)
	back := chainOrder(e, "R2", "b", "a", e.now, deadline)
	plan, ok := e.planChain([]*orderBook{out, back})
	if !ok {
		t.Fatal("the two orders should chain")
	}
	postedAt := e.now - 30
	window := func(ob *orderBook) *api.TimeWindow {
		return bufferedDelivery(&ob.order.Legs[0], ob.deadlineOf(), e.buffer(ob, []int{0}))
	}
	snap := api.Gig{ID: "g9", Status: snapOpen, FeeCents: 5400, PostedAt: postedAt,
		Schedule: []api.FeeStep{{At: postedAt, FeeCents: 5400}},
		Stops: []api.StopRef{
			{OrderID: "R1", Leg: 0, Kind: stopPickup}, {OrderID: "R1", Leg: 0, Kind: stopDelivery, Window: window(out)},
			{OrderID: "R2", Leg: 0, Kind: stopPickup}, {OrderID: "R2", Leg: 0, Kind: stopDelivery, Window: window(back)},
		}}
	e.adoptGig(&snap)
	g := e.gigs["g9"]
	if g == nil || g.mode != modeChain || len(g.legs) != 2 {
		t.Fatalf("an open gig with the cars of two orders is a chain, got %+v", g)
	}
	if want := min(plan.startBy, postedAt+e.P.ChainTryMin); g.chainUntil != want || g.bookableUntil != want || g.chainLast != plan.startBy {
		t.Errorf("attempt ends %d (bookable until %d, start by %d), want %d and start by %d", g.chainUntil, g.bookableUntil, g.chainLast, want, plan.startBy)
	}
	if out.chainTries != 1 || back.chainTries != 1 {
		t.Errorf("both orders have had their attempt, got %d and %d", out.chainTries, back.chainTries)
	}
	e.SetNow(g.chainUntil + 1)
	e.Handle(nil)
	sent := e.Drain()
	posts := 0
	for _, p := range sent {
		if p.Action.Type == api.ActPost && len(p.Action.Stops) == 2 {
			posts++
		}
	}
	if len(sent) != 3 || sent[0].Action.Type != api.ActWithdraw || sent[0].Action.GigID != "g9" || posts != 2 {
		t.Errorf("want withdraw g9 + two single posts, got %+v", sent)
	}
}

// A chain that a driver already holds is adopted as booked: nothing is posted for its cars.
func TestRejoinKeepsABookedChain(t *testing.T) {
	e := chainEngine(t)
	deadline := cal.At(0, 20*cal.Hour)
	out := chainOrder(e, "R1", "a", "b", e.now, deadline)
	back := chainOrder(e, "R2", "b", "a", e.now, deadline)
	driver := "drv1"
	snap := api.Gig{ID: "g9", Status: snapBooked, FeeCents: 5400, PostedAt: e.now - 30, DriverID: &driver,
		Stops: []api.StopRef{
			{OrderID: "R1", Leg: 0, Kind: stopPickup}, {OrderID: "R1", Leg: 0, Kind: stopDelivery},
			{OrderID: "R2", Leg: 0, Kind: stopPickup}, {OrderID: "R2", Leg: 0, Kind: stopDelivery},
		}}
	e.adoptGig(&snap)
	g := e.gigs["g9"]
	if g == nil || g.status != gigBooked || out.legs[0].gig != g || back.legs[0].gig != g {
		t.Fatalf("booked chain not in the books: %+v", g)
	}
	e.adoptOrphans()
	if sent := e.Drain(); len(sent) != 0 {
		t.Errorf("both cars have a driver: nothing to send, got %+v", sent)
	}
}

func TestChainChanceFallsWithLengthAndLateInTheWeek(t *testing.T) {
	short := chainChanceOf(120, 20, 10, false, 0, 0, 50)
	cases := []struct {
		name   string
		chance float64
	}{
		{"a long drive", chainChanceOf(480, 20, 10, false, 0, 0, 50)},
		{"an end far from the start", chainChanceOf(120, 400, 10, false, 0, 0, 50)},
		{"a car that must go today", chainChanceOf(120, 20, 10, true, 0, 0, 50)},
		{"a partner that has waited", chainChanceOf(120, 20, 10, false, 200, 0, 50)},
		{"a Friday", chainChanceOf(120, 20, 10, false, 0, 4, 50)},
		{"no drivers at the end", chainChanceOf(120, 20, 10, false, 0, 0, 0)},
	}
	if short < 0.8 || short > 1 {
		t.Fatalf("a short round trip on a Monday should book nearly always, got %.2f", short)
	}
	for _, c := range cases {
		if c.chance >= short || c.chance <= 0 {
			t.Errorf("%s should lower the chance below %.2f, got %.2f", c.name, short, c.chance)
		}
	}
}

// With a bar on the chance a chain is only tried when a driver is likely to take it.
func TestUnlikelyChainIsNotTried(t *testing.T) {
	e := chainEngine(t)
	deadline := cal.At(0, 20*cal.Hour)
	first := chainOrder(e, "R1", "a", "b", e.now, deadline)
	e.dispatch(first)
	sent := e.Drain()
	e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g1"}})
	second := chainOrder(e, "R2", "b", "a", e.now, deadline)
	plan, ok := e.planChain([]*orderBook{first, second})
	if !ok {
		t.Fatal("the two orders should chain")
	}
	chance := e.chainChance(plan)
	if chance <= 0 || chance >= 1 {
		t.Fatalf("chance = %v", chance)
	}
	e.P.ChainMinChance = chance + 0.01
	e.dispatch(second)
	sent = e.Drain()
	if len(sent) != 1 || sent[0].Action.Type != api.ActPost || len(sent[0].Action.Stops) != 2 {
		t.Fatalf("below the bar the order goes alone, got %+v", sent)
	}
	if second.chainTries != 0 || e.stats.ChainsSkipped != 1 {
		t.Errorf("a skipped chain is not an attempt: tries %d, skipped %d", second.chainTries, e.stats.ChainsSkipped)
	}
}

// A request that would make a chain with an open gig's car is cheaper to carry: one
// driver moves both. Its price is set on a lower cost.
func TestChainProspectLowersTheCostOfARequest(t *testing.T) {
	e := chainEngine(t)
	e.P.ChainCostGain = 0.3
	deadline := cal.At(0, 20*cal.Hour)
	req := testRequest("R2", e.now)
	req.Legs[0].Pickup.PlaceID, req.Legs[0].Delivery.PlaceID = "b", "a"
	opt := api.DeliveryOption{Kind: api.SameDay, Deadline: deadline, ListPriceCents: 6000}
	req.Options = []api.DeliveryOption{opt}
	if p := e.chainProspect(req, opt); p != 0 {
		t.Fatalf("no open gig yet: prospect %v", p)
	}
	alone := e.costMultFor(req, opt)

	first := chainOrder(e, "R1", "a", "b", e.now, deadline)
	e.dispatch(first)
	sent := e.Drain()
	e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g1"}})
	p := e.chainProspect(req, opt)
	if p <= 0.3 || p >= 1 {
		t.Fatalf("an open return trip is a good prospect, got %v", p)
	}
	if got, want := e.costMultFor(req, opt), alone-0.3*p; got < want-1e-9 || got > want+1e-9 {
		t.Errorf("cost multiple %v, want %v (alone %v, prospect %v)", got, want, alone, p)
	}
	if len(e.orders) != 1 || first.chainTries != 0 || e.gigs["g1"].status != gigOpen {
		t.Error("looking at a prospect must not touch the books")
	}
	e.P.ChainCostGain = 0
	if got := e.costMultFor(req, opt); got != alone {
		t.Errorf("switched off, the cost multiple is %v, want %v", got, alone)
	}
}

// The fee of a gig with the cars of two orders is one ledger line without an order: the
// report shares it between the orders by their typical fees.
func TestChainFeeIsSharedBetweenItsOrders(t *testing.T) {
	e := chainEngine(t)
	deadline := cal.At(0, 20*cal.Hour)
	out := chainOrder(e, "R1", "a", "b", e.now, deadline)
	back := chainOrder(e, "R2", "b", "a", e.now, deadline)
	back.req.ReferenceFeeCents = 1000 // a quarter of the two typical fees
	// Both cars are delivered when the fee is paid: the gig's own list of cars is empty by then.
	g := &gigBook{id: "g1", status: gigDone, post: api.Action{Type: api.ActPost, Stops: []api.StopRef{
		{OrderID: "R1", Leg: 0, Kind: stopPickup}, {OrderID: "R1", Leg: 0, Kind: stopDelivery},
		{OrderID: "R2", Leg: 0, Kind: stopPickup}, {OrderID: "R2", Leg: 0, Kind: stopDelivery},
	}}}
	e.gigs["g1"] = g
	gid := "g1"
	e.onLedger(&api.LedgerLine{Kind: "driver_fee", Cents: -4000, GigID: &gid})
	if out.feesPaid != 3000 || back.feesPaid != 1000 {
		t.Errorf("fees paid = %d and %d, want 3000 and 1000", out.feesPaid, back.feesPaid)
	}
}

// After a chain attempt that no driver took, the cars go alone on a fee clock that kept
// running: they do not open at the lowest fee again.
func TestSplitCarsKeepTheFeeClock(t *testing.T) {
	for _, c := range []struct {
		share float64
		want  float64 // opening fee over the typical fee after a 90-minute attempt
		slack float64 // steps sit on a 5-minute grid, so half the time can show one step less
	}{{0, 0.8, 0.011}, {1, 1.1, 0.011}, {0.5, 0.925, 0.036}} {
		e := chainEngine(t)
		e.P.SplitClock = c.share
		deadline := cal.At(1, 20*cal.Hour)
		first := chainOrder(e, "R1", "a", "b", e.now, deadline)
		e.dispatch(first)
		e.Resolve(e.Drain(), []api.ActionResult{{OK: true, ID: "g1"}})
		second := chainOrder(e, "R2", "b", "a", e.now, deadline)
		e.dispatch(second)
		e.Resolve(e.Drain(), []api.ActionResult{{OK: true, ID: "g1"}, {OK: true, ID: "g2"}})
		chain := e.gigs["g2"]
		if chain == nil || chain.chainUntil != e.now+e.P.ChainTryMin {
			t.Fatalf("share %v: no 90-minute chain attempt: %+v", c.share, chain)
		}
		e.SetNow(chain.chainUntil + 1)
		e.Handle(nil)
		for _, p := range e.Drain() {
			if p.Action.Type != api.ActPost {
				continue
			}
			got := float64(p.Action.Schedule[0].FeeCents) / 3000
			if got < c.want-c.slack || got > c.want+c.slack {
				t.Errorf("share %v: a split car opens at %.2f of its typical fee, want %.2f", c.share, got, c.want)
			}
			if p.Action.Schedule[0].At != e.now {
				t.Errorf("share %v: the first step starts now, got %d", c.share, p.Action.Schedule[0].At)
			}
		}
	}
}
