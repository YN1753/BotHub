package uk.starryheavens.vesper.bothub.ui.screens

import android.net.Uri
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.scaleOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.ArrowUpward
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material.icons.filled.Build
import androidx.compose.material.icons.filled.CheckCircleOutline
import androidx.compose.material.icons.filled.ChevronLeft
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.DeleteSweep
import androidx.compose.material.icons.filled.Description
import androidx.compose.material.icons.filled.ErrorOutline
import androidx.compose.material.icons.filled.FormatQuote
import androidx.compose.material.icons.filled.GroupAdd
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.Image
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material.icons.filled.MoreHoriz
import androidx.compose.material.icons.filled.Psychology
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Share
import androidx.compose.material.icons.filled.Stop
import androidx.compose.material.icons.filled.TaskAlt
import androidx.compose.material.icons.outlined.DeleteOutline
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import org.json.JSONObject
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.BotThreadItem
import uk.starryheavens.vesper.bothub.data.ChatToolTraceUi
import uk.starryheavens.vesper.bothub.data.ChatTurnUi
import uk.starryheavens.vesper.bothub.data.ModelOptionItem
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.data.SkillItemUi
import uk.starryheavens.vesper.bothub.data.TodoItemUi
import uk.starryheavens.vesper.bothub.data.formatHumanToolName
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.components.BotAvatar
import uk.starryheavens.vesper.bothub.ui.components.ChatStructuredToolCards
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.RichMarkdownText
import uk.starryheavens.vesper.bothub.ui.components.VesperButtonTone
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperDivider
import uk.starryheavens.vesper.bothub.ui.components.VesperIconButton
import uk.starryheavens.vesper.bothub.ui.components.VesperMenuAction
import uk.starryheavens.vesper.bothub.ui.components.VesperPopoverMenu
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.VesperTextField
import uk.starryheavens.vesper.bothub.ui.components.vesperPressable
import uk.starryheavens.vesper.bothub.ui.sheets.ModelPickerSheet
import uk.starryheavens.vesper.bothub.ui.sheets.resolveModelOptionDisplayName
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

private data class PlusActionTile(
    val label: String,
    val icon: ImageVector,
    val accent: Color,
    val onClick: () -> Unit
)

private data class SlashCommandItem(
    val cmd: String,
    val title: String,
    val desc: String,
    val prompt: String = "",
    val action: String = "",
    val anyOfTools: Set<String> = emptySet()
)

private val BUILTIN_SLASH_SPECS = listOf(
    SlashCommandItem("/search", "全网检索", "联网搜索最新资讯与公开资料", prompt = "请帮我联网搜索：", anyOfTools = setOf("web_search")),
    SlashCommandItem("/assets", "资产全面审计", "盘点 VPS、域名与订阅到期风险", prompt = "请调用工具盘点当前全部数字资产与临期风险并给出续费建议。", anyOfTools = setOf("manage_assets")),
    SlashCommandItem("/nezha", "探针节点巡检", "查看哪吒节点在线率与负载情况", prompt = "请调用 query_nezha_nodes 检查哪吒监控面板节点的实时在线与健康状态。", anyOfTools = setOf("query_nezha_nodes")),
    SlashCommandItem("/todo", "记录或核查待办", "添加行动项、查看未完成或勾选完结事项", prompt = "请帮我整理或记录以下待办事项：", anyOfTools = setOf("manage_todos")),
    SlashCommandItem("/plan", "目标拆解排期", "把复杂目标拆解成多条带优先级的待办子任务", prompt = "请调用 manage_todos(action=\"batch_add\") 将以下目标拆解为可执行的分步行动清单并设定优先级：", anyOfTools = setOf("manage_todos")),
    SlashCommandItem("/note", "整理产出工作手记", "将方案、总结或草稿沉淀到工作手记", prompt = "请将以下内容整理为结构化 Markdown 工作文档并保存：", anyOfTools = setOf("manage_notes")),
    SlashCommandItem("/stash", "检索或收纳收藏", "全文搜索收藏条目或秒收新链接与灵感", prompt = "请调用 manage_stash 检索或收纳以下内容：", anyOfTools = setOf("manage_stash")),
    SlashCommandItem("/tasks", "调度队列核查", "检查各自动化例程状态与下次触发时间", prompt = "请核查当前所有自动化调度任务的运行状态与上次执行摘要。", anyOfTools = setOf("manage_tasks")),
    SlashCommandItem("/remind", "创建定时/单次提醒", "设定一次性定时提醒或周期性自动任务", prompt = "请帮我设定一个定时提醒：", anyOfTools = setOf("manage_tasks")),
    SlashCommandItem("/briefing", "今日行动全景简报", "汇总逾期/今日待办、临期资产与未读预警", prompt = "请调用 workspace_briefing 为我梳理今日重点待办、日程与风险，并给出行动建议。", anyOfTools = setOf("workspace_briefing")),
    SlashCommandItem("/models", "巡检模型池与费用", "查看统一模型池健康度、缓存命中率及近 7 日费用统计", prompt = "请调用 manage_models(action=\"status\") 巡检当前统一模型池健康状态及近 7 日用量与费用统计。", anyOfTools = setOf("manage_models")),
    SlashCommandItem("/newbot", "创建专属 Bot", "用自然语言描述角色定位，自动配置并创建", prompt = "请调用 manage_bots(action=\"create_bot\") 帮我创建一个专属 Bot：", anyOfTools = setOf("manage_bots")),
    SlashCommandItem("/group", "组建协同群聊", "指定成员与讨论主题，一键组建多 Bot 群聊", prompt = "请调用 manage_bots(action=\"create_group\") 帮我拉一个多 Bot 协同群聊：", anyOfTools = setOf("manage_bots")),
    SlashCommandItem("/newskill", "创建自定义技能", "创建 Prompt 或 Webhook 技能并挂载给 Bot", prompt = "请调用 manage_skills(action=\"upsert\") 帮我创建一个自定义技能：", anyOfTools = setOf("manage_skills")),
    SlashCommandItem("/consult", "协调成员协办", "调用其他专精 Bot 协助完成子任务并汇总", prompt = "请协调工作区里最合适的专业成员共同处理：", anyOfTools = setOf("consult_bot")),
    SlashCommandItem("/calc", "精确表达式计算", "使用高精度计算引擎核算费用或比例", prompt = "请调用 calculate_eval 精确计算：", anyOfTools = setOf("calculate_eval")),
    SlashCommandItem("/memories", "回忆关键信息", "检索与当前会话相关的历史重要记忆", prompt = "请先检索我们的历史记忆，梳理目前你记住的关键事实与约定。", anyOfTools = setOf("manage_memory", "recall_memory")),
    SlashCommandItem("/dream", "AutoDream 深度整理记忆", "回放近期对话、消歧矛盾记忆并重建 MEMORY.md 索引", prompt = "请调用 manage_memory(action=\"dream\") 执行 AutoDream 四阶段深度记忆整理，并简报本次提炼与清理结果。", anyOfTools = setOf("manage_memory")),
    SlashCommandItem("/compact", "压缩历史上下文", "将较早的对话记录压缩为滚动摘要，降低 Token 开销", action = "compact_thread"),
    SlashCommandItem("/thread", "新建独立话题", "在当前会话内开启一条干净的新话题线", action = "new_thread"),
    SlashCommandItem("/summarize", "提炼话题存记忆", "提炼当前话题核心结论并沉淀到专属记忆库", action = "summarize_thread"),
    SlashCommandItem("/clear", "清空当前对话", "清空当前话题聊天时间线", action = "clear_timeline")
)

private fun buildBotSlashCommands(bot: BotItem?, skills: List<SkillItemUi>): List<SlashCommandItem> {
    val allowedTools = bot?.allowedTools?.toSet().orEmpty()
    val boundSkillIds = bot?.skillIds?.toSet().orEmpty()
    val out = mutableListOf<SlashCommandItem>()

    skills.forEach { sk ->
        if (sk.enabled && sk.id in boundSkillIds && sk.slug.isNotBlank()) {
            out.add(
                SlashCommandItem(
                    cmd = "/${sk.slug}",
                    title = "技能 · ${sk.name}",
                    desc = sk.description.ifBlank { if (sk.kind == "webhook") "调用外部技能 skill_${sk.slug}" else "按专属技能规范响应" },
                    prompt = if (sk.kind == "webhook") "请使用 skill_${sk.slug} 工具协助处理当前问题：" else "请遵循「${sk.name}」技能规范回答："
                )
            )
        }
    }

    BUILTIN_SLASH_SPECS.forEach { spec ->
        if (spec.anyOfTools.isEmpty() || allowedTools.isEmpty() || spec.anyOfTools.any { it in allowedTools }) {
            out.add(spec)
        }
    }
    return out
}

/**
 * Extracts a concise, human-readable one-line summary for a tool call step from its arguments & status.
 */
internal fun summarizeToolStep(trace: ChatToolTraceUi): String {
    val rawArgs = trace.arguments.trim()
    val argSummary = if (rawArgs.startsWith("{")) {
        runCatching {
            val obj = JSONObject(rawArgs)
            val priorityKeys = listOf("query", "q", "url", "action", "title", "name", "expression", "expr", "bot_id", "category", "keyword")
            val picked = priorityKeys.mapNotNull { key ->
                val v = obj.optString(key, "").trim()
                if (v.isNotEmpty()) v else null
            }
            picked.take(2).joinToString(" · ")
        }.getOrDefault("")
    } else {
        rawArgs.replace('\n', ' ').take(48)
    }
    return argSummary.take(64)
}

private fun isToolPending(status: String): Boolean =
    status == "进行中" || status == "执行中" || status.equals("running", ignoreCase = true)

private fun isToolError(status: String): Boolean =
    status.equals("error", ignoreCase = true) || status.contains("失败") || status.contains("异常")

