package engine

import (
	"fmt"
	"math"
	"testing"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

// laneEngine is a level 8 engine whose test lane a to b has a fee of 50.00 in the table.
func laneEngine(t *testing.T, level int, refCap float64, price int) *Engine {
	t.Helper()
	laneLogRef["a|b"] = math.Log(5000)
	t.Cleanup(func() { delete(laneLogRef, "a|b") })
	p := DefaultParamsFor(level)
	p.RefCap, p.RefCapPrice, p.RefCapRisk = refCap, price, 0 // each test turns on what it tests
	p.RiskMax = 1                                            // the test map has no drivers
	e := New(level, p, testMap(t))
	e.now, e.endsAt = cal.At(0, 10*cal.Hour), 14280
	return e
}

func laneRequest(id string, now, fee int) *api.PriceRequest {
	req := testRequest(id, now)
	req.ReferenceFeeCents = fee
	return req
}

// seeWorld feeds n requests whose typical fee is 1.1 times the lane's: the world's factor.
func seeWorld(e *Engine, n int) {
	for i := 0; i < n; i++ {
		e.onRequest(laneRequest(fmt.Sprintf("S%02d", i), e.now, 5500))
	}
	e.Drain()
}

// The typical fee of a request is capped at the lane's fee times the world's factor and
// the allowance; one at or under the lane's is left alone.
func TestRefCapLearnsTheWorldAndCapsHighFees(t *testing.T) {
	e := laneEngine(t, 8, 1.04, 0)
	seeWorld(e, 20)
	high, fair := laneRequest("R1", e.now, 7000), laneRequest("R2", e.now, 5000)
	e.onRequest(high)
	e.onRequest(fair)
	if got := e.fairRef(high); got != 5720 {
		t.Errorf("a typical fee of 7000 on a lane worth 5500 here: capped at %d, want 5720", got)
	}
	if got := e.fairRef(fair); got != 5000 {
		t.Errorf("a typical fee under the lane's is left alone, got %d", got)
	}
	if e.stats.RefCapped != 1 {
		t.Errorf("capped requests counted %d, want 1", e.stats.RefCapped)
	}
}

// Nothing is capped without the switch, below level 8, on a lane the table does not
// have, for an order of two cars, or before twenty requests have shown the world's factor.
func TestRefCapLeavesTheRestAlone(t *testing.T) {
	tests := []struct {
		name   string
		level  int
		refCap float64
		seen   int
		change func(*api.PriceRequest)
	}{
		{"switch off", 8, 0, 20, nil},
		{"level 7", 7, 1.04, 20, nil},
		{"too few requests seen", 8, 1.04, 19, nil},
		{"a lane that is not in the table", 8, 1.04, 20, func(r *api.PriceRequest) { r.Legs[0].Delivery.PlaceID = "a" }},
		{"two cars", 8, 1.04, 20, func(r *api.PriceRequest) { r.Legs = append(r.Legs, r.Legs[0]) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := laneEngine(t, tc.level, tc.refCap, 0)
			seeWorld(e, tc.seen)
			req := laneRequest("R1", e.now, 7000)
			if tc.change != nil {
				tc.change(req)
			}
			e.onRequest(req)
			if got := e.fairRef(req); got != 7000 {
				t.Errorf("fairRef %d, want the request's own 7000", got)
			}
		})
	}
}

// The fee clock of the won order opens on the capped fee; the quote does not change
// unless RefCapPrice says so; and after a restart the order goes by its own fee again.
func TestRefCapAnchorsTheFeeClockNotTheQuote(t *testing.T) {
	quoteAndPost := func(refCap float64, price int) (quote, opening int, e *Engine, ob *orderBook) {
		e = laneEngine(t, 8, refCap, price)
		seeWorld(e, 20)
		req := laneRequest("R1", e.now, 7000)
		e.onRequest(req)
		sent := e.Drain()
		if len(sent) != 1 || sent[0].Action.Type != api.ActQuote {
			t.Fatalf("want one quote, got %+v", sent)
		}
		quote = sent[0].Action.Prices[api.NextDay]
		order := testOrder(req, "open", nil)
		order.PriceCents = quote
		e.onWon(order, e.now)
		for _, p := range e.Drain() {
			if p.Action.Type == api.ActPost {
				opening = p.Action.Schedule[0].FeeCents
			}
		}
		return quote, opening, e, e.orders["R1"]
	}
	plainQuote, plainOpening, _, _ := quoteAndPost(0, 0)
	quote, opening, e, ob := quoteAndPost(1.04, 0)
	if quote != plainQuote {
		t.Errorf("the quote moved from %d to %d although only the fee clock is capped", plainQuote, quote)
	}
	if got := e.baseFee(ob, []int{0}); got != 5720 {
		t.Errorf("fee base %d, want the capped 5720", got)
	}
	if want := feeOf(5720, e.P.FeeStart); opening != want || opening >= plainOpening {
		t.Errorf("the gig opens at %d, want %d, under the %d of the uncapped clock", opening, want, plainOpening)
	}
	if priced, _, _, _ := quoteAndPost(1.04, 1); priced >= plainQuote {
		t.Errorf("with the cost on the capped fee the quote should fall: %d against %d", priced, plainQuote)
	}
	// A restart: the engine has the order but not what it learned about the lane.
	again := laneEngine(t, 8, 1.04, 0)
	held := &orderBook{order: ob.order, req: ob.req, legs: make([]legBook, 1)}
	if got := again.baseFee(held, []int{0}); got != 7000 {
		t.Errorf("after a restart the fee base is %d, want the request's own 7000", got)
	}
}

// With RefCapRisk the lateness model reads the capped fee: a request whose typical fee is
// above its lane's no longer looks safer than the lane is.
func TestRefCapRiskFeedsTheCappedFeeToTheModel(t *testing.T) {
	riskOf := func(refCapRisk int) float64 {
		e := laneEngine(t, 8, 1.04, 0)
		e.P.RefCapRisk = refCapRisk
		seeWorld(e, 20)
		req := laneRequest("R1", e.now, 7000)
		e.onRequest(req)
		return e.lateRisk(req, req.Options[0])
	}
	raw, capped := riskOf(0), riskOf(1)
	if raw <= 0 || capped <= raw {
		t.Errorf("modelled risk on the raw fee %.4f, on the capped fee %.4f: want the capped one higher (a lower fee is booked later)", raw, capped)
	}
}

// The fee clock can have a cap of its own, below the one the quote's cost is built on.
func TestRefCapClockIsTheClocksOwnCap(t *testing.T) {
	e := laneEngine(t, 8, 1.04, 1)
	e.P.RefCapClock = 0.96
	seeWorld(e, 20)
	req := laneRequest("R1", e.now, 7000)
	e.onRequest(req)
	if got := e.fairRef(req); got != 5280 {
		t.Errorf("the clock's fee base is %d, want 0.96 of the lane's 5500 = 5280", got)
	}
	if got := e.priceRef(req); got != 5720 {
		t.Errorf("the quote's cost is built on %d, want 1.04 of the lane's 5500 = 5720", got)
	}
	low := laneRequest("R2", e.now, 5000)
	e.onRequest(low)
	if e.fairRef(low) != 5000 || e.priceRef(low) != 5000 {
		t.Errorf("a typical fee under both caps is left alone: clock %d, price %d", e.fairRef(low), e.priceRef(low))
	}
}

// The lane's fee is level 8's (e24); the levels below keep the request's own typical fee.
func TestLaneFeeByLevel(t *testing.T) {
	for level := 1; level <= 8; level++ {
		p := DefaultParamsFor(level)
		wantCap, wantOn := 0.0, 0
		if level >= 8 {
			wantCap, wantOn = 1.04, 1
		}
		if p.RefCap != wantCap || p.RefCapPrice != wantOn || p.RefCapRisk != wantOn {
			t.Errorf("level %d: refCap %.2f, refCapPrice %d, refCapRisk %d; want %.2f, %d, %d", level, p.RefCap, p.RefCapPrice, p.RefCapRisk, wantCap, wantOn, wantOn)
		}
		// Measured and left off at every level.
		if p.RefCapClock != 0 || p.GuardGraceMin != 0 || p.RiskMonday != 0 || p.ReturnReprice != 0 || p.MorningFreshMin != 0 {
			t.Errorf("level %d: a switch that was not adopted is on", level)
		}
	}
}

// On a lane where drivers book readily the fee clock gets a lower cap; the quote's cost
// and the lateness model keep the general one unless they are given their own.
func TestEasyLanesGetALowerClockCap(t *testing.T) {
	tests := []struct {
		name                 string
		quick                float64
		known                bool
		easyPrice            float64
		wantClock, wantPrice int
	}{
		{"an easy lane", 0.50, true, 0, 4785, 5500},
		{"a middle lane", 0.25, true, 0, 5170, 5500},
		{"a hard lane", 0.10, true, 0, 5500, 5500},
		{"a lane without a record", 0, false, 0, 5500, 5500},
		{"an easy lane with a cap for the cost too", 0.50, true, 0.95, 4785, 5225},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := laneEngine(t, 8, 1.04, 1)
			e.P.RefCapRisk = 1
			e.P.EasyCap, e.P.MidCap, e.P.EasyPrice = 0.87, 0.94, tc.easyPrice
			if tc.known {
				laneQuick["a|b"] = tc.quick
				t.Cleanup(func() { delete(laneQuick, "a|b") })
			}
			seeWorld(e, 20)
			req := laneRequest("R1", e.now, 5500) // the lane's own fee in this world
			e.onRequest(req)
			if got := e.fairRef(req); got != tc.wantClock {
				t.Errorf("the clock's fee base is %d, want %d", got, tc.wantClock)
			}
			if got := e.priceRef(req); got != tc.wantPrice {
				t.Errorf("the quote's cost is built on %d, want %d", got, tc.wantPrice)
			}
			if got := e.riskRef(req); got != 5500 {
				t.Errorf("the lateness model reads %d, want the general cap's 5500", got)
			}
		})
	}
	if p := DefaultParamsFor(8); p.EasyCap != 0.87 || p.MidCap != 0.94 || p.EasyPrice != 0.95 || p.MidPrice != 0 || p.EasyFrom != 0.34 || p.MidFrom != 0.22 || p.RescueFrom != 0.10 || p.OpenScale != 0 {
		t.Errorf("level 8 defaults (e25): caps 0.87 and 0.94, easy price 0.95, classes from 0.34 and 0.22, rescue from 0.10; got %+v", []float64{p.EasyCap, p.MidCap, p.EasyPrice, p.MidPrice, p.EasyFrom, p.MidFrom, p.RescueFrom, p.OpenScale})
	}
	if p := DefaultParamsFor(7); p.EasyCap != 0 || p.MidCap != 0 || p.EasyPrice != 0 || p.RescueFrom != 0 || p.ThinRisk != 0 {
		t.Errorf("level 7 keeps the lower caps and the thin-world caution off, got %+v", []float64{p.EasyCap, p.MidCap, p.EasyPrice, p.RescueFrom, p.ThinRisk})
	}
}

