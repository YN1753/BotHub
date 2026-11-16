package modules

import (
	"context"
	"net/http"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

type RunResult struct {
	Summary     string
	ItemsPushed int
}

type Module interface {
	ID() string
	Name() string
	Description() string
	Run(ctx context.Context) (RunResult, error)
}

type TaskRunner interface {
	RunWithTask(ctx context.Context, task store.Task) (RunResult, error)
}

type ParamField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // "text", "number", "select"
	DefaultVal  string   `json:"default_val"`
	Placeholder string   `json:"placeholder"`
	Options     []string `json:"options,omitempty"`
	Hint        string   `json:"hint,omitempty"`
}

type TaskTemplate struct {
	Type         string       `json:"type"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	DefaultCron  string       `json:"default_cron"`
	ParamsSchema []ParamField `json:"params_schema"`
}

func BuiltinTemplates() []TaskTemplate {
	return []TaskTemplate{
		{
			Type:        "morning",
			Name:        "每日早报简报",
			Description: "汇总目标城市气象预报、出行建议、每日格言、节点脉搏与临期资产",
			DefaultCron: "0 8 * * *",
			ParamsSchema: []ParamField{
				{
					Key:         "city",
					Label:       "目标监测城市",
					Type:        "text",
					DefaultVal:  "Shanghai",
					Placeholder: "输入城市英文或中文名（如 Shanghai / Tokyo / London）",
					Hint:        "支持任意全球城市实时气温、降水概率与出行建议抓取",
				},
			},
		},
		{
			Type:        "radar",
			Name:        "开源技术雷达",
			Description: "抓取 GitHub Trending 热门仓库与 Hacker News 高分热帖，去重精选推送",
			DefaultCron: "0 12,20 * * *",
			ParamsSchema: []ParamField{
				{
					Key:         "hn_min_score",
					Label:       "Hacker News 最低分数阈值",
					Type:        "number",
					DefaultVal:  "150",
					Placeholder: "150",
					Hint:        "过滤低于该热度的帖子（建议 100~300）",
				},
				{
					Key:         "limit",
					Label:       "单次推送条目上限",
					Type:        "number",
					DefaultVal:  "8",
					Placeholder: "8",
					Hint:        "单次抓取推送的最多条目数量（建议 5~15）",
				},
			},
		},
		{
			Type:        "asset_watcher",
			Name:        "数字资产守望",
			Description: "定期巡检数字资产台账到期日，自动触发自定义适配同步与临期预警推送",
			DefaultCron: "0 3 * * *",
			ParamsSchema: []ParamField{},
		},
	}
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}

func ShanghaiLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}
