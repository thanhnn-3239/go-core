package utils_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	. "go-core/internal/utils"

	"go-core/internal/testutil"
)

func TestWriteError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
		body string
	}{
		{"not found", NotFound("Employee not found"), 404, `{"error":"Employee not found","code":404}`},
		{"wrapped", fmt.Errorf("svc: %w", Conflict(`department "Engineering" already exists`, nil)), 409,
			`{"error":"department \"Engineering\" already exists","code":409}`},
		{"details", Validation("validation failed", FieldError{Field: "name", Message: "is required"}), 400,
			`{"error":"validation failed","code":400,"details":[{"field":"name","message":"is required"}]}`},
		{"bare deadline", fmt.Errorf("q: %w", context.DeadlineExceeded), 504, `{"error":"request timed out","code":504}`},
		{"unknown", errors.New("boom"), 500, `{"error":"internal server error","code":500}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteError(rec, tc.err)
			if rec.Code != tc.code || strings.TrimSpace(rec.Body.String()) != tc.body {
				t.Fatalf("got %d %s", rec.Code, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
				t.Fatalf("content-type %q", ct)
			}
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	type in struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	cases := []struct{ name, body, wantMsg string }{
		{"ok", `{"name":"A","age":1}`, ""},
		{"empty", ``, "invalid request body: body must not be empty"},
		{"unknown field", `{"agee":1}`, `invalid request body: unknown field "agee"`},
		{"trailing data", `{"name":"A"}{"x":1}`, "invalid request body: unexpected data after JSON object"},
		{"too large", `{"name":"` + strings.Repeat("a", MaxBodyBytes) + `"}`, "invalid request body: body exceeds 1MB limit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			var dst in
			err := DecodeJSON(httptest.NewRecorder(), r, &dst)
			if tc.wantMsg == "" {
				if err != nil || dst.Name != "A" {
					t.Fatalf("err=%v dst=%+v", err, dst)
				}
				return
			}
			testutil.AssertAppError(t, err, KindValidation, tc.wantMsg)
		})
	}
	// Sai kiểu dữ liệu: thông điệp không lộ tên kiểu Go
	for _, tc := range []struct{ name, body, wantMsg string }{
		{"type mismatch", `{"age":"abc"}`, `invalid request body: field "age" must be a number`},
		{"top-level array", `[]`, "invalid request body: body must be a JSON object"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			err := DecodeJSON(httptest.NewRecorder(), r, &struct {
				Age int `json:"age"`
			}{})
			testutil.AssertAppError(t, err, KindValidation, tc.wantMsg)
		})
	}
}

func TestHandle(t *testing.T) {
	h := Handle(func(w http.ResponseWriter, r *http.Request) error { return NotFound("Employee not found") })
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 404 {
		t.Fatalf("code %d", rec.Code)
	}
}

func TestMain(m *testing.M) {
	// WriteError logs 5xx errors; keep test output clean.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}
