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
}

// NewAPI creates a new API instance with struct member APIHandler uninitialized.
func NewAPI(l infra.Logger) *API {
	api := &API{
		Log: l,
	}
	return api
}

// responseWriter stores whether response has been already written in the
// isWritten variable.
type responseWriter struct {
	http.ResponseWriter
	isWritten bool
	status    int
}

func (w *responseWriter) WriteHeader(status int) {
	w.isWritten = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.isWritten = true
	return w.ResponseWriter.Write(b)
}

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
			a.Error(w, http.StatusUnsupportedMediaType, msg)
			return
		}
	}
	rw := &responseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
	ctx := r.Context()
	err := a.APIHandler.Serve(ctx, rw, r)
	if err == nil && !rw.isWritten {
		status := http.StatusNotImplemented
		a.Error(rw, status, http.StatusText(status))
		return
	}
	var apiError APIError
	isAPIErr := errors.As(err, &apiError)
	fmt.Println(isAPIErr, rw.status)
	switch responseStatus := rw.status; {
	case responseStatus > 399 && responseStatus < 500:
		a.Log.Printf("client error: %v", responseStatus)
	case responseStatus == 500 || responseStatus > 501:
		a.Log.Printf("server error: %v: %v", responseStatus, err)
		if isAPIErr {
			a.Log.Printf("API ERROR: %v", apiError)
		}
	}
	if err == nil {
		return
	}
	if isAPIErr {
		a.Error(rw, apiError.StatusCode(), apiError.Errors()...)
		return
	}
	status := http.StatusInternalServerError
	a.Error(rw, status, http.StatusText(status))
}

// ErrorMessage is JSON formatted error message targetted to be consumed by machine.
type ErrorMessage struct {
	Errors []string `json:"errors"`
}

func NewErrorMessage(errors ...string) *ErrorMessage {
	return &ErrorMessage{
		Errors: errors,
	}
}

// Error writes a JSON formatted error response.
func (a API) Error(w http.ResponseWriter, statusCode int, messages ...string) {
	type Response struct {
		Errors []string `json:"errors"`
	}
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	msg := NewErrorMessage(messages...)
	if errOnError := EncodeJSON(w, &msg); errOnError != nil {
		a.Log.Printf("rest: error while writing error response: %v", errOnError)
		return
	}
}

// EncodeJSON encodes a JSON message to HTTP response.
func EncodeJSON(w http.ResponseWriter, msg any) error {
	encoder := json.NewEncoder(w)
	if cause := encoder.Encode(msg); cause != nil {
		return NewAPIError(http.StatusInternalServerError, cause)
	}
	return nil
}

// DecodeJSON reads a JSON message from HTTP request.
func DecodeJSON(body io.Reader, o any) error {
	decoder := json.NewDecoder(body)
	if cause := decoder.Decode(o); cause != nil {
		return NewAPIError(http.StatusBadRequest, cause).(error)
	}
	return nil
}
