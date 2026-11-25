package handlers

import (
    "encoding/json"
    "log"
)

type CommuterEvent struct {
    Type      string          `json:"type"`
    Event     string          `json:"event"`
    UserID    string          `json:"user_id"`
    TripID    string          `json:"trip_id,omitempty"`
    Timestamp int64           `json:"timestamp"`
    Data      json.RawMessage `json:"data"`
}

func HandleCommuterEvent(payload []byte) {
    var event CommuterEvent

    if err := json.Unmarshal(payload, &event); err != nil {
        log.Println("Invalid commuter event:", err)
        return
    }

    switch event.Event {
    case "commuter.pass.activated":
        log.Println("Commuter pass activated")

    case "commuter.qr.generated":
        log.Println("Commuter QR generated")

    case "commuter.qr.validated":
        log.Println("Commuter QR validated")

    case "commuter.route.updated":
        log.Println("Commuter route updated")

    case "commuter.trip.started":
        log.Println("Commuter trip started")

    case "commuter.trip.completed":
        log.Println("Commuter trip completed")

    default:
        log.Println("Unknown commuter event:", event.Event)
    }
}