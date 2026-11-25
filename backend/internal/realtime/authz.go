package realtime

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Authorizer decides who may subscribe to which topic.
//
// Membership sets are written by the services that create the thing:
//   - booking/dispatch writes  trip:{id}:members  (passenger(s) + assigned driver)
//     For a SHARED cab every co-passenger is added to the same set.
//   - bus booking writes       bus:{id}:members    (ticket holders + the bus driver)
//
// Give these sets a TTL a few hours after the trip/run ends.
type Authorizer struct {
	rdb redis.UniversalClient
}

func NewAuthorizer(rdb redis.UniversalClient) *Authorizer { return &Authorizer{rdb: rdb} }

func TripMembersKey(tripID string) string { return "trip:" + tripID + ":members" }
func BusMembersKey(busID string) string   { return "bus:" + busID + ":members" }

func (a *Authorizer) CanSubscribe(ctx context.Context, id Identity, topic string) (bool, error) {
	kind, key, ok := ParseTopic(topic)
	if !ok {
		return false, NewClientError("bad_topic", "topic must look like trip:<id>")
	}
	if id.Role == RoleAdmin {
		return true, nil // support / ops dashboards
	}
	switch kind {
	case KindUser:
		return key == id.UserID, nil
	case KindTrip:
		return a.rdb.SIsMember(ctx, TripMembersKey(key), id.UserID).Result()
	case KindBus:
		return a.rdb.SIsMember(ctx, BusMembersKey(key), id.UserID).Result()
	case KindStation:
		return true, nil // crowd levels are public
	}
	return false, nil
}
