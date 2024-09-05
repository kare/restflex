package restflex

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"kkn.fi/httpx"
	"kkn.fi/infra"
)

// handler holds necessary components for constructing a REST API HTTP request handler.
type handler struct {
	httpx.HandlerWithContext
	// Log logs messages
	Log infra.Logger
}

func NewHandlerWithContext(l infra.Logger, h httpx.HandlerWithContext) http.Handler {
	api := &handler{
		Log:                l,
		HandlerWithContext: h,
	}
	return api
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
			h.Error(w, http.StatusUnsupportedMediaType, msg)
			return
		}
	}
	rw := &responseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
	ctx := r.Context()
	err := h.ServeHTTPWithContext(ctx, rw, r)
	h.Log.Printf("error: %v is written: %v", err, rw.isWritten)
	if err == nil && !rw.isWritten {
		status := http.StatusNotImplemented
		h.Error(rw, status, http.StatusText(status))
		return
	}
	var apiError APIError
	isAPIErr := errors.As(err, &apiError)
	switch responseStatus := rw.status; {
	case responseStatus > 399 && responseStatus < 500:
		h.Log.Printf("client error: %v", responseStatus)
	case responseStatus == 500 || responseStatus > 501:
		if !isAPIErr {
			h.Log.Printf("server error: %v: %v", responseStatus, err)
		}
	}
	if err == nil {
		return
	}
	if isAPIErr {
		h.Error(rw, apiError.StatusCode(), apiError.Errors()...)
		return
	}
	status := http.StatusInternalServerError
	h.Error(rw, status, http.StatusText(status))
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
func (h handler) Error(w http.ResponseWriter, statusCode int, messages ...string) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	msg := NewErrorMessage(messages...)
	if errOnError := EncodeJSON(w, &msg); errOnError != nil {
		h.Log.Printf("restflex: error while writing error response: %v", errOnError)
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
		return NewAPIError(http.StatusBadRequest, cause)
	}
	return nil
}
