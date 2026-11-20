package luxury_bus

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// Stop is a restaurant halt (the boarding point can be stop 0 like a train pantry).
type Stop struct {
	ID           string      `json:"id"`
	RestaurantID string      `json:"restaurant_id"`
	Name         string      `json:"name"`
	Location     utils.Point `json:"location"`
	ETA          time.Time   `json:"eta"`
	HaltMin      int         `json:"halt_min"`
	IsOrigin     bool        `json:"is_origin"`
}

// MenuItem is a dish a restaurant offers.
type MenuItem struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Veg   bool    `json:"veg"`
}

// OrderCutoff: orders for a stop close 45 min before the bus reaches it.
const OrderCutoff = 45 * time.Minute

func (s *Store) PutStops(ctx context.Context, tripID string, stops []Stop) error {
	pipe := s.rdb.TxPipeline()
	for _, st := range stops {
		pipe.ZAdd(ctx, cache.KeyLuxStops(tripID), redis.Z{Score: float64(st.ETA.Unix()), Member: st.ID})
		k := cache.KeyLuxStop(tripID, st.ID)
		pipe.HSet(ctx, k, map[string]any{"restaurant": st.RestaurantID, "name": st.Name, "lat": st.Location.Lat,
			"lng": st.Location.Lng, "eta": st.ETA.Unix(), "halt": st.HaltMin, "origin": st.IsOrigin})
		pipe.Expire(ctx, k, cache.TTLLuxTrip)
		pipe.ZAdd(ctx, cache.KeyLuxDispatching(), redis.Z{Score: float64(st.ETA.Unix()), Member: tripID + "|" + st.ID})
	}
	pipe.Expire(ctx, cache.KeyLuxStops(tripID), cache.TTLLuxTrip)
	_, err := pipe.Exec(ctx)
	return err
}

// Stops returns the trip's restaurant stops in route order.
func (s *Store) Stops(ctx context.Context, tripID string) ([]Stop, error) {
	ids, err := s.rdb.ZRange(ctx, cache.KeyLuxStops(tripID), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Stop, 0, len(ids))
	for _, id := range ids {
		st, err := s.Stop(ctx, tripID, id)
		if err == nil {
			out = append(out, *st)
		}
	}
	return out, nil
}

func (s *Store) Stop(ctx context.Context, tripID, stopID string) (*Stop, error) {
	m, err := s.rdb.HGetAll(ctx, cache.KeyLuxStop(tripID, stopID)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, apperr.NotFound("stop not found")
	}
	origin, _ := strconv.ParseBool(m["origin"])
	return &Stop{ID: stopID, RestaurantID: m["restaurant"], Name: m["name"], IsOrigin: origin,
		Location: utils.Point{Lat: cache.HGetFloat(m, "lat"), Lng: cache.HGetFloat(m, "lng")},
		ETA:      time.Unix(cache.HGetInt(m, "eta"), 0), HaltMin: int(cache.HGetInt(m, "halt"))}, nil
}

// UpdateETA shifts a stop when the bus is late (fed from GPS).
func (s *Store) UpdateETA(ctx context.Context, tripID, stopID string, eta time.Time) error {
	pipe := s.rdb.TxPipeline()
	pipe.ZAdd(ctx, cache.KeyLuxStops(tripID), redis.Z{Score: float64(eta.Unix()), Member: stopID})
	pipe.HSet(ctx, cache.KeyLuxStop(tripID, stopID), "eta", eta.Unix())
	pipe.ZAdd(ctx, cache.KeyLuxDispatching(), redis.Z{Score: float64(eta.Unix()), Member: tripID + "|" + stopID})
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) PutMenu(ctx context.Context, restaurantID string, items []MenuItem) error {
	vals := make(map[string]any, len(items))
	for _, it := range items {
		b, _ := json.Marshal(it)
		vals[it.ID] = string(b)
	}
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, cache.KeyLuxMenu(restaurantID))
	pipe.HSet(ctx, cache.KeyLuxMenu(restaurantID), vals)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Menu(ctx context.Context, restaurantID string) (map[string]MenuItem, error) {
	m, err := s.rdb.HGetAll(ctx, cache.KeyLuxMenu(restaurantID)).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[string]MenuItem, len(m))
	for id, raw := range m {
		var it MenuItem
		if json.Unmarshal([]byte(raw), &it) == nil {
			out[id] = it
		}
	}
	return out, nil
}
