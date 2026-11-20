package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Service provides generic cache-aside operations on top of Redis.
type Service struct {
	rdb *redis.Client
}

func NewService(rdb *redis.Client) *Service { return &Service{rdb: rdb} }

// Client exposes the raw client for module-specific commands.
func (s *Service) Client() *redis.Client { return s.rdb }

// GetJSON loads key into dst. found=false when the key is missing.
func (s *Service) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	b, err := s.rdb.Get(ctx, key).Bytes()
	if IsNil(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, dst); err != nil {
		// corrupted entry: drop it so the next read repopulates
		_ = s.rdb.Del(ctx, key).Err()
		return false, nil
	}
	return true, nil
}

// SetJSON stores v as JSON with a TTL (0 = no expiry).
func (s *Service) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("cache marshal %s: %w", key, err)
	}
	return s.rdb.Set(ctx, key, b, ttl).Err()
}

// Del removes keys.
func (s *Service) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return s.rdb.Del(ctx, keys...).Err()
}

// Remember is the cache-aside pattern: return the cached value or load, store and return it.
func Remember[T any](ctx context.Context, s *Service, key string, ttl time.Duration, load func(context.Context) (T, error)) (T, error) {
	var v T
	if found, err := s.GetJSON(ctx, key, &v); err == nil && found {
		return v, nil
	}
	v, err := load(ctx)
	if err != nil {
		return v, err
	}
	_ = s.SetJSON(ctx, key, v, ttl) // a cache write failure must not fail the request
	return v, nil
}

// Idempotent claims an idempotency key. If it was already claimed it returns the
// stored value and fresh=false, so the caller can return the original result.
func (s *Service) Idempotent(ctx context.Context, scope, idemKey, value string) (stored string, fresh bool, err error) {
	if idemKey == "" {
		return value, true, nil
	}
	k := KeyIdempotency(scope, idemKey)
	ok, err := s.rdb.SetNX(ctx, k, value, TTLIdempotency).Result()
	if err != nil {
		return "", false, err
	}
	if ok {
		return value, true, nil
	}
	stored, err = s.rdb.Get(ctx, k).Result()
	return stored, false, err
}

// Publish sends v as JSON on a pub/sub channel (consumed by the websocket hub).
func (s *Service) Publish(ctx context.Context, channel string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.rdb.Publish(ctx, channel, b).Err()
}

// IncrWithTTL increments a counter and sets its TTL on first creation.
func (s *Service) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := s.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// HSetWithTTL writes a hash and refreshes its TTL atomically.
func (s *Service) HSetWithTTL(ctx context.Context, key string, ttl time.Duration, values map[string]any) error {
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, values)
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// HGetFloat reads a float field (0 when missing).
func HGetFloat(m map[string]string, field string) float64 {
	f, _ := strconv.ParseFloat(m[field], 64)
	return f
}

// HGetInt reads an int field (0 when missing).
func HGetInt(m map[string]string, field string) int64 {
	i, _ := strconv.ParseInt(m[field], 10, 64)
	return i
}
