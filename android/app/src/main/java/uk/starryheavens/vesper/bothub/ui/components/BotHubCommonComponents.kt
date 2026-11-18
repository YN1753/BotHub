package uk.starryheavens.vesper.bothub.ui.components

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.BookmarkBorder
import androidx.compose.material.icons.filled.ChatBubbleOutline
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.DoneAll
import androidx.compose.material.icons.filled.Inbox
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
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
    defaultSeedKey: String = "bothub_bot",
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

@Composable
fun BotHubTopBar(
    selectedTab: BotHubTab,
    activeBot: BotItem?,
    allBots: List<BotItem>,
    isStreaming: Boolean,
    serverUrl: String,
    unreadInboxCount: Int = 0,
    showStashCapture: Boolean = false,
    onOpenSessions: () -> Unit = {},
    onNewSession: () -> Unit = {},
    onMarkAllRead: () -> Unit = {},
    onToggleStashCapture: () -> Unit = {},
    onRefresh: () -> Unit,
    onOpenSettings: () -> Unit
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .background(Color(0xD9070B16))
            .statusBarsPadding()
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 14.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            when (selectedTab) {
                BotHubTab.AGENT -> {
                    // Dynamic-Island Identity Capsule (tapping opens Session History)
                    Surface(
                        onClick = onOpenSessions,
                        shape = RoundedCornerShape(999.dp),
                        color = Color(0xFF0E1428),
                        border = BorderStroke(1.dp, Color(0x24FFF8EE)),
                        modifier = Modifier
                            .weight(1f, fill = false)
                            .sizeIn(minHeight = 42.dp)
                    ) {
                        Row(
                            modifier = Modifier.padding(start = 6.dp, end = 12.dp, top = 5.dp, bottom = 5.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            if (activeBot != null) {
                                BotAvatarOrCluster(
                                    bot = activeBot,
                                    allBots = allBots,
                                    state = if (isStreaming) "working" else activeBot.state,
                                    size = 30.dp,
                                    interactive = false,
                                    defaultSeedKey = "bothub_top"
                                )
                            } else {
                                VesperBrandEmblem(size = 28.dp)
                            }
                            Column(modifier = Modifier.widthIn(max = 168.dp)) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                                ) {
                                    Text(
                                        text = activeBot?.name ?: "BotHub",
                                        style = MaterialTheme.typography.titleSmall,
                                        fontWeight = FontWeight.SemiBold,
                                        color = Color(0xFFFFF8EE),
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis
                                    )
                                    Icon(
                                        Icons.Default.KeyboardArrowDown,
                                        contentDescription = "历史会话",
                                        tint = Color(0xFF94A3B8),
                                        modifier = Modifier.size(15.dp)
                                    )
                                }
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(5.dp)
                                ) {
                                    Box(
                                        modifier = Modifier
                                            .size(6.dp)
                                            .background(
                                                color = if (isStreaming) Color(0xFFF2B880) else Color(0xFF34D399),
                                                shape = CircleShape
                                            )
                                    )
                                    Text(
                                        text = if (isStreaming) {
                                            "正在推演"
                                        } else {
                                            activeBot?.roleTitle?.ifBlank { "轻触查看历史会话" } ?: "智能中枢"
                                        },
                                        style = MaterialTheme.typography.labelSmall,
                                        fontSize = 10.5.sp,
                                        color = if (isStreaming) Color(0xFFFFE2BE) else Color(0xFF94A3B8),
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis
                                    )
                                }
                            }
                        }
                    }
                }

                BotHubTab.INBOX -> {
                    HeaderBrandTitle(
                        title = "情报收件箱",
                        subtitle = if (unreadInboxCount > 0) "$unreadInboxCount 条未读推送" else "锁屏实时直连中"
                    )
                }

                BotHubTab.STASH -> {
                    HeaderBrandTitle(
                        title = "收藏与闪念",
                        subtitle = "系统分享秒收 · AI 自动提炼"
                    )
                }

                BotHubTab.HUB -> {
                    HeaderBrandTitle(
                        title = "资产与调度",
                        subtitle = UrlUtils.formatDisplayHost(serverUrl)
                    )
                }
            }

            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                when (selectedTab) {
                    BotHubTab.AGENT -> {
                        HeaderPillAction(
                            label = "新对话",
                            icon = Icons.Default.Add,
                            accent = true,
                            onClick = onNewSession
                        )
                    }
                    BotHubTab.INBOX -> {
                        if (unreadInboxCount > 0) {
                            HeaderPillAction(
                                label = "已读",
                                icon = Icons.Default.DoneAll,
                                accent = true,
                                onClick = onMarkAllRead
                            )
                        } else {
                            HeaderIconCircle(
                                icon = Icons.Default.Refresh,
                                contentDescription = "刷新收件箱",
                                onClick = onRefresh
                            )
                        }
                    }
                    BotHubTab.STASH -> {
                        HeaderPillAction(
                            label = if (showStashCapture) "收起" else "秒收",
                            icon = Icons.Default.Add,
                            accent = !showStashCapture,
                            onClick = onToggleStashCapture
                        )
                    }
                    BotHubTab.HUB -> {
                        HeaderIconCircle(
                            icon = Icons.Default.Refresh,
                            contentDescription = "刷新状态",
                            onClick = onRefresh
                        )
                    }
                }

                HeaderIconCircle(
                    icon = Icons.Default.Settings,
                    contentDescription = "连接与推送设置",
                    onClick = onOpenSettings
                )
            }
        }

        HorizontalDivider(thickness = 0.5.dp, color = Color(0x18FFF8EE))
    }
}

