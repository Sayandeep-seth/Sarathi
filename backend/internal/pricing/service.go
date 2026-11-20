package pricing

import (
	"context"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// Service exposes rate cards (Redis-cached) and ride quotes with surge.
//
//	sr:pricing:rate:<class>  HASH base,per_km,per_min,min_fare,wait_per_min (1 h)
type Service struct {
	rdb   *redis.Client
	Surge *Surge
}

func NewService(rdb *redis.Client, s *Surge) *Service { return &Service{rdb: rdb, Surge: s} }

// RateCard returns the active card for a class (Redis -> default).
func (s *Service) RateCard(ctx context.Context, class string) RateCard {
	m, err := s.rdb.HGetAll(ctx, cache.KeyRateCard(class)).Result()
	if err == nil && len(m) > 0 {
		return RateCard{
			Class: class, Base: cache.HGetFloat(m, "base"), PerKm: cache.HGetFloat(m, "per_km"),
			PerMin: cache.HGetFloat(m, "per_min"), MinFare: cache.HGetFloat(m, "min_fare"),
			WaitPerMin: cache.HGetFloat(m, "wait_per_min"),
		}
	}
	if rc, ok := DefaultRates[class]; ok {
		return rc
	}
	return DefaultRates["cab"]
}

// SetRateCard updates a card (admin); takes effect for all pods immediately.
func (s *Service) SetRateCard(ctx context.Context, rc RateCard) error {
	if rc.Class == "" || rc.PerKm <= 0 {
		return apperr.BadRequest("class and per_km required")
	}
	// admin-managed config: no expiry, every pod reads the same card
	return s.rdb.HSet(ctx, cache.KeyRateCard(rc.Class),
		"base", rc.Base, "per_km", rc.PerKm, "per_min", rc.PerMin, "min_fare", rc.MinFare, "wait_per_min", rc.WaitPerMin).Err()
}

// QuoteRide prices a ride including live surge at the pickup point.
func (s *Service) QuoteRide(ctx context.Context, class string, pickup utils.Point, km, minutes float64) Breakdown {
	return RideFare(s.RateCard(ctx, class), km, minutes, s.Surge.Multiplier(ctx, class, pickup))
}

// EstimateTrip gives a straight-line based estimate when no route is supplied
// (road distance ≈ 1.35 × crow-fly in Indian cities, 22 km/h average).
func EstimateTrip(a, b utils.Point) (km, minutes float64) {
	km = utils.Round(utils.HaversineKm(a, b)*1.35, 2)
	return km, utils.Round(km/22*60, 0)
}
