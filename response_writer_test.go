package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponseWriter_WriteHeader(t *testing.T) {
	t.Run("isWritten is updated on call to WriteHeader()", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		rw := &responseWriter{
			ResponseWriter: rec,
		}
		if rw.isWritten != false {
			t.Error("expecting isWritten to be false on initialization")
		}

		status := http.StatusOK
		rw.WriteHeader(status)
		_ = rec.Result()
		if rw.isWritten != true {
			t.Error("expecting isWritten to be true after calling WriteHeader()")
		}
		if rw.status != status {
			t.Errorf("expecting status %v, got %v", status, rw.status)
		}
	})
}

func TestResponseWriter_Write(t *testing.T) {
	t.Run("isWritten is updated on call to Write()", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		rw := &responseWriter{
			ResponseWriter: rec,
		}
		if rw.isWritten != false {
			t.Error("expecting isWritten to be false on initialization")
		}

		rw.Write(make([]byte, 0))
		_ = rec.Result()
		if rw.isWritten != true {
			t.Error("expecting isWritten to be true after calling Write()")
		}
	})
}
