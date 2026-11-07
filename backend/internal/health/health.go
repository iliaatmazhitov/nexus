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

func (h *Health) Live(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, statusResponse{
		Status: "ok",
	})
}

func (h *Health) Ready(w http.ResponseWriter, _ *http.Request) {
	if !h.ready.Load() {
		httpx.WriteError(w, http.StatusServiceUnavailable, "not_ready", "service is starting or shutting down")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, statusResponse{
		Status: "ready",
	})
}
