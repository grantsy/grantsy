package entitlements

import (
	"net/http"

	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"

	"github.com/grantsy/grantsy/internal/httptools"
	oa "github.com/grantsy/grantsy/internal/openapi"
)

type FeaturesResponse struct {
	Features []Feature `json:"features" description:"All available features" nullable:"false" required:"true"`
}

type RouteFeatures struct {
	service *Service
}

func NewRouteFeatures(service *Service) *RouteFeatures {
	return &RouteFeatures{service: service}
}

func (route *RouteFeatures) Register(mux *http.ServeMux, r *openapi31.Reflector) {
	mux.Handle("GET /v1/features", route.Handler())
	RegisterFeaturesSchema(r)
}

func RegisterFeaturesSchema(r *openapi31.Reflector) {
	op, _ := r.NewOperationContext(http.MethodGet, "/v1/features")
	op.AddRespStructure(struct {
		Data FeaturesResponse `json:"data"`
		Meta httptools.Meta   `json:"meta"`
		_    struct{}         `title:"FeaturesResponse"`
	}{}, func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusOK
		cu.Description = "All available features"
	})
	oa.AddErrorResponses(op)
	op.SetSummary("List all features")
	op.SetDescription("Get all available feature definitions")
	op.SetTags("Features")
	op.AddSecurity("ApiKeyAuth")
	r.AddOperation(op)
}

func (route *RouteFeatures) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		features := route.service.GetFeatures()

		featureDTOs := make([]Feature, len(features))
		for i, f := range features {
			featureDTOs[i] = ToFeature(f)
		}

		httptools.JSON(w, r, http.StatusOK, FeaturesResponse{
			Features: featureDTOs,
		})
	})
}
