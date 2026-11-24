package uk.starryheavens.vesper.bothub.data

import androidx.compose.runtime.Immutable

private val MD_LINK_STRIP_REGEX = Regex("""\[([^\]\n]+)\]\(https?://[^\s)]+\)""")
private val MD_HEADING_STRIP_REGEX = Regex("""(?m)^#{1,6}\s+""")
private val MD_DECORATION_STRIP_REGEX = Regex("""(\*\*|__|`+)""")
private val WHITESPACE_COLLAPSE_REGEX = Regex("""\s+""")

private fun cleanMarkdownPreview(raw: String): String {
    if (raw.isBlank()) return ""
    return raw
        .replace(MD_LINK_STRIP_REGEX, "$1")
        .replace(MD_HEADING_STRIP_REGEX, "")
        .replace(MD_DECORATION_STRIP_REGEX, "")
        .replace(WHITESPACE_COLLAPSE_REGEX, " ")
        .trim()
}

@Immutable
data class NotificationItem(
    val id: Long,
    val moduleId: String,
    val title: String,
    val summary: String,
    val markdownBody: String,
    val priority: String,
    val createdAt: String,
    val readAt: String
) {
    val isUnread: Boolean
        get() = readAt.isEmpty()

    val isHighPriority: Boolean
        get() = priority.equals("high", ignoreCase = true) ||
            priority.equals("critical", ignoreCase = true)

    fun previewText(maxChars: Int = 96): String {
        val source = summary.ifEmpty { markdownBody }
        return cleanMarkdownPreview(source).take(maxChars)
    }
}

@Immutable
data class AssetCategoryItem(
    val id: String,
    val label: String,
    val icon: String = "tag"
)

@Immutable
data class AssetItem(
    val id: String,
    val name: String,
    val category: String,
    val spec: String,
    val price: String,
    val dueDate: String,
    val status: String,
    val daysLeft: Int,
    val autoRenew: Boolean,
    val warnDays: String = "[7, 3, 1]",
    val extraConfig: String = "{}"
) {
    val isPerpetual: Boolean
        get() = status == "长期有效" || dueDate.startsWith("2099") || daysLeft > 3650

    val isExpiringSoon: Boolean
        get() = !isPerpetual && status != "已停用" && status != "已归档" && daysLeft <= 7

    val dueBadgeLabel: String
        get() = when {
            isPerpetual -> "长期"
            daysLeft < 0 -> "逾期 ${kotlin.math.abs(daysLeft)} 天"
            else -> "${daysLeft} 天"
        }
}

@Immutable
data class TaskItem(
    val id: String,
    val name: String,
    val type: String,
    val cronExpr: String,
    val enabled: Boolean,
    val nextRunAt: String,
    val lastSummary: String,
    val model: String = "",
    val botId: String = "",
    val description: String = "",
    val runAt: String = "",
    val params: String = "{}"
) {
    fun formattedSchedule(): String {
        val shortRunAt = formatShortNextRun(runAt)
        val shortNext = formatShortNextRun(nextRunAt)
        if (cronExpr.isBlank() && shortRunAt.isNotEmpty()) {
            return "单次 · $shortRunAt"
        }
        return buildString {
            if (cronExpr.isNotBlank()) append(cronExpr.trim())
            if (shortNext.isNotEmpty()) {
                if (isNotEmpty()) append(" · 下次 ")
                append(shortNext)
            }
        }
    }
}

@Immutable
data class TodoItemUi(
    val id: String,
    val title: String,
    val notes: String = "",
    val status: String = "todo",
    val priority: String = "normal",
    val dueAt: String = "",
    val botId: String = "",
    val updatedAt: String = ""
) {
    val isDone: Boolean
        get() = status.equals("done", ignoreCase = true)
}

