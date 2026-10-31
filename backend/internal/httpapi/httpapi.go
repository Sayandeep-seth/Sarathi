// Package httpapi wires HTTP routes, JSON helpers and health endpoints.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/sarathi/sarathi/backend/internal/middleware"
)

// Checker is a dependency whose health contributes to readiness.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// Options configure NewHandler.
type Options struct {
	Log            *slog.Logger
	Version        string
	Env            string
	Checkers       []Checker
	MaxBodyBytes   int64
	RequestTimeout time.Duration
	CORSOrigins    []string
}

type response struct {
	Data any `json:"data"`
}

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// WriteJSON writes {"data": v} with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response{Data: v})
}

// WriteError writes the standard error envelope {"error": {...}}.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]errorBody{"error": {
		Code: code, Message: msg, RequestID: middleware.RequestIDFrom(r.Context()),
	}})
}

// NewHandler builds the full middleware-wrapped router.
func NewHandler(o Options) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz(o))
	mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"version": o.Version, "environment": o.Env})
	})
	// Unknown routes and wrong methods use the standard error envelope.
	root := http.NewServeMux()
	root.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern == "" {
			WriteError(w, r, http.StatusNotFound, "not_found", "resource not found")
			return
		}
		mux.ServeHTTP(w, r)
	}))

	return middleware.Chain(root,
		middleware.RequestID,
		middleware.Recover(o.Log),
		middleware.Logging(o.Log),
		middleware.SecurityHeaders,
		middleware.CORS(o.CORSOrigins),
		middleware.BodyLimit(o.MaxBodyBytes),
		middleware.Deadline(o.RequestTimeout),
	)
}

// healthz is liveness: the process is up and serving. It checks no dependencies.
func healthz(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz is readiness: every dependency must answer within its deadline.
// Failure detail is deliberately not exposed to callers; only the dependency name.
func readyz(o Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		results := make(map[string]string, len(o.Checkers))
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, c := range o.Checkers {
			wg.Add(1)
			go func(c Checker) {
				defer wg.Done()
				status := "ok"
				if err := c.Check(ctx); err != nil {
					status = "unavailable"
					o.Log.Warn("readiness_check_failed",
						slog.String("dependency", c.Name()),
						slog.String("error", err.Error()),
						slog.String("request_id", middleware.RequestIDFrom(r.Context())))
				}
				mu.Lock()
				results[c.Name()] = status
				mu.Unlock()
			}(c)
		}
		wg.Wait()

		names := make([]string, 0, len(results))
		for n := range results {
			names = append(names, n)
		}
		sort.Strings(names)
		code, overall := http.StatusOK, "ready"
		for _, n := range names {
			if results[n] != "ok" {
				code, overall = http.StatusServiceUnavailable, "not_ready"
			}
		}
		WriteJSON(w, code, map[string]any{"status": overall, "checks": results})
	}
}
