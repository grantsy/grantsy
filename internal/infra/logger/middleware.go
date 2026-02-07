package logger

import (
	"bytes"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-http-utils/headers"
	"github.com/grantsy/grantsy/internal/infra/tracing"
	"github.com/zenazn/goji/web/mutil"
)

// Middleware creates HTTP logging middleware
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create request-scoped logger with request data
		log := slog.Default().With(
			slog.Group("request",
				slog.String("method", r.Method),
				slog.String("url", r.URL.String()),
				slog.String("ip", r.RemoteAddr),
				slog.Any("headers", r.Header),
				slog.String("request_id", tracing.GetRequestID(r.Context())),
			),
		)

		// Store logger in context
		ctx := WithLogger(r.Context(), log)
		r = r.WithContext(ctx)

		// Wrap response writer to capture status and size
		lw := mutil.WrapWriter(w)
		buf := bytes.NewBuffer(nil)
		lw.Tee(buf)

		// Serve request
		next.ServeHTTP(lw, r)

		// Log response
		duration := time.Since(start)
		status := lw.Status()
		size := lw.BytesWritten()
		body := buf.String()

		responseDataHandlerCallback(w, r, status, size, duration, body)
	})
}

func responseDataHandlerCallback(
	w http.ResponseWriter,
	r *http.Request,
	status, size int,
	duration time.Duration,
	body string,
) {
	var msg string
	var level slog.Level

	if status >= 500 {
		msg = "server error"
		level = slog.LevelError
	} else if status >= 400 {
		msg = "client error"
		level = slog.LevelInfo
	} else {
		msg = "success"
		level = slog.LevelInfo
	}

	log := FromContext(r.Context())

	// Build response attributes
	responseAttrs := []any{
		slog.Int("status", status),
		slog.Int("size", size),
		slog.Duration("duration", duration),
		slog.Any("headers", w.Header()),
	}

	// Include body at debug level
	if log.Enabled(r.Context(), slog.LevelDebug) {
		switch w.Header().Get(headers.ContentType) {
		case "application/json":
			responseAttrs = append(responseAttrs, slog.String("body", body))
		case "application/zip":
			responseAttrs = append(responseAttrs, slog.String("body", "<zip file>"))
		default:
			responseAttrs = append(responseAttrs, slog.String("body", body))
		}
	}

	log.Log(r.Context(), level, msg, slog.Group("response", responseAttrs...))
}

// RecoveryMiddleware creates panic recovery middleware
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log := FromContext(r.Context())
				log.Error("panic recovered",
					"error", rec,
					"stack", string(debug.Stack()),
				)
				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
