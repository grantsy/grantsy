package httptools

import (
	"net/http"

	"github.com/swaggest/openapi-go/openapi31"
)

type Route interface {
	Register(mux *http.ServeMux, r *openapi31.Reflector)
}
