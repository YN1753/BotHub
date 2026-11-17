package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.animation.animateContentSize
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.Send
import androidx.compose.material.icons.filled.Stop
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import java.util.Calendar
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.ChatTurnUi
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.components.BotAvatarOrCluster

@OptIn(ExperimentalMaterial3Api::class)
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
    val greetingPrefix = remember {
        val h = Calendar.getInstance().get(Calendar.HOUR_OF_DAY)
        when {
            h < 5 -> "深夜好"
            h < 12 -> "早安"
            h < 18 -> "午安"
            else -> "晚上好"
        }
    }
    val lastAssistantId = messages.lastOrNull { it.role != "user" }?.id

    Column(modifier = modifier.padding(horizontal = 14.dp, vertical = 6.dp)) {
        // Living GrokBot Team & Group Roster Cards
        if (bots.isNotEmpty()) {
            LazyRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.fillMaxWidth()
            ) {
                items(bots, key = { it.id }) { b ->
                    val isSelected = activeBotId == b.id
                    val isStreamingThisBot = isSelected && isStreaming
                    val liveState = b.liveState(isStreamingThisBot)
                    val statusLabel = b.statusLabel(isStreamingThisBot)

                    Surface(
                        onClick = {
                            haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                            onSelectBot(b.id)
                        },
                        shape = RoundedCornerShape(18.dp),
                        color = if (isSelected) {
                            MaterialTheme.colorScheme.secondaryContainer
                        } else {
                            MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.72f)
                        },
                        border = BorderStroke(
                            width = 1.dp,
                            color = if (isSelected) {
                                MaterialTheme.colorScheme.primary.copy(alpha = 0.68f)
                            } else {
                                MaterialTheme.colorScheme.outline.copy(alpha = 0.42f)
                            }
                        ),
                        modifier = Modifier.sizeIn(minHeight = 52.dp)
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 10.dp, vertical = 7.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            BotAvatarOrCluster(
                                bot = b,
                                allBots = bots,
                                state = liveState,
                                size = 32.dp,
                                defaultSeedKey = b.id
                            )
                            Column {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(5.dp)
                                ) {
                                    Text(
                                        text = b.name,
                                        style = MaterialTheme.typography.labelLarge,
                                        fontWeight = if (isSelected) FontWeight.SemiBold else FontWeight.Medium,
                                        color = MaterialTheme.colorScheme.onSurface,
                                        maxLines = 1
                                    )
                                    if (b.unreadCount > 0) {
                                        Box(
                                            modifier = Modifier
                                                .background(
                                                    color = MaterialTheme.colorScheme.primary,
                                                    shape = RoundedCornerShape(8.dp)
                                                )
                                                .padding(horizontal = 5.dp, vertical = 1.dp)
                                        ) {
                                            Text(
                                                text = b.unreadCount.toString(),
                                                style = MaterialTheme.typography.labelSmall,
                                                fontWeight = FontWeight.Bold,
                                                color = MaterialTheme.colorScheme.onPrimary
                                            )
                                        }
                                    }
                                }
                                Text(
                                    text = statusLabel,
                                    style = MaterialTheme.typography.labelSmall,
                                    color = if (liveState == "working" || liveState == "thinking") {
                                        MaterialTheme.colorScheme.primary
                                    } else {
                                        MaterialTheme.colorScheme.onSurfaceVariant
                                    },
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                    modifier = Modifier.widthIn(max = 96.dp)
                                )
                            }
                        }
                    }
                }
            }
            Spacer(modifier = Modifier.height(6.dp))
        }

        // Streamlined Control Chips
        LazyRow(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth()
        ) {
            if (bots.isEmpty()) {
                val personas = listOf(
                    "grok" to "Grok 锐评",
                    "butler" to "执更管家",
                    "architect" to "架构师"
                )
                items(personas) { (id, label) ->
                    FilterChip(
                        selected = activePersona == id,
                        onClick = { onPersonaChange(id) },
                        label = { Text(label) },
                        modifier = Modifier.sizeIn(minHeight = 44.dp)
                    )
                }
            }
            val models = listOf(
                "deepseek-flash" to "主力引擎",
                "free-pool" to "免费模型池"
            )
            items(models) { (id, label) ->
                FilterChip(
                    selected = activeModel == id,
                    onClick = { onModelChange(id) },
                    label = { Text(label) },
                    modifier = Modifier.sizeIn(minHeight = 44.dp)
                )
            }
            item {
                FilterChip(
                    selected = enableThinking,
                    onClick = { onThinkingChange(!enableThinking) },
                    label = { Text(if (enableThinking) "思考: 开" else "思考: 关") },
                    modifier = Modifier.sizeIn(minHeight = 44.dp)
                )
            }
            item {
                AssistChip(
                    onClick = onOpenSessions,
                    label = { Text("历史会话") },
                    modifier = Modifier.sizeIn(minHeight = 44.dp)
                )
            }
            item {
                AssistChip(
                    onClick = onNewSession,
                    label = { Text("+ 新对话") },
                    modifier = Modifier.sizeIn(minHeight = 44.dp)
                )
            }
        }

        Spacer(modifier = Modifier.height(6.dp))

        LazyColumn(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth(),
            verticalArrangement = Arrangement.spacedBy(14.dp),
            contentPadding = PaddingValues(vertical = 8.dp)
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
                            .padding(top = 18.dp, bottom = 10.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        BotAvatarOrCluster(
                            bot = activeBot,
                            allBots = bots,
                            state = if (isStreaming) "working" else (activeBot?.state ?: "idle"),
                            size = 96.dp,
                            interactive = true,
                            isListening = inputText.isNotBlank(),
                            defaultSeedKey = "butler_stage"
                        )

                        Surface(
                            shape = RoundedCornerShape(999.dp),
                            color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.75f),
                            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.4f))
                        ) {
                            Text(
                                text = if (activeBot?.isGroup == true) {
                                    "轻触后排团子切换 C 位 · 轻触主位触发特技"
                                } else {
                                    "轻触精灵触发跳跃 / 空翻 / 律动 / 害羞"
                                },
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.primary,
                                modifier = Modifier.padding(horizontal = 12.dp, vertical = 4.dp)
                            )
                        }

                        Text(
                            text = "$greetingPrefix，我是${activeBot?.name ?: "长庚管家"}",
                            style = MaterialTheme.typography.titleLarge,
                            fontWeight = FontWeight.Bold,
                            textAlign = TextAlign.Center
                        )

                        Text(
                            text = activeBot?.roleTitle?.ifEmpty {
                                "支持流式对话、哪吒探针直连、资产任务管控与系统分享秒收提炼"
                            } ?: "支持流式对话、哪吒探针直连、资产任务管控与系统分享秒收提炼",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            textAlign = TextAlign.Center,
                            modifier = Modifier.padding(horizontal = 16.dp)
                        )

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
                                                haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                                                onQuickPrompt(prompt)
                                            },
                                            shape = RoundedCornerShape(14.dp),
                                            color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.72f),
                                            border = BorderStroke(
                                                1.dp,
                                                MaterialTheme.colorScheme.outline.copy(alpha = 0.45f)
                                            ),
                                            modifier = Modifier
                                                .weight(1f)
                                                .sizeIn(minHeight = 48.dp)
                                        ) {
                                            Box(
                                                modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
                                                contentAlignment = Alignment.CenterStart
                                            ) {
                                                Text(
                                                    text = title,
                                                    style = MaterialTheme.typography.labelLarge,
                                                    fontWeight = FontWeight.Medium,
                                                    color = MaterialTheme.colorScheme.onSurface
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
                                topEnd = 20.dp,
                                bottomStart = 20.dp,
                                bottomEnd = 6.dp
                            ),
                            color = MaterialTheme.colorScheme.primaryContainer,
                            border = BorderStroke(
                                1.dp,
                                MaterialTheme.colorScheme.primary.copy(alpha = 0.30f)
                            ),
                            modifier = Modifier.widthIn(max = 312.dp)
                        ) {
                            Text(
                                text = msg.content,
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onPrimaryContainer,
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
                            size = 32.dp,
                            interactive = true,
                            seedKey = msg.id,
                            modifier = Modifier.padding(top = 2.dp)
                        )

                        Column(
                            modifier = Modifier.weight(1f),
                            verticalArrangement = Arrangement.spacedBy(6.dp)
                        ) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                Text(
                                    text = msg.persona.ifEmpty { "长庚伙伴" },
                                    style = MaterialTheme.typography.labelLarge,
                                    fontWeight = FontWeight.SemiBold,
                                    color = MaterialTheme.colorScheme.onSurface
                                )
                                Text(
                                    text = msg.model,
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }

                            if (msg.reasoning.isNotEmpty()) {
                                Surface(
                                    onClick = { expandReasoning = !expandReasoning },
                                    shape = RoundedCornerShape(12.dp),
                                    color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.68f),
                                    border = BorderStroke(
                                        1.dp,
                                        MaterialTheme.colorScheme.outline.copy(alpha = 0.4f)
                                    ),
                                    modifier = Modifier.fillMaxWidth()
                                ) {
                                    Column(modifier = Modifier.padding(10.dp)) {
                                        Text(
                                            text = if (expandReasoning) "深度思考脉络 (点击收起)" else "深度思考脉络 (点击展开)",
                                            style = MaterialTheme.typography.labelMedium,
                                            color = MaterialTheme.colorScheme.tertiary
                                        )
                                        if (expandReasoning) {
                                            Spacer(modifier = Modifier.height(6.dp))
                                            Text(
                                                text = msg.reasoning,
                                                style = MaterialTheme.typography.bodySmall,
                                                color = MaterialTheme.colorScheme.onSurfaceVariant
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
                                            color = MaterialTheme.colorScheme.secondaryContainer.copy(alpha = 0.72f),
                                            border = BorderStroke(
                                                1.dp,
                                                MaterialTheme.colorScheme.primary.copy(alpha = 0.25f)
                                            )
                                        ) {
                                            Text(
                                                text = tLabel,
                                                style = MaterialTheme.typography.labelSmall,
                                                color = MaterialTheme.colorScheme.primary,
                                                modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp)
                                            )
                                        }
                                    }
                                }
                            }

                            Surface(
                                shape = RoundedCornerShape(
                                    topStart = 6.dp,
                                    topEnd = 18.dp,
                                    bottomStart = 18.dp,
                                    bottomEnd = 18.dp
                                ),
                                color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.78f),
                                border = BorderStroke(
                                    1.dp,
                                    MaterialTheme.colorScheme.outline.copy(alpha = 0.35f)
                                ),
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Column(
                                    modifier = Modifier.padding(12.dp),
                                    verticalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    Text(
                                        text = msg.content.ifEmpty { if (isStreaming) "正在推演与生成..." else "" },
                                        style = MaterialTheme.typography.bodyMedium,
                                        color = MaterialTheme.colorScheme.onSurface
                                    )
                                    if (msg.content.isNotEmpty()) {
                                        Row(
                                            modifier = Modifier.fillMaxWidth(),
                                            horizontalArrangement = Arrangement.End
                                        ) {
                                            Surface(
                                                onClick = { onCopyText(msg.content) },
                                                shape = RoundedCornerShape(999.dp),
                                                color = MaterialTheme.colorScheme.surface.copy(alpha = 0.8f),
                                                border = BorderStroke(
                                                    1.dp,
                                                    MaterialTheme.colorScheme.outline.copy(alpha = 0.45f)
                                                ),
                                                modifier = Modifier.sizeIn(minHeight = 36.dp)
                                            ) {
                                                Row(
                                                    modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp),
                                                    verticalAlignment = Alignment.CenterVertically,
                                                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                                                ) {
                                                    Icon(
                                                        Icons.Default.ContentCopy,
                                                        contentDescription = "复制",
                                                        modifier = Modifier
                                                            .width(14.dp)
                                                            .height(14.dp),
                                                        tint = MaterialTheme.colorScheme.onSurfaceVariant
                                                    )
                                                    Text(
                                                        text = "复制",
                                                        style = MaterialTheme.typography.labelSmall,
                                                        color = MaterialTheme.colorScheme.onSurfaceVariant
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
        }

        // Floating Capsule Composer
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(top = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            OutlinedTextField(
                value = inputText,
                onValueChange = onInputChange,
                placeholder = {
                    Text("向 ${activeBot?.name ?: "长庚管家"} 发送指令或粘贴链接...")
                },
                modifier = Modifier.weight(1f),
                maxLines = 4,
                shape = RoundedCornerShape(24.dp)
            )
            if (isStreaming) {
                Button(
                    onClick = onStop,
                    shape = CircleShape,
                    contentPadding = PaddingValues(0.dp),
                    modifier = Modifier.sizeIn(minWidth = 50.dp, minHeight = 50.dp)
                ) {
                    Icon(Icons.Default.Stop, contentDescription = "停止")
                }
            } else {
                Button(
                    onClick = onSend,
                    enabled = inputText.isNotBlank(),
                    shape = CircleShape,
                    contentPadding = PaddingValues(0.dp),
                    modifier = Modifier.sizeIn(minWidth = 50.dp, minHeight = 50.dp)
                ) {
                    Icon(Icons.Default.Send, contentDescription = "发送")
                }
            }
        }
    }
}
