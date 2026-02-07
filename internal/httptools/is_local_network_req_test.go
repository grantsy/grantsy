package httptools_test

import (
	"net/http/httptest"
	"testing"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/stretchr/testify/assert"
)

func TestIsLocalNetworkReq_NoProxyHeaders(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	assert.True(t, httptools.IsLocalNetworkReq(r))
}

func TestIsLocalNetworkReq_WithXRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "1.2.3.4")
	assert.False(t, httptools.IsLocalNetworkReq(r))
}

func TestIsLocalNetworkReq_WithXForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	assert.False(t, httptools.IsLocalNetworkReq(r))
}

func TestIsLocalNetworkReq_BothHeaders(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "1.2.3.4")
	r.Header.Set("X-Forwarded-For", "5.6.7.8")
	assert.False(t, httptools.IsLocalNetworkReq(r))
}
