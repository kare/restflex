package rest

// TODO: rename package to framework name
// TODO: name this framework first!

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"kkn.fi/infra"
)

// Handler interface supports use of Context and centralized error handling.
type Handler interface {
	Serve(ctx context.Context, w http.ResponseWriter, r *http.Request) error
}

// HandlerFunc adapts any function to Handler type.
type HandlerFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request) error

func (h HandlerFunc) Serve(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return h(ctx, w, r)
}

// API holds necessary components for constructing an API.
type API struct {
	http.Handler
	APIHandler Handler
	// Log logs messages
	Log infra.Logger
	// timeout for context timeouts
	timeout time.Duration
}

// NewAPI creates a new API instance with struct member APIHandler uninitialized.
func NewAPI(l infra.Logger, timeout time.Duration) *API {
	api := &API{
		Log:     l,
		timeout: timeout,
	}
	return api
}

// responseWriter stores whether response has been already written in the
// isWritten variable.
type responseWriter struct {
	http.ResponseWriter
	isWritten bool
}

func (w *responseWriter) WriteHeader(status int) {
	w.isWritten = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.isWritten = true
	return w.ResponseWriter.Write(b)
}

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), a.timeout)
	defer cancel()
	go func() {
		<-ctx.Done()
		if err := ctx.Err(); err != nil {
			switch {
			case errors.Is(err, context.DeadlineExceeded):
				a.Log.Printf("rest: API timeout in %v path '%v': %v", a.timeout, r.RequestURI, err)
				a.Error(w, "request took too long to complete", http.StatusTooManyRequests)
				return
			case errors.Is(err, context.Canceled):
				// context was cancelled after successful operation
				return
			}
		}
	}()
	if method := r.Method; method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		correctContentTypeFound := false
		acceptedContentTypes := []string{
			"application/json",
			"application/x-www-form-urlencoded",
		}
		contentType := r.Header.Get("Content-Type")
		for _, v := range strings.Split(contentType, ",") {
			t, _, err := mime.ParseMediaType(v)
			if err != nil {
				continue
			}
			for _, acceptedContentType := range acceptedContentTypes {
				if strings.HasPrefix(t, acceptedContentType) {
					correctContentTypeFound = true
					break
				}
			}
		}
		if !correctContentTypeFound {
			msg := "POST, PUT, and PATCH methods require request content type of "
			for i, acceptedContentType := range acceptedContentTypes {
				msg += fmt.Sprintf("%q", acceptedContentType)
				if i-1 < len(acceptedContentTypes) {
					msg += " or "
				}
			}
			a.Error(w, msg, http.StatusUnsupportedMediaType)
			return
		}
	}
	rw := &responseWriter{
		ResponseWriter: w,
	}
	err := a.APIHandler.Serve(ctx, rw, r)
	if err == nil && !rw.isWritten {
		status := http.StatusNotImplemented
		a.Error(rw, http.StatusText(status), status)
		return
	}
	if err == nil {
		return
	}
	var apiError APIError
	if errors.As(err, &apiError) {
		a.Error(w, apiError.ErrorAPI(), apiError.StatusCode())
		return
	}
	status := http.StatusInternalServerError
	a.Error(w, http.StatusText(status), status)
}

// ErrorMessage is JSON formatted error message targetted to be consumed by machine.
type ErrorMessage struct {
	Message string `json:"message"`
}

// Error is helper function for writing responses containing JSON Object with struct.
func (a API) Error(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	msg := ErrorMessage{
		Message: message,
	}
	if errOnError := EncodeJSON(w, &msg); errOnError != nil {
		a.Log.Printf("rest: error while writing JSON error response: %v", errOnError)
		return
	}
}

// EncodeJSON encodes a JSON message to HTTP response.
func EncodeJSON(w http.ResponseWriter, msg any) error {
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(msg); err != nil {
		return NewAPIErrorWithCause(err, ErrInternal)
	}
	return nil
}

// DecodeJSON reads a JSON message from HTTP request.
func DecodeJSON(w http.ResponseWriter, body io.Reader, o any) error {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(o); err != nil {
		return NewAPIErrorWithCause(err, ErrInvalidRequestBody)
	}
	return nil
}
