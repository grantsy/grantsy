package entitlements

import (
	"net/http"
	"slices"

	"github.com/iamolegga/valmid"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/grantsy/grantsy/internal/infra/metrics"
	oa "github.com/grantsy/grantsy/internal/openapi"
)

type CheckExpand string

func (e *CheckExpand) UnmarshalText(text []byte) error {
	*e = CheckExpand(text)
	return nil
}

func (CheckExpand) Enum() []any {
	return []any{CheckExpandFeature, CheckExpandPlan, CheckExpandPlanFeatures}
}

const (
	CheckExpandFeature      CheckExpand = "feature"
	CheckExpandPlan         CheckExpand = "plan"
	CheckExpandPlanFeatures CheckExpand = "plan.features"
)

type CheckRequest struct {
	UserID  string        `in:"query=user_id" query:"user_id" validate:"required"                              description:"User ID to check access for"`
	Feature string        `in:"query=feature" query:"feature" validate:"required"                              description:"Feature ID to check access for"`
	Expand  []CheckExpand `in:"query=expand"  query:"expand"  validate:"dive,oneof=feature plan plan.features" description:"Fields to expand (use ?expand=feature&expand=plan&expand=plan.features)"`
}

type CheckResponse struct {
	Allowed bool        `json:"allowed"           description:"Whether the user has access to this feature"`
	UserID  string      `json:"user_id"           description:"The user ID"`
	Reason  CheckReason `json:"reason"            description:"Reason for the access decision"                                         enum:"no_subscription,default_plan,feature_in_plan,insufficient_plan"`
	Feature *Feature    `json:"feature,omitempty" description:"The checked feature (requires expand=feature)"`
	Plan    *Plan       `json:"plan,omitempty"    description:"The user's current plan (requires expand=plan or expand=plan.features)"`
}

type RouteCheck struct {
	service *Service
}

func NewRouteCheck(service *Service) *RouteCheck {
	return &RouteCheck{service: service}
}

func (route *RouteCheck) Register(mux *http.ServeMux, r *openapi3.Reflector) {
	mux.Handle("GET /v1/check",
		valmid.Middleware[CheckRequest]()(route.Handler()),
	)
	RegisterCheckSchema(r)
}

func RegisterCheckSchema(r *openapi3.Reflector) {
	op, _ := r.NewOperationContext(http.MethodGet, "/v1/check")
	op.AddReqStructure(new(CheckRequest))
	op.AddRespStructure(struct {
		Data CheckResponse  `json:"data"`
		Meta httptools.Meta `json:"meta"`
		_    struct{}       `title:"CheckResponse"`
	}{}, func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusOK
		cu.Description = "Feature access check result"
	})
	oa.AddErrorResponses(op)
	op.SetSummary("Check feature access")
	op.SetDescription(
		"Check if a user has access to a specific feature based on their subscription plan. Use ?expand=feature&expand=plan&expand=plan.features to include additional details.",
	)
	op.SetTags("Entitlements")
	op.AddSecurity("ApiKeyAuth")
	r.AddOperation(op)
}

func (route *RouteCheck) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := valmid.Get[CheckRequest](r)

		result := route.service.CheckFeature(input.UserID, input.Feature)
		metrics.RecordEntitlementCheck(result.FeatureID, result.Allowed)

		resp := CheckResponse{
			Allowed: result.Allowed,
			UserID:  result.UserID,
			Reason:  result.Reason,
		}

		if slices.Contains(input.Expand, CheckExpandFeature) {
			var featureDTO Feature
			if f := route.service.GetFeature(result.FeatureID); f != nil {
				featureDTO = ToFeature(*f)
			} else {
				featureDTO = Feature{ID: result.FeatureID}
			}
			resp.Feature = &featureDTO
		}

		if slices.Contains(input.Expand, CheckExpandPlanFeatures) {
			features := route.service.GetFeatures()
			var planDTO Plan
			if p := route.service.GetPlan(result.PlanID); p != nil {
				planDTO = ToPlan(*p, features, nil)
			} else {
				planDTO = Plan{ID: result.PlanID}
			}
			resp.Plan = &planDTO
		} else if slices.Contains(input.Expand, CheckExpandPlan) {
			var planDTO Plan
			if p := route.service.GetPlan(result.PlanID); p != nil {
				planDTO = ToPlanSummary(*p, nil)
			} else {
				planDTO = Plan{ID: result.PlanID}
			}
			resp.Plan = &planDTO
		}

		httptools.JSON(w, r, http.StatusOK, resp)
	})
}
