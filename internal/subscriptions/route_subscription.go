package subscriptions

import (
	"context"
	"net/http"

	"github.com/iamolegga/valmid"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/grantsy/grantsy/internal/infra/logger"
	oa "github.com/grantsy/grantsy/internal/openapi"
)

type SubscriptionInput struct {
	UserID string `in:"query=user_id" query:"user_id" validate:"required" description:"User ID to get subscription for"`
}

type SubscriptionResponse struct {
	UserID          string   `json:"user_id" description:"The user ID"`
	Plan            string   `json:"plan" description:"The user's current plan ID"`
	Status          string   `json:"status" description:"Subscription status (active, on_trial, paused, past_due, cancelled, expired)"`
	Features        []string `json:"features" description:"List of feature IDs available to the user"`
	HasSubscription bool     `json:"has_subscription" description:"Whether the user has an active subscription"`
	TrialEndsAt     *int64   `json:"trial_ends_at" description:"Unix timestamp when trial ends (if on trial)"`
	RenewsAt        *int64   `json:"renews_at" description:"Unix timestamp when subscription renews"`
	Cancelled       bool     `json:"cancelled" description:"Whether the subscription has been cancelled"`
}

// SubscriptionReader reads subscription data.
type SubscriptionReader interface {
	GetSubscriptionByUserID(ctx context.Context, userID string) (*Subscription, error)
}

// PlanProvider provides plan and feature info for a user.
type PlanProvider interface {
	GetUserPlan(userID string) string
	GetUserFeatures(userID string) []string
}

type RouteSubscription struct {
	repo         SubscriptionReader
	planProvider PlanProvider
}

func NewRouteSubscription(
	repo SubscriptionReader,
	planProvider PlanProvider,
) *RouteSubscription {
	return &RouteSubscription{
		repo:         repo,
		planProvider: planProvider,
	}
}

func (route *RouteSubscription) Register(mux *http.ServeMux, r *openapi3.Reflector) {
	mux.Handle("GET /v1/subscription",
		valmid.Middleware[SubscriptionInput]()(route.Handler()),
	)
	RegisterSubscriptionSchema(r)
}

func RegisterSubscriptionSchema(r *openapi3.Reflector) {
	op, _ := r.NewOperationContext(http.MethodGet, "/v1/subscription")
	op.AddReqStructure(new(SubscriptionInput))
	op.AddRespStructure(struct {
		Data SubscriptionResponse `json:"data"`
		Meta httptools.Meta       `json:"meta"`
		_    struct{}             `title:"SubscriptionDataResponse"`
	}{}, func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = http.StatusOK
		cu.Description = "User subscription details"
	})
	oa.AddErrorResponses(op)
	op.SetSummary("Get user subscription")
	op.SetDescription("Get subscription details for a user including their plan, status, and available features")
	op.SetTags("Subscriptions")
	op.AddSecurity("ApiKeyAuth")
	r.AddOperation(op)
}

func (route *RouteSubscription) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := valmid.Get[SubscriptionInput](r)

		sub, err := route.repo.GetSubscriptionByUserID(r.Context(), input.UserID)
		if err != nil && !isNotFound(err) {
			logger.FromContext(r.Context()).
				Error("failed to get subscription", "error", err, "user_id", input.UserID)
			httptools.InternalError(w, r)
			return
		}

		planID := route.planProvider.GetUserPlan(input.UserID)
		features := route.planProvider.GetUserFeatures(input.UserID)

		resp := SubscriptionResponse{
			UserID:          input.UserID,
			Plan:            planID,
			Status:          "active",
			Features:        features,
			HasSubscription: sub != nil,
			Cancelled:       false,
		}

		if sub != nil {
			resp.Status = sub.Status
			resp.TrialEndsAt = sub.TrialEndsAt
			resp.RenewsAt = &sub.RenewsAt
			resp.Cancelled = sub.Cancelled
		}

		httptools.JSON(w, r, http.StatusOK, resp)
	})
}

func isNotFound(err error) bool {
	return err != nil && err.Error() == "sql: no rows in result set"
}
