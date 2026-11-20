// Package map_service plans routes that a specific car can physically drive,
// checks every road against the car's real size, and overlays live traffic.
package map_service

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/vehicles"
)

// Envelope is the physical footprint of a car used for road checks.
type Envelope struct {
	Model          string  `json:"model"`
	DisplayName    string  `json:"display_name"`
	WidthM         float64 `json:"width_m"` // incl. mirrors
	LengthM        float64 `json:"length_m"`
	HeightM        float64 `json:"height_m"`
	TurningRadiusM float64 `json:"turning_radius_m"`
}

// CarSpecCache is a 2-level cache (in-process 1 min + Redis 24 h) of envelopes.
// Route planning touches it for every request, so we avoid even a Redis hop when hot.
//
//	sr:map:envelope:<model>  HASH width_m,length_m,height_m,turning_radius_m,display_name
type CarSpecCache struct {
	rdb      *redis.Client
	vehicles *vehicles.Service
	local    sync.Map // model -> localEnvelope
}

type localEnvelope struct {
	env Envelope
	at  time.Time
}

func NewCarSpecCache(rdb *redis.Client, v *vehicles.Service) *CarSpecCache {
	return &CarSpecCache{rdb: rdb, vehicles: v}
}

func (c *CarSpecCache) Envelope(ctx context.Context, carName string) (Envelope, error) {
	model := vehicles.NormalizeModel(carName)
	if v, ok := c.local.Load(model); ok && time.Since(v.(localEnvelope).at) < time.Minute {
		return v.(localEnvelope).env, nil
	}
	key := cache.KeyCarEnvelope(model)
	if m, err := c.rdb.HGetAll(ctx, key).Result(); err == nil && len(m) > 0 {
		env := Envelope{
			Model: model, DisplayName: m["display_name"],
			WidthM: cache.HGetFloat(m, "width_m"), LengthM: cache.HGetFloat(m, "length_m"),
			HeightM: cache.HGetFloat(m, "height_m"), TurningRadiusM: cache.HGetFloat(m, "turning_radius_m"),
		}
		c.local.Store(model, localEnvelope{env, time.Now()})
		return env, nil
	}
	spec, err := c.vehicles.GetSpec(ctx, carName)
	if err != nil {
		return Envelope{}, err
	}
	env := Envelope{
		Model: spec.Model, DisplayName: spec.DisplayName, WidthM: spec.EffectiveWidthM(),
		LengthM: spec.LengthM(), HeightM: spec.HeightM(), TurningRadiusM: spec.TurningRadiusM,
	}
	pipe := c.rdb.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{
		"display_name": env.DisplayName, "width_m": env.WidthM, "length_m": env.LengthM,
		"height_m": env.HeightM, "turning_radius_m": env.TurningRadiusM,
	})
	pipe.Expire(ctx, key, cache.TTLCarEnvelope)
	_, _ = pipe.Exec(ctx)
	c.local.Store(model, localEnvelope{env, time.Now()})
	return env, nil
}
