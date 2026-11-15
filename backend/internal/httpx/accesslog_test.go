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
		{
			name:          "503 на пробе остаётся info",
			path:          "/readyz",
			handlerStatus: http.StatusServiceUnavailable,
			wantLevel:     "INFO",
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

			if _, ok := entry["duration"]; !ok {
				t.Error("duration: field is missing")
			}
		})
	}
}
