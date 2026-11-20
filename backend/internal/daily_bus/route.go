// Package daily_bus runs premium office buses for daily passengers: everyone gets a
// reserved seat (no standing), and the bus drops passengers in front of their
// workplace where the route allows. Cheaper than a cab, as comfortable.
package daily_bus

import (
	"math"

	"sarathi/internal/common/utils"
)

// Stop is a bus stop; office-front stops are added where the bus can reach them.
type Stop struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Location     utils.Point `json:"location"`
	Seq          int         `json:"seq"`
	ETAOffsetMin int         `json:"eta_offset_min"` // minutes after departure
	OfficeFront  bool        `json:"office_front"`   // drop exactly at a workplace gate
}

// Route is a fixed daily route.
type Route struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	City       string  `json:"city"`
	DepartTime string  `json:"depart_time"` // "08:15"
	BusSeats   int     `json:"bus_seats"`
	FarePerKm  float64 `json:"fare_per_km"`
	MinFare    float64 `json:"min_fare"`
	Stops      []Stop  `json:"stops"`
}

func (r *Route) Stop(id string) (Stop, bool) {
	for _, s := range r.Stops {
		if s.ID == id {
			return s, true
		}
	}
	return Stop{}, false
}

// KmBetween sums stop-to-stop distance (× 1.25 road factor).
func (r *Route) KmBetween(from, to Stop) float64 {
	km := 0.0
	var prev *Stop
	for i := range r.Stops {
		s := r.Stops[i]
		if s.Seq < from.Seq || s.Seq > to.Seq {
			continue
		}
		if prev != nil {
			km += utils.HaversineKm(prev.Location, s.Location)
		}
		prev = &s
	}
	return km * 1.25
}

// Fare for one ride; typically ~40 % of a cab for the same distance.
func (r *Route) Fare(from, to Stop) float64 {
	return math.Max(r.MinFare, math.Round(r.KmBetween(from, to)*r.FarePerKm))
}
