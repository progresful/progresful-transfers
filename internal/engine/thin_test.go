package engine

import (
	"fmt"
	"testing"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

func rosterOf(n int) []api.DriverProfile {
	roster := make([]api.DriverProfile, n)
	for i := range roster {
		roster[i] = api.DriverProfile{ID: fmt.Sprintf("d%04d", i), HomePlaceID: "a"}
	}
	return roster
}

// A world with few drivers runs later than the lane model says (its weeks were late
// 1.3-1.4 times as often as modelled, the others 0.7-1.0 times): there the modelled
// risk of a request is multiplied by ThinRisk.
func TestThinRosterRaisesTheRisk(t *testing.T) {
	tests := []struct {
		name    string
		drivers int
		factor  float64
		want    float64 // the risk as a multiple of the same request's in a world with drivers enough
	}{
		{"few drivers", 600, 1.5, 1.5},
		{"just under the bar", 719, 1.5, 1.5},
		{"at the bar", 720, 1.5, 1},
		{"drivers enough", 900, 1.5, 1},
		{"switched off", 600, 0, 1},
		{"no roster known", 0, 1.5, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := DefaultParamsFor(8)
			p.ThinRoster, p.ThinRisk = 720, tc.factor
			e := New(8, p, testMap(t))
			e.Begin(&api.Snapshot{Now: cal.At(0, 10*cal.Hour), EndsAt: 14280}, rosterOf(tc.drivers))
			if got := e.thinRisk(0.04); got < tc.want*0.04-1e-12 || got > tc.want*0.04+1e-12 {
				t.Errorf("risk %.4f, want %.4f", got, tc.want*0.04)
			}
			if got := e.thinRisk(0.9); got > 1 {
				t.Errorf("risk %.2f is above 1", got)
			}
		})
	}
}

// The factor reaches the price request: the same request is riskier in the thin world.
func TestThinRosterReachesTheQuote(t *testing.T) {
	risk := func(drivers int) float64 {
		p := DefaultParamsFor(8)
		p.ThinRoster, p.ThinRisk = 720, 2
		e := New(8, p, testMap(t))
		e.Begin(&api.Snapshot{Now: cal.At(0, 10*cal.Hour), EndsAt: 14280}, rosterOf(drivers))
		req := testRequest("R1", e.now)
		return e.lateRisk(req, req.Options[0])
	}
	// The lane model counts the drivers near the pickup, so compare two thin and two full rosters.
	thin, full := risk(700), risk(721)
	if thin <= full || full <= 0 {
		t.Errorf("risk with 700 drivers %.4f, with 721 drivers %.4f: want the thin world riskier", thin, full)
	}
}

// In a world with drivers enough an order is priced on RosterCost; with fewer drivers,
// or with the switch off, on CostMult.
func TestRosterCost(t *testing.T) {
	tests := []struct {
		name    string
		cost    float64
		drivers int
		want    float64
	}{
		{"many drivers", 1.1, 800, 1.1},
		{"at the bar", 1.1, 720, 1.1},
		{"few drivers", 1.1, 719, 1.2},
		{"switched off", 0, 800, 1.2},
		{"no roster known", 1.1, 0, 1.2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := DefaultParamsFor(8)
			p.RosterCost, p.RosterFrom = tc.cost, 720
			e := New(8, p, testMap(t))
			// A world joined on its Monday: one joined later has the cost of a late join.
			e.Begin(&api.Snapshot{Now: cal.At(0, 10*cal.Hour), EndsAt: 14280}, rosterOf(tc.drivers))
			if got := e.costMult(); got != tc.want {
				t.Errorf("cost %.2f, want %.2f", got, tc.want)
			}
		})
	}
}
