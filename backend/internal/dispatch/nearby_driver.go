package dispatch

import (
	"context"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
	"sarathi/internal/driver"
)

// Candidate is a driver who could take a ride.
type Candidate struct {
	DriverID   string  `json:"driver_id"`
	DistKm     float64 `json:"distance_km"`
	Rating     float64 `json:"rating"`
	Acceptance float64 `json:"acceptance"`
	Score      float64 `json:"score"`
}

// Finder searches sr:geo:drivers:<class> with expanding radius and filters out
// drivers that are locked (already being offered another ride), not available,
// or already rejected this booking — all in one pipeline.
type Finder struct {
	rdb *redis.Client
	geo *cache.Geo
}

func NewFinder(rdb *redis.Client, geo *cache.Geo) *Finder { return &Finder{rdb: rdb, geo: geo} }

var searchRadiiKm = []float64{1, 2, 4, 6, 10}

func (f *Finder) Find(ctx context.Context, class string, p utils.Point, exclude map[string]bool, maxRadiusKm float64) ([]Candidate, error) {
	radii := searchRadiiKm
	if maxRadiusKm > 0 {
		radii = nil
		for _, r := range searchRadiiKm {
			if r <= maxRadiusKm {
				radii = append(radii, r)
			}
		}
		if len(radii) == 0 {
			radii = []float64{maxRadiusKm}
		}
	}
	hits, _, err := f.geo.NearbyExpanding(ctx, cache.KeyDriversGeo(class), p, radii, 3, 25)
	if err != nil || len(hits) == 0 {
		return nil, err
	}
	pipe := f.rdb.Pipeline()
	states := make([]*redis.SliceCmd, len(hits))
	locks := make([]*redis.IntCmd, len(hits))
	beats := make([]*redis.IntCmd, len(hits))
	for i, h := range hits {
		states[i] = pipe.HMGet(ctx, cache.KeyDriver(h.Member), "status", "rating")
		locks[i] = pipe.Exists(ctx, cache.KeyLock(cache.KeyDriverLockName(h.Member)))
		beats[i] = pipe.Exists(ctx, cache.KeyDriverHeartbeat(h.Member))
	}
	if _, err := pipe.Exec(ctx); err != nil && !cache.IsNil(err) {
		return nil, err
	}
	out := make([]Candidate, 0, len(hits))
	for i, h := range hits {
		if exclude[h.Member] {
			continue
		}
		vals := states[i].Val()
		status, _ := vals[0].(string)
		// must be available, app alive (heartbeat) and not currently holding another offer
		if status != driver.StatusAvailable || beats[i].Val() == 0 || locks[i].Val() == 1 {
			continue
		}
		rating := 4.5
		if rs, ok := vals[1].(string); ok {
			if v := parseFloat(rs); v > 0 {
				rating = v
			}
		}
		out = append(out, Candidate{DriverID: h.Member, DistKm: h.DistKm, Rating: rating})
	}
	return out, nil
}
