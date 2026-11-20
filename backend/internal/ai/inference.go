package ai

import (
	"context"
	"fmt"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// Inference wraps the client with a Redis prediction cache keyed by
// model + version + rounded features, so identical questions (same road class,
// same hour, similar distance) are answered without calling the model again.
type Inference struct {
	client *Client
	loader *ModelLoader
	cs     *cache.Service
}

func NewInference(c *Client, l *ModelLoader, cs *cache.Service) *Inference {
	return &Inference{client: c, loader: l, cs: cs}
}

func (i *Inference) Enabled() bool { return i != nil && i.client.Enabled() }

func (i *Inference) Predict(ctx context.Context, model string, features []float64) ([]float64, error) {
	if !i.Enabled() {
		return nil, fmt.Errorf("ai disabled")
	}
	version := i.loader.Version(ctx, model)
	rounded := make([]any, len(features))
	for j, f := range features {
		rounded[j] = utils.Round(f, 2)
	}
	key := cache.KeyAIPrediction(model, version, utils.HashKey(rounded...))
	return cache.Remember(ctx, i.cs, key, cache.TTLAIPrediction, func(ctx context.Context) ([]float64, error) {
		return i.client.Predict(ctx, model, version, features)
	})
}
