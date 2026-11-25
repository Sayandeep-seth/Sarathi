// Command realtime-gateway runs Sarathi's WebSocket gateway.
//
// It is a separate process from the REST API so it can be scaled on its own
// (sockets are long-lived and memory-bound; REST is short-lived and CPU-bound).
// Both talk to the same Redis. Run as many gateway replicas as you like:
// Redis Pub/Sub makes every message reach every replica that needs it.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"sarathi/backend/internal/dispatch"
	"sarathi/backend/internal/geo"
	"sarathi/backend/internal/realtime"
	"sarathi/backend/internal/realtime/handlers"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	addr := env("WS_ADDR", ":8081")
	gatewayID := env("GATEWAY_ID", hostname())
	origins := strings.Split(env("WS_ALLOWED_ORIGINS", "http://localhost:3000"), ",")

	// --- Redis ------------------------------------------------------------
	rdb := redis.NewClient(&redis.Options{
		Addr:         env("REDIS_ADDR", "localhost:6379"),
		Password:     os.Getenv("REDIS_PASSWORD"),
		DB:           0,
		PoolSize:     100,
		MinIdleConns: 10,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Error("redis not reachable", "err", err)
			os.Exit(1)
		}
		cancel()
	}
	defer rdb.Close()

	// ctx is cancelled on SIGINT/SIGTERM (Ctrl+C, docker stop, k8s rollout).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- Wiring -------------------------------------------------------------
	pub := realtime.NewPublisher(rdb)
	hub := realtime.NewHub(rdb, gatewayID, log)
	tickets := realtime.NewTicketStore(rdb, 30*time.Second)
	router := realtime.NewRouter(hub, realtime.NewAuthorizer(rdb), log)

	drivers := geo.NewDriverIndex(rdb, log)
	stations := geo.NewStationTracker(rdb, pub)
	disp := dispatch.NewService(rdb, drivers, pub, nil /* plug your Postgres TripRepository */, log)

	handlers.Register(router, handlers.Deps{
		Drivers: drivers, Stations: stations, Dispatch: disp, Pub: pub, Log: log,
	})

	hubErr := make(chan error, 1)
	go func() { hubErr <- hub.Run(ctx) }()
	go drivers.RunJanitor(ctx, 15*time.Second)

	// --- HTTP ---------------------------------------------------------------
	mux := http.NewServeMux()
	mux.Handle("GET /ws", realtime.NewServer(ctx, hub, tickets, router, origins, log))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := rdb.Ping(r.Context()).Err(); err != nil {
			http.Error(w, "redis down", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("realtime gateway listening", "addr", addr, "gateway", gatewayID)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			stop()
		}
	}()

	// --- Graceful shutdown --------------------------------------------------
	select {
	case <-ctx.Done():
	case err := <-hubErr:
		log.Error("hub stopped", "err", err)
	}
	log.Info("shutting down")

	// http.Server.Shutdown does NOT touch hijacked (WebSocket) connections,
	// so close them ourselves with 1012 "service restart". Clients reconnect
	// (with random jitter) to another replica.
	hub.CloseAll(websocket.CloseServiceRestart, "server restarting")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	time.Sleep(500 * time.Millisecond) // let close frames flush
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "gw-local"
	}
	return h
}
