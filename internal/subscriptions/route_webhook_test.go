package subscriptions_test

import (
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

func validWebhookPayload(t *testing.T, eventName string) string {
	t.Helper()
	req := lemonsqueezy.WebhookRequestSubscription{
		Meta: lemonsqueezy.WebhookRequestMeta{
			EventName:  eventName,
			CustomData: map[string]any{"user_id": "user-123"},
		},
		Data: lemonsqueezy.WebhookRequestData[lemonsqueezy.Subscription, lemonsqueezy.ApiResponseRelationshipsSubscription]{
			ID: "42",
			Attributes: lemonsqueezy.Subscription{
				CustomerID:      100,
				ProductID:       300,
				Status:          "active",
				StatusFormatted: "Active",
				RenewsAt:        time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC),
				CreatedAt:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt:       time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	return string(b)
}

func TestRouteWebhook_MissingSignature(t *testing.T) {
	verifier := mocks.NewMockWebhookVerifier(t)
	writer := mocks.NewMockSubscriptionWriter(t)
	observer := mocks.NewMockSubscriptionObserver(t)

	route := subscriptions.NewRouteWebhook(verifier, writer, observer)
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

	route := subscriptions.NewRouteWebhook(verifier, writer, observer)
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

	route := subscriptions.NewRouteWebhook(verifier, writer, observer)
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

	route := subscriptions.NewRouteWebhook(verifier, writer, observer)
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

func TestRouteWebhook_UpsertError(t *testing.T) {
	body := validWebhookPayload(t, "subscription_created")

	verifier := mocks.NewMockWebhookVerifier(t)
	verifier.EXPECT().VerifyWebhook(mock.Anything, "valid-sig", []byte(body)).Return(true)

	writer := mocks.NewMockSubscriptionWriter(t)
	writer.EXPECT().UpsertSubscription(mock.Anything, mock.Anything).Return(assert.AnError)

	observer := mocks.NewMockSubscriptionObserver(t)

	route := subscriptions.NewRouteWebhook(verifier, writer, observer)
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

	route := subscriptions.NewRouteWebhook(verifier, writer, observer)
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

	route := subscriptions.NewRouteWebhook(verifier, writer, observer)
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

	route := subscriptions.NewRouteWebhook(verifier, writer, observer)
	handler := route.Handler()

	req := httptest.NewRequest(http.MethodPost, "/v1/webhook/lemonsqueezy", strings.NewReader(body))
	req.Header.Set("X-Signature", "valid-sig")
	req.Header.Set("X-Event-Name", "subscription_updated")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
