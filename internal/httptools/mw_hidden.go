package httptools

import "net/http"

func Hidden(
	pass func(r *http.Request) bool,
	statusCode int,
) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !pass(r) {
				w.WriteHeader(statusCode)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
