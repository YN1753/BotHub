package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

// BotRoutineExecutor decouples scheduled cron tasks from agent.Engine without package cycles.
type BotRoutineExecutor interface {
	RunBotRoutine(ctx context.Context, botID, prompt, taskName string) (reply string, err error)
}

type BotPromptModule struct {
	store *store.Store
	push  *notifier.Client
	mu    sync.RWMutex
	exec  BotRoutineExecutor
}

func NewBotPromptModule(st *store.Store, push *notifier.Client) *BotPromptModule {
	return &BotPromptModule{store: st, push: push}
}

func (m *BotPromptModule) SetExecutor(exec BotRoutineExecutor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exec = exec
}

func (m *BotPromptModule) ID() string          { return "bot_prompt" }
func (m *BotPromptModule) Name() string        { return "AI Bot 自主定时指令" }
func (m *BotPromptModule) Description() string { return "定时唤醒指定 Bot，结合其人格、专属记忆与绑定工具主动思考并发送汇报或问候" }

func (m *BotPromptModule) Run(ctx context.Context) (RunResult, error) {
	return RunResult{}, fmt.Errorf("bot_prompt 需要通过绑定了 bot_id 的调度任务执行")
}

func (m *BotPromptModule) RunWithTask(ctx context.Context, task store.Task) (RunResult, error) {
	botID := strings.TrimSpace(task.BotID)
	if botID == "" {
		return RunResult{}, fmt.Errorf("请先为定时任务「%s」绑定目标 Bot", task.Name)
	}

	var params map[string]any
	_ = json.Unmarshal([]byte(task.Params), &params)

	prompt := ""
	notifyInbox := true
	if params != nil {
		if p, ok := params["prompt"].(string); ok {
			prompt = strings.TrimSpace(p)
		}
		if n, ok := params["notify"].(string); ok {
			notifyInbox = !strings.EqualFold(strings.TrimSpace(n), "false") && strings.TrimSpace(n) != "0"
		} else if nb, ok := params["notify"].(bool); ok {
			notifyInbox = nb
		}
	}
	if prompt == "" {
		prompt = strings.TrimSpace(task.Description)
	}
	if prompt == "" {
		prompt = fmt.Sprintf("执行例行任务「%s」，简明同步结果或自然问候。", task.Name)
	}

	m.mu.RLock()
	exec := m.exec
	m.mu.RUnlock()
	if exec == nil {
		return RunResult{}, fmt.Errorf("AI Agent 执行器尚未初始化")
	}

	reply, err := exec.RunBotRoutine(ctx, botID, prompt, task.Name)
	if err != nil {
		return RunResult{}, err
	}

	itemsPushed := 0
	if notifyInbox && m.push != nil && strings.TrimSpace(reply) != "" {
		botName := botID
		if b, bErr := m.store.GetBot(botID); bErr == nil && b != nil && strings.TrimSpace(b.Name) != "" {
			botName = b.Name
		}
		title := fmt.Sprintf("[%s] %s", botName, task.Name)
		if err := m.push.SendMarkdown(ctx, task.ID, title, reply); err == nil {
			itemsPushed = 1
		}
	}

	clean := notifier.StripEmoji(strings.TrimSpace(reply))
	runes := []rune(clean)
	if len(runes) > 120 {
		clean = string(runes[:120]) + "..."
	}
	return RunResult{
		Summary:     clean,
		ItemsPushed: itemsPushed,
	}, nil
}
