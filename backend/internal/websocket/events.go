package websocket

import (
    "encoding/json"
    "log"
)


func (c *Client) handleIncoming(raw []byte) {
    var message Message

    if err := json.Unmarshal(raw, &message); err != nil {
        log.Printf("Invalid websocket message: %v", err)
        return
    }

    switch message.Event {

    case "ride.join":
        c.handleRideJoin(message)

    case "ride.leave":
        c.handleRideLeave(message)

    case "driver.location.update":
        c.handleDriverLocation(message)

    case "booking.status.request":
        c.handleBookingStatus(message)

    case "ping":
        c.handlePing()

    default:
        log.Printf("Unknown event: %s", message.Event)
    }
}
