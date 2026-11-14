package engine

import (
	"testing"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
	"highwaycoins/internal/geo"
)

// The dealer's note of a real request: the pickup site is open 10:00-17:00 on weekdays,
// although the request states 08:00-20:00.
const shortHoursNote = "hei. noudettavissa arkisin klo 10.00-17.00. vastaanotto arkisin klo 8.00-20.00. terveisin myynti"

// farMap has a short lane (a-b, 30 minutes) and a long one (a-c, 9 hours).
func farMap(t *testing.T) *geo.Map {
	t.Helper()
	ids := []string{"a", "b", "c"}
	minutes := map[string]int{"aa": 10, "bb": 10, "cc": 10, "ab": 30, "ba": 30, "ac": 549, "ca": 549, "bc": 540, "cb": 540}
	var roads []api.Road
	for _, from := range ids {
		for _, to := range ids {
			roads = append(roads, api.Road{From: from, To: to, Km: float64(minutes[from+to]) * 1.4, Minutes: minutes[from+to]})
		}
	}
	m, err := geo.New(&api.WorldMap{
		Places: []api.Place{{ID: "a", Lat: 60.2, Lon: 24.9}, {ID: "b", Lat: 60.3, Lon: 25.0}, {ID: "c", Lat: 66.5, Lon: 25.7}},
		Roads:  roads,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// narrowRequest is a single car from a to the given place whose note shortens the pickup hours.
func narrowRequest(id, to string, arrived, deadline int) *api.PriceRequest {
	h := &api.OpeningHours{Opens: 8 * cal.Hour, Closes: 20 * cal.Hour, Weekdays: []int{0, 1, 2, 3, 4}}
	w := api.TimeWindow{From: arrived, To: deadline}
	leg := api.Leg{Pickup: api.StopPlace{PlaceID: "a", Window: w, Hours: h}, Delivery: api.StopPlace{PlaceID: to, Window: w, Hours: h}}
	return &api.PriceRequest{
		ID: id, DealerID: "D01", ArrivedAt: arrived, RespondBy: arrived + 15, Legs: []api.Leg{leg},
		Options:           []api.DeliveryOption{{Kind: api.NextDay, Deadline: deadline, ListPriceCents: 22300}},
		ReferenceFeeCents: 20000, Note: &api.Note{Text: shortHoursNote, Language: "fi"},
	}
}

func narrowEngine(t *testing.T, now int) *Engine {
	t.Helper()
	e := New(8, DefaultParamsFor(8), farMap(t))
	e.now, e.endsAt = now, 14280
	return e
}

// A car that its real hours let no driver collect and deliver on one day is not quoted:
// drivers plan by the stated hours, and day windows cannot keep them inside the real ones.
func TestShortHoursThatFitNoDayAreDeclined(t *testing.T) {
	now := cal.At(1, 9*cal.Hour) // Tuesday 09:00
	cases := []struct {
		name string
		to   string
		want string
	}{
		{"a short drive fits inside the real hours", "b", ""},
		{"a nine-hour drive fits no day", "c", "shorter hours"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := narrowEngine(t, now)
			req := narrowRequest("R1", c.to, now, cal.At(2, 20*cal.Hour))
			if !e.narrowed(req) {
				t.Fatal("the note shortens the pickup hours: the request must read as narrowed")
			}
			if got := e.noteRefusal(req); got != c.want {
				t.Errorf("refusal = %q, want %q", got, c.want)
			}
		})
	}
}

func insideRealHours(t *testing.T, w *api.TimeWindow) {
	t.Helper()
	if w == nil {
		t.Fatal("the pickup needs a window of its own: drivers plan by the stated hours")
	}
	if cal.MinuteOfDay(w.From) < 10*cal.Hour || cal.MinuteOfDay(w.To) > 17*cal.Hour || cal.DayOf(w.From) != cal.DayOf(w.To) || w.To <= w.From {
		t.Errorf("pickup window %s - %s is not inside 10:00-17:00 of one day", cal.Format(w.From), cal.Format(w.To))
	}
}

// If such a car is in the books all the same (the dealer changed the order, or a restart),
// every gig for it keeps the driver inside the real pickup hours, also when it is late and
// after a failed visit.
func TestPickupAtShortHoursAlwaysCarriesARealWindow(t *testing.T) {
	now := cal.At(1, 13*cal.Hour) // Tuesday 13:00
	e := narrowEngine(t, now)
	req := narrowRequest("R1", "c", now-15, cal.At(2, 20*cal.Hour))
	e.reqs["R1"] = req
	order := &api.Order{ID: "R1", DealerID: "D01", Option: api.NextDay, PriceCents: 34000, Deadline: cal.At(2, 20*cal.Hour),
		Legs: req.Legs, Status: "open", DeliveredAt: []*int{nil}, LastPickupAt: cal.At(2, 10*cal.Hour+51)}
	e.onWon(order, now)
	sent := e.Drain()
	if len(sent) != 1 || sent[0].Action.Type != api.ActPost {
		t.Fatalf("want one post, got %+v", sent)
	}
	first := sent[0].Action.Stops[0].Window
	insideRealHours(t, first)
	if cal.DayOf(first.From) != 1 || first.From != now {
		t.Errorf("the first window is today from now, got %s", cal.Format(first.From))
	}
	e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g1"}})

	// Nobody takes it; long after the deadline a driver books it and comes at 07:00, when
	// the site is still closed.
	e.SetNow(cal.At(3, 6*cal.Hour))
	e.Handle([]api.Event{{Kind: api.KindGigExpired, At: e.now, GigID: "g1"}})
	sent = e.Drain()
	if countType(sent, api.ActPost) != 1 {
		t.Fatalf("the car is posted again, got %+v", sent)
	}
	late := sent[len(sent)-1].Action.Stops[0].Window
	insideRealHours(t, late)
	if cal.DayOf(late.From) != 3 {
		t.Errorf("a late car's window is on the next open day (Thursday), got %s", cal.Format(late.From))
	}
	e.Resolve(sent, []api.ActionResult{{OK: true, ID: "g2"}})
	e.Handle([]api.Event{{Kind: api.KindGigBooked, At: e.now, GigID: "g2", DriverID: "d1", FeeCents: 30000}})
	e.SetNow(cal.At(3, 7*cal.Hour))
	e.Handle([]api.Event{{Kind: api.KindVisitFailed, At: e.now, GigID: "g2", OrderID: "R1", Leg: 0, Reason: reasonClosed}})
	sent = e.Drain()
	if countType(sent, api.ActPost) != 1 {
		t.Fatalf("after the failed visit the car is posted again, got %+v", sent)
	}
	retry := sent[len(sent)-1].Action.Stops[0].Window
	insideRealHours(t, retry)
	if retry.From != cal.At(3, 10*cal.Hour) {
		t.Errorf("the retry starts when the site really opens (Thu 10:00), got %s", cal.Format(retry.From))
	}
}
