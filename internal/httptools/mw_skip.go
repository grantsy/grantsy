package httptools

import (
	"net/http"
	"path"
)

func Skip(
	mw Middleware,
	forPaths ...string,
) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var matched bool
			for _, p := range forPaths {
				matched, _ = path.Match(p, r.URL.Path)
				if matched {
					break
				}
			}
			if matched {
				next.ServeHTTP(w, r)
				return
			}
			mw(next).ServeHTTP(w, r)
		})
	}
}
