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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
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
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

@Composable
fun ContactsTabScreen(
    bots: List<BotItem>,
    unreadInboxCount: Int,
    onOpenProfile: (BotItem) -> Unit,
    onOpenChat: (String) -> Unit,
    onCreateFriend: () -> Unit,
    onCreateGroup: () -> Unit,
    onOpenServiceFeed: () -> Unit,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
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
            placeholder = "搜索好友或群聊",
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)
        )

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            if (query.isBlank()) {
                item(key = "__top_entries__") {
                    SpecularGlassCard(
                        cornerRadius = 18.dp,
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        ContactActionEntryRow(
                            icon = Icons.Default.PersonAddAlt1,
                            accent = theme.accent,
                            title = "添加 AI 好友",
                            onClick = onCreateFriend
                        )
                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = theme.divider,
                            modifier = Modifier.padding(start = 64.dp, end = 14.dp)
                        )
                        ContactActionEntryRow(
                            icon = Icons.Default.GroupAdd,
                            accent = Color(0xFF6366F1),
                            title = "发起群聊",
                            onClick = onCreateGroup
                        )
                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = theme.divider,
                            modifier = Modifier.padding(start = 64.dp, end = 14.dp)
                        )
                        ContactActionEntryRow(
                            icon = Icons.Default.NotificationsNone,
                            accent = Color(0xFF10B981),
                            title = "服务通知",
                            badgeCount = unreadInboxCount,
                            onClick = onOpenServiceFeed
                        )
                    }
                }
            }

            if (groups.isNotEmpty()) {
                item(key = "__sec_groups__") {
                    Column(verticalArrangement = Arrangement.spacedBy(7.dp)) {
                        Text(
                            text = "群聊 (${groups.size})",
                            style = TextStyle(
                                fontSize = 12.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textMuted,
                                platformStyle = NoFontPaddingStyle
                            ),
                            maxLines = 1,
                            softWrap = false,
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
                                            color = theme.textPrimary,
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis
                                        )
                                        Text(
                                            text = "${members.size}人",
                                            fontSize = 12.sp,
                                            color = theme.textMuted,
                                            maxLines = 1,
                                            softWrap = false
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
                                        color = theme.divider,
                                        modifier = Modifier.padding(start = 70.dp, end = 14.dp)
                                    )
                                }
                            }
                        }
                    }
                }
            }

            if (friends.isNotEmpty()) {
                item(key = "__sec_friends__") {
                    Column(verticalArrangement = Arrangement.spacedBy(7.dp)) {
                        Text(
                            text = "好友 (${friends.size})",
                            style = TextStyle(
                                fontSize = 12.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textMuted,
                                platformStyle = NoFontPaddingStyle
                            ),
                            maxLines = 1,
                            softWrap = false,
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
                                        Text(
                                            text = friend.name,
                                            fontSize = 15.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            color = theme.textPrimary,
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis
                                        )
                                        val bio = friend.traits.ifBlank { friend.roleTitle }
                                        if (bio.isNotBlank()) {
                                            Text(
                                                text = bio,
                                                fontSize = 12.sp,
                                                color = theme.textMuted,
                                                maxLines = 1,
                                                overflow = TextOverflow.Ellipsis
                                            )
                                        }
                                    }

                                    QuickChatCircleButton(
                                        contentDescription = "发消息",
                                        onClick = { onOpenChat(friend.id) }
                                    )
                                }

                                if (index < friends.lastIndex) {
                                    HorizontalDivider(
                                        thickness = 0.5.dp,
                                        color = theme.divider,
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
    val theme = BotHubThemeTokens
    Box(
        modifier = Modifier
            .size(34.dp)
            .tactileClickable(pressedScale = 0.90f, hapticOnTap = true, onClick = onClick)
            .clip(CircleShape)
            .background(
                Brush.verticalGradient(
                    listOf(
                        theme.accent.copy(alpha = 0.18f),
                        theme.accentDeep.copy(alpha = 0.08f)
                    )
                ),
                CircleShape
            )
            .border(
                BorderStroke(
                    0.8.dp,
                    specularRimBrush(theme.accent.copy(alpha = 0.45f), theme.accent.copy(alpha = 0.16f), Color.Transparent)
                ),
                CircleShape
            ),
        contentAlignment = Alignment.Center
    ) {
        Icon(
            imageVector = Icons.Default.ChatBubbleOutline,
            contentDescription = contentDescription,
            tint = if (theme.isLight) theme.accentDeep else theme.accentLight,
            modifier = Modifier.size(15.dp)
        )
    }
}

@Composable
private fun ContactActionEntryRow(
    icon: ImageVector,
    accent: Color,
    title: String,
    badgeCount: Int = 0,
    onClick: () -> Unit
) {
    val theme = BotHubThemeTokens
    val sq = RoundedCornerShape(11.dp)
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
                .size(38.dp)
                .clip(sq)
                .background(
                    Brush.verticalGradient(
                        listOf(accent.copy(alpha = 0.20f), accent.copy(alpha = 0.08f))
                    ),
                    sq
                )
                .border(
                    BorderStroke(
                        0.75.dp,
                        specularRimBrush(accent.copy(alpha = 0.50f), accent.copy(alpha = 0.16f), Color.Transparent)
                    ),
                    sq
                ),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = accent,
                modifier = Modifier.size(19.dp)
            )
        }

        Text(
            text = title,
            fontSize = 15.sp,
            fontWeight = FontWeight.Medium,
            color = theme.textPrimary,
            maxLines = 1,
            softWrap = false,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f)
        )

        if (badgeCount > 0) {
            CrispUnreadBadge(count = badgeCount)
        }

        Icon(
            imageVector = Icons.Default.KeyboardArrowRight,
            contentDescription = null,
            tint = theme.textMuted,
            modifier = Modifier.size(18.dp)
        )
    }
}
