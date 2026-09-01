package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/iamolegga/lemonsqueezy-go"
	"github.com/swaggest/openapi-go/openapi31"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/grantsy/grantsy/internal/infra/logger"
)

// SubscriptionObserver is notified when subscriptions change state.
// Uses primitives + any to avoid import cycles.
type SubscriptionObserver interface {
	OnSubscriptionChange(
		ctx context.Context,
		userID string,
		productID int,
		active bool,
		subscription any,
	) error
}

// SubscriptionWriter writes subscription data.
type SubscriptionWriter interface {
	UpsertSubscription(ctx context.Context, sub *Subscription) error
	MarkRefundedBySubscriptionID(
		ctx context.Context,
		subscriptionID int,
		at int64,
	) (*Subscription, error)
	MarkRefundedByOrderID(
		ctx context.Context,
		orderID int,
		at int64,
	) (*Subscription, error)
}

// WebhookVerifier verifies incoming webhook signatures.
type WebhookVerifier interface {
	VerifyWebhook(ctx context.Context, signature string, body []byte) bool
}

// PriceFetcher fetches price data from the billing provider.
type PriceFetcher interface {
	GetPrice(ctx context.Context, priceID int) (*PriceInfo, error)
}

type RouteWebhook struct {
	repo     SubscriptionWriter
	observer SubscriptionObserver
	provider WebhookVerifier
	pricing  PriceFetcher
	strictAccess bool
}

func NewRouteWebhook(
	provider WebhookVerifier,
	pricing PriceFetcher,
	repo SubscriptionWriter,
	observer SubscriptionObserver,
	strictAccess bool,
) *RouteWebhook {
	return &RouteWebhook{
		repo:     repo,
		observer: observer,
		provider: provider,
		pricing:  pricing,
		strictAccess: strictAccess,
	}
}

func (route *RouteWebhook) Register(mux *http.ServeMux, _ *openapi31.Reflector) {
	mux.Handle("POST /v1/webhook/lemonsqueezy", route.Handler())
	// Webhook intentionally excluded from OpenAPI documentation
}

func (route *RouteWebhook) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())

		if err := route.validateWebhook(r); err != nil {
			log.Info("failed to validate webhook", "error", err)
			httptools.WriteStatus(w, http.StatusBadRequest)
			return
		}

		eventName := r.Header.Get("X-Event-Name")
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			log.Info("failed to read webhook payload", "error", err)
			httptools.WriteStatus(w, http.StatusBadRequest)
			return
		}

		switch eventName {
		case lemonsqueezy.WebhookEventSubscriptionCreated,
			lemonsqueezy.WebhookEventSubscriptionUpdated:
			var request lemonsqueezy.WebhookRequestSubscription
			if err := json.Unmarshal(payload, &request); err != nil {
				log.Info("failed to unmarshal webhook payload", "error", err)
				httptools.WriteStatus(w, http.StatusBadRequest)
				return
			}
			log.Debug(eventName, "request", request)
			sub := MapLemonsqueezyToSubscription(request)
			if sub.UserID == "" {
				log.Warn(
					"webhook subscription has empty user_id",
					"subscription_id", sub.ID,
					"event", eventName,
				)
			}
			if sub.PriceID == 0 {
				log.Error("missing price_id in webhook payload", "subscription_id", sub.ID)
				httptools.WriteStatus(w, http.StatusBadRequest)
				return
			}
			price, err := route.pricing.GetPrice(r.Context(), sub.PriceID)
			if err != nil {
				log.Error("failed to fetch price", "error", err, "price_id", sub.PriceID)
				httptools.WriteStatus(w, http.StatusInternalServerError)
				return
			}
			sub.UnitPrice = price.UnitPrice
			sub.RenewalIntervalUnit = price.RenewalIntervalUnit
			sub.RenewalIntervalQuantity = price.RenewalIntervalQuantity
			if err := route.repo.UpsertSubscription(r.Context(), sub); err != nil {
				log.Info("failed to upsert subscription", "error", err)
				httptools.WriteStatus(w, http.StatusInternalServerError)
				return
			}
			if err := route.notifyObserver(r.Context(), sub); err != nil {
				log.Info("failed to update entitlements", "error", err)
				httptools.WriteStatus(w, http.StatusInternalServerError)
				return
			}
			httptools.WriteStatus(w, http.StatusOK)
			return

		case lemonsqueezy.WebhookEventSubscriptionPaymentRefunded:
			var request lemonsqueezy.WebhookRequestSubscriptionInvoice
			if err := json.Unmarshal(payload, &request); err != nil {
				log.Info("failed to unmarshal webhook payload", "error", err)
				httptools.WriteStatus(w, http.StatusBadRequest)
				return
			}
			log.Debug(eventName, "request", request)
			sub, err := route.repo.MarkRefundedBySubscriptionID(
				r.Context(),
				request.Data.Attributes.SubscriptionID,
				refundedAt(request.Data.Attributes.RefundedAt),
			)
			route.finishRefund(w, r, eventName, sub, err)
			return

		case lemonsqueezy.WebhookEventOrderRefunded:
			var request lemonsqueezy.WebhookRequestOrder
			if err := json.Unmarshal(payload, &request); err != nil {
				log.Info("failed to unmarshal webhook payload", "error", err)
				httptools.WriteStatus(w, http.StatusBadRequest)
				return
			}
			log.Debug(eventName, "request", request)
			orderID, err := strconv.Atoi(request.Data.ID)
			if err != nil {
				log.Info(
					"invalid order id in webhook payload",
					"error", err,
					"order_id", request.Data.ID,
				)
				httptools.WriteStatus(w, http.StatusBadRequest)
				return
			}
			sub, err := route.repo.MarkRefundedByOrderID(
				r.Context(),
				orderID,
				refundedAt(request.Data.Attributes.RefundedAt),
			)
			route.finishRefund(w, r, eventName, sub, err)
			return

		default:
			log.Info("invalid event", "event", eventName, "payload", string(payload))
			httptools.WriteStatus(w, http.StatusBadRequest)
			return
		}
	})
}

