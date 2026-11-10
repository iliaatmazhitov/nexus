package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/iliaatmazhitov/nexus/internal/health"
)

func TestLiveOk(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)

	h := health.New()
	h.Live(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: got %q, want %q", ct, "application/json")
	}

	var body struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("status: got %q, want %q", body.Status, "ok")
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name               string
		isReady            bool
		expectedHTTPStatus int
		expectedStatus     string
		expectedCode       string
	}{
		{
			name:               "ready",
			isReady:            true,
			expectedHTTPStatus: http.StatusOK,
			expectedStatus:     "ready",
			expectedCode:       "",
		},
		{
			name:               "not ready",
			isReady:            false,
			expectedHTTPStatus: http.StatusServiceUnavailable,
			expectedStatus:     "",
			expectedCode:       "not_ready",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := health.New()
			h.SetReady(tc.isReady)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

			h.Ready(rec, req)

			if rec.Code != tc.expectedHTTPStatus {
				t.Errorf("http status: got %d, want %d", rec.Code, tc.expectedHTTPStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("content-type: got %q, want %q", ct, "application/json")
			}

			var body struct {
				Status string `json:"status"`
				Error  struct {
					Code string `json:"code"`
				} `json:"error"`
			}

			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Status != tc.expectedStatus {
				t.Errorf("status: got %q, want %q", body.Status, tc.expectedStatus)
			}
			if body.Error.Code != tc.expectedCode {
				t.Errorf("error.code: got %q, want %q", body.Error.Code, tc.expectedCode)
			}
		})
	}
}

// Тест ничего не утверждает, он нужен нам для запуска с -race
func TestReadyConcurrent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	h := health.New()
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			if i%2 == 0 {
				h.SetReady(true)
			}
			h.SetReady(false)
		}
	}()

	for i := 0; i < 100; i++ {
		rec := httptest.NewRecorder()
		h.Ready(rec, req)
	}

	wg.Wait()
}
