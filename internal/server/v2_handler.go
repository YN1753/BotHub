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
	if !decodeJSON(w, r, 4096, &body) {
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

	appToken, err := s.ensureAppBearerToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "签发 App Token 失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"token":   appToken,
		"version": s.version,
	})
}

func (s *Server) handleV2RotateAppToken(w http.ResponseWriter, r *http.Request) {
	generated, err := store.GenerateAppBearerToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "轮换 App Token 失败")
		return
	}
	if err := s.store.SetConfig(store.KeyAppBearerToken, generated); err != nil {
		writeError(w, http.StatusInternalServerError, "保存新 Token 失败")
		return
	}
	s.KickPushSubscribers()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"token":   generated,
		"message": "已生成全新 App Bearer Token，旧令牌已即时失效",
	})
}

// handleV2Events streams real-time notifications via SSE and replays missed notifications when last_event_id > 0.
func (s *Server) handleV2Events(w http.ResponseWriter, r *http.Request) {
	var lastEventID int64
	if raw := strings.TrimSpace(r.URL.Query().Get("last_event_id")); raw != "" {
		lastEventID, _ = strconv.ParseInt(raw, 10, 64)
	} else if rawHeader := strings.TrimSpace(r.Header.Get("Last-Event-ID")); rawHeader != "" {
		lastEventID, _ = strconv.ParseInt(rawHeader, 10, 64)
	}

	flusher, ok := startSSEStream(w)
	if !ok {
		return
	}

	latestID := s.store.LatestNotificationID()
	initData, _ := json.Marshal(map[string]any{
		"ok":           true,
		"latest_id":    latestID,
		"unread_count": s.store.CountUnreadNotifications(),
		"revisions":    s.revisionsSnapshot(),
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

func (s *Server) handleV2ClearReadNotifications(w http.ResponseWriter, r *http.Request) {
	deleted, err := s.store.ClearReadNotifications()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if deleted > 0 && s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("assistant_update", map[string]any{
			"kind": "notifications_cleared",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"deleted":      deleted,
		"unread_count": s.store.CountUnreadNotifications(),
	})
}

func (s *Server) handleV2DeleteNotification(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSpace(r.PathValue("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效通知 ID")
		return
	}
	deleted, err := s.store.DeleteNotification(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if deleted && s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("assistant_update", map[string]any{
			"kind": "notifications_cleared",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           deleted,
		"unread_count": s.store.CountUnreadNotifications(),
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
	// Also allow direct localhost loopback from co-located Nezha Dashboard when nezha_webhook_token is not explicitly set
	// (requests arriving via local Nginx carry X-Real-IP / X-Forwarded-For and must NOT match direct loopback)
	if !authorized && configuredHookToken == "" && isDirectLoopbackRequest(r) {
		authorized = true
	}

	if !authorized {
		writeError(w, http.StatusUnauthorized, "未授权的 Webhook 调用")
		return
	}

	rawBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "Webhook 请求体超过 64KB 限制或读取失败")
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
	markdown := fmt.Sprintf("### %s\n\n- **告警时间**：`%s`\n- **告警来源**：`哪吒探针 Webhook 回环`\n\n%s\n\n---\n*来自 BotHub*", title, now, content)

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if s.push != nil {
		_ = s.push.SendWithPriority(ctx, "nezha_webhook", title, markdown, "high")
	} else {
		_, _ = s.store.CreateNotification("nezha_webhook", title, content, markdown, "high")
	}

	targetBotID := s.resolveAlertTargetBotID()

	if shouldTriggerNezhaAITriage(title, content) && s.agent != nil {
		go s.triageNezhaAlertAsync(targetBotID, title, content)
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

func (s *Server) triageNezhaAlertAsync(targetBotID, title, content string) {
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
}

func (s *Server) resolveAlertTargetBotID() string {
	bots, _ := s.store.ListBots()
	for _, b := range bots {
		if b.IsGroup {
			continue
		}
		for _, tool := range b.AllowedToolList() {
			if tool == "query_nezha_nodes" {
				return b.ID
			}
		}
	}
	for _, b := range bots {
		if !b.IsGroup {
			return b.ID
		}
	}
	return ""
}

func (s *Server) appendAlertToBot(botID, title, state, markdown string) {
	if botID == "" {
		return
	}
	now := store.NowShanghai()
	msg := store.AgentMessage{
		SessionID:   "bot:" + botID,
		BotID:       botID,
		SenderBotID: botID,
		MsgKind:     "routine",
		Role:        "assistant",
		Content:     markdown,
	}
	if mid, mErr := s.store.AppendAgentMessage(msg); mErr == nil {
		msg.ID = mid
		msg.CreatedAt = now
	}
	_ = s.store.SetBotState(botID, state, title)
	_ = s.store.IncrementBotUnread(botID, title)
	unread := 0
	if b, _ := s.store.GetBot(botID); b != nil {
		unread = b.UnreadCount
	}
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("bot_message", msg)
		s.push.Hub().PublishEvent("bot_state", map[string]any{
			"bot_id":       botID,
			"state":        state,
			"unread_count": unread,
			"last_summary": title,
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
		"ok":          true,
		"version":     s.version,
		"commit":      s.commit,
		"build_time":  s.buildTime,
		"mirror_path": "/api/v2/app/apk",
		"apk_url":     "https://github.com/YN1753/BotHub/releases/latest/download/vesper-bothub-android.apk",
	})
}

// handleV2AppApk streams the latest signed Android APK through the BotHub server so in-app OTA
// updates work fast and reliably on domestic cellular networks without opening a browser or VPN.
func (s *Server) handleV2AppApk(w http.ResponseWriter, r *http.Request) {
	const fallbackLatest = "https://github.com/YN1753/BotHub/releases/latest/download/vesper-bothub-android.apk"
	candidates := make([]string, 0, 2)
	ver := strings.TrimSpace(s.version)
	if ver != "" && ver != "dev" {
		if !strings.HasPrefix(ver, "v") {
			ver = "v" + ver
		}
		candidates = append(candidates, fmt.Sprintf("https://github.com/YN1753/BotHub/releases/download/%s/vesper-bothub-android.apk", ver))
	}
	candidates = append(candidates, fallbackLatest)

	method := http.MethodGet
	if r.Method == http.MethodHead {
		method = http.MethodHead
	}

	client := &http.Client{Timeout: 180 * time.Second}
	var upstream *http.Response
	for _, targetURL := range candidates {
		req, err := http.NewRequestWithContext(r.Context(), method, targetURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "BotHub-OTA-Proxy/2.0")
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			upstream = resp
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	if upstream == nil {
		writeError(w, http.StatusBadGateway, "暂时无法拉取安装包，请稍后重试")
		return
	}
	defer upstream.Body.Close()

	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	w.Header().Set("Content-Disposition", `attachment; filename="bothub-android.apk"`)
	w.Header().Set("Cache-Control", "no-store")
	if upstream.ContentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(upstream.ContentLength, 10))
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, upstream.Body)
	}
}

// handleV2AppBootstrap returns the exact compact mobile launch state (bots, notifications,
// stash, assets, scheduled tasks, and version metadata) in a single HTTP round-trip, eliminating
// 4 separate requests and 10 unused heavy SQLite queries on mobile startup.
func (s *Server) handleV2AppBootstrap(w http.ResponseWriter, r *http.Request) {
	tasks, _ := s.schedulerTasksList()
	bots, _ := s.store.ListBots()
	skills, _ := s.store.ListSkills()
	notifications, _ := s.store.ListNotifications(0, 40, false)
	assets, _ := s.store.ListAssets()
	stash, _ := s.store.SearchStashItemsWithKind("", "all", "", "", 60)
	todos, _ := s.store.ListTodos("all", "", 100)
	notes, _ := s.store.ListNotes("", "", 60)
	providers, pooledModels, poolStrategies := s.store.GetModelConfig()
	usageSummary, _ := s.store.SummarizeModelUsage(7)
	var modelOptions any
	var modelHealth any
	if s.agent != nil {
		modelOptions = s.agent.ModelOptionsPayload()
		modelHealth = s.agent.GetModelHealth()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                  true,
		"version":             s.version,
		"mirror_path":         "/api/v2/app/apk",
		"apk_url":             "https://github.com/YN1753/BotHub/releases/latest/download/vesper-bothub-android.apk",
		"bots":                bots,
		"skills":              skills,
		"notifications":       notifications,
		"stash":               stash,
		"assets":              assets,
		"asset_categories":    s.store.ListAssetCategories(),
		"tasks":               tasks,
		"todos":               todos,
		"notes":               notes,
		"model_pool":          store.MaskModelPool(s.store.GetModelPool()),
		"model_providers":     store.MaskModelProviders(providers),
		"pooled_models":       pooledModels,
		"pool_strategies":     poolStrategies,
		"model_usage_summary": usageSummary,
		"model_options":       modelOptions,
		"model_health":        modelHealth,
		"configs":             s.safeMaskedConfigs(),
		"revisions":           s.revisionsSnapshot(),
		"server_now":          store.NowShanghai(),
	})
}

// handleV2WebSession exchanges an authenticated App Bearer Token request for a 24h web session cookie
// so the native mobile client can seamlessly mount the Vue 3 full-featured console WebView with zero re-login.
func (s *Server) handleV2WebSession(w http.ResponseWriter, r *http.Request) {
	const ttl = 24 * time.Hour
	const maxAge = 24 * 3600
	token, err := s.issueSessionCookieWithTTL(ttl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "签发混合工作台会话失败")
		return
	}
	writeSessionCookie(w, r, token, maxAge)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"cookie_name":  sessionCookieName,
		"cookie_value": token,
		"max_age":      maxAge,
	})
}
