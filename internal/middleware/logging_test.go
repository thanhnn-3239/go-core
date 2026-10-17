package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingRecordsStatusAndUser(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestInfoFrom(r.Context()).User = "admin" // simulates Auth recording the user name
		w.WriteHeader(201)
		w.Write([]byte("hello"))
	})
	Logging(logger)(inner).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/employees?x=1", nil))
	line := buf.String()
	for _, want := range []string{"level=INFO", "msg=request", "method=POST", `url="/employees?x=1"`, "status=201", "bytes=5", "duration=", "user=admin"} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q in %s", want, line)
		}
	}
}

func TestLoggingDefaultsWithoutAuth(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	Logging(logger)(inner).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/health", nil))
	line := buf.String()
	for _, want := range []string{"level=INFO", "status=200", "bytes=2", `user=""`} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q in %s", want, line)
		}
	}
}

func TestRequestInfoFromWithoutLoggingIsUsable(t *testing.T) {
	ri := requestInfoFrom(httptest.NewRequest("GET", "/", nil).Context())
	if ri == nil {
		t.Fatal("requestInfoFrom must never return nil")
	}
	ri.User = "x" // must not panic
}

func TestLoggingThenRecoverLogs500(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), Logging(logger), Recover(logger))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 500 || strings.TrimSpace(rec.Body.String()) != `{"error":"internal server error","code":500}` {
		t.Fatal(rec.Code, rec.Body)
	}
	if !strings.Contains(buf.String(), "status=500") || !strings.Contains(buf.String(), "level=ERROR") {
		t.Fatal(buf.String())
	}
}
