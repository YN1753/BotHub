package uk.starryheavens.vesper.bothub.ui.screens

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
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
import androidx.compose.material.icons.filled.ChevronLeft
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.DeleteSweep
import androidx.compose.material.icons.filled.FormatQuote
import androidx.compose.material.icons.filled.GroupAdd
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Lightbulb
import androidx.compose.material.icons.filled.MoreHoriz
import androidx.compose.material.icons.filled.Psychology
import androidx.compose.material.icons.filled.Stop
import androidx.compose.material.icons.filled.Tune
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.PlatformTextStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.ChatTurnUi
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.components.BotAvatarOrCluster
import uk.starryheavens.vesper.bothub.ui.components.RichMarkdownText

/**
 * Full-Screen Level-2 IM Chat Room (WeChat / QQ / GrokBot Chat Paradigm).
 * Covers the root tab dock for maximum vertical reading & typing space, supporting
 * 1-on-1 AI Friend chat, Multi-AI Group Chat (@mentions & per-speaker avatars),
 * quoted replies, and an expandable WeChat-style "+" action panel.
 */
@Composable
fun ChatRoomScreen(
    activeBot: BotItem?,
    allBots: List<BotItem>,
    otherUnreadCount: Int,
    activeModel: String,
    enableThinking: Boolean,
    messages: List<ChatTurnUi>,
    inputText: String,
    isStreaming: Boolean,
    onBack: () -> Unit,
    onInputChange: (String) -> Unit,
    onSendPrompt: (String) -> Unit,
    onStop: () -> Unit,
    onModelChange: (String) -> Unit,
    onThinkingChange: (Boolean) -> Unit,
    onCopyText: (String) -> Unit,
    onOpenChatInfo: () -> Unit,
    onOpenBotProfile: (BotItem) -> Unit,
    onInviteToGroup: () -> Unit,
    onOpenSessions: () -> Unit,
    onClearHistory: () -> Unit,
    modifier: Modifier = Modifier
) {
    BackHandler(onBack = onBack)

    val haptics = LocalHapticFeedback.current
    val noPadStyle = remember { PlatformTextStyle(includeFontPadding = false) }
    val listState = rememberLazyListState()
    var showPlusDrawer by remember { mutableStateOf(false) }
    var composerFocused by remember { mutableStateOf(false) }
    var quoteTarget by remember { mutableStateOf<Pair<String, String>?>(null) } // sender to snippet
    var dismissAnnouncement by remember(activeBot?.id) { mutableStateOf(false) }

    val isGroup = activeBot?.isGroup == true
    val groupMembers = remember(activeBot, allBots) {
        if (isGroup && activeBot != null) activeBot.resolveGroupMembers(allBots) else emptyList()
    }
    val lastAssistantId = messages.lastOrNull { it.role == "assistant" }?.id

    LaunchedEffect(messages.size) {
        if (messages.isNotEmpty()) {
            listState.animateScrollToItem(messages.lastIndex)
        }
    }

    LaunchedEffect(composerFocused) {
        if (composerFocused) {
            showPlusDrawer = false
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
            .background(Color(0xFF050811))
            .imePadding()
    ) {
        // 1. Authentic WeChat / QQ IM Chat Header
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(Color(0xF0080C1A))
                .statusBarsPadding()
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 10.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                // Left: "< 消息 (unread)"
                Surface(
                    onClick = onBack,
                    shape = RoundedCornerShape(999.dp),
                    color = Color.Transparent,
                    modifier = Modifier.sizeIn(minWidth = 64.dp, minHeight = 38.dp)
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 6.dp, vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(2.dp)
                    ) {
                        Icon(
                            Icons.Default.ChevronLeft,
                            contentDescription = "返回消息列表",
                            tint = Color(0xFFFFF8EE),
                            modifier = Modifier.size(22.dp)
                        )
                        Text(
                            text = if (otherUnreadCount > 0) "消息($otherUnreadCount)" else "消息",
                            style = TextStyle(
                                fontSize = 14.sp,
                                fontWeight = FontWeight.Medium,
                                color = Color(0xFFFFF8EE),
                                platformStyle = noPadStyle
                            )
                        )
                    }
                }

                // Center: Friend Name / Group Name (count) + Live Status
                Column(
                    modifier = Modifier
                        .weight(1f)
                        .clickable {
                            if (activeBot != null) onOpenBotProfile(activeBot)
                        }
                        .padding(horizontal = 8.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(2.dp)
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
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFF8EE),
                            platformStyle = noPadStyle
                        ),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )

                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(5.dp)
                    ) {
                        Box(
                            modifier = Modifier
                                .size(6.dp)
                                .background(
                                    color = if (isStreaming) Color(0xFFF2B880) else Color(0xFF34D399),
                                    shape = CircleShape
                                )
                        )
                        Text(
                            text = when {
                                isStreaming && isGroup -> "群成员正在思考与接力..."
                                isStreaming -> "对方正在输入..."
                                isGroup -> "${activeBot?.groupModeLabel() ?: "智能协同"} · ${if (activeModel == "free-pool") "免费池" else "主力引擎"}"
                                else -> "在线 · ${if (activeModel == "free-pool") "免费池" else "主力引擎"}"
                            },
                            style = TextStyle(
                                fontSize = 10.5.sp,
                                color = if (isStreaming) Color(0xFFFFE2BE) else Color(0xFF94A3B8),
                                platformStyle = noPadStyle
                            ),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }

                // Right: "···" Chat Detail Sheet Button (WeChat top-right menu)
                Surface(
                    onClick = onOpenChatInfo,
                    shape = CircleShape,
                    color = Color(0xFF10172C),
                    border = BorderStroke(1.dp, Color(0x22FFF8EE)),
                    modifier = Modifier.size(38.dp)
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(
                            Icons.Default.MoreHoriz,
                            contentDescription = "聊天信息与设置",
                            tint = Color(0xFFFFF8EE),
                            modifier = Modifier.size(20.dp)
                        )
                    }
                }
            }

            HorizontalDivider(thickness = 0.5.dp, color = Color(0x1AFFF8EE))
        }

        // 2. Pinned Group Announcement Banner (WeChat "群公告")
        if (isGroup && !dismissAnnouncement && !activeBot?.announcement.isNullOrBlank()) {
            Surface(
                color = Color(0xE60D152B),
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 14.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Surface(
                        shape = RoundedCornerShape(4.dp),
                        color = Color(0x29F2B880)
                    ) {
                        Text(
                            text = "群公告",
                            fontSize = 10.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFE2BE),
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                        )
                    }
                    Text(
                        text = activeBot?.announcement.orEmpty(),
                        fontSize = 12.sp,
                        color = Color(0xFFCBD5E1),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f)
                    )
                    Icon(
                        Icons.Default.Close,
                        contentDescription = "收起公告",
                        tint = Color(0xFF64748B),
                        modifier = Modifier
                            .size(16.dp)
                            .clickable { dismissAnnouncement = true }
                    )
                }
            }
            HorizontalDivider(thickness = 0.5.dp, color = Color(0x18FFF8EE))
        }

        // 3. IM Message Bubble Stream
        LazyColumn(
            state = listState,
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .clickable(
                    interactionSource = remember { androidx.compose.foundation.interaction.MutableInteractionSource() },
                    indication = null
                ) {
                    if (showPlusDrawer) showPlusDrawer = false
                },
            verticalArrangement = Arrangement.spacedBy(14.dp),
            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 14.dp)
        ) {
            if (messages.isEmpty()) {
                item(key = "__chat_welcome__") {
                    val icebreakers = listOf(
                        "盘点资产临期风险" to "请调用工具盘点全部数字资产与算力节点，并按到期风险给出锐评。",
                        "探查哪吒节点脉搏" to "请调用 query_nezha_nodes 检查哪吒监控面板节点的实时在线状态。",
                        "检查调度任务运行" to "请列出当前所有定时调度任务的状态与下次触发时间。",
                        "串联最近收藏干货" to "请调用 manage_stash 检索我最近收入收藏匣的未读干货并做简要串联。"
                    )
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 8.dp, vertical = 18.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        // WeChat "你已添加 X，现在可以开始聊天了" System Pill
                        Surface(
                            shape = RoundedCornerShape(999.dp),
                            color = Color(0x990E152A)
                        ) {
                            Text(
                                text = if (isGroup) {
                                    "群聊已就绪，输入 @ 可点名指定 AI 成员发言"
                                } else {
                                    "你已连接 ${activeBot?.name ?: "AI 好友"}，现在可以开始聊天了"
                                },
                                fontSize = 11.5.sp,
                                color = Color(0xFF94A3B8),
                                modifier = Modifier.padding(horizontal = 14.dp, vertical = 5.dp)
                            )
                        }

                        Surface(
                            shape = RoundedCornerShape(20.dp),
                            color = Color(0xB30C1224),
                            border = BorderStroke(1.dp, Color(0x20FFF8EE)),
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            Column(
                                modifier = Modifier.padding(18.dp),
                                horizontalAlignment = Alignment.CenterHorizontally,
                                verticalArrangement = Arrangement.spacedBy(10.dp)
                            ) {
                                BotAvatarOrCluster(
                                    bot = activeBot,
                                    allBots = allBots,
                                    state = activeBot?.state ?: "idle",
                                    size = 64.dp,
                                    interactive = true,
                                    defaultSeedKey = activeBot?.id ?: "chat_empty"
                                )
                                Text(
                                    text = activeBot?.name ?: "AI 好友",
                                    fontSize = 17.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = Color(0xFFFFF8EE)
                                )
                                Text(
                                    text = activeBot?.traits?.ifBlank { activeBot.roleTitle }
                                        ?.ifBlank { "随时在线待命，直接发消息聊聊或点下方快捷话题" }
                                        ?: "随时在线待命",
                                    fontSize = 12.5.sp,
                                    color = Color(0xFF94A3B8),
                                    textAlign = TextAlign.Center
                                )

                                Spacer(modifier = Modifier.height(2.dp))

                                icebreakers.chunked(2).forEach { pair ->
                                    Row(
                                        modifier = Modifier
                                            .fillMaxWidth()
                                            .height(IntrinsicSize.Min),
                                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                                    ) {
                                        pair.forEach { (label, prompt) ->
                                            Surface(
                                                onClick = { onSendPrompt(prompt) },
                                                shape = RoundedCornerShape(12.dp),
                                                color = Color(0xFF111932),
                                                border = BorderStroke(1.dp, Color(0x24FFF8EE)),
                                                modifier = Modifier
                                                    .weight(1f)
                                                    .fillMaxHeight()
                                                    .sizeIn(minHeight = 44.dp)
                                            ) {
                                                Row(
                                                    modifier = Modifier.padding(horizontal = 10.dp, vertical = 10.dp),
                                                    verticalAlignment = Alignment.CenterVertically,
                                                    horizontalArrangement = Arrangement.SpaceBetween
                                                ) {
                                                    Text(
                                                        text = label,
                                                        fontSize = 12.sp,
                                                        fontWeight = FontWeight.Medium,
                                                        color = Color(0xFFE2E8F0),
                                                        maxLines = 1,
                                                        overflow = TextOverflow.Ellipsis,
                                                        modifier = Modifier.weight(1f)
                                                    )
                                                    Icon(
                                                        Icons.Default.KeyboardArrowRight,
                                                        contentDescription = null,
                                                        tint = Color(0x99F2B880),
                                                        modifier = Modifier.size(14.dp)
                                                    )
                                                }
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }

            items(messages, key = { it.id }) { msg ->
                when (msg.role) {
                    "system" -> {
                        // Centered WeChat System Notice Pill
                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(vertical = 2.dp),
                            contentAlignment = Alignment.Center
                        ) {
                            Surface(
                                shape = RoundedCornerShape(10.dp),
                                color = Color(0x990D1428)
                            ) {
                                Text(
                                    text = msg.content,
                                    fontSize = 11.5.sp,
                                    lineHeight = 16.sp,
                                    color = Color(0xFF8C9AB6),
                                    textAlign = TextAlign.Center,
                                    modifier = Modifier
                                        .widthIn(max = 310.dp)
                                        .padding(horizontal = 12.dp, vertical = 5.dp)
                                )
                            }
                        }
                    }

                    "user" -> {
                        // Right-Aligned User IM Bubble
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.End,
                            verticalAlignment = Alignment.Top
                        ) {
                            Surface(
                                shape = RoundedCornerShape(
                                    topStart = 18.dp,
                                    topEnd = 4.dp,
                                    bottomStart = 18.dp,
                                    bottomEnd = 18.dp
                                ),
                                color = Color(0xFF281E17),
                                border = BorderStroke(1.dp, Color(0x5CF2B880)),
                                modifier = Modifier.widthIn(max = 304.dp)
                            ) {
                                Text(
                                    text = msg.content,
                                    style = MaterialTheme.typography.bodyMedium.copy(
                                        fontSize = 14.5.sp,
                                        lineHeight = 21.sp
                                    ),
                                    color = Color(0xFFFFF8EE),
                                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp)
                                )
                            }
                        }
                    }

                    else -> {
                        // Left-Aligned AI Friend / Group Member IM Bubble
                        var expandReasoning by remember { mutableStateOf(false) }
                        val isLiveTurn = isStreaming && msg.id == lastAssistantId
                        val avatarLiveState = when {
                            isLiveTurn && msg.content.isEmpty() -> "thinking"
                            isLiveTurn -> "working"
                            else -> "idle"
                        }
                        val speakerBot = remember(msg.senderBotId, allBots) {
                            allBots.find { it.id == msg.senderBotId } ?: activeBot
                        }

                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .animateContentSize(),
                            horizontalArrangement = Arrangement.spacedBy(9.dp),
                            verticalAlignment = Alignment.Top
                        ) {
                            // Tappable AI Avatar (opens Profile Card just like WeChat/QQ!)
                            Surface(
                                onClick = {
                                    if (speakerBot != null) onOpenBotProfile(speakerBot)
                                },
                                shape = RoundedCornerShape(11.dp),
                                color = Color(0xFF0E152B),
                                border = BorderStroke(0.8.dp, Color(0x22FFF8EE)),
                                modifier = Modifier
                                    .padding(top = 2.dp)
                                    .size(38.dp)
                            ) {
                                Box(contentAlignment = Alignment.Center) {
                                    GrokBotAvatar(
                                        avatarStyle = msg.avatarStyle,
                                        state = avatarLiveState,
                                        size = 32.dp,
                                        interactive = false,
                                        seedKey = msg.id
                                    )
                                }
                            }

                            Column(
                                modifier = Modifier.weight(1f),
                                verticalArrangement = Arrangement.spacedBy(4.dp)
                            ) {
                                // Show sender name header in Group Chat or Routine Push
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
                                            color = Color(0xFF94A3B8)
                                        )
                                        if (msg.msgKind == "routine") {
                                            Surface(
                                                shape = RoundedCornerShape(4.dp),
                                                color = Color(0x22F2B880)
                                            ) {
                                                Text(
                                                    text = "主动汇报",
                                                    fontSize = 9.5.sp,
                                                    color = Color(0xFFFFE2BE),
                                                    modifier = Modifier.padding(horizontal = 5.dp, vertical = 1.dp)
                                                )
                                            }
                                        }
                                    }
                                }

                                // IM Speech Bubble
                                Surface(
                                    shape = RoundedCornerShape(
                                        topStart = 4.dp,
                                        topEnd = 18.dp,
                                        bottomStart = 18.dp,
                                        bottomEnd = 18.dp
                                    ),
                                    color = Color(0xEB0F162B),
                                    border = BorderStroke(1.dp, Color(0x22FFF8EE)),
                                    modifier = Modifier.fillMaxWidth()
                                ) {
                                    Column(
                                        modifier = Modifier.padding(horizontal = 13.dp, vertical = 10.dp),
                                        verticalArrangement = Arrangement.spacedBy(7.dp)
                                    ) {
                                        if (msg.reasoning.isNotEmpty()) {
                                            Surface(
                                                onClick = { expandReasoning = !expandReasoning },
                                                shape = RoundedCornerShape(10.dp),
                                                color = Color(0x99080C1A),
                                                border = BorderStroke(0.8.dp, Color(0x29F2B880)),
                                                modifier = Modifier.fillMaxWidth()
                                            ) {
                                                Column(modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp)) {
                                                    Row(
                                                        verticalAlignment = Alignment.CenterVertically,
                                                        horizontalArrangement = Arrangement.spacedBy(4.dp)
                                                    ) {
                                                        Icon(
                                                            if (expandReasoning) Icons.Default.KeyboardArrowDown else Icons.Default.KeyboardArrowRight,
                                                            contentDescription = null,
                                                            tint = Color(0xFFFBBF24),
                                                            modifier = Modifier.size(14.dp)
                                                        )
                                                        Text(
                                                            text = "思考过程 (${msg.reasoning.length} 字)",
                                                            fontFamily = FontFamily.Monospace,
                                                            fontSize = 11.sp,
                                                            color = Color(0xFFFDE68A)
                                                        )
                                                    }
                                                    AnimatedVisibility(
                                                        visible = expandReasoning,
                                                        enter = fadeIn() + expandVertically(),
                                                        exit = fadeOut() + shrinkVertically()
                                                    ) {
                                                        Text(
                                                            text = msg.reasoning,
                                                            fontFamily = FontFamily.Monospace,
                                                            fontSize = 11.5.sp,
                                                            lineHeight = 17.5.sp,
                                                            color = Color(0xFF94A3B8),
                                                            modifier = Modifier.padding(top = 5.dp)
                                                        )
                                                    }
                                                }
                                            }
                                        }

                                        if (msg.tools.isNotEmpty()) {
                                            LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                                items(msg.tools) { tLabel ->
                                                    Surface(
                                                        shape = RoundedCornerShape(999.dp),
                                                        color = Color(0xCC0A1224),
                                                        border = BorderStroke(0.8.dp, Color(0x3834D399))
                                                    ) {
                                                        Row(
                                                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.5.dp),
                                                            verticalAlignment = Alignment.CenterVertically,
                                                            horizontalArrangement = Arrangement.spacedBy(5.dp)
                                                        ) {
                                                            Box(
                                                                modifier = Modifier
                                                                    .size(5.dp)
                                                                    .background(Color(0xFF34D399), CircleShape)
                                                            )
                                                            Text(
                                                                text = tLabel,
                                                                fontFamily = FontFamily.Monospace,
                                                                fontSize = 10.5.sp,
                                                                color = Color(0xFFA7F3D0)
                                                            )
                                                        }
                                                    }
                                                }
                                            }
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
                                                        .background(Color(0xFFF2B880), CircleShape)
                                                )
                                                Text(
                                                    text = "正在组织回复...",
                                                    fontSize = 12.5.sp,
                                                    color = Color(0xFF94A3B8)
                                                )
                                            }
                                        }
                                    }
                                }

                                // Subtle IM Bubble Footer Actions (Copy / Quote)
                                if (msg.content.isNotEmpty() && !isLiveTurn) {
                                    Row(
                                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                                        verticalAlignment = Alignment.CenterVertically,
                                        modifier = Modifier.padding(start = 4.dp)
                                    ) {
                                        Row(
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(3.dp),
                                            modifier = Modifier.clickable { onCopyText(msg.content) }
                                        ) {
                                            Icon(
                                                Icons.Default.ContentCopy,
                                                contentDescription = "复制",
                                                tint = Color(0xFF64748B),
                                                modifier = Modifier.size(11.dp)
                                            )
                                            Text(
                                                text = "复制",
                                                fontSize = 10.5.sp,
                                                color = Color(0xFF64748B)
                                            )
                                        }

                                        Row(
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(3.dp),
                                            modifier = Modifier.clickable {
                                                val sender = msg.persona.ifEmpty { speakerBot?.name ?: "AI" }
                                                quoteTarget = sender to msg.content
                                            }
                                        ) {
                                            Icon(
                                                Icons.Default.FormatQuote,
                                                contentDescription = "引用回复",
                                                tint = Color(0xFF64748B),
                                                modifier = Modifier.size(12.dp)
                                            )
                                            Text(
                                                text = "引用",
                                                fontSize = 10.5.sp,
                                                color = Color(0xFF64748B)
                                            )
                                        }
                                    }
                                }
                            }
                        }
                    }
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
                    .background(Color(0xD9080C1A))
            ) {
                items(groupMembers, key = { "at_${it.id}" }) { member ->
                    Surface(
                        onClick = {
                            val tag = "@${member.name} "
                            if (!inputText.contains(tag)) {
                                val sep = if (inputText.isNotEmpty() && !inputText.endsWith(" ")) " " else ""
                                onInputChange("$inputText$sep$tag")
                            }
                        },
                        shape = RoundedCornerShape(999.dp),
                        color = Color(0xFF111932),
                        border = BorderStroke(0.8.dp, Color(0x28FFF8EE))
                    ) {
                        Row(
                            modifier = Modifier.padding(start = 5.dp, end = 9.dp, top = 3.dp, bottom = 3.dp),
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
                                color = Color(0xFFCBD5E1)
                            )
                        }
                    }
                }
            }
        }

        // 5. Quoted Reply Preview Strip (WeChat style)
        quoteTarget?.let { (sender, snippet) ->
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Color(0xF00B1124))
                    .padding(horizontal = 14.dp, vertical = 7.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Text(
                    text = "引用 $sender: ${snippet.replace('\n', ' ')}",
                    fontSize = 11.5.sp,
                    color = Color(0xFF94A3B8),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier
                        .weight(1f)
                        .padding(end = 8.dp)
                )
                Icon(
                    Icons.Default.Close,
                    contentDescription = "取消引用",
                    tint = Color(0xFF94A3B8),
                    modifier = Modifier
                        .size(16.dp)
                        .clickable { quoteTarget = null }
                )
            }
        }

        // 6. WeChat / GrokBot Bottom Input Bar + Expandable "+" Action Panel
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(Color(0xF5080C1A))
                .navigationBarsPadding()
        ) {
            HorizontalDivider(thickness = 0.5.dp, color = Color(0x1FFFF8EE))

            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 10.dp, vertical = 8.dp),
                verticalAlignment = Alignment.Bottom,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                // Thinking Mode Toggle Circle
                Surface(
                    onClick = {
                        haptics.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                        onThinkingChange(!enableThinking)
                    },
                    shape = CircleShape,
                    color = if (enableThinking) Color(0x28F2B880) else Color(0xFF11182E),
                    border = BorderStroke(
                        1.dp,
                        if (enableThinking) Color(0x73F2B880) else Color(0x24FFF8EE)
                    ),
                    modifier = Modifier.size(40.dp)
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(
                            Icons.Default.Psychology,
                            contentDescription = if (enableThinking) "深度思考已开启" else "深度思考已关闭",
                            tint = if (enableThinking) Color(0xFFFFE2BE) else Color(0xFF64748B),
                            modifier = Modifier.size(20.dp)
                        )
                    }
                }

                // Adaptive Rounded Message Input Field
                Surface(
                    shape = RoundedCornerShape(20.dp),
                    color = Color(0xFF0E152B),
                    border = BorderStroke(
                        1.dp,
                        if (composerFocused) Color(0x7AF2B880) else Color(0x24FFF8EE)
                    ),
                    modifier = Modifier
                        .weight(1f)
                        .sizeIn(minHeight = 40.dp)
                ) {
                    BasicTextField(
                        value = inputText,
                        onValueChange = onInputChange,
                        textStyle = TextStyle(
                            color = Color(0xFFFFF8EE),
                            fontSize = 14.5.sp,
                            lineHeight = 20.sp
                        ),
                        cursorBrush = SolidColor(Color(0xFFF2B880)),
                        maxLines = 5,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 14.dp, vertical = 10.dp)
                            .onFocusChanged { composerFocused = it.isFocused },
                        decorationBox = { inner ->
                            Box(contentAlignment = Alignment.CenterStart) {
                                if (inputText.isEmpty()) {
                                    Text(
                                        text = if (isGroup) "发送群聊消息..." else "发消息给 ${activeBot?.name ?: "AI 好友"}...",
                                        fontSize = 14.sp,
                                        color = Color(0xFF64748B),
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis
                                    )
                                }
                                inner()
                            }
                        }
                    )
                }

                // "+" Action Panel Button
                Surface(
                    onClick = { showPlusDrawer = !showPlusDrawer },
                    shape = CircleShape,
                    color = if (showPlusDrawer) Color(0x28F2B880) else Color(0xFF11182E),
                    border = BorderStroke(
                        1.dp,
                        if (showPlusDrawer) Color(0x66F2B880) else Color(0x24FFF8EE)
                    ),
                    modifier = Modifier.size(40.dp)
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(
                            if (showPlusDrawer) Icons.Default.Close else Icons.Default.Add,
                            contentDescription = "展开快捷功能",
                            tint = if (showPlusDrawer) Color(0xFFFFE2BE) else Color(0xFFCBD5E1),
                            modifier = Modifier.size(20.dp)
                        )
                    }
                }

                // Send Pill or Stop Button (WeChat green/gold "发送" button appears when text non-empty)
                if (isStreaming) {
                    Surface(
                        onClick = onStop,
                        shape = RoundedCornerShape(999.dp),
                        color = Color(0x33F43F5E),
                        border = BorderStroke(1.dp, Color(0x80FB7185)),
                        modifier = Modifier.height(40.dp)
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(4.dp)
                        ) {
                            Icon(
                                Icons.Default.Stop,
                                contentDescription = "停止",
                                tint = Color(0xFFFDA4AF),
                                modifier = Modifier.size(16.dp)
                            )
                            Text(
                                text = "停止",
                                fontSize = 13.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFFFDA4AF)
                            )
                        }
                    }
                } else if (inputText.isNotBlank()) {
                    Surface(
                        onClick = { dispatchSend() },
                        shape = RoundedCornerShape(999.dp),
                        color = Color(0xFFF2B880),
                        modifier = Modifier.height(40.dp)
                    ) {
                        Box(
                            contentAlignment = Alignment.Center,
                            modifier = Modifier.padding(horizontal = 15.dp)
                        ) {
                            Text(
                                text = "发送",
                                style = TextStyle(
                                    fontSize = 13.5.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = Color(0xFF090C16),
                                    platformStyle = noPadStyle
                                )
                            )
                        }
                    }
                }
            }

            // Expandable WeChat "+" Action Grid
            AnimatedVisibility(
                visible = showPlusDrawer,
                enter = fadeIn() + expandVertically(),
                exit = fadeOut() + shrinkVertically()
            ) {
                val isPool = activeModel == "free-pool"
                val row1 = listOf(
                    Triple("盘点资产", Icons.Default.Lightbulb) {
                        showPlusDrawer = false
                        onSendPrompt("请调用工具盘点全部数字资产与算力节点，并按到期风险给出锐评。")
                    },
                    Triple("哪吒脉搏", Icons.Default.Lightbulb) {
                        showPlusDrawer = false
                        onSendPrompt("请调用 query_nezha_nodes 检查哪吒监控面板节点的实时在线状态。")
                    },
                    Triple("检查调度", Icons.Default.Lightbulb) {
                        showPlusDrawer = false
                        onSendPrompt("请列出当前所有定时调度任务的状态与下次触发时间。")
                    },
                    Triple("串联收藏", Icons.Default.Lightbulb) {
                        showPlusDrawer = false
                        onSendPrompt("请调用 manage_stash 检索我最近收入收藏匣的未读干货并做简要串联。")
                    }
                )
                val row2 = listOf(
                    Triple(if (isPool) "引擎:免费" else "引擎:主力", Icons.Default.Tune) {
                        onModelChange(if (isPool) "deepseek-flash" else "free-pool")
                    },
                    Triple("拉人建群", Icons.Default.GroupAdd) {
                        showPlusDrawer = false
                        onInviteToGroup()
                    },
                    Triple("历史话题", Icons.Default.History) {
                        showPlusDrawer = false
                        onOpenSessions()
                    },
                    Triple("清空记录", Icons.Default.DeleteSweep) {
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
                    HorizontalDivider(thickness = 0.5.dp, color = Color(0x16FFF8EE))
                    listOf(row1, row2).forEach { rowItems ->
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.spacedBy(10.dp)
                        ) {
                            rowItems.forEach { (label, icon, action) ->
                                PlusTrayItem(
                                    label = label,
                                    icon = icon,
                                    onClick = action,
                                    modifier = Modifier.weight(1f)
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun PlusTrayItem(
    label: String,
    icon: ImageVector,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    Column(
        modifier = modifier.clickable(onClick = onClick),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(6.dp)
    ) {
        Surface(
            shape = RoundedCornerShape(16.dp),
            color = Color(0xFF111932),
            border = BorderStroke(1.dp, Color(0x22FFF8EE)),
            modifier = Modifier.size(52.dp)
        ) {
            Box(contentAlignment = Alignment.Center) {
                Icon(
                    icon,
                    contentDescription = label,
                    tint = Color(0xFFFFE2BE),
                    modifier = Modifier.size(22.dp)
                )
            }
        }
        Text(
            text = label,
            fontSize = 11.sp,
            color = Color(0xFF94A3B8),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis
        )
    }
}
