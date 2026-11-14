package engine

import (
	"testing"

	"highwaycoins/internal/cal"
)

// An order that builds a dealer's trust is worth more than its margin: each on-time
// order lifts the price of the dealer's later orders. With TrustInvestRisk the engine
// bids for that worth only on requests that are safe to deliver, because a late order
// halves the trust it was meant to build.
func TestTrustInvestOnlyOnSafeRequests(t *testing.T) {
	tests := []struct {
		name   string
		invest float64
		limit  float64
		risk   float64
		trust  int
		worth  bool
	}{
		{"safe request", 0.5, 0.02, 0.01, 0, true},
		{"at the limit", 0.5, 0.02, 0.02, 0, true},
		{"risky request", 0.5, 0.02, 0.05, 0, false},
		{"no limit set: every request", 0.5, 0, 0.05, 0, true},
		{"investment off", 0, 0.02, 0.01, 0, false},
		{"trust already full", 0.5, 0.02, 0.01, maxTrust, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := DefaultParamsFor(8)
			p.TrustInvest, p.TrustInvestRisk = tc.invest, tc.limit
			e := New(8, p, testMap(t))
			e.now, e.endsAt = cal.At(0, 13*cal.Hour), 14280
			e.onRequest(testRequest("R0", e.now)) // a request seen, so a list price is known
			d := e.market.dealer("D01")
			d.requests, d.trust = 10, tc.trust
			if got := e.trustWorthFor(d, tc.risk); (got > 0) != tc.worth {
				t.Errorf("worth of the trust an order builds: %.0f cents, want some = %v", got, tc.worth)
			}
		})
	}
}
