package subscriptions_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iamolegga/lemonsqueezy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grantsy/grantsy/internal/subscriptions"
	"github.com/grantsy/grantsy/internal/subscriptions/mocks"
)

// --- TimePtrToUnix tests ---

func TestTimePtrToUnix_Nil(t *testing.T) {
	assert.Nil(t, subscriptions.TimePtrToUnix(nil))
}

func TestTimePtrToUnix_ZeroTime(t *testing.T) {
	zero := time.Time{}
	assert.Nil(t, subscriptions.TimePtrToUnix(&zero))
}

func TestTimePtrToUnix_ValidTime(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	result := subscriptions.TimePtrToUnix(&ts)
	require.NotNil(t, result)
	assert.Equal(t, ts.Unix(), *result)
}

// --- MapLemonsqueezyToSubscription tests ---

func TestMapLemonsqueezyToSubscription(t *testing.T) {
	trialEnd := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	endsAt := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	renewsAt := time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	req := lemonsqueezy.WebhookRequestSubscription{
		Meta: lemonsqueezy.WebhookRequestMeta{
			CustomData: map[string]any{"user_id": "user-123"},
		},
		Data: lemonsqueezy.WebhookRequestData[lemonsqueezy.Subscription, lemonsqueezy.ApiResponseRelationshipsSubscription]{
			ID: "42",
			Attributes: lemonsqueezy.Subscription{
				CustomerID:      100,
				OrderID:         200,
				ProductID:       300,
				ProductName:     "Pro Plan",
				VariantID:       400,
				VariantName:     "Monthly",
				Status:          "active",
				StatusFormatted: "Active",
				CardBrand:       "visa",
				CardLastFour:    "4242",
				Cancelled:       false,
				TrialEndsAt:     &trialEnd,
				BillingAnchor:   15,
				FirstSubscriptionItem: &lemonsqueezy.SubscriptionFirstSubscriptionItem{
					ID: 999,
				},
				RenewsAt:  renewsAt,
				EndsAt:    &endsAt,
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			},
		},
	}

	sub := subscriptions.MapLemonsqueezyToSubscription(req)

	assert.Equal(t, 42, sub.ID)
	assert.Equal(t, "user-123", sub.UserID)
	assert.Equal(t, 100, sub.CustomerID)
	assert.Equal(t, 200, sub.OrderID)
	assert.Equal(t, 300, sub.ProductID)
	assert.Equal(t, "Pro Plan", sub.ProductName)
	assert.Equal(t, 400, sub.VariantID)
	assert.Equal(t, "Monthly", sub.VariantName)
	assert.Equal(t, "active", sub.Status)
	assert.Equal(t, "Active", sub.StatusFormatted)
	assert.Equal(t, "visa", sub.CardBrand)
	assert.Equal(t, "4242", sub.CardLastFour)
	assert.False(t, sub.Cancelled)
	require.NotNil(t, sub.TrialEndsAt)
	assert.Equal(t, trialEnd.Unix(), *sub.TrialEndsAt)
	assert.Equal(t, 15, sub.BillingAnchor)
	assert.Equal(t, 999, sub.SubscriptionItemID)
	assert.Equal(t, renewsAt.Unix(), sub.RenewsAt)
	require.NotNil(t, sub.EndsAt)
	assert.Equal(t, endsAt.Unix(), *sub.EndsAt)
	assert.Equal(t, createdAt.Unix(), sub.CreatedAt)
	assert.Equal(t, updatedAt.Unix(), sub.UpdatedAt)
}

func TestMapLemonsqueezyToSubscription_NilFirstSubscriptionItem(t *testing.T) {
	req := lemonsqueezy.WebhookRequestSubscription{
		Meta: lemonsqueezy.WebhookRequestMeta{
			CustomData: map[string]any{"user_id": "user-123"},
		},
		Data: lemonsqueezy.WebhookRequestData[lemonsqueezy.Subscription, lemonsqueezy.ApiResponseRelationshipsSubscription]{
			ID: "1",
			Attributes: lemonsqueezy.Subscription{
				FirstSubscriptionItem: nil,
				RenewsAt:              time.Now(),
				CreatedAt:             time.Now(),
				UpdatedAt:             time.Now(),
			},
		},
	}

	sub := subscriptions.MapLemonsqueezyToSubscription(req)
	assert.Equal(t, 0, sub.SubscriptionItemID)
}

