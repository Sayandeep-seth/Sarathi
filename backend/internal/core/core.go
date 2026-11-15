// Package core implements the trip and Wi-Fi business rules on top of the stores.
// HTTP handlers, the simulator and gateway reports all go through it, so there is one
// code path for every state change.
package core

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/sarathi/sarathi/backend/internal/auth"
	"github.com/sarathi/sarathi/backend/internal/geo"
	"github.com/sarathi/sarathi/backend/internal/live"
	"github.com/sarathi/sarathi/backend/internal/store"
	"github.com/sarathi/sarathi/backend/internal/telemetry"
)

// Errors surfaced to callers.
var (
	ErrSamePlace        = errors.New("pickup and drop must differ")
	ErrUnknownPlace     = errors.New("unknown place")
	ErrPaymentsDisabled = errors.New("payments are not enabled")
	ErrBadBoardingCode  = errors.New("boarding code does not match")
	ErrTripNotMetered   = errors.New("trip is not in progress")
)

// Core bundles the dependencies of the business layer.
type Core struct {
	Store           *store.Store
	Live            *live.Live
	Tel             *telemetry.Store
	Log             *slog.Logger
	Secret          []byte
	PaymentProvider string
}

// Book prices the trip, finds the nearest available cab and assigns it.
func (c *Core) Book(ctx context.Context, userID, pickupID, dropID, idemKey string) (store.Trip, bool, error) {
	from, ok1 := geo.PlaceByID(pickupID)
	to, ok2 := geo.PlaceByID(dropID)
	if !ok1 || !ok2 {
		return store.Trip{}, false, ErrUnknownPlace
	}
	if from.ID == to.ID {
		return store.Trip{}, false, ErrSamePlace
	}
	candidates, err := c.Live.NearestAvailable(ctx, from.Lat, from.Lng, 25)
	if err != nil {
		return store.Trip{}, false, err
	}
	est := geo.Plan(from, to)
	trip, replayed, err := c.Store.BookTrip(ctx, store.NewTrip{
		PassengerID: userID, PickupPlace: from.Name, DropPlace: to.Name,
		PickupLat: from.Lat, PickupLng: from.Lng, DropLat: to.Lat, DropLng: to.Lng,
		DistanceM: est.DistanceM, DurationS: est.DurationS, FarePaise: est.FarePaise,
		IdempotencyKey: idemKey, CandidateCabs: candidates,
	})
	if err != nil {
		return trip, false, err
	}
	if trip, err = c.Store.Trip(ctx, trip.ID); err != nil {
		return trip, false, err
	}
	if !replayed {
		c.afterChange(ctx, trip, "assigned")
	}
	trip.BoardingCode = auth.BoardingCode(c.Secret, trip.ID)
	return trip, replayed, nil
}

// StartTrip moves an assigned trip to in_progress. When code is non-nil it must match
// the passenger's boarding code (driver flow); the simulator passes nil.
func (c *Core) StartTrip(ctx context.Context, tripID string, code *string) (store.Trip, error) {
	if code != nil && !auth.CheckBoardingCode(c.Secret, tripID, *code) {
		return store.Trip{}, ErrBadBoardingCode
	}
	return c.transition(ctx, tripID, "assigned", "in_progress")
}

// CompleteTrip ends an in-progress trip and frees the cab.
func (c *Core) CompleteTrip(ctx context.Context, tripID string) (store.Trip, error) {
	return c.transition(ctx, tripID, "in_progress", "completed")
}

// CancelTrip cancels a trip that has not started and frees the cab.
func (c *Core) CancelTrip(ctx context.Context, tripID string) (store.Trip, error) {
	return c.transition(ctx, tripID, "assigned", "cancelled")
}

func (c *Core) transition(ctx context.Context, tripID, from, to string) (store.Trip, error) {
	trip, err := c.Store.Transition(ctx, tripID, from, to)
	if err != nil {
		return trip, err
	}
	c.afterChange(ctx, trip, to)
	return trip, nil
}

