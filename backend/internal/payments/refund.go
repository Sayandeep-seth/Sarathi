package payments

import (
	"context"
	"errors"
	"math"
	"time"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
)

// Refund is a (partial) refund of an intent.
type Refund struct {
	ID       string    `json:"id"`
	IntentID string    `json:"intent_id"`
	Amount   float64   `json:"amount"`
	Reason   string    `json:"reason"`
	At       time.Time `json:"at"`
}

// Refund returns money (cancellations, deposit release). A Redis lock per intent
// prevents two concurrent refunds from exceeding the paid amount.
func (s *Service) Refund(ctx context.Context, intentID string, amount float64, reason string) (*Refund, error) {
	var out *Refund
	err := s.locker.WithLock(ctx, "refund:"+intentID, 30*time.Second, func(ctx context.Context) error {
		in, err := s.Get(ctx, intentID)
		if err != nil {
			return err
		}
		if in.Status != StatusPaid && in.Status != StatusPartRef {
			return apperr.Conflict("payment is not refundable in status " + in.Status)
		}
		left := in.Amount - in.Refunded
		if amount <= 0 || amount > left+0.001 {
			return apperr.BadRequest("refund exceeds refundable amount")
		}
		refundID, err := s.gw.Refund(ctx, in.PaymentID, int64(math.Round(amount*100)))
		if err != nil {
			return apperr.Unavailable("gateway refund failed")
		}
		status := StatusPartRef
		if left-amount < 0.01 {
			status = StatusRefunded
		}
		pipe := s.rdb.TxPipeline()
		pipe.HIncrByFloat(ctx, cache.KeyPaymentIntent(in.ID), "refunded", amount)
		pipe.HSet(ctx, cache.KeyPaymentIntent(in.ID), "status", status)
		pipe.HSet(ctx, cache.KeyRefund(refundID), "intent_id", in.ID, "amount", amount, "reason", reason, "at", time.Now().Unix())
		pipe.Expire(ctx, cache.KeyRefund(refundID), cache.TTLPaymentIntent)
		if _, err := pipe.Exec(ctx); err != nil {
			return err
		}
		if _, err := s.db.Exec(ctx, `UPDATE payments SET refunded = refunded + $2, status = $3 WHERE id = $1`, in.ID, amount, status); err != nil {
			s.log.Error("persist refund", "err", err)
		}
		_, _ = s.db.Exec(ctx, `INSERT INTO refunds (id, payment_id, amount, reason) VALUES ($1,$2,$3,$4)`, refundID, in.ID, amount, reason)
		in.Status = status
		s.notify(ctx, *in)
		out = &Refund{ID: refundID, IntentID: in.ID, Amount: amount, Reason: reason, At: time.Now()}
		return nil
	})
	if errors.Is(err, cache.ErrLockNotAcquired) {
		return nil, apperr.Conflict("a refund for this payment is already in progress")
	}
	return out, err
}

// RefundByRef refunds the intent of a business reference (cancellation helpers).
func (s *Service) RefundByRef(ctx context.Context, module, refID string, amount float64, reason string) (*Refund, error) {
	in, err := s.ByRef(ctx, module, refID)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		amount = in.Amount - in.Refunded
	}
	return s.Refund(ctx, in.ID, amount, reason)
}
