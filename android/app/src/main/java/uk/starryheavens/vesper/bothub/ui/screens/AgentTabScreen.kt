package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
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
import androidx.compose.material.icons.filled.ArrowUpward
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Stop
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
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.util.Calendar
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.ChatTurnUi
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.components.BotAvatarOrCluster
import uk.starryheavens.vesper.bothub.ui.components.RichMarkdownText

@Composable
fun AgentTabScreen(
    activePersona: String,
    activeModel: String,
    enableThinking: Boolean,
    bots: List<BotItem>,
    activeBotId: String,
    onSelectBot: (String) -> Unit,
    onPersonaChange: (String) -> Unit,
    onModelChange: (String) -> Unit,
    onThinkingChange: (Boolean) -> Unit,
    onOpenSessions: () -> Unit,
    onNewSession: () -> Unit,
    messages: List<ChatTurnUi>,
    inputText: String,
    onInputChange: (String) -> Unit,
    isStreaming: Boolean,
    onSend: () -> Unit,
    onStop: () -> Unit,
    onCopyText: (String) -> Unit,
    onQuickPrompt: (String) -> Unit,
    modifier: Modifier = Modifier
) {
    val haptics = LocalHapticFeedback.current
    val activeBot = bots.find { it.id == activeBotId }
    val listState = rememberLazyListState()
    var composerFocused by remember { mutableStateOf(false) }

    val greetingPrefix = remember {
        val h = Calendar.getInstance().get(Calendar.HOUR_OF_DAY)
        when {
            h < 5 -> "夜深了"
            h < 12 -> "早安"
            h < 18 -> "下午好"
            else -> "晚上好"
        }
    }
    val lastAssistantId = messages.lastOrNull { it.role != "user" }?.id

    // Smooth auto-scroll when a new turn starts
    LaunchedEffect(messages.size) {
        if (messages.isNotEmpty()) {
            listState.animateScrollToItem(messages.lastIndex)
        }
    }

    Column(modifier = modifier.fillMaxSize()) {
        // Compact Spatial Spirit Strip (only shown when multiple Bots exist)
        if (bots.size > 1) {
            LazyRow(
                contentPadding = PaddingValues(horizontal = 14.dp, vertical = 6.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.fillMaxWidth()
            ) {
                items(bots, key = { it.id }) { b ->
                    val isSelected = activeBotId == b.id
                    val isStreamingThisBot = isSelected && isStreaming
                    val liveState = b.liveState(isStreamingThisBot)

                    Surface(
                        onClick = {
                            if (!isSelected) {
                                haptics.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                onSelectBot(b.id)
                            }
                        },
                        shape = RoundedCornerShape(999.dp),
                        color = if (isSelected) Color(0x26F2B880) else Color(0x990D1326),
                        border = BorderStroke(
                            width = 1.dp,
                            color = if (isSelected) Color(0x88F2B880) else Color(0x1FFFF8EE)
                        ),
                        modifier = Modifier.sizeIn(minHeight = 40.dp)
                    ) {
                        Row(
                            modifier = Modifier.padding(start = 6.dp, end = 12.dp, top = 5.dp, bottom = 5.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(7.dp)
                        ) {
                            BotAvatarOrCluster(
                                bot = b,
                                allBots = bots,
                                state = liveState,
                                size = 26.dp,
                                defaultSeedKey = b.id
                            )
                            Text(
                                text = b.name,
                                style = MaterialTheme.typography.labelLarge,
                                fontSize = 12.5.sp,
                                fontWeight = if (isSelected) FontWeight.SemiBold else FontWeight.Medium,
                                color = if (isSelected) Color(0xFFFFE2BE) else Color(0xFFCBD5E1),
                                maxLines = 1
                            )
                            if (b.unreadCount > 0) {
                                Box(
                                    modifier = Modifier
                                        .background(Color(0xFFF2B880), CircleShape)
                                        .padding(horizontal = 5.dp, vertical = 1.dp)
                                ) {
                                    Text(
                                        text = b.unreadCount.toString(),
                                        fontSize = 9.5.sp,
                                        fontFamily = FontFamily.Monospace,
                                        fontWeight = FontWeight.Bold,
                                        color = Color(0xFF090C16)
                                    )
                                }
                            }
                        }
                    }
                }
            }
        } else if (bots.isEmpty()) {
            // Fallback persona strip when server has 0 persistent bots
            val personas = listOf(
                "grok" to "Grok 锐评",
                "butler" to "执更管家",
                "architect" to "架构师"
            )
            LazyRow(
                contentPadding = PaddingValues(horizontal = 14.dp, vertical = 6.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.fillMaxWidth()
            ) {
                items(personas) { (id, label) ->
                    val selected = activePersona == id
                    Surface(
                        onClick = { onPersonaChange(id) },
                        shape = RoundedCornerShape(999.dp),
                        color = if (selected) Color(0x26F2B880) else Color(0x990D1326),
                        border = BorderStroke(
                            1.dp,
                            if (selected) Color(0x88F2B880) else Color(0x1FFFF8EE)
                        ),
                        modifier = Modifier.sizeIn(minHeight = 36.dp)
                    ) {
                        Box(
                            contentAlignment = Alignment.Center,
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 6.dp)
                        ) {
                            Text(
                                text = label,
                                fontSize = 12.sp,
                                fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                                color = if (selected) Color(0xFFFFE2BE) else Color(0xFFCBD5E1)
                            )
                        }
                    }
                }
            }
        }

        // Conversation Stream
        LazyColumn(
            state = listState,
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth(),
            verticalArrangement = Arrangement.spacedBy(18.dp),
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 12.dp)
        ) {
            if (messages.isEmpty()) {
                item {
                    val starters = listOf(
                        "盘点资产与临期风险" to "请调用工具盘点全部数字资产与算力节点，并按到期风险给出锐评。",
                        "探查哪吒节点脉搏" to "请调用 query_nezha_nodes 检查哪吒监控面板节点的实时在线状态。",
                        "检查调度任务运行" to "请列出当前所有定时调度任务的状态与下次触发时间。",
                        "检索最近收藏干货" to "请调用 manage_stash 检索我最近收入收藏匣的未读干货并做简要串联。"
                    )
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(top = 24.dp, bottom = 12.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Box(
                                modifier = Modifier
                                    .size(124.dp)
                                    .background(
                                        brush = Brush.radialGradient(
                                            0.0f to Color(0x2EF2B880),
                                            0.65f to Color(0x0DF2B880),
                                            1.0f to Color.Transparent
                                        ),
                                        shape = CircleShape
                                    )
                            )
                            BotAvatarOrCluster(
                                bot = activeBot,
                                allBots = bots,
                                state = if (isStreaming) "working" else (activeBot?.state ?: "idle"),
                                size = 88.dp,
                                interactive = true,
                                isListening = inputText.isNotBlank(),
                                defaultSeedKey = "butler_stage"
                            )
                        }

                        Column(
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(4.dp)
                        ) {
                            Text(
                                text = "$greetingPrefix，今天我们先推进哪件事",
                                style = MaterialTheme.typography.titleLarge,
                                fontSize = 20.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFFFFF8EE),
                                textAlign = TextAlign.Center
                            )
                            Text(
                                text = buildString {
                                    append(activeBot?.name ?: "管家")
                                    val role = activeBot?.roleTitle.orEmpty()
                                    if (role.isNotBlank()) {
                                        append(" · ")
                                        append(role)
                                    } else {
                                        append(" · 实时探针、资产守望与收藏提炼随时待命")
                                    }
                                },
                                style = MaterialTheme.typography.bodySmall,
                                color = Color(0xFF94A3B8),
                                textAlign = TextAlign.Center,
                                modifier = Modifier.padding(horizontal = 20.dp)
                            )
                        }

                        Spacer(modifier = Modifier.height(6.dp))

                        Column(
                            modifier = Modifier.fillMaxWidth(),
                            verticalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            starters.chunked(2).forEach { pair ->
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    pair.forEach { (title, prompt) ->
                                        Surface(
                                            onClick = {
                                                haptics.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                                onQuickPrompt(prompt)
                                            },
                                            shape = RoundedCornerShape(16.dp),
                                            color = Color(0xB30D1326),
                                            border = BorderStroke(1.dp, Color(0x22FFF8EE)),
                                            modifier = Modifier
                                                .weight(1f)
                                                .sizeIn(minHeight = 52.dp)
                                        ) {
                                            Row(
                                                modifier = Modifier.padding(horizontal = 12.dp, vertical = 11.dp),
                                                verticalAlignment = Alignment.CenterVertically,
                                                horizontalArrangement = Arrangement.SpaceBetween
                                            ) {
                                                Text(
                                                    text = title,
                                                    style = MaterialTheme.typography.labelLarge,
                                                    fontSize = 12.5.sp,
                                                    fontWeight = FontWeight.Medium,
                                                    color = Color(0xFFE2E8F0),
                                                    maxLines = 2,
                                                    modifier = Modifier.weight(1f)
                                                )
                                                Icon(
                                                    Icons.Default.KeyboardArrowRight,
                                                    contentDescription = null,
                                                    tint = Color(0x99F2B880),
                                                    modifier = Modifier.size(15.dp)
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
                val isUser = msg.role == "user"
                var expandReasoning by remember { mutableStateOf(false) }

                if (isUser) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.End
                    ) {
                        Surface(
                            shape = RoundedCornerShape(
                                topStart = 20.dp,
                                topEnd = 6.dp,
                                bottomStart = 20.dp,
                                bottomEnd = 20.dp
                            ),
                            color = Color(0xFF211A16),
                            border = BorderStroke(1.dp, Color(0x4DF2B880)),
                            modifier = Modifier.widthIn(max = 308.dp)
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
                } else {
                    val isLiveTurn = isStreaming && msg.id == lastAssistantId
                    val avatarLiveState = when {
                        isLiveTurn && msg.content.isEmpty() -> "thinking"
                        isLiveTurn -> "working"
                        else -> "idle"
                    }

                    // Open Editorial Assistant Turn (zero outer card-in-card box)
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .animateContentSize(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                        verticalAlignment = Alignment.Top
                    ) {
                        GrokBotAvatar(
                            avatarStyle = msg.avatarStyle,
                            state = avatarLiveState,
                            size = 28.dp,
                            interactive = true,
                            seedKey = msg.id,
                            modifier = Modifier.padding(top = 2.dp)
                        )

                        Column(
                            modifier = Modifier.weight(1f),
                            verticalArrangement = Arrangement.spacedBy(7.dp)
                        ) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                                ) {
                                    Text(
                                        text = msg.persona.ifEmpty { activeBot?.name ?: "BotHub" },
                                        style = MaterialTheme.typography.labelLarge,
                                        fontSize = 13.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        color = Color(0xFFFFE2BE)
                                    )
                                    if (msg.model.isNotEmpty()) {
                                        Text(
                                            text = msg.model,
                                            fontFamily = FontFamily.Monospace,
                                            fontSize = 10.5.sp,
                                            color = Color(0xFF64748B),
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis,
                                            modifier = Modifier.widthIn(max = 128.dp)
                                        )
                                    }
                                }

                                if (msg.content.isNotEmpty() && !isLiveTurn) {
                                    Surface(
                                        onClick = { onCopyText(msg.content) },
                                        shape = RoundedCornerShape(999.dp),
                                        color = Color(0x800D1326),
                                        border = BorderStroke(1.dp, Color(0x1FFFF8EE))
                                    ) {
                                        Row(
                                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(4.dp)
                                        ) {
                                            Icon(
                                                Icons.Default.ContentCopy,
                                                contentDescription = "复制",
                                                modifier = Modifier.size(11.dp),
                                                tint = Color(0xFF94A3B8)
                                            )
                                            Text(
                                                text = "复制",
                                                fontSize = 10.5.sp,
                                                color = Color(0xFF94A3B8)
                                            )
                                        }
                                    }
                                }
                            }

                            if (msg.reasoning.isNotEmpty()) {
                                Surface(
                                    onClick = { expandReasoning = !expandReasoning },
                                    shape = RoundedCornerShape(14.dp),
                                    color = Color(0x990A0F1E),
                                    border = BorderStroke(1.dp, Color(0x26F2B880)),
                                    modifier = Modifier.fillMaxWidth()
                                ) {
                                    Column(modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp)) {
                                        Row(
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(4.dp)
                                        ) {
                                            Icon(
                                                if (expandReasoning) Icons.Default.KeyboardArrowDown else Icons.Default.KeyboardArrowRight,
                                                contentDescription = null,
                                                tint = Color(0xFFFBBF24),
                                                modifier = Modifier.size(15.dp)
                                            )
                                            Text(
                                                text = "思考过程 (${msg.reasoning.length} 字)",
                                                fontFamily = FontFamily.Monospace,
                                                fontSize = 11.5.sp,
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
                                                fontSize = 12.sp,
                                                lineHeight = 18.sp,
                                                color = Color(0xFF94A3B8),
                                                modifier = Modifier.padding(top = 6.dp)
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
                                            color = Color(0xCC0E162B),
                                            border = BorderStroke(1.dp, Color(0x3334D399))
                                        ) {
                                            Row(
                                                modifier = Modifier.padding(horizontal = 9.dp, vertical = 4.dp),
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
                                                    fontSize = 11.sp,
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
                                    modifier = Modifier.padding(vertical = 4.dp)
                                ) {
                                    Box(
                                        modifier = Modifier
                                            .size(6.dp)
                                            .background(Color(0xFFF2B880), CircleShape)
                                    )
                                    Text(
                                        text = "正在构思与调度...",
                                        fontSize = 12.5.sp,
                                        color = Color(0xFF94A3B8)
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }

        // Unified Obsidian-Champagne Glass Composer Dock (matches Web BotChatComposer 1:1)
        Surface(
            shape = RoundedCornerShape(24.dp),
            color = Color(0xEB0B1022),
            border = BorderStroke(
                width = 1.dp,
                color = if (composerFocused || isStreaming) Color(0x7AF2B880) else Color(0x26FFF8EE)
            ),
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 8.dp)
        ) {
            Column(
                modifier = Modifier.padding(start = 14.dp, end = 10.dp, top = 11.dp, bottom = 8.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                BasicTextField(
                    value = inputText,
                    onValueChange = onInputChange,
                    textStyle = TextStyle(
                        color = Color(0xFFFFF8EE),
                        fontSize = 15.sp,
                        lineHeight = 22.sp
                    ),
                    cursorBrush = SolidColor(Color(0xFFF2B880)),
                    maxLines = 5,
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 26.dp)
                        .onFocusChanged { composerFocused = it.isFocused },
                    decorationBox = { innerTextField ->
                        Box(contentAlignment = Alignment.CenterStart) {
                            if (inputText.isEmpty()) {
                                Text(
                                    text = "向 ${activeBot?.name ?: "管家"} 下达指令或粘贴链接...",
                                    fontSize = 14.5.sp,
                                    color = Color(0xFF64748B),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                            innerTextField()
                        }
                    }
                )

                // Inline Micro-Toolbar: Model Toggle + Thinking Toggle + Send/Stop Jewel Button
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        val isPool = activeModel == "free-pool"
                        Surface(
                            onClick = {
                                onModelChange(if (isPool) "deepseek-flash" else "free-pool")
                            },
                            shape = RoundedCornerShape(999.dp),
                            color = Color(0xFF131B33),
                            border = BorderStroke(1.dp, Color(0x26FFF8EE)),
                            modifier = Modifier.sizeIn(minHeight = 32.dp)
                        ) {
                            Box(
                                contentAlignment = Alignment.Center,
                                modifier = Modifier.padding(horizontal = 10.dp, vertical = 5.dp)
                            ) {
                                Text(
                                    text = if (isPool) "免费模型池" else "主力引擎",
                                    fontSize = 11.5.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = Color(0xFFCBD5E1)
                                )
                            }
                        }

                        Surface(
                            onClick = { onThinkingChange(!enableThinking) },
                            shape = RoundedCornerShape(999.dp),
                            color = if (enableThinking) Color(0x24F2B880) else Color(0xFF131B33),
                            border = BorderStroke(
                                1.dp,
                                if (enableThinking) Color(0x66F2B880) else Color(0x22FFF8EE)
                            ),
                            modifier = Modifier.sizeIn(minHeight = 32.dp)
                        ) {
                            Row(
                                modifier = Modifier.padding(horizontal = 10.dp, vertical = 5.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(5.dp)
                            ) {
                                Box(
                                    modifier = Modifier
                                        .size(6.dp)
                                        .background(
                                            color = if (enableThinking) Color(0xFFF2B880) else Color(0xFF64748B),
                                            shape = CircleShape
                                        )
                                )
                                Text(
                                    text = "思考",
                                    fontSize = 11.5.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = if (enableThinking) Color(0xFFFFE2BE) else Color(0xFF94A3B8)
                                )
                            }
                        }
                    }

                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        if (inputText.length > 24 && !isStreaming) {
                            Surface(
                                onClick = { onInputChange("") },
                                shape = RoundedCornerShape(999.dp),
                                color = Color.Transparent,
                                modifier = Modifier.sizeIn(minHeight = 32.dp)
                            ) {
                                Box(
                                    contentAlignment = Alignment.Center,
                                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp)
                                ) {
                                    Text(
                                        text = "清空",
                                        fontSize = 11.sp,
                                        color = Color(0xFF94A3B8)
                                    )
                                }
                            }
                        }

                        if (isStreaming) {
                            Surface(
                                onClick = onStop,
                                shape = CircleShape,
                                color = Color(0x33F43F5E),
                                border = BorderStroke(1.dp, Color(0x80FB7185)),
                                modifier = Modifier.size(38.dp)
                            ) {
                                Box(contentAlignment = Alignment.Center) {
                                    Icon(
                                        Icons.Default.Stop,
                                        contentDescription = "停止生成",
                                        tint = Color(0xFFFDA4AF),
                                        modifier = Modifier.size(18.dp)
                                    )
                                }
                            }
                        } else {
                            val canSend = inputText.isNotBlank()
                            Surface(
                                onClick = { if (canSend) onSend() },
                                enabled = canSend,
                                shape = CircleShape,
                                color = if (canSend) Color(0xFFF2B880) else Color(0xFF1A223B),
                                modifier = Modifier.size(38.dp)
                            ) {
                                Box(contentAlignment = Alignment.Center) {
                                    Icon(
                                        Icons.Default.ArrowUpward,
                                        contentDescription = "发送指令",
                                        tint = if (canSend) Color(0xFF090C16) else Color(0xFF475569),
                                        modifier = Modifier.size(18.dp)
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
