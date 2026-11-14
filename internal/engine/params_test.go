package engine

import "testing"

// The defaults of each level are what its bench weeks judged best; a ranking run plays
// exactly these. This pins them so that a change is a decision, not an accident.
func TestDefaultsByLevel(t *testing.T) {
	cases := []struct {
		level                        int
		feeStart, feeStep            float64
		minLead                      int
		leadDrive                    float64
		chains, splitTradeIns        int
		costMult, floorMult, riskMax float64
	}{
		{1, 0.60, 0.05, 180, 1, 0, 0, 0.8, 1.10, 1},
		{2, 0.75, 0.02, 180, 1, 0, 0, 0.8, 1.10, 1},
		{3, 0.75, 0.02, 180, 1, 1, 1, 0.8, 1.10, 1},
		{4, 0.75, 0.02, 180, 1, 1, 1, 0.92, 0.85, 1},
		{5, 0.75, 0.02, 180, 1, 1, 1, 0.92, 0.85, 1},
		{6, 0.75, 0.02, 180, 1, 1, 1, 0.92, 0.85, 1},
		{7, 0.80, 0.05, 0, 0, 1, 0, 1.2, 1.10, 0.15},
		{8, 0.80, 0.05, 0, 0, 1, 0, 1.2, 1.10, 0.12},
	}
	for _, c := range cases {
		p := DefaultParamsFor(c.level)
		if p.FeeStart != c.feeStart || p.FeeStep != c.feeStep {
			t.Errorf("level %d: fee clock %.2f / %.2f, want %.2f / %.2f", c.level, p.FeeStart, p.FeeStep, c.feeStart, c.feeStep)
		}
		if p.MinLeadMin != c.minLead || p.MinLeadDrive != c.leadDrive {
			t.Errorf("level %d: lead rule %d + %.1f x drive, want %d + %.1f", c.level, p.MinLeadMin, p.MinLeadDrive, c.minLead, c.leadDrive)
		}
		if p.Chains != c.chains || p.SplitTradeIns != c.splitTradeIns {
			t.Errorf("level %d: chains %d, split trade-ins %d, want %d and %d", c.level, p.Chains, p.SplitTradeIns, c.chains, c.splitTradeIns)
		}
		if p.CostMult != c.costMult || p.FloorMult != c.floorMult || p.RiskMax != c.riskMax {
			t.Errorf("level %d: cost %.2f, floor %.2f, risk cap %.2f, want %.2f, %.2f, %.2f", c.level, p.CostMult, p.FloorMult, p.RiskMax, c.costMult, c.floorMult, c.riskMax)
		}
	}
}

// The fee a gig holds at before the deadline ramp: the typical fee, except where the bench
// found another level better.
func TestHoldFeeByLevel(t *testing.T) {
	want := map[int]float64{1: 1.0, 2: 0.9, 3: 1.0, 4: 1.0, 5: 1.0, 6: 1.0, 7: 1.15, 8: 1.0}
	for level, mid := range want {
		if got := DefaultParamsFor(level).FeeMid; got != mid {
			t.Errorf("level %d holds at %.2f, want %.2f", level, got, mid)
		}
	}
}

// Level 8's fee clock since e18: a low hold that creeps up before the deadline pressure,
// and no rise for a car that is already late. The other levels keep the plain clock, and
// the switches that came with e18 are off everywhere.
func TestFeeClockByLevel(t *testing.T) {
	for level := 1; level <= 8; level++ {
		p := DefaultParamsFor(level)
		holdMax, lateMax := 0.0, 1.6
		if level == 8 {
			holdMax, lateMax = 1.25, 1.0
		}
		if p.HoldMax != holdMax || p.LateMax != lateMax {
			t.Errorf("level %d: hold rises to %.2f, late fee to %.2f, want %.2f and %.2f", level, p.HoldMax, p.LateMax, holdMax, lateMax)
		}
		if p.SnowBuffer != 0 || p.SnowRisk != 0 || p.ThinRisk != 0 || p.ThinRoster != 0 || p.Outage != 0 {
			t.Errorf("level %d: snow, thin-roster and outage switches should be off: %+v", level, []float64{p.SnowBuffer, p.SnowRisk, p.ThinRisk, float64(p.ThinRoster), p.Outage})
		}
		chainGain, rush := 0.0, 0.0
		if level == 8 {
			chainGain, rush = 0.25, 0.08 // e20
		}
		if p.ChainCostGain != chainGain || p.TrustRush != rush {
			t.Errorf("level %d: chain discount %.2f, trust rush %.2f, want %.2f and %.2f", level, p.ChainCostGain, p.TrustRush, chainGain, rush)
		}
		if p.FirstDayCost != 0 || p.RosterCost != 0 || p.SpeedPremium != 0 {
			t.Errorf("level %d: first-day cost, roster cost and speed premium should be off", level)
		}
	}
}

