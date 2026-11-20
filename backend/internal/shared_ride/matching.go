package shared_ride

import (
	"context"

	"sarathi/internal/common/utils"
)

// Match is an open trip a new rider can join.
type Match struct {
	Trip      *Trip   `json:"trip"`
	PickupKm  float64 `json:"pickup_distance_km"`
	DetourPct float64 `json:"detour_pct"`
}

const (
	pickupRadiusKm = 1.5
	maxDetour      = 0.25 // existing riders accept at most +25 % distance
)

// Matcher finds the open pool with the smallest detour for a new rider.
type Matcher struct{ store *Store }

func NewMatcher(s *Store) *Matcher { return &Matcher{store: s} }

func (m *Matcher) Find(ctx context.Context, pickup, drop utils.Point, seats int) (*Match, error) {
	hits, err := m.store.OpenNear(ctx, pickup, pickupRadiusKm)
	if err != nil {
		return nil, err
	}
	var best *Match
	for _, h := range hits {
		t, err := m.store.Get(ctx, h.Member)
		if err != nil || t.State != "open" || t.SeatsLeft < seats {
			continue
		}
		d := Detour(t.Origin, t.Dest, pickup, drop)
		if d > maxDetour {
			continue
		}
		if best == nil || d < best.DetourPct {
			best = &Match{Trip: t, PickupKm: utils.Round(h.DistKm, 2), DetourPct: utils.Round(d, 3)}
		}
	}
	return best, nil
}

// Detour is the extra distance (fraction) for the pool when a rider is added:
// origin -> pickup -> (drop/dest in the better order) vs origin -> dest.
func Detour(origin, dest, pickup, drop utils.Point) float64 {
	direct := utils.HaversineKm(origin, dest)
	if direct < 0.5 {
		return 1
	}
	viaDropFirst := utils.HaversineKm(origin, pickup) + utils.HaversineKm(pickup, drop) + utils.HaversineKm(drop, dest)
	viaDestFirst := utils.HaversineKm(origin, pickup) + utils.HaversineKm(pickup, dest) + utils.HaversineKm(dest, drop)
	with := viaDropFirst
	if viaDestFirst < with {
		with = viaDestFirst
	}
	return with/direct - 1
}
