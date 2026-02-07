package httptools

import "net/http"

func IsLocalNetworkReq(r *http.Request) bool {
	// If either header is present, we consider the request as coming from outside
	if r.Header.Get("X-Real-IP") != "" || r.Header.Get("X-Forwarded-For") != "" {
		return false
	}

	// Otherwise, we consider it as a local network request
	return true
}
