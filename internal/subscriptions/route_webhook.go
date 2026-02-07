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
	"github.com/swaggest/openapi-go/openapi3"

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
}

// WebhookVerifier verifies incoming webhook signatures.
type WebhookVerifier interface {
	VerifyWebhook(ctx context.Context, signature string, body []byte) bool
}

type RouteWebhook struct {
	repo     SubscriptionWriter
	observer SubscriptionObserver
	provider WebhookVerifier
}

func NewRouteWebhook(
	provider WebhookVerifier,
	repo SubscriptionWriter,
	observer SubscriptionObserver,
) *RouteWebhook {
	return &RouteWebhook{
		repo:     repo,
		observer: observer,
		provider: provider,
	}
}

func (route *RouteWebhook) Register(mux *http.ServeMux, _ *openapi3.Reflector) {
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

		default:
			log.Info("invalid event", "event", eventName, "payload", string(payload))
			httptools.WriteStatus(w, http.StatusBadRequest)
			return
		}
	})
}

func (route *RouteWebhook) notifyObserver(
	ctx context.Context,
	sub *Subscription,
) error {
	return route.observer.OnSubscriptionChange(
		ctx,
		sub.UserID,
		sub.ProductID,
		sub.IsActive(),
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
	var subscriptionItemID int
	if s.Data.Attributes.FirstSubscriptionItem != nil {
		subscriptionItemID = s.Data.Attributes.FirstSubscriptionItem.ID
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
