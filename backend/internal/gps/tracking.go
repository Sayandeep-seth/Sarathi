// Package gps ingests location pings from driver/vehicle apps, keeps live positions
// in Redis, streams trip paths, and fires geofence events (petrol station visits,
// test-drive zones).
package gps

import (
	"time"

	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// Ping is one GPS sample.
type Ping struct {
	VehicleID string      `json:"vehicle_id"`
	DriverID  string      `json:"driver_id,omitempty"`
	TripID    string      `json:"trip_id,omitempty"`
	RoadID    string      `json:"road_id,omitempty"` // from on-device map matching
	Location  utils.Point `json:"location"`
	SpeedKmh  float64     `json:"speed_kmh"`
	Heading   float64     `json:"heading"`
	AccuracyM float64     `json:"accuracy_m"`
	At        time.Time   `json:"at"`
}

// Validate rejects junk pings (bad coordinates, poor accuracy, clock skew).
func (p *Ping) Validate() error {
	if p.VehicleID == "" {
		return apperr.BadRequest("vehicle_id required")
	}
	if !p.Location.Valid() {
		return apperr.BadRequest("invalid location")
	}
	if p.AccuracyM > 100 {
		return apperr.BadRequest("location accuracy too low")
	}
	now := time.Now()
	if p.At.IsZero() || p.At.After(now.Add(30*time.Second)) {
		p.At = now
	}
	if now.Sub(p.At) > 10*time.Minute {
		return apperr.BadRequest("ping too old")
	}
	return nil
}
