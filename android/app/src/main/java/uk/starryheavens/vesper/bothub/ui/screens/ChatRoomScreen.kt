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
import androidx.compose.material.icons.filled.FormatQuote
import androidx.compose.material.icons.filled.GroupAdd
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.MoreHoriz
import androidx.compose.material.icons.filled.Psychology
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Stop
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
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
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.ChatTurnUi
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.components.CrispTagPill
import uk.starryheavens.vesper.bothub.ui.components.FloatingGlassPopover
import uk.starryheavens.vesper.bothub.ui.components.GlassMenuAction
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.RichMarkdownText
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
    onClearHistory: () -> Unit,
    modifier: Modifier = Modifier
) {
    BackHandler(onBack = onBack)

    val haptics = LocalHapticFeedback.current
    val scope = rememberCoroutineScope()
    val listState = rememberLazyListState()
    var showPlusDrawer by remember { mutableStateOf(false) }
    var composerFocused by remember { mutableStateOf(false) }
    var quoteTarget by remember { mutableStateOf<Pair<String, String>?>(null) }
    var dismissAnnouncement by remember(activeBot?.id) { mutableStateOf(false) }

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
        // 1. Clean WeChat / QQ IM Chat Header
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
                        contentDescription = "返回",
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
                            color = Color(0xFFFFF8EE),
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
                                fontSize = 10.5.sp,
                                color = Color(0xFFFFE2BE),
                                platformStyle = NoFontPaddingStyle
                            ),
                            maxLines = 1,
                            softWrap = false
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
                        contentDescription = "聊天详情",
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
                CrispTagPill(
                    text = "群公告",
                    bgColor = Color(0x2CF2B880),
                    borderColor = Color(0x52F2B880),
                    textColor = Color(0xFFFFE2BE)
                )
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
                            color = Color(0xFFFFF8EE),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        val desc = activeBot?.traits?.ifBlank { activeBot.roleTitle }.orEmpty()
                        if (desc.isNotBlank()) {
                            Text(
                                text = desc,
                                fontSize = 12.5.sp,
                                color = Color(0xFF8C9BB5),
                                textAlign = TextAlign.Center,
                                maxLines = 2,
                                overflow = TextOverflow.Ellipsis
                            )
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
                                        label = "复制",
                                        icon = Icons.Default.ContentCopy,
                                        iconTint = Color(0xFFFFE2BE),
                                        onClick = { onCopyText(msg.content) }
                                    ),
                                    GlassMenuAction(
                                        label = "引用",
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
                                                color = Color(0xFF8C9BB5),
                                                maxLines = 1,
                                                softWrap = false,
                                                overflow = TextOverflow.Ellipsis,
                                                modifier = Modifier.weight(1f, fill = false)
                                            )
                                            if (msg.msgKind == "routine") {
                                                CrispTagPill(
                                                    text = "提醒",
                                                    bgColor = Color(0x28FBBF24),
                                                    borderColor = Color(0x52FBBF24),
                                                    textColor = Color(0xFFFDE68A)
                                                )
                                            }
                                        }
                                    }

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
                                                        text = "思考过程",
                                                        fontSize = 11.sp,
                                                        color = Color(0xFFFDE68A),
                                                        maxLines = 1,
                                                        softWrap = false
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
                                                            color = Color(0xFFA7F3D0),
                                                            maxLines = 1,
                                                            softWrap = false
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
                                                    text = "正在输入...",
                                                    fontSize = 12.5.sp,
                                                    color = Color(0xFF94A3B8)
                                                )
                                            }
                                        }
                                    }

                                    val hasTurnError = !isLiveTurn && (
                                        msg.content.startsWith("[请求异常]") ||
                                            msg.content.contains("\n[中断:")
                                        )

                                    if ((isLatestReply || hasTurnError) && msg.content.isNotEmpty()) {
                                        Row(
                                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                                            verticalAlignment = Alignment.CenterVertically,
                                            modifier = Modifier.padding(start = 4.dp, top = 1.dp)
                                        ) {
                                            if (hasTurnError) {
                                                val prevUserPrompt = remember(messages, msg.id) {
                                                    val idx = messages.indexOfFirst { it.id == msg.id }
                                                    if (idx > 0) {
                                                        messages.subList(0, idx).lastOrNull { it.role == "user" }?.content.orEmpty()
                                                    } else {
                                                        ""
                                                    }
                                                }
                                                if (prevUserPrompt.isNotBlank()) {
                                                    Row(
                                                        verticalAlignment = Alignment.CenterVertically,
                                                        horizontalArrangement = Arrangement.spacedBy(3.dp),
                                                        modifier = Modifier
                                                            .tactileClickable(pressedScale = 0.94f, hapticOnTap = true) {
                                                                onSendPrompt(prevUserPrompt)
                                                            }
                                                    ) {
                                                        Icon(
                                                            imageVector = Icons.Default.Refresh,
                                                            contentDescription = "重试",
                                                            tint = Color(0xFFF2B880),
                                                            modifier = Modifier.size(12.dp)
                                                        )
                                                        Text(
                                                            text = "重试",
                                                            fontSize = 10.5.sp,
                                                            fontWeight = FontWeight.Medium,
                                                            color = Color(0xFFF2B880),
                                                            maxLines = 1,
                                                            softWrap = false
                                                        )
                                                    }
                                                }
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
                                                    tint = Color(0xFF5D6D89),
                                                    modifier = Modifier.size(11.dp)
                                                )
                                                Text(
                                                    text = "复制",
                                                    fontSize = 10.5.sp,
                                                    color = Color(0xFF5D6D89),
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
                                                    tint = Color(0xFF5D6D89),
                                                    modifier = Modifier.size(12.dp)
                                                )
                                                Text(
                                                    text = "引用",
                                                    fontSize = 10.5.sp,
                                                    color = Color(0xFF5D6D89),
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
                                            label = "复制",
                                            icon = Icons.Default.ContentCopy,
                                            iconTint = Color(0xFFFFE2BE),
                                            onClick = { onCopyText(msg.content) }
                                        ),
                                        GlassMenuAction(
                                            label = "引用",
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
                        .shadow(8.dp, pillShape, ambientColor = Color(0xAA000000))
                        .tactileClickable(pressedScale = 0.92f, hapticOnTap = true) {
                            scope.launch {
                                if (messages.isNotEmpty()) {
                                    listState.animateScrollToItem(messages.lastIndex)
                                }
                            }
                        }
                        .clip(pillShape)
                        .background(
                            Brush.verticalGradient(listOf(Color(0xF2192444), Color(0xF20D1326))),
                            pillShape
                        )
                        .border(
                            BorderStroke(
                                0.85.dp,
                                specularRimBrush(Color(0x78F2B880), Color(0x2CF2B880), Color.Transparent)
                            ),
                            pillShape
                        )
                        .padding(horizontal = 12.dp, vertical = 7.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    Icon(
                        imageVector = Icons.Default.KeyboardArrowDown,
                        contentDescription = "回到底部",
                        tint = Color(0xFFFFE2BE),
                        modifier = Modifier.size(15.dp)
                    )
                    Text(
                        text = if (isStreaming) "新回复" else "回到底部",
                        style = TextStyle(
                            fontSize = 11.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFF8EE),
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
                            color = Color(0xFFCBD5E1),
                            maxLines = 1,
                            softWrap = false
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
                    text = "$sender: ${snippet.replace('\n', ' ')}",
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

        // 6. Clean Bottom Composer + 1-Row 4-Tile IM "+" Toolbar
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
                                        text = "发消息...",
                                        fontSize = 14.sp,
                                        color = Color(0xFF5A6A85),
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
                        contentDescription = "更多功能",
                        tint = if (showPlusDrawer) Color(0xFFFFE2BE) else Color(0xFFCBD5E1),
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

            // Pure 1-Row 4-Tile Native Chat Controls (Zero hardcoded personal prompts!)
            AnimatedVisibility(
                visible = showPlusDrawer,
                enter = fadeIn() + expandVertically(),
                exit = fadeOut() + shrinkVertically()
            ) {
                val isPool = activeModel == "free-pool"
                val tools = listOf(
                    PlusActionTile(
                        label = if (enableThinking) "思考:开" else "思考:关",
                        icon = Icons.Default.Psychology,
                        accent = if (enableThinking) Color(0xFFF2B880) else Color(0xFF8C9BB5)
                    ) {
                        onThinkingChange(!enableThinking)
                    },
                    PlusActionTile(
                        label = if (isPool) "免费模型" else "主力模型",
                        icon = Icons.Default.AutoAwesome,
                        accent = Color(0xFFC084FC)
                    ) {
                        onModelChange(if (isPool) "deepseek-flash" else "free-pool")
                    },
                    PlusActionTile(
                        label = "拉人建群",
                        icon = Icons.Default.GroupAdd,
                        accent = Color(0xFF38BDF8)
                    ) {
                        showPlusDrawer = false
                        onInviteToGroup()
                    },
                    PlusActionTile(
                        label = "清空记录",
                        icon = Icons.Default.DeleteSweep,
                        accent = Color(0xFFFB7185)
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
                    HorizontalDivider(thickness = 0.5.dp, color = Color(0x14FFF8EE))
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        tools.forEach { tile ->
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
            softWrap = false,
            overflow = TextOverflow.Ellipsis
        )
    }
}
