package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.combinedClickable
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
import androidx.compose.material.icons.outlined.Forum
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.data.formatImTime
import uk.starryheavens.vesper.bothub.ui.components.BotAvatar
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.VesperBadge
import uk.starryheavens.vesper.bothub.ui.components.VesperBrandEmblem
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperDivider
import uk.starryheavens.vesper.bothub.ui.components.VesperEmptyState
import uk.starryheavens.vesper.bothub.ui.components.VesperMenuAction
import uk.starryheavens.vesper.bothub.ui.components.VesperPopoverMenu
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSearchField
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSectionHeader
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.formatModuleBadge
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
    val (pinnedBots, regularBots) = remember(bots, searchQuery) {
        val q = searchQuery.trim().lowercase()
        val matched = bots.filter { b ->
            if (q.isEmpty()) true
            else {
                b.name.lowercase().contains(q) ||
                    b.roleTitle.lowercase().contains(q) ||
                    b.lastSummary.lowercase().contains(q)
            }
        }
        val order = compareByDescending<BotItem> { it.unreadCount > 0 }
            .thenByDescending { it.updatedAt }
        val pinned = matched.filter { it.isPinned }.sortedWith(order)
        val regular = matched.filter { !it.isPinned }.sortedWith(order)
        pinned to regular
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
        VesperSearchField(
            value = searchQuery,
            onValueChange = onSearchQueryChange,
            placeholder = "搜索对话、好友或摘要",
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 10.dp)
        )

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(bottom = 24.dp)
        ) {
            if (showServiceChannel) {
                item(key = "__service_channel__", contentType = "service") {
                    ServiceChannelBannerRow(
                        unreadCount = unreadServiceCount,
                        latestNotification = latestNotification,
                        onClick = onOpenServiceFeed,
                        modifier = Modifier.padding(start = 16.dp, end = 16.dp, bottom = 10.dp)
                    )
                }
            }

            if (pinnedBots.isEmpty() && regularBots.isEmpty()) {
                item(key = "__empty_im__", contentType = "empty") {
                    VesperEmptyState(
                        icon = Icons.Outlined.Forum,
                        title = if (searchQuery.isNotBlank()) "未找到匹配的对话" else "还没有进行中的对话",
                        subtitle = if (searchQuery.isNotBlank()) {
                            "尝试切换关键词或清空搜索条件"
                        } else {
                            "添加专属 AI 好友或组建多智能体协作群聊即可开始"
                        },
                        actions = if (searchQuery.isBlank()) {
                            {
                                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                                    VesperPrimaryButton(
                                        text = "添加 AI 好友",
                                        cornerRadius = 999.dp,
                                        minHeight = 44.dp,
                                        onClick = onCreateFriend
                                    )
                                    if (bots.size >= 2) {
                                        VesperSecondaryButton(
                                            text = "发起群聊",
                                            cornerRadius = 999.dp,
                                            minHeight = 44.dp,
                                            onClick = onCreateGroup
                                        )
                                    }
                                }
                            }
                        } else null,
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 16.dp)
                    )
                }
            }

            if (pinnedBots.isNotEmpty()) {
                item(key = "__hdr_pinned__", contentType = "section") {
                    VesperSectionHeader(
                        title = "置顶对话",
                        count = pinnedBots.size,
                        modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 6.dp)
                    )
                }
                itemsIndexed(
                    items = pinnedBots,
                    key = { _, b -> "pin_${b.id}" },
                    contentType = { _, _ -> "chat_row" }
                ) { index, bot ->
                    ConversationListRow(
                        bot = bot,
                        allBots = bots,
                        draftText = draftsByBot[bot.id]?.trim().orEmpty(),
                        isStreamingThisBot = isStreaming && activeBotId == bot.id,
                        showDivider = index < pinnedBots.lastIndex,
                        pinnedSurface = true,
                        onOpenChat = { onOpenChat(bot.id) },
                        onMarkRead = { onMarkBotRead(bot) },
                        onTogglePin = { onTogglePinBot(bot) },
                        onOpenProfile = { onOpenBotProfile(bot) },
                        onClearHistory = { onClearBotHistory(bot) }
                    )
                }
            }

            if (regularBots.isNotEmpty()) {
                if (pinnedBots.isNotEmpty()) {
                    item(key = "__hdr_recent__", contentType = "section") {
                        VesperSectionHeader(
                            title = "最近对话",
                            count = regularBots.size,
                            modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 14.dp, bottom = 6.dp)
                        )
                    }
                }
                itemsIndexed(
                    items = regularBots,
                    key = { _, b -> b.id },
                    contentType = { _, _ -> "chat_row" }
                ) { index, bot ->
                    ConversationListRow(
                        bot = bot,
                        allBots = bots,
                        draftText = draftsByBot[bot.id]?.trim().orEmpty(),
                        isStreamingThisBot = isStreaming && activeBotId == bot.id,
                        showDivider = index < regularBots.lastIndex,
                        pinnedSurface = false,
                        onOpenChat = { onOpenChat(bot.id) },
                        onMarkRead = { onMarkBotRead(bot) },
                        onTogglePin = { onTogglePinBot(bot) },
                        onOpenProfile = { onOpenBotProfile(bot) },
                        onClearHistory = { onClearBotHistory(bot) }
                    )
                }
            }
        }
    }
}

