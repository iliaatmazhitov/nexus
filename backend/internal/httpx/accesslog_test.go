package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func TestAccessLog(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		handlerStatus int
		wantLevel     string
	}{
		{
			name:          "обычный запрос пишется как info",
			path:          "/api/v1/me",
			handlerStatus: http.StatusOK,
			wantLevel:     "INFO",
		},
		{
			name:          "пятисотка пишется как error",
			path:          "/api/v1/me",
			handlerStatus: http.StatusInternalServerError,
			wantLevel:     "ERROR",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.handlerStatus)
			})

			handler := AccessLog(logger)(next)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("decode log entry: %v (raw: %s)", err, buf.String())
			}

			if entry["level"] != tc.wantLevel {
				t.Errorf("level: got %v, want %q", entry["level"], tc.wantLevel)
			}

			if entry["method"] != http.MethodGet {
				t.Errorf("method: got %v, want %q", entry["method"], http.MethodGet)
			}

			if entry["path"] != tc.path {
				t.Errorf("path: got %v, want %q", entry["path"], tc.path)
			}

			if entry["status"] != float64(tc.handlerStatus) {
				t.Errorf("status: got %v, want %d", entry["status"], tc.handlerStatus)
			}

			if _, ok := entry["duration_ms"]; !ok {
				t.Error("duration_ms: field is missing")
			}
		})
	}
}

func TestAccessLogSchema(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil)).With("service", "nexus", "env", "test", "version", "0.0.1")

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.ClientIPFromRemoteAddr, AccessLog(logger))
	router.Get("/api/v1/me", next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me?token=query-secret", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Request-Id", "schema-request-id")
	req.Header.Set("X-Forwarded-For", "198.51.100.99")
	req.Header.Set("Authorization", "Bearer auth-secret")
	req.Header.Set("Cookie", "session=cookie-secret")
	router.ServeHTTP(httptest.NewRecorder(), req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v (raw: %s)", err, buf.String())
	}

	required := []string{
		"time", "level", "msg",
		"service", "env", "version",
		"request_id", "client_ip",
		"method", "path", "route", "status",
		"duration_ms", "bytes",
	}
	for _, key := range required {
		if _, ok := entry[key]; !ok {
			t.Errorf("field %q is missing", key)
		}
	}

	for _, key := range []string{"time", "level", "msg", "service", "env", "version", "request_id", "client_ip", "method", "path", "route"} {
		if _, ok := entry[key].(string); !ok {
			t.Errorf("%s: got %T, want string", key, entry[key])
		}
	}
	if timestamp, ok := entry["time"].(string); ok {
		if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil {
			t.Errorf("time: %q is not RFC3339: %v", timestamp, err)
		}
	}
	for key, want := range map[string]any{
		"service": "nexus", "env": "test", "version": "0.0.1",
		"msg": "http request", "level": "INFO", "method": http.MethodGet,
		"request_id": "schema-request-id", "client_ip": "192.0.2.10",
		"path": "/api/v1/me", "route": "/api/v1/me",
		"status": float64(http.StatusOK), "bytes": float64(0),
	} {
		if entry[key] != want {
			t.Errorf("%s: got %v, want %v", key, entry[key], want)
		}
	}
	for _, key := range []string{"status", "bytes"} {
		n, ok := entry[key].(float64)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || math.Trunc(n) != n {
			t.Errorf("%s: got %v, want non-negative integer", key, entry[key])
		}
	}
	if d, ok := entry["duration_ms"].(float64); !ok || math.IsNaN(d) || math.IsInf(d, 0) || d < 0 {
		t.Errorf("duration_ms: got %v, want non-negative number", entry["duration_ms"])
	}
	for _, key := range []string{"error_code", "error"} {
		if value, ok := entry[key]; ok {
			t.Errorf("%s must be absent for success: got %v", key, value)
		}
	}
	for _, secret := range []string{"query-secret", "auth-secret", "cookie-secret"} {
		if bytes.Contains(buf.Bytes(), []byte(secret)) {
			t.Errorf("log contains secret %q", secret)
		}
	}
}

func TestAccessLogRoute(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantPath   string
		wantRoute  string
	}{
		{
			name:       "полный шаблон вложенного маршрута",
			target:     "/api/v1/channels/42",
			wantStatus: http.StatusOK,
			wantPath:   "/api/v1/channels/42",
			wantRoute:  "/api/v1/channels/{id}",
		},
		{
			name:       "другой id не меняет шаблон",
			target:     "/api/v1/channels/99",
			wantStatus: http.StatusOK,
			wantPath:   "/api/v1/channels/99",
			wantRoute:  "/api/v1/channels/{id}",
		},
		{
			name:       "query отсутствует в пути и шаблоне",
			target:     "/api/v1/channels/42?token=secret",
			wantStatus: http.StatusOK,
			wantPath:   "/api/v1/channels/42",
			wantRoute:  "/api/v1/channels/{id}",
		},
		{
			name:       "неизвестный маршрут",
			target:     "/does-not-exist",
			wantStatus: http.StatusNotFound,
			wantPath:   "/does-not-exist",
			wantRoute:  "UNMATCHED",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			router := chi.NewRouter()
			router.Use(AccessLog(logger))
			router.Route("/api/v1", func(r chi.Router) {
				r.Get("/channels/{id}", func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
			})

			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("http status: got %d, want %d", rec.Code, tc.wantStatus)
			}

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("decode log entry: %v (raw: %s)", err, buf.String())
			}

			if entry["path"] != tc.wantPath {
				t.Errorf("path: got %v, want %q", entry["path"], tc.wantPath)
			}
			if entry["route"] != tc.wantRoute {
				t.Errorf("route: got %v, want %q", entry["route"], tc.wantRoute)
			}
			if entry["status"] != float64(tc.wantStatus) {
				t.Errorf("logged status: got %v, want %d", entry["status"], tc.wantStatus)
			}
		})
	}
}
