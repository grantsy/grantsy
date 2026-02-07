package httptools_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/stretchr/testify/assert"
)

func TestWrap_NoMiddleware(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := httptools.Wrap(handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped.ServeHTTP(w, r)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWrap_SingleMiddleware(t *testing.T) {
	var order []string

	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1")
			next.ServeHTTP(w, r)
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})

	wrapped := httptools.Wrap(handler, mw)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped.ServeHTTP(w, r)

	assert.Equal(t, []string{"mw1", "handler"}, order)
}

func TestWrap_MultipleMiddleware_Order(t *testing.T) {
	var order []string

	makeMW := func(name string) httptools.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})

	wrapped := httptools.Wrap(handler, makeMW("mw1"), makeMW("mw2"), makeMW("mw3"))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped.ServeHTTP(w, r)

	assert.Equal(t, []string{"mw1", "mw2", "mw3", "handler"}, order)
}
