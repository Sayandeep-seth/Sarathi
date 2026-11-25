package events

type StationTrafficUpdated struct {
    Type      string `json:"type"`
    Event     string `json:"event"`
    StationID string `json:"station_id"`
    CarsCount int    `json:"cars_count"`
    Timestamp int64  `json:"timestamp"`
}