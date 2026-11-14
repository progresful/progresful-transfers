package engine

import (
	"testing"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

// A request is declined when too little time is left to find a driver for its drive: the
// lead to the last on-time pickup must cover MinLeadMin plus MinLeadDrive times the drive.
func TestShortLeadIsDeclined(t *testing.T) {
	cases := []struct {
		name     string
		level    int
		arrived  int
		wantSent bool
	}{
		{"enough lead at a list-price level", 2, cal.At(1, 15*cal.Hour), true},
		{"too little lead at a list-price level", 2, cal.At(1, 16*cal.Hour), false},
		{"enough lead where prices decide", 5, cal.At(1, 15*cal.Hour), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := DefaultParamsFor(c.level)
			p.MinLeadMin, p.MinLeadDrive = 180, 1 // the drive is 30 minutes: 210 minutes of lead
			e := New(c.level, p, testMap(t))
			e.now, e.endsAt = c.arrived, 14280
			req := testRequest("R1", c.arrived) // last pickup 19:30 on the same day
			if got, want := e.lead(req, req.Options[0]), cal.At(1, 19*cal.Hour+30)-(c.arrived+15); got != want {
				t.Fatalf("lead = %d, want %d", got, want)
			}
			e.onRequest(req)
			sent := e.Drain()
			if got := len(sent) == 1 && sent[0].Action.Type == api.ActQuote; got != c.wantSent {
				t.Errorf("quote sent = %v, want %v (%+v)", got, c.wantSent, sent)
			}
		})
	}
	// Where prices decide, a declined request still gets a quote: one far above the list price.
	p := DefaultParamsFor(5)
	p.MinLeadMin, p.MinLeadDrive = 180, 1
	e := New(5, p, testMap(t))
	e.now, e.endsAt = cal.At(1, 16*cal.Hour), 14280
	req := testRequest("R2", e.now)
	e.onRequest(req)
	sent := e.Drain()
	if len(sent) != 1 || sent[0].Action.Prices[api.NextDay] < 2*req.Options[0].ListPriceCents {
		t.Errorf("want a declining quote, got %+v", sent)
	}
}

// With HoldMax the fee keeps rising after the discovery clock, up to HoldMax when the
// deadline pressure starts; without it the plan is what it was.
func TestHoldRampRisesBetweenDiscoveryAndPressure(t *testing.T) {
	p := DefaultParamsFor(8)
	p.HoldMax = 0
	now, last, base := 600, 600+30*60, 10000
	plain := p.Schedule(now, last, base)
	p.HoldMax = 1.5
	held := p.Schedule(now, last, base)
	leadStart := last - p.LeadMin
	if got, want := feeAt(plain, leadStart-5), feeOf(base, p.FeeMid); got != want {
		t.Fatalf("without the ramp the fee before the pressure is %d, want %d", got, want)
	}
	if got, want := feeAt(held, leadStart), feeOf(base, 1.5); got < want-100 || got > want {
		t.Errorf("with the ramp the fee at the start of the pressure is %d, want about %d", got, want)
	}
	mid := now + p.discoveryEnd() + (leadStart-now-p.discoveryEnd())/2
	if got := feeAt(held, mid); got <= feeOf(base, p.FeeMid) || got >= feeOf(base, 1.5) {
		t.Errorf("halfway the fee is %d, want between %d and %d", got, feeOf(base, p.FeeMid), feeOf(base, 1.5))
	}
	for i := 1; i < len(held); i++ {
		if held[i].FeeCents <= held[i-1].FeeCents || held[i].At <= held[i-1].At {
			t.Fatalf("step %d does not rise: %+v after %+v", i, held[i], held[i-1])
		}
	}
	if got, want := feeAt(held, now), feeAt(plain, now); got != want {
		t.Errorf("the opening fee changed: %d, want %d", got, want)
	}
	if got, want := held[len(held)-1].FeeCents, plain[len(plain)-1].FeeCents; got != want {
		t.Errorf("the top fee changed: %d, want %d", got, want)
	}
	// A gig posted inside the pressure window has no room for the ramp.
	short := p.Schedule(now, now+p.LeadMin-30, base)
	p.HoldMax = 0
	if want := p.Schedule(now, now+p.LeadMin-30, base); !sameSchedule(short, want) {
		t.Errorf("a gig posted inside the pressure window should keep its plan: %+v vs %+v", short, want)
	}
}
