package gps

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// DriverLocationUpdater is implemented by driver.Service (moves the driver in its GEO sets).
type DriverLocationUpdater interface {
	UpdateLocation(ctx context.Context, driverID string, p utils.Point) error
}

// TrafficRecorder is implemented by map_service.Service (live road speeds).
type TrafficRecorder interface {
	Record(ctx context.Context, roadID string, speedKmh float64) error
}

type Service struct {
	tracking *RedisTracking
	geofence *Geofence
	repo     *LocationRepository
	drivers  DriverLocationUpdater
	traffic  TrafficRecorder
	rdb      *redis.Client
	log      *slog.Logger
}

func NewService(t *RedisTracking, g *Geofence, repo *LocationRepository, d DriverLocationUpdater, tr TrafficRecorder, rdb *redis.Client, log *slog.Logger) *Service {
	return &Service{tracking: t, geofence: g, repo: repo, drivers: d, traffic: tr, rdb: rdb, log: log}
}

// Ingest handles one ping: live position, driver GEO, traffic speed, geofences.
func (s *Service) Ingest(ctx context.Context, p Ping) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if err := s.tracking.Store(ctx, p); err != nil {
		return err
	}
	if p.DriverID != "" && s.drivers != nil {
		if err := s.drivers.UpdateLocation(ctx, p.DriverID, p.Location); err != nil {
			s.log.Debug("driver location", "err", err)
		}
	}
	if p.RoadID != "" && s.traffic != nil {
		_ = s.traffic.Record(ctx, p.RoadID, p.SpeedKmh)
	}
	s.geofence.Evaluate(ctx, p)
	return nil
}

func (s *Service) Last(ctx context.Context, vehicleID string) (*Ping, error) {
	return s.tracking.Last(ctx, vehicleID)
}

func (s *Service) TripPath(ctx context.Context, tripID string) ([]Ping, error) {
	return s.tracking.TripPath(ctx, tripID, 5000)
}

// CleanupStale is called by the worker every minute.
func (s *Service) CleanupStale(ctx context.Context) (int, error) {
	return s.tracking.RemoveStale(ctx, 3*time.Minute)
}

const persistGroup = "gps-persist"

// RunPersister drains sr:stream:gps:persist with a consumer group and batch-inserts
// into Postgres. Several workers can run it; each message is processed once.
func (s *Service) RunPersister(ctx context.Context, consumer string) error {
	err := s.rdb.XGroupCreateMkStream(ctx, cache.StreamGPSPersist, persistGroup, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	for ctx.Err() == nil {
		streams, err := s.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: persistGroup, Consumer: consumer, Streams: []string{cache.StreamGPSPersist, ">"},
			Count: 500, Block: 2 * time.Second,
		}).Result()
		if errors.Is(err, redis.Nil) || (err != nil && ctx.Err() != nil) {
			continue
		}
		if err != nil {
			s.log.Error("gps persister read", "err", err)
			time.Sleep(time.Second)
			continue
		}
		for _, st := range streams {
			batch := make([]Ping, 0, len(st.Messages))
			ids := make([]string, 0, len(st.Messages))
			for _, m := range st.Messages {
				var p Ping
				if raw, ok := m.Values["p"].(string); ok && json.Unmarshal([]byte(raw), &p) == nil {
					batch = append(batch, p)
				}
				ids = append(ids, m.ID)
			}
			if err := s.repo.InsertBatch(ctx, batch); err != nil {
				s.log.Error("gps persist batch", "err", err, "size", len(batch))
				continue // not acked -> redelivered later
			}
			s.rdb.XAck(ctx, cache.StreamGPSPersist, persistGroup, ids...)
			s.rdb.XDel(ctx, cache.StreamGPSPersist, ids...)
		}
	}
	return ctx.Err()
}

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

type DriverLocation struct {
    DriverID  string    `json:"driver_id"`
    RideID    string    `json:"ride_id"`
    Latitude  float64   `json:"latitude"`
    Longitude float64   `json:"longitude"`
    Speed     float64   `json:"speed"`
    Heading   float64   `json:"heading"`
    Accuracy  float64   `json:"accuracy"`
    UpdatedAt time.Time `json:"updated_at"`
}

type Service struct {
    Redis *redis.Client
}

func NewService(redisClient *redis.Client) *Service {
    return &Service{
        Redis: redisClient,
    }
}

func (s *Service) UpdateLocation(
    ctx context.Context,
    location DriverLocation,
) error {

    location.UpdatedAt = time.Now()

    payload, err := json.Marshal(location)
    if err != nil {
        return err
    }

    key := fmt.Sprintf("sarathi:gps:driver:%s", location.DriverID)

    // Save the driver's latest location.
    if err := s.Redis.Set(
        ctx,
        key,
        payload,
        30*time.Second,
    ).Err(); err != nil {
        return err
    }

    // Publish a real-time location event.
    event := map[string]interface{}{
        "type": "location",
        "event": "driver.location.update",
        "user_id": location.DriverID,
        "ride_id": location.RideID,
        "timestamp": location.UpdatedAt.Unix(),
        "data": location,
    }

    eventJSON, err := json.Marshal(event)
    if err != nil {
        return err
    }

    return s.Redis.Publish(
        ctx,
        "sarathi:ws:location",
        eventJSON,
    ).Err()
}