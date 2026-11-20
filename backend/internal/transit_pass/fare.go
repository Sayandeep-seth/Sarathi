package transit_pass

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/constants"
)

// Fares looks up operator fares per leg.
//
//	sr:transit:fare:<mode>:<from>:<to>  STRING fare (24 h), loaded from transit_fares
type Fares struct {
	rdb *redis.Client
	db  *pgxpool.Pool
}

func NewFares(rdb *redis.Client, db *pgxpool.Pool) *Fares { return &Fares{rdb: rdb, db: db} }

// flat fallbacks when an operator has not published a fare for a pair
var fallbackFare = map[string]float64{
	constants.ModeTrain: 15, constants.ModeMetro: 40, constants.ModeBus: 25, constants.ModeCab: 150,
}

func (f *Fares) LegFare(ctx context.Context, l LegSpec) float64 {
	k := cache.KeyTransitFare(l.Mode, l.From, l.To)
	if v, err := f.rdb.Get(ctx, k).Float64(); err == nil {
		return v
	}
	fare := fallbackFare[l.Mode]
	_ = f.db.QueryRow(ctx, `SELECT fare FROM transit_fares WHERE mode=$1 AND from_code=$2 AND to_code=$3`, l.Mode, l.From, l.To).Scan(&fare)
	f.rdb.Set(ctx, k, fare, cache.TTLFare)
	return fare
}

// Price fills fares for every leg and returns the total.
func (f *Fares) Price(ctx context.Context, specs []LegSpec) ([]Leg, float64) {
	legs := make([]Leg, len(specs))
	total := 0.0
	for i, s := range specs {
		legs[i] = Leg{LegSpec: s, Fare: f.LegFare(ctx, s)}
		total += legs[i].Fare
	}
	return legs, total
}
