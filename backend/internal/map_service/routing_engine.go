package map_service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"sarathi/internal/common/utils"
)

// RawRoute is an alternative returned by the routing engine (before car checks).
type RawRoute struct {
	DistanceKm  float64            `json:"distance_km"`
	DurationMin float64            `json:"duration_min"`
	Polyline    []utils.Point      `json:"polyline"`
	RoadIDs     []string           `json:"road_ids"`
	RoadNames   map[string]string  `json:"road_names"`
	RoadKm      map[string]float64 `json:"road_km"`
}

// RoutingEngine returns all reasonable alternatives between two points.
type RoutingEngine interface {
	Alternatives(ctx context.Context, from, to utils.Point, max int) ([]RawRoute, error)
}

// OSRMEngine talks to an OSRM server. Road ids are derived from the step name/ref
// with the same hash the road-ingestion job uses, so they join with the roads table.
type OSRMEngine struct {
	baseURL string
	client  *http.Client
}

func NewOSRMEngine(baseURL string) *OSRMEngine {
	return &OSRMEngine{baseURL: baseURL, client: &http.Client{Timeout: 5 * time.Second}}
}

// RoadID is the shared road identifier function.
func RoadID(name, ref string) string { return "rd_" + utils.HashKey(name, ref)[:14] }

type osrmResp struct {
	Code   string `json:"code"`
	Routes []struct {
		Distance float64 `json:"distance"`
		Duration float64 `json:"duration"`
		Geometry struct {
			Coordinates [][2]float64 `json:"coordinates"`
		} `json:"geometry"`
		Legs []struct {
			Steps []struct {
				Name     string  `json:"name"`
				Ref      string  `json:"ref"`
				Distance float64 `json:"distance"`
			} `json:"steps"`
		} `json:"legs"`
	} `json:"routes"`
}

func (e *OSRMEngine) Alternatives(ctx context.Context, from, to utils.Point, max int) ([]RawRoute, error) {
	url := fmt.Sprintf("%s/route/v1/driving/%f,%f;%f,%f?alternatives=%d&steps=true&overview=full&geometries=geojson",
		e.baseURL, from.Lng, from.Lat, to.Lng, to.Lat, max)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("routing engine: %w", err)
	}
	defer resp.Body.Close()
	var body osrmResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if body.Code != "Ok" {
		return nil, fmt.Errorf("routing engine: %s", body.Code)
	}
	out := make([]RawRoute, 0, len(body.Routes))
	for _, r := range body.Routes {
		rr := RawRoute{
			DistanceKm: r.Distance / 1000, DurationMin: r.Duration / 60,
			RoadNames: map[string]string{}, RoadKm: map[string]float64{},
		}
		for _, c := range r.Geometry.Coordinates {
			rr.Polyline = append(rr.Polyline, utils.Point{Lng: c[0], Lat: c[1]})
		}
		for _, leg := range r.Legs {
			for _, st := range leg.Steps {
				if st.Name == "" && st.Ref == "" {
					continue
				}
				id := RoadID(st.Name, st.Ref)
				if _, seen := rr.RoadNames[id]; !seen {
					rr.RoadIDs = append(rr.RoadIDs, id)
					rr.RoadNames[id] = st.Name
				}
				rr.RoadKm[id] += st.Distance / 1000
			}
		}
		out = append(out, rr)
	}
	return out, nil
}
