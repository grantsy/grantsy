package entitlements

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/iamolegga/valmid"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"

	"github.com/grantsy/grantsy/internal/httptools"
	oa "github.com/grantsy/grantsy/internal/openapi"
)

type PlanExpand string

func (e *PlanExpand) UnmarshalText(text []byte) error {
	*e = PlanExpand(text)
	return nil
}

func (PlanExpand) Enum() []any {
	return []any{PlanExpandFeatures}
}

const (
	PlanExpandFeatures PlanExpand = "features"
)

type PlanRequest struct {
	PlanID string       `in:"path=plan_id" path:"plan_id" validate:"required"            description:"Plan ID to look up"`
	Expand []PlanExpand `in:"query=expand"                validate:"dive,oneof=features" description:"Fields to expand (use ?expand=features)" query:"expand"`
}

type PlanResponse struct {
	Plan Plan `json:"plan" description:"Plan details"`
}

type RoutePlan struct {
	service *Service
	pricing PricingProvider
}

func NewRoutePlan(service *Service, pricing PricingProvider) *RoutePlan {
	return &RoutePlan{service: service, pricing: pricing}
}

func (route *RoutePlan) Register(mux *http.ServeMux, r *openapi3.Reflector) {
	mux.Handle("GET /v1/plans/{plan_id}",
		valmid.Middleware[PlanRequest]()(route.Handler()),
	)
	RegisterPlanSchema(r)
}

func RegisterPlanSchema(r *openapi3.Reflector) {
	op, _ := r.NewOperationContext(http.MethodGet, "/v1/plans/{plan_id}")
	op.AddReqStructure(new(PlanRequest))
	op.AddRespStructure(struct {
		Data PlanResponse   `json:"data"`
		Meta httptools.Meta `json:"meta"`
		_    struct{}       `title:"PlanResponse"`
	}{}, func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusOK
		cu.Description = "Plan details"
	})
	oa.AddErrorResponses(op)
	op.SetSummary("Get plan by ID")
	op.SetDescription(
		"Get details of a specific plan by its identifier. Use ?expand=features to include feature details.",
	)
	op.SetTags("Plans")
	op.AddSecurity("ApiKeyAuth")
	r.AddOperation(op)
}

func (route *RoutePlan) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := valmid.Get[PlanRequest](r)

		p := route.service.GetPlan(input.PlanID)
		if p == nil {
			httptools.NotFound(w, r, fmt.Sprintf("Plan '%s' not found", input.PlanID))
			return
		}

		variants := route.pricing.GetPlanVariants(input.PlanID)

		var planDTO Plan
		if slices.Contains(input.Expand, PlanExpandFeatures) {
			planDTO = ToPlan(*p, route.service.GetFeatures(), variants)
		} else {
			planDTO = ToPlanSummary(*p, variants)
		}

		httptools.JSON(w, r, http.StatusOK, PlanResponse{
			Plan: planDTO,
		})
	})
}
