// Package luxury_bus sells long-distance luxury bus tickets with train-style meal
// ordering: passengers pick food from restaurants on the route, the bus halts to
// collect it, and the bus owner earns a fixed commission on every order.
package luxury_bus

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
)

// Trip is one scheduled run.
type Trip struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	BusName     string    `json:"bus_name"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Departure   time.Time `json:"departure"`
	Seats       int       `json:"seats"`
	FarePerSeat float64   `json:"fare_per_seat"`
}

// Store:
//
//	sr:lbus:trip:<id>                 HASH trip (7 days)
//	sr:{lbus:seats:<trip>}            BITMAP seats (reserve_seat.lua)
//	sr:lbus:trip:<id>:stops           ZSET  stop -> ETA unix (restaurants in order)
//	sr:lbus:trip:<id>:stop:<stop>     HASH  restaurant, name, eta, halt_min, dispatched
//	sr:lbus:menu:<restaurant>         HASH  item -> json
//	sr:lbus:food:<trip>:<stop>        HASH  item -> total qty (what the restaurant prepares)
//	sr:lbus:ticket:<pnr>              HASH  ticket
//	sr:lbus:ticket:<pnr>:food         LIST  order ids
//	sr:lbus:order:<id>                HASH  order
//	sr:lbus:commission:<owner>:<mon>  HASH  food, food_count
type Store struct{ rdb *redis.Client }

func NewStore(rdb *redis.Client) *Store { return &Store{rdb: rdb} }

func (s *Store) PutTrip(ctx context.Context, t Trip) error {
	k := cache.KeyLuxTrip(t.ID)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, k, map[string]any{"owner": t.OwnerID, "bus": t.BusName, "from": t.From, "to": t.To,
		"departure": t.Departure.Unix(), "seats": t.Seats, "fare": t.FarePerSeat})
	pipe.Expire(ctx, k, cache.TTLLuxTrip)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Trip(ctx context.Context, id string) (*Trip, error) {
	m, err := s.rdb.HGetAll(ctx, cache.KeyLuxTrip(id)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, apperr.NotFound("bus trip not found")
	}
	return &Trip{ID: id, OwnerID: m["owner"], BusName: m["bus"], From: m["from"], To: m["to"],
		Departure: time.Unix(cache.HGetInt(m, "departure"), 0), Seats: int(cache.HGetInt(m, "seats")),
		FarePerSeat: cache.HGetFloat(m, "fare")}, nil
}
