package rest

import (
	"net/http"
)

var (
	ErrAuth = NewAPIErrorWithMessage(http.StatusUnauthorized, "authorization required")

	ErrNotFound = NewAPIErrorWithMessage(http.StatusNotFound, "item not found")

	ErrInvalidRequestBody = NewAPIErrorWithMessage(http.StatusBadRequest, "expecting well formed JSON request body")

	ErrInternal = NewAPIErrorWithMessage(http.StatusInternalServerError, "internal server error")
)

type APIError interface {
	// StatusCode returns HTTP status code.
	StatusCode() int
	// Error returns the underlying error message.
	Error() string
	// ErrorAPI returns an API compatible error message.
	ErrorAPI() string
}

type basicAPIError struct {
	statusCode int
	message    string
}

func NewAPIError(statusCode int) APIError {
	return &basicAPIError{
		statusCode: statusCode,
		message:    http.StatusText(statusCode),
	}
}

func NewAPIErrorWithMessage(statusCode int, message string) APIError {
	return &basicAPIError{
		statusCode: statusCode,
		message:    message,
	}
}

func (e *basicAPIError) StatusCode() int {
	return e.statusCode
}

func (e *basicAPIError) ErrorAPI() string {
	return e.message
}

func (e *basicAPIError) Error() string {
	return e.message
}

func (e *basicAPIError) Is(target error) bool {
	if _, ok := target.(APIError); ok {
		return true
	}
	return false
}

// NewValidationError is called when a data validation error occurs.
func NewValidationError(message string) APIError {
	return &basicAPIError{
		statusCode: http.StatusUnprocessableEntity,
		message:    message,
	}
}

func NewValidationErrorWithCause(cause error, apiError APIError) APIError {
	return &APIErrorWithCause{
		error:    cause,
		APIError: apiError,
	}
}

type APIErrorWithCause struct {
	// error is the error that causes this error in the first place.
	error
	APIError
}

func NewAPIErrorWithCause(cause error, apiError APIError) APIError {
	return &APIErrorWithCause{
		error:    cause,
		APIError: apiError,
	}
}
func (e *APIErrorWithCause) Error() string {
	return e.error.Error()
}
func (e *APIErrorWithCause) Is(err error) bool {
	return e.APIError == err
}
