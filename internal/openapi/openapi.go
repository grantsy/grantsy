package openapi

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"

	"github.com/grantsy/grantsy/internal/httptools"
)

// WrappedResponse is a generic wrapper for API responses matching httptools.JSON format.
type WrappedResponse[T any] struct {
	Data T              `json:"data"`
	Meta httptools.Meta `json:"meta"`
}

// NewReflector creates a configured OpenAPI reflector with API info and security.
func NewReflector() *openapi3.Reflector {
	r := openapi3.NewReflector()
	r.Spec.Info.
		WithTitle("Grantsy Entitlements API").
		WithVersion("1.0.0").
		WithDescription("Microservice for managing SaaS feature entitlements")

	r.Spec.SetAPIKeySecurity("ApiKeyAuth", "X-Api-Key", "header", "API key for authentication")

	// Strip package prefix from schema names (e.g., "EntitlementsCheckResult" -> "CheckResult")
	r.JSONSchemaReflector().InterceptDefName(func(t reflect.Type, defaultDefName string) string {
		// Remove package prefix (e.g., "Entitlements", "Httptools", "Subscriptions")
		prefixes := []string{"Entitlements", "Httptools", "Subscriptions"}
		for _, prefix := range prefixes {
			if strings.HasPrefix(defaultDefName, prefix) {
				return strings.TrimPrefix(defaultDefName, prefix)
			}
		}
		return defaultDefName
	})

	return r
}

// AddErrorResponses adds common error response types to an operation.
func AddErrorResponses(op openapi.OperationContext) {
	op.AddRespStructure(new(httptools.ErrorResponse), func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusBadRequest
		cu.Description = "Bad Request"
	})
	op.AddRespStructure(new(httptools.ErrorResponse), func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusUnauthorized
		cu.Description = "Unauthorized - missing or invalid API key"
	})
	op.AddRespStructure(new(httptools.ErrorResponse), func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusUnprocessableEntity
		cu.Description = "Validation Failed"
	})
	op.AddRespStructure(new(httptools.ErrorResponse), func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusInternalServerError
		cu.Description = "Internal Server Error"
	})
}
