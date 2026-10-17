package utils

import (
	"errors"
	"fmt"
	"testing"
)

func TestStatusFor(t *testing.T) {
	want := map[Kind]int{KindValidation: 400, KindUnauthorized: 401, KindNotFound: 404,
		KindConflict: 409, KindBusinessRule: 422, KindInternal: 500, KindTimeout: 504}
	for k, code := range want {
		if got := StatusFor(k); got != code {
			t.Errorf("StatusFor(%d) = %d, want %d", k, got, code)
		}
	}
}

func TestConstructors(t *testing.T) {
	base := errors.New("disk full")
	if e := Internal(base); e.Message != "internal server error" || !errors.Is(e, base) {
		t.Errorf("Internal: %+v", e)
	}
	if e := Timeout(nil); e.Kind != KindTimeout || e.Message != "request timed out" {
		t.Errorf("Timeout: %+v", e)
	}
	if e := Unauthorized(); e.Kind != KindUnauthorized || e.Message != "unauthorized" {
		t.Errorf("Unauthorized: %+v", e)
	}
	e := Validation("validation failed", FieldError{Field: "age", Message: "must be greater than 0"})
	if len(e.Details) != 1 || e.Details[0].Field != "age" {
		t.Errorf("Validation details: %+v", e.Details)
	}
	var target *AppError
	if !errors.As(fmt.Errorf("wrap: %w", NotFound("Employee not found")), &target) || target.Kind != KindNotFound {
		t.Error("errors.As through wrap failed")
	}
}
