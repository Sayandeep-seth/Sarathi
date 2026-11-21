package websocket

import "encoding/json"

type Message struct {
    Type      string          `json:"type"`
    Event     string          `json:"event"`
    UserID   string          `json:"user_id,omitempty"`
    RideID   string          `json:"ride_id,omitempty"`
    RoomID   string          `json:"room_id,omitempty"`
    Timestamp int64           `json:"timestamp,omitempty"`
    Data      json.RawMessage `json:"data,omitempty"`
}