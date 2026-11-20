package ev

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/constants"
	"sarathi/internal/common/utils"
)

// Cache keeps chargers and live port availability in Redis:
//
//	sr:geo:stations:ev       GEO   charger positions
//	sr:station:ev:<id>       HASH  name,operator,lat,lng,connectors,power_kw,total_ports
//	sr:station:ev:<id>:ports HASH  free,total,updated  (10 min; operator/OCPP feed)
type Cache struct {
	rdb *redis.Client
	geo *cache.Geo
}

func NewCache(rdb *redis.Client, geo *cache.Geo) *Cache { return &Cache{rdb: rdb, geo: geo} }

var geoKey = cache.KeyStationGeo(constants.StationEV)

func (c *Cache) Put(ctx context.Context, chargers ...Charger) error {
	pipe := c.rdb.Pipeline()
	for _, ch := range chargers {
		cache.AddPipe(ctx, pipe, geoKey, ch.ID, ch.Location)
		k := cache.KeyStation(constants.StationEV, ch.ID)
		pipe.HSet(ctx, k, map[string]any{
			"name": ch.Name, "operator": ch.Operator, "lat": ch.Location.Lat, "lng": ch.Location.Lng,
			"connectors": strings.Join(ch.Connectors, ","), "power_kw": ch.PowerKW, "total_ports": ch.TotalPorts,
		})
		pipe.Expire(ctx, k, cache.TTLStation)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// SetPorts stores live free-port counts pushed by the charging operator.
func (c *Cache) SetPorts(ctx context.Context, id string, free, total int) error {
	k := cache.KeyEVPorts(id)
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, k, "free", free, "total", total, "updated", time.Now().Unix())
	pipe.Expire(ctx, k, cache.TTLEVPorts)
	_, err := pipe.Exec(ctx)
	return err
}

// Details loads charger hashes and live ports for ids.
func (c *Cache) Details(ctx context.Context, ids []string) (map[string]Charger, error) {
	pipe := c.rdb.Pipeline()
	hs := make([]*redis.MapStringStringCmd, len(ids))
	ps := make([]*redis.MapStringStringCmd, len(ids))
	for i, id := range ids {
		hs[i] = pipe.HGetAll(ctx, cache.KeyStation(constants.StationEV, id))
		ps[i] = pipe.HGetAll(ctx, cache.KeyEVPorts(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && !cache.IsNil(err) {
		return nil, err
	}
	out := make(map[string]Charger, len(ids))
	for i, id := range ids {
		m := hs[i].Val()
		if len(m) == 0 {
			continue
		}
		ch := Charger{
			ID: id, Name: m["name"], Operator: m["operator"],
			Location:   utils.Point{Lat: cache.HGetFloat(m, "lat"), Lng: cache.HGetFloat(m, "lng")},
			Connectors: strings.Split(m["connectors"], ","), PowerKW: cache.HGetFloat(m, "power_kw"),
			TotalPorts: int(cache.HGetInt(m, "total_ports")),
		}
		if p := ps[i].Val(); len(p) > 0 {
			ch.FreePorts = int(cache.HGetInt(p, "free"))
		} else {
			ch.FreePorts = -1 // unknown
		}
		out[id] = ch
	}
	return out, nil
}

func (c *Cache) Nearby(ctx context.Context, p utils.Point, radiusKm float64, limit int) ([]cache.GeoHit, error) {
	return c.geo.Nearby(ctx, geoKey, p, radiusKm, limit)
}

func (c *Cache) AlongRoute(ctx context.Context, samples []utils.Sample, radiusKm float64) ([]cache.RouteHit, error) {
	return c.geo.AlongRoute(ctx, geoKey, samples, radiusKm, 5)
}
