package engine

import (
	"testing"

	"highwaycoins/internal/cal"
)

// Every price request leaves a record of how it was answered: the reason when it was
// passed, the modelled lateness and the limit when it was priced. The records are handed
// over once.
func TestDecisionsSayWhyARequestWasPassed(t *testing.T) {
	e := New(8, DefaultParamsFor(8), testMap(t))
	e.SetNow(cal.At(0, 10*cal.Hour))

	premium := testRequest("R1", e.now)
	premium.Legs[0].Car.Premium = true
	e.onRequest(premium)

	plain := testRequest("R2", e.now)
	e.onRequest(plain)

	got := e.Decisions()
	if len(got) != 2 {
		t.Fatalf("%d decisions, want one per request", len(got))
	}
	if got[0].RequestID != "R1" || got[0].Why != whyPremium || got[0].At != e.now {
		t.Errorf("premium car: %+v, want passed as %q at minute %d", got[0], whyPremium, e.now)
	}
	if got[1].RequestID != "R2" {
		t.Errorf("second decision is for %s, want R2", got[1].RequestID)
	}
	if got[1].Why == "" && (got[1].Cap <= 0 || got[1].Risk < 0 || got[1].Risk > got[1].Cap) {
		t.Errorf("a priced request should carry its risk under the limit: %+v", got[1])
	}
	if got[1].Why != "" && got[1].Why != whyRisky && got[1].Why != whyNoTime && got[1].Why != whyNoPrice {
		t.Errorf("an ordinary car was passed for an unknown reason: %+v", got[1])
	}
	if again := e.Decisions(); len(again) != 0 {
		t.Errorf("decisions are handed over once, got %d more", len(again))
	}
}

// Up to level 3 every quote wins at the list price: a request is passed when the list
// price is under the cost or there is too little time, and the record says which.
func TestDecisionsAtListPriceLevels(t *testing.T) {
	e := New(1, DefaultParamsFor(1), testMap(t))
	e.SetNow(cal.At(0, 10*cal.Hour))
	cheap := testRequest("R1", e.now)
	cheap.Options[0].ListPriceCents = 1000 // far under the typical fee of 3000
	e.onRequest(cheap)
	got := e.Decisions()
	if len(got) != 1 || got[0].Why != whyNoMargin {
		t.Errorf("a list price under the cost: %+v, want %q", got, whyNoMargin)
	}
}
