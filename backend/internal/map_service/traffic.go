package map_service

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// Traffic aggregates live speeds reported by GPS pings into 5-minute buckets.
//
//	sr:map:traffic:<roadID>:<bucket>  HASH sum,count  (15 min)
type Traffic struct{ rdb *redis.Client }

func NewTraffic(rdb *redis.Client) *Traffic { return &Traffic{rdb: rdb} }

const bucketSeconds = 300

func bucketOf(t time.Time) string { return strconv.FormatInt(t.Unix()/bucketSeconds, 10) }

// Record adds one speed sample for a road (called by the GPS service).
func (t *Traffic) Record(ctx context.Context, roadID string, speedKmh float64) error {
	if roadID == "" || speedKmh < 0 || speedKmh > 160 {
		return nil
	}
	k := cache.KeyRoadTraffic(roadID, bucketOf(time.Now()))
	pipe := t.rdb.Pipeline()
	pipe.HIncrByFloat(ctx, k, "sum", speedKmh)
	pipe.HIncrBy(ctx, k, "count", 1)
	pipe.Expire(ctx, k, cache.TTLTrafficBucket)
	_, err := pipe.Exec(ctx)
	return err
}

// AvgSpeeds returns the average speed per road over the current + previous bucket.
func (t *Traffic) AvgSpeeds(ctx context.Context, roadIDs []string) (map[string]float64, error) {
	now := time.Now()
	buckets := []string{bucketOf(now), bucketOf(now.Add(-bucketSeconds * time.Second))}
	pipe := t.rdb.Pipeline()
	cmds := map[string][]*redis.MapStringStringCmd{}
	for _, id := range roadIDs {
		for _, b := range buckets {
			cmds[id] = append(cmds[id], pipe.HGetAll(ctx, cache.KeyRoadTraffic(id, b)))
		}
	}
	if _, err := pipe.Exec(ctx); err != nil && !cache.IsNil(err) {
		return nil, err
	}
	out := map[string]float64{}
	for id, cs := range cmds {
		var sum, n float64
		for _, c := range cs {
			sum += cache.HGetFloat(c.Val(), "sum")
			n += cache.HGetFloat(c.Val(), "count")
		}
		if n >= 3 { // need a few cars before trusting the number
			out[id] = sum / n
		}
	}
	return out, nil
}

// Level converts an average speed to 0 (free flow) … 1 (standstill).
func Level(avgKmh float64, class string) float64 {
	free := map[string]float64{"motorway": 80, "trunk": 60, "primary": 45, "secondary": 40, "tertiary": 35}[class]
	if free == 0 {
		free = 25
	}
	return utils.Round(utils.Clamp(1-avgKmh/free, 0, 1), 2)
}
