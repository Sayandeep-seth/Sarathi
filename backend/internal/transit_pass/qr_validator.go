package transit_pass

import (
	"context"
	"time"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
)

// ScanRequest comes from a gate / conductor / cab driver device.
type ScanRequest struct {
	Token      string `json:"token"`
	Mode       string `json:"mode"`
	StationID  string `json:"station"` // stop where the passenger boards
	OperatorID string `json:"operator"`
	DeviceID   string `json:"device_id"`
}

// ScanResult tells the gate whether to open.
type ScanResult struct {
	Allowed       bool    `json:"allowed"`
	Reason        string  `json:"reason,omitempty"`
	JourneyID     string  `json:"journey_id,omitempty"`
	Leg           int     `json:"leg"`
	Fare          float64 `json:"fare_settled,omitempty"` // amount the operator is paid for this leg
	LegsRemaining int     `json:"legs_remaining"`
}

// QRValidator checks a scan: signature, Redis nonce, matching unused leg, one-time use.
type QRValidator struct {
	gen     *QRGenerator
	store   *Store
	limiter *cache.RateLimiter
}

func NewQRValidator(g *QRGenerator, s *Store, l *cache.RateLimiter) *QRValidator {
	return &QRValidator{gen: g, store: s, limiter: l}
}

func (v *QRValidator) Validate(ctx context.Context, req ScanRequest) (*ScanResult, error) {
	if req.DeviceID == "" {
		return nil, apperr.BadRequest("device_id required")
	}
	if res, _ := v.limiter.Allow(ctx, "qrscan", req.DeviceID, 120, time.Minute); !res.Allowed {
		return nil, apperr.TooMany("scanner rate limit")
	}
	tok, ok := v.gen.Parse(req.Token)
	if !ok {
		return &ScanResult{Reason: "invalid or expired QR"}, nil
	}
	journeyID, err := v.store.JourneyByNonce(ctx, tok.Nonce)
	if err != nil || journeyID != tok.JourneyID {
		return &ScanResult{Reason: "QR replaced by a newer one"}, nil
	}
	j, _, err := v.store.Journey(ctx, journeyID)
	if err != nil {
		return &ScanResult{Reason: "journey not found"}, nil
	}

	// the leg to use: first unused leg of this mode, preferring the boarding station
	leg := -1
	for i, l := range j.Legs {
		if l.Used || l.Mode != req.Mode {
			continue
		}
		if l.From == req.StationID {
			leg = i
			break
		}
		if leg == -1 {
			leg = i
		}
	}
	if leg == -1 {
		return &ScanResult{JourneyID: j.ID, Reason: "no unused " + req.Mode + " leg in this journey"}, nil
	}
	switch code, err := v.store.UseLeg(ctx, j.ID, leg, tok.Nonce); {
	case err != nil:
		return nil, err
	case code == 0:
		return &ScanResult{JourneyID: j.ID, Leg: leg, Reason: "leg already used"}, nil
	case code == -1:
		return &ScanResult{JourneyID: j.ID, Reason: "journey not active"}, nil
	case code == -2:
		return &ScanResult{JourneyID: j.ID, Reason: "QR replaced by a newer one"}, nil
	}
	remaining := 0
	for i, l := range j.Legs {
		if !l.Used && i != leg {
			remaining++
		}
	}
	return &ScanResult{Allowed: true, JourneyID: j.ID, Leg: leg, Fare: j.Legs[leg].Fare, LegsRemaining: remaining}, nil
}
