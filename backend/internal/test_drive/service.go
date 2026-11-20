package test_drive

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"sarathi/internal/booking"
	"sarathi/internal/cache"
	"sarathi/internal/common/constants"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
	"sarathi/internal/payments"
	"sarathi/internal/pricing"
)

const slotsPerDay = 48 // half-hour slots

type Service struct {
	cache       *Cache
	eligibility *Eligibility
	geofence    *Geofence
	commission  *Commission
	slots       *cache.Slots
	bookings    *booking.Service
	log         *slog.Logger
}

func NewService(c *Cache, e *Eligibility, g *Geofence, cm *Commission, sl *cache.Slots, b *booking.Service, p *payments.Service, log *slog.Logger) *Service {
	s := &Service{cache: c, eligibility: e, geofence: g, commission: cm, slots: sl, bookings: b, log: log}
	p.OnPaid("booking", s.onPaid)
	b.OnCancel(constants.BookingTestDrive, s.onCancel)
	b.OnComplete(constants.BookingTestDriveCab, s.onCabComplete)
	return s
}

// TestDriveRequest books a half-hour test drive.
type TestDriveRequest struct {
	ShowroomID string `json:"showroom_id"`
	Model      string `json:"model"`
	Date       string `json:"date"` // yyyy-mm-dd
	Slot       int    `json:"slot"` // 0..47 (slot 20 = 10:00–10:30)
}

// BookTestDrive: eligibility (once per model per showroom) -> free demo car slot -> prepaid booking.
func (s *Service) BookTestDrive(ctx context.Context, customerID string, req TestDriveRequest) (*booking.Booking, error) {
	day, err := utils.ParseDate(req.Date)
	if err != nil || req.Slot < 0 || req.Slot >= slotsPerDay {
		return nil, apperr.BadRequest("valid date and slot (0..47) required")
	}
	start := day.Add(time.Duration(req.Slot) * 30 * time.Minute)
	if start.Before(time.Now()) {
		return nil, apperr.BadRequest("slot is in the past")
	}
	cars, err := s.cache.CarsOfModel(ctx, req.ShowroomID, req.Model)
	if err != nil {
		return nil, err
	}
	if len(cars) == 0 {
		return nil, apperr.NotFound("no demo car of this model at the showroom")
	}
	model := utils.NormalizeName(req.Model)
	if err := s.eligibility.Claim(ctx, req.ShowroomID, model, customerID); err != nil {
		return nil, err
	}
	owner := utils.NewID("hold")
	dateKey := utils.DateKey(day)
	var car *Car
	for i := range cars {
		if s.slots.Hold(ctx, cache.KeyTDSlots(cars[i].VehicleID, dateKey), owner, slotsPerDay, cache.TTLCalendarHold, req.Slot) == nil {
			car = &cars[i]
			break
		}
	}
	if car == nil {
		_ = s.eligibility.Unclaim(ctx, req.ShowroomID, model, customerID)
		return nil, apperr.Conflict("all demo cars are booked for that slot")
	}
	zone, _ := s.cache.Zone(ctx, req.ShowroomID)
	var at utils.Point
	if zone != nil {
		at = zone.Center
	}
	fare := pricing.NewBreakdown()
	fare.AddFixed("test_drive_fee", TestDriveFee)
	fare.Finalize()
	end := start.Add(30 * time.Minute)
	b, err := s.bookings.Create(ctx, booking.CreateInput{
		UserID: customerID, Type: constants.BookingTestDrive, CarID: car.VehicleID, CarModel: car.Model,
		Pickup: at, Drop: at, ScheduledAt: &start, EndsAt: &end, Fare: &fare, Prepaid: true,
		Meta: map[string]string{"hold_owner": owner, "showroom_id": req.ShowroomID, "dealer_id": car.DealerID,
			"model": model, "date": dateKey, "slot": strconv.Itoa(req.Slot)},
	})
	if err != nil {
		_ = s.slots.Release(ctx, cache.KeyTDSlots(car.VehicleID, dateKey), owner, slotsPerDay, req.Slot)
		_ = s.eligibility.Unclaim(ctx, req.ShowroomID, model, customerID)
		return nil, err
	}
	return b, nil
}

func (s *Service) onPaid(ctx context.Context, in payments.Intent) error {
	b, err := s.bookings.Get(ctx, in.RefID)
	if err != nil || b.Type != constants.BookingTestDrive {
		return err
	}
	slot, _ := strconv.Atoi(b.Meta["slot"])
	if err := s.slots.Confirm(ctx, cache.KeyTDSlots(b.CarID, b.Meta["date"]), b.Meta["hold_owner"], slotsPerDay, 3*24*time.Hour, slot); err != nil {
		return s.bookings.Cancel(ctx, b.UserID, b.ID, "slot no longer available")
	}
	return s.commission.TestDrive(ctx, b.Meta["dealer_id"], b.ID)
}

func (s *Service) onCancel(ctx context.Context, b *booking.Booking) {
	slot, _ := strconv.Atoi(b.Meta["slot"])
	k := cache.KeyTDSlots(b.CarID, b.Meta["date"])
	_ = s.slots.Release(ctx, k, b.Meta["hold_owner"], slotsPerDay, slot)
	_ = s.slots.Cancel(ctx, k, b.Meta["hold_owner"], slotsPerDay, slot)
	_ = s.eligibility.Unclaim(ctx, b.Meta["showroom_id"], b.Meta["model"], b.UserID)
}

// RequestCab books a ride served by a demo car (only inside a showroom zone).
func (s *Service) RequestCab(ctx context.Context, userID string, pickup, drop utils.Point) (*booking.Booking, error) {
	return s.bookings.Create(ctx, booking.CreateInput{
		UserID: userID, Type: constants.BookingTestDriveCab, Class: constants.ClassTestDrive, Pickup: pickup, Drop: drop,
	})
}

// onCabComplete credits the dealer when a demo car finishes a cab ride.
func (s *Service) onCabComplete(ctx context.Context, b *booking.Booking) {
	vehicleID, _ := s.geofence.rdb.HGet(ctx, cache.KeyDriver(b.DriverID), "vehicle_id").Result()
	car, _ := s.cache.Car(ctx, vehicleID)
	if car == nil {
		s.log.Warn("test-drive cab without demo car", "booking", b.ID)
		return
	}
	if err := s.commission.Cab(ctx, car.DealerID, b.ID, b.Fare.Total); err != nil {
		s.log.Error("dealer commission", "err", err)
	}
}

// Earnings for the dealer dashboard.
func (s *Service) Earnings(ctx context.Context, dealerID string) (map[string]string, error) {
	return s.commission.Earnings(ctx, dealerID, utils.MonthKey(time.Now()))
}
