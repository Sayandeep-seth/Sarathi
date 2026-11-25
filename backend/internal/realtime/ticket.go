package realtime

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// Why tickets instead of sending the JWT on the socket?
//
// Browsers cannot set an Authorization header on `new WebSocket(url)`, so the
// token would have to go in the URL, where proxies and access logs record it.
// Instead the app calls POST /v1/realtime/ticket with its normal JWT, gets a
// random 32-byte ticket that lives 30 seconds in Redis, and opens
// wss://.../ws?ticket=XXXX. The gateway redeems it with GETDEL, so it works
// exactly once and any gateway instance can redeem it.

var ErrInvalidTicket = errors.New("invalid or expired ticket")

const ticketKeyPrefix = "wsticket:"

type TicketStore struct {
	rdb redis.UniversalClient
	ttl time.Duration
}

func NewTicketStore(rdb redis.UniversalClient, ttl time.Duration) *TicketStore {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &TicketStore{rdb: rdb, ttl: ttl}
}

// Issue stores the identity under a random ticket and returns the ticket.
func (s *TicketStore) Issue(ctx context.Context, id Identity) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	ticket := base64.RawURLEncoding.EncodeToString(buf)

	payload, err := json.Marshal(id)
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, ticketKeyPrefix+ticket, payload, s.ttl).Err(); err != nil {
		return "", err
	}
	return ticket, nil
}

// Redeem atomically reads and deletes the ticket (GETDEL needs Redis >= 6.2).
func (s *TicketStore) Redeem(ctx context.Context, ticket string) (Identity, error) {
	if len(ticket) < 40 || len(ticket) > 64 {
		return Identity{}, ErrInvalidTicket
	}
	raw, err := s.rdb.GetDel(ctx, ticketKeyPrefix+ticket).Bytes()
	if errors.Is(err, redis.Nil) {
		return Identity{}, ErrInvalidTicket
	}
	if err != nil {
		return Identity{}, err
	}
	var id Identity
	if err := json.Unmarshal(raw, &id); err != nil || id.UserID == "" {
		return Identity{}, ErrInvalidTicket
	}
	return id, nil
}

// TicketHandler is mounted in your EXISTING REST API, behind your existing
// auth middleware. identityFrom pulls the authenticated user out of the
// request (for example from the context your JWT middleware filled in).
//
//	mux.Handle("POST /v1/realtime/ticket",
//	    authMiddleware(realtime.TicketHandler(tickets, identityFromContext)))
func TicketHandler(store *TicketStore, identityFrom func(*http.Request) (Identity, bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := identityFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ticket, err := store.Issue(r.Context(), id)
		if err != nil {
			http.Error(w, "could not issue ticket", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ticket":    ticket,
			"expiresIn": int(store.ttl.Seconds()),
		})
	}
}
