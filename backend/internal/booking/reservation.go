package booking

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
)

// Reservations keeps future bookings in a sorted set by pickup time; the worker
// dispatches them `lead` minutes before pickup.
//
//	sr:booking:scheduled  ZSET booking -> pickup unix
type Reservations struct{ rdb *redis.Client }

func NewReservations(rdb *redis.Client) *Reservations { return &Reservations{rdb: rdb} }

func (r *Reservations) Schedule(ctx context.Context, bookingID string, pickup time.Time) error {
	return r.rdb.ZAdd(ctx, cache.KeyBookingScheduled(), redis.Z{Score: float64(pickup.Unix()), Member: bookingID}).Err()
}

func (r *Reservations) Unschedule(ctx context.Context, bookingID string) error {
	return r.rdb.ZRem(ctx, cache.KeyBookingScheduled(), bookingID).Err()
}

// Due returns bookings to dispatch now; ZREM makes each claim exclusive across workers.
func (r *Reservations) Due(ctx context.Context, lead time.Duration) ([]string, error) {
	max := strconv.FormatInt(time.Now().Add(lead).Unix(), 10)
	ids, err := r.rdb.ZRangeByScore(ctx, cache.KeyBookingScheduled(), &redis.ZRangeBy{Min: "-inf", Max: max, Count: 100}).Result()
	if err != nil {
		return nil, err
	}
	claimed := ids[:0]
	for _, id := range ids {
		if n, _ := r.rdb.ZRem(ctx, cache.KeyBookingScheduled(), id).Result(); n == 1 {
			claimed = append(claimed, id)
		}
	}
	return claimed, nil
}
