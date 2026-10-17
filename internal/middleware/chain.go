// Package middleware holds the HTTP middleware of the API: Logging, Recover,
// Auth and Timeout, plus the Chain helper that composes them.
package middleware

import "net/http"

// Middleware wraps an http.Handler with additional behaviour.
type Middleware func(http.Handler) http.Handler

// Chain wraps h with mws. The first middleware is the outermost layer, so
// Chain(h, A, B) behaves like A(B(h)).
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
