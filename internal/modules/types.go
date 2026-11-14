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

type AssetTemplate struct {
	Category    string       `json:"category"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Fields      []ParamField `json:"fields"`
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

func BuiltinAssetTemplates() []AssetTemplate {
	return []AssetTemplate{
		{
			Category:    "compute",
			Name:        "算力与云主机 (VPS / Cloud)",
			Description: "云服务器、边缘算力、轻量主机及 NAT 机器",
			Fields: []ParamField{
				{Key: "provider", Label: "服务提供商", Type: "text", Placeholder: "如 BandwagonHost / RackNerd / 阿里云"},
				{Key: "anchor_day", Label: "每月固定扣费日 (1~31)", Type: "number", DefaultVal: "1", Placeholder: "每月扣费日锚点，如 12"},
				{Key: "ip_address", Label: "公网 IP / 探针域名", Type: "text", Placeholder: "如 1.2.3.4 或 la.node.com"},
			},
		},
		{
			Category:    "domain",
			Name:        "核心域名资产 (Domain)",
			Description: "顶级域名、二级业务域名与权威 DNS 资产",
			Fields: []ParamField{
				{Key: "domain_name", Label: "域名主体", Type: "text", Placeholder: "如 starryheavens.uk"},
				{Key: "registrar", Label: "注册托管机构", Type: "text", Placeholder: "如 Cloudflare / Spaceship / NameSilo"},
			},
		},
		{
			Category:    "life",
			Name:        "数字会员与生活订阅 (Subscription)",
			Description: "流媒体、AI 工具、云盘与各类软件定期扣款服务",
			Fields: []ParamField{
				{Key: "platform", Label: "订阅服务平台", Type: "text", Placeholder: "如 Spotify / Netflix / ChatGPT / B站大会员"},
				{Key: "credentials", Label: "嗅探凭证/Cookie (可选)", Type: "text", Placeholder: "用于自动嗅探到期的 SESSDATA 或 Token"},
				{Key: "billing_note", Label: "扣费渠道与绑卡备注", Type: "text", Placeholder: "如 绑定中行Visa卡 / 虚拟卡尾号8888"},
			},
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
