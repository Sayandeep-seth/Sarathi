package self_drive

import (
	"context"

	"sarathi/internal/cache"
)

const calendarDays = 3650 // day-index bitmap from 2024-01-01

// Calendar reserves whole days of a car atomically (reserve_seat.lua treats each
// day index as a "seat"), so two renters can never overlap.
type Calendar struct{ slots *cache.Slots }

func NewCalendar(s *cache.Slots) *Calendar { return &Calendar{slots: s} }

func (c *Calendar) Hold(ctx context.Context, carID, owner string, days []int) error {
	return c.slots.Hold(ctx, cache.KeySelfDriveCalendar(carID), owner, calendarDays, cache.TTLCalendarHold, days...)
}

func (c *Calendar) Confirm(ctx context.Context, carID, owner string, days []int) error {
	return c.slots.Confirm(ctx, cache.KeySelfDriveCalendar(carID), owner, calendarDays, 0, days...)
}

func (c *Calendar) Free(ctx context.Context, carID, owner string, days []int) {
	_ = c.slots.Release(ctx, cache.KeySelfDriveCalendar(carID), owner, calendarDays, days...)
	_ = c.slots.Cancel(ctx, cache.KeySelfDriveCalendar(carID), owner, calendarDays, days...)
}

func (c *Calendar) Available(ctx context.Context, carID string, days []int) (bool, error) {
	return c.slots.AllFree(ctx, cache.KeySelfDriveCalendar(carID), days...)
}
