package realtime

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Presence answers "is this user connected to ANY gateway right now?"
// (needed before offering a trip to a driver, or to decide between a socket
// push and an FCM/APNs push notification).
//
// One sorted set per user:  online:{userID}  member = connID, score = expiry (unix s)
// writePump refreshes the score on every ping (54s). If a gateway crashes its
// entries simply age out, so nobody stays "online" forever.
const presenceTTL = 90 * time.Second

type Presence struct {
	rdb redis.UniversalClient
}

func NewPresence(rdb redis.UniversalClient) *Presence { return &Presence{rdb: rdb} }

func presenceKey(userID string) string { return "online:" + userID }

func (p *Presence) Online(c *Client) { p.Touch(c) }

func (p *Presence) Touch(c *Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	key := presenceKey(c.ID.UserID)
	expiry := time.Now().Add(presenceTTL).Unix()
	pipe := p.rdb.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(expiry), Member: c.ConnID})
	pipe.Expire(ctx, key, presenceTTL)
	_, _ = pipe.Exec(ctx)
}

func (p *Presence) Offline(c *Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	p.rdb.ZRem(ctx, presenceKey(c.ID.UserID), c.ConnID)
}

// IsOnline can be called from any service (dispatch, notifications ...).
func IsOnline(ctx context.Context, rdb redis.UniversalClient, userID string) (bool, error) {
	now := strconv.FormatInt(time.Now().Unix(), 10)
	n, err := rdb.ZCount(ctx, presenceKey(userID), now, "+inf").Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
