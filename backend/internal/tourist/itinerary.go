package tourist

import (
	"context"
	"encoding/json"

	"sarathi/internal/common/utils"
)

// Stop is a tourist place on the trip.
type Stop struct {
	Name     string      `json:"name"`
	Location utils.Point `json:"location"`
}

// DayPlan is one day of the itinerary.
type DayPlan struct {
	Day   int     `json:"day"`
	Stops []Stop  `json:"stops"`
	Km    float64 `json:"km"`
}

// Itinerary is the full trip plan.
type Itinerary struct {
	Destination string    `json:"destination"`
	Days        []DayPlan `json:"days"`
	TotalKm     float64   `json:"total_km"`
}

// Plan spreads stops over days and estimates road km (crow-fly × 1.3), including the way back.
func Plan(origin utils.Point, dest string, stops []Stop, days int) Itinerary {
	if days < 1 {
		days = 1
	}
	it := Itinerary{Destination: dest}
	perDay := (len(stops) + days - 1) / days
	prev := origin
	for d := 0; d < days; d++ {
		dp := DayPlan{Day: d + 1}
		for i := d * perDay; i < min((d+1)*perDay, len(stops)); i++ {
			dp.Km += utils.HaversineKm(prev, stops[i].Location) * 1.3
			dp.Stops = append(dp.Stops, stops[i])
			prev = stops[i].Location
		}
		if d == days-1 {
			dp.Km += utils.HaversineKm(prev, origin) * 1.3
		}
		dp.Km = utils.Round(dp.Km, 1)
		it.TotalKm += dp.Km
		it.Days = append(it.Days, dp)
	}
	it.TotalKm = utils.Round(it.TotalKm, 1)
	return it
}

// PopularStops returns the cached itinerary template for a destination (e.g. "jaipur").
func (c *Cache) PopularStops(ctx context.Context, dest string) ([]Stop, error) {
	raw, err := c.Itinerary(ctx, utils.NormalizeName(dest))
	if err != nil || raw == nil {
		return nil, err
	}
	var stops []Stop
	return stops, json.Unmarshal(raw, &stops)
}
