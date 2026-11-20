package dispatch

import (
	"context"
	"errors"
	"strconv"
	"time"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/websocket"
)

const maxAttempts = 8

// OfferPayload is what the driver app shows.
type OfferPayload struct {
	BookingID string    `json:"booking_id"`
	Type      string    `json:"type"`
	Pickup    any       `json:"pickup"`
	Drop      any       `json:"drop"`
	Fare      float64   `json:"fare"`
	DistKm    float64   `json:"distance_to_pickup_km"`
	ExpiresAt time.Time `json:"expires_at"`
}

// offerNext locks the best free driver and sends him the offer. If nobody is
// left (or too many attempts), the booking listener is told there is no driver.
func (s *Service) offerNext(ctx context.Context, bookingID string) error {
	req, state, err := s.store.Request(ctx, bookingID)
	if err != nil {
		return err
	}
	if req == nil {
		return apperr.NotFound("dispatch request expired")
	}
	if n, _ := strconv.Atoi(state["attempts"]); n >= maxAttempts {
		return s.giveUp(ctx, bookingID)
	}
	rejected, err := s.store.Rejected(ctx, bookingID)
	if err != nil {
		return err
	}
	cands, err := s.finder.Find(ctx, req.Class, req.Pickup, rejected, req.MaxRadiusKm)
	if err != nil {
		return err
	}
	if f, ok := s.filters[req.Class]; ok {
		kept := cands[:0]
		for _, c := range cands {
			if f.Allow(ctx, c.DriverID, *req) {
				kept = append(kept, c)
			}
		}
		cands = kept
	}
	cands = Rank(cands, func(id string) float64 { return s.stats.AcceptanceRate(ctx, id) })

	for _, c := range cands {
		lk, err := s.locker.Acquire(ctx, cache.KeyDriverLockName(c.DriverID), cache.TTLDriverLock)
		if errors.Is(err, cache.ErrLockNotAcquired) {
			continue // another booking is offering this driver right now
		}
		if err != nil {
			return err
		}
		expires := time.Now().Add(cache.TTLOffer)
		if err := s.store.SetOffer(ctx, bookingID, c.DriverID, lk.Token, expires); err != nil {
			_ = lk.Release(ctx)
			return err
		}
		_ = s.pub.Driver(ctx, c.DriverID, websocket.EventOffer, OfferPayload{
			BookingID: bookingID, Type: req.Type, Pickup: req.Pickup, Drop: req.Drop,
			Fare: req.Fare, DistKm: c.DistKm, ExpiresAt: expires,
		})
		s.log.Debug("ride offered", "booking", bookingID, "driver", c.DriverID, "dist_km", c.DistKm)
		return nil
	}
	return s.giveUp(ctx, bookingID)
}

func (s *Service) giveUp(ctx context.Context, bookingID string) error {
	_ = s.store.Delete(ctx, bookingID)
	if s.listener != nil {
		return s.listener.OnNoDriver(ctx, bookingID)
	}
	return nil
}

// Accept is called by the driver app.
func (s *Service) Accept(ctx context.Context, driverID, bookingID string) error {
	_, state, err := s.store.Request(ctx, bookingID)
	if err != nil {
		return err
	}
	if state == nil {
		return apperr.NotFound("offer no longer exists")
	}
	res, err := s.store.Accept(ctx, bookingID, driverID)
	if err != nil {
		return err
	}
	switch res {
	case 0:
		return apperr.Conflict("this offer is not yours")
	case -1:
		return apperr.Conflict("offer expired")
	}
	s.stats.RecordOffer(ctx, driverID, true)
	if err := s.drivers.MarkOnTrip(ctx, driverID); err != nil {
		s.log.Warn("mark on trip", "driver", driverID, "err", err)
	}
	_ = s.locker.ReleaseToken(ctx, cache.KeyDriverLockName(driverID), state["token"])
	if s.listener != nil {
		if err := s.listener.OnDriverAssigned(ctx, bookingID, driverID); err != nil {
			return err
		}
	}
	return s.store.Delete(ctx, bookingID)
}

// Reject is called by the driver app (or by timeout).
func (s *Service) Reject(ctx context.Context, driverID, bookingID string) error {
	_, state, err := s.store.Request(ctx, bookingID)
	if err != nil || state == nil {
		return err
	}
	if state["driver"] != driverID {
		return apperr.Conflict("this offer is not yours")
	}
	s.stats.RecordOffer(ctx, driverID, false)
	_ = s.locker.ReleaseToken(ctx, cache.KeyDriverLockName(driverID), state["token"])
	if err := s.store.ClearOffer(ctx, bookingID, driverID); err != nil {
		return err
	}
	return s.offerNext(ctx, bookingID)
}

// ExpireDue re-offers bookings whose driver did not answer in time (worker, every second).
func (s *Service) ExpireDue(ctx context.Context) (int, error) {
	ids, err := s.store.Due(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		_, state, err := s.store.Request(ctx, id)
		if err != nil || state == nil || state["state"] != "offered" {
			continue
		}
		if err := s.Reject(ctx, state["driver"], id); err != nil {
			s.log.Warn("offer timeout handling", "booking", id, "err", err)
		}
	}
	return len(ids), nil
}
