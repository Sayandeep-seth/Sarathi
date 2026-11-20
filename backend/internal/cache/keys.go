package cache

import (
	"fmt"
	"strings"
)

// Every Redis key used by Sarathi is defined here so the key-space is documented
// in one place. Format: sr:<module>:<entity>:<id>. Keys wrapped in {…} carry a
// Redis-Cluster hash tag so a Lua script can touch the key and its derived keys.

const ns = "sr:"

func key(parts ...any) string {
	s := make([]string, len(parts))
	for i, p := range parts {
		s[i] = fmt.Sprint(p)
	}
	return ns + strings.Join(s, ":")
}

// tagged builds "sr:{a:b:c}" — a hash-tagged key for slot scripts.
func tagged(parts ...any) string {
	s := make([]string, len(parts))
	for i, p := range parts {
		s[i] = fmt.Sprint(p)
	}
	return ns + "{" + strings.Join(s, ":") + "}"
}

// ---------- generic ----------

func KeyLock(name string) string                  { return key("lock", name) }
func KeyRateLimit(scope, id string) string        { return key("rl", scope, id) }
func KeyIdempotency(scope, idemKey string) string { return key("idem", scope, idemKey) }

// ---------- vehicles (car specifications) ----------

func KeyCarSpec(model string) string        { return key("vehicle", "spec", model) }         // STRING json
func KeyCarSpecMissing(model string) string { return key("vehicle", "spec", "miss", model) } // negative cache
func KeyCarModelIndex() string              { return key("vehicle", "models") }              // ZSET (lex) autocomplete

// ---------- map ----------

func KeyRoute(hash string) string                 { return key("map", "route", hash) }             // STRING json routes
func KeyRoadAttr(roadID string) string            { return key("map", "road", roadID) }            // HASH width,oneway,lanes…
func KeyRoadCompat(model string) string           { return key("map", "compat", model) }           // HASH roadID -> 1/0
func KeyRoadTraffic(roadID, bucket string) string { return key("map", "traffic", roadID, bucket) } // HASH sum,count
func KeyCarEnvelope(model string) string          { return key("map", "envelope", model) }         // HASH width,height,length

// ---------- stations ----------

func KeyStationGeo(kind string) string        { return key("geo", "stations", kind) }                 // GEO
func KeyStation(kind, id string) string       { return key("station", kind, id) }                     // HASH
func KeyStationVisits(id, date string) string { return key("station", "petrol", id, "visits", date) } // HLL
func KeyStationLive(id string) string         { return key("station", "petrol", id, "live") }         // ZSET vehicle->ts
func KeyStationHourly(id, hour string) string { return key("station", "petrol", id, "hourly", hour) } // STRING counter
func KeyStationRank(kind string) string       { return key("rank", "stations", kind) }                // ZSET id->rating
func KeyStationVisitDedup(vehicleID, stationID string) string {
	return key("station", "seen", vehicleID, stationID)
}
func KeyEVPorts(id string) string { return key("station", "ev", id, "ports") } // HASH free,total,updated

// ---------- gps ----------

func KeyVehiclesLiveGeo() string            { return key("geo", "vehicles", "live") } // GEO
func KeyVehiclesLastSeen() string           { return key("gps", "lastseen") }         // ZSET vehicle->unix
func KeyVehicleLoc(vehicleID string) string { return key("gps", "loc", vehicleID) }   // HASH
func KeyTripTrack(tripID string) string     { return key("gps", "track", tripID) }    // STREAM

const StreamGPSPersist = ns + "stream:gps:persist"

// ---------- driver ----------

func KeyDriver(id string) string          { return key("driver", id) }            // HASH
func KeyDriverHeartbeat(id string) string { return key("driver", id, "hb") }      // STRING ttl
func KeyDriversGeo(class string) string   { return key("geo", "drivers", class) } // GEO
func KeyDriversLastSeen() string          { return key("drivers", "lastseen") }   // ZSET id->unix

// ---------- dispatch ----------

func KeyDriverLockName(driverID string) string { return "driver:" + driverID }                   // used with KeyLock
func KeyOffer(bookingID string) string         { return key("dispatch", "offer", bookingID) }    // HASH
func KeyOfferRejected(bookingID string) string { return key("dispatch", "rejected", bookingID) } // SET
func KeyOfferDeadlines() string                { return key("dispatch", "deadlines") }           // ZSET bookingID->unixms

// ---------- booking ----------

