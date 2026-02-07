package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/zenazn/goji/web/mutil"
)

// Middleware returns HTTP middleware that records request metrics.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status
		lw := mutil.WrapWriter(w)

		// Serve request
		next.ServeHTTP(lw, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(lw.Status())
		path := r.Pattern

		recordHTTPRequest(r.Method, path, status)
		recordHTTPDuration(r.Method, path, status, duration)
	})
}
