package driver

import (
	"context"
	"time"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// GoOnline loads the profile into Redis, stores the position and marks available.
func (s *Service) GoOnline(ctx context.Context, id string, p utils.Point) error {
	d, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.SaveProfile(ctx, *d); err != nil {
		return err
	}
	if err := s.store.UpdateLocation(ctx, id, p); err != nil {
		return err
	}
	return s.store.SetStatus(ctx, id, StatusAvailable)
}

func (s *Service) GoOffline(ctx context.Context, id string) error {
	return s.store.SetStatus(ctx, id, StatusOffline)
}

func (s *Service) TakeBreak(ctx context.Context, id string) error {
	return s.store.SetStatus(ctx, id, StatusBreak)
}

// MarkOnTrip is called when the driver accepts a ride (removes from GEO sets).
func (s *Service) MarkOnTrip(ctx context.Context, id string) error {
	return s.store.SetStatus(ctx, id, StatusOnTrip)
}

// MarkAvailable is called when a trip completes or is cancelled.
func (s *Service) MarkAvailable(ctx context.Context, id string) error {
	return s.store.SetStatus(ctx, id, StatusAvailable)
}

// IsAvailable checks status and heartbeat.
func (s *Service) IsAvailable(ctx context.Context, id string) bool {
	m, err := s.store.Get(ctx, id)
	if err != nil || m["status"] != StatusAvailable {
		return false
	}
	n, _ := s.rdb.Exists(ctx, cache.KeyDriverHeartbeat(id)).Result()
	return n == 1
}

// CountNearby is the supply number for surge pricing.
func (s *Service) CountNearby(ctx context.Context, class string, p utils.Point, radiusKm float64) (int, error) {
	return s.geo.Count(ctx, cache.KeyDriversGeo(class), p, radiusKm)
}

// SweepStale takes drivers offline whose app stopped sending locations (worker, every 30 s).
// A driver on a trip is kept but logged; an available driver is made offline so it
// is never offered a ride it cannot see.
func (s *Service) SweepStale(ctx context.Context) (int, error) {
	ids, err := s.store.Stale(ctx, cache.TTLDriverStale)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		m, _ := s.store.Get(ctx, id)
		if m["status"] == StatusOnTrip {
			s.log.Warn("driver on trip lost GPS", "driver", id, "since", time.Unix(cache.HGetInt(m, "updated"), 0))
			continue
		}
		if err := s.store.SetStatus(ctx, id, StatusOffline); err == nil {
			n++
		}
		_ = s.store.ForgetLastSeen(ctx, id)
	}
	return n, nil
}
