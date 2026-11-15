package geo

import (
	"math"
	"testing"
)

func TestRouteIsContiguousAndMatchesDistance(t *testing.T) {
	for _, a := range Places {
		for _, b := range Places {
			path := Route(a.Node, b.Node)
			if path[0] != a.Node || path[len(path)-1] != b.Node {
				t.Fatalf("%s->%s: bad endpoints", a.ID, b.ID)
			}
			for i := 1; i < len(path); i++ {
				if abs(path[i].I-path[i-1].I)+abs(path[i].J-path[i-1].J) != 1 || !path[i].Valid() {
					t.Fatalf("%s->%s: non-adjacent step at %d", a.ID, b.ID, i)
				}
			}
			if got := int(float64(len(path)-1) * Spacing); got != DistanceM(a.Node, b.Node) {
				t.Fatalf("%s->%s: path %dm, distance %dm", a.ID, b.ID, got, DistanceM(a.Node, b.Node))
			}
		}
	}
}

func TestLatLngRoundTrip(t *testing.T) {
	for _, p := range Places {
		back := FromLatLng(p.Lat, p.Lng)
		if math.Abs(back.X-p.X) > 0.01 || math.Abs(back.Y-p.Y) > 0.01 {
			t.Fatalf("%s: round trip drifted to %+v", p.ID, back)
		}
		if NearestNode(back) != p.Node || !InServiceArea(p.Lat, p.Lng) {
			t.Fatalf("%s: snapping failed", p.ID)
		}
	}
	if InServiceArea(28.6, 77.2) {
		t.Fatal("Delhi must be outside the service area")
	}
}

func TestPlaceIDsUniqueAndPlanPositive(t *testing.T) {
	seen := map[string]bool{}
	nodes := map[Node]bool{}
	for _, p := range Places {
		if seen[p.ID] || nodes[p.Node] {
			t.Fatalf("duplicate place %s", p.ID)
		}
		seen[p.ID], nodes[p.Node] = true, true
	}
	a, _ := PlaceByID("mg-road")
	b, _ := PlaceByID("hebbal")
	e := Plan(a, b)
	// 5 blocks * 450 m = 2250 m; 282 s; 4000 + 3375 + 705 paise.
	if e.DistanceM != 2250 || e.DurationS != 282 || e.FarePaise != 8080 {
		t.Fatalf("unexpected plan %+v", e)
	}
}
