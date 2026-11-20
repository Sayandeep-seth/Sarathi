// Package premium books premium and vintage cars for events and ceremonies.
// The passenger picks the exact model; the fare is charged per hour.
package premium

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
)

const (
	CategoryPremium = "premium"
	CategoryVintage = "vintage"
)

// Car is a chauffeur-driven premium or vintage car.
type Car struct {
	ID          string  `json:"id"`
	Model       string  `json:"model"` // e.g. "Mercedes-Benz S-Class", "1957 Chevrolet Bel Air"
	Category    string  `json:"category"`
	City        string  `json:"city"`
	Year        int     `json:"year"`
	Seats       int     `json:"seats"`
	Color       string  `json:"color"`
	RatePerHour float64 `json:"rate_per_hour"`
}

// Cache is the fleet catalogue.
//
//	sr:premium:car:<id>          HASH model,category,city,year,seats,color,rate (24 h)
//	sr:premium:fleet:<category>  SET  car ids
type Cache struct {
	rdb *redis.Client
	db  *pgxpool.Pool
}

func NewCache(rdb *redis.Client, db *pgxpool.Pool) *Cache { return &Cache{rdb: rdb, db: db} }

func (c *Cache) Put(ctx context.Context, cars ...Car) error {
	pipe := c.rdb.Pipeline()
	for _, car := range cars {
		k := cache.KeyPremiumCar(car.ID)
		pipe.HSet(ctx, k, map[string]any{"model": car.Model, "category": car.Category, "city": car.City,
			"year": car.Year, "seats": car.Seats, "color": car.Color, "rate": car.RatePerHour})
		pipe.Expire(ctx, k, cache.TTLFleetCar)
		pipe.SAdd(ctx, cache.KeyPremiumFleet(car.Category), car.ID)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Cache) Car(ctx context.Context, id string) (*Car, error) {
	m, err := c.rdb.HGetAll(ctx, cache.KeyPremiumCar(id)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		var car Car
		err := c.db.QueryRow(ctx, `SELECT id, model, category, city, year, seats, color, rate_per_hour FROM premium_cars WHERE id=$1 AND active`, id).
			Scan(&car.ID, &car.Model, &car.Category, &car.City, &car.Year, &car.Seats, &car.Color, &car.RatePerHour)
		if err != nil {
			return nil, apperr.NotFound("car not found")
		}
		_ = c.Put(ctx, car)
		return &car, nil
	}
	return fromHash(id, m), nil
}

// Fleet lists cars of a category (optionally one city).
func (c *Cache) Fleet(ctx context.Context, category, city string) ([]Car, error) {
	ids, err := c.rdb.SMembers(ctx, cache.KeyPremiumFleet(category)).Result()
	if err != nil {
		return nil, err
	}
	pipe := c.rdb.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.HGetAll(ctx, cache.KeyPremiumCar(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && !cache.IsNil(err) {
		return nil, err
	}
	out := make([]Car, 0, len(ids))
	for i, id := range ids {
		if m := cmds[i].Val(); len(m) > 0 && (city == "" || m["city"] == city) {
			out = append(out, *fromHash(id, m))
		}
	}
	return out, nil
}

// WarmUp loads the fleet from Postgres.
func (c *Cache) WarmUp(ctx context.Context) error {
	rows, err := c.db.Query(ctx, `SELECT id, model, category, city, year, seats, color, rate_per_hour FROM premium_cars WHERE active`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var cars []Car
	for rows.Next() {
		var car Car
		if err := rows.Scan(&car.ID, &car.Model, &car.Category, &car.City, &car.Year, &car.Seats, &car.Color, &car.RatePerHour); err != nil {
			return err
		}
		cars = append(cars, car)
	}
	if len(cars) == 0 {
		return rows.Err()
	}
	return c.Put(ctx, cars...)
}

func fromHash(id string, m map[string]string) *Car {
	return &Car{ID: id, Model: m["model"], Category: m["category"], City: m["city"], Color: m["color"],
		Year: int(cache.HGetInt(m, "year")), Seats: int(cache.HGetInt(m, "seats")), RatePerHour: cache.HGetFloat(m, "rate")}
}
