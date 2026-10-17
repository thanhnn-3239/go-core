package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"go-core/internal/utils"
)

// headerTracker records whether the response header has been written.
type headerTracker struct {
	http.ResponseWriter
	wroteHeader bool
}

func (h *headerTracker) WriteHeader(code int) {
	h.wroteHeader = true
	h.ResponseWriter.WriteHeader(code)
}

func (h *headerTracker) Write(p []byte) (int, error) {
	h.wroteHeader = true
	return h.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the underlying ResponseWriter.
func (h *headerTracker) Unwrap() http.ResponseWriter { return h.ResponseWriter }

// Recover turns a handler panic into a 500 JSON response and logs the panic
// value with its stack. http.ErrAbortHandler is re-panicked, as net/http
// expects. If the header was already written, the panic is only logged.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ht := &headerTracker{ResponseWriter: w}
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if v == http.ErrAbortHandler {
					panic(v)
				}
				logger.ErrorContext(r.Context(), "panic",
					slog.Any("panic", v),
					slog.String("stack", string(debug.Stack())),
				)
				if !ht.wroteHeader {
					utils.WriteError(ht, utils.Internal(nil))
				}
			}()
			next.ServeHTTP(ht, r)
		})
	}
}
