package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/grantsy/grantsy/pkg/gracefulshutdown"
)

func New(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: handler,
		BaseContext: func(_ net.Listener) context.Context {
			return gracefulshutdown.GetServerBaseContext()
		},
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