@OptIn(ExperimentalFoundationApi::class, ExperimentalMaterial3Api::class)
@Composable
fun ChatRoomScreen(
    activeBot: BotItem?,
    allBots: List<BotItem>,
    availableSkills: List<SkillItemUi> = emptyList(),
    otherUnreadCount: Int,
    activeModel: String,
    modelOptions: List<ModelOptionItem> = emptyList(),
    enableThinking: Boolean,
    messages: List<ChatTurnUi>,
    botThreads: List<BotThreadItem> = emptyList(),
    activeThreadId: String = "",
    compactionSummary: String = "",
    compactingThread: Boolean = false,
    pendingImages: List<String> = emptyList(),
    uploadingImage: Boolean = false,
    inputText: String,
    isStreaming: Boolean,
    quotedNotification: NotificationItem? = null,
    onDismissQuotedNotification: () -> Unit = {},
    onBack: () -> Unit,
    onInputChange: (String) -> Unit,
    onPickImageUri: (Uri) -> Unit = {},
    onRemovePendingImage: (String) -> Unit = {},
    onLoadChatImage: suspend (String) -> ImageBitmap? = { null },
    onSendPrompt: (String) -> Unit,
    onStop: () -> Unit,
    onModelChange: (String) -> Unit,
    onThinkingChange: (Boolean) -> Unit,
    onSelectThread: (String) -> Unit = {},
    onCreateThread: (String) -> Unit = {},
    onDeleteThread: (String) -> Unit = {},
    onSummarizeThread: () -> Unit = {},
    onCompactThread: () -> Unit = {},
    onForkFromTurn: (ChatTurnUi) -> Unit = {},
    onRetryTurnWithModel: (ChatTurnUi, String) -> Unit = { _, _ -> },
    onCopyText: (String) -> Unit,
    onDeleteTurn: (ChatTurnUi) -> Unit = {},
    onForwardMessage: (BotItem, String, String, String, Boolean) -> Unit = { _, _, _, _, _ -> },
    liveTodos: List<TodoItemUi> = emptyList(),
    onConvertTurnToTodo: (ChatTurnUi) -> Unit = {},
    onSaveTurnAsNote: (ChatTurnUi) -> Unit = {},
    onQuickToggleTodo: (String, String) -> Unit = { _, _ -> },
    searchOpenToken: Int = 0,
    onOpenChatInfo: () -> Unit,
    onOpenBotProfile: (BotItem) -> Unit,
    onInviteToGroup: () -> Unit,
    onClearHistory: () -> Unit,
    modifier: Modifier = Modifier
) {
    val haptics = LocalHapticFeedback.current
    val focusManager = LocalFocusManager.current
    val scope = rememberCoroutineScope()
    val listState = rememberLazyListState()
    var showPlusDrawer by remember { mutableStateOf(false) }
    var showModelPickerSheet by remember { mutableStateOf(false) }
    var retryingTurn by remember { mutableStateOf<ChatTurnUi?>(null) }
    var composerFocused by remember { mutableStateOf(false) }
    var quoteTarget by remember { mutableStateOf<Pair<String, String>?>(null) }
    var forwardPayload by remember { mutableStateOf<Pair<String, String>?>(null) }
    var dismissAnnouncement by remember(activeBot?.id) { mutableStateOf(false) }
    var expandCompactionSummary by remember(activeBot?.id, activeThreadId) { mutableStateOf(false) }
    var showSearchBar by remember(activeBot?.id) { mutableStateOf(false) }
    var searchQuery by remember(activeBot?.id) { mutableStateOf("") }
    var activeMatchCursor by remember(activeBot?.id) { mutableStateOf(0) }

    val imagePickerLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.GetContent()
    ) { uri: Uri? ->
        if (uri != null) {
            onPickImageUri(uri)
        }
    }

    BackHandler {
        if (showSearchBar) {
            showSearchBar = false
            searchQuery = ""
        } else if (showPlusDrawer) {
            showPlusDrawer = false
        } else {
            onBack()
        }
    }

    LaunchedEffect(searchOpenToken) {
        if (searchOpenToken > 0) {
            showSearchBar = true
        }
    }

    val theme = BotHubThemeTokens
    val normalizedSearch = searchQuery.trim().lowercase()
    val searchMatchIndices = remember(messages, normalizedSearch) {
        if (normalizedSearch.isEmpty()) {
            emptyList()
        } else {
            messages.mapIndexedNotNull { idx, turn ->
                if (turn.content.lowercase().contains(normalizedSearch) ||
                    turn.reasoning.lowercase().contains(normalizedSearch)
                ) {
                    idx
                } else {
                    null
                }
            }
        }
    }
    val safeMatchCursor = if (searchMatchIndices.isEmpty()) {
        0
    } else {
        activeMatchCursor.coerceIn(0, searchMatchIndices.lastIndex)
    }
    val activeMatchedTurnId = if (searchMatchIndices.isNotEmpty()) {
        messages.getOrNull(searchMatchIndices[safeMatchCursor])?.id
    } else {
        null
    }
    val matchedTurnIds = remember(messages, searchMatchIndices) {
        searchMatchIndices.mapNotNull { messages.getOrNull(it)?.id }.toSet()
    }

    LaunchedEffect(normalizedSearch) {
        activeMatchCursor = if (searchMatchIndices.isNotEmpty()) {
            searchMatchIndices.lastIndex
        } else {
            0
        }
    }

    LaunchedEffect(activeMatchedTurnId, showSearchBar) {
        if (showSearchBar && activeMatchedTurnId != null) {
            val targetIdx = messages.indexOfFirst { it.id == activeMatchedTurnId }
            if (targetIdx >= 0) {
                listState.animateScrollToItem(targetIdx)
            }
        }
    }

    val isGroup = activeBot?.isGroup == true
    val groupMembers = remember(activeBot, allBots) {
        if (isGroup && activeBot != null) activeBot.resolveGroupMembers(allBots) else emptyList()
    }
    val lastAssistantTurn = messages.lastOrNull { it.role == "assistant" }
    val lastAssistantId = lastAssistantTurn?.id
    val lastStreamLen = (lastAssistantTurn?.content?.length ?: 0) + (lastAssistantTurn?.reasoning?.length ?: 0)
    val modelDisplayName = remember(activeModel, modelOptions) {
        resolveModelOptionDisplayName(activeModel, modelOptions)
    }

    val isNearBottom by remember {
        derivedStateOf {
            val info = listState.layoutInfo
            val total = info.totalItemsCount
            if (total == 0) true
            else {
                val lastVisible = info.visibleItemsInfo.lastOrNull()
                lastVisible != null && lastVisible.index >= total - 2
            }
        }
    }

    LaunchedEffect(activeBot?.id, activeThreadId) {
        if (messages.isNotEmpty()) {
            listState.scrollToItem(messages.lastIndex, 200_000)
        }
    }

    LaunchedEffect(messages.size) {
        if (messages.isNotEmpty() && isNearBottom) {
            listState.scrollToItem(messages.lastIndex, 200_000)
        }
    }

    LaunchedEffect(isStreaming, lastStreamLen) {
        if (isStreaming && messages.isNotEmpty() && isNearBottom && !listState.isScrollInProgress) {
            delay(75L)
            if (!listState.isScrollInProgress) {
                listState.scrollToItem(messages.lastIndex, 200_000)
            }
        }
    }

    LaunchedEffect(composerFocused) {
        if (composerFocused) {
            showPlusDrawer = false
        }
    }

    LaunchedEffect(listState.isScrollInProgress) {
        if (listState.isScrollInProgress && composerFocused && !isNearBottom) {
            focusManager.clearFocus()
        }
    }

    fun dispatchSend() {
        val raw = inputText.trim()
        if ((raw.isEmpty() && pendingImages.isEmpty()) || isStreaming) return
        val finalPrompt = quoteTarget?.let { (sender, snippet) ->
            val shortQuote = snippet.replace('\n', ' ').take(90)
            if (raw.isNotEmpty()) "> 引用 $sender: $shortQuote\n\n$raw" else "> 引用 $sender: $shortQuote"
        } ?: raw
        quoteTarget = null
        showPlusDrawer = false
        onSendPrompt(finalPrompt)
    }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(theme.bgCanvas)
            .imePadding()
    ) {
        // 1. Clean Top Header Bar
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(theme.surfaceBar)
                .statusBarsPadding()
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 4.dp, vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Row(
                    modifier = Modifier
                        .sizeIn(minWidth = 48.dp, minHeight = 48.dp)
                        .vesperPressable(pressedScale = 0.94f, onClick = onBack)
                        .padding(horizontal = 6.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(2.dp)
                ) {
                    Icon(
                        imageVector = Icons.Default.ChevronLeft,
                        contentDescription = "返回消息",
                        tint = theme.textPrimary,
                        modifier = Modifier.size(24.dp)
                    )
                    if (otherUnreadCount > 0) {
                        Box(
                            modifier = Modifier
                                .clip(RoundedCornerShape(999.dp))
                                .background(theme.accentSoft)
                                .padding(horizontal = 7.dp, vertical = 2.dp)
                        ) {
                            Text(
                                text = if (otherUnreadCount > 99) "99+" else otherUnreadCount.toString(),
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.sp,
                                fontWeight = FontWeight.Bold,
                                color = theme.accentForeground
                            )
                        }
                    }
                }

                Column(
                    modifier = Modifier
                        .weight(1f)
                        .sizeIn(minHeight = 48.dp)
                        .clickable {
                            if (activeBot != null) onOpenBotProfile(activeBot)
                        }
                        .padding(horizontal = 6.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center
                ) {
                    Text(
                        text = buildString {
                            append(activeBot?.name ?: "AI 好友")
                            if (isGroup && groupMembers.isNotEmpty()) {
                                append(" (${groupMembers.size})")
                            }
                        },
                        style = TextStyle(
                            fontSize = 16.sp,
                            fontWeight = FontWeight.Bold,
                            color = theme.textPrimary,
                            platformStyle = NoFontPaddingStyle
                        ),
                        maxLines = 1,
                        softWrap = false,
                        overflow = TextOverflow.Ellipsis
                    )
                    val subtitleText = when {
                        isStreaming && isGroup -> "成员协作发言中..."
                        isStreaming -> "正在生成回复..."
                        activeBot?.roleTitle?.isNotBlank() == true -> activeBot.roleTitle
                        else -> ""
                    }
                    if (subtitleText.isNotBlank()) {
                        Text(
                            text = subtitleText,
                            style = TextStyle(
                                fontSize = 11.sp,
                                fontWeight = if (isStreaming) FontWeight.Medium else FontWeight.Normal,
                                color = if (isStreaming) theme.accentForeground else theme.textMuted,
                                platformStyle = NoFontPaddingStyle
                            ),
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }

                Row(verticalAlignment = Alignment.CenterVertically) {
                    VesperIconButton(
                        icon = Icons.Default.Search,
                        contentDescription = "查找聊天内容",
                        accent = showSearchBar,
                        filled = showSearchBar,
                        onClick = {
                            showSearchBar = !showSearchBar
                            if (!showSearchBar) searchQuery = ""
                        }
                    )
                    VesperIconButton(
                        icon = Icons.Default.MoreHoriz,
                        contentDescription = "聊天详情",
                        filled = false,
                        onClick = onOpenChatInfo
                    )
                }
            }

            // Expandable In-Chat Search Bar
            AnimatedVisibility(
                visible = showSearchBar,
                enter = fadeIn() + expandVertically(),
                exit = fadeOut() + shrinkVertically()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(start = 12.dp, end = 12.dp, bottom = 8.dp)
                        .clip(RoundedCornerShape(12.dp))
                        .background(theme.surfaceInput)
                        .border(1.dp, theme.accentMuted, RoundedCornerShape(12.dp))
                        .padding(start = 12.dp, end = 4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    Icon(
                        imageVector = Icons.Default.Search,
                        contentDescription = null,
                        tint = theme.accentForeground,
                        modifier = Modifier.size(16.dp)
                    )
                    BasicTextField(
                        value = searchQuery,
                        onValueChange = { searchQuery = it },
                        singleLine = true,
                        textStyle = TextStyle(
                            color = theme.textPrimary,
                            fontSize = 13.5.sp
                        ),
                        cursorBrush = SolidColor(theme.accent),
                        modifier = Modifier
                            .weight(1f)
                            .padding(vertical = 10.dp),
                        decorationBox = { inner ->
                            Box(contentAlignment = Alignment.CenterStart) {
                                if (searchQuery.isEmpty()) {
                                    Text(
                                        text = "查找聊天记录...",
                                        fontSize = 13.sp,
                                        color = theme.textMuted,
                                        maxLines = 1,
                                        softWrap = false
                                    )
                                }
                                inner()
                            }
                        }
                    )
                    if (normalizedSearch.isNotEmpty()) {
                        Text(
                            text = if (searchMatchIndices.isEmpty()) "无匹配" else "${safeMatchCursor + 1}/${searchMatchIndices.size}",
                            fontFamily = FontFamily.Monospace,
                            fontSize = 11.sp,
                            color = if (searchMatchIndices.isEmpty()) theme.danger else theme.accentForeground,
                            maxLines = 1,
                            softWrap = false
                        )
                        if (searchMatchIndices.size > 1) {
                            Box(
                                modifier = Modifier
                                    .size(36.dp)
                                    .clickable {
                                        activeMatchCursor = if (safeMatchCursor > 0) {
                                            safeMatchCursor - 1
                                        } else {
                                            searchMatchIndices.lastIndex
                                        }
                                    },
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = Icons.Default.KeyboardArrowUp,
                                    contentDescription = "上一条",
                                    tint = theme.textSecondary,
                                    modifier = Modifier.size(20.dp)
                                )
                            }
                            Box(
                                modifier = Modifier
                                    .size(36.dp)
                                    .clickable {
                                        activeMatchCursor = (safeMatchCursor + 1) % searchMatchIndices.size
                                    },
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = Icons.Default.KeyboardArrowDown,
                                    contentDescription = "下一条",
                                    tint = theme.textSecondary,
                                    modifier = Modifier.size(20.dp)
                                )
                            }
                        }
                    }
                    Box(
                        modifier = Modifier
                            .size(38.dp)
                            .clickable {
                                showSearchBar = false
                                searchQuery = ""
                            },
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = Icons.Default.Close,
                            contentDescription = "关闭搜索",
                            tint = theme.textMuted,
                            modifier = Modifier.size(17.dp)
                        )
                    }
                }
            }

            VesperDivider()
        }

        // 1b. Multi-Topic Thread Bar (Shown when threads exist)
        if (activeBot != null && botThreads.isNotEmpty()) {
            LazyRow(
                contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
                verticalAlignment = Alignment.CenterVertically,
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.surfaceBar)
            ) {
                item(key = "__thread_main__") {
                    ThreadPill(
                        label = "主对话",
                        active = activeThreadId.isBlank(),
                        canDelete = false,
                        onSelect = { if (!isStreaming && activeThreadId.isNotBlank()) onSelectThread("") },
                        onDelete = {}
                    )
                }

                items(botThreads, key = { "th_${it.id}" }) { th ->
                    val isActive = activeThreadId == th.id
                    ThreadPill(
                        label = th.title.ifBlank { "新话题" },
                        active = isActive,
                        canDelete = isActive && !isStreaming,
                        onSelect = { if (!isStreaming && !isActive) onSelectThread(th.id) },
                        onDelete = { onDeleteThread(th.id) }
                    )
                }

                item(key = "__thread_add__") {
                    val shape = RoundedCornerShape(999.dp)
                    Row(
                        modifier = Modifier
                            .sizeIn(minHeight = 32.dp)
                            .clip(shape)
                            .background(theme.surfaceControl, shape)
                            .border(1.dp, theme.border, shape)
                            .vesperPressable(enabled = !isStreaming, pressedScale = 0.94f, hapticOnTap = true) {
                                onCreateThread("")
                            }
                            .padding(horizontal = 10.dp, vertical = 5.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(3.dp)
                    ) {
                        Icon(
                            imageVector = Icons.Default.Add,
                            contentDescription = "新建话题",
                            tint = theme.textSecondary,
                            modifier = Modifier.size(13.dp)
                        )
                        Text(
                            text = "新话题",
                            fontSize = 11.5.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textSecondary,
                            maxLines = 1,
                            softWrap = false
                        )
                    }
                }
            }
            VesperDivider()
        }

        // 2. Group Announcement Banner
        if (isGroup && !dismissAnnouncement && !activeBot?.announcement.isNullOrBlank()) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.surfaceCard)
                    .padding(start = 14.dp, end = 4.dp, top = 6.dp, bottom = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                VesperTag(text = "群公告")
                Text(
                    text = activeBot?.announcement.orEmpty(),
                    fontSize = 12.5.sp,
                    color = theme.textSecondary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f)
                )
                Box(
                    modifier = Modifier
                        .size(38.dp)
                        .clickable { dismissAnnouncement = true },
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = Icons.Default.Close,
                        contentDescription = "收起公告",
                        tint = theme.textMuted,
                        modifier = Modifier.size(16.dp)
                    )
                }
            }
            VesperDivider()
        }

        // 2b. Rolling Context Compaction Summary Banner
        if (compactionSummary.isNotBlank()) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.surfaceCard)
                    .clickable { expandCompactionSummary = !expandCompactionSummary }
                    .padding(horizontal = 14.dp, vertical = 7.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    VesperTag(text = "已压缩上下文", toneColor = theme.info)
                    Text(
                        text = compactionSummary.replace('\n', ' '),
                        fontSize = 12.sp,
                        color = theme.textSecondary,
                        maxLines = if (expandCompactionSummary) 8 else 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f)
                    )
                    Icon(
                        imageVector = if (expandCompactionSummary) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown,
                        contentDescription = if (expandCompactionSummary) "收起摘要" else "展开摘要",
                        tint = theme.textMuted,
                        modifier = Modifier.size(16.dp)
                    )
                }
            }
            VesperDivider()
        }

        // 3. Message Stream + Scroll-to-Bottom Pill
        Box(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
        ) {
            LazyColumn(
                state = listState,
                modifier = Modifier
                    .fillMaxSize()
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = null
                    ) {
                        if (showPlusDrawer) showPlusDrawer = false
                        if (composerFocused) focusManager.clearFocus()
                    },
                verticalArrangement = Arrangement.spacedBy(14.dp),
                contentPadding = PaddingValues(horizontal = 14.dp, vertical = 16.dp)
            ) {
                if (messages.isEmpty()) {
                    item(key = "__chat_welcome__", contentType = "welcome") {
                        Column(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(horizontal = 24.dp, vertical = 52.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(10.dp)
                        ) {
                            BotAvatar(
                                bot = activeBot,
                                allBots = allBots,
                                containerSize = 68.dp,
                                interactive = true,
                                seedKey = activeBot?.id ?: "chat_empty"
                            )
                            Text(
                                text = activeBot?.name ?: "AI 好友",
                                fontSize = 18.sp,
                                fontWeight = FontWeight.Bold,
                                color = theme.textPrimary,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                            val desc = activeBot?.traits?.ifBlank { activeBot.roleTitle }.orEmpty()
                            if (desc.isNotBlank()) {
                                Text(
                                    text = desc,
                                    fontSize = 13.sp,
                                    lineHeight = 19.sp,
                                    color = theme.textSecondary,
                                    textAlign = TextAlign.Center,
                                    maxLines = 2,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                            Text(
                                text = "输入 / 可快速调用当前助手的专属指令与技能",
                                fontSize = 12.sp,
                                color = theme.textMuted,
                                textAlign = TextAlign.Center
                            )
                        }
                    }
                }

                items(
                    items = messages,
                    key = { it.id },
                    contentType = { it.role }
                ) { msg ->
                    when (msg.role) {
                        "system" -> {
                            Box(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(vertical = 2.dp),
                                contentAlignment = Alignment.Center
                            ) {
                                Box(
                                    modifier = Modifier
                                        .widthIn(max = 310.dp)
                                        .clip(RoundedCornerShape(10.dp))
                                        .background(theme.surfaceCard)
                                        .border(0.75.dp, theme.border, RoundedCornerShape(10.dp))
                                        .padding(horizontal = 12.dp, vertical = 5.dp)
                                ) {
                                    Text(
                                        text = msg.content,
                                        fontSize = 11.5.sp,
                                        lineHeight = 16.sp,
                                        color = theme.textMuted,
                                        textAlign = TextAlign.Center
                                    )
                                }
                            }
                        }

                        "user" -> {
                            var showUserMenu by remember { mutableStateOf(false) }
                            val isActiveHit = msg.id == activeMatchedTurnId
                            val isSearchHit = msg.id in matchedTurnIds
                            val userShape = RoundedCornerShape(
                                topStart = 18.dp,
                                topEnd = 5.dp,
                                bottomStart = 18.dp,
                                bottomEnd = 18.dp
                            )
                            val userHitBorder = when {
                                isActiveHit -> Modifier.border(2.dp, theme.accentForeground, userShape)
                                isSearchHit -> Modifier.border(1.dp, theme.accentMuted, userShape)
                                else -> Modifier
                            }

                            Box(
                                modifier = Modifier.fillMaxWidth(),
                                contentAlignment = Alignment.CenterEnd
                            ) {
                                Column(
                                    modifier = Modifier
                                        .widthIn(max = 304.dp)
                                        .clip(userShape)
                                        .background(theme.userBubble, userShape)
                                        .then(userHitBorder)
                                        .combinedClickable(
                                            onClick = {},
                                            onLongClick = {
                                                haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                                                showUserMenu = true
                                            }
                                        )
                                        .padding(horizontal = 14.dp, vertical = 10.dp),
                                    verticalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    if (msg.attachments.isNotEmpty()) {
                                        LazyRow(
                                            horizontalArrangement = Arrangement.spacedBy(6.dp)
                                        ) {
                                            items(msg.attachments, key = { it }) { imgUrl ->
                                                AuthenticatedChatImage(
                                                    url = imgUrl,
                                                    onLoadImage = onLoadChatImage,
                                                    modifier = Modifier
                                                        .sizeIn(minWidth = 96.dp, maxWidth = 180.dp, minHeight = 96.dp, maxHeight = 180.dp)
                                                        .clip(RoundedCornerShape(10.dp))
                                                )
                                            }
                                        }
                                    }
                                    if (msg.content.isNotBlank()) {
                                        Text(
                                            text = msg.content,
                                            style = MaterialTheme.typography.bodyMedium.copy(
                                                fontSize = 14.5.sp,
                                                lineHeight = 21.5.sp
                                            ),
                                            color = theme.userBubbleText
                                        )
                                    }
                                }

                                VesperPopoverMenu(
                                    expanded = showUserMenu,
                                    onDismissRequest = { showUserMenu = false },
                                    alignment = Alignment.TopEnd,
                                    offsetY = 38.dp,
                                    items = listOf(
                                        VesperMenuAction(
                                            label = "复制",
                                            icon = Icons.Default.ContentCopy,
                                            iconTint = theme.accentForeground,
                                            onClick = { onCopyText(msg.content) }
                                        ),
                                        VesperMenuAction(
                                            label = "引用",
                                            icon = Icons.Default.FormatQuote,
                                            iconTint = theme.info,
                                            onClick = { quoteTarget = "我" to msg.content }
                                        ),
                                        VesperMenuAction(
                                            label = "转发",
                                            icon = Icons.Default.Share,
                                            iconTint = theme.success,
                                            onClick = { forwardPayload = "我" to msg.content }
                                        ),
                                        VesperMenuAction(
                                            label = "由此分叉",
                                            icon = Icons.Default.AutoAwesome,
                                            iconTint = theme.accentForeground,
                                            onClick = { onForkFromTurn(msg) }
                                        ),
                                        VesperMenuAction(
                                            label = "删除",
                                            icon = Icons.Outlined.DeleteOutline,
                                            danger = true,
                                            onClick = { onDeleteTurn(msg) }
                                        )
                                    )
                                )
                            }
                        }

                        else -> {
                            AssistantMessageRow(
                                msg = msg,
                                activeBot = activeBot,
                                allBots = allBots,
                                modelOptions = modelOptions,
                                isGroup = isGroup,
                                isLiveTurn = isStreaming && msg.id == lastAssistantId,
                                isLatestReply = msg.id == lastAssistantId && !(isStreaming && msg.id == lastAssistantId),
                                isActiveHit = msg.id == activeMatchedTurnId,
                                isSearchHit = msg.id in matchedTurnIds,
                                liveTodos = liveTodos,
                                onOpenBotProfile = onOpenBotProfile,
                                onQuickToggleTodo = onQuickToggleTodo,
                                onRetry = { if (!isStreaming) retryingTurn = msg },
                                onCopy = { onCopyText(msg.content) },
                                onQuote = { sender -> quoteTarget = sender to msg.content },
                                onConvertToTodo = { onConvertTurnToTodo(msg) },
                                onSaveAsNote = { onSaveTurnAsNote(msg) },
                                onForward = { sender -> forwardPayload = sender to msg.content },
                                onFork = { onForkFromTurn(msg) },
                                onDelete = { onDeleteTurn(msg) }
                            )
                        }
                    }
                }
            }

            // Floating Scroll-to-Bottom Pill
            androidx.compose.animation.AnimatedVisibility(
                visible = !isNearBottom && messages.size > 3,
                enter = fadeIn(tween(140)) + scaleIn(tween(140), initialScale = 0.88f),
                exit = fadeOut(tween(120)) + scaleOut(tween(120), targetScale = 0.88f),
                modifier = Modifier
                    .align(Alignment.BottomEnd)
                    .padding(end = 14.dp, bottom = 12.dp)
            ) {
                val pillShape = RoundedCornerShape(999.dp)
                Row(
                    modifier = Modifier
                        .sizeIn(minHeight = 40.dp)
                        .shadow(
                            6.dp,
                            pillShape,
                            ambientColor = Color.Black.copy(alpha = if (theme.isLight) 0.14f else 0.45f)
                        )
                        .vesperPressable(pressedScale = 0.93f, hapticOnTap = true) {
                            scope.launch {
                                if (messages.isNotEmpty()) {
                                    listState.animateScrollToItem(messages.lastIndex)
                                }
                            }
                        }
                        .clip(pillShape)
                        .background(theme.surfacePopover, pillShape)
                        .border(1.dp, theme.borderStrong, pillShape)
                        .padding(horizontal = 13.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(5.dp)
                ) {
                    Icon(
                        imageVector = Icons.Default.KeyboardArrowDown,
                        contentDescription = "回到底部",
                        tint = theme.accentForeground,
                        modifier = Modifier.size(16.dp)
                    )
                    Text(
                        text = if (isStreaming) "新回复" else "回到底部",
                        style = TextStyle(
                            fontSize = 12.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textPrimary,
                            platformStyle = NoFontPaddingStyle
                        ),
                        maxLines = 1,
                        softWrap = false
                    )
                }
            }
        }

        // 4. Group Chat "@点名成员" Quick Strip
        if (isGroup && groupMembers.isNotEmpty()) {
            LazyRow(
                contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.surfaceBar)
            ) {
                items(groupMembers, key = { "at_${it.id}" }) { member ->
                    val chipShape = RoundedCornerShape(999.dp)
                    Row(
                        modifier = Modifier
                            .sizeIn(minHeight = 32.dp)
                            .vesperPressable(pressedScale = 0.94f, hapticOnTap = true) {
                                val tag = "@${member.name} "
                                if (!inputText.contains(tag)) {
                                    val sep = if (inputText.isNotEmpty() && !inputText.endsWith(" ")) " " else ""
                                    onInputChange("$inputText$sep$tag")
                                }
                            }
                            .clip(chipShape)
                            .background(theme.surfaceControl, chipShape)
                            .border(1.dp, theme.border, chipShape)
                            .padding(start = 5.dp, end = 10.dp, top = 4.dp, bottom = 4.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(5.dp)
                    ) {
                        GrokBotAvatar(
                            avatarStyle = member.avatarStyle,
                            state = member.state,
                            size = 18.dp,
                            interactive = false,
                            seedKey = member.id
                        )
                        Text(
                            text = "@${member.name}",
                            fontSize = 11.5.sp,
                            color = theme.textSecondary,
                            maxLines = 1,
                            softWrap = false
                        )
                    }
                }
            }
        }

        // 4b. Dynamic "/" Slash Command Popover
        val allSlashCommands = remember(activeBot?.id, activeBot?.allowedTools, activeBot?.skillIds, availableSkills) {
            buildBotSlashCommands(activeBot, availableSkills)
        }
        val trimmedInput = inputText.trim()
        val matchingSlashCommands = remember(allSlashCommands, trimmedInput) {
            if (!trimmedInput.startsWith("/") || trimmedInput.contains('\n') || trimmedInput.contains(' ')) {
                emptyList()
            } else {
                val q = trimmedInput.lowercase()
                val qBare = q.removePrefix("/")
                allSlashCommands.filter {
                    it.cmd.lowercase().contains(q) || (qBare.isNotEmpty() && it.title.lowercase().contains(qBare))
                }
            }
        }

        AnimatedVisibility(
            visible = matchingSlashCommands.isNotEmpty() && !isStreaming,
            enter = fadeIn(tween(120)) + expandVertically(tween(140)),
            exit = fadeOut(tween(100)) + shrinkVertically(tween(120))
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.surfaceSheet)
            ) {
                VesperDivider()
                LazyColumn(
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(maxHeight = 228.dp),
                    contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                    verticalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    items(matchingSlashCommands, key = { it.cmd }) { item ->
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .sizeIn(minHeight = 48.dp)
                                .clip(RoundedCornerShape(10.dp))
                                .background(theme.surfaceInput)
                                .clickable {
                                    when (item.action) {
                                        "new_thread" -> {
                                            onInputChange("")
                                            onCreateThread("")
                                        }
                                        "summarize_thread" -> {
                                            onInputChange("")
                                            onSummarizeThread()
                                        }
                                        "compact_thread" -> {
                                            onInputChange("")
                                            onCompactThread()
                                        }
                                        "clear_timeline" -> {
                                            onInputChange("")
                                            onClearHistory()
                                        }
                                        else -> {
                                            if (item.prompt.endsWith("：")) {
                                                onInputChange(item.prompt)
                                            } else if (item.prompt.isNotBlank()) {
                                                onInputChange("")
                                                onSendPrompt(item.prompt)
                                            }
                                        }
                                    }
                                }
                                .padding(horizontal = 12.dp, vertical = 8.dp),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Row(
                                modifier = Modifier
                                    .weight(1f)
                                    .padding(end = 8.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp)
                            ) {
                                Text(
                                    text = item.cmd,
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 12.sp,
                                    fontWeight = FontWeight.Bold,
                                    color = theme.accentForeground,
                                    maxLines = 1,
                                    softWrap = false
                                )
                                Text(
                                    text = item.title,
                                    fontSize = 13.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = theme.textPrimary,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                            Text(
                                text = item.desc,
                                fontSize = 11.sp,
                                color = theme.textMuted,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                                modifier = Modifier.weight(1f, fill = false)
                            )
                        }
                    }
                }
            }
        }

        // 5. Quoted Reply & Notification Preview Strips
        quoteTarget?.let { (sender, snippet) ->
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.surfaceCard)
                    .padding(start = 14.dp, end = 4.dp, top = 4.dp, bottom = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Text(
                    text = "引用 $sender: ${snippet.replace('\n', ' ')}",
                    fontSize = 12.sp,
                    color = theme.textSecondary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier
                        .weight(1f)
                        .padding(end = 6.dp)
                )
                Box(
                    modifier = Modifier
                        .size(38.dp)
                        .clickable { quoteTarget = null },
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = Icons.Default.Close,
                        contentDescription = "取消引用",
                        tint = theme.textMuted,
                        modifier = Modifier.size(16.dp)
                    )
                }
            }
        }

        if (quotedNotification != null) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.surfaceBar)
                    .padding(horizontal = 12.dp, vertical = 8.dp),
                verticalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Column(
                        modifier = Modifier
                            .weight(1f)
                            .padding(end = 8.dp),
                        verticalArrangement = Arrangement.spacedBy(2.dp)
                    ) {
                        Text(
                            text = "引用服务通知 · ${quotedNotification.title}",
                            fontSize = 12.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.accentForeground,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        val qPreview = quotedNotification.previewText(80)
                        if (qPreview.isNotEmpty()) {
                            Text(
                                text = qPreview,
                                fontSize = 11.5.sp,
                                color = theme.textMuted,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                    }
                    Box(
                        modifier = Modifier
                            .size(38.dp)
                            .clickable { onDismissQuotedNotification() },
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = Icons.Default.Close,
                            contentDescription = "取消引用通知",
                            tint = theme.textMuted,
                            modifier = Modifier.size(17.dp)
                        )
                    }
                }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    val chipShape = RoundedCornerShape(999.dp)
                    Box(
                        modifier = Modifier
                            .sizeIn(minHeight = 34.dp)
                            .clip(chipShape)
                            .background(theme.accentSoft, chipShape)
                            .border(1.dp, theme.accentMuted, chipShape)
                            .vesperPressable(enabled = !isStreaming, pressedScale = 0.95f, hapticOnTap = true) {
                                onSendPrompt("请针对这条服务通知深入展开讲讲，补充关键背景、细节与延伸思考。")
                            }
                            .padding(horizontal = 12.dp, vertical = 6.dp),
                        contentAlignment = Alignment.Center
                    ) {
                        Text(
                            text = "深入展开讲讲",
                            style = TextStyle(
                                fontSize = 11.5.sp,
                                fontWeight = FontWeight.Medium,
                                color = theme.accentForeground,
                                platformStyle = NoFontPaddingStyle
                            )
                        )
                    }
                    Box(
                        modifier = Modifier
                            .sizeIn(minHeight = 34.dp)
                            .clip(chipShape)
                            .background(theme.surfaceControl, chipShape)
                            .border(1.dp, theme.border, chipShape)
                            .vesperPressable(enabled = !isStreaming, pressedScale = 0.95f, hapticOnTap = true) {
                                onSendPrompt("请结合我的个人情况、近期记忆与待办，分析这条服务通知对我有什么直接启发或行动建议。")
                            }
                            .padding(horizontal = 12.dp, vertical = 6.dp),
                        contentAlignment = Alignment.Center
                    ) {
                        Text(
                            text = "结合我的情况分析",
                            style = TextStyle(
                                fontSize = 11.5.sp,
                                color = theme.textSecondary,
                                platformStyle = NoFontPaddingStyle
                            )
                        )
                    }
                }
            }
        }

        // 6. Bottom Composer with Inline Model Switcher Capsule & Thinking Toggle
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(theme.surfaceBar)
                .navigationBarsPadding()
        ) {
            VesperDivider()

            // Quick Model Switcher Capsule + Deep Thinking Toggle Strip
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(start = 12.dp, end = 12.dp, top = 7.dp, bottom = 2.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                val capsuleShape = RoundedCornerShape(999.dp)
                Row(
                    modifier = Modifier
                        .weight(1f, fill = false)
                        .sizeIn(minHeight = 32.dp)
                        .clip(capsuleShape)
                        .background(theme.surfaceControl, capsuleShape)
                        .border(1.dp, theme.border, capsuleShape)
                        .vesperPressable(enabled = !isStreaming, pressedScale = 0.96f, hapticOnTap = true) {
                            showModelPickerSheet = true
                        }
                        .padding(start = 9.dp, end = 7.dp, top = 5.dp, bottom = 5.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(5.dp)
                ) {
                    Icon(
                        imageVector = Icons.Default.AutoAwesome,
                        contentDescription = null,
                        tint = theme.accentForeground,
                        modifier = Modifier.size(13.dp)
                    )
                    Text(
                        text = modelDisplayName,
                        style = TextStyle(
                            fontSize = 11.5.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textPrimary,
                            platformStyle = NoFontPaddingStyle
                        ),
                        maxLines = 1,
                        softWrap = false,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f, fill = false)
                    )
                    Icon(
                        imageVector = Icons.Default.KeyboardArrowDown,
                        contentDescription = "切换模型",
                        tint = theme.textMuted,
                        modifier = Modifier.size(14.dp)
                    )
                }

                Row(
                    modifier = Modifier
                        .sizeIn(minHeight = 32.dp)
                        .clip(capsuleShape)
                        .background(if (enableThinking) theme.accentSoft else theme.surfaceControl, capsuleShape)
                        .border(1.dp, if (enableThinking) theme.accentMuted else theme.border, capsuleShape)
                        .vesperPressable(pressedScale = 0.95f, hapticOnTap = true) {
                            onThinkingChange(!enableThinking)
                        }
                        .padding(horizontal = 10.dp, vertical = 5.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    Icon(
                        imageVector = Icons.Default.Psychology,
                        contentDescription = null,
                        tint = if (enableThinking) theme.accentForeground else theme.textMuted,
                        modifier = Modifier.size(13.dp)
                    )
                    Text(
                        text = if (enableThinking) "深度思考" else "快答",
                        style = TextStyle(
                            fontSize = 11.5.sp,
                            fontWeight = if (enableThinking) FontWeight.SemiBold else FontWeight.Medium,
                            color = if (enableThinking) theme.accentForeground else theme.textSecondary,
                            platformStyle = NoFontPaddingStyle
                        ),
                        maxLines = 1,
                        softWrap = false
                    )
                }
            }

            // Pending Vision Image Strip
            if (pendingImages.isNotEmpty() || uploadingImage) {
                LazyRow(
                    contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    items(pendingImages, key = { it }) { imgUrl ->
                        Box(modifier = Modifier.size(64.dp)) {
                            AuthenticatedChatImage(
                                url = imgUrl,
                                onLoadImage = onLoadChatImage,
                                modifier = Modifier
                                    .fillMaxSize()
                                    .clip(RoundedCornerShape(10.dp))
                                    .border(1.dp, theme.border, RoundedCornerShape(10.dp))
                            )
                            Box(
                                modifier = Modifier
                                    .align(Alignment.TopEnd)
                                    .padding(2.dp)
                                    .size(20.dp)
                                    .clip(CircleShape)
                                    .background(Color.Black.copy(alpha = 0.68f))
                                    .clickable { onRemovePendingImage(imgUrl) },
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = Icons.Default.Close,
                                    contentDescription = "移除图片",
                                    tint = Color.White,
                                    modifier = Modifier.size(12.dp)
                                )
                            }
                        }
                    }
                    if (uploadingImage) {
                        item(key = "__img_uploading__") {
                            Box(
                                modifier = Modifier
                                    .size(64.dp)
                                    .clip(RoundedCornerShape(10.dp))
                                    .background(theme.surfaceControl)
                                    .border(1.dp, theme.border, RoundedCornerShape(10.dp)),
                                contentAlignment = Alignment.Center
                            ) {
                                Text(
                                    text = "上传中",
                                    fontSize = 11.sp,
                                    color = theme.textMuted
                                )
                            }
                        }
                    }
                }
            }

            // Input Row
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(start = 4.dp, end = 10.dp, top = 4.dp, bottom = 8.dp),
                verticalAlignment = Alignment.Bottom,
                horizontalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                VesperIconButton(
                    icon = if (showPlusDrawer) Icons.Default.Close else Icons.Default.Add,
                    contentDescription = "更多功能",
                    accent = showPlusDrawer,
                    visualSize = 36.dp,
                    iconSize = 20.dp,
                    onClick = { showPlusDrawer = !showPlusDrawer }
                )

                VesperIconButton(
                    icon = Icons.Default.Image,
                    contentDescription = "添加图片",
                    accent = pendingImages.isNotEmpty(),
                    visualSize = 36.dp,
                    iconSize = 19.dp,
                    onClick = {
                        if (!isStreaming && !uploadingImage) {
                            imagePickerLauncher.launch("image/*")
                        }
                    }
                )

                val inputShape = RoundedCornerShape(22.dp)
                val inputBorder by animateColorAsState(
                    targetValue = if (composerFocused) theme.accent else theme.border,
                    animationSpec = tween(140),
                    label = "composerBorder"
                )
                Box(
                    modifier = Modifier
                        .weight(1f)
                        .sizeIn(minHeight = 46.dp)
                        .clip(inputShape)
                        .background(theme.surfaceInput, inputShape)
                        .border(if (composerFocused) 1.3.dp else 1.dp, inputBorder, inputShape),
                    contentAlignment = Alignment.CenterStart
                ) {
                    BasicTextField(
                        value = inputText,
                        onValueChange = onInputChange,
                        textStyle = TextStyle(
                            color = theme.textPrimary,
                            fontSize = 14.5.sp,
                            lineHeight = 20.sp
                        ),
                        cursorBrush = SolidColor(theme.accent),
                        maxLines = 5,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 15.dp, vertical = 12.dp)
                            .onFocusChanged { composerFocused = it.isFocused },
                        decorationBox = { inner ->
                            Box(contentAlignment = Alignment.CenterStart) {
                                if (inputText.isEmpty()) {
                                    Text(
                                        text = if (pendingImages.isNotEmpty()) "补充图片提问（也可直接发送）..." else "发消息或输入 / 调出快捷指令...",
                                        fontSize = 14.sp,
                                        color = theme.textMuted,
                                        maxLines = 1,
                                        softWrap = false,
                                        overflow = TextOverflow.Ellipsis
                                    )
                                }
                                inner()
                            }
                        }
                    )
                }

                if (isStreaming) {
                    Box(
                        modifier = Modifier
                            .size(46.dp)
                            .vesperPressable(pressedScale = 0.92f, hapticOnTap = true, onClick = onStop),
                        contentAlignment = Alignment.Center
                    ) {
                        Box(
                            modifier = Modifier
                                .size(40.dp)
                                .clip(CircleShape)
                                .background(theme.danger.copy(alpha = 0.16f))
                                .border(1.dp, theme.danger.copy(alpha = 0.45f), CircleShape),
                            contentAlignment = Alignment.Center
                        ) {
                            Icon(
                                imageVector = Icons.Default.Stop,
                                contentDescription = "停止生成",
                                tint = theme.danger,
                                modifier = Modifier.size(19.dp)
                            )
                        }
                    }
                } else {
                    val canSend = (inputText.isNotBlank() || pendingImages.isNotEmpty()) && !uploadingImage
                    Box(
                        modifier = Modifier
                            .size(46.dp)
                            .vesperPressable(
                                enabled = canSend,
                                pressedScale = 0.92f,
                                hapticOnTap = true,
                                onClick = { dispatchSend() }
                            ),
                        contentAlignment = Alignment.Center
                    ) {
                        Box(
                            modifier = Modifier
                                .size(40.dp)
                                .clip(CircleShape)
                                .background(if (canSend) theme.accent else theme.surfaceControl)
                                .then(
                                    if (canSend) Modifier else Modifier.border(1.dp, theme.border, CircleShape)
                                ),
                            contentAlignment = Alignment.Center
                        ) {
                            Icon(
                                imageVector = Icons.Default.ArrowUpward,
                                contentDescription = "发送消息",
                                tint = if (canSend) theme.onAccent else theme.textMuted,
                                modifier = Modifier.size(20.dp)
                            )
                        }
                    }
                }
            }

            // Expandable 2x3 Action Tray
            AnimatedVisibility(
                visible = showPlusDrawer,
                enter = fadeIn() + expandVertically(),
                exit = fadeOut() + shrinkVertically()
            ) {
                val tools = listOf(
                    PlusActionTile(
                        label = "发送图片",
                        icon = Icons.Default.Image,
                        accent = theme.accentForeground
                    ) {
                        showPlusDrawer = false
                        if (!isStreaming && !uploadingImage) {
                            imagePickerLauncher.launch("image/*")
                        }
                    },
                    PlusActionTile(
                        label = if (compactingThread) "压缩中..." else "压缩上下文",
                        icon = Icons.Default.History,
                        accent = theme.info
                    ) {
                        showPlusDrawer = false
                        if (!isStreaming && !compactingThread) onCompactThread()
                    },
                    PlusActionTile(
                        label = "新开话题",
                        icon = Icons.Default.Add,
                        accent = theme.success
                    ) {
                        showPlusDrawer = false
                        if (!isStreaming) onCreateThread("")
                    },
                    PlusActionTile(
                        label = "提炼记忆",
                        icon = Icons.Default.Description,
                        accent = theme.accentForeground
                    ) {
                        showPlusDrawer = false
                        if (!isStreaming) onSummarizeThread()
                    },
                    PlusActionTile(
                        label = if (isGroup) "群成员" else "拉人建群",
                        icon = Icons.Default.GroupAdd,
                        accent = theme.warning
                    ) {
                        showPlusDrawer = false
                        onInviteToGroup()
                    },
                    PlusActionTile(
                        label = "清空记录",
                        icon = Icons.Default.DeleteSweep,
                        accent = theme.danger
                    ) {
                        showPlusDrawer = false
                        onClearHistory()
                    }
                )

                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(start = 14.dp, end = 14.dp, top = 4.dp, bottom = 14.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    VesperDivider()
                    tools.chunked(3).forEach { rowTiles ->
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.spacedBy(10.dp)
                        ) {
                            rowTiles.forEach { tile ->
                                PlusTrayItem(
                                    tile = tile,
                                    modifier = Modifier.weight(1f)
                                )
                            }
                        }
                    }
                }
            }
        }
    }

    if (showModelPickerSheet) {
        ModelPickerSheet(
            activeModel = activeModel,
            modelOptions = modelOptions,
            onSelect = { chosen ->
                showModelPickerSheet = false
                onModelChange(chosen)
            },
            onDismiss = { showModelPickerSheet = false }
        )
    }

    retryingTurn?.let { targetTurn ->
        ModelPickerSheet(
            activeModel = targetTurn.model.ifBlank { activeModel },
            modelOptions = modelOptions,
            onSelect = { chosen ->
                retryingTurn = null
                onRetryTurnWithModel(targetTurn, chosen)
            },
            onDismiss = { retryingTurn = null }
        )
    }

    forwardPayload?.let { (senderName, bodyContent) ->
        ForwardMessageSheet(
            senderName = senderName,
            bodyContent = bodyContent,
            activeBotId = activeBot?.id,
            allBots = allBots,
            onForward = { targetBot, note, openTarget ->
                onForwardMessage(targetBot, senderName, bodyContent, note, openTarget)
                forwardPayload = null
            },
            onDismiss = { forwardPayload = null }
        )
    }
}

