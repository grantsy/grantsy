package entitlements_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/swaggest/openapi-go/openapi31"

	"github.com/grantsy/grantsy/internal/entitlements"
	"github.com/grantsy/grantsy/internal/entitlements/mocks"
	"github.com/grantsy/grantsy/internal/httptools"

	_ "github.com/grantsy/grantsy/internal/infra/validation"
)

func newCheckMux(t *testing.T) (*http.ServeMux, *entitlements.Service) {
	t.Helper()
	loader := mocks.NewMockSubscriptionLoader(t)
	loader.EXPECT().GetActiveUserPlans(mock.Anything).Return(map[string]int{"prouser": 100}, nil)

	svc := newTestService(t, loader, nil)
	route := entitlements.NewRouteCheck(svc)
	mux := http.NewServeMux()
	route.Register(mux, openapi31.NewReflector())
	return mux, svc
}

func TestRouteCheck_AllowedFeature(t *testing.T) {
	mux, _ := newCheckMux(t)

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/check?user_id=prouser&feature=api&expand=feature&expand=plan",
		nil,
	)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httptools.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp.Data.(map[string]any)
	assert.Equal(t, true, data["allowed"])
	assert.Equal(t, "feature_in_plan", data["reason"])
	assert.Equal(t, "prouser", data["user_id"])

	feature := data["feature"].(map[string]any)
	assert.Equal(t, "api", feature["id"])
	assert.Equal(t, "API", feature["name"])

	plan := data["plan"].(map[string]any)
	assert.Equal(t, "pro", plan["id"])
	assert.Equal(t, "Pro", plan["name"])
}

func TestRouteCheck_AllowedDefaultPlan(t *testing.T) {
	mux, _ := newCheckMux(t)

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/check?user_id=freeuser&feature=dashboard&expand=feature&expand=plan",
		nil,
	)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httptools.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp.Data.(map[string]any)
	assert.Equal(t, true, data["allowed"])
	assert.Equal(t, "default_plan", data["reason"])
}

func TestRouteCheck_DeniedFeature(t *testing.T) {
	mux, _ := newCheckMux(t)

	req := httptest.NewRequest(
		http.MethodGet,
		"/v1/check?user_id=freeuser&feature=api&expand=feature&expand=plan",
		nil,
	)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httptools.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp.Data.(map[string]any)
	assert.Equal(t, false, data["allowed"])
}

func TestRouteCheck_MissingUserID(t *testing.T) {
	mux, _ := newCheckMux(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/check?feature=api", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestRouteCheck_MissingFeature(t *testing.T) {
	mux, _ := newCheckMux(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/check?user_id=user1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestRouteCheck_MissingBothParams(t *testing.T) {
	mux, _ := newCheckMux(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/check", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
