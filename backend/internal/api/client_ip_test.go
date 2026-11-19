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

// IP клиента берётся из TCP-соединения (RemoteAddr), а не из заголовков,
// которые клиент может подделать. Тест ловит возврат к RealIP
// или неправильный порядок middleware в роутере.
func TestClientIPIgnoresSpoofedHeaders(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	r, err := api.NewRouter(api.Dependencies{Health: health.New(), Logger: logger})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil) // RemoteAddr = 192.0.2.1:1234
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	req.Header.Set("X-Real-IP", "203.0.113.8")
	req.Header.Set("True-Client-IP", "203.0.113.9")

	r.ServeHTTP(httptest.NewRecorder(), req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v (raw: %s)", err, buf.String())
	}

	if entry["client_ip"] != "192.0.2.1" {
		t.Errorf("client_ip: got %v, want %q", entry["client_ip"], "192.0.2.1")
	}
}
