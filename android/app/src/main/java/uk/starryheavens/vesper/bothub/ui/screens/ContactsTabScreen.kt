package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.GroupAdd
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.PersonAddAlt1
import androidx.compose.material.icons.outlined.PeopleOutline
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.ui.components.BotAvatar
import uk.starryheavens.vesper.bothub.ui.components.VesperButtonTone
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperDivider
import uk.starryheavens.vesper.bothub.ui.components.VesperEmptyState
import uk.starryheavens.vesper.bothub.ui.components.VesperIconButton
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSearchField
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSectionHeader
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
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
        VesperSearchField(
            value = query,
            onValueChange = { query = it },
            placeholder = "搜索 AI 好友、角色头衔或群聊",
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 10.dp)
        )

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 2.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            if (query.isBlank()) {
                item(key = "__top_entries__") {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        VesperSecondaryButton(
                            text = "添加 AI 好友",
                            leadingIcon = Icons.Default.PersonAddAlt1,
                            tone = VesperButtonTone.ACCENT,
                            cornerRadius = 12.dp,
                            minHeight = 46.dp,
                            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 8.dp),
                            onClick = onCreateFriend,
                            modifier = Modifier.weight(1f)
                        )
                        VesperSecondaryButton(
                            text = "发起协同群聊",
                            leadingIcon = Icons.Default.GroupAdd,
                            tone = VesperButtonTone.NEUTRAL,
                            cornerRadius = 12.dp,
                            minHeight = 46.dp,
                            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 8.dp),
                            onClick = onCreateGroup,
                            modifier = Modifier.weight(1f)
                        )
                    }
                }
            }

            if (groups.isEmpty() && friends.isEmpty()) {
                item(key = "__contacts_empty__") {
                    VesperEmptyState(
                        icon = Icons.Outlined.PeopleOutline,
                        title = if (query.isNotBlank()) "未找到匹配的联系人" else "通讯录为空",
                        subtitle = if (query.isNotBlank()) {
                            "请尝试更换搜索关键词"
                        } else {
                            "创建你的第一位专属 AI 好友，打造个人智能体协作团队"
                        },
                        actions = if (query.isBlank()) {
                            {
                                VesperPrimaryButton(
                                    text = "添加 AI 好友",
                                    leadingIcon = Icons.Default.PersonAddAlt1,
                                    cornerRadius = 999.dp,
                                    minHeight = 44.dp,
                                    onClick = onCreateFriend
                                )
                            }
                        } else null
                    )
                }
            }

            if (groups.isNotEmpty()) {
                item(key = "__sec_groups__") {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        VesperSectionHeader(title = "多智能体协同群聊", count = groups.size)
                        VesperCard(
                            cornerRadius = 16.dp,
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            groups.forEachIndexed { index, group ->
                                val members = remember(group.id, group.memberIds, bots) {
                                    group.resolveGroupMembers(bots)
                                }
                                val memberPreview = remember(members) {
                                    members.take(4).joinToString("、") { it.name }
                                }
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .sizeIn(minHeight = 64.dp)
                                        .clickable { onOpenChat(group.id) }
                                        .padding(start = 14.dp, end = 4.dp, top = 10.dp, bottom = 10.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                                ) {
                                    BotAvatar(
                                        bot = group,
                                        allBots = bots,
                                        containerSize = 44.dp,
                                        animateIdle = true,
                                        onClick = { onOpenProfile(group) }
                                    )

                                    Column(
                                        modifier = Modifier.weight(1f),
                                        verticalArrangement = Arrangement.spacedBy(4.dp)
                                    ) {
                                        Row(
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(6.dp)
                                        ) {
                                            Text(
                                                text = group.name,
                                                fontSize = 15.sp,
                                                fontWeight = FontWeight.SemiBold,
                                                color = theme.textPrimary,
                                                maxLines = 1,
                                                overflow = TextOverflow.Ellipsis,
                                                modifier = Modifier.weight(1f, fill = false)
                                            )
                                            VesperTag(text = group.groupModeLabel())
                                        }
                                        Text(
                                            text = if (memberPreview.isNotBlank()) {
                                                "${members.size} 人 · $memberPreview"
                                            } else {
                                                "${members.size} 人协作群"
                                            },
                                            fontSize = 12.sp,
                                            color = theme.textMuted,
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis
                                        )
                                    }

                                    VesperIconButton(
                                        icon = Icons.Default.Info,
                                        contentDescription = "查看群资料",
                                        filled = false,
                                        iconSize = 18.dp,
                                        onClick = { onOpenProfile(group) }
                                    )
                                }

                                if (index < groups.lastIndex) {
                                    VesperDivider(startIndent = 70.dp, endIndent = 14.dp)
                                }
                            }
                        }
                    }
                }
            }

            if (friends.isNotEmpty()) {
                item(key = "__sec_friends__") {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        VesperSectionHeader(title = "AI 好友", count = friends.size)
                        VesperCard(
                            cornerRadius = 16.dp,
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            friends.forEachIndexed { index, friend ->
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .sizeIn(minHeight = 64.dp)
                                        .clickable { onOpenChat(friend.id) }
                                        .padding(start = 14.dp, end = 4.dp, top = 10.dp, bottom = 10.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                                ) {
                                    BotAvatar(
                                        bot = friend,
                                        allBots = bots,
                                        containerSize = 44.dp,
                                        showPresenceDot = true,
                                        animateIdle = true,
                                        onClick = { onOpenProfile(friend) }
                                    )

                                    Column(
                                        modifier = Modifier.weight(1f),
                                        verticalArrangement = Arrangement.spacedBy(4.dp)
                                    ) {
                                        Row(
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(6.dp)
                                        ) {
                                            Text(
                                                text = friend.name,
                                                fontSize = 15.sp,
                                                fontWeight = FontWeight.SemiBold,
                                                color = theme.textPrimary,
                                                maxLines = 1,
                                                overflow = TextOverflow.Ellipsis,
                                                modifier = Modifier.weight(1f, fill = false)
                                            )
                                            val roleBadge = friend.roleTitle.trim().takeIf { it.isNotEmpty() && it != friend.name }
                                                ?: friend.modeBadgeLabel()
                                            VesperTag(
                                                text = roleBadge.take(12),
                                                bgColor = theme.surfaceInput,
                                                borderColor = theme.border,
                                                textColor = theme.textSecondary
                                            )
                                        }
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

                                    VesperIconButton(
                                        icon = Icons.Default.Info,
                                        contentDescription = "查看资料",
                                        filled = false,
                                        iconSize = 18.dp,
                                        onClick = { onOpenProfile(friend) }
                                    )
                                }

                                if (index < friends.lastIndex) {
                                    VesperDivider(startIndent = 70.dp, endIndent = 14.dp)
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
