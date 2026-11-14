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
				{Key: "city", Label: "目标城市", Type: "text", DefaultVal: "Shanghai", Placeholder: "如 Shanghai / Beijing / Tokyo", Hint: "支持中文或拼音城市名，早报将自动抓取当地气象与出行指数"},
				{Key: "nezha_url", Label: "哪吒监控地址", Type: "text", DefaultVal: "https://dash.starryheavens.uk", Placeholder: "https://...", Hint: "哪吒探针面板公网 URL，早报将自动探查服务器节点脉搏与在线率"},
			},
		},
		{
			Type:        "radar",
			Name:        "全网开源技术雷达",
			Description: "抓取 GitHub Trending 热门仓库与 Hacker News 高分热帖，自动排版去重推送",
			DefaultCron: "0 12,20 * * *",
			ParamsSchema: []ParamField{
				{Key: "hn_min_score", Label: "Hacker News 最低分数", Type: "number", DefaultVal: "120", Placeholder: "120", Hint: "过滤杂讯，建议 100~200 分，只推送最热门开源技术爆款"},
				{Key: "limit", Label: "单次推送最大条数", Type: "number", DefaultVal: "8", Placeholder: "8", Hint: "单次微信推送精选条数，建议 6~10 条，避免微信长文本刷屏"},
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
				{Key: "provider", Label: "服务提供商", Type: "text", Placeholder: "如 BandwagonHost / RackNerd / 阿里云", Hint: "云服务商名称，便于到期时快速前往对应控制台充值"},
				{Key: "anchor_day", Label: "每月固定扣费日 (1~31)", Type: "number", DefaultVal: "1", Placeholder: "每月扣费日锚点，如 12", Hint: "平台每月免密扣款的日期，到期前系统将触发防自动扣费提醒"},
				{Key: "ip_address", Label: "公网 IP / 探针域名", Type: "text", Placeholder: "如 1.2.3.4 或 la.node.com", Hint: "云服务器的公网 IP 或监控探针域名，便于随时核对"},
			},
		},
		{
			Category:    "domain",
			Name:        "核心域名资产 (Domain)",
			Description: "顶级域名、二级业务域名与权威 DNS 资产",
			Fields: []ParamField{
				{Key: "domain_name", Label: "域名主体", Type: "text", Placeholder: "如 starryheavens.uk", Hint: "包含顶级后缀的完整主域名，建议与资产名称保持一致"},
				{Key: "registrar", Label: "注册托管机构", Type: "text", Placeholder: "如 Cloudflare / Spaceship / NameSilo", Hint: "购买并管理此域名的注册局或托管商"},
			},
		},
		{
			Category:    "life",
			Name:        "数字会员与生活订阅 (Subscription)",
			Description: "流媒体、AI 工具、云盘与各类软件定期扣款服务",
			Fields: []ParamField{
				{Key: "platform", Label: "订阅服务平台", Type: "text", Placeholder: "如 Spotify / Netflix / ChatGPT / B站大会员", Hint: "订阅服务所属的官方产品或 App 平台"},
				{Key: "credentials", Label: "嗅探凭证/Cookie (可选)", Type: "text", Placeholder: "用于自动嗅探到期的 SESSDATA 或 Token", Hint: "如 B站 SESSDATA，配置后支持点击右上角【嗅探同步】自动获取官方到期日"},
				{Key: "billing_note", Label: "扣费渠道与绑卡备注", Type: "text", Placeholder: "如 绑定中行Visa卡 / 微信免密支付", Hint: "记录该服务绑定的扣款卡号或扣费方式，便于换卡维护"},
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
