// Package realtime is Sarathi's WebSocket transport layer.
//
// It knows nothing about trips, buses or fuel stations. It only knows how to:
//   - authenticate a socket (one-time ticket stored in Redis),
//   - keep the socket healthy (ping/pong, write deadlines, back-pressure),
//   - let a socket subscribe to "topics" (user:42, trip:abc, station:IOC123 ...),
//   - fan messages out to every socket on every gateway instance via Redis Pub/Sub.
//
// Feature code (dispatch, geo, bookings) lives in other packages and talks to
// sockets only through Publisher (server -> client) and Router.On (client -> server).
package realtime

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Envelope is the ONE JSON shape used for every frame, in both directions.
//
//	{"type":"subscribe","ref":"c1","topic":"trip:7f3a"}
//	{"type":"driver.location","topic":"trip:7f3a","data":{...},"ts":1764084000000}
type Envelope struct {
	Type  string          `json:"type"`
	Ref   string          `json:"ref,omitempty"`   // client-chosen id; echoed back in ack/error
	Topic string          `json:"topic,omitempty"` // which topic this frame belongs to
	Data  json.RawMessage `json:"data,omitempty"`  // type-specific payload
	TS    int64           `json:"ts,omitempty"`    // unix millis, set by the server
}

// Client -> server message types.
const (
	TypeSubscribe          = "subscribe"
	TypeUnsubscribe        = "unsubscribe"
	TypePing               = "ping"
	TypeLocationUpdate     = "location.update"
	TypeDriverAvailability = "driver.availability"
	TypeOfferAccept        = "offer.accept"
	TypeOfferDecline       = "offer.decline"
)

// Server -> client message types.
const (
	TypeWelcome            = "welcome"
	TypeAck                = "ack"
	TypeError              = "error"
	TypePong               = "pong"
	TypeTripOffer          = "trip.offer"           // to a driver's user:{id}
	TypeTripOfferCancelled = "trip.offer.cancelled" // someone else took it / passenger cancelled
	TypeTripStatus         = "trip.status"          // searching, driver_assigned, arriving, started, completed ...
	TypeDriverLocation     = "driver.location"      // live car position on trip:{id}
	TypeBusLocation        = "bus.location"         // live bus position on bus:{id}
	TypeStationCrowd       = "station.crowd"        // cars at a fuel/EV station right now
	TypeFoodOrder          = "food.order"           // luxury-bus meal status
	TypePassScanned        = "pass.scanned"         // daily-commuter QR pass was scanned
	TypeBookingUpdate      = "booking.update"       // premium / tourist / self-drive / test-drive bookings
)

// NewEnvelope marshals data and stamps the server time.
func NewEnvelope(typ, topic string, data any) (Envelope, error) {
	env := Envelope{Type: typ, Topic: topic, TS: time.Now().UnixMilli()}
	if data != nil {
		raw, err := json.Marshal(data)
		if err != nil {
			return Envelope{}, fmt.Errorf("marshal %s payload: %w", typ, err)
		}
		env.Data = raw
	}
	return env, nil
}

// ---------------------------------------------------------------------------
// Topics
// ---------------------------------------------------------------------------

// Topic kinds. A topic string is always "<kind>:<id>".
const (
	KindUser    = "user"    // private inbox of one user (auto-subscribed on connect)
	KindTrip    = "trip"    // one ride: passengers + driver
	KindBus     = "bus"     // one bus run: commuters / long-distance passengers
	KindStation = "station" // one fuel or EV station: public crowd level
)

func UserTopic(userID string) string       { return KindUser + ":" + userID }
func TripTopic(tripID string) string       { return KindTrip + ":" + tripID }
func BusTopic(busID string) string         { return KindBus + ":" + busID }
func StationTopic(stationID string) string { return KindStation + ":" + stationID }

// channelPrefix namespaces our Pub/Sub channels so they never clash with
// anything else that uses the same Redis.
const channelPrefix = "rt:"

func redisChannel(topic string) string { return channelPrefix + topic }

var topicIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ParseTopic splits and validates "kind:id". It rejects wildcards, extra
// colons and unknown kinds so a client can never subscribe to something odd.
func ParseTopic(topic string) (kind, id string, ok bool) {
	kind, id, found := strings.Cut(topic, ":")
	if !found || !topicIDPattern.MatchString(id) {
		return "", "", false
	}
	switch kind {
	case KindUser, KindTrip, KindBus, KindStation:
		return kind, id, true
	}
	return "", "", false
}

// ---------------------------------------------------------------------------
// Identity
// ---------------------------------------------------------------------------

type Role string

const (
	RolePassenger Role = "passenger"
	RoleDriver    Role = "driver"     // cab / shared cab / premium / test-drive drivers
	RoleBusDriver Role = "bus_driver" // commuter bus + long-distance luxury bus
	RoleAdmin     Role = "admin"
)

// Identity is who is on the other end of a socket. It is produced by your
// normal REST auth (JWT/session) and carried into the socket by a ticket.
type Identity struct {
	UserID       string `json:"userId"`
	Role         Role   `json:"role"`
	VehicleID    string `json:"vehicleId,omitempty"`    // car or bus id (drivers, and private cars in Map mode)
	VehicleClass string `json:"vehicleClass,omitempty"` // e.g. hatch5, sedan5, suv7, premium, vintage
}

// ClientError is an error whose Code/Msg are safe to show the client.
type ClientError struct {
	Code string
	Msg  string
}

func (e *ClientError) Error() string { return e.Code + ": " + e.Msg }

func NewClientError(code, msg string) *ClientError { return &ClientError{Code: code, Msg: msg} }

var (
	ErrForbidden    = NewClientError("forbidden", "you are not allowed to do that")
	ErrBadPayload   = NewClientError("bad_payload", "message data is missing or invalid")
	ErrRateLimited  = NewClientError("rate_limited", "slow down")
	ErrTooManyTopic = NewClientError("too_many_topics", "subscription limit reached")
)
