package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func ok(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestRequestIDGeneratedAndPropagated(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = RequestIDFrom(r.Context()) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if seen == "" || rec.Header().Get(RequestIDHeader) != seen {
		t.Fatalf("id not generated/propagated: %q vs %q", seen, rec.Header().Get(RequestIDHeader))
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(RequestIDHeader, "client-supplied-123")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if seen != "client-supplied-123" {
		t.Fatalf("valid inbound id should be kept, got %q", seen)
	}
}

func TestRequestIDRejectsMalformed(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = RequestIDFrom(r.Context()) }))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(RequestIDHeader, "bad id\nwith injection")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if strings.Contains(seen, "injection") || seen == "" {
		t.Fatalf("malformed id must be replaced, got %q", seen)
	}
}

func TestRecoverReturnsGeneric500(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("secret-detail") }), RequestID, Recover(log))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "secret-detail") {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(buf.String(), "secret-detail") {
		t.Fatal("panic detail should be logged server-side")
	}
}

func TestBodyLimit(t *testing.T) {
	h := BodyLimit(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/", strings.NewReader("0123456789")))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/", strings.NewReader("small")))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestCORSAllowListOnly(t *testing.T) {
	h := CORS([]string{"https://admin.example"})(http.HandlerFunc(ok))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://admin.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://admin.example" {
		t.Fatal("allowed origin should be echoed")
	}

	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unknown origin must not receive CORS headers")
	}

	// Empty allow-list: never emits CORS headers.
	h = CORS(nil)(http.HandlerFunc(ok))
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://admin.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("empty allow-list must emit no CORS headers")
	}
}

func TestLoggingOmitsQueryAndHeaders(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	h := Logging(log)(http.HandlerFunc(ok))
	req := httptest.NewRequest("GET", "/x?token=SECRET", nil)
	req.Header.Set("Authorization", "Bearer SECRET")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if strings.Contains(buf.String(), "SECRET") {
		t.Fatalf("log leaked secret: %s", buf.String())
	}
}
