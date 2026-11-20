package ai

import (
	"context"
	"time"

	"sarathi/internal/common/utils"
)

// RouteFeatures describe a candidate route for ETA prediction.
type RouteFeatures struct {
	DistanceKm      float64
	BaseDurationMin float64 // free-flow duration from the routing engine
	TrafficLevel    float64 // 0 (free) … 1 (jammed), from live GPS speeds in Redis
	NarrowRoadKm    float64 // km on roads with little spare width (slower driving)
	At              time.Time
}

// RoutePredictor predicts a realistic ETA (minutes).
type RoutePredictor struct{ inf *Inference }

func NewRoutePredictor(inf *Inference) *RoutePredictor { return &RoutePredictor{inf: inf} }

func (p *RoutePredictor) PredictETA(ctx context.Context, f RouteFeatures) float64 {
	t := f.At.In(utils.IST)
	if p.inf.Enabled() {
		out, err := p.inf.Predict(ctx, ModelRouteETA, []float64{
			f.DistanceKm, f.BaseDurationMin, f.TrafficLevel, f.NarrowRoadKm,
			float64(t.Hour()), float64(t.Weekday()),
		})
		if err == nil && len(out) > 0 && out[0] > 0 {
			return utils.Round(out[0], 1)
		}
	}
	return utils.Round(heuristicETA(f, t), 1)
}

// heuristicETA is the fallback when the model is unavailable.
func heuristicETA(f RouteFeatures, t time.Time) float64 {
	eta := f.BaseDurationMin * (1 + 1.5*f.TrafficLevel)
	h := t.Hour()
	if (h >= 8 && h <= 11) || (h >= 17 && h <= 21) { // Indian city peak hours
		eta *= 1.25
	}
	eta += f.NarrowRoadKm * 1.5 // ~1.5 extra min per km on tight roads
	return eta
}
