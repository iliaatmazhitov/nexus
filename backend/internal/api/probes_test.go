package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iliaatmazhitov/nexus/internal/api"
	"github.com/iliaatmazhitov/nexus/internal/health"
)

func TestNewRouterRequiresHealth(t *testing.T) {
	handler, err := api.NewRouter(api.Dependencies{Health: nil, Logger: slog.New(slog.DiscardHandler)})

	if !errors.Is(err, api.ErrHealthRequired) {
		t.Errorf("error: got %v, want %v", err, api.ErrHealthRequired)
	}

	if handler != nil {
		t.Errorf("handler: got %v, want nil", handler)
	}
}

func TestNewRouterRequiresLogger(t *testing.T) {
	handler, err := api.NewRouter(api.Dependencies{Health: health.New(), Logger: nil})

	if !errors.Is(err, api.ErrLoggerRequired) {
		t.Errorf("error: got %v, want %v", err, api.ErrLoggerRequired)
	}

	if handler != nil {
		t.Errorf("handler: got %v, want nil", handler)
	}
}

func TestProbesAreRouted(t *testing.T) {
	tests := []struct {
		name               string
		path               string
		isReady            bool
		expectedHTTPStatus int
		expectedErrCode    string
	}{
		{
			name:               "livez when not ready",
			path:               "/livez",
			isReady:            false,
			expectedHTTPStatus: http.StatusOK,
		},
		{
			name:               "livez when ready",
			path:               "/livez",
			isReady:            true,
			expectedHTTPStatus: http.StatusOK,
		},
		{
			name:               "readyz when not ready",
			path:               "/readyz",
			isReady:            false,
			expectedHTTPStatus: http.StatusServiceUnavailable,
			expectedErrCode:    "not_ready",
		},
		{
			name:               "readyz when ready",
			path:               "/readyz",
			isReady:            true,
			expectedHTTPStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := health.New()
			h.SetReady(tc.isReady)
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			r, err := api.NewRouter(api.Dependencies{Health: h, Logger: logger})

			if err != nil {
				t.Fatalf("error: got %v, want %v", err, nil)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)

			r.ServeHTTP(rec, req)

			if rec.Code != tc.expectedHTTPStatus {
				t.Errorf("http status: got %d, want %d", rec.Code, tc.expectedHTTPStatus)
			}

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("decode log entry: %v (raw: %s)", err, buf.String())
			}
			if entry["level"] != "DEBUG" {
				t.Errorf("level: got %v, want DEBUG", entry["level"])
			}
			if entry["status"] != float64(tc.expectedHTTPStatus) {
				t.Errorf("logged status: got %v, want %d", entry["status"], tc.expectedHTTPStatus)
			}
			if entry["route"] != tc.path {
				t.Errorf("route: got %v, want %q", entry["route"], tc.path)
			}
			if tc.expectedErrCode == "" {
				if code, ok := entry["error_code"]; ok {
					t.Errorf("error_code must be absent: got %v", code)
				}
			} else {
				var body errorResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if body.Error.Code != tc.expectedErrCode {
					t.Errorf("response error.code: got %q, want %q", body.Error.Code, tc.expectedErrCode)
				}
				if entry["error_code"] != body.Error.Code {
					t.Errorf("logged error_code: got %v, want %q", entry["error_code"], body.Error.Code)
				}
			}
			if reason, ok := entry["error"]; ok {
				t.Errorf("error must be absent for probes without an internal cause: got %v", reason)
			}
		})
	}
}

func TestProbeAccessLogsAreFilteredAtInfo(t *testing.T) {
	for _, path := range []string{"/livez", "/readyz"} {
		t.Run(path, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			router, err := api.NewRouter(api.Dependencies{Health: health.New(), Logger: logger})
			if err != nil {
				t.Fatal(err)
			}
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
			if buf.Len() != 0 {
				t.Errorf("probe must not log at Info threshold: %s", buf.String())
			}
		})
	}
}
