// Package cache is the Redis core of Sarathi: connection, key catalogue, TTLs,
// generic caching, GEO, locks/slot holds, rate limiting and Lua scripts.
package cache

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed scripts/*.lua
var scriptFS embed.FS

// Lua scripts (loaded once with SCRIPT LOAD, executed with EVALSHA).
var (
	ReserveSeatScript = mustScript("scripts/reserve_seat.lua")
	ReleaseLockScript = mustScript("scripts/release_lock.lua")
	RateLimitScript   = mustScript("scripts/rate_limit.lua")
)

func mustScript(path string) *redis.Script {
	b, err := scriptFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("cache: missing lua script %s: %v", path, err))
	}
	return redis.NewScript(string(b))
}

// Options mirrors config.RedisConfig without importing the app layer.
type Options struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// NewClient connects, pings and preloads all Lua scripts.
func NewClient(ctx context.Context, o Options) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         o.Addr,
		Password:     o.Password,
		DB:           o.DB,
		PoolSize:     o.PoolSize,
		MinIdleConns: o.MinIdleConns,
		DialTimeout:  o.DialTimeout,
		ReadTimeout:  o.ReadTimeout,
		WriteTimeout: o.WriteTimeout,
	})
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("redis ping %s: %w", o.Addr, err)
	}
	for _, s := range []*redis.Script{ReserveSeatScript, ReleaseLockScript, RateLimitScript} {
		if err := s.Load(pctx, rdb).Err(); err != nil {
			_ = rdb.Close()
			return nil, fmt.Errorf("redis script load: %w", err)
		}
	}
	return rdb, nil
}

// IsNil reports whether err is a Redis "key does not exist" reply.
func IsNil(err error) bool { return errors.Is(err, redis.Nil) }
