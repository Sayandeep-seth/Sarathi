package realtime

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// Server is the http.Handler mounted at GET /ws.
type Server struct {
	baseCtx  context.Context // lives as long as the process, NOT the request
	hub      *Hub
	tickets  *TicketStore
	router   *Router
	upgrader websocket.Upgrader
	log      *slog.Logger
}

// NewServer. allowedOrigins are full origins such as "https://app.sarathi.in".
// Native mobile apps send no Origin header and are allowed; ticket auth is
// what actually protects the endpoint, the origin check stops other websites
// from using a logged-in browser's tickets (cross-site WebSocket hijacking).
func NewServer(baseCtx context.Context, hub *Hub, tickets *TicketStore, router *Router, allowedOrigins []string, log *slog.Logger) *Server {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = true
		}
	}
	return &Server{
		baseCtx: baseCtx,
		hub:     hub,
		tickets: tickets,
		router:  router,
		log:     log.With("component", "ws-server"),
		upgrader: websocket.Upgrader{
			ReadBufferSize:    1024,
			WriteBufferSize:   1024,
			EnableCompression: false, // location frames are tiny; compression costs more CPU than it saves
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == "" || allowed[origin]
			},
		},
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate BEFORE upgrading, so a bad ticket gets a normal 401.
	id, err := s.tickets.Redeem(r.Context(), r.URL.Query().Get("ticket"))
	if err != nil {
		if errors.Is(err, ErrInvalidTicket) {
			http.Error(w, "invalid ticket", http.StatusUnauthorized)
		} else {
			s.log.Error("ticket redeem failed", "err", err)
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		}
		return
	}

	// 2. Upgrade HTTP -> WebSocket. On failure the upgrader already replied.
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warn("upgrade failed", "err", err)
		return
	}

	// 3. Register and start the two pumps. ServeHTTP returns immediately;
	//    the goroutines own the connection from here on.
	c := newClient(s.hub, s.router, conn, id, s.log)
	s.hub.register(c)
	go c.writePump()
	go c.readPump(s.baseCtx)

	c.SendEnvelope(mustEnvelope(TypeWelcome, "", map[string]any{
		"connId":  c.ConnID,
		"userId":  id.UserID,
		"role":    id.Role,
		"gateway": s.hub.gatewayID,
	}))
}

func mustEnvelope(typ, topic string, data any) Envelope {
	env, err := NewEnvelope(typ, topic, data)
	if err != nil {
		panic(err)
	}
	return env
}