@Immutable
data class WorkNoteItemUi(
    val id: String,
    val title: String,
    val content: String = "",
    val category: String = "deliverable",
    val pinned: Boolean = false,
    val botId: String = "",
    val updatedAt: String = ""
) {
    fun previewText(maxChars: Int = 96): String =
        cleanMarkdownPreview(content).take(maxChars)
}

@Immutable
data class BotThreadItem(
    val id: String,
    val botId: String,
    val title: String,
    val summary: String = "",
    val msgCount: Int = 0,
    val archived: Boolean = false,
    val updatedAt: String = ""
)

data class BotMessagesPayload(
    val messages: List<AgentHistoryMessage>,
    val threads: List<BotThreadItem> = emptyList(),
    val activeThreadId: String = "",
    val compactionSummary: String = ""
)

fun normalizeModelIdForSinglePool(raw: String): String {
    val clean = raw.trim()
    return when (clean.lowercase()) {
        "", "auto", "auto:free", "auto:paid", "free-pool", "paid-pool", "primary" -> "auto"
        else -> clean
    }
}

private fun formatShortNextRun(raw: String): String {
    val clean = raw.trim()
    if (clean.isEmpty()) return ""
    // Expected e.g. "2026-11-19 03:00:00" -> "11-19 03:00"
    if (clean.length >= 16 && clean[4] == '-' && clean[10] == ' ') {
        return clean.substring(5, 16)
    }
    return clean.take(16)
}

data class AppVersionInfo(
    val version: String,
    val apkUrl: String
)

@Immutable
data class ModelOptionItem(
    val id: String,
    val label: String,
    val isAuto: Boolean = false,
    val isLight: Boolean = false,
    val provider: String = "",
    val model: String = "",
    val available: Boolean = true
)

@Immutable
data class ModelProviderItem(
    val id: String,
    val name: String,
    val protocol: String = "openai",
    val baseUrl: String = "",
    val apiKey: String = "",
    val apiKeys: List<String> = emptyList(),
    val enabled: Boolean = true,
    val note: String = ""
) {
    fun effectiveApiKeys(): List<String> {
        val raw = if (apiKeys.isNotEmpty()) apiKeys else listOf(apiKey)
        return raw.flatMap { it.split('\n', '\r', ',', ';') }
            .map { it.trim() }
            .filter { it.isNotEmpty() }
            .distinct()
    }
}

data class UpstreamFetchResult(
    val models: List<String>,
    val filteredOutCount: Int = 0,
    val failedVerifyCount: Int = 0
)

@Immutable
data class PooledModelItem(
    val providerId: String,
    val modelId: String,
    val displayName: String = "",
    val enabled: Boolean = true,
    val priority: Int = 10,
    val isLight: Boolean = false,
    val inputPricePerM: Double = 0.0,
    val outputPricePerM: Double = 0.0,
    val supportsTools: Boolean? = null,
    val supportsThinking: Boolean? = null,
    val lastLatencyMs: Long = 0L
) {
    val canonicalTargetId: String
        get() = "pool:${providerId.trim()}::${modelId.trim()}"
}

@Immutable
data class ModelHealthItem(
    val targetId: String,
    val status: String = "healthy",
    val successCount: Long = 0L,
    val failCount: Long = 0L,
    val lastLatencyMs: Long = 0L,
    val lastError: String = "",
    val cooldownSec: Long = 0L
)

@Immutable
data class ModelUsageModelStatItem(
    val targetId: String = "",
    val modelId: String = "",
    val calls: Int = 0,
    val tokens: Int = 0,
    val cachedTokens: Int = 0,
    val costUsd: Double = 0.0,
    val avgLatencyMs: Long = 0L
)

@Immutable
data class ModelUsageCallerStatItem(
    val botId: String = "",
    val taskId: String = "",
    val source: String = "",
    val calls: Int = 0,
    val tokens: Int = 0,
    val costUsd: Double = 0.0
)