// afterChange fans a committed trip change out to Redis, MongoDB and subscribers.
// Failures here are logged, never returned: the database commit already happened.
func (c *Core) afterChange(ctx context.Context, trip store.Trip, event string) {
	c.Tel.RecordEvent(trip.ID, event, map[string]any{"status": trip.Status})

	if trip.Cab != nil {
		if pos, ok := c.Live.Cab(ctx, trip.Cab.ID); ok && pos.Status != trip.Cab.Status {
			pos.Status, pos.TS = trip.Cab.Status, time.Now().UnixMilli()
			if err := c.Live.UpdateCabs(ctx, []live.CabPos{pos}); err != nil {
				c.Log.Warn("cab_status_sync_failed", slog.String("error", err.Error()))
			}
		}
		if trip.Cab.DriverID != nil {
			c.publish(ctx, live.UserTopic(*trip.Cab.DriverID), map[string]any{"type": "driver_trip", "trip": trip})
		}
	}
	mine := trip
	mine.BoardingCode = auth.BoardingCode(c.Secret, trip.ID)
	c.publish(ctx, live.UserTopic(trip.PassengerID), map[string]any{"type": "trip", "trip": mine})
}

func (c *Core) publish(ctx context.Context, topic string, ev any) {
	if err := c.Live.Publish(ctx, topic, ev); err != nil {
		c.Log.Warn("publish_failed", slog.String("topic", topic), slog.String("error", err.Error()))
	}
}

// ReportPositions records live cab positions: Redis for the hot path and fan-out,
// MongoDB for history, and PostgreSQL at a throttled rate for durability.
// tripByCab maps cab id to its active trip id, when known.
func (c *Core) ReportPositions(ctx context.Context, cabs []live.CabPos, tripByCab map[string]string) error {
	if err := c.Live.UpdateCabs(ctx, cabs); err != nil {
		return err
	}
	for _, p := range cabs {
		c.Tel.RecordPosition(telemetry.Position{
			CabID: p.ID, TripID: tripByCab[p.ID], Lat: p.Lat, Lng: p.Lng, Heading: p.Heading, TS: time.UnixMilli(p.TS).UTC(),
		})
		if c.Live.ShouldPersist(ctx, p.ID, 10*time.Second) {
			if err := c.Store.SaveCabPosition(ctx, p.ID, p.Lat, p.Lng, p.Heading); err != nil {
				c.Log.Warn("cab_position_persist_failed", slog.String("error", err.Error()))
			}
		}
	}
	return nil
}

// Usage is the result of a metering report.
type Usage struct {
	DebitedBytes   int64 `json:"debited_bytes"`
	RemainingBytes int64 `json:"remaining_bytes"`
	// Allow tells the gateway whether the passenger may keep using the network.
	Allow bool `json:"allow"`
}

// ReportUsage debits metered bytes for an in-progress trip. Reports are idempotent
// by reportID; a replay returns the current balance without debiting again.
func (c *Core) ReportUsage(ctx context.Context, tripID, reportID string, bytes int64) (Usage, error) {
	trip, err := c.Store.Trip(ctx, tripID)
	if err != nil {
		return Usage{}, err
	}
	if trip.Status != "in_progress" {
		return Usage{}, ErrTripNotMetered
	}
	debited, remaining, err := c.Store.DebitUsage(ctx, trip.PassengerID, trip.ID, reportID, bytes)
	if errors.Is(err, store.ErrDuplicateRepo) {
		return Usage{RemainingBytes: remaining, Allow: remaining > 0}, nil
	}
	if err != nil {
		return Usage{}, err
	}
	u := Usage{DebitedBytes: debited, RemainingBytes: remaining, Allow: remaining > 0}
	if debited > 0 {
		c.publish(ctx, live.UserTopic(trip.PassengerID), map[string]any{
			"type": "quota", "remaining_bytes": remaining, "debited_bytes": debited, "trip_id": trip.ID,
		})
	}
	return u, nil
}

// PurchasePlan charges the configured provider and credits the plan's quota.
// Only the non-production mock provider exists; with provider "none" purchases are
// refused rather than granted for free.
func (c *Core) PurchasePlan(ctx context.Context, userID, planID string) (int64, store.Plan, error) {
	plan, err := c.Store.Plan(ctx, planID)
	if err != nil {
		return 0, plan, err
	}
	if c.PaymentProvider != "mock" {
		return 0, plan, ErrPaymentsDisabled
	}
	balance, err := c.Store.GrantPlan(ctx, userID, plan, "mock", "mock-"+time.Now().UTC().Format("20060102T150405.000000000"))
	if err != nil {
		return 0, plan, err
	}
	c.publish(ctx, live.UserTopic(userID), map[string]any{"type": "quota", "remaining_bytes": balance})
	return balance, plan, nil
}
