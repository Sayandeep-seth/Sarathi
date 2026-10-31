package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeChecker struct {
	name string
	err  error
}

func (f fakeChecker) Name() string                { return f.name }
func (f fakeChecker) Check(context.Context) error { return f.err }

func handler(cs ...Checker) http.Handler {
	return NewHandler(Options{
		Log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version:        "test",
		Env:            "local",
		Checkers:       cs,
		MaxBodyBytes:   1024,
		RequestTimeout: time.Second,
	})
}

func get(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestHealthzIgnoresDependencies(t *testing.T) {
	rec := get(handler(fakeChecker{"postgres", errors.New("down")}), "GET", "/healthz")
	if rec.Code != 200 {
		t.Fatalf("liveness must not depend on dependencies, got %d", rec.Code)
	}
}

func TestReadyz(t *testing.T) {
	rec := get(handler(fakeChecker{"postgres", nil}, fakeChecker{"redis", nil}), "GET", "/readyz")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"ready"`) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = get(handler(fakeChecker{"postgres", nil}, fakeChecker{"redis", errors.New("conn refused 10.0.0.5:6379")}), "GET", "/readyz")
	if rec.Code != 503 {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Fatal("readiness response must not leak internal error detail")
	}
	var body struct {
		Data struct {
			Status string            `json:"status"`
			Checks map[string]string `json:"checks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Status != "not_ready" || body.Data.Checks["redis"] != "unavailable" || body.Data.Checks["postgres"] != "ok" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestVersionAndHeaders(t *testing.T) {
	rec := get(handler(), "GET", "/api/v1/version")
	if rec.Code != 200 || rec.Header().Get("X-Request-ID") == "" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("code=%d headers=%v", rec.Code, rec.Header())
	}
}

func TestUnknownRouteAndMethodUseErrorEnvelope(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/v1/does-not-exist"},
		{"POST", "/healthz"},
	} {
		rec := get(handler(), tc.method, tc.path)
		if rec.Code != 404 || !strings.Contains(rec.Body.String(), `"not_found"`) {
			t.Fatalf("%s %s: code=%d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}
