package subscriptions

import (
	"context"
	"encoding/json"
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
	UserID          string           `json:"user_id"          description:"The user ID"`
	Plan            string           `json:"plan"             description:"The user's current plan ID"`
	Status          string           `json:"status"           description:"Subscription status (active, on_trial, paused, past_due, cancelled, expired)"`
	Features        []string         `json:"features"         description:"List of feature IDs available to the user"`
	HasSubscription bool             `json:"has_subscription" description:"Whether the user has an active subscription"`
	TrialEndsAt     *int64           `json:"trial_ends_at"    description:"Unix timestamp when trial ends (if on trial)"`
	RenewsAt        *int64           `json:"renews_at"        description:"Unix timestamp when subscription renews"`
	Cancelled       bool             `json:"cancelled"        description:"Whether the subscription has been cancelled"`
	Raw             *RawSubscription `json:"raw"              description:"Raw provider-specific subscription data (null if no subscription)"`
}

type RawSubscription struct {
	Provider string               `json:"provider" enum:"lemonsqueezy" description:"Provider identifier"`
	Data     ProviderSubscription `json:"data"     description:"Provider-specific subscription data"`
}

// ProviderSubscription wraps provider-specific subscription data.
// Implements jsonschema.OneOfExposer for typed OpenAPI schema generation.
type ProviderSubscription struct {
	Value any
}

func (p ProviderSubscription) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.Value)
}

func (ProviderSubscription) JSONSchemaOneOf() []any {
	return []any{LemonSqueezySubscriptionDTO{}}
}

type LemonSqueezySubscriptionDTO struct {
	ID                 int    `json:"id"                   description:"LemonSqueezy subscription ID"`
	CustomerID         int    `json:"customer_id"          description:"LemonSqueezy customer ID"`
	OrderID            int    `json:"order_id"             description:"LemonSqueezy order ID"`
	ProductID          int    `json:"product_id"           description:"LemonSqueezy product ID"`
	ProductName        string `json:"product_name"         description:"Product display name"`
	VariantID          int    `json:"variant_id"           description:"LemonSqueezy variant ID"`
	VariantName        string `json:"variant_name"         description:"Variant display name"`
	Status             string `json:"status"               description:"Subscription status"`
	StatusFormatted    string `json:"status_formatted"     description:"Human-readable subscription status"`
	CardBrand          string `json:"card_brand"           description:"Payment card brand"`
	CardLastFour       string `json:"card_last_four"       description:"Last four digits of payment card"`
	Cancelled          bool   `json:"cancelled"            description:"Whether the subscription has been cancelled"`
	TrialEndsAt        *int64 `json:"trial_ends_at"        description:"Unix timestamp when trial ends"`
	BillingAnchor      int    `json:"billing_anchor"       description:"Day of month for billing"`
	SubscriptionItemID int    `json:"subscription_item_id" description:"LemonSqueezy subscription item ID"`
	RenewsAt           int64  `json:"renews_at"            description:"Unix timestamp when subscription renews"`
	EndsAt             *int64 `json:"ends_at"              description:"Unix timestamp when subscription ends"`
	CreatedAt          int64  `json:"created_at"           description:"Unix timestamp when subscription was created"`
	UpdatedAt          int64  `json:"updated_at"           description:"Unix timestamp when subscription was last updated"`
}

// SubscriptionReader reads subscription data.
type SubscriptionReader interface {
	GetSubscriptionByUserID(
		ctx context.Context,
		userID string,
	) (*Subscription, error)
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

func (route *RouteSubscription) Register(
	mux *http.ServeMux,
	r *openapi3.Reflector,
) {
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
	op.SetDescription(
		"Get subscription details for a user including their plan, status, and available features",
	)
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
			resp.Raw = toRawSubscription(sub)
		}

		httptools.JSON(w, r, http.StatusOK, resp)
	})
}

func toRawSubscription(sub *Subscription) *RawSubscription {
	return &RawSubscription{
		Provider: "lemonsqueezy",
		Data: ProviderSubscription{Value: LemonSqueezySubscriptionDTO{
			ID:                 sub.ID,
			CustomerID:         sub.CustomerID,
			OrderID:            sub.OrderID,
			ProductID:          sub.ProductID,
			ProductName:        sub.ProductName,
			VariantID:          sub.VariantID,
			VariantName:        sub.VariantName,
			Status:             sub.Status,
			StatusFormatted:    sub.StatusFormatted,
			CardBrand:          sub.CardBrand,
			CardLastFour:       sub.CardLastFour,
			Cancelled:          sub.Cancelled,
			TrialEndsAt:        sub.TrialEndsAt,
			BillingAnchor:      sub.BillingAnchor,
			SubscriptionItemID: sub.SubscriptionItemID,
			RenewsAt:           sub.RenewsAt,
			EndsAt:             sub.EndsAt,
			CreatedAt:          sub.CreatedAt,
			UpdatedAt:          sub.UpdatedAt,
		}},
	}
}

func isNotFound(err error) bool {
	return err != nil && err.Error() == "sql: no rows in result set"
}
