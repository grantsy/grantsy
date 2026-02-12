package entitlements

import (
	"fmt"
	"net/http"

	"github.com/iamolegga/valmid"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"

	"github.com/grantsy/grantsy/internal/httptools"
	oa "github.com/grantsy/grantsy/internal/openapi"
)

type FeatureRequest struct {
	FeatureID string `in:"path=feature_id" path:"feature_id" validate:"required" description:"Feature ID to look up"`
}

type FeatureResponse struct {
	Feature Feature `json:"feature" description:"Feature details"`
}

type RouteFeature struct {
	service *Service
}

func NewRouteFeature(service *Service) *RouteFeature {
	return &RouteFeature{service: service}
}

func (route *RouteFeature) Register(mux *http.ServeMux, r *openapi3.Reflector) {
	mux.Handle("GET /v1/features/{feature_id}",
		valmid.Middleware[FeatureRequest]()(route.Handler()),
	)
	RegisterFeatureSchema(r)
}

func RegisterFeatureSchema(r *openapi3.Reflector) {
	op, _ := r.NewOperationContext(http.MethodGet, "/v1/features/{feature_id}")
	op.AddReqStructure(new(FeatureRequest))
	op.AddRespStructure(struct {
		Data FeatureResponse `json:"data"`
		Meta httptools.Meta  `json:"meta"`
		_    struct{}        `title:"FeatureResponse"`
	}{}, func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusOK
		cu.Description = "Feature details"
	})
	oa.AddErrorResponses(op)
	op.SetSummary("Get feature by ID")
	op.SetDescription("Get details of a specific feature by its identifier")
	op.SetTags("Features")
	op.AddSecurity("ApiKeyAuth")
	r.AddOperation(op)
}

func (route *RouteFeature) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := valmid.Get[FeatureRequest](r)

		f := route.service.GetFeature(input.FeatureID)
		if f == nil {
			httptools.NotFound(w, r, fmt.Sprintf("Feature '%s' not found", input.FeatureID))
			return
		}

		httptools.JSON(w, r, http.StatusOK, FeatureResponse{
			Feature: ToFeature(*f),
		})
	})
}
