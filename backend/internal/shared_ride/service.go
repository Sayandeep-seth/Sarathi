package shared_ride

import (
	"context"
	"strconv"

	"sarathi/internal/booking"
	"sarathi/internal/common/constants"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
	"sarathi/internal/pricing"
	"sarathi/internal/websocket"
)

const poolSeats = 4

type Service struct {
	store    *Store
	matcher  *Matcher
	bookings *booking.Service
	pricing  *pricing.Service
	pub      *websocket.Publisher
}

func NewService(st *Store, m *Matcher, b *booking.Service, p *pricing.Service, pub *websocket.Publisher) *Service {
	return &Service{store: st, matcher: m, bookings: b, pricing: p, pub: pub}
}

type Request struct {
	Pickup utils.Point `json:"pickup"`
	Drop   utils.Point `json:"drop"`
	Seats  int         `json:"seats"`
}

type Result struct {
	Booking *booking.Booking `json:"booking"`
	TripID  string           `json:"shared_trip_id"`
	Joined  bool             `json:"joined_existing"`
	Riders  int              `json:"riders"`
}

// Request books a shared seat: join a matching pool, or open a new one and dispatch a driver.
func (s *Service) Request(ctx context.Context, userID string, req Request) (*Result, error) {
	if req.Seats <= 0 {
		req.Seats = 1
	}
	if req.Seats > 2 {
		return nil, apperr.BadRequest("max 2 seats per shared booking")
	}
	if !req.Pickup.Valid() || !req.Drop.Valid() {
		return nil, apperr.BadRequest("pickup and drop required")
	}
	km, min := pricing.EstimateTrip(req.Pickup, req.Drop)
	solo := s.pricing.QuoteRide(ctx, constants.ClassShared, req.Pickup, km, min)

	match, err := s.matcher.Find(ctx, req.Pickup, req.Drop, req.Seats)
	if err != nil {
		return nil, err
	}
	if match != nil {
		if _, err := s.store.Join(ctx, match.Trip.ID, userID, req.Seats); err == nil {
			riders := len(match.Trip.Riders) + 1
			fare := SharedFare(solo, riders, req.Seats)
			first, _ := s.bookings.Get(ctx, match.Trip.ID)
			meta := map[string]string{"shared_trip": match.Trip.ID, "seats": strconv.Itoa(req.Seats)}
			if first != nil && first.DriverID != "" {
				meta["driver_id"] = first.DriverID
			}
			b, err := s.bookings.Create(ctx, booking.CreateInput{
				UserID: userID, Type: constants.BookingShared, Pickup: req.Pickup, Drop: req.Drop,
				DistanceKm: km, DurationMin: min, Fare: &fare, Meta: meta,
			})
			if err != nil {
				_ = s.store.Leave(ctx, match.Trip.ID, userID, req.Seats)
				return nil, err
			}
			_ = s.pub.Trip(ctx, match.Trip.ID, "rider_joined", map[string]any{"pickup": req.Pickup, "drop": req.Drop, "booking_id": b.ID})
			return &Result{Booking: b, TripID: match.Trip.ID, Joined: true, Riders: riders}, nil
		}
		// lost the race for the last seat: fall through and open a new pool
	}

	fare := SharedFare(solo, 1, req.Seats)
	b, err := s.bookings.Create(ctx, booking.CreateInput{
		UserID: userID, Type: constants.BookingShared, Class: constants.ClassShared,
		Pickup: req.Pickup, Drop: req.Drop, DistanceKm: km, DurationMin: min, Fare: &fare,
		Meta: map[string]string{"shared_trip": "", "seats": strconv.Itoa(req.Seats)},
	})
	if err != nil {
		return nil, err
	}
	t := Trip{ID: b.ID, Origin: req.Pickup, Dest: req.Drop, SeatsTotal: poolSeats, SeatsLeft: poolSeats - req.Seats}
	if err := s.store.Create(ctx, t, userID); err != nil {
		return nil, err
	}
	return &Result{Booking: b, TripID: b.ID, Riders: 1}, nil
}

// Leave removes a rider before pickup.
func (s *Service) Leave(ctx context.Context, userID, tripID, bookingID string, seats int) error {
	if err := s.bookings.Cancel(ctx, userID, bookingID, "left shared ride"); err != nil {
		return err
	}
	return s.store.Leave(ctx, tripID, userID, seats)
}

// Close is called when the driver starts the pooled trip.
func (s *Service) Close(ctx context.Context, tripID string) error { return s.store.Close(ctx, tripID) }

func (s *Service) Get(ctx context.Context, tripID string) (*Trip, error) {
	return s.store.Get(ctx, tripID)
}
