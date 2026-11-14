package engine

import (
	"testing"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

func TestCapScheduleStopsAtTheOrdersWorth(t *testing.T) {
	plan := []api.FeeStep{{At: 100, FeeCents: 3000}, {At: 200, FeeCents: 6000}, {At: 300, FeeCents: 9000}, {At: 400, FeeCents: 12000}}
	got := capSchedule(plan, 7000)
	want := []api.FeeStep{{At: 100, FeeCents: 3000}, {At: 200, FeeCents: 6000}, {At: 300, FeeCents: 7000}}
	if !sameSchedule(got, want) {
		t.Errorf("capSchedule = %+v, want %+v", got, want)
	}
	if !sameSchedule(capSchedule(plan, 0), plan) {
		t.Error("a zero cap means no cap")
	}
	if got := capSchedule(plan, 10); len(got) != 1 || got[0].FeeCents != minFeeCents {
		t.Errorf("a cap under the least fee still posts the least fee, got %+v", got)
	}
}

func TestUnbookedLateRiskGrowsAsTimeRunsOut(t *testing.T) {
	prev := 0.0
	for _, left := range []int{3 * cal.Day, cal.Day, 12 * cal.Hour, 6 * cal.Hour, 3 * cal.Hour, cal.Hour, 0, -30} {
		risk := unbookedLateRisk(left)
		if risk < prev || risk <= 0 || risk > 1 {
			t.Errorf("risk with %d minutes left is %.2f after %.2f", left, risk, prev)
		}
		prev = risk
	}
	if unbookedLateRisk(0) != 1 {
		t.Error("an order past its last on-time pickup is late")
	}
}

// A dealer may send a won car to another place: the gig's fee must follow the new lane.
func TestBaseFeeFollowsAChangedDestination(t *testing.T) {
	e := New(8, DefaultParamsFor(8), testMap(t))
	req := testRequest("R1", 600) // a -> b: 30 road minutes, typical fee 3000
	order := testOrder(req, "waiting", nil)
	ob := &orderBook{order: *order, req: req, legs: make([]legBook, 1)}
	if got := e.baseFee(ob, []int{0}); got != 3000 {
		t.Fatalf("base fee on the quoted lane = %d, want the typical fee 3000", got)
	}
	// The dealer changes the delivery to the pickup's own town: a 10-minute lane.
	moved := *order
	moved.Legs = []api.Leg{order.Legs[0]}
	moved.Legs[0].Delivery.PlaceID = "a"
	ob.order = moved
	if got := e.baseFee(ob, []int{0}); got != 1000 {
		t.Errorf("base fee on a lane a third as long = %d, want 1000", got)
	}
}

func TestOutlookCountsOpenOrdersByTheirState(t *testing.T) {
	e := New(8, DefaultParamsFor(8), testMap(t))
	e.now, e.endsAt = cal.At(1, 12*cal.Hour), 14280
	req := testRequest("R1", 600)
	late := &orderBook{order: *testOrder(req, "waiting", nil), req: req, legs: make([]legBook, 1), refunded: true}
	booked := &orderBook{order: *testOrder(req, "booked", nil), req: req, legs: []legBook{{gig: &gigBook{status: gigBooked}}}}
	waiting := &orderBook{order: *testOrder(req, "waiting", nil), req: req, legs: make([]legBook, 1)}
	waiting.order.LastPickupAt = e.now + 30 // half an hour left and no driver
	e.orders["late"], e.orders["booked"], e.orders["waiting"] = late, booked, waiting
	want := 1 - (guardPriorLate+1+bookedLateRisk+0.9)/(guardPriorOrders+3)
	if got := e.onTimeOutlook(); got < want-1e-9 || got > want+1e-9 {
		t.Errorf("outlook = %.4f, want %.4f", got, want)
	}
	// The guard and the weekday factor only ever tighten the cap.
	if limit := e.riskCap(e.market.dealer("D01")); limit > e.P.RiskMax || limit <= 0 {
		t.Errorf("risk cap %.3f is outside (0, RiskMax]", limit)
	}
}

// A trade-in's old car is collected in the visit that delivers the new car: its one-hour
// slot must not make the request look impossible, and the new car is due in the customer's hour.
func TestTradeInIsJudgedByTheNewCar(t *testing.T) {
	e := New(8, DefaultParamsFor(8), testMap(t))
	arrived := cal.At(0, 10*cal.Hour)
	hour := api.TimeWindow{From: cal.At(1, 15*cal.Hour), To: cal.At(1, 16*cal.Hour)}
	dealer := &api.OpeningHours{Opens: 7 * cal.Hour, Closes: 20 * cal.Hour, Weekdays: []int{0, 1, 2, 3, 4}}
	out := api.Leg{
		Pickup:   api.StopPlace{PlaceID: "a", Window: api.TimeWindow{From: cal.At(1, 8*cal.Hour), To: cal.At(1, 20*cal.Hour)}, Hours: dealer},
		Delivery: api.StopPlace{PlaceID: "b", PrivateAddress: true, Window: hour},
	}
	back := api.Leg{
		Pickup:   api.StopPlace{PlaceID: "b", PrivateAddress: true, Window: hour},
		Delivery: api.StopPlace{PlaceID: "a", Window: api.TimeWindow{From: hour.From, To: cal.At(1, 20*cal.Hour)}, Hours: dealer},
	}
	req := &api.PriceRequest{ID: "R9", DealerID: "D01", ArrivedAt: arrived, RespondBy: arrived + 15, Legs: []api.Leg{out, back},
		Options: []api.DeliveryOption{{Kind: api.SameDay, Deadline: cal.At(1, 20*cal.Hour), ListPriceCents: 9000}}, ReferenceFeeCents: 5000}
	if !e.sameVisit(req.Legs, 1) {
		t.Fatal("the old car of a trade-in is collected in the same visit")
	}
	if got := legDeadline(&out, cal.At(1, 20*cal.Hour)); got != hour.To+privateGraceMin {
		t.Errorf("the new car is due %s, want half an hour after the customer's hour", cal.Format(got))
	}
	// The new car can leave the dealer from 08:00 and must leave by about 16:00: hours of room.
	if slack := e.slack(req, req.Options[0]); slack < 6*cal.Hour {
		t.Errorf("slack of the trade-in = %d minutes, want the new car's room (over 6 hours), not the old car's one-hour slot", slack)
	}
	// Below level 3 a gig carries one car, so the old car's slot does count.
	single := New(2, DefaultParamsFor(2), testMap(t))
	if single.sameVisit(req.Legs, 1) {
		t.Error("without multi-car gigs the old car is a gig of its own")
	}
}

// The fee model: a car ready later that goes to a town full of drivers books cheaper
// than one ready today that goes where nobody lives.
func TestExpectedFeeMultFollowsWhereAndWhen(t *testing.T) {
	e := New(8, DefaultParamsFor(8), testMap(t))
	roster := make([]api.DriverProfile, 0, 200)
	for i := 0; i < 200; i++ {
		roster = append(roster, api.DriverProfile{ID: "d" + string(rune('a'+i%26)) + string(rune('a'+i/26)), HomePlaceID: "b"})
	}
	e.Begin(&api.Snapshot{Now: 0, EndsAt: 14280}, roster)
	monday := cal.At(0, 10*cal.Hour)
	later := testLeg(cal.At(1, 9*cal.Hour))                 // a -> b, ready tomorrow, delivery where 200 drivers live
	today := testLeg(monday)                                // ready now
	today.Pickup.PlaceID, today.Delivery.PlaceID = "b", "a" // delivery where no driver lives
	cheap := e.expectedFeeMult(&later, monday, api.NextDay)
	dear := e.expectedFeeMult(&today, cal.At(4, 15*cal.Hour), api.SameDay)
	if cheap >= dear || cheap < 0.5 || dear > 1.6 {
		t.Errorf("expected fee multiples: easy gig %.2f, hard gig %.2f", cheap, dear)
	}
}
