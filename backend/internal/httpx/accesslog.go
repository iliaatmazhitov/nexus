package httpx

import (
	"log/slog"
	"net/http"
	"time"

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

			defer func() {
				// Если хендлер ничего не записал, клиент получит 200.
				status := ww.Status()
				if status == 0 {
					status = http.StatusOK
				}

				level := slog.LevelInfo
				switch {
				case isProbe(r.URL.Path):
					level = slog.LevelDebug
				case status >= 500:
					level = slog.LevelError
				}

				logger.Log(r.Context(), level, "http request", "request_id", middleware.GetReqID(r.Context()), "client_ip", middleware.GetClientIP(r.Context()),
					"method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", float64(time.Since(start))/float64(time.Millisecond), "bytes", ww.BytesWritten())
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