func KeyBooking(id string) string               { return key("booking", id) } // HASH
func KeyUserActiveBooking(userID string) string { return key("user", userID, "active_booking") }
func KeyBookingScheduled() string               { return key("booking", "scheduled") } // ZSET id->pickup unix

// ---------- shared ride ----------

func KeySharedOpenGeo() string         { return key("geo", "shared", "open") }        // GEO trip pickup
func KeySharedTrip(id string) string   { return key("shared", "trip", id) }           // HASH
func KeySharedRiders(id string) string { return key("shared", "trip", id, "riders") } // SET

// ---------- pickup & return ----------

func KeyHubsGeo() string                   { return key("geo", "hubs") }              // GEO airports/stations
func KeyHub(id string) string              { return key("hub", id) }                  // HASH
func KeyParkingTariff(hubID string) string { return key("parking", "tariff", hubID) } // HASH

// ---------- premium / vintage ----------

func KeyPremiumCar(id string) string               { return key("premium", "car", id) }             // HASH
func KeyPremiumFleet(category string) string       { return key("premium", "fleet", category) }     // SET
func KeyPremiumCalendar(carID, date string) string { return tagged("premium", "cal", carID, date) } // BITMAP 24h

// ---------- tourist ----------

func KeyTouristCar(id string) string         { return key("tourist", "car", id) }
func KeyTouristFleet(class string) string    { return key("tourist", "fleet", class) }
func KeyTouristCalendar(carID string) string { return tagged("tourist", "cal", carID) } // BITMAP day-index
func KeyTouristRates(class string) string    { return key("tourist", "rates", class) }
func KeyTouristItinerary(dest string) string { return key("tourist", "itinerary", dest) }

// ---------- self drive ----------

func KeySelfDriveBands() string                { return key("selfdrive", "bands") }              // STRING json
func KeySelfDriveCar(id string) string         { return key("selfdrive", "car", id) }            // HASH
func KeySelfDriveBandCars(band string) string  { return key("selfdrive", "band", band, "cars") } // SET
func KeySelfDriveGeo() string                  { return key("geo", "selfdrive") }                // GEO
func KeySelfDriveCalendar(carID string) string { return tagged("selfdrive", "cal", carID) }      // BITMAP
func KeySelfDriveReservation(id string) string { return key("selfdrive", "res", id) }            // HASH
func KeySelfDriveDeposit(resID string) string  { return key("selfdrive", "deposit", resID) }     // HASH

// ---------- transit pass ----------

func KeyTransitRoutes(userID string) string           { return key("transit", "routes", userID) } // HASH name->json
func KeyTransitJourney(id string) string              { return key("transit", "journey", id) }    // HASH
func KeyTransitUserJourneys(userID string) string     { return key("transit", "user", userID) }   // ZSET
func KeyTransitQR(nonce string) string                { return key("transit", "qr", nonce) }      // STRING journeyID
func KeyTransitScan(journeyID string, leg int) string { return key("transit", "scan", journeyID, leg) }
func KeyTransitFare(mode, from, to string) string     { return key("transit", "fare", mode, from, to) }

// ---------- daily bus ----------

func KeyDailyBusRoute(id string) string           { return key("dbus", "route", id) }              // STRING json
func KeyDailyBusRoutes() string                   { return key("dbus", "routes") }                 // SET
func KeyDailyBusStopsGeo() string                 { return key("geo", "dbus", "stops") }           // GEO "route|stop"
func KeyDailyBusSeats(tripID, date string) string { return tagged("dbus", "seats", tripID, date) } // BITMAP
func KeyDailyBusTrip(id string) string            { return key("dbus", "trip", id) }               // HASH
func KeyDailyBusSubscription(id string) string    { return key("dbus", "sub", id) }                // HASH

// ---------- test drive ----------

func KeyTDUsed(showroomID, model string) string   { return key("td", "used", showroomID, model) }      // SET customerIDs
func KeyTDZone(showroomID string) string          { return key("td", "zone", showroomID) }             // HASH lat,lng,radius
func KeyTDCar(vehicleID string) string            { return key("td", "car", vehicleID) }               // HASH
func KeyTDShowroomCars(showroomID string) string  { return key("td", "showroom", showroomID, "cars") } // SET
func KeyTDSlots(carID, date string) string        { return tagged("td", "slots", carID, date) }        // BITMAP 48 half-hours
func KeyTDEarnings(dealerID, month string) string { return key("td", "earn", dealerID, month) }        // HASH
func KeyTDZoneAlert(vehicleID string) string      { return key("td", "alert", vehicleID) }

const StreamTDLedger = ns + "stream:td:ledger"

