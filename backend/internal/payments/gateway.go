// Package payments handles payment intents, gateway confirmation, webhooks and
// refunds. Redis provides idempotency, webhook de-duplication, refund locks and
// a hot copy of every intent.
package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"sarathi/internal/common/utils"
)

// Order is a gateway order the app pays against.
type Order struct {
	ID          string `json:"id"`
	AmountPaise int64  `json:"amount_paise"`
	Currency    string `json:"currency"`
	Receipt     string `json:"receipt"`
}

// Gateway abstracts Razorpay/Cashfree/PayU style gateways.
type Gateway interface {
	CreateOrder(ctx context.Context, amountPaise int64, receipt string) (Order, error)
	Refund(ctx context.Context, paymentID string, amountPaise int64) (refundID string, err error)
	// VerifyPayment checks the signature the app receives after checkout.
	VerifyPayment(orderID, paymentID, signature string) bool
	// VerifyWebhook checks the signature header of a webhook body.
	VerifyWebhook(body []byte, signature string) bool
}

// HMACGateway implements the Razorpay signature scheme with a local order id
// generator. Use it in development; the production adapter only replaces
// CreateOrder/Refund with real API calls — the signature checks are identical.
type HMACGateway struct {
	keySecret     string
	webhookSecret string
}

func NewHMACGateway(keySecret, webhookSecret string) *HMACGateway {
	return &HMACGateway{keySecret: keySecret, webhookSecret: webhookSecret}
}

func (g *HMACGateway) CreateOrder(_ context.Context, amountPaise int64, receipt string) (Order, error) {
	return Order{ID: utils.NewID("order"), AmountPaise: amountPaise, Currency: "INR", Receipt: receipt}, nil
}

func (g *HMACGateway) Refund(_ context.Context, _ string, _ int64) (string, error) {
	return utils.NewID("rfnd"), nil
}

func (g *HMACGateway) VerifyPayment(orderID, paymentID, signature string) bool {
	return hmac.Equal([]byte(Sign(g.keySecret, orderID+"|"+paymentID)), []byte(signature))
}

func (g *HMACGateway) VerifyWebhook(body []byte, signature string) bool {
	return hmac.Equal([]byte(Sign(g.webhookSecret, string(body))), []byte(signature))
}

// Sign returns hex(HMAC-SHA256(secret, msg)).
func Sign(secret, msg string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}
