package self_drive

import (
	"context"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// Car is a self-drive rental car.
type Car struct {
	ID           string      `json:"id"`
	Model        string      `json:"model"`
	PriceINR     float64     `json:"price_inr"` // ex-showroom -> band
	BandID       string      `json:"band_id"`
	City         string      `json:"city"`
	Location     utils.Point `json:"location"` // parking hub
	Seats        int         `json:"seats"`
	FuelType     string      `json:"fuel_type"`
	Transmission string      `json:"transmission"`
}

// Cache:
//
//	sr:selfdrive:bands              STRING json band table
//	sr:selfdrive:car:<id>           HASH  car
//	sr:selfdrive:band:<band>:cars   SET   car ids in a band (same price for all of them)
//	sr:geo:selfdrive                GEO   car parking positions
//	sr:{selfdrive:cal:<car>}        BITMAP day-index -> rented (reserve_seat.lua)
//	sr:selfdrive:deposit:<booking>  HASH  deposit status
type Cache struct {
	rdb   *redis.Client
	cs    *cache.Service
	geo   *cache.Geo
	slots *cache.Slots
}

func NewCache(cs *cache.Service, geo *cache.Geo, slots *cache.Slots) *Cache {
	return &Cache{rdb: cs.Client(), cs: cs, geo: geo, slots: slots}
}

func (c *Cache) PutCar(ctx context.Context, car Car) error {
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, cache.KeySelfDriveCar(car.ID), map[string]any{
		"model": car.Model, "price": car.PriceINR, "band": car.BandID, "city": car.City,
		"lat": car.Location.Lat, "lng": car.Location.Lng, "seats": car.Seats, "fuel": car.FuelType, "gear": car.Transmission,
	})
	pipe.SAdd(ctx, cache.KeySelfDriveBandCars(car.BandID), car.ID)
	if car.Location.Valid() {
		cache.AddPipe(ctx, pipe, cache.KeySelfDriveGeo(), car.ID, car.Location)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Cache) Car(ctx context.Context, id string) (*Car, error) {
	m, err := c.rdb.HGetAll(ctx, cache.KeySelfDriveCar(id)).Result()
	if err != nil || len(m) == 0 {
		return nil, err
	}
	return &Car{
		ID: id, Model: m["model"], PriceINR: cache.HGetFloat(m, "price"), BandID: m["band"], City: m["city"],
		Location: utils.Point{Lat: cache.HGetFloat(m, "lat"), Lng: cache.HGetFloat(m, "lng")},
		Seats:    int(cache.HGetInt(m, "seats")), FuelType: m["fuel"], Transmission: m["gear"],
	}, nil
}

func (c *Cache) BandCarIDs(ctx context.Context, band string) ([]string, error) {
	return c.rdb.SMembers(ctx, cache.KeySelfDriveBandCars(band)).Result()
}

func (c *Cache) NearbyIDs(ctx context.Context, p utils.Point, radiusKm float64) ([]cache.GeoHit, error) {
	return c.geo.Nearby(ctx, cache.KeySelfDriveGeo(), p, radiusKm, 100)
}