// A late car's fee plan at level 8 stays at the typical fee and still rises towards the
// end of the world, so no car is left in the yard for want of a fee.
func TestLateFeeStaysFlatThenRisesAtTheEnd(t *testing.T) {
	p := DefaultParamsFor(8)
	now, base, endsAt := 3000, 10000, 14280
	steps := p.lateSchedule(now, base, endsAt, 4*base)
	if len(steps) < 2 || steps[0].FeeCents != base || steps[0].At != now {
		t.Fatalf("late plan %+v: want it to open at the typical fee now", steps)
	}
	if steps[1].At < now+p.LateRampMin {
		t.Errorf("the fee rises at %d, before the late ramp would have ended (%d)", steps[1].At, now+p.LateRampMin)
	}
	if last := steps[len(steps)-1]; last.FeeCents != 4*base || last.At > endsAt-p.LastLeadMin+stepGridMin {
		t.Errorf("last step %+v: want the top fee %d by %d", last, 4*base, endsAt-p.LastLeadMin)
	}
}

// An order may be part of one chain attempt; of two where drivers are plentiful and every
// quote wins (level 3), and at level 8 since e21 (+354 a week on 48 worlds).
func TestChainAttemptsByLevel(t *testing.T) {
	want := map[int]int{3: 2, 5: 1, 7: 1, 8: 2}
	for level, tries := range want {
		if got := DefaultParamsFor(level).ChainTries; got != tries {
			t.Errorf("level %d: %d chain attempts per order, want %d", level, got, tries)
		}
	}
}

// Level 8 prices each lane against its own level and prices the lateness risk into the
// cost; the levels below keep one level per world and a flat cost.
func TestLanePricingByLevel(t *testing.T) {
	for level := 1; level <= 8; level++ {
		p := DefaultParamsFor(level)
		wantLane, wantDest, wantSlope := 0.0, 0.0, 0.0
		if level == 8 {
			wantLane, wantDest, wantSlope = 1, 0.02, 1
		}
		if p.LaneWeight != wantLane || p.DestRate != wantDest || p.RiskSlope != wantSlope {
			t.Errorf("level %d: lane weight %.2f, destination rate %.3f, risk slope %.2f, want %.2f, %.3f, %.2f",
				level, p.LaneWeight, p.DestRate, p.RiskSlope, wantLane, wantDest, wantSlope)
		}
	}
}

// Levels 7 and 8 price with the belief model of the market and select with the lane risk
// model, whose calibrated chance carries the weekday itself; the levels below keep the
// running-rate market model and have no risk model.
func TestShortageModelsByLevel(t *testing.T) {
	for level := 1; level <= 8; level++ {
		p := DefaultParamsFor(level)
		wantMarket, wantRisk := 0, 2
		if level >= 7 {
			wantMarket, wantRisk = marketModelBeliefs, riskModelLaneID
		}
		if p.MarketModel != wantMarket || p.RiskModel != wantRisk {
			t.Errorf("level %d: market model %d, risk model %d, want %d and %d", level, p.MarketModel, p.RiskModel, wantMarket, wantRisk)
		}
		// The lane model carries the weekday itself; level 8 widens Friday's cap and lowers
		// Friday's cost since e22 (the last day of requests, with Saturday and Monday to deliver).
		friday, fridayCost := 1.0, 1.0
		if level == 8 {
			friday, fridayCost = 1.25, 0.95
		}
		if p.RiskThursday != 1 || p.RiskFriday != friday || p.FridayCost != fridayCost {
			t.Errorf("level %d: Thursday factor %.2f, Friday factor %.2f, Friday cost %.2f, want 1, %.2f and %.2f", level, p.RiskThursday, p.RiskFriday, p.FridayCost, friday, fridayCost)
		}
	}
}

func TestOverridesByName(t *testing.T) {
	p, err := DefaultParamsFor(8).WithOverrides(map[string]string{"costMult": "1.2", "chains": "0"})
	if err != nil {
		t.Fatal(err)
	}
	if p.CostMult != 1.2 || p.Chains != 0 {
		t.Errorf("overrides not applied: cost %.2f, chains %d", p.CostMult, p.Chains)
	}
	if _, err := DefaultParamsFor(8).WithOverrides(map[string]string{"noSuchSetting": "1"}); err == nil {
		t.Error("an unknown setting must be an error, not silently ignored")
	}
}
