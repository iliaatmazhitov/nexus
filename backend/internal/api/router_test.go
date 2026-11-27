package api_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
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

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()

	r, err := api.NewRouter(api.Dependencies{Health: health.New(), Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}

	return r
}

func TestRouterErrors(t *testing.T) {
	tests := []struct {
		name               string
		method             string
		path               string
		expectedHTTPStatus int
		expectedErrCode    string
	}{
		{
			name:               "401 me without token",
			method:             http.MethodGet,
			path:               "/api/v1/me",
			expectedHTTPStatus: http.StatusUnauthorized,
			expectedErrCode:    "unauthenticated",
		},
		{
			name:               "404 wrong address",
			method:             http.MethodGet,
			path:               "/random",
			expectedHTTPStatus: http.StatusNotFound,
			expectedErrCode:    "not_found",
		},
		{
			name:               "404 wrong address in right route group",
			method:             http.MethodGet,
			path:               "/api/v1/addressForTest",
			expectedHTTPStatus: http.StatusNotFound,
			expectedErrCode:    "not_found",
		},
		{
			name:               "404 edge case of router settings",
			method:             http.MethodGet,
			path:               "/LIVEZ",
			expectedHTTPStatus: http.StatusNotFound,
			expectedErrCode:    "not_found",
		},
		{
			name:               "404 second edge case of router settings",
			method:             http.MethodGet,
			path:               "/livez/",
			expectedHTTPStatus: http.StatusNotFound,
			expectedErrCode:    "not_found",
		},
		{
			name:               "405 wrong method right address",
			method:             http.MethodPost,
			path:               "/livez",
			expectedHTTPStatus: http.StatusMethodNotAllowed,
			expectedErrCode:    "method_not_allowed",
		},
	}

	r := newTestRouter(t)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

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

			if body.Error.Code != tc.expectedErrCode {
				t.Errorf("error.code: got %q, want %q", body.Error.Code, tc.expectedErrCode)
			}
		})
	}
}

func TestMeSetsWWWAuthenticate(t *testing.T) {
	r := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	want := "Bearer"
	if got := rec.Header().Get("WWW-Authenticate"); got != want {
		t.Errorf("WWW-Authenticate: got %q, want %q", got, want)
	}
}

func TestRouterErrorAccessLog(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		target    string
		status    int
		code      string
		wantRoute string
		wantLevel string
	}{
		{
			name:      "unauthenticated",
			method:    http.MethodGet,
			target:    "/api/v1/me",
			status:    401,
			code:      "unauthenticated",
			wantRoute: "/api/v1/me",
			wantLevel: "INFO",
		},
		{
			name:      "not found",
			method:    http.MethodGet,
			target:    "/random?token=query-secret",
			status:    404,
			code:      "not_found",
			wantRoute: "UNMATCHED",
			wantLevel: "INFO",
		},
		{
			name:      "method not allowed",
			method:    http.MethodPost,
			target:    "/livez",
			status:    405,
			code:      "method_not_allowed",
			wantRoute: "UNMATCHED",
			wantLevel: "DEBUG",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			router, err := api.NewRouter(api.Dependencies{Health: health.New(), Logger: logger})
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.target, nil)
			req.RemoteAddr = "192.0.2.10:1234"
			req.Header.Set("X-Forwarded-For", "198.51.100.99")
			req.Header.Set("X-Real-IP", "198.51.100.98")
			router.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Errorf("http status: got %d, want %d", rec.Code, tc.status)
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error.Code != tc.code {
				t.Errorf("response error.code: got %q, want %q", body.Error.Code, tc.code)
			}
			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("decode log: %v (raw: %s)", err, buf.String())
			}
			for key, want := range map[string]any{
				"status": float64(tc.status), "error_code": body.Error.Code,
				"path": req.URL.Path, "route": tc.wantRoute, "level": tc.wantLevel,
				"bytes":     float64(rec.Body.Len()),
				"client_ip": "192.0.2.10",
			} {
				if entry[key] != want {
					t.Errorf("%s: got %v, want %v", key, entry[key], want)
				}
			}
			if id, ok := entry["request_id"].(string); !ok || id == "" {
				t.Errorf("request_id: got %v, want non-empty string", entry["request_id"])
			}
			if reason, ok := entry["error"]; ok {
				t.Errorf("error must be absent without internal cause: got %v", reason)
			}
			if bytes.Contains(buf.Bytes(), []byte("query-secret")) {
				t.Error("query secret appeared in log")
			}
		})
	}
}
