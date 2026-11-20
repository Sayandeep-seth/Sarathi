package map_service

import (
	"context"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// RouteCache caches two layers:
//   - raw alternatives from the routing engine (car independent, 5 min)
//   - final per-car route answers (5 min, traffic is re-read so short TTL)
//
// Coordinates are rounded to 4 decimals (~11 m) so nearby requests share entries.
//
//	sr:map:route:<hash>  STRING json
type RouteCache struct{ cs *cache.Service }

func NewRouteCache(cs *cache.Service) *RouteCache { return &RouteCache{cs: cs} }

func rawKey(o, d utils.Point) string {
	return cache.KeyRoute(utils.HashKey("raw", utils.Round(o.Lat, 4), utils.Round(o.Lng, 4), utils.Round(d.Lat, 4), utils.Round(d.Lng, 4)))
}

func carKey(o, d utils.Point, model string) string {
	return cache.KeyRoute(utils.HashKey("car", model, utils.Round(o.Lat, 4), utils.Round(o.Lng, 4), utils.Round(d.Lat, 4), utils.Round(d.Lng, 4)))
}

func (c *RouteCache) Raw(ctx context.Context, o, d utils.Point, load func(context.Context) ([]RawRoute, error)) ([]RawRoute, error) {
	return cache.Remember(ctx, c.cs, rawKey(o, d), cache.TTLRoute, load)
}

func (c *RouteCache) GetForCar(ctx context.Context, o, d utils.Point, model string) (*RouteResponse, bool) {
	var r RouteResponse
	found, err := c.cs.GetJSON(ctx, carKey(o, d, model), &r)
	if err != nil || !found {
		return nil, false
	}
	r.Cached = true
	return &r, true
}

func (c *RouteCache) SetForCar(ctx context.Context, o, d utils.Point, model string, r *RouteResponse) {
	_ = c.cs.SetJSON(ctx, carKey(o, d, model), r, cache.TTLRoute)
}
