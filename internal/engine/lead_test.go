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
