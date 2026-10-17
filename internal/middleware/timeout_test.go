package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeoutSetsDeadline(t *testing.T) {
	var deadline time.Time
	var ok bool
	Timeout(2*time.Second)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		deadline, ok = r.Context().Deadline()
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !ok || time.Until(deadline) > 2*time.Second || time.Until(deadline) < time.Second {
		t.Fatal(deadline, ok)
	}
}
