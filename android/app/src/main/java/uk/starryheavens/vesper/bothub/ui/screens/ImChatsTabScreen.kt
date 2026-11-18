package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AccountBox
import androidx.compose.material.icons.filled.DoneAll
import androidx.compose.material.icons.filled.PushPin
import androidx.compose.material.icons.outlined.DeleteOutline
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.data.formatImTime
import uk.starryheavens.vesper.bothub.ui.components.CrispTagPill
import uk.starryheavens.vesper.bothub.ui.components.CrispUnreadBadge
import uk.starryheavens.vesper.bothub.ui.components.FloatingGlassPopover
import uk.starryheavens.vesper.bothub.ui.components.GlassMenuAction
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.RecessedSearchBar
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.SpiritSquircleAvatar
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperBrandEmblem
import uk.starryheavens.vesper.bothub.ui.components.formatModuleBadge
import uk.starryheavens.vesper.bothub.ui.components.specularRimBrush
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

@OptIn(ExperimentalFoundationApi::class)
@Composable
fun ImChatsTabScreen(
    bots: List<BotItem>,
    notifications: List<NotificationItem>,
    draftsByBot: Map<String, String> = emptyMap(),
    activeBotId: String,
    isStreaming: Boolean,
    searchQuery: String,
    onSearchQueryChange: (String) -> Unit,
    onOpenChat: (String) -> Unit,
    onOpenServiceFeed: () -> Unit,
    onTogglePinBot: (BotItem) -> Unit,
    onMarkBotRead: (BotItem) -> Unit = {},
    onClearBotHistory: (BotItem) -> Unit = {},
    onOpenBotProfile: (BotItem) -> Unit,
    onCreateFriend: () -> Unit,
    onCreateGroup: () -> Unit,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val haptics = LocalHapticFeedback.current

    val sortedBots = remember(bots, searchQuery) {
        val q = searchQuery.trim().lowercase()
        bots.filter { b ->
            if (q.isEmpty()) true
            else {
                b.name.lowercase().contains(q) ||
                    b.roleTitle.lowercase().contains(q) ||
                    b.lastSummary.lowercase().contains(q)
            }
        }.sortedWith(
            compareByDescending<BotItem> { it.isPinned }
                .thenByDescending { it.unreadCount > 0 }
                .thenByDescending { it.updatedAt }
        )
    }

    val unreadServiceCount = remember(notifications) { notifications.count { it.isUnread } }
    val latestNotification = notifications.firstOrNull()
    val showServiceChannel = remember(searchQuery, latestNotification) {
        val q = searchQuery.trim().lowercase()
        q.isEmpty() ||
            "服务通知 简报 雷达 告警".contains(q) ||
            (latestNotification?.title?.lowercase()?.contains(q) == true)
    }

    Column(modifier = modifier.fillMaxSize()) {
        RecessedSearchBar(
            value = searchQuery,
            onValueChange = onSearchQueryChange,
            placeholder = "搜索",
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)
        )

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(bottom = 20.dp)
        ) {
            // 1. Official Service Feed Row ("服务通知")
            if (showServiceChannel) {
                item(key = "__service_channel__") {
                    val interaction = remember { MutableInteractionSource() }
                    val pressed by interaction.collectIsPressedAsState()
                    val rowBg by animateColorAsState(
                        targetValue = if (pressed) theme.accent.copy(alpha = 0.12f) else theme.cardTop.copy(alpha = if (theme.isLight) 0.55f else 0.30f),
                        animationSpec = tween(100),
                        label = "serviceRowBg"
                    )

                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(rowBg)
                            .combinedClickable(
                                interactionSource = interaction,
                                indication = null,
                                onClick = onOpenServiceFeed
                            )
                    ) {
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(horizontal = 16.dp, vertical = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(13.dp)
                        ) {
                            Box(modifier = Modifier.size(50.dp)) {
                                val sqShape = RoundedCornerShape(15.dp)
                                Box(
                                    modifier = Modifier
                                        .fillMaxSize()
                                        .shadow(if (theme.isLight) 2.dp else 4.dp, sqShape, ambientColor = Color(0x55000000))
                                        .clip(sqShape)
                                        .background(
                                            Brush.verticalGradient(
                                                listOf(theme.secondaryBtnTop, theme.secondaryBtnBottom)
                                            ),
                                            sqShape
                                        )
                                        .border(
                                            BorderStroke(
                                                0.8.dp,
                                                specularRimBrush(
                                                    theme.accent.copy(alpha = 0.45f),
                                                    theme.rimMid,
                                                    theme.rimSubtle
                                                )
                                            ),
                                            sqShape
                                        ),
                                    contentAlignment = Alignment.Center
                                ) {
                                    VesperBrandEmblem(size = 33.dp)
                                }
                                if (unreadServiceCount > 0) {
                                    CrispUnreadBadge(
                                        count = unreadServiceCount,
                                        modifier = Modifier
                                            .align(Alignment.TopEnd)
                                            .offset(x = 4.dp, y = (-4).dp)
                                    )
                                }
                            }

                            Column(
                                modifier = Modifier.weight(1f),
                                verticalArrangement = Arrangement.spacedBy(4.dp)
                            ) {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Row(
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                                        modifier = Modifier
                                            .weight(1f)
                                            .padding(end = 8.dp)
                                    ) {
                                        Text(
                                            text = "服务通知",
                                            style = TextStyle(
                                                fontSize = 15.5.sp,
                                                fontWeight = FontWeight.SemiBold,
                                                color = theme.textPrimary,
                                                platformStyle = NoFontPaddingStyle
                                            ),
                                            maxLines = 1,
                                            softWrap = false,
                                            overflow = TextOverflow.Ellipsis,
                                            modifier = Modifier.weight(1f, fill = false)
                                        )
                                    }

                                    if (latestNotification != null) {
                                        Text(
                                            text = formatImTime(latestNotification.createdAt),
                                            fontFamily = FontFamily.Monospace,
                                            fontSize = 11.sp,
                                            color = theme.textMuted,
                                            maxLines = 1,
                                            softWrap = false
                                        )
                                    }
                                }

                                val servicePreview = buildAnnotatedString {
                                    if (unreadServiceCount > 0) {
                                        withStyle(SpanStyle(color = theme.accent, fontWeight = FontWeight.Medium)) {
                                            append("[${unreadServiceCount}条] ")
                                        }
                                    }
                                    if (latestNotification != null) {
                                        val (modName, _) = formatModuleBadge(latestNotification.moduleId)
                                        append("$modName · ${latestNotification.title}")
                                    } else {
                                        append("暂无新通知")
                                    }
                                }

                                Text(
                                    text = servicePreview,
                                    fontSize = 13.sp,
                                    lineHeight = 18.sp,
                                    color = theme.textMuted,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                        }

                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = theme.divider,
                            modifier = Modifier.padding(start = 79.dp, end = 16.dp)
                        )
                    }
                }
            }

            // 2. Empty Roster State
            if (sortedBots.isEmpty()) {
                item(key = "__empty_im__") {
                    SpecularGlassCard(
                        cornerRadius = 20.dp,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 16.dp, vertical = 20.dp)
                    ) {
                        Column(
                            modifier = Modifier.padding(horizontal = 20.dp, vertical = 26.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(12.dp)
                        ) {
                            Text(
                                text = if (searchQuery.isNotBlank()) "无匹配结果" else "暂无聊天",
                                fontSize = 15.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textPrimary,
                                textAlign = TextAlign.Center
                            )
                            if (searchQuery.isBlank()) {
                                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                                    TactilePrimaryButton(
                                        text = "添加 AI 好友",
                                        cornerRadius = 999.dp,
                                        minHeight = 38.dp,
                                        onClick = onCreateFriend
                                    )
                                    if (bots.size >= 2) {
                                        TactileSecondaryButton(
                                            text = "发起群聊",
                                            cornerRadius = 999.dp,
                                            minHeight = 38.dp,
                                            onClick = onCreateGroup
                                        )
                                    }
                                }
                            }
                        }
                    }
                }
            }

            // 3. AI Friends & AI Group Chat Conversation Rows
            itemsIndexed(sortedBots, key = { _, b -> b.id }) { index, bot ->
                var showContextMenu by remember { mutableStateOf(false) }
                val isStreamingThisBot = isStreaming && activeBotId == bot.id
                val liveState = bot.liveState(isStreamingThisBot)
                val interaction = remember { MutableInteractionSource() }
                val pressed by interaction.collectIsPressedAsState()
                val rowBg by animateColorAsState(
                    targetValue = when {
                        pressed -> theme.accent.copy(alpha = 0.14f)
                        bot.isPinned -> theme.accent.copy(alpha = 0.06f)
                        else -> Color.Transparent
                    },
                    animationSpec = tween(100),
                    label = "chatRowBg"
                )

                Box(modifier = Modifier.fillMaxWidth()) {
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(rowBg)
                            .combinedClickable(
                                interactionSource = interaction,
                                indication = null,
                                onClick = { onOpenChat(bot.id) },
                                onLongClick = {
                                    haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                                    showContextMenu = true
                                }
                            )
                    ) {
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(horizontal = 16.dp, vertical = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(13.dp)
                        ) {
                            Box(modifier = Modifier.size(50.dp)) {
                                SpiritSquircleAvatar(
                                    bot = bot,
                                    allBots = bots,
                                    state = liveState,
                                    containerSize = 50.dp,
                                    cornerRadius = 15.dp,
                                    showPresenceDot = true
                                )

                                if (bot.unreadCount > 0) {
                                    CrispUnreadBadge(
                                        count = bot.unreadCount,
                                        modifier = Modifier
                                            .align(Alignment.TopEnd)
                                            .offset(x = 4.dp, y = (-4).dp)
                                    )
                                }
                            }

                            Column(
                                modifier = Modifier.weight(1f),
                                verticalArrangement = Arrangement.spacedBy(4.dp)
                            ) {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Row(
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                                        modifier = Modifier
                                            .weight(1f)
                                            .padding(end = 8.dp)
                                    ) {
                                        Text(
                                            text = bot.name,
                                            style = TextStyle(
                                                fontSize = 15.5.sp,
                                                fontWeight = FontWeight.SemiBold,
                                                color = theme.textPrimary,
                                                platformStyle = NoFontPaddingStyle
                                            ),
                                            maxLines = 1,
                                            softWrap = false,
                                            overflow = TextOverflow.Ellipsis,
                                            modifier = Modifier.weight(1f, fill = false)
                                        )

                                        if (bot.isGroup) {
                                            CrispTagPill(text = "群聊")
                                        }

                                        if (bot.isPinned) {
                                            Icon(
                                                imageVector = Icons.Default.PushPin,
                                                contentDescription = "已置顶",
                                                tint = theme.accent.copy(alpha = 0.72f),
                                                modifier = Modifier.size(11.dp)
                                            )
                                        }
                                    }

                                    if (bot.updatedAt.isNotBlank()) {
                                        Text(
                                            text = formatImTime(bot.updatedAt),
                                            fontFamily = FontFamily.Monospace,
                                            fontSize = 11.sp,
                                            color = theme.textMuted,
                                            maxLines = 1,
                                            softWrap = false
                                        )
                                    }
                                }

                                val draftText = draftsByBot[bot.id]?.trim().orEmpty()
                                val snippet = bot.conversationSnippet(isStreamingThisBot)
                                val annotatedSnippet = buildAnnotatedString {
                                    when {
                                        isStreamingThisBot -> {
                                            withStyle(SpanStyle(color = theme.accent, fontWeight = FontWeight.Medium)) {
                                                append(snippet)
                                            }
                                        }
                                        draftText.isNotEmpty() -> {
                                            withStyle(SpanStyle(color = Color(0xFFF43F5E), fontWeight = FontWeight.SemiBold)) {
                                                append("[草稿] ")
                                            }
                                            append(draftText.replace('\n', ' '))
                                        }
                                        bot.unreadCount > 0 -> {
                                            withStyle(SpanStyle(color = theme.accent, fontWeight = FontWeight.Medium)) {
                                                append("[${bot.unreadCount}条] ")
                                            }
                                            append(snippet)
                                        }
                                        else -> append(snippet)
                                    }
                                }

                                Text(
                                    text = annotatedSnippet,
                                    fontSize = 13.sp,
                                    lineHeight = 18.sp,
                                    color = if (bot.unreadCount > 0 || draftText.isNotEmpty()) theme.textSecondary else theme.textMuted,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                        }

                        if (index < sortedBots.lastIndex) {
                            HorizontalDivider(
                                thickness = 0.5.dp,
                                color = theme.divider,
                                modifier = Modifier.padding(start = 79.dp, end = 16.dp)
                            )
                        }
                    }

                    val contextActions = buildList {
                        if (bot.unreadCount > 0) {
                            add(
                                GlassMenuAction(
                                    label = "标为已读",
                                    icon = Icons.Default.DoneAll,
                                    iconTint = Color(0xFF34D399),
                                    onClick = { onMarkBotRead(bot) }
                                )
                            )
                        }
                        add(
                            GlassMenuAction(
                                label = if (bot.isPinned) "取消置顶" else "置顶聊天",
                                icon = Icons.Default.PushPin,
                                iconTint = Color(0xFFF2B880),
                                onClick = { onTogglePinBot(bot) }
                            )
                        )
                        add(
                            GlassMenuAction(
                                label = if (bot.isGroup) "群聊资料" else "好友资料",
                                icon = Icons.Default.AccountBox,
                                iconTint = Color(0xFFA5B4FC),
                                onClick = { onOpenBotProfile(bot) }
                            )
                        )
                        add(
                            GlassMenuAction(
                                label = "清空记录",
                                icon = Icons.Outlined.DeleteOutline,
                                iconTint = Color(0xFFFB7185),
                                onClick = { onClearBotHistory(bot) }
                            )
                        )
                    }

                    FloatingGlassPopover(
                        expanded = showContextMenu,
                        onDismissRequest = { showContextMenu = false },
                        alignment = Alignment.TopEnd,
                        offsetX = (-14).dp,
                        offsetY = 44.dp,
                        items = contextActions
                    )
                }
            }
        }
    }
}
