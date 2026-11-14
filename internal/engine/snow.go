package engine

import (
	"math"

	"progresful-transfers/internal/api"
)

// snowSpell is one forecast: snow in some places between two minutes of the world. A day
// ahead the server says where it will snow; driving there then takes longer (a quarter
// longer on the median, up to 1.45 times the map's minutes) and some drivers stay at home.
type snowSpell struct {
	from, to int
	places   map[string]bool
}

// onForecast remembers a forecast. It is also read from the history when a world is joined.
func (e *Engine) onForecast(ev *api.Event) {
	if ev.To <= ev.From || len(ev.PlaceIDs) == 0 {
		return
	}
	places := make(map[string]bool, len(ev.PlaceIDs))
	for _, id := range ev.PlaceIDs {
		places[id] = true
	}
	e.snow = append(e.snow, snowSpell{from: ev.From, to: ev.To, places: places})
}

// snowOn reports whether forecast snow lies on a car's way: at its pickup or delivery
// place, at some time between minute from and the deadline.
func (e *Engine) snowOn(leg *api.Leg, from, deadline int) bool {
	for _, s := range e.snow {
		if s.from < deadline && from < s.to && (s.places[leg.Pickup.PlaceID] || s.places[leg.Delivery.PlaceID]) {
			return true
		}
	}
	return false
}

// bufferShare is the share of a car's drive its delivery should arrive before the
// deadline: BufferShare, or SnowBuffer when the drive may run through forecast snow.
func (e *Engine) bufferShare(leg *api.Leg, deadline int) float64 {
	if e.P.SnowBuffer > e.P.BufferShare && e.snowOn(leg, e.now, deadline) {
		return e.P.SnowBuffer
	}
	return e.P.BufferShare
}

// bufferOf is the buffer of one car in minutes.
func (e *Engine) bufferOf(leg *api.Leg, deadline int) int {
	return max(e.P.BufferMin, int(e.bufferShare(leg, deadline)*float64(e.driveMin(leg))))
}

// snowRisk is the modelled chance of arriving late, raised by SnowRisk for a car whose
// way lies through forecast snow (0 or 1: no change).
func (e *Engine) snowRisk(risk float64, leg *api.Leg, from, deadline int) float64 {
	if e.P.SnowRisk <= 0 || e.P.SnowRisk == 1 || !e.snowOn(leg, from, deadline) {
		return risk
	}
	return math.Min(1, risk*e.P.SnowRisk)
}
