package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

// handleV2AuthToken exchanges the admin password (or existing app token) for the persistent App Bearer Token.
func (s *Server) handleV2AuthToken(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.limiter != nil && s.limiter.isLocked(ip) {
		writeError(w, http.StatusTooManyRequests, "尝试过于频繁，请 3 分钟后再试")
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}

	secret := strings.TrimSpace(body.Password)
	if !s.verifyPassword(secret) && !s.verifyAppToken(secret) {
		if s.limiter != nil {
			s.limiter.recordFail(ip)
		}
		writeError(w, http.StatusUnauthorized, "密钥验证失败")
		return
	}
	if s.limiter != nil {
		s.limiter.recordSuccess(ip)
	}

	appToken, _ := s.store.GetConfig(store.KeyAppBearerToken)
	appToken = strings.TrimSpace(appToken)
	if appToken == "" {
		appToken = secret
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"token":   appToken,
		"version": s.version,
	})
}

// handleV2Events streams real-time notifications via SSE and replays missed notifications when last_event_id > 0.
func (s *Server) handleV2Events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "不支持流式连接")
		return
	}

	var lastEventID int64
	if raw := strings.TrimSpace(r.URL.Query().Get("last_event_id")); raw != "" {
		lastEventID, _ = strconv.ParseInt(raw, 10, 64)
	} else if rawHeader := strings.TrimSpace(r.Header.Get("Last-Event-ID")); rawHeader != "" {
		lastEventID, _ = strconv.ParseInt(rawHeader, 10, 64)
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	latestID := s.store.LatestNotificationID()
	initData, _ := json.Marshal(map[string]any{
		"ok":           true,
		"latest_id":    latestID,
		"unread_count": s.store.CountUnreadNotifications(),
		"server_now":   store.NowShanghai(),
	})
	_, _ = fmt.Fprintf(w, "event: connected\ndata: %s\n\n", string(initData))
	flusher.Flush()

	// Replay any missed notifications since lastEventID (when lastEventID > 0 or catchup=1)
	catchupAll := r.URL.Query().Get("catchup") == "1"
	if lastEventID > 0 || catchupAll {
		missed, err := s.store.ListNotifications(lastEventID, 100, false)
		if err == nil {
			for _, n := range missed {
				b, _ := json.Marshal(n)
				_, _ = fmt.Fprintf(w, "id: %d\nevent: notification\ndata: %s\n\n", n.ID, string(b))
				if n.ID > lastEventID {
					lastEventID = n.ID
				}
			}
			flusher.Flush()
		}
	}

	// If client only requested a one-shot catchup stream (e.g., tests or short poll), allow ?oneshot=1
	if r.URL.Query().Get("oneshot") == "1" {
		return
	}

	if s.push == nil || s.push.Hub() == nil {
		return
	}

	ch, unsubscribe := s.push.Hub().Subscribe()
	defer unsubscribe()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case n, ok := <-ch:
			if !ok {
				return
			}
			if n.ID <= lastEventID {
				continue
			}
			lastEventID = n.ID
			b, _ := json.Marshal(n)
			_, _ = fmt.Fprintf(w, "id: %d\nevent: notification\ndata: %s\n\n", n.ID, string(b))
			flusher.Flush()
		case <-ticker.C:
			hb, _ := json.Marshal(map[string]any{
				"time":      store.NowShanghai(),
				"latest_id": s.store.LatestNotificationID(),
			})
			_, _ = fmt.Fprintf(w, "event: heartbeat\ndata: %s\n\n", string(hb))
			flusher.Flush()
		}
	}
}

func (s *Server) handleV2ListNotifications(w http.ResponseWriter, r *http.Request) {
	var afterID int64
	if raw := strings.TrimSpace(r.URL.Query().Get("after_id")); raw != "" {
		afterID, _ = strconv.ParseInt(raw, 10, 64)
	}
	limit := 50
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if v, err := strconv.Atoi(rawLimit); err == nil && v > 0 {
			limit = v
		}
	}
	unreadOnly := r.URL.Query().Get("unread_only") == "1" || r.URL.Query().Get("unread_only") == "true"

	list, err := s.store.ListNotifications(afterID, limit, unreadOnly)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询收件箱失败: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"notifications": list,
		"unread_count":  s.store.CountUnreadNotifications(),
		"latest_id":     s.store.LatestNotificationID(),
		"server_now":    store.NowShanghai(),
	})
}

