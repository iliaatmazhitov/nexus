package health

import (
	"net/http"
	"sync/atomic"

	"github.com/iliaatmazhitov/nexus/internal/httpx"
)

type Health struct {
	ready atomic.Bool
}

type statusResponse struct {
	Status string `json:"status"`
}

func New() *Health {
	return &Health{}
}

func (h *Health) SetReady(ready bool) {
	h.ready.Store(ready)
}

func (h *Health) Live(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(r.Context(), w, http.StatusOK, statusResponse{
		Status: "ok",
	})
}

func (h *Health) Ready(w http.ResponseWriter, r *http.Request) {
	if !h.ready.Load() {
		httpx.SetRequestError(r.Context(), "not_ready", nil)
		httpx.WriteError(r.Context(), w, http.StatusServiceUnavailable, "not_ready", "service is starting or shutting down")
		return
	}

	httpx.WriteJSON(r.Context(), w, http.StatusOK, statusResponse{
		Status: "ready",
	})
}
