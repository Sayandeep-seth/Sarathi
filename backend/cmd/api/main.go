// Command api runs the SARATHI HTTP API, or applies migrations with `api migrate`.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sarathi/sarathi/backend/internal/cache"
	"github.com/sarathi/sarathi/backend/internal/config"
	"github.com/sarathi/sarathi/backend/internal/database"
	"github.com/sarathi/sarathi/backend/internal/httpapi"
	"github.com/sarathi/sarathi/backend/migrations"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
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
		return fmt.Errorf("unknown command %q (supported: migrate)", args[0])
	}

	checkers := []httpapi.Checker{database.Checker{DB: db}}
	if cfg.RedisAddr != "" {
		checkers = append(checkers, cache.Checker{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
	}

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewHandler(httpapi.Options{
			Log: log, Version: cfg.Version, Env: cfg.Env, Checkers: checkers,
			MaxBodyBytes: cfg.MaxBodyBytes, RequestTimeout: cfg.RequestTimeout, CORSOrigins: cfg.CORSAllowedOrigins,
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
