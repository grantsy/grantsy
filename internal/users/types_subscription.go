package users

import (
	"encoding/json"

	"github.com/grantsy/grantsy/internal/subscriptions"
)

// UserSubscription is the subscription display type for user responses.
type UserSubscription struct {
	Status      string           `json:"status"        description:"Subscription status (active, on_trial, paused, past_due, cancelled, expired)"`
	TrialEndsAt *int64           `json:"trial_ends_at" description:"Unix timestamp when trial ends (if on trial)"`
	RenewsAt    *int64           `json:"renews_at"     description:"Unix timestamp when subscription renews"`
	EndsAt      *int64           `json:"ends_at"       description:"Unix timestamp when subscription ends (if cancelled)"`
	Cancelled   bool             `json:"cancelled"     description:"Whether the subscription has been cancelled"`
	Raw         *RawSubscription `json:"raw"           description:"Raw provider-specific subscription data"`
}

// RawSubscription wraps provider-specific subscription data with a provider identifier.
type RawSubscription struct {
	Provider string               `json:"provider" enum:"lemonsqueezy" description:"Provider identifier"`
	Data     ProviderSubscription `json:"data"                         description:"Provider-specific subscription data"`
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
	return []any{LemonSqueezySubscription{}}
}

type LemonSqueezySubscription struct {
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

// ToRawSubscription converts a domain Subscription to a RawSubscription display type.
func ToRawSubscription(sub *subscriptions.Subscription) *RawSubscription {
	return &RawSubscription{
		Provider: "lemonsqueezy",
		Data: ProviderSubscription{Value: LemonSqueezySubscription{
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

// ToUserSubscription converts a domain Subscription to a UserSubscription display type.
func ToUserSubscription(sub *subscriptions.Subscription) *UserSubscription {
	return &UserSubscription{
		Status:      sub.Status,
		TrialEndsAt: sub.TrialEndsAt,
		RenewsAt:    &sub.RenewsAt,
		EndsAt:      sub.EndsAt,
		Cancelled:   sub.Cancelled,
		Raw:         ToRawSubscription(sub),
	}
}
