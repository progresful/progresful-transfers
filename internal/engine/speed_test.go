package engine

import (
	"testing"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

func twoOptionRequest(id string, arrived int) *api.PriceRequest {
	req := testRequest(id, arrived)
	req.Options = []api.DeliveryOption{
		{Kind: api.SameDay, Deadline: cal.At(0, 20*cal.Hour), ListPriceCents: 6600},
		{Kind: api.NextDay, Deadline: cal.At(1, 20*cal.Hour), ListPriceCents: 6000},
	}
	return req
}

func speedEngine(t *testing.T, premium float64) *Engine {
	t.Helper()
	p := DefaultParamsFor(8)
	p.SpeedPremium, p.SpeedShare, p.SpeedMin = premium, 0.9, 4
	p.RiskMax = 1 // the test map has no drivers
	e := New(8, p, testMap(t))
	e.now, e.endsAt = cal.At(0, 9*cal.Hour), 14280
	return e
}

// The engine counts, for each dealer, how often it took the fastest of several options.
func TestDealersTasteForSpeedIsCounted(t *testing.T) {
	e := speedEngine(t, 0)
	win := func(id, option string) {
		req := twoOptionRequest(id, e.now)
		e.onRequest(req)
		if got := len(e.Drain()); got != 1 {
			t.Fatalf("want one quote, got %d", got)
		}
		e.onWon(&api.Order{ID: id, DealerID: req.DealerID, Option: option, PriceCents: 7000, Deadline: cal.At(1, 20*cal.Hour),
			Legs: req.Legs, DeliveredAt: []*int{nil}, LastPickupAt: cal.At(1, 19*cal.Hour)}, e.now)
		e.Drain()
	}
	win("R1", api.SameDay)
	win("R2", api.SameDay)
	win("R3", api.NextDay)
	d := e.market.dealer("D01")
	if d.multi != 3 || d.fastest != 2 {
		t.Errorf("dealer took the fastest in %d of %d orders with a choice, want 2 of 3", d.fastest, d.multi)
	}
	// A request with one option is no choice.
	req := testRequest("R4", e.now)
	e.onRequest(req)
	e.Drain()
	e.onWon(testOrder(req, "won", nil), e.now)
	if d.multi != 3 {
		t.Errorf("an order with one option counted as a choice: %d", d.multi)
	}
}

// A dealer that takes the fastest option nearly every time pays a premium on it; the
// slower options keep their price, and a dealer that is led by the price pays none.
func TestSpeedPremiumOnlyForDealersWhoTakeTheFastest(t *testing.T) {
	tests := []struct {
		name           string
		premium        float64
		fastest, multi int
		want           bool
	}{
		{"takes the fastest every time", 1.05, 10, 10, true},
		{"mostly the cheaper option", 1.05, 5, 10, false},
		{"too few orders to tell", 1.05, 3, 3, false},
		{"switched off", 0, 10, 10, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			quote := func(premium float64) map[string]int {
				e := speedEngine(t, premium)
				d := e.market.dealer("D01")
				d.fastest, d.multi = tc.fastest, tc.multi
				e.onRequest(twoOptionRequest("R1", e.now))
				sent := e.Drain()
				if len(sent) != 1 {
					t.Fatalf("want one quote, got %d", len(sent))
				}
				return sent[0].Action.Prices
			}
			plain, got := quote(0), quote(tc.premium)
			if got[api.NextDay] != plain[api.NextDay] {
				t.Errorf("the slower option moved: %d against %d", got[api.NextDay], plain[api.NextDay])
			}
			if higher := got[api.SameDay] > plain[api.SameDay]; higher != tc.want {
				t.Errorf("fastest option %d against %d without the premium, want higher = %v", got[api.SameDay], plain[api.SameDay], tc.want)
			}
		})
	}
}
