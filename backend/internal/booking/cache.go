package booking

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
)

// Cache is the hot copy of bookings.
//
//	sr:booking:<id>                  HASH data(json), state, user_id, driver_id, updated (48 h)
//	sr:user:<id>:active_booking      STRING booking id (one active cab per rider)
//
// "state" and "driver_id" are separate fields so transitions are atomic (Lua CAS)
// and the websocket hub can authorise trip:<id> subscribers with one HMGET.
type Cache struct{ rdb *redis.Client }

func NewCache(rdb *redis.Client) *Cache { return &Cache{rdb: rdb} }

func (c *Cache) Save(ctx context.Context, b *Booking) error {
	data, _ := json.Marshal(b)
	k := cache.KeyBooking(b.ID)
	pipe := c.rdb.TxPipeline()
	pipe.HSet(ctx, k, "data", data, "state", b.State, "user_id", b.UserID, "driver_id", b.DriverID, "updated", time.Now().Unix())
	pipe.Expire(ctx, k, cache.TTLBooking)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Cache) Get(ctx context.Context, id string) (*Booking, bool, error) {
	m, err := c.rdb.HGetAll(ctx, cache.KeyBooking(id)).Result()
	if err != nil || len(m) == 0 {
		return nil, false, err
	}
	var b Booking
	if err := json.Unmarshal([]byte(m["data"]), &b); err != nil {
		return nil, false, nil
	}
	b.State, b.DriverID = m["state"], m["driver_id"]
	return &b, true, nil
}

// transitionScript: compare-and-set state.
// KEYS[1] booking hash; ARGV[1] to; ARGV[2] n; ARGV[3..2+n] allowed from; rest field/value pairs.
var transitionScript = redis.NewScript(`
local cur = redis.call('HGET', KEYS[1], 'state')
if not cur then return {-1, ''} end
local n = tonumber(ARGV[2])
local ok = false
for i = 3, 2 + n do
  if ARGV[i] == cur then ok = true end
end
if not ok then return {0, cur} end
redis.call('HSET', KEYS[1], 'state', ARGV[1])
for i = 3 + n, #ARGV, 2 do
  redis.call('HSET', KEYS[1], ARGV[i], ARGV[i + 1])
end
return {1, cur}`)

// Transition atomically moves a booking to `to`, setting extra fields.
func (c *Cache) Transition(ctx context.Context, id, to string, fields map[string]string) error {
	froms := AllowedFrom(to)
	args := []any{to, len(froms)}
	for _, f := range froms {
		args = append(args, f)
	}
	for k, v := range fields {
		args = append(args, k, v)
	}
	res, err := transitionScript.Run(ctx, c.rdb, []string{cache.KeyBooking(id)}, args...).Slice()
	if err != nil {
		return err
	}
	switch code, _ := res[0].(int64); code {
	case 1:
		return nil
	case -1:
		return apperr.NotFound("booking not found")
	default:
		cur, _ := res[1].(string)
		return apperr.Conflict("booking is " + cur + ", cannot move to " + to)
	}
}

// ClaimActive enforces one active cab per rider (SET NX).
func (c *Cache) ClaimActive(ctx context.Context, userID, bookingID string) (bool, string, error) {
	k := cache.KeyUserActiveBooking(userID)
	ok, err := c.rdb.SetNX(ctx, k, bookingID, 6*time.Hour).Result()
	if err != nil || ok {
		return ok, bookingID, err
	}
	cur, err := c.rdb.Get(ctx, k).Result()
	return false, cur, err
}

func (c *Cache) ActiveOf(ctx context.Context, userID string) (string, error) {
	id, err := c.rdb.Get(ctx, cache.KeyUserActiveBooking(userID)).Result()
	if cache.IsNil(err) {
		return "", nil
	}
	return id, err
}

// ReleaseActive clears the rider's active booking if it is still this one.
func (c *Cache) ReleaseActive(ctx context.Context, userID, bookingID string) error {
	return cache.ReleaseLockScript.Run(ctx, c.rdb, []string{cache.KeyUserActiveBooking(userID)}, bookingID).Err()
}
