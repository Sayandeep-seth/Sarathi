package websocket

import (
	"context"
	"encoding/json"
	"time"

	ws "github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second
	maxMessage = 4096
)

// Client is one connected socket.
type Client struct {
	hub    *Hub
	conn   *ws.Conn
	send   chan []byte
	userID string
}

type clientMsg struct {
	Action string `json:"action"` // subscribe | unsubscribe
	Topic  string `json:"topic"`
}

// readPump handles subscribe/unsubscribe requests from the app.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.remove(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessage)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.hub.touchPresence(ctx, c.userID)
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var m clientMsg
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.Action {
		case "subscribe":
			if Authorize(ctx, c.hub.rdb, c.userID, m.Topic) {
				c.hub.subscribe(c, m.Topic)
				c.trySend(mustJSON(map[string]string{"type": "subscribed", "topic": m.Topic}))
			} else {
				c.trySend(mustJSON(map[string]string{"type": "error", "topic": m.Topic, "message": "forbidden"}))
			}
		case "unsubscribe":
			c.hub.unsubscribe(c, m.Topic)
		}
	}
}

// writePump forwards hub messages and keeps the connection alive.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(ws.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(ws.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(ws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// trySend never blocks the hub: slow clients drop messages.
func (c *Client) trySend(b []byte) {
	select {
	case c.send <- b:
	default:
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
