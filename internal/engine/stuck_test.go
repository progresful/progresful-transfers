package engine

import (
	"fmt"
	"math"
	"testing"

	"progresful-transfers/internal/cal"
)

// stuckEngine holds orders won four hours ago, of which the first unbooked have no driver.
func stuckEngine(t *testing.T, orders, unbooked int) *Engine {
	t.Helper()
	p := DefaultParamsFor(8)
	p.StuckFrom, p.StuckFull, p.StuckMin = 0.08, 0.14, 0.3
	e := New(8, p, testMap(t))
	e.now, e.endsAt = cal.At(1, 14*cal.Hour), 14280
	for i := 0; i < orders; i++ {
		ob := chainOrder(e, fmt.Sprintf("R%03d", i), "a", "b", e.now, cal.At(3, 20*cal.Hour))
		ob.wonAt = e.now - 4*cal.Hour
		if i >= unbooked {
			ob.firstBookedAt = ob.wonAt + 20
			ob.legs[0].gig = &gigBook{id: fmt.Sprintf("g%d", i), status: gigBooked}
		}
	}
	return e
}

// The stuck guard shrinks the risk cap when many orders never found a driver, and says
// nothing in an ordinary week or while few orders are in.
func TestStuckGuardShrinksTheRiskCap(t *testing.T) {
	tests := []struct {
		name             string
		orders, unbooked int
		share, factor    float64
	}{
		{"an ordinary week", 100, 3, 0.03, 1},
		{"at the start of the guard", 100, 8, 0.08, 1},
		{"half way", 100, 11, 0.11, 0.65},
		{"a week going wrong", 100, 21, 0.21, 0.3},
		{"too few orders to tell", 20, 10, 0, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := stuckEngine(t, tc.orders, tc.unbooked)
			if got := e.stuckShare(); math.Abs(got-tc.share) > 1e-9 {
				t.Errorf("stuck share %.3f, want %.3f", got, tc.share)
			}
			if got := e.stuckFactor(); math.Abs(got-tc.factor) > 1e-9 {
				t.Errorf("factor on the risk cap %.3f, want %.3f", got, tc.factor)
			}
			guarded := e.riskCap(nil)
			e.P.StuckFrom = 0
			if want := e.riskCap(nil) * tc.factor; math.Abs(guarded-want) > 1e-9 {
				t.Errorf("risk cap %.4f, want %.4f: the plain cap times the factor", guarded, want)
			}
		})
	}
}

// An order that was won in the last three hours, or that a driver holds although the time
// of its first booking is not known (after a restart), is not stuck.
func TestStuckGuardCountsOnlyOrdersLeftWithoutADriver(t *testing.T) {
	e := stuckEngine(t, 100, 10)
	for i := 0; i < 5; i++ {
		e.orders[fmt.Sprintf("R%03d", i)].wonAt = e.now - cal.Hour // too young to count
	}
	for i := 5; i < 8; i++ {
		ob := e.orders[fmt.Sprintf("R%03d", i)]
		ob.legs[0].gig = &gigBook{id: "adopted", status: gigBooked} // held, first booking unknown
	}
	if got, want := e.stuckShare(), 2.0/95; math.Abs(got-want) > 1e-9 {
		t.Errorf("stuck share %.4f, want %.4f", got, want)
	}
	if DefaultParams().StuckFrom != 0 {
		t.Error("the stuck guard is a switch: off in the base defaults")
	}
}
