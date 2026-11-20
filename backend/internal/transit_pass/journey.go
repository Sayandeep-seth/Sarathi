// Package transit_pass lets daily passengers pay the whole fare of a multi-modal
// journey (train, metro, bus, cab) once and travel with one QR code scanned by
// every operator. Daily routes are saved so a journey starts with one tap.
package transit_pass

import (
	"time"

	"sarathi/internal/common/constants"
	apperr "sarathi/internal/common/errors"
)

// LegSpec is one ride of a saved route.
type LegSpec struct {
	Mode     string `json:"mode"` // train | metro | bus | cab
	From     string `json:"from"` // station / stop code
	To       string `json:"to"`
	Operator string `json:"operator"` // e.g. DMRC, BEST, WR
}

// SavedRoute is a passenger's daily route ("Home → Office").
type SavedRoute struct {
	Name string    `json:"name"`
	Legs []LegSpec `json:"legs"`
}

// Leg is a paid ride inside a journey.
type Leg struct {
	LegSpec
	Fare   float64    `json:"fare"`
	Used   bool       `json:"used"`
	UsedAt *time.Time `json:"used_at,omitempty"`
}

// Journey statuses.
const (
	JourneyPending   = "pending_payment"
	JourneyActive    = "active"
	JourneyCompleted = "completed"
	JourneyExpired   = "expired"
)

// Journey is one paid trip made of several legs.
type Journey struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	RouteName  string    `json:"route_name"`
	Status     string    `json:"status"`
	Legs       []Leg     `json:"legs"`
	Total      float64   `json:"total"`
	PaymentID  string    `json:"payment_intent_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	ValidUntil time.Time `json:"valid_until"`
}

var validModes = map[string]bool{constants.ModeTrain: true, constants.ModeMetro: true, constants.ModeBus: true, constants.ModeCab: true}

// Validate checks a saved route.
func (r SavedRoute) Validate() error {
	if r.Name == "" || len(r.Legs) == 0 || len(r.Legs) > 6 {
		return apperr.BadRequest("route needs a name and 1..6 legs")
	}
	for _, l := range r.Legs {
		if !validModes[l.Mode] || l.From == "" || l.To == "" {
			return apperr.BadRequest("each leg needs mode (train/metro/bus/cab), from and to")
		}
	}
	return nil
}