@Immutable
data class ModelUsageSummaryItem(
    val days: Int = 7,
    val totalCalls: Int = 0,
    val successCalls: Int = 0,
    val totalPromptTokens: Int = 0,
    val totalCachedTokens: Int = 0,
    val totalCompletionTokens: Int = 0,
    val totalTokens: Int = 0,
    val cacheHitRate: Double = 0.0,
    val totalCostUsd: Double = 0.0,
    val totalCostCny: Double = 0.0,
    val topModels: List<ModelUsageModelStatItem> = emptyList(),
    val topCallers: List<ModelUsageCallerStatItem> = emptyList()
)

@Immutable
data class SkillItemUi(
    val id: String,
    val slug: String = "",
    val name: String,
    val description: String = "",
    val kind: String = "prompt",
    val httpUrl: String = "",
    val toolCount: Int = 0,
    val enabled: Boolean = true
) {
    val isAppConnector: Boolean
        get() = id.trim().startsWith("mcp_app_")

    val isBuiltinMcp: Boolean
        get() = kind == "builtin_mcp" || id.trim().startsWith("mcp_")
}

@Immutable
data class MemoryMarkdownPayload(
    val scope: String = "",
    val count: Int = 0,
    val markdown: String = "",
    val lastDreamAt: String = "",
    val lastDreamSummary: String = ""
)

@Immutable
data class MemoryItemUi(
    val id: Long,
    val scope: String = "global",
    val category: String = "user",
    val factKey: String = "",
    val content: String = "",
    val importance: Int = 3,
    val pinned: Boolean = false,
    val updatedAt: String = ""
)

@Immutable
data class BatchModelProbeEntryItem(
    val targetId: String,
    val providerId: String,
    val modelId: String,
    val ok: Boolean = false,
    val latencyMs: Long = 0L,
    val error: String = "",
    val supportsTools: Boolean? = null,
    val supportsThinking: Boolean? = null
)

@Immutable
data class BatchModelProbePayload(
    val results: List<BatchModelProbeEntryItem> = emptyList(),
    val modelHealth: Map<String, ModelHealthItem> = emptyMap(),
    val pooledModels: List<PooledModelItem>? = null,
    val modelOptions: List<ModelOptionItem>? = null
)

@Immutable
data class ModelConfigPayload(
    val providers: List<ModelProviderItem> = emptyList(),
    val models: List<PooledModelItem> = emptyList(),
    val strategy: String = "priority",
    val modelOptions: List<ModelOptionItem> = emptyList(),
    val modelHealth: Map<String, ModelHealthItem> = emptyMap(),
    val usageSummary: ModelUsageSummaryItem = ModelUsageSummaryItem(days = 7),
    val usage30d: ModelUsageSummaryItem = ModelUsageSummaryItem(days = 30)
)

@Immutable
data class ModelProbeResultItem(
    val ok: Boolean,
    val latencyMs: Long = 0L,
    val error: String = "",
    val supportsTools: Boolean? = null,
    val supportsThinking: Boolean? = null,
    val modelHealth: Map<String, ModelHealthItem> = emptyMap(),
    val pooledModels: List<PooledModelItem>? = null,
    val modelOptions: List<ModelOptionItem>? = null
)

@Immutable
data class BootstrapPayload(
    val bots: List<BotItem>,
    val skills: List<SkillItemUi> = emptyList(),
    val notifications: List<NotificationItem>,
    val dashboard: DashboardSnapshot,
    val stashItems: List<StashItemUi> = emptyList(),
    val todos: List<TodoItemUi> = emptyList(),
    val notes: List<WorkNoteItemUi> = emptyList(),
    val modelOptions: List<ModelOptionItem> = emptyList(),
    val modelProviders: List<ModelProviderItem> = emptyList(),
    val pooledModels: List<PooledModelItem> = emptyList(),
    val poolStrategy: String = "priority",
    val modelHealth: Map<String, ModelHealthItem> = emptyMap(),
    val modelUsageSummary: ModelUsageSummaryItem = ModelUsageSummaryItem(days = 7),
    val modelUsage30d: ModelUsageSummaryItem = ModelUsageSummaryItem(days = 30),
    val revisions: Map<String, Long> = emptyMap()
)

