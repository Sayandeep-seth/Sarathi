package self_drive

import (
	"context"
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
	"sarathi/internal/vehicles"
)

type Service struct {
	cache    *Cache
	calendar *Calendar
	deposit  *Deposit
	bookings *booking.Service
	vehicles *vehicles.Service
	log      *slog.Logger
}

func NewService(c *Cache, cal *Calendar, d *Deposit, b *booking.Service, p *payments.Service, v *vehicles.Service, log *slog.Logger) *Service {
	s := &Service{cache: c, calendar: cal, deposit: d, bookings: b, vehicles: v, log: log}
	p.OnPaid("booking", s.onPaid)
	b.OnCancel(constants.BookingSelfDrive, s.onCancel)
	return s
}

// AddCar registers a car; its band comes from the ex-showroom price
// (taken from the vehicle spec when not given).
func (s *Service) AddCar(ctx context.Context, car Car) error {
	if car.PriceINR <= 0 {
		spec, err := s.vehicles.GetSpec(ctx, car.Model)
		if err != nil {
			return err
		}
		car.PriceINR = spec.PriceINR
	}
	band, err := BandFor(s.cache.Bands(ctx), car.PriceINR)
	if err != nil {
		return err
	}
	car.BandID = band.ID
	return s.cache.PutCar(ctx, car)
}

type ListItem struct {
	Car       Car               `json:"car"`
	Band      pricing.Band      `json:"band"`
	Available bool              `json:"available"`
	Quote     pricing.Breakdown `json:"quote"`
}

// List shows cars near a point for the dates, grouped by band price.
func (s *Service) List(ctx context.Context, near utils.Point, start time.Time, days int) ([]ListItem, error) {
	hits, err := s.cache.NearbyIDs(ctx, near, 15)
	if err != nil {
		return nil, err
	}
	bands := s.cache.Bands(ctx)
	dayIdx := utils.DayRange(start, start.AddDate(0, 0, days-1))
	out := make([]ListItem, 0, len(hits))
	for _, h := range hits {
		car, err := s.cache.Car(ctx, h.Member)
		if err != nil || car == nil {
			continue
		}
		band, err := BandFor(bands, car.PriceINR)
		if err != nil {
			continue
		}
		ok, _ := s.calendar.Available(ctx, car.ID, dayIdx)
		out = append(out, ListItem{Car: *car, Band: band, Available: ok, Quote: pricing.SelfDriveFare(band, days, 0)})
	}
	return out, nil
}

type ReserveRequest struct {
	CarID     string  `json:"car_id"`
	StartDate string  `json:"start_date"`
	Days      int     `json:"days"`
	EstKm     float64 `json:"est_km"`
	LicenseNo string  `json:"license_no"`
}

// Reserve holds the car's days and creates a prepaid booking (rent + deposit).
func (s *Service) Reserve(ctx context.Context, userID string, req ReserveRequest) (*booking.Booking, error) {
	if req.Days < 1 || req.Days > 30 {
		return nil, apperr.BadRequest("days must be 1..30")
	}
	if len(strings.TrimSpace(req.LicenseNo)) < 10 {
		return nil, apperr.BadRequest("valid driving licence number required")
	}
	start, err := utils.ParseDate(req.StartDate)
	if err != nil {
		return nil, apperr.BadRequest("start_date yyyy-mm-dd required")
	}
	car, err := s.cache.Car(ctx, req.CarID)
	if err != nil || car == nil {
		return nil, apperr.NotFound("car not found")
	}
	band, err := BandFor(s.cache.Bands(ctx), car.PriceINR)
	if err != nil {
		return nil, err
	}
	days := utils.DayRange(start, start.AddDate(0, 0, req.Days-1))
	owner := utils.NewID("hold")
	if err := s.calendar.Hold(ctx, car.ID, owner, days); err != nil {
		return nil, err
	}
	fare := pricing.SelfDriveFare(band, req.Days, req.EstKm)
	pickup := start.Add(9 * time.Hour)
	end := start.AddDate(0, 0, req.Days).Add(9 * time.Hour)
	b, err := s.bookings.Create(ctx, booking.CreateInput{
		UserID: userID, Type: constants.BookingSelfDrive, CarID: car.ID, CarModel: car.Model,
		Pickup: car.Location, Drop: car.Location, ScheduledAt: &pickup, EndsAt: &end,
		Fare: &fare, Prepaid: true,
		Meta: map[string]string{
			"hold_owner": owner, "days": joinInts(days), "band": band.ID,
			"deposit": strconv.FormatFloat(band.Deposit, 'f', 0, 64), "license": req.LicenseNo,
		},
	})
	if err != nil {
		s.calendar.Free(ctx, car.ID, owner, days)
		return nil, err
	}
	return b, nil
}

func (s *Service) onPaid(ctx context.Context, in payments.Intent) error {
	b, err := s.bookings.Get(ctx, in.RefID)
	if err != nil || b.Type != constants.BookingSelfDrive {
		return err
	}
	if err := s.calendar.Confirm(ctx, b.CarID, b.Meta["hold_owner"], splitInts(b.Meta["days"])); err != nil {
		s.log.Warn("self-drive hold lost after payment", "booking", b.ID, "err", err)
		return s.bookings.Cancel(ctx, b.UserID, b.ID, "car no longer available")
	}
	dep, _ := strconv.ParseFloat(b.Meta["deposit"], 64)
	return s.deposit.Hold(ctx, b.ID, dep)
}

func (s *Service) onCancel(ctx context.Context, b *booking.Booking) {
	s.calendar.Free(ctx, b.CarID, b.Meta["hold_owner"], splitInts(b.Meta["days"]))
}

// Return closes the rental: inspection deductions are kept, the rest of the deposit refunded.
func (s *Service) Return(ctx context.Context, bookingID string, damage, extraKm float64, lateHours int) (float64, error) {
	b, err := s.bookings.Get(ctx, bookingID)
	if err != nil {
		return 0, err
	}
	band, _ := BandFor(s.cache.Bands(ctx), 0)
	for _, bd := range s.cache.Bands(ctx) {
		if bd.ID == b.Meta["band"] {
			band = bd
		}
	}
	deduction := damage + extraKm*band.ExtraKm + float64(lateHours)*band.DailyRate/24
	return s.deposit.Release(ctx, bookingID, deduction, "returned")
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
