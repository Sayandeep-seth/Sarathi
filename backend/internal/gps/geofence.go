package gps

import (
	"context"
	"log/slog"
	"time"

	"sarathi/internal/cache"
	"sarathi/internal/common/constants"
	"sarathi/internal/common/utils"
)

// StationVisitRecorder is implemented by petrol.Service.
type StationVisitRecorder interface {
	RecordVisit(ctx context.Context, stationID, vehicleID string, at time.Time) (bool, error)
}

// ZoneChecker is implemented by test_drive.Service (keeps test-drive cars inside their area).
type ZoneChecker interface {
	CheckVehicle(ctx context.Context, vehicleID string, p utils.Point) error
}

// Geofence turns pings into events using Redis GEOSEARCH against station positions.
// A car that is (almost) stopped within 60 m of a petrol station counts as a visit;
// those visits feed the GPS-based crowd rating.
type Geofence struct {
	geo      *cache.Geo
	stations StationVisitRecorder
	zones    ZoneChecker
	log      *slog.Logger
}

const (
	stationRadiusKm = 0.06
	stoppedSpeedKmh = 8.0
)

func NewGeofence(geo *cache.Geo, s StationVisitRecorder, z ZoneChecker, log *slog.Logger) *Geofence {
	return &Geofence{geo: geo, stations: s, zones: z, log: log}
}

func (g *Geofence) Evaluate(ctx context.Context, p Ping) {
	if g.stations != nil && p.SpeedKmh <= stoppedSpeedKmh {
		hits, err := g.geo.Nearby(ctx, cache.KeyStationGeo(constants.StationPetrol), p.Location, stationRadiusKm, 1)
		if err == nil && len(hits) == 1 {
			if _, err := g.stations.RecordVisit(ctx, hits[0].Member, p.VehicleID, p.At); err != nil {
				g.log.Warn("record station visit", "err", err)
			}
		}
	}
	if g.zones != nil {
		if err := g.zones.CheckVehicle(ctx, p.VehicleID, p.Location); err != nil {
			g.log.Warn("zone check", "vehicle", p.VehicleID, "err", err)
		}
	}
}
