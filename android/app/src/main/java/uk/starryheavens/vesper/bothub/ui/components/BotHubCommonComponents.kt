package uk.starryheavens.vesper.bothub.ui.components

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material.icons.filled.BookmarkBorder
import androidx.compose.material.icons.filled.ChatBubble
import androidx.compose.material.icons.filled.ChatBubbleOutline
import androidx.compose.material.icons.filled.Dashboard
import androidx.compose.material.icons.filled.DoneAll
import androidx.compose.material.icons.filled.GroupAdd
import androidx.compose.material.icons.filled.People
import androidx.compose.material.icons.filled.PeopleOutline
import androidx.compose.material.icons.filled.PersonAddAlt1
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.outlined.Dashboard
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
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.navigation.BotHubTab
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

fun formatModuleBadge(moduleId: String): Pair<String, Color> {
    return when (moduleId.trim().lowercase()) {
        "morning" -> "晨报" to Color(0xFFD97706)
        "radar" -> "雷达" to Color(0xFF6366F1)
        "asset_watcher" -> "资产" to Color(0xFFF59E0B)
        "nezha_webhook" -> "告警" to Color(0xFFE11D48)
        "system_test" -> "自检" to Color(0xFF059669)
        else -> moduleId.ifBlank { "通知" } to Color(0xFF64748B)
    }
}

/**
 * Clean flagship top bar with strict >= 48dp touch targets and crisp hierarchy.
 */
@Composable
fun BotHubTopBar(
    selectedTab: BotHubTab,
    isRefreshing: Boolean = false,
    totalUnreadMessages: Int = 0,
    showStashCapture: Boolean = false,
    onCreateFriend: () -> Unit = {},
    onCreateGroup: () -> Unit = {},
    onQuickCaptureStash: () -> Unit = {},
    onMarkAllRead: () -> Unit = {},
    onToggleStashCapture: () -> Unit = {},
    onRefresh: () -> Unit,
    onOpenSettings: () -> Unit
) {
    val theme = BotHubThemeTokens
    var showPlusMenu by remember { mutableStateOf(false) }

    val headerTitle = when (selectedTab) {
        BotHubTab.AGENT -> when {
            isRefreshing -> "同步中..."
            totalUnreadMessages > 0 -> "消息 ($totalUnreadMessages)"
            else -> "消息"
        }
        BotHubTab.INBOX -> "好友"
        BotHubTab.STASH -> "收藏"
        BotHubTab.HUB -> "中枢"
    }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .background(theme.surfaceBar)
            .statusBarsPadding()
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(start = 16.dp, end = 8.dp, top = 4.dp, bottom = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Text(
                text = headerTitle,
                style = TextStyle(
                    fontSize = 21.sp,
                    fontWeight = FontWeight.Bold,
                    letterSpacing = (-0.3).sp,
                    color = theme.textPrimary,
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 1,
                softWrap = false,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier
                    .weight(1f, fill = false)
                    .padding(end = 12.dp)
            )

            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(2.dp)
            ) {
                when (selectedTab) {
                    BotHubTab.AGENT -> {
                        Box {
                            VesperIconButton(
                                icon = Icons.Default.Add,
                                contentDescription = "快捷菜单",
                                accent = showPlusMenu,
                                onClick = { showPlusMenu = true }
                            )
                            VesperPopoverMenu(
                                expanded = showPlusMenu,
                                onDismissRequest = { showPlusMenu = false },
                                offsetY = 48.dp,
                                items = listOf(
                                    VesperMenuAction(
                                        label = "发起群聊",
                                        icon = Icons.Default.GroupAdd,
                                        iconTint = theme.accentForeground,
                                        onClick = onCreateGroup
                                    ),
                                    VesperMenuAction(
                                        label = "添加 AI 好友",
                                        icon = Icons.Default.PersonAddAlt1,
                                        iconTint = theme.info,
                                        onClick = onCreateFriend
                                    ),
                                    VesperMenuAction(
                                        label = "新建收藏",
                                        icon = Icons.Default.BookmarkBorder,
                                        iconTint = theme.success,
                                        onClick = onQuickCaptureStash
                                    ),
                                    VesperMenuAction(
                                        label = "通知全部已读",
                                        icon = Icons.Default.DoneAll,
                                        iconTint = theme.textSecondary,
                                        onClick = onMarkAllRead
                                    ),
                                    VesperMenuAction(
                                        label = "设置与主题",
                                        icon = Icons.Default.Settings,
                                        iconTint = theme.warning,
                                        onClick = onOpenSettings
                                    )
                                )
                            )
                        }
                    }

                    BotHubTab.INBOX -> {
                        HeaderPillAction(
                            label = "加好友",
                            icon = Icons.Default.PersonAddAlt1,
                            accent = true,
                            onClick = onCreateFriend
                        )
                    }

                    BotHubTab.STASH -> {
                        HeaderPillAction(
                            label = if (showStashCapture) "收起" else "添加",
                            icon = Icons.Default.Add,
                            accent = !showStashCapture,
                            onClick = onToggleStashCapture
                        )
                    }

                    BotHubTab.HUB -> {
                        VesperIconButton(
                            icon = Icons.Default.Refresh,
                            contentDescription = "刷新中枢数据",
                            onClick = onRefresh
                        )
                        VesperIconButton(
                            icon = Icons.Default.Settings,
                            contentDescription = "设置与主题",
                            accent = true,
                            onClick = onOpenSettings
                        )
                    }
                }
            }
        }

        VesperDivider()
    }
}

