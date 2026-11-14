package engine

import (
	"testing"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

// pastOf is the history of a world that ran without the system: requests that nobody
// answered and that went to the competitor.
func pastOf(n int) []api.Event {
	var history []api.Event
	for i := 0; i < n; i++ {
		at := cal.At(i%3, 9*cal.Hour) + i
		req := testRequest("P"+string(rune('a'+i%26))+string(rune('a'+i/26)), at)
		history = append(history,
			api.Event{Seq: 2*i + 1, At: at, Kind: api.KindPriceRequest, Request: req},
			api.Event{Seq: 2*i + 2, At: at + 15, Kind: api.KindOrderLost, RequestID: req.ID})
	}
	return history
}

// Joining a world with a past must not teach the price model anything: the dealers'
// old answers were not answers to our prices. The engines up to e16 priced every past
// request anew and counted each as a lost quote.
func TestRejoinLearnsNothingFromRequestsItNeverQuoted(t *testing.T) {
	thursday := cal.At(3, 0)
	snap := &api.Snapshot{Now: thursday, EndsAt: cal.At(9, 22*cal.Hour)}

	p := DefaultParamsFor(8)
	p.RiskMax = 1 // the test map has no drivers: without a limit every request is priced, as most are in a real week
	e := New(8, p, testMap(t))
	before := e.market.worldRatio()
	e.Rejoin(snap, nil, pastOf(60))
	if got := e.market.worldRatio(); got != before {
		t.Errorf("the price level moved from %.4f to %.4f on a past we took no part in", before, got)
	}
	if len(e.quotes) != 0 {
		t.Errorf("%d quotes wait for an answer after the replay, want none", len(e.quotes))
	}
	if n := len(e.Decisions()); n != 0 {
		t.Errorf("%d decisions recorded for requests of the past, want none", n)
	}
	if e.stats.Requests != 60 || len(e.reqs) != 60 {
		t.Errorf("the past's requests should still be known: counted %d, kept %d", e.stats.Requests, len(e.reqs))
	}

	p.ReplayQuotes = 1
	old := New(8, p, testMap(t))
	old.Rejoin(snap, nil, pastOf(60))
	if got := old.market.worldRatio(); got >= before {
		t.Errorf("with the old fault switched on the level should fall: %.4f -> %.4f", before, got)
	}
}

// A world joined after its first day with no order of ours is priced on JoinCost; a full
// week and a restart in the middle of one keep CostMult; a restart of a late join keeps
// what the first join found.
func TestJoinCostOnlyInAWorldJoinedLate(t *testing.T) {
	p := DefaultParamsFor(8)
	if p.JoinCost != 1.1 || p.CostMult != 1.2 {
		t.Fatalf("level 8 defaults: join cost %.2f, cost %.2f, want 1.1 and 1.2", p.JoinCost, p.CostMult)
	}
	for level := 1; level <= 7; level++ {
		if got := DefaultParamsFor(level).JoinCost; got != 0 {
			t.Errorf("level %d: join cost %.2f, want none", level, got)
		}
	}
	end := cal.At(9, 22*cal.Hour)
	req := testRequest("R1", 600)
	won := api.Event{Seq: 2, At: 615, Kind: api.KindOrderWon, Order: testOrder(req, "waiting", nil)}
	cases := []struct {
		name    string
		now     int
		history []api.Event
		orders  []api.Order
		want    float64
	}{
		{"a full week from Monday", 0, nil, nil, 1.2},
		{"joined on Thursday, nothing of ours", cal.At(3, 0), pastOf(10), nil, 1.1},
		{"a restart on Thursday with an order of ours", cal.At(3, 0), []api.Event{{Seq: 1, At: 600, Kind: api.KindPriceRequest, Request: req}, won}, []api.Order{*testOrder(req, "waiting", nil)}, 1.2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := New(8, p, testMap(t))
			e.Rejoin(&api.Snapshot{Now: tc.now, EndsAt: end, Orders: tc.orders}, nil, tc.history)
			if got := e.costMult(); got != tc.want {
				t.Errorf("cost multiple %.2f, want %.2f", got, tc.want)
			}
		})
	}
	// The practice path (no history) and the saved state.
	e := New(8, p, testMap(t))
	e.Begin(&api.Snapshot{Now: cal.At(3, 0), EndsAt: end}, nil)
	if e.costMult() != 1.1 {
		t.Errorf("a practice world joined on Thursday: cost %.2f, want 1.1", e.costMult())
	}
	path := t.TempDir() + "/state.json"
	if err := e.SaveState(path); err != nil {
		t.Fatal(err)
	}
	again := New(8, p, testMap(t))
	again.Rejoin(&api.Snapshot{Now: cal.At(3, 12*cal.Hour), EndsAt: end, Orders: []api.Order{*testOrder(req, "waiting", nil)}}, nil, []api.Event{{Seq: 1, At: 600, Kind: api.KindPriceRequest, Request: req}, won})
	if err := again.LoadState(path); err != nil {
		t.Fatal(err)
	}
	if again.costMult() != 1.1 {
		t.Errorf("a restart of a late join should keep its cost: %.2f, want 1.1", again.costMult())
	}
	p.JoinCost = 0
	off := New(8, p, testMap(t))
	off.Begin(&api.Snapshot{Now: cal.At(3, 0), EndsAt: end}, nil)
	if off.costMult() != 1.2 {
		t.Errorf("switched off, a late join keeps the full cost: %.2f", off.costMult())
	}
}

// On the Monday of a full week the cost of an order is FirstDayCost: the day's orders are
// the trust that the rest of the week is sold on. From Tuesday on, and with the switch
// off, it is CostMult.
func TestFirstDayCost(t *testing.T) {
	tests := []struct {
		name  string
		first float64
		day   int
		want  float64
	}{
		{"monday", 1.05, 0, 1.05},
		{"tuesday", 1.05, 1, 1.2},
		{"friday", 1.05, 4, 1.2},
		{"switched off", 0, 0, 1.2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := DefaultParamsFor(8)
			p.FirstDayCost = tc.first
			e := New(8, p, testMap(t))
			e.now = cal.At(tc.day, 10*cal.Hour)
			if got := e.costMult(); got != tc.want {
				t.Errorf("cost %.2f, want %.2f", got, tc.want)
			}
		})
	}
}

// Friday's cost factor applies in a week played from its Monday; a world joined mid-week
// with no order of ours has its own cost and gets no second discount.
func TestFridayCostNotOnTopOfTheJoinCost(t *testing.T) {
	p := DefaultParamsFor(8)
	p.FridayCost = 0.9
	e := New(8, p, testMap(t))
	e.now = cal.At(4, 10*cal.Hour)
	if got := e.dayCost(); got != 0.9 {
		t.Errorf("a full week's Friday: factor %.2f, want 0.90", got)
	}
	e.cold = true
	if got := e.dayCost(); got != 1 {
		t.Errorf("a world joined mid-week: factor %.2f, want 1", got)
	}
	e.cold = false
	e.now = cal.At(3, 10*cal.Hour)
	if got := e.dayCost(); got != 1 {
		t.Errorf("a Thursday: factor %.2f, want 1", got)
	}
}
