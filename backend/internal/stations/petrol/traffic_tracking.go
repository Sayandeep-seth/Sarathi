package petrol

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/constants"
	"sarathi/internal/common/utils"
)

// TrafficTracker counts cars seen at each station using GPS geofence events.
//
//	sr:station:seen:<vehicle>:<station>       STRING  30 min de-dup (one visit per stop)
//	sr:station:petrol:<id>:visits:<yyyymmdd>  HLL     distinct vehicles per day (35 days)
//	sr:station:petrol:<id>:live               ZSET    vehicle -> unix ts (cars there right now)
//	sr:station:petrol:<id>:hourly:<yyyymmddhh> STRING visits per hour (8 days)
//	sr:stats:station:petrol:<yyyymmdd>        ZSET    station -> visits (analytics leaderboard)
type TrafficTracker struct{ rdb *redis.Client }

func NewTrafficTracker(rdb *redis.Client) *TrafficTracker { return &TrafficTracker{rdb: rdb} }

// RecordVisit registers that a vehicle stopped at a station. Returns false for a repeat ping.
func (t *TrafficTracker) RecordVisit(ctx context.Context, stationID, vehicleID string, at time.Time) (bool, error) {
	fresh, err := t.rdb.SetNX(ctx, cache.KeyStationVisitDedup(vehicleID, stationID), "1", cache.TTLVisitDedup).Result()
	if err != nil || !fresh {
		return false, err
	}
	day, hour := utils.DateKey(at), utils.HourKey(at)
	visits := cache.KeyStationVisits(stationID, day)
	live := cache.KeyStationLive(stationID)
	hourly := cache.KeyStationHourly(stationID, hour)
	stats := cache.KeyStatStationVisits(constants.StationPetrol, day)

	pipe := t.rdb.Pipeline()
	pipe.PFAdd(ctx, visits, vehicleID)
	pipe.Expire(ctx, visits, cache.TTLStationVisits)
	pipe.ZAdd(ctx, live, redis.Z{Score: float64(at.Unix()), Member: vehicleID})
	pipe.ZRemRangeByScore(ctx, live, "-inf", strconv.FormatInt(at.Add(-cache.TTLStationLiveWindow).Unix(), 10))
	pipe.Expire(ctx, live, 2*cache.TTLStationLiveWindow)
	pipe.Incr(ctx, hourly)
	pipe.Expire(ctx, hourly, cache.TTLStationHourly)
	pipe.ZIncrBy(ctx, stats, 1, stationID)
	pipe.Expire(ctx, stats, cache.TTLStats)
	_, err = pipe.Exec(ctx)
	return err == nil, err
}

// CarsNow returns how many cars were seen at each station in the last 15 minutes.
func (t *TrafficTracker) CarsNow(ctx context.Context, ids []string) (map[string]int64, error) {
	min := strconv.FormatInt(time.Now().Add(-cache.TTLStationLiveWindow).Unix(), 10)
	pipe := t.rdb.Pipeline()
	cmds := make([]*redis.IntCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.ZCount(ctx, cache.KeyStationLive(id), min, "+inf")
	}
	if _, err := pipe.Exec(ctx); err != nil && !cache.IsNil(err) {
		return nil, err
	}
	out := make(map[string]int64, len(ids))
	for i, id := range ids {
		out[id] = cmds[i].Val()
	}
	return out, nil
}

// UniqueVisits returns distinct cars over the last `days` days (PFCOUNT merges HLLs).
func (t *TrafficTracker) UniqueVisits(ctx context.Context, ids []string, days int, now time.Time) (map[string]int64, error) {
	pipe := t.rdb.Pipeline()
	cmds := make([]*redis.IntCmd, len(ids))
	for i, id := range ids {
		keys := make([]string, days)
		for d := 0; d < days; d++ {
			keys[d] = cache.KeyStationVisits(id, utils.DateKey(now.AddDate(0, 0, -d)))
		}
		cmds[i] = pipe.PFCount(ctx, keys...)
	}
	if _, err := pipe.Exec(ctx); err != nil && !cache.IsNil(err) {
		return nil, err
	}
	out := make(map[string]int64, len(ids))
	for i, id := range ids {
		out[id] = cmds[i].Val()
	}
	return out, nil
}

// Hourly returns visit counts for the last n hours (busy-hours chart).
func (t *TrafficTracker) Hourly(ctx context.Context, id string, hours int, now time.Time) ([]int64, error) {
	keys := make([]string, hours)
	for h := 0; h < hours; h++ {
		keys[hours-1-h] = cache.KeyStationHourly(id, utils.HourKey(now.Add(-time.Duration(h)*time.Hour)))
	}
	vals, err := t.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	out := make([]int64, hours)
	for i, v := range vals {
		if s, ok := v.(string); ok {
			out[i], _ = strconv.ParseInt(s, 10, 64)
		}
	}
	return out, nil
}
