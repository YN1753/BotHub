package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func (s *Server) handleGetBotMessages(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	bot, err := s.store.GetBot(id)
	if err != nil || bot == nil {
		writeError(w, http.StatusNotFound, "Bot 不存在")
		return
	}

	threadID := strings.TrimSpace(r.URL.Query().Get("thread_id"))
	hasThreadParam := r.URL.Query().Has("thread_id")
	if !hasThreadParam {
		threadID = strings.TrimSpace(bot.ActiveThreadID)
	} else if threadID != strings.TrimSpace(bot.ActiveThreadID) {
		_ = s.store.SetBotActiveThread(id, threadID)
		bot.ActiveThreadID = threadID
	}

	msgs, err := s.store.ListBotMessagesByThread(id, threadID, 120)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取 Bot 对话流失败: "+err.Error())
		return
	}
	threads, _ := s.store.ListBotThreads(id)
	memScopes := []string{"bot:" + id}
	if bot.MemoryPolicy != store.MemoryPolicyIsolated {
		memScopes = append(memScopes, "global")
	}
	memories, _ := s.store.ListScopedMemories(memScopes, "", 60)
	executions, _ := s.store.ListBotExecutions(id, 40)

	_ = s.store.ClearBotUnread(id)
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("bot_state", map[string]any{
			"bot_id":       id,
			"state":        store.BotStateIdle,
			"unread_count": 0,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"bot":        bot,
		"thread_id":  threadID,
		"messages":   msgs,
		"threads":    threads,
		"memories":   memories,
		"executions": executions,
	})
}

