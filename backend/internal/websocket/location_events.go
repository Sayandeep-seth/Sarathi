package websocket

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
)

// Topics a client may subscribe to:
//
//	user:<self>          personal notifications
//	driver:<self>        ride offers (drivers)
//	trip:<id>            trip status (rider or driver of that trip)
//	trip:<id>:loc        live driver location during the trip
//	restaurant:<self>    luxury-bus food orders
//	dealer:<self>        test-drive alerts
//
// Redis channel = cache.ChannelPrefix + topic.

// TopicOf converts a Redis channel to a websocket topic.
func TopicOf(channel string) string { return strings.TrimPrefix(channel, cache.ChannelPrefix) }

// ChannelOf converts a topic to a Redis channel.
func ChannelOf(topic string) string { return cache.ChannelPrefix + topic }

// Authorize checks that userID may listen to topic. Trip membership is read from
// the booking hash in Redis (fields user_id / driver_id).
func Authorize(ctx context.Context, rdb *redis.Client, userID, topic string) bool {
	parts := strings.Split(topic, ":")
	if len(parts) < 2 || userID == "" {
		return false
	}
	switch parts[0] {
	case "user", "driver", "restaurant", "dealer":
		return parts[1] == userID
	case "trip":
		vals, err := rdb.HMGet(ctx, cache.KeyBooking(parts[1]), "user_id", "driver_id").Result()
		if err != nil {
			return false
		}
		for _, v := range vals {
			if s, ok := v.(string); ok && s == userID {
				return true
			}
		}
		// shared ride co-passengers watch the shared trip
		ok, _ := rdb.SIsMember(ctx, cache.KeySharedRiders(parts[1]), userID).Result()
		return ok
	}
	return false
}

// LocationUpdate is what riders receive on trip:<id>:loc (published by gps).
type LocationUpdate struct {
	VehicleID string  `json:"vehicle_id"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Heading   float64 `json:"heading"`
	SpeedKmh  float64 `json:"speed_kmh"`
}
