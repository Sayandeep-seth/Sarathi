// Command api runs the SARATHI HTTP API. Subcommands: `api migrate` applies database
// migrations; `api healthcheck` probes the local liveness endpoint.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sarathi/sarathi/backend/internal/auth"
	"github.com/sarathi/sarathi/backend/internal/config"
	"github.com/sarathi/sarathi/backend/internal/core"
	"github.com/sarathi/sarathi/backend/internal/database"
	"github.com/sarathi/sarathi/backend/internal/httpapi"
	"github.com/sarathi/sarathi/backend/internal/live"
	"github.com/sarathi/sarathi/backend/internal/realtime"
	"github.com/sarathi/sarathi/backend/internal/simulator"
	"github.com/sarathi/sarathi/backend/internal/store"
	"github.com/sarathi/sarathi/backend/internal/telemetry"
	"github.com/sarathi/sarathi/backend/migrations"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

// healthcheck probes the local liveness endpoint. It exists because the distroless
// image has no shell or curl for a container HEALTHCHECK.
func healthcheck() error {
	addr := os.Getenv("SARATHI_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz returned %d", resp.StatusCode)
	}
	return nil
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "healthcheck" {
		return healthcheck()
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})).
		With(slog.String("service", "sarathi-api"), slog.String("env", cfg.Env), slog.String("version", cfg.Version))

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(args) > 0 && args[0] == "migrate" {
		mctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		applied, err := database.Migrate(mctx, db, migrations.FS, log)
		if err != nil {
			return err
		}
		log.Info("migrations_complete", slog.Int("applied", len(applied)))
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("unknown command %q (supported: migrate, healthcheck)", args[0])
	}

	if cfg.RedisAddr == "" {
		return fmt.Errorf("config: SARATHI_REDIS_ADDR is required to serve the API")
	}
	lv := live.New(cfg.RedisAddr, cfg.RedisPassword, cfg.JWTSecret)
	defer lv.Close()

	var tel *telemetry.Store
	var advisory []httpapi.Checker
	if cfg.MongoURL != "" {
		octx, cancel := context.WithTimeout(ctx, 15*time.Second)
		tel, err = telemetry.Open(octx, cfg.MongoURL, cfg.MongoDatabase, log)
		cancel()
		if err != nil {
			return fmt.Errorf("mongodb: %w", err)
		}
		advisory = append(advisory, telemetry.Checker{S: tel})
	} else {
		log.Warn("mongodb_not_configured", slog.String("effect", "trip timelines and position history are disabled"))
	}

	st := &store.Store{DB: db}
	co := &core.Core{Store: st, Live: lv, Tel: tel, Log: log, Secret: cfg.JWTSecret, PaymentProvider: cfg.PaymentProvider}
	hub := realtime.NewHub(log, lv.RDB, cfg.CORSAllowedOrigins)

	var sender auth.Sender = auth.LogSender{Log: log}
	if cfg.OTPSender == "webhook" {
		sender = auth.WebhookSender{URL: cfg.OTPWebhookURL, Token: cfg.OTPWebhookToken}
	}

	// Background workers stop when bg is cancelled, after the HTTP server has drained.
	bg, stopBG := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	spawn := func(fn func(context.Context)) {
		workers.Add(1)
		go func() { defer workers.Done(); fn(bg) }()
	}
	every := func(d time.Duration, fn func(context.Context)) {
		spawn(func(ctx context.Context) {
			t := time.NewTicker(d)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					fn(ctx)
				}
			}
		})
	}
	spawn(hub.Run)
	every(5*time.Second, func(context.Context) { httpapi.WSConnections.Set(float64(hub.Connections())) })
	every(time.Hour, func(ctx context.Context) {
		if n, err := st.PurgeExpiredRefreshTokens(ctx); err != nil {
			log.Warn("refresh_token_purge_failed", slog.String("error", err.Error()))
		} else if n > 0 {
			log.Info("refresh_tokens_purged", slog.Int64("count", n))
		}
	})
	if cfg.Simulator {
		log.Warn("simulator_enabled", slog.Int("cabs", cfg.SimulatorCabs), slog.Float64("rate", cfg.SimulatorRate))
		sim := &simulator.Simulator{Core: co, Log: log, Cabs: cfg.SimulatorCabs, Rate: cfg.SimulatorRate}
		spawn(sim.Run)
	}
	defer func() {
		stopBG()
		workers.Wait()
		fctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tel.Close(fctx)
	}()

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewHandler(httpapi.Options{
			Log: log, Version: cfg.Version, Env: cfg.Env,
			Checkers: []httpapi.Checker{database.Checker{DB: db}, live.Checker{L: lv}}, Advisory: advisory,
			MaxBodyBytes: cfg.MaxBodyBytes, RequestTimeout: cfg.RequestTimeout, CORSOrigins: cfg.CORSAllowedOrigins,
			API: &httpapi.API{Cfg: cfg, Log: log, Core: co, Store: st, Live: lv, Tel: tel,
				Tokens: auth.Tokens{Secret: cfg.JWTSecret, TTL: cfg.AccessTokenTTL}, Sender: sender, Hub: hub},
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      cfg.RequestTimeout + 5*time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api_listening", slog.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutdown_started")
	}
	sctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("shutdown_complete")
	return nil
}
