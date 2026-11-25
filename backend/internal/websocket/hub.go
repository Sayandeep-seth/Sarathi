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
	Clients map[*Client]bool
    Rooms   map[string]*Room

    Register   chan *Client
    Unregister chan *Client
    Broadcast  chan []byte
}

func NewHub(rdb *redis.Client, instanceID string, log *slog.Logger) *Hub {
	return &Hub{
		rdb: rdb, instanceID: instanceID, log: log,
		upgrader: ws.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024, CheckOrigin: func(*http.Request) bool { return true }},
		topics:   map[string]map[*Client]struct{}{},
		subs:     map[*Client]map[string]struct{}{},
		Clients:    make(map[*Client]bool),
        Register:   make(chan *Client),
        Unregister: make(chan *Client),
        Broadcast:  make(chan []byte)
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

package websocket

import (
    "sync"
)

type Hub struct {
    Clients map[*Client]bool
    Rooms  map[string]map[*Client]bool

    Register   chan *Client
    Unregister chan *Client

    mu sync.RWMutex
}

func NewHub() *Hub {
    return Hub{
        Clients:    make(map[*Client]bool),
        Rooms:      make(map[string]map[*Client]bool),
        Register:   make(chan *Client),
        Unregister: make(chan *Client),
    }
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.Register:
            h.mu.Lock()
            h.Clients[client] = true
            h.mu.Unlock()

        case client := <-h.Unregister:
            h.mu.Lock()

            if _, ok := h.Clients[client]; ok {
                delete(h.Clients, client)

                for roomID, members := range h.Rooms {
                    delete(members, client)

                    if len(members) == 0 {
                        delete(h.Rooms, roomID)
                    }
                }

                close(client.Send)
            }

            h.mu.Unlock()
        }
    }
}

func (h *Hub) JoinRoom(roomID string, client *Client) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if h.Rooms[roomID] == nil {
        h.Rooms[roomID] = make(map[*Client]bool)
    }

    h.Rooms[roomID][client] = true
}

func (h *Hub) LeaveRoom(roomID string, client *Client) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if members, ok := h.Rooms[roomID]; ok {
        delete(members, client)

        if len(members) == 0 {
            delete(h.Rooms, roomID)
        }
    }
}

func (h *Hub) SendToRoom(roomID string, message []byte) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    for client := range h.Rooms[roomID] {
        select {
        case client.Send <- message:
        default:
            // Client's outgoing queue is full.
            // Its connection will be closed by the client lifecycle.
        }
    }
}

func (h *Hub) SendToUser(userID string, message []byte) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    for client := range h.Clients {
        if client.UserID == userID {
            select {
            case client.Send <- message:
            default:
            }
        }
    }
}