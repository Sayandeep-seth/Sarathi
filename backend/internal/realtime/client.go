package realtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Timing rules for one socket. The browser/OS answers our ping frames with a
// pong automatically; if no pong (or any other frame) arrives for pongWait the
// read fails and the socket is cleaned up. Mobile networks drop silently, so
// without this you would keep "ghost" drivers online forever.
const (
	writeWait      = 10 * time.Second    // max time to write one frame
	pongWait       = 60 * time.Second    // max silence from the client
	pingPeriod     = (pongWait * 9) / 10 // 54s: must be < pongWait
	maxMessageSize = 8 * 1024            // largest frame a client may send
	sendBufferSize = 256                 // queued outbound frames before we call the client "slow"
	maxTopics      = 50                  // subscriptions per socket
)

// Client is one WebSocket connection. Exactly two goroutines touch the
// underlying *websocket.Conn: readPump (reads) and writePump (writes).
// gorilla/websocket supports one concurrent reader and one concurrent writer,
// nothing more, so every other goroutine sends through the `send` channel.
type Client struct {
	hub    *Hub
	router *Router
	conn   *websocket.Conn
	log    *slog.Logger

	ID     Identity
	ConnID string

	send chan []byte   // outbound frames (already-encoded JSON)
	done chan struct{} // closed exactly once when the client is shutting down

	closeOnce   sync.Once
	closeCode   int
	closeReason string

	// topics is guarded by hub.mu (only the hub mutates it).
	topics map[string]struct{}

	// Only touched from the readPump goroutine, so no lock needed.
	lastLocationAt time.Time
}

func newClient(hub *Hub, router *Router, conn *websocket.Conn, id Identity, log *slog.Logger) *Client {
	connID := randomID()
	return &Client{
		hub:    hub,
		router: router,
		conn:   conn,
		ID:     id,
		ConnID: connID,
		log:    log.With("conn", connID, "user", id.UserID, "role", id.Role),
		send:   make(chan []byte, sendBufferSize),
		done:   make(chan struct{}),
		topics: make(map[string]struct{}),
	}
}

// Enqueue queues a frame without ever blocking the caller. If the client's
// buffer is full it is too slow to keep up (bad network, frozen app) and we
// disconnect it rather than let it slow down fan-out for everyone else.
func (c *Client) Enqueue(frame []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- frame:
		return true
	default:
		c.log.Warn("slow consumer, disconnecting")
		c.Close(websocket.CloseTryAgainLater, "slow consumer")
		return false
	}
}

// SendEnvelope encodes and queues one envelope.
func (c *Client) SendEnvelope(env Envelope) {
	if env.TS == 0 {
		env.TS = time.Now().UnixMilli()
	}
	b, err := json.Marshal(env)
	if err != nil {
		c.log.Error("encode envelope", "err", err)
		return
	}
	c.Enqueue(b)
}

func (c *Client) sendAck(ref string) {
	if ref != "" {
		c.SendEnvelope(Envelope{Type: TypeAck, Ref: ref})
	}
}

func (c *Client) sendError(ref, code, msg string) {
	data, _ := json.Marshal(map[string]string{"code": code, "message": msg})
	c.SendEnvelope(Envelope{Type: TypeError, Ref: ref, Data: data})
}

// Close asks writePump to send a close frame and shut the socket. Safe to call
// many times from any goroutine.
func (c *Client) Close(code int, reason string) {
	c.closeOnce.Do(func() {
		c.closeCode, c.closeReason = code, reason
		close(c.done)
	})
}

// readPump reads frames until the socket dies, then unregisters the client.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.unregister(c)
		c.Close(websocket.CloseNormalClosure, "")
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		msgType, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				c.log.Info("socket closed unexpectedly", "err", err)
			}
			return
		}
		// Any frame proves the client is alive.
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))

		if msgType != websocket.TextMessage {
			c.sendError("", "bad_frame", "only JSON text frames are accepted")
			continue
		}
		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil || env.Type == "" {
			c.sendError("", "bad_json", "frame must be a JSON envelope with a type")
			continue
		}
		c.router.Handle(ctx, c, env)
	}
}

// writePump is the ONLY goroutine that writes to the socket.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close() // makes readPump's ReadMessage fail -> unregister
	}()

	for {
		select {
		case frame := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, frame); err != nil {
				c.Close(websocket.CloseAbnormalClosure, "")
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.Close(websocket.CloseAbnormalClosure, "")
				return
			}
			// Keep the user's presence key alive while the socket is healthy.
			c.hub.presence.Touch(c)

		case <-c.done:
			msg := websocket.FormatCloseMessage(c.closeCode, c.closeReason)
			_ = c.conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(writeWait))
			return
		}
	}
}

// AllowLocation is a tiny per-socket rate limiter for GPS frames. It is only
// called from readPump's goroutine (handlers run there), so no lock is needed.
func (c *Client) AllowLocation(minInterval time.Duration) bool {
	now := time.Now()
	if now.Sub(c.lastLocationAt) < minInterval {
		return false
	}
	c.lastLocationAt = now
	return true
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
