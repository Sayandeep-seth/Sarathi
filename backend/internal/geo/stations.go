package geo

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Station crowd + rating from GPS.
//
// Your requirement: "rate petrol pumps by the number of cars seen there,
// using GPS". The naive version (count every car within 60 m) counts every
// car that simply DRIVES PAST on the main road. So a car only counts as a
// visit when it:
//  1. is inside the station geofence (60 m),
//  2. is moving slowly (< 8 km/h), and
//  3. is still inside the fence >= 90 seconds after we first saw it there.
//
// Redis keys
//
//	geo:stations                    GEO set   member=stationID (loaded from Postgres at boot)
//	station:{id}:meta               HASH      type=petrol|ev|cng, name=...
//	stdwell:{sid}:{vehicle}         STRING    first-seen unix   (TTL 15m)
//	stvisit:{sid}:{vehicle}         STRING    "1"               (TTL 2h, prevents double counting)
//	station:{sid}:present           ZSET      vehicle -> last seen unix (cars there NOW)
//	station:{sid}:cars:{yyyymmdd}   HyperLogLog of unique visiting vehicles per day (TTL 8d)
//	station:{sid}:crowdpub          STRING    publish throttle (TTL 15s)
const (
	stationsGeoKey   = "geo:stations"
	fenceMeters      = 60
	maxDwellSpeedKmh = 8
	minDwell         = 90 * time.Second
	presentWindow    = 10 * time.Minute
)

// CrowdPublisher is satisfied by *realtime.Publisher (kept as an interface so
// geo does not import realtime).
type CrowdPublisher interface {
	Publish(ctx context.Context, topic, msgType string, data any) error
}

type StationTracker struct {
	rdb redis.UniversalClient
	pub CrowdPublisher
}

func NewStationTracker(rdb redis.UniversalClient, pub CrowdPublisher) *StationTracker {
	return &StationTracker{rdb: rdb, pub: pub}
}

type Station struct {
	ID   string
	Name string
	Type string // petrol | diesel | cng | ev
	Lat  float64
	Lng  float64
}

// Load puts stations into Redis. Call at startup and whenever ops edit stations.
func (s *StationTracker) Load(ctx context.Context, stations []Station) error {
	pipe := s.rdb.Pipeline()
	for _, st := range stations {
		pipe.GeoAdd(ctx, stationsGeoKey, &redis.GeoLocation{Name: st.ID, Latitude: st.Lat, Longitude: st.Lng})
		pipe.HSet(ctx, "station:"+st.ID+":meta", "name", st.Name, "type", st.Type)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// Observe is called for every location update from any car (cab drivers AND
// private cars using Map mode). It is cheap when the car is not at a station:
// one GEOSEARCH that returns nothing.
func (s *StationTracker) Observe(ctx context.Context, vehicleID string, lat, lng, speedKmh float64) error {
	if vehicleID == "" || speedKmh > maxDwellSpeedKmh {
		return nil
	}
	hits, err := s.rdb.GeoSearch(ctx, stationsGeoKey, &redis.GeoSearchQuery{
		Latitude: lat, Longitude: lng,
		Radius: fenceMeters, RadiusUnit: "m",
		Sort: "ASC", Count: 1,
	}).Result()
	if err != nil || len(hits) == 0 {
		return err
	}
	sid := hits[0]
	now := time.Now()

	// Dwell check: remember when we first saw this car here.
	dwellKey := "stdwell:" + sid + ":" + vehicleID
	firstSeen := now.Unix()
	set, err := s.rdb.SetNX(ctx, dwellKey, firstSeen, 15*time.Minute).Result()
	if err != nil {
		return err
	}
	if !set {
		if v, err := s.rdb.Get(ctx, dwellKey).Int64(); err == nil {
			firstSeen = v
		}
	}
	if now.Sub(time.Unix(firstSeen, 0)) < minDwell {
		return nil // not yet a real visit
	}

	presentKey := "station:" + sid + ":present"
	dayKey := "station:" + sid + ":cars:" + now.Format("20060102")
	var carsNow *redis.IntCmd
	var newVisit *redis.BoolCmd
	_, err = s.rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
		p.ZAdd(ctx, presentKey, redis.Z{Score: float64(now.Unix()), Member: vehicleID})
		p.ZRemRangeByScore(ctx, presentKey, "-inf", strconv.FormatInt(now.Add(-presentWindow).Unix(), 10))
		carsNow = p.ZCard(ctx, presentKey)
		p.Expire(ctx, presentKey, time.Hour)
		newVisit = p.SetNX(ctx, "stvisit:"+sid+":"+vehicleID, "1", 2*time.Hour)
		return nil
	})
	if err != nil {
		return err
	}
	if newVisit.Val() {
		pipe := s.rdb.Pipeline()
		pipe.PFAdd(ctx, dayKey, vehicleID)
		pipe.Expire(ctx, dayKey, 8*24*time.Hour)
		if _, err := pipe.Exec(ctx); err != nil {
			return err
		}
	}

	// Tell anyone watching station:{sid} at most once every 15 s.
	if ok, _ := s.rdb.SetNX(ctx, "station:"+sid+":crowdpub", "1", 15*time.Second).Result(); ok {
		return s.pub.Publish(ctx, "station:"+sid, "station.crowd", map[string]any{
			"stationId": sid,
			"carsNow":   carsNow.Val(),
			"level":     CrowdLevel(carsNow.Val()),
		})
	}
	return nil
}

// CrowdLevel turns a car count into something a driver can act on.
func CrowdLevel(carsNow int64) string {
	switch {
	case carsNow <= 2:
		return "empty"
	case carsNow <= 6:
		return "moderate"
	default:
		return "busy"
	}
}

// WeeklyVisitors = unique cars that really stopped here in the last 7 days.
// PFCOUNT over several HyperLogLogs returns the size of their UNION, so a car
// that fuels up 3 times a week counts once. ~0.8% error, 12 KB per key max.
func (s *StationTracker) WeeklyVisitors(ctx context.Context, sid string) (int64, error) {
	keys := make([]string, 7)
	for i := 0; i < 7; i++ {
		keys[i] = "station:" + sid + ":cars:" + time.Now().AddDate(0, 0, -i).Format("20060102")
	}
	return s.rdb.PFCount(ctx, keys...).Result()
}

// Stars converts visitor counts of the stations along a route into 1.0-5.0
// stars RELATIVE to each other (a highway pump and a village pump are only
// compared with their neighbours). log scale so one giant pump does not
// squash everybody else to 1 star.
func Stars(visitors map[string]int64) map[string]float64 {
	var maxV int64
	for _, v := range visitors {
		if v > maxV {
			maxV = v
		}
	}
	out := make(map[string]float64, len(visitors))
	for id, v := range visitors {
		if maxV == 0 {
			out[id] = 3.0 // no data yet: neutral
			continue
		}
		ratio := math.Log1p(float64(v)) / math.Log1p(float64(maxV))
		out[id] = math.Round((1+4*ratio)*10) / 10
	}
	return out
}

// NearRoute returns stations of a given type within radiusKm of a point.
// For "stations along my journey" call it for points sampled every ~5 km of
// the route polyline and merge the results (see the guide, Step 10).
func (s *StationTracker) NearRoute(ctx context.Context, lat, lng, radiusKm float64, limit int) ([]redis.GeoLocation, error) {
	return s.rdb.GeoSearchLocation(ctx, stationsGeoKey, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Latitude: lat, Longitude: lng,
			Radius: radiusKm, RadiusUnit: "km",
			Sort: "ASC", Count: limit,
		},
		WithDist:  true,
		WithCoord: true,
	}).Result()
}
