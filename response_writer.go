package rest

import "net/http"

// responseWriter stores whether response has been already written in the
// isWritten variable.
type responseWriter struct {
	http.ResponseWriter
	isWritten bool
	status    int
}

func (w *responseWriter) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
	w.status = status
	w.isWritten = true
}

func (w *responseWriter) Write(b []byte) (int, error) {
	i, err := w.ResponseWriter.Write(b)
	w.isWritten = true
	return i, err
}
