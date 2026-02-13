package entitlements

import (
	"net/http"
	"slices"

	"github.com/iamolegga/valmid"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/grantsy/grantsy/internal/infra/config"
	oa "github.com/grantsy/grantsy/internal/openapi"
)

// PlanSchema mirrors Plan for OpenAPI spec generation with nullable features.
type PlanSchema struct {
	ID          string    `json:"id"                    description:"Plan identifier"                required:"true"`
	Name        string    `json:"name"                  description:"Plan display name"              required:"true"`
	Description string    `json:"description,omitempty" description:"Plan description"`
	Features    []Feature `json:"features"              description:"Features included in this plan"  nullable:"true"`
	Variants    []Variant `json:"variants,omitempty"    description:"Pricing variants for this plan"`
}

type plansResponseSchema struct {
	Plans []PlanSchema `json:"plans" description:"List of available plans" nullable:"false" required:"true"`
}

type planResponseSchema struct {
	Plan PlanSchema `json:"plan" description:"Plan details" required:"true"`
}

type PlansExpand string

func (e *PlansExpand) UnmarshalText(text []byte) error {
	*e = PlansExpand(text)
	return nil
}

func (PlansExpand) Enum() []any {
	return []any{PlansExpandFeatures}
}

const (
	PlansExpandFeatures PlansExpand = "features"
)

type PlansRequest struct {
	Expand []PlansExpand `in:"query=expand" query:"expand" validate:"dive,oneof=features" description:"Fields to expand (use ?expand=features)"`
}

type PlansResponse struct {
	Plans []Plan `json:"plans" description:"List of available plans"`
}

type Plan struct {
	ID          string                         `json:"id"                    description:"Plan identifier"`
	Name        string                         `json:"name"                  description:"Plan display name"`
	Description string                         `json:"description,omitempty" description:"Plan description"`
	Features    httptools.Expandable[[]Feature] `json:"features,omitzero"    description:"Features included in this plan"`
	Variants    []Variant                      `json:"variants,omitempty"    description:"Pricing variants for this plan"`
}

type Feature struct {
	ID          string `json:"id"                    description:"Feature identifier"   required:"true"`
	Name        string `json:"name"                  description:"Feature display name" required:"true"`
	Description string `json:"description,omitempty" description:"Feature description"`
}

type Variant struct {
	ID                 int    `json:"id"                             description:"Variant identifier"                    required:"true"`
	Name               string `json:"name"                           description:"Variant display name"                  required:"true"`
	Price              any    `json:"price"                          description:"Price in cents"                         required:"true"`
	Interval           string `json:"interval"                       description:"Billing interval (month, year, etc.)"  required:"true"`
	IntervalCount      int    `json:"interval_count"                 description:"Number of intervals between billings"  required:"true"`
	HasFreeTrial       bool   `json:"has_free_trial"                 description:"Whether this variant has a free trial"  required:"true"`
	TrialInterval      string `json:"trial_interval,omitempty"       description:"Trial billing interval"`
	TrialIntervalCount int    `json:"trial_interval_count,omitempty" description:"Trial duration in intervals"`
	Sort               int    `json:"sort"                           description:"Display order"                          required:"true"`
}

type RoutePlans struct {
	service *Service
	pricing PricingProvider
}

func NewRoutePlans(service *Service, pricing PricingProvider) *RoutePlans {
	return &RoutePlans{service: service, pricing: pricing}
}

func (route *RoutePlans) Register(mux *http.ServeMux, r *openapi31.Reflector) {
	mux.Handle("GET /v1/plans",
		valmid.Middleware[PlansRequest]()(route.Handler()),
	)
	RegisterPlansSchema(r)
}

func RegisterPlansSchema(r *openapi31.Reflector) {
	op, _ := r.NewOperationContext(http.MethodGet, "/v1/plans")
	op.AddReqStructure(new(PlansRequest))
	op.AddRespStructure(struct {
		Data plansResponseSchema `json:"data"`
		Meta httptools.Meta      `json:"meta"`
		_    struct{}            `title:"PlansResponse"`
	}{}, func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusOK
		cu.Description = "List of available plans"
	})
	oa.AddErrorResponses(op)
	op.SetSummary("List all plans")
	op.SetDescription(
		"Get all available subscription plans with their pricing variants. Use ?expand=features to include features.",
	)
	op.SetTags("Plans")
	op.AddSecurity("ApiKeyAuth")
	r.AddOperation(op)
}

func (route *RoutePlans) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := valmid.Get[PlansRequest](r)
		includeFeatures := slices.Contains(input.Expand, PlansExpandFeatures)

		plans := route.service.GetPlans()
		features := route.service.GetFeatures()

		planDTOs := make([]Plan, len(plans))
		for i, p := range plans {
			if includeFeatures {
				planDTOs[i] = ToPlan(p, features, route.pricing.GetPlanVariants(p.ID))
			} else {
				planDTOs[i] = ToPlanSummary(p, route.pricing.GetPlanVariants(p.ID))
			}
		}

		httptools.JSON(w, r, http.StatusOK, PlansResponse{
			Plans: planDTOs,
		})
	})
}

func ToFeature(f config.FeatureConfig) Feature {
	return Feature{
		ID:          f.ID,
		Name:        f.Name,
		Description: f.Description,
	}
}

func ToPlan(plan config.PlanConfig, allFeatures []config.FeatureConfig, variants []Variant) Plan {
	featureIndex := make(map[string]config.FeatureConfig, len(allFeatures))
	for _, f := range allFeatures {
		featureIndex[f.ID] = f
	}

	features := make([]Feature, 0, len(plan.Features))
	for _, fID := range plan.Features {
		if f, ok := featureIndex[fID]; ok {
			features = append(features, ToFeature(f))
		}
	}

	return Plan{
		ID:          plan.ID,
		Name:        plan.Name,
		Description: plan.Description,
		Features:    httptools.Set(features),
		Variants:    variants,
	}
}

// ToPlanSummary creates a Plan without features (for default responses without expand).
func ToPlanSummary(plan config.PlanConfig, variants []Variant) Plan {
	return Plan{
		ID:          plan.ID,
		Name:        plan.Name,
		Description: plan.Description,
		Variants:    variants,
	}
}
