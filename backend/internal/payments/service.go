package payments

import (
	"context"
	"log/slog"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// Intent statuses.
const (
	StatusCreated  = "created"
	StatusPaid     = "paid"
	StatusFailed   = "failed"
	StatusRefunded = "refunded"
	StatusPartRef  = "partially_refunded"
)

// Intent is one payment for one business reference (booking, journey, ticket…).
type Intent struct {
	ID        string    `json:"id"`
	Module    string    `json:"module"` // booking, transit, daily_bus, luxury_bus, self_drive…
	RefID     string    `json:"ref_id"`
	UserID    string    `json:"user_id"`
	OrderID   string    `json:"order_id"`
	PaymentID string    `json:"payment_id,omitempty"`
	Amount    float64   `json:"amount"`
	Refunded  float64   `json:"refunded"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// PaidListener lets modules react when their payment succeeds (confirm seats, issue QR…).
type PaidListener func(ctx context.Context, in Intent) error

// Service manages intents.
//
//	sr:pay:intent:<id>          HASH   intent (7 days)
//	sr:pay:ref:<module>:<ref>   STRING intent id  (one intent per business ref = idempotency)
//	sr:pay:ref:order:<orderID>  STRING intent id  (webhook lookup)
//	sr:ch:payments              PUBSUB status changes
type Service struct {
	rdb *redis.Client
	db  *pgxpool.Pool
	gw  Gateway
	log *slog.Logger

	locker    *cache.Locker
	mu        sync.RWMutex
	listeners map[string][]PaidListener
}

func NewService(rdb *redis.Client, db *pgxpool.Pool, gw Gateway, l *cache.Locker, log *slog.Logger) *Service {
	return &Service{rdb: rdb, db: db, gw: gw, locker: l, log: log, listeners: map[string][]PaidListener{}}
}

// OnPaid registers a listener for a module.
func (s *Service) OnPaid(module string, fn PaidListener) {
	s.mu.Lock()
	s.listeners[module] = append(s.listeners[module], fn)
	s.mu.Unlock()
}

// CreateIntent is idempotent per (module, ref): calling it twice returns the same intent.
func (s *Service) CreateIntent(ctx context.Context, module, refID, userID string, amount float64) (*Intent, error) {
	if amount <= 0 {
		return nil, apperr.BadRequest("amount must be positive")
	}
	refKey := cache.KeyPaymentRef(module, refID)
	id := utils.NewID("pi")
	ok, err := s.rdb.SetNX(ctx, refKey, id, cache.TTLPaymentIntent).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		existing, err := s.rdb.Get(ctx, refKey).Result()
		if err != nil {
			return nil, err
		}
		return s.Get(ctx, existing)
	}
	order, err := s.gw.CreateOrder(ctx, int64(math.Round(amount*100)), id)
	if err != nil {
		s.rdb.Del(ctx, refKey)
		return nil, apperr.Unavailable("payment gateway unavailable")
	}
	in := &Intent{ID: id, Module: module, RefID: refID, UserID: userID, OrderID: order.ID, Amount: amount, Status: StatusCreated, CreatedAt: time.Now()}
	if err := s.save(ctx, in); err != nil {
		return nil, err
	}
	s.rdb.Set(ctx, cache.KeyPaymentRef("order", order.ID), id, cache.TTLPaymentIntent)
	_, err = s.db.Exec(ctx, `INSERT INTO payments (id, module, ref_id, user_id, order_id, amount, status) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		in.ID, in.Module, in.RefID, in.UserID, in.OrderID, in.Amount, in.Status)
	return in, err
}

func (s *Service) save(ctx context.Context, in *Intent) error {
	k := cache.KeyPaymentIntent(in.ID)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, k, map[string]any{
		"module": in.Module, "ref_id": in.RefID, "user_id": in.UserID, "order_id": in.OrderID,
		"payment_id": in.PaymentID, "amount": in.Amount, "refunded": in.Refunded, "status": in.Status,
		"created_at": in.CreatedAt.Unix(),
	})
	pipe.Expire(ctx, k, cache.TTLPaymentIntent)
	_, err := pipe.Exec(ctx)
	return err
}

