package httptools

import (
	"net/http"
)

func Wrap(h http.Handler, mw ...Middleware) http.HandlerFunc {
	for i := range mw {
		h = mw[len(mw)-1-i](h)
	}
	return h.ServeHTTP
}
