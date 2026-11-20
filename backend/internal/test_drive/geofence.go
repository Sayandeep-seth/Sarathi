package test_drive

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
	"sarathi/internal/dispatch"
	"sarathi/internal/websocket"
)

// Geofence keeps demo cars inside the showroom's area.
//   - as dispatch.CandidateFilter: a demo car is offered a cab ride only if both
//     pickup and drop are inside its zone;
//   - as gps.ZoneChecker: when a demo car leaves its zone the dealer gets an alert
//     (de-duplicated for 5 min with SET NX sr:td:alert:<vehicle>).
type Geofence struct {
	rdb   *redis.Client
	cache *Cache
	pub   *websocket.Publisher
}

func NewGeofence(rdb *redis.Client, c *Cache, pub *websocket.Publisher) *Geofence {
	return &Geofence{rdb: rdb, cache: c, pub: pub}
}

func inside(z *Zone, p utils.Point) bool {
	return z != nil && utils.HaversineKm(z.Center, p) <= z.RadiusKm
}

// InZone reports whether p is inside the showroom's area.
func (g *Geofence) InZone(ctx context.Context, showroomID string, p utils.Point) bool {
	z, _ := g.cache.Zone(ctx, showroomID)
	return inside(z, p)
}

// Allow implements dispatch.CandidateFilter for class "testdrive".
func (g *Geofence) Allow(ctx context.Context, driverID string, req dispatch.Request) bool {
	vehicleID, err := g.rdb.HGet(ctx, cache.KeyDriver(driverID), "vehicle_id").Result()
	if err != nil {
		return false
	}
	car, err := g.cache.Car(ctx, vehicleID)
	if err != nil || car == nil {
		return false
	}
	z, _ := g.cache.Zone(ctx, car.ShowroomID)
	return inside(z, req.Pickup) && inside(z, req.Drop)
}

// CheckVehicle implements gps.ZoneChecker.
func (g *Geofence) CheckVehicle(ctx context.Context, vehicleID string, p utils.Point) error {
	car, err := g.cache.Car(ctx, vehicleID)
	if err != nil || car == nil {
		return err // not a demo car
	}
	z, err := g.cache.Zone(ctx, car.ShowroomID)
	if err != nil || z == nil || inside(z, p) {
		return err
	}
	first, err := g.rdb.SetNX(ctx, cache.KeyTDZoneAlert(vehicleID), "1", cache.TTLTDZoneAlert).Result()
	if err != nil || !first {
		return err
	}
	return g.pub.Dealer(ctx, car.DealerID, websocket.EventZoneAlert, map[string]any{
		"vehicle_id": vehicleID, "showroom_id": car.ShowroomID, "location": p,
		"distance_km": utils.Round(utils.HaversineKm(z.Center, p), 2), "at": time.Now(),
	})
}
