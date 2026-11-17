package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
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
	handler := AccessLog(logger)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me?token=secret", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v (raw: %s)", err, buf.String())
	}

	required := []string{
		"time", "level", "msg",
		"service", "env", "version",
		"request_id", "client_ip",
		"method", "path", "status",
		"duration_ms", "bytes",
	}
	for _, key := range required {
		if _, ok := entry[key]; !ok {
			t.Errorf("field %q is missing", key)
		}
	}

	if entry["service"] != "nexus" {
		t.Errorf("service: got %v, want %q", entry["service"], "nexus")
	}
	if entry["msg"] != "http request" {
		t.Errorf("msg: got %v, want %q", entry["msg"], "http request")
	}
	if entry["path"] != "/api/v1/me" {
		t.Errorf("path must not contain query: got %v", entry["path"])
	}
	if d, ok := entry["duration_ms"].(float64); !ok || d < 0 {
		t.Errorf("duration_ms: got %v, want non-negative number", entry["duration_ms"])
	}
}
