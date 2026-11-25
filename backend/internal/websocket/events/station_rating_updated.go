package events

type StationRatingUpdated struct {
    Type      string  `json:"type"`
    Event     string  `json:"event"`
    StationID string  `json:"station_id"`
    Rating    float64 `json:"rating"`
    Timestamp int64   `json:"timestamp"`
}