func (s *Server) handleV2MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSpace(r.PathValue("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效通知 ID")
		return
	}
	if err := s.store.MarkNotificationRead(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"unread_count": s.store.CountUnreadNotifications(),
	})
}

func (s *Server) handleV2MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if err := s.store.MarkAllNotificationsRead(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"unread_count": 0,
	})
}

// handleNezhaWebhook receives alert callbacks from Nezha Monitoring Dashboard (/api/v1/webhook/nezha).
func (s *Server) handleNezhaWebhook(w http.ResponseWriter, r *http.Request) {
	// Verify webhook token or standard auth
	configuredHookToken, _ := s.store.GetConfig(store.KeyNezhaWebhookToken)
	configuredHookToken = strings.TrimSpace(configuredHookToken)

	providedToken := strings.TrimSpace(r.URL.Query().Get("token"))
	if providedToken == "" {
		providedToken = strings.TrimSpace(r.Header.Get("X-Webhook-Token"))
	}

	authorized := false
	if configuredHookToken != "" && providedToken != "" {
		if subtle.ConstantTimeCompare([]byte(providedToken), []byte(configuredHookToken)) == 1 {
			authorized = true
		}
	}
	if !authorized && s.isAuthenticated(r) {
		authorized = true
	}
	// Also allow localhost loopback from co-located Nezha Dashboard when nezha_webhook_token is not explicitly set
	if !authorized && configuredHookToken == "" {
		ip := clientIP(r)
		if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(r.RemoteAddr, "127.0.0.1:") || strings.HasPrefix(r.RemoteAddr, "[::1]:") {
			authorized = true
		}
	}

	if !authorized {
		writeError(w, http.StatusUnauthorized, "未授权的 Webhook 调用")
		return
	}

	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取请求体失败")
		return
	}

	var payload map[string]any
	title := "哪吒探针节点监控告警"
	content := strings.TrimSpace(string(rawBody))

	if json.Unmarshal(rawBody, &payload) == nil && len(payload) > 0 {
		if t, ok := payload["title"].(string); ok && strings.TrimSpace(t) != "" {
			title = strings.TrimSpace(t)
		} else if sName, ok := payload["server_name"].(string); ok && strings.TrimSpace(sName) != "" {
			title = "哪吒节点告警 · " + strings.TrimSpace(sName)
		}

		for _, key := range []string{"content", "message", "body", "text", "description"} {
			if v, ok := payload[key].(string); ok && strings.TrimSpace(v) != "" {
				content = strings.TrimSpace(v)
				break
			}
		}
	}

	title = notifier.StripEmoji(title)
	content = notifier.StripEmoji(content)
	if content == "" {
		content = "收到哪吒监控回环告警事件，请检查节点状态。"
	}

	now := store.NowShanghai()
	markdown := fmt.Sprintf("### %s\n\n- **告警时间**：`%s`\n- **告警来源**：`哪吒探针 Webhook 回环`\n\n%s\n\n---\n*来自 Vesper BotHub · bot.starryheavens.uk*", title, now, content)

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if s.push != nil {
		_ = s.push.SendWithPriority(ctx, "nezha_webhook", title, markdown, "high")
	} else {
		_, _ = s.store.CreateNotification("nezha_webhook", title, content, markdown, "high")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"latest_id": s.store.LatestNotificationID(),
	})
}

func (s *Server) handleV2AppVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"version":    s.version,
		"commit":     s.commit,
		"build_time": s.buildTime,
		"apk_url":    "https://github.com/YN1753/BotHub/releases/latest/download/vesper-bothub-android.apk",
	})
}