@Composable
private fun ThreadPill(
    label: String,
    active: Boolean,
    canDelete: Boolean,
    onSelect: () -> Unit,
    onDelete: () -> Unit
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(999.dp)
    Row(
        modifier = Modifier
            .sizeIn(minHeight = 32.dp)
            .clip(shape)
            .background(if (active) theme.accentSoft else theme.surfaceControl, shape)
            .border(1.dp, if (active) theme.accentMuted else theme.border, shape)
            .vesperPressable(pressedScale = 0.95f, hapticOnTap = true, onClick = onSelect)
            .padding(start = 11.dp, end = if (canDelete) 6.dp else 11.dp, top = 5.dp, bottom = 5.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(5.dp)
    ) {
        Text(
            text = label,
            fontSize = 11.5.sp,
            fontWeight = if (active) FontWeight.SemiBold else FontWeight.Medium,
            color = if (active) theme.accentForeground else theme.textSecondary,
            maxLines = 1,
            softWrap = false,
            overflow = TextOverflow.Ellipsis
        )
        if (canDelete) {
            Box(
                modifier = Modifier
                    .size(20.dp)
                    .clip(CircleShape)
                    .clickable(onClick = onDelete),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = Icons.Default.Close,
                    contentDescription = "删除话题",
                    tint = theme.textMuted,
                    modifier = Modifier.size(13.dp)
                )
            }
        }
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun AssistantMessageRow(
    msg: ChatTurnUi,
    activeBot: BotItem?,
    allBots: List<BotItem>,
    modelOptions: List<ModelOptionItem>,
    isGroup: Boolean,
    isLiveTurn: Boolean,
    isLatestReply: Boolean,
    isActiveHit: Boolean,
    isSearchHit: Boolean,
    liveTodos: List<TodoItemUi>,
    onOpenBotProfile: (BotItem) -> Unit,
    onQuickToggleTodo: (String, String) -> Unit,
    onRetry: () -> Unit,
    onCopy: () -> Unit,
    onQuote: (String) -> Unit,
    onConvertToTodo: () -> Unit,
    onSaveAsNote: () -> Unit,
    onForward: (String) -> Unit,
    onFork: () -> Unit,
    onDelete: () -> Unit
) {
    val theme = BotHubThemeTokens
    val haptics = LocalHapticFeedback.current
    var expandReasoning by remember { mutableStateOf(false) }
    var expandTools by remember { mutableStateOf(false) }
    var showBubbleMenu by remember { mutableStateOf(false) }

    val avatarLiveState = when {
        isLiveTurn && msg.content.isEmpty() -> "thinking"
        isLiveTurn -> "working"
        else -> "idle"
    }
    val speakerBot = remember(msg.senderBotId, allBots, activeBot) {
        allBots.find { it.id == msg.senderBotId } ?: activeBot
    }
    val senderName = msg.persona.ifEmpty { speakerBot?.name ?: "AI" }
    val aiShape = RoundedCornerShape(
        topStart = 5.dp,
        topEnd = 16.dp,
        bottomStart = 16.dp,
        bottomEnd = 16.dp
    )

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .then(if (!isLiveTurn) Modifier.animateContentSize() else Modifier),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.Top
    ) {
        BotAvatar(
            bot = speakerBot,
            allBots = allBots,
            avatarStyleOverride = msg.avatarStyle,
            state = avatarLiveState,
            containerSize = 36.dp,
            seedKey = msg.id,
            onClick = { if (speakerBot != null) onOpenBotProfile(speakerBot) },
            modifier = Modifier.padding(top = 2.dp)
        )

        Box(modifier = Modifier.weight(1f)) {
            Column(verticalArrangement = Arrangement.spacedBy(5.dp)) {
                if (isGroup || msg.msgKind == "routine") {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                        modifier = Modifier.padding(start = 2.dp)
                    ) {
                        Text(
                            text = senderName,
                            fontSize = 12.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textSecondary,
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f, fill = false)
                        )
                        if (msg.msgKind == "routine") {
                            VesperTag(text = "定时提醒", toneColor = theme.warning)
                        }
                    }
                }

                val aiBorderWidth = if (isActiveHit) 1.6.dp else 1.dp
                val aiBorderColor = when {
                    isActiveHit -> theme.accent
                    isSearchHit -> theme.accentMuted
                    else -> theme.border
                }

                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clip(aiShape)
                        .background(theme.aiBubble, aiShape)
                        .border(aiBorderWidth, aiBorderColor, aiShape)
                        .combinedClickable(
                            onClick = {},
                            onLongClick = {
                                if (msg.content.isNotEmpty()) {
                                    haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                                    showBubbleMenu = true
                                }
                            }
                        )
                        .padding(horizontal = 14.dp, vertical = 12.dp),
                    verticalArrangement = Arrangement.spacedBy(9.dp)
                ) {
                    // Collapsible Reasoning Accordion
                    if (msg.reasoning.isNotEmpty()) {
                        val boxShape = RoundedCornerShape(10.dp)
                        Column(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clip(boxShape)
                                .background(theme.codeBlockBg, boxShape)
                                .border(0.75.dp, theme.border, boxShape)
                        ) {
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .sizeIn(minHeight = 38.dp)
                                    .clickable { expandReasoning = !expandReasoning }
                                    .padding(horizontal = 10.dp, vertical = 7.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.SpaceBetween
                            ) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                                ) {
                                    Icon(
                                        imageVector = Icons.Default.Psychology,
                                        contentDescription = null,
                                        tint = theme.accentForeground,
                                        modifier = Modifier.size(14.dp)
                                    )
                                    Text(
                                        text = if (isLiveTurn && msg.content.isEmpty()) {
                                            "正在深度思考 · ${msg.reasoning.length}字"
                                        } else {
                                            "思考过程 · ${msg.reasoning.length}字"
                                        },
                                        fontSize = 11.5.sp,
                                        fontWeight = FontWeight.Medium,
                                        color = theme.textSecondary,
                                        maxLines = 1,
                                        softWrap = false
                                    )
                                }
                                Icon(
                                    imageVector = if (expandReasoning) Icons.Default.KeyboardArrowDown else Icons.AutoMirrored.Filled.KeyboardArrowRight,
                                    contentDescription = if (expandReasoning) "收起思考" else "展开思考",
                                    tint = theme.textMuted,
                                    modifier = Modifier.size(16.dp)
                                )
                            }
                            AnimatedVisibility(
                                visible = expandReasoning,
                                enter = fadeIn() + expandVertically(),
                                exit = fadeOut() + shrinkVertically()
                            ) {
                                Column {
                                    VesperDivider()
                                    Text(
                                        text = msg.reasoning.replace("<think>", "").replace("</think>", "").trim(),
                                        fontSize = 12.sp,
                                        lineHeight = 18.5.sp,
                                        color = theme.textSecondary,
                                        modifier = Modifier.padding(horizontal = 10.dp, vertical = 9.dp)
                                    )
                                }
                            }
                        }
                    }

                    // Collapsible Tool Execution Steps Card (uses summarizeToolStep)
                    if (msg.toolTraces.isNotEmpty() || msg.tools.isNotEmpty()) {
                        ToolStepsAccordionCard(
                            traces = msg.toolTraces,
                            fallbackLabels = msg.tools,
                            expanded = expandTools,
                            onToggle = { expandTools = !expandTools }
                        )
                    }

                    // Structured Rich Cards
                    if (msg.toolTraces.isNotEmpty()) {
                        ChatStructuredToolCards(
                            traces = msg.toolTraces,
                            onToggleTodo = onQuickToggleTodo,
                            liveTodos = liveTodos
                        )
                    }

                    // Main Response Body or Live Streaming Pulse
                    if (msg.content.isNotEmpty()) {
                        RichMarkdownText(
                            markdown = msg.content,
                            modifier = Modifier.fillMaxWidth()
                        )
                        if (isLiveTurn) {
                            StreamingPulseRow(label = "正在输出")
                        }
                    } else if (isLiveTurn) {
                        val liveStatusLabel = when {
                            msg.toolTraces.any { isToolPending(it.status) } -> "正在调用工具"
                            msg.reasoning.isNotEmpty() -> "正在整理回复"
                            else -> "正在思考"
                        }
                        StreamingPulseRow(label = liveStatusLabel)
                    }
                }

                // Inline Action Footer on Latest Reply or Error Turn
                val hasTurnError = !isLiveTurn && (
                    msg.content.startsWith("[请求异常]") ||
                        msg.content.contains("\n[中断:")
                    )

                val matchedModelOpt = remember(msg.model, msg.modelTargetId, modelOptions) {
                    val cleanTarget = msg.modelTargetId.trim()
                    val clean = msg.model.replace(Regex("\\s*\\(已回退\\)$"), "").trim()
                    if (cleanTarget.isNotEmpty()) {
                        modelOptions.firstOrNull { it.id == cleanTarget }
                    } else {
                        null
                    } ?: if (clean.isEmpty()) {
                        null
                    } else {
                        modelOptions.firstOrNull {
                            it.id == clean || it.model == clean || it.id.endsWith("::$clean")
                        }
                    }
                }
                val shortModelUsed = remember(msg.model, matchedModelOpt) {
                    val raw = msg.model.ifBlank { matchedModelOpt?.model?.ifBlank { matchedModelOpt.label }.orEmpty() }
                    raw.removePrefix("pool:").substringAfter("::").substringAfterLast("/").take(24)
                }

                if (!isLiveTurn && msg.content.isNotEmpty() && (isLatestReply || hasTurnError)) {
                    LazyRow(
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        modifier = Modifier.padding(start = 2.dp, top = 1.dp)
                    ) {
                        if (shortModelUsed.isNotBlank()) {
                            item(key = "model_pill") {
                                Box(
                                    modifier = Modifier
                                        .sizeIn(minHeight = 32.dp)
                                        .vesperPressable(pressedScale = 0.94f, hapticOnTap = true, onClick = onRetry),
                                    contentAlignment = Alignment.Center
                                ) {
                                    VesperTag(
                                        text = shortModelUsed,
                                        toneColor = theme.accentForeground
                                    )
                                }
                            }
                        }

                        item(key = "retry_btn") {
                            InlineBubbleAction(
                                icon = Icons.Default.Refresh,
                                label = "换模型重试",
                                tint = theme.accentForeground,
                                onClick = onRetry
                            )
                        }

                        item(key = "copy_btn") {
                            InlineBubbleAction(
                                icon = Icons.Default.ContentCopy,
                                label = "复制",
                                tint = theme.textMuted,
                                onClick = onCopy
                            )
                        }

                        item(key = "quote_btn") {
                            InlineBubbleAction(
                                icon = Icons.Default.FormatQuote,
                                label = "引用",
                                tint = theme.textMuted,
                                onClick = { onQuote(senderName) }
                            )
                        }
                    }
                }
            }

            VesperPopoverMenu(
                expanded = showBubbleMenu,
                onDismissRequest = { showBubbleMenu = false },
                alignment = Alignment.TopStart,
                offsetX = 6.dp,
                offsetY = 28.dp,
                items = listOf(
                    VesperMenuAction(
                        label = "换模型重试",
                        icon = Icons.Default.Refresh,
                        iconTint = theme.accentForeground,
                        onClick = onRetry
                    ),
                    VesperMenuAction(
                        label = "复制",
                        icon = Icons.Default.ContentCopy,
                        iconTint = theme.accentForeground,
                        onClick = onCopy
                    ),
                    VesperMenuAction(
                        label = "引用",
                        icon = Icons.Default.FormatQuote,
                        iconTint = theme.info,
                        onClick = { onQuote(senderName) }
                    ),
                    VesperMenuAction(
                        label = "转为待办",
                        icon = Icons.Default.TaskAlt,
                        iconTint = theme.info,
                        onClick = onConvertToTodo
                    ),
                    VesperMenuAction(
                        label = "存为笔记",
                        icon = Icons.Default.Description,
                        iconTint = theme.accentForeground,
                        onClick = onSaveAsNote
                    ),
                    VesperMenuAction(
                        label = "转发",
                        icon = Icons.Default.Share,
                        iconTint = theme.success,
                        onClick = { onForward(senderName) }
                    ),
                    VesperMenuAction(
                        label = "由此分叉",
                        icon = Icons.Default.AutoAwesome,
                        iconTint = theme.warning,
                        onClick = onFork
                    ),
                    VesperMenuAction(
                        label = "删除",
                        icon = Icons.Outlined.DeleteOutline,
                        danger = true,
                        onClick = onDelete
                    )
                )
            )
        }
    }
}