func (s *Server) handleClearBotMessages(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if r.URL.Query().Has("thread_id") {
		tid := strings.TrimSpace(r.URL.Query().Get("thread_id"))
		if err := s.store.ClearBotMessagesInThread(id, tid); err != nil {
			writeError(w, http.StatusInternalServerError, "清空话题对话流失败: "+err.Error())
			return
		}
	} else if err := s.store.ClearBotMessages(id); err != nil {
		writeError(w, http.StatusInternalServerError, "清空 Bot 对话流失败: "+err.Error())
		return
	}
	_ = s.store.ClearBotUnread(id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteBotMessage(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	msgID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("msgId")), 10, 64)
	if err != nil || msgID <= 0 {
		writeError(w, http.StatusBadRequest, "无效消息 ID")
		return
	}
	if err := s.store.DeleteBotMessage(id, msgID); err != nil {
		writeError(w, http.StatusInternalServerError, "删除消息失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleForwardBotMessage(w http.ResponseWriter, r *http.Request) {
	targetID := strings.TrimSpace(r.PathValue("id"))
	targetBot, err := s.store.GetBot(targetID)
	if err != nil || targetBot == nil {
		writeError(w, http.StatusNotFound, "目标会话不存在")
		return
	}

	var req struct {
		FromName string `json:"from_name"`
		Content  string `json:"content"`
		Note     string `json:"note"`
		ThreadID string `json:"thread_id"`
	}
	if !decodeJSON(w, r, 32*1024, &req) {
		return
	}

	cleanContent := notifier.StripEmoji(strings.TrimSpace(req.Content))
	if cleanContent == "" {
		writeError(w, http.StatusBadRequest, "转发内容不能为空")
		return
	}
	fromName := notifier.StripEmoji(strings.TrimSpace(req.FromName))
	if fromName == "" {
		fromName = "其他会话"
	}
	note := store.TruncateRunes(notifier.StripEmoji(strings.TrimSpace(req.Note)), 500)
	targetThreadID := strings.TrimSpace(req.ThreadID)
	if targetThreadID == "" {
		targetThreadID = strings.TrimSpace(targetBot.ActiveThreadID)
	}

	var body strings.Builder
	body.WriteString(fmt.Sprintf("[来自「%s」的转发记录]\n", fromName))
	body.WriteString(cleanContent)
	if note != "" {
		body.WriteString("\n\n附言：" + note)
	}

	sysMsg := store.AgentMessage{
		SessionID: "bot:" + targetBot.ID,
		BotID:     targetBot.ID,
		ThreadID:  targetThreadID,
		MsgKind:   "system",
		Role:      "system",
		Content:   fmt.Sprintf("已从「%s」转发一条消息", fromName),
	}
	if sid, sErr := s.store.AppendAgentMessage(sysMsg); sErr == nil {
		sysMsg.ID = sid
		if s.push != nil && s.push.Hub() != nil {
			s.push.Hub().PublishEvent("bot_message", sysMsg)
		}
	}
	sysNotice := &sysMsg

	fwdMsg := store.AgentMessage{
		SessionID: "bot:" + targetBot.ID,
		BotID:     targetBot.ID,
		ThreadID:  targetThreadID,
		MsgKind:   "chat",
		Role:      "user",
		Content:   body.String(),
	}
	id, err := s.store.AppendAgentMessage(fwdMsg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "转发写入失败: "+err.Error())
		return
	}
	fwdMsg.ID = id
	_ = s.store.IncrementBotUnread(targetBot.ID, fmt.Sprintf("收到来自「%s」的转发消息", fromName))
	updatedTarget, _ := s.store.GetBot(targetBot.ID)
	unread := 0
	if updatedTarget != nil {
		unread = updatedTarget.UnreadCount
	}
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("bot_message", fwdMsg)
		s.push.Hub().PublishEvent("bot_state", map[string]any{
			"bot_id":       targetBot.ID,
			"unread_count": unread,
			"last_summary": fmt.Sprintf("收到来自「%s」的转发消息", fromName),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"message":       fwdMsg,
		"system_notice": sysNotice,
	})
}

func (s *Server) handleCreateBotThread(w http.ResponseWriter, r *http.Request) {
	botID := strings.TrimSpace(r.PathValue("id"))
	bot, err := s.store.GetBot(botID)
	if err != nil || bot == nil {
		writeError(w, http.StatusNotFound, "Bot 不存在")
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := decodeOptionalJSON(w, r, 8*1024, &req); err != nil {
		writeError(w, http.StatusBadRequest, "话题请求格式错误")
		return
	}

	b := make([]byte, 4)
	_, _ = rand.Read(b)
	threadID := "th_" + hex.EncodeToString(b)
	title := store.TruncateRunes(notifier.StripEmoji(strings.TrimSpace(req.Title)), 80)
	if title == "" {
		title = "新话题"
	}
	th := store.BotThread{
		ID:    threadID,
		BotID: botID,
		Title: title,
	}
	if err := s.store.SaveBotThread(th); err != nil {
		writeError(w, http.StatusInternalServerError, "创建话题失败: "+err.Error())
		return
	}
	_ = s.store.SetBotActiveThread(botID, threadID)
	threads, _ := s.store.ListBotThreads(botID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"thread":  th,
		"threads": threads,
	})
}

func (s *Server) handleForkBotThread(w http.ResponseWriter, r *http.Request) {
	botID := strings.TrimSpace(r.PathValue("id"))
	bot, err := s.store.GetBot(botID)
	if err != nil || bot == nil {
		writeError(w, http.StatusNotFound, "Bot 不存在")
		return
	}
	var req struct {
		SourceThreadID string `json:"source_thread_id"`
		UpToMsgID      int64  `json:"up_to_msg_id"`
		Title          string `json:"title"`
	}
	if err := decodeOptionalJSON(w, r, 8*1024, &req); err != nil {
		writeError(w, http.StatusBadRequest, "分支请求格式错误")
		return
	}

	srcMsgs, _ := s.store.ListBotMessagesByThread(botID, strings.TrimSpace(req.SourceThreadID), 60)
	var contextSlice []store.AgentMessage
	for _, m := range srcMsgs {
		if req.UpToMsgID > 0 && m.ID > req.UpToMsgID {
			break
		}
		if m.Role == "user" || m.Role == "assistant" {
			contextSlice = append(contextSlice, m)
		}
	}
	if len(contextSlice) > 12 {
		contextSlice = contextSlice[len(contextSlice)-12:]
	}

	b := make([]byte, 4)
	_, _ = rand.Read(b)
	newThreadID := "th_" + hex.EncodeToString(b)
	title := store.TruncateRunes(notifier.StripEmoji(strings.TrimSpace(req.Title)), 80)
	if title == "" {
		if len(contextSlice) > 0 {
			lastTxt := strings.TrimSpace(contextSlice[len(contextSlice)-1].Content)
			runes := []rune(lastTxt)
			if len(runes) > 14 {
				lastTxt = string(runes[:14]) + "..."
			}
			title = "分支: " + lastTxt
		} else {
			title = "新话题分支"
		}
	}

	th := store.BotThread{
		ID:    newThreadID,
		BotID: botID,
		Title: title,
	}
	if err := s.store.SaveBotThread(th); err != nil {
		writeError(w, http.StatusInternalServerError, "创建分支话题失败: "+err.Error())
		return
	}
	_ = s.store.SetBotActiveThread(botID, newThreadID)

	_, _ = s.store.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:" + botID,
		BotID:     botID,
		ThreadID:  newThreadID,
		MsgKind:   "system",
		Role:      "system",
		Content:   fmt.Sprintf("已从历史对话分叉开启话题「%s」（继承 %d 条上下文）", title, len(contextSlice)),
	})
	for _, m := range contextSlice {
		_, _ = s.store.AppendAgentMessage(store.AgentMessage{
			SessionID:    "bot:" + botID,
			BotID:        botID,
			ThreadID:     newThreadID,
			SenderBotID:  m.SenderBotID,
			MsgKind:      m.MsgKind,
			Role:         m.Role,
			Content:      m.Content,
			ReplyPreview: m.ReplyPreview,
			ModelUsed:    m.ModelUsed,
		})
	}

	threads, _ := s.store.ListBotThreads(botID)
	msgs, _ := s.store.ListBotMessagesByThread(botID, newThreadID, 60)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"thread":   th,
		"threads":  threads,
		"messages": msgs,
	})
}

