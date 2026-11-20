package cache

import "time"

// Expiration policy for every key family. Values are tuned for freshness vs. load:
// live data (locations, offers) is seconds, reference data (car specs) is days.
const (
	// vehicles / map
	TTLCarSpec        = 7 * 24 * time.Hour
	TTLCarSpecMissing = 10 * time.Minute
	TTLRoute          = 5 * time.Minute
	TTLRoadAttr       = 24 * time.Hour
	TTLRoadCompat     = 24 * time.Hour
	TTLTrafficBucket  = 15 * time.Minute
	TTLCarEnvelope    = 24 * time.Hour

	// stations
	TTLStation           = 24 * time.Hour
	TTLStationLiveWindow = 15 * time.Minute // a car seen in the last 15 min counts as "in the queue"
	TTLStationVisits     = 35 * 24 * time.Hour
	TTLStationHourly     = 8 * 24 * time.Hour
	TTLVisitDedup        = 30 * time.Minute
	TTLEVPorts           = 10 * time.Minute

	// gps / drivers / dispatch
	TTLVehicleLoc      = 2 * time.Minute
	TTLTripTrack       = 48 * time.Hour
	TTLDriverHeartbeat = 45 * time.Second
	TTLDriverStale     = 90 * time.Second
	TTLOffer           = 20 * time.Second
	TTLDriverLock      = 25 * time.Second

	// bookings & reservations
	TTLBooking       = 48 * time.Hour
	TTLIdempotency   = 24 * time.Hour
	TTLSharedTrip    = 3 * time.Hour
	TTLSeatHold      = 10 * time.Minute
	TTLCalendarHold  = 15 * time.Minute
	TTLParkingTariff = 6 * time.Hour
	TTLItinerary     = 12 * time.Hour
	TTLFleetCar      = 24 * time.Hour
	TTLDailyBusSeats = 3 * 24 * time.Hour
	TTLLuxTrip       = 7 * 24 * time.Hour
	TTLTDZoneAlert   = 5 * time.Minute

	// transit
	TTLTransitJourney = 24 * time.Hour
	TTLTransitScan    = 24 * time.Hour
	TTLFare           = 24 * time.Hour

	// pricing
	TTLRateCard        = time.Hour
	TTLSurgeDemand     = 15 * time.Minute
	TTLSurgeMultiplier = 60 * time.Second

	// payments
	TTLPaymentIntent = 7 * 24 * time.Hour
	TTLWebhookEvent  = 7 * 24 * time.Hour

	// misc
	TTLStats        = 90 * 24 * time.Hour
	TTLAIPrediction = 10 * time.Minute
	TTLWSPresence   = 90 * time.Second
)
