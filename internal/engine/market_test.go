package engine

import (
	"math"
	"testing"

	"highwaycoins/internal/cal"
)

func testMarket() *market {
	p := DefaultParamsFor(8)
	p.MarketStart = 1.35
	return newMarket(p, 8)
}

func TestWinProbFallsWithPrice(t *testing.T) {
	m := testMarket()
	level := m.level(m.dealer("D01"), 1)
	prev := 1.0
	for _, ratio := range []float64{0.9, 1.1, 1.3, 1.35, 1.5, 1.8} {
		p := m.winProb(level, ratio)
		if p >= prev {
			t.Errorf("win chance at ratio %.2f is %.3f, not below %.3f", ratio, p, prev)
		}
		prev = p
	}
	if p := m.winProb(level, 1.35); math.Abs(p-0.5) > 1e-9 {
		t.Errorf("win chance at the market level is %.3f, want 0.5", p)
	}
}

func TestBestRatio(t *testing.T) {
	m := testMarket()
	level := m.level(m.dealer("D01"), 1)
	cheap, cheapChance := m.bestRatio(level, 0.8, 0, 0.9)
	dear, dearChance := m.bestRatio(level, 1.3, 0, 1.4)
	if cheap <= 0.8 || cheap >= 1.35 {
		t.Errorf("a cheap car is quoted at %.3f: want above its cost and under the market level", cheap)
	}
	if dear < 1.4 {
		t.Errorf("a dear car is quoted at %.3f, under its floor 1.4", dear)
	}
	if cheapChance <= dearChance {
		t.Errorf("the cheap car should win more often: %.3f vs %.3f", cheapChance, dearChance)
	}
	withBonus, _ := m.bestRatio(level, 0.8, 0.5, 0.9)
	if withBonus > cheap {
		t.Errorf("an order worth more when won should not be quoted higher: %.3f vs %.3f", withBonus, cheap)
	}
}

func TestLearnFollowsOutcomes(t *testing.T) {
	m := testMarket()
	d := m.dealer("D01")
	before := m.mu
	for i := 0; i < 30; i++ {
		m.learn(d, m.level(d, 1), 1.30, false) // quotes under the level keep losing
	}
	if m.mu >= before || d.offset >= 0 {
		t.Errorf("after losses the level should fall: mu %.3f -> %.3f, dealer offset %.3f", before, m.mu, d.offset)
	}
	low := m.mu
	for i := 0; i < 30; i++ {
		m.learn(d, m.level(d, 1), 1.40, true) // quotes above the level keep winning
	}
	if m.mu <= low {
		t.Errorf("after wins the level should rise: mu %.3f -> %.3f", low, m.mu)
	}
}

func TestTodayShiftLearnsFastAndForgetsOvernight(t *testing.T) {
	m := testMarket()
	m.dayRate = 0.03
	d := m.dealer("D01")
	if got := m.todayShift("D01", "helsinki", 2); got != 0 {
		t.Fatalf("a new day starts at no shift, got %.3f", got)
	}
	for i := 0; i < 20; i++ {
		// A day of quotes far above the usual level that all win: an outage.
		surprise := m.learn(d, m.level(d, 1), 1.6, true)
		m.learnToday("D01", "helsinki", 2, surprise)
	}
	if got := m.todayShift("D01", "helsinki", 2); got <= 0.3 {
		t.Errorf("after 20 surprising wins today's shift is %.3f, want well above 0.3", got)
	}
	if other := m.todayShift("D09", "oulu", 2); other <= 0 || other >= m.todayShift("D01", "helsinki", 2) {
		t.Errorf("another dealer elsewhere shares only the global part, got %.3f", other)
	}
	if got := m.todayShift("D01", "helsinki", 3); got != 0 {
		t.Errorf("the next morning the shift is gone, got %.3f", got)
	}
}

func TestTrustLiftsLevel(t *testing.T) {
	m := testMarket()
	d := m.dealer("D01")
	base := m.level(d, 1)
	d.trust = maxTrust
	if got, want := m.level(d, 1)-base, math.Log(1.25); math.Abs(got-want) > 1e-9 {
		t.Errorf("full trust lifts the level by %.4f, want log(1.25) = %.4f", got, want)
	}
}

