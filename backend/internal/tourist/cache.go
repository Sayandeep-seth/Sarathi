// Package tourist books a car + driver for trips to tourist places, staying
// there for days; charged per day plus km beyond the daily allowance.
package tourist

import (
	"context"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
)

// Car is a tourist fleet vehicle (always with driver).
type Car struct {
	ID       string `json:"id"`
	Model    string `json:"model"`
	Class    string `json:"class"` // sedan | suv | tempo_traveller
	Seats    int    `json:"seats"`
	City     string `json:"city"`
	DriverID string `json:"driver_id"`
}

// calendarDays covers ~10 years of day indexes from 2024-01-01.
const calendarDays = 3650

// Cache:
//
//	sr:tourist:car:<id>           HASH model,class,seats,city,driver
//	sr:tourist:fleet:<class>      SET  car ids
//	sr:{tourist:cal:<car>}        BITMAP day-index -> booked (reserve_seat.lua)
//	sr:tourist:rates:<class>      HASH per_day,km_per_day,extra_km,driver_allowance,night_charge
//	sr:tourist:itinerary:<dest>   STRING json popular itinerary (12 h)
type Cache struct {
	rdb   *redis.Client
	slots *cache.Slots
}

func NewCache(rdb *redis.Client, s *cache.Slots) *Cache { return &Cache{rdb: rdb, slots: s} }

func (c *Cache) PutCar(ctx context.Context, car Car) error {
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, cache.KeyTouristCar(car.ID), "model", car.Model, "class", car.Class, "seats", car.Seats, "city", car.City, "driver", car.DriverID)
	pipe.SAdd(ctx, cache.KeyTouristFleet(car.Class), car.ID)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Cache) Car(ctx context.Context, id string) (*Car, error) {
	m, err := c.rdb.HGetAll(ctx, cache.KeyTouristCar(id)).Result()
	if err != nil || len(m) == 0 {
		return nil, err
	}
	return &Car{ID: id, Model: m["model"], Class: m["class"], Seats: int(cache.HGetInt(m, "seats")), City: m["city"], DriverID: m["driver"]}, nil
}

func (c *Cache) FleetIDs(ctx context.Context, class string) ([]string, error) {
	return c.rdb.SMembers(ctx, cache.KeyTouristFleet(class)).Result()
}

// HoldDays temporarily reserves the car for all days (all-or-nothing).
func (c *Cache) HoldDays(ctx context.Context, carID, owner string, days []int) error {
	return c.slots.Hold(ctx, cache.KeyTouristCalendar(carID), owner, calendarDays, cache.TTLCalendarHold, days...)
}

func (c *Cache) ConfirmDays(ctx context.Context, carID, owner string, days []int) error {
	return c.slots.Confirm(ctx, cache.KeyTouristCalendar(carID), owner, calendarDays, 0, days...)
}

func (c *Cache) ReleaseDays(ctx context.Context, carID, owner string, days []int) {
	_ = c.slots.Release(ctx, cache.KeyTouristCalendar(carID), owner, calendarDays, days...)
	_ = c.slots.Cancel(ctx, cache.KeyTouristCalendar(carID), owner, calendarDays, days...)
}

func (c *Cache) DaysFree(ctx context.Context, carID string, days []int) (bool, error) {
	return c.slots.AllFree(ctx, cache.KeyTouristCalendar(carID), days...)
}

// Itinerary templates.
func (c *Cache) SetItinerary(ctx context.Context, dest string, raw []byte) error {
	return c.rdb.Set(ctx, cache.KeyTouristItinerary(dest), raw, cache.TTLItinerary).Err()
}

func (c *Cache) Itinerary(ctx context.Context, dest string) ([]byte, error) {
	b, err := c.rdb.Get(ctx, cache.KeyTouristItinerary(dest)).Bytes()
	if cache.IsNil(err) {
		return nil, nil
	}
	return b, err
}
