package engine

import (
	"math"
	"path/filepath"
	"testing"

	"progresful-transfers/internal/api"
	"progresful-transfers/internal/cal"
)

func warParams() Params {
	p := beliefParams()
	p.War = 0.8
	return p
}

// answer makes a quote against today's level, shifted by offset (log ratio), and feeds its answer.
func answer(b *beliefBook, day int, offset float64, won bool) {
	level, _ := b.level("D01", "oulu", "kemi", day)
	b.learn(pendingQuote{Dealer: "D01", Place: "oulu", Dest: "kemi", Day: day, Level: level, Base: level, Ratio: math.Exp(level + offset)}, won)
}

// warDay feeds n answers of a day on which the competitor is a fifth cheaper than the usual
// level: each quote goes out near the level the model believes in, a little over or under
// it, and wins only when it is under the war's level.
func warDay(b *beliefBook, day int, usual float64, n int) {
	truth := usual + math.Log(0.8)
	for i := 0; i < n; i++ {
		level, _ := b.level("D01", "oulu", "kemi", day)
		lr := level + 0.03
		if i%2 == 1 {
			lr = level - 0.03
		}
		if chance, shift := b.warCase(); chance >= 0.5 {
			lr += shift
		}
		b.learn(pendingQuote{Dealer: "D01", Place: "oulu", Dest: "kemi", Day: day, Level: level, Base: level, Ratio: math.Exp(lr)}, lr < truth)
	}
}

// calmDay feeds a working day of answers as the usual prices give them: quotes at the level
// win half the time.
func calmDay(b *beliefBook, day int) {
	for i := 0; i < 30; i++ {
		answer(b, day, 0, i%2 == 0)
	}
}

// Before any answer the chance of a war today is one in the working days still open, and a
// week whose first four days were calm leaves Friday (nearly) sure.
func TestWarPriorFollowsTheDaysLeft(t *testing.T) {
	b := newBeliefBook(warParams())
	want := []float64{1.0 / 5, 1.0 / 4, 1.0 / 3, 1.0 / 2, warPriorMax}
	for day := 0; day < requestDays; day++ {
		b.level("D01", "oulu", "kemi", day)
		if got := b.war.chance(); math.Abs(got-want[day]) > 1e-9 {
			t.Errorf("day %d: chance of a war %.3f before any answer, want %.3f", day, got, want[day])
		}
		calmDay(b, day)
		if got := b.war.chance(); got > warCalm/2 {
			t.Errorf("day %d: after a calm day's answers the chance is %.3f, want it far under %.2f", day, got, warCalm)
		}
	}
	b.level("D01", "oulu", "kemi", 5)
	if got := b.war.chance(); got != 0 {
		t.Errorf("a Saturday has no war, got %.3f", got)
	}
}

// A day that was not watched (too few answers) may have been the war day: it is not
// counted as calm.
func TestWarPriorIgnoresADayWithoutAnswers(t *testing.T) {
	b := newBeliefBook(warParams())
	b.level("D01", "oulu", "kemi", 0)
	answer(b, 0, 0, true)
	b.level("D01", "oulu", "kemi", 1)
	if got := b.war.chance(); math.Abs(got-0.2) > 1e-9 {
		t.Errorf("after a day with one answer the chance is %.3f, want the 0.2 of an unwatched week", got)
	}
}

// Quotes at the usual price that keep losing make a war likely within a handful of
// answers, and the found war is the week's only one.
func TestWarIsFoundInAFewLosses(t *testing.T) {
	b := newBeliefBook(warParams())
	calmDay(b, 0)
	calmDay(b, 1)
	usual, _ := b.level("D01", "oulu", "kemi", 2)
	for i := 0; i < 6; i++ {
		answer(b, 2, -0.02, false)
	}
	if got := b.war.chance(); got < 0.85 {
		t.Fatalf("after six lost quotes just under the level the chance of a war is %.2f, want over 0.85", got)
	}
	// The rest of the day goes as a war day does: only quotes a fifth lower win.
	warDay(b, 2, usual, 30)
	if got := b.war.chance(); got < warSure {
		t.Fatalf("at the end of the war day the chance is %.2f, want it sure", got)
	}
	b.level("D01", "oulu", "kemi", 3)
	if got := b.war.chance(); got != 0 || !b.war.over {
		t.Errorf("the day after a found war: chance %.2f, over %v; the week has no other", got, b.war.over)
	}
}

