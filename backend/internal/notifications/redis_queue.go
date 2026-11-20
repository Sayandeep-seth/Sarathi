// Package notifications queues push/SMS/email messages in a Redis Stream and
// delivers them from the worker with retries and a dead-letter stream.
package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

const (
	ChannelPush  = "push"
	ChannelSMS   = "sms"
	ChannelEmail = "email"

	group       = "notifiers"
	maxAttempts = 5
)

// Notification is one message to one user on one channel.
type Notification struct {
	ID      string            `json:"id"`
	UserID  string            `json:"user_id"`
	Channel string            `json:"channel"`
	Title   string            `json:"title"`
	Body    string            `json:"body"`
	Data    map[string]string `json:"data,omitempty"`
	Attempt int               `json:"attempt"`
}

// Sender delivers one channel.
type Sender interface {
	Send(ctx context.Context, n Notification) error
}

// Queue is the producer (API) and consumer (worker) of sr:stream:notifications.
type Queue struct {
	rdb     *redis.Client
	log     *slog.Logger
	senders map[string]Sender
}

func NewQueue(rdb *redis.Client, log *slog.Logger) *Queue {
	return &Queue{rdb: rdb, log: log, senders: map[string]Sender{}}
}

func (q *Queue) RegisterSender(channel string, s Sender) { q.senders[channel] = s }

// Enqueue adds a notification (never blocks the request on delivery).
func (q *Queue) Enqueue(ctx context.Context, n Notification) error {
	if n.ID == "" {
		n.ID = utils.NewID("ntf")
	}
	b, _ := json.Marshal(n)
	return q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: cache.StreamNotifications, MaxLen: 200_000, Approx: true, Values: map[string]any{"n": b},
	}).Err()
}

// Notify fans a message out to several channels (default: push).
func (q *Queue) Notify(ctx context.Context, userID, title, body string, data map[string]string, channels ...string) {
	if len(channels) == 0 {
		channels = []string{ChannelPush}
	}
	for _, ch := range channels {
		if err := q.Enqueue(ctx, Notification{UserID: userID, Channel: ch, Title: title, Body: body, Data: data}); err != nil {
			q.log.Warn("enqueue notification", "err", err)
		}
	}
}

// Run consumes the stream (worker). Failures are re-queued with attempt+1;
// after maxAttempts they go to the dead-letter stream. Messages left pending by a
// crashed consumer are reclaimed with XAUTOCLAIM.
func (q *Queue) Run(ctx context.Context, consumer string) error {
	if err := q.rdb.XGroupCreateMkStream(ctx, cache.StreamNotifications, group, "0").Err(); err != nil &&
		!strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	lastClaim := time.Now()
	for ctx.Err() == nil {
		if time.Since(lastClaim) > 30*time.Second {
			q.reclaim(ctx, consumer)
			lastClaim = time.Now()
		}
		streams, err := q.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: group, Consumer: consumer, Streams: []string{cache.StreamNotifications, ">"},
			Count: 100, Block: 2 * time.Second,
		}).Result()
		if errors.Is(err, redis.Nil) || ctx.Err() != nil {
			continue
		}
		if err != nil {
			q.log.Error("notification read", "err", err)
			time.Sleep(time.Second)
			continue
		}
		for _, st := range streams {
			for _, m := range st.Messages {
				q.handle(ctx, m)
			}
		}
	}
	return ctx.Err()
}

func (q *Queue) reclaim(ctx context.Context, consumer string) {
	msgs, _, err := q.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream: cache.StreamNotifications, Group: group, Consumer: consumer,
		MinIdle: time.Minute, Start: "0-0", Count: 100,
	}).Result()
	if err != nil {
		return
	}
	for _, m := range msgs {
		q.handle(ctx, m)
	}
}

func (q *Queue) handle(ctx context.Context, m redis.XMessage) {
	defer q.rdb.XAck(ctx, cache.StreamNotifications, group, m.ID)
	raw, _ := m.Values["n"].(string)
	var n Notification
	if json.Unmarshal([]byte(raw), &n) != nil {
		return
	}
	sender, ok := q.senders[n.Channel]
	if !ok {
		q.log.Warn("no sender for channel", "channel", n.Channel)
		return
	}
	sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err := sender.Send(sctx, n)
	cancel()
	if err == nil {
		return
	}
	n.Attempt++
	if n.Attempt >= maxAttempts {
		b, _ := json.Marshal(n)
		q.rdb.XAdd(ctx, &redis.XAddArgs{Stream: cache.StreamNotificationsDLQ, MaxLen: 50_000, Approx: true,
			Values: map[string]any{"n": b, "err": err.Error()}})
		q.log.Error("notification dead-lettered", "id", n.ID, "err", err)
		return
	}
	time.Sleep(time.Duration(n.Attempt) * 200 * time.Millisecond) // small backoff
	_ = q.Enqueue(ctx, n)
}
