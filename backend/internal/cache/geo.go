package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/common/utils"
)

// GeoHit is a member found by a GEO search.
type GeoHit struct {
	Member string      `json:"id"`
	Point  utils.Point `json:"location"`
	DistKm float64     `json:"distance_km"`
}

// Geo wraps Redis GEO commands (GEOADD / GEOSEARCH / GEOPOS / GEODIST).
// Used for drivers, live vehicles, petrol/EV stations, hubs, bus stops, shared trips.
type Geo struct {
	rdb *redis.Client
}

func NewGeo(rdb *redis.Client) *Geo { return &Geo{rdb: rdb} }

// Add upserts a member position.
func (g *Geo) Add(ctx context.Context, key, member string, p utils.Point) error {
	if !p.Valid() {
		return fmt.Errorf("geo: invalid point %+v", p)
	}
	return g.rdb.GeoAdd(ctx, key, &redis.GeoLocation{Name: member, Latitude: p.Lat, Longitude: p.Lng}).Err()
}

// AddPipe queues a GEOADD on a pipeline.
func AddPipe(ctx context.Context, pipe redis.Pipeliner, key, member string, p utils.Point) {
	pipe.GeoAdd(ctx, key, &redis.GeoLocation{Name: member, Latitude: p.Lat, Longitude: p.Lng})
}

// Remove deletes members (GEO sets are sorted sets).
func (g *Geo) Remove(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	args := make([]any, len(members))
	for i, m := range members {
		args[i] = m
	}
	return g.rdb.ZRem(ctx, key, args...).Err()
}

// Nearby returns members within radiusKm sorted by distance (limit 0 = unlimited).
func (g *Geo) Nearby(ctx context.Context, key string, p utils.Point, radiusKm float64, limit int) ([]GeoHit, error) {
	locs, err := g.rdb.GeoSearchLocation(ctx, key, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  p.Lng,
			Latitude:   p.Lat,
			Radius:     radiusKm,
			RadiusUnit: "km",
			Sort:       "ASC",
			Count:      limit,
		},
		WithCoord: true,
		WithDist:  true,
	}).Result()
	if err != nil {
		return nil, err
	}
	out := make([]GeoHit, len(locs))
	for i, l := range locs {
		out[i] = GeoHit{Member: l.Name, Point: utils.Point{Lat: l.Latitude, Lng: l.Longitude}, DistKm: l.Dist}
	}
	return out, nil
}

// NearbyExpanding searches growing radii until at least `min` members are found.
func (g *Geo) NearbyExpanding(ctx context.Context, key string, p utils.Point, radiiKm []float64, min, limit int) ([]GeoHit, float64, error) {
	var hits []GeoHit
	var err error
	for _, r := range radiiKm {
		hits, err = g.Nearby(ctx, key, p, r, limit)
		if err != nil {
			return nil, r, err
		}
		if len(hits) >= min {
			return hits, r, nil
		}
	}
	return hits, radiiKm[len(radiiKm)-1], nil
}

// Count returns how many members are within radiusKm (used for supply in surge pricing).
func (g *Geo) Count(ctx context.Context, key string, p utils.Point, radiusKm float64) (int, error) {
	names, err := g.rdb.GeoSearch(ctx, key, &redis.GeoSearchQuery{
		Longitude: p.Lng, Latitude: p.Lat, Radius: radiusKm, RadiusUnit: "km",
	}).Result()
	return len(names), err
}

// Position returns the stored coordinate of a member.
func (g *Geo) Position(ctx context.Context, key, member string) (utils.Point, bool, error) {
	pos, err := g.rdb.GeoPos(ctx, key, member).Result()
	if err != nil {
		return utils.Point{}, false, err
	}
	if len(pos) == 0 || pos[0] == nil {
		return utils.Point{}, false, nil
	}
	return utils.Point{Lat: pos[0].Latitude, Lng: pos[0].Longitude}, true, nil
}

// RouteHit is a member found near a route, with how far along the route it is.
type RouteHit struct {
	GeoHit
	AlongKm float64 `json:"along_route_km"`
}

// AlongRoute runs one pipelined GEOSEARCH per route sample and returns each
// member once, keeping the earliest position along the route. This is how we find
// petrol pumps / EV chargers "during the journey" with a single Redis round-trip.
func (g *Geo) AlongRoute(ctx context.Context, key string, samples []utils.Sample, radiusKm float64, perPoint int) ([]RouteHit, error) {
	pipe := g.rdb.Pipeline()
	cmds := make([]*redis.GeoSearchLocationCmd, len(samples))
	for i, s := range samples {
		cmds[i] = pipe.GeoSearchLocation(ctx, key, &redis.GeoSearchLocationQuery{
			GeoSearchQuery: redis.GeoSearchQuery{
				Longitude: s.Point.Lng, Latitude: s.Point.Lat,
				Radius: radiusKm, RadiusUnit: "km", Sort: "ASC", Count: perPoint,
			},
			WithCoord: true, WithDist: true,
		})
	}
	if _, err := pipe.Exec(ctx); err != nil && !IsNil(err) {
		return nil, err
	}
	best := map[string]RouteHit{}
	order := []string{}
	for i, c := range cmds {
		for _, l := range c.Val() {
			along := samples[i].AlongKm + l.Dist
			if prev, ok := best[l.Name]; ok && prev.AlongKm <= along {
				continue
			} else if !ok {
				order = append(order, l.Name)
			}
			best[l.Name] = RouteHit{
				GeoHit:  GeoHit{Member: l.Name, Point: utils.Point{Lat: l.Latitude, Lng: l.Longitude}, DistKm: l.Dist},
				AlongKm: along,
			}
		}
	}
	out := make([]RouteHit, 0, len(order))
	for _, id := range order {
		out = append(out, best[id])
	}
	return out, nil
}

// DistanceKm returns the distance between two members of the same set.
func (g *Geo) DistanceKm(ctx context.Context, key, a, b string) (float64, error) {
	return g.rdb.GeoDist(ctx, key, a, b, "km").Result()
}
