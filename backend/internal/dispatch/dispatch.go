// Package dispatch matches a ride request with nearby drivers in real time.
//
// Flow (all three booking modes: solo, shared, pickup-and-return):
//
//	Passenger app --POST /v1/trips--> REST API --dispatch.Offer()--> Redis
//	   GEOSEARCH nearby idle drivers -> PUBLISH trip.offer to each driver's user:{id}
//	Driver app --WS offer.accept--> gateway --dispatch.Accept()--> Lua script (atomic)
//	   first driver wins -> PUBLISH trip.status to trip:{id} + passenger's user:{id}
//	                     -> PUBLISH trip.offer.cancelled to the other drivers
package dispatch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/backend/internal/geo"
	"sarathi/backend/internal/realtime"
)

type Mode string

const (
	ModeSolo         Mode = "solo"          // normal cab, no parking fee
	ModeShared       Mode = "shared"        // pooled cab, cheaper, driver earns more per km
	ModePickupReturn Mode = "pickup_return" // go to airport/station, pick someone up, come back; parking fee applies
)

type RideRequest struct {
	TripID       string  `json:"tripId"`
	PassengerID  string  `json:"passengerId"`
	Mode         Mode    `json:"mode"`
	VehicleClass string  `json:"vehicleClass"` // pickup_return: only sedan5/hatch5 (5 seats) or suv7
	PickupLat    float64 `json:"pickupLat"`
	PickupLng    float64 `json:"pickupLng"`
	DropLat      float64 `json:"dropLat"`
	DropLng      float64 `json:"dropLng"`
	FarePaise    int64   `json:"farePaise"` // computed by your pricing service
	ParkingPaise int64   `json:"parkingPaise,omitempty"`
}

// TripRepository is your Postgres layer. Postgres is the source of truth;
// Redis is the fast, atomic "who got it first" lock.
type TripRepository interface {
	MarkDriverAssigned(ctx context.Context, tripID, driverID string) error
}

type Service struct {
	rdb      redis.UniversalClient
	drivers  *geo.DriverIndex
	pub      *realtime.Publisher
	repo     TripRepository
	log      *slog.Logger
	OfferTTL time.Duration // how long a driver has to accept
	RadiusKm float64
	MaxOffer int // drivers offered at once
}

func NewService(rdb redis.UniversalClient, drivers *geo.DriverIndex, pub *realtime.Publisher, repo TripRepository, log *slog.Logger) *Service {
	return &Service{
		rdb: rdb, drivers: drivers, pub: pub, repo: repo,
		log:      log.With("component", "dispatch"),
		OfferTTL: 20 * time.Second,
		RadiusKm: 3,
		MaxOffer: 5,
	}
}

func tripKey(id string) string        { return "trip:" + id }
func tripOfferedKey(id string) string { return "trip:" + id + ":offered" }

var ErrNoDrivers = errors.New("no drivers available nearby")

