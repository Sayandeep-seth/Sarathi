// Package geo models the service area as a stylised road grid.
//
// The grid is NOT real street data: it is a regular lattice anchored at a real
// coordinate so that distances, fares and ETAs are internally consistent. Swapping
// in a real router (e.g. OSRM) means replacing Route and Places.
package geo

import (
	"math"
)

const (
	// OriginLat and OriginLng anchor the grid centre (Bengaluru).
	OriginLat = 12.9716
	OriginLng = 77.5946
	// Size is the number of intersections per side; Spacing is metres between them.
	Size    = 11
	Spacing = 450.0

	metresPerDegLat = 111_320.0
	// CruiseSpeed is the planning speed in metres/second (~29 km/h urban average).
	CruiseSpeed = 8.0
)

// Point is a position in metres east (X) and north (Y) of the origin.
type Point struct{ X, Y float64 }

// Node is a grid intersection, 0 <= I,J < Size.
type Node struct{ I, J int }

// Place is a named pickup/drop location at an intersection.
type Place struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Kind string  `json:"kind"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Node Node    `json:"-"`
}

func mk(id, name, kind string, i, j int) Place {
	n := Node{i, j}
	p := n.Point()
	lat, lng := ToLatLng(p)
	return Place{ID: id, Name: name, Kind: kind, Lat: lat, Lng: lng, X: p.X, Y: p.Y, Node: n}
}

// Places lists the bookable locations. Positions are stylised, not surveyed.
var Places = []Place{
	mk("mg-road", "MG Road", "commercial", 5, 5),
	mk("cubbon-park", "Cubbon Park", "park", 4, 5),
	mk("indiranagar", "Indiranagar", "commercial", 8, 6),
	mk("koramangala", "Koramangala", "commercial", 7, 2),
	mk("majestic", "Majestic Station", "transit", 2, 6),
	mk("malleshwaram", "Malleshwaram", "residential", 2, 9),
	mk("jayanagar", "Jayanagar", "residential", 4, 1),
	mk("hsr-layout", "HSR Layout", "residential", 9, 0),
	mk("whitefield-gate", "Whitefield Gate", "tech", 10, 7),
	mk("hebbal", "Hebbal", "transit", 5, 10),
	mk("ulsoor-lake", "Ulsoor Lake", "park", 7, 7),
	mk("lalbagh", "Lalbagh", "park", 4, 3),
	mk("airport-road", "Old Airport Road", "commercial", 9, 4),
	mk("rajajinagar", "Rajajinagar", "residential", 0, 8),
	mk("banashankari", "Banashankari", "residential", 1, 0),
	mk("tech-park", "Tech Park", "tech", 10, 2),
}

// PlaceByID returns the place with the given id.
func PlaceByID(id string) (Place, bool) {
	for _, p := range Places {
		if p.ID == id {
			return p, true
		}
	}
	return Place{}, false
}

// Point returns the node's position in metres.
func (n Node) Point() Point {
	half := float64(Size-1) / 2
	return Point{X: (float64(n.I) - half) * Spacing, Y: (float64(n.J) - half) * Spacing}
}

// Valid reports whether the node lies on the grid.
func (n Node) Valid() bool { return n.I >= 0 && n.I < Size && n.J >= 0 && n.J < Size }

// ToLatLng converts local metres to WGS84 degrees.
func ToLatLng(p Point) (lat, lng float64) {
	lat = OriginLat + p.Y/metresPerDegLat
	lng = OriginLng + p.X/(metresPerDegLat*math.Cos(OriginLat*math.Pi/180))
	return lat, lng
}

// FromLatLng converts WGS84 degrees to local metres.
func FromLatLng(lat, lng float64) Point {
	return Point{
		X: (lng - OriginLng) * metresPerDegLat * math.Cos(OriginLat*math.Pi/180),
		Y: (lat - OriginLat) * metresPerDegLat,
	}
}

// InServiceArea reports whether a coordinate is within the grid plus a small margin.
func InServiceArea(lat, lng float64) bool {
	p := FromLatLng(lat, lng)
	lim := float64(Size-1)/2*Spacing + Spacing
	return math.Abs(p.X) <= lim && math.Abs(p.Y) <= lim
}

// NearestNode snaps a point to the closest intersection.
func NearestNode(p Point) Node {
	half := float64(Size-1) / 2
	clamp := func(v float64) int {
		i := int(math.Round(v/Spacing + half))
		return max(0, min(Size-1, i))
	}
	return Node{clamp(p.X), clamp(p.Y)}
}

// Route returns the intersections from a to b inclusive, as an L-shaped path.
// The leg order alternates by parity so traffic spreads over both axes.
func Route(a, b Node) []Node {
	path := []Node{a}
	cur := a
	walkI := func() {
		for cur.I != b.I {
			cur.I += sign(b.I - cur.I)
			path = append(path, cur)
		}
	}
	walkJ := func() {
		for cur.J != b.J {
			cur.J += sign(b.J - cur.J)
			path = append(path, cur)
		}
	}
	if (a.I+a.J+b.I+b.J)%2 == 0 {
		walkI()
		walkJ()
	} else {
		walkJ()
		walkI()
	}
	return path
}

// DistanceM is the driving distance in metres between two intersections.
func DistanceM(a, b Node) int {
	return int(float64(abs(a.I-b.I)+abs(a.J-b.J)) * Spacing)
}

// Estimate is a priced trip plan.
type Estimate struct {
	DistanceM int   `json:"distance_m"`
	DurationS int   `json:"duration_s"`
	FarePaise int64 `json:"fare_paise"`
}

// Fare components, in paise.
const (
	fareBase   = 4000
	farePerKm  = 1500
	farePerMin = 150
)

// Plan prices a trip between two distinct places.
func Plan(from, to Place) Estimate {
	d := DistanceM(from.Node, to.Node)
	dur := int(math.Ceil(float64(d) / CruiseSpeed))
	fare := int64(fareBase) + int64(d)*farePerKm/1000 + int64(dur)*farePerMin/60
	return Estimate{DistanceM: d, DurationS: dur, FarePaise: fare}
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
