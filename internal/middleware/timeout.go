package middleware

import (
	"context"
	"net/http"
	"time"
)

// Timeout gives each request a context that expires after d. The deadline
// propagates to database calls; it does not interrupt the handler itself.
func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
