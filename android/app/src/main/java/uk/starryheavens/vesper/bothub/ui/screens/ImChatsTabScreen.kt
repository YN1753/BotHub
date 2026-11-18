package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.PlatformTextStyle
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
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.GrokBotGroupCluster
import uk.starryheavens.vesper.bothub.ui.components.VesperBrandEmblem
import uk.starryheavens.vesper.bothub.ui.components.formatModuleBadge

/**
 * WeChat / QQ / GrokBot Style IM Conversation Roster ("消息" Tab).
 * Treats every AI Spirit and AI Group as a living contact/group conversation row,
 * alongside a pinned Official Service Channel ("服务通知") for automated intelligence pushes.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun ImChatsTabScreen(
    bots: List<BotItem>,
    notifications: List<NotificationItem>,
    activeBotId: String,
    isStreaming: Boolean,
    searchQuery: String,
    onSearchQueryChange: (String) -> Unit,
    onOpenChat: (String) -> Unit,
    onOpenServiceFeed: () -> Unit,
    onTogglePinBot: (BotItem) -> Unit,
    onOpenBotProfile: (BotItem) -> Unit,
    onCreateFriend: () -> Unit,
    onCreateGroup: () -> Unit,
    modifier: Modifier = Modifier
) {
    val haptics = LocalHapticFeedback.current
    val noPadStyle = remember { PlatformTextStyle(includeFontPadding = false) }

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
            "服务通知 情报 晨报 雷达 告警".contains(q) ||
            (latestNotification?.title?.lowercase()?.contains(q) == true)
    }

    Column(modifier = modifier.fillMaxSize()) {
        // IM Search Capsule (WeChat / QQ top search bar)
        Surface(
            shape = RoundedCornerShape(12.dp),
            color = Color(0xCC0B1022),
            border = BorderStroke(1.dp, Color(0x1FFFF8EE)),
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 14.dp, vertical = 8.dp)
                .sizeIn(minHeight = 38.dp)
        ) {
            Row(
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Icon(
                    Icons.Default.Search,
                    contentDescription = null,
                    tint = Color(0xFF64748B),
                    modifier = Modifier.size(16.dp)
                )
                BasicTextField(
                    value = searchQuery,
                    onValueChange = onSearchQueryChange,
                    singleLine = true,
                    textStyle = TextStyle(
                        color = Color(0xFFFFF8EE),
                        fontSize = 13.5.sp
                    ),
                    cursorBrush = SolidColor(Color(0xFFF2B880)),
                    modifier = Modifier.weight(1f),
                    decorationBox = { inner ->
                        Box(contentAlignment = Alignment.CenterStart) {
                            if (searchQuery.isEmpty()) {
                                Text(
                                    text = "搜索 AI 好友、群聊或近期消息...",
                                    fontSize = 13.sp,
                                    color = Color(0xFF64748B)
                                )
                            }
                            inner()
                        }
                    }
                )
                if (searchQuery.isNotEmpty()) {
                    Surface(
                        onClick = { onSearchQueryChange("") },
                        shape = CircleShape,
                        color = Color(0x24FFFFFF),
                        modifier = Modifier.size(18.dp)
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Icon(
                                Icons.Default.Close,
                                contentDescription = "清空搜索",
                                tint = Color(0xFFCBD5E1),
                                modifier = Modifier.size(11.dp)
                            )
                        }
                    }
                }
            }
        }

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(bottom = 20.dp)
        ) {
            // 1. Official Service Account Row ("服务通知" — WeChat Official Service Notice Row)
            if (showServiceChannel) {
                item(key = "__service_channel__") {
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(Color(0x4D0D1428))
                            .combinedClickable(
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
                            // Service Emblem Avatar + Unread Badge
                            Box(modifier = Modifier.size(50.dp)) {
                                Surface(
                                    shape = RoundedCornerShape(14.dp),
                                    color = Color(0xFF101830),
                                    border = BorderStroke(1.dp, Color(0x38F2B880)),
                                    modifier = Modifier.fillMaxSize()
                                ) {
                                    Box(contentAlignment = Alignment.Center) {
                                        VesperBrandEmblem(size = 34.dp)
                                    }
                                }
                                if (unreadServiceCount > 0) {
                                    ImUnreadBadge(
                                        count = unreadServiceCount,
                                        noPadStyle = noPadStyle,
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
                                        horizontalArrangement = Arrangement.spacedBy(6.dp)
                                    ) {
                                        Text(
                                            text = "服务通知",
                                            style = TextStyle(
                                                fontSize = 15.5.sp,
                                                fontWeight = FontWeight.SemiBold,
                                                color = Color(0xFFFFF8EE),
                                                platformStyle = noPadStyle
                                            )
                                        )
                                        Surface(
                                            shape = RoundedCornerShape(5.dp),
                                            color = Color(0x22A5B4FC),
                                            border = BorderStroke(0.5.dp, Color(0x4DA5B4FC))
                                        ) {
                                            Text(
                                                text = "订阅号",
                                                style = TextStyle(
                                                    fontSize = 9.5.sp,
                                                    fontWeight = FontWeight.Medium,
                                                    color = Color(0xFFC7D2FE),
                                                    platformStyle = noPadStyle
                                                ),
                                                modifier = Modifier.padding(horizontal = 5.dp, vertical = 1.5.dp)
                                            )
                                        }
                                    }

                                    if (latestNotification != null) {
                                        Text(
                                            text = formatImTime(latestNotification.createdAt),
                                            fontFamily = FontFamily.Monospace,
                                            fontSize = 11.sp,
                                            color = Color(0xFF64748B)
                                        )
                                    }
                                }

                                val servicePreview = buildAnnotatedString {
                                    if (unreadServiceCount > 0) {
                                        withStyle(SpanStyle(color = Color(0xFFF2B880), fontWeight = FontWeight.Medium)) {
                                            append("[${unreadServiceCount}条未读] ")
                                        }
                                    }
                                    if (latestNotification != null) {
                                        val (modName, _) = formatModuleBadge(latestNotification.moduleId)
                                        append("$modName: ${latestNotification.title}")
                                    } else {
                                        append("晨间简报 · 技术雷达 · 资产与探针告警实时直达")
                                    }
                                }

                                Text(
                                    text = servicePreview,
                                    fontSize = 13.sp,
                                    lineHeight = 18.sp,
                                    color = Color(0xFF94A3B8),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                        }

                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = Color(0x18FFF8EE),
                            modifier = Modifier.padding(start = 79.dp)
                        )
                    }
                }
            }

            // 2. Empty Roster Onboarding Card (if no bots exist yet)
            if (sortedBots.isEmpty()) {
                item(key = "__empty_im__") {
                    Surface(
                        shape = RoundedCornerShape(18.dp),
                        color = Color(0x990D1326),
                        border = BorderStroke(1.dp, Color(0x20FFF8EE)),
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 16.dp, vertical = 20.dp)
                    ) {
                        Column(
                            modifier = Modifier.padding(horizontal = 20.dp, vertical = 24.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(10.dp)
                        ) {
                            Text(
                                text = if (searchQuery.isNotBlank()) "未找到匹配的 AI 好友或群聊" else "还没有添加 AI 好友",
                                fontSize = 15.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFFFFF8EE)
                            )
                            Text(
                                text = "每个 AI 都可以拥有独立性格、职责与记忆，还能拉多个 AI 组建协作群聊",
                                fontSize = 12.5.sp,
                                lineHeight = 18.sp,
                                color = Color(0xFF94A3B8),
                                textAlign = TextAlign.Center
                            )
                            if (searchQuery.isBlank()) {
                                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                                    Surface(
                                        onClick = onCreateFriend,
                                        shape = RoundedCornerShape(999.dp),
                                        color = Color(0xFFF2B880)
                                    ) {
                                        Text(
                                            text = "+ 添加 AI 好友",
                                            fontSize = 12.5.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            color = Color(0xFF090C16),
                                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 8.dp)
                                        )
                                    }
                                    if (bots.size >= 2) {
                                        Surface(
                                            onClick = onCreateGroup,
                                            shape = RoundedCornerShape(999.dp),
                                            color = Color(0xFF131B33),
                                            border = BorderStroke(1.dp, Color(0x29FFF8EE))
                                        ) {
                                            Text(
                                                text = "发起群聊",
                                                fontSize = 12.5.sp,
                                                color = Color(0xFFE2E8F0),
                                                modifier = Modifier.padding(horizontal = 14.dp, vertical = 8.dp)
                                            )
                                        }
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
                val rowBg = if (bot.isPinned) Color(0x660E152A) else Color.Transparent

                Box(modifier = Modifier.fillMaxWidth()) {
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(rowBg)
                            .combinedClickable(
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
                            // Avatar (WeChat Rounded-Square Cluster for Groups, Living Spirit + Status Dot for Friends)
                            Box(modifier = Modifier.size(50.dp)) {
                                if (bot.isGroup) {
                                    Surface(
                                        shape = RoundedCornerShape(14.dp),
                                        color = Color(0xFF111932),
                                        border = BorderStroke(1.dp, Color(0x28FFF8EE)),
                                        modifier = Modifier.fillMaxSize()
                                    ) {
                                        Box(contentAlignment = Alignment.Center) {
                                            GrokBotGroupCluster(
                                                members = bot.resolveGroupMembers(bots),
                                                fallbackStyle = bot.avatarStyle,
                                                state = liveState,
                                                size = 44.dp,
                                                interactive = false
                                            )
                                        }
                                    }
                                } else {
                                    Surface(
                                        shape = RoundedCornerShape(14.dp),
                                        color = Color(0xFF0E152B),
                                        border = BorderStroke(1.dp, Color(0x22FFF8EE)),
                                        modifier = Modifier.fillMaxSize()
                                    ) {
                                        Box(contentAlignment = Alignment.Center) {
                                            GrokBotAvatar(
                                                avatarStyle = bot.avatarStyle,
                                                state = liveState,
                                                size = 44.dp,
                                                interactive = false,
                                                seedKey = bot.id
                                            )
                                        }
                                    }
                                    // Live Presence Dot (Online / Thinking / Working / Blocked)
                                    val dotColor = when (liveState) {
                                        "working" -> Color(0xFFF2B880)
                                        "thinking" -> Color(0xFFFBBF24)
                                        "blocked" -> Color(0xFFFB7185)
                                        else -> Color(0xFF34D399)
                                    }
                                    Box(
                                        modifier = Modifier
                                            .align(Alignment.BottomEnd)
                                            .offset(x = 2.dp, y = 2.dp)
                                            .size(12.dp)
                                            .background(Color(0xFF050811), CircleShape)
                                            .padding(2.dp)
                                            .background(dotColor, CircleShape)
                                    )
                                }

                                if (bot.unreadCount > 0) {
                                    ImUnreadBadge(
                                        count = bot.unreadCount,
                                        noPadStyle = noPadStyle,
                                        modifier = Modifier
                                            .align(Alignment.TopEnd)
                                            .offset(x = 4.dp, y = (-4).dp)
                                    )
                                }
                            }

                            // Name, Tag, Timestamp & Last Message Preview
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
                                                color = Color(0xFFFFF8EE),
                                                platformStyle = noPadStyle
                                            ),
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis
                                        )

                                        Surface(
                                            shape = RoundedCornerShape(5.dp),
                                            color = if (bot.isGroup) Color(0x24F2B880) else Color(0x1FFFFFFF),
                                            border = BorderStroke(
                                                0.5.dp,
                                                if (bot.isGroup) Color(0x59F2B880) else Color(0x28FFF8EE)
                                            )
                                        ) {
                                            Text(
                                                text = if (bot.isGroup) {
                                                    "群聊"
                                                } else {
                                                    bot.modeBadgeLabel()
                                                },
                                                style = TextStyle(
                                                    fontSize = 9.5.sp,
                                                    fontWeight = FontWeight.Medium,
                                                    color = if (bot.isGroup) Color(0xFFFFE2BE) else Color(0xFF94A3B8),
                                                    platformStyle = noPadStyle
                                                ),
                                                modifier = Modifier.padding(horizontal = 5.dp, vertical = 1.5.dp)
                                            )
                                        }

                                        if (bot.isPinned) {
                                            Text(
                                                text = "置顶",
                                                style = TextStyle(
                                                    fontSize = 9.5.sp,
                                                    color = Color(0xFFF2B880),
                                                    platformStyle = noPadStyle
                                                )
                                            )
                                        }
                                    }

                                    if (bot.updatedAt.isNotBlank()) {
                                        Text(
                                            text = formatImTime(bot.updatedAt),
                                            fontFamily = FontFamily.Monospace,
                                            fontSize = 11.sp,
                                            color = Color(0xFF64748B)
                                        )
                                    }
                                }

                                val snippet = bot.conversationSnippet(isStreamingThisBot)
                                val annotatedSnippet = buildAnnotatedString {
                                    when {
                                        isStreamingThisBot -> {
                                            withStyle(SpanStyle(color = Color(0xFFF2B880), fontWeight = FontWeight.Medium)) {
                                                append(snippet)
                                            }
                                        }
                                        bot.unreadCount > 0 -> {
                                            withStyle(SpanStyle(color = Color(0xFFF2B880), fontWeight = FontWeight.Medium)) {
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
                                    color = if (bot.unreadCount > 0) Color(0xFFCBD5E1) else Color(0xFF8C9AB6),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                        }

                        if (index < sortedBots.lastIndex) {
                            HorizontalDivider(
                                thickness = 0.5.dp,
                                color = Color(0x16FFF8EE),
                                modifier = Modifier.padding(start = 79.dp)
                            )
                        }
                    }

                    DropdownMenu(
                        expanded = showContextMenu,
                        onDismissRequest = { showContextMenu = false },
                        modifier = Modifier.background(Color(0xFF10172C))
                    ) {
                        DropdownMenuItem(
                            text = {
                                Text(
                                    text = if (bot.isPinned) "取消置顶" else "置顶该聊天",
                                    color = Color(0xFFFFF8EE)
                                )
                            },
                            onClick = {
                                showContextMenu = false
                                onTogglePinBot(bot)
                            }
                        )
                        DropdownMenuItem(
                            text = {
                                Text(
                                    text = if (bot.isGroup) "查看群资料" else "查看好友资料卡",
                                    color = Color(0xFFFFF8EE)
                                )
                            },
                            onClick = {
                                showContextMenu = false
                                onOpenBotProfile(bot)
                            }
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun ImUnreadBadge(
    count: Int,
    noPadStyle: PlatformTextStyle,
    modifier: Modifier = Modifier
) {
    Box(
        modifier = modifier
            .defaultMinSize(minWidth = 17.dp, minHeight = 17.dp)
            .height(17.dp)
            .background(Color(0xFFF43F5E), RoundedCornerShape(999.dp))
            .padding(horizontal = 4.5.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = if (count > 99) "99+" else count.toString(),
            style = TextStyle(
                fontSize = 10.sp,
                lineHeight = 11.sp,
                fontFamily = FontFamily.Monospace,
                fontWeight = FontWeight.Bold,
                color = Color.White,
                platformStyle = noPadStyle
            ),
            maxLines = 1
        )
    }
}
