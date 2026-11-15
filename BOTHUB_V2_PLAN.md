# BotHub 2.0（DeepSeek 智能中枢 + Android 原生客户端）架构与分阶段研发蓝图

> **所属体系**：`YN1753/BotHub`（本地目录 [bothub/](file:///Users/starry/Documents/projects/bothub)）原地演进为 **v2.0**  
> **线上域名**：`bot.starryheavens.uk`（主节点 CloudCone LA `103.232.94.2`，可随时单目录迁移至未来主力机）  
> **核心目标**：以自建 **DeepSeek Agent + 原生手机直推客户端** 取代传统固定模板 BotHub 1.x 与第三方微信推送依赖，同时无缝保留现有资产接口与自动部署流水线。

---

## 一、架构演进总览（v1.x -> v2.0）

| 维度 | BotHub v1.x（现状） | BotHub v2.0（目标架构） |
| :--- | :--- | :--- |
| **大脑能力** | 固定代码模板拼接早报、雷达与到期文本 | **DeepSeek Agent 引擎**（`deepseek-chat` / `deepseek-reasoner`），支持多轮推理、流式输出与 Function Calling 工具调用 |
| **情报生产** | 爬虫抓取后直接截断原文硬拼 Markdown | 抓完原始数据后交由 DeepSeek 二次筛选、主题归纳、生成锐评与风险提示 |
| **交互方式** | 单向定时推送 + 网页控制台开关按钮 | **网页端 + Android 原生 App 双向对话**、自然语言操控基建、系统级分享链接直达总结 |
| **消息推送** | 依赖企业微信 / PushPlus 第三方下发 | **自建原生收件箱 (`notifications`) + SSE/WebSocket 长连接 + WorkManager 后台保活补发**，App 直接弹 Android 原生通知；企微与 PushPlus 降级为可选备份开关 |
| **数据兼容** | SQLite `/opt/bothub/data/bothub.db` | **100% 原地无痛升级**：新增表自动迁移，完全保留现有 `assets`、`tasks`、`configs`，`/api/v1/assets/overview` 对 Vesper 门户保持完全兼容 |

---

## 二、四大核心子系统详细设计

### 子系统 1：DeepSeek Agent 引擎 (`bothub/internal/agent/`)
1. **模型与协议兼容**：
   - 直连 DeepSeek 官方 OpenAI 兼容接口（`https://api.deepseek.com/v1/chat/completions`），支持配置自定义 `base_url`（方便未来无缝切入 Cloudflare AI Gateway 或其它兼容模型）。
   - 支持 **快速对话 (`deepseek-chat`)** 与 **深度推理 (`deepseek-reasoner`)** 切换，并在流式 SSE 中透传推理思考内容 (`reasoning_content`) 与工具执行轨迹。
2. **四大内置 Tool Calling 工具箱**：
   - **联网搜索与网页正文提取 (`web_search` / `read_url`)**：
     - `read_url(url)`：直接抓取并清洗目标网页 / GitHub 仓库页面 / 技术文章为纯净 Markdown（支持 Jina Reader 或本地可读性提取 + 长度截断保护），彻底实现「随手甩链接直接总结」。
     - `web_search(query)`：聚合实时搜索结果，支撑类似 GrokBot 的实时热点核查与问答。
   - **本机哪吒监控直连工具 (`query_nezha_nodes`)**：
     - 同一台机器内直读哪吒 SQLite 数据库（只读模式）或本机面板状态接口，回答「现在两台 VPS 负载如何」「RakSmart SV 本月双向流量剩多少」「最近有没有离线服务」。
   - **资产账单与调度直控工具 (`manage_assets` / `manage_tasks`)**：
     - 直接调用 BotHub 内部 `store` 与 `scheduler`，支持通过自然语言完成「列出所有临期资产」「帮我新增一个域名/VPS/会员资产」「把某项会员到期日改到下个月」「立刻触发一次开源雷达」。
   - **长期记忆工具 (`remember_fact` / `recall_memory`)**：
     - 在对话中自动提炼并持久化你的偏好、服务器架构背景与常用习惯到 SQLite，每次对话自动按相关性注入上下文。
3. **可切换人设 (Personas)**：
   - **Grok 锐评模式**：信息密度高、先给核心结论与犀利判断、不讲套话；
   - **执更管家模式**：严谨、条理清晰，偏重运维状态、风险排查与账单校验；
   - **架构师模式**：聚焦系统设计、代码结构、排查根因与方案对比。

---

### 子系统 2：AI 驱动的定时情报流 (`bothub/internal/modules/`)
对现有 `morning`、`radar` 与 `asset_watcher` 进行智能化重构（保留原调度器 `robfig/cron` 触发框架）：
1. **AI 智能晨报 (`morning`)**：
   - 汇总天气指数、哪吒双节点实时负载与流量百分比、近期临期资产后，交由 DeepSeek 浓缩成一份重点突出、自带待办优先级判断的晨间简报。
2. **AI 开源技术雷达 (`radar`)**：
   - 从 GitHub Trending 与 Hacker News 抓取高分条目并去重后，先由 DeepSeek 按你的技术栈兴趣做 Top N 筛选、提炼每条的核心亮点与适用场景，输出高可读中文简报。
3. **向后兼容**：
   - 若尚未配置 DeepSeek API Key 或模型调用超时，自动回退为现有的规则模板拼装，保证定时推送零中断。

---

### 子系统 3：自建原生推送流与离线补发 (`bothub/internal/pushstream/`)
彻底摆脱第三方推送的 IP 白名单、Markdown 长度与审核限制：
1. **持久化收件箱 (`notifications` 表)**：
   - 每次晨报、雷达、资产临期告警、哪吒回环告警或 Agent 异步任务完成时，先分配单调递增事件 ID 写入 SQLite `notifications` 表（记录 `id`, `module_id`, `title`, `summary`, `markdown_body`, `priority`, `created_at`, `read_at`）。
2. **实时长连接 + 断线补发协议**：
   - 提供 `GET /api/v2/events?last_event_id=<id>`（SSE 事件流，附带心跳保活）与 `GET /api/v2/notifications?after_id=<id>` 分页拉取接口。
   - **离线必达机制**：手机锁屏断网或被系统杀后台期间错过的通知，一旦重连或周期唤醒并携带本地记录的 `last_event_id`，服务端立即按序下发全部增量通知，由 Android 客户端在本地补齐通知栏提醒。
3. **原生推送与历史通道并存**：
   - 默认向内部收件箱与在线客户端直推；同时保留可选开关，允许在需要时同步抄送给企业微信「长庚执更」或 PushPlus。

---

### 子系统 4：Android 原生客户端 (`bothub/android/` · Kotlin + Jetpack Compose)
放在同一个 GitHub 仓库的 `android/` 子目录下，作为整个项目的专属移动控制台与推送接收端：
1. **视觉与交互规范（Material Design 3 × Vesper 暮光深空）**：
   - 遵循 Edge-to-Edge 沉浸式适配与 `WindowInsets` 避让，深空渐变暗色主题（兼容 Android 12+ 动态语义层级），所有可点击热区不小于 `48dp`。
   - **底部三栏结构 (`NavigationBar`)**：
     1. **对话 (Agent)**：流式 Markdown 对话界面、思考过程折叠卡片、人设快捷切换芯片、工具调用状态徽章；
     2. **情报箱 (Inbox)**：类似微信订阅号的时间线收件箱，统一承接晨报、AI 雷达、资产预警与哪吒告警，支持 Markdown 富文本阅读与已读标记；
     3. **中枢 (Hub)**：随手查看两台 VPS 脉搏摘要、资产到期倒计时、手动一键触发任务与 DeepSeek / 服务器连接配置。
2. **系统级高频能力**：
   - **系统分享面板一键直达 (`ACTION_SEND`)**：在手机浏览器、GitHub 客户端或微信看到文章/链接时，直接点系统「分享 -> Vesper」，自动拉起对话页并立即让 DeepSeek 抓网页总结。
   - **原生通知与后台保活**：
     - 结合可按需开启的前台保活常驻连接服务（实时秒推，锁屏直弹系统 Notification）+ **`WorkManager` 周期后台兜底同步**（省电不杀进程场景下定时拉取 `after_id` 增量），并针对告警与普通日报分设独立 `NotificationChannel`（支持响铃震动分级）。
   - **独立鉴权与可迁移性**：
     - App 端在设置页支持直接填入服务端 URL（默认 `https://bot.starryheavens.uk`）与专属 App Bearer Token（`POST /api/v2/auth/token` 或独立签名密钥），确保后续服务从 CloudCone LA 迁往主力机或经过 Cloudflare Access 保护的域名下（针对 App API 路径配置精确 Token / Header 规则）也能无缝连接。
3. **GitHub Actions 云端自动编发 APK**：
   - 在现有 `.github/workflows/` 中增加 Android Gradle 构建任务（CI 云端自动配置 JDK 与 Android SDK）。
   - **零本地构建负担**：你无需在 Mac 上长期启动 Android Studio，每次代码推送至仓库，Actions 在云端自动编译并签名发布 `.apk` 至 GitHub Release（或同步放到你的 `pan.starryheavens.uk` 专属目录），手机直接扫码或点击下载安装更新。

---

## 三、分阶段研发路线图（4 个 Sprint 稳步推进）

| Sprint | 阶段目标 | 核心交付物 | 验收标准 |
| :--- | :--- | :--- | :--- |
| **Sprint 1** | **DeepSeek Agent 大脑与核心工具箱（服务端）** | - 新增 `internal/agent/`、DeepSeek 流式与 Tool Calling 客户端<br>- 实现 `read_url`、`web_search`、`query_nezha_nodes`、`manage_assets`、`remember_fact`<br>- 扩展 `configs`（`deepseek_api_key`、`deepseek_base_url`、`ai_model`、`active_persona`）与长期记忆表 | 本地 `go test` 通过；网页端可通过 SSE 流式与 DeepSeek 对话、成功调用工具查哪吒状态/资产并记住偏好 |
| **Sprint 2** | **AI 情报接管 + 原生推送事件流 (`v2` API)** | - `morning` / `radar` 接入 DeepSeek 筛选总结与锐评（含降级兜底）<br>- 实现 `notifications` 收件箱表与 `GET /api/v2/events` (`last_event_id` 补发)<br>- 对接哪吒监控回环 Webhook (`/api/v1/webhook/nezha`) | 晨报、雷达与模拟断线重连测试全链路打通；所有新事件同时沉淀入收件箱并支持增量拉取 |
| **Sprint 3** | **Android 原生客户端上线 (`Kotlin + Jetpack Compose`)** | - 初始化 `bothub/android/` M3 Compose 工程<br>- 实现 **对话 / 情报箱 / 中枢** 三栏界面、流式 Markdown 渲染、系统「分享链接到 Vesper」<br>- 实现通知渠道分级、`WorkManager` + 后台增量拉取直弹通知 | 真机安装 APK 可正常登录 `bot.starryheavens.uk`、收发流式对话、分享网页链接直出总结、离线重连弹出通知 |
| **Sprint 4** | **CI/CD 全自动云端编译 APK + 门户与迁机收口** | - 完善 GitHub Actions（Go 二进制部署 VPS + Android APK 自动打包上传 Release）<br>- 支持 App 端一键检查更新或直接从 `pan.starryheavens.uk` 拉取新版 APK<br>- 更新单目录打包脚本，保证未来 1 分钟迁至主力机 | push 到仓库即可同时完成「服务端自动热更新 + 云端产出最新安装包」，旧 BotHub 正式退役 |

---

## 四、实施前的关键衔接说明

1. **鉴权与 Cloudflare Access 兼容性**：
   - `bot.starryheavens.uk` 目前前端套有 Cloudflare Zero Trust Access。Android 原生客户端通过非浏览器通道请求接口，在正式接入 App 时需要为 `/api/v2/*` 配置独立的高强度 API Token / Cloudflare Service Token 或精准路径 Bypass（应用侧严格校验高强度密钥），避免手机端被浏览器式 Passkey 页面拦截。
2. **零破坏平滑过渡**：
   - Sprint 1 和 Sprint 2 全都在现有 `bothub` Go 二进制和 `bothub.db` 上增量升级，升级期间 `/api/v1/assets/overview`、`starryheavens.uk` 门户展示、以及现有企业微信「长庚执更」提醒**完全不受影响、零停机切换**。
