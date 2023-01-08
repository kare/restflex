package rest

import "net/http"

var (
	ErrAuth = NewAPIError(http.StatusUnauthorized, nil, "authorization required")

	ErrNotFound = NewAPIError(http.StatusNotFound, nil, "item not found")

	ErrInvalidRequestBody = NewAPIError(http.StatusBadRequest, nil, "expecting well formed request body")

	ErrBadRequest = NewBadRequest(http.StatusText(http.StatusBadRequest))

	ErrInternal = NewAPIError(http.StatusInternalServerError, nil, "internal server error")
)

type APIError interface {
	error
	// StatusCode returns HTTP status code.
	StatusCode() int
	// Unwrap returns the underlying cause for this APIError if any.
	Unwrap() error
	// Errors returns an API compatible list of error messages.
	Errors() []string
}

type apiError struct {
	statusCode int
	cause      error
	messages   []string
}

func NewAPIError(statusCode int, cause error, messages ...string) APIError {
	return &apiError{
		statusCode: statusCode,
		cause:      cause,
		messages:   messages,
	}
}

// NewValidationError is called when a data validation error occurs.
func NewValidationError(messages ...string) APIError {
	return NewAPIError(http.StatusUnprocessableEntity, nil, messages...)
}

func NewBadRequest(messages ...string) APIError {
	return NewAPIError(http.StatusBadRequest, nil, messages...)
}

func (e *apiError) Error() string {
	if e.cause != nil {
		return e.cause.Error()
	}
	if len(e.messages) > 0 {
		return e.messages[0]
	}
	return "unknown API error"
}

func (e *apiError) StatusCode() int {
	return e.statusCode
}

func (e *apiError) Unwrap() error {
	return e.cause
}

func (e *apiError) Errors() []string {
	return e.messages
}

func (e *apiError) Is(target error) bool {
	if _, ok := target.(APIError); ok {
		return true
	}
	return false
}
