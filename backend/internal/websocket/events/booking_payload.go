
package events

import "time"

type BookingEvent struct {
    Type      string      `json:"type"`
    Event     string      `json:"event"`
    BookingID string      `json:"booking_id"`
    UserID    string      `json:"user_id"`
    DriverID  string      `json:"driver_id,omitempty"`
    Timestamp time.Time   `json:"timestamp"`
    Data      interface{} `json:"data,omitempty"`
}