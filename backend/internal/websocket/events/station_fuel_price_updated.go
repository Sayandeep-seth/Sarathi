package events

type StationFuelPriceUpdated struct {
    Type      string  `json:"type"`
    Event     string  `json:"event"`
    StationID string  `json:"station_id"`
    FuelType  string  `json:"fuel_type"`
    Price     float64 `json:"price"`
    Timestamp int64   `json:"timestamp"`
}