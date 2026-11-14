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
			Name:        "每日早报小秘书",
			Description: "汇总目标城市气象预报、出行建议、每日格言、节点脉搏与临期资产",
			DefaultCron: "0 8 * * *",
			ParamsSchema: []ParamField{
				{Key: "city", Label: "目标城市", Type: "text", DefaultVal: "Shanghai", Placeholder: "如 Shanghai / Beijing / Tokyo"},
				{Key: "nezha_url", Label: "哪吒监控地址", Type: "text", DefaultVal: "https://dash.starryheavens.uk", Placeholder: "https://..."},
			},
		},
		{
			Type:        "radar",
			Name:        "全网开源技术雷达",
			Description: "抓取 GitHub Trending 热门仓库与 Hacker News 高分热帖，自动排版去重推送",
			DefaultCron: "0 12,20 * * *",
			ParamsSchema: []ParamField{
				{Key: "hn_min_score", Label: "Hacker News 最低分数", Type: "number", DefaultVal: "120", Placeholder: "120"},
				{Key: "limit", Label: "单次推送最大条数", Type: "number", DefaultVal: "8", Placeholder: "8"},
			},
		},
		{
			Type:        "asset_watcher",
			Name:        "数字资产与会员守望",
			Description: "自动巡检所有资产到期时间，触发个性化临期预警与防扣费强提醒",
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
