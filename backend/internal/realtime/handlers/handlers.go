// Package handlers wires Sarathi features to inbound WebSocket messages.
// realtime = transport, geo/dispatch = business logic, this = the glue.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"sarathi/backend/internal/dispatch"
	"sarathi/backend/internal/geo"
	"sarathi/backend/internal/realtime"
)

type Deps struct {
	Drivers  *geo.DriverIndex
	Stations *geo.StationTracker
	Dispatch *dispatch.Service
	Pub      *realtime.Publisher
	Log      *slog.Logger
}

func Register(r *realtime.Router, d Deps) {
	r.On(realtime.TypeLocationUpdate, d.locationUpdate)
	r.On(realtime.TypeDriverAvailability, d.driverAvailability)
	r.On(realtime.TypeOfferAccept, d.offerAccept)
	r.On(realtime.TypeOfferDecline, d.offerDecline)
}

// ---------------------------------------------------------------------------
// location.update  (drivers, bus drivers, and private cars in Map mode)
//
//	{"type":"location.update","data":{"lat":28.61,"lng":77.20,"heading":90,
//	 "speedKmh":32,"accuracyM":8,"tripId":"7f3a"}}
// ---------------------------------------------------------------------------

type locationMsg struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Heading   float64 `json:"heading"`
	SpeedKmh  float64 `json:"speedKmh"`
	AccuracyM float64 `json:"accuracyM"`
	TripID    string  `json:"tripId,omitempty"`
}

const minLocationInterval = 900 * time.Millisecond // ~1 update/second per socket

func (d Deps) locationUpdate(ctx context.Context, c *realtime.Client, env realtime.Envelope) error {
	var m locationMsg
	if err := json.Unmarshal(env.Data, &m); err != nil {
		return realtime.ErrBadPayload
	}
	if m.Lat < -90 || m.Lat > 90 || m.Lng < -180 || m.Lng > 180 || (m.Lat == 0 && m.Lng == 0) {
		return realtime.ErrBadPayload
	}
	if !c.AllowLocation(minLocationInterval) {
		return realtime.ErrRateLimited
	}
	if m.AccuracyM > 100 {
		return nil // too inaccurate to use; silently ignore
	}

	switch c.ID.Role {
	case realtime.RoleDriver:
		if c.ID.VehicleClass == "" {
			return realtime.NewClientError("no_vehicle", "driver has no vehicle class")
		}
		if err := d.Drivers.Update(ctx, c.ID.VehicleClass, c.ID.UserID, m.Lat, m.Lng); err != nil {
			return err
		}
		// Forward to passengers only if this driver really is assigned to that trip.
		if m.TripID != "" {
			if assigned, err := d.Dispatch.AssignedDriver(ctx, m.TripID); err == nil && assigned == c.ID.UserID {
				_ = d.Pub.Publish(ctx, realtime.TripTopic(m.TripID), realtime.TypeDriverLocation, map[string]any{
					"tripId": m.TripID, "lat": m.Lat, "lng": m.Lng, "heading": m.Heading, "speedKmh": m.SpeedKmh,
				})
			}
		}

	case realtime.RoleBusDriver:
		if c.ID.VehicleID == "" {
			return realtime.NewClientError("no_vehicle", "bus driver has no bus assigned")
		}
		_ = d.Pub.Publish(ctx, realtime.BusTopic(c.ID.VehicleID), realtime.TypeBusLocation, map[string]any{
			"busId": c.ID.VehicleID, "lat": m.Lat, "lng": m.Lng, "heading": m.Heading, "speedKmh": m.SpeedKmh,
		})
	}

	// Every car feeds the fuel/EV station crowd rating.
	vehicle := c.ID.VehicleID
	if vehicle == "" {
		vehicle = c.ID.UserID
	}
	if c.ID.Role != realtime.RoleBusDriver && m.AccuracyM <= 30 {
		if err := d.Stations.Observe(ctx, vehicle, m.Lat, m.Lng, m.SpeedKmh); err != nil {
			d.Log.Warn("station observe", "err", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// driver.availability   {"type":"driver.availability","data":{"online":true}}
// ---------------------------------------------------------------------------

func (d Deps) driverAvailability(ctx context.Context, c *realtime.Client, env realtime.Envelope) error {
	if c.ID.Role != realtime.RoleDriver {
		return realtime.ErrForbidden
	}
	var m struct {
		Online bool `json:"online"`
	}
	if err := json.Unmarshal(env.Data, &m); err != nil {
		return realtime.ErrBadPayload
	}
	if !m.Online {
		_ = d.Drivers.Remove(ctx, c.ID.VehicleClass, c.ID.UserID)
	}
	return d.Drivers.SetAvailability(ctx, c.ID.UserID, m.Online)
}

// ---------------------------------------------------------------------------
// offer.accept / offer.decline   {"type":"offer.accept","ref":"a1","data":{"tripId":"7f3a"}}
// ---------------------------------------------------------------------------

type offerMsg struct {
	TripID string `json:"tripId"`
}

func (d Deps) offerAccept(ctx context.Context, c *realtime.Client, env realtime.Envelope) error {
	if c.ID.Role != realtime.RoleDriver {
		return realtime.ErrForbidden
	}
	var m offerMsg
	if err := json.Unmarshal(env.Data, &m); err != nil || m.TripID == "" {
		return realtime.ErrBadPayload
	}
	return d.Dispatch.Accept(ctx, m.TripID, c.ID.UserID)
}

func (d Deps) offerDecline(ctx context.Context, c *realtime.Client, env realtime.Envelope) error {
	if c.ID.Role != realtime.RoleDriver {
		return realtime.ErrForbidden
	}
	var m offerMsg
	if err := json.Unmarshal(env.Data, &m); err != nil || m.TripID == "" {
		return realtime.ErrBadPayload
	}
	return d.Dispatch.Decline(ctx, m.TripID, c.ID.UserID)
}
