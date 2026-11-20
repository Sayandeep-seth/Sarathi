// Package shared_ride pools riders going the same way into one cab: cheaper for
// riders, more money per trip for the driver.
package shared_ride

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// Trip is an open pool.
type Trip struct {
	ID         string      `json:"id"` // = booking id of the first rider
	Origin     utils.Point `json:"origin"`
	Dest       utils.Point `json:"dest"`
	SeatsTotal int         `json:"seats_total"`
	SeatsLeft  int         `json:"seats_left"`
	State      string      `json:"state"` // open | closed
	Riders     []string    `json:"riders"`
}

// Store:
//
//	sr:geo:shared:open            GEO  open trips at their origin (matching)
//	sr:shared:trip:<id>           HASH origin, dest, seats_total, seats_left, state (3 h)
//	sr:shared:trip:<id>:riders    SET  user ids (also authorises websocket trip topic)
type Store struct {
	rdb *redis.Client
	geo *cache.Geo
}

func NewStore(rdb *redis.Client, geo *cache.Geo) *Store { return &Store{rdb: rdb, geo: geo} }

func (s *Store) Create(ctx context.Context, t Trip, firstRider string) error {
	k, riders := cache.KeySharedTrip(t.ID), cache.KeySharedRiders(t.ID)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, k, map[string]any{
		"o_lat": t.Origin.Lat, "o_lng": t.Origin.Lng, "d_lat": t.Dest.Lat, "d_lng": t.Dest.Lng,
		"seats_total": t.SeatsTotal, "seats_left": t.SeatsLeft, "state": "open", "created": time.Now().Unix(),
	})
	pipe.SAdd(ctx, riders, firstRider)
	pipe.Expire(ctx, k, cache.TTLSharedTrip)
	pipe.Expire(ctx, riders, cache.TTLSharedTrip)
	cache.AddPipe(ctx, pipe, cache.KeySharedOpenGeo(), t.ID, t.Origin)
	_, err := pipe.Exec(ctx)
	return err
}

// joinScript atomically takes seats if the pool is open and has room.
// KEYS[1] trip hash, KEYS[2] riders set; ARGV[1] user, ARGV[2] seats
// returns seats left, or -1 closed, -2 full, -3 already joined
var joinScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'state') ~= 'open' then return -1 end
if redis.call('SISMEMBER', KEYS[2], ARGV[1]) == 1 then return -3 end
local left = tonumber(redis.call('HGET', KEYS[1], 'seats_left'))
local want = tonumber(ARGV[2])
if left < want then return -2 end
redis.call('HINCRBY', KEYS[1], 'seats_left', -want)
redis.call('SADD', KEYS[2], ARGV[1])
return left - want`)

func (s *Store) Join(ctx context.Context, tripID, userID string, seats int) (int64, error) {
	left, err := joinScript.Run(ctx, s.rdb, []string{cache.KeySharedTrip(tripID), cache.KeySharedRiders(tripID)}, userID, seats).Int64()
	if err != nil {
		return 0, err
	}
	switch left {
	case -1:
		return 0, apperr.Conflict("shared trip already closed")
	case -2:
		return 0, apperr.Conflict("not enough seats left")
	case -3:
		return 0, apperr.Conflict("already in this shared trip")
	}
	if left == 0 {
		_ = s.Close(ctx, tripID) // full: stop matching
	}
	return left, nil
}

func (s *Store) Leave(ctx context.Context, tripID, userID string, seats int) error {
	pipe := s.rdb.TxPipeline()
	pipe.SRem(ctx, cache.KeySharedRiders(tripID), userID)
	pipe.HIncrBy(ctx, cache.KeySharedTrip(tripID), "seats_left", int64(seats))
	_, err := pipe.Exec(ctx)
	return err
}

// Close stops new riders joining (trip started or full).
func (s *Store) Close(ctx context.Context, tripID string) error {
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, cache.KeySharedTrip(tripID), "state", "closed")
	pipe.ZRem(ctx, cache.KeySharedOpenGeo(), tripID)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Get(ctx context.Context, id string) (*Trip, error) {
	pipe := s.rdb.Pipeline()
	h := pipe.HGetAll(ctx, cache.KeySharedTrip(id))
	r := pipe.SMembers(ctx, cache.KeySharedRiders(id))
	if _, err := pipe.Exec(ctx); err != nil && !cache.IsNil(err) {
		return nil, err
	}
	m := h.Val()
	if len(m) == 0 {
		return nil, apperr.NotFound("shared trip not found")
	}
	return &Trip{
		ID: id, State: m["state"], Riders: r.Val(),
		Origin:     utils.Point{Lat: cache.HGetFloat(m, "o_lat"), Lng: cache.HGetFloat(m, "o_lng")},
		Dest:       utils.Point{Lat: cache.HGetFloat(m, "d_lat"), Lng: cache.HGetFloat(m, "d_lng")},
		SeatsTotal: int(cache.HGetInt(m, "seats_total")), SeatsLeft: int(cache.HGetInt(m, "seats_left")),
	}, nil
}

func (s *Store) OpenNear(ctx context.Context, p utils.Point, radiusKm float64) ([]cache.GeoHit, error) {
	return s.geo.Nearby(ctx, cache.KeySharedOpenGeo(), p, radiusKm, 20)
}
