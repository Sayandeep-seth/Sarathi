package events

type EVStationUnavailable struct {
    Type      string `json:"type"`
    Event     string `json:"event"`
    StationID string `json:"station_id"`
    Reason    string `json:"reason"`
    Timestamp int64  `json:"timestamp"`
}