@Composable
private fun HeaderBrandTitle(
    title: String,
    subtitle: String
) {
    Row(
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        VesperBrandEmblem(size = 30.dp)
        Column {
            Text(
                text = title,
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                color = Color(0xFFFFF8EE)
            )
            Text(
                text = subtitle,
                style = MaterialTheme.typography.labelSmall,
                fontSize = 11.sp,
                color = Color(0xFF94A3B8),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }
    }
}

@Composable
private fun HeaderPillAction(
    label: String,
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    accent: Boolean,
    onClick: () -> Unit
) {
    Surface(
        onClick = onClick,
        shape = RoundedCornerShape(999.dp),
        color = if (accent) Color(0x22F2B880) else Color(0xFF10172C),
        border = BorderStroke(
            1.dp,
            if (accent) Color(0x5CF2B880) else Color(0x24FFF8EE)
        ),
        modifier = Modifier.sizeIn(minHeight = 38.dp)
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 7.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp)
        ) {
            Icon(
                icon,
                contentDescription = null,
                tint = if (accent) Color(0xFFFFE2BE) else Color(0xFFCBD5E1),
                modifier = Modifier.size(15.dp)
            )
            Text(
                text = label,
                style = MaterialTheme.typography.labelMedium,
                fontWeight = FontWeight.Medium,
                color = if (accent) Color(0xFFFFE2BE) else Color(0xFFE2E8F0)
            )
        }
    }
}

@Composable
private fun HeaderIconCircle(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    contentDescription: String,
    onClick: () -> Unit
) {
    Surface(
        onClick = onClick,
        shape = CircleShape,
        color = Color(0xFF10172C),
        border = BorderStroke(1.dp, Color(0x22FFF8EE)),
        modifier = Modifier.size(40.dp)
    ) {
        Box(contentAlignment = Alignment.Center) {
            Icon(
                icon,
                contentDescription = contentDescription,
                tint = Color(0xFFCBD5E1),
                modifier = Modifier.size(18.dp)
            )
        }
    }
}

/**
 * Custom compact obsidian-champagne bottom navigation dock (single inset, zero M3 double-padding bloat).
 */
@Composable
fun BotHubBottomBar(
    selectedTab: BotHubTab,
    unreadInboxCount: Int,
    unreadStashCount: Int,
    onSelectTab: (BotHubTab) -> Unit
) {
    val haptics = LocalHapticFeedback.current
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .background(Color(0xEB080C1A))
            .navigationBarsPadding()
    ) {
        HorizontalDivider(thickness = 0.5.dp, color = Color(0x1FFFF8EE))
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 10.dp, vertical = 6.dp),
            horizontalArrangement = Arrangement.SpaceAround,
            verticalAlignment = Alignment.CenterVertically
        ) {
            BotHubTab.entries.forEach { tab ->
                val isSelected = selectedTab == tab
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
                val pillBg by animateColorAsState(
                    targetValue = if (isSelected) Color(0x26F2B880) else Color.Transparent,
                    animationSpec = tween(180),
                    label = "navPillBg"
                )
                val fgColor by animateColorAsState(
                    targetValue = if (isSelected) Color(0xFFFFE2BE) else Color(0xFF8C9AB6),
                    animationSpec = tween(180),
                    label = "navFg"
                )

                Box(
                    modifier = Modifier
                        .weight(1f)
                        .sizeIn(minHeight = 48.dp)
                        .clip(RoundedCornerShape(16.dp))
                        .background(pillBg)
                        .clickable(
                            interactionSource = remember { MutableInteractionSource() },
                            indication = null,
                            role = Role.Tab
                        ) {
                            if (!isSelected) {
                                haptics.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                            }
                            onSelectTab(tab)
                        }
                        .padding(vertical = 5.dp),
                    contentAlignment = Alignment.Center
                ) {
                    Column(
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(2.dp)
                    ) {
                        Box(contentAlignment = Alignment.TopEnd) {
                            Icon(
                                tabIcon,
                                contentDescription = tab.contentDescription,
                                tint = fgColor,
                                modifier = Modifier
                                    .padding(horizontal = 8.dp)
                                    .size(20.dp)
                            )
                            if (badgeCount > 0) {
                                Box(
                                    modifier = Modifier
                                        .background(Color(0xFFF2B880), CircleShape)
                                        .padding(horizontal = 4.5.dp, vertical = 0.5.dp)
                                ) {
                                    Text(
                                        text = if (badgeCount > 99) "99+" else badgeCount.toString(),
                                        fontSize = 9.sp,
                                        fontFamily = FontFamily.Monospace,
                                        fontWeight = FontWeight.Bold,
                                        color = Color(0xFF090C16)
                                    )
                                }
                            }
                        }
                        Text(
                            text = tab.label,
                            fontSize = 11.sp,
                            fontWeight = if (isSelected) FontWeight.SemiBold else FontWeight.Medium,
                            color = fgColor,
                            maxLines = 1
                        )
                    }
                }
            }
        }
    }
}

@Composable
fun MetricMiniCard(
    label: String,
    value: String,
    accentColor: Color = MaterialTheme.colorScheme.primary,
    modifier: Modifier = Modifier
) {
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(18.dp),
        color = Color(0xCC0D1326),
        border = BorderStroke(1.dp, Color(0x22FFF8EE))
    ) {
        Column(modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp)) {
            Text(
                text = label,
                style = MaterialTheme.typography.labelSmall,
                fontSize = 11.sp,
                color = Color(0xFF94A3B8),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = value,
                fontFamily = FontFamily.Monospace,
                fontSize = 24.sp,
                fontWeight = FontWeight.Bold,
                color = accentColor
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
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = Color(0x990D1326),
        border = BorderStroke(1.dp, Color(0x1FFFF8EE))
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 14.dp, vertical = if (compact) 10.dp else 12.dp)
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
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Spacer(modifier = Modifier.height(2.dp))
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
    }
}
