package uk.starryheavens.vesper.bothub.ui.screens

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.scaleOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.ExperimentalFoundationApi
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
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material.icons.filled.ChevronLeft
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.DeleteSweep
import androidx.compose.material.icons.filled.Description
import androidx.compose.material.icons.filled.FormatQuote
import androidx.compose.material.icons.filled.GroupAdd
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowRight
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
import androidx.compose.material3.HorizontalDivider
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
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
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
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.BotThreadItem
import uk.starryheavens.vesper.bothub.data.ChatTurnUi
import uk.starryheavens.vesper.bothub.data.ModelOptionItem
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.data.SkillItemUi
import uk.starryheavens.vesper.bothub.data.TodoItemUi
import uk.starryheavens.vesper.bothub.data.formatHumanToolName
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.components.ChatStructuredToolCards
import uk.starryheavens.vesper.bothub.ui.components.CrispTagPill
import uk.starryheavens.vesper.bothub.ui.components.FloatingGlassPopover
import uk.starryheavens.vesper.bothub.ui.components.GlassMenuAction
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.RichMarkdownText
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.SpecularHairline
import uk.starryheavens.vesper.bothub.ui.components.SpiritSquircleAvatar
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.tactileClickable
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
    SlashCommandItem("/models", "巡检模型池与用量", "查看免费池/付费池健康度及近 7 日统计", prompt = "请调用 manage_models(action=\"status\") 巡检当前免费池与付费池健康状态及近 7 日调用统计。", anyOfTools = setOf("manage_models")),
    SlashCommandItem("/newbot", "创建专属 Bot", "用自然语言描述角色定位，自动配置并创建", prompt = "请调用 manage_bots(action=\"create_bot\") 帮我创建一个专属 Bot：", anyOfTools = setOf("manage_bots")),
    SlashCommandItem("/group", "组建协同群聊", "指定成员与讨论主题，一键组建多 Bot 群聊", prompt = "请调用 manage_bots(action=\"create_group\") 帮我拉一个多 Bot 协同群聊：", anyOfTools = setOf("manage_bots")),
    SlashCommandItem("/newskill", "创建自定义技能", "创建 Prompt 或 Webhook 技能并挂载给 Bot", prompt = "请调用 manage_skills(action=\"upsert\") 帮我创建一个自定义技能：", anyOfTools = setOf("manage_skills")),
    SlashCommandItem("/consult", "协调成员协办", "调用其他专精 Bot 协助完成子任务并汇总", prompt = "请协调工作区里最合适的专业成员共同处理：", anyOfTools = setOf("consult_bot")),
    SlashCommandItem("/calc", "精确表达式计算", "使用高精度计算引擎核算费用或比例", prompt = "请调用 calculate_eval 精确计算：", anyOfTools = setOf("calculate_eval")),
    SlashCommandItem("/memories", "回忆关键信息", "检索与当前会话相关的历史重要记忆", prompt = "请先检索我们的历史记忆，梳理目前你记住的关键事实与约定。", anyOfTools = setOf("manage_memory", "recall_memory")),
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
    inputText: String,
    isStreaming: Boolean,
    quotedNotification: NotificationItem? = null,
    onDismissQuotedNotification: () -> Unit = {},
    onBack: () -> Unit,
    onInputChange: (String) -> Unit,
    onSendPrompt: (String) -> Unit,
    onStop: () -> Unit,
    onModelChange: (String) -> Unit,
    onThinkingChange: (Boolean) -> Unit,
    onSelectThread: (String) -> Unit = {},
    onCreateThread: (String) -> Unit = {},
    onDeleteThread: (String) -> Unit = {},
    onSummarizeThread: () -> Unit = {},
    onForkFromTurn: (ChatTurnUi) -> Unit = {},
    onRetryTurnWithModel: (ChatTurnUi, String, Boolean) -> Unit = { _, _, _ -> },
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
    var showSearchBar by remember(activeBot?.id) { mutableStateOf(false) }
    var searchQuery by remember(activeBot?.id) { mutableStateOf("") }
    var activeMatchCursor by remember(activeBot?.id) { mutableStateOf(0) }

    BackHandler {
        if (showSearchBar) {
            showSearchBar = false
            searchQuery = ""
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

    // 0ms instant jump to latest message on room entry (zero scroll-past-history lag)
    LaunchedEffect(activeBot?.id) {
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
        if (raw.isEmpty() || isStreaming) return
        val finalPrompt = quoteTarget?.let { (sender, snippet) ->
            val shortQuote = snippet.replace('\n', ' ').take(90)
            "> 引用 $sender: $shortQuote\n\n$raw"
        } ?: raw
        quoteTarget = null
        showPlusDrawer = false
        onSendPrompt(finalPrompt)
    }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(theme.bgBase)
            .imePadding()
    ) {
        // 1. Clean WeChat / QQ IM Chat Header
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(theme.topBarTop)
                .statusBarsPadding()
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 10.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Row(
                    modifier = Modifier
                        .sizeIn(minWidth = 64.dp, minHeight = 36.dp)
                        .tactileClickable(pressedScale = 0.94f, onClick = onBack)
                        .padding(horizontal = 6.dp, vertical = 6.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(2.dp)
                ) {
                    Icon(
                        imageVector = Icons.Default.ChevronLeft,
                        contentDescription = "返回",
                        tint = theme.textPrimary,
                        modifier = Modifier.size(22.dp)
                    )
                    Text(
                        text = if (otherUnreadCount > 0) "消息($otherUnreadCount)" else "消息",
                        style = TextStyle(
                            fontSize = 14.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textPrimary,
                            platformStyle = NoFontPaddingStyle
                        ),
                        maxLines = 1,
                        softWrap = false
                    )
                }

                Column(
                    modifier = Modifier
                        .weight(1f)
                        .clickable {
                            if (activeBot != null) onOpenBotProfile(activeBot)
                        }
                        .padding(horizontal = 8.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(1.dp)
                ) {
                    Text(
                        text = if (isStreaming && !isGroup) {
                            "对方正在输入..."
                        } else {
                            buildString {
                                append(activeBot?.name ?: "AI 好友")
                                if (isGroup && groupMembers.isNotEmpty()) {
                                    append(" (${groupMembers.size})")
                                }
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
                    if (isStreaming && isGroup) {
                        Text(
                            text = "成员协作发言中...",
                            style = TextStyle(
                                fontSize = 11.sp,
                                color = if (theme.isLight) theme.accentDeep else theme.accentLight,
                                platformStyle = NoFontPaddingStyle
                            ),
                            maxLines = 1,
                            softWrap = false
                        )
                    }
                }

                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    Box(
                        modifier = Modifier
                            .size(34.dp)
                            .tactileClickable(pressedScale = 0.92f, hapticOnTap = true) {
                                showSearchBar = !showSearchBar
                                if (!showSearchBar) searchQuery = ""
                            }
                            .clip(CircleShape)
                            .background(
                                if (showSearchBar) {
                                    theme.accent.copy(alpha = if (theme.isLight) 0.12f else 0.18f)
                                } else {
                                    theme.secondaryBtnTop
                                },
                                CircleShape
                            )
                            .border(
                                1.dp,
                                if (showSearchBar) theme.accent.copy(alpha = 0.45f) else theme.divider,
                                CircleShape
                            ),
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = Icons.Default.Search,
                            contentDescription = "查找聊天内容",
                            tint = if (showSearchBar) {
                                if (theme.isLight) theme.accentDeep else theme.accentLight
                            } else {
                                theme.textSecondary
                            },
                            modifier = Modifier.size(17.dp)
                        )
                    }

                    Box(
                        modifier = Modifier
                            .size(36.dp)
                            .tactileClickable(pressedScale = 0.92f, hapticOnTap = true, onClick = onOpenChatInfo)
                            .clip(CircleShape)
                            .background(theme.secondaryBtnTop, CircleShape)
                            .border(1.dp, theme.divider, CircleShape),
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = Icons.Default.MoreHoriz,
                            contentDescription = "聊天详情",
                            tint = theme.textPrimary,
                            modifier = Modifier.size(19.dp)
                        )
                    }
                }
            }

            AnimatedVisibility(
                visible = showSearchBar,
                enter = fadeIn() + expandVertically(),
                exit = fadeOut() + shrinkVertically()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(start = 12.dp, end = 12.dp, bottom = 8.dp)
                        .clip(RoundedCornerShape(14.dp))
                        .background(theme.inputTop)
                        .border(0.85.dp, theme.accent.copy(alpha = 0.35f), RoundedCornerShape(14.dp))
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Icon(
                        imageVector = Icons.Default.Search,
                        contentDescription = null,
                        tint = theme.accent,
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
                        modifier = Modifier.weight(1f),
                        decorationBox = { inner ->
                            Box(contentAlignment = Alignment.CenterStart) {
                                if (searchQuery.isEmpty()) {
                                    Text(
                                        text = "查找聊天内容...",
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
                            text = if (searchMatchIndices.isEmpty()) {
                                "无匹配"
                            } else {
                                "${safeMatchCursor + 1}/${searchMatchIndices.size}"
                            },
                            fontFamily = FontFamily.Monospace,
                            fontSize = 11.sp,
                            color = if (searchMatchIndices.isEmpty()) {
                                if (theme.isLight) Color(0xFFDC2626) else Color(0xFFFDA4AF)
                            } else {
                                if (theme.isLight) theme.accentDeep else theme.accentLight
                            },
                            maxLines = 1,
                            softWrap = false
                        )
                        if (searchMatchIndices.size > 1) {
                            Icon(
                                imageVector = Icons.Default.KeyboardArrowUp,
                                contentDescription = "上一条",
                                tint = theme.textSecondary,
                                modifier = Modifier
                                    .size(20.dp)
                                    .clickable {
                                        activeMatchCursor = if (safeMatchCursor > 0) {
                                            safeMatchCursor - 1
                                        } else {
                                            searchMatchIndices.lastIndex
                                        }
                                    }
                            )
                            Icon(
                                imageVector = Icons.Default.KeyboardArrowDown,
                                contentDescription = "下一条",
                                tint = theme.textSecondary,
                                modifier = Modifier
                                    .size(20.dp)
                                    .clickable {
                                        activeMatchCursor = (safeMatchCursor + 1) % searchMatchIndices.size
                                    }
                            )
                        }
                    }
                    Icon(
                        imageVector = Icons.Default.Close,
                        contentDescription = "关闭搜索",
                        tint = theme.textMuted,
                        modifier = Modifier
                            .size(17.dp)
                            .clickable {
                                showSearchBar = false
                                searchQuery = ""
                            }
                    )
                }
            }

            SpecularHairline()
        }

        // 1b. Multi-Topic Thread Bar (Auto-hidden when only Main Thread exists — matches Web BotThreadBar)
        if (activeBot != null && botThreads.isNotEmpty()) {
            LazyRow(
                contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
                verticalAlignment = Alignment.CenterVertically,
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.cardTop)
            ) {
                item(key = "__thread_main__") {
                    val isMainActive = activeThreadId.isBlank()
                    val pillShape = RoundedCornerShape(999.dp)
                    Row(
                        modifier = Modifier
                            .clip(pillShape)
                            .background(
                                if (isMainActive) theme.accent.copy(alpha = 0.18f) else theme.inputTop
                            )
                            .border(
                                0.85.dp,
                                if (isMainActive) theme.accent.copy(alpha = 0.48f) else theme.rimSubtle,
                                pillShape
                            )
                            .tactileClickable(pressedScale = 0.94f, hapticOnTap = true) {
                                if (!isStreaming && !isMainActive) onSelectThread("")
                            }
                            .padding(horizontal = 11.dp, vertical = 4.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "主对话",
                            fontSize = 11.5.sp,
                            fontWeight = if (isMainActive) FontWeight.SemiBold else FontWeight.Normal,
                            color = if (isMainActive) {
                                if (theme.isLight) theme.accentDeep else theme.accentLight
                            } else {
                                theme.textSecondary
                            }
                        )
                    }
                }

                items(botThreads, key = { "th_${it.id}" }) { th ->
                    val isActive = activeThreadId == th.id
                    val pillShape = RoundedCornerShape(999.dp)
                    Row(
                        modifier = Modifier
                            .clip(pillShape)
                            .background(
                                if (isActive) theme.accent.copy(alpha = 0.18f) else theme.inputTop
                            )
                            .border(
                                0.85.dp,
                                if (isActive) theme.accent.copy(alpha = 0.48f) else theme.rimSubtle,
                                pillShape
                            )
                            .tactileClickable(pressedScale = 0.94f, hapticOnTap = true) {
                                if (!isStreaming && !isActive) onSelectThread(th.id)
                            }
                            .padding(start = 11.dp, end = if (isActive) 7.dp else 11.dp, top = 4.dp, bottom = 4.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(5.dp)
                    ) {
                        Text(
                            text = th.title.ifBlank { "新话题" },
                            fontSize = 11.5.sp,
                            fontWeight = if (isActive) FontWeight.SemiBold else FontWeight.Normal,
                            color = if (isActive) {
                                if (theme.isLight) theme.accentDeep else theme.accentLight
                            } else {
                                theme.textSecondary
                            },
                            maxLines = 1
                        )
                        if (isActive && !isStreaming) {
                            Icon(
                                imageVector = Icons.Default.Close,
                                contentDescription = "删除话题",
                                tint = theme.textMuted,
                                modifier = Modifier
                                    .size(13.dp)
                                    .clickable { onDeleteThread(th.id) }
                            )
                        }
                    }
                }
            }
            HorizontalDivider(thickness = 0.5.dp, color = theme.divider)
        }

        // 2. Pinned Group Announcement Banner
        if (isGroup && !dismissAnnouncement && !activeBot?.announcement.isNullOrBlank()) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.cardTop)
                    .padding(horizontal = 14.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                CrispTagPill(
                    text = "群公告",
                    bgColor = theme.accent.copy(alpha = 0.18f),
                    borderColor = theme.accent.copy(alpha = 0.36f),
                    textColor = if (theme.isLight) theme.accentDeep else theme.accentLight
                )
                Text(
                    text = activeBot?.announcement.orEmpty(),
                    fontSize = 12.sp,
                    color = theme.textSecondary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f)
                )
                Icon(
                    imageVector = Icons.Default.Close,
                    contentDescription = "收起公告",
                    tint = theme.textMuted,
                    modifier = Modifier
                        .size(16.dp)
                        .clickable { dismissAnnouncement = true }
                )
            }
            HorizontalDivider(thickness = 0.5.dp, color = theme.divider)
        }

        // 3. IM Message Bubble Stream + Floating Scroll-to-Bottom Pill
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
                contentPadding = PaddingValues(horizontal = 12.dp, vertical = 14.dp)
            ) {
            // Pure, uncluttered empty chat state (Avatar + Name + Bio only — zero gimmick prompt buttons!)
            if (messages.isEmpty()) {
                item(key = "__chat_welcome__") {
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 24.dp, vertical = 48.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        SpiritSquircleAvatar(
                            bot = activeBot,
                            allBots = allBots,
                            containerSize = 68.dp,
                            cornerRadius = 20.dp,
                            interactive = true,
                            seedKey = activeBot?.id ?: "chat_empty"
                        )
                        Text(
                            text = activeBot?.name ?: "AI 好友",
                            fontSize = 17.sp,
                            fontWeight = FontWeight.Bold,
                            color = theme.textPrimary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        val desc = activeBot?.traits?.ifBlank { activeBot.roleTitle }.orEmpty()
                        if (desc.isNotBlank()) {
                            Text(
                                text = desc,
                                fontSize = 12.5.sp,
                                color = theme.textSecondary,
                                textAlign = TextAlign.Center,
                                maxLines = 2,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                        Text(
                            text = "输入 / 可快速调用当前助手的专属指令与技能",
                            fontSize = 11.5.sp,
                            color = theme.textMuted,
                            textAlign = TextAlign.Center
                        )
                    }
                }
            }

            items(messages, key = { it.id }) { msg ->
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
                                    .background(theme.cardTop)
                                    .padding(horizontal = 12.dp, vertical = 5.dp)
                            ) {
                                Text(
                                    text = msg.content,
                                    fontSize = 11.5.sp,
                                    lineHeight = 16.sp,
                                    color = theme.textSecondary,
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
                            topStart = 14.dp,
                            topEnd = 4.dp,
                            bottomStart = 14.dp,
                            bottomEnd = 14.dp
                        )
                        val userHitBorder = when {
                            isActiveHit -> Modifier.border(1.5.dp, theme.accentLight, userShape)
                            isSearchHit -> Modifier.border(1.dp, theme.accent.copy(alpha = 0.65f), userShape)
                            else -> Modifier
                        }

                        Box(
                            modifier = Modifier.fillMaxWidth(),
                            contentAlignment = Alignment.CenterEnd
                        ) {
                            Box(
                                modifier = Modifier
                                    .widthIn(max = 304.dp)
                                    .clip(userShape)
                                    .background(theme.userBubbleTop, userShape)
                                    .then(userHitBorder)
                                    .combinedClickable(
                                        onClick = {},
                                        onLongClick = {
                                            haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                                            showUserMenu = true
                                        }
                                    )
                                    .padding(horizontal = 14.dp, vertical = 10.dp)
                            ) {
                                Text(
                                    text = msg.content,
                                    style = MaterialTheme.typography.bodyMedium.copy(
                                        fontSize = 14.5.sp,
                                        lineHeight = 21.sp
                                    ),
                                    color = theme.userBubbleText
                                )
                            }

                            FloatingGlassPopover(
                                expanded = showUserMenu,
                                onDismissRequest = { showUserMenu = false },
                                alignment = Alignment.TopEnd,
                                offsetY = 38.dp,
                                items = listOf(
                                    GlassMenuAction(
                                        label = "复制",
                                        icon = Icons.Default.ContentCopy,
                                        iconTint = if (theme.isLight) theme.accentDeep else theme.accentLight,
                                        onClick = { onCopyText(msg.content) }
                                    ),
                                    GlassMenuAction(
                                        label = "引用",
                                        icon = Icons.Default.FormatQuote,
                                        iconTint = if (theme.isLight) Color(0xFF4F46E5) else Color(0xFFA5B4FC),
                                        onClick = { quoteTarget = "我" to msg.content }
                                    ),
                                    GlassMenuAction(
                                        label = "转发",
                                        icon = Icons.Default.Share,
                                        iconTint = if (theme.isLight) Color(0xFF059669) else Color(0xFF34D399),
                                        onClick = { forwardPayload = "我" to msg.content }
                                    ),
                                    GlassMenuAction(
                                        label = "由此分叉",
                                        icon = Icons.Default.AutoAwesome,
                                        iconTint = theme.accent,
                                        onClick = { onForkFromTurn(msg) }
                                    ),
                                    GlassMenuAction(
                                        label = "删除",
                                        icon = Icons.Outlined.DeleteOutline,
                                        iconTint = if (theme.isLight) Color(0xFFDC2626) else Color(0xFFFB7185),
                                        onClick = { onDeleteTurn(msg) }
                                    )
                                )
                            )
                        }
                    }

                    else -> {
                        var expandReasoning by remember { mutableStateOf(false) }
                        var showBubbleMenu by remember { mutableStateOf(false) }
                        val isActiveHit = msg.id == activeMatchedTurnId
                        val isSearchHit = msg.id in matchedTurnIds
                        val isLiveTurn = isStreaming && msg.id == lastAssistantId
                        val isLatestReply = msg.id == lastAssistantId && !isLiveTurn
                        val avatarLiveState = when {
                            isLiveTurn && msg.content.isEmpty() -> "thinking"
                            isLiveTurn -> "working"
                            else -> "idle"
                        }
                        val speakerBot = remember(msg.senderBotId, allBots) {
                            allBots.find { it.id == msg.senderBotId } ?: activeBot
                        }
                        val aiShape = RoundedCornerShape(
                            topStart = 4.dp,
                            topEnd = 14.dp,
                            bottomStart = 14.dp,
                            bottomEnd = 14.dp
                        )

                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .then(if (!isLiveTurn) Modifier.animateContentSize() else Modifier),
                            horizontalArrangement = Arrangement.spacedBy(9.dp),
                            verticalAlignment = Alignment.Top
                        ) {
                            SpiritSquircleAvatar(
                                bot = speakerBot,
                                allBots = allBots,
                                avatarStyleOverride = msg.avatarStyle,
                                state = avatarLiveState,
                                containerSize = 38.dp,
                                cornerRadius = 12.dp,
                                seedKey = msg.id,
                                onClick = {
                                    if (speakerBot != null) onOpenBotProfile(speakerBot)
                                },
                                modifier = Modifier.padding(top = 2.dp)
                            )

                            Box(modifier = Modifier.weight(1f)) {
                                Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                                    if (isGroup || msg.msgKind == "routine") {
                                        Row(
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(6.dp),
                                            modifier = Modifier.padding(start = 2.dp)
                                        ) {
                                            Text(
                                                text = msg.persona.ifEmpty { speakerBot?.name ?: "AI" },
                                                fontSize = 11.5.sp,
                                                fontWeight = FontWeight.Medium,
                                                color = theme.textSecondary,
                                                maxLines = 1,
                                                softWrap = false,
                                                overflow = TextOverflow.Ellipsis,
                                                modifier = Modifier.weight(1f, fill = false)
                                            )
                                            if (msg.msgKind == "routine") {
                                                val warnTint = if (theme.isLight) Color(0xFFB45309) else Color(0xFFFDE68A)
                                                CrispTagPill(
                                                    text = "提醒",
                                                    bgColor = Color(0xFFFBBF24).copy(alpha = 0.16f),
                                                    borderColor = Color(0xFFFBBF24).copy(alpha = 0.34f),
                                                    textColor = warnTint
                                                )
                                            }
                                        }
                                    }

                                    val aiBorderWidth = if (isActiveHit) 1.5.dp else 1.dp
                                    val aiBorderColor = when {
                                        isActiveHit -> theme.accentLight
                                        isSearchHit -> theme.accent.copy(alpha = 0.65f)
                                        else -> theme.divider
                                    }

                                    Column(
                                        modifier = Modifier
                                            .fillMaxWidth()
                                            .clip(aiShape)
                                            .background(theme.aiBubbleTop, aiShape)
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
                                            .padding(horizontal = 13.dp, vertical = 10.dp),
                                        verticalArrangement = Arrangement.spacedBy(7.dp)
                                    ) {
                                        val visibleTraces = remember(msg.toolTraces) {
                                            val structuredNames = setOf(
                                                "manage_assets",
                                                "query_nezha_nodes",
                                                "manage_todos",
                                                "manage_tasks",
                                                "workspace_briefing"
                                            )
                                            msg.toolTraces.filter { tr ->
                                                val pendingOrErr = tr.status == "进行中" ||
                                                    tr.status == "执行中" ||
                                                    tr.status.equals("error", ignoreCase = true) ||
                                                    tr.status.contains("失败")
                                                pendingOrErr || tr.name !in structuredNames || tr.result.isBlank()
                                            }.distinctBy { it.name to it.status }
                                        }
                                        if (msg.reasoning.isNotEmpty() || visibleTraces.isNotEmpty() || (msg.toolTraces.isEmpty() && msg.tools.isNotEmpty())) {
                                            val toolOk = if (theme.isLight) Color(0xFF059669) else Color(0xFF34D399)
                                            val toolOkText = if (theme.isLight) Color(0xFF065F46) else Color(0xFFA7F3D0)
                                            val toolPending = if (theme.isLight) Color(0xFFD97706) else Color(0xFFFBBF24)
                                            val toolPendingText = if (theme.isLight) Color(0xFF92400E) else Color(0xFFFDE68A)
                                            val toolErr = if (theme.isLight) Color(0xFFDC2626) else Color(0xFFFB7185)
                                            val toolErrText = if (theme.isLight) Color(0xFFBE123C) else Color(0xFFFDA4AF)
                                            val reasonTint = if (theme.isLight) theme.accentDeep else theme.accentLight

                                            LazyRow(
                                                horizontalArrangement = Arrangement.spacedBy(6.dp),
                                                verticalAlignment = Alignment.CenterVertically
                                            ) {
                                                if (msg.reasoning.isNotEmpty()) {
                                                    item(key = "__reason_pill__") {
                                                        val rPillShape = RoundedCornerShape(999.dp)
                                                        Row(
                                                            modifier = Modifier
                                                                .clip(rPillShape)
                                                                .background(
                                                                    if (expandReasoning) {
                                                                        theme.accent.copy(alpha = 0.18f)
                                                                    } else {
                                                                        theme.codeBlockBg
                                                                    },
                                                                    rPillShape
                                                                )
                                                                .border(
                                                                    0.75.dp,
                                                                    theme.accent.copy(alpha = if (expandReasoning) 0.52f else 0.28f),
                                                                    rPillShape
                                                                )
                                                                .clickable { expandReasoning = !expandReasoning }
                                                                .padding(horizontal = 8.dp, vertical = 3.5.dp),
                                                            verticalAlignment = Alignment.CenterVertically,
                                                            horizontalArrangement = Arrangement.spacedBy(4.dp)
                                                        ) {
                                                            Icon(
                                                                imageVector = Icons.Default.Psychology,
                                                                contentDescription = null,
                                                                tint = reasonTint,
                                                                modifier = Modifier.size(13.dp)
                                                            )
                                                            Text(
                                                                text = "思考 · ${msg.reasoning.length}字",
                                                                fontSize = 11.sp,
                                                                fontWeight = FontWeight.Medium,
                                                                color = reasonTint,
                                                                maxLines = 1,
                                                                softWrap = false
                                                            )
                                                            Icon(
                                                                imageVector = if (expandReasoning) Icons.Default.KeyboardArrowDown else Icons.Default.KeyboardArrowRight,
                                                                contentDescription = null,
                                                                tint = reasonTint,
                                                                modifier = Modifier.size(13.dp)
                                                            )
                                                        }
                                                    }
                                                }

                                                if (visibleTraces.isNotEmpty()) {
                                                    items(visibleTraces, key = { "tr_${it.id}_${it.name}" }) { trace ->
                                                        val isPending = trace.status == "进行中" || trace.status == "执行中"
                                                        val isError = trace.status.equals("error", ignoreCase = true) ||
                                                            trace.status.contains("失败")
                                                        val dotColor = when {
                                                            isPending -> toolPending
                                                            isError -> toolErr
                                                            else -> toolOk
                                                        }
                                                        val labelColor = when {
                                                            isPending -> toolPendingText
                                                            isError -> toolErrText
                                                            else -> toolOkText
                                                        }
                                                        val baseLabel = trace.displayName.ifBlank {
                                                            formatHumanToolName(trace.name)
                                                        }
                                                        val pillText = if (isPending) "$baseLabel · 进行中" else baseLabel
                                                        val tShape = RoundedCornerShape(999.dp)
                                                        Row(
                                                            modifier = Modifier
                                                                .clip(tShape)
                                                                .background(theme.codeInlineBg, tShape)
                                                                .border(0.7.dp, dotColor.copy(alpha = 0.28f), tShape)
                                                                .padding(horizontal = 8.dp, vertical = 3.5.dp),
                                                            verticalAlignment = Alignment.CenterVertically,
                                                            horizontalArrangement = Arrangement.spacedBy(5.dp)
                                                        ) {
                                                            Box(
                                                                modifier = Modifier
                                                                    .size(5.dp)
                                                                    .background(dotColor, CircleShape)
                                                            )
                                                            Text(
                                                                text = pillText,
                                                                fontSize = 11.sp,
                                                                color = labelColor,
                                                                maxLines = 1,
                                                                softWrap = false
                                                            )
                                                        }
                                                    }
                                                } else if (msg.toolTraces.isEmpty() && msg.tools.isNotEmpty()) {
                                                    items(msg.tools) { tLabel ->
                                                        val isPending = tLabel.contains("进行中") || tLabel.contains("执行中")
                                                        val dotColor = if (isPending) toolPending else toolOk
                                                        val labelColor = if (isPending) toolPendingText else toolOkText
                                                        val tShape = RoundedCornerShape(999.dp)
                                                        Row(
                                                            modifier = Modifier
                                                                .clip(tShape)
                                                                .background(theme.codeInlineBg, tShape)
                                                                .border(0.7.dp, dotColor.copy(alpha = 0.28f), tShape)
                                                                .padding(horizontal = 8.dp, vertical = 3.5.dp),
                                                            verticalAlignment = Alignment.CenterVertically,
                                                            horizontalArrangement = Arrangement.spacedBy(5.dp)
                                                        ) {
                                                            Box(
                                                                modifier = Modifier
                                                                    .size(5.dp)
                                                                    .background(dotColor, CircleShape)
                                                            )
                                                            Text(
                                                                text = tLabel,
                                                                fontSize = 11.sp,
                                                                color = labelColor,
                                                                maxLines = 1,
                                                                softWrap = false
                                                            )
                                                        }
                                                    }
                                                }
                                            }
                                        }

                                        if (msg.reasoning.isNotEmpty()) {
                                            AnimatedVisibility(
                                                visible = expandReasoning,
                                                enter = fadeIn() + expandVertically(),
                                                exit = fadeOut() + shrinkVertically()
                                            ) {
                                                val rShape = RoundedCornerShape(10.dp)
                                                Column(
                                                    modifier = Modifier
                                                        .fillMaxWidth()
                                                        .clip(rShape)
                                                        .background(theme.codeBlockBg, rShape)
                                                        .border(0.7.dp, theme.accent.copy(alpha = 0.22f), rShape)
                                                        .padding(horizontal = 10.dp, vertical = 8.dp)
                                                ) {
                                                    Text(
                                                        text = msg.reasoning.replace("<think>", "").replace("</think>", "").trim(),
                                                        fontSize = 12.sp,
                                                        lineHeight = 18.sp,
                                                        color = theme.textSecondary
                                                    )
                                                }
                                            }
                                        }

                                        if (msg.toolTraces.isNotEmpty()) {
                                            ChatStructuredToolCards(
                                                traces = msg.toolTraces,
                                                onToggleTodo = onQuickToggleTodo,
                                                liveTodos = liveTodos
                                            )
                                        }

                                        if (msg.content.isNotEmpty()) {
                                            RichMarkdownText(
                                                markdown = msg.content,
                                                modifier = Modifier.fillMaxWidth()
                                            )
                                        } else if (isLiveTurn) {
                                            Row(
                                                verticalAlignment = Alignment.CenterVertically,
                                                horizontalArrangement = Arrangement.spacedBy(6.dp),
                                                modifier = Modifier.padding(vertical = 2.dp)
                                            ) {
                                                Box(
                                                    modifier = Modifier
                                                        .size(6.dp)
                                                        .background(theme.accent, CircleShape)
                                                )
                                                Text(
                                                    text = "正在输入...",
                                                    fontSize = 12.5.sp,
                                                    color = theme.textSecondary
                                                )
                                            }
                                        }
                                    }

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
                                    val isPaidModelUsed = remember(msg.model, msg.modelTier, msg.modelTargetId, matchedModelOpt) {
                                        msg.modelTier.equals("paid", ignoreCase = true) ||
                                            msg.modelTargetId == "primary" ||
                                            msg.modelTargetId == "auto:paid" ||
                                            msg.model == "auto:paid" ||
                                            matchedModelOpt?.tier == "paid"
                                    }
                                    val shortModelUsed = remember(msg.model, matchedModelOpt) {
                                        val raw = msg.model.ifBlank { matchedModelOpt?.model?.ifBlank { matchedModelOpt.label }.orEmpty() }
                                        raw.removePrefix("pool:").substringAfter("::").substringAfterLast("/").take(26)
                                    }

                                    if (!isLiveTurn && msg.content.isNotEmpty() && (isLatestReply || hasTurnError)) {
                                        Row(
                                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                                            verticalAlignment = Alignment.CenterVertically,
                                            modifier = Modifier.padding(start = 4.dp, top = 2.dp)
                                        ) {
                                            if (shortModelUsed.isNotBlank()) {
                                                Box(
                                                    modifier = Modifier.tactileClickable(pressedScale = 0.94f, hapticOnTap = true) {
                                                        if (!isStreaming) retryingTurn = msg
                                                    }
                                                ) {
                                                    CrispTagPill(
                                                        text = if (isPaidModelUsed) "付费 · $shortModelUsed" else shortModelUsed,
                                                        bgColor = if (isPaidModelUsed) Color(0x22F59E0B) else Color(0x1F10B981),
                                                        borderColor = if (isPaidModelUsed) Color(0x4AF59E0B) else Color(0x4210B981),
                                                        textColor = if (isPaidModelUsed) Color(0xFFD97706) else Color(0xFF10B981)
                                                    )
                                                }
                                            }

                                            Row(
                                                verticalAlignment = Alignment.CenterVertically,
                                                horizontalArrangement = Arrangement.spacedBy(3.dp),
                                                modifier = Modifier
                                                    .tactileClickable(pressedScale = 0.94f, hapticOnTap = true) {
                                                        if (!isStreaming) retryingTurn = msg
                                                    }
                                            ) {
                                                Icon(
                                                    imageVector = Icons.Default.Refresh,
                                                    contentDescription = "换模型重试",
                                                    tint = theme.accent,
                                                    modifier = Modifier.size(12.dp)
                                                )
                                                Text(
                                                    text = "换模型重试",
                                                    fontSize = 11.5.sp,
                                                    fontWeight = FontWeight.Medium,
                                                    color = theme.accent,
                                                    maxLines = 1,
                                                    softWrap = false
                                                )
                                            }

                                            Row(
                                                verticalAlignment = Alignment.CenterVertically,
                                                horizontalArrangement = Arrangement.spacedBy(3.dp),
                                                modifier = Modifier
                                                    .tactileClickable(pressedScale = 0.94f) {
                                                        onCopyText(msg.content)
                                                    }
                                            ) {
                                                Icon(
                                                    imageVector = Icons.Default.ContentCopy,
                                                    contentDescription = "复制",
                                                    tint = theme.textMuted,
                                                    modifier = Modifier.size(12.dp)
                                                )
                                                Text(
                                                    text = "复制",
                                                    fontSize = 11.5.sp,
                                                    color = theme.textMuted,
                                                    maxLines = 1,
                                                    softWrap = false
                                                )
                                            }

                                            Row(
                                                verticalAlignment = Alignment.CenterVertically,
                                                horizontalArrangement = Arrangement.spacedBy(3.dp),
                                                modifier = Modifier
                                                    .tactileClickable(pressedScale = 0.94f) {
                                                        val sender = msg.persona.ifEmpty { speakerBot?.name ?: "AI" }
                                                        quoteTarget = sender to msg.content
                                                    }
                                            ) {
                                                Icon(
                                                    imageVector = Icons.Default.FormatQuote,
                                                    contentDescription = "引用",
                                                    tint = theme.textMuted,
                                                    modifier = Modifier.size(12.dp)
                                                )
                                                Text(
                                                    text = "引用",
                                                    fontSize = 11.5.sp,
                                                    color = theme.textMuted,
                                                    maxLines = 1,
                                                    softWrap = false
                                                )
                                            }
                                        }
                                    }
                                }

                                FloatingGlassPopover(
                                    expanded = showBubbleMenu,
                                    onDismissRequest = { showBubbleMenu = false },
                                    alignment = Alignment.TopStart,
                                    offsetX = 6.dp,
                                    offsetY = 28.dp,
                                    items = listOf(
                                        GlassMenuAction(
                                            label = "换模型重试",
                                            icon = Icons.Default.Refresh,
                                            iconTint = theme.accent,
                                            onClick = {
                                                if (!isStreaming) retryingTurn = msg
                                            }
                                        ),
                                        GlassMenuAction(
                                            label = "复制",
                                            icon = Icons.Default.ContentCopy,
                                            iconTint = if (theme.isLight) theme.accentDeep else theme.accentLight,
                                            onClick = { onCopyText(msg.content) }
                                        ),
                                        GlassMenuAction(
                                            label = "引用",
                                            icon = Icons.Default.FormatQuote,
                                            iconTint = if (theme.isLight) Color(0xFF4F46E5) else Color(0xFFA5B4FC),
                                            onClick = {
                                                val sender = msg.persona.ifEmpty { speakerBot?.name ?: "AI" }
                                                quoteTarget = sender to msg.content
                                            }
                                        ),
                                        GlassMenuAction(
                                            label = "转为待办",
                                            icon = Icons.Default.TaskAlt,
                                            iconTint = if (theme.isLight) Color(0xFF0284C7) else Color(0xFF38BDF8),
                                            onClick = { onConvertTurnToTodo(msg) }
                                        ),
                                        GlassMenuAction(
                                            label = "存为笔记",
                                            icon = Icons.Default.Description,
                                            iconTint = theme.accent,
                                            onClick = { onSaveTurnAsNote(msg) }
                                        ),
                                        GlassMenuAction(
                                            label = "转发",
                                            icon = Icons.Default.Share,
                                            iconTint = if (theme.isLight) Color(0xFF059669) else Color(0xFF34D399),
                                            onClick = {
                                                val sender = msg.persona.ifEmpty { speakerBot?.name ?: "AI" }
                                                forwardPayload = sender to msg.content
                                            }
                                        ),
                                        GlassMenuAction(
                                            label = "由此分叉",
                                            icon = Icons.Default.AutoAwesome,
                                            iconTint = theme.accent,
                                            onClick = { onForkFromTurn(msg) }
                                        ),
                                        GlassMenuAction(
                                            label = "删除",
                                            icon = Icons.Outlined.DeleteOutline,
                                            iconTint = if (theme.isLight) Color(0xFFDC2626) else Color(0xFFFB7185),
                                            onClick = { onDeleteTurn(msg) }
                                        )
                                    )
                                )
                            }
                        }
                    }
                }
            }
            }

            // Floating "回到底部" Glass Pill when scrolled up
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
                        .shadow(
                            6.dp,
                            pillShape,
                            ambientColor = Color.Black.copy(alpha = if (theme.isLight) 0.14f else 0.45f)
                        )
                        .tactileClickable(pressedScale = 0.92f, hapticOnTap = true) {
                            scope.launch {
                                if (messages.isNotEmpty()) {
                                    listState.animateScrollToItem(messages.lastIndex)
                                }
                            }
                        }
                        .clip(pillShape)
                        .background(theme.popoverTop, pillShape)
                        .border(1.dp, theme.rimHighlight, pillShape)
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    Icon(
                        imageVector = Icons.Default.KeyboardArrowDown,
                        contentDescription = "回到底部",
                        tint = if (theme.isLight) theme.accentDeep else theme.accentLight,
                        modifier = Modifier.size(15.dp)
                    )
                    Text(
                        text = if (isStreaming) "新回复" else "回到底部",
                        style = TextStyle(
                            fontSize = 11.5.sp,
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
                contentPadding = PaddingValues(horizontal = 12.dp, vertical = 5.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.bottomBarTop)
            ) {
                items(groupMembers, key = { "at_${it.id}" }) { member ->
                    val chipShape = RoundedCornerShape(999.dp)
                    Row(
                        modifier = Modifier
                            .tactileClickable(pressedScale = 0.94f, hapticOnTap = true) {
                                val tag = "@${member.name} "
                                if (!inputText.contains(tag)) {
                                    val sep = if (inputText.isNotEmpty() && !inputText.endsWith(" ")) " " else ""
                                    onInputChange("$inputText$sep$tag")
                                }
                            }
                            .clip(chipShape)
                            .background(theme.secondaryBtnTop, chipShape)
                            .border(0.7.dp, theme.rimMid, chipShape)
                            .padding(start = 5.dp, end = 9.dp, top = 3.dp, bottom = 3.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(4.dp)
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
                            fontSize = 11.sp,
                            color = theme.textSecondary,
                            maxLines = 1,
                            softWrap = false
                        )
                    }
                }
            }
        }

        // 4b. Dynamic "/" Slash Command Popover (filtered by activeBot.allowedTools & bound skills)
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
            enter = fadeIn(tween(130)) + expandVertically(tween(150)),
            exit = fadeOut(tween(110)) + shrinkVertically(tween(130))
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.sheetBg)
            ) {
                HorizontalDivider(thickness = 0.5.dp, color = theme.divider)
                LazyColumn(
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(maxHeight = 220.dp),
                    contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                    verticalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    items(matchingSlashCommands, key = { it.cmd }) { item ->
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clip(RoundedCornerShape(10.dp))
                                .background(theme.inputTop.copy(alpha = 0.72f))
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
                                .padding(horizontal = 11.dp, vertical = 8.dp),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Row(
                                modifier = Modifier.weight(1f).padding(end = 8.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp)
                            ) {
                                Text(
                                    text = item.cmd,
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 12.sp,
                                    fontWeight = FontWeight.Bold,
                                    color = if (theme.isLight) theme.accentDeep else theme.accentLight,
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

        // 5. Quoted Reply Preview Strip
        quoteTarget?.let { (sender, snippet) ->
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.inputTop)
                    .padding(horizontal = 14.dp, vertical = 7.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Text(
                    text = "$sender: ${snippet.replace('\n', ' ')}",
                    fontSize = 11.5.sp,
                    color = theme.textSecondary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier
                        .weight(1f)
                        .padding(end = 8.dp)
                )
                Icon(
                    imageVector = Icons.Default.Close,
                    contentDescription = "取消引用",
                    tint = theme.textSecondary,
                    modifier = Modifier
                        .size(16.dp)
                        .clickable { quoteTarget = null }
                )
            }
        }

        if (quotedNotification != null) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(theme.bottomBarTop)
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
                            color = theme.accent,
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
                    Icon(
                        imageVector = Icons.Default.Close,
                        contentDescription = "取消引用通知",
                        tint = theme.textSecondary,
                        modifier = Modifier
                            .size(18.dp)
                            .clickable { onDismissQuotedNotification() }
                    )
                }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    val chipShape = RoundedCornerShape(999.dp)
                    Box(
                        modifier = Modifier
                            .clip(chipShape)
                            .background(theme.accent.copy(alpha = 0.14f), chipShape)
                            .border(0.7.dp, theme.accent.copy(alpha = 0.42f), chipShape)
                            .tactileClickable(enabled = !isStreaming, pressedScale = 0.94f, hapticOnTap = true) {
                                onSendPrompt("请针对这条服务通知深入展开讲讲，补充关键背景、细节与延伸思考。")
                            }
                            .padding(horizontal = 10.dp, vertical = 5.dp)
                    ) {
                        Text(
                            text = "深入展开讲讲",
                            style = TextStyle(
                                fontSize = 11.5.sp,
                                fontWeight = FontWeight.Medium,
                                color = theme.accent,
                                platformStyle = NoFontPaddingStyle
                            )
                        )
                    }
                    Box(
                        modifier = Modifier
                            .clip(chipShape)
                            .background(theme.inputTop, chipShape)
                            .border(0.7.dp, theme.divider, chipShape)
                            .tactileClickable(enabled = !isStreaming, pressedScale = 0.94f, hapticOnTap = true) {
                                onSendPrompt("请结合我的个人情况、近期记忆与待办，分析这条服务通知对我有什么直接启发或行动建议。")
                            }
                            .padding(horizontal = 10.dp, vertical = 5.dp)
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

        // 6. Clean Bottom Composer + 1-Row 4-Tile IM "+" Toolbar
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(theme.bottomBarTop)
                .navigationBarsPadding()
        ) {
            SpecularHairline()

            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 10.dp, vertical = 8.dp),
                verticalAlignment = Alignment.Bottom,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                val inputShape = RoundedCornerShape(18.dp)
                val inputBorder by animateColorAsState(
                    targetValue = if (composerFocused) theme.accent else theme.divider,
                    animationSpec = tween(150),
                    label = "composerBorder"
                )
                Box(
                    modifier = Modifier
                        .weight(1f)
                        .sizeIn(minHeight = 40.dp)
                        .clip(inputShape)
                        .background(theme.inputTop, inputShape)
                        .border(if (composerFocused) 1.2.dp else 1.dp, inputBorder, inputShape),
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
                            .padding(horizontal = 14.dp, vertical = 10.dp)
                            .onFocusChanged { composerFocused = it.isFocused },
                        decorationBox = { inner ->
                            Box(contentAlignment = Alignment.CenterStart) {
                                if (inputText.isEmpty()) {
                                    Text(
                                        text = "发消息或输入 / 调出快捷指令...",
                                        fontSize = 13.5.sp,
                                        color = theme.textMuted,
                                        maxLines = 1,
                                        softWrap = false
                                    )
                                }
                                inner()
                            }
                        }
                    )
                }

                Box(
                    modifier = Modifier
                        .size(40.dp)
                        .tactileClickable(pressedScale = 0.90f, hapticOnTap = true) {
                            showPlusDrawer = !showPlusDrawer
                        }
                        .clip(CircleShape)
                        .background(
                            if (showPlusDrawer) {
                                theme.accent.copy(alpha = if (theme.isLight) 0.12f else 0.18f)
                            } else {
                                theme.secondaryBtnTop
                            },
                            CircleShape
                        )
                        .border(
                            1.dp,
                            if (showPlusDrawer) theme.accent.copy(alpha = 0.45f) else theme.divider,
                            CircleShape
                        ),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = if (showPlusDrawer) Icons.Default.Close else Icons.Default.Add,
                        contentDescription = "更多功能",
                        tint = if (showPlusDrawer) {
                            if (theme.isLight) theme.accentDeep else theme.accentLight
                        } else {
                            theme.textSecondary
                        },
                        modifier = Modifier.size(20.dp)
                    )
                }

                if (isStreaming) {
                    TactileSecondaryButton(
                        text = "停止",
                        leadingIcon = Icons.Default.Stop,
                        danger = true,
                        cornerRadius = 999.dp,
                        minHeight = 40.dp,
                        contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                        onClick = onStop
                    )
                } else if (inputText.isNotBlank()) {
                    TactilePrimaryButton(
                        text = "发送",
                        cornerRadius = 999.dp,
                        minHeight = 40.dp,
                        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 6.dp),
                        onClick = { dispatchSend() }
                    )
                }
            }

            // Pure 2x3 Native Chat Controls (Model, Thinking, New Thread, Summarize Memory, Group, Clear)
            AnimatedVisibility(
                visible = showPlusDrawer,
                enter = fadeIn() + expandVertically(),
                exit = fadeOut() + shrinkVertically()
            ) {
                val modelShortLabel = resolveModelOptionDisplayName(activeModel, modelOptions)
                val tools = listOf(
                    PlusActionTile(
                        label = modelShortLabel,
                        icon = Icons.Default.AutoAwesome,
                        accent = if (theme.isLight) Color(0xFF9333EA) else Color(0xFFC084FC)
                    ) {
                        showModelPickerSheet = true
                    },
                    PlusActionTile(
                        label = if (enableThinking) "思考:开" else "思考:关",
                        icon = Icons.Default.Psychology,
                        accent = if (enableThinking) theme.accent else theme.textSecondary
                    ) {
                        onThinkingChange(!enableThinking)
                    },
                    PlusActionTile(
                        label = "新开话题",
                        icon = Icons.Default.Add,
                        accent = if (theme.isLight) Color(0xFF059669) else Color(0xFF34D399)
                    ) {
                        showPlusDrawer = false
                        if (!isStreaming) onCreateThread("")
                    },
                    PlusActionTile(
                        label = "提炼记忆",
                        icon = Icons.Default.Description,
                        accent = theme.accent
                    ) {
                        showPlusDrawer = false
                        if (!isStreaming) onSummarizeThread()
                    },
                    PlusActionTile(
                        label = if (isGroup) "群成员" else "拉人建群",
                        icon = Icons.Default.GroupAdd,
                        accent = if (theme.isLight) Color(0xFF0284C7) else Color(0xFF38BDF8)
                    ) {
                        showPlusDrawer = false
                        onInviteToGroup()
                    },
                    PlusActionTile(
                        label = "清空记录",
                        icon = Icons.Default.DeleteSweep,
                        accent = if (theme.isLight) Color(0xFFDC2626) else Color(0xFFFB7185)
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
                    HorizontalDivider(thickness = 0.5.dp, color = theme.divider)
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
                val isPaid = chosen == "auto:paid" || modelOptions.any {
                    (it.id == chosen || it.model == chosen) && it.tier == "paid"
                }
                onRetryTurnWithModel(targetTurn, chosen, isPaid)
            },
            onDismiss = { retryingTurn = null }
        )
    }

    forwardPayload?.let { (senderName, bodyContent) ->
        val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
        var selectedBotId by remember(allBots, activeBot?.id) {
            mutableStateOf(
                allBots.firstOrNull { it.id != activeBot?.id }?.id
                    ?: allBots.firstOrNull()?.id.orEmpty()
            )
        }
        var attachNote by remember { mutableStateOf("") }

        ModalBottomSheet(
            onDismissRequest = { forwardPayload = null },
            sheetState = sheetState,
            shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
            containerColor = theme.sheetBg,
            scrimColor = theme.scrim,
            tonalElevation = 0.dp,
            dragHandle = { ObsidianSheetDragHandle() }
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
                    fontSize = 17.5.sp,
                    fontWeight = FontWeight.Bold,
                    color = theme.textPrimary
                )

                SpecularGlassCard(
                    cornerRadius = 15.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Text(
                        text = "$senderName: ${bodyContent.replace('\n', ' ').take(100)}",
                        fontSize = 12.5.sp,
                        color = theme.textSecondary,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp)
                    )
                }

                LazyRow(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    items(allBots, key = { "fwd_${it.id}" }) { candidate ->
                        val chosen = candidate.id == selectedBotId
                        Column(
                            modifier = Modifier
                                .widthIn(min = 64.dp, max = 76.dp)
                                .tactileClickable(pressedScale = 0.94f, hapticOnTap = true) {
                                    selectedBotId = candidate.id
                                }
                                .clip(RoundedCornerShape(14.dp))
                                .background(
                                    if (chosen) theme.accent.copy(alpha = 0.16f) else theme.secondaryBtnTop
                                )
                                .border(
                                    0.85.dp,
                                    if (chosen) theme.accent.copy(alpha = 0.55f) else theme.rimMid,
                                    RoundedCornerShape(14.dp)
                                )
                                .padding(horizontal = 8.dp, vertical = 8.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(5.dp)
                        ) {
                            SpiritSquircleAvatar(
                                bot = candidate,
                                allBots = allBots,
                                containerSize = 40.dp,
                                cornerRadius = 12.dp
                            )
                            Text(
                                text = candidate.name,
                                fontSize = 11.sp,
                                fontWeight = if (chosen) FontWeight.SemiBold else FontWeight.Normal,
                                color = if (chosen) {
                                    if (theme.isLight) theme.accentDeep else theme.accentLight
                                } else {
                                    theme.textSecondary
                                },
                                maxLines = 1,
                                softWrap = false,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                    }
                }

                ObsidianGlassInput(
                    value = attachNote,
                    onValueChange = { attachNote = it },
                    placeholder = "附言 (可选)",
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    val targetBot = allBots.find { it.id == selectedBotId }
                    TactileSecondaryButton(
                        text = "仅转发",
                        enabled = targetBot != null,
                        minHeight = 45.dp,
                        onClick = {
                            if (targetBot != null) {
                                onForwardMessage(targetBot, senderName, bodyContent, attachNote, false)
                                forwardPayload = null
                            }
                        },
                        modifier = Modifier.weight(1f)
                    )
                    TactilePrimaryButton(
                        text = "转发并进入",
                        enabled = targetBot != null,
                        minHeight = 45.dp,
                        onClick = {
                            if (targetBot != null) {
                                onForwardMessage(targetBot, senderName, bodyContent, attachNote, true)
                                forwardPayload = null
                            }
                        },
                        modifier = Modifier.weight(1.25f)
                    )
                }
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
    val sqShape = RoundedCornerShape(12.dp)
    Column(
        modifier = modifier.tactileClickable(
            pressedScale = 0.92f,
            hapticOnTap = true,
            onClick = tile.onClick
        ),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(6.dp)
    ) {
        Box(
            modifier = Modifier
                .size(52.dp)
                .clip(sqShape)
                .background(theme.secondaryBtnTop, sqShape)
                .border(1.dp, theme.divider, sqShape),
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
                fontSize = 11.sp,
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
