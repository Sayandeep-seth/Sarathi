package websocket

func (c *Client) handleRideJoin(message Message) {
    if message.RideID == "" {
        return
    }

    roomID := "ride:" + message.RideID

    c.Hub.JoinRoom(roomID, c)
    c.RideID = message.RideID
}

func (c *Client) handleRideLeave(message Message) {
    if message.RideID == "" {
        return
    }

    c.Hub.LeaveRoom("ride:"+message.RideID, c)
    c.RideID = ""
}

func (c *Client) handlePing() {
    // A lightweight app-level heartbeat acknowledgement.
}

func (c *Client) handleBookingStatus(message Message) {
    // Later connect this to booking service.
}

func (c *Client) handleDriverLocation(message Message) {
    // Next step: connect GPS -> Redis -> Pub/Sub -> ride room.
}

import (
    "net/http"

    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,

    // Restrict this to your frontend origins in production.
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

func ServeWS(hub *Hub, userID, role string, w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }

    client := &Client{
        Hub:    hub,
        Conn:   conn,
        Send:   make(chan []byte, 256),
        UserID: userID,
        Role:   role,
    }

    hub.Register <- client

    go client.WritePump()
    go client.ReadPump()
}

