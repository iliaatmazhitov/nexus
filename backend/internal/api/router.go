package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/iliaatmazhitov/nexus/internal/health"
	"github.com/iliaatmazhitov/nexus/internal/httpx"
)

var (
	ErrHealthRequired = errors.New("api: Health is required")
	ErrLoggerRequired = errors.New("api: Logger is required")
)

// Dependencies: тут будет все, что нужно роутеру извне
type Dependencies struct {
	Health *health.Health
	Logger *slog.Logger
}

// Собирает http роутинг приложения
func NewRouter(d Dependencies) (http.Handler, error) {
	if d.Health == nil {
		return nil, ErrHealthRequired
	}

	if d.Logger == nil {
		return nil, ErrLoggerRequired
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.ClientIPFromRemoteAddr, httpx.AccessLog(d.Logger), httpx.Recoverer)

	r.NotFound(handleNotFound)
	r.MethodNotAllowed(handleMethodNotAllowed)

	r.Get("/livez", d.Health.Live)
	r.Get("/readyz", d.Health.Ready)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/me", handleMe) // mock
	})

	return r, nil
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	httpx.SetRequestError(r.Context(), "not_found", nil)
	httpx.WriteError(r.Context(), w, http.StatusNotFound, "not_found", "route not found")
}

func handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	httpx.SetRequestError(r.Context(), "method_not_allowed", nil)
	httpx.WriteError(r.Context(), w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}

// mock func
func handleMe(w http.ResponseWriter, r *http.Request) {
	httpx.SetRequestError(r.Context(), "unauthenticated", nil)
	w.Header().Set("WWW-Authenticate", "Bearer")
	httpx.WriteError(r.Context(), w, http.StatusUnauthorized, "unauthenticated", "missing or invalid token")
}
