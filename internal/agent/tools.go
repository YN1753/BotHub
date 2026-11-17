package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
)

type toolScopeCtxKey struct{}

// ToolScope defines isolated execution/memory boundaries for the current Bot or Group turn.
type ToolScope struct {
	BotID         string
	TimelineBotID string
	ThreadID      string
	MemoryPolicy  string
	Depth         int
	VisitedBots   []string
}

func WithToolScope(ctx context.Context, sc ToolScope) context.Context {
	return context.WithValue(ctx, toolScopeCtxKey{}, sc)
}

func toolScopeFromContext(ctx context.Context) ToolScope {
	if ctx == nil {
		return ToolScope{}
	}
	if v, ok := ctx.Value(toolScopeCtxKey{}).(ToolScope); ok {
		return v
	}
	return ToolScope{}
}

// TaskController abstracts Scheduler operations for tool execution.
type TaskController interface {
	Status() ([]scheduler.TaskStatus, error)
	RunNow(ctx context.Context, taskID string) (modules.RunResult, error)
	Reload() error
}

// ToolDefinition matches the OpenAI / DeepSeek function calling schema.
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type AvailableToolMeta struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

func AvailableToolsPayload() []AvailableToolMeta {
	defs := BuiltinTools()
	out := make([]AvailableToolMeta, 0, len(defs))
	for _, d := range defs {
		out = append(out, AvailableToolMeta{
			ID:          d.Function.Name,
			Description: d.Function.Description,
		})
	}
	return out
}

