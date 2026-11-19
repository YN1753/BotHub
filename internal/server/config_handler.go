package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if !decodeJSON(w, r, 16384, &updates) {
		return
	}

	_ = s.store.SetConfig(store.KeyPushChannel, "native")
	for k, v := range updates {
		spec, ok := store.LookupConfigSpec(k)
		if !ok || !spec.Editable {
			continue
		}
		val := strings.TrimSpace(v)
		if spec.IsURL && !validateHTTPURL(val) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("配置项 %s 必须为有效的 http/https URL", k))
			return
		}
		if spec.Sensitive && strings.Contains(val, "****") {
			continue
		}
		_ = s.store.SetConfig(k, val)
	}

	if s.scheduler != nil {
		if err := s.scheduler.Reload(); err != nil {
			writeError(w, http.StatusBadRequest, "重载调度失败: "+err.Error())
			return
		}
	}
	s.publishAssistantEvent("configs,models")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleTestPush(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	channelName := "原生 APP 通知通道 (Inbox + SSE)"
	now := store.NowShanghai()
	content := fmt.Sprintf("### BotHub 通知链路自检\n\n- **活跃通道**：`%s`\n- **触发时间**：`%s`\n- **链路状态**：正常直连\n\n---\n*来自 BotHub*", channelName, now)
	if err := s.push.SendWithPriority(ctx, "system_test", "BotHub 原生 APP 通知测试", content, "high"); err != nil {
		writeError(w, http.StatusBadGateway, "推送失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "测试通知已成功推送至原生 APP 收件箱与 SSE 实时通道"})
}
