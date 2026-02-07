package db

import "net/http"

func HealthCheckMiddleware(db *DB) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := db.Ping(); err != nil {
				http.Error(w, "database is down", http.StatusServiceUnavailable)
				return
			}
			if next != nil {
				next.ServeHTTP(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
		})
	}
}