// BuiltinTools returns all OpenAI-compatible tool schemas exposed to DeepSeek.
func BuiltinTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "read_url",
				Description: "抓取指定网页 URL 的正文内容并转换为纯文本摘要，用于阅读文章、文档或监控页面。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"url": map[string]any{
							"type":        "string",
							"description": "要抓取的完整 HTTP/HTTPS 网页地址",
						},
					},
					"required": []string{"url"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "web_search",
				Description: "通过互联网搜索关键词获取最新资讯、技术方案或公开信息。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "搜索关键词",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "返回结果条数（默认 5，最大 10）",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_nezha_nodes",
				Description: "查询哪吒探针（Nezha）服务器节点的实时在线状态、系统信息、最近心跳与本地算力资产台账。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"keyword": map[string]any{
							"type":        "string",
							"description": "可选：按节点名称或备注过滤关键词",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_assets",
				Description: "查询或管理 BotHub 资产台账（VPS、域名、订阅、保号卡）。支持 list（列表）、summary（月度与年度费用分币种汇总）、expiring（查询即将到期）、upsert（新增或修改资产）、delete（删除资产）、sync（立即执行资产外部同步任务）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "summary", "expiring", "upsert", "delete", "sync"},
							"description": "操作类型：list | summary | expiring | upsert | delete | sync",
						},
						"category": map[string]any{
							"type":        "string",
							"description": "可选：按分类过滤或设置分类（例如 compute / domain / life / other 或自定义分类）",
						},
						"within_days": map[string]any{
							"type":        "integer",
							"description": "当 action=expiring 时，筛选多少天内到期的资产（默认 30）",
						},
						"id": map[string]any{
							"type":        "string",
							"description": "资产唯一标识 ID（upsert 或 delete 时必填）",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "资产名称（upsert 时使用）",
						},
						"spec": map[string]any{
							"type":        "string",
							"description": "规格或区域描述（upsert 时使用）",
						},
						"price": map[string]any{
							"type":        "string",
							"description": "费用与周期描述，如 $15.00 / 年、¥99 / 月（upsert 时使用）",
						},
						"status": map[string]any{
							"type":        "string",
							"description": "生命周期状态：生效中 | 已到期 | 已停用（upsert 时使用）",
						},
						"due_date": map[string]any{
							"type":        "string",
							"description": "到期日期 YYYY-MM-DD（upsert 时使用）",
						},
						"auto_renew": map[string]any{
							"type":        "boolean",
							"description": "是否自动续费（upsert 时使用）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_tasks",
				Description: "查询或管理 BotHub 自动化调度任务与一次性定时提醒。支持 list、run、upsert（支持周期 cron_expr 或一次性 delay_minutes / run_at 提醒，默认绑定当前 Bot）、toggle、delete。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "run", "upsert", "toggle", "delete"},
							"description": "操作类型：list | run | upsert | toggle | delete",
						},
						"id": map[string]any{
							"type":        "string",
							"description": "任务 ID（run / upsert / toggle / delete 时填写，新建也可自动生成）",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "任务或提醒名称（upsert 时使用）",
						},
						"type": map[string]any{
							"type":        "string",
							"description": "任务执行器类型，默认 bot_prompt（也支持 morning / radar / asset_watcher）",
						},
						"bot_id": map[string]any{
							"type":        "string",
							"description": "可选：绑定的 Bot ID，缺省自动绑定当前对话 Bot",
						},
						"cron_expr": map[string]any{
							"type":        "string",
							"description": "标准 5 段周期 Cron 表达式（与一次性 run_at / delay_minutes 二选一）",
						},
						"run_at": map[string]any{
							"type":        "string",
							"description": "可选：一次性触发的上海时间 YYYY-MM-DD HH:MM（到点自动执行一次后停用）",
						},
						"delay_minutes": map[string]any{
							"type":        "integer",
							"description": "可选：多少分钟后触发一次性提醒（自动换算 run_at，如 15 表示 15 分钟后）",
						},
						"prompt": map[string]any{
							"type":        "string",
							"description": "可选：bot_prompt 类型任务到期时要执行的具体指令内容",
						},
						"enabled": map[string]any{
							"type":        "boolean",
							"description": "是否启用任务（upsert 或 toggle 时使用）",
						},
						"description": map[string]any{
							"type":        "string",
							"description": "任务说明（upsert 时使用）",
						},
						"params": map[string]any{
							"type":        "string",
							"description": "JSON 字符串格式的任务参数（upsert 时使用）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "remember_fact",
				Description: "将与当前会话相关的用户偏好、关系故事、重要约定或核心事实写入隔离的长期记忆库，默认仅绑定当前 Bot/群聊作用域。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"category": map[string]any{
							"type":        "string",
							"description": "记忆分类：preference（偏好）/ relation（人际与故事）/ emotion（心情与生活）/ infra（节点与工程）/ schedule（约定）/ insight（洞察）/ general（通用）",
						},
						"fact_key": map[string]any{
							"type":        "string",
							"description": "简短唯一的事实键名（同作用域下同名会覆盖更新），如 user_tea_habit / deploy_rule",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "要记住的具体事实内容",
						},
						"importance": map[string]any{
							"type":        "integer",
							"description": "重要程度 1~5（默认 3，核心约定可设为 5）",
						},
						"pinned": map[string]any{
							"type":        "boolean",
							"description": "是否置顶为高优先常驻记忆",
						},
						"scope": map[string]any{
							"type":        "string",
							"description": "可选：'self'（默认，仅当前 Bot/群私有）或 'global'（显式跨全团队共享）",
						},
					},
					"required": []string{"fact_key", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "recall_memory",
				Description: "检索当前 Bot 或群聊专属作用域内的长期记忆（自动隔离其他 Bot 隐私记忆），支持关键词、分类和仅置顶过滤，并按相关性与权重排序。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "可选：检索关键词（支持空格分隔多关键词；留空则返回当前作用域核心记忆）",
						},
						"category": map[string]any{
							"type":        "string",
							"description": "可选：按记忆分类过滤，如 preference / infra / schedule / insight / relation / emotion / general",
						},
						"pinned_only": map[string]any{
							"type":        "boolean",
							"description": "可选：设为 true 时仅返回置顶的高优先记忆",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "可选：返回最大条数（默认 20，最大 40）",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_memory",
				Description: "对当前 Bot 或群聊作用域内的长期记忆进行整理：更正旧事实(upsert)、置顶或取消置顶(pin)、遗忘过时或错误条目(forget)、合并去重相似记忆(consolidate)、或列出现有记忆键(list)。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "upsert", "pin", "forget", "consolidate"},
							"description": "操作类型：list | upsert | pin | forget | consolidate",
						},
						"fact_key": map[string]any{
							"type":        "string",
							"description": "目标记忆事实键名（upsert/pin/forget 时必填）",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "更新后的记忆内容（upsert 时必填）",
						},
						"category": map[string]any{
							"type":        "string",
							"description": "记忆分类（preference/relation/emotion/infra/schedule/insight/general）",
						},
						"importance": map[string]any{
							"type":        "integer",
							"description": "重要度权重 1~5",
						},
						"pinned": map[string]any{
							"type":        "boolean",
							"description": "是否置顶",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_todos",
				Description: "管理用户的个人待办与行动清单（Muse 个人助手核心能力）。支持 list（查看待办）、add（新增单条待办）、batch_add（一次批量拆解多条子任务）、update（修改优先级/截止时间/状态）、complete（勾选完成，可传 id 或标题关键词）、delete（删除）、clear_done（清理已完成）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "add", "batch_add", "update", "complete", "delete", "clear_done"},
							"description": "操作类型：list | add | batch_add | update | complete | delete | clear_done",
						},
						"items": map[string]any{
							"type":        "array",
							"description": "当 action=batch_add 时传入批量待办数组（最多 20 条），每项含 title 及可选 priority/due_at/notes/tags",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"title":    map[string]any{"type": "string"},
									"priority": map[string]any{"type": "string", "enum": []string{"high", "normal", "low"}},
									"due_at":   map[string]any{"type": "string"},
									"notes":    map[string]any{"type": "string"},
									"tags":     map[string]any{"type": "string"},
								},
								"required": []string{"title"},
							},
						},
						"id": map[string]any{
							"type":        "string",
							"description": "待办条目 ID（update/delete 时使用）",
						},
						"title": map[string]any{
							"type":        "string",
							"description": "待办事项标题（add 必填；complete 也可传标题关键词匹配）",
						},
						"notes": map[string]any{
							"type":        "string",
							"description": "可选：补充备注或子步骤说明",
						},
						"status": map[string]any{
							"type":        "string",
							"enum":        []string{"pending", "doing", "done"},
							"description": "状态：pending | doing | done",
						},
						"priority": map[string]any{
							"type":        "string",
							"enum":        []string{"high", "normal", "low"},
							"description": "优先级：high | normal | low",
						},
						"due_at": map[string]any{
							"type":        "string",
							"description": "可选：截止日期或时间，如 2026-11-20 或 2026-11-20 18:00",
						},
						"tags": map[string]any{
							"type":        "string",
							"description": "可选：逗号分隔标签，如 工作,财务",
						},
						"filter": map[string]any{
							"type":        "string",
							"description": "当 action=list 时筛选状态：open（默认未完成）| all | done | doing | pending",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_notes",
				Description: "读写个人工作文档与成果交付区（报告、周报、邮件草稿、行程单、脚本模板等长文成品）。支持 list、search、get、upsert（覆写或新建）、append（向已有文档末尾安全追加新章节或日志，不会覆盖原文）、delete。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "search", "get", "upsert", "append", "delete"},
							"description": "操作类型：list | search | get | upsert | append | delete",
						},
						"id": map[string]any{
							"type":        "string",
							"description": "文档 ID（更新或读取已有文档时使用）",
						},
						"title": map[string]any{
							"type":        "string",
							"description": "文档标题",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "Markdown 格式的文档正文",
						},
						"category": map[string]any{
							"type":        "string",
							"description": "分类标签，如 deliverable / draft / plan / summary",
						},
						"pinned": map[string]any{
							"type":        "boolean",
							"description": "是否置顶展示",
						},
						"query": map[string]any{
							"type":        "string",
							"description": "搜索关键词（search 时使用）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_stash",
				Description: "管理与检索用户的「智能收藏与闪念匣（Smart Stash）」。支持 capture（秒收网页链接、B站分享文案、GitHub仓库或随手文字闪念并触发异步AI干货提炼）、search（基于全文检索引擎搜之前收藏过的文章/视频/闪念）、get（读取单条完整要点与正文快照）、update（修改标题/批注/标签/置顶/归档/标记已读）、delete（删除）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"capture", "search", "get", "update", "delete"},
							"description": "操作类型：capture | search | get | update | delete",
						},
						"id": map[string]any{
							"type":        "string",
							"description": "收藏条目 ID（get / update / delete 时使用）",
						},
						"raw_input": map[string]any{
							"type":        "string",
							"description": "当 action=capture 时必填：原始链接、手机端带标题前缀分享文案或纯文字闪念",
						},
						"user_note": map[string]any{
							"type":        "string",
							"description": "可选：附带的个人批注或想法备注",
						},
						"query": map[string]any{
							"type":        "string",
							"description": "当 action=search 时可选：中英文全文检索关键词（支持长短词召回）",
						},
						"filter": map[string]any{
							"type":        "string",
							"description": "可选：筛选视图 unread | all | pinned | note | archived",
						},
						"tag": map[string]any{
							"type":        "string",
							"description": "可选：按单个标签精确过滤",
						},
						"kind": map[string]any{
							"type":        "string",
							"description": "可选：按来源类型过滤 link | bilibili | github | note",
						},
						"title": map[string]any{
							"type":        "string",
							"description": "当 action=update 时可选：修改后的标题",
						},
						"tags": map[string]any{
							"type":        "string",
							"description": "可选：逗号分隔分类标签，如 AI,架构,Bilibili",
						},
						"status": map[string]any{
							"type":        "string",
							"description": "当 action=update 时可选：ready | archived",
						},
						"pinned": map[string]any{
							"type":        "boolean",
							"description": "当 action=update 时可选：是否置顶",
						},
						"mark_read": map[string]any{
							"type":        "boolean",
							"description": "当 action=update 时可选：设为 true 则增加已读计数",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "当 action=search 时返回最大条数（默认 20，最大 50）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "send_notification",
				Description: "主动向用户收件箱与桌面推送一条重要卡片通知（支持 high 高优先级提醒），适合在巡检异常、耗时分析完成或紧急提醒时使用。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title": map[string]any{
							"type":        "string",
							"description": "推送标题",
						},
						"markdown_body": map[string]any{
							"type":        "string",
							"description": "Markdown 格式的推送正文摘要",
						},
						"priority": map[string]any{
							"type":        "string",
							"enum":        []string{"normal", "high"},
							"description": "通知优先级：normal | high",
						},
					},
					"required": []string{"title", "markdown_body"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "consult_bot",
				Description: "主动协同工作区中的专业 Bot 成员协助完成子任务。支持单人委派（bot_id / bot_name + instruction）或传入 targets 数组并发委派多位成员同步作业，支持最多 2 层深度且自动防循环。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"bot_id": map[string]any{
							"type":        "string",
							"description": "单人委派时：目标成员 Bot ID（与 bot_name 二选一）",
						},
						"bot_name": map[string]any{
							"type":        "string",
							"description": "单人委派时：目标成员 Bot 名称",
						},
						"instruction": map[string]any{
							"type":        "string",
							"description": "单人委派时：清晰的执行指令与上下文",
						},
						"context_snippet": map[string]any{
							"type":        "string",
							"description": "可选：附带给协办成员的关键前置上下文片段，减少重复检索",
						},
						"targets": map[string]any{
							"type":        "array",
							"description": "可选：并发委派多位成员同步协办",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"bot_id":          map[string]any{"type": "string"},
									"bot_name":        map[string]any{"type": "string"},
									"instruction":     map[string]any{"type": "string"},
									"context_snippet": map[string]any{"type": "string"},
								},
								"required": []string{"instruction"},
							},
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "workspace_briefing",
				Description: "一键汇总用户的今日工作全景简报（逾期与今日到期待办、高优先待办、14天内到期资产、启用中的定时任务、未读通知与近期工作文档），适合个人助手规划今日日程。",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "calculate_eval",
				Description: "高精度数学与财务表达式计算器，支持四则运算、乘方(^)、括号、百分比(如 15%) 及 round/abs/min/max/sqrt/floor/ceil 函数，避免估算误差。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"expression": map[string]any{
							"type":        "string",
							"description": "数学表达式，如 (198.5 * 12) * 0.85 或 round(100 / 3, 2)",
						},
					},
					"required": []string{"expression"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "datetime_calc",
				Description: "精确获取或推算当前上海时间、星期、相对日期偏移（如 30 分钟后 / 7 天后可用于设定 run_at 提醒）以及预测 Cron 下 5 次执行时刻。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"now", "add", "diff", "cron_next"},
							"description": "操作类型：now | add | diff | cron_next",
						},
						"base_time": map[string]any{
							"type":        "string",
							"description": "可选：基准时间 YYYY-MM-DD HH:MM（缺省为当前时间）",
						},
						"target_time": map[string]any{
							"type":        "string",
							"description": "当 action=diff 时必填：目标时间 YYYY-MM-DD HH:MM 或 YYYY-MM-DD",
						},
						"delta_minutes": map[string]any{
							"type":        "integer",
							"description": "分钟偏移量（可正可负）",
						},
						"delta_hours": map[string]any{
							"type":        "integer",
							"description": "小时偏移量（可正可负）",
						},
						"delta_days": map[string]any{
							"type":        "integer",
							"description": "天数偏移量（可正可负）",
						},
						"cron_expr": map[string]any{
							"type":        "string",
							"description": "当 action=cron_next 时必填的 5 段 Cron 表达式",
						},
					},
				},
			},
		},
	}
}

