package engine

import (
	"testing"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
	"highwaycoins/internal/geo"
)

// laneMap has a hop inside a town, a middle lane and a long haul.
func laneMap(t *testing.T) *geo.Map {
	t.Helper()
	places := []api.Place{{ID: "a", Name: "A", Lat: 60.2, Lon: 24.9}, {ID: "b", Name: "B", Lat: 61.5, Lon: 23.8}, {ID: "c", Name: "C", Lat: 65.0, Lon: 25.5}}
	km := map[string]float64{"aa": 5, "bb": 5, "cc": 5, "ab": 180, "ba": 180, "ac": 600, "ca": 600, "bc": 480, "cb": 480}
	var roads []api.Road
	for _, from := range places {
		for _, to := range places {
			d := km[from.ID+to.ID]
			roads = append(roads, api.Road{From: from.ID, To: to.ID, Km: d, Minutes: int(d * 0.7)})
		}
	}
	m, err := geo.New(&api.WorldMap{Places: places, Roads: roads})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func laneLeg(from, to string, ready int) *api.Leg {
	h := &api.OpeningHours{Opens: 7 * cal.Hour, Closes: 20 * cal.Hour, Weekdays: []int{0, 1, 2, 3, 4}}
	w := api.TimeWindow{From: ready, To: cal.At(3, 20*cal.Hour)}
	return &api.Leg{Pickup: api.StopPlace{PlaceID: from, Window: w, Hours: h}, Delivery: api.StopPlace{PlaceID: to, Window: w, Hours: h}}
}

// The lane model: a long haul is riskier than a middle lane, a hop inside a town riskier
// than a middle lane too, more time to find a driver makes an order safer, and a car due
// the day it is won is riskier than the same car due the next day.
func TestLaneRiskModelKnowsTheLaneAndTheLead(t *testing.T) {
	p := DefaultParamsFor(8)
	p.RiskModel = riskModelLaneID
	e := New(8, p, laneMap(t))
	at := cal.At(0, 10*cal.Hour)
	e.now = at
	risk := func(from, to string, deadlineDay int, option string, fee, list int) float64 {
		return e.laneRisk(laneLeg(from, to, at), at, at, cal.At(deadlineDay, 20*cal.Hour), option, fee, list)
	}
	middle, haul, hop := risk("a", "b", 1, api.NextDay, 9000, 10000), risk("a", "c", 1, api.NextDay, 25000, 30000), risk("a", "a", 1, api.NextDay, 1500, 2500)
	for name, r := range map[string]float64{"middle": middle, "haul": haul, "hop": hop} {
		if r <= 0 || r >= 1 {
			t.Fatalf("%s lane: risk %.4f is not a chance", name, r)
		}
	}
	if haul <= middle {
		t.Errorf("a 600 km haul (%.4f) should be riskier than a 180 km lane (%.4f)", haul, middle)
	}
	if hop <= middle {
		t.Errorf("a hop inside a town (%.4f) should be riskier than a 180 km lane (%.4f)", hop, middle)
	}
	if later := risk("a", "c", 3, api.NextDay, 25000, 30000); later >= haul {
		t.Errorf("two more days to find a driver should lower the risk: %.4f vs %.4f", later, haul)
	}
	if sameDay := risk("a", "b", 0, api.SameDay, 9000, 10000); sameDay <= middle {
		t.Errorf("due today (%.4f) should be riskier than due tomorrow (%.4f)", sameDay, middle)
	}
}

// The model measures the weekday, the hour and the lead from the dealer's answer, and
// "ready today" means ready on the day of the answer.
func TestLaneFeaturesAreMeasuredFromTheAnswer(t *testing.T) {
	p := DefaultParamsFor(8)
	p.RiskModel = riskModelLaneID
	e := New(8, p, laneMap(t))
	at := cal.At(3, 15*cal.Hour) // Thursday 15:00
	e.now = at
	ready := cal.At(4, 9*cal.Hour) // the car is ready on Friday morning
	f := e.laneFeatures(laneLeg("a", "b", ready), at, ready, cal.At(4, 20*cal.Hour), api.SameDay, 9000, 10000)
	const today, thursday, friday, hourOfDay, afternoon, sameDay, todaySame, leadHours = 6, 12, 13, 14, 15, 16, 29, 23
	if f[today] != 0 || f[todaySame] != 0 {
		t.Errorf("a car ready tomorrow is not ready today: today %.0f, today and same day %.0f", f[today], f[todaySame])
	}
	if f[thursday] != 1 || f[friday] != 0 {
		t.Errorf("the weekday is that of the answer: thursday %.0f friday %.0f", f[thursday], f[friday])
	}
	if f[hourOfDay] != 0.7 || f[afternoon] != 1 || f[sameDay] != 1 {
		t.Errorf("hour %.2f (want 0.7), afternoon %.0f, same day %.0f", f[hourOfDay], f[afternoon], f[sameDay])
	}
	// Last pickup: Friday 20:00 less the 126 minutes of the drive; the answer came Thursday 15:00.
	if want := float64(cal.At(4, 20*cal.Hour)-126-at) / cal.Hour; f[leadHours] < want-0.05 || f[leadHours] > want+0.05 {
		t.Errorf("lead %.2f h, want %.2f h from the answer", f[leadHours], want)
	}
}

// The day's load counts the orders won today, those still without a driver and those
// won in the last two hours.
func TestBookLoadCountsTheDaysOrders(t *testing.T) {
	e := New(8, DefaultParamsFor(8), laneMap(t))
	e.now = cal.At(1, 12*cal.Hour)
	add := func(id string, wonAt, bookings int, cancelled bool) {
		e.orders[id] = &orderBook{order: api.Order{ID: id, Legs: []api.Leg{*laneLeg("a", "b", wonAt)}}, legs: make([]legBook, 1), wonAt: wonAt, bookings: bookings, cancelled: cancelled}
	}
	add("yesterday, no driver", cal.At(0, 15*cal.Hour), 0, false)
	add("today, early, booked", cal.At(1, 9*cal.Hour), 1, false)
	add("today, just now", cal.At(1, 11*cal.Hour), 0, false)
	add("today, cancelled", cal.At(1, 11*cal.Hour), 0, true)
	if got, want := e.bookLoad(), (load{wonToday: 2, open: 2, recent: 1}); got != want {
		t.Errorf("load %+v, want %+v", got, want)
	}
}

// Every model below the lane model keeps its inputs.
func TestSupplyModelIgnoresTheLaneModel(t *testing.T) {
	p := DefaultParamsFor(8)
	p.RiskModel = 2
	e := New(8, p, laneMap(t))
	at := cal.At(0, 10*cal.Hour)
	req := &api.PriceRequest{ID: "R1", DealerID: "D01", ArrivedAt: at - 15, RespondBy: at, Legs: []api.Leg{*laneLeg("a", "b", at)},
		Options: []api.DeliveryOption{{Kind: api.NextDay, Deadline: cal.At(1, 20*cal.Hour), ListPriceCents: 10000}}, ReferenceFeeCents: 9000}
	want := e.legRisk(&req.Legs[0], at, cal.At(1, 20*cal.Hour), api.NextDay)
	if got := e.lateRisk(req, req.Options[0]); got != want {
		t.Errorf("the supply model's risk changed: %.4f, want %.4f", got, want)
	}
	p.RiskModel = riskModelLaneID
	lane := New(8, p, laneMap(t))
	lane.now = at - 15
	if got := lane.lateRisk(req, req.Options[0]); got == want || got <= 0 || got >= 1 {
		t.Errorf("the lane model should give its own chance, got %.4f (supply model %.4f)", got, want)
	}
}
