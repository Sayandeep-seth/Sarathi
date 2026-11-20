package analytics

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// RecordRide updates ride counters by booking type, revenue, demand heatmap and
// unique riders — one pipelined round-trip.
//
//	sr:stats:rides:<date>    HASH type -> count
//	sr:stats:revenue:<date>  HASH type -> INR
//	sr:stats:heat:<date>     ZSET grid cell -> pickups
func (s *Service) RecordRide(ctx context.Context, bookingType, userID string, fare float64, pickup utils.Point, at time.Time) {
	date := utils.DateKey(at)
	rides, rev, heat := cache.KeyStatRides(date), cache.KeyStatRevenue(date), cache.KeyStatHeatmap(date)
	uniq := cache.KeyStatUnique(MetricActiveUsers, date)
	pipe := s.rdb.Pipeline()
	pipe.HIncrBy(ctx, rides, bookingType, 1)
	pipe.HIncrByFloat(ctx, rev, bookingType, fare)
	pipe.ZIncrBy(ctx, heat, 1, utils.Cell(pickup, 0.01))
	pipe.PFAdd(ctx, uniq, userID)
	for _, k := range []string{rides, rev, heat, uniq} {
		pipe.Expire(ctx, k, cache.TTLStats)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		s.log.Debug("record ride stats", "err", err)
	}
}

// Heatmap returns the busiest pickup cells of a day.
func (s *Service) Heatmap(ctx context.Context, date string, n int64) ([]redis.Z, error) {
	return s.rdb.ZRevRangeWithScores(ctx, cache.KeyStatHeatmap(date), 0, n-1).Result()
}