// ---------- luxury bus ----------

func KeyLuxTrip(id string) string                   { return key("lbus", "trip", id) }                     // HASH
func KeyLuxSeats(tripID string) string              { return tagged("lbus", "seats", tripID) }             // BITMAP
func KeyLuxTicket(pnr string) string                { return key("lbus", "ticket", pnr) }                  // HASH
func KeyLuxStops(tripID string) string              { return key("lbus", "trip", tripID, "stops") }        // ZSET stop->eta
func KeyLuxStop(tripID, stopID string) string       { return key("lbus", "trip", tripID, "stop", stopID) } // HASH
func KeyLuxMenu(restaurantID string) string         { return key("lbus", "menu", restaurantID) }           // HASH item->json
func KeyLuxFoodAgg(tripID, stopID string) string    { return key("lbus", "food", tripID, stopID) }         // HASH item->qty
func KeyLuxTicketFood(pnr string) string            { return key("lbus", "ticket", pnr, "food") }          // LIST order ids
func KeyLuxFoodOrder(orderID string) string         { return key("lbus", "order", orderID) }               // HASH
func KeyLuxDispatching() string                     { return key("lbus", "dispatching") }                  // ZSET trip|stop -> eta
func KeyLuxCommission(ownerID, month string) string { return key("lbus", "commission", ownerID, month) }   // HASH

// ---------- payments ----------

func KeyPaymentIntent(id string) string       { return key("pay", "intent", id) }       // HASH
func KeyPaymentRef(module, ref string) string { return key("pay", "ref", module, ref) } // STRING intentID
func KeyWebhookEvent(eventID string) string   { return key("pay", "webhook", eventID) }
func KeyRefund(id string) string              { return key("pay", "refund", id) }

// ---------- notifications ----------

const (
	StreamNotifications    = ns + "stream:notifications"
	StreamNotificationsDLQ = ns + "stream:notifications:dlq"
)

func KeyUserDevices(userID string) string { return key("user", userID, "devices") } // SET push tokens
func KeyUserContact(userID string) string { return key("user", userID, "contact") } // HASH phone,email

// ---------- websocket ----------

func KeyWSPresence(userID string) string { return key("ws", "presence", userID) } // STRING instance id

// ---------- pricing ----------

func KeyRateCard(class string) string                 { return key("pricing", "rate", class) } // HASH
func KeySurgeDemand(cell string, bucket int64) string { return key("surge", "demand", cell, bucket) }
func KeySurgeMultiplier(cell string) string           { return key("surge", "mult", cell) }

// ---------- analytics ----------

func KeyStatCounter(metric, date string) string     { return key("stats", "c", metric, date) }
func KeyStatUnique(metric, date string) string      { return key("stats", "u", metric, date) }            // HLL
func KeyStatRides(date string) string               { return key("stats", "rides", date) }                // HASH type->count
func KeyStatRevenue(date string) string             { return key("stats", "revenue", date) }              // HASH type->amount
func KeyStatHeatmap(date string) string             { return key("stats", "heat", date) }                 // ZSET cell->count
func KeyStatDriverEarnings(month string) string     { return key("stats", "driver", "earn", month) }      // ZSET
func KeyStatDriverOffers(driverID string) string    { return key("stats", "driver", "offers", driverID) } // HASH
func KeyStatStationVisits(kind, date string) string { return key("stats", "station", kind, date) }        // ZSET

// ---------- ai ----------

func KeyAIModelVersion(name string) string       { return key("ai", "model", name, "version") }
func KeyAIModelMeta(name, version string) string { return key("ai", "model", name, version) }
func KeyAIPrediction(model, version, hash string) string {
	return key("ai", "pred", model, version, hash)
}

// ---------- pub/sub channels (websocket topic = channel without ChannelPrefix) ----------

const ChannelPrefix = ns + "ch:"

func ChanTrip(tripID string) string     { return ChannelPrefix + "trip:" + tripID }
func ChanTripLoc(tripID string) string  { return ChannelPrefix + "trip:" + tripID + ":loc" }
func ChanDriver(driverID string) string { return ChannelPrefix + "driver:" + driverID }
func ChanUser(userID string) string     { return ChannelPrefix + "user:" + userID }
func ChanRestaurant(id string) string   { return ChannelPrefix + "restaurant:" + id }
func ChanDealer(dealerID string) string { return ChannelPrefix + "dealer:" + dealerID }
func ChanPayments() string              { return ChannelPrefix + "payments" }
func ChanAIModel() string               { return ChannelPrefix + "ai:model" }
