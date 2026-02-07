package entitlements

import (
	"net/http"

	"github.com/iamolegga/valmid"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"

	"github.com/grantsy/grantsy/internal/httptools"
	oa "github.com/grantsy/grantsy/internal/openapi"
)

type FeaturesInput struct {
	UserID string `in:"query=user_id" query:"user_id" validate:"required" description:"User ID to get features for"`
}

type FeaturesResponse struct {
	UserID   string   `json:"user_id" description:"The user ID"`
	Plan     string   `json:"plan" description:"The user's current plan ID"`
	Features []string `json:"features" description:"List of feature IDs available to the user"`
}

type RouteFeatures struct {
	service *Service
}

func NewRouteFeatures(service *Service) *RouteFeatures {
	return &RouteFeatures{service: service}
}

func (route *RouteFeatures) Register(mux *http.ServeMux, r *openapi3.Reflector) {
	mux.Handle("GET /v1/features",
		valmid.Middleware[FeaturesInput]()(route.Handler()),
	)
	RegisterFeaturesSchema(r)
}

func RegisterFeaturesSchema(r *openapi3.Reflector) {
	op, _ := r.NewOperationContext(http.MethodGet, "/v1/features")
	op.AddReqStructure(new(FeaturesInput))
	op.AddRespStructure(struct {
		Data FeaturesResponse `json:"data"`
		Meta httptools.Meta   `json:"meta"`
		_    struct{}         `title:"FeaturesDataResponse"`
	}{}, func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusOK
		cu.Description = "User features list"
	})
	oa.AddErrorResponses(op)
	op.SetSummary("List user features")
	op.SetDescription("Get all features available to a user based on their subscription plan")
	op.SetTags("Entitlements")
	op.AddSecurity("ApiKeyAuth")
	r.AddOperation(op)
}

func (route *RouteFeatures) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := valmid.Get[FeaturesInput](r)

		planID := route.service.GetUserPlan(input.UserID)
		features := route.service.GetUserFeatures(input.UserID)

		httptools.JSON(w, r, http.StatusOK, FeaturesResponse{
			UserID:   input.UserID,
			Plan:     planID,
			Features: features,
		})
	})
}