var reTrailingJSONComma = regexp.MustCompile(`,\s*([}\]])`)

func normalizeToolArgsJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "{}"
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	s = reTrailingJSONComma.ReplaceAllString(s, "$1")
	if s == "" {
		return "{}"
	}
	return s
}

const maxToolOutputRunes = 12000

// ExecuteTool dispatches built-in tools and user-defined custom webhook skills.
func (e *Engine) ExecuteTool(ctx context.Context, name string, argsJSON string) (out string, err error) {
	timeout := 12 * time.Second
	switch {
	case name == "consult_bot":
		timeout = 60 * time.Second
	case name == "read_url" || name == "web_search" || name == "query_nezha_nodes" || strings.HasPrefix(name, "skill_"):
		timeout = 25 * time.Second
	}
	toolCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			out = ""
			err = fmt.Errorf("工具 %s 执行异常中断: %v", name, r)
		}
	}()

	cleanArgs := normalizeToolArgsJSON(argsJSON)
	if strings.HasPrefix(name, "skill_") {
		slug := strings.TrimPrefix(name, "skill_")
		sk, gErr := e.store.GetSkillBySlug(slug)
		if gErr != nil || sk == nil || !sk.Enabled || sk.Kind != "webhook" {
			return "", fmt.Errorf("未找到或未启用自定义技能: %s", name)
		}
		var args map[string]any
		if cleanArgs != "{}" {
			if uErr := json.Unmarshal([]byte(cleanArgs), &args); uErr != nil {
				return "", fmt.Errorf("解析技能参数 JSON 失败: %w", uErr)
			}
		}
		statusCode, bodyText, execErr := e.ExecuteWebhookSkill(toolCtx, *sk, args)
		if execErr != nil {
			return "", execErr
		}
		b, _ := json.Marshal(map[string]any{
			"ok":          true,
			"skill":       sk.Name,
			"status_code": statusCode,
			"result":      bodyText,
		})
		out = string(b)
	} else {
		switch name {
		case "read_url":
			out, err = e.toolReadURL(toolCtx, cleanArgs)
		case "web_search":
			out, err = e.toolWebSearch(toolCtx, cleanArgs)
		case "query_nezha_nodes":
			out, err = e.toolQueryNezhaNodes(toolCtx, cleanArgs)
		case "manage_assets":
			out, err = e.toolManageAssets(toolCtx, cleanArgs)
		case "manage_tasks":
			out, err = e.toolManageTasks(toolCtx, cleanArgs)
		case "remember_fact":
			out, err = e.toolRememberFact(toolCtx, cleanArgs)
		case "recall_memory":
			out, err = e.toolRecallMemory(toolCtx, cleanArgs)
		case "manage_memory":
			out, err = e.toolManageMemory(toolCtx, cleanArgs)
		case "manage_todos":
			out, err = e.toolManageTodos(toolCtx, cleanArgs)
		case "manage_notes":
			out, err = e.toolManageNotes(toolCtx, cleanArgs)
		case "manage_stash":
			out, err = e.toolManageStash(toolCtx, cleanArgs)
		case "send_notification":
			out, err = e.toolSendNotification(toolCtx, cleanArgs)
		case "consult_bot":
			out, err = e.toolConsultBot(toolCtx, cleanArgs)
		case "workspace_briefing":
			out, err = e.toolWorkspaceBriefing(toolCtx, cleanArgs)
		case "calculate_eval":
			out, err = e.toolCalculateEval(toolCtx, cleanArgs)
		case "datetime_calc":
			out, err = e.toolDatetimeCalc(toolCtx, cleanArgs)
		default:
			return "", fmt.Errorf("未知工具: %s", name)
		}
	}
	if err != nil {
		return "", err
	}
	runes := []rune(out)
	if len(runes) > maxToolOutputRunes {
		out = string(runes[:maxToolOutputRunes]) + "\n...[工具输出过长已自动裁剪]"
	}
	return out, nil
}

