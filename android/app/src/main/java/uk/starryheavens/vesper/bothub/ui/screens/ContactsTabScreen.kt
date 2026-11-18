package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material.icons.filled.ChatBubbleOutline
import androidx.compose.material.icons.filled.GroupAdd
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.NotificationsNone
import androidx.compose.material.icons.filled.PersonAddAlt1
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
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.ui.components.CrispUnreadBadge
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.RecessedSearchBar
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.SpiritSquircleAvatar
import uk.starryheavens.vesper.bothub.ui.components.specularRimBrush
import uk.starryheavens.vesper.bothub.ui.components.tactileClickable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape

/**
 * WeChat "通讯录" / QQ "好友" Flagship Directory Screen.
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
        RecessedSearchBar(
            value = query,
            onValueChange = { query = it },
            placeholder = "搜索 AI 好友昵称、专长或群聊...",
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)
        )

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            // Top WeChat-Style Functional Entries Card
            if (query.isBlank()) {
                item(key = "__top_entries__") {
                    SpecularGlassCard(
                        cornerRadius = 18.dp,
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        ContactActionEntryRow(
                            icon = Icons.Default.PersonAddAlt1,
                            accent = Color(0xFFF2B880),
                            title = "新的 AI 好友",
                            subtitle = "定制性格、说话口吻与专属工具",
                            onClick = onCreateFriend
                        )
                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = Color(0x14FFF8EE),
                            modifier = Modifier.padding(start = 66.dp, end = 14.dp)
                        )
                        ContactActionEntryRow(
                            icon = Icons.Default.GroupAdd,
                            accent = Color(0xFF818CF8),
                            title = "发起群聊",
                            subtitle = "拉多位 AI 好友进群分工或头脑风暴",
                            onClick = onCreateGroup
                        )
                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = Color(0x14FFF8EE),
                            modifier = Modifier.padding(start = 66.dp, end = 14.dp)
                        )
                        ContactActionEntryRow(
                            icon = Icons.Default.NotificationsNone,
                            accent = Color(0xFF34D399),
                            title = "服务通知订阅号",
                            subtitle = if (unreadInboxCount > 0) "$unreadInboxCount 条未读情报推送" else "晨间简报 · 技术雷达 · 异常告警",
                            badgeCount = unreadInboxCount,
                            onClick = onOpenServiceFeed
                        )
                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = Color(0x14FFF8EE),
                            modifier = Modifier.padding(start = 66.dp, end = 14.dp)
                        )
                        ContactActionEntryRow(
                            icon = Icons.Default.AutoAwesome,
                            accent = Color(0xFFC084FC),
                            title = "技能工坊与记忆库",
                            subtitle = "编排 AI 好友自定义技能与长期记忆",
                            onClick = { onOpenConsole("skills", "技能工坊与记忆库") }
                        )
                    }
                }
            }

            // Section 1: Group Chats Inset Card
            if (groups.isNotEmpty()) {
                item(key = "__sec_groups__") {
                    Column(verticalArrangement = Arrangement.spacedBy(7.dp)) {
                        Text(
                            text = "我的群聊 (${groups.size})",
                            style = TextStyle(
                                fontSize = 12.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFF8C9BB5),
                                platformStyle = NoFontPaddingStyle
                            ),
                            modifier = Modifier.padding(start = 4.dp)
                        )
                        SpecularGlassCard(
                            cornerRadius = 18.dp,
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            groups.forEachIndexed { index, group ->
                                val members = group.resolveGroupMembers(bots)
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .clickable { onOpenProfile(group) }
                                        .padding(horizontal = 14.dp, vertical = 11.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                                ) {
                                    SpiritSquircleAvatar(
                                        bot = group,
                                        allBots = bots,
                                        containerSize = 44.dp,
                                        cornerRadius = 13.dp
                                    )

                                    Column(
                                        modifier = Modifier.weight(1f),
                                        verticalArrangement = Arrangement.spacedBy(2.dp)
                                    ) {
                                        Text(
                                            text = group.name,
                                            fontSize = 15.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            color = Color(0xFFFFF8EE),
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis
                                        )
                                        Text(
                                            text = "${members.size}位AI成员 · ${group.groupModeLabel()}",
                                            fontSize = 12.sp,
                                            color = Color(0xFF8C9BB5),
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis
                                        )
                                    }

                                    QuickChatCircleButton(
                                        contentDescription = "进入群聊",
                                        onClick = { onOpenChat(group.id) }
                                    )
                                }

                                if (index < groups.lastIndex) {
                                    HorizontalDivider(
                                        thickness = 0.5.dp,
                                        color = Color(0x14FFF8EE),
                                        modifier = Modifier.padding(start = 70.dp, end = 14.dp)
                                    )
                                }
                            }
                        }
                    }
                }
            }

            // Section 2: AI Friends Inset Card
            if (friends.isNotEmpty()) {
                item(key = "__sec_friends__") {
                    Column(verticalArrangement = Arrangement.spacedBy(7.dp)) {
                        Text(
                            text = "AI 好友 (${friends.size})",
                            style = TextStyle(
                                fontSize = 12.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFF8C9BB5),
                                platformStyle = NoFontPaddingStyle
                            ),
                            modifier = Modifier.padding(start = 4.dp)
                        )
                        SpecularGlassCard(
                            cornerRadius = 18.dp,
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            friends.forEachIndexed { index, friend ->
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .clickable { onOpenProfile(friend) }
                                        .padding(horizontal = 14.dp, vertical = 11.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                                ) {
                                    SpiritSquircleAvatar(
                                        bot = friend,
                                        allBots = bots,
                                        containerSize = 44.dp,
                                        cornerRadius = 13.dp,
                                        showPresenceDot = true
                                    )

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
                                                fontWeight = FontWeight.SemiBold,
                                                color = Color(0xFFFFF8EE),
                                                maxLines = 1,
                                                overflow = TextOverflow.Ellipsis
                                            )
                                            Box(
                                                modifier = Modifier
                                                    .clip(RoundedCornerShape(5.dp))
                                                    .background(Color(0x16FFFFFF))
                                                    .border(0.5.dp, Color(0x22FFF8EE), RoundedCornerShape(5.dp))
                                                    .padding(horizontal = 5.dp, vertical = 1.dp)
                                            ) {
                                                Text(
                                                    text = friend.modeBadgeLabel(),
                                                    style = TextStyle(
                                                        fontSize = 9.5.sp,
                                                        color = Color(0xFFCBD5E1),
                                                        platformStyle = NoFontPaddingStyle
                                                    )
                                                )
                                            }
                                        }
                                        Text(
                                            text = friend.traits.ifBlank { friend.roleTitle }.ifBlank { "随时在线" },
                                            fontSize = 12.sp,
                                            color = Color(0xFF8493AE),
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis
                                        )
                                    }

                                    QuickChatCircleButton(
                                        contentDescription = "发消息",
                                        onClick = { onOpenChat(friend.id) }
                                    )
                                }

                                if (index < friends.lastIndex) {
                                    HorizontalDivider(
                                        thickness = 0.5.dp,
                                        color = Color(0x14FFF8EE),
                                        modifier = Modifier.padding(start = 70.dp, end = 14.dp)
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

@Composable
private fun QuickChatCircleButton(
    contentDescription: String,
    onClick: () -> Unit
) {
    Box(
        modifier = Modifier
            .size(34.dp)
            .tactileClickable(pressedScale = 0.90f, hapticOnTap = true, onClick = onClick)
            .clip(CircleShape)
            .background(
                Brush.verticalGradient(listOf(Color(0x2EF2B880), Color(0x18C88A4E))),
                CircleShape
            )
            .border(
                BorderStroke(
                    0.8.dp,
                    specularRimBrush(Color(0x66F2B880), Color(0x26F2B880), Color(0x0CF2B880))
                ),
                CircleShape
            ),
        contentAlignment = Alignment.Center
    ) {
        Icon(
            imageVector = Icons.Default.ChatBubbleOutline,
            contentDescription = contentDescription,
            tint = Color(0xFFFFE2BE),
            modifier = Modifier.size(15.dp)
        )
    }
}

@Composable
private fun ContactActionEntryRow(
    icon: ImageVector,
    accent: Color,
    title: String,
    subtitle: String,
    badgeCount: Int = 0,
    onClick: () -> Unit
) {
    val sq = RoundedCornerShape(12.dp)
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Box(
            modifier = Modifier
                .size(40.dp)
                .clip(sq)
                .background(
                    Brush.verticalGradient(
                        listOf(accent.copy(alpha = 0.24f), accent.copy(alpha = 0.10f))
                    ),
                    sq
                )
                .border(
                    BorderStroke(
                        0.75.dp,
                        specularRimBrush(accent.copy(alpha = 0.55f), accent.copy(alpha = 0.18f), Color.Transparent)
                    ),
                    sq
                ),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = accent,
                modifier = Modifier.size(20.dp)
            )
        }

        Column(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(2.dp)
        ) {
            Text(
                text = title,
                fontSize = 14.5.sp,
                fontWeight = FontWeight.SemiBold,
                color = Color(0xFFFFF8EE)
            )
            Text(
                text = subtitle,
                fontSize = 11.5.sp,
                color = Color(0xFF8C9BB5),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }

        if (badgeCount > 0) {
            CrispUnreadBadge(count = badgeCount)
        }

        Icon(
            imageVector = Icons.Default.KeyboardArrowRight,
            contentDescription = null,
            tint = Color(0xFF5D6D89),
            modifier = Modifier.size(18.dp)
        )
    }
}
