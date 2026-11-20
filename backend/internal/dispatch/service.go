package dispatch

import (
	"context"
	"log/slog"

	"sarathi/internal/analytics"
	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
	"sarathi/internal/driver"
	"sarathi/internal/websocket"
)

// Request is a booking that needs a driver.
type Request struct {
	BookingID   string      `json:"booking_id"`
	UserID      string      `json:"user_id"`
	Type        string      `json:"type"`
	Class       string      `json:"class"` // GEO set to search: cab, seat5, seat7, shared, testdrive…
	Pickup      utils.Point `json:"pickup"`
	Drop        utils.Point `json:"drop"`
	Fare        float64     `json:"fare"`
	MaxRadiusKm float64     `json:"max_radius_km,omitempty"`
}

// AssignmentListener is implemented by booking.Service.
type AssignmentListener interface {
	OnDriverAssigned(ctx context.Context, bookingID, driverID string) error
	OnNoDriver(ctx context.Context, bookingID string) error
}

// CandidateFilter lets a module veto drivers for a class (test-drive cars must stay in their zone).
type CandidateFilter interface {
	Allow(ctx context.Context, driverID string, req Request) bool
}

type Service struct {
	finder   *Finder
	store    *Store
	locker   *cache.Locker
	drivers  *driver.Service
	pub      *websocket.Publisher
	stats    *analytics.Service
	listener AssignmentListener
	filters  map[string]CandidateFilter
	log      *slog.Logger
}

func NewService(f *Finder, st *Store, l *cache.Locker, d *driver.Service, pub *websocket.Publisher, stats *analytics.Service, log *slog.Logger) *Service {
	return &Service{finder: f, store: st, locker: l, drivers: d, pub: pub, stats: stats, filters: map[string]CandidateFilter{}, log: log}
}

func (s *Service) SetListener(l AssignmentListener)          { s.listener = l }
func (s *Service) SetFilter(class string, f CandidateFilter) { s.filters[class] = f }

// Dispatch starts matching a booking.
func (s *Service) Dispatch(ctx context.Context, req Request) error {
	if req.BookingID == "" || req.Class == "" || !req.Pickup.Valid() {
		return apperr.BadRequest("dispatch needs booking, class and pickup")
	}
	if err := s.store.SaveRequest(ctx, req); err != nil {
		return err
	}
	return s.offerNext(ctx, req.BookingID)
}

// Cancel stops matching (rider cancelled before a driver accepted).
func (s *Service) Cancel(ctx context.Context, bookingID string) error {
	_, state, err := s.store.Request(ctx, bookingID)
	if err != nil || state == nil {
		return err
	}
	if d := state["driver"]; d != "" && state["state"] == "offered" {
		_ = s.locker.ReleaseToken(ctx, cache.KeyDriverLockName(d), state["token"])
		_ = s.pub.Driver(ctx, d, websocket.EventTripCancelled, map[string]string{"booking_id": bookingID})
	}
	return s.store.Delete(ctx, bookingID)
}

// Nearby returns available drivers around a point (rider map: "cars near you").
func (s *Service) Nearby(ctx context.Context, class string, p utils.Point) ([]Candidate, error) {
	return s.finder.Find(ctx, class, p, nil, 3)
}