// The war's losses belong to the war: the dealer's and the world's prices of the next day
// stay where they were, instead of a few percent lower.
func TestWarLeavesThePersistentPricesAlone(t *testing.T) {
	after := func(war float64) (before, next float64) {
		p := beliefParams()
		p.War = war
		b := newBeliefBook(p)
		calmDay(b, 0)
		calmDay(b, 1)
		before, _ = b.level("D01", "oulu", "kemi", 2)
		warDay(b, 2, before, 40)
		next, _ = b.level("D01", "oulu", "kemi", 3)
		return before, next
	}
	before, plain := after(0)
	_, watched := after(0.8)
	if plain >= before-0.005 {
		t.Fatalf("without the watch the war should drag the next day's level down: %.3f -> %.3f", before, plain)
	}
	if math.Abs(watched-before) >= math.Abs(plain-before) {
		t.Errorf("with the watch the next day's level is %.3f, without %.3f; it started at %.3f and should stay nearer", watched, plain, before)
	}
}

// With BeliefFresh an answer is judged against the level as it is now, not as it was when
// the quote went out: two quotes sent together and both lost do not count the surprise twice.
func TestBeliefsLearnAgainstTheCurrentLevel(t *testing.T) {
	drop := func(fresh int) float64 {
		p := beliefParams()
		p.BeliefFresh = fresh
		b := newBeliefBook(p)
		level, _ := b.level("D01", "oulu", "kemi", 0)
		q := pendingQuote{Dealer: "D01", Place: "oulu", Dest: "kemi", Day: 0, Level: level, Base: level, Ratio: math.Exp(level - 0.05)}
		b.learn(q, false)
		b.learn(q, false)
		after, _ := b.level("D01", "oulu", "kemi", 0)
		return level - after
	}
	stale, fresh := drop(0), drop(1)
	if stale <= 0 || fresh <= 0 || fresh >= stale {
		t.Errorf("two losses lower the level by %.4f against the saved level and %.4f against the current one: want the second smaller", stale, fresh)
	}
}

// The engine prices on both cases: once a war is the likely case the quote is lower than
// the belief model alone would make it, the decision records the chance, and without the
// switch nothing changes.
func TestEngineQuotesLowerInAWar(t *testing.T) {
	quote := func(war float64, losses int) (price int, chance float64) {
		p := DefaultParamsFor(8)
		p.War = war
		p.RiskMax = 1 // the test map has no drivers
		e := New(8, p, testMap(t))
		e.now, e.endsAt = cal.At(1, 10*cal.Hour), 14280
		for i := 0; i < losses; i++ {
			req := testRequest("L"+string(rune('a'+i)), e.now)
			e.onRequest(req)
			e.Drain()
			e.onQuoteAnswer(req.ID, false)
		}
		e.Decisions()
		req := testRequest("R1", e.now)
		e.onRequest(req)
		sent := e.Drain()
		decided := e.Decisions()
		if len(sent) != 1 || len(decided) != 1 {
			t.Fatalf("want one quote and one decision, got %d and %d", len(sent), len(decided))
		}
		return sent[0].Action.Prices[api.NextDay], decided[0].War
	}
	if DefaultParamsFor(8).War != 0 {
		t.Skip("the default is covered by TestFeeClockByLevel when the switch is on")
	}
	first, none := quote(0, 0)
	if watched, prior := quote(0.8, 0); watched != first || none != 0 || math.Abs(prior-0.2) > 1e-9 {
		t.Errorf("before any answer: quote %d with the watch (chance %.2f), %d without (chance %.2f); want the same price and one chance in five", watched, prior, first, none)
	}
	plain, _ := quote(0, 6)
	watched, chance := quote(0.8, 6)
	if watched >= plain || chance < 0.8 {
		t.Errorf("after six losses: quote %d with the watch (chance of a war %.2f), %d without; want it lower and the chance over 0.8", watched, chance, plain)
	}
}

// The watch survives a restart.
func TestWarWatchIsSavedWithTheBeliefs(t *testing.T) {
	p := DefaultParamsFor(8)
	p.War = 0.8
	e := New(8, p, testMap(t))
	b := e.market.beliefs
	calmDay(b, 0)
	b.level("D01", "oulu", "kemi", 1)
	for i := 0; i < 6; i++ {
		answer(b, 1, -0.02, false)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := e.SaveState(path); err != nil {
		t.Fatal(err)
	}
	again := New(8, p, testMap(t))
	if err := again.LoadState(path); err != nil {
		t.Fatal(err)
	}
	got, want := again.market.beliefs.war, b.war
	if got.calmDays != want.calmDays || got.over != want.over || got.answers != want.answers ||
		math.Abs(got.odds-want.odds) > 1e-9 || math.Abs(got.day.M-want.day.M) > 1e-9 {
		t.Errorf("restored watch %+v, want %+v", got, want)
	}
}
