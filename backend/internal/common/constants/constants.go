// Package constants holds enums shared by several modules.
package constants

// Booking types (Trip part).
const (
	BookingRide          = "ride"           // 1) normal cab, no parking fee
	BookingShared        = "shared"         // 2) shared cab, cheaper
	BookingPickupReturn  = "pickup_return"  // 3) pick someone up and come back, parking fee
	BookingPremiumHourly = "premium_hourly" // premium car for events, per hour
	BookingVintage       = "vintage"        // vintage car for ceremonies, per hour
	BookingTourist       = "tourist"        // tourist trip, per day
	BookingSelfDrive     = "self_drive"     // drive yourself, price bands + deposit
	BookingTestDrive     = "test_drive"     // customer test drive at a showroom
	BookingTestDriveCab  = "test_drive_cab" // dealer test-drive car used as cab
)

// Vehicle classes = Redis GEO sets of available drivers ("sr:geo:drivers:<class>").
const (
	ClassCab       = "cab"
	ClassSeat5     = "seat5"
	ClassSeat7     = "seat7"
	ClassShared    = "shared"
	ClassPremium   = "premium"
	ClassVintage   = "vintage"
	ClassTourist   = "tourist"
	ClassTestDrive = "testdrive"
)

// Fuel types.
const (
	FuelPetrol = "petrol"
	FuelDiesel = "diesel"
	FuelCNG    = "cng"
	FuelEV     = "ev"
	FuelHybrid = "hybrid"
)

// Station kinds.
const (
	StationPetrol = "petrol"
	StationEV     = "ev"
)

// Public transport modes accepted by the transit QR.
const (
	ModeTrain = "train"
	ModeMetro = "metro"
	ModeBus   = "bus"
	ModeCab   = "cab"
)

// Hub place types for pickup & return.
const (
	PlaceAirport = "airport"
	PlaceRailway = "railway"
	PlaceMetro   = "metro"
	PlaceBus     = "bus_stand"
)

// HeaderUserID is set by the auth gateway after JWT validation.
const HeaderUserID = "X-User-ID"
