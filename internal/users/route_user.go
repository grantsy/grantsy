package users

import (
	"context"
	"net/http"
	"slices"

	"github.com/iamolegga/valmid"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"

	"github.com/grantsy/grantsy/internal/entitlements"
	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/grantsy/grantsy/internal/infra/config"
	"github.com/grantsy/grantsy/internal/infra/logger"
	oa "github.com/grantsy/grantsy/internal/openapi"
	"github.com/grantsy/grantsy/internal/subscriptions"
)

// EntitlementService provides plan and feature data for users.
type EntitlementService interface {
	GetUserPlan(userID string) string
	GetPlan(planID string) *config.PlanConfig
	GetFeature(featureID string) *config.FeatureConfig
	GetUserFeatures(userID string) []string
}

// SubscriptionRepo reads subscription data from the database.
type SubscriptionRepo interface {
	GetSubscriptionByUserID(ctx context.Context, userID string) (*subscriptions.Subscription, error)
}

type UserExpand string

func (e *UserExpand) UnmarshalText(text []byte) error {
	*e = UserExpand(text)
	return nil
}

func (UserExpand) Enum() []any {
	return []any{UserExpandPlan, UserExpandFeatures, UserExpandSubscription}
}

const (
	UserExpandPlan         UserExpand = "plan"
	UserExpandFeatures     UserExpand = "features"
	UserExpandSubscription UserExpand = "subscription"
)

type UserRequest struct {
	UserID string       `in:"path=user_id" path:"user_id" validate:"required"                              description:"User ID to look up"`
	Expand []UserExpand `in:"query=expand"                validate:"dive,oneof=plan features subscription" description:"Fields to expand (use ?expand=plan&expand=features&expand=subscription)" query:"expand"`
}

type UserResponse struct {
	UserID       string                                      `json:"user_id"              description:"The user ID"`
	PlanID       string                                      `json:"plan_id"              description:"The user's current plan ID"`
	Plan         httptools.Expandable[entitlements.Plan]      `json:"plan,omitzero"        description:"Plan details (requires expand=plan)"`
	Features     httptools.Expandable[[]entitlements.Feature] `json:"features,omitzero"    description:"Features available to the user (requires expand=features)"`
	Subscription httptools.Expandable[UserSubscription]       `json:"subscription,omitzero" description:"Subscription details (requires expand=subscription)"`
}

// userResponseSchema mirrors UserResponse for OpenAPI spec generation with nullable fields.
type userResponseSchema struct {
	UserID       string                  `json:"user_id"       description:"The user ID"                                                                  required:"true"`
	PlanID       string                  `json:"plan_id"       description:"The user's current plan ID"                                                    required:"true"`
	Plan         *entitlements.PlanSchema `json:"plan"          description:"Plan details (requires expand=plan)"`
	Features     []entitlements.Feature  `json:"features"      description:"Features available to the user (requires expand=features)" nullable:"true"`
	Subscription *UserSubscription       `json:"subscription"  description:"Subscription details (requires expand=subscription)"`
}

type RouteUser struct {
	entService EntitlementService
	subRepo    SubscriptionRepo
}

func NewRouteUser(entService EntitlementService, subRepo SubscriptionRepo) *RouteUser {
	return &RouteUser{entService: entService, subRepo: subRepo}
}

func (route *RouteUser) Register(mux *http.ServeMux, r *openapi31.Reflector) {
	mux.Handle("GET /v1/users/{user_id}",
		valmid.Middleware[UserRequest]()(route.Handler()),
	)
	RegisterUserSchema(r)
}

func RegisterUserSchema(r *openapi31.Reflector) {
	op, _ := r.NewOperationContext(http.MethodGet, "/v1/users/{user_id}")
	op.AddReqStructure(new(UserRequest))
	op.AddRespStructure(struct {
		Data userResponseSchema `json:"data"`
		Meta httptools.Meta     `json:"meta"`
		_    struct{}           `title:"UserResponse"`
	}{}, func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusOK
		cu.Description = "User state"
	})
	oa.AddErrorResponses(op)
	op.SetSummary("Get user state")
	op.SetDescription(
		"Get the current state for a user. Always returns plan_id. Use ?expand=plan,features,subscription to include additional details.",
	)
	op.SetTags("Users")
	op.AddSecurity("ApiKeyAuth")
	r.AddOperation(op)
}

func (route *RouteUser) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := valmid.Get[UserRequest](r)

		planID := route.entService.GetUserPlan(input.UserID)

		resp := UserResponse{
			UserID: input.UserID,
			PlanID: planID,
		}

		if slices.Contains(input.Expand, UserExpandPlan) {
			if p := route.entService.GetPlan(planID); p != nil {
				resp.Plan = httptools.Set(entitlements.ToPlanSummary(*p, nil))
			} else {
				resp.Plan = httptools.Set(entitlements.Plan{ID: planID})
			}
		}

		if slices.Contains(input.Expand, UserExpandFeatures) {
			featureIDs := route.entService.GetUserFeatures(input.UserID)
			featureDTOs := make([]entitlements.Feature, 0, len(featureIDs))
			for _, fID := range featureIDs {
				if f := route.entService.GetFeature(fID); f != nil {
					featureDTOs = append(featureDTOs, entitlements.ToFeature(*f))
				} else {
					featureDTOs = append(featureDTOs, entitlements.Feature{ID: fID})
				}
			}
			resp.Features = httptools.Set(featureDTOs)
		}

		if slices.Contains(input.Expand, UserExpandSubscription) {
			sub, err := route.subRepo.GetSubscriptionByUserID(r.Context(), input.UserID)
			if err != nil {
				logger.FromContext(r.Context()).
					Error("failed to get subscription", "error", err, "user_id", input.UserID)
				httptools.InternalError(w, r)
				return
			}
			if sub != nil {
				resp.Subscription = httptools.Set(*ToUserSubscription(sub))
			} else {
				resp.Subscription = httptools.Null[UserSubscription]()
			}
		}

		httptools.JSON(w, r, http.StatusOK, resp)
	})
}
