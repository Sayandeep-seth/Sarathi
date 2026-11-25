package realtime

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// Publisher is the ONLY thing other services need to push data to sockets.
// It works from any process (REST API, workers, cron jobs): it just PUBLISHes
// a ready-to-send JSON frame on rt:<topic>. Every gateway with a subscriber
// forwards the bytes as-is.
//
//	pub.Publish(ctx, realtime.UserTopic(uid), realtime.TypePassScanned, scan)
//
// Pub/Sub is fire-and-forget: if nobody is listening the message is dropped.
// That is fine for live data (locations, crowd levels) because the next update
// replaces it. For STATE (trip status, booking status) always store the state
// in Postgres/Redis first, then publish; a reconnecting client re-reads the
// state over REST, so a missed message is never lost information.
type Publisher struct {
	rdb redis.UniversalClient
}

func NewPublisher(rdb redis.UniversalClient) *Publisher { return &Publisher{rdb: rdb} }

func (p *Publisher) Publish(ctx context.Context, topic, msgType string, data any) error {
	env, err := NewEnvelope(msgType, topic, data)
	if err != nil {
		return err
	}
	frame, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return p.rdb.Publish(ctx, redisChannel(topic), frame).Err()
}

// PublishPipe adds a publish to an existing pipeline, so a state change and
// its notification go to Redis in one round trip.
func (p *Publisher) PublishPipe(ctx context.Context, pipe redis.Pipeliner, topic, msgType string, data any) error {
	env, err := NewEnvelope(msgType, topic, data)
	if err != nil {
		return err
	}
	frame, err := json.Marshal(env)
	if err != nil {
		return err
	}
	pipe.Publish(ctx, redisChannel(topic), frame)
	return nil
}
