package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
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

func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "bh_" + hex.EncodeToString(b), nil
}

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
		generated, err := generateRandomToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "签发 App Token 失败")
			return
		}
		_ = s.store.SetConfig(store.KeyAppBearerToken, generated)
		appToken = generated
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"token":   appToken,
		"version": s.version,
	})
}

func (s *Server) handleV2RotateAppToken(w http.ResponseWriter, r *http.Request) {
	generated, err := generateRandomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "轮换 App Token 失败")
		return
	}
	if err := s.store.SetConfig(store.KeyAppBearerToken, generated); err != nil {
		writeError(w, http.StatusInternalServerError, "保存新 Token 失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"token":   generated,
		"message": "已生成全新 App Bearer Token，旧令牌已即时失效",
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

	ch, unsubscribe := s.push.Hub().SubscribeEvents()
	defer unsubscribe()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.push.Hub().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if ev.Event == "notification" {
				if n, isNotif := ev.Data.(store.Notification); isNotif {
					if n.ID <= lastEventID {
						continue
					}
					lastEventID = n.ID
					b, _ := json.Marshal(n)
					_, _ = fmt.Fprintf(w, "id: %d\nevent: notification\ndata: %s\n\n", n.ID, string(b))
					flusher.Flush()
					continue
				}
			}
			b, _ := json.Marshal(ev.Data)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, string(b))
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

	// Deduplicate rapid repeated alerts for the same node/title within a 5-minute cooldown window
	dedupKey := strings.ToLower(strings.TrimSpace(title))
	if s.isNezhaAlertInCooldown(dedupKey, 5*time.Minute) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           true,
			"deduplicated": true,
			"latest_id":    s.store.LatestNotificationID(),
		})
		return
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

	// Post alert to Sentinel Bot timeline and update its state to blocked
	sentinelMsg := store.AgentMessage{
		SessionID:   "bot:sentinel",
		BotID:       "sentinel",
		SenderBotID: "sentinel",
		MsgKind:     "routine",
		Role:        "assistant",
		Content:     markdown,
	}
	if mid, mErr := s.store.AppendAgentMessage(sentinelMsg); mErr == nil {
		sentinelMsg.ID = mid
		sentinelMsg.CreatedAt = now
	}
	_ = s.store.SetBotState("sentinel", store.BotStateBlocked, title)
	_ = s.store.IncrementBotUnread("sentinel", title)
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("bot_message", sentinelMsg)
		s.push.Hub().PublishEvent("bot_state", map[string]any{
			"bot_id":       "sentinel",
			"state":        store.BotStateBlocked,
			"last_summary": title,
		})
	}

	if shouldTriggerNezhaAITriage(title, content) && s.agent != nil {
		go s.triageNezhaAlertAsync(title, content)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"latest_id": s.store.LatestNotificationID(),
	})
}

func shouldTriggerNezhaAITriage(title, content string) bool {
	combined := strings.ToLower(title + " " + content)
	for _, kw := range []string{"离线", "宕机", "offline", "down", "timeout", "高负载", "cpu", "内存", "oom"} {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

func (s *Server) triageNezhaAlertAsync(title, content string) {
	lightKey, _ := s.store.GetConfig(store.KeyLightAPIKey)
	mainKey, _ := s.store.GetConfig(store.KeyDeepSeekAPIKey)
	if strings.TrimSpace(lightKey) == "" && strings.TrimSpace(mainKey) == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	nodeSnapshot, _ := s.agent.ExecuteTool(ctx, "query_nezha_nodes", "{}")
	sysPrompt := "你是运维根因诊断助手。结合哪吒探针实时快照与告警内容，给出不超过 3 条、高信息密度的排查建议，严禁使用任何 Emoji。"
	userPrompt := fmt.Sprintf("告警标题：%s\n告警详情：%s\n实时探针快照：%s", title, content, nodeSnapshot)

	analysis, err := s.agent.CallLightModel(ctx, sysPrompt, userPrompt)
	if err != nil || strings.TrimSpace(analysis) == "" {
		return
	}
	triageTitle := notifier.StripEmoji("AI 诊断 · " + title)
	triageMD := fmt.Sprintf("### %s\n\n%s\n\n---\n*由 BotHub 轻量诊断引擎自动生成*", triageTitle, notifier.StripEmoji(analysis))
	if s.push != nil {
		_ = s.push.SendWithPriority(ctx, "nezha_ai_triage", triageTitle, triageMD, "normal")
	} else {
		_, _ = s.store.CreateNotification("nezha_ai_triage", triageTitle, analysis, triageMD, "normal")
	}

	triageMsg := store.AgentMessage{
		SessionID:   "bot:sentinel",
		BotID:       "sentinel",
		SenderBotID: "sentinel",
		MsgKind:     "routine",
		Role:        "assistant",
		Content:     triageMD,
	}
	if mid, mErr := s.store.AppendAgentMessage(triageMsg); mErr == nil {
		triageMsg.ID = mid
		triageMsg.CreatedAt = store.NowShanghai()
	}
	_ = s.store.SetBotState("sentinel", store.BotStateDone, triageTitle)
	_ = s.store.IncrementBotUnread("sentinel", triageTitle)
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("bot_message", triageMsg)
		s.push.Hub().PublishEvent("bot_state", map[string]any{
			"bot_id":       "sentinel",
			"state":        store.BotStateDone,
			"last_summary": triageTitle,
		})
	}
}

func (s *Server) isNezhaAlertInCooldown(key string, window time.Duration) bool {
	if key == "" {
		return false
	}
	s.nezhaAlertMu.Lock()
	defer s.nezhaAlertMu.Unlock()

	now := time.Now()
	if s.nezhaAlertCooldown == nil {
		s.nezhaAlertCooldown = make(map[string]time.Time)
	}
	// Clean up expired keys occasionally
	for k, ts := range s.nezhaAlertCooldown {
		if now.Sub(ts) >= window {
			delete(s.nezhaAlertCooldown, k)
		}
	}
	if last, ok := s.nezhaAlertCooldown[key]; ok && now.Sub(last) < window {
		return true
	}
	s.nezhaAlertCooldown[key] = now
	return false
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
