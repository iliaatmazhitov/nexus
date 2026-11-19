package httpx

import (
	"fmt"
	"net/http"
)

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				SetRequestError(r.Context(), "internal", fmt.Errorf("%v", v))
				w.WriteHeader(http.StatusInternalServerError)
			}

		}()

		next.ServeHTTP(w, r)
	})
}
