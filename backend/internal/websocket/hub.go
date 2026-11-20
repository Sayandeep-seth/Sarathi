// Package websocket delivers realtime trip, location, offer and order events.
// Every API pod runs a Hub that PSUBSCRIBEs to "sr:ch:*" in Redis, so an event
// published by any pod (or the worker) reaches sockets connected to any pod.
package websocket

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	ws "github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

type Hub struct {
	rdb        *redis.Client
	instanceID string
	log        *slog.Logger
	upgrader   ws.Upgrader

	mu     sync.RWMutex
	topics map[string]map[*Client]struct{}
	subs   map[*Client]map[string]struct{}
}

func NewHub(rdb *redis.Client, instanceID string, log *slog.Logger) *Hub {
	return &Hub{
		rdb: rdb, instanceID: instanceID, log: log,
		upgrader: ws.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024, CheckOrigin: func(*http.Request) bool { return true }},
		topics:   map[string]map[*Client]struct{}{},
		subs:     map[*Client]map[string]struct{}{},
	}
}

// Run fans Redis pub/sub messages out to local sockets until ctx ends.
func (h *Hub) Run(ctx context.Context) {
	ps := h.rdb.PSubscribe(ctx, cache.ChannelPrefix+"*")
	defer ps.Close()
	ch := ps.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			topic := TopicOf(msg.Channel)
			h.mu.RLock()
			for c := range h.topics[topic] {
				c.trySend([]byte(msg.Payload))
			}
			h.mu.RUnlock()
		}
	}
}

// ServeWS upgrades GET /ws. The user is auto-subscribed to user:<id> and driver:<id>.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	userID := utils.UserID(r)
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Debug("ws upgrade", "err", err)
		return
	}
	c := &Client{hub: h, conn: conn, send: make(chan []byte, 64), userID: userID}
	h.subscribe(c, "user:"+userID)
	h.subscribe(c, "driver:"+userID)
	ctx := context.WithoutCancel(r.Context())
	h.touchPresence(ctx, userID)
	go c.writePump()
	go c.readPump(ctx)
}

// IsOnline reports whether a user has a live socket on any pod.
func (h *Hub) IsOnline(ctx context.Context, userID string) bool {
	n, _ := h.rdb.Exists(ctx, cache.KeyWSPresence(userID)).Result()
	return n == 1
}

func (h *Hub) touchPresence(ctx context.Context, userID string) {
	h.rdb.Set(ctx, cache.KeyWSPresence(userID), h.instanceID, cache.TTLWSPresence)
}

func (h *Hub) subscribe(c *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.topics[topic] == nil {
		h.topics[topic] = map[*Client]struct{}{}
	}
	h.topics[topic][c] = struct{}{}
	if h.subs[c] == nil {
		h.subs[c] = map[string]struct{}{}
	}
	h.subs[c][topic] = struct{}{}
}

func (h *Hub) unsubscribe(c *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.topics[topic], c)
	if len(h.topics[topic]) == 0 {
		delete(h.topics, topic)
	}
	delete(h.subs[c], topic)
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	for t := range h.subs[c] {
		delete(h.topics[t], c)
		if len(h.topics[t]) == 0 {
			delete(h.topics, t)
		}
	}
	_, existed := h.subs[c]
	delete(h.subs, c)
	h.mu.Unlock()
	if existed {
		close(c.send)
	}
}
