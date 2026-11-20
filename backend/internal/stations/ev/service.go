package ev

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"sarathi/app/config"
	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
	"sarathi/internal/vehicles"
)

type Service struct {
	cache    *Cache
	repo     *Repository
	vehicles *vehicles.Service
	cfg      config.MapConfig
	log      *slog.Logger
}

func NewService(c *Cache, repo *Repository, v *vehicles.Service, cfg config.MapConfig, log *slog.Logger) *Service {
	return &Service{cache: c, repo: repo, vehicles: v, cfg: cfg, log: log}
}

// AlongRouteRequest: EV model + battery left + the chosen route.
type AlongRouteRequest struct {
	CarName      string        `json:"car_name"`
	BatteryLevel float64       `json:"battery_level"`
	BatteryUnit  string        `json:"battery_unit"` // percent | kwh
	Route        []utils.Point `json:"route"`
}

type AlongRouteResult struct {
	RangeKm       float64   `json:"range_km"`
	UsableRangeKm float64   `json:"usable_range_km"`
	RouteKm       float64   `json:"route_km"`
	NeedsCharge   bool      `json:"needs_charge"`
	Recommended   *Charger  `json:"recommended,omitempty"`
	Chargers      []Charger `json:"chargers"`
}

func (s *Service) AlongRoute(ctx context.Context, req AlongRouteRequest) (*AlongRouteResult, error) {
	if len(req.Route) < 2 {
		return nil, apperr.BadRequest("route polyline is required")
	}
	if req.BatteryUnit == "" {
		req.BatteryUnit = "percent"
	}
	spec, err := s.vehicles.GetSpec(ctx, req.CarName)
	if err != nil {
		return nil, err
	}
	if !spec.IsEV() {
		return nil, apperr.BadRequest("not an EV, use /api/v1/stations/petrol/along-route")
	}
	rng, err := spec.RangeFor(req.BatteryLevel, req.BatteryUnit)
	if err != nil {
		return nil, err
	}
	res := &AlongRouteResult{
		RangeKm:       utils.Round(rng, 1),
		UsableRangeKm: utils.Round(rng*(1-s.cfg.ReserveFuelPct/100), 1),
		RouteKm:       utils.Round(utils.PolylineKm(req.Route), 1),
	}
	res.NeedsCharge = res.RouteKm > res.UsableRangeKm

	hits, err := s.cache.AlongRoute(ctx, utils.SamplePolyline(req.Route, s.cfg.StationSampleEveryKm), s.cfg.StationSearchRadiusKm)
	if err != nil {
		return nil, err
	}
	list, err := s.enrich(ctx, hits, spec)
	if err != nil {
		return nil, err
	}
	for i := range list {
		c := &list[i]
		c.Reachable = c.AlongRouteKm+c.DistKm <= res.UsableRangeKm
		// battery % on arrival -> minutes to reach 80 %
		usedPct := (c.AlongRouteKm + c.DistKm) / spec.FullRangeKm * 100
		arrivalPct := currentPct(req, spec) - usedPct
		if c.PowerKW > 0 && arrivalPct < 80 {
			c.ChargeMinutes = utils.Round((80-arrivalPct)/100*spec.BatteryKWh/c.PowerKW*60, 0)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].AlongRouteKm < list[j].AlongRouteKm })
	// recommend: the farthest reachable compatible charger with a free port (fewest stops)
	for i := range list {
		c := list[i]
		if c.Reachable && c.Compatible && c.FreePorts != 0 {
			res.Recommended = &c
		}
	}
	res.Chargers = list
	return res, nil
}

func currentPct(req AlongRouteRequest, spec *vehicles.Spec) float64 {
	if req.BatteryUnit == "kwh" && spec.BatteryKWh > 0 {
		return req.BatteryLevel / spec.BatteryKWh * 100
	}
	return req.BatteryLevel
}

// Nearby lists chargers around a point; when car is given, only compatible ones first.
func (s *Service) Nearby(ctx context.Context, p utils.Point, radiusKm float64, carName string) ([]Charger, error) {
	if !p.Valid() {
		return nil, apperr.BadRequest("valid lat/lng required")
	}
	var spec *vehicles.Spec
	if carName != "" {
		var err error
		if spec, err = s.vehicles.GetSpec(ctx, carName); err != nil {
			return nil, err
		}
	}
	hits, err := s.cache.Nearby(ctx, p, radiusKm, 30)
	if err != nil {
		return nil, err
	}
	rh := make([]cache.RouteHit, len(hits))
	for i, h := range hits {
		rh[i] = cache.RouteHit{GeoHit: h}
	}
	list, err := s.enrich(ctx, rh, spec)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Compatible != list[j].Compatible {
			return list[i].Compatible
		}
		return list[i].DistKm < list[j].DistKm
	})
	return list, nil
}

func (s *Service) enrich(ctx context.Context, hits []cache.RouteHit, spec *vehicles.Spec) ([]Charger, error) {
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.Member
	}
	details, err := s.cache.Details(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Charger, 0, len(hits))
	for _, h := range hits {
		c, ok := details[h.Member]
		if !ok {
			continue
		}
		c.DistKm, c.AlongRouteKm = utils.Round(h.DistKm, 2), utils.Round(h.AlongKm, 1)
		c.Compatible = spec == nil || spec.ConnectorType == "" || hasConnector(c.Connectors, spec.ConnectorType)
		out = append(out, c)
	}
	return out, nil
}

// UpdatePorts is called by the operator webhook.
func (s *Service) UpdatePorts(ctx context.Context, id string, free, total int) error {
	if free < 0 || total <= 0 || free > total {
		return apperr.BadRequest("invalid port counts")
	}
	return s.cache.SetPorts(ctx, id, free, total)
}

func (s *Service) Upsert(ctx context.Context, c Charger) error {
	if c.ID == "" || !c.Location.Valid() {
		return apperr.BadRequest("id and location required")
	}
	if err := s.repo.Upsert(ctx, c); err != nil {
		return err
	}
	return s.cache.Put(ctx, c)
}

// WarmUp loads all chargers into Redis.
func (s *Service) WarmUp(ctx context.Context) error {
	all, err := s.repo.All(ctx)
	if err != nil || len(all) == 0 {
		return err
	}
	if err := s.cache.Put(ctx, all...); err != nil {
		return err
	}
	s.log.Info("ev chargers cached", "count", len(all))
	return nil
}

func hasConnector(list []string, want string) bool {
	for _, c := range list {
		if strings.EqualFold(c, want) {
			return true
		}
	}
	return false
}
