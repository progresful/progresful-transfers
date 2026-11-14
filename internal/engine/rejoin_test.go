package engine

import (
	"testing"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
	"progresful-transfers/internal/geo"
)

func testMap(t *testing.T) *geo.Map {
	t.Helper()
	m, err := geo.New(&api.WorldMap{
		Places: []api.Place{{ID: "a", Name: "A", Lat: 60.2, Lon: 24.9}, {ID: "b", Name: "B", Lat: 60.3, Lon: 25.0}},
		Roads: []api.Road{
			{From: "a", To: "a", Km: 5, Minutes: 10}, {From: "a", To: "b", Km: 20, Minutes: 30},
			{From: "b", To: "a", Km: 20, Minutes: 30}, {From: "b", To: "b", Km: 5, Minutes: 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func testLeg(from int) api.Leg {
	h := &api.OpeningHours{Opens: 7 * cal.Hour, Closes: 20 * cal.Hour, Weekdays: []int{0, 1, 2, 3, 4}}
	w := api.TimeWindow{From: from, To: cal.At(1, 20*cal.Hour)}
	return api.Leg{Pickup: api.StopPlace{PlaceID: "a", Window: w, Hours: h}, Delivery: api.StopPlace{PlaceID: "b", Window: w, Hours: h}}
}

func testRequest(id string, arrived int) *api.PriceRequest {
	return &api.PriceRequest{
		ID: id, DealerID: "D01", ArrivedAt: arrived, RespondBy: arrived + 15, Legs: []api.Leg{testLeg(arrived)},
		Options:           []api.DeliveryOption{{Kind: api.NextDay, Deadline: cal.At(1, 20*cal.Hour), ListPriceCents: 6000}},
		ReferenceFeeCents: 3000,
	}
}

func testOrder(req *api.PriceRequest, status string, delivered *int) *api.Order {
	return &api.Order{
		ID: req.ID, DealerID: req.DealerID, Option: api.NextDay, PriceCents: 8000, Deadline: cal.At(1, 20*cal.Hour),
		Legs: req.Legs, Status: status, DeliveredAt: []*int{delivered}, LastPickupAt: cal.At(1, 19*cal.Hour),
	}
}

// A restart in the middle of a week: one request still open, one order waiting without a
// gig, one order with a booked gig, one delivered. The engine must quote the open request,
// post only the car that has no gig, and count the delivered order's trust.
func TestRejoinRebuildsBooksAndActsOnlyOnWhatIsOpen(t *testing.T) {
	now := cal.At(0, 12*cal.Hour)
	open, waiting, booked, done := testRequest("R4", now-5), testRequest("R1", 600), testRequest("R2", 610), testRequest("R3", 620)
	deliveredAt := cal.At(0, 11*cal.Hour)
	driver, orderID := "drv1", "R3"
	history := []api.Event{
		{Seq: 1, At: 600, Kind: api.KindPriceRequest, Request: waiting},
		{Seq: 2, At: 610, Kind: api.KindPriceRequest, Request: booked},
		{Seq: 3, At: 615, Kind: api.KindOrderWon, Order: testOrder(waiting, "waiting", nil)},
		{Seq: 4, At: 620, Kind: api.KindPriceRequest, Request: done},
		{Seq: 5, At: 625, Kind: api.KindOrderWon, Order: testOrder(booked, "waiting", nil)},
		{Seq: 6, At: 635, Kind: api.KindOrderWon, Order: testOrder(done, "waiting", nil)},
		{Seq: 7, At: deliveredAt, Kind: api.KindLedger, Line: &api.LedgerLine{At: deliveredAt, Kind: "revenue", Cents: 8000, OrderID: &orderID}},
		{Seq: 8, At: now - 5, Kind: api.KindPriceRequest, Request: open},
	}
	snap := &api.Snapshot{
		Now: now, EndsAt: cal.At(9, 22*cal.Hour),
		OpenRequests: []api.PriceRequest{*open},
		Orders: []api.Order{
			*testOrder(waiting, "waiting", nil), *testOrder(booked, "booked", nil), *testOrder(done, "delivered", &deliveredAt),
		},
		Gigs: []api.Gig{{
			ID: "g7", Status: "booked", PostedAt: 626, FeeCents: 3100, DriverID: &driver,
			Stops:    []api.StopRef{{OrderID: "R2", Leg: 0, Kind: stopPickup}, {OrderID: "R2", Leg: 0, Kind: stopDelivery}},
			Schedule: []api.FeeStep{{At: 626, FeeCents: 3100}},
		}},
	}
	e := New(8, DefaultParamsFor(8), testMap(t))
	e.Rejoin(snap, []api.DriverProfile{{ID: driver, HomePlaceID: "a", Rating: 4.9, Licence: "manual", CompletedGigs: 80}}, history)
	if sent := e.Drain(); len(sent) != 0 {
		t.Fatalf("Rejoin alone must not act (the saved market is loaded first), got %d actions", len(sent))
	}
	e.Resume(snap)

	var quotes, posts []api.Action
	for _, p := range e.Drain() {
		switch p.Action.Type {
		case api.ActQuote:
			quotes = append(quotes, p.Action)
		case api.ActPost:
			posts = append(posts, p.Action)
		default:
			t.Errorf("unexpected %s after a rejoin", p.Action.Type)
		}
	}
	if len(quotes) != 1 || quotes[0].RequestID != "R4" {
		t.Errorf("want one quote for the open request R4, got %+v", quotes)
	}
	if len(posts) != 1 || posts[0].Stops[0].OrderID != "R1" {
		t.Errorf("want one post for the order without a gig (R1), got %+v", posts)
	}
	if g := e.gigs["g7"]; g == nil || g.status != gigBooked || g.driver != driver {
		t.Errorf("the booked gig was not adopted: %+v", g)
	}
	if got := e.orders["R2"].bookings; got != 1 {
		t.Errorf("the booked order counts %d bookings, want 1 (it must not look like backlog)", got)
	}
	if !e.orders["R3"].done() {
		t.Error("the delivered order should be done")
	}
	if got := e.market.dealer("D01").trust; got != 1 {
		t.Errorf("dealer trust = %d, want 1 from the order paid without a refund", got)
	}
	if e.Now() != now {
		t.Errorf("engine clock = %d, want the snapshot's %d", e.Now(), now)
	}
}

// Nothing may be sent while the history is replayed.
func TestReplayedHistorySendsNothing(t *testing.T) {
	req := testRequest("R1", 600)
	e := New(8, DefaultParamsFor(8), testMap(t))
	snap := &api.Snapshot{Now: 700, EndsAt: 14280, Orders: []api.Order{*testOrder(req, "delivered", new(int))}}
	e.Rejoin(snap, nil, []api.Event{{Seq: 1, At: 600, Kind: api.KindPriceRequest, Request: req}, {Seq: 2, At: 615, Kind: api.KindOrderWon, Order: testOrder(req, "waiting", nil)}})
	e.Resume(snap)
	if sent := e.Drain(); len(sent) != 0 {
		t.Errorf("a finished world needs no action, got %d", len(sent))
	}
}
