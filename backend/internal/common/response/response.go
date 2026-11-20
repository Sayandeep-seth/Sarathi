// Package response contains JSON helpers for HTTP handlers.
package response

import (
	"encoding/json"
	"io"
	"net/http"

	apperr "sarathi/internal/common/errors"
)

// JSON writes v as JSON with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func OK(w http.ResponseWriter, v any)      { JSON(w, http.StatusOK, v) }
func Created(w http.ResponseWriter, v any) { JSON(w, http.StatusCreated, v) }

// Error writes an AppError (or a generic 500) as JSON.
func Error(w http.ResponseWriter, err error) {
	ae := apperr.As(err)
	JSON(w, ae.Status, map[string]any{"error": ae})
}

// Decode parses a JSON body (max 1 MiB) into v.
func Decode(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		return apperr.BadRequest("invalid JSON body: " + err.Error())
	}
	return nil
}