// Offer is called by the REST handler that creates the trip (after the trip
// row is inserted in Postgres). Returns how many drivers were offered the ride.
func (s *Service) Offer(ctx context.Context, req RideRequest) (int, error) {
	if req.Mode == ModePickupReturn && req.VehicleClass != "hatch5" && req.VehicleClass != "sedan5" && req.VehicleClass != "suv7" {
		return 0, fmt.Errorf("pickup_return only supports 5 or 7 seat classes")
	}

	nearby, err := s.drivers.Nearby(ctx, req.VehicleClass, req.PickupLat, req.PickupLng, s.RadiusKm, s.MaxOffer*3)
	if err != nil {
		return 0, err
	}

	// Keep only drivers whose status is "idle" (one pipelined MGET-style round trip).
	statusCmds := make([]*redis.StringCmd, len(nearby))
	pipe := s.rdb.Pipeline()
	for i, d := range nearby {
		statusCmds[i] = pipe.Get(ctx, geo.DriverStatusKey(d.Name))
	}
	_, _ = pipe.Exec(ctx) // redis.Nil for offline drivers is expected

	var chosen []redis.GeoLocation
	for i, d := range nearby {
		if statusCmds[i].Val() == "idle" {
			chosen = append(chosen, d)
			if len(chosen) == s.MaxOffer {
				break
			}
		}
	}
	if len(chosen) == 0 {
		return 0, ErrNoDrivers
	}

	expiresAt := time.Now().Add(s.OfferTTL)
	ids := make([]any, len(chosen))
	for i, d := range chosen {
		ids[i] = d.Name
	}

	// Store trip state + who was offered + membership, then notify, in ONE round trip.
	pipe = s.rdb.TxPipeline()
	pipe.HSet(ctx, tripKey(req.TripID),
		"status", "searching",
		"passenger_id", req.PassengerID,
		"mode", string(req.Mode),
		"class", req.VehicleClass,
	)
	pipe.Expire(ctx, tripKey(req.TripID), 24*time.Hour)
	pipe.SAdd(ctx, tripOfferedKey(req.TripID), ids...)
	pipe.Expire(ctx, tripOfferedKey(req.TripID), s.OfferTTL) // expired offer == not offered
	pipe.SAdd(ctx, realtime.TripMembersKey(req.TripID), req.PassengerID)
	pipe.Expire(ctx, realtime.TripMembersKey(req.TripID), 24*time.Hour)

	for _, d := range chosen {
		_ = s.pub.PublishPipe(ctx, pipe, realtime.UserTopic(d.Name), realtime.TypeTripOffer, map[string]any{
			"tripId":       req.TripID,
			"mode":         req.Mode,
			"pickup":       map[string]float64{"lat": req.PickupLat, "lng": req.PickupLng},
			"drop":         map[string]float64{"lat": req.DropLat, "lng": req.DropLng},
			"distanceKm":   d.Dist, // driver -> pickup
			"farePaise":    req.FarePaise,
			"parkingPaise": req.ParkingPaise,
			"expiresAt":    expiresAt.UnixMilli(),
		})
	}
	_ = s.pub.PublishPipe(ctx, pipe, realtime.TripTopic(req.TripID), realtime.TypeTripStatus, map[string]any{
		"tripId": req.TripID, "status": "searching", "driversOffered": len(chosen),
	})
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return len(chosen), nil
}

