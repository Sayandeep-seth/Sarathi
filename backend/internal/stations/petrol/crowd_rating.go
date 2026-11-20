package petrol

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/constants"
	"sarathi/internal/common/utils"
)

// CrowdRater turns GPS visit counts into a 1..5 star rating.
//
// Idea: drivers vote with their wheels. A station that many distinct cars choose
// every week is trusted (quality fuel, honest metering, good service). We use
// distinct vehicles over 7 days (HyperLogLog), log-scaled against the busiest
// station in the city so one mega-station does not flatten everyone else.
//
//	rating = 1 + 4 × ln(1 + visits) / ln(1 + maxVisits)
type CrowdRater struct {
	rdb     *redis.Client
	tracker *TrafficTracker
	cache   *Cache
	repo    *Repository
	log     *slog.Logger
}

func NewCrowdRater(rdb *redis.Client, t *TrafficTracker, c *Cache, repo *Repository, log *slog.Logger) *CrowdRater {
	return &CrowdRater{rdb: rdb, tracker: t, cache: c, repo: repo, log: log}
}

const ratingWindowDays = 7

// RecomputeAll is run by the worker every 10 minutes.
func (r *CrowdRater) RecomputeAll(ctx context.Context, now time.Time) error {
	ids, err := r.cache.AllIDs(ctx)
	if err != nil || len(ids) == 0 {
		return err
	}
	const batch = 500
	visits := make(map[string]int64, len(ids))
	var max int64
	for i := 0; i < len(ids); i += batch {
		end := min(i+batch, len(ids))
		v, err := r.tracker.UniqueVisits(ctx, ids[i:end], ratingWindowDays, now)
		if err != nil {
			return err
		}
		for id, n := range v {
			visits[id] = n
			if n > max {
				max = n
			}
		}
	}

	rankKey := cache.KeyStationRank(constants.StationPetrol)
	pipe := r.rdb.Pipeline()
	day := utils.DateKey(now)
	for id, n := range visits {
		rating := Rating(n, max)
		pipe.ZAdd(ctx, rankKey, redis.Z{Score: rating, Member: id})
		pipe.HSet(ctx, cache.KeyStation(constants.StationPetrol, id), "rating", rating, "weekly", n)
		if now.Hour() == 23 && now.Minute() < 10 { // persist one snapshot per day
			go func(id string, n int64, rating float64) {
				if err := r.repo.SaveDailyRating(context.WithoutCancel(ctx), id, day, n, rating); err != nil {
					r.log.Warn("save station rating", "id", id, "err", err)
				}
			}(id, n, rating)
		}
	}
	_, err = pipe.Exec(ctx)
	return err
}

// Rating maps weekly distinct visits to 1..5 stars.
func Rating(visits, maxVisits int64) float64 {
	if maxVisits <= 0 || visits <= 0 {
		return 1
	}
	return utils.Round(1+4*math.Log1p(float64(visits))/math.Log1p(float64(maxVisits)), 1)
}

// Score ranks stations for "best petrol pump": quality first, then short queue, then distance.
func Score(s Station) float64 {
	queuePenalty := math.Min(float64(s.CarsNow)*0.15, 1.5) // ~2 min per car, capped
	distPenalty := math.Min(s.DistKm*0.2, 1.5)
	return utils.Round(s.Rating*2-queuePenalty-distPenalty, 2)
}

// TopRated returns the best rated stations city-wide.
func (r *CrowdRater) TopRated(ctx context.Context, n int64) ([]redis.Z, error) {
	return r.rdb.ZRevRangeWithScores(ctx, cache.KeyStationRank(constants.StationPetrol), 0, n-1).Result()
}
