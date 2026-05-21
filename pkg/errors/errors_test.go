package errors

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewAppError_CreatesCorrectFields(t *testing.T) {
	t.Parallel()

	err := NewAppError("TEST_CODE", "test message", 418)

	if err.Code != "TEST_CODE" {
		t.Errorf("Code = %q, want %q", err.Code, "TEST_CODE")
	}
	if err.Message != "test message" {
		t.Errorf("Message = %q, want %q", err.Message, "test message")
	}
	if err.Status != 418 {
		t.Errorf("Status = %d, want 418", err.Status)
	}
}

func TestAppError_Error_ReturnsFormattedString(t *testing.T) {
	t.Parallel()

	err := NewAppError("ERR_001", "something went wrong", 500)
	s := err.Error()

	expected := "[ERR_001] something went wrong"
	if s != expected {
		t.Errorf("Error() = %q, want %q", s, expected)
	}
}

func TestNotFound_StatusIs404(t *testing.T) {
	t.Parallel()

	err := NotFound("resource not found")

	if err.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", err.Status)
	}
	if err.Code != "NOT_FOUND" {
		t.Errorf("Code = %q, want NOT_FOUND", err.Code)
	}
}

func TestBadRequest_StatusIs400(t *testing.T) {
	t.Parallel()

	err := BadRequest("invalid input")

	if err.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", err.Status)
	}
	if err.Code != "BAD_REQUEST" {
		t.Errorf("Code = %q, want BAD_REQUEST", err.Code)
	}
}

func TestUnauthorized_StatusIs401(t *testing.T) {
	t.Parallel()

	err := Unauthorized("not authenticated")

	if err.Status != http.StatusUnauthorized {
		t.Errorf("Status = %d, want 401", err.Status)
	}
	if err.Code != "UNAUTHORIZED" {
		t.Errorf("Code = %q, want UNAUTHORIZED", err.Code)
	}
}

func TestForbidden_StatusIs403(t *testing.T) {
	t.Parallel()

	err := Forbidden("access denied")

	if err.Status != http.StatusForbidden {
		t.Errorf("Status = %d, want 403", err.Status)
	}
	if err.Code != "FORBIDDEN" {
		t.Errorf("Code = %q, want FORBIDDEN", err.Code)
	}
}

func TestInternal_StatusIs500(t *testing.T) {
	t.Parallel()

	err := Internal("server error")

	if err.Status != http.StatusInternalServerError {
		t.Errorf("Status = %d, want 500", err.Status)
	}
	if err.Code != "INTERNAL_ERROR" {
		t.Errorf("Code = %q, want INTERNAL_ERROR", err.Code)
	}
}

func TestConflict_StatusIs409(t *testing.T) {
	t.Parallel()

	err := Conflict("duplicate")

	if err.Status != http.StatusConflict {
		t.Errorf("Status = %d, want 409", err.Status)
	}
	if err.Code != "CONFLICT" {
		t.Errorf("Code = %q, want CONFLICT", err.Code)
	}
}

func TestTooManyRequests_StatusIs429(t *testing.T) {
	t.Parallel()

	err := TooManyRequests("rate limit exceeded")

	if err.Status != http.StatusTooManyRequests {
		t.Errorf("Status = %d, want 429", err.Status)
	}
	if err.Code != "TOO_MANY_REQUESTS" {
		t.Errorf("Code = %q, want TOO_MANY_REQUESTS", err.Code)
	}
}

func TestWrapError_PreservesOriginal(t *testing.T) {
	t.Parallel()

	original := errors.New("original error")
	wrapped := Wrap(original, "WRAPPED", "wrapped message", 500)

	if wrapped.Code != "WRAPPED" {
		t.Errorf("Code = %q, want WRAPPED", wrapped.Code)
	}
	if wrapped.Status != 500 {
		t.Errorf("Status = %d, want 500", wrapped.Status)
	}
	if wrapped.Error() != "[WRAPPED] wrapped message: original error" {
		t.Errorf("Error() = %q, want %q", wrapped.Error(), "[WRAPPED] wrapped message: original error")
	}
}
