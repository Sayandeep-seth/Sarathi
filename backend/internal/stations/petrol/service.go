package petrol

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"sarathi/app/config"
	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
	"sarathi/internal/vehicles"
)

type Service struct {
	cache    *Cache
	tracker  *TrafficTracker
	rater    *CrowdRater
	repo     *Repository
	vehicles *vehicles.Service
	cfg      config.MapConfig
	log      *slog.Logger
}

func NewService(c *Cache, t *TrafficTracker, r *CrowdRater, repo *Repository, v *vehicles.Service, cfg config.MapConfig, log *slog.Logger) *Service {
	return &Service{cache: c, tracker: t, rater: r, repo: repo, vehicles: v, cfg: cfg, log: log}
}

// AlongRouteRequest: car model + fuel in the tank + the chosen route.
type AlongRouteRequest struct {
	CarName    string        `json:"car_name"`
	FuelAmount float64       `json:"fuel_amount"`
	FuelUnit   string        `json:"fuel_unit"` // litres | percent
	Route      []utils.Point `json:"route"`
}

type AlongRouteResult struct {
	RangeKm       float64   `json:"range_km"`
	UsableRangeKm float64   `json:"usable_range_km"`
	RouteKm       float64   `json:"route_km"`
	NeedsRefuel   bool      `json:"needs_refuel"`
	Best          *Station  `json:"best,omitempty"`
	Stations      []Station `json:"stations"`
}

// AlongRoute lists petrol stations during the journey that the car can reach with
// the fuel it has, marks the best one, and says whether a refuel is required.
func (s *Service) AlongRoute(ctx context.Context, req AlongRouteRequest) (*AlongRouteResult, error) {
	if len(req.Route) < 2 {
		return nil, apperr.BadRequest("route polyline is required")
	}
	spec, err := s.vehicles.GetSpec(ctx, req.CarName)
	if err != nil {
		return nil, err
	}
	if spec.IsEV() {
		return nil, apperr.BadRequest("this is an EV, use /api/v1/stations/ev/along-route")
	}
	rng, err := spec.RangeFor(req.FuelAmount, req.FuelUnit)
	if err != nil {
		return nil, err
	}
	res := &AlongRouteResult{
		RangeKm:       utils.Round(rng, 1),
		UsableRangeKm: utils.Round(rng*(1-s.cfg.ReserveFuelPct/100), 1),
		RouteKm:       utils.Round(utils.PolylineKm(req.Route), 1),
	}
	res.NeedsRefuel = res.RouteKm > res.UsableRangeKm

	samples := utils.SamplePolyline(req.Route, s.cfg.StationSampleEveryKm)
	hits, err := s.cache.AlongRoute(ctx, samples, s.cfg.StationSearchRadiusKm)
	if err != nil {
		return nil, err
	}
	stations, err := s.enrich(ctx, hits)
	if err != nil {
		return nil, err
	}
	for i := range stations {
		st := &stations[i]
		st.Reachable = st.AlongRouteKm+st.DistKm <= res.UsableRangeKm
		if fuel := pumpFuel(spec.FuelType); fuel != "" && len(st.Fuels) > 0 && !contains(st.Fuels, fuel) {
			st.Reachable = false
		}
		st.Score = Score(*st)
	}
	sort.Slice(stations, func(i, j int) bool { return stations[i].AlongRouteKm < stations[j].AlongRouteKm })
	for i := range stations {
		if stations[i].Reachable && (res.Best == nil || stations[i].Score > res.Best.Score) {
			b := stations[i]
			res.Best = &b
		}
	}
	res.Stations = stations
	return res, nil
}

// Nearby returns stations around a point sorted by crowd-score ("best petrol pump").
func (s *Service) Nearby(ctx context.Context, p utils.Point, radiusKm float64) ([]Station, error) {
	if !p.Valid() {
		return nil, apperr.BadRequest("valid lat/lng required")
	}
	hits, err := s.cache.Nearby(ctx, p, radiusKm, 30)
	if err != nil {
		return nil, err
	}
	rh := make([]cache.RouteHit, len(hits))
	for i, h := range hits {
		rh[i] = cache.RouteHit{GeoHit: h}
	}
	out, err := s.enrich(ctx, rh)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Score = Score(out[i])
		out[i].Reachable = true
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}

func (s *Service) enrich(ctx context.Context, hits []cache.RouteHit) ([]Station, error) {
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.Member
	}
	details, err := s.cache.Details(ctx, ids)
	if err != nil {
		return nil, err
	}
	now, err := s.tracker.CarsNow(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Station, 0, len(hits))
	for _, h := range hits {
		st, ok := details[h.Member]
		if !ok {
			continue
		}
		st.DistKm = utils.Round(h.DistKm, 2)
		st.AlongRouteKm = utils.Round(h.AlongKm, 1)
		st.CarsNow = now[h.Member]
		if st.Rating == 0 {
			st.Rating = 1
		}
		out = append(out, st)
	}
	return out, nil
}

// Busy returns the station's hourly visit counts for the last 24 h.
func (s *Service) Busy(ctx context.Context, id string) ([]int64, error) {
	return s.tracker.Hourly(ctx, id, 24, time.Now())
}

// RecordVisit implements gps.StationVisitRecorder.
func (s *Service) RecordVisit(ctx context.Context, stationID, vehicleID string, at time.Time) (bool, error) {
	return s.tracker.RecordVisit(ctx, stationID, vehicleID, at)
}

// Upsert saves a station to Postgres and Redis.
func (s *Service) Upsert(ctx context.Context, st Station) error {
	if st.ID == "" || !st.Location.Valid() {
		return apperr.BadRequest("id and location required")
	}
	if err := s.repo.Upsert(ctx, st); err != nil {
		return err
	}
	return s.cache.Put(ctx, st)
}

// WarmUp loads every station from Postgres into Redis (boot / nightly).
func (s *Service) WarmUp(ctx context.Context) error {
	all, err := s.repo.All(ctx)
	if err != nil || len(all) == 0 {
		return err
	}
	for i := 0; i < len(all); i += 1000 {
		if err := s.cache.Put(ctx, all[i:min(i+1000, len(all))]...); err != nil {
			return err
		}
	}
	s.log.Info("petrol stations cached", "count", len(all))
	return nil
}

// RecomputeRatings is called by the worker.
func (s *Service) RecomputeRatings(ctx context.Context) error {
	return s.rater.RecomputeAll(ctx, time.Now())
}

// pumpFuel maps a car's fuel type to what the pump must sell (hybrids fill petrol).
func pumpFuel(carFuel string) string {
	if carFuel == "hybrid" {
		return "petrol"
	}
	return carFuel
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
