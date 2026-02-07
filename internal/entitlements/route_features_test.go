package entitlements_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/swaggest/openapi-go/openapi3"

	"github.com/grantsy/grantsy/internal/entitlements"
	"github.com/grantsy/grantsy/internal/entitlements/mocks"
	"github.com/grantsy/grantsy/internal/httptools"

	_ "github.com/grantsy/grantsy/internal/infra/validation"
)

func newFeaturesMux(t *testing.T) *http.ServeMux {
	t.Helper()
	loader := mocks.NewMockSubscriptionLoader(t)
	loader.EXPECT().GetActiveUserPlans(mock.Anything).Return(map[string]int{"prouser": 100}, nil)

	svc := newTestService(t, loader, nil)
	route := entitlements.NewRouteFeatures(svc)
	mux := http.NewServeMux()
	route.Register(mux, openapi3.NewReflector())
	return mux
}

func TestRouteFeatures_WithSubscription(t *testing.T) {
	mux := newFeaturesMux(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/features?user_id=prouser", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httptools.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp.Data.(map[string]any)
	assert.Equal(t, "prouser", data["user_id"])
	assert.Equal(t, "pro", data["plan"])
	features := data["features"].([]any)
	assert.Len(t, features, 3)
}

func TestRouteFeatures_DefaultPlan(t *testing.T) {
	mux := newFeaturesMux(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/features?user_id=freeuser", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httptools.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp.Data.(map[string]any)
	assert.Equal(t, "free", data["plan"])
	features := data["features"].([]any)
	assert.Len(t, features, 1)
}

func TestRouteFeatures_MissingUserID(t *testing.T) {
	mux := newFeaturesMux(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/features", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
