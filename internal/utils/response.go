package utils

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// MaxBodyBytes is the largest request body DecodeJSON accepts.
const MaxBodyBytes = 1 << 20

// ErrorResponse is the JSON body of every error response.
type ErrorResponse struct {
	Error   string       `json:"error"`
	Code    int          `json:"code"`
	Details []FieldError `json:"details,omitempty"`
}

// WriteJSON writes v as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// WriteError is the single place that maps an error to an HTTP response.
// Errors that are not an *AppError become Timeout (context deadline) or
// Internal. The underlying error is logged when the status is 500 or above.
func WriteError(w http.ResponseWriter, err error) {
	var ae *AppError
	if !errors.As(err, &ae) {
		if errors.Is(err, context.DeadlineExceeded) {
			ae = Timeout(err)
		} else {
			ae = Internal(err)
		}
	}
	status := StatusFor(ae.Kind)
	if status >= http.StatusInternalServerError {
		cause := ae.Err
		if cause == nil {
			cause = ae
		}
		slog.Default().Error("request failed", "status", status, "error", cause)
	}
	_ = WriteJSON(w, status, ErrorResponse{Error: ae.Message, Code: status, Details: ae.Details})
}

// HandlerFunc is an HTTP handler that reports failures by returning an error.
type HandlerFunc func(http.ResponseWriter, *http.Request) error

// Handle adapts a HandlerFunc to http.HandlerFunc, writing any returned error
// with WriteError.
func Handle(fn HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			WriteError(w, err)
		}
	}
}

// DecodeJSON reads one JSON object from the request body into dst, rejecting
// unknown fields, oversized bodies and trailing data. Every error it returns
// is a Validation error whose message starts with "invalid request body: ".
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return badBody(bodyErrorMessage(err))
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return badBody(bodyErrorMessage(err))
		}
		return badBody("unexpected data after JSON object")
	}
	return nil
}

func badBody(msg string) *AppError {
	return Validation("invalid request body: " + msg)
}

func bodyErrorMessage(err error) string {
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, io.EOF):
		return "body must not be empty"
	case errors.As(err, &tooLarge):
		return "body exceeds 1MB limit"
	default:
		return strings.TrimPrefix(err.Error(), "json: ")
	}
}
