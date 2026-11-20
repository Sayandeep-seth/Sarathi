package daily_bus

import (
	"context"
	"time"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
)

// Seats manages per-run seat bitmaps: sr:{dbus:seats:<route>:<yyyymmdd>}.
// Capacity = number of seats in the bus, so a run can never be over-sold and
// nobody has to stand.
type Seats struct{ slots *cache.Slots }

func NewSeats(s *cache.Slots) *Seats { return &Seats{slots: s} }

func key(routeID, date string) string { return cache.KeyDailyBusSeats(routeID, date) }

// Map returns free(true)/taken(false) for every seat of a run.
func (s *Seats) Map(ctx context.Context, routeID, date string, capacity int) ([]bool, error) {
	all := make([]int, capacity)
	for i := range all {
		all[i] = i
	}
	return s.slots.Free(ctx, key(routeID, date), all...)
}

// PickSameSeat finds one seat number free on ALL dates (monthly pass keeps the same seat).
func (s *Seats) PickSameSeat(ctx context.Context, routeID string, dates []string, capacity int) (int, error) {
	free := make([]bool, capacity)
	for i := range free {
		free[i] = true
	}
	for _, d := range dates {
		m, err := s.Map(ctx, routeID, d, capacity)
		if err != nil {
			return -1, err
		}
		for i, f := range m {
			free[i] = free[i] && f
		}
	}
	for i, f := range free {
		if f {
			return i, nil
		}
	}
	return -1, apperr.Conflict("bus is full on one of the selected days")
}

// HoldAll holds `seat` on every date (all-or-nothing).
func (s *Seats) HoldAll(ctx context.Context, routeID, owner string, seat int, dates []string, capacity int) error {
	for i, d := range dates {
		if err := s.slots.Hold(ctx, key(routeID, d), owner, capacity, cache.TTLSeatHold, seat); err != nil {
			for _, prev := range dates[:i] {
				_ = s.slots.Release(ctx, key(routeID, prev), owner, capacity, seat)
			}
			return err
		}
	}
	return nil
}

func (s *Seats) ConfirmAll(ctx context.Context, routeID, owner string, seat int, dates []string, capacity int) error {
	for _, d := range dates {
		// keep the run's bitmap until a few days after the last ride of a monthly pass
		if err := s.slots.Confirm(ctx, key(routeID, d), owner, capacity, cache.TTLDailyBusSeats+31*24*time.Hour, seat); err != nil {
			return err
		}
	}
	return nil
}

func (s *Seats) FreeAll(ctx context.Context, routeID, owner string, seat int, dates []string, capacity int) {
	for _, d := range dates {
		_ = s.slots.Release(ctx, key(routeID, d), owner, capacity, seat)
		_ = s.slots.Cancel(ctx, key(routeID, d), owner, capacity, seat)
	}
}
