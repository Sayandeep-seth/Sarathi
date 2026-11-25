package websocket

import (
    "log"
    "time"

    "github.com/gorilla/websocket"
)

func (c *Client) ReadPump() {
    defer func() {
        c.Hub.Unregister <- c
        c.Conn.Close()
    }()

    c.Conn.SetReadLimit(MaxMessageSize)
    c.Conn.SetReadDeadline(time.Now().Add(PongWait))

    c.Conn.SetPongHandler(func(string) error {
        c.Conn.SetReadDeadline(time.Now().Add(PongWait))
        return nil
    })

    for {
        _, message, err := c.Conn.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(
                err,
                websocket.CloseGoingAway,
                websocket.CloseAbnormalClosure,
            ) {
                log.Printf("WebSocket error: %v", err)
            }
            break
        }

        c.handleIncoming(message)
    }
}

func (c *Client) WritePump() {
    ticker := time.NewTicker(PingPeriod)

    defer func() {
        ticker.Stop()
        c.Conn.Close()
    }()

    for {
        select {
        case message, ok := <-c.Send:
            c.Conn.SetWriteDeadline(time.Now().Add(WriteWait))

            if !ok {
                c.Conn.WriteMessage(
                    websocket.CloseMessage,
                    []byte{},
                )
                return
            }

            if err := c.Conn.WriteMessage(
                websocket.TextMessage,
                message,
            ); err != nil {
                return
            }

        case <-ticker.C:
            c.Conn.SetWriteDeadline(time.Now().Add(WriteWait))

            if err := c.Conn.WriteMessage(
                websocket.PingMessage,
                nil,
            ); err != nil {
                return
            }
        }
    }
}