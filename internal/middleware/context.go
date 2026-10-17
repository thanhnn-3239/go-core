package middleware

import "context"

type ctxKey int

const (
	userKey ctxKey = iota
	requestInfoKey
)

// WithUser returns a copy of ctx carrying the authenticated user name.
func WithUser(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, userKey, name)
}

// UserFrom returns the authenticated user name stored by WithUser.
func UserFrom(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(userKey).(string)
	return name, ok
}

// requestInfo is a mutable per-request record created by Logging. Inner
// middleware (Auth) fills it in so the outer Logging can report it after the
// handler has finished.
type requestInfo struct {
	User string
}

func withRequestInfo(ctx context.Context, ri *requestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey, ri)
}

// requestInfoFrom returns the requestInfo stored by Logging. It never returns
// nil: without Logging in the chain it returns a throwaway value so callers
// can write to it unconditionally.
func requestInfoFrom(ctx context.Context) *requestInfo {
	if ri, ok := ctx.Value(requestInfoKey).(*requestInfo); ok {
		return ri
	}
	return &requestInfo{}
}
