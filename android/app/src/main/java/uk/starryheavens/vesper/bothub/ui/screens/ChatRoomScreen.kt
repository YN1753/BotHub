package uk.starryheavens.vesper.bothub.ui.screens

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.interaction.MutableInteractionSource
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
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AccountBalanceWallet
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material.icons.filled.Bookmarks
import androidx.compose.material.icons.filled.ChevronLeft
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.DeleteSweep
import androidx.compose.material.icons.filled.FormatQuote
import androidx.compose.material.icons.filled.GroupAdd
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.MonitorHeart
import androidx.compose.material.icons.filled.MoreHoriz
import androidx.compose.material.icons.filled.Psychology
import androidx.compose.material.icons.filled.Schedule
import androidx.compose.material.icons.filled.Stop
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.ChatTurnUi
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.components.FloatingGlassPopover
import uk.starryheavens.vesper.bothub.ui.components.GlassMenuAction
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.RichMarkdownText
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.SpecularHairline
import uk.starryheavens.vesper.bothub.ui.components.SpiritSquircleAvatar
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.specularRimBrush
import uk.starryheavens.vesper.bothub.ui.components.tactileClickable

private data class PlusActionTile(
    val label: String,
    val icon: ImageVector,
    val accent: Color,
    val onClick: () -> Unit
)

/**
 * Full-Screen Level-2 Flagship IM Chat Room (WeChat / QQ / GrokBot Chat Paradigm).
 */