func TestRequestTimeLeft(t *testing.T) {
	tests := []struct {
		name string
		now  int
		want float64
	}{
		{"before the first request", cal.At(0, 6*cal.Hour), 1},
		{"monday noon", cal.At(0, 13*cal.Hour), 0.9},
		{"wednesday closing", cal.At(2, 18*cal.Hour), 0.4},
		{"friday after the last request", cal.At(4, 19*cal.Hour), 0},
		{"saturday", cal.At(5, 12*cal.Hour), 0},
	}
	for _, tc := range tests {
		if got := requestTimeLeft(tc.now); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("%s: requestTimeLeft = %.3f, want %.3f", tc.name, got, tc.want)
		}
	}
}

func TestShiftBusinessDays(t *testing.T) {
	deadline := cal.At(1, 20*cal.Hour) // Tuesday 20:00
	tests := []struct{ days, want int }{
		{0, deadline},
		{1, cal.At(2, 20*cal.Hour)},
		{4, cal.At(7, 20*cal.Hour)}, // Saturday rolls to the last Monday
		{9, cal.At(7, 20*cal.Hour)}, // never after the last Monday
		{-1, deadline},              // a car ready earlier moves nothing
	}
	for _, tc := range tests {
		if got := shiftBusinessDays(deadline, tc.days); got != tc.want {
			t.Errorf("shift by %d days = %s, want %s", tc.days, cal.Format(got), cal.Format(tc.want))
		}
	}
}

func TestLaneShift(t *testing.T) {
	// Deliveries to remote towns sell dearer than into the capital area.
	if far, near := laneShift("vantaa", "kemi"), laneShift("vantaa", "porvoo"); far <= near+0.2 {
		t.Errorf("vantaa->kemi shift %.3f should be well above vantaa->porvoo %.3f", far, near)
	}
	if got := laneShift("nowhere", "kemi"); got != 0 {
		t.Errorf("an unknown place counts as average, got %.3f", got)
	}
	// Over all lanes of the table the shift is centred near zero.
	sum, n := 0.0, 0
	for p := range lanePickupEffect {
		for d := range laneDeliveryEffect {
			sum += laneShift(p, d)
			n++
		}
	}
	if mean := sum / float64(n); mean < -0.25 || mean > 0.25 {
		t.Errorf("unweighted mean shift %.3f is far from zero", mean)
	}
}

// The part of a lane's price that is new in each world shows in the week's own wins and
// losses: a delivery place where quotes keep winning gets a higher level, one where they
// keep losing a lower one, and other places are left alone.
func TestDestinationShiftFollowsItsOwnOutcomes(t *testing.T) {
	p := DefaultParamsFor(8)
	p.DestRate = 0.02
	m := newMarket(p, 8)
	for i := 0; i < 20; i++ {
		m.learnDest("kemi", 0.6)    // won although the model gave it 40 %
		m.learnDest("vantaa", -0.6) // lost although the model gave it 60 %
	}
	if up, down, other := m.destShift("kemi"), m.destShift("vantaa"), m.destShift("oulu"); up <= 0.05 || down >= -0.05 || other != 0 {
		t.Errorf("shifts: kemi %.3f (want up), vantaa %.3f (want down), oulu %.3f (want 0)", up, down, other)
	}
	if s := m.destShift("kemi"); s > destShiftCap {
		t.Errorf("a shift is capped at %.2f, got %.3f", destShiftCap, s)
	}
	p.DestRate = 0
	off := newMarket(p, 8)
	off.learnDest("kemi", 0.6)
	if off.destShift("kemi") != 0 {
		t.Error("switched off, nothing is learned")
	}
}

// What the week taught about delivery places survives a restart.
func TestSavedStateKeepsDestinationShifts(t *testing.T) {
	p := DefaultParamsFor(8)
	p.DestRate = 0.02
	e := New(8, p, testMap(t))
	e.market.learnDest("b", 0.5)
	e.market.learnDest("b", 0.5)
	path := t.TempDir() + "/state.json"
	if err := e.SaveState(path); err != nil {
		t.Fatal(err)
	}
	fresh := New(8, p, testMap(t))
	if err := fresh.LoadState(path); err != nil {
		t.Fatal(err)
	}
	if got, want := fresh.market.destShift("b"), e.market.destShift("b"); got != want || want == 0 {
		t.Errorf("restored shift %.4f, want %.4f (not zero)", got, want)
	}
	if fresh.market.dest["b"].quotes != 2 {
		t.Errorf("restored quote count %d, want 2", fresh.market.dest["b"].quotes)
	}
}
