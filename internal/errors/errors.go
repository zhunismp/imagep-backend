package errors

import "net/http"

const (
	ErrCodeNotFound     = "NOT_FOUND"
	ErrCodeValidation   = "VALIDATION_ERROR"
	ErrCodeUnauthorized = "UNAUTHORIZED"
	ErrCodeForbidden    = "FORBIDDEN"
	ErrCodeInternal     = "INTERNAL_ERROR"
)

func (a *AppError) MapToHttpCode() int {
	switch a.Code {
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeValidation:
		return http.StatusUnprocessableEntity
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
