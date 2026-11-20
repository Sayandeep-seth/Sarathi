package cache

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/common/utils"
)

// RateLimiter is a sliding-window limiter backed by a sorted set (rate_limit.lua).
type RateLimiter struct {
	rdb *redis.Client
	log *slog.Logger
}

func NewRateLimiter(rdb *redis.Client, log *slog.Logger) *RateLimiter {
	return &RateLimiter{rdb: rdb, log: log}
}

// Result of a rate-limit check.
type Result struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

// Allow records one hit for (scope,id) and reports whether it is within limit per window.
func (r *RateLimiter) Allow(ctx context.Context, scope, id string, limit int, window time.Duration) (Result, error) {
	now := time.Now().UnixMilli()
	member := strconv.FormatInt(now, 10) + "-" + utils.RandomToken(4)
	res, err := RateLimitScript.Run(ctx, r.rdb, []string{KeyRateLimit(scope, id)},
		now, window.Milliseconds(), limit, member).Int64Slice()
	if err != nil {
		return Result{Allowed: true}, err
	}
	return Result{
		Allowed:    res[0] == 1,
		Remaining:  int(res[1]),
		RetryAfter: time.Duration(res[2]) * time.Millisecond,
	}, nil
}

// Middleware limits requests per key (user id, else client IP). Fails open if Redis is down.
func (r *RateLimiter) Middleware(scope string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			id := utils.UserID(req)
			if id == "" {
				id = clientIP(req)
			}
			res, err := r.Allow(req.Context(), scope, id, limit, window)
			if err != nil {
				r.log.Warn("rate limiter unavailable, failing open", "err", err)
				next.ServeHTTP(w, req)
				return
			}
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
			if !res.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(res.RetryAfter.Seconds())+1))
				http.Error(w, `{"error":{"code":"rate_limited","message":"too many requests"}}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
