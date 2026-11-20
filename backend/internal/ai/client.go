// Package ai talks to the model-serving endpoint and caches predictions in Redis.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client calls a TF-Serving/KServe style endpoint:
// POST {endpoint}/v1/models/{model}/versions/{version}:predict {"instances":[[...]]}
type Client struct {
	endpoint string
	http     *http.Client
}

func NewClient(endpoint string) *Client {
	return &Client{endpoint: endpoint, http: &http.Client{Timeout: 800 * time.Millisecond}}
}

// Enabled is false when no endpoint is configured; callers then use heuristics.
func (c *Client) Enabled() bool { return c != nil && c.endpoint != "" }

func (c *Client) Predict(ctx context.Context, model, version string, features []float64) ([]float64, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("ai: endpoint not configured")
	}
	body, _ := json.Marshal(map[string]any{"instances": [][]float64{features}})
	url := fmt.Sprintf("%s/v1/models/%s/versions/%s:predict", c.endpoint, model, version)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai: %s returned %d", model, resp.StatusCode)
	}
	var out struct {
		Predictions [][]float64 `json:"predictions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Predictions) == 0 {
		return nil, fmt.Errorf("ai: empty prediction")
	}
	return out.Predictions[0], nil
}
