package uk.starryheavens.vesper.bothub.ui.sheets

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material.icons.filled.BookmarkBorder
import androidx.compose.material.icons.filled.ChatBubble
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.DeleteOutline
import androidx.compose.material.icons.filled.Description
import androidx.compose.material.icons.outlined.Psychology
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.MemoryItemUi
import uk.starryheavens.vesper.bothub.data.MemoryMarkdownPayload
import uk.starryheavens.vesper.bothub.data.ModelOptionItem
import uk.starryheavens.vesper.bothub.data.SkillItemUi
import uk.starryheavens.vesper.bothub.data.normalizeModelIdForSinglePool
import uk.starryheavens.vesper.bothub.ui.GROKBOT_COLOR_OPTIONS
import uk.starryheavens.vesper.bothub.ui.GROKBOT_SHAPE_OPTIONS
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.parseBotAvatarParts
import uk.starryheavens.vesper.bothub.ui.components.BotAvatar
import uk.starryheavens.vesper.bothub.ui.components.RichMarkdownText
import uk.starryheavens.vesper.bothub.ui.components.VesperButtonTone
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperDivider
import uk.starryheavens.vesper.bothub.ui.components.VesperEmptyState
import uk.starryheavens.vesper.bothub.ui.components.VesperFilterChip
import uk.starryheavens.vesper.bothub.ui.components.VesperIconButton
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSectionHeader
import uk.starryheavens.vesper.bothub.ui.components.VesperSegmentedTabs
import uk.starryheavens.vesper.bothub.ui.components.VesperSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.VesperSwitch
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.VesperTextField
import uk.starryheavens.vesper.bothub.ui.components.vesperPressable
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun BuiltinAndCustomSkillsBlock(
    @Suppress("UNUSED_PARAMETER") botMode: String,
    selectedSkillIds: List<String>,
    availableSkills: List<SkillItemUi>,
    onToggleSkill: (String) -> Unit
) {
    val theme = BotHubThemeTokens
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        VesperCard(
            cornerRadius = 14.dp,
            modifier = Modifier.fillMaxWidth()
        ) {
            Column(
                modifier = Modifier.padding(horizontal = 13.dp, vertical = 11.dp),
                verticalArrangement = Arrangement.spacedBy(5.dp)
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "Agent 原生认知底座",
                        fontSize = 12.5.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = theme.textPrimary
                    )
                    VesperTag(text = "全员内置", toneColor = theme.success)
                }
                Text(
                    text = "实时多源联网搜索、网页正文读取、长期记忆、时间计算与多 Bot 协同已默认内置；外部扩展与 MCP 服务可在下方按需挂载。",
                    fontSize = 11.5.sp,
                    lineHeight = 16.5.sp,
                    color = theme.textMuted
                )
            }
        }

        if (availableSkills.isNotEmpty()) {
            VesperSectionHeader(
                title = "已挂载 MCP 服务与扩展 (${selectedSkillIds.size}/${availableSkills.size})"
            )
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(6.dp),
                verticalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                availableSkills.forEach { sk ->
                    val checked = sk.id in selectedSkillIds
                    VesperFilterChip(
                        label = sk.name,
                        selected = checked,
                        onClick = { onToggleSkill(sk.id) }
                    )
                }
            }
        }
    }
}

