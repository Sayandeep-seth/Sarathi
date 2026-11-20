package ai

import "context"

// RoadPredictor estimates carriageway width when a road has no measured width
// (common for Indian residential lanes in OSM). The estimate is cached in Redis
// by the map service together with the road attributes.
type RoadPredictor struct{ inf *Inference }

func NewRoadPredictor(inf *Inference) *RoadPredictor { return &RoadPredictor{inf: inf} }

var classIndex = map[string]float64{
	"motorway": 1, "trunk": 2, "primary": 3, "secondary": 4, "tertiary": 5,
	"residential": 6, "service": 7, "unclassified": 8, "track": 9,
}

// typical total carriageway width (m) by road class
var classWidth = map[string]float64{
	"motorway": 14, "trunk": 10.5, "primary": 9, "secondary": 7.5, "tertiary": 6,
	"residential": 4.5, "service": 3.5, "unclassified": 4, "track": 3,
}

func (p *RoadPredictor) PredictWidthM(ctx context.Context, class string, lanes int, oneWay bool, laneWidthM float64) float64 {
	oneWayF := 0.0
	if oneWay {
		oneWayF = 1
	}
	if p.inf.Enabled() {
		out, err := p.inf.Predict(ctx, ModelRoadWidth, []float64{classIndex[class], float64(lanes), oneWayF})
		if err == nil && len(out) > 0 && out[0] > 1.5 {
			return out[0]
		}
	}
	if lanes > 0 {
		return float64(lanes) * laneWidthM
	}
	if w, ok := classWidth[class]; ok {
		return w
	}
	return 4.0
}
