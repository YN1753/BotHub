package server

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{
		"error": msg,
		"ok":    false,
	})
}

func remoteHost(remoteAddr string) string {
	trimmed := strings.TrimSpace(remoteAddr)
	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		return host
	}
	return strings.Trim(trimmed, "[]")
}

func isLoopbackPeer(remoteAddr string) bool {
	ip := net.ParseIP(remoteHost(remoteAddr))
	return ip != nil && ip.IsLoopback()
}

func isTrustedProxyPeer(remoteAddr string) bool {
	ip := net.ParseIP(remoteHost(remoteAddr))
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

func hasProxyForwardHeaders(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("CF-Connecting-IP")) != "" ||
		strings.TrimSpace(r.Header.Get("X-Forwarded-For")) != "" ||
		strings.TrimSpace(r.Header.Get("X-Real-IP")) != "" ||
		strings.TrimSpace(r.Header.Get("Cf-Ray")) != ""
}

func isDirectLoopbackRequest(r *http.Request) bool {
	return isLoopbackPeer(r.RemoteAddr) && !hasProxyForwardHeaders(r)
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if xfp := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); xfp != "" {
		first := strings.TrimSpace(strings.Split(xfp, ",")[0])
		if strings.EqualFold(first, "https") {
			return true
		}
	}
	if cfVisitor := strings.ToLower(r.Header.Get("Cf-Visitor")); strings.Contains(cfVisitor, `"https"`) {
		return true
	}
	return false
}

func clientIP(r *http.Request) string {
	if isTrustedProxyPeer(r.RemoteAddr) {
		if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
			return cf
		}
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			for _, part := range strings.Split(xff, ",") {
				if candidate := strings.TrimSpace(part); candidate != "" {
					return candidate
				}
			}
		}
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			return xri
		}
	}
	return remoteHost(r.RemoteAddr)
}
