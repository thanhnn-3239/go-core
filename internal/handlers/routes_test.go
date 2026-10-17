package handlers

import (
	"strings"
	"testing"
)

func TestRouterFallbacks(t *testing.T) {
	h, _ := newTestRouter(t)
	rec := do(t, h, "PATCH", "/departments", "")
	if rec.Code != 405 || rec.Header().Get("Allow") != "GET, POST" ||
		strings.TrimSpace(rec.Body.String()) != `{"error":"method not allowed","code":405}` {
		t.Fatal(rec.Code, rec.Header(), rec.Body)
	}
	for _, p := range []string{"/nope", "/departments/"} {
		rec = do(t, h, "GET", p, "")
		if rec.Code != 404 || strings.TrimSpace(rec.Body.String()) != `{"error":"route not found","code":404}` {
			t.Fatal(p, rec.Code, rec.Body)
		}
	}
	rec = do(t, h, "GET", "/health", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatal(rec.Body)
	}
}
