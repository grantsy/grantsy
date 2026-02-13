package openapi

import (
	"encoding/json"
	"net/http"

	"github.com/swaggest/openapi-go/openapi31"
)

type Route struct {
	reflector *openapi31.Reflector
}

func NewRoute(reflector *openapi31.Reflector) *Route {
	return &Route{reflector: reflector}
}

func (route *Route) Register(mux *http.ServeMux, r *openapi31.Reflector) {
	mux.HandleFunc("GET /openapi.json", route.serveJSON)
	mux.HandleFunc("GET /docs", route.serveUI)
}

func (route *Route) serveJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(route.reflector.Spec)
}

func (route *Route) serveUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(swaggerUIHTML))
}

const swaggerUIHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Grantsy API Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "/openapi.json",
      dom_id: '#swagger-ui',
    })
  </script>
</body>
</html>`
