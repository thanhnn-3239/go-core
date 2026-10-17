package utils

import (
	"fmt"
	"net/url"
	"strconv"
)

const (
	defaultLimit = 10
	maxLimit     = 100
)

// ParseID parses a positive integer path parameter. entity names the resource
// in the error message, e.g. "employee" or "department".
func ParseID(raw, entity string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, Validation(fmt.Sprintf("invalid %s id: %q", entity, raw))
	}
	return id, nil
}

// ParsePagination reads the limit (1-100, default 10) and offset (>= 0,
// default 0) query parameters.
func ParsePagination(q url.Values) (limit, offset int, err error) {
	limit = defaultLimit
	if raw := q.Get("limit"); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n < 1 || n > maxLimit {
			return 0, 0, Validation("limit must be between 1 and 100")
		}
		limit = n
	}
	if raw := q.Get("offset"); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n < 0 {
			return 0, 0, Validation("offset must be a non-negative integer")
		}
		offset = n
	}
	return limit, offset, nil
}

// ParseOptionalID reads an optional positive integer query parameter. It
// returns nil, nil when the parameter is absent or empty.
func ParseOptionalID(q url.Values, key string) (*int64, error) {
	raw := q.Get(key)
	if raw == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return nil, Validation(fmt.Sprintf("invalid %s: %q", key, raw))
	}
	return &id, nil
}
