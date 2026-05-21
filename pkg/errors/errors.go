package errors

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewAppError(code string, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}

func NotFound(message string) *AppError {
	return NewAppError("NOT_FOUND", message, http.StatusNotFound)
}

func BadRequest(message string) *AppError {
	return NewAppError("BAD_REQUEST", message, http.StatusBadRequest)
}

func Unauthorized(message string) *AppError {
	return NewAppError("UNAUTHORIZED", message, http.StatusUnauthorized)
}

func Forbidden(message string) *AppError {
	return NewAppError("FORBIDDEN", message, http.StatusForbidden)
}

func Internal(message string) *AppError {
	return NewAppError("INTERNAL_ERROR", message, http.StatusInternalServerError)
}

func Conflict(message string) *AppError {
	return NewAppError("CONFLICT", message, http.StatusConflict)
}

func TooManyRequests(message string) *AppError {
	return NewAppError("TOO_MANY_REQUESTS", message, http.StatusTooManyRequests)
}

func Wrap(err error, code string, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf("%s: %v", message, err),
		Status:  status,
	}
}
