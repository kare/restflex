//go:build !integration

package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"kkn.fi/infra"
	"kkn.fi/rest"
)

func TestMain(m *testing.M) {
	if infra.IsCI() {
		log.SetOutput(io.Discard)
	}
	os.Exit(m.Run())
}

func Test_default_response_is_HTTP_501(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		handler     rest.HandlerFunc
		wantStatus  int
		expectedErr string
	}{
		{
			name:   "default response is 501 not implemented",
			method: http.MethodGet,
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
				return nil
			},
			wantStatus: http.StatusNotImplemented,
		},
		{
			name:   "default response can be overridden by writing response status header",
			method: http.MethodGet,
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "default response can be overridden by writing response body",
			method: http.MethodGet,
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
				_, _ = w.Write([]byte(`{"ok":true}`))
				return nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "on error no default response",
			method: http.MethodGet,
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
				return errors.New("mocked error in test case")
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:   "on API error no default response",
			method: http.MethodGet,
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
				return rest.NewBadRequest("something went wrong")
			},
			wantStatus:  http.StatusBadRequest,
			expectedErr: "something went wrong",
		},
		{
			name:   "on API error with cause",
			method: http.MethodGet,
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
				return rest.NewAPIError(http.StatusInternalServerError, errors.New("test server error"), "custom error message")
			},
			wantStatus:  http.StatusInternalServerError,
			expectedErr: "custom error message",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tt.method, "/", nil)
			rec := httptest.NewRecorder()
			srv := rest.NewAPI(log.Default())
			srv.APIHandler = tt.handler
			srv.ServeHTTP(rec, req)

			res := rec.Result()
			expectedStatusCode := tt.wantStatus
			if res.StatusCode != expectedStatusCode {
				t.Errorf("expected status code %d, but got %d", expectedStatusCode, res.StatusCode)
			}
			var response rest.ErrorMessage
			if err := json.NewDecoder(res.Body).Decode(&response); err != nil && err != io.EOF {
				t.Errorf("HTTP response JSON decoding error: %v", err)
			}
			if tt.expectedErr != "" {
				rerr := response.Errors[0]
				if rerr != tt.expectedErr {
					t.Errorf("expected error message '%v', but got '%v'", tt.expectedErr, rerr)
				}
			}
		})
	}
}

func Test_default_response_is_501_not_implemented(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv := rest.NewAPI(log.Default())
	srv.APIHandler = rest.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		return nil
	})
	srv.ServeHTTP(rec, req)

	res := rec.Result()
	expectedStatusCode := http.StatusNotImplemented
	if res.StatusCode != expectedStatusCode {
		t.Errorf("expected status code %d, but got %d", expectedStatusCode, res.StatusCode)
	}
	var response rest.ErrorMessage
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil && err != io.EOF {
		t.Errorf("HTTP response JSON decoding error: %v", err)
	}
}

func Test_request_with_body_has_JSON_content_type(t *testing.T) {
	tests := []struct {
		name               string
		method             string
		requestContentType string
		wantStatus         int
	}{
		{
			name:       "POST without content type",
			method:     http.MethodPost,
			wantStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:       "PUT without content type",
			method:     http.MethodPut,
			wantStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:       "PATCH without content type",
			method:     http.MethodPatch,
			wantStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:               "POST with content type",
			method:             http.MethodPost,
			requestContentType: "application/json",
			wantStatus:         http.StatusOK,
		},
		{
			name:               "POST with content type and charset",
			method:             http.MethodPost,
			requestContentType: "application/json; charset=utf-8",
			wantStatus:         http.StatusOK,
		},
		{
			name:               "PUT with content type",
			method:             http.MethodPut,
			requestContentType: "application/json",
			wantStatus:         http.StatusOK,
		},
		{
			name:               "PATCH with content type",
			method:             http.MethodPatch,
			requestContentType: "application/json",
			wantStatus:         http.StatusOK,
		},
		{
			name:       "GET without content type",
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
		},
		{
			name:               "GET with content type",
			method:             http.MethodGet,
			requestContentType: "application/json",
			wantStatus:         http.StatusOK,
		},
		{
			name:               "POST with POST Form",
			method:             http.MethodPost,
			requestContentType: "application/x-www-form-urlencoded",
			wantStatus:         http.StatusOK,
		},
		{
			name:               "PUT with POST Form",
			method:             http.MethodPut,
			requestContentType: "application/x-www-form-urlencoded",
			wantStatus:         http.StatusOK,
		},
		{
			name:               "PATCH with POST Form",
			method:             http.MethodPatch,
			requestContentType: "application/x-www-form-urlencoded",
			wantStatus:         http.StatusOK,
		},
	}
	for _, tt := range tests {
		tt := tt
		type request struct {
			URL string `json:"url"`
		}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, w := io.Pipe()
			go func() {
				if strings.HasPrefix(tt.requestContentType, "application/json") {
					data := &request{
						URL: "https://example.com",
					}
					if err := w.CloseWithError(json.NewEncoder(w).Encode(data)); err != nil {
						t.Errorf("pipe close error: %v", err)
					}
				} else {
					data := url.Values{
						"URL": {"https://example.com"},
					}
					reader := strings.NewReader(data.Encode())
					_, _ = io.Copy(w, reader)
					if err := w.Close(); err != nil {
						t.Errorf("pipe close error: %v", err)
					}
				}
			}()
			defer func(r io.Closer) {
				if err := r.Close(); err != nil {
					t.Errorf("error while closing io.Closer: %v", err)
				}
			}(r)

			req := httptest.NewRequest(tt.method, "/", r)
			if tt.requestContentType != "" {
				req.Header.Set("Content-Type", tt.requestContentType)
			}
			rec := httptest.NewRecorder()
			srv := rest.NewAPI(log.Default())
			srv.APIHandler = rest.HandlerFunc(
				func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
					expectedURL := "https://example.com"
					if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
						data := &struct {
							URL string `json:"url"`
						}{}
						if err := json.NewDecoder(r.Body).Decode(data); err != nil {
							t.Errorf("error while reading request JSON body: %v", err)
						}
						if data.URL != expectedURL {
							t.Errorf("expecting %q, got %q", expectedURL, data.URL)
						}
					} else {
						if r.Method != http.MethodGet {
							if err := r.ParseForm(); err != nil {
								t.Errorf("error while parsing POST Form: %v", err)
							}
							if u := r.PostFormValue("URL"); u != expectedURL {
								t.Errorf("expecting %q, got %q", expectedURL, u)
							}
						} else {
							body, err := io.ReadAll(r.Body)
							if err != nil {
								t.Errorf("error while reading POST Form request body: %v", err)
							}
							values, err := url.ParseQuery(string(body))
							if err != nil {
								t.Errorf("error while parsing POST Form query: %v", err)
							}
							if u := values.Get("URL"); u != expectedURL {
								t.Errorf("expecting %q, got %q\n%v", expectedURL, u, values)
							}
						}
					}
					w.WriteHeader(http.StatusOK)
					return nil
				})
			srv.ServeHTTP(rec, req)

			res := rec.Result()
			expectedStatusCode := tt.wantStatus
			if res.StatusCode != expectedStatusCode {
				t.Errorf("expected status code %d, but got %d", expectedStatusCode, res.StatusCode)
			}
			var response rest.ErrorMessage
			if err := json.NewDecoder(res.Body).Decode(&response); err != nil && err != io.EOF {
				t.Errorf("HTTP response JSON decoding error: %v", err)
			}
		})
	}
}
