package premium

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"sarathi/internal/booking"
	"sarathi/internal/common/constants"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
	"sarathi/internal/payments"
	"sarathi/internal/pricing"
)

const premiumMinHours = 4

// HourlyRequest books one specific car for an event.
type HourlyRequest struct {
	CarID        string      `json:"car_id"`
	Start        time.Time   `json:"start"`
	Hours        int         `json:"hours"`
	EventAddress utils.Point `json:"event_address"`
	EventType    string      `json:"event_type"` // wedding, reception, corporate, party…
	Extras       []string    `json:"extras"`
}

// Service handles premium (hourly) and vintage bookings.
type Service struct {
	cache    *Cache
	avail    *Availability
	bookings *booking.Service
	payments *payments.Service
	log      *slog.Logger
}

func NewService(c *Cache, a *Availability, b *booking.Service, p *payments.Service, log *slog.Logger) *Service {
	s := &Service{cache: c, avail: a, bookings: b, payments: p, log: log}
	p.OnPaid("booking", s.onPaid)
	b.OnCancel(constants.BookingPremiumHourly, s.onCancel)
	b.OnCancel(constants.BookingVintage, s.onCancel)
	return s
}

// BookHourly books a premium car, charged per hour (min 4 h).
func (s *Service) BookHourly(ctx context.Context, userID string, req HourlyRequest) (*booking.Booking, error) {
	return s.book(ctx, userID, req, CategoryPremium, constants.BookingPremiumHourly, premiumMinHours, nil)
}

func (s *Service) book(ctx context.Context, userID string, req HourlyRequest, category, bookingType string, minHours int, extras map[string]float64) (*booking.Booking, error) {
	if req.Hours <= 0 || req.Hours > 24 {
		return nil, apperr.BadRequest("hours must be 1..24")
	}
	if time.Until(req.Start) < 2*time.Hour {
		return nil, apperr.BadRequest("premium bookings need at least 2 hours notice")
	}
	if !req.EventAddress.Valid() {
		return nil, apperr.BadRequest("event address required")
	}
	car, err := s.cache.Car(ctx, req.CarID)
	if err != nil {
		return nil, err
	}
	if car.Category != category {
		return nil, apperr.BadRequest("car is not a " + category + " car")
	}
	if req.Hours < minHours {
		req.Hours = minHours
	}
	owner := utils.NewID("hold")
	if err := s.avail.Hold(ctx, car.ID, owner, req.Start, req.Hours); err != nil {
		return nil, err
	}
	fare := pricing.HourlyFare(car.RatePerHour, req.Hours, minHours, extras)
	end := req.Start.Add(time.Duration(req.Hours) * time.Hour)
	b, err := s.bookings.Create(ctx, booking.CreateInput{
		UserID: userID, Type: bookingType, CarID: car.ID, CarModel: car.Model,
		Pickup: req.EventAddress, Drop: req.EventAddress, ScheduledAt: &req.Start, EndsAt: &end,
		Fare: &fare, Prepaid: true,
		Meta: map[string]string{"hold_owner": owner, "hours": strconv.Itoa(req.Hours), "event_type": req.EventType},
	})
	if err != nil {
		s.avail.Release(ctx, car.ID, owner, req.Start, req.Hours)
		return nil, err
	}
	return b, nil
}

// onPaid turns the temporary hold into a confirmed reservation.
func (s *Service) onPaid(ctx context.Context, in payments.Intent) error {
	b, err := s.bookings.Get(ctx, in.RefID)
	if err != nil || (b.Type != constants.BookingPremiumHourly && b.Type != constants.BookingVintage) {
		return err
	}
	hours, _ := strconv.Atoi(b.Meta["hours"])
	if err := s.avail.Confirm(ctx, b.CarID, b.Meta["hold_owner"], *b.ScheduledAt, hours); err != nil {
		// hold expired before payment and someone else took the slot: cancel + refund
		s.log.Warn("premium hold lost after payment", "booking", b.ID, "err", err)
		return s.bookings.Cancel(ctx, b.UserID, b.ID, "slot no longer available")
	}
	return nil
}

func (s *Service) onCancel(ctx context.Context, b *booking.Booking) {
	hours, _ := strconv.Atoi(b.Meta["hours"])
	if b.ScheduledAt == nil {
		return
	}
	s.avail.Cancel(ctx, b.CarID, b.Meta["hold_owner"], *b.ScheduledAt, hours)
	s.avail.Release(ctx, b.CarID, b.Meta["hold_owner"], *b.ScheduledAt, hours)
}

// Register mounts the premium + vintage endpoints (no separate handler file).
func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/premium/cars", func(w http.ResponseWriter, r *http.Request) {
		cat := r.URL.Query().Get("category")
		if cat == "" {
			cat = CategoryPremium
		}
		cars, err := s.cache.Fleet(r.Context(), cat, r.URL.Query().Get("city"))
		if err != nil {
			response.Error(w, err)
			return
		}
		response.OK(w, map[string]any{"cars": cars})
	})
	mux.HandleFunc("GET /api/v1/premium/cars/{id}/availability", func(w http.ResponseWriter, r *http.Request) {
		d, err := utils.ParseDate(r.URL.Query().Get("date"))
		if err != nil {
			response.Error(w, apperr.BadRequest("date=yyyy-mm-dd required"))
			return
		}
		free, err := s.avail.FreeHours(r.Context(), r.PathValue("id"), d)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.OK(w, map[string]any{"free_hours": free})
	})
	mux.HandleFunc("POST /api/v1/premium/book", func(w http.ResponseWriter, r *http.Request) {
		var req HourlyRequest
		if err := response.Decode(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		b, err := s.BookHourly(r.Context(), utils.UserID(r), req)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Created(w, b)
	})
	mux.HandleFunc("POST /api/v1/vintage/book", func(w http.ResponseWriter, r *http.Request) {
		var req HourlyRequest
		if err := response.Decode(r, &req); err != nil {
			response.Error(w, err)
			return
		}
		b, err := s.BookVintage(r.Context(), utils.UserID(r), req)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.Created(w, b)
	})
}
