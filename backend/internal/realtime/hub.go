// Package realtime fans Redis pub/sub events out to WebSocket clients.
//
// Every API replica subscribes to the same Redis channels, so an event published by
// any replica reaches clients connected to all of them. Clients receive the public
// fleet topic plus their own private user topic. The socket is server-push only.
package realtime

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/sarathi/sarathi/backend/internal/live"
)

const (
	sendBuffer     = 64
	writeTimeout   = 5 * time.Second
	pingInterval   = 25 * time.Second
	maxPerUser     = 5
	maxConnections = 20000
)

type client struct {
	userID string
	send   chan []byte
	cancel context.CancelFunc
}

// Hub tracks connected clients on this replica.
type Hub struct {
	log     *slog.Logger
	rdb     *redis.Client
	origins []string

	mu      sync.RWMutex
	clients map[*client]struct{}
	perUser map[string]int
	closing bool
}

// NewHub creates a hub. allowedOrigins are the configured CORS origins; same-host
// upgrades are always allowed, anything else must match one of them.
func NewHub(log *slog.Logger, rdb *redis.Client, allowedOrigins []string) *Hub {
	h := &Hub{log: log, rdb: rdb, clients: map[*client]struct{}{}, perUser: map[string]int{}}
	for _, o := range allowedOrigins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			h.origins = append(h.origins, u.Host)
		}
	}
	return h
}

// Connections returns the number of clients on this replica.
func (h *Hub) Connections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Run consumes Redis pub/sub until ctx is cancelled. go-redis resubscribes
// automatically after connection loss.
func (h *Hub) Run(ctx context.Context) {
	ps := h.rdb.PSubscribe(ctx, live.ChannelPrefix+"*")
	defer ps.Close()
	ch := ps.Channel(redis.WithChannelSize(1024))
	for {
		select {
		case <-ctx.Done():
			h.closeAll()
			return
		case m, ok := <-ch:
			if !ok {
				return
			}
			h.dispatch(strings.TrimPrefix(m.Channel, live.ChannelPrefix), []byte(m.Payload))
		}
	}
}

func (h *Hub) dispatch(topic string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if topic != live.FleetTopic && topic != live.UserTopic(c.userID) {
			continue
		}
		select {
		case c.send <- payload:
		default:
			// A client that cannot keep up is disconnected; it will reconnect and resync.
			c.cancel()
		}
	}
}

func (h *Hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closing = true
	for c := range h.clients {
		c.cancel()
	}
}

var errBusy = errors.New("connection limit reached")

func (h *Hub) add(c *client) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closing || len(h.clients) >= maxConnections || h.perUser[c.userID] >= maxPerUser {
		return errBusy
	}
	h.clients[c] = struct{}{}
	h.perUser[c.userID]++
	return nil
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	if h.perUser[c.userID]--; h.perUser[c.userID] <= 0 {
		delete(h.perUser, c.userID)
	}
}

// Serve upgrades the request and streams events for userID until the peer leaves.
// The caller must already have authenticated the user.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, userID string) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.origins})
	if err != nil {
		return // Accept already wrote the HTTP error.
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1024)

	// The request context is tied to middleware deadlines; the socket outlives them.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &client{userID: userID, send: make(chan []byte, sendBuffer), cancel: cancel}
	if err := h.add(c); err != nil {
		_ = conn.Close(websocket.StatusTryAgainLater, "too many connections")
		return
	}
	defer h.remove(c)

	// CloseRead discards inbound data frames and cancels readCtx when the peer closes.
	readCtx := conn.CloseRead(ctx)
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()

	_ = write(ctx, conn, []byte(`{"type":"hello"}`))
	for {
		select {
		case <-readCtx.Done():
			_ = conn.Close(websocket.StatusGoingAway, "")
			return
		case msg := <-c.send:
			if write(ctx, conn, msg) != nil {
				return
			}
		case <-ping.C:
			pctx, pcancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Ping(pctx)
			pcancel()
			if err != nil {
				return
			}
		}
	}
}

func write(ctx context.Context, conn *websocket.Conn, msg []byte) error {
	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return conn.Write(wctx, websocket.MessageText, msg)
}
