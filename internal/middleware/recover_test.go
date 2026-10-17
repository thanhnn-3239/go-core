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

func TestRecoverAbortHandlerRepanics(t *testing.T) {
	defer func() {
		if v := recover(); v != http.ErrAbortHandler {
			t.Fatalf("got %v", v)
		}
	}()
	Recover(slog.New(slog.NewTextHandler(io.Discard, nil)))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}

func TestRecoverAfterHeaderWritten(t *testing.T) {
	rec := httptest.NewRecorder()
	Recover(slog.New(slog.NewTextHandler(io.Discard, nil)))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(202)
		panic("late")
	})).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 202 || rec.Body.Len() != 0 {
		t.Fatal(rec.Code, rec.Body)
	}
}

func TestRecoverLogsPanicAndStack(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	rec := httptest.NewRecorder()
	Recover(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })).
		ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	out := buf.String()
	for _, want := range []string{"level=ERROR", "msg=panic", "panic=boom", "stack="} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}
