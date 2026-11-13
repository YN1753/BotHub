# Vesper BotHub (`bot.starryheavens.uk`)

Vesper 生态的自动化情报与微信推送中枢。采用 **Go 单二进制 + 纯 Go SQLite (`modernc.org/sqlite`) + 原生 `systemd`** 架构，**零 Docker 依赖**，Web 控制台严格遵循 Vesper **暮色星海（Twilight Seascape）+ 烟熏水晶（Smoked-Crystal）** 视觉规范，全链路 **STRICT NO EMOJI**。

## 核心功能

1. **每日早报小秘书 (`morning`)**
   - 默认每天 `08:00 (Asia/Shanghai)` 自动运行。
   - 接入免 Key 的 **Open-Meteo**（含 `wttr.in` 备用回退），获取目标城市实时温度、气温区间、降水概率与风速，并生成穿衣/带伞建议。
   - 附带每日短句/技术格言、`dash.starryheavens.uk` 哪吒探针节点脉搏，以及未来 7 天内**临期数字资产简要提示**。
2. **全网开源技术雷达 (`radar`)**
   - 默认每天 `12:00, 20:00 (Asia/Shanghai)` 自动运行。
   - 并发抓取 **GitHub Trending（每日热门仓库）** 与 **Hacker News Top Stories** 高分条目。
   - 通过 SQLite `seen_items` 表自动去重，仅推送未曾推送过的新增精选。
3. **数字资产与会员守望 (`asset_watcher`)**
   - 默认每天 `03:00 (Asia/Shanghai)` 或管理员手动触发。
   - **自动化嗅探与同步**：
     - **Cloudflare 域名**：调用官方 Registrar API 获取 `starryheavens.uk` 与 `looseleaf.sarl` 到期日与自动续费状态。
     - **Bilibili 大会员**：通过 `SESSDATA` 自动提取会员状态与到期时间戳。
     - **QQ 音乐绿钻**：监控活动体验月卡与到期时间。
     - **VPS 算力基建**：根据设定基准扣费日（`anchor_day`）自动顺延推算下月扣费日。
   - **临期防扣款强提醒**：剩余天数等于 7/3/1 天时，通过 PushPlus 微信强提醒，并对活动月卡附带 **「[防自动扣款提醒] 您的活动免费体验即将到期，请及时前往微信/支付宝确认是否已关闭免密续费授权！」**
   - **对外脱敏只读 API**：`GET /api/v1/assets/overview`（支持 CORS，供 `starryheavens.uk` 主页直接异步渲染，100% 过滤隐私凭证）。
4. **轻量 Web 管理控制台 (`:8080`)**
   - 展示模块运行状态、数字资产状态与倒计时、Cron 表达式、最近执行结果与 SQLite 历史推送流水。
   - 支持在控制台直接查看/编辑各资产规格、到期日与鉴权凭证（Cookie/Token/锚点日），支持 **「立即嗅探同步」**、**「立即触发一次」** 与 **「测试推送」**。
   - 内置轻量管理密钥登录保护（Cookie HMAC 会话 + `Authorization: Bearer` 支持）。

## 本地构建与运行

```bash
# 编译当前平台二进制
CGO_ENABLED=0 go build -o bothub .

# 启动服务（默认监听 :8080，默认初始管理密钥为 vesper-bothub）
BOTHUB_ADMIN_TOKEN="vesper-bothub" ./bothub -addr :8080 -db ./data/bothub.db
```

## 对外脱敏只读 API

- **路由**：`GET /api/v1/assets/overview`
- **支持**：全局 CORS 跨域请求
- **响应示例**：
  ```json
  {
    "code": 0,
    "data": [
      {
        "id": "qq_music",
        "name": "QQ 音乐",
        "category": "life",
        "spec": "豪华绿钻",
        "price": "活动月卡",
        "status": "活动体验中",
        "due_date": "2026-11-20T23:59:59+08:00",
        "auto_renew": false,
        "days_left": 7,
        "warn_flag": true
      }
    ],
    "updated_at": "2026-11-13 22:30:00"
  }
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
| `BOTHUB_CF_API_TOKEN` | 空 | 可选的 Cloudflare API Token |
