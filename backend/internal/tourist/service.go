package tourist

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"sarathi/internal/booking"
	"sarathi/internal/common/constants"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
	"sarathi/internal/payments"
	"sarathi/internal/pricing"
)

type Service struct {
	cache    *Cache
	bookings *booking.Service
	log      *slog.Logger
}

func NewService(c *Cache, b *booking.Service, p *payments.Service, log *slog.Logger) *Service {
	s := &Service{cache: c, bookings: b, log: log}
	p.OnPaid("booking", s.onPaid)
	b.OnCancel(constants.BookingTourist, s.onCancel)
	return s
}

// Request for a tourist trip.
type Request struct {
	Class       string      `json:"class"` // sedan | suv | tempo_traveller
	Origin      utils.Point `json:"origin"`
	Destination string      `json:"destination"`
	Stops       []Stop      `json:"stops"` // empty = popular stops of the destination
	StartDate   string      `json:"start_date"`
	Days        int         `json:"days"`
}

type Quote struct {
	Itinerary Itinerary         `json:"itinerary"`
	Fare      pricing.Breakdown `json:"fare"`
}

func (s *Service) Quote(ctx context.Context, req Request) (*Quote, error) {
	if req.Days < 1 || req.Days > 30 {
		return nil, apperr.BadRequest("days must be 1..30")
	}
	if !req.Origin.Valid() {
		return nil, apperr.BadRequest("origin required")
	}
	if _, ok := defaultRates[req.Class]; !ok {
		return nil, apperr.BadRequest("class must be sedan, suv or tempo_traveller")
	}
	stops := req.Stops
	if len(stops) == 0 {
		var err error
		if stops, err = s.cache.PopularStops(ctx, req.Destination); err != nil {
			return nil, err
		}
	}
	if len(stops) == 0 {
		return nil, apperr.BadRequest("add at least one tourist place")
	}
	it := Plan(req.Origin, req.Destination, stops, req.Days)
	return &Quote{Itinerary: it, Fare: s.cache.QuoteDaily(ctx, req.Class, req.Days, it.TotalKm)}, nil
}

// Book finds a free car of the class for every day of the trip and holds it.
func (s *Service) Book(ctx context.Context, userID string, req Request) (*booking.Booking, error) {
	q, err := s.Quote(ctx, req)
	if err != nil {
		return nil, err
	}
	start, err := utils.ParseDate(req.StartDate)
	if err != nil || start.Before(time.Now().Truncate(24*time.Hour)) {
		return nil, apperr.BadRequest("start_date must be today or later (yyyy-mm-dd)")
	}
	end := start.AddDate(0, 0, req.Days-1)
	days := utils.DayRange(start, end)

	ids, err := s.cache.FleetIDs(ctx, req.Class)
	if err != nil {
		return nil, err
	}
	owner := utils.NewID("hold")
	var car *Car
	for _, id := range ids {
		if ok, _ := s.cache.DaysFree(ctx, id, days); !ok {
			continue
		}
		if s.cache.HoldDays(ctx, id, owner, days) == nil {
			car, _ = s.cache.Car(ctx, id)
			break
		}
	}
	if car == nil {
		return nil, apperr.Conflict("no " + req.Class + " available for those dates")
	}
	itin, _ := json.Marshal(q.Itinerary)
	pickup := start.Add(7 * time.Hour) // 7 am on day 1
	b, err := s.bookings.Create(ctx, booking.CreateInput{
		UserID: userID, Type: constants.BookingTourist, CarID: car.ID, CarModel: car.Model,
		Pickup: req.Origin, Drop: req.Origin, ScheduledAt: &pickup, EndsAt: &end,
		DistanceKm: q.Itinerary.TotalKm, Fare: &q.Fare, Prepaid: true,
		Meta: map[string]string{
			"hold_owner": owner, "days": joinInts(days), "destination": req.Destination,
			"driver_id": car.DriverID, "itinerary": string(itin),
		},
	})
	if err != nil {
		s.cache.ReleaseDays(ctx, car.ID, owner, days)
		return nil, err
	}
	return b, nil
}

func (s *Service) onPaid(ctx context.Context, in payments.Intent) error {
	b, err := s.bookings.Get(ctx, in.RefID)
	if err != nil || b.Type != constants.BookingTourist {
		return err
	}
	if err := s.cache.ConfirmDays(ctx, b.CarID, b.Meta["hold_owner"], splitInts(b.Meta["days"])); err != nil {
		s.log.Warn("tourist hold lost after payment", "booking", b.ID, "err", err)
		return s.bookings.Cancel(ctx, b.UserID, b.ID, "car no longer available")
	}
	return nil
}

func (s *Service) onCancel(ctx context.Context, b *booking.Booking) {
	s.cache.ReleaseDays(ctx, b.CarID, b.Meta["hold_owner"], splitInts(b.Meta["days"]))
}

func joinInts(v []int) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, ",")
}

func splitInts(s string) []int {
	var out []int
	for _, p := range strings.Split(s, ",") {
		if n, err := strconv.Atoi(p); err == nil {
			out = append(out, n)
		}
	}
	return out
}
