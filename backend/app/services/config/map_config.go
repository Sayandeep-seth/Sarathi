package config

// MapConfig drives the Map part: routing, car-size road compatibility and fuel stops.
type MapConfig struct {
	// RoutingURL is an OSRM-compatible routing server (returns alternative routes).
	RoutingURL string
	// CarSpecProviderURL is the live car-specification source used when a model
	// is not yet in Postgres/Redis (GET {url}?model=<name> -> JSON spec).
	CarSpecProviderURL string

	// SafetyClearanceM is the free gap we demand on each side of a vehicle.
	SafetyClearanceM float64
	// OncomingVehicleWidthM is the width of the widest vehicle we expect from the
	// opposite direction on a two-way road (default: a large SUV with mirrors).
	OncomingVehicleWidthM float64
	// DefaultLaneWidthM is used when a road has a lane count but no measured width.
	DefaultLaneWidthM float64

	MaxAlternatives       int
	StationSearchRadiusKm float64
	StationSampleEveryKm  float64
	// ReserveFuelPct of the range we never plan to use (safety margin).
	ReserveFuelPct float64
}

func loadMap() MapConfig {
	return MapConfig{
		RoutingURL:            getEnv("ROUTING_URL", "http://localhost:5000"),
		CarSpecProviderURL:    getEnv("CAR_SPEC_PROVIDER_URL", ""),
		SafetyClearanceM:      getFloat("SAFETY_CLEARANCE_M", 0.30),
		OncomingVehicleWidthM: getFloat("ONCOMING_VEHICLE_WIDTH_M", 2.10),
		DefaultLaneWidthM:     getFloat("DEFAULT_LANE_WIDTH_M", 3.0),
		MaxAlternatives:       getInt("MAX_ROUTE_ALTERNATIVES", 3),
		StationSearchRadiusKm: getFloat("STATION_SEARCH_RADIUS_KM", 2.0),
		StationSampleEveryKm:  getFloat("STATION_SAMPLE_EVERY_KM", 2.0),
		ReserveFuelPct:        getFloat("RESERVE_FUEL_PCT", 10),
	}
}
