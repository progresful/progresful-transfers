package engine

import (
	"testing"

	"highwaycoins/internal/api"
	"highwaycoins/internal/cal"
)

func snowEngine(t *testing.T) *Engine {
	t.Helper()
	p := DefaultParamsFor(8)
	p.BufferShare, p.BufferMin, p.SnowBuffer = 0.2, 1, 0.5
	e := New(8, p, testMap(t))
	e.now, e.endsAt = cal.At(0, 10*cal.Hour), 14280
	return e
}

func forecast(from, to int, places ...string) *api.Event {
	return &api.Event{Kind: api.KindForecast, At: from - cal.Day, From: from, To: to, PlaceIDs: places}
}

// A forecast is remembered as a spell of snow in its places; one without places or
// without a span of time is not.
func TestForecastIsRemembered(t *testing.T) {
	e := snowEngine(t)
	e.handle(forecast(cal.At(1, 0), cal.At(2, 0), "b"))
	e.handle(forecast(cal.At(3, 0), cal.At(3, 0), "a"))
	e.handle(forecast(cal.At(3, 0), cal.At(4, 0)))
	if len(e.snow) != 1 || !e.snow[0].places["b"] || e.snow[0].from != cal.At(1, 0) || e.snow[0].to != cal.At(2, 0) {
		t.Fatalf("snow spells: %+v, want the one of place b on day 1", e.snow)
	}
}

// The delivery of a car that may be driven through forecast snow gets the longer buffer:
// driving there takes a quarter longer on the median and up to 1.45 times as long.
func TestSnowLengthensTheBuffer(t *testing.T) {
	tests := []struct {
		name     string
		from, to int
		places   []string
		deadline int
		want     float64
	}{
		{"snow at the delivery before the deadline", cal.At(1, 0), cal.At(2, 0), []string{"b"}, cal.At(1, 20*cal.Hour), 0.5},
		{"snow at the pickup before the deadline", cal.At(1, 0), cal.At(2, 0), []string{"a"}, cal.At(1, 20*cal.Hour), 0.5},
		{"snow elsewhere", cal.At(1, 0), cal.At(2, 0), []string{"c"}, cal.At(1, 20*cal.Hour), 0.2},
		{"snow that starts after the deadline", cal.At(2, 0), cal.At(3, 0), []string{"b"}, cal.At(1, 20*cal.Hour), 0.2},
		{"snow that is over", cal.At(0, 0), cal.At(0, 9*cal.Hour), []string{"b"}, cal.At(1, 20*cal.Hour), 0.2},
		{"no forecast", 0, 0, nil, cal.At(1, 20*cal.Hour), 0.2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := snowEngine(t)
			if tc.places != nil {
				e.handle(forecast(tc.from, tc.to, tc.places...))
			}
			ob := chainOrder(e, "R1", "a", "b", e.now, tc.deadline)
			drive := e.Map.Minutes("a", "b")
			if got, want := e.buffer(ob, []int{0}), int(tc.want*float64(drive)); got != want {
				t.Errorf("buffer %d minutes, want %d (%.0f%% of the %d minute drive)", got, want, 100*tc.want, drive)
			}
		})
	}
}

// Without the switch a forecast changes nothing.
func TestSnowBufferIsASwitch(t *testing.T) {
	e := snowEngine(t)
	e.P.SnowBuffer = 0
	e.handle(forecast(cal.At(1, 0), cal.At(2, 0), "b"))
	ob := chainOrder(e, "R1", "a", "b", e.now, cal.At(1, 20*cal.Hour))
	if got, want := e.buffer(ob, []int{0}), int(0.2*float64(e.Map.Minutes("a", "b"))); got != want {
		t.Errorf("buffer %d minutes, want %d", got, want)
	}
}

// A request through forecast snow counts as riskier by SnowRisk, so fewer of the tight
// ones are quoted; the factor is off at 0 and never lifts a risk above 1.
func TestSnowRaisesTheRiskOfARequest(t *testing.T) {
	e := snowEngine(t)
	req := testRequest("R1", e.now)
	opt := req.Options[0]
	dry := e.lateRisk(req, opt)
	e.handle(forecast(cal.At(0, 12*cal.Hour), cal.At(1, 12*cal.Hour), "b"))
	if got := e.lateRisk(req, opt); got != dry {
		t.Fatalf("with snowRisk off the risk moved: %.4f -> %.4f", dry, got)
	}
	e.P.SnowRisk = 2
	if got := e.lateRisk(req, opt); dry <= 0 || got < 1.99*dry || got > 2.01*dry {
		t.Errorf("snow risk %.4f, want twice the dry %.4f", got, dry)
	}
	e.P.SnowRisk = 1000
	if got := e.lateRisk(req, opt); got != 1 {
		t.Errorf("risk %.4f, want it capped at 1", got)
	}
}