func toolErrorPayload(name string, err error) string {
	msg := err.Error()
	lower := strings.ToLower(msg)
	retryable := strings.Contains(lower, "timeout") || strings.Contains(lower, "超时") ||
		strings.Contains(lower, "connection") || strings.Contains(lower, "eof") || strings.Contains(lower, "temporary")
	var hint string
	switch {
	case strings.Contains(lower, "json") || strings.Contains(msg, "参数") || strings.Contains(msg, "必填"):
		hint = "请检查并修正工具参数 JSON 字段后重试"
	case strings.Contains(msg, "未找到") || strings.Contains(msg, "不存在"):
		hint = "建议先调用 list/search 动作核对准确 id 或 fact_key"
	case name == "read_url" || name == "web_search":
		if retryable {
			hint = "目标站点访问超时，可更换关键词或改用另一参考源"
		} else {
			hint = "该目标地址不可访问或受策略拦截，请勿编造正文，如实向用户说明或更换公开来源"
		}
	case retryable:
		hint = "遇到瞬时连接或超时问题，可重试一次或降级为精简查询"
	}
	payload := map[string]any{
		"ok":        false,
		"tool":      name,
		"error":     msg,
		"retryable": retryable,
	}
	if hint != "" {
		payload["hint"] = hint
	}
	b, _ := json.Marshal(payload)
	return string(b)
}
