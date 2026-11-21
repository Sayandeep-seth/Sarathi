package bootstrap

import (
	"context"

	"github.com/redis/go-redis/v9"

	"sarathi/app/config"
	"sarathi/internal/cache"
)

// NewRedis connects to Redis and preloads the Lua scripts.
func NewRedis(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	return cache.NewClient(ctx, cache.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})
}
