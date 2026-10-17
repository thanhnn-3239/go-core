package utils

import "net/http"

// Kind classifies an AppError; WriteError maps it to an HTTP status.
type Kind int

const (
	KindValidation Kind = iota + 1
	KindNotFound
	KindConflict
	KindBusinessRule
	KindUnauthorized
	KindTimeout
	KindInternal
)

// FieldError describes one invalid field in a request.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// AppError is the error type shared by all layers.
type AppError struct {
	Kind    Kind
	Message string       // message sent to the client, in English
	Details []FieldError // only used for field validation errors
	Err     error        // underlying error, logged only
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func Validation(msg string, details ...FieldError) *AppError {
	return &AppError{Kind: KindValidation, Message: msg, Details: details}
}

func NotFound(msg string) *AppError {
	return &AppError{Kind: KindNotFound, Message: msg}
}

func Conflict(msg string, err error) *AppError {
	return &AppError{Kind: KindConflict, Message: msg, Err: err}
}

func BusinessRule(msg string, err error) *AppError {
	return &AppError{Kind: KindBusinessRule, Message: msg, Err: err}
}

func Unauthorized() *AppError {
	return &AppError{Kind: KindUnauthorized, Message: "unauthorized"}
}

func Timeout(err error) *AppError {
	return &AppError{Kind: KindTimeout, Message: "request timed out", Err: err}
}

func Internal(err error) *AppError {
	return &AppError{Kind: KindInternal, Message: "internal server error", Err: err}
}

// StatusFor maps a Kind to its HTTP status code.
func StatusFor(k Kind) int {
	switch k {
	case KindValidation:
		return http.StatusBadRequest
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindNotFound:
		return http.StatusNotFound
	case KindConflict:
		return http.StatusConflict
	case KindBusinessRule:
		return http.StatusUnprocessableEntity
	case KindTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
