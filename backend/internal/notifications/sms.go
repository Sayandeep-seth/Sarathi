package notifications

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
)

// SMSSender sends SMS (OTP, QR pass links, bus boarding alerts).
// Phone comes from sr:user:<id>:contact; a per-number rate limit (5/hour)
// protects against SMS-pumping fraud.
type SMSSender struct {
	rdb      *redis.Client
	limiter  *cache.RateLimiter
	endpoint string
	apiKey   string
	sender   string
	http     *http.Client
}

func NewSMSSender(rdb *redis.Client, l *cache.RateLimiter, endpoint, apiKey, sender string) *SMSSender {
	return &SMSSender{rdb: rdb, limiter: l, endpoint: endpoint, apiKey: apiKey, sender: sender, http: &http.Client{Timeout: 5 * time.Second}}
}

func (s *SMSSender) Send(ctx context.Context, n Notification) error {
	phone, err := s.rdb.HGet(ctx, cache.KeyUserContact(n.UserID), "phone").Result()
	if cache.IsNil(err) || phone == "" {
		return nil
	}
	if err != nil {
		return err
	}
	if res, _ := s.limiter.Allow(ctx, "sms", phone, 5, time.Hour); !res.Allowed {
		return nil // silently drop: limit protects cost, not delivery
	}
	if s.endpoint == "" {
		return nil
	}
	form := url.Values{"apikey": {s.apiKey}, "sender": {s.sender}, "numbers": {phone}, "message": {n.Body}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("sms provider %d", resp.StatusCode)
	}
	return nil
}
