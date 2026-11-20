package driver

import (
	"context"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

type Service struct {
	repo  *Repository
	store *Store
	geo   *cache.Geo
	rdb   *redis.Client
	log   *slog.Logger
}

func NewService(repo *Repository, store *Store, geo *cache.Geo, rdb *redis.Client, log *slog.Logger) *Service {
	return &Service{repo: repo, store: store, geo: geo, rdb: rdb, log: log}
}

// UpdateLocation implements gps.DriverLocationUpdater.
func (s *Service) UpdateLocation(ctx context.Context, driverID string, p utils.Point) error {
	return s.store.UpdateLocation(ctx, driverID, p)
}

// State returns the live Redis state of a driver.
func (s *Service) State(ctx context.Context, id string) (map[string]string, error) {
	return s.store.Get(ctx, id)
}

// Position returns the driver's last known position.
func (s *Service) Position(ctx context.Context, id string) (utils.Point, error) {
	m, err := s.store.Get(ctx, id)
	if err != nil {
		return utils.Point{}, err
	}
	return utils.Point{Lat: cache.HGetFloat(m, "lat"), Lng: cache.HGetFloat(m, "lng")}, nil
}

// AddEarning persists a payout for a completed trip.
func (s *Service) AddEarning(ctx context.Context, driverID, bookingID string, amount float64) error {
	return s.repo.AddEarning(ctx, driverID, bookingID, amount)
}
