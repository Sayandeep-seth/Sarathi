package transit_pass

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sarathi/internal/analytics"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
	"sarathi/internal/payments"
)

type Service struct {
	store     *Store
	fares     *Fares
	gen       *QRGenerator
	validator *QRValidator
	payments  *payments.Service
	stats     *analytics.Service
	db        *pgxpool.Pool
	qrTTL     time.Duration
	log       *slog.Logger
}

func NewService(st *Store, f *Fares, g *QRGenerator, v *QRValidator, p *payments.Service, a *analytics.Service, db *pgxpool.Pool, qrTTL time.Duration, log *slog.Logger) *Service {
	s := &Service{store: st, fares: f, gen: g, validator: v, payments: p, stats: a, db: db, qrTTL: qrTTL, log: log}
	p.OnPaid("transit", s.onPaid)
	return s
}

func (s *Service) SaveRoute(ctx context.Context, userID string, r SavedRoute) error {
	if err := r.Validate(); err != nil {
		return err
	}
	return s.store.SaveRoute(ctx, userID, r)
}

func (s *Service) Routes(ctx context.Context, userID string) ([]SavedRoute, error) {
	return s.store.Routes(ctx, userID)
}

func (s *Service) DeleteRoute(ctx context.Context, userID, name string) error {
	return s.store.DeleteRoute(ctx, userID, name)
}

// Start prices a saved route and opens a payment for the WHOLE fare.
func (s *Service) Start(ctx context.Context, userID, routeName string) (*Journey, *payments.Intent, error) {
	r, err := s.store.Route(ctx, userID, routeName)
	if err != nil {
		return nil, nil, err
	}
	legs, total := s.fares.Price(ctx, r.Legs)
	now := time.Now()
	j := &Journey{
		ID: utils.NewID("jr"), UserID: userID, RouteName: r.Name, Status: JourneyPending,
		Legs: legs, Total: total, CreatedAt: now, ValidUntil: now.Add(s.qrTTL),
	}
	if err := s.store.SaveJourney(ctx, j); err != nil {
		return nil, nil, err
	}
	intent, err := s.payments.CreateIntent(ctx, "transit", j.ID, userID, total)
	if err != nil {
		return nil, nil, err
	}
	j.PaymentID = intent.ID
	_ = s.store.SaveJourney(ctx, j)
	return j, intent, nil
}

// onPaid activates the journey; the app then fetches its QR.
func (s *Service) onPaid(ctx context.Context, in payments.Intent) error {
	j, _, err := s.store.Journey(ctx, in.RefID)
	if err != nil {
		return err
	}
	j.Status = JourneyActive
	j.ValidUntil = time.Now().Add(s.qrTTL) // validity starts at payment
	if err := s.store.SaveJourney(ctx, j); err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `INSERT INTO transit_journeys (id, user_id, route_name, total, paid_at) VALUES ($1,$2,$3,$4,now())`,
		j.ID, j.UserID, j.RouteName, j.Total)
	return err
}

// QR returns a fresh rotating QR for an active journey of the user.
func (s *Service) QR(ctx context.Context, userID, journeyID string) (*QR, error) {
	j, _, err := s.store.Journey(ctx, journeyID)
	if err != nil {
		return nil, err
	}
	if j.UserID != userID {
		return nil, apperr.Forbidden("not your journey")
	}
	if j.Status != JourneyActive {
		return nil, apperr.Conflict("journey is " + j.Status)
	}
	if time.Now().After(j.ValidUntil) {
		_ = s.store.SetStatus(ctx, j.ID, JourneyExpired)
		return nil, apperr.Conflict("journey expired")
	}
	return s.gen.Issue(ctx, j)
}

// Scan is called by train/metro gates, bus conductors and cab drivers.
func (s *Service) Scan(ctx context.Context, req ScanRequest) (*ScanResult, error) {
	res, err := s.validator.Validate(ctx, req)
	if err == nil && res.Allowed {
		s.stats.Count(ctx, analytics.MetricQRScans, 1)
		_, _ = s.db.Exec(ctx, `INSERT INTO transit_settlements (journey_id, leg, operator, mode, amount) VALUES ($1,$2,$3,$4,$5)`,
			res.JourneyID, res.Leg, req.OperatorID, req.Mode, res.Fare)
	}
	return res, err
}

func (s *Service) Journey(ctx context.Context, userID, id string) (*Journey, error) {
	j, _, err := s.store.Journey(ctx, id)
	if err != nil {
		return nil, err
	}
	if j.UserID != userID {
		return nil, apperr.Forbidden("not your journey")
	}
	return j, nil
}
