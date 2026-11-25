package realtime

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Hub keeps track of the sockets connected to THIS gateway process and bridges
// them to Redis Pub/Sub so that a message published by any process (another
// gateway, the REST API, a worker) reaches every interested socket.
//
//	REST API / worker --PUBLISH rt:trip:42--> Redis --> every gateway that
//	has at least one local subscriber to trip:42 --> those sockets
//
// The gateway only SUBSCRIBEs to a Redis channel while it has at least one
// local socket interested in that topic, so traffic stays proportional to
// what is actually being watched.
type Hub struct {
	rdb       redis.UniversalClient
	ps        *redis.PubSub
	gatewayID string
	log       *slog.Logger
	presence  *Presence

	mu      sync.RWMutex
	clients map[*Client]struct{}
	topics  map[string]map[*Client]struct{}

	// Redis SUBSCRIBE/UNSUBSCRIBE are done by one goroutine (reconcileLoop)
	// so they can never race with each other. Callers just mark a topic as
	// "needs checking"; the loop compares local interest with what Redis has.
	pendingMu sync.Mutex
	pending   map[string]struct{}
	wake      chan struct{}
	redisSubs map[string]bool // owned by reconcileLoop only
}

func NewHub(rdb redis.UniversalClient, gatewayID string, log *slog.Logger) *Hub {
	return &Hub{
		rdb:       rdb,
		gatewayID: gatewayID,
		log:       log.With("component", "hub"),
		presence:  NewPresence(rdb),
		clients:   make(map[*Client]struct{}),
		topics:    make(map[string]map[*Client]struct{}),
		pending:   make(map[string]struct{}),
		wake:      make(chan struct{}, 1),
		redisSubs: make(map[string]bool),
	}
}

// Run blocks until ctx is cancelled. It must be running before the HTTP
// server starts accepting sockets.
func (h *Hub) Run(ctx context.Context) error {
	// Subscribe to a per-gateway control channel first. This opens the
	// dedicated Pub/Sub connection; Receive waits for Redis to confirm it.
	h.ps = h.rdb.Subscribe(ctx, channelPrefix+"_gw:"+h.gatewayID)
	if _, err := h.ps.Receive(ctx); err != nil {
		return err
	}
	defer h.ps.Close()

	go h.reconcileLoop(ctx)

	// go-redis reconnects and re-subscribes automatically if Redis restarts.
	msgs := h.ps.Channel(redis.WithChannelSize(4096))
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-msgs:
			if !ok {
				return nil
			}
			topic := strings.TrimPrefix(msg.Channel, channelPrefix)
			h.deliverLocal(topic, []byte(msg.Payload))
		}
	}
}

// deliverLocal pushes an already-encoded frame to every local subscriber.
// We copy the subscriber list under a read lock, then send outside the lock,
// so one slow client never blocks the hub.
func (h *Hub) deliverLocal(topic string, frame []byte) {
	h.mu.RLock()
	subs := h.topics[topic]
	targets := make([]*Client, 0, len(subs))
	for c := range subs {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		c.Enqueue(frame)
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	// Every socket automatically listens to its owner's private inbox.
	_ = h.subscribe(c, UserTopic(c.ID.UserID))
	h.presence.Online(c)
	h.log.Info("client connected", "user", c.ID.UserID, "conn", c.ConnID)
}

func (h *Hub) unregister(c *Client) {
	var emptied []string
	h.mu.Lock()
	if _, ok := h.clients[c]; !ok {
		h.mu.Unlock()
		return
	}
	delete(h.clients, c)
	for topic := range c.topics {
		if set := h.topics[topic]; set != nil {
			delete(set, c)
			if len(set) == 0 {
				delete(h.topics, topic)
				emptied = append(emptied, topic)
			}
		}
	}
	c.topics = map[string]struct{}{}
	h.mu.Unlock()

	for _, t := range emptied {
		h.markPending(t)
	}
	h.presence.Offline(c)
	h.log.Info("client disconnected", "user", c.ID.UserID, "conn", c.ConnID)
}

// subscribe adds c to topic. Authorization must already have been checked.
func (h *Hub) subscribe(c *Client, topic string) error {
	h.mu.Lock()
	if _, already := c.topics[topic]; already {
		h.mu.Unlock()
		return nil
	}
	if len(c.topics) >= maxTopics {
		h.mu.Unlock()
		return ErrTooManyTopic
	}
	set := h.topics[topic]
	first := set == nil
	if first {
		set = make(map[*Client]struct{})
		h.topics[topic] = set
	}
	set[c] = struct{}{}
	c.topics[topic] = struct{}{}
	h.mu.Unlock()

	if first {
		h.markPending(topic)
	}
	return nil
}

func (h *Hub) unsubscribe(c *Client, topic string) {
	if topic == UserTopic(c.ID.UserID) {
		return // the private inbox cannot be left
	}
	h.mu.Lock()
	delete(c.topics, topic)
	set := h.topics[topic]
	emptied := false
	if set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.topics, topic)
			emptied = true
		}
	}
	h.mu.Unlock()

	if emptied {
		h.markPending(topic)
	}
}

func (h *Hub) markPending(topic string) {
	h.pendingMu.Lock()
	h.pending[topic] = struct{}{}
	h.pendingMu.Unlock()
	select {
	case h.wake <- struct{}{}:
	default: // loop is already awake
	}
}

func (h *Hub) reconcileLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.wake:
		}

		h.pendingMu.Lock()
		batch := h.pending
		h.pending = make(map[string]struct{})
		h.pendingMu.Unlock()

		var toSub, toUnsub []string
		h.mu.RLock()
		for topic := range batch {
			want := len(h.topics[topic]) > 0
			have := h.redisSubs[topic]
			switch {
			case want && !have:
				toSub = append(toSub, redisChannel(topic))
			case !want && have:
				toUnsub = append(toUnsub, redisChannel(topic))
			}
		}
		h.mu.RUnlock()

		if len(toSub) > 0 {
			if err := h.ps.Subscribe(ctx, toSub...); err != nil {
				h.log.Error("redis subscribe failed, will retry", "err", err)
				h.retryLater(toSub)
			} else {
				for _, ch := range toSub {
					h.redisSubs[strings.TrimPrefix(ch, channelPrefix)] = true
				}
			}
		}
		if len(toUnsub) > 0 {
			if err := h.ps.Unsubscribe(ctx, toUnsub...); err != nil {
				h.log.Error("redis unsubscribe failed, will retry", "err", err)
				h.retryLater(toUnsub)
			} else {
				for _, ch := range toUnsub {
					delete(h.redisSubs, strings.TrimPrefix(ch, channelPrefix))
				}
			}
		}
	}
}

func (h *Hub) retryLater(channels []string) {
	time.AfterFunc(time.Second, func() {
		for _, ch := range channels {
			h.markPending(strings.TrimPrefix(ch, channelPrefix))
		}
	})
}

// CloseAll is called on shutdown. Clients get close code 1012 ("service
// restart") and reconnect to another instance after a random back-off.
func (h *Hub) CloseAll(code int, reason string) {
	h.mu.RLock()
	all := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		all = append(all, c)
	}
	h.mu.RUnlock()
	for _, c := range all {
		c.Close(code, reason)
	}
}

// Stats is handy for a /metrics or /healthz endpoint.
func (h *Hub) Stats() (clients, topics int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients), len(h.topics)
}