// OpenScale scales the count of orders without a driver that the lateness model reads:
// with fewer counted, the same request looks safer.
func TestOpenScaleLowersTheModelsLoad(t *testing.T) {
	riskOf := func(scale float64) float64 {
		p := DefaultParamsFor(8)
		p.OpenScale = scale
		e := New(8, p, testMap(t))
		e.now, e.endsAt = cal.At(1, 10*cal.Hour), 14280
		for i := 0; i < 12; i++ {
			chainOrder(e, fmt.Sprintf("W%02d", i), "a", "b", e.now, cal.At(2, 20*cal.Hour)).wonAt = e.now - 30
		}
		req := testRequest("R1", e.now)
		return e.lateRisk(req, req.Options[0])
	}
	plain, scaled := riskOf(0), riskOf(0.9)
	if plain <= 0 || scaled >= plain {
		t.Errorf("modelled risk %.4f with the plain count, %.4f with 0.9 of it: want it lower", plain, scaled)
	}
	if same := riskOf(1); same != plain {
		t.Errorf("a scale of 1 is the plain count: %.4f against %.4f", same, plain)
	}
}

// In a week where many orders never found a driver the lower caps of easy lanes are off:
// the clock goes back to the general cap.
func TestRescueTurnsTheEasyCapsOff(t *testing.T) {
	for _, tc := range []struct {
		name     string
		unbooked int
		want     int
	}{{"an ordinary week", 3, 4785}, {"a week short of drivers", 15, 5500}} {
		t.Run(tc.name, func(t *testing.T) {
			e := laneEngine(t, 8, 1.04, 1)
			e.P.EasyCap, e.P.RescueFrom = 0.87, 0.10
			laneQuick["a|b"] = 0.5
			t.Cleanup(func() { delete(laneQuick, "a|b") })
			seeWorld(e, 20)
			e.SetNow(cal.At(1, 14*cal.Hour))
			for i := 0; i < 100; i++ {
				ob := chainOrder(e, fmt.Sprintf("W%03d", i), "a", "b", e.now, cal.At(3, 20*cal.Hour))
				ob.wonAt = e.now - 4*cal.Hour
				if i >= tc.unbooked {
					ob.firstBookedAt = ob.wonAt + 20
					ob.legs[0].gig = &gigBook{id: fmt.Sprintf("g%d", i), status: gigBooked}
				}
			}
			req := laneRequest("R1", e.now, 5500)
			e.onRequest(req)
			if got := e.fairRef(req); got != tc.want {
				t.Errorf("the clock's fee base is %d, want %d", got, tc.want)
			}
		})
	}
}

