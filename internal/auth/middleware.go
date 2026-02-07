package auth

import (
	"crypto/subtle"
	"net/http"

	"github.com/grantsy/grantsy/internal/httptools"
)

const headerName = "X-Api-Key"

func Middleware(apiKey string) httptools.Middleware {
	keyBytes := []byte(apiKey)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get(headerName)
			if provided == "" {
				httptools.Error(w, r, http.StatusUnauthorized,
					"https://grantsy.example/errors/unauthorized",
					"Unauthorized",
					"Missing API key",
				)
				return
			}

			if subtle.ConstantTimeCompare([]byte(provided), keyBytes) != 1 {
				httptools.Error(w, r, http.StatusUnauthorized,
					"https://grantsy.example/errors/unauthorized",
					"Unauthorized",
					"Invalid API key",
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
