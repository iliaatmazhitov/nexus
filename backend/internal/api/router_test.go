package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iliaatmazhitov/nexus/internal/api"
	"github.com/iliaatmazhitov/nexus/internal/health"
)

type errorBody struct {
	Code string `json:"code"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

func TestRouterErrors(t *testing.T) {
	tests := []struct {
		name               string
		method             string
		path               string
		expectedHTTPStatus int
		err                errorBody
	}{
		{
			name:               "401 me without token",
			method:             http.MethodGet,
			path:               "/api/v1/me",
			expectedHTTPStatus: http.StatusUnauthorized,
			err:                errorBody{Code: "unauthenticated"},
		},
		{
			name:               "404 wrong address",
			method:             http.MethodGet,
			path:               "/random",
			expectedHTTPStatus: http.StatusNotFound,
			err:                errorBody{Code: "not_found"},
		},
		{
			name:               "404 wrong address in right route group",
			method:             http.MethodGet,
			path:               "/api/v1/adressForTest",
			expectedHTTPStatus: http.StatusNotFound,
			err:                errorBody{Code: "not_found"},
		},
		{
			name:               "404 edge case of router settings",
			method:             http.MethodGet,
			path:               "/LIVEZ",
			expectedHTTPStatus: http.StatusNotFound,
			err:                errorBody{Code: "not_found"},
		},
		{
			name:               "404 second edge case of router settings",
			method:             http.MethodGet,
			path:               "/livez/",
			expectedHTTPStatus: http.StatusNotFound,
			err:                errorBody{Code: "not_found"},
		},
		{
			name:               "405 wrong method right address",
			method:             http.MethodPost,
			path:               "/livez",
			expectedHTTPStatus: http.StatusMethodNotAllowed,
			err:                errorBody{Code: "method_not_allowed"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			h := health.New()
			r, err := api.NewRouter(api.Dependencies{Health: h})

			if err != nil {
				t.Fatalf("error: got %v, want %v", err, nil)
			}
			r.ServeHTTP(rec, req)

			if rec.Code != tc.expectedHTTPStatus {
				t.Errorf("http status: got %d, want %d", rec.Code, tc.expectedHTTPStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("content-type: got %q, want %q", ct, "application/json")
			}

			var body errorResponse
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Error.Code != tc.err.Code {
				t.Errorf("error.code: got %q, want %q", body.Error.Code, tc.err.Code)
			}
		})
	}
}

func TestMeSetsWWWAuthenticate(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()

	h := health.New()
	r, err := api.NewRouter(api.Dependencies{Health: h})

	if err != nil {
		t.Fatalf("error: got %v, want %v", err, nil)
	}
	r.ServeHTTP(rec, req)

	hint := "Bearer"
	if auth := rec.Header().Get("WWW-Authenticate"); auth != hint {
		t.Errorf("WWW-Authenticate: got %q, want %q", auth, hint)
	}
}
