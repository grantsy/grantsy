package entitlements

import (
	"net/http"

	"github.com/iamolegga/valmid"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/grantsy/grantsy/internal/infra/metrics"
	oa "github.com/grantsy/grantsy/internal/openapi"
)

type CheckInput struct {
	UserID  string `in:"query=user_id" query:"user_id" validate:"required" description:"User ID to check access for"`
	Feature string `in:"query=feature" query:"feature" validate:"required" description:"Feature ID to check access for"`
}

type RouteCheck struct {
	service *Service
}

func NewRouteCheck(service *Service) *RouteCheck {
	return &RouteCheck{service: service}
}

func (route *RouteCheck) Register(mux *http.ServeMux, r *openapi3.Reflector) {
	mux.Handle("GET /v1/check",
		valmid.Middleware[CheckInput]()(route.Handler()),
	)
	RegisterCheckSchema(r)
}

func RegisterCheckSchema(r *openapi3.Reflector) {
	op, _ := r.NewOperationContext(http.MethodGet, "/v1/check")
	op.AddReqStructure(new(CheckInput))
	op.AddRespStructure(struct {
		Data CheckResult    `json:"data"`
		Meta httptools.Meta `json:"meta"`
		_    struct{}       `title:"CheckResultResponse"`
	}{}, func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusOK
		cu.Description = "Feature access check result"
	})
	oa.AddErrorResponses(op)
	op.SetSummary("Check feature access")
	op.SetDescription("Check if a user has access to a specific feature based on their subscription plan")
	op.SetTags("Entitlements")
	op.AddSecurity("ApiKeyAuth")
	r.AddOperation(op)
}

func (route *RouteCheck) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := valmid.Get[CheckInput](r)

		result := route.service.CheckFeature(input.UserID, input.Feature)
		metrics.RecordEntitlementCheck(result.FeatureID, result.Allowed)

		httptools.JSON(w, r, http.StatusOK, result)
	})
}