/**
 * 1. Contact / Group Profile Card Sheet ("资料卡").
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BotProfileSheet(
    bot: BotItem,
    allBots: List<BotItem>,
    onSendMessage: () -> Unit,
    onTogglePin: () -> Unit,
    onStartGroupWithBot: () -> Unit,
    onOpenMemories: () -> Unit = {},
    onEditBot: () -> Unit,
    onDeleteBot: () -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val theme = BotHubThemeTokens
    val members = remember(bot, allBots) {
        if (bot.isGroup) bot.resolveGroupMembers(allBots) else emptyList()
    }

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
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(14.dp)
            ) {
                BotAvatar(
                    bot = bot,
                    allBots = allBots,
                    containerSize = 62.dp,
                    showPresenceDot = true,
                    interactive = true
                )

                Column(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(5.dp)
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Text(
                            text = bot.name,
                            fontSize = 19.sp,
                            fontWeight = FontWeight.Bold,
                            color = theme.textPrimary,
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f, fill = false)
                        )
                        VesperTag(text = if (bot.isGroup) bot.groupModeLabel() else bot.modeBadgeLabel())
                    }

                    val subtitle = bot.roleTitle.ifBlank { if (bot.isGroup) "多智能体协同群" else "AI 好友" }
                    Text(
                        text = subtitle,
                        fontSize = 13.sp,
                        color = theme.textSecondary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }

            if (bot.traits.isNotBlank() || (bot.isGroup && members.isNotEmpty())) {
                VesperCard(
                    cornerRadius = 16.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(horizontal = 15.dp, vertical = 13.dp),
                        verticalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        if (bot.traits.isNotBlank()) {
                            ProfileFieldRow(label = "简介", value = bot.traits)
                        }
                        if (bot.traits.isNotBlank() && bot.isGroup && members.isNotEmpty()) {
                            VesperDivider()
                        }
                        if (bot.isGroup && members.isNotEmpty()) {
                            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                                Text(
                                    text = "成员 (${members.size})",
                                    fontSize = 12.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = theme.textSecondary,
                                    maxLines = 1,
                                    softWrap = false
                                )
                                LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                                    items(members, key = { it.id }) { m ->
                                        Column(
                                            horizontalAlignment = Alignment.CenterHorizontally,
                                            verticalArrangement = Arrangement.spacedBy(4.dp)
                                        ) {
                                            BotAvatar(
                                                bot = m,
                                                allBots = allBots,
                                                containerSize = 40.dp
                                            )
                                            Text(
                                                text = m.name,
                                                fontSize = 11.sp,
                                                color = theme.textPrimary,
                                                textAlign = TextAlign.Center,
                                                maxLines = 1,
                                                softWrap = false,
                                                overflow = TextOverflow.Ellipsis,
                                                modifier = Modifier.width(52.dp)
                                            )
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }

            VesperCard(
                cornerRadius = 16.dp,
                onClick = onTogglePin,
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 56.dp)
                        .padding(start = 15.dp, end = 6.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "置顶聊天",
                        fontSize = 14.5.sp,
                        fontWeight = FontWeight.Medium,
                        color = theme.textPrimary,
                        maxLines = 1,
                        softWrap = false
                    )
                    VesperSwitch(
                        checked = bot.isPinned,
                        onCheckedChange = { onTogglePin() }
                    )
                }
            }

            VesperPrimaryButton(
                text = if (bot.isGroup) "进入群聊" else "发消息",
                leadingIcon = Icons.Default.ChatBubble,
                minHeight = 48.dp,
                onClick = onSendMessage,
                modifier = Modifier.fillMaxWidth()
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                if (!bot.isGroup) {
                    VesperSecondaryButton(
                        text = "发起群聊",
                        minHeight = 44.dp,
                        contentPadding = PaddingValues(horizontal = 8.dp, vertical = 8.dp),
                        onClick = onStartGroupWithBot,
                        modifier = Modifier.weight(1f)
                    )
                    VesperSecondaryButton(
                        text = "长期记忆",
                        minHeight = 44.dp,
                        contentPadding = PaddingValues(horizontal = 8.dp, vertical = 8.dp),
                        onClick = onOpenMemories,
                        modifier = Modifier.weight(1f)
                    )
                }
                VesperSecondaryButton(
                    text = "编辑资料",
                    minHeight = 44.dp,
                    contentPadding = PaddingValues(horizontal = 8.dp, vertical = 8.dp),
                    onClick = onEditBot,
                    modifier = Modifier.weight(1f)
                )
                if (!bot.isBuiltin) {
                    VesperSecondaryButton(
                        text = if (bot.isGroup) "解散" else "删除",
                        tone = VesperButtonTone.DANGER,
                        minHeight = 44.dp,
                        contentPadding = PaddingValues(horizontal = 8.dp, vertical = 8.dp),
                        onClick = onDeleteBot,
                        modifier = Modifier.weight(0.85f)
                    )
                }
            }

            Spacer(modifier = Modifier.height(6.dp))
        }
    }
}

private fun canonicalMemoryType(raw: String): String {
    return when (raw.trim().lowercase()) {
        "feedback", "correction", "rule" -> "feedback"
        "project", "context", "decision" -> "project"
        "reference", "link", "resource" -> "reference"
        else -> "user"
    }
}

private fun memoryTypeLabel(canonical: String): String {
    return when (canonical) {
        "feedback" -> "纠偏"
        "project" -> "项目"
        "reference" -> "参考"
        else -> "偏好"
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BotMemoriesSheet(
    bot: BotItem,
    memories: List<MemoryItemUi>,
    loading: Boolean,
    memoryMarkdown: MemoryMarkdownPayload? = null,
    dreaming: Boolean = false,
    onLoadMarkdown: () -> Unit = {},
    onRunDream: () -> Unit = {},
    onCreateMemory: (category: String, factKey: String, content: String, pinned: Boolean) -> Unit,
    onTogglePin: (MemoryItemUi) -> Unit,
    onDeleteMemory: (MemoryItemUi) -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val theme = BotHubThemeTokens
    var showAddForm by remember { mutableStateOf(false) }
    var showMarkdownIndex by remember { mutableStateOf(false) }
    var activeTypeFilter by remember { mutableStateOf("all") }
    var factKey by remember { mutableStateOf("") }
    var content by remember { mutableStateOf("") }
    var category by remember { mutableStateOf("feedback") }
    var pinned by remember { mutableStateOf(false) }

    val filteredMemories = remember(memories, activeTypeFilter) {
        if (activeTypeFilter == "all") {
            memories
        } else {
            memories.filter { canonicalMemoryType(it.category) == activeTypeFilter }
        }
    }

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
                .imePadding()
                .navigationBarsPadding()
                .padding(horizontal = 18.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(11.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f).padding(end = 10.dp)) {
                    Text(
                        text = "${bot.name} · 长期记忆库",
                        fontSize = 18.sp,
                        fontWeight = FontWeight.Bold,
                        color = theme.textPrimary
                    )
                    Text(
                        text = "共 ${memories.size} 条记忆（Claude 4 类结构化记忆 + AutoDream）",
                        fontSize = 11.5.sp,
                        color = theme.textMuted
                    )
                }
                VesperSecondaryButton(
                    text = if (showAddForm) "收起" else "新增",
                    leadingIcon = if (showAddForm) null else Icons.Default.Add,
                    tone = VesperButtonTone.ACCENT,
                    cornerRadius = 999.dp,
                    minHeight = 38.dp,
                    contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                    onClick = { showAddForm = !showAddForm }
                )
            }

            // Action strip: MEMORY.md Index Viewer + AutoDream Consolidation
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                VesperSecondaryButton(
                    text = if (showMarkdownIndex) "返回列表" else "MEMORY.md 索引",
                    leadingIcon = Icons.Default.Description,
                    tone = if (showMarkdownIndex) VesperButtonTone.ACCENT else VesperButtonTone.NEUTRAL,
                    minHeight = 44.dp,
                    contentPadding = PaddingValues(horizontal = 10.dp, vertical = 8.dp),
                    onClick = {
                        val next = !showMarkdownIndex
                        showMarkdownIndex = next
                        if (next) onLoadMarkdown()
                    },
                    modifier = Modifier.weight(1f)
                )
                VesperSecondaryButton(
                    text = if (dreaming) "AutoDream 整合中..." else "AutoDream 整合",
                    leadingIcon = Icons.Default.AutoAwesome,
                    tone = VesperButtonTone.ACCENT,
                    enabled = !dreaming,
                    minHeight = 44.dp,
                    contentPadding = PaddingValues(horizontal = 10.dp, vertical = 8.dp),
                    onClick = onRunDream,
                    modifier = Modifier.weight(1f)
                )
            }

            if (showAddForm) {
                VesperCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                    Column(
                        modifier = Modifier.padding(14.dp),
                        verticalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        val cats = listOf(
                            "feedback" to "纠偏",
                            "user" to "偏好",
                            "project" to "项目",
                            "reference" to "参考"
                        )
                        VesperSegmentedTabs(
                            options = cats.map { it.second },
                            selectedIndex = cats.indexOfFirst { it.first == category }.coerceAtLeast(0),
                            onSelect = { idx -> category = cats[idx].first }
                        )
                        VesperTextField(
                            value = factKey,
                            onValueChange = { factKey = it },
                            label = "记忆主题标签",
                            placeholder = "如：回答风格 / 服务器运维规则",
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth()
                        )
                        VesperTextField(
                            value = content,
                            onValueChange = { content = it },
                            label = "记忆内容",
                            placeholder = "希望 Bot 长期牢记的规则、偏好或项目上下文...",
                            singleLine = false,
                            maxLines = 3,
                            modifier = Modifier.fillMaxWidth()
                        )
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .sizeIn(minHeight = 48.dp)
                                .clickable { pinned = !pinned },
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text("置顶长期注入", fontSize = 13.5.sp, color = theme.textPrimary)
                            VesperSwitch(checked = pinned, onCheckedChange = { pinned = it })
                        }
                        VesperPrimaryButton(
                            text = "写入记忆",
                            enabled = content.isNotBlank(),
                            minHeight = 46.dp,
                            onClick = {
                                onCreateMemory(category, factKey.ifBlank { "note" }, content.trim(), pinned)
                                factKey = ""
                                content = ""
                                pinned = false
                                showAddForm = false
                            },
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                }
            }

            if (showMarkdownIndex) {
                VesperCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .heightIn(max = 420.dp)
                            .verticalScroll(rememberScrollState())
                            .padding(14.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = "MEMORY.md 实时索引",
                                fontSize = 13.sp,
                                fontWeight = FontWeight.SemiBold,
                                fontFamily = FontFamily.Monospace,
                                color = theme.accentForeground
                            )
                            val lastAt = memoryMarkdown?.lastDreamAt.orEmpty().trim()
                            if (lastAt.isNotEmpty()) {
                                Text(
                                    text = "上次整合 ${lastAt.take(16)}",
                                    fontSize = 11.sp,
                                    color = theme.textMuted
                                )
                            }
                        }
                        VesperDivider()
                        val mdBody = memoryMarkdown?.markdown.orEmpty().trim()
                        if (mdBody.isEmpty()) {
                            Text(
                                text = "正在读取 MEMORY.md 索引视图...",
                                fontSize = 13.sp,
                                color = theme.textMuted
                            )
                        } else {
                            RichMarkdownText(
                                markdown = mdBody,
                                modifier = Modifier.fillMaxWidth()
                            )
                        }
                    }
                }
            } else {
                // 4-Type Canonical Filter Pills
                val filterOptions = listOf(
                    "all" to "全部 (${memories.size})",
                    "feedback" to "纠偏 (${memories.count { canonicalMemoryType(it.category) == "feedback" }})",
                    "user" to "偏好 (${memories.count { canonicalMemoryType(it.category) == "user" }})",
                    "project" to "项目 (${memories.count { canonicalMemoryType(it.category) == "project" }})",
                    "reference" to "参考 (${memories.count { canonicalMemoryType(it.category) == "reference" }})"
                )
                LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    items(filterOptions, key = { it.first }) { (key, label) ->
                        VesperFilterChip(
                            label = label,
                            selected = activeTypeFilter == key,
                            onClick = { activeTypeFilter = key }
                        )
                    }
                }

                LazyColumn(
                    modifier = Modifier
                        .fillMaxWidth()
                        .heightIn(max = 380.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    if (loading && memories.isEmpty()) {
                        item {
                            Text(
                                text = "正在加载长期记忆...",
                                fontSize = 13.sp,
                                color = theme.textMuted,
                                modifier = Modifier.padding(vertical = 16.dp)
                            )
                        }
                    } else if (filteredMemories.isEmpty()) {
                        item {
                            VesperEmptyState(
                                icon = Icons.Outlined.Psychology,
                                title = if (activeTypeFilter == "all") "暂无长期记忆" else "该分类暂无记忆",
                                subtitle = "对话中可通过「AutoDream 整合」自动提炼，或点击右上角「新增」手动写入"
                            )
                        }
                    }
                    items(filteredMemories, key = { it.id }) { mem ->
                        val canonType = canonicalMemoryType(mem.category)
                        val typeTone = when (canonType) {
                            "feedback" -> theme.warning
                            "project" -> theme.info
                            "reference" -> theme.success
                            else -> theme.accent
                        }
                        VesperCard(cornerRadius = 14.dp, modifier = Modifier.fillMaxWidth()) {
                            Column(
                                modifier = Modifier.padding(start = 13.dp, end = 4.dp, top = 8.dp, bottom = 11.dp),
                                verticalArrangement = Arrangement.spacedBy(4.dp)
                            ) {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Row(
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(5.dp),
                                        modifier = Modifier.weight(1f)
                                    ) {
                                        VesperTag(
                                            text = memoryTypeLabel(canonType),
                                            toneColor = typeTone
                                        )
                                        VesperTag(
                                            text = if (mem.scope == "global") "全局" else "专属",
                                            bgColor = theme.surfaceInput,
                                            borderColor = theme.border,
                                            textColor = theme.textSecondary
                                        )
                                        Text(
                                            text = mem.factKey.ifBlank { canonType },
                                            fontSize = 13.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            fontFamily = FontFamily.Monospace,
                                            color = theme.textPrimary,
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis
                                        )
                                    }
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        VesperIconButton(
                                            icon = if (mem.pinned) Icons.Default.Bookmark else Icons.Default.BookmarkBorder,
                                            contentDescription = if (mem.pinned) "取消置顶" else "置顶记忆",
                                            accent = mem.pinned,
                                            filled = false,
                                            iconSize = 18.dp,
                                            onClick = { onTogglePin(mem) }
                                        )
                                        VesperIconButton(
                                            icon = Icons.Default.DeleteOutline,
                                            contentDescription = "删除记忆",
                                            danger = true,
                                            filled = false,
                                            iconSize = 18.dp,
                                            onClick = { onDeleteMemory(mem) }
                                        )
                                    }
                                }
                                Text(
                                    text = mem.content,
                                    fontSize = 13.sp,
                                    lineHeight = 19.sp,
                                    color = theme.textSecondary,
                                    modifier = Modifier.padding(end = 9.dp)
                                )
                            }
                        }
                    }
                }
            }
            Spacer(modifier = Modifier.height(6.dp))
        }
    }
}

@Composable
private fun ProfileFieldRow(label: String, value: String) {
    val theme = BotHubThemeTokens
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = label,
            fontSize = 12.5.sp,
            color = theme.textMuted,
            maxLines = 1,
            softWrap = false,
            modifier = Modifier.width(42.dp)
        )
        Text(
            text = value,
            fontSize = 13.5.sp,
            lineHeight = 20.sp,
            color = theme.textPrimary,
            modifier = Modifier.weight(1f)
        )
    }
}

/**
 * 2. Native Edit Friend / Group Profile Sheet.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EditBotSheet(
    bot: BotItem,
    availableSkills: List<SkillItemUi> = emptyList(),
    onForgePrompt: (
        botId: String?,
        name: String,
        roleTitle: String,
        botMode: String,
        traits: String,
        systemPrompt: String,
        onResult: (String?, String?) -> Unit
    ) -> Unit = { _, _, _, _, _, _, cb -> cb(null, null) },
    onSave: (
        name: String,
        roleTitle: String,
        avatarStyle: String,
        traits: String,
        systemPrompt: String,
        groupMode: String,
        announcement: String,
        allowedTools: List<String>?,
        skillIds: List<String>?
    ) -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val theme = BotHubThemeTokens
    var name by remember(bot.id) { mutableStateOf(bot.name) }
    var roleTitle by remember(bot.id) { mutableStateOf(bot.roleTitle) }
    var avatarStyle by remember(bot.id) { mutableStateOf(bot.avatarStyle.ifBlank { "butler:gold" }) }
    var traits by remember(bot.id) { mutableStateOf(bot.traits) }
    var systemPrompt by remember(bot.id) { mutableStateOf(bot.systemPrompt) }
    var groupMode by remember(bot.id) { mutableStateOf(bot.groupMode.ifBlank { "smart" }) }
    var announcement by remember(bot.id) { mutableStateOf(bot.announcement) }
    var selectedSkillIds by remember(bot.id) { mutableStateOf(bot.skillIds) }
    var forgingPrompt by remember(bot.id) { mutableStateOf(false) }

    val groupModes = listOf(
        "smart" to "智能协同",
        "sequential" to "轮流接力",
        "parallel" to "全员并发"
    )

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
                .imePadding()
                .navigationBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            Text(
                text = if (bot.isGroup) "编辑群资料" else "编辑好友资料",
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )

            if (!bot.isGroup) {
                AvatarPickerRow(
                    selected = avatarStyle,
                    onSelect = { avatarStyle = it }
                )
            }

            VesperTextField(
                value = name,
                onValueChange = { name = it },
                label = if (bot.isGroup) "群名称" else "昵称",
                placeholder = "输入名称",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            VesperTextField(
                value = roleTitle,
                onValueChange = { roleTitle = it },
                label = if (bot.isGroup) "群定位" else "角色头衔",
                placeholder = "简短描述角色",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            if (bot.isGroup) {
                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    VesperSectionHeader(title = "协作模式")
                    VesperSegmentedTabs(
                        options = groupModes.map { it.second },
                        selectedIndex = groupModes.indexOfFirst { it.first == groupMode }.coerceAtLeast(0),
                        onSelect = { idx -> groupMode = groupModes[idx].first }
                    )
                }

                VesperTextField(
                    value = announcement,
                    onValueChange = { announcement = it },
                    label = "群公告（可选）",
                    placeholder = "输入群公告...",
                    singleLine = false,
                    maxLines = 3,
                    modifier = Modifier.fillMaxWidth()
                )
            } else {
                VesperTextField(
                    value = traits,
                    onValueChange = { traits = it },
                    label = "简介与说话风格（可选）",
                    placeholder = "例如：言简意赅、自然直率",
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )

                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "系统提示词",
                            fontSize = 12.sp,
                            color = theme.textMuted
                        )
                        VesperSecondaryButton(
                            text = if (forgingPrompt) "AI 精修中..." else "AI 深度精修人设",
                            leadingIcon = Icons.Default.AutoAwesome,
                            tone = VesperButtonTone.ACCENT,
                            enabled = !forgingPrompt && name.isNotBlank(),
                            cornerRadius = 999.dp,
                            minHeight = 36.dp,
                            contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                            onClick = {
                                forgingPrompt = true
                                onForgePrompt(bot.id, name, roleTitle, bot.botMode, traits, systemPrompt) { fp, ft ->
                                    if (!fp.isNullOrBlank()) systemPrompt = fp
                                    if (!ft.isNullOrBlank()) traits = ft
                                    forgingPrompt = false
                                }
                            }
                        )
                    }
                    VesperTextField(
                        value = systemPrompt,
                        onValueChange = { systemPrompt = it },
                        placeholder = "设定角色指令与边界...",
                        singleLine = false,
                        maxLines = 5,
                        modifier = Modifier.fillMaxWidth()
                    )
                }

                BuiltinAndCustomSkillsBlock(
                    botMode = bot.botMode,
                    selectedSkillIds = selectedSkillIds,
                    availableSkills = availableSkills,
                    onToggleSkill = { sid ->
                        selectedSkillIds = if (sid in selectedSkillIds) {
                            selectedSkillIds.filterNot { it == sid }
                        } else {
                            selectedSkillIds + sid
                        }
                    }
                )
            }

            VesperPrimaryButton(
                text = "保存修改",
                enabled = name.isNotBlank(),
                minHeight = 48.dp,
                onClick = {
                    if (name.isNotBlank()) {
                        onSave(
                            name.trim(),
                            roleTitle.trim(),
                            avatarStyle,
                            traits.trim(),
                            systemPrompt.trim(),
                            groupMode,
                            announcement.trim(),
                            if (bot.isGroup) null else emptyList(),
                            if (bot.isGroup) null else selectedSkillIds
                        )
                    }
                },
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(8.dp))
        }
    }
}

@Composable
private fun AvatarPickerRow(
    selected: String,
    onSelect: (String) -> Unit
) {
    val theme = BotHubThemeTokens
    val (curShape, curColor) = remember(selected) { parseBotAvatarParts(selected) }
    val normalizedStyle = "$curShape:$curColor"

    VesperCard(
        cornerRadius = 16.dp,
        modifier = Modifier.fillMaxWidth()
    ) {
        Column(
            modifier = Modifier.padding(12.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            // 8 Unified Bot Palette Swatches
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                GrokBotAvatar(
                    avatarStyle = normalizedStyle,
                    state = "idle",
                    size = 48.dp,
                    interactive = true,
                    animateIdle = true,
                    seedKey = "picker_preview"
                )
                LazyRow(
                    modifier = Modifier.weight(1f),
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    items(GROKBOT_COLOR_OPTIONS, key = { it.id }) { colorOpt ->
                        val isSelectedColor = curColor == colorOpt.id
                        Box(
                            modifier = Modifier
                                .size(40.dp)
                                .vesperPressable(pressedScale = 0.90f, hapticOnTap = true) {
                                    onSelect("$curShape:${colorOpt.id}")
                                },
                            contentAlignment = Alignment.Center
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(34.dp)
                                    .clip(CircleShape)
                                    .border(
                                        width = if (isSelectedColor) 2.dp else 1.dp,
                                        color = if (isSelectedColor) theme.accent else theme.border,
                                        shape = CircleShape
                                    )
                                    .padding(3.dp)
                                    .clip(CircleShape)
                                    .background(
                                        Brush.linearGradient(
                                            listOf(
                                                colorOpt.palette.shellHi,
                                                colorOpt.palette.shellMid,
                                                colorOpt.palette.ring
                                            )
                                        ),
                                        CircleShape
                                    )
                            )
                        }
                    }
                }
            }

            // 6 Unified Monolithic Shapes
            LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                items(GROKBOT_SHAPE_OPTIONS, key = { it.id }) { shapeOpt ->
                    val active = curShape == shapeOpt.id
                    val itemStyle = "${shapeOpt.id}:$curColor"
                    val itemShape = RoundedCornerShape(12.dp)
                    Column(
                        modifier = Modifier
                            .vesperPressable(pressedScale = 0.94f, hapticOnTap = true) {
                                onSelect(itemStyle)
                            }
                            .clip(itemShape)
                            .background(if (active) theme.accentSoft else theme.surfaceControl, itemShape)
                            .border(
                                width = 1.dp,
                                color = if (active) theme.accentMuted else theme.border,
                                shape = itemShape
                            )
                            .padding(horizontal = 11.dp, vertical = 8.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(4.dp)
                    ) {
                        GrokBotAvatar(
                            avatarStyle = itemStyle,
                            state = if (active) "done" else "idle",
                            size = 34.dp,
                            interactive = false,
                            animateIdle = active,
                            seedKey = itemStyle
                        )
                        Text(
                            text = shapeOpt.label,
                            fontSize = 10.5.sp,
                            fontWeight = if (active) FontWeight.SemiBold else FontWeight.Normal,
                            color = if (active) theme.accentForeground else theme.textSecondary,
                            maxLines = 1,
                            softWrap = false
                        )
                    }
                }
            }
        }
    }
}

/**
 * 3. Native Bot Forge Sheet ("添加 AI 好友").
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CreateFriendSheet(
    availableSkills: List<SkillItemUi> = emptyList(),
    onForgePrompt: (
        botId: String?,
        name: String,
        roleTitle: String,
        botMode: String,
        traits: String,
        systemPrompt: String,
        onResult: (String?, String?) -> Unit
    ) -> Unit = { _, _, _, _, _, _, cb -> cb(null, null) },
    onSubmit: (
        name: String,
        roleTitle: String,
        avatarStyle: String,
        botMode: String,
        traits: String,
        systemPrompt: String,
        allowedTools: List<String>,
        skillIds: List<String>
    ) -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val theme = BotHubThemeTokens
    val modeOptions = listOf(
        "companion" to "日常伴聊",
        "hybrid" to "全能伙伴",
        "worker" to "工具助理"
    )

    var selectedAvatar by remember { mutableStateOf("swarm:violet") }
    var selectedMode by remember { mutableStateOf("hybrid") }
    var name by remember { mutableStateOf("") }
    var roleTitle by remember { mutableStateOf("") }
    var traits by remember { mutableStateOf("") }
    var systemPrompt by remember { mutableStateOf("") }
    var selectedSkillIds by remember { mutableStateOf<List<String>>(emptyList()) }
    var forgingPrompt by remember { mutableStateOf(false) }

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
                .imePadding()
                .navigationBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            Text(
                text = "添加 AI 好友",
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )

            AvatarPickerRow(
                selected = selectedAvatar,
                onSelect = { selectedAvatar = it }
            )

            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                VesperSectionHeader(title = "交互模式")
                VesperSegmentedTabs(
                    options = modeOptions.map { it.second },
                    selectedIndex = modeOptions.indexOfFirst { it.first == selectedMode }.coerceAtLeast(1),
                    onSelect = { idx -> selectedMode = modeOptions[idx].first }
                )
            }

            VesperTextField(
                value = name,
                onValueChange = { name = it },
                label = "昵称",
                placeholder = "例如：架构顾问 / 英语陪练",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            VesperTextField(
                value = roleTitle,
                onValueChange = { roleTitle = it },
                label = "角色头衔（可选）",
                placeholder = "一句话角色描述",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            VesperTextField(
                value = traits,
                onValueChange = { traits = it },
                label = "说话风格（可选）",
                placeholder = "例如：言简意赅、自然直率",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "系统提示词（留空自动生成）",
                        fontSize = 12.sp,
                        color = theme.textMuted
                    )
                    VesperSecondaryButton(
                        text = if (forgingPrompt) "AI 精修中..." else "AI 深度精修人设",
                        leadingIcon = Icons.Default.AutoAwesome,
                        tone = VesperButtonTone.ACCENT,
                        enabled = !forgingPrompt && name.isNotBlank(),
                        cornerRadius = 999.dp,
                        minHeight = 36.dp,
                        contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                        onClick = {
                            forgingPrompt = true
                            onForgePrompt(null, name, roleTitle, selectedMode, traits, systemPrompt) { fp, ft ->
                                if (!fp.isNullOrBlank()) systemPrompt = fp
                                if (!ft.isNullOrBlank()) traits = ft
                                forgingPrompt = false
                            }
                        }
                    )
                }
                VesperTextField(
                    value = systemPrompt,
                    onValueChange = { systemPrompt = it },
                    placeholder = "补充设定或回答偏好，也可点击右上角一键 AI 精修...",
                    singleLine = false,
                    maxLines = 4,
                    modifier = Modifier.fillMaxWidth()
                )
            }

            BuiltinAndCustomSkillsBlock(
                botMode = selectedMode,
                selectedSkillIds = selectedSkillIds,
                availableSkills = availableSkills,
                onToggleSkill = { sid ->
                    selectedSkillIds = if (sid in selectedSkillIds) {
                        selectedSkillIds.filterNot { it == sid }
                    } else {
                        selectedSkillIds + sid
                    }
                }
            )

            VesperPrimaryButton(
                text = "创建 AI 好友",
                enabled = name.isNotBlank(),
                minHeight = 48.dp,
                onClick = {
                    val cleanName = name.trim()
                    if (cleanName.isNotEmpty()) {
                        onSubmit(
                            cleanName,
                            roleTitle.trim().ifEmpty { "AI 好友" },
                            selectedAvatar,
                            selectedMode,
                            traits.trim(),
                            systemPrompt.trim(),
                            emptyList(),
                            selectedSkillIds
                        )
                    }
                },
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(8.dp))
        }
    }
}

/**
 * 4. Select Friends to Start Group Chat Sheet ("发起群聊").
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CreateGroupSheet(
    allBots: List<BotItem>,
    sourceBotId: String,
    onSubmit: (
        sourceBotId: String,
        memberIds: List<String>,
        name: String,
        groupMode: String,
        carryContext: Boolean
    ) -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val candidates = remember(allBots) { allBots.filter { !it.isGroup } }
    val selectedIds = remember(sourceBotId) {
        mutableStateListOf<String>().apply {
            if (sourceBotId.isNotBlank()) add(sourceBotId)
        }
    }
    val theme = BotHubThemeTokens
    var groupName by remember { mutableStateOf("") }
    var groupMode by remember { mutableStateOf("smart") }
    var carryContext by remember(sourceBotId) { mutableStateOf(sourceBotId.isNotBlank()) }

    val modes = listOf(
        "smart" to "智能点名",
        "sequential" to "轮流接力",
        "parallel" to "全员并发"
    )

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
                .imePadding()
                .navigationBarsPadding()
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "发起协同群聊",
                    fontSize = 18.sp,
                    fontWeight = FontWeight.Bold,
                    color = theme.textPrimary,
                    maxLines = 1,
                    softWrap = false
                )

                VesperPrimaryButton(
                    text = "创建 (${selectedIds.size})",
                    enabled = selectedIds.size >= 2,
                    cornerRadius = 999.dp,
                    minHeight = 40.dp,
                    contentPadding = PaddingValues(horizontal = 16.dp, vertical = 7.dp),
                    onClick = {
                        if (selectedIds.size >= 2) {
                            onSubmit(sourceBotId, selectedIds.toList(), groupName, groupMode, carryContext)
                        }
                    }
                )
            }

            VesperTextField(
                value = groupName,
                onValueChange = { groupName = it },
                placeholder = "群名称（可选，留空自动命名）",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            VesperSegmentedTabs(
                options = modes.map { it.second },
                selectedIndex = modes.indexOfFirst { it.first == groupMode }.coerceAtLeast(0),
                onSelect = { idx -> groupMode = modes[idx].first }
            )

            if (sourceBotId.isNotBlank()) {
                VesperCard(
                    cornerRadius = 14.dp,
                    onClick = { carryContext = !carryContext },
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 54.dp)
                            .padding(start = 14.dp, end = 6.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "同步最近单聊上下文",
                            fontSize = 13.5.sp,
                            color = theme.textPrimary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        )
                        VesperSwitch(
                            checked = carryContext,
                            onCheckedChange = { carryContext = it }
                        )
                    }
                }
            }

            LazyColumn(
                modifier = Modifier
                    .fillMaxWidth()
                    .weight(1f, fill = false),
                verticalArrangement = Arrangement.spacedBy(8.dp),
                contentPadding = PaddingValues(bottom = 16.dp)
            ) {
                items(candidates, key = { it.id }) { friend ->
                    val checked = selectedIds.contains(friend.id)
                    val locked = friend.id == sourceBotId
                    MemberSelectableRow(
                        friend = friend,
                        allBots = allBots,
                        checked = checked,
                        locked = locked,
                        onToggle = {
                            if (!locked) {
                                if (checked) selectedIds.remove(friend.id) else selectedIds.add(friend.id)
                            }
                        }
                    )
                }
            }
        }
    }
}

@Composable
private fun MemberSelectableRow(
    friend: BotItem,
    allBots: List<BotItem>,
    checked: Boolean,
    locked: Boolean = false,
    onToggle: () -> Unit
) {
    val theme = BotHubThemeTokens
    VesperCard(
        cornerRadius = 14.dp,
        accentTone = if (checked) theme.accent else null,
        onClick = onToggle,
        modifier = Modifier.fillMaxWidth()
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .sizeIn(minHeight = 60.dp)
                .padding(horizontal = 13.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(22.dp)
                    .clip(CircleShape)
                    .background(if (checked) theme.accent else theme.surfaceControl, CircleShape)
                    .border(
                        1.dp,
                        if (checked) theme.accent else theme.borderStrong,
                        CircleShape
                    ),
                contentAlignment = Alignment.Center
            ) {
                if (checked) {
                    Icon(
                        imageVector = Icons.Default.Check,
                        contentDescription = null,
                        tint = theme.onAccent,
                        modifier = Modifier.size(14.dp)
                    )
                }
            }

            BotAvatar(
                bot = friend,
                allBots = allBots,
                containerSize = 40.dp
            )

            Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(
                    text = friend.name,
                    fontSize = 14.5.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = theme.textPrimary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (friend.roleTitle.isNotBlank()) {
                    Text(
                        text = friend.roleTitle,
                        fontSize = 12.sp,
                        color = theme.textMuted,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }

            if (locked) {
                VesperTag(
                    text = "发起人",
                    bgColor = theme.surfaceInput,
                    borderColor = theme.border,
                    textColor = theme.textMuted
                )
            }
        }
    }
}

/**
 * 5. Chat Room Info Drawer ("聊天详情").
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ChatInfoSheet(
    activeBot: BotItem?,
    allBots: List<BotItem>,
    activeModel: String,
    modelOptions: List<ModelOptionItem> = emptyList(),
    enableThinking: Boolean,
    onModelChange: (String) -> Unit,
    onThinkingChange: (Boolean) -> Unit,
    onTogglePin: () -> Unit,
    onInviteToGroup: () -> Unit,
    onOpenMemberProfile: (BotItem) -> Unit,
    onSearchInChat: () -> Unit = {},
    onEditBot: () -> Unit,
    onClearHistory: () -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var showModelPicker by remember { mutableStateOf(false) }
    val isGroup = activeBot?.isGroup == true
    val members = remember(activeBot, allBots) {
        when {
            activeBot == null -> emptyList()
            activeBot.isGroup -> activeBot.resolveGroupMembers(allBots)
            else -> listOf(activeBot)
        }
    }

    val theme = BotHubThemeTokens

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
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            Text(
                text = if (isGroup) "群聊详情 (${members.size})" else "聊天详情",
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )

            VesperCard(
                cornerRadius = 16.dp,
                modifier = Modifier.fillMaxWidth()
            ) {
                LazyRow(
                    contentPadding = PaddingValues(horizontal = 14.dp, vertical = 14.dp),
                    horizontalArrangement = Arrangement.spacedBy(14.dp)
                ) {
                    items(members, key = { it.id }) { m ->
                        Column(
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(5.dp),
                            modifier = Modifier.vesperPressable(pressedScale = 0.93f) {
                                onDismiss()
                                onOpenMemberProfile(m)
                            }
                        ) {
                            BotAvatar(
                                bot = m,
                                allBots = allBots,
                                containerSize = 48.dp
                            )
                            Text(
                                text = m.name,
                                fontSize = 11.sp,
                                color = theme.textPrimary,
                                textAlign = TextAlign.Center,
                                maxLines = 1,
                                softWrap = false,
                                overflow = TextOverflow.Ellipsis,
                                modifier = Modifier.width(52.dp)
                            )
                        }
                    }

                    item(key = "__invite_plus__") {
                        val plusShape = RoundedCornerShape(14.dp)
                        Column(
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(5.dp),
                            modifier = Modifier.vesperPressable(pressedScale = 0.93f) {
                                onDismiss()
                                onInviteToGroup()
                            }
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(48.dp)
                                    .clip(plusShape)
                                    .background(theme.surfaceControl, plusShape)
                                    .border(1.dp, theme.border, plusShape),
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = Icons.Default.Add,
                                    contentDescription = if (isGroup) "管理群成员" else "拉人建群",
                                    tint = theme.textSecondary,
                                    modifier = Modifier.size(22.dp)
                                )
                            }
                            Text(
                                text = if (isGroup) "管理" else "添加",
                                fontSize = 11.sp,
                                color = theme.textSecondary,
                                maxLines = 1,
                                softWrap = false
                            )
                        }
                    }
                }
            }

            VesperCard(
                cornerRadius = 16.dp,
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(modifier = Modifier.fillMaxWidth()) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 54.dp)
                            .clickable {
                                onDismiss()
                                onSearchInChat()
                            }
                            .padding(horizontal = 15.dp, vertical = 12.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text("查找聊天内容", fontSize = 14.5.sp, color = theme.textPrimary)
                        Icon(
                            imageVector = Icons.AutoMirrored.Filled.KeyboardArrowRight,
                            contentDescription = null,
                            tint = theme.textMuted,
                            modifier = Modifier.size(18.dp)
                        )
                    }
                    VesperDivider(startIndent = 15.dp, endIndent = 15.dp)

                    if (activeBot != null) {
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .sizeIn(minHeight = 54.dp)
                                .clickable { onTogglePin() }
                                .padding(start = 15.dp, end = 6.dp),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text("置顶聊天", fontSize = 14.5.sp, color = theme.textPrimary)
                            VesperSwitch(
                                checked = activeBot.isPinned,
                                onCheckedChange = { onTogglePin() }
                            )
                        }
                        VesperDivider(startIndent = 15.dp, endIndent = 15.dp)
                    }

                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 54.dp)
                            .clickable { onThinkingChange(!enableThinking) }
                            .padding(start = 15.dp, end = 6.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text("深度思考", fontSize = 14.5.sp, color = theme.textPrimary)
                        VesperSwitch(
                            checked = enableThinking,
                            onCheckedChange = onThinkingChange
                        )
                    }

                    VesperDivider(startIndent = 15.dp, endIndent = 15.dp)

                    val currentModelLabel = resolveModelOptionDisplayName(activeModel, modelOptions)
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 54.dp)
                            .clickable { showModelPicker = true }
                            .padding(horizontal = 15.dp, vertical = 12.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text("推理模型", fontSize = 14.5.sp, color = theme.textPrimary)
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(2.dp),
                            modifier = Modifier.weight(1f, fill = false).padding(start = 12.dp)
                        ) {
                            Text(
                                text = currentModelLabel,
                                fontSize = 13.sp,
                                color = theme.accentForeground,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                            Icon(
                                imageVector = Icons.AutoMirrored.Filled.KeyboardArrowRight,
                                contentDescription = null,
                                tint = theme.accentForeground,
                                modifier = Modifier.size(18.dp)
                            )
                        }
                    }

                    if (activeBot != null) {
                        VesperDivider(startIndent = 15.dp, endIndent = 15.dp)

                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .sizeIn(minHeight = 54.dp)
                                .clickable {
                                    onDismiss()
                                    onEditBot()
                                }
                                .padding(horizontal = 15.dp, vertical = 12.dp),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = if (isGroup) "编辑群资料" else "编辑好友资料",
                                fontSize = 14.5.sp,
                                color = theme.textPrimary
                            )
                            Icon(
                                imageVector = Icons.AutoMirrored.Filled.KeyboardArrowRight,
                                contentDescription = null,
                                tint = theme.textMuted,
                                modifier = Modifier.size(18.dp)
                            )
                        }
                    }
                }
            }

            VesperSecondaryButton(
                text = "清空聊天记录",
                tone = VesperButtonTone.DANGER,
                minHeight = 48.dp,
                onClick = {
                    onDismiss()
                    onClearHistory()
                },
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(8.dp))
        }
    }

    if (showModelPicker) {
        ModelPickerSheet(
            activeModel = activeModel,
            modelOptions = modelOptions,
            onSelect = { chosen ->
                showModelPicker = false
                onModelChange(chosen)
            },
            onDismiss = { showModelPicker = false }
        )
    }
}

/**
 * 6. Manage Existing Group Members Sheet ("管理群成员").
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ManageGroupMembersSheet(
    groupBot: BotItem,
    allBots: List<BotItem>,
    onSubmit: (memberIds: List<String>) -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val theme = BotHubThemeTokens
    val candidates = remember(allBots) { allBots.filter { !it.isGroup } }
    val selectedIds = remember(groupBot.id, groupBot.memberIds) {
        mutableStateListOf<String>().apply {
            addAll(groupBot.memberIds)
        }
    }

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
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "群成员管理",
                    fontSize = 18.sp,
                    fontWeight = FontWeight.Bold,
                    color = theme.textPrimary,
                    maxLines = 1,
                    softWrap = false
                )

                VesperPrimaryButton(
                    text = "保存 (${selectedIds.size})",
                    enabled = selectedIds.isNotEmpty(),
                    cornerRadius = 999.dp,
                    minHeight = 40.dp,
                    contentPadding = PaddingValues(horizontal = 16.dp, vertical = 7.dp),
                    onClick = {
                        if (selectedIds.isNotEmpty()) {
                            onSubmit(selectedIds.toList())
                        }
                    }
                )
            }

            LazyColumn(
                modifier = Modifier
                    .fillMaxWidth()
                    .weight(1f, fill = false),
                verticalArrangement = Arrangement.spacedBy(8.dp),
                contentPadding = PaddingValues(bottom = 16.dp)
            ) {
                items(candidates, key = { "mgr_${it.id}" }) { friend ->
                    val checked = selectedIds.contains(friend.id)
                    MemberSelectableRow(
                        friend = friend,
                        allBots = allBots,
                        checked = checked,
                        onToggle = {
                            if (checked) {
                                if (selectedIds.size > 1) selectedIds.remove(friend.id)
                            } else {
                                selectedIds.add(friend.id)
                            }
                        }
                    )
                }
            }
        }
    }
}

internal fun resolveModelOptionDisplayName(
    activeModel: String,
    modelOptions: List<ModelOptionItem>
): String {
    val clean = normalizeModelIdForSinglePool(activeModel)
    modelOptions.firstOrNull { it.id == clean }?.let { opt ->
        return opt.model.takeIf { it.isNotBlank() && !opt.isAuto }
            ?: if (opt.isAuto || opt.id == "auto") "自动 · 统一模型池" else opt.label
    }
    return when {
        clean.isEmpty() || clean == "auto" -> "自动 · 统一模型池"
        clean.contains("::") -> clean.substringAfter("::")
        else -> clean
    }
}

/**
 * 7. Unified Single-Pool Model Picker Sheet ("切换推理模型").
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ModelPickerSheet(
    activeModel: String,
    modelOptions: List<ModelOptionItem>,
    onSelect: (String) -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val theme = BotHubThemeTokens
    val normalizedActive = remember(activeModel) { normalizeModelIdForSinglePool(activeModel) }
    val effectiveOptions = remember(modelOptions) {
        modelOptions.ifEmpty {
            listOf(
                ModelOptionItem(
                    id = "auto",
                    label = "自动路由 · 统一模型池",
                    isAuto = true,
                    provider = "pool",
                    model = "auto",
                    available = true
                )
            )
        }
    }
    val sections = remember(effectiveOptions) {
        listOf(
            "自动路由" to effectiveOptions.filter { it.isAuto || it.id == "auto" },
            "统一模型池" to effectiveOptions.filter { !it.isAuto && it.id != "auto" }
        ).filter { it.second.isNotEmpty() }
    }

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
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            Text(
                text = "切换推理模型",
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )
            Text(
                text = "按统一模型池优先级与健康状态自动故障转移，或指定固定模型",
                fontSize = 12.sp,
                color = theme.textMuted
            )

            LazyColumn(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(360.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                sections.forEach { (sectionTitle, itemsList) ->
                    item(key = "hdr_$sectionTitle") {
                        VesperSectionHeader(
                            title = sectionTitle,
                            count = itemsList.size,
                            modifier = Modifier.padding(top = 6.dp, bottom = 2.dp)
                        )
                    }
                    items(itemsList, key = { it.id }) { opt ->
                        val selected = normalizedActive == opt.id
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .sizeIn(minHeight = 54.dp)
                                .clip(RoundedCornerShape(14.dp))
                                .background(if (selected) theme.accentSoft else theme.surfaceCard)
                                .border(
                                    width = if (selected) 1.2.dp else 1.dp,
                                    color = if (selected) theme.accentMuted else theme.border,
                                    shape = RoundedCornerShape(14.dp)
                                )
                                .clickable { onSelect(opt.id) }
                                .padding(horizontal = 14.dp, vertical = 11.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(10.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(8.dp)
                                    .clip(CircleShape)
                                    .background(
                                        when {
                                            !opt.available -> theme.textMuted
                                            opt.isAuto || opt.id == "auto" -> theme.accent
                                            opt.isLight -> theme.info
                                            else -> theme.success
                                        }
                                    )
                            )
                            Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                                ) {
                                    Text(
                                        text = opt.label,
                                        fontSize = 14.sp,
                                        fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                                        color = theme.textPrimary,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        modifier = Modifier.weight(1f, fill = false)
                                    )
                                    if (opt.isLight) {
                                        VesperTag(text = "轻量", toneColor = theme.info)
                                    }
                                }
                                if (!opt.isAuto && opt.id != "auto" && opt.provider.isNotBlank()) {
                                    Text(
                                        text = "${opt.provider} · ${opt.model.ifBlank { opt.id }}",
                                        fontFamily = FontFamily.Monospace,
                                        fontSize = 11.sp,
                                        color = theme.textMuted,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis
                                    )
                                }
                            }
                            if (selected) {
                                Icon(
                                    imageVector = Icons.Default.Check,
                                    contentDescription = "已选中",
                                    tint = theme.accentForeground,
                                    modifier = Modifier.size(18.dp)
                                )
                            }
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(6.dp))
        }
    }
}