// Get reads an intent from Redis, falling back to Postgres.
func (s *Service) Get(ctx context.Context, id string) (*Intent, error) {
	m, err := s.rdb.HGetAll(ctx, cache.KeyPaymentIntent(id)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) > 0 {
		return &Intent{
			ID: id, Module: m["module"], RefID: m["ref_id"], UserID: m["user_id"], OrderID: m["order_id"],
			PaymentID: m["payment_id"], Amount: cache.HGetFloat(m, "amount"), Refunded: cache.HGetFloat(m, "refunded"),
			Status: m["status"], CreatedAt: time.Unix(cache.HGetInt(m, "created_at"), 0),
		}, nil
	}
	in := &Intent{ID: id}
	err = s.db.QueryRow(ctx, `SELECT module, ref_id, user_id, order_id, coalesce(payment_id,''), amount, refunded, status, created_at FROM payments WHERE id=$1`, id).
		Scan(&in.Module, &in.RefID, &in.UserID, &in.OrderID, &in.PaymentID, &in.Amount, &in.Refunded, &in.Status, &in.CreatedAt)
	if err != nil {
		return nil, apperr.NotFound("payment not found")
	}
	_ = s.save(ctx, in)
	return in, nil
}

// ByRef returns the intent of a business reference.
func (s *Service) ByRef(ctx context.Context, module, refID string) (*Intent, error) {
	id, err := s.rdb.Get(ctx, cache.KeyPaymentRef(module, refID)).Result()
	if cache.IsNil(err) {
		return nil, apperr.NotFound("no payment for " + module + "/" + refID)
	}
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// IsPaid is used by modules before activating a service.
func (s *Service) IsPaid(ctx context.Context, module, refID string) (bool, error) {
	in, err := s.ByRef(ctx, module, refID)
	if apperr.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return in.Status == StatusPaid, nil
}

// markPaidScript moves created -> paid exactly once (webhook and app confirm may race).
var markPaidScript = redis.NewScript(`
local st = redis.call('HGET', KEYS[1], 'status')
if st ~= 'created' and st ~= 'failed' then return 0 end
redis.call('HSET', KEYS[1], 'status', 'paid', 'payment_id', ARGV[1])
return 1`)

// Confirm verifies the checkout signature returned to the app and marks the intent paid.
func (s *Service) Confirm(ctx context.Context, intentID, paymentID, signature string) (*Intent, error) {
	in, err := s.Get(ctx, intentID)
	if err != nil {
		return nil, err
	}
	if !s.gw.VerifyPayment(in.OrderID, paymentID, signature) {
		return nil, apperr.BadRequest("invalid payment signature")
	}
	return s.markPaid(ctx, in, paymentID)
}

func (s *Service) markPaid(ctx context.Context, in *Intent, paymentID string) (*Intent, error) {
	changed, err := markPaidScript.Run(ctx, s.rdb, []string{cache.KeyPaymentIntent(in.ID)}, paymentID).Int64()
	if err != nil {
		return nil, err
	}
	in.Status, in.PaymentID = StatusPaid, paymentID
	if changed == 0 {
		return in, nil // already processed
	}
	if _, err := s.db.Exec(ctx, `UPDATE payments SET status='paid', payment_id=$2, paid_at=now() WHERE id=$1`, in.ID, paymentID); err != nil {
		s.log.Error("persist payment", "id", in.ID, "err", err)
	}
	s.notify(ctx, *in)
	s.mu.RLock()
	ls := s.listeners[in.Module]
	s.mu.RUnlock()
	for _, fn := range ls {
		if err := fn(ctx, *in); err != nil {
			s.log.Error("payment listener failed", "module", in.Module, "ref", in.RefID, "err", err)
		}
	}
	return in, nil
}

func (s *Service) markFailed(ctx context.Context, in *Intent) error {
	s.rdb.HSet(ctx, cache.KeyPaymentIntent(in.ID), "status", StatusFailed)
	in.Status = StatusFailed
	_, err := s.db.Exec(ctx, `UPDATE payments SET status='failed' WHERE id=$1 AND status='created'`, in.ID)
	s.notify(ctx, *in)
	return err
}

func (s *Service) notify(ctx context.Context, in Intent) {
	msg := `{"type":"payment","intent_id":"` + in.ID + `","module":"` + in.Module + `","ref_id":"` + in.RefID + `","status":"` + in.Status + `","amount":` + strconv.FormatFloat(in.Amount, 'f', 2, 64) + `}`
	pipe := s.rdb.Pipeline()
	pipe.Publish(ctx, cache.ChanPayments(), msg)
	pipe.Publish(ctx, cache.ChanUser(in.UserID), msg)
	_, _ = pipe.Exec(ctx)
}
