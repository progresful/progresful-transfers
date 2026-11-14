package engine

import (
	"testing"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

func requoteEngine(t *testing.T, lead int) *Engine {
	t.Helper()
	p := DefaultParamsFor(8)
	p.RequoteLead, p.RequoteMin = lead, 0.02
	p.RiskMax = 1 // the test map has no drivers
	e := New(8, p, testMap(t))
	e.now, e.endsAt = cal.At(0, 10*cal.Hour), 14280
	return e
}

func quotesOf(sent []Pending, id string) []map[string]int {
	var out []map[string]int
	for _, p := range sent {
		if p.Action.Type == api.ActQuote && p.Action.RequestID == id {
			out = append(out, p.Action.Prices)
		}
	}
	return out
}

// A quote can be replaced until its deadline. When other dealers' answers have moved the
// engine's belief since a request was quoted, the engine quotes it again shortly before
// the deadline, once, and learns from the answer against the price that counts.
func TestRequoteWhenTheBeliefMoved(t *testing.T) {
	e := requoteEngine(t, 3)
	req := testRequest("R1", e.now)
	e.onRequest(req)
	first := quotesOf(e.Drain(), "R1")
	if len(first) != 1 {
		t.Fatalf("want one quote at once, got %d", len(first))
	}
	if wake := e.NextWake(); wake != req.RespondBy-3 {
		t.Fatalf("the engine wakes at %s, want three minutes before the deadline %s", cal.Format(wake), cal.Format(req.RespondBy))
	}
	// Other quotes are lost in the meantime: the market is lower than believed.
	for i := 0; i < 12; i++ {
		other := testRequest("L"+string(rune('a'+i)), e.now)
		e.onRequest(other)
		e.Drain()
		e.onQuoteAnswer(other.ID, false)
	}
	e.SetNow(req.RespondBy - 3)
	e.Handle(nil)
	second := quotesOf(e.Drain(), "R1")
	if len(second) != 1 {
		t.Fatalf("want one new quote before the deadline, got %d", len(second))
	}
	if second[0][api.NextDay] >= first[0][api.NextDay] {
		t.Errorf("after a run of lost quotes the new price %d should be under the first %d", second[0][api.NextDay], first[0][api.NextDay])
	}
	if got := e.quotes["R1"].Ratio * 6000; got < float64(second[0][api.NextDay])-1 || got > float64(second[0][api.NextDay])+1 {
		t.Errorf("the pending quote holds ratio %.4f, want the new price %d over the list price 6000", e.quotes["R1"].Ratio, second[0][api.NextDay])
	}
	e.SetNow(req.RespondBy - 2)
	e.Handle(nil)
	if again := quotesOf(e.Drain(), "R1"); len(again) != 0 {
		t.Errorf("a request is quoted again once, got %d more", len(again))
	}
}

// Nothing is sent when the belief has not moved, when the switch is off, or for a
// request the engine declined.
func TestNoRequoteWithoutNews(t *testing.T) {
	e := requoteEngine(t, 3)
	req := testRequest("R1", e.now)
	e.onRequest(req)
	e.Drain()
	e.SetNow(req.RespondBy - 3)
	e.Handle(nil)
	if again := quotesOf(e.Drain(), "R1"); len(again) != 0 {
		t.Errorf("no answer came in between: want no new quote, got %d", len(again))
	}

	off := requoteEngine(t, 0)
	req = testRequest("R2", off.now)
	off.onRequest(req)
	off.Drain()
	if wake := off.NextWake(); wake == req.RespondBy-3 {
		t.Errorf("switched off, the engine should not wake before the deadline")
	}

	declined := requoteEngine(t, 3)
	declined.P.RiskMax = 0 // every option is too risky
	req = testRequest("R3", declined.now)
	declined.onRequest(req)
	declined.Drain()
	declined.SetNow(req.RespondBy - 3)
	declined.Handle(nil)
	if again := quotesOf(declined.Drain(), "R3"); len(again) != 0 {
		t.Errorf("a declined request should not be quoted again, got %d", len(again))
	}
}
