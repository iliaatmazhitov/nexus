package httpx

import (
	"log/slog"
	"net/http"
)

// AccessLog возвращает middleware, который пишет в logger одну запись на любой HTTP-запрос, после того как он был обработан
// В запись попадает путь без query, в ней бывают токены и персональные данные
// Снаружи ставим Recoverer, чтобы запросы, завершившиеся паникой, тоже попадали в лог.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer logger.Info("http request", "method", r.Method, "path", r.URL.Path) // defer если в ServeHTTP пролетит паника
			next.ServeHTTP(w, r)
		})
	}
}
