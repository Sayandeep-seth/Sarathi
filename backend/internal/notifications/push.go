package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
)

// PushSender sends to every registered device of a user.
//
//	sr:user:<id>:devices  SET of FCM/APNs tokens
type PushSender struct {
	rdb       *redis.Client
	endpoint  string // FCM HTTP endpoint
	serverKey string
	http      *http.Client
}

func NewPushSender(rdb *redis.Client, endpoint, serverKey string) *PushSender {
	return &PushSender{rdb: rdb, endpoint: endpoint, serverKey: serverKey, http: &http.Client{Timeout: 5 * time.Second}}
}

// RegisterDevice stores a push token for a user (called at app login).
func (p *PushSender) RegisterDevice(ctx context.Context, userID, token string) error {
	return p.rdb.SAdd(ctx, cache.KeyUserDevices(userID), token).Err()
}

func (p *PushSender) Send(ctx context.Context, n Notification) error {
	tokens, err := p.rdb.SMembers(ctx, cache.KeyUserDevices(n.UserID)).Result()
	if err != nil || len(tokens) == 0 {
		return err // no device: nothing to do
	}
	if p.endpoint == "" {
		return nil // push disabled in this environment
	}
	var lastErr error
	for _, t := range tokens {
		body, _ := json.Marshal(map[string]any{
			"to": t, "notification": map[string]string{"title": n.Title, "body": n.Body}, "data": n.Data,
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
		req.Header.Set("Authorization", "key="+p.serverKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := p.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
			p.rdb.SRem(ctx, cache.KeyUserDevices(n.UserID), t) // token no longer valid
		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("push provider %d", resp.StatusCode)
		}
	}
	return lastErr
}
