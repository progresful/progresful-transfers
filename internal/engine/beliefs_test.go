package engine

import (
	"math"
	"testing"
)

func beliefParams() Params {
	p := DefaultParamsFor(8)
	p.MarketModel = marketModelBeliefs
	p.MarketStart = 1.35
	return p
}

func quoteAt(b *beliefBook, dealer, pickup, delivery string, day int, ratio float64) pendingQuote {
	level, _ := b.level(dealer, pickup, delivery, day)
	return pendingQuote{Dealer: dealer, Place: pickup, Dest: delivery, Day: day, Level: level, Ratio: ratio}
}

// Lost quotes pull the level down and won quotes push it up, and every answer makes the
// win curve narrower.
func TestBeliefsFollowAnswers(t *testing.T) {
	tests := []struct {
		name  string
		ratio float64
		won   bool
		up    bool
	}{
		{"quotes under the level that keep losing", 1.25, false, false},
		{"quotes over the level that keep winning", 1.45, true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := newBeliefBook(beliefParams())
			before, wide := b.level("D01", "oulu", "kemi", 0)
			for i := 0; i < 20; i++ {
				b.learn(quoteAt(b, "D01", "oulu", "kemi", 0, tc.ratio), tc.won)
			}
			after, narrow := b.level("D01", "oulu", "kemi", 0)
			if tc.up != (after > before) || after == before {
				t.Errorf("level %.3f -> %.3f, want up=%v", before, after, tc.up)
			}
			if narrow >= wide {
				t.Errorf("width %.4f -> %.4f: answers should narrow the curve", wide, narrow)
			}
			if narrow < b.scale {
				t.Errorf("width %.4f is under the dealer's own width %.4f", narrow, b.scale)
			}
		})
	}
}

// The less a part is known, the more of a surprise it takes: the first answer of a week
// moves the world level more than a dealer's part, and an answer after many moves little.
func TestBeliefsShareTheSurpriseByWhatIsUnknown(t *testing.T) {
	b := newBeliefBook(beliefParams())
	world := b.world.M
	b.learn(quoteAt(b, "D01", "oulu", "kemi", 0, 1.2), false)
	worldStep, dealerStep := world-b.world.M, -b.dealers["D01"].M
	if worldStep <= dealerStep || dealerStep <= 0 {
		t.Errorf("first answer: world moved %.4f, dealer %.4f; want world > dealer > 0", worldStep, dealerStep)
	}
	for i := 0; i < 200; i++ {
		b.learn(quoteAt(b, "D01", "oulu", "kemi", 0, math.Exp(b.world.M)), i%2 == 0)
	}
	world = b.world.M
	b.learn(quoteAt(b, "D01", "oulu", "kemi", 0, 1.2), false)
	if late := world - b.world.M; late >= worldStep/4 {
		t.Errorf("after 200 answers one more moved the world by %.4f, the first by %.4f", late, worldStep)
	}
}

// A new day forgets the day factor, and the competitor may have moved its prices for a
// dealer or a place overnight; an answer to yesterday's quote says nothing about today.
func TestBeliefsStartEachDayAfresh(t *testing.T) {
	b := newBeliefBook(beliefParams())
	for i := 0; i < 30; i++ {
		b.learn(quoteAt(b, "D01", "oulu", "kemi", 0, 1.2), false)
	}
	if b.day.M >= 0 {
		t.Fatalf("a day of losses should lower the day factor, got %.4f", b.day.M)
	}
	dealerV := b.dealers["D01"].V
	late := quoteAt(b, "D01", "oulu", "kemi", 0, 1.2)
	b.level("D01", "oulu", "kemi", 1)
	if b.day.M != 0 || b.day.V != b.priorDay*b.priorDay {
		t.Errorf("new day: day factor %.4f (variance %.5f), want the prior", b.day.M, b.day.V)
	}
	if b.dealers["D01"].V <= dealerV {
		t.Errorf("new day: the dealer's variance should grow, %.6f -> %.6f", dealerV, b.dealers["D01"].V)
	}
	b.learn(late, false)
	if b.day.M != 0 {
		t.Errorf("yesterday's answer moved today's factor to %.4f", b.day.M)
	}
}

// The level of a lane carries the place effects that hold in every world.
func TestBeliefLevelCarriesPlaceEffects(t *testing.T) {
	b := newBeliefBook(beliefParams())
	toCapital, _ := b.level("D01", "oulu", "vantaa", 0)
	toRemote, _ := b.level("D01", "oulu", "kajaani", 0)
	if want := placeDeliveryEffect["kajaani"] - placeDeliveryEffect["vantaa"]; math.Abs(toRemote-toCapital-want) > 1e-9 || want <= 0 {
		t.Errorf("remote minus capital delivery: %.3f, want %.3f (positive)", toRemote-toCapital, want)
	}
	if unknown, _ := b.level("D01", "nowhere", "elsewhere", 0); math.Abs(unknown-b.world.M) > 1e-9 {
		t.Errorf("unknown places should count as average: %.3f vs %.3f", unknown, b.world.M)
	}
}

// The engine quotes with the belief model, learns from the answer, and keeps what it
// learned over a restart.
func TestEngineQuotesAndLearnsWithBeliefs(t *testing.T) {
	p := beliefParams()
	e := New(8, p, testMap(t))
	if e.market.beliefs == nil {
		t.Fatal("marketModel 2 should build the belief model")
	}
	d := e.market.dealer("D01")
	d.trust = 10
	level, width := e.market.beliefs.level("D01", "a", "b", 0)
	q := pendingQuote{Dealer: "D01", Place: "a", Dest: "b", Day: 0, Level: level + e.market.trustShift(d, 1), Ratio: 1.2}
	if q.Level <= level || width <= 0 {
		t.Fatalf("trust should raise the level: %.3f vs %.3f (width %.3f)", q.Level, level, width)
	}
	e.quotes["R1"] = q
	before := e.market.worldRatio()
	e.onQuoteAnswer("R1", false)
	if e.market.worldRatio() >= before || d.quotes != 1 || d.wins != 0 {
		t.Errorf("a lost quote: world ratio %.3f -> %.3f, dealer quotes %d wins %d", before, e.market.worldRatio(), d.quotes, d.wins)
	}
	if _, waiting := e.quotes["R1"]; waiting {
		t.Error("an answered quote should be forgotten")
	}
	path := t.TempDir() + "/state.json"
	if err := e.SaveState(path); err != nil {
		t.Fatal(err)
	}
	fresh := New(8, p, testMap(t))
	if err := fresh.LoadState(path); err != nil {
		t.Fatal(err)
	}
	got, want := fresh.market.beliefs, e.market.beliefs
	if got.world != want.world || got.day != want.day || got.dayIndex != want.dayIndex {
		t.Errorf("restored world %+v day %+v (%d), want %+v %+v (%d)", got.world, got.day, got.dayIndex, want.world, want.day, want.dayIndex)
	}
	if got.dealers["D01"] == nil || *got.dealers["D01"] != *want.dealers["D01"] || *got.pickups["a"] != *want.pickups["a"] || *got.deliveries["b"] != *want.deliveries["b"] {
		t.Error("restored dealer and place beliefs differ from the saved ones")
	}
}

// Without the switch the running-rate model stays in charge.
func TestBeliefsAreOffByDefaultBelowTheSwitch(t *testing.T) {
	p := DefaultParams()
	if e := New(5, p, testMap(t)); e.market.beliefs != nil {
		t.Error("marketModel 0 should not build the belief model")
	}
}
