// Package test_drive lets dealers earn from their demo cars: the same car serves
// customer test drives and works as a cab inside a fixed area around the showroom.
// A customer may test drive each model only once per showroom.
package test_drive

import (
	"context"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// Car is a dealer demo car.
type Car struct {
	VehicleID  string `json:"vehicle_id"`
	DealerID   string `json:"dealer_id"`
	ShowroomID string `json:"showroom_id"`
	Model      string `json:"model"`
	DriverID   string `json:"driver_id"` // dealer's driver when used as a cab
}

// Zone is the allowed operating area of a showroom.
type Zone struct {
	ShowroomID string      `json:"showroom_id"`
	DealerID   string      `json:"dealer_id"`
	Center     utils.Point `json:"center"`
	RadiusKm   float64     `json:"radius_km"`
}

// Cache:
//
//	sr:td:car:<vehicle>            HASH dealer, showroom, model, driver
//	sr:td:showroom:<id>:cars       SET  vehicle ids
//	sr:td:zone:<showroom>          HASH lat,lng,radius_km,dealer
type Cache struct{ rdb *redis.Client }

func NewCache(rdb *redis.Client) *Cache { return &Cache{rdb: rdb} }

func (c *Cache) PutCar(ctx context.Context, car Car) error {
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, cache.KeyTDCar(car.VehicleID), "dealer", car.DealerID, "showroom", car.ShowroomID, "model", car.Model, "driver", car.DriverID)
	pipe.SAdd(ctx, cache.KeyTDShowroomCars(car.ShowroomID), car.VehicleID)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Cache) Car(ctx context.Context, vehicleID string) (*Car, error) {
	m, err := c.rdb.HGetAll(ctx, cache.KeyTDCar(vehicleID)).Result()
	if err != nil || len(m) == 0 {
		return nil, err
	}
	return &Car{VehicleID: vehicleID, DealerID: m["dealer"], ShowroomID: m["showroom"], Model: m["model"], DriverID: m["driver"]}, nil
}

// CarsOfModel returns the showroom's demo cars of a model.
func (c *Cache) CarsOfModel(ctx context.Context, showroomID, model string) ([]Car, error) {
	ids, err := c.rdb.SMembers(ctx, cache.KeyTDShowroomCars(showroomID)).Result()
	if err != nil {
		return nil, err
	}
	var out []Car
	for _, id := range ids {
		if car, _ := c.Car(ctx, id); car != nil && utils.NormalizeName(car.Model) == utils.NormalizeName(model) {
			out = append(out, *car)
		}
	}
	return out, nil
}

func (c *Cache) SetZone(ctx context.Context, z Zone) error {
	if !z.Center.Valid() || z.RadiusKm <= 0 || z.RadiusKm > 30 {
		return apperr.BadRequest("zone needs a center and radius 0..30 km")
	}
	return c.rdb.HSet(ctx, cache.KeyTDZone(z.ShowroomID), "lat", z.Center.Lat, "lng", z.Center.Lng, "radius_km", z.RadiusKm, "dealer", z.DealerID).Err()
}

func (c *Cache) Zone(ctx context.Context, showroomID string) (*Zone, error) {
	m, err := c.rdb.HGetAll(ctx, cache.KeyTDZone(showroomID)).Result()
	if err != nil || len(m) == 0 {
		return nil, err
	}
	return &Zone{ShowroomID: showroomID, DealerID: m["dealer"], RadiusKm: cache.HGetFloat(m, "radius_km"),
		Center: utils.Point{Lat: cache.HGetFloat(m, "lat"), Lng: cache.HGetFloat(m, "lng")}}, nil
}