data class DashboardSlicePayload(
    val bots: List<BotItem>? = null,
    val skills: List<SkillItemUi>? = null,
    val assets: List<AssetItem>? = null,
    val assetCategories: List<AssetCategoryItem>? = null,
    val fxRates: Map<String, Double>? = null,
    val tasks: List<TaskItem>? = null,
    val todos: List<TodoItemUi>? = null,
    val notes: List<WorkNoteItemUi>? = null,
    val notifications: List<NotificationItem>? = null,
    val stashItems: List<StashItemUi>? = null,
    val modelOptions: List<ModelOptionItem>? = null,
    val modelProviders: List<ModelProviderItem>? = null,
    val pooledModels: List<PooledModelItem>? = null,
    val poolStrategy: String? = null,
    val modelHealth: Map<String, ModelHealthItem>? = null,
    val modelUsageSummary: ModelUsageSummaryItem? = null,
    val modelUsage30d: ModelUsageSummaryItem? = null,
    val revisions: Map<String, Long> = emptyMap()
)

fun formatImTime(raw: String): String {
    val clean = raw.trim()
    if (clean.isEmpty()) return ""
    // Format "2026-11-18 08:05:11" -> "08:05" if today, else "11-18"
    if (clean.length >= 16 && clean[4] == '-' && clean[10] == ' ') {
        val today = java.time.LocalDate.now().toString()
        return if (clean.startsWith(today)) {
            clean.substring(11, 16)
        } else {
            clean.substring(5, 10)
        }
    }
    return clean.take(10)
}

@Immutable
data class BotItem(
    val id: String,
    val name: String,
    val roleTitle: String,
    val avatarStyle: String,
    val model: String,
    val enableThinking: Boolean,
    val state: String,
    val unreadCount: Int,
    val lastSummary: String,
    val isGroup: Boolean = false,
    val memberIds: List<String> = emptyList(),
    val botMode: String = "worker",
    val isPinned: Boolean = false,
    val isBuiltin: Boolean = false,
    val traits: String = "",
    val systemPrompt: String = "",
    val groupMode: String = "smart",
    val announcement: String = "",
    val updatedAt: String = "",
    val allowedTools: List<String> = emptyList(),
    val skillIds: List<String> = emptyList()
) {
    fun resolveGroupMembers(allBots: List<BotItem>): List<BotItem> {
        val specialists = allBots.filter { !it.isGroup }
        if (memberIds.isNotEmpty()) {
            val byId = specialists.associateBy { it.id }
            val found = memberIds.mapNotNull { byId[it] }
            if (found.isNotEmpty()) return found
        }
        return specialists.take(4)
    }

    fun liveState(isStreamingThisBot: Boolean): String =
        if (isStreamingThisBot) "working" else state

    fun modeBadgeLabel(): String {
        if (isGroup) return "群聊"
        return when (botMode.lowercase()) {
            "companion" -> "伴聊"
            "worker" -> "助理"
            else -> "全能"
        }
    }

    fun groupModeLabel(): String {
        return when (groupMode.lowercase()) {
            "sequential" -> "轮流接力"
            "parallel" -> "全员并发"
            "mention_only" -> "仅@点名"
            else -> "智能协同"
        }
    }

    fun conversationSnippet(isStreamingThisBot: Boolean): String {
        if (isStreamingThisBot) {
            return if (isGroup) "群成员正在协作发言..." else "对方正在输入..."
        }
        val base = cleanMarkdownPreview(lastSummary)
            .ifEmpty { traits.trim() }
            .ifEmpty { roleTitle.trim() }
            .ifEmpty { if (isGroup) "点击进入群聊开始讨论" else "点击打个招呼吧" }
        return base
    }
}

