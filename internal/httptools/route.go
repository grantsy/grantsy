package httptools

import (
	"net/http"

	"github.com/swaggest/openapi-go/openapi3"
)

type Route interface {
	Register(mux *http.ServeMux, r *openapi3.Reflector)
}
