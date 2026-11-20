// Package errors provides typed application errors that map cleanly to HTTP.
package errors

import (
	stderrors "errors"
	"fmt"
	"net/http"
)

// AppError is an error with an HTTP status and a stable machine-readable code.
type AppError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return e.Code + ": " + e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

// New builds an AppError.
func New(status int, code, msg string) *AppError {
	return &AppError{Status: status, Code: code, Message: msg}
}

func BadRequest(msg string) *AppError  { return New(http.StatusBadRequest, "bad_request", msg) }
func NotFound(msg string) *AppError    { return New(http.StatusNotFound, "not_found", msg) }
func Conflict(msg string) *AppError    { return New(http.StatusConflict, "conflict", msg) }
func Forbidden(msg string) *AppError   { return New(http.StatusForbidden, "forbidden", msg) }
func Unavailable(msg string) *AppError { return New(http.StatusServiceUnavailable, "unavailable", msg) }
func TooMany(msg string) *AppError     { return New(http.StatusTooManyRequests, "rate_limited", msg) }

// Internal wraps an unexpected error.
func Internal(err error) *AppError {
	return &AppError{Status: http.StatusInternalServerError, Code: "internal", Message: "internal error", Err: err}
}

// As converts any error into an AppError (unknown errors become 500).
func As(err error) *AppError {
	var ae *AppError
	if stderrors.As(err, &ae) {
		return ae
	}
	return Internal(err)
}

// IsNotFound reports whether err is a 404 AppError.
func IsNotFound(err error) bool {
	var ae *AppError
	return stderrors.As(err, &ae) && ae.Status == http.StatusNotFound
}
