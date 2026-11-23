package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/iliaatmazhitov/nexus/internal/httpx"
)

func TestAccessLogRequestError(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		code       string
		cause      error
		wantLevel  string
		wantReason string
	}{
		{
			name:      "успешный запрос без полей ошибки",
			status:    http.StatusOK,
			wantLevel: "INFO",
		},
		{
			name:      "404 с кодом без внутренней причины",
			status:    http.StatusNotFound,
			code:      "not_found",
			wantLevel: "INFO",
		},
		{
			name:       "500 с кодом и внутренней причиной",
			status:     http.StatusInternalServerError,
			code:       "internal",
			cause:      errors.New("storage unavailable"),
			wantLevel:  "ERROR",
			wantReason: "storage unavailable",
		},
		{
			name:      "500 без причины не добавляет пустое поле error",
			status:    http.StatusInternalServerError,
			code:      "internal",
			wantLevel: "ERROR",
		},
		{
			name:      "причина 4xx не попадает в лог",
			status:    http.StatusNotFound,
			code:      "not_found",
			cause:     errors.New("channel does not exist"),
			wantLevel: "INFO",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil)).With(
				"service", "nexus", "env", "test", "version", "test-commit",
			)
			router := chi.NewRouter()
			router.Use(httpx.AccessLog(logger))
			router.Get("/channels/{id}", func(w http.ResponseWriter, r *http.Request) {
				if tc.code != "" {
					httpx.SetRequestError(r.Context(), tc.code, tc.cause)
					httpx.WriteError(r.Context(), w, tc.status, tc.code, "request failed")
					return
				}
				w.WriteHeader(tc.status)
			})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/channels/42", nil))
			if rec.Code != tc.status {
				t.Errorf("http status: got %d, want %d", rec.Code, tc.status)
			}

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("decode log entry: %v (raw: %s)", err, buf.String())
			}
			for key, want := range map[string]any{
				"msg": "http request", "level": tc.wantLevel,
				"status": float64(tc.status), "route": "/channels/{id}",
				"version": "test-commit",
			} {
				if entry[key] != want {
					t.Errorf("%s: got %v, want %v", key, entry[key], want)
				}
			}

			code, hasCode := entry["error_code"]
			if tc.code == "" {
				if hasCode {
					t.Errorf("error_code must be absent: got %v", code)
				}
			} else {
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if body.Error.Code != tc.code {
					t.Errorf("response error.code: got %q, want %q", body.Error.Code, tc.code)
				}
				if !hasCode || code != body.Error.Code {
					t.Errorf("logged error_code: got %v, want response code %q", code, body.Error.Code)
				}
			}

			reason, hasReason := entry["error"]
			if tc.wantReason == "" {
				if hasReason {
					t.Errorf("error must be absent: got %v", reason)
				}
			} else if !hasReason || reason != tc.wantReason {
				t.Errorf("error: got %v, want %q", reason, tc.wantReason)
			}
			if tc.cause != nil && bytes.Contains(rec.Body.Bytes(), []byte(tc.cause.Error())) {
				t.Error("response must not expose the internal cause")
			}
		})
	}
}

func TestSetRequestErrorWithoutAccessLog(t *testing.T) {
	// Обработчик может использоваться без middleware, например в отдельном тесте.
	// Отсутствие состояния в контексте не должно вызывать панику.
	httpx.SetRequestError(context.Background(), "internal", errors.New("storage unavailable"))
}

func TestWriteJSONEncodeFailureAccessLog(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		reason string
	}{
		{
			name:   "NaN",
			value:  math.NaN(),
			reason: "json: unsupported value: NaN",
		},
		{
			name:   "channel",
			value:  make(chan int),
			reason: "json: unsupported type: chan int",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil)).With("version", "test-commit")
			router := chi.NewRouter()
			router.Use(httpx.AccessLog(logger))
			router.Get("/encode", func(w http.ResponseWriter, r *http.Request) {
				httpx.WriteJSON(r.Context(), w, http.StatusOK, tc.value)
			})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/encode", nil))
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status: got %d, want 500", rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("content-type: got %q, want application/json", got)
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error.Code != "internal" || body.Error.Message != "internal error" {
				t.Errorf("unexpected client error: %+v", body.Error)
			}
			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("decode access log: %v (raw: %s)", err, buf.String())
			}
			for key, want := range map[string]any{
				"msg": "http request", "level": "ERROR", "status": float64(500),
				"route": "/encode", "version": "test-commit",
				"error_code": body.Error.Code, "error": tc.reason,
				"bytes": float64(rec.Body.Len()),
			} {
				if entry[key] != want {
					t.Errorf("%s: got %v, want %v", key, entry[key], want)
				}
			}
		})
	}
}

func TestAccessLogRequestErrorsAreIsolated(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	handler := httpx.AccessLog(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fail") == "1" {
			httpx.SetRequestError(r.Context(), "internal", errors.New("storage unavailable"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))

	for _, target := range []string{"/test?fail=1", "/test"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))
	}
	decoder := json.NewDecoder(&buf)
	var failed, success map[string]any
	if err := decoder.Decode(&failed); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&success); err != nil {
		t.Fatal(err)
	}
	if failed["error_code"] != "internal" || failed["error"] != "storage unavailable" {
		t.Errorf("first request lost its error: %v", failed)
	}
	for _, key := range []string{"error_code", "error"} {
		if value, ok := success[key]; ok {
			t.Errorf("second request inherited %s: %v", key, value)
		}
	}
	if success["status"] != float64(200) || success["bytes"] != float64(2) {
		t.Errorf("unexpected second response metrics: %v", success)
	}
}

func TestAccessLogEmptyHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	handler := httpx.AccessLog(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/empty", nil))
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || entry["status"] != float64(200) || entry["bytes"] != float64(0) {
		t.Errorf("empty handler: http=%d log=%v", rec.Code, entry)
	}
}

func TestAccessLogRecoveredPanicStatus(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, nil)).With("version", "test-commit")
	router := chi.NewRouter()
	router.Use(httpx.AccessLog(logger), httpx.Recoverer)

	router.Get("/panic", func(http.ResponseWriter, *http.Request) {
		panic("storage unavailable")
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("decode access log: %v (raw: %s)", err, buf.String())
	}

	for key, want := range map[string]any{
		"msg": "http request", "status": float64(500), "level": "ERROR",
		"route": "/panic", "version": "test-commit",
		"error_code": "internal",
		"error":      "storage unavailable",
	} {
		if entry[key] != want {
			t.Errorf("%s: got %v, want %v", key, entry[key], want)
		}
	}
}
