# Vesper BotHub (`bot.starryheavens.uk`)

Vesper 生态的自动化情报与微信推送中枢。采用 **Go 单二进制内嵌前端（`go:embed`）+ 纯 Go SQLite (`modernc.org/sqlite`) + 原生 `systemd`** 架构，**STRICT ZERO DOCKER**，Web 控制台严格遵循 Vesper **暮色星海（Twilight Seascape）+ 烟熏水晶（Smoked-Crystal）** 视觉规范，全链路 **STRICT NO EMOJI**。

每次代码推送均通过 **GitHub Actions 全自动多架构构建 Release**（Linux amd64/arm64、macOS arm64/amd64、Windows amd64），生成单一静态可执行文件与校验和。

---

## 一、核心特性与架构

1. **前后端单二进制内嵌 (Single Binary Embedding)**：
   - 前端 Web 控制台（HTML、CSS、JS、Lucide 矢量线性 SVG、Canvas 暮色星穹引擎）全部通过 Go 官方标准库 `//go:embed static/*` 编译期内嵌进单个二进制文件。
   - 部署时**仅需单个二进制可执行文件**，无任何外部静态文件或运行时依赖，零 Docker 侵入。
2. **全自动 CI/CD Release (GitHub Actions)**：
   - **推送到 `main` 分支**：全自动并发跨平台编译，自动发布/覆盖 `latest` Release，生成 SHA256 校验和。
   - **推送标签（`v*`）**：全自动生成正式版本 GitHub Release，自动注入构建版本号、Git Commit SHA 与构建时间。
3. **每日早报小秘书 (`morning`)**：
   - 默认每天 `08:00 (Asia/Shanghai)` 自动运行。
   - 接入免 Key 的 **Open-Meteo**（含 `wttr.in` 备用回退），获取城市实时温度、气温区间、降水概率与穿衣带伞建议。
   - 附带每日短句/技术格言、哪吒探针节点脉搏，以及未来 7 天内**临期数字资产简报**。
4. **全网开源技术雷达 (`radar`)**：
   - 默认每天 `12:00, 20:00 (Asia/Shanghai)` 自动运行。
   - 并发抓取 **GitHub Trending** 与 **Hacker News Top Stories** 高分条目，经 SQLite `seen_items` 自动去重后推送到微信。
5. **数字资产与会员守望 (`asset_watcher`)**：
   - 默认每天 `03:00 (Asia/Shanghai)` 或管理员手动触发。
   - **自动化嗅探与同步**：
     - **Cloudflare 域名**：调用官方 Registrar API 获取 `starryheavens.uk` 与 `looseleaf.sarl` 到期日与自动续费状态。
     - **Bilibili 大会员**：通过 `SESSDATA` 自动提取会员状态与到期时间戳。
     - **QQ 音乐绿钻**：监控活动体验月卡与到期时间。
     - **VPS 算力基建**：根据设定基准扣费日（`anchor_day`）自动顺延推算下月扣费日。
   - **临期防扣款强提醒**：剩余天数等于 7/3/1 天时通过 PushPlus 微信强提醒；针对活动月卡附带 **「[防自动扣款提醒] 您的活动免费体验即将到期，请及时前往微信/支付宝确认是否已关闭免密续费授权！」**
   - **对外脱敏只读 API**：`GET /api/v1/assets/overview`（支持 CORS，供 `starryheavens.uk` 主页直接异步渲染，100% 过滤隐私凭证）。
6. **小白专属极致防呆与自然语言交互 (Novice-Friendly UX & Natural Language)**：
   - **Vesper CustomSelect 组件库**：彻底消灭操作系统原生下拉栏，采用 Twilight 暮色磨砂水晶质感微组件，全键盘与跨端交互支持。
   - **金额与付款周期解耦**：币种选择（CNY/USD/EUR/GBP/HKD）+ 纯数字金额输入框 + 付款周期下拉（月付/季付/半年/年付/终身/免费），支持实时反显与双向解析。
   - **到期时间人性化推算**：告别冷冰冰的 RFC3339 字符串手填，提供快捷跳转胶囊（+1个月/+3个月/+半年/+1年/永久有效）与年/月/日三级联动自定义选择器。
   - **任务自然语言调度构建器**：彻底告别手写 Cron 表达式，提供频次（每天/工作日/周末/每周/每月/每隔N小时）+ 时分选择器，实时生成中文语义说明并自动折算为标准 Cron。
   - **常用服务一键开箱快捷库**：内置搬瓦工 VPS、Cloudflare 域名、B站大会员、QQ音乐绿钻、ChatGPT Plus、Netflix 奈飞、Spotify 等服务快捷预设，点击一键全量参数填入。
   - **数据便携性与灾备恢复**：支持全量资产、任务与全局系统配置的一键导出（`GET /api/system/export`）与一键导入恢复（`POST /api/system/import`）。
