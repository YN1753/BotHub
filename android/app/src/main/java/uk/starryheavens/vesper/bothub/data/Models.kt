package uk.starryheavens.vesper.bothub.data

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

data class AssetItem(
    val id: String,
    val name: String,
    val category: String,
    val spec: String,
    val price: String,
    val dueDate: String,
    val status: String,
    val daysLeft: Int,
    val autoRenew: Boolean
) {
    val isPerpetual: Boolean
        get() = dueDate.startsWith("2099")

    val isExpiringSoon: Boolean
        get() = !isPerpetual && daysLeft in 0..7

    val dueBadgeLabel: String
        get() = if (isPerpetual) "长期" else "${daysLeft}天"
}

data class TaskItem(
    val id: String,
    val name: String,
    val type: String,
    val cronExpr: String,
    val enabled: Boolean,
    val nextRunAt: String,
    val lastSummary: String
) {
    fun formattedSchedule(): String {
        val shortNext = formatShortNextRun(nextRunAt)
        return buildString {
            if (cronExpr.isNotBlank()) append(cronExpr.trim())
            if (shortNext.isNotEmpty()) {
                if (isNotEmpty()) append(" · 下次 ")
                append(shortNext)
            }
        }
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

data class WebSessionTicket(
    val cookieName: String,
    val cookieValue: String,
    val maxAge: Int
)

data class AppVersionInfo(
    val version: String,
    val apkUrl: String
)

data class AgentSessionSummary(
    val id: String,
    val title: String,
    val persona: String,
    val model: String,
    val updatedAt: String
)

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
    val botMode: String = "worker"
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

    fun statusLabel(isStreamingThisBot: Boolean): String {
        return when (liveState(isStreamingThisBot)) {
            "thinking" -> "思考中"
            "working" -> "执行中"
            "blocked" -> "需关注"
            "done" -> "已完成"
            else -> if (isGroup) "协作群组" else roleTitle.ifEmpty { "待命中" }
        }
    }
}

data class AgentHistoryMessage(
    val id: Long,
    val role: String,
    val content: String,
    val reasoningContent: String,
    val msgKind: String = "chat",
    val senderBotId: String = ""
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
        return ChatTurnUi(
            id = "${idPrefix}_$id",
            role = role,
            persona = senderBot?.name ?: defaultPersona,
            model = senderBot?.model ?: defaultModel,
            reasoning = reasoningContent,
            content = content,
            msgKind = msgKind,
            senderBotId = senderBot?.id ?: defaultBotId,
            avatarStyle = senderBot?.avatarStyle ?: defaultAvatarStyle
        )
    }
}

data class DashboardSnapshot(
    val version: String,
    val totalAssets: Int,
    val expiringCount: Int,
    val activeTasks: Int,
    val unreadNotifications: Int,
    val assets: List<AssetItem>,
    val tasks: List<TaskItem>
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

data class ChatTurnUi(
    val id: String,
    val role: String,
    val persona: String,
    val model: String,
    val reasoning: String = "",
    val tools: List<String> = emptyList(),
    val content: String = "",
    val msgKind: String = "chat",
    val senderBotId: String = "",
    val avatarStyle: String = "butler:gold"
)
