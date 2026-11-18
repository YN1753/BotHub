package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material.icons.filled.ChatBubbleOutline
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.GroupAdd
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.NotificationsNone
import androidx.compose.material.icons.filled.PersonAddAlt1
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
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
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.PlatformTextStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.GrokBotGroupCluster

/**
 * WeChat "通讯录" / QQ "好友" Directory Screen.
 * Organizes quick friend/group creation entries, AI Group Chats, and Specialist AI Friends.
 */
@Composable
fun ContactsTabScreen(
    bots: List<BotItem>,
    unreadInboxCount: Int,
    onOpenProfile: (BotItem) -> Unit,
    onOpenChat: (String) -> Unit,
    onCreateFriend: () -> Unit,
    onCreateGroup: () -> Unit,
    onOpenServiceFeed: () -> Unit,
    onOpenConsole: (String, String) -> Unit,
    modifier: Modifier = Modifier
) {
    var query by remember { mutableStateOf("") }
    val noPadStyle = remember { PlatformTextStyle(includeFontPadding = false) }

    val filtered = remember(bots, query) {
        val q = query.trim().lowercase()
        if (q.isEmpty()) bots
        else bots.filter {
            it.name.lowercase().contains(q) ||
                it.roleTitle.lowercase().contains(q) ||
                it.traits.lowercase().contains(q)
        }
    }

    val groups = remember(filtered) { filtered.filter { it.isGroup } }
    val friends = remember(filtered) { filtered.filter { !it.isGroup } }

    Column(modifier = modifier.fillMaxSize()) {
        // Contacts Search Bar
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
                    value = query,
                    onValueChange = { query = it },
                    singleLine = true,
                    textStyle = TextStyle(
                        color = Color(0xFFFFF8EE),
                        fontSize = 13.5.sp
                    ),
                    cursorBrush = SolidColor(Color(0xFFF2B880)),
                    modifier = Modifier.weight(1f),
                    decorationBox = { inner ->
                        Box(contentAlignment = Alignment.CenterStart) {
                            if (query.isEmpty()) {
                                Text(
                                    text = "搜索 AI 好友昵称、职责或群聊...",
                                    fontSize = 13.sp,
                                    color = Color(0xFF64748B)
                                )
                            }
                            inner()
                        }
                    }
                )
                if (query.isNotEmpty()) {
                    Surface(
                        onClick = { query = "" },
                        shape = CircleShape,
                        color = Color(0x24FFFFFF),
                        modifier = Modifier.size(18.dp)
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Icon(
                                Icons.Default.Close,
                                contentDescription = "清空",
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
            contentPadding = PaddingValues(bottom = 24.dp)
        ) {
            // Top WeChat-Style Functional Entries
            if (query.isBlank()) {
                item(key = "__top_entries__") {
                    Surface(
                        shape = RoundedCornerShape(18.dp),
                        color = Color(0xB30D1326),
                        border = BorderStroke(1.dp, Color(0x1FFFF8EE)),
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 14.dp, vertical = 4.dp)
                    ) {
                        Column(modifier = Modifier.fillMaxWidth()) {
                            ContactActionEntryRow(
                                icon = Icons.Default.PersonAddAlt1,
                                iconBg = Color(0x2EF2B880),
                                iconTint = Color(0xFFFFE2BE),
                                title = "新的 AI 好友",
                                subtitle = "定制性格、说话口吻与专属工具",
                                onClick = onCreateFriend
                            )
                            HorizontalDivider(
                                thickness = 0.5.dp,
                                color = Color(0x16FFF8EE),
                                modifier = Modifier.padding(start = 66.dp)
                            )
                            ContactActionEntryRow(
                                icon = Icons.Default.GroupAdd,
                                iconBg = Color(0x2EA5B4FC),
                                iconTint = Color(0xFFC7D2FE),
                                title = "发起群聊",
                                subtitle = "拉多位 AI 好友进群分工或头脑风暴",
                                onClick = onCreateGroup
                            )
                            HorizontalDivider(
                                thickness = 0.5.dp,
                                color = Color(0x16FFF8EE),
                                modifier = Modifier.padding(start = 66.dp)
                            )
                            ContactActionEntryRow(
                                icon = Icons.Default.NotificationsNone,
                                iconBg = Color(0x2E34D399),
                                iconTint = Color(0xFFA7F3D0),
                                title = "服务通知订阅号",
                                subtitle = if (unreadInboxCount > 0) "$unreadInboxCount 条未读情报推送" else "晨间简报 · 技术雷达 · 探针告警",
                                badgeCount = unreadInboxCount,
                                onClick = onOpenServiceFeed
                            )
                            HorizontalDivider(
                                thickness = 0.5.dp,
                                color = Color(0x16FFF8EE),
                                modifier = Modifier.padding(start = 66.dp)
                            )
                            ContactActionEntryRow(
                                icon = Icons.Default.AutoAwesome,
                                iconBg = Color(0x2EC084FC),
                                iconTint = Color(0xFFE9D5FF),
                                title = "技能工坊与记忆库",
                                subtitle = "编排 AI 好友自定义技能与长期记忆",
                                onClick = { onOpenConsole("skills", "技能工坊与记忆库") }
                            )
                        }
                    }
                }
            }

            // Section 1: Group Chats
            if (groups.isNotEmpty()) {
                item(key = "__hdr_groups__") {
                    Text(
                        text = "我的群聊 (${groups.size})",
                        style = TextStyle(
                            fontSize = 12.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFF94A3B8),
                            platformStyle = noPadStyle
                        ),
                        modifier = Modifier.padding(start = 18.dp, end = 18.dp, top = 16.dp, bottom = 6.dp)
                    )
                }

                itemsIndexed(groups, key = { _, g -> "grp_${g.id}" }) { index, group ->
                    val members = group.resolveGroupMembers(bots)
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable { onOpenProfile(group) }
                    ) {
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(horizontal = 16.dp, vertical = 10.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(12.dp)
                        ) {
                            Surface(
                                shape = RoundedCornerShape(12.dp),
                                color = Color(0xFF111932),
                                border = BorderStroke(1.dp, Color(0x26FFF8EE)),
                                modifier = Modifier.size(44.dp)
                            ) {
                                Box(contentAlignment = Alignment.Center) {
                                    GrokBotGroupCluster(
                                        members = members,
                                        fallbackStyle = group.avatarStyle,
                                        state = group.state,
                                        size = 38.dp,
                                        interactive = false
                                    )
                                }
                            }

                            Column(
                                modifier = Modifier.weight(1f),
                                verticalArrangement = Arrangement.spacedBy(2.dp)
                            ) {
                                Text(
                                    text = group.name,
                                    fontSize = 15.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = Color(0xFFFFF8EE),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                                Text(
                                    text = "${members.size}位AI成员 · ${group.groupModeLabel()}",
                                    fontSize = 12.sp,
                                    color = Color(0xFF94A3B8),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }

                            Surface(
                                onClick = { onOpenChat(group.id) },
                                shape = CircleShape,
                                color = Color(0xFF10172C),
                                border = BorderStroke(1.dp, Color(0x24FFF8EE)),
                                modifier = Modifier.size(34.dp)
                            ) {
                                Box(contentAlignment = Alignment.Center) {
                                    Icon(
                                        Icons.Default.ChatBubbleOutline,
                                        contentDescription = "进入群聊",
                                        tint = Color(0xFFFFE2BE),
                                        modifier = Modifier.size(15.dp)
                                    )
                                }
                            }
                        }

                        if (index < groups.lastIndex) {
                            HorizontalDivider(
                                thickness = 0.5.dp,
                                color = Color(0x14FFF8EE),
                                modifier = Modifier.padding(start = 72.dp)
                            )
                        }
                    }
                }
            }

            // Section 2: AI Friends
            item(key = "__hdr_friends__") {
                Text(
                    text = "AI 好友 (${friends.size})",
                    style = TextStyle(
                        fontSize = 12.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = Color(0xFF94A3B8),
                        platformStyle = noPadStyle
                    ),
                    modifier = Modifier.padding(start = 18.dp, end = 18.dp, top = 16.dp, bottom = 6.dp)
                )
            }

            itemsIndexed(friends, key = { _, f -> "frd_${f.id}" }) { index, friend ->
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { onOpenProfile(friend) }
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 16.dp, vertical = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        Surface(
                            shape = RoundedCornerShape(12.dp),
                            color = Color(0xFF0E152B),
                            border = BorderStroke(1.dp, Color(0x22FFF8EE)),
                            modifier = Modifier.size(44.dp)
                        ) {
                            Box(contentAlignment = Alignment.Center) {
                                GrokBotAvatar(
                                    avatarStyle = friend.avatarStyle,
                                    state = friend.state,
                                    size = 38.dp,
                                    interactive = false,
                                    seedKey = friend.id
                                )
                            }
                        }

                        Column(
                            modifier = Modifier.weight(1f),
                            verticalArrangement = Arrangement.spacedBy(2.dp)
                        ) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                Text(
                                    text = friend.name,
                                    fontSize = 15.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = Color(0xFFFFF8EE),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                                Surface(
                                    shape = RoundedCornerShape(5.dp),
                                    color = Color(0x1AFFFFFF),
                                    border = BorderStroke(0.5.dp, Color(0x26FFF8EE))
                                ) {
                                    Text(
                                        text = friend.modeBadgeLabel(),
                                        style = TextStyle(
                                            fontSize = 9.5.sp,
                                            color = Color(0xFFCBD5E1),
                                            platformStyle = noPadStyle
                                        ),
                                        modifier = Modifier.padding(horizontal = 5.dp, vertical = 1.dp)
                                    )
                                }
                            }
                            Text(
                                text = friend.traits.ifBlank { friend.roleTitle }.ifBlank { "随时在线" },
                                fontSize = 12.sp,
                                color = Color(0xFF8C9AB6),
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                        }

                        Surface(
                            onClick = { onOpenChat(friend.id) },
                            shape = CircleShape,
                            color = Color(0xFF10172C),
                            border = BorderStroke(1.dp, Color(0x24FFF8EE)),
                            modifier = Modifier.size(34.dp)
                        ) {
                            Box(contentAlignment = Alignment.Center) {
                                Icon(
                                    Icons.Default.ChatBubbleOutline,
                                    contentDescription = "发消息",
                                    tint = Color(0xFFFFE2BE),
                                    modifier = Modifier.size(15.dp)
                                )
                            }
                        }
                    }

                    if (index < friends.lastIndex) {
                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = Color(0x14FFF8EE),
                            modifier = Modifier.padding(start = 72.dp)
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun ContactActionEntryRow(
    icon: ImageVector,
    iconBg: Color,
    iconTint: Color,
    title: String,
    subtitle: String,
    badgeCount: Int = 0,
    onClick: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Surface(
            shape = RoundedCornerShape(11.dp),
            color = iconBg,
            modifier = Modifier.size(40.dp)
        ) {
            Box(contentAlignment = Alignment.Center) {
                Icon(
                    icon,
                    contentDescription = null,
                    tint = iconTint,
                    modifier = Modifier.size(20.dp)
                )
            }
        }

        Column(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(2.dp)
        ) {
            Text(
                text = title,
                fontSize = 14.5.sp,
                fontWeight = FontWeight.Medium,
                color = Color(0xFFFFF8EE)
            )
            Text(
                text = subtitle,
                fontSize = 11.5.sp,
                color = Color(0xFF94A3B8),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }

        if (badgeCount > 0) {
            Surface(
                shape = RoundedCornerShape(999.dp),
                color = Color(0xFFF43F5E)
            ) {
                Text(
                    text = if (badgeCount > 99) "99+" else badgeCount.toString(),
                    fontFamily = FontFamily.Monospace,
                    fontSize = 10.sp,
                    fontWeight = FontWeight.Bold,
                    color = Color.White,
                    modifier = Modifier.padding(horizontal = 6.dp, vertical = 1.5.dp)
                )
            }
        }

        Icon(
            Icons.Default.KeyboardArrowRight,
            contentDescription = null,
            tint = Color(0xFF64748B),
            modifier = Modifier.size(18.dp)
        )
    }
}
