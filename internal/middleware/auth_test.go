package middleware

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func basic(s string) string { return "Basic " + base64.StdEncoding.EncodeToString([]byte(s)) }

func TestAuth(t *testing.T) {
	users := map[string]string{"admin": "admin123"}
	var seenUser string
	h := Auth(users, "/health")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUser, _ = UserFrom(r.Context())
	}))
	cases := []struct {
		name, path, header string
		code               int
		user               string
	}{
		{"no header", "/employees", "", 401, ""},
		{"wrong prefix", "/employees", "Bearer abc", 401, ""},
		{"bad base64", "/employees", "Basic !!!", 401, ""},
		{"no colon", "/employees", basic("admin"), 401, ""},
		{"unknown user", "/employees", basic("bob:admin123"), 401, ""},
		{"wrong password", "/employees", basic("admin:nope"), 401, ""},
		{"ok", "/employees", basic("admin:admin123"), 200, "admin"},
		{"skip health", "/health", "", 200, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seenUser = ""
			r := httptest.NewRequest("GET", tc.path, nil)
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			if rec.Code != tc.code || seenUser != tc.user {
				t.Fatalf("code %d user %q", rec.Code, seenUser)
			}
			if tc.code == 401 {
				if rec.Header().Get("WWW-Authenticate") != `Basic realm="ems"` ||
					strings.TrimSpace(rec.Body.String()) != `{"error":"unauthorized","code":401}` {
					t.Fatal(rec.Header(), rec.Body)
				}
			}
		})
	}
}

func TestAuthRecordsUserInRequestInfo(t *testing.T) {
	ri := &requestInfo{}
	h := Auth(map[string]string{"admin": "admin123"})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	r := httptest.NewRequest("GET", "/employees", nil)
	r.Header.Set("Authorization", basic("admin:admin123"))
	r = r.WithContext(withRequestInfo(r.Context(), ri))
	h.ServeHTTP(httptest.NewRecorder(), r)
	if ri.User != "admin" {
		t.Fatalf("requestInfo.User = %q", ri.User)
	}
}
