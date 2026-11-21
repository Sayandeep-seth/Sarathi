// Package bootstrap builds the application: config, Postgres, Redis, services, router.
package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sarathi/app/config"
	"sarathi/internal/common/logger"
	"sarathi/internal/common/response"
)

// App holds process-wide dependencies.
type App struct {
	Cfg      *config.Config
	DB       *pgxpool.Pool
	RDB      *redis.Client
	Log      *slog.Logger
	Services *Services
}

// New connects to Postgres + Redis and wires all services.
func New(ctx context.Context) (*App, error) {
	cfg := config.Load()
	log := logger.New(cfg.Env)
	db, err := NewPostgres(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}
	rdb, err := NewRedis(ctx, cfg.Redis)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &App{Cfg: cfg, DB: db, RDB: rdb, Log: log, Services: BuildServices(cfg, db, rdb, log)}, nil
}

// Router builds the HTTP handler with middleware.
func (a *App) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		status := map[string]string{"redis": "ok", "postgres": "ok"}
		if err := a.RDB.Ping(ctx).Err(); err != nil {
			status["redis"] = err.Error()
		}
		if err := a.DB.Ping(ctx); err != nil {
			status["postgres"] = err.Error()
		}
		response.OK(w, status)
	})
	a.Services.Register(mux)

	limit := a.Services.RateLimiter.Middleware("api", a.Cfg.Security.RateLimitPerMinute, time.Minute)
	return a.recoverer(a.accessLog(limit(mux)))
}

// WarmCaches loads reference data into Redis (safe to run on every boot).
func (a *App) WarmCaches(ctx context.Context) {
	steps := map[string]func(context.Context) error{
		"vehicle model index": a.Services.Vehicles.WarmIndex,
		"petrol stations":     a.Services.Petrol.WarmUp,
		"ev chargers":         a.Services.EV.WarmUp,
		"premium fleet":       a.Services.PremCache.WarmUp,
	}
	for name, fn := range steps {
		if err := fn(ctx); err != nil {
			a.Log.Warn("cache warm-up failed", "step", name, "err", err)
		}
	}
}

func (a *App) Close() {
	_ = a.RDB.Close()
	a.DB.Close()
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (a *App) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws" {
			next.ServeHTTP(w, r) // keep the raw writer for the upgrade
			return
		}
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		a.Log.Debug("http", "method", r.Method, "path", r.URL.Path, "status", sw.status, "dur", time.Since(start))
	})
}

func (a *App) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				a.Log.Error("panic", "err", rec, "stack", string(debug.Stack()))
				http.Error(w, `{"error":{"code":"internal","message":"internal error"}}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
