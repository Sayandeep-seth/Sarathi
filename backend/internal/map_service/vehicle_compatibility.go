package map_service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"sarathi/app/config"
	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// CompatResult says whether a car fits a road.
type CompatResult struct {
	RoadID          string  `json:"road_id"`
	Passable        bool    `json:"passable"`
	RequiredWidthM  float64 `json:"required_width_m"`
	AvailableWidthM float64 `json:"available_width_m"`
	MarginM         float64 `json:"margin_m"`
	Reason          string  `json:"reason,omitempty"`
}

// Compatibility applies the core rule of the Map part:
//
//	one-way road : road width ≥ car width + 2 × clearance
//	two-way road : road width ≥ car width + oncoming vehicle width + 3 × clearance
//	              (so any car coming from the opposite side can pass easily)
//	height       : car height + 0.10 m ≤ height limit (if the road has one)
//
// Results are cached per car model:  sr:map:compat:<model>  HASH roadID -> json (24 h)
type Compatibility struct {
	rdb *redis.Client
	cfg config.MapConfig
}

func NewCompatibility(rdb *redis.Client, cfg config.MapConfig) *Compatibility {
	return &Compatibility{rdb: rdb, cfg: cfg}
}

func (c *Compatibility) RequiredWidth(env Envelope, oneWay bool) float64 {
	if oneWay {
		return env.WidthM + 2*c.cfg.SafetyClearanceM
	}
	return env.WidthM + c.cfg.OncomingVehicleWidthM + 3*c.cfg.SafetyClearanceM
}

func (c *Compatibility) Check(env Envelope, road RoadAttr) CompatResult {
	req := c.RequiredWidth(env, road.OneWay)
	res := CompatResult{
		RoadID: road.ID, RequiredWidthM: utils.Round(req, 2), AvailableWidthM: road.WidthM,
		MarginM: utils.Round(road.WidthM-req, 2), Passable: road.WidthM >= req,
	}
	if !res.Passable {
		if road.OneWay {
			res.Reason = fmt.Sprintf("one-way road is %.1fm wide, %s needs %.1fm", road.WidthM, env.DisplayName, req)
		} else {
			res.Reason = fmt.Sprintf("two-way road is %.1fm wide; %s plus an oncoming car need %.1fm", road.WidthM, env.DisplayName, req)
		}
	}
	if road.MaxHeightM > 0 && env.HeightM+0.10 > road.MaxHeightM {
		res.Passable = false
		res.Reason = fmt.Sprintf("height limit %.2fm, car is %.2fm", road.MaxHeightM, env.HeightM)
	}
	return res
}

// CheckRoads checks many roads, using the per-model Redis cache.
func (c *Compatibility) CheckRoads(ctx context.Context, env Envelope, roads []RoadAttr) (map[string]CompatResult, error) {
	out := make(map[string]CompatResult, len(roads))
	if len(roads) == 0 {
		return out, nil
	}
	key := cache.KeyRoadCompat(env.Model)
	fields := make([]string, len(roads))
	for i, r := range roads {
		fields[i] = r.ID
	}
	cached, err := c.rdb.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, err
	}
	toWrite := map[string]any{}
	for i, r := range roads {
		if s, ok := cached[i].(string); ok {
			var cr CompatResult
			// reuse only if the road width has not been corrected since
			if json.Unmarshal([]byte(s), &cr) == nil && cr.AvailableWidthM == r.WidthM {
				out[r.ID] = cr
				continue
			}
		}
		cr := c.Check(env, r)
		out[r.ID] = cr
		b, _ := json.Marshal(cr)
		toWrite[r.ID] = string(b)
	}
	if len(toWrite) > 0 {
		pipe := c.rdb.Pipeline()
		pipe.HSet(ctx, key, toWrite)
		pipe.Expire(ctx, key, cache.TTLRoadCompat)
		_, _ = pipe.Exec(ctx)
	}
	return out, nil
}
