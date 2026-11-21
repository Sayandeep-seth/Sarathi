// Command server runs the SARATHI HTTP API and WebSocket hub.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"sarathi/app/bootstrap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Connects Postgres + Redis and builds all services (including the WebSocket hub).
	app, err := bootstrap.New(ctx)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	defer app.Close()

	app.WarmCaches(ctx)

	// Hub.Run subscribes to Redis pub/sub and fans messages out to connected sockets.
	go app.Services.Hub.Run(ctx)

	// Router registers GET /ws (Hub.ServeWS) along with every other route.
	srv := &http.Server{
		Addr:              app.Cfg.HTTPAddr,
		Handler:           app.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		app.Log.Info("Sarathi backend started", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	app.Log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		app.Log.Error("graceful shutdown failed", "err", err)
	}
}
