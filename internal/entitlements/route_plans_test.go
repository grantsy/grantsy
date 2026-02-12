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

func newPlansMux(t *testing.T, pricing entitlements.PricingProvider) *http.ServeMux {
	t.Helper()
	svc := newTestService(t, newEmptyLoader(t), nil)
	route := entitlements.NewRoutePlans(svc, pricing)
	mux := http.NewServeMux()
	route.Register(mux, openapi3.NewReflector())
	return mux
}

func TestRoutePlans_BasicList(t *testing.T) {
	pricing := mocks.NewMockPricingProvider(t)
	pricing.EXPECT().GetPlanVariants("free").Return(nil)
	pricing.EXPECT().GetPlanVariants("pro").Return([]entitlements.Variant{
		{ID: 1, Name: "Monthly", Price: 999, Interval: "month", IntervalCount: 1, Sort: 1},
	})

	mux := newPlansMux(t, pricing)

	req := httptest.NewRequest(http.MethodGet, "/v1/plans", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httptools.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp.Data.(map[string]any)
	plans := data["plans"].([]any)
	require.Len(t, plans, 2)

	proPlan := plans[1].(map[string]any)
	assert.Equal(t, "pro", proPlan["id"])
	assert.Equal(t, "Pro", proPlan["name"])

	// Features excluded by default
	_, hasFeatures := proPlan["features"]
	assert.False(t, hasFeatures, "features should not be included without expand=features")

	variants := proPlan["variants"].([]any)
	require.Len(t, variants, 1)
	assert.Equal(t, "Monthly", variants[0].(map[string]any)["name"])
}

func TestRoutePlans_ExpandFeatures(t *testing.T) {
	pricing := mocks.NewMockPricingProvider(t)
	pricing.EXPECT().GetPlanVariants(mock.Anything).Return(nil)

	mux := newPlansMux(t, pricing)

	req := httptest.NewRequest(http.MethodGet, "/v1/plans?expand=features", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httptools.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp.Data.(map[string]any)
	plans := data["plans"].([]any)
	require.Len(t, plans, 2)

	proPlan := plans[1].(map[string]any)
	features := proPlan["features"].([]any)
	require.Len(t, features, 3)
	assert.Equal(t, "dashboard", features[0].(map[string]any)["id"])
}

func TestRoutePlans_EmptyVariants(t *testing.T) {
	pricing := mocks.NewMockPricingProvider(t)
	pricing.EXPECT().GetPlanVariants(mock.Anything).Return(nil)

	mux := newPlansMux(t, pricing)

	req := httptest.NewRequest(http.MethodGet, "/v1/plans", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httptools.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp.Data.(map[string]any)
	plans := data["plans"].([]any)
	for _, p := range plans {
		plan := p.(map[string]any)
		// variants should be nil/absent when pricing returns nil
		_, hasVariants := plan["variants"]
		assert.False(t, hasVariants, "plan %s should not have variants", plan["id"])
	}
}
