package geo

import (
	"math"
	"testing"

	"progresful-transfers/internal/api"
)

func twoPlaces() *api.WorldMap {
	return &api.WorldMap{
		Places: []api.Place{{ID: "helsinki", Lat: 60.1699, Lon: 24.9384}, {ID: "tampere", Lat: 61.4978, Lon: 23.7610}},
		Roads: []api.Road{
			{From: "helsinki", To: "helsinki", Km: 6, Minutes: 12}, {From: "helsinki", To: "tampere", Km: 178, Minutes: 115},
			{From: "tampere", To: "helsinki", Km: 179, Minutes: 117}, {From: "tampere", To: "tampere", Km: 5, Minutes: 10},
		},
	}
}

func TestMapLookups(t *testing.T) {
	m, err := New(twoPlaces())
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Minutes("helsinki", "tampere"); got != 115 {
		t.Errorf("road minutes = %d, want 115", got)
	}
	if got := m.Minutes("tampere", "helsinki"); got != 117 {
		t.Errorf("the way back has its own minutes: %d, want 117", got)
	}
	if got := m.Km("helsinki", "tampere"); got != 178 {
		t.Errorf("road km = %v, want 178", got)
	}
	// Helsinki - Tampere is about 161 km in a straight line.
	if got := m.AirKm("helsinki", "tampere"); math.Abs(got-161) > 3 {
		t.Errorf("straight-line km = %.1f, want about 161", got)
	}
	if got := m.AirKm("helsinki", "helsinki"); got != 0 {
		t.Errorf("a place is 0 km from itself, got %v", got)
	}
	if !m.Has("tampere") || m.Has("oulu") || len(m.Places()) != 2 {
		t.Error("Has/Places do not match the map")
	}
	// An unknown place is far from everything, so no rule ever treats it as near.
	if m.Minutes("helsinki", "oulu") < 1e6 || m.Km("oulu", "helsinki") < 1e6 || m.AirKm("oulu", "helsinki") < 1e6 {
		t.Error("an unknown place must read as very far")
	}
}

func TestMapMustBeComplete(t *testing.T) {
	cases := []struct {
		name string
		edit func(*api.WorldMap)
	}{
		{"no places", func(m *api.WorldMap) { m.Places = nil }},
		{"a missing road", func(m *api.WorldMap) { m.Roads = m.Roads[:3] }},
		{"a road to an unknown place", func(m *api.WorldMap) { m.Roads[1].To = "oulu" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := twoPlaces()
			c.edit(m)
			if _, err := New(m); err == nil {
				t.Error("want an error, got none")
			}
		})
	}
}
