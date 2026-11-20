package luxury_bus

import (
	"context"
	"time"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
)

// Seats wraps the generic slot bitmap for a bus trip (sr:{lbus:seats:<trip>}).
type Seats struct{ slots *cache.Slots }

func NewSeats(s *cache.Slots) *Seats { return &Seats{slots: s} }

func (s *Seats) Map(ctx context.Context, t *Trip) ([]bool, error) {
	all := make([]int, t.Seats)
	for i := range all {
		all[i] = i
	}
	return s.slots.Free(ctx, cache.KeyLuxSeats(t.ID), all...)
}

// Pick returns the requested seats, or auto-picks n free seats (adjacent if possible).
func (s *Seats) Pick(ctx context.Context, t *Trip, wanted []int, n int) ([]int, error) {
	if len(wanted) > 0 {
		return wanted, nil
	}
	free, err := s.Map(ctx, t)
	if err != nil {
		return nil, err
	}
	for start := 0; start+n <= len(free); start++ { // first run of n adjacent seats
		ok := true
		for i := start; i < start+n; i++ {
			ok = ok && free[i]
		}
		if ok {
			out := make([]int, n)
			for i := range out {
				out[i] = start + i
			}
			return out, nil
		}
	}
	var out []int
	for i, f := range free {
		if f && len(out) < n {
			out = append(out, i)
		}
	}
	if len(out) < n {
		return nil, apperr.Conflict("not enough seats left")
	}
	return out, nil
}

func (s *Seats) Hold(ctx context.Context, t *Trip, owner string, seats []int) error {
	return s.slots.Hold(ctx, cache.KeyLuxSeats(t.ID), owner, t.Seats, cache.TTLSeatHold, seats...)
}

func (s *Seats) Confirm(ctx context.Context, t *Trip, owner string, seats []int) error {
	ttl := time.Until(t.Departure) + 48*time.Hour
	return s.slots.Confirm(ctx, cache.KeyLuxSeats(t.ID), owner, t.Seats, ttl, seats...)
}

func (s *Seats) Free(ctx context.Context, t *Trip, owner string, seats []int) {
	_ = s.slots.Release(ctx, cache.KeyLuxSeats(t.ID), owner, t.Seats, seats...)
	_ = s.slots.Cancel(ctx, cache.KeyLuxSeats(t.ID), owner, t.Seats, seats...)
}
