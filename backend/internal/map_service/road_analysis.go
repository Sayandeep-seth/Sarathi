package map_service

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"

	"sarathi/app/config"
	"sarathi/internal/ai"
	"sarathi/internal/cache"
)

// RoadAttr is what we know about a road's physical size.
type RoadAttr struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Class      string  `json:"class"`
	WidthM     float64 `json:"width_m"` // total carriageway width
	OneWay     bool    `json:"one_way"`
	Lanes      int     `json:"lanes"`
	MaxHeightM float64 `json:"max_height_m,omitempty"` // underpass / height barrier
	Source     string  `json:"source"`                 // measured | predicted
}

// RoadAnalyzer resolves road attributes: Redis HASH -> Postgres -> AI width prediction.
//
//	sr:map:road:<roadID>  HASH name,class,width_m,one_way,lanes,max_height_m,source (24 h)
type RoadAnalyzer struct {
	rdb       *redis.Client
	repo      *Repository
	predictor *ai.RoadPredictor
	cfg       config.MapConfig
}

func NewRoadAnalyzer(rdb *redis.Client, repo *Repository, p *ai.RoadPredictor, cfg config.MapConfig) *RoadAnalyzer {
	return &RoadAnalyzer{rdb: rdb, repo: repo, predictor: p, cfg: cfg}
}

// Attributes returns attributes for every id (one pipelined Redis round-trip).
func (a *RoadAnalyzer) Attributes(ctx context.Context, ids []string, names map[string]string) (map[string]RoadAttr, error) {
	out := make(map[string]RoadAttr, len(ids))
	pipe := a.rdb.Pipeline()
	cmds := make(map[string]*redis.MapStringStringCmd, len(ids))
	for _, id := range ids {
		cmds[id] = pipe.HGetAll(ctx, cache.KeyRoadAttr(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && !cache.IsNil(err) {
		return nil, err
	}
	var missing []string
	for id, c := range cmds {
		m := c.Val()
		if len(m) == 0 {
			missing = append(missing, id)
			continue
		}
		oneWay, _ := strconv.ParseBool(m["one_way"])
		out[id] = RoadAttr{
			ID: id, Name: m["name"], Class: m["class"], WidthM: cache.HGetFloat(m, "width_m"),
			OneWay: oneWay, Lanes: int(cache.HGetInt(m, "lanes")), MaxHeightM: cache.HGetFloat(m, "max_height_m"),
			Source: m["source"],
		}
	}
	if len(missing) == 0 {
		return out, nil
	}

	fromDB, err := a.repo.GetRoads(ctx, missing)
	if err != nil {
		return nil, err
	}
	wpipe := a.rdb.Pipeline()
	for _, id := range missing {
		attr, ok := fromDB[id]
		if !ok {
			attr = RoadAttr{ID: id, Name: names[id], Class: "unclassified"}
		}
		if attr.WidthM <= 0 {
			attr.WidthM = a.predictor.PredictWidthM(ctx, attr.Class, attr.Lanes, attr.OneWay, a.cfg.DefaultLaneWidthM)
			attr.Source = "predicted"
		}
		out[id] = attr
		k := cache.KeyRoadAttr(id)
		wpipe.HSet(ctx, k, map[string]any{
			"name": attr.Name, "class": attr.Class, "width_m": attr.WidthM, "one_way": attr.OneWay,
			"lanes": attr.Lanes, "max_height_m": attr.MaxHeightM, "source": attr.Source,
		})
		wpipe.Expire(ctx, k, cache.TTLRoadAttr)
	}
	_, _ = wpipe.Exec(ctx)
	return out, nil
}

// Invalidate drops a road from cache (after a width correction).
func (a *RoadAnalyzer) Invalidate(ctx context.Context, roadID string) error {
	return a.rdb.Del(ctx, cache.KeyRoadAttr(roadID)).Err()
}
