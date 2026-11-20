package map_service

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"sarathi/internal/ai"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// RouteRequest: starting location, destination and car name.
type RouteRequest struct {
	Origin      utils.Point `json:"origin"`
	Destination utils.Point `json:"destination"`
	CarName     string      `json:"car_name"`
}

// RoadCheck is one road on a route with its fit result and live traffic.
type RoadCheck struct {
	RoadAttr
	LengthKm     float64      `json:"length_km"`
	Compat       CompatResult `json:"compatibility"`
	AvgSpeedKmh  float64      `json:"avg_speed_kmh,omitempty"`
	TrafficLevel float64      `json:"traffic_level"`
}

// Route is a candidate route evaluated for the user's car.
type Route struct {
	Index        int           `json:"index"`
	DistanceKm   float64       `json:"distance_km"`
	BaseMin      float64       `json:"base_duration_min"`
	PredictedMin float64       `json:"predicted_duration_min"`
	TrafficLevel float64       `json:"traffic_level"`
	Passable     bool          `json:"passable"`
	Blockers     []string      `json:"blockers,omitempty"`
	Roads        []RoadCheck   `json:"roads"`
	Polyline     []utils.Point `json:"polyline"`
}

// RouteResponse lists every possible route, passable ones first.
type RouteResponse struct {
	Car    Envelope `json:"car"`
	Routes []Route  `json:"routes"`
	Cached bool     `json:"cached"`
}

type Service struct {
	engine    RoutingEngine
	routes    *RouteCache
	carSpecs  *CarSpecCache
	roads     *RoadAnalyzer
	compat    *Compatibility
	traffic   *Traffic
	predictor *ai.RoutePredictor
	repo      *Repository
	maxAlt    int
	log       *slog.Logger
}

func NewService(engine RoutingEngine, rc *RouteCache, cs *CarSpecCache, ra *RoadAnalyzer, c *Compatibility,
	t *Traffic, p *ai.RoutePredictor, repo *Repository, maxAlt int, log *slog.Logger) *Service {
	return &Service{engine: engine, routes: rc, carSpecs: cs, roads: ra, compat: c, traffic: t, predictor: p, repo: repo, maxAlt: maxAlt, log: log}
}

// PlanRoutes returns all routes to the destination and whether the given car can
// drive each one (two-way: an oncoming car can pass; one-way: the car fits).
func (s *Service) PlanRoutes(ctx context.Context, userID string, req RouteRequest) (*RouteResponse, error) {
	if !req.Origin.Valid() || !req.Destination.Valid() {
		return nil, apperr.BadRequest("origin and destination are required")
	}
	env, err := s.carSpecs.Envelope(ctx, req.CarName)
	if err != nil {
		return nil, err
	}
	if cached, ok := s.routes.GetForCar(ctx, req.Origin, req.Destination, env.Model); ok {
		return cached, nil
	}

	raws, err := s.routes.Raw(ctx, req.Origin, req.Destination, func(ctx context.Context) ([]RawRoute, error) {
		return s.engine.Alternatives(ctx, req.Origin, req.Destination, s.maxAlt)
	})
	if err != nil {
		return nil, apperr.Unavailable("routing engine unavailable")
	}

	// gather every road once across all alternatives
	names := map[string]string{}
	var ids []string
	for _, r := range raws {
		for _, id := range r.RoadIDs {
			if _, ok := names[id]; !ok {
				names[id] = r.RoadNames[id]
				ids = append(ids, id)
			}
		}
	}
	attrs, err := s.roads.Attributes(ctx, ids, names)
	if err != nil {
		return nil, err
	}
	list := make([]RoadAttr, 0, len(attrs))
	for _, a := range attrs {
		list = append(list, a)
	}
	compat, err := s.compat.CheckRoads(ctx, env, list)
	if err != nil {
		return nil, err
	}
	speeds, _ := s.traffic.AvgSpeeds(ctx, ids)

	resp := &RouteResponse{Car: env}
	for i, raw := range raws {
		rt := Route{Index: i, DistanceKm: utils.Round(raw.DistanceKm, 2), BaseMin: utils.Round(raw.DurationMin, 1), Passable: true, Polyline: raw.Polyline}
		var weighted, narrowKm float64
		for _, id := range raw.RoadIDs {
			a, c := attrs[id], compat[id]
			rc := RoadCheck{RoadAttr: a, LengthKm: utils.Round(raw.RoadKm[id], 2), Compat: c}
			if sp, ok := speeds[id]; ok {
				rc.AvgSpeedKmh = utils.Round(sp, 1)
				rc.TrafficLevel = Level(sp, a.Class)
			}
			weighted += rc.TrafficLevel * raw.RoadKm[id]
			if c.MarginM < 0.5 {
				narrowKm += raw.RoadKm[id]
			}
			if !c.Passable {
				rt.Passable = false
				rt.Blockers = append(rt.Blockers, a.Name+": "+c.Reason)
			}
			rt.Roads = append(rt.Roads, rc)
		}
		if raw.DistanceKm > 0 {
			rt.TrafficLevel = utils.Round(weighted/raw.DistanceKm, 2)
		}
		rt.PredictedMin = s.predictor.PredictETA(ctx, ai.RouteFeatures{
			DistanceKm: raw.DistanceKm, BaseDurationMin: raw.DurationMin, TrafficLevel: rt.TrafficLevel,
			NarrowRoadKm: narrowKm, At: time.Now(),
		})
		resp.Routes = append(resp.Routes, rt)
	}
	sort.SliceStable(resp.Routes, func(i, j int) bool {
		a, b := resp.Routes[i], resp.Routes[j]
		if a.Passable != b.Passable {
			return a.Passable
		}
		return a.PredictedMin < b.PredictedMin
	})

	s.routes.SetForCar(ctx, req.Origin, req.Destination, env.Model, resp)
	go func() {
		if err := s.repo.SaveSearch(context.WithoutCancel(ctx), userID, req, resp); err != nil {
			s.log.Debug("save route search", "err", err)
		}
	}()
	return resp, nil
}

// RoadCompat checks one road for one car.
func (s *Service) RoadCompat(ctx context.Context, roadID, carName string) (CompatResult, error) {
	env, err := s.carSpecs.Envelope(ctx, carName)
	if err != nil {
		return CompatResult{}, err
	}
	attrs, err := s.roads.Attributes(ctx, []string{roadID}, nil)
	if err != nil {
		return CompatResult{}, err
	}
	res, err := s.compat.CheckRoads(ctx, env, []RoadAttr{attrs[roadID]})
	return res[roadID], err
}

// ReportRoadWidth lets surveyors/drivers correct a road width; caches are refreshed.
func (s *Service) ReportRoadWidth(ctx context.Context, a RoadAttr) error {
	if a.ID == "" || a.WidthM <= 1 {
		return apperr.BadRequest("road id and width are required")
	}
	if err := s.repo.UpsertRoad(ctx, a); err != nil {
		return err
	}
	return s.roads.Invalidate(ctx, a.ID)
}

// Record implements gps.TrafficRecorder.
func (s *Service) Record(ctx context.Context, roadID string, speedKmh float64) error {
	return s.traffic.Record(ctx, roadID, speedKmh)
}