7. **双模板驱动引擎与可视化配置 (Schema-Driven Templates Engine)**：
   - 资产（Assets）与任务（Tasks）全面统一为后端 Schema 驱动的双模板体系，彻底告别手写原始 JSON。
   - 动态分类参数渲染：算力主机提供扣费锚点日、生活订阅提供平台与凭证、域名提供注册托管；早报与雷达提供阈值参数动态表单。
   - 智能 Slug 生成与常用 Cron 预设胶囊，兼顾零门槛操作与极客折叠 JSON 双向同步。
   - 底部配置面板彻底去重，收敛为纯粹的「全局系统凭证与安全」。
7. **轻量 Web 管理控制台 (`:8080`)**：
   - 支持在控制台直接查看/编辑/删除各资产与任务，支持 **「立即嗅探同步」**、**「立即触发一次」** 与 **「测试推送」**。
   - 安全鉴权保护：支持管理密钥登录（Cookie HMAC 会话）及 Cloudflare Access（Zero Trust Passkey / TouchID / FaceID 免密信任）。

---

## 二、从 GitHub Release 一键部署与升级

由于 GitHub Actions 每次推送都会全自动编译发布静态单二进制，VPS 部署与升级只需一条命令：

### 1. 首次部署 (Ubuntu 22.04 VPS)

```bash
# 1. 创建目录并下载最新 linux/amd64 单二进制
mkdir -p /opt/bothub/data
curl -fsSL -o /opt/bothub/bothub https://github.com/YN1753/BotHub/releases/latest/download/bothub-linux-amd64
chmod +x /opt/bothub/bothub

# 2. 下载 systemd 服务文件
curl -fsSL -o /etc/systemd/system/bothub.service https://raw.githubusercontent.com/YN1753/BotHub/main/bothub.service

# 3. 启动并启用开机自启
systemctl daemon-reload
systemctl enable --now bothub
systemctl status bothub
```

### 2. 后续一键在线更新升级

```bash
# 停止服务并拉取最新 Release 二进制即可完成升级
systemctl stop bothub
curl -fsSL -o /opt/bothub/bothub https://github.com/YN1753/BotHub/releases/latest/download/bothub-linux-amd64
chmod +x /opt/bothub/bothub
systemctl start bothub
```

---

## 三、本地静态单二进制构建

若需在本地手动编译：

```bash
# 1. 编译当前平台单二进制 (自包含内嵌前端)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bothub .

# 2. 交叉编译 Linux amd64 静态单二进制
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bothub-linux-amd64 .

# 3. 检查版本
./bothub -v
```

---

## 四、对外脱敏只读 API

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
        "due_date": "2026-11-20T12:00:00+08:00",
        "auto_renew": false,
        "days_left": 6,
        "warn_flag": true
      }
    ],
    "updated_at": "2026-11-14 18:00:00"
  }
  ```

---

## 五、环境变量配置说明

| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `BOTHUB_ADDR` | `:8080` | HTTP 监听地址 |
| `BOTHUB_DB` | `./data/bothub.db` | SQLite 数据库路径 |
| `BOTHUB_ADMIN_TOKEN` | `vesper-bothub` | 控制台登录密钥（可在控制台修改） |
| `BOTHUB_PUSHPLUS_TOKEN` | 内置默认 Token | PushPlus 微信推送 Token |
| `BOTHUB_CF_API_TOKEN` | 空 | 可选的 Cloudflare API Token（用于域名到期嗅探） |
