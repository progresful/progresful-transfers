package engine

import (
	"testing"

	"progresful-transfers/internal/cal"
)

// After Friday 18:00 no request comes. An order due after that can lose no price by
// arriving late (trust halves at the late arrival): its risk cap may be wider, and no
// trust is at stake for it.
func TestEndOfWeekRiskAndTrust(t *testing.T) {
	p := DefaultParamsFor(8)
	p.EndRisk, p.TrustByDeadline = 1.5, 1
	e := New(8, p, testMap(t))
	e.now, e.endsAt = cal.At(3, 11*cal.Hour), 14280 // Thursday
	e.onRequest(testRequest("R0", e.now))           // a list price is known
	e.Drain()
	d := e.market.dealer("D01")
	d.requests, d.trust = 200, 20
	tests := []struct {
		name     string
		deadline int
		cap      float64
		stake    bool
	}{
		{"due on Thursday evening", cal.At(3, 20*cal.Hour), 1, true},
		{"due on Friday at noon (a private slot)", cal.At(4, 12*cal.Hour), 1, true},
		{"due on Friday evening", cal.At(4, 20*cal.Hour), 1.5, false},
		{"due on the Monday", cal.At(7, 20*cal.Hour), 1.5, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := e.endRisk(tc.deadline); got != tc.cap {
				t.Errorf("factor on the risk cap %.2f, want %.2f", got, tc.cap)
			}
			if got := e.trustAtStakeBy(d, tc.deadline) > 0; got != tc.stake {
				t.Errorf("trust at stake: %v, want %v", got, tc.stake)
			}
		})
	}
	// Off by default: every deadline is treated alike.
	off := New(8, DefaultParamsFor(8), testMap(t))
	off.now = e.now
	off.onRequest(testRequest("R0", off.now))
	od := off.market.dealer("D01")
	od.requests, od.trust = 200, 20
	if off.endRisk(cal.At(7, 20*cal.Hour)) != 1 || off.trustAtStakeBy(od, cal.At(7, 20*cal.Hour)) <= 0 {
		t.Error("with the switches off a deadline after the last request should change nothing")
	}
}