@Composable
private fun HeaderPillAction(
    label: String,
    icon: ImageVector,
    accent: Boolean,
    onClick: () -> Unit
) {
    Box(
        modifier = Modifier
            .sizeIn(minHeight = 48.dp)
            .padding(end = 8.dp),
        contentAlignment = Alignment.Center
    ) {
        VesperSecondaryButton(
            text = label,
            leadingIcon = icon,
            tone = if (accent) VesperButtonTone.ACCENT else VesperButtonTone.NEUTRAL,
            cornerRadius = 999.dp,
            minHeight = 36.dp,
            contentPadding = PaddingValues(horizontal = 13.dp, vertical = 7.dp),
            onClick = onClick
        )
    }
}

/**
 * Clean bottom navigation dock with pill active indicator and edge-to-edge gesture padding.
 */
@Composable
fun BotHubBottomBar(
    selectedTab: BotHubTab,
    unreadMessagesCount: Int,
    unreadStashCount: Int,
    onSelectTab: (BotHubTab) -> Unit
) {
    val theme = BotHubThemeTokens
    val haptics = LocalHapticFeedback.current

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .background(theme.surfaceBar)
            .navigationBarsPadding()
    ) {
        VesperDivider()
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 8.dp, vertical = 4.dp),
            horizontalArrangement = Arrangement.SpaceEvenly,
            verticalAlignment = Alignment.CenterVertically
        ) {
            BotHubTab.entries.forEach { tab ->
                val isSelected = selectedTab == tab
                val badgeCount = when (tab) {
                    BotHubTab.AGENT -> unreadMessagesCount
                    BotHubTab.STASH -> unreadStashCount
                    else -> 0
                }
                val tabIcon = when (tab) {
                    BotHubTab.AGENT -> if (isSelected) Icons.Default.ChatBubble else Icons.Default.ChatBubbleOutline
                    BotHubTab.INBOX -> if (isSelected) Icons.Default.People else Icons.Default.PeopleOutline
                    BotHubTab.STASH -> if (isSelected) Icons.Default.Bookmark else Icons.Default.BookmarkBorder
                    BotHubTab.HUB -> if (isSelected) Icons.Default.Dashboard else Icons.Outlined.Dashboard
                }
                val fgColor by animateColorAsState(
                    targetValue = if (isSelected) theme.accentForeground else theme.textMuted,
                    animationSpec = tween(160),
                    label = "navFgColor"
                )
                val pillBg by animateColorAsState(
                    targetValue = if (isSelected) theme.accentSoft else Color.Transparent,
                    animationSpec = tween(160),
                    label = "navPillBg"
                )
                val iconScale by animateFloatAsState(
                    targetValue = if (isSelected) 1.05f else 1f,
                    animationSpec = spring(dampingRatio = 0.70f, stiffness = 560f),
                    label = "navIconScale"
                )

                Box(
                    modifier = Modifier
                        .weight(1f)
                        .height(54.dp)
                        .clickable(
                            interactionSource = remember { MutableInteractionSource() },
                            indication = null,
                            role = Role.Tab
                        ) {
                            if (!isSelected) {
                                haptics.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                            }
                            onSelectTab(tab)
                        },
                    contentAlignment = Alignment.Center
                ) {
                    Column(
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(3.dp)
                    ) {
                        Box(
                            modifier = Modifier
                                .width(52.dp)
                                .height(28.dp)
                                .clip(RoundedCornerShape(999.dp))
                                .background(pillBg),
                            contentAlignment = Alignment.Center
                        ) {
                            Icon(
                                imageVector = tabIcon,
                                contentDescription = tab.contentDescription,
                                tint = fgColor,
                                modifier = Modifier
                                    .size(20.dp)
                                    .graphicsLayer {
                                        scaleX = iconScale
                                        scaleY = iconScale
                                    }
                            )
                            if (badgeCount > 0) {
                                VesperBadge(
                                    count = badgeCount,
                                    containerColor = if (tab == BotHubTab.AGENT) theme.danger else theme.accent,
                                    contentColor = if (tab == BotHubTab.AGENT) Color.White else theme.onAccent,
                                    modifier = Modifier
                                        .align(Alignment.TopEnd)
                                        .offset(x = 2.dp, y = (-2).dp)
                                )
                            }
                        }
                        Text(
                            text = tab.label,
                            style = TextStyle(
                                fontSize = 11.sp,
                                lineHeight = 12.sp,
                                fontWeight = if (isSelected) FontWeight.SemiBold else FontWeight.Medium,
                                color = fgColor,
                                platformStyle = NoFontPaddingStyle
                            ),
                            maxLines = 1,
                            softWrap = false
                        )
                    }
                }
            }
        }
    }
}

@Composable
fun ZeroClickCaptureRow(
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    title: String = "系统分享直接收藏",
    subtitle: String = "分享内容自动存入收藏",
    compact: Boolean = false,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    VesperCard(
        modifier = modifier.fillMaxWidth(),
        cornerRadius = 14.dp,
        onClick = { onCheckedChange(!checked) }
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .sizeIn(minHeight = 56.dp)
                .padding(start = 14.dp, end = 6.dp, top = if (compact) 4.dp else 6.dp, bottom = if (compact) 4.dp else 6.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(
                modifier = Modifier
                    .weight(1f)
                    .padding(end = 10.dp),
                verticalArrangement = Arrangement.spacedBy(2.dp)
            ) {
                Text(
                    text = title,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.Medium,
                    color = theme.textPrimary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = subtitle,
                    fontSize = 12.sp,
                    lineHeight = 16.sp,
                    color = theme.textMuted,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
            VesperSwitch(
                checked = checked,
                onCheckedChange = onCheckedChange
            )
        }
    }
}