func (s *Server) handleUpdateBotThread(w http.ResponseWriter, r *http.Request) {
	botID := strings.TrimSpace(r.PathValue("id"))
	threadID := strings.TrimSpace(r.PathValue("threadId"))
	th, err := s.store.GetBotThread(botID, threadID)
	if err != nil || th == nil {
		writeError(w, http.StatusNotFound, "话题不存在")
		return
	}
	var req struct {
		Title    *string `json:"title"`
		Archived *bool   `json:"archived"`
	}
	if !decodeJSON(w, r, 8*1024, &req) {
		return
	}
	if req.Title != nil {
		if v := notifier.StripEmoji(strings.TrimSpace(*req.Title)); v != "" {
			th.Title = v
		}
	}
	if req.Archived != nil {
		th.Archived = *req.Archived
	}
	if err := s.store.SaveBotThread(*th); err != nil {
		writeError(w, http.StatusInternalServerError, "更新话题失败: "+err.Error())
		return
	}
	threads, _ := s.store.ListBotThreads(botID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "thread": th, "threads": threads})
}

func (s *Server) handleDeleteBotThread(w http.ResponseWriter, r *http.Request) {
	botID := strings.TrimSpace(r.PathValue("id"))
	threadID := strings.TrimSpace(r.PathValue("threadId"))
	if err := s.store.DeleteBotThread(botID, threadID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	threads, _ := s.store.ListBotThreads(botID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "threads": threads})
}

func (s *Server) handleSummarizeBotThread(w http.ResponseWriter, r *http.Request) {
	botID := strings.TrimSpace(r.PathValue("id"))
	threadID := strings.TrimSpace(r.PathValue("threadId"))
	if threadID == "main" {
		threadID = ""
	}
	bot, err := s.store.GetBot(botID)
	if err != nil || bot == nil {
		writeError(w, http.StatusNotFound, "Bot 不存在")
		return
	}

	var req struct {
		SaveToMemory *bool `json:"save_to_memory"`
	}
	_ = decodeOptionalJSON(w, r, 4096, &req)
	saveMem := true
	if req.SaveToMemory != nil {
		saveMem = *req.SaveToMemory
	}

	var th *store.BotThread
	if threadID != "" {
		th, err = s.store.GetBotThread(botID, threadID)
		if err != nil || th == nil {
			writeError(w, http.StatusNotFound, "话题不存在")
			return
		}
	}

	msgs, err := s.store.ListBotMessagesByThread(botID, threadID, 80)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取话题记录失败: "+err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	summary, savedFactKey, err := s.agent.SummarizeThreadToMemory(ctx, bot, th, msgs, saveMem)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	threads, _ := s.store.ListBotThreads(botID)
	memScopes := []string{"bot:" + botID}
	if bot.MemoryPolicy != store.MemoryPolicyIsolated {
		memScopes = append(memScopes, "global")
	}
	memories, _ := s.store.ListScopedMemories(memScopes, "", 60)

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"summary":        summary,
		"saved_fact_key": savedFactKey,
		"threads":        threads,
		"memories":       memories,
	})
}
