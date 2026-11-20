package payments

import (
	"context"
	"encoding/json"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
)

// WebhookEvent is the gateway callback (Razorpay-like shape).
type WebhookEvent struct {
	ID      string `json:"id"`
	Event   string `json:"event"` // payment.captured | payment.failed | refund.processed
	Payload struct {
		OrderID   string `json:"order_id"`
		PaymentID string `json:"payment_id"`
		RefundID  string `json:"refund_id"`
	} `json:"payload"`
}

// HandleWebhook verifies, de-duplicates (SET NX sr:pay:webhook:<eventID>) and applies an event.
// Gateways retry webhooks, so the same event may arrive several times.
func (s *Service) HandleWebhook(ctx context.Context, body []byte, signature string) error {
	if !s.gw.VerifyWebhook(body, signature) {
		return apperr.Forbidden("bad webhook signature")
	}
	var ev WebhookEvent
	if err := json.Unmarshal(body, &ev); err != nil || ev.ID == "" {
		return apperr.BadRequest("malformed webhook")
	}
	first, err := s.rdb.SetNX(ctx, cache.KeyWebhookEvent(ev.ID), ev.Event, cache.TTLWebhookEvent).Result()
	if err != nil {
		return err
	}
	if !first {
		return nil // duplicate delivery
	}
	intentID, err := s.rdb.Get(ctx, cache.KeyPaymentRef("order", ev.Payload.OrderID)).Result()
	if err != nil {
		// unknown order: allow a retry later
		s.rdb.Del(ctx, cache.KeyWebhookEvent(ev.ID))
		return apperr.NotFound("unknown order")
	}
	in, err := s.Get(ctx, intentID)
	if err != nil {
		s.rdb.Del(ctx, cache.KeyWebhookEvent(ev.ID))
		return err
	}
	switch ev.Event {
	case "payment.captured":
		_, err = s.markPaid(ctx, in, ev.Payload.PaymentID)
	case "payment.failed":
		err = s.markFailed(ctx, in)
	case "refund.processed":
		s.log.Info("refund processed", "intent", in.ID, "refund", ev.Payload.RefundID)
	}
	if err != nil {
		s.rdb.Del(ctx, cache.KeyWebhookEvent(ev.ID)) // let the gateway retry
	}
	return err
}