func TestMapLemonsqueezyToSubscription_MissingUserID(t *testing.T) {
	req := lemonsqueezy.WebhookRequestSubscription{
		Meta: lemonsqueezy.WebhookRequestMeta{
			CustomData: map[string]any{},
		},
		Data: lemonsqueezy.WebhookRequestData[lemonsqueezy.Subscription, lemonsqueezy.ApiResponseRelationshipsSubscription]{
			ID: "1",
			Attributes: lemonsqueezy.Subscription{
				RenewsAt:  time.Now(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}

	sub := subscriptions.MapLemonsqueezyToSubscription(req)
	assert.Equal(t, "", sub.UserID)
}

// --- Webhook handler tests ---

func webhookPayload(t *testing.T, eventName string, firstItem *lemonsqueezy.SubscriptionFirstSubscriptionItem) string {
	t.Helper()
	req := lemonsqueezy.WebhookRequestSubscription{
		Meta: lemonsqueezy.WebhookRequestMeta{
			EventName:  eventName,
			CustomData: map[string]any{"user_id": "user-123"},
		},
		Data: lemonsqueezy.WebhookRequestData[lemonsqueezy.Subscription, lemonsqueezy.ApiResponseRelationshipsSubscription]{
			ID: "42",
			Attributes: lemonsqueezy.Subscription{
				CustomerID:            100,
				ProductID:             300,
				Status:                "active",
				StatusFormatted:       "Active",
				FirstSubscriptionItem: firstItem,
				RenewsAt:              time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC),
				CreatedAt:             time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt:             time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	return string(b)
}

var testFirstItem = &lemonsqueezy.SubscriptionFirstSubscriptionItem{
	ID:               999,
	SubscriptionItem: lemonsqueezy.SubscriptionItem{PriceID: 555},
}

func validWebhookPayload(t *testing.T, eventName string) string {
	t.Helper()
	return webhookPayload(t, eventName, testFirstItem)
}

func TestRouteWebhook_MissingSignature(t *testing.T) {
	verifier := mocks.NewMockWebhookVerifier(t)
	writer := mocks.NewMockSubscriptionWriter(t)
	observer := mocks.NewMockSubscriptionObserver(t)

	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)
	handler := route.Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/webhook/lemonsqueezy", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRouteWebhook_InvalidSignature(t *testing.T) {
	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "bad-sig", mock.Anything).Return(false)

	writer := mocks.NewMockSubscriptionWriter(t)
	observer := mocks.NewMockSubscriptionObserver(t)

	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)
	handler := route.Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/webhook/lemonsqueezy", strings.NewReader("{}"))
	req.Header.Set("X-Signature", "bad-sig")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRouteWebhook_InvalidEventName(t *testing.T) {
	body := validWebhookPayload(t, "subscription_created")

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	observer := mocks.NewMockSubscriptionObserver(t)

	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)
	handler := route.Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/webhook/lemonsqueezy", strings.NewReader(body))
	req.Header.Set("X-Signature", "valid-sig")
	req.Header.Set("X-Event-Name", "order_created")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRouteWebhook_InvalidPayload(t *testing.T) {
	invalidBody := "not-json"

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(invalidBody)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	observer := mocks.NewMockSubscriptionObserver(t)

	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)
	handler := route.Handler()

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/webhook/lemonsqueezy",
		strings.NewReader(invalidBody),
	)
	req.Header.Set("X-Signature", "valid-sig")
	req.Header.Set("X-Event-Name", "subscription_created")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRouteWebhook_MissingPriceID(t *testing.T) {
	body := webhookPayload(t, "subscription_created", nil)

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	observer := mocks.NewMockSubscriptionObserver(t)
	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)
	handler := route.Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/webhook/lemonsqueezy", strings.NewReader(body))
	req.Header.Set("X-Signature", "valid-sig")
	req.Header.Set("X-Event-Name", "subscription_created")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRouteWebhook_PriceFetchError(t *testing.T) {
	body := validWebhookPayload(t, "subscription_created")

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	observer := mocks.NewMockSubscriptionObserver(t)
	pricing := mocks.NewMockPriceFetcher(t)
	pricing.EXPECT().GetPrice(mock.Anything, 555).Return(nil, assert.AnError)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)
	handler := route.Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/webhook/lemonsqueezy", strings.NewReader(body))
	req.Header.Set("X-Signature", "valid-sig")
	req.Header.Set("X-Event-Name", "subscription_created")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRouteWebhook_UpsertError(t *testing.T) {
	body := validWebhookPayload(t, "subscription_created")

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	writer.EXPECT().UpsertSubscription(mock.Anything, mock.Anything).Return(assert.AnError)

	observer := mocks.NewMockSubscriptionObserver(t)

	pricing := mocks.NewMockPriceFetcher(t)
	pricing.EXPECT().GetPrice(mock.Anything, 555).Return(&subscriptions.PriceInfo{UnitPrice: 999}, nil)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)
	handler := route.Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/webhook/lemonsqueezy", strings.NewReader(body))
	req.Header.Set("X-Signature", "valid-sig")
	req.Header.Set("X-Event-Name", "subscription_created")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRouteWebhook_ObserverError(t *testing.T) {
	body := validWebhookPayload(t, "subscription_created")

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	writer.EXPECT().UpsertSubscription(mock.Anything, mock.Anything).Return(nil)

	observer := mocks.NewMockSubscriptionObserver(t)
	observer.EXPECT().
		OnSubscriptionChange(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(assert.AnError)

	pricing := mocks.NewMockPriceFetcher(t)
	pricing.EXPECT().GetPrice(mock.Anything, 555).Return(&subscriptions.PriceInfo{UnitPrice: 999}, nil)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)
	handler := route.Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/webhook/lemonsqueezy", strings.NewReader(body))
	req.Header.Set("X-Signature", "valid-sig")
	req.Header.Set("X-Event-Name", "subscription_created")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRouteWebhook_Success_Created(t *testing.T) {
	body := validWebhookPayload(t, "subscription_created")

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	writer.EXPECT().UpsertSubscription(mock.Anything, mock.Anything).Return(nil)

	observer := mocks.NewMockSubscriptionObserver(t)
	observer.EXPECT().
		OnSubscriptionChange(mock.Anything, "user-123", 300, true, mock.Anything).
		Return(nil)

	pricing := mocks.NewMockPriceFetcher(t)
	pricing.EXPECT().GetPrice(mock.Anything, 555).Return(&subscriptions.PriceInfo{UnitPrice: 999}, nil)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)
	handler := route.Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/webhook/lemonsqueezy", strings.NewReader(body))
	req.Header.Set("X-Signature", "valid-sig")
	req.Header.Set("X-Event-Name", "subscription_created")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRouteWebhook_Success_Updated(t *testing.T) {
	body := validWebhookPayload(t, "subscription_updated")

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	writer.EXPECT().UpsertSubscription(mock.Anything, mock.Anything).Return(nil)

	observer := mocks.NewMockSubscriptionObserver(t)
	observer.EXPECT().
		OnSubscriptionChange(mock.Anything, "user-123", 300, true, mock.Anything).
		Return(nil)

	pricing := mocks.NewMockPriceFetcher(t)
	pricing.EXPECT().GetPrice(mock.Anything, 555).Return(&subscriptions.PriceInfo{UnitPrice: 999}, nil)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)
	handler := route.Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/webhook/lemonsqueezy", strings.NewReader(body))
	req.Header.Set("X-Signature", "valid-sig")
	req.Header.Set("X-Event-Name", "subscription_updated")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- Refund webhook tests ---

func paymentRefundedPayload(t *testing.T, subscriptionID int, refundedAt *time.Time) string {
	t.Helper()
	req := lemonsqueezy.WebhookRequestSubscriptionInvoice{
		Meta: lemonsqueezy.WebhookRequestMeta{
			EventName: lemonsqueezy.WebhookEventSubscriptionPaymentRefunded,
		},
		Data: lemonsqueezy.WebhookRequestData[lemonsqueezy.SubscriptionInvoiceAttributes, lemonsqueezy.APIResponseRelationshipsSubscriptionInvoice]{
			ID: "777",
			Attributes: lemonsqueezy.SubscriptionInvoiceAttributes{
				SubscriptionID: subscriptionID,
				BillingReason:  "renewal",
				Status:         "refunded",
				Refunded:       true,
				RefundedAt:     refundedAt,
			},
		},
	}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	return string(b)
}

func orderRefundedPayload(t *testing.T, orderID string, refundedAt *time.Time) string {
	t.Helper()
	req := lemonsqueezy.WebhookRequestOrder{
		Meta: lemonsqueezy.WebhookRequestMeta{
			EventName:  lemonsqueezy.WebhookEventOrderRefunded,
			CustomData: map[string]any{"user_id": "user-123"},
		},
		Data: lemonsqueezy.WebhookRequestData[lemonsqueezy.OrderAttributes, lemonsqueezy.APIResponseRelationshipsOrder]{
			ID: orderID,
			Attributes: lemonsqueezy.OrderAttributes{
				Status:     "refunded",
				Refunded:   true,
				RefundedAt: refundedAt,
			},
		},
	}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	return string(b)
}

// refundedSub is what the repo hands back after stamping refunded_at: the
// provider status is untouched, so only RefundedAt makes it inactive.
func refundedSub(at int64) *subscriptions.Subscription {
	return &subscriptions.Subscription{
		ID:         42,
		UserID:     "user-123",
		ProductID:  300,
		OrderID:    2042,
		Status:     "active",
		RefundedAt: &at,
	}
}

func refundRequest(t *testing.T, body, eventName string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhook/lemonsqueezy", strings.NewReader(body))
	req.Header.Set("X-Signature", "valid-sig")
	req.Header.Set("X-Event-Name", eventName)
	return req
}

func TestRouteWebhook_PaymentRefunded_Success(t *testing.T) {
	refundTime := time.Date(2024, 8, 1, 12, 0, 0, 0, time.UTC)
	body := paymentRefundedPayload(t, 42, &refundTime)

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	writer.EXPECT().
		MarkRefundedBySubscriptionID(mock.Anything, 42, refundTime.Unix()).
		Return(refundedSub(refundTime.Unix()), nil)

	observer := mocks.NewMockSubscriptionObserver(t)
	observer.EXPECT().
		OnSubscriptionChange(mock.Anything, "user-123", 300, false, mock.Anything).
		Return(nil)

	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)

	w := httptest.NewRecorder()
	route.Handler().ServeHTTP(w, refundRequest(t, body, lemonsqueezy.WebhookEventSubscriptionPaymentRefunded))

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRouteWebhook_OrderRefunded_Success(t *testing.T) {
	refundTime := time.Date(2024, 8, 1, 12, 0, 0, 0, time.UTC)
	body := orderRefundedPayload(t, "2042", &refundTime)

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	writer.EXPECT().
		MarkRefundedByOrderID(mock.Anything, 2042, refundTime.Unix()).
		Return(refundedSub(refundTime.Unix()), nil)

	observer := mocks.NewMockSubscriptionObserver(t)
	observer.EXPECT().
		OnSubscriptionChange(mock.Anything, "user-123", 300, false, mock.Anything).
		Return(nil)

	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)

	w := httptest.NewRecorder()
	route.Handler().ServeHTTP(w, refundRequest(t, body, lemonsqueezy.WebhookEventOrderRefunded))

	assert.Equal(t, http.StatusOK, w.Code)
}

// refunded_at doubles as the revocation flag, so a payload without a timestamp
// must still produce one rather than leaving the column NULL.
func TestRouteWebhook_PaymentRefunded_MissingRefundedAt(t *testing.T) {
	body := paymentRefundedPayload(t, 42, nil)

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	var gotAt int64
	writer := mocks.NewMockSubscriptionWriter(t)
	writer.EXPECT().
		MarkRefundedBySubscriptionID(mock.Anything, 42, mock.Anything).
		Run(func(_ context.Context, _ int, at int64) { gotAt = at }).
		Return(refundedSub(1), nil)

	observer := mocks.NewMockSubscriptionObserver(t)
	observer.EXPECT().
		OnSubscriptionChange(mock.Anything, "user-123", 300, false, mock.Anything).
		Return(nil)

	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)

	before := time.Now().Unix()
	w := httptest.NewRecorder()
	route.Handler().ServeHTTP(w, refundRequest(t, body, lemonsqueezy.WebhookEventSubscriptionPaymentRefunded))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.GreaterOrEqual(t, gotAt, before)
	assert.LessOrEqual(t, gotAt, time.Now().Unix())
}

// A refund matching no row answers 500 on purpose: webhook delivery is not
// ordered, so the provider must retry rather than have the refund dropped.
func TestRouteWebhook_PaymentRefunded_UnknownSubscription(t *testing.T) {
	refundTime := time.Date(2024, 8, 1, 12, 0, 0, 0, time.UTC)
	body := paymentRefundedPayload(t, 42, &refundTime)

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	writer.EXPECT().
		MarkRefundedBySubscriptionID(mock.Anything, 42, refundTime.Unix()).
		Return(nil, nil)

	observer := mocks.NewMockSubscriptionObserver(t)
	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)

	w := httptest.NewRecorder()
	route.Handler().ServeHTTP(w, refundRequest(t, body, lemonsqueezy.WebhookEventSubscriptionPaymentRefunded))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRouteWebhook_OrderRefunded_UnknownOrder(t *testing.T) {
	refundTime := time.Date(2024, 8, 1, 12, 0, 0, 0, time.UTC)
	body := orderRefundedPayload(t, "2042", &refundTime)

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	writer.EXPECT().
		MarkRefundedByOrderID(mock.Anything, 2042, refundTime.Unix()).
		Return(nil, nil)

	observer := mocks.NewMockSubscriptionObserver(t)
	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)

	w := httptest.NewRecorder()
	route.Handler().ServeHTTP(w, refundRequest(t, body, lemonsqueezy.WebhookEventOrderRefunded))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRouteWebhook_PaymentRefunded_RepoError(t *testing.T) {
	refundTime := time.Date(2024, 8, 1, 12, 0, 0, 0, time.UTC)
	body := paymentRefundedPayload(t, 42, &refundTime)

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	writer.EXPECT().
		MarkRefundedBySubscriptionID(mock.Anything, 42, refundTime.Unix()).
		Return(nil, assert.AnError)

	observer := mocks.NewMockSubscriptionObserver(t)
	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)

	w := httptest.NewRecorder()
	route.Handler().ServeHTTP(w, refundRequest(t, body, lemonsqueezy.WebhookEventSubscriptionPaymentRefunded))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRouteWebhook_PaymentRefunded_ObserverError(t *testing.T) {
	refundTime := time.Date(2024, 8, 1, 12, 0, 0, 0, time.UTC)
	body := paymentRefundedPayload(t, 42, &refundTime)

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	writer.EXPECT().
		MarkRefundedBySubscriptionID(mock.Anything, 42, refundTime.Unix()).
		Return(refundedSub(refundTime.Unix()), nil)

	observer := mocks.NewMockSubscriptionObserver(t)
	observer.EXPECT().
		OnSubscriptionChange(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(assert.AnError)

	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)

	w := httptest.NewRecorder()
	route.Handler().ServeHTTP(w, refundRequest(t, body, lemonsqueezy.WebhookEventSubscriptionPaymentRefunded))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRouteWebhook_PaymentRefunded_InvalidPayload(t *testing.T) {
	body := "not-json"

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	observer := mocks.NewMockSubscriptionObserver(t)
	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)

	w := httptest.NewRecorder()
	route.Handler().ServeHTTP(w, refundRequest(t, body, lemonsqueezy.WebhookEventSubscriptionPaymentRefunded))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// The order ID is a string in the payload and an integer column here.
func TestRouteWebhook_OrderRefunded_InvalidOrderID(t *testing.T) {
	refundTime := time.Date(2024, 8, 1, 12, 0, 0, 0, time.UTC)
	body := orderRefundedPayload(t, "not-a-number", &refundTime)

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	observer := mocks.NewMockSubscriptionObserver(t)
	pricing := mocks.NewMockPriceFetcher(t)
	route := subscriptions.NewRouteWebhook(verifier, pricing, writer, observer, false)

	w := httptest.NewRecorder()
	route.Handler().ServeHTTP(w, refundRequest(t, body, lemonsqueezy.WebhookEventOrderRefunded))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
