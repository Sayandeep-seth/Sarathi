package websocket

import (
	"context"
	"time"

	"sarathi/internal/cache"
)

// Event is the envelope every realtime message uses.
type Event struct {
	Topic string    `json:"topic"`
	Type  string    `json:"type"`
	Data  any       `json:"data"`
	At    time.Time `json:"at"`
}

// Trip event types.
const (
	EventDriverAssigned = "driver_assigned"
	EventDriverArriving = "driver_arriving"
	EventTripStarted    = "trip_started"
	EventTripCompleted  = "trip_completed"
	EventTripCancelled  = "trip_cancelled"
	EventOffer          = "ride_offer"
	EventPayment        = "payment"
	EventSeatHeld       = "seat_held"
	EventFoodOrder      = "food_order"
	EventZoneAlert      = "zone_alert"
)

// Publisher publishes events to Redis pub/sub; every API pod's Hub receives them
// and forwards to its locally connected sockets. Services never talk to sockets directly.
type Publisher struct{ cs *cache.Service }

func NewPublisher(cs *cache.Service) *Publisher { return &Publisher{cs: cs} }

func (p *Publisher) publish(ctx context.Context, channel, typ string, data any) error {
	return p.cs.Publish(ctx, channel, Event{Topic: TopicOf(channel), Type: typ, Data: data, At: time.Now()})
}

// Trip notifies everyone watching a trip (rider + driver).
func (p *Publisher) Trip(ctx context.Context, tripID, typ string, data any) error {
	return p.publish(ctx, cache.ChanTrip(tripID), typ, data)
}

// User notifies one user on all their devices.
func (p *Publisher) User(ctx context.Context, userID, typ string, data any) error {
	return p.publish(ctx, cache.ChanUser(userID), typ, data)
}

// Driver notifies one driver (ride offers, cancellations).
func (p *Publisher) Driver(ctx context.Context, driverID, typ string, data any) error {
	return p.publish(ctx, cache.ChanDriver(driverID), typ, data)
}

// Restaurant pushes aggregated food orders to a restaurant tablet.
func (p *Publisher) Restaurant(ctx context.Context, restaurantID, typ string, data any) error {
	return p.publish(ctx, cache.ChanRestaurant(restaurantID), typ, data)
}

// Dealer pushes test-drive alerts to a dealership dashboard.
func (p *Publisher) Dealer(ctx context.Context, dealerID, typ string, data any) error {
	return p.publish(ctx, cache.ChanDealer(dealerID), typ, data)
}
