package subscriptions_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/swaggest/openapi-go/openapi3"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/grantsy/grantsy/internal/subscriptions"
	"github.com/grantsy/grantsy/internal/subscriptions/mocks"

	_ "github.com/grantsy/grantsy/internal/infra/validation"
)

func newSubscriptionMux(
	t *testing.T,
	reader *mocks.MockSubscriptionReader,
	planProvider *mocks.MockPlanProvider,
) *http.ServeMux {
	t.Helper()
	route := subscriptions.NewRouteSubscription(reader, planProvider)
	mux := http.NewServeMux()
	route.Register(mux, openapi3.NewReflector())
	return mux
}

func TestRouteSubscription_WithActiveSubscription(t *testing.T) {
	trialEnds := int64(1719792000)
	renewsAt := int64(1721001600)

	reader := mocks.NewMockSubscriptionReader(t)
	reader.EXPECT().GetSubscriptionByUserID(mock.Anything, "user-123").Return(&subscriptions.Subscription{
		ID:          42,
		UserID:      "user-123",
		ProductID:   300,
		Status:      "active",
		TrialEndsAt: &trialEnds,
		RenewsAt:    renewsAt,
		Cancelled:   false,
	}, nil)

	planProvider := mocks.NewMockPlanProvider(t)
	planProvider.EXPECT().GetUserPlan("user-123").Return("pro")
	planProvider.EXPECT().GetUserFeatures("user-123").Return([]string{"dashboard", "api", "sso"})

	mux := newSubscriptionMux(t, reader, planProvider)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscription?user_id=user-123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httptools.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp.Data.(map[string]any)
	assert.Equal(t, "user-123", data["user_id"])
	assert.Equal(t, "pro", data["plan"])
	assert.Equal(t, "active", data["status"])
	assert.Equal(t, true, data["has_subscription"])
	assert.Equal(t, false, data["cancelled"])
	assert.ElementsMatch(t, []any{"dashboard", "api", "sso"}, data["features"])
	assert.Equal(t, float64(trialEnds), data["trial_ends_at"])
	assert.Equal(t, float64(renewsAt), data["renews_at"])
}

func TestRouteSubscription_NoSubscription(t *testing.T) {
	reader := mocks.NewMockSubscriptionReader(t)
	reader.EXPECT().GetSubscriptionByUserID(mock.Anything, "user-456").Return(nil, sql.ErrNoRows)

	planProvider := mocks.NewMockPlanProvider(t)
	planProvider.EXPECT().GetUserPlan("user-456").Return("free")
	planProvider.EXPECT().GetUserFeatures("user-456").Return([]string{"dashboard"})

	mux := newSubscriptionMux(t, reader, planProvider)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscription?user_id=user-456", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httptools.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp.Data.(map[string]any)
	assert.Equal(t, false, data["has_subscription"])
	assert.Equal(t, "free", data["plan"])
	assert.Equal(t, "active", data["status"]) // default status when no subscription
}

func TestRouteSubscription_RepoError(t *testing.T) {
	reader := mocks.NewMockSubscriptionReader(t)
	reader.EXPECT().GetSubscriptionByUserID(mock.Anything, "user-789").Return(nil, errors.New("db connection lost"))

	planProvider := mocks.NewMockPlanProvider(t)

	mux := newSubscriptionMux(t, reader, planProvider)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscription?user_id=user-789", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRouteSubscription_MissingUserID(t *testing.T) {
	reader := mocks.NewMockSubscriptionReader(t)
	planProvider := mocks.NewMockPlanProvider(t)

	mux := newSubscriptionMux(t, reader, planProvider)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscription", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestRouteSubscription_CancelledSubscription(t *testing.T) {
	renewsAt := int64(1721001600)

	reader := mocks.NewMockSubscriptionReader(t)
	reader.EXPECT().GetSubscriptionByUserID(mock.Anything, "user-cancel").Return(&subscriptions.Subscription{
		ID:        42,
		UserID:    "user-cancel",
		ProductID: 300,
		Status:    "cancelled",
		RenewsAt:  renewsAt,
		Cancelled: true,
	}, nil)

	planProvider := mocks.NewMockPlanProvider(t)
	planProvider.EXPECT().GetUserPlan("user-cancel").Return("pro")
	planProvider.EXPECT().GetUserFeatures("user-cancel").Return([]string{"dashboard", "api"})

	mux := newSubscriptionMux(t, reader, planProvider)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscription?user_id=user-cancel", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httptools.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp.Data.(map[string]any)
	assert.Equal(t, true, data["cancelled"])
	assert.Equal(t, "cancelled", data["status"])
	assert.Equal(t, true, data["has_subscription"])
}
