package testutil

import (
	"errors"
	"testing"

	"go-core/internal/utils"
)

// AssertAppError fails the test unless err wraps an *utils.AppError with the
// given kind and message. It returns the AppError so callers can check Details.
func AssertAppError(t *testing.T, err error, kind utils.Kind, msg string) *utils.AppError {
	t.Helper()
	var ae *utils.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("error = %v (%T), want *utils.AppError", err, err)
	}
	if ae.Kind != kind {
		t.Fatalf("kind = %d, want %d (message %q)", ae.Kind, kind, ae.Message)
	}
	if ae.Message != msg {
		t.Fatalf("message = %q, want %q", ae.Message, msg)
	}
	return ae
}
