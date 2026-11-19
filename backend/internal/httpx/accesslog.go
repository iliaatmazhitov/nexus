package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func isProbe(path string) bool {
	return path == "/readyz" || path == "/livez"
}

// AccessLog возвращает middleware, который пишет в logger одну запись на любой HTTP-запрос, после того как он был обработан
// В запись попадает путь без query, в ней бывают токены и персональные данные
// Снаружи ставим Recoverer, чтобы запросы, завершившиеся паникой, тоже попадали в лог.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			state := &requestLog{}
			ctx := context.WithValue(r.Context(), requestLogKey{}, state)
			r = r.WithContext(ctx)

			defer func() {
				// Если хендлер ничего не записал, клиент получит 200.
				status := ww.Status()
				if status == 0 {
					status = http.StatusOK
				}

				routeCtx := chi.RouteContext(r.Context())
				route := routeCtx.RoutePattern()

				if route == "" {
					route = "UNMATCHED"
				}

				level := slog.LevelInfo
				switch {
				case isProbe(r.URL.Path):
					level = slog.LevelDebug
				case status >= 500:
					level = slog.LevelError
				}

				attrs := []slog.Attr{
					slog.String("request_id", middleware.GetReqID(r.Context())),
					slog.String("client_ip", middleware.GetClientIP(r.Context())),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("route", route),
					slog.Int("status", status),
					slog.Float64("duration_ms", float64(time.Since(start))/float64(time.Millisecond)),
					slog.Int("bytes", ww.BytesWritten()),
				}

				if state.ErrorCode != "" {
					attrs = append(attrs, slog.String("error_code", state.ErrorCode))
				}

				if status >= 500 && state.Error != nil {
					attrs = append(attrs, slog.String("error", state.Error.Error()))
				}

				logger.LogAttrs(r.Context(), level, "http request", attrs...)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
