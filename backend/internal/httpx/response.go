package httpx

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Тело ошибки
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Обертка ошибки, в json будет лежать ошибка под ключом "error"
type errorResponse struct {
	Error ErrorBody `json:"error"`
}

var fallbackInternal = []byte(`{"error":{"code":"internal","message":"internal error"}}`)

func WriteJSON(ctx context.Context, w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		SetRequestError(ctx, "internal", err)
		slog.Error("encode response", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(fallbackInternal)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if _, err := w.Write(data); err != nil {
		slog.Debug("write response", "err", err)
	}
}

// Пишем ошибку в стандартном формате API
func WriteError(ctx context.Context, w http.ResponseWriter, status int, code, message string) {
	WriteJSON(ctx, w, status, errorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
		},
	})
}
