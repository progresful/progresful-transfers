package engine

import (
	"testing"

	"highwaycoins/internal/cal"
)

func dealerCostEngine(t *testing.T) *Engine {
	t.Helper()
	p := DefaultParamsFor(8)
	p.LeadCost, p.MinorCost = 0.9, 1.05
	e := New(8, p, testMap(t))
	e.now, e.endsAt = cal.At(0, 10*cal.Hour), 14280
	e.stats.Requests = 100
	e.market.dealer("D01").requests = 30 // three requests in ten
	e.market.dealer("D03").requests = 9
	e.market.dealer("D20").requests = 2
	return e
}

// The cost of a request by its dealer: lower for a big dealer's short car that can arrive
// today while its trust can grow, higher for a small dealer, unchanged in between.
func TestDealerCost(t *testing.T) {
	tests := []struct {
		name   string
		dealer string
		trust  int
		day    int
		hour   int
		want   float64
	}{
		{"big dealer, a car for today", "D01", 0, 0, 10, 0.9},
		{"big dealer, trust full", "D01", maxTrust, 0, 10, 1},
		{"big dealer, too late to arrive today", "D01", 0, 0, 19, 1},
		{"big dealer on a Friday", "D01", 0, 4, 10, 1},
		{"middle dealer", "D03", 0, 0, 10, 1},
		{"small dealer", "D20", 0, 0, 10, 1.05},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := dealerCostEngine(t)
			e.now = cal.At(tc.day, tc.hour*cal.Hour)
			d := e.market.dealer(tc.dealer)
			d.trust = tc.trust
			req := testRequest("R1", e.now)
			req.DealerID = tc.dealer
			if got := e.dealerCost(req, d); got != tc.want {
				t.Errorf("cost factor %.2f, want %.2f", got, tc.want)
			}
		})
	}
}

// With the switches off, and before enough requests are in to know a dealer's share,
// every dealer costs the same.
func TestDealerCostIsOffByDefault(t *testing.T) {
	e := dealerCostEngine(t)
	e.P.LeadCost, e.P.MinorCost = 0, 0
	req := testRequest("R1", e.now)
	if got := e.dealerCost(req, e.market.dealer("D01")); got != 1 {
		t.Errorf("switches off: factor %.2f, want 1", got)
	}
	e.P.LeadCost, e.P.MinorCost = 0.9, 1.05
	e.stats.Requests = 5
	if got := e.dealerCost(req, e.market.dealer("D20")); got != 1 {
		t.Errorf("five requests in: factor %.2f, want 1", got)
	}
	if p := DefaultParamsFor(8); p.LeadCost != 0 || p.MinorCost != 0 || p.FridayMarkup != 0 {
		t.Errorf("the dealer cost and Friday switches should be off by default")
	}
}
