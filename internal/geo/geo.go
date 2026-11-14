// Package geo indexes the published map: road minutes/km between places and straight-line distance.
package geo

import (
	"fmt"
	"math"

	"progresful-transfers/internal/api"
)

const earthRadiusKm = 6371.0

// Map is an indexed road matrix.
type Map struct {
	places  []api.Place
	index   map[string]int
	minutes [][]int
	km      [][]float64
	air     [][]float64
}

// New builds the index and checks that the matrix is complete.
func New(m *api.WorldMap) (*Map, error) {
	n := len(m.Places)
	if n == 0 {
		return nil, fmt.Errorf("map has no places")
	}
	g := &Map{places: m.Places, index: make(map[string]int, n)}
	for i, p := range m.Places {
		g.index[p.ID] = i
	}
	g.minutes, g.km, g.air = make([][]int, n), make([][]float64, n), make([][]float64, n)
	for i := range n {
		g.minutes[i], g.km[i], g.air[i] = make([]int, n), make([]float64, n), make([]float64, n)
		for j := range n {
			g.minutes[i][j] = -1
			g.air[i][j] = haversine(m.Places[i], m.Places[j])
		}
	}
	for _, r := range m.Roads {
		i, okFrom := g.index[r.From]
		j, okTo := g.index[r.To]
		if !okFrom || !okTo {
			return nil, fmt.Errorf("road %s->%s names an unknown place", r.From, r.To)
		}
		g.minutes[i][j], g.km[i][j] = r.Minutes, r.Km
	}
	for i := range n {
		for j := range n {
			if g.minutes[i][j] < 0 {
				return nil, fmt.Errorf("map has no road %s->%s", m.Places[i].ID, m.Places[j].ID)
			}
		}
	}
	return g, nil
}

// Places lists the places in map order.
func (g *Map) Places() []api.Place { return g.places }

// Has reports whether the place id is on the map.
func (g *Map) Has(id string) bool { _, ok := g.index[id]; return ok }

// Minutes is the road time between two places; a large value for an unknown place.
func (g *Map) Minutes(from, to string) int {
	i, okFrom := g.index[from]
	j, okTo := g.index[to]
	if !okFrom || !okTo {
		return math.MaxInt32
	}
	return g.minutes[i][j]
}

// Km is the road distance between two places.
func (g *Map) Km(from, to string) float64 {
	i, okFrom := g.index[from]
	j, okTo := g.index[to]
	if !okFrom || !okTo {
		return math.MaxFloat32
	}
	return g.km[i][j]
}

// AirKm is the straight-line distance between two places (used by the 30 km notification rule).
func (g *Map) AirKm(from, to string) float64 {
	i, okFrom := g.index[from]
	j, okTo := g.index[to]
	if !okFrom || !okTo {
		return math.MaxFloat32
	}
	return g.air[i][j]
}

func haversine(a, b api.Place) float64 {
	rad := math.Pi / 180
	dLat, dLon := (b.Lat-a.Lat)*rad, (b.Lon-a.Lon)*rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(h)))
}
