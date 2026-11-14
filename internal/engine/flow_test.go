package engine

import (
	"testing"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

// flowEngine is an engine at a level with quotes, in the morning of the first day.
func flowEngine(t *testing.T) *Engine {
	t.Helper()
	e := New(5, DefaultParamsFor(5), testMap(t))
	e.now, e.endsAt = cal.At(0, 10*cal.Hour), 14280
	return e
}

// winOrder walks one request through quote, win and post, and returns the order's book
// with its gig open in the app under the given id.
func winOrder(t *testing.T, e *Engine, id, gigID string) *orderBook {
	t.Helper()
	req := testRequest(id, e.now)
	e.Handle([]api.Event{{Kind: api.KindPriceRequest, At: e.now, Request: req}})
	quotes := e.Drain()
	if len(quotes) != 1 || quotes[0].Action.Type != api.ActQuote || quotes[0].Action.RequestID != id {
		t.Fatalf("a request gets one quote, got %+v", quotes)
	}
	e.Resolve(quotes, []api.ActionResult{{OK: true, ID: id}})
	e.Handle([]api.Event{{Kind: api.KindOrderWon, At: e.now, Order: testOrder(req, "open", nil)}})
	posts := e.Drain()
	if len(posts) != 1 || posts[0].Action.Type != api.ActPost || len(posts[0].Action.Stops) != 2 {
		t.Fatalf("a won order gets one gig with a pickup and a delivery, got %+v", posts)
	}
	e.Resolve(posts, []api.ActionResult{{OK: true, ID: gigID}})
	ob := e.orders[id]
	if ob == nil || ob.legs[0].gig == nil || ob.legs[0].gig.id != gigID || ob.legs[0].gig.status != gigOpen {
		t.Fatalf("order %s should have the open gig %s", id, gigID)
	}
	return ob
}

func countType(sent []Pending, kind string) int {
	n := 0
	for _, p := range sent {
		if p.Action.Type == kind {
			n++
		}
	}
	return n
}

// The whole life of an order that goes well: nothing more is sent once a driver has it,
// and the account's lines land on the order.
func TestOrderFromRequestToDelivery(t *testing.T) {
	e := flowEngine(t)
	ob := winOrder(t, e, "R1", "g1")
	quote := e.stats.Quoted
	if quote != 1 || e.stats.Won != 1 || e.stats.Posts != 1 {
		t.Fatalf("stats after the win: quoted %d, won %d, posts %d", quote, e.stats.Won, e.stats.Posts)
	}

	e.SetNow(e.now + 5)
	e.Handle([]api.Event{
		{Kind: api.KindGigViewed, At: e.now, GigID: "g1", DriverID: "d1"},
		{Kind: api.KindGigBooked, At: e.now, GigID: "g1", DriverID: "d1", FeeCents: 2400},
	})
	g := e.gigs["g1"]
	if g.status != gigBooked || g.driver != "d1" || g.fee != 2400 || g.views != 1 {
		t.Fatalf("booked gig: %+v", g)
	}
	if ob.bookings != 1 || ob.firstBookedAt != e.now {
		t.Errorf("order after the booking: bookings %d, first booked at %d", ob.bookings, ob.firstBookedAt)
	}
	if sent := e.Drain(); len(sent) != 0 {
		t.Errorf("a booked car needs nothing, got %+v", sent)
	}

	e.SetNow(e.now + 60)
	e.Handle([]api.Event{
		{Kind: api.KindGigStarted, At: e.now, GigID: "g1"},
		{Kind: api.KindPickedUp, At: e.now, GigID: "g1", OrderID: "R1", Leg: 0, PlaceID: "a"},
	})
	if g.status != gigWorking || !ob.legs[0].pickedUp || g.pickedAt != e.now {
		t.Errorf("after the pickup: status %s, picked up %v at %d", g.status, ob.legs[0].pickedUp, g.pickedAt)
	}

	e.SetNow(e.now + 35)
	rev, gid := "R1", "g1"
	e.Handle([]api.Event{
		{Kind: api.KindDelivered, At: e.now, GigID: "g1", OrderID: "R1", Leg: 0, PlaceID: "b"},
		{Kind: api.KindLedger, At: e.now, Line: &api.LedgerLine{Kind: "revenue", Cents: 8000, OrderID: &rev}},
		{Kind: api.KindLedger, At: e.now, Line: &api.LedgerLine{Kind: "driver_fee", Cents: -2400, OrderID: &rev, GigID: &gid}},
	})
	if !ob.done() || !ob.paid || ob.refunded || ob.feesPaid != 2400 || g.status != gigDone {
		t.Errorf("after delivery: done %v, paid %v, refunded %v, fees %d, gig %s", ob.done(), ob.paid, ob.refunded, ob.feesPaid, g.status)
	}
	if e.stats.Delivered != 1 || e.stats.LateLegs != 0 {
		t.Errorf("stats: delivered %d, late %d", e.stats.Delivered, e.stats.LateLegs)
	}
	e.SetNow(e.now + 600)
	e.Handle(nil)
	if sent := e.Drain(); len(sent) != 0 {
		t.Errorf("a delivered order is finished, got %+v", sent)
	}
}

// A driver gives the gig back: it is open again with its schedule and must not be posted
// a second time; the next booking counts.
func TestDroppedGigIsOpenAgain(t *testing.T) {
	e := flowEngine(t)
	ob := winOrder(t, e, "R1", "g1")
	e.Handle([]api.Event{{Kind: api.KindGigBooked, At: e.now, GigID: "g1", DriverID: "d1", FeeCents: 2400}})
	e.SetNow(e.now + 30)
	e.Handle([]api.Event{{Kind: api.KindGigDropped, At: e.now, GigID: "g1", DriverID: "d1"}})
	g := e.gigs["g1"]
	if g.status != gigOpen || g.driver != "" || ob.drops != 1 || e.stats.Drops != 1 {
		t.Fatalf("after the drop: status %s, driver %q, drops %d", g.status, g.driver, ob.drops)
	}
	if n := countType(e.Drain(), api.ActPost); n != 0 {
		t.Errorf("the world reopens a dropped gig itself: %d new posts", n)
	}
	e.Handle([]api.Event{{Kind: api.KindNoShow, At: e.now, GigID: "g1", DriverID: "d2"}})
	if e.stats.NoShows != 1 || ob.drops != 2 {
		t.Errorf("a no-show reopens the gig too: no-shows %d, drops %d", e.stats.NoShows, ob.drops)
	}
	e.Handle([]api.Event{{Kind: api.KindGigBooked, At: e.now, GigID: "g1", DriverID: "d3", FeeCents: 2600}})
	if g.status != gigBooked || g.driver != "d3" || ob.bookings != 2 {
		t.Errorf("second booking: status %s, driver %s, bookings %d", g.status, g.driver, ob.bookings)
	}
}

// A gig that leaves the app unbooked, and a visit that fails, both leave a car without a
// gig: it is posted again at once. No car may be left behind.
func TestCarWithoutAGigIsPostedAgain(t *testing.T) {
	e := flowEngine(t)
	ob := winOrder(t, e, "R1", "g1")
	e.SetNow(e.now + 120)
	e.Handle([]api.Event{{Kind: api.KindGigExpired, At: e.now, GigID: "g1"}})
	sent := e.Drain()
	if countType(sent, api.ActPost) != 1 || e.gigs["g1"].status != gigWithdrawn || e.stats.Expired != 1 {
		t.Fatalf("an expired gig's car is posted again, got %+v", sent)
	}
	e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g2"}})
	if ob.legs[0].gig == nil || ob.legs[0].gig.id != "g2" {
		t.Fatal("the car should ride on the new gig g2")
	}

	e.Handle([]api.Event{{Kind: api.KindGigBooked, At: e.now, GigID: "g2", DriverID: "d1", FeeCents: 2500}})
	e.SetNow(e.now + 30)
	e.Handle([]api.Event{{Kind: api.KindVisitFailed, At: e.now, GigID: "g2", OrderID: "R1", Leg: 0, Reason: reasonLicence}})
	sent = e.Drain()
	if countType(sent, api.ActPost) != 1 {
		t.Fatalf("after a failed visit the car is posted again, got %+v", sent)
	}
	post := sent[len(sent)-1].Action
	if post.Audience == nil || !post.Audience.ManualLicence {
		t.Errorf("the car turned out to need a manual licence: the new gig must ask for one, got %+v", post.Audience)
	}
	if !ob.legs[0].needManual || e.stats.FailedVisits[reasonLicence] != 1 {
		t.Errorf("the cause is remembered: manual %v, stats %v", ob.legs[0].needManual, e.stats.FailedVisits)
	}
}

// A dealer cancels: the gig is withdrawn by the world and the order needs nothing more.
func TestCancelledOrderIsLeftAlone(t *testing.T) {
	e := flowEngine(t)
	ob := winOrder(t, e, "R1", "g1")
	e.Handle([]api.Event{{Kind: api.KindOrderCancelled, At: e.now, OrderID: "R1", Gigs: []api.CancelledGig{{GigID: "g1", Outcome: outcomeWithdrawn}}}})
	if !ob.cancelled || e.gigs["g1"].status != gigWithdrawn || e.stats.Cancelled != 1 {
		t.Fatalf("cancelled order: %v, gig %s", ob.cancelled, e.gigs["g1"].status)
	}
	e.SetNow(e.now + 300)
	e.Handle(nil)
	if sent := e.Drain(); len(sent) != 0 {
		t.Errorf("nothing is posted for a cancelled order, got %+v", sent)
	}
}

// The world warns three hours before the last on-time pickup: the order is urgent, takes
// the world's last pickup, and its open gig keeps a fee plan that never falls.
func TestDeadlineWarningMakesTheOrderUrgent(t *testing.T) {
	e := flowEngine(t)
	ob := winOrder(t, e, "R1", "g1")
	g := e.gigs["g1"]
	shown := feeAt(g.plan, e.now)
	last := e.now + 180
	e.SetNow(e.now + 10)
	e.Handle([]api.Event{{Kind: api.KindDeadlineClose, At: e.now, OrderID: "R1", LastPickupAt: last}})
	if !ob.urgent || ob.order.LastPickupAt != last || e.stats.DeadlineWarnings != 1 {
		t.Fatalf("urgent %v, last pickup %d (want %d)", ob.urgent, ob.order.LastPickupAt, last)
	}
	for _, p := range e.Drain() {
		if p.Action.Type == api.ActReprice && p.Action.Schedule[0].FeeCents < shown {
			t.Errorf("a new fee plan must not open below the fee shown (%d): %+v", shown, p.Action.Schedule)
		}
	}
	if ob.legs[0].gig == nil || !ob.legs[0].gig.live() {
		t.Error("the car must still have a live gig after the warning")
	}
}

// A lost quote teaches the market and leaves no order behind.
func TestLostQuoteIsLearnedFrom(t *testing.T) {
	e := flowEngine(t)
	req := testRequest("R1", e.now)
	e.Handle([]api.Event{{Kind: api.KindPriceRequest, At: e.now, Request: req}})
	e.Resolve(e.Drain(), []api.ActionResult{{OK: true, ID: "R1"}})
	before := e.market.mu
	e.Handle([]api.Event{{Kind: api.KindOrderLost, At: e.now + 15, RequestID: "R1"}})
	if e.stats.Lost != 1 || len(e.orders) != 0 || len(e.quotes) != 0 {
		t.Errorf("lost %d, orders %d, open quotes %d", e.stats.Lost, len(e.orders), len(e.quotes))
	}
	if e.market.mu >= before {
		t.Errorf("after a lost quote the level should fall: %.4f -> %.4f", before, e.market.mu)
	}
}

// How a car is posted depends on the time that is left: with room, the delivery window
// ends a buffer before the deadline and the fee starts low; close to the last pickup the
// order's own windows and the top fee; after it, the late plan that starts at the typical
// fee and keeps rising.
func TestPostingModeFollowsTheTimeLeft(t *testing.T) {
	cases := []struct {
		name      string
		now       int
		wantMode  int
		wantFirst float64 // opening fee over the typical fee (3000)
		window    bool    // the delivery stop carries a window of its own
	}{
		{"a day of room", cal.At(0, 10*cal.Hour), modeBuffered, 0.75, true},
		{"minutes before the last pickup", cal.At(1, 18*cal.Hour+50), modeTight, 2.0, false},
		{"after the last pickup", cal.At(1, 19*cal.Hour+30), modeLate, 1.0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := flowEngine(t)
			req := testRequest("R1", cal.At(0, 9*cal.Hour))
			e.reqs["R1"] = req
			e.now = c.now
			e.onWon(testOrder(req, "open", nil), c.now) // last pickup: day 1, 19:00
			sent := e.Drain()
			if len(sent) != 1 || sent[0].Action.Type != api.ActPost {
				t.Fatalf("want one post, got %+v", sent)
			}
			post := sent[0].Action
			g := e.orders["R1"].legs[0].gig
			if g.mode != c.wantMode {
				t.Errorf("mode = %d, want %d", g.mode, c.wantMode)
			}
			if got := float64(post.Schedule[0].FeeCents) / 3000; got < c.wantFirst-0.011 || got > c.wantFirst+0.011 {
				t.Errorf("opening fee %.2f of the typical fee, want %.2f", got, c.wantFirst)
			}
			for i := 1; i < len(post.Schedule); i++ {
				if post.Schedule[i].FeeCents <= post.Schedule[i-1].FeeCents || post.Schedule[i].At <= post.Schedule[i-1].At {
					t.Errorf("fee steps must rise in time and fee: %+v", post.Schedule)
				}
			}
			delivery := post.Stops[1]
			if (delivery.Window != nil) != c.window {
				t.Errorf("delivery window present = %v, want %v", delivery.Window != nil, c.window)
			}
			if c.window {
				// The site closes at 20:00 on the deadline day; the drive is 30 minutes, so the buffer is the 15-minute minimum.
				if want := cal.At(1, 20*cal.Hour) - 15; delivery.Window.To != want {
					t.Errorf("delivery window ends %s, want %s", cal.Format(delivery.Window.To), cal.Format(want))
				}
				if g.bookableUntil != cal.At(1, 19*cal.Hour)-15 {
					t.Errorf("bookable until %s, want 15 minutes before the last pickup", cal.Format(g.bookableUntil))
				}
			}
		})
	}
}
