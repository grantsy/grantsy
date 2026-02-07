package httptools_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/stretchr/testify/assert"
)

func TestHidden_PassTrue(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	mw := httptools.Hidden(func(r *http.Request) bool { return true }, http.StatusNotFound)
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped.ServeHTTP(w, r)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHidden_PassFalse_404(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	mw := httptools.Hidden(func(r *http.Request) bool { return false }, http.StatusNotFound)
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped.ServeHTTP(w, r)

	assert.False(t, handlerCalled)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHidden_PassFalse_403(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	mw := httptools.Hidden(func(r *http.Request) bool { return false }, http.StatusForbidden)
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped.ServeHTTP(w, r)

	assert.False(t, handlerCalled)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
