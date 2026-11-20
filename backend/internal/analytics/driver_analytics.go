package analytics

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// RecordEarning adds to a driver's monthly earnings leaderboard.
//
//	sr:stats:driver:earn:<yyyymm>  ZSET driver -> INR
func (s *Service) RecordEarning(ctx context.Context, driverID string, amount float64, at time.Time) {
	k := cache.KeyStatDriverEarnings(utils.MonthKey(at))
	pipe := s.rdb.Pipeline()
	pipe.ZIncrBy(ctx, k, amount, driverID)
	pipe.Expire(ctx, k, cache.TTLStats)
	_, _ = pipe.Exec(ctx)
}

// RecordOffer tracks offers vs acceptances (used in dispatch matching score).
//
//	sr:stats:driver:offers:<driver>  HASH offered, accepted
func (s *Service) RecordOffer(ctx context.Context, driverID string, accepted bool) {
	k := cache.KeyStatDriverOffers(driverID)
	pipe := s.rdb.Pipeline()
	pipe.HIncrBy(ctx, k, "offered", 1)
	if accepted {
		pipe.HIncrBy(ctx, k, "accepted", 1)
	}
	pipe.Expire(ctx, k, 30*24*time.Hour)
	_, _ = pipe.Exec(ctx)
}

// AcceptanceRate returns accepted/offered (0.8 default for new drivers).
func (s *Service) AcceptanceRate(ctx context.Context, driverID string) float64 {
	m, err := s.rdb.HGetAll(ctx, cache.KeyStatDriverOffers(driverID)).Result()
	offered := cache.HGetFloat(m, "offered")
	if err != nil || offered < 5 {
		return 0.8
	}
	return cache.HGetFloat(m, "accepted") / offered
}

// TopEarners returns the month leaderboard.
func (s *Service) TopEarners(ctx context.Context, month string, n int64) ([]redis.Z, error) {
	return s.rdb.ZRevRangeWithScores(ctx, cache.KeyStatDriverEarnings(month), 0, n-1).Result()
}
