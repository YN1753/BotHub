package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, dst any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes)).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return false
	}
	return true
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, dst any) error {
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes)).Decode(dst)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func writeSessionCookie(w http.ResponseWriter, r *http.Request, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func startSSEStream(w http.ResponseWriter) (http.Flusher, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "不支持流式连接")
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})
	w.WriteHeader(http.StatusOK)
	return flusher, true
}

func (s *Server) ensureAppBearerToken() (string, error) {
	appToken, _ := s.store.GetConfig(store.KeyAppBearerToken)
	appToken = strings.TrimSpace(appToken)
	if appToken != "" {
		return appToken, nil
	}
	generated, err := store.GenerateAppBearerToken()
	if err != nil {
		return "", err
	}
	if err := s.store.SetConfig(store.KeyAppBearerToken, generated); err != nil {
		return "", err
	}
	return generated, nil
}

func validateHTTPURL(raw string) bool {
	val := strings.TrimSpace(raw)
	if val == "" {
		return true
	}
	parsed, err := url.Parse(val)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
