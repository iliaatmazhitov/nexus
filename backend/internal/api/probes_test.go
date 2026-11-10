package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iliaatmazhitov/nexus/internal/api"
	"github.com/iliaatmazhitov/nexus/internal/health"
)

func TestNewRouterRequiresHealth(t *testing.T) {
	handler, err := api.NewRouter(api.Dependencies{Health: nil})

	if !errors.Is(err, api.ErrHealthRequired) {
		t.Errorf("error: got %v, want %v", err, api.ErrHealthRequired)
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
	}{
		{
			name:               "livez when not ready",
			path:               "/livez",
			isReady:            false,
			expectedHTTPStatus: http.StatusOK,
		},
		{
			name:               "readyz when not ready",
			path:               "/readyz",
			isReady:            false,
			expectedHTTPStatus: http.StatusServiceUnavailable,
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
			r, err := api.NewRouter(api.Dependencies{Health: h})

			if err != nil {
				t.Fatalf("error: got %v, want %v", err, nil)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)

			r.ServeHTTP(rec, req)

			if rec.Code != tc.expectedHTTPStatus {
				t.Errorf("http status: got %d, want %d", rec.Code, tc.expectedHTTPStatus)
			}

		})
	}
}
