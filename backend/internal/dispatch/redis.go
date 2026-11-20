// Package dispatch matches ride requests to nearby available drivers and runs
// the offer → accept/reject/timeout loop on Redis.
package dispatch

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
)

// Store holds the dispatch state of every booking being matched.
//
//	sr:dispatch:offer:<booking>     HASH  req(json), driver, token, expires_ms, attempts, state (10 min)
//	sr:dispatch:rejected:<booking>  SET   drivers who rejected / timed out (10 min)
//	sr:dispatch:deadlines           ZSET  booking -> offer expiry ms (worker scans it)
//	sr:lock:driver:<id>             lock  a driver is offered one ride at a time
type Store struct{ rdb *redis.Client }

func NewStore(rdb *redis.Client) *Store { return &Store{rdb: rdb} }

const dispatchTTL = 10 * time.Minute

func (s *Store) SaveRequest(ctx context.Context, req Request) error {
	b, _ := json.Marshal(req)
	k := cache.KeyOffer(req.BookingID)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, k, "req", b, "state", "searching", "attempts", 0)
	pipe.Expire(ctx, k, dispatchTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Request(ctx context.Context, bookingID string) (*Request, map[string]string, error) {
	m, err := s.rdb.HGetAll(ctx, cache.KeyOffer(bookingID)).Result()
	if err != nil || len(m) == 0 {
		return nil, nil, err
	}
	var req Request
	if err := json.Unmarshal([]byte(m["req"]), &req); err != nil {
		return nil, nil, err
	}
	return &req, m, nil
}

// SetOffer records the driver currently offered and its deadline.
func (s *Store) SetOffer(ctx context.Context, bookingID, driverID, token string, expires time.Time) error {
	k := cache.KeyOffer(bookingID)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, k, "driver", driverID, "token", token, "expires_ms", expires.UnixMilli(), "state", "offered")
	pipe.HIncrBy(ctx, k, "attempts", 1)
	pipe.ZAdd(ctx, cache.KeyOfferDeadlines(), redis.Z{Score: float64(expires.UnixMilli()), Member: bookingID})
	_, err := pipe.Exec(ctx)
	return err
}

// acceptScript: accept only if this driver holds the live offer.
// KEYS[1] offer hash, KEYS[2] deadlines zset; ARGV[1] driver, ARGV[2] now ms, ARGV[3] booking id
var acceptScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'state') ~= 'offered' then return 0 end
if redis.call('HGET', KEYS[1], 'driver') ~= ARGV[1] then return 0 end
if tonumber(redis.call('HGET', KEYS[1], 'expires_ms')) < tonumber(ARGV[2]) then return -1 end
redis.call('HSET', KEYS[1], 'state', 'accepted')
redis.call('ZREM', KEYS[2], ARGV[3])
return 1`)

// Accept returns 1 accepted, 0 not your offer, -1 expired.
func (s *Store) Accept(ctx context.Context, bookingID, driverID string) (int64, error) {
	return acceptScript.Run(ctx, s.rdb, []string{cache.KeyOffer(bookingID), cache.KeyOfferDeadlines()},
		driverID, time.Now().UnixMilli(), bookingID).Int64()
}

// ClearOffer marks the current driver as rejected and resets the offer.
func (s *Store) ClearOffer(ctx context.Context, bookingID, driverID string) error {
	k, rej := cache.KeyOffer(bookingID), cache.KeyOfferRejected(bookingID)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, k, "state", "searching", "driver", "", "token", "")
	pipe.SAdd(ctx, rej, driverID)
	pipe.Expire(ctx, rej, dispatchTTL)
	pipe.ZRem(ctx, cache.KeyOfferDeadlines(), bookingID)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Rejected(ctx context.Context, bookingID string) (map[string]bool, error) {
	ids, err := s.rdb.SMembers(ctx, cache.KeyOfferRejected(bookingID)).Result()
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, err
}

// Due returns bookings whose offer has expired (claimed with ZREM so one worker handles each).
func (s *Store) Due(ctx context.Context, now time.Time) ([]string, error) {
	ids, err := s.rdb.ZRangeByScore(ctx, cache.KeyOfferDeadlines(), &redis.ZRangeBy{
		Min: "-inf", Max: strconv.FormatInt(now.UnixMilli(), 10), Count: 200,
	}).Result()
	if err != nil {
		return nil, err
	}
	claimed := ids[:0]
	for _, id := range ids {
		if n, _ := s.rdb.ZRem(ctx, cache.KeyOfferDeadlines(), id).Result(); n == 1 {
			claimed = append(claimed, id)
		}
	}
	return claimed, nil
}

func (s *Store) Delete(ctx context.Context, bookingID string) error {
	pipe := s.rdb.Pipeline()
	pipe.Del(ctx, cache.KeyOffer(bookingID), cache.KeyOfferRejected(bookingID))
	pipe.ZRem(ctx, cache.KeyOfferDeadlines(), bookingID)
	_, err := pipe.Exec(ctx)
	return err
}
