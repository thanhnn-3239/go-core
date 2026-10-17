package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// utils.WriteError logs 5xx errors; keep test output clean.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

func TestChainOrder(t *testing.T) {
	var order []string
	mk := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { order = append(order, "h") }), mk("a"), mk("b")).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if strings.Join(order, ",") != "a,b,h" {
		t.Fatal(order)
	}
}

func TestUserContext(t *testing.T) {
	if _, ok := UserFrom(httptest.NewRequest("GET", "/", nil).Context()); ok {
		t.Fatal("expected no user on a bare context")
	}
	r := httptest.NewRequest("GET", "/", nil)
	ctx := WithUser(r.Context(), "admin")
	if u, ok := UserFrom(ctx); !ok || u != "admin" {
		t.Fatalf("got %q, %v", u, ok)
	}
}