@Immutable
data class ChatToolTraceUi(
    val id: String,
    val name: String,
    val displayName: String,
    val arguments: String = "",
    val result: String = "",
    val status: String = "已完成"
)

fun formatHumanToolName(toolName: String): String {
    val clean = toolName.trim()
    return when {
        clean == "web_search" -> "联网搜索"
        clean == "read_url" -> "阅读网页"
        clean == "manage_bots" -> "Bot 与群聊管理"
        clean == "manage_models" -> "AI 模型池巡检调优"
        clean == "manage_skills" -> "MCP 与扩展中心"
        clean == "mount_mcp" -> "动态挂载 MCP"
        clean == "consult_bot" -> "协同成员"
        clean == "manage_todos" -> "待办管理"
        clean == "manage_notes" -> "工作文档"
        clean == "manage_stash" -> "收藏闪念"
        clean == "manage_tasks" -> "定时提醒"
        clean == "workspace_briefing" -> "今日简报"
        clean == "send_notification" -> "主动推送"
        clean == "calculate_eval" -> "精确计算"
        clean == "datetime_calc" -> "日期推算"
        clean == "remember_fact" -> "写入记忆"
        clean == "recall_memory" -> "检索记忆"
        clean == "manage_memory" -> "整理记忆"
        clean == "manage_assets" -> "资产台账"
        clean == "query_nezha_nodes" -> "探针巡检"
        clean == "github_connector" -> "GitHub 连接器"
        clean == "email_connector" -> "Gmail / 邮箱连接器"
        clean == "cloudflare_connector" -> "Cloudflare 连接器"
        clean == "rss_connector" -> "RSS 资讯订阅连接器"
        clean == "webdav_connector" -> "WebDAV 云盘连接器"
        clean.startsWith("mcp_") -> clean.removePrefix("mcp_").replace("__", " · ")
        clean.startsWith("skill_") -> clean.removePrefix("skill_")
        clean.isBlank() -> "扩展能力"
        else -> clean
    }
}

data class AgentHistoryMessage(
    val id: Long,
    val role: String,
    val content: String,
    val reasoningContent: String,
    val msgKind: String = "chat",
    val senderBotId: String = "",
    val modelUsed: String = "",
    val modelTargetId: String = "",
    val attachments: List<String> = emptyList(),
    val toolTraces: List<ChatToolTraceUi> = emptyList()
) {
    fun toChatTurnUi(
        idPrefix: String,
        defaultPersona: String,
        defaultModel: String,
        defaultBotId: String = "",
        defaultAvatarStyle: String = "butler:gold",
        bots: List<BotItem> = emptyList()
    ): ChatTurnUi {
        val senderBot = if (senderBotId.isNotEmpty()) {
            bots.find { it.id == senderBotId }
        } else {
            null
        } ?: if (defaultBotId.isNotEmpty()) {
            bots.find { it.id == defaultBotId }
        } else {
            null
        }
        val derivedTools = toolTraces
            .map { it.displayName.ifBlank { formatHumanToolName(it.name) } }
            .distinct()
        return ChatTurnUi(
            id = "${idPrefix}_$id",
            role = role,
            persona = senderBot?.name ?: defaultPersona,
            model = modelUsed.ifBlank { senderBot?.model ?: defaultModel },
            modelTargetId = modelTargetId,
            reasoning = reasoningContent,
            tools = derivedTools,
            toolTraces = toolTraces,
            content = content,
            attachments = attachments,
            msgKind = msgKind,
            senderBotId = senderBot?.id ?: defaultBotId,
            avatarStyle = senderBot?.avatarStyle ?: defaultAvatarStyle
        )
    }
}

