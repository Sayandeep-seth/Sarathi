package petrol

import (
	"context"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/constants"
	"sarathi/internal/common/utils"
)

// Cache keeps every petrol station in Redis:
//
//	sr:geo:stations:petrol   GEO   station positions (nearby / along-route search)
//	sr:station:petrol:<id>   HASH  name,brand,address,lat,lng,fuels,open,rating,weekly
//	sr:rank:stations:petrol  ZSET  id -> crowd rating
type Cache struct {
	rdb *redis.Client
	geo *cache.Geo
}

func NewCache(rdb *redis.Client, geo *cache.Geo) *Cache { return &Cache{rdb: rdb, geo: geo} }

var geoKey = cache.KeyStationGeo(constants.StationPetrol)

func (c *Cache) Put(ctx context.Context, stations ...Station) error {
	pipe := c.rdb.Pipeline()
	for _, s := range stations {
		cache.AddPipe(ctx, pipe, geoKey, s.ID, s.Location)
		k := cache.KeyStation(constants.StationPetrol, s.ID)
		pipe.HSet(ctx, k, map[string]any{
			"name": s.Name, "brand": s.Brand, "address": s.Address,
			"lat": s.Location.Lat, "lng": s.Location.Lng,
			"fuels": strings.Join(s.Fuels, ","), "open": s.Open24x7,
		})
		pipe.Expire(ctx, k, cache.TTLStation)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Cache) Remove(ctx context.Context, id string) error {
	pipe := c.rdb.Pipeline()
	pipe.ZRem(ctx, geoKey, id)
	pipe.Del(ctx, cache.KeyStation(constants.StationPetrol, id))
	pipe.ZRem(ctx, cache.KeyStationRank(constants.StationPetrol), id)
	_, err := pipe.Exec(ctx)
	return err
}

// Details loads station hashes + ratings for ids in one round-trip.
func (c *Cache) Details(ctx context.Context, ids []string) (map[string]Station, error) {
	pipe := c.rdb.Pipeline()
	hs := make([]*redis.MapStringStringCmd, len(ids))
	for i, id := range ids {
		hs[i] = pipe.HGetAll(ctx, cache.KeyStation(constants.StationPetrol, id))
	}
	if _, err := pipe.Exec(ctx); err != nil && !cache.IsNil(err) {
		return nil, err
	}
	out := make(map[string]Station, len(ids))
	for i, id := range ids {
		m := hs[i].Val()
		if len(m) == 0 {
			continue
		}
		open, _ := strconv.ParseBool(m["open"])
		var fuels []string
		if m["fuels"] != "" {
			fuels = strings.Split(m["fuels"], ",")
		}
		out[id] = Station{
			ID: id, Name: m["name"], Brand: m["brand"], Address: m["address"],
			Location: utils.Point{Lat: cache.HGetFloat(m, "lat"), Lng: cache.HGetFloat(m, "lng")},
			Fuels:    fuels, Open24x7: open,
			Rating: cache.HGetFloat(m, "rating"), WeeklyVisits: cache.HGetInt(m, "weekly"),
		}
	}
	return out, nil
}

func (c *Cache) Nearby(ctx context.Context, p utils.Point, radiusKm float64, limit int) ([]cache.GeoHit, error) {
	return c.geo.Nearby(ctx, geoKey, p, radiusKm, limit)
}

func (c *Cache) AlongRoute(ctx context.Context, samples []utils.Sample, radiusKm float64) ([]cache.RouteHit, error) {
	return c.geo.AlongRoute(ctx, geoKey, samples, radiusKm, 5)
}

// AllIDs returns every station id in the GEO set.
func (c *Cache) AllIDs(ctx context.Context) ([]string, error) {
	return c.rdb.ZRange(ctx, geoKey, 0, -1).Result()
}

// Count is used at boot to decide whether a warm-up is needed.
func (c *Cache) Count(ctx context.Context) (int64, error) { return c.rdb.ZCard(ctx, geoKey).Result() }
