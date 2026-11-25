// Package geo holds the live, in-Redis geography of Sarathi:
// where every available driver is, and which fuel / EV station each car is at.
package geo

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Vehicle classes used for dispatch. Passengers booking "pick up and return"
// can only choose seats (5 or 7), premium/vintage/self-drive choose a model,
// so one GEO set per class keeps every search small.
var VehicleClasses = []string{"hatch5", "sedan5", "suv7", "premium", "vintage", "testdrive"}

const (
	DriverStaleAfter = 60 * time.Second  // no GPS for 60s -> not offered trips
	DriverStatusTTL  = 120 * time.Second // availability expires if the app goes silent
)

// Redis keys
//
//	geo:drivers:{class}           GEO set  member=driverID          (GEOADD / GEOSEARCH)
//	geo:drivers:{class}:seen      ZSET     member=driverID score=unix  (for pruning)
//	driver:{id}:status            STRING   idle | busy               (TTL refreshed by GPS)
//	driver:{id}:class             STRING   vehicle class (so dispatch can find the GEO set)
func driversKey(class string) string     { return "geo:drivers:" + class }
func driversSeenKey(class string) string { return "geo:drivers:" + class + ":seen" }
func DriverStatusKey(id string) string   { return "driver:" + id + ":status" }

type DriverIndex struct {
	rdb redis.UniversalClient
	log *slog.Logger
}

func NewDriverIndex(rdb redis.UniversalClient, log *slog.Logger) *DriverIndex {
	return &DriverIndex{rdb: rdb, log: log.With("component", "driver-index")}
}

// Update is called for every driver GPS ping (about once a second).
// One pipelined round-trip: position + last-seen + keep availability alive.
func (d *DriverIndex) Update(ctx context.Context, class, driverID string, lat, lng float64) error {
	now := time.Now().Unix()
	pipe := d.rdb.Pipeline()
	pipe.GeoAdd(ctx, driversKey(class), &redis.GeoLocation{Name: driverID, Latitude: lat, Longitude: lng})
	pipe.ZAdd(ctx, driversSeenKey(class), redis.Z{Score: float64(now), Member: driverID})
	pipe.Expire(ctx, DriverStatusKey(driverID), DriverStatusTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// Nearby returns up to limit drivers of a class within radiusKm, closest first.
func (d *DriverIndex) Nearby(ctx context.Context, class string, lat, lng, radiusKm float64, limit int) ([]redis.GeoLocation, error) {
	return d.rdb.GeoSearchLocation(ctx, driversKey(class), &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Latitude:   lat,
			Longitude:  lng,
			Radius:     radiusKm,
			RadiusUnit: "km",
			Sort:       "ASC",
			Count:      limit,
		},
		WithDist:  true,
		WithCoord: true,
	}).Result()
}

// SetAvailability: online -> "idle" (but never overwrite "busy"); offline ->
// delete only if idle, so a driver cannot drop a passenger mid-trip by
// toggling the switch.
var goOfflineScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == 'idle' then
  return redis.call('DEL', KEYS[1])
end
return 0`)

func (d *DriverIndex) SetAvailability(ctx context.Context, driverID string, online bool) error {
	if online {
		return d.rdb.SetNX(ctx, DriverStatusKey(driverID), "idle", DriverStatusTTL).Err()
	}
	return goOfflineScript.Run(ctx, d.rdb, []string{DriverStatusKey(driverID)}).Err()
}

// Remove takes a driver off the map immediately (e.g. on logout).
func (d *DriverIndex) Remove(ctx context.Context, class, driverID string) error {
	pipe := d.rdb.Pipeline()
	pipe.ZRem(ctx, driversKey(class), driverID) // a GEO set IS a sorted set
	pipe.ZRem(ctx, driversSeenKey(class), driverID)
	_, err := pipe.Exec(ctx)
	return err
}

// RunJanitor removes drivers whose last GPS ping is older than DriverStaleAfter.
// GEO members never expire on their own, so without this a phone that lost
// signal would keep appearing on the passenger map. Safe to run on every
// gateway: ZREM of an already-removed member is a no-op.
func (d *DriverIndex) RunJanitor(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			cutoff := strconv.FormatInt(time.Now().Add(-DriverStaleAfter).Unix(), 10)
			for _, class := range VehicleClasses {
				stale, err := d.rdb.ZRangeByScore(ctx, driversSeenKey(class), &redis.ZRangeBy{Min: "-inf", Max: cutoff, Count: 1000}).Result()
				if err != nil || len(stale) == 0 {
					continue
				}
				members := make([]any, len(stale))
				for i, s := range stale {
					members[i] = s
				}
				pipe := d.rdb.Pipeline()
				pipe.ZRem(ctx, driversKey(class), members...)
				pipe.ZRem(ctx, driversSeenKey(class), members...)
				if _, err := pipe.Exec(ctx); err != nil {
					d.log.Warn("prune stale drivers", "class", class, "err", err)
				}
			}
		}
	}
}
