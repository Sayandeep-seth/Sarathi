// Package simulator drives a demo fleet so the platform is usable without real
// drivers or gateways. Simulated cabs wander the grid, accept bookings, drive to the
// pickup and drop, and report metered Wi-Fi usage — all through the same core
// functions real drivers and gateways use.
//
// It is enabled by SARATHI_SIMULATOR and defaults to off outside local. With several
// API replicas, a Redis lease ensures exactly one of them runs the fleet.
package simulator

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"time"

	"github.com/sarathi/sarathi/backend/internal/core"
	"github.com/sarathi/sarathi/backend/internal/geo"
	"github.com/sarathi/sarathi/backend/internal/live"
	"github.com/sarathi/sarathi/backend/internal/store"
)

const (
	tick     = time.Second
	leaseTTL = 5 * time.Second
)

var models = []struct {
	name     string
	seats    int
	electric bool
}{
	{"Maruti Suzuki Swift", 4, false},
	{"Tata Nexon EV", 4, true},
	{"Hyundai Creta", 4, false},
	{"Toyota Innova Crysta", 6, false},
	{"MG ZS EV", 4, true},
}

var drivers = []string{"Arjun", "Meera", "Imran", "Lakshmi", "Ravi", "Fatima", "Suresh", "Anita", "Joseph", "Kavya"}

type simCab struct {
	cab     store.Cab
	pos     geo.Point
	heading float64
	way     []geo.Node // remaining waypoints; way[0] is the next intersection
	tripID  string
	phase   string // trip status the current route was planned for
	dwell   int
}

// Simulator owns the simulated fleet state for the leading replica.
type Simulator struct {
	Core     *core.Core
	Log      *slog.Logger
	Cabs     int
	Rate     float64 // simulated seconds per real second
	instance string
	fleet    []*simCab
	seq      int
}

// Run ticks until ctx is cancelled.
func (s *Simulator) Run(ctx context.Context) {
	s.instance = fmt.Sprintf("%016x", rand.Uint64())
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			if err := s.step(tctx); err != nil && ctx.Err() == nil {
				s.Log.Warn("simulator_step_failed", slog.String("error", err.Error()))
			}
			cancel()
		}
	}
}

func (s *Simulator) step(ctx context.Context) error {
	leader, err := s.Core.Live.AcquireLeader(ctx, s.instance, leaseTTL)
	if err != nil {
		return err
	}
	if !leader {
		s.fleet = nil // another replica drives; reload from shared state if we take over
		return nil
	}
	if s.fleet == nil {
		if err := s.load(ctx); err != nil {
			return err
		}
		s.Log.Info("simulator_leading", slog.Int("cabs", len(s.fleet)))
	}

	trips, err := s.Core.Store.ActiveSimulatedTrips(ctx)
	if err != nil {
		return err
	}
	byCab := make(map[string]store.Trip, len(trips))
	for _, t := range trips {
		if t.CabID != nil {
			byCab[*t.CabID] = t
		}
	}

	now := time.Now().UnixMilli()
	positions := make([]live.CabPos, 0, len(s.fleet))
	tripByCab := map[string]string{}
	for _, c := range s.fleet {
		status := "available"
		if trip, ok := byCab[c.cab.ID]; ok {
			status = "on_trip"
			tripByCab[c.cab.ID] = trip.ID
			s.serve(ctx, c, trip)
		} else {
			c.tripID, c.phase = "", ""
			if len(c.way) == 0 {
				c.way = geo.Route(geo.NearestNode(c.pos), geo.Node{I: rand.IntN(geo.Size), J: rand.IntN(geo.Size)})
			}
			s.move(c)
		}
		lat, lng := geo.ToLatLng(c.pos)
		positions = append(positions, live.CabPos{ID: c.cab.ID, Lat: lat, Lng: lng, Heading: c.heading, Status: status, TS: now})
	}
	return s.Core.ReportPositions(ctx, positions, tripByCab)
}

