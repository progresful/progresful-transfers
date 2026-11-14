package engine

import (
	"math"
	"testing"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
	"highwaycoins/internal/geo"
)

// outageMap has two towns close together (one region) and one far away.
func outageMap(t *testing.T) *geo.Map {
	t.Helper()
	places := []api.Place{
		{ID: "a", Name: "A", Lat: 60.2, Lon: 24.9}, {ID: "b", Name: "B", Lat: 60.3, Lon: 25.0},
		{ID: "far", Name: "Far", Lat: 65.0, Lon: 25.5},
	}
	var roads []api.Road
	for _, from := range places {
		for _, to := range places {
			roads = append(roads, api.Road{From: from.ID, To: to.ID, Km: 30, Minutes: 30})
		}
	}
	m, err := geo.New(&api.WorldMap{Places: places, Roads: roads})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

const (
	testLevel = 0.30 // log of 1.35 times the list price
	testWidth = 0.07
)

func watchAt(t *testing.T, day, hour int) (*outageWatch, int) {
	t.Helper()
	return newOutageWatch(outageMap(t), 0.05, 1.3), cal.At(day, hour*cal.Hour)
}

// A run of wins at the usual price in one region says its dealers are paying the backup;
// one lost quote there says they are not.
func TestOutageWatchFollowsARegionsAnswers(t *testing.T) {
	w, now := watchAt(t, 1, 10)
	usual := math.Exp(testLevel) // a quote at the level: it wins half the time without an outage
	before := w.chance("a", now)
	if before <= 0 || before > 0.1 {
		t.Fatalf("chance before any answer %.3f, want the small prior", before)
	}
	for i := 0; i < 6; i++ {
		w.learn("a", now, testLevel, testWidth, usual, true)
	}
	sure := w.chance("a", now)
	if sure < 0.7 {
		t.Errorf("after six wins in a row the chance is %.2f, want over 0.7", sure)
	}
	if near := w.chance("b", now); near != sure {
		t.Errorf("a town in the same region has chance %.2f, want %.2f", near, sure)
	}
	if far := w.chance("far", now); far > 0.1 {
		t.Errorf("a town far away has chance %.2f, want the prior", far)
	}
	w.learn("b", now, testLevel, testWidth, usual, false)
	if after := w.chance("a", now); after > 0.3 {
		t.Errorf("after a lost quote in the region the chance is %.2f, want it low again", after)
	}
}

// The outage lasts one working day from 08:00 to 18:00, and there is one a week.
func TestOutageWatchKeepsToItsHoursAndItsDay(t *testing.T) {
	w, now := watchAt(t, 1, 10)
	usual := math.Exp(testLevel)
	for i := 0; i < 8; i++ {
		w.learn("a", now, testLevel, testWidth, usual, true)
	}
	if got := w.chance("a", cal.At(1, 19*cal.Hour)); got != 0 {
		t.Errorf("after 18:00 the chance is %.2f, want 0", got)
	}
	if got := w.chance("a", cal.At(2, 10*cal.Hour)); got != 0 {
		t.Errorf("the day after a found outage the chance is %.2f, want 0: there is one a week", got)
	}
	// Answers outside the hours, and of a quote made on an earlier day, teach nothing.
	w, _ = watchAt(t, 1, 7)
	w.learn("a", cal.At(1, 7*cal.Hour), testLevel, testWidth, usual, true)
	w.learn("a", cal.At(0, 12*cal.Hour), testLevel, testWidth, usual, true)
	if got, prior := w.chance("a", cal.At(1, 9*cal.Hour)), 0.05/1.05; math.Abs(got-prior) > 1e-9 {
		t.Errorf("chance %.4f, want the prior %.4f", got, prior)
	}
	// A weekend has no outage.
	if got := w.chance("a", cal.At(5, 10*cal.Hour)); got != 0 {
		t.Errorf("on a Saturday the chance is %.2f, want 0", got)
	}
}

// The quote follows the watch: with the region's dealers on the backup the price that
// pays best is far above the usual one, and with a small chance it stays where it was.
func TestOutageRatioPricesOnBothCases(t *testing.T) {
	cost, floor := 0.75, 0.8
	usual, _ := bestRatioAt(testLevel, testWidth, cost, 0, floor)
	tests := []struct {
		name   string
		chance float64
		higher bool
	}{
		{"hardly any chance", 0.05, false},
		{"an even chance", 0.5, false},
		{"nearly sure", 0.9, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, win := outageRatio(tc.chance, 1.3, testLevel, testWidth, cost, 0, floor)
			if tc.higher && got < usual*1.15 {
				t.Errorf("ratio %.3f, want well above the usual %.3f", got, usual)
			}
			if !tc.higher && math.Abs(got-usual) > 1e-9 {
				t.Errorf("ratio %.3f, want the usual %.3f", got, usual)
			}
			if win <= 0 || win > 1 {
				t.Errorf("win chance %.3f out of range", win)
			}
		})
	}
}

// The engine wires it together: off by default, and with the switch a region's win streak
// lifts the next quote there.
func TestEngineQuotesHigherInAnOutage(t *testing.T) {
	quote := func(outage float64, wins int) int {
		p := DefaultParamsFor(8)
		p.Outage = outage
		p.RiskMax = 1 // the test map has no drivers
		e := New(8, p, testMap(t))
		e.now, e.endsAt = cal.At(1, 10*cal.Hour), 14280
		for i := 0; i < wins; i++ {
			req := testRequest("W"+string(rune('a'+i)), e.now)
			e.onRequest(req)
			e.Drain()
			e.onQuoteAnswer(req.ID, true)
		}
		req := testRequest("R1", e.now)
		e.onRequest(req)
		sent := e.Drain()
		if len(sent) != 1 {
			t.Fatalf("want one quote, got %d", len(sent))
		}
		return sent[0].Action.Prices[api.NextDay]
	}
	if DefaultParamsFor(8).Outage != 0 {
		t.Skip("the default is covered by TestFeeClockByLevel when the switch is on")
	}
	calm, watched := quote(1.3, 0), quote(1.3, 20)
	if calm != quote(0, 0) {
		t.Errorf("without a win streak the watch should not move the quote: %d against %d", calm, quote(0, 0))
	}
	if watched < calm*120/100 {
		t.Errorf("after twenty wins in the region the quote is %d, the first of the day was %d: want it at least 20%% higher", watched, calm)
	}
}
