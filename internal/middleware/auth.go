package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"slices"
	"strings"

	"go-core/internal/utils"
)

// dummyPassword is compared against when the user is unknown so that the
// response time does not reveal whether a user exists.
const dummyPassword = "dummy-password-for-unknown-users"

// Auth enforces HTTP Basic authentication against users (name -> password).
// Requests whose path exactly matches one of skipPaths are passed through.
func Auth(users map[string]string, skipPaths ...string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if slices.Contains(skipPaths, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			name, ok := authenticate(users, r.Header.Get("Authorization"))
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="ems"`)
				utils.WriteError(w, utils.Unauthorized())
				return
			}
			ctx := r.Context()
			requestInfoFrom(ctx).User = name
			next.ServeHTTP(w, r.WithContext(WithUser(ctx, name)))
		})
	}
}

// authenticate parses a Basic Authorization header and verifies the
// credentials, returning the user name on success.
func authenticate(users map[string]string, header string) (string, bool) {
	encoded, ok := strings.CutPrefix(header, "Basic ")
	if !ok {
		return "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	name, got, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return "", false
	}
	want, known := users[name]
	if !known {
		want = dummyPassword
	}
	match := passwordsEqual(got, want)
	return name, match && known
}

// passwordsEqual compares fixed-length digests so the comparison time does
// not depend on the password contents or length.
func passwordsEqual(got, want string) bool {
	g := sha256.Sum256([]byte(got))
	w := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(g[:], w[:]) == 1
}
