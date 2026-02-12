package entitlements_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/swaggest/openapi-go/openapi3"

	"github.com/grantsy/grantsy/internal/entitlements"
	"github.com/grantsy/grantsy/internal/httptools"

	_ "github.com/grantsy/grantsy/internal/infra/validation"
)

func newFeaturesMux(t *testing.T) *http.ServeMux {
	t.Helper()
	svc := newTestService(t, newEmptyLoader(t), nil)
	route := entitlements.NewRouteFeatures(svc)
	mux := http.NewServeMux()
	route.Register(mux, openapi3.NewReflector())
	return mux
}

func TestRouteFeatures_ListAll(t *testing.T) {
	mux := newFeaturesMux(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/features", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp httptools.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp.Data.(map[string]any)
	features := data["features"].([]any)
	assert.Len(t, features, 3)

	firstFeature := features[0].(map[string]any)
	assert.Equal(t, "dashboard", firstFeature["id"])
	assert.Equal(t, "Dashboard", firstFeature["name"])
}
