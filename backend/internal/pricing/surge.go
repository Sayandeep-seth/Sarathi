package pricing

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// Surge computes a demand/supply multiplier per ~2 km grid cell.
//
//	sr:surge:demand:<cell>:<5-min bucket>  STRING  ride requests (15 min)
//	sr:surge:mult:<cell>                   STRING  cached multiplier (60 s)
//	supply = drivers in sr:geo:drivers:<class> within 3 km (GEOSEARCH)
type Surge struct {
	rdb *redis.Client
	geo *cache.Geo
}

func NewSurge(rdb *redis.Client, geo *cache.Geo) *Surge { return &Surge{rdb: rdb, geo: geo} }

const (
	cellDeg      = 0.02
	maxSurge     = 2.0
	supplyRadius = 3.0
)

func bucket(t time.Time) int64 { return t.Unix() / 300 }

// RecordDemand counts a ride request in the pickup cell.
func (s *Surge) RecordDemand(ctx context.Context, p utils.Point) error {
	k := cache.KeySurgeDemand(utils.Cell(p, cellDeg), bucket(time.Now()))
	pipe := s.rdb.Pipeline()
	pipe.Incr(ctx, k)
	pipe.Expire(ctx, k, cache.TTLSurgeDemand)
	_, err := pipe.Exec(ctx)
	return err
}

// Multiplier returns 1.0 … 2.0 for a pickup point and vehicle class.
func (s *Surge) Multiplier(ctx context.Context, class string, p utils.Point) float64 {
	cell := utils.Cell(p, cellDeg)
	mk := cache.KeySurgeMultiplier(cell + ":" + class)
	if v, err := s.rdb.Get(ctx, mk).Float64(); err == nil {
		return v
	}
	now := time.Now()
	vals, err := s.rdb.MGet(ctx,
		cache.KeySurgeDemand(cell, bucket(now)),
		cache.KeySurgeDemand(cell, bucket(now)-1)).Result()
	if err != nil {
		return 1
	}
	var demand float64
	for _, v := range vals {
		if str, ok := v.(string); ok {
			n, _ := strconv.ParseFloat(str, 64)
			demand += n
		}
	}
	supply, err := s.geo.Count(ctx, cache.KeyDriversGeo(class), p, supplyRadius)
	if err != nil {
		return 1
	}
	ratio := demand / float64(max(supply, 1))
	mult := 1.0
	if ratio > 1.2 {
		mult = utils.Round(utils.Clamp(1+(ratio-1)*0.25, 1, maxSurge), 1)
	}
	s.rdb.Set(ctx, mk, mult, cache.TTLSurgeMultiplier)
	return mult
}