// serve advances one cab along its trip: to the pickup, then to the drop.
func (s *Simulator) serve(ctx context.Context, c *simCab, trip store.Trip) {
	target := geo.NearestNode(geo.FromLatLng(trip.PickupLat, trip.PickupLng))
	if trip.Status == "in_progress" {
		target = geo.NearestNode(geo.FromLatLng(trip.DropLat, trip.DropLng))
	}
	if c.tripID != trip.ID || c.phase != trip.Status {
		start := geo.NearestNode(c.pos)
		if len(c.way) > 0 {
			start = c.way[0] // finish the current block before turning
		}
		c.way = geo.Route(start, target)
		c.tripID, c.phase, c.dwell = trip.ID, trip.Status, 3
	}
	s.move(c)

	if trip.Status == "in_progress" {
		s.seq++
		bytes := int64(200<<10) + rand.Int64N(1<<20)
		if _, err := s.Core.ReportUsage(ctx, trip.ID, fmt.Sprintf("sim-%s-%d-%d", s.instance, time.Now().UnixNano(), s.seq), bytes); err != nil {
			s.Log.Warn("simulator_usage_failed", slog.String("error", err.Error()))
		}
	}
	if len(c.way) > 0 {
		return
	}
	var err error
	switch trip.Status {
	case "assigned":
		if c.dwell--; c.dwell > 0 {
			return // passenger boarding
		}
		_, err = s.Core.StartTrip(ctx, trip.ID, nil)
	case "in_progress":
		_, err = s.Core.CompleteTrip(ctx, trip.ID)
	}
	if err != nil {
		s.Log.Warn("simulator_transition_failed", slog.String("trip", trip.ID), slog.String("error", err.Error()))
	}
}

// move advances a cab along its waypoints by one tick of travel.
func (s *Simulator) move(c *simCab) {
	remaining := geo.CruiseSpeed * s.Rate * tick.Seconds()
	for remaining > 0 && len(c.way) > 0 {
		t := c.way[0].Point()
		dx, dy := t.X-c.pos.X, t.Y-c.pos.Y
		dist := math.Hypot(dx, dy)
		if dist > 0.01 {
			// Compass bearing: 0 = north, 90 = east.
			c.heading = math.Mod(math.Atan2(dx, dy)*180/math.Pi+360, 360)
		}
		if dist <= remaining {
			c.pos, remaining, c.way = t, remaining-dist, c.way[1:]
			continue
		}
		c.pos.X += dx / dist * remaining
		c.pos.Y += dy / dist * remaining
		remaining = 0
	}
}

// load registers the simulated fleet and restores positions from shared state.
func (s *Simulator) load(ctx context.Context) error {
	for i := range s.Cabs {
		m := models[i%len(models)]
		if err := s.Core.Store.EnsureSimulatedCab(ctx, store.NewCab{
			Plate: fmt.Sprintf("KA 01 SM %04d", 1001+i), Model: m.name, Seats: m.seats, Electric: m.electric,
			DriverName: drivers[i%len(drivers)],
		}); err != nil {
			return err
		}
	}
	cabs, err := s.Core.Store.SimulatedCabs(ctx)
	if err != nil {
		return err
	}
	fleet := make([]*simCab, 0, s.Cabs)
	for i, cab := range cabs {
		if i >= s.Cabs && cab.Status != "on_trip" {
			// Fleet was shrunk: retire the extras (mid-trip ones finish their trip first).
			if cab.Status == "available" {
				_ = s.Core.Store.SetCabAvailability(ctx, cab.ID, false)
				_ = s.Core.Live.UpdateCabs(ctx, []live.CabPos{{ID: cab.ID, Status: "offline"}})
			}
			continue
		}
		if cab.Status == "offline" {
			if err := s.Core.Store.SetCabAvailability(ctx, cab.ID, true); err != nil {
				return err
			}
		}
		c := &simCab{cab: cab, pos: geo.Node{I: rand.IntN(geo.Size), J: rand.IntN(geo.Size)}.Point()}
		if p, ok := s.Core.Live.Cab(ctx, cab.ID); ok {
			c.pos, c.heading = geo.FromLatLng(p.Lat, p.Lng), p.Heading
		} else if cab.Lat != nil && cab.Lng != nil {
			c.pos, c.heading = geo.FromLatLng(*cab.Lat, *cab.Lng), cab.Heading
		}
		// Restored positions may be mid-block; snap so routes stay on the grid.
		c.pos = geo.NearestNode(c.pos).Point()
		fleet = append(fleet, c)
	}
	s.fleet = fleet
	return nil
}
