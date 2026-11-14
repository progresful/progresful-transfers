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
		{7, 0.80, 0.05, 0, 0, 1, 0, 1.22, 1.10, 0.35},
		{8, 0.80, 0.05, 0, 0, 1, 0, 1.2, 1.10, 0.35},
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
	want := map[int]float64{1: 1.0, 2: 0.9, 3: 1.0, 4: 1.0, 5: 1.0, 6: 1.0, 7: 1.15, 8: 1.15}
	for level, mid := range want {
		if got := DefaultParamsFor(level).FeeMid; got != mid {
			t.Errorf("level %d holds at %.2f, want %.2f", level, got, mid)
		}
	}
}

// An order may be part of one chain attempt, of two where drivers are plentiful and every
// quote wins (level 3).
func TestChainAttemptsByLevel(t *testing.T) {
	want := map[int]int{3: 2, 5: 1, 7: 1, 8: 1}
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
