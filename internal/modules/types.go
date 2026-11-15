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
				{
					Key:         "city",
					Label:       "目标监测城市",
					Type:        "select",
					DefaultVal:  "Shanghai",
					Placeholder: "选择常驻城市",
					Options: []string{
						"Shanghai|上海 (Shanghai)",
						"Beijing|北京 (Beijing)",
						"Shenzhen|深圳 (Shenzhen)",
						"Guangzhou|广州 (Guangzhou)",
						"Hangzhou|杭州 (Hangzhou)",
						"Chengdu|成都 (Chengdu)",
						"Wuhan|武汉 (Wuhan)",
						"Nanjing|南京 (Nanjing)",
						"Chongqing|重庆 (Chongqing)",
						"Hong Kong|香港 (Hong Kong)",
						"Tokyo|东京 (Tokyo)",
						"Singapore|新加坡 (Singapore)",
						"London|伦敦 (London)",
						"New York|纽约 (New York)",
					},
					Hint: "早报将自动抓取该城市实时气温、降雨概率与穿衣出行建议",
				},
				{
					Key:         "nezha_url",
					Label:       "哪吒探针地址 (可选)",
					Type:        "text",
					DefaultVal:  "https://dash.starryheavens.uk",
					Placeholder: "https://dash.example.com",
					Hint:        "哪吒监控面板公网 URL，早报将自动探查服务器节点脉搏与存活率",
				},
			},
		},
		{
			Type:        "radar",
			Name:        "全网开源技术雷达",
			Description: "抓取 GitHub Trending 热门仓库与 Hacker News 高分热帖，自动排版去重推送",
			DefaultCron: "0 12,20 * * *",
			ParamsSchema: []ParamField{
				{
					Key:         "hn_min_score",
					Label:       "Hacker News 热度过滤阈值",
					Type:        "select",
					DefaultVal:  "150",
					Placeholder: "选择热度分数",
					Options: []string{
						"100|100 分 (覆盖广泛，不错过前沿动态)",
						"150|150 分 (优质爆款，推荐精选)",
						"200|200 分 (极高热度，顶级热帖)",
						"300|300 分 (现象级超级爆款)",
					},
					Hint: "过滤低分杂讯，建议 150 分，兼顾资讯广度与优质内容深度",
				},
				{
					Key:         "limit",
					Label:       "单次推送精选条数上限",
					Type:        "select",
					DefaultVal:  "8",
					Placeholder: "选择推送条数",
					Options: []string{
						"5|5 条 (极简摘要，快速浏览)",
						"8|8 条 (标准精选，微信排版推荐)",
						"10|10 条 (深度汇总)",
						"15|15 条 (全量长文)",
					},
					Hint: "建议选择 8 条，保证微信卡片排版美观且避免阅读疲劳",
				},
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
				{
					Key:         "provider",
					Label:       "云服务提供商",
					Type:        "select",
					DefaultVal:  "BandwagonHost",
					Placeholder: "选择主机提供商",
					Options: []string{
						"BandwagonHost|BandwagonHost (搬瓦工 VPS)",
						"RackNerd|RackNerd (便宜实用年付机)",
						"AlibabaCloud|阿里云 (Alibaba Cloud)",
						"TencentCloud|腾讯云 (Tencent Cloud)",
						"HuaweiCloud|华为云 (Huawei Cloud)",
						"Cloudflare|Cloudflare (边缘算力/Workers)",
						"OracleCloud|Oracle Cloud (甲骨文云)",
						"DigitalOcean|DigitalOcean",
						"AWS|AWS (亚马逊云)",
						"SelfHosted|自建独立物理机 / 局域网主机",
						"Other|其他云主机平台",
					},
					Hint: "服务商平台名称，便于到期时快速对账充值",
				},
				{
					Key:         "anchor_day",
					Label:       "每月免密扣款日锚点",
					Type:        "select",
					DefaultVal:  "1",
					Placeholder: "选择每月扣款日",
					Options: []string{
						"1|每月 01 号",
						"5|每月 05 号",
						"10|每月 10 号",
						"12|每月 12 号 (月中)",
						"15|每月 15 号 (月中)",
						"20|每月 20 号",
						"25|每月 25 号 (下旬)",
						"28|每月 28 号 (月末)",
					},
					Hint: "平台每月免密自动扣费的日期，到期前系统将触发防扣费强提醒",
				},
				{
					Key:         "ip_address",
					Label:       "公网 IP / 探针域名 (可选)",
					Type:        "text",
					Placeholder: "如 1.2.3.4 或 la.node.com",
					Hint:        "云服务器的公网 IP 或监控探针域名，便于快速定位机器",
				},
			},
		},
		{
			Category:    "domain",
			Name:        "核心域名资产 (Domain)",
			Description: "顶级域名、二级业务域名与权威 DNS 资产",
			Fields: []ParamField{
				{
					Key:         "domain_name",
					Label:       "域名主体",
					Type:        "text",
					Placeholder: "如 starryheavens.uk",
					Hint:        "包含顶级后缀的完整主域名，建议与资产名称保持一致",
				},
				{
					Key:         "registrar",
					Label:       "注册局与托管机构",
					Type:        "select",
					DefaultVal:  "Cloudflare",
					Placeholder: "选择注册托管商",
					Options: []string{
						"Cloudflare|Cloudflare Registrar (权威托管)",
						"Spaceship|Spaceship (新锐低价注册商)",
						"NameSilo|NameSilo (老牌免隐私费)",
						"Porkbun|Porkbun (猪肉包)",
						"GoDaddy|GoDaddy",
						"Namecheap|Namecheap",
						"Wanwang|阿里云 (万网)",
						"DNSPod|腾讯云 (DNSPod)",
						"Other|其他域名托管机构",
					},
					Hint: "购买并管理此域名的注册局或托管商",
				},
			},
		},
		{
			Category:    "life",
			Name:        "数字会员与生活订阅 (Subscription)",
			Description: "流媒体、AI 工具、云盘与各类软件定期扣款服务",
			Fields: []ParamField{
				{
					Key:         "platform",
					Label:       "订阅服务平台",
					Type:        "select",
					DefaultVal:  "bilibili",
					Placeholder: "选择订阅所属平台",
					Options: []string{
						"bilibili|哔哩哔哩大会员 (B站)",
						"qqmusic|QQ音乐 (豪华绿钻)",
						"netease_music|网易云音乐 (黑胶 VIP)",
						"chatgpt|ChatGPT Plus (OpenAI)",
						"claude|Claude Pro (Anthropic)",
						"netflix|Netflix (奈飞 4K 车位)",
						"spotify|Spotify (声田音乐家庭组)",
						"youtube_premium|YouTube Premium",
						"disney_plus|Disney+ 影视",
						"apple_one|Apple One / iCloud+",
						"github_copilot|GitHub Copilot",
						"google_one|Google One 会员",
						"other|其他生活数字订阅",
					},
					Hint: "订阅服务所属的官方产品或 App 平台",
				},
				{
					Key:         "billing_note",
					Label:       "扣费渠道与绑卡支付方式",
					Type:        "select",
					DefaultVal:  "微信免密扣款",
					Placeholder: "选择自动扣费渠道",
					Options: []string{
						"微信免密扣款|微信免密扣款",
						"支付宝自动续费|支付宝自动续费",
						"国内银联信用卡|国内银联信用卡",
						"Visa外币信用卡|Visa / MasterCard 外币卡",
						"PayPal账户扣款|PayPal 账户扣款",
						"ApplePay自动扣费|Apple Pay / App Store 订阅",
						"GooglePlay自动扣费|Google Play 自动扣款",
						"人工手动充值|无自动扣款 (人工手动充值)",
						"其他支付方式|其他支付渠道",
					},
					Hint: "记录绑定的扣款卡号或扣费方式，便于换卡或退订时维护",
				},
				{
					Key:         "credentials",
					Label:       "嗅探凭证/Cookie (可选)",
					Type:        "text",
					Placeholder: "用于自动嗅探到期的 SESSDATA 或 Token",
					Hint:        "如 B站 SESSDATA，配置后支持点击右上角【嗅探同步】自动获取官方到期日",
				},
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
