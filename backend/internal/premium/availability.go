package premium

import (
	"context"
	"time"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// Availability: one 24-bit bitmap per car per day; bit h = hour h is booked.
// Holds/confirmations go through reserve_seat.lua so two users can never get the
// same car for overlapping hours.
//
//	sr:{premium:cal:<car>:<yyyymmdd>}            BITMAP 24 slots
//	sr:{premium:cal:<car>:<yyyymmdd>}:hold:<h>   STRING owner (15 min)
type Availability struct{ slots *cache.Slots }

func NewAvailability(s *cache.Slots) *Availability { return &Availability{slots: s} }

const hoursPerDay = 24

// span splits [start, start+hours) into per-day hour lists (bookings can cross midnight).
func span(start time.Time, hours int) map[string][]int {
	out := map[string][]int{}
	t := start.In(utils.IST).Truncate(time.Hour)
	for i := 0; i < hours; i++ {
		h := t.Add(time.Duration(i) * time.Hour)
		d := utils.DateKey(h)
		out[d] = append(out[d], h.Hour())
	}
	return out
}

// Hold locks the hours for owner; all-or-nothing across days.
func (a *Availability) Hold(ctx context.Context, carID, owner string, start time.Time, hours int) error {
	var done []string
	for day, hs := range span(start, hours) {
		if err := a.slots.Hold(ctx, cache.KeyPremiumCalendar(carID, day), owner, hoursPerDay, cache.TTLCalendarHold, hs...); err != nil {
			for _, d := range done {
				_ = a.slots.Release(ctx, cache.KeyPremiumCalendar(carID, d), owner, hoursPerDay, span(start, hours)[d]...)
			}
			return err
		}
		done = append(done, day)
	}
	return nil
}

func (a *Availability) Confirm(ctx context.Context, carID, owner string, start time.Time, hours int) error {
	for day, hs := range span(start, hours) {
		if err := a.slots.Confirm(ctx, cache.KeyPremiumCalendar(carID, day), owner, hoursPerDay, 60*24*time.Hour, hs...); err != nil {
			return err
		}
	}
	return nil
}

func (a *Availability) Release(ctx context.Context, carID, owner string, start time.Time, hours int) {
	for day, hs := range span(start, hours) {
		_ = a.slots.Release(ctx, cache.KeyPremiumCalendar(carID, day), owner, hoursPerDay, hs...)
	}
}

func (a *Availability) Cancel(ctx context.Context, carID, owner string, start time.Time, hours int) {
	for day, hs := range span(start, hours) {
		_ = a.slots.Cancel(ctx, cache.KeyPremiumCalendar(carID, day), owner, hoursPerDay, hs...)
	}
}

// FreeHours returns the bookable hours of a car on a date.
func (a *Availability) FreeHours(ctx context.Context, carID string, date time.Time) ([]int, error) {
	all := make([]int, hoursPerDay)
	for i := range all {
		all[i] = i
	}
	free, err := a.slots.Free(ctx, cache.KeyPremiumCalendar(carID, utils.DateKey(date)), all...)
	if err != nil {
		return nil, err
	}
	var out []int
	for h, f := range free {
		if f {
			out = append(out, h)
		}
	}
	return out, nil
}
