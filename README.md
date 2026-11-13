# Vesper BotHub (`bot.starryheavens.uk`)

Vesper 生态的自动化情报与微信推送中枢。采用 **Go 单二进制 + 纯 Go SQLite (`modernc.org/sqlite`) + 原生 `systemd`** 架构，**零 Docker 依赖**，Web 控制台严格遵循 Vesper **暮色星海（Twilight Seascape）+ 烟熏水晶（Smoked-Crystal）** 视觉规范，全链路 **STRICT NO EMOJI**。

## 核心功能

1. **每日早报小秘书 (`morning`)**
   - 默认每天 `08:00 (Asia/Shanghai)` 自动运行。
   - 接入免 Key 的 **Open-Meteo**（含 `wttr.in` 备用回退），获取目标城市实时温度、气温区间、降水概率与风速，并生成穿衣/带伞建议。
   - 附带每日短句/技术格言，以及 `dash.starryheavens.uk` 哪吒探针节点脉搏摘要。
2. **全网开源技术雷达 (`radar`)**
   - 默认每天 `12:00, 20:00 (Asia/Shanghai)` 自动运行。
   - 并发抓取 **GitHub Trending（每日热门仓库）** 与 **Hacker News Top Stories** 高分条目。
   - 通过 SQLite `seen_items` 表自动去重，仅推送未曾推送过的新增精选。
3. **轻量 Web 管理控制台 (`:8080`)**
   - 展示模块运行状态、Cron 表达式、下次触发时间、最近执行结果与 SQLite 历史推送流水。
   - 支持网页端动态修改配置（模块开关、Cron、城市、HN 阈值、PushPlus Token）并即时重载调度器，支持 **「立即触发一次」** 与 **「测试推送」**。
   - 内置轻量管理密钥登录保护（Cookie HMAC 会话 + `Authorization: Bearer` 支持）。

## 本地构建与运行

```bash
# 编译当前平台二进制
CGO_ENABLED=0 go build -o bothub .

# 启动服务（默认监听 :8080，默认初始管理密钥为 vesper-bothub）
BOTHUB_ADMIN_TOKEN="vesper-bothub" ./bothub -addr :8080 -db ./data/bothub.db
```

## Ubuntu 22.04 VPS 一键部署

```bash
chmod +x deploy.sh

# 仅编译 linux/amd64 静态二进制
./deploy.sh

# 或直接编译并推送至 VPS 启用 bothub.service
./deploy.sh root@<VPS_IP> 22
```

### 手动部署步骤

1. 在本地交叉编译静态二进制（无需 CGO）：
   ```bash
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bothub-linux-amd64 .
   ```
2. 上传至 VPS：
   ```bash
   ssh root@<VPS_IP> "mkdir -p /opt/bothub/data"
   scp bothub-linux-amd64 root@<VPS_IP>:/opt/bothub/bothub
   scp bothub.service root@<VPS_IP>:/etc/systemd/system/bothub.service
   ```
3. 启动并设为开机自启：
   ```bash
   ssh root@<VPS_IP> "chmod +x /opt/bothub/bothub && systemctl daemon-reload && systemctl enable --now bothub"
   ```
4. **Cloudflare 接入**：在 Cloudflare DNS 为 `bot.starryheavens.uk` 开启代理（Proxied），并配置 Origin Rule 将回源端口指向 `8080`。

## 环境变量说明

| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `BOTHUB_ADDR` | `:8080` | HTTP 监听地址 |
| `BOTHUB_DB` | `./data/bothub.db` | SQLite 数据库路径 |
| `BOTHUB_ADMIN_TOKEN` | `vesper-bothub` | 控制台登录密钥（建议上线后修改） |
| `BOTHUB_PUSHPLUS_TOKEN` | 内置默认 Token | PushPlus 微信推送 Token |
