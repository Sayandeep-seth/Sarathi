package events

type EVStationAvailable struct {
    Type      string `json:"type"`
    Event     string `json:"event"`
    StationID string `json:"station_id"`
    Available int    `json:"available_chargers"`
    Timestamp int64  `json:"timestamp"`
}