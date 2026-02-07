package entitlements

import (
	"net/http"

	"github.com/iamolegga/valmid"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/grantsy/grantsy/internal/infra/config"
	oa "github.com/grantsy/grantsy/internal/openapi"
)

type PlansInput struct {
	Expand string `in:"query=expand" query:"expand" description:"Set to 'features' to include feature details"`
}

type PlansResponse struct {
	Plans       []PlanDTO    `json:"plans" description:"List of available plans"`
	AllFeatures []FeatureDTO `json:"all_features,omitempty" description:"All feature definitions (when expand=features)"`
}

type PlanDTO struct {
	ID       string       `json:"id" description:"Plan identifier"`
	Name     string       `json:"name" description:"Plan display name"`
	Features []string     `json:"features" description:"List of feature IDs included in this plan"`
	Variants []VariantDTO `json:"variants,omitempty" description:"Pricing variants for this plan"`
}

type FeatureDTO struct {
	ID          string `json:"id" description:"Feature identifier"`
	Name        string `json:"name" description:"Feature display name"`
	Description string `json:"description,omitempty" description:"Feature description"`
}

type VariantDTO struct {
	ID                 int    `json:"id" description:"Variant identifier"`
	Name               string `json:"name" description:"Variant display name"`
	Price              any    `json:"price" description:"Price in cents"`
	Interval           string `json:"interval" description:"Billing interval (month, year, etc.)"`
	IntervalCount      int    `json:"interval_count" description:"Number of intervals between billings"`
	HasFreeTrial       bool   `json:"has_free_trial" description:"Whether this variant has a free trial"`
	TrialInterval      string `json:"trial_interval,omitempty" description:"Trial billing interval"`
	TrialIntervalCount int    `json:"trial_interval_count,omitempty" description:"Trial duration in intervals"`
	Sort               int    `json:"sort" description:"Display order"`
}

type RoutePlans struct {
	service *Service
	pricing PricingProvider
}

func NewRoutePlans(service *Service, pricing PricingProvider) *RoutePlans {
	return &RoutePlans{service: service, pricing: pricing}
}

func (route *RoutePlans) Register(mux *http.ServeMux, r *openapi3.Reflector) {
	mux.Handle("GET /v1/plans",
		valmid.Middleware[PlansInput]()(route.Handler()),
	)
	RegisterPlansSchema(r)
}

func RegisterPlansSchema(r *openapi3.Reflector) {
	op, _ := r.NewOperationContext(http.MethodGet, "/v1/plans")
	op.AddReqStructure(new(PlansInput))
	op.AddRespStructure(struct {
		Data PlansResponse  `json:"data"`
		Meta httptools.Meta `json:"meta"`
		_    struct{}       `title:"PlansDataResponse"`
	}{}, func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusOK
		cu.Description = "List of available plans"
	})
	oa.AddErrorResponses(op)
	op.SetSummary("List all plans")
	op.SetDescription("Get all available subscription plans. Use ?expand=features to include feature details")
	op.SetTags("Plans")
	op.AddSecurity("ApiKeyAuth")
	r.AddOperation(op)
}

func (route *RoutePlans) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := valmid.Get[PlansInput](r)
		expand := input.Expand

		plans := route.service.GetPlans()
		planDTOs := make([]PlanDTO, len(plans))
		for i, p := range plans {
			planDTOs[i] = PlanDTO{
				ID:       p.ID,
				Name:     p.Name,
				Features: p.Features,
				Variants: route.pricing.GetPlanVariants(p.ID),
			}
		}

		resp := PlansResponse{
			Plans: planDTOs,
		}

		if expand == "features" {
			features := route.service.GetFeatures()
			resp.AllFeatures = make([]FeatureDTO, len(features))
			for i, f := range features {
				resp.AllFeatures[i] = toFeatureDTO(f)
			}
		}

		httptools.JSON(w, r, http.StatusOK, resp)
	})
}

func toFeatureDTO(f config.FeatureConfig) FeatureDTO {
	return FeatureDTO{
		ID:          f.ID,
		Name:        f.Name,
		Description: f.Description,
	}
}