@Immutable
data class DashboardSnapshot(
    val version: String,
    val totalAssets: Int,
    val expiringCount: Int,
    val activeTasks: Int,
    val unreadNotifications: Int,
    val assets: List<AssetItem>,
    val tasks: List<TaskItem>,
    val todos: List<TodoItemUi> = emptyList(),
    val notes: List<WorkNoteItemUi> = emptyList(),
    val assetCategories: List<AssetCategoryItem> = emptyList(),
    val fxRates: Map<String, Double> = emptyMap(),
    val modelOptions: List<ModelOptionItem> = emptyList(),
    val revisions: Map<String, Long> = emptyMap()
)

enum class ShareKind(
    val wireKind: String,
    val badgeLabel: String,
    val shareHeaderLabel: String,
    val avatarStyle: String
) {
    BILIBILI(
        wireKind = "bilibili",
        badgeLabel = "Bilibili",
        shareHeaderLabel = "Bilibili 视频",
        avatarStyle = "radar:emerald"
    ),
    GITHUB(
        wireKind = "github",
        badgeLabel = "GitHub",
        shareHeaderLabel = "GitHub 仓库",
        avatarStyle = "sentinel:indigo"
    ),
    XIAOHONGSHU(
        wireKind = "xiaohongshu",
        badgeLabel = "小红书",
        shareHeaderLabel = "小红书笔记",
        avatarStyle = "sentinel:rose"
    ),
    DOUYIN(
        wireKind = "douyin",
        badgeLabel = "抖音",
        shareHeaderLabel = "抖音视频",
        avatarStyle = "butler:cyan"
    ),
    WECHAT(
        wireKind = "wechat",
        badgeLabel = "公众号",
        shareHeaderLabel = "公众号文章",
        avatarStyle = "radar:emerald"
    ),
    WEB_LINK(
        wireKind = "link",
        badgeLabel = "Web",
        shareHeaderLabel = "网页外链",
        avatarStyle = "butler:gold"
    ),
    PLAIN_NOTE(
        wireKind = "note",
        badgeLabel = "闪念",
        shareHeaderLabel = "纯文字闪念",
        avatarStyle = "swarm:violet"
    );

    companion object {
        fun fromWireKind(raw: String?): ShareKind {
            val normalized = raw?.trim()?.lowercase().orEmpty()
            return entries.firstOrNull { it.wireKind == normalized } ?: WEB_LINK
        }
    }
}

data class SharePreviewInfo(
    val kind: ShareKind,
    val candidateTitle: String,
    val extractedUrl: String
) {
    val kindLabel: String
        get() = kind.shareHeaderLabel
}

data class PrefetchedShareMeta(
    val canonicalUrl: String = "",
    val title: String = "",
    val excerpt: String = "",
    val tags: String = "",
    val imageDataUrl: String = ""
) {
    val isEmpty: Boolean
        get() = canonicalUrl.isBlank() &&
            title.isBlank() &&
            excerpt.isBlank() &&
            tags.isBlank() &&
            imageDataUrl.isBlank()
}

@Immutable
data class StashItemUi(
    val id: String,
    val kind: String,
    val url: String,
    val domain: String,
    val title: String,
    val rawInput: String,
    val userNote: String,
    val aiTldr: String,
    val points: List<String>,
    val contentExcerpt: String = "",
    val tags: String,
    val status: String,
    val errorMsg: String,
    val pinned: Boolean,
    val readCount: Int,
    val createdAt: String
) {
    val isUnread: Boolean
        get() = status != "archived" && readCount == 0

    val shareKind: ShareKind
        get() = ShareKind.fromWireKind(kind)
}

@Immutable
data class ChatTurnUi(
    val id: String,
    val role: String,
    val persona: String,
    val model: String,
    val modelTargetId: String = "",
    val reasoning: String = "",
    val tools: List<String> = emptyList(),
    val toolTraces: List<ChatToolTraceUi> = emptyList(),
    val content: String = "",
    val attachments: List<String> = emptyList(),
    val msgKind: String = "chat",
    val senderBotId: String = "",
    val avatarStyle: String = "butler:gold"
)
