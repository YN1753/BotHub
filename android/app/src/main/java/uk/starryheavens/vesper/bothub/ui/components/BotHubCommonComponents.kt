package uk.starryheavens.vesper.bothub.ui.components

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.BookmarkBorder
import androidx.compose.material.icons.filled.ChatBubbleOutline
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Inbox
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.UrlUtils
import uk.starryheavens.vesper.bothub.navigation.BotHubTab
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.GrokBotGroupCluster

@Composable
fun BotAvatarOrCluster(
    bot: BotItem?,
    allBots: List<BotItem>,
    state: String = bot?.state ?: "idle",
    size: Dp = 36.dp,
    interactive: Boolean = false,
    isListening: Boolean = false,
    activeSpeakerId: String = "",
    defaultStyle: String = "butler:gold",
    defaultSeedKey: String = "vesper_bot",
    modifier: Modifier = Modifier
) {
    if (bot?.isGroup == true) {
        GrokBotGroupCluster(
            members = bot.resolveGroupMembers(allBots),
            fallbackStyle = bot.avatarStyle,
            state = state,
            activeSpeakerId = activeSpeakerId,
            size = size,
            interactive = interactive,
            modifier = modifier
        )
    } else {
        GrokBotAvatar(
            avatarStyle = bot?.avatarStyle ?: defaultStyle,
            state = state,
            size = size,
            interactive = interactive,
            isListening = isListening,
            seedKey = bot?.id ?: defaultSeedKey,
            modifier = modifier
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BotHubTopBar(
    activeBot: BotItem?,
    allBots: List<BotItem>,
    isStreaming: Boolean,
    serverUrl: String,
    onRefresh: () -> Unit,
    onOpenSettings: () -> Unit
) {
    TopAppBar(
        title = {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                BotAvatarOrCluster(
                    bot = activeBot,
                    allBots = allBots,
                    state = if (isStreaming) "working" else (activeBot?.state ?: "idle"),
                    size = 30.dp,
                    interactive = true,
                    defaultSeedKey = "vesper_top"
                )
                Column {
                    Text(
                        text = "Vesper BotHub",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold
                    )
                    Text(
                        text = UrlUtils.formatDisplayHost(serverUrl),
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
        },
        actions = {
            IconButton(
                onClick = onRefresh,
                modifier = Modifier.sizeIn(minWidth = 48.dp, minHeight = 48.dp)
            ) {
                Icon(Icons.Default.Refresh, contentDescription = "刷新状态")
            }
            IconButton(
                onClick = onOpenSettings,
                modifier = Modifier.sizeIn(minWidth = 48.dp, minHeight = 48.dp)
            ) {
                Icon(Icons.Default.Settings, contentDescription = "连接与推送设置")
            }
        },
        colors = TopAppBarDefaults.topAppBarColors(
            containerColor = MaterialTheme.colorScheme.surface
        )
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BotHubBottomBar(
    selectedTab: BotHubTab,
    unreadInboxCount: Int,
    unreadStashCount: Int,
    onSelectTab: (BotHubTab) -> Unit
) {
    val haptics = LocalHapticFeedback.current
    NavigationBar(
        modifier = Modifier.navigationBarsPadding(),
        containerColor = MaterialTheme.colorScheme.surface
    ) {
        BotHubTab.entries.forEach { tab ->
            val badgeCount = when (tab) {
                BotHubTab.INBOX -> unreadInboxCount
                BotHubTab.STASH -> unreadStashCount
                else -> 0
            }
            val tabIcon = when (tab) {
                BotHubTab.AGENT -> Icons.Default.ChatBubbleOutline
                BotHubTab.INBOX -> Icons.Default.Inbox
                BotHubTab.STASH -> Icons.Default.BookmarkBorder
                BotHubTab.HUB -> Icons.Default.Dns
            }
            NavigationBarItem(
                selected = selectedTab == tab,
                onClick = {
                    haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                    onSelectTab(tab)
                },
                icon = {
                    if (badgeCount > 0) {
                        BadgedBox(badge = { Badge { Text(badgeCount.toString()) } }) {
                            Icon(tabIcon, contentDescription = tab.contentDescription)
                        }
                    } else {
                        Icon(tabIcon, contentDescription = tab.contentDescription)
                    }
                },
                label = { Text(tab.label) }
            )
        }
    }
}

@Composable
fun MetricMiniCard(
    label: String,
    value: String,
    modifier: Modifier = Modifier
) {
    Card(
        modifier = modifier,
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceVariant
        )
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            Text(
                text = label,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = value,
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.primary
            )
        }
    }
}

@Composable
fun ZeroClickCaptureRow(
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    title: String = "系统分享零点击秒收",
    subtitle: String = "开启后在任意应用点击系统分享将自动收入收藏匣并立即返回原应用",
    compact: Boolean = false,
    modifier: Modifier = Modifier
) {
    if (compact) {
        Row(
            modifier = modifier
                .fillMaxWidth()
                .sizeIn(minHeight = 48.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(
                modifier = Modifier
                    .weight(1f)
                    .padding(end = 12.dp)
            ) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.bodySmall,
                    fontWeight = FontWeight.Medium,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = subtitle,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Switch(
                checked = checked,
                onCheckedChange = onCheckedChange
            )
        }
    } else {
        ListItem(
            modifier = modifier,
            headlineContent = { Text(title) },
            supportingContent = { Text(subtitle) },
            trailingContent = {
                Switch(
                    checked = checked,
                    onCheckedChange = onCheckedChange
                )
            }
        )
    }
}