// finishRefund revokes access for a subscription that was just marked refunded
// and writes the response.
//
// A refund that matched no row answers 500 so the provider retries: webhook
// delivery is not ordered, and a refund arriving before its
// subscription_created must not be dropped silently. Provider retries are
// bounded, so a refund of a product this service does not track cannot loop.
func (route *RouteWebhook) finishRefund(
	w http.ResponseWriter,
	r *http.Request,
	eventName string,
	sub *Subscription,
	err error,
) {
	log := logger.FromContext(r.Context())

	if err != nil {
		log.Error(
			"failed to mark subscription refunded",
			"error", err,
			"event", eventName,
		)
		httptools.WriteStatus(w, http.StatusInternalServerError)
		return
	}

	if sub == nil {
		log.Warn("refund for unknown subscription", "event", eventName)
		httptools.WriteStatus(w, http.StatusInternalServerError)
		return
	}

	if err := route.notifyObserver(r.Context(), sub); err != nil {
		log.Info("failed to update entitlements", "error", err)
		httptools.WriteStatus(w, http.StatusInternalServerError)
		return
	}

	httptools.WriteStatus(w, http.StatusOK)
}

// refundedAt falls back to now when the provider omits the refund timestamp:
// refunded_at doubles as the revocation flag, so it must never stay NULL.
func refundedAt(t *time.Time) int64 {
	if t == nil || t.IsZero() {
		return time.Now().Unix()
	}
	return t.Unix()
}

func (route *RouteWebhook) notifyObserver(
	ctx context.Context,
	sub *Subscription,
) error {
	return route.observer.OnSubscriptionChange(
		ctx,
		sub.UserID,
		sub.ProductID,
		sub.IsActive(route.strictAccess),
		sub,
	)
}

func (route *RouteWebhook) validateWebhook(r *http.Request) error {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	signature := r.Header.Get("X-Signature")
	if signature == "" {
		return errors.New("missing X-Signature header")
	}

	if !route.provider.VerifyWebhook(r.Context(), signature, bodyBytes) {
		return errors.New("invalid signature")
	}

	return nil
}

func MapLemonsqueezyToSubscription(
	s lemonsqueezy.WebhookRequestSubscription,
) *Subscription {
	subscriptionID, _ := strconv.Atoi(s.Data.ID)
	var subscriptionItemID, priceID int
	if s.Data.Attributes.FirstSubscriptionItem != nil {
		subscriptionItemID = s.Data.Attributes.FirstSubscriptionItem.ID
		priceID = s.Data.Attributes.FirstSubscriptionItem.PriceID
	}

	userID, _ := s.Meta.CustomData["user_id"].(string)

	return &Subscription{
		ID:                 subscriptionID,
		UserID:             userID,
		CustomerID:         s.Data.Attributes.CustomerID,
		OrderID:            s.Data.Attributes.OrderID,
		ProductID:          s.Data.Attributes.ProductID,
		ProductName:        s.Data.Attributes.ProductName,
		VariantID:          s.Data.Attributes.VariantID,
		VariantName:        s.Data.Attributes.VariantName,
		Status:             s.Data.Attributes.Status,
		StatusFormatted:    s.Data.Attributes.StatusFormatted,
		CardBrand:          s.Data.Attributes.CardBrand,
		CardLastFour:       s.Data.Attributes.CardLastFour,
		Cancelled:          s.Data.Attributes.Cancelled,
		TrialEndsAt:        TimePtrToUnix(s.Data.Attributes.TrialEndsAt),
		BillingAnchor:      s.Data.Attributes.BillingAnchor,
		SubscriptionItemID: subscriptionItemID,
		PriceID:            priceID,
		RenewsAt:           s.Data.Attributes.RenewsAt.Unix(),
		EndsAt:             TimePtrToUnix(s.Data.Attributes.EndsAt),
		CreatedAt:          s.Data.Attributes.CreatedAt.Unix(),
		UpdatedAt:          s.Data.Attributes.UpdatedAt.Unix(),
	}
}

func TimePtrToUnix(t *time.Time) *int64 {
	if t == nil || t.IsZero() {
		return nil
	}
	unix := t.Unix()
	return &unix
}