@Composable
private fun ServiceChannelBannerRow(
    unreadCount: Int,
    latestNotification: NotificationItem?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    VesperCard(
        cornerRadius = 16.dp,
        elevated = unreadCount > 0,
        accentTone = if (unreadCount > 0) theme.accent else null,
        onClick = onClick,
        modifier = modifier.fillMaxWidth()
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 14.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(modifier = Modifier.size(46.dp)) {
                val emblemShape = RoundedCornerShape(13.dp)
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .clip(emblemShape)
                        .background(theme.surfaceControl, emblemShape)
                        .border(1.dp, theme.border, emblemShape),
                    contentAlignment = Alignment.Center
                ) {
                    VesperBrandEmblem(size = 30.dp)
                }
                if (unreadCount > 0) {
                    VesperBadge(
                        count = unreadCount,
                        containerColor = theme.danger,
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
                                fontSize = 15.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textPrimary,
                                platformStyle = NoFontPaddingStyle
                            ),
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis
                        )
                        if (latestNotification != null) {
                            val (modName, modColor) = formatModuleBadge(latestNotification.moduleId)
                            VesperTag(text = modName, toneColor = modColor)
                        }
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

                val servicePreview = remember(unreadCount, latestNotification, theme.accentForeground) {
                    buildAnnotatedString {
                        if (unreadCount > 0) {
                            withStyle(SpanStyle(color = theme.accentForeground, fontWeight = FontWeight.SemiBold)) {
                                append("[${unreadCount}条未读] ")
                            }
                        }
                        if (latestNotification != null) {
                            append(latestNotification.title)
                        } else {
                            append("系统简报、雷达动态与探针告警将汇总于此")
                        }
                    }
                }

                Text(
                    text = servicePreview,
                    fontSize = 13.sp,
                    lineHeight = 18.sp,
                    color = if (unreadCount > 0) theme.textSecondary else theme.textMuted,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun ConversationListRow(
    bot: BotItem,
    allBots: List<BotItem>,
    draftText: String,
    isStreamingThisBot: Boolean,
    showDivider: Boolean,
    pinnedSurface: Boolean,
    onOpenChat: () -> Unit,
    onMarkRead: () -> Unit,
    onTogglePin: () -> Unit,
    onOpenProfile: () -> Unit,
    onClearHistory: () -> Unit
) {
    val theme = BotHubThemeTokens
    val haptics = LocalHapticFeedback.current
    var showContextMenu by remember { mutableStateOf(false) }
    val liveState = bot.liveState(isStreamingThisBot)
    val rowBg = if (pinnedSurface) theme.surfaceCard.copy(alpha = if (theme.isLight) 0.72f else 0.55f) else Color.Transparent

    Box(modifier = Modifier.fillMaxWidth()) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(rowBg)
                .combinedClickable(
                    onClick = onOpenChat,
                    onLongClick = {
                        haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                        showContextMenu = true
                    }
                )
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 13.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(13.dp)
            ) {
                Box(modifier = Modifier.size(50.dp)) {
                    BotAvatar(
                        bot = bot,
                        allBots = allBots,
                        state = liveState,
                        containerSize = 50.dp,
                        showPresenceDot = true,
                        animateIdle = true
                    )

                    if (bot.unreadCount > 0) {
                        VesperBadge(
                            count = bot.unreadCount,
                            containerColor = theme.danger,
                            modifier = Modifier
                                .align(Alignment.TopEnd)
                                .offset(x = 4.dp, y = (-4).dp)
                        )
                    }
                }

                Column(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(5.dp)
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
                                VesperTag(text = "群聊")
                            }

                            if (bot.isPinned) {
                                Icon(
                                    imageVector = Icons.Default.PushPin,
                                    contentDescription = "已置顶",
                                    tint = theme.textMuted,
                                    modifier = Modifier.size(12.dp)
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

                    val snippet = remember(bot.lastSummary, bot.traits, bot.roleTitle, bot.isGroup, isStreamingThisBot) {
                        bot.conversationSnippet(isStreamingThisBot)
                    }
                    val annotatedSnippet = remember(isStreamingThisBot, draftText, bot.unreadCount, snippet, theme.id) {
                        buildAnnotatedString {
                            when {
                                isStreamingThisBot -> {
                                    withStyle(SpanStyle(color = theme.accentForeground, fontWeight = FontWeight.Medium)) {
                                        append(snippet)
                                    }
                                }
                                draftText.isNotEmpty() -> {
                                    withStyle(SpanStyle(color = theme.danger, fontWeight = FontWeight.SemiBold)) {
                                        append("[草稿] ")
                                    }
                                    append(draftText.replace('\n', ' '))
                                }
                                bot.unreadCount > 0 -> {
                                    withStyle(SpanStyle(color = theme.accentForeground, fontWeight = FontWeight.Medium)) {
                                        append("[${bot.unreadCount}条] ")
                                    }
                                    append(snippet)
                                }
                                else -> append(snippet)
                            }
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

            if (showDivider) {
                VesperDivider(startIndent = 79.dp, endIndent = 16.dp)
            }
        }

        val contextActions = buildList {
            if (bot.unreadCount > 0) {
                add(
                    VesperMenuAction(
                        label = "标为已读",
                        icon = Icons.Default.DoneAll,
                        iconTint = theme.success,
                        onClick = onMarkRead
                    )
                )
            }
            add(
                VesperMenuAction(
                    label = if (bot.isPinned) "取消置顶" else "置顶聊天",
                    icon = Icons.Default.PushPin,
                    iconTint = theme.warning,
                    onClick = onTogglePin
                )
            )
            add(
                VesperMenuAction(
                    label = if (bot.isGroup) "群聊资料" else "好友资料",
                    icon = Icons.Default.AccountBox,
                    iconTint = theme.info,
                    onClick = onOpenProfile
                )
            )
            add(
                VesperMenuAction(
                    label = "清空记录",
                    icon = Icons.Outlined.DeleteOutline,
                    danger = true,
                    onClick = onClearHistory
                )
            )
        }

        VesperPopoverMenu(
            expanded = showContextMenu,
            onDismissRequest = { showContextMenu = false },
            alignment = Alignment.TopEnd,
            offsetX = (-14).dp,
            offsetY = 48.dp,
            items = contextActions
        )
    }
}
