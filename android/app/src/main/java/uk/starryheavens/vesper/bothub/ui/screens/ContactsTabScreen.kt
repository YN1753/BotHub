package uk.starryheavens.vesper.bothub.ui.screens

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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.GroupAdd
import androidx.compose.material.icons.filled.Info
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
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.RecessedSearchBar
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.SpiritSquircleAvatar
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.tactileClickable
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

@Composable
fun ContactsTabScreen(
    bots: List<BotItem>,
    onOpenProfile: (BotItem) -> Unit,
    onOpenChat: (String) -> Unit,
    onCreateFriend: () -> Unit,
    onCreateGroup: () -> Unit,
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
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 2.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            if (query.isBlank()) {
                item(key = "__top_entries__") {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        TactileSecondaryButton(
                            text = "添加 AI 好友",
                            leadingIcon = Icons.Default.PersonAddAlt1,
                            accent = true,
                            cornerRadius = 12.dp,
                            minHeight = 38.dp,
                            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                            onClick = onCreateFriend,
                            modifier = Modifier.weight(1f)
                        )
                        TactileSecondaryButton(
                            text = "发起群聊",
                            leadingIcon = Icons.Default.GroupAdd,
                            cornerRadius = 12.dp,
                            minHeight = 38.dp,
                            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                            onClick = onCreateGroup,
                            modifier = Modifier.weight(1f)
                        )
                    }
                }
            }

            if (groups.isNotEmpty()) {
                item(key = "__sec_groups__") {
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(
                            text = "群聊 ${groups.size}",
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
                            cornerRadius = 16.dp,
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            groups.forEachIndexed { index, group ->
                                val members = group.resolveGroupMembers(bots)
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .clickable { onOpenChat(group.id) }
                                        .padding(horizontal = 14.dp, vertical = 10.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                                ) {
                                    SpiritSquircleAvatar(
                                        bot = group,
                                        allBots = bots,
                                        containerSize = 42.dp,
                                        cornerRadius = 12.dp,
                                        animateIdle = true,
                                        onClick = { onOpenProfile(group) }
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
                                            text = "${members.size} 人协作群",
                                            fontSize = 12.sp,
                                            color = theme.textMuted,
                                            maxLines = 1,
                                            softWrap = false
                                        )
                                    }

                                    Box(
                                        modifier = Modifier
                                            .size(32.dp)
                                            .clip(RoundedCornerShape(9.dp))
                                            .tactileClickable(pressedScale = 0.90f, hapticOnTap = true) {
                                                onOpenProfile(group)
                                            },
                                        contentAlignment = Alignment.Center
                                    ) {
                                        Icon(
                                            imageVector = Icons.Default.Info,
                                            contentDescription = "群资料",
                                            tint = theme.textMuted,
                                            modifier = Modifier.size(17.dp)
                                        )
                                    }
                                }

                                if (index < groups.lastIndex) {
                                    HorizontalDivider(
                                        thickness = 0.5.dp,
                                        color = theme.divider,
                                        modifier = Modifier.padding(start = 68.dp, end = 14.dp)
                                    )
                                }
                            }
                        }
                    }
                }
            }

            if (friends.isNotEmpty()) {
                item(key = "__sec_friends__") {
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(
                            text = "好友 ${friends.size}",
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
                            cornerRadius = 16.dp,
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            friends.forEachIndexed { index, friend ->
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .clickable { onOpenChat(friend.id) }
                                        .padding(horizontal = 14.dp, vertical = 10.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                                ) {
                                    SpiritSquircleAvatar(
                                        bot = friend,
                                        allBots = bots,
                                        containerSize = 42.dp,
                                        cornerRadius = 12.dp,
                                        showPresenceDot = true,
                                        animateIdle = true,
                                        onClick = { onOpenProfile(friend) }
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

                                    Box(
                                        modifier = Modifier
                                            .size(32.dp)
                                            .clip(RoundedCornerShape(9.dp))
                                            .tactileClickable(pressedScale = 0.90f, hapticOnTap = true) {
                                                onOpenProfile(friend)
                                            },
                                        contentAlignment = Alignment.Center
                                    ) {
                                        Icon(
                                            imageVector = Icons.Default.Info,
                                            contentDescription = "查看资料",
                                            tint = theme.textMuted,
                                            modifier = Modifier.size(17.dp)
                                        )
                                    }
                                }

                                if (index < friends.lastIndex) {
                                    HorizontalDivider(
                                        thickness = 0.5.dp,
                                        color = theme.divider,
                                        modifier = Modifier.padding(start = 68.dp, end = 14.dp)
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
