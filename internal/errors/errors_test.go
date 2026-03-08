package errors

import (
	"fmt"
	"net/http"
	"testing"
)

func TestMapToHttpCode_ReturnsNotFoundForNotFoundCode(t *testing.T) {
	err := New(ErrCodeNotFound, "not found", nil)
	if got := err.MapToHttpCode(); got != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, got)
	}
}

func TestMapToHttpCode_ReturnsUnprocessableEntityForValidationCode(t *testing.T) {
	err := New(ErrCodeValidation, "validation", nil)
	if got := err.MapToHttpCode(); got != http.StatusUnprocessableEntity {
		t.Errorf("expected %d, got %d", http.StatusUnprocessableEntity, got)
	}
}

func TestMapToHttpCode_ReturnsUnauthorizedForUnauthorizedCode(t *testing.T) {
	err := New(ErrCodeUnauthorized, "unauthorized", nil)
	if got := err.MapToHttpCode(); got != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, got)
	}
}

func TestMapToHttpCode_ReturnsForbiddenForForbiddenCode(t *testing.T) {
	err := New(ErrCodeForbidden, "forbidden", nil)
	if got := err.MapToHttpCode(); got != http.StatusForbidden {
		t.Errorf("expected %d, got %d", http.StatusForbidden, got)
	}
}

func TestMapToHttpCode_ReturnsInternalServerErrorForInternalCode(t *testing.T) {
	err := New(ErrCodeInternal, "internal", nil)
	if got := err.MapToHttpCode(); got != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, got)
	}
}

func TestMapToHttpCode_ReturnsInternalServerErrorForUnknownCode(t *testing.T) {
	err := New("UNKNOWN", "unknown", nil)
	if got := err.MapToHttpCode(); got != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, got)
	}
}

func TestError_IncludesOriginalErrorWhenWrapped(t *testing.T) {
	orig := fmt.Errorf("original error")
	err := New(ErrCodeInternal, "wrapped", orig)
	got := err.Error()
	if got != "INTERNAL_ERROR - wrapped | original_err: original error" {
		t.Errorf("unexpected error string: %s", got)
	}
}

func TestError_OmitsOriginalErrorWhenNil(t *testing.T) {
	err := New(ErrCodeNotFound, "missing", nil)
	got := err.Error()
	if got != "NOT_FOUND - missing" {
		t.Errorf("unexpected error string: %s", got)
	}
}

func TestNew_SetsAllFields(t *testing.T) {
	orig := fmt.Errorf("cause")
	err := New(ErrCodeValidation, "bad input", orig)

	if err.Code != ErrCodeValidation {
		t.Errorf("expected code %s, got %s", ErrCodeValidation, err.Code)
	}
	if err.Message != "bad input" {
		t.Errorf("expected message 'bad input', got %s", err.Message)
	}
	if err.Unwrap() != orig {
		t.Errorf("expected wrapped error to be the original")
	}
}
