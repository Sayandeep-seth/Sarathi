package analytics

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// Station visit leaderboards are written by petrol.TrafficTracker into
// sr:stats:station:<kind>:<date> (ZSET station -> visits).

// TopStations returns the most visited stations on a day.
func (s *Service) TopStations(ctx context.Context, kind, date string, n int64) ([]redis.Z, error) {
	return s.rdb.ZRevRangeWithScores(ctx, cache.KeyStatStationVisits(kind, date), 0, n-1).Result()
}

// TopStationsWeek merges the last 7 daily boards with ZUNIONSTORE into a short-lived key.
func (s *Service) TopStationsWeek(ctx context.Context, kind string, n int64) ([]redis.Z, error) {
	now := time.Now()
	keys := make([]string, 7)
	for d := 0; d < 7; d++ {
		keys[d] = cache.KeyStatStationVisits(kind, utils.DateKey(now.AddDate(0, 0, -d)))
	}
	dst := cache.KeyStatStationVisits(kind, "week:"+utils.DateKey(now))
	pipe := s.rdb.Pipeline()
	pipe.ZUnionStore(ctx, dst, &redis.ZStore{Keys: keys, Aggregate: "SUM"})
	pipe.Expire(ctx, dst, 10*time.Minute)
	top := pipe.ZRevRangeWithScores(ctx, dst, 0, n-1)
	if _, err := pipe.Exec(ctx); err != nil && !cache.IsNil(err) {
		return nil, err
	}
	return top.Val(), nil
}