// A car under the pressure of its deadline, or posted when it is already tight or late,
// leaves the lower cap of an easy lane: its fee base is the general cap's.
func TestPressedCarsLeaveTheEasyCap(t *testing.T) {
	setup := func(deadline int) (*Engine, *orderBook, []Pending) {
		e := laneEngine(t, 8, 1.04, 1)
		e.P.EasyCap, e.P.Chains = 0.87, 0
		laneQuick["a|b"] = 0.5
		t.Cleanup(func() { delete(laneQuick, "a|b") })
		seeWorld(e, 20)
		req := laneRequest("R1", e.now, 5500)
		req.Options[0].Deadline = deadline
		req.Legs[0].Pickup.Window.To, req.Legs[0].Delivery.Window.To = deadline, deadline
		e.onRequest(req)
		e.Drain()
		order := testOrder(req, "open", nil)
		order.Deadline, order.LastPickupAt = deadline, deadline-30
		e.onWon(order, e.now)
		return e, e.orders["R1"], e.Drain()
	}
	// With a day in hand the gig opens on the easy cap, and the deadline's warning lifts it.
	e, ob, sent := setup(cal.At(1, 20*cal.Hour))
	g := ob.legs[0].gig
	if len(sent) != 1 || g == nil || g.base != 4785 {
		t.Fatalf("an easy lane's gig should open on a base of 4785, got %+v", g)
	}
	e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g1"}})
	e.SetNow(ob.order.LastPickupAt - 170)
	e.Handle([]api.Event{{Kind: api.KindDeadlineClose, At: e.now, OrderID: "R1", LastPickupAt: ob.order.LastPickupAt}})
	if open := ob.legs[0].gig; open == nil || open.base != 5500 {
		t.Errorf("after the deadline's warning the fee base should be the general cap's 5500, got %+v", open)
	}
	// An order won with little time left is posted tight: on the general cap from the start.
	e, ob, _ = setup(cal.At(0, 10*cal.Hour) + 50)
	if tight := ob.legs[0].gig; tight == nil || tight.mode == modeBuffered || tight.base != 5500 {
		t.Errorf("a tight car should be posted on a base of 5500, got %+v", tight)
	}
}
