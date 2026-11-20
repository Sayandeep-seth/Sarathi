package gps

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// RedisTracking stores live positions.
//
//	sr:geo:vehicles:live   GEO     every moving vehicle (station crowd, nearby queries)
//	sr:gps:lastseen        ZSET    vehicle -> unix (stale cleanup)
//	sr:gps:loc:<vehicle>   HASH    lat,lng,speed,heading,trip,driver,at (2 min)
//	sr:gps:track:<trip>    STREAM  full trip path (48 h, ~5000 points)
//	sr:ch:trip:<trip>:loc  PUBSUB  live position to rider app via websocket
//	sr:stream:gps:persist  STREAM  async batch insert into Postgres
type RedisTracking struct{ rdb *redis.Client }

func NewRedisTracking(rdb *redis.Client) *RedisTracking { return &RedisTracking{rdb: rdb} }

func (t *RedisTracking) Store(ctx context.Context, p Ping) error {
	payload, _ := json.Marshal(p)
	pipe := t.rdb.Pipeline()
	cache.AddPipe(ctx, pipe, cache.KeyVehiclesLiveGeo(), p.VehicleID, p.Location)
	pipe.ZAdd(ctx, cache.KeyVehiclesLastSeen(), redis.Z{Score: float64(p.At.Unix()), Member: p.VehicleID})
	locKey := cache.KeyVehicleLoc(p.VehicleID)
	pipe.HSet(ctx, locKey, map[string]any{
		"lat": p.Location.Lat, "lng": p.Location.Lng, "speed": p.SpeedKmh, "heading": p.Heading,
		"trip": p.TripID, "driver": p.DriverID, "at": p.At.Unix(),
	})
	pipe.Expire(ctx, locKey, cache.TTLVehicleLoc)
	if p.TripID != "" {
		track := cache.KeyTripTrack(p.TripID)
		pipe.XAdd(ctx, &redis.XAddArgs{Stream: track, MaxLen: 5000, Approx: true, Values: map[string]any{"p": payload}})
		pipe.Expire(ctx, track, cache.TTLTripTrack)
		pipe.Publish(ctx, cache.ChanTripLoc(p.TripID), payload)
	}
	pipe.XAdd(ctx, &redis.XAddArgs{Stream: cache.StreamGPSPersist, MaxLen: 1_000_000, Approx: true, Values: map[string]any{"p": payload}})
	_, err := pipe.Exec(ctx)
	return err
}

// Last returns the latest known position of a vehicle.
func (t *RedisTracking) Last(ctx context.Context, vehicleID string) (*Ping, error) {
	m, err := t.rdb.HGetAll(ctx, cache.KeyVehicleLoc(vehicleID)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, apperr.NotFound("no recent location for vehicle")
	}
	return &Ping{
		VehicleID: vehicleID, DriverID: m["driver"], TripID: m["trip"],
		Location: utils.Point{Lat: cache.HGetFloat(m, "lat"), Lng: cache.HGetFloat(m, "lng")},
		SpeedKmh: cache.HGetFloat(m, "speed"), Heading: cache.HGetFloat(m, "heading"),
		At: time.Unix(cache.HGetInt(m, "at"), 0),
	}, nil
}

// TripPath returns the recorded path of a trip (for the trip summary / disputes).
func (t *RedisTracking) TripPath(ctx context.Context, tripID string, limit int64) ([]Ping, error) {
	msgs, err := t.rdb.XRangeN(ctx, cache.KeyTripTrack(tripID), "-", "+", limit).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Ping, 0, len(msgs))
	for _, m := range msgs {
		var p Ping
		if s, ok := m.Values["p"].(string); ok && json.Unmarshal([]byte(s), &p) == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// RemoveStale drops vehicles that stopped pinging from the live GEO set.
func (t *RedisTracking) RemoveStale(ctx context.Context, olderThan time.Duration) (int, error) {
	max := strconv.FormatInt(time.Now().Add(-olderThan).Unix(), 10)
	stale, err := t.rdb.ZRangeByScore(ctx, cache.KeyVehiclesLastSeen(), &redis.ZRangeBy{Min: "-inf", Max: max, Count: 5000}).Result()
	if err != nil || len(stale) == 0 {
		return 0, err
	}
	members := make([]any, len(stale))
	for i, s := range stale {
		members[i] = s
	}
	pipe := t.rdb.Pipeline()
	pipe.ZRem(ctx, cache.KeyVehiclesLiveGeo(), members...)
	pipe.ZRem(ctx, cache.KeyVehiclesLastSeen(), members...)
	_, err = pipe.Exec(ctx)
	return len(stale), err
}
