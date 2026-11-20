package luxury_bus

import (
	"context"
	"crypto/rand"
	"strconv"
	"strings"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
)

// Ticket is a seat booking (PNR) on a luxury bus trip.
type Ticket struct {
	PNR       string  `json:"pnr"`
	TripID    string  `json:"trip_id"`
	UserID    string  `json:"user_id"`
	Seats     []int   `json:"seats"`
	Fare      float64 `json:"fare"`
	FoodTotal float64 `json:"food_total"`
	Status    string  `json:"status"` // pending_payment | confirmed | cancelled
	PaymentID string  `json:"payment_intent_id,omitempty"`
}

// NewPNR returns a 10-character PNR without ambiguous characters.
func NewPNR() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

func (s *Store) SaveTicket(ctx context.Context, t *Ticket) error {
	seats := make([]string, len(t.Seats))
	for i, n := range t.Seats {
		seats[i] = strconv.Itoa(n)
	}
	k := cache.KeyLuxTicket(t.PNR)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, k, map[string]any{"trip": t.TripID, "user": t.UserID, "seats": strings.Join(seats, ","),
		"fare": t.Fare, "food": t.FoodTotal, "status": t.Status, "payment": t.PaymentID})
	pipe.Expire(ctx, k, cache.TTLLuxTrip)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Ticket(ctx context.Context, pnr string) (*Ticket, error) {
	m, err := s.rdb.HGetAll(ctx, cache.KeyLuxTicket(pnr)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, apperr.NotFound("PNR not found")
	}
	t := &Ticket{PNR: pnr, TripID: m["trip"], UserID: m["user"], Fare: cache.HGetFloat(m, "fare"),
		FoodTotal: cache.HGetFloat(m, "food"), Status: m["status"], PaymentID: m["payment"]}
	for _, p := range strings.Split(m["seats"], ",") {
		if n, err := strconv.Atoi(p); err == nil {
			t.Seats = append(t.Seats, n)
		}
	}
	return t, nil
}

func (s *Store) SetTicketStatus(ctx context.Context, pnr, status string) error {
	return s.rdb.HSet(ctx, cache.KeyLuxTicket(pnr), "status", status).Err()
}
