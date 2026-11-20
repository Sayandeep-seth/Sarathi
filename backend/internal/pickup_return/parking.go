// Package pickup_return books a cab that goes to a hub (airport, railway station,
// metro), picks someone up and returns to the origin. Parking is charged; only
// the seat count (5 or 7) can be chosen, not the model.
package pickup_return

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
	"sarathi/internal/pricing"
)

// Hub is a pickup place with a parking tariff.
type Hub struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     string      `json:"type"` // airport | railway | metro | bus_stand
	Location utils.Point `json:"location"`
}

// Parking resolves hubs and their parking tariffs.
//
//	sr:geo:hubs                 GEO  hub positions (detect that the destination is a hub)
//	sr:hub:<id>                 HASH name,type
//	sr:parking:tariff:<hubID>   HASH free_minutes,base_fee,per_hour,max_fee (6 h)
type Parking struct {
	rdb *redis.Client
	geo *cache.Geo
	db  *pgxpool.Pool
}

func NewParking(rdb *redis.Client, geo *cache.Geo, db *pgxpool.Pool) *Parking {
	return &Parking{rdb: rdb, geo: geo, db: db}
}

// HubAt returns the hub within 800 m of a point (the pickup destination).
func (p *Parking) HubAt(ctx context.Context, pt utils.Point) (*Hub, error) {
	hits, err := p.geo.Nearby(ctx, cache.KeyHubsGeo(), pt, 0.8, 1)
	if err != nil || len(hits) == 0 {
		return nil, err
	}
	m, err := p.rdb.HGetAll(ctx, cache.KeyHub(hits[0].Member)).Result()
	if err != nil {
		return nil, err
	}
	return &Hub{ID: hits[0].Member, Name: m["name"], Type: m["type"], Location: hits[0].Point}, nil
}

// Tariff returns the parking tariff (Redis -> Postgres -> safe default).
func (p *Parking) Tariff(ctx context.Context, hubID string) (pricing.ParkingTariff, error) {
	k := cache.KeyParkingTariff(hubID)
	m, err := p.rdb.HGetAll(ctx, k).Result()
	if err == nil && len(m) > 0 {
		return pricing.ParkingTariff{HubID: hubID, Name: m["name"], FreeMinutes: cache.HGetFloat(m, "free_minutes"),
			BaseFee: cache.HGetFloat(m, "base_fee"), PerHour: cache.HGetFloat(m, "per_hour"), MaxFee: cache.HGetFloat(m, "max_fee")}, nil
	}
	t := pricing.ParkingTariff{HubID: hubID, FreeMinutes: 10, BaseFee: 90, PerHour: 60, MaxFee: 600}
	_ = p.db.QueryRow(ctx, `SELECT name, free_minutes, base_fee, per_hour, max_fee FROM parking_tariffs WHERE hub_id=$1`, hubID).
		Scan(&t.Name, &t.FreeMinutes, &t.BaseFee, &t.PerHour, &t.MaxFee)
	pipe := p.rdb.Pipeline()
	pipe.HSet(ctx, k, "name", t.Name, "free_minutes", t.FreeMinutes, "base_fee", t.BaseFee, "per_hour", t.PerHour, "max_fee", t.MaxFee)
	pipe.Expire(ctx, k, cache.TTLParkingTariff)
	_, _ = pipe.Exec(ctx)
	return t, nil
}

// UpsertHub registers a hub (admin).
func (p *Parking) UpsertHub(ctx context.Context, h Hub) error {
	pipe := p.rdb.Pipeline()
	cache.AddPipe(ctx, pipe, cache.KeyHubsGeo(), h.ID, h.Location)
	pipe.HSet(ctx, cache.KeyHub(h.ID), "name", h.Name, "type", h.Type)
	_, err := pipe.Exec(ctx)
	return err
}
