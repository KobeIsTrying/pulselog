package auth

import (
	"net"
	"net/http"
	"strings"
)

func APIKeyFromRequest(r *http.Request) string {
	if k := strings.TrimSpace(r.Header.Get("X-API-Key")); k != "" {
		return k
	}
	tok := bearerToken(r)
	if LooksLikeAPIKey(tok) {
		return tok
	}
	return ""
}

func BearerToken(r *http.Request) string {
	tok := bearerToken(r)
	if LooksLikeAPIKey(tok) {
		return ""
	}
	return tok
}

func bearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) < 8 || !strings.EqualFold(h[:7], "bearer ") {
		return ""
	}
	return strings.TrimSpace(h[7:])
}

func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
