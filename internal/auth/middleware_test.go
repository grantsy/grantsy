package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grantsy/grantsy/internal/auth"
	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAPIKey = "test-secret-key"

func setupMiddleware() http.Handler {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return auth.Middleware(testAPIKey)(next)
}

func TestMiddleware_MissingKey(t *testing.T) {
	handler := setupMiddleware()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp httptools.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "Missing API key", resp.Error.Detail)
}

func TestMiddleware_InvalidKey(t *testing.T) {
	handler := setupMiddleware()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", "wrong-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp httptools.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "Invalid API key", resp.Error.Detail)
}

func TestMiddleware_ValidKey(t *testing.T) {
	handler := setupMiddleware()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", testAPIKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddleware_EmptyKeyInHeader(t *testing.T) {
	handler := setupMiddleware()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", "")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp httptools.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "Missing API key", resp.Error.Detail)
}