@Composable
private fun ToolStepsAccordionCard(
    traces: List<ChatToolTraceUi>,
    fallbackLabels: List<String>,
    expanded: Boolean,
    onToggle: () -> Unit
) {
    val theme = BotHubThemeTokens
    val pendingTrace = traces.firstOrNull { isToolPending(it.status) }
    val hasError = traces.any { isToolError(it.status) }
    val stepCount = if (traces.isNotEmpty()) traces.size else fallbackLabels.size
    val statusColor = when {
        pendingTrace != null -> theme.warning
        hasError -> theme.danger
        else -> theme.success
    }
    val headerText = remember(traces, fallbackLabels) {
        when {
            pendingTrace != null -> {
                val label = pendingTrace.displayName.ifBlank { formatHumanToolName(pendingTrace.name) }
                "正在执行 · $label"
            }
            traces.isNotEmpty() -> {
                val names = traces
                    .map { it.displayName.ifBlank { formatHumanToolName(it.name) } }
                    .distinct()
                    .take(2)
                    .joinToString("、")
                if (stepCount > 1) "已执行 $stepCount 项工具 · $names" else "已调用工具 · $names"
            }
            else -> {
                val names = fallbackLabels.take(2).joinToString("、")
                if (fallbackLabels.size > 1) "已执行 ${fallbackLabels.size} 项工具 · $names" else "已调用工具 · $names"
            }
        }
    }

    val boxShape = RoundedCornerShape(10.dp)
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clip(boxShape)
            .background(theme.codeBlockBg, boxShape)
            .border(0.75.dp, theme.border, boxShape)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .sizeIn(minHeight = 38.dp)
                .clickable(onClick = onToggle)
                .padding(horizontal = 10.dp, vertical = 7.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Row(
                modifier = Modifier
                    .weight(1f)
                    .padding(end = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(7.dp)
            ) {
                Box(
                    modifier = Modifier
                        .size(7.dp)
                        .background(statusColor, CircleShape)
                )
                Icon(
                    imageVector = Icons.Default.Build,
                    contentDescription = null,
                    tint = theme.textMuted,
                    modifier = Modifier.size(13.dp)
                )
                Text(
                    text = headerText,
                    fontSize = 11.5.sp,
                    fontWeight = FontWeight.Medium,
                    color = theme.textSecondary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
            Icon(
                imageVector = if (expanded) Icons.Default.KeyboardArrowDown else Icons.AutoMirrored.Filled.KeyboardArrowRight,
                contentDescription = if (expanded) "收起工具步骤" else "展开工具步骤",
                tint = theme.textMuted,
                modifier = Modifier.size(16.dp)
            )
        }

        AnimatedVisibility(
            visible = expanded,
            enter = fadeIn() + expandVertically(),
            exit = fadeOut() + shrinkVertically()
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(start = 10.dp, end = 10.dp, bottom = 9.dp),
                verticalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                VesperDivider()
                if (traces.isNotEmpty()) {
                    traces.forEach { trace ->
                        val pending = isToolPending(trace.status)
                        val error = isToolError(trace.status)
                        val stepTint = when {
                            pending -> theme.warning
                            error -> theme.danger
                            else -> theme.success
                        }
                        val stepIcon = when {
                            error -> Icons.Default.ErrorOutline
                            else -> Icons.Default.CheckCircleOutline
                        }
                        val humanName = trace.displayName.ifBlank { formatHumanToolName(trace.name) }
                        val stepSummary = remember(trace.id, trace.arguments) { summarizeToolStep(trace) }
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            verticalAlignment = Alignment.Top,
                            horizontalArrangement = Arrangement.spacedBy(7.dp)
                        ) {
                            Icon(
                                imageVector = stepIcon,
                                contentDescription = null,
                                tint = stepTint,
                                modifier = Modifier
                                    .padding(top = 2.dp)
                                    .size(13.dp)
                            )
                            Column(
                                modifier = Modifier.weight(1f),
                                verticalArrangement = Arrangement.spacedBy(1.dp)
                            ) {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Text(
                                        text = humanName,
                                        fontSize = 11.5.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        color = theme.textPrimary,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        modifier = Modifier.weight(1f)
                                    )
                                    Text(
                                        text = if (pending) "进行中" else if (error) "失败" else "完成",
                                        fontSize = 10.5.sp,
                                        color = stepTint,
                                        maxLines = 1,
                                        softWrap = false
                                    )
                                }
                                if (stepSummary.isNotEmpty()) {
                                    Text(
                                        text = stepSummary,
                                        fontFamily = FontFamily.Monospace,
                                        fontSize = 10.5.sp,
                                        color = theme.textMuted,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis
                                    )
                                }
                            }
                        }
                    }
                } else {
                    fallbackLabels.forEach { label ->
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(7.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(5.dp)
                                    .background(theme.success, CircleShape)
                            )
                            Text(
                                text = label,
                                fontSize = 11.5.sp,
                                color = theme.textSecondary,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun StreamingPulseRow(label: String) {
    val theme = BotHubThemeTokens
    val transition = rememberInfiniteTransition(label = "streamingPulse")
    val phase by transition.animateFloat(
        initialValue = 0f,
        targetValue = 3f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 900, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "pulsePhase"
    )
    Row(
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        modifier = Modifier.padding(top = 2.dp)
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(3.dp)) {
            for (idx in 0..2) {
                val dist = kotlin.math.abs(phase - idx)
                val dotAlpha = (1f - (dist.coerceAtMost(1f) * 0.65f)).coerceIn(0.32f, 1f)
                Box(
                    modifier = Modifier
                        .size(5.dp)
                        .graphicsLayer { alpha = dotAlpha }
                        .background(theme.accent, CircleShape)
                )
            }
        }
        Text(
            text = label,
            fontSize = 11.5.sp,
            color = theme.textMuted
        )
    }
}

@Composable
private fun InlineBubbleAction(
    icon: ImageVector,
    label: String,
    tint: Color,
    onClick: () -> Unit
) {
    Row(
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(4.dp),
        modifier = Modifier
            .sizeIn(minHeight = 32.dp)
            .clip(RoundedCornerShape(8.dp))
            .vesperPressable(pressedScale = 0.94f, hapticOnTap = true, onClick = onClick)
            .padding(horizontal = 6.dp, vertical = 4.dp)
    ) {
        Icon(
            imageVector = icon,
            contentDescription = label,
            tint = tint,
            modifier = Modifier.size(13.dp)
        )
        Text(
            text = label,
            fontSize = 11.5.sp,
            fontWeight = FontWeight.Medium,
            color = tint,
            maxLines = 1,
            softWrap = false
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ForwardMessageSheet(
    senderName: String,
    bodyContent: String,
    activeBotId: String?,
    allBots: List<BotItem>,
    onForward: (targetBot: BotItem, note: String, openTarget: Boolean) -> Unit,
    onDismiss: () -> Unit
) {
    val theme = BotHubThemeTokens
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var selectedBotId by remember(allBots, activeBotId) {
        mutableStateOf(
            allBots.firstOrNull { it.id != activeBotId }?.id
                ?: allBots.firstOrNull()?.id.orEmpty()
        )
    }
    var attachNote by remember { mutableStateOf("") }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 24.dp, topEnd = 24.dp),
        containerColor = theme.surfaceSheet,
        scrimColor = theme.scrim,
        tonalElevation = 0.dp,
        dragHandle = { VesperSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .padding(horizontal = 20.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Text(
                text = "转发消息",
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )

            VesperCard(
                cornerRadius = 14.dp,
                modifier = Modifier.fillMaxWidth()
            ) {
                Text(
                    text = "$senderName: ${bodyContent.replace('\n', ' ').take(100)}",
                    fontSize = 13.sp,
                    color = theme.textSecondary,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 11.dp)
                )
            }

            LazyRow(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                items(allBots, key = { "fwd_${it.id}" }) { candidate ->
                    val chosen = candidate.id == selectedBotId
                    Column(
                        modifier = Modifier
                            .widthIn(min = 66.dp, max = 78.dp)
                            .vesperPressable(pressedScale = 0.94f, hapticOnTap = true) {
                                selectedBotId = candidate.id
                            }
                            .clip(RoundedCornerShape(14.dp))
                            .background(if (chosen) theme.accentSoft else theme.surfaceControl)
                            .border(
                                1.dp,
                                if (chosen) theme.accentMuted else theme.border,
                                RoundedCornerShape(14.dp)
                            )
                            .padding(horizontal = 8.dp, vertical = 9.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(5.dp)
                    ) {
                        BotAvatar(
                            bot = candidate,
                            allBots = allBots,
                            containerSize = 40.dp
                        )
                        Text(
                            text = candidate.name,
                            fontSize = 11.sp,
                            fontWeight = if (chosen) FontWeight.SemiBold else FontWeight.Normal,
                            color = if (chosen) theme.accentForeground else theme.textSecondary,
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }
            }

            VesperTextField(
                value = attachNote,
                onValueChange = { attachNote = it },
                placeholder = "附言（可选）",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                val targetBot = allBots.find { it.id == selectedBotId }
                VesperSecondaryButton(
                    text = "仅转发",
                    enabled = targetBot != null,
                    minHeight = 48.dp,
                    onClick = { if (targetBot != null) onForward(targetBot, attachNote, false) },
                    modifier = Modifier.weight(1f)
                )
                VesperPrimaryButton(
                    text = "转发并进入",
                    enabled = targetBot != null,
                    minHeight = 48.dp,
                    onClick = { if (targetBot != null) onForward(targetBot, attachNote, true) },
                    modifier = Modifier.weight(1.25f)
                )
            }
        }
    }
}

@Composable
private fun PlusTrayItem(
    tile: PlusActionTile,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val sqShape = RoundedCornerShape(14.dp)
    Column(
        modifier = modifier
            .sizeIn(minHeight = 76.dp)
            .vesperPressable(
                pressedScale = 0.94f,
                hapticOnTap = true,
                onClick = tile.onClick
            ),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(6.dp, Alignment.CenterVertically)
    ) {
        Box(
            modifier = Modifier
                .size(52.dp)
                .clip(sqShape)
                .background(theme.surfaceControl, sqShape)
                .border(1.dp, theme.border, sqShape),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = tile.icon,
                contentDescription = tile.label,
                tint = tile.accent,
                modifier = Modifier.size(22.dp)
            )
        }
        Text(
            text = tile.label,
            style = TextStyle(
                fontSize = 11.5.sp,
                fontWeight = FontWeight.Medium,
                color = theme.textSecondary,
                platformStyle = NoFontPaddingStyle
            ),
            maxLines = 1,
            softWrap = false,
            overflow = TextOverflow.Ellipsis
        )
    }
}

@Composable
private fun AuthenticatedChatImage(
    url: String,
    onLoadImage: suspend (String) -> ImageBitmap?,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    var bitmap by remember(url) { mutableStateOf<ImageBitmap?>(null) }
    var failed by remember(url) { mutableStateOf(false) }

    LaunchedEffect(url) {
        failed = false
        val loaded = onLoadImage(url)
        bitmap = loaded
        if (loaded == null) {
            failed = true
        }
    }

    val currentBmp = bitmap
    if (currentBmp != null) {
        Image(
            bitmap = currentBmp,
            contentDescription = "聊天图片附件",
            contentScale = ContentScale.Crop,
            modifier = modifier.background(theme.surfaceControl)
        )
    } else {
        Box(
            modifier = modifier.background(theme.surfaceControl),
            contentAlignment = Alignment.Center
        ) {
            Text(
                text = if (failed) "图片加载失败" else "加载中...",
                fontSize = 11.sp,
                color = theme.textMuted
            )
        }
    }
}
