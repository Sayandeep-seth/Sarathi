package transit_pass

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
)

// Store:
//
//	sr:transit:routes:<user>       HASH  route name -> json (saved daily routes, no expiry)
//	sr:transit:journey:<id>        HASH  data(json), status, qr_nonce, leg:<i> -> used unix (24 h)
//	sr:transit:user:<user>         ZSET  journey ids by creation time
//	sr:transit:qr:<nonce>          STRING journey id (valid until journey expiry)
//	sr:transit:scan:<journey>:<i>  STRING anti-replay marker for a leg scan
type Store struct{ rdb *redis.Client }

func NewStore(rdb *redis.Client) *Store { return &Store{rdb: rdb} }

func (s *Store) SaveRoute(ctx context.Context, userID string, r SavedRoute) error {
	b, _ := json.Marshal(r)
	return s.rdb.HSet(ctx, cache.KeyTransitRoutes(userID), r.Name, b).Err()
}

func (s *Store) DeleteRoute(ctx context.Context, userID, name string) error {
	return s.rdb.HDel(ctx, cache.KeyTransitRoutes(userID), name).Err()
}

func (s *Store) Routes(ctx context.Context, userID string) ([]SavedRoute, error) {
	m, err := s.rdb.HGetAll(ctx, cache.KeyTransitRoutes(userID)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]SavedRoute, 0, len(m))
	for _, v := range m {
		var r SavedRoute
		if json.Unmarshal([]byte(v), &r) == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Store) Route(ctx context.Context, userID, name string) (*SavedRoute, error) {
	v, err := s.rdb.HGet(ctx, cache.KeyTransitRoutes(userID), name).Result()
	if cache.IsNil(err) {
		return nil, apperr.NotFound("saved route not found: " + name)
	}
	if err != nil {
		return nil, err
	}
	var r SavedRoute
	return &r, json.Unmarshal([]byte(v), &r)
}

func (s *Store) SaveJourney(ctx context.Context, j *Journey) error {
	b, _ := json.Marshal(j)
	k := cache.KeyTransitJourney(j.ID)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, k, "data", b, "status", j.Status, "legs", len(j.Legs), "user_id", j.UserID)
	pipe.Expire(ctx, k, cache.TTLTransitJourney)
	pipe.ZAdd(ctx, cache.KeyTransitUserJourneys(j.UserID), redis.Z{Score: float64(j.CreatedAt.Unix()), Member: j.ID})
	_, err := pipe.Exec(ctx)
	return err
}

// Journey loads the journey and overlays live leg usage + status.
func (s *Store) Journey(ctx context.Context, id string) (*Journey, map[string]string, error) {
	m, err := s.rdb.HGetAll(ctx, cache.KeyTransitJourney(id)).Result()
	if err != nil {
		return nil, nil, err
	}
	if len(m) == 0 {
		return nil, nil, apperr.NotFound("journey not found or expired")
	}
	var j Journey
	if err := json.Unmarshal([]byte(m["data"]), &j); err != nil {
		return nil, nil, err
	}
	j.Status = m["status"]
	for i := range j.Legs {
		if ts, ok := m["leg:"+strconv.Itoa(i)]; ok {
			unix, _ := strconv.ParseInt(ts, 10, 64)
			t := time.Unix(unix, 0)
			j.Legs[i].Used, j.Legs[i].UsedAt = true, &t
		}
	}
	return &j, m, nil
}

func (s *Store) SetStatus(ctx context.Context, id, status string) error {
	return s.rdb.HSet(ctx, cache.KeyTransitJourney(id), "status", status).Err()
}

// RotateQR stores a new QR nonce: old QR codes (screenshots) stop working.
func (s *Store) RotateQR(ctx context.Context, journeyID, nonce string, ttl time.Duration) error {
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, cache.KeyTransitJourney(journeyID), "qr_nonce", nonce)
	pipe.Set(ctx, cache.KeyTransitQR(nonce), journeyID, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) JourneyByNonce(ctx context.Context, nonce string) (string, error) {
	id, err := s.rdb.Get(ctx, cache.KeyTransitQR(nonce)).Result()
	if cache.IsNil(err) {
		return "", apperr.Forbidden("QR expired")
	}
	return id, err
}

// useLegScript marks a leg used exactly once and completes the journey after the last leg.
// KEYS[1] journey hash; ARGV[1] leg index; ARGV[2] now unix; ARGV[3] expected qr nonce
// returns 1 ok, 0 already used, -1 not active, -2 stale QR
var useLegScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'status') ~= 'active' then return -1 end
if redis.call('HGET', KEYS[1], 'qr_nonce') ~= ARGV[3] then return -2 end
local f = 'leg:' .. ARGV[1]
if redis.call('HSETNX', KEYS[1], f, ARGV[2]) == 0 then return 0 end
local total = tonumber(redis.call('HGET', KEYS[1], 'legs'))
local used = 0
for i = 0, total - 1 do
  if redis.call('HEXISTS', KEYS[1], 'leg:' .. i) == 1 then used = used + 1 end
end
if used == total then redis.call('HSET', KEYS[1], 'status', 'completed') end
return 1`)

func (s *Store) UseLeg(ctx context.Context, journeyID string, leg int, nonce string) (int64, error) {
	return useLegScript.Run(ctx, s.rdb, []string{cache.KeyTransitJourney(journeyID)}, leg, time.Now().Unix(), nonce).Int64()
}