@OptIn(ExperimentalFoundationApi::class)
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
    val listState = rememberLazyListState()
    var showPlusDrawer by remember { mutableStateOf(false) }
    var composerFocused by remember { mutableStateOf(false) }
    var quoteTarget by remember { mutableStateOf<Pair<String, String>?>(null) }
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
                .background(
                    Brush.verticalGradient(
                        listOf(Color(0xF8060913), Color(0xEB080C1A))
                    )
                )
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
                            platformStyle = NoFontPaddingStyle
                        )
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
                            fontWeight = FontWeight.Bold,
                            color = Color(0xFFFFF8EE),
                            platformStyle = NoFontPaddingStyle
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
                                .size(5.5.dp)
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
                                color = if (isStreaming) Color(0xFFFFE2BE) else Color(0xFF8C9BB5),
                                platformStyle = NoFontPaddingStyle
                            ),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }

                Box(
                    modifier = Modifier
                        .size(36.dp)
                        .tactileClickable(pressedScale = 0.92f, hapticOnTap = true, onClick = onOpenChatInfo)
                        .clip(CircleShape)
                        .background(
                            Brush.verticalGradient(listOf(Color(0xFF16203C), Color(0xFF0C1224))),
                            CircleShape
                        )
                        .border(
                            BorderStroke(
                                0.85.dp,
                                specularRimBrush(Color(0x33FFF8EE), Color(0x14FFF8EE), Color(0x06FFF8EE))
                            ),
                            CircleShape
                        ),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = Icons.Default.MoreHoriz,
                        contentDescription = "聊天信息与设置",
                        tint = Color(0xFFFFF8EE),
                        modifier = Modifier.size(19.dp)
                    )
                }
            }

            SpecularHairline()
        }

        // 2. Pinned Group Announcement Banner
        if (isGroup && !dismissAnnouncement && !activeBot?.announcement.isNullOrBlank()) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Color(0xEB0C1327))
                    .padding(horizontal = 14.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Box(
                    modifier = Modifier
                        .clip(RoundedCornerShape(4.dp))
                        .background(Color(0x2CF2B880))
                        .padding(horizontal = 6.dp, vertical = 2.dp)
                ) {
                    Text(
                        text = "群公告",
                        style = TextStyle(
                            fontSize = 10.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFE2BE),
                            platformStyle = NoFontPaddingStyle
                        )
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
                    imageVector = Icons.Default.Close,
                    contentDescription = "收起公告",
                    tint = Color(0xFF64748B),
                    modifier = Modifier
                        .size(16.dp)
                        .clickable { dismissAnnouncement = true }
                )
            }
            HorizontalDivider(thickness = 0.5.dp, color = Color(0x14FFF8EE))
        }

        // 3. IM Message Bubble Stream
        LazyColumn(
            state = listState,
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .clickable(
                    interactionSource = remember { MutableInteractionSource() },
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
                        Triple("盘点资产风险", Icons.Default.AccountBalanceWallet, "请调用工具盘点全部数字资产与算力节点，并按到期风险给出锐评。"),
                        Triple("探查节点脉搏", Icons.Default.MonitorHeart, "请调用 query_nezha_nodes 检查哪吒监控面板节点的实时在线状态。"),
                        Triple("检查定时调度", Icons.Default.Schedule, "请列出当前所有定时调度任务的状态与下次触发时间。"),
                        Triple("串联近期收藏", Icons.Default.Bookmarks, "请调用 manage_stash 检索我最近收入收藏匣的未读干货并做简要串联。")
                    )
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 6.dp, vertical = 16.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        Box(
                            modifier = Modifier
                                .clip(RoundedCornerShape(999.dp))
                                .background(Color(0x990D1428))
                                .border(0.6.dp, Color(0x18FFF8EE), RoundedCornerShape(999.dp))
                                .padding(horizontal = 14.dp, vertical = 5.dp)
                        ) {
                            Text(
                                text = if (isGroup) {
                                    "群聊已就绪，输入 @ 可点名指定 AI 成员发言"
                                } else {
                                    "你已连接 ${activeBot?.name ?: "AI 好友"}，现在可以开始聊天了"
                                },
                                fontSize = 11.5.sp,
                                color = Color(0xFF8C9BB5)
                            )
                        }

                        SpecularGlassCard(
                            cornerRadius = 22.dp,
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            Column(
                                modifier = Modifier.padding(18.dp),
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
                                    fontSize = 17.5.sp,
                                    fontWeight = FontWeight.Bold,
                                    color = Color(0xFFFFF8EE)
                                )
                                Text(
                                    text = activeBot?.traits?.ifBlank { activeBot.roleTitle }
                                        ?.ifBlank { "随时在线待命，直接发消息聊聊或点下方快捷话题" }
                                        ?: "随时在线待命",
                                    fontSize = 12.5.sp,
                                    color = Color(0xFF8C9BB5),
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
                                        pair.forEach { (label, icon, prompt) ->
                                            val tileShape = RoundedCornerShape(13.dp)
                                            Row(
                                                modifier = Modifier
                                                    .weight(1f)
                                                    .fillMaxHeight()
                                                    .sizeIn(minHeight = 44.dp)
                                                    .tactileClickable(
                                                        pressedScale = 0.96f,
                                                        hapticOnTap = true,
                                                        onClick = { onSendPrompt(prompt) }
                                                    )
                                                    .clip(tileShape)
                                                    .background(
                                                        Brush.verticalGradient(
                                                            listOf(Color(0xFF151E39), Color(0xFF0D1326))
                                                        ),
                                                        tileShape
                                                    )
                                                    .border(
                                                        BorderStroke(
                                                            0.8.dp,
                                                            specularRimBrush(Color(0x2EFFF8EE), Color(0x12FFF8EE), Color.Transparent)
                                                        ),
                                                        tileShape
                                                    )
                                                    .padding(horizontal = 10.dp, vertical = 10.dp),
                                                verticalAlignment = Alignment.CenterVertically,
                                                horizontalArrangement = Arrangement.spacedBy(6.dp)
                                            ) {
                                                Icon(
                                                    imageVector = icon,
                                                    contentDescription = null,
                                                    tint = Color(0xFFF2B880),
                                                    modifier = Modifier.size(14.dp)
                                                )
                                                Text(
                                                    text = label,
                                                    fontSize = 12.sp,
                                                    fontWeight = FontWeight.Medium,
                                                    color = Color(0xFFE6ECF8),
                                                    maxLines = 1,
                                                    overflow = TextOverflow.Ellipsis,
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
                                    .background(Color(0x990D1428))
                                    .padding(horizontal = 12.dp, vertical = 5.dp)
                            ) {
                                Text(
                                    text = msg.content,
                                    fontSize = 11.5.sp,
                                    lineHeight = 16.sp,
                                    color = Color(0xFF8C9BB5),
                                    textAlign = TextAlign.Center
                                )
                            }
                        }
                    }

                    "user" -> {
                        var showUserMenu by remember { mutableStateOf(false) }
                        val userShape = RoundedCornerShape(
                            topStart = 18.dp,
                            topEnd = 4.dp,
                            bottomStart = 18.dp,
                            bottomEnd = 18.dp
                        )

                        Box(
                            modifier = Modifier.fillMaxWidth(),
                            contentAlignment = Alignment.CenterEnd
                        ) {
                            Box(
                                modifier = Modifier
                                    .widthIn(max = 304.dp)
                                    .shadow(4.dp, userShape, ambientColor = Color(0x66000000))
                                    .clip(userShape)
                                    .background(
                                        Brush.verticalGradient(
                                            listOf(Color(0xFF3A281C), Color(0xFF241812))
                                        ),
                                        userShape
                                    )
                                    .border(
                                        BorderStroke(
                                            0.85.dp,
                                            specularRimBrush(
                                                Color(0x78F2B880),
                                                Color(0x2CF2B880),
                                                Color(0x12F2B880)
                                            )
                                        ),
                                        userShape
                                    )
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
                                    color = Color(0xFFFFF8EE)
                                )
                            }

                            FloatingGlassPopover(
                                expanded = showUserMenu,
                                onDismissRequest = { showUserMenu = false },
                                alignment = Alignment.TopEnd,
                                offsetY = 38.dp,
                                items = listOf(
                                    GlassMenuAction(
                                        label = "复制消息",
                                        icon = Icons.Default.ContentCopy,
                                        iconTint = Color(0xFFFFE2BE),
                                        onClick = { onCopyText(msg.content) }
                                    ),
                                    GlassMenuAction(
                                        label = "引用该条",
                                        icon = Icons.Default.FormatQuote,
                                        iconTint = Color(0xFFA5B4FC),
                                        onClick = { quoteTarget = "我" to msg.content }
                                    )
                                )
                            )
                        }
                    }

                    else -> {
                        var expandReasoning by remember { mutableStateOf(false) }
                        var showBubbleMenu by remember { mutableStateOf(false) }
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
                            topEnd = 18.dp,
                            bottomStart = 18.dp,
                            bottomEnd = 18.dp
                        )

                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .animateContentSize(),
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
                                                color = Color(0xFF8C9BB5)
                                            )
                                            if (msg.msgKind == "routine") {
                                                Box(
                                                    modifier = Modifier
                                                        .clip(RoundedCornerShape(4.dp))
                                                        .background(Color(0x28FBBF24))
                                                        .padding(horizontal = 5.dp, vertical = 1.dp)
                                                ) {
                                                    Text(
                                                        text = "异常提醒",
                                                        style = TextStyle(
                                                            fontSize = 9.5.sp,
                                                            color = Color(0xFFFDE68A),
                                                            platformStyle = NoFontPaddingStyle
                                                        )
                                                    )
                                                }
                                            }
                                        }
                                    }

                                    // Specular Glass AI Speech Bubble (Long-Press for Copy / Quote!)
                                    Column(
                                        modifier = Modifier
                                            .fillMaxWidth()
                                            .shadow(5.dp, aiShape, ambientColor = Color(0x77000000))
                                            .clip(aiShape)
                                            .background(
                                                Brush.verticalGradient(
                                                    listOf(Color(0xF0141D37), Color(0xF00B1021))
                                                ),
                                                aiShape
                                            )
                                            .border(
                                                BorderStroke(
                                                    0.85.dp,
                                                    specularRimBrush(
                                                        Color(0x36FFF8EE),
                                                        Color(0x14FFF8EE),
                                                        Color(0x06FFF8EE)
                                                    )
                                                ),
                                                aiShape
                                            )
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
                                        if (msg.reasoning.isNotEmpty()) {
                                            val rShape = RoundedCornerShape(10.dp)
                                            Column(
                                                modifier = Modifier
                                                    .fillMaxWidth()
                                                    .clip(rShape)
                                                    .background(Color(0xA6070B16), rShape)
                                                    .border(0.7.dp, Color(0x2CF2B880), rShape)
                                                    .clickable { expandReasoning = !expandReasoning }
                                                    .padding(horizontal = 10.dp, vertical = 6.dp)
                                            ) {
                                                Row(
                                                    verticalAlignment = Alignment.CenterVertically,
                                                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                                                ) {
                                                    Icon(
                                                        imageVector = if (expandReasoning) Icons.Default.KeyboardArrowDown else Icons.Default.KeyboardArrowRight,
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

                                        if (msg.tools.isNotEmpty()) {
                                            LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                                items(msg.tools) { tLabel ->
                                                    val tShape = RoundedCornerShape(999.dp)
                                                    Row(
                                                        modifier = Modifier
                                                            .clip(tShape)
                                                            .background(Color(0xCC091322), tShape)
                                                            .border(0.7.dp, Color(0x3D34D399), tShape)
                                                            .padding(horizontal = 8.dp, vertical = 3.5.dp),
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

                                    // Clean 1-tap micro pill ONLY on the latest reply (long-press works on ALL bubbles)
                                    if (isLatestReply && msg.content.isNotEmpty()) {
                                        Row(
                                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                                            verticalAlignment = Alignment.CenterVertically,
                                            modifier = Modifier.padding(start = 4.dp, top = 1.dp)
                                        ) {
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
                                                    tint = Color(0xFF5D6D89),
                                                    modifier = Modifier.size(11.dp)
                                                )
                                                Text(
                                                    text = "复制",
                                                    fontSize = 10.5.sp,
                                                    color = Color(0xFF5D6D89)
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
                                                    contentDescription = "引用回复",
                                                    tint = Color(0xFF5D6D89),
                                                    modifier = Modifier.size(12.dp)
                                                )
                                                Text(
                                                    text = "引用",
                                                    fontSize = 10.5.sp,
                                                    color = Color(0xFF5D6D89)
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
                                            label = "复制消息内容",
                                            icon = Icons.Default.ContentCopy,
                                            iconTint = Color(0xFFFFE2BE),
                                            onClick = { onCopyText(msg.content) }
                                        ),
                                        GlassMenuAction(
                                            label = "引用并回复",
                                            icon = Icons.Default.FormatQuote,
                                            iconTint = Color(0xFFA5B4FC),
                                            onClick = {
                                                val sender = msg.persona.ifEmpty { speakerBot?.name ?: "AI" }
                                                quoteTarget = sender to msg.content
                                            }
                                        )
                                    )
                                )
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
                            .background(Color(0xFF131B34), chipShape)
                            .border(0.7.dp, Color(0x28FFF8EE), chipShape)
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
                            color = Color(0xFFCBD5E1)
                        )
                    }
                }
            }
        }

        // 5. Quoted Reply Preview Strip
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
                    imageVector = Icons.Default.Close,
                    contentDescription = "取消引用",
                    tint = Color(0xFF94A3B8),
                    modifier = Modifier
                        .size(16.dp)
                        .clickable { quoteTarget = null }
                )
            }
        }

        // 6. Flagship Bottom Composer + Duotone "+" Action Tray
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(
                    Brush.verticalGradient(
                        listOf(Color(0xF5090E1E), Color(0xFA050811))
                    )
                )
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
                // Thinking Mode Toggle Circle
                Box(
                    modifier = Modifier
                        .size(40.dp)
                        .tactileClickable(pressedScale = 0.90f, hapticOnTap = true) {
                            onThinkingChange(!enableThinking)
                        }
                        .clip(CircleShape)
                        .background(
                            Brush.verticalGradient(
                                if (enableThinking) {
                                    listOf(Color(0x36F2B880), Color(0x1CC88A4E))
                                } else {
                                    listOf(Color(0xFF151E38), Color(0xFF0C1224))
                                }
                            ),
                            CircleShape
                        )
                        .border(
                            BorderStroke(
                                0.85.dp,
                                if (enableThinking) {
                                    specularRimBrush(Color(0x80F2B880), Color(0x33F2B880), Color.Transparent)
                                } else {
                                    specularRimBrush(Color(0x2EFFF8EE), Color(0x12FFF8EE), Color.Transparent)
                                }
                            ),
                            CircleShape
                        ),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = Icons.Default.Psychology,
                        contentDescription = if (enableThinking) "深度思考已开启" else "深度思考已关闭",
                        tint = if (enableThinking) Color(0xFFFFE2BE) else Color(0xFF7B8BA6),
                        modifier = Modifier.size(20.dp)
                    )
                }

                // Recessed Rounded Message Input Field
                val inputShape = RoundedCornerShape(20.dp)
                val inputBorder by animateColorAsState(
                    targetValue = if (composerFocused) Color(0x80F2B880) else Color(0x22FFF8EE),
                    animationSpec = tween(150),
                    label = "composerBorder"
                )
                Box(
                    modifier = Modifier
                        .weight(1f)
                        .sizeIn(minHeight = 40.dp)
                        .clip(inputShape)
                        .background(
                            Brush.verticalGradient(listOf(Color(0xF2070B16), Color(0xF20C1224))),
                            inputShape
                        )
                        .border(0.9.dp, inputBorder, inputShape),
                    contentAlignment = Alignment.CenterStart
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
                                        color = Color(0xFF5A6A85),
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
                Box(
                    modifier = Modifier
                        .size(40.dp)
                        .tactileClickable(pressedScale = 0.90f, hapticOnTap = true) {
                            showPlusDrawer = !showPlusDrawer
                        }
                        .clip(CircleShape)
                        .background(
                            Brush.verticalGradient(
                                if (showPlusDrawer) {
                                    listOf(Color(0x36F2B880), Color(0x1CC88A4E))
                                } else {
                                    listOf(Color(0xFF151E38), Color(0xFF0C1224))
                                }
                            ),
                            CircleShape
                        )
                        .border(
                            BorderStroke(
                                0.85.dp,
                                if (showPlusDrawer) {
                                    specularRimBrush(Color(0x80F2B880), Color(0x33F2B880), Color.Transparent)
                                } else {
                                    specularRimBrush(Color(0x2EFFF8EE), Color(0x12FFF8EE), Color.Transparent)
                                }
                            ),
                            CircleShape
                        ),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = if (showPlusDrawer) Icons.Default.Close else Icons.Default.Add,
                        contentDescription = "展开快捷功能",
                        tint = if (showPlusDrawer) Color(0xFFFFE2BE) else Color(0xFFCBD5E1),
                        modifier = Modifier.size(20.dp)
                    )
                }

                // Jewel Send Pill or Stop Button
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

            // Expandable WeChat "+" Duotone Action Grid (8 Distinct Icons!)
            AnimatedVisibility(
                visible = showPlusDrawer,
                enter = fadeIn() + expandVertically(),
                exit = fadeOut() + shrinkVertically()
            ) {
                val isPool = activeModel == "free-pool"
                val row1 = listOf(
                    PlusActionTile("盘点资产", Icons.Default.AccountBalanceWallet, Color(0xFFF2B880)) {
                        showPlusDrawer = false
                        onSendPrompt("请调用工具盘点全部数字资产与算力节点，并按到期风险给出锐评。")
                    },
                    PlusActionTile("哪吒脉搏", Icons.Default.MonitorHeart, Color(0xFF34D399)) {
                        showPlusDrawer = false
                        onSendPrompt("请调用 query_nezha_nodes 检查哪吒监控面板节点的实时在线状态。")
                    },
                    PlusActionTile("检查调度", Icons.Default.Schedule, Color(0xFF818CF8)) {
                        showPlusDrawer = false
                        onSendPrompt("请列出当前所有定时调度任务的状态与下次触发时间。")
                    },
                    PlusActionTile("串联收藏", Icons.Default.Bookmarks, Color(0xFFFBBF24)) {
                        showPlusDrawer = false
                        onSendPrompt("请调用 manage_stash 检索我最近收入收藏匣的未读干货并做简要串联。")
                    }
                )
                val row2 = listOf(
                    PlusActionTile(
                        if (isPool) "引擎:免费" else "引擎:主力",
                        Icons.Default.AutoAwesome,
                        Color(0xFFC084FC)
                    ) {
                        onModelChange(if (isPool) "deepseek-flash" else "free-pool")
                    },
                    PlusActionTile("拉人建群", Icons.Default.GroupAdd, Color(0xFF38BDF8)) {
                        showPlusDrawer = false
                        onInviteToGroup()
                    },
                    PlusActionTile("历史话题", Icons.Default.History, Color(0xFFCBD5E1)) {
                        showPlusDrawer = false
                        onOpenSessions()
                    },
                    PlusActionTile("清空记录", Icons.Default.DeleteSweep, Color(0xFFFB7185)) {
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
                    HorizontalDivider(thickness = 0.5.dp, color = Color(0x14FFF8EE))
                    listOf(row1, row2).forEach { rowItems ->
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.spacedBy(10.dp)
                        ) {
                            rowItems.forEach { tile ->
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
}

@Composable
private fun PlusTrayItem(
    tile: PlusActionTile,
    modifier: Modifier = Modifier
) {
    val sqShape = RoundedCornerShape(16.dp)
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
                .size(54.dp)
                .shadow(4.dp, sqShape, ambientColor = Color(0x66000000))
                .clip(sqShape)
                .background(
                    Brush.verticalGradient(
                        listOf(
                            Color(
                                red = (0x14 / 255f * 0.8f + tile.accent.red * 0.2f).coerceIn(0f, 1f),
                                green = (0x1D / 255f * 0.8f + tile.accent.green * 0.2f).coerceIn(0f, 1f),
                                blue = (0x36 / 255f * 0.8f + tile.accent.blue * 0.2f).coerceIn(0f, 1f),
                                alpha = 1f
                            ),
                            Color(0xFF0C1224)
                        )
                    ),
                    sqShape
                )
                .border(
                    BorderStroke(
                        0.85.dp,
                        specularRimBrush(
                            tile.accent.copy(alpha = 0.50f),
                            tile.accent.copy(alpha = 0.16f),
                            Color(0x08FFFFFF)
                        )
                    ),
                    sqShape
                ),
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
                color = Color(0xFF94A3B8),
                platformStyle = NoFontPaddingStyle
            ),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis
        )
    }
}
