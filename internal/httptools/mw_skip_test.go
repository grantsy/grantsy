package httptools_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/stretchr/testify/assert"
)

// testMarkerMiddleware sets a header "X-MW-Applied" to "true" when applied.
func testMarkerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-MW-Applied", "true")
		next.ServeHTTP(w, r)
	})
}

func TestSkip_MatchingPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := httptools.Skip(testMarkerMiddleware, "/healthz")
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	wrapped.ServeHTTP(w, r)

	assert.Empty(t, w.Header().Get("X-MW-Applied"))
}

func TestSkip_NonMatchingPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := httptools.Skip(testMarkerMiddleware, "/healthz")
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/check", nil)
	wrapped.ServeHTTP(w, r)

	assert.Equal(t, "true", w.Header().Get("X-MW-Applied"))
}

func TestSkip_MultipleSkipPaths(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := httptools.Skip(testMarkerMiddleware, "/healthz", "/metrics")

	tests := []struct {
		path    string
		applied bool
	}{
		{"/healthz", false},
		{"/metrics", false},
		{"/v1/check", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			wrapped := mw(handler)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)
			wrapped.ServeHTTP(w, r)

			if tt.applied {
				assert.Equal(t, "true", w.Header().Get("X-MW-Applied"))
			} else {
				assert.Empty(t, w.Header().Get("X-MW-Applied"))
			}
		})
	}
}

func TestSkip_GlobPattern(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := httptools.Skip(testMarkerMiddleware, "/v1/*")
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/check", nil)
	wrapped.ServeHTTP(w, r)

	assert.Empty(t, w.Header().Get("X-MW-Applied"))
}

func TestSkip_EmptyPaths(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := httptools.Skip(testMarkerMiddleware)
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/anything", nil)
	wrapped.ServeHTTP(w, r)

	assert.Equal(t, "true", w.Header().Get("X-MW-Applied"))
}