// acceptScript makes "first driver wins" atomic. Redis runs a Lua script
// without interleaving any other command, so two drivers tapping Accept in
// the same millisecond can never both get the trip.
//
// KEYS[1]=trip:{id}  KEYS[2]=trip:{id}:offered  KEYS[3]=trip:{id}:members  KEYS[4]=driver:{d}:status
// ARGV[1]=driverID
//
// NOTE: uses keys from different hash slots, fine for single Redis / Sentinel.
// For Redis Cluster, use hash tags e.g. trip:{abc} and keep driver status in the trip slot.
var acceptScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'status') ~= 'searching' then return 0 end
if redis.call('SISMEMBER', KEYS[2], ARGV[1]) == 0 then return -1 end
if redis.call('GET', KEYS[4]) ~= 'idle' then return -2 end
redis.call('HSET', KEYS[1], 'status', 'driver_assigned', 'driver_id', ARGV[1])
redis.call('SET', KEYS[4], 'busy', 'EX', 7200)
redis.call('SADD', KEYS[3], ARGV[1])
return 1`)

var (
	ErrTripTaken    = realtime.NewClientError("trip_taken", "another driver already accepted this trip")
	ErrOfferExpired = realtime.NewClientError("offer_expired", "this offer has expired")
	ErrNotIdle      = realtime.NewClientError("not_idle", "go online and finish your current trip first")
)

func (s *Service) Accept(ctx context.Context, tripID, driverID string) error {
	keys := []string{tripKey(tripID), tripOfferedKey(tripID), realtime.TripMembersKey(tripID), geo.DriverStatusKey(driverID)}
	res, err := acceptScript.Run(ctx, s.rdb, keys, driverID).Int()
	if err != nil {
		return err
	}
	switch res {
	case 0:
		return ErrTripTaken
	case -1:
		return ErrOfferExpired
	case -2:
		return ErrNotIdle
	}

	// Persist. If Postgres fails, undo the Redis lock so the trip can be re-offered.
	if s.repo != nil {
		if err := s.repo.MarkDriverAssigned(ctx, tripID, driverID); err != nil {
			pipe := s.rdb.TxPipeline()
			pipe.HSet(ctx, tripKey(tripID), "status", "searching", "driver_id", "")
			pipe.Set(ctx, geo.DriverStatusKey(driverID), "idle", geo.DriverStatusTTL)
			pipe.SRem(ctx, realtime.TripMembersKey(tripID), driverID)
			_, _ = pipe.Exec(ctx)
			return err
		}
	}

	passengerID, _ := s.rdb.HGet(ctx, tripKey(tripID), "passenger_id").Result()
	others, _ := s.rdb.SMembers(ctx, tripOfferedKey(tripID)).Result()

	status := map[string]any{"tripId": tripID, "status": "driver_assigned", "driverId": driverID}
	pipe := s.rdb.Pipeline()
	_ = s.pub.PublishPipe(ctx, pipe, realtime.TripTopic(tripID), realtime.TypeTripStatus, status)
	if passengerID != "" {
		_ = s.pub.PublishPipe(ctx, pipe, realtime.UserTopic(passengerID), realtime.TypeTripStatus, status)
	}
	for _, other := range others {
		if other != driverID {
			_ = s.pub.PublishPipe(ctx, pipe, realtime.UserTopic(other), realtime.TypeTripOfferCancelled, map[string]any{"tripId": tripID})
		}
	}
	pipe.Del(ctx, tripOfferedKey(tripID))
	_, err = pipe.Exec(ctx)
	return err
}

// Decline removes the driver from the offer list.
func (s *Service) Decline(ctx context.Context, tripID, driverID string) error {
	return s.rdb.SRem(ctx, tripOfferedKey(tripID), driverID).Err()
}

// UpdateStatus is called by REST endpoints / the driver app for the rest of
// the trip lifecycle: arriving, started, completed, cancelled.
func (s *Service) UpdateStatus(ctx context.Context, tripID, status string) error {
	driverID, _ := s.rdb.HGet(ctx, tripKey(tripID), "driver_id").Result()
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, tripKey(tripID), "status", status)
	if (status == "completed" || status == "cancelled") && driverID != "" {
		pipe.Set(ctx, geo.DriverStatusKey(driverID), "idle", geo.DriverStatusTTL)
		pipe.Expire(ctx, realtime.TripMembersKey(tripID), 2*time.Hour)
	}
	_ = s.pub.PublishPipe(ctx, pipe, realtime.TripTopic(tripID), realtime.TypeTripStatus, map[string]any{
		"tripId": tripID, "status": status,
	})
	_, err := pipe.Exec(ctx)
	return err
}

// AssignedDriver tells the location handler whether to forward a driver's
// GPS to trip:{id}.
func (s *Service) AssignedDriver(ctx context.Context, tripID string) (string, error) {
	id, err := s.rdb.HGet(ctx, tripKey(tripID), "driver_id").Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return id, err
}

// AddSharedPassenger adds a co-passenger to an already running shared trip so
// they can subscribe to trip:{id} and see the same car moving.
func (s *Service) AddSharedPassenger(ctx context.Context, tripID, passengerID string) error {
	pipe := s.rdb.TxPipeline()
	pipe.SAdd(ctx, realtime.TripMembersKey(tripID), passengerID)
	_ = s.pub.PublishPipe(ctx, pipe, realtime.TripTopic(tripID), realtime.TypeTripStatus, map[string]any{
		"tripId": tripID, "status": "co_passenger_added",
	})
	_, err := pipe.Exec(ctx)
	return err
}
