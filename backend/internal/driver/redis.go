package driver

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// Store is the Redis state of drivers.
//
//	sr:driver:<id>           HASH status,classes,vehicle_id,model,seats,rating,name,lat,lng,updated
//	sr:driver:<id>:hb        STRING heartbeat (45 s)
//	sr:geo:drivers:<class>   GEO   AVAILABLE drivers only, one set per class
//	sr:drivers:lastseen      ZSET  driver -> unix (stale sweep)
type Store struct {
	rdb *redis.Client
}

func NewStore(rdb *redis.Client) *Store { return &Store{rdb: rdb} }

// setStatusScript: atomic compare-and-set of the driver status.
// KEYS[1] driver hash; ARGV[1] new status; ARGV[2..] allowed current statuses.
// Returns {1, old} on success, {0, current} otherwise.
var setStatusScript = redis.NewScript(`
local cur = redis.call('HGET', KEYS[1], 'status') or 'offline'
for i = 2, #ARGV do
  if ARGV[i] == cur then
    redis.call('HSET', KEYS[1], 'status', ARGV[1], 'updated', redis.call('TIME')[1])
    return {1, cur}
  end
end
return {0, cur}`)

// SetStatus changes status atomically and keeps GEO sets consistent.
func (s *Store) SetStatus(ctx context.Context, id, to string) error {
	args := []any{to}
	for _, f := range AllowedFrom(to) {
		args = append(args, f)
	}
	res, err := setStatusScript.Run(ctx, s.rdb, []string{cache.KeyDriver(id)}, args...).Slice()
	if err != nil {
		return err
	}
	if ok, _ := res[0].(int64); ok != 1 {
		return apperr.Conflict("driver is " + toString(res[1]) + ", cannot become " + to)
	}
	return s.syncGeo(ctx, id, to)
}

// syncGeo: available -> present in every class set; otherwise removed from all.
func (s *Store) syncGeo(ctx context.Context, id, status string) error {
	m, err := s.rdb.HMGet(ctx, cache.KeyDriver(id), "classes", "lat", "lng").Result()
	if err != nil {
		return err
	}
	classes := strings.Split(toString(m[0]), ",")
	lat, _ := strconv.ParseFloat(toString(m[1]), 64)
	lng, _ := strconv.ParseFloat(toString(m[2]), 64)
	p := utils.Point{Lat: lat, Lng: lng}
	pipe := s.rdb.Pipeline()
	for _, c := range classes {
		if c == "" {
			continue
		}
		if status == StatusAvailable && p.Valid() {
			cache.AddPipe(ctx, pipe, cache.KeyDriversGeo(c), id, p)
		} else {
			pipe.ZRem(ctx, cache.KeyDriversGeo(c), id)
		}
	}
	_, err = pipe.Exec(ctx)
	return err
}

// SaveProfile writes the static part of a driver into the hash.
func (s *Store) SaveProfile(ctx context.Context, d Driver) error {
	return s.rdb.HSet(ctx, cache.KeyDriver(d.ID), map[string]any{
		"name": d.Name, "vehicle_id": d.VehicleID, "model": d.VehicleModel, "seats": d.Seats,
		"rating": d.Rating, "classes": strings.Join(d.Classes, ","), "dealer_id": d.DealerID,
	}).Err()
}

// UpdateLocation stores the position; if available, moves the driver in its GEO sets.
func (s *Store) UpdateLocation(ctx context.Context, id string, p utils.Point) error {
	k := cache.KeyDriver(id)
	m, err := s.rdb.HMGet(ctx, k, "status", "classes").Result()
	if err != nil {
		return err
	}
	status, classes := toString(m[0]), strings.Split(toString(m[1]), ",")
	now := time.Now()
	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, k, "lat", p.Lat, "lng", p.Lng, "updated", now.Unix())
	pipe.Set(ctx, cache.KeyDriverHeartbeat(id), now.Unix(), cache.TTLDriverHeartbeat)
	pipe.ZAdd(ctx, cache.KeyDriversLastSeen(), redis.Z{Score: float64(now.Unix()), Member: id})
	if status == StatusAvailable {
		for _, c := range classes {
			if c != "" {
				cache.AddPipe(ctx, pipe, cache.KeyDriversGeo(c), id, p)
			}
		}
	}
	_, err = pipe.Exec(ctx)
	return err
}

// Get returns the driver hash.
func (s *Store) Get(ctx context.Context, id string) (map[string]string, error) {
	m, err := s.rdb.HGetAll(ctx, cache.KeyDriver(id)).Result()
	if err == nil && len(m) == 0 {
		return nil, apperr.NotFound("driver not found")
	}
	return m, err
}

// Stale returns drivers that have not pinged for olderThan.
func (s *Store) Stale(ctx context.Context, olderThan time.Duration) ([]string, error) {
	max := strconv.FormatInt(time.Now().Add(-olderThan).Unix(), 10)
	return s.rdb.ZRangeByScore(ctx, cache.KeyDriversLastSeen(), &redis.ZRangeBy{Min: "-inf", Max: max, Count: 1000}).Result()
}

// ForgetLastSeen removes drivers from the last-seen index.
func (s *Store) ForgetLastSeen(ctx context.Context, ids ...string) error {
	members := make([]any, len(ids))
	for i, id := range ids {
		members[i] = id
	}
	return s.rdb.ZRem(ctx, cache.KeyDriversLastSeen(), members...).Err()
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}
