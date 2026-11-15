package engine

import (
	"math"
	"testing"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

// cheapTwoOptionRequest is a cheap car to move with a same-day and a next-day option.
func cheapTwoOptionRequest(now int) *api.PriceRequest {
	req := testRequest("R1", now)
	req.ReferenceFeeCents = 1500 // cheap to carry: the price that pays best is far under the market
	req.Options = []api.DeliveryOption{
		{Kind: api.SameDay, Deadline: cal.At(1, 20*cal.Hour), ListPriceCents: 6500},
		{Kind: api.NextDay, Deadline: cal.At(2, 20*cal.Hour), ListPriceCents: 6000},
	}
	return req
}

// A menu without the request's fastest option is refused by the dealers that want speed,
// whatever its price: a quote deeper than the point where that shows buys nothing.
func TestPartialMenuIsNotPricedDeep(t *testing.T) {
	ratioFor := func(partialCap float64, option int) (ratio, level, width float64) {
		p := DefaultParamsFor(8)
		p.PartialCap = partialCap
		p.RiskMax = 1
		e := New(8, p, testMap(t))
		e.now, e.endsAt = cal.At(1, 10*cal.Hour), 14280
		req := cheapTwoOptionRequest(e.now)
		ratio = e.quoteRatio(req, req.Options[option], 0, 0)
		q := e.quotes[req.ID]
		return ratio, q.Level, q.Width
	}
	deep, level, width := ratioFor(0, 1)
	if z := (level - math.Log(deep)) / width; z < 0.8 {
		t.Fatalf("the test needs a quote priced deep under the market, got z = %.2f", z)
	}
	capped, _, _ := ratioFor(0.62, 1)
	if want := math.Exp(level - math.Log(0.62/0.38)*width); math.Abs(capped-want) > 1e-9 {
		t.Errorf("the slower option alone is quoted at %.4f of list, want %.4f: no deeper than a 62%% chance", capped, want)
	}
	if full, _, _ := ratioFor(0.62, 0); math.Abs(full-func() float64 { r, _, _ := ratioFor(0, 0); return r }()) > 1e-12 {
		t.Errorf("a menu with the fastest option must not change, got %.4f", full)
	}
	if !func() bool {
		p := DefaultParamsFor(8)
		p.PartialCap, p.RiskMax = 0.62, 1
		e := New(8, p, testMap(t))
		e.now, e.endsAt = cal.At(1, 10*cal.Hour), 14280
		req := cheapTwoOptionRequest(e.now)
		e.quoteRatio(req, req.Options[1], 0, 0)
		return e.quotes[req.ID].Partial
	}() {
		t.Error("the quote should be marked as one without the fastest option")
	}
}

// A lost quote without the fastest option says less about the price: the dealer may have
// wanted the speed. A won one says as much as any.
func TestPartialMenuLossTeachesLess(t *testing.T) {
	move := func(share float64, partial, won bool) float64 {
		p := beliefParams()
		p.PartialShare = share
		b := newBeliefBook(p)
		level, _ := b.level("D01", "oulu", "kemi", 0)
		b.learn(pendingQuote{Dealer: "D01", Place: "oulu", Dest: "kemi", Day: 0, Level: level, Base: level, Ratio: math.Exp(level - 0.03), Partial: partial}, won)
		after, _ := b.level("D01", "oulu", "kemi", 0)
		return after - level
	}
	full, damped := move(0.7, false, false), move(0.7, true, false)
	if full >= 0 || damped >= 0 || damped < 0.8*full {
		t.Errorf("a loss moves the level by %.4f, one without the fastest option by %.4f: want clearly less", full, damped)
	}
	if off := move(0, true, false); off != full {
		t.Errorf("switched off, a partial loss moves the level by %.4f, want %.4f", off, full)
	}
	if win, partialWin := move(0.7, false, true), move(0.7, true, true); win != partialWin || win <= 0 {
		t.Errorf("a win teaches the same either way: %.4f and %.4f", win, partialWin)
	}
}
