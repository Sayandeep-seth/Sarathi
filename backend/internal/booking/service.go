package booking

import (
	"context"
	"log/slog"
	"time"

	"sarathi/internal/analytics"
	"sarathi/internal/cache"
	"sarathi/internal/common/constants"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
	"sarathi/internal/dispatch"
	"sarathi/internal/driver"
	"sarathi/internal/notifications"
	"sarathi/internal/payments"
	"sarathi/internal/pricing"
	"sarathi/internal/websocket"
)

// Booking is any trip of any type.
type Booking struct {
	ID          string            `json:"id"`
	UserID      string            `json:"user_id"`
	DriverID    string            `json:"driver_id,omitempty"`
	Type        string            `json:"type"`
	State       string            `json:"state"`
	Class       string            `json:"class,omitempty"`
	CarModel    string            `json:"car_model,omitempty"`
	CarID       string            `json:"car_id,omitempty"`
	Pickup      utils.Point       `json:"pickup"`
	Drop        utils.Point       `json:"drop"`
	ScheduledAt *time.Time        `json:"scheduled_at,omitempty"`
	EndsAt      *time.Time        `json:"ends_at,omitempty"`
	DistanceKm  float64           `json:"distance_km"`
	DurationMin float64           `json:"duration_min"`
	Fare        pricing.Breakdown `json:"fare"`
	PaymentID   string            `json:"payment_intent_id,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

// CreateInput is used by every booking module.
type CreateInput struct {
	UserID         string
	Type           string
	Class          string // dispatch class; empty = no driver dispatch
	CarModel       string
	CarID          string
	Pickup, Drop   utils.Point
	ScheduledAt    *time.Time
	EndsAt         *time.Time
	DistanceKm     float64
	DurationMin    float64
	Fare           *pricing.Breakdown // nil = distance/time quote with surge
	Prepaid        bool               // pay before the service (premium, tourist, self-drive…)
	Meta           map[string]string
	IdempotencyKey string
}

type Service struct {
	repo     *Repository
	cache    *Cache
	cs       *cache.Service
	resv     *Reservations
	dispatch *dispatch.Service
	drivers  *driver.Service
	pricing  *pricing.Service
	payments *payments.Service
	pub      *websocket.Publisher
	notify   *notifications.Queue
	stats    *analytics.Service
	log      *slog.Logger

	cancelHooks   map[string][]func(context.Context, *Booking)
	completeHooks map[string][]func(context.Context, *Booking)
}

// OnComplete lets a module react to finished trips (e.g. dealer commission).
func (s *Service) OnComplete(bookingType string, fn func(context.Context, *Booking)) {
	s.completeHooks[bookingType] = append(s.completeHooks[bookingType], fn)
}

// OnCancel lets a module release its own reservations (calendar days, hourly
// slots, test-drive slots) when a booking of its type is cancelled.
func (s *Service) OnCancel(bookingType string, fn func(context.Context, *Booking)) {
	s.cancelHooks[bookingType] = append(s.cancelHooks[bookingType], fn)
}

func NewService(repo *Repository, c *Cache, cs *cache.Service, resv *Reservations, d *dispatch.Service, drv *driver.Service,
	p *pricing.Service, pay *payments.Service, pub *websocket.Publisher, n *notifications.Queue, st *analytics.Service, log *slog.Logger) *Service {
	s := &Service{repo: repo, cache: c, cs: cs, resv: resv, dispatch: d, drivers: drv, pricing: p, payments: pay, pub: pub, notify: n, stats: st, log: log,
		cancelHooks: map[string][]func(context.Context, *Booking){}, completeHooks: map[string][]func(context.Context, *Booking){}}
	d.SetListener(s)
	pay.OnPaid("booking", s.onPaid)
	return s
}

const dispatchLead = 20 * time.Minute

// Create books a trip. Cab-like types are dispatched now (or scheduled);
// prepaid types wait for payment first.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Booking, error) {
	if in.UserID == "" {
		return nil, apperr.Forbidden("login required")
	}
	if !in.Pickup.Valid() {
		return nil, apperr.BadRequest("pickup location required")
	}
	id := utils.NewID("bk")
	if stored, fresh, err := s.cs.Idempotent(ctx, "booking", in.IdempotencyKey, id); err != nil {
		return nil, err
	} else if !fresh {
		return s.Get(ctx, stored) // client retried the same request
	}

	b := &Booking{
		ID: id, UserID: in.UserID, Type: in.Type, Class: in.Class, CarModel: in.CarModel, CarID: in.CarID,
		Pickup: in.Pickup, Drop: in.Drop, ScheduledAt: in.ScheduledAt, EndsAt: in.EndsAt,
		DistanceKm: in.DistanceKm, DurationMin: in.DurationMin, Meta: in.Meta, CreatedAt: time.Now(),
	}
	if b.DistanceKm == 0 && in.Drop.Valid() {
		b.DistanceKm, b.DurationMin = pricing.EstimateTrip(in.Pickup, in.Drop)
	}
	if in.Fare != nil {
		b.Fare = *in.Fare
	} else {
		_ = s.pricing.Surge.RecordDemand(ctx, in.Pickup)
		b.Fare = s.pricing.QuoteRide(ctx, in.Class, in.Pickup, b.DistanceKm, b.DurationMin)
	}

	immediate := in.Class != "" && !in.Prepaid && (in.ScheduledAt == nil || time.Until(*in.ScheduledAt) < dispatchLead)
	if immediate {
		ok, cur, err := s.cache.ClaimActive(ctx, in.UserID, id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, apperr.Conflict("you already have an active ride: " + cur)
		}
	}

	switch {
	case in.Prepaid:
		b.State = StatePendingPayment
	case immediate:
		b.State = StateSearching
	case in.Class != "" && in.ScheduledAt != nil:
		b.State = StateScheduled
	default:
		b.State = StateConfirmed // no driver to find (e.g. joined an existing shared trip)
	}
	if err := s.repo.Insert(ctx, b); err != nil {
		_ = s.cache.ReleaseActive(ctx, in.UserID, id)
		return nil, err
	}
	if err := s.cache.Save(ctx, b); err != nil {
		return nil, err
	}
	s.stats.Count(ctx, analytics.MetricBookings, 1)
	s.stats.Unique(ctx, analytics.MetricActiveUsers, in.UserID)

	switch b.State {
	case StatePendingPayment:
		intent, err := s.payments.CreateIntent(ctx, "booking", b.ID, b.UserID, b.Fare.Total+b.Fare.Deposit)
		if err != nil {
			return nil, err
		}
		b.PaymentID = intent.ID
		_ = s.cache.Save(ctx, b)
	case StateScheduled:
		if err := s.resv.Schedule(ctx, b.ID, *b.ScheduledAt); err != nil {
			return nil, err
		}
	case StateSearching:
		if err := s.startDispatch(ctx, b); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (s *Service) startDispatch(ctx context.Context, b *Booking) error {
	return s.dispatch.Dispatch(ctx, dispatch.Request{
		BookingID: b.ID, UserID: b.UserID, Type: b.Type, Class: b.Class,
		Pickup: b.Pickup, Drop: b.Drop, Fare: b.Fare.Total,
	})
}

// onPaid: prepaid booking paid -> confirmed (or scheduled for a chauffeur dispatch).
func (s *Service) onPaid(ctx context.Context, in payments.Intent) error {
	b, err := s.Get(ctx, in.RefID)
	if err != nil {
		return err
	}
	if err := s.move(ctx, b.ID, StateConfirmed, nil); err != nil {
		return err
	}
	if b.Class != "" && b.ScheduledAt != nil {
		_ = s.resv.Schedule(ctx, b.ID, *b.ScheduledAt)
	}
	s.notify.Notify(ctx, b.UserID, "Booking confirmed", "Your "+b.Type+" booking "+b.ID+" is confirmed.", map[string]string{"booking_id": b.ID})
	return s.pub.User(ctx, b.UserID, websocket.EventPayment, map[string]string{"booking_id": b.ID, "state": StateConfirmed})
}

// Get reads from Redis, falling back to Postgres.
func (s *Service) Get(ctx context.Context, id string) (*Booking, error) {
	if b, ok, err := s.cache.Get(ctx, id); err == nil && ok {
		return b, nil
	}
	b, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Save(ctx, b)
	return b, nil
}

// move = atomic Redis transition, then refresh + persist the full record.
func (s *Service) move(ctx context.Context, id, to string, mutate func(*Booking)) error {
	err := s.cache.Transition(ctx, id, to, nil)
	if apperr.IsNotFound(err) { // evicted from Redis: reload and retry once
		if _, gerr := s.Get(ctx, id); gerr != nil {
			return gerr
		}
		err = s.cache.Transition(ctx, id, to, nil)
	}
	if err != nil {
		return err
	}
	b, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	b.State = to
	if mutate != nil {
		mutate(b)
	}
	if err := s.cache.Save(ctx, b); err != nil {
		return err
	}
	return s.repo.Update(ctx, b)
}

// OnDriverAssigned implements dispatch.AssignmentListener.
func (s *Service) OnDriverAssigned(ctx context.Context, bookingID, driverID string) error {
	if err := s.move(ctx, bookingID, StateAssigned, func(b *Booking) { b.DriverID = driverID }); err != nil {
		_ = s.drivers.MarkAvailable(ctx, driverID) // booking was cancelled meanwhile
		return err
	}
	b, _ := s.Get(ctx, bookingID)
	pos, _ := s.drivers.Position(ctx, driverID)
	payload := map[string]any{"booking_id": bookingID, "driver_id": driverID, "driver_location": pos}
	_ = s.pub.Trip(ctx, bookingID, websocket.EventDriverAssigned, payload)
	s.notify.Notify(ctx, b.UserID, "Driver assigned", "Your driver is on the way.", map[string]string{"booking_id": bookingID})
	return nil
}

// OnNoDriver implements dispatch.AssignmentListener.
func (s *Service) OnNoDriver(ctx context.Context, bookingID string) error {
	if err := s.move(ctx, bookingID, StateNoDriver, nil); err != nil {
		return err
	}
	b, _ := s.Get(ctx, bookingID)
	_ = s.cache.ReleaseActive(ctx, b.UserID, bookingID)
	_ = s.pub.Trip(ctx, bookingID, websocket.EventTripCancelled, map[string]string{"booking_id": bookingID, "reason": "no_driver"})
	return nil
}

// Arrived / Start / Complete are called by the driver app.
func (s *Service) Arrived(ctx context.Context, driverID, id string) error {
	if err := s.ownDriver(ctx, driverID, id); err != nil {
		return err
	}
	if err := s.move(ctx, id, StateArrived, nil); err != nil {
		return err
	}
	return s.pub.Trip(ctx, id, websocket.EventDriverArriving, map[string]string{"booking_id": id})
}

func (s *Service) Start(ctx context.Context, driverID, id string) error {
	if err := s.ownDriver(ctx, driverID, id); err != nil {
		return err
	}
	now := time.Now()
	if err := s.move(ctx, id, StateInProgress, func(b *Booking) { b.StartedAt = &now }); err != nil {
		return err
	}
	return s.pub.Trip(ctx, id, websocket.EventTripStarted, map[string]string{"booking_id": id})
}

// Complete finishes a trip, frees the driver, records stats and opens the payment.
func (s *Service) Complete(ctx context.Context, driverID, id string, actualKm float64) (*Booking, error) {
	if err := s.ownDriver(ctx, driverID, id); err != nil {
		return nil, err
	}
	now := time.Now()
	err := s.move(ctx, id, StateCompleted, func(b *Booking) {
		b.CompletedAt = &now
		if actualKm > 0 && b.Type == constants.BookingRide && b.StartedAt != nil {
			b.DistanceKm = actualKm
			b.Fare = s.pricing.QuoteRide(ctx, b.Class, b.Pickup, actualKm, now.Sub(*b.StartedAt).Minutes())
		}
	})
	if err != nil {
		return nil, err
	}
	b, _ := s.Get(ctx, id)
	if b.DriverID != "" {
		_ = s.drivers.MarkAvailable(ctx, b.DriverID)
		earning := b.Fare.Total * 0.8 // 20 % platform commission
		s.stats.RecordEarning(ctx, b.DriverID, earning, now)
		_ = s.drivers.AddEarning(ctx, b.DriverID, b.ID, earning)
	}
	_ = s.cache.ReleaseActive(ctx, b.UserID, id)
	s.stats.RecordRide(ctx, b.Type, b.UserID, b.Fare.Total, b.Pickup, now)
	for _, hook := range s.completeHooks[b.Type] {
		hook(ctx, b)
	}
	if b.PaymentID == "" {
		if intent, err := s.payments.CreateIntent(ctx, "booking", b.ID, b.UserID, b.Fare.Total); err == nil {
			b.PaymentID = intent.ID
			_ = s.cache.Save(ctx, b)
		}
	}
	_ = s.pub.Trip(ctx, id, websocket.EventTripCompleted, map[string]any{"booking_id": id, "fare": b.Fare, "payment_intent_id": b.PaymentID})
	return b, nil
}

// Cancel by the rider.
func (s *Service) Cancel(ctx context.Context, userID, id, reason string) error {
	b, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if b.UserID != userID {
		return apperr.Forbidden("not your booking")
	}
	if err := s.move(ctx, id, StateCancelled, func(b *Booking) {
		if b.Meta == nil {
			b.Meta = map[string]string{}
		}
		b.Meta["cancel_reason"] = reason
	}); err != nil {
		return err
	}
	_ = s.dispatch.Cancel(ctx, id)
	_ = s.resv.Unschedule(ctx, id)
	_ = s.cache.ReleaseActive(ctx, userID, id)
	if b.DriverID != "" {
		_ = s.drivers.MarkAvailable(ctx, b.DriverID)
	}
	for _, hook := range s.cancelHooks[b.Type] {
		hook(ctx, b)
	}
	if paid, _ := s.payments.IsPaid(ctx, "booking", id); paid {
		if _, err := s.payments.RefundByRef(ctx, "booking", id, 0, "cancelled: "+reason); err != nil {
			s.log.Error("refund on cancel", "booking", id, "err", err)
		}
	}
	s.stats.Count(ctx, analytics.MetricCancellations, 1)
	return s.pub.Trip(ctx, id, websocket.EventTripCancelled, map[string]string{"booking_id": id, "reason": reason})
}

// DispatchDue runs in the worker: scheduled bookings close to pickup start matching.
func (s *Service) DispatchDue(ctx context.Context) (int, error) {
	ids, err := s.resv.Due(ctx, dispatchLead)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.move(ctx, id, StateSearching, nil); err != nil {
			s.log.Warn("scheduled dispatch", "booking", id, "err", err)
			continue
		}
		b, _ := s.Get(ctx, id)
		if err := s.startDispatch(ctx, b); err != nil {
			s.log.Warn("scheduled dispatch", "booking", id, "err", err)
		}
	}
	return len(ids), nil
}

func (s *Service) ownDriver(ctx context.Context, driverID, id string) error {
	b, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if b.DriverID == "" || b.DriverID != driverID {
		return apperr.Forbidden("not your trip")
	}
	return nil
}

// Quote prices a cab ride before booking.
func (s *Service) Quote(ctx context.Context, class string, pickup, drop utils.Point) (map[string]any, error) {
	if !pickup.Valid() || !drop.Valid() {
		return nil, apperr.BadRequest("pickup and drop required")
	}
	km, min := pricing.EstimateTrip(pickup, drop)
	fare := s.pricing.QuoteRide(ctx, class, pickup, km, min)
	return map[string]any{"class": class, "distance_km": km, "duration_min": min, "fare": fare}, nil
}
