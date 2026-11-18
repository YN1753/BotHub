package uk.starryheavens.vesper.bothub.ui.components

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
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
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.navigation.BotHubTab
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.GrokBotGroupCluster
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

fun formatModuleBadge(moduleId: String): Pair<String, Color> {
    return when (moduleId.trim().lowercase()) {
        "morning" -> "晨报" to Color(0xFFD97706)
        "radar" -> "雷达" to Color(0xFF6366F1)
        "asset_watcher" -> "资产" to Color(0xFFF59E0B)
        "nezha_webhook" -> "告警" to Color(0xFFF43F5E)
        "system_test" -> "自检" to Color(0xFF10B981)
        else -> moduleId.ifBlank { "通知" } to Color(0xFF64748B)
    }
}

@Composable
fun ObsidianSheetDragHandle(modifier: Modifier = Modifier) {
    val theme = BotHubThemeTokens
    Column(
        modifier = modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        SpecularHairline()
        Spacer(modifier = Modifier.height(10.dp))
        Box(
            modifier = Modifier
                .width(38.dp)
                .height(4.dp)
                .background(theme.textMuted.copy(alpha = 0.42f), RoundedCornerShape(999.dp))
        )
        Spacer(modifier = Modifier.height(6.dp))
    }
}

@Composable
fun ObsidianSwitch(
    checked: Boolean,
    onCheckedChange: ((Boolean) -> Unit)?,
    enabled: Boolean = true,
    modifier: Modifier = Modifier
) {
    TactileToggle(
        checked = checked,
        onCheckedChange = onCheckedChange,
        enabled = enabled,
        modifier = modifier
    )
}

@Composable
fun ObsidianGlassInput(
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    modifier: Modifier = Modifier,
    label: String = "",
    singleLine: Boolean = true,
    maxLines: Int = if (singleLine) 1 else 4,
    visualTransformation: VisualTransformation = VisualTransformation.None,
    keyboardOptions: KeyboardOptions = KeyboardOptions.Default
) {
    val theme = BotHubThemeTokens
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(14.dp)
    val borderColor by animateColorAsState(
        targetValue = if (focused) theme.accent.copy(alpha = 0.65f) else theme.divider,
        animationSpec = tween(160),
        label = "inputFocusBorder"
    )

    Column(
        modifier = modifier,
        verticalArrangement = Arrangement.spacedBy(5.dp)
    ) {
        if (label.isNotEmpty()) {
            Text(
                text = label,
                style = TextStyle(
                    fontSize = 11.5.sp,
                    fontWeight = FontWeight.Medium,
                    color = if (focused) theme.accent else theme.textMuted,
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 1,
                softWrap = false,
                modifier = Modifier.padding(start = 4.dp)
            )
        }
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .sizeIn(minHeight = 46.dp)
                .clip(shape)
                .background(
                    Brush.verticalGradient(listOf(theme.inputTop, theme.inputBottom)),
                    shape
                )
                .border(0.9.dp, borderColor, shape),
            contentAlignment = Alignment.CenterStart
        ) {
            BasicTextField(
                value = value,
                onValueChange = onValueChange,
                singleLine = singleLine,
                maxLines = maxLines,
                visualTransformation = visualTransformation,
                keyboardOptions = keyboardOptions,
                textStyle = TextStyle(
                    color = theme.textPrimary,
                    fontSize = 14.sp,
                    lineHeight = 20.sp
                ),
                cursorBrush = SolidColor(theme.accent),
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 14.dp, vertical = 12.dp)
                    .onFocusChanged { focused = it.isFocused },
                decorationBox = { inner ->
                    Box(contentAlignment = Alignment.CenterStart) {
                        if (value.isEmpty()) {
                            Text(
                                text = placeholder,
                                fontSize = 13.5.sp,
                                color = theme.textMuted,
                                maxLines = maxLines,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                        inner()
                    }
                }
            )
        }
    }
}

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

/**
 * Clean, Theme-Adaptive WeChat / QQ Top Bar.
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
            isRefreshing -> "收取中..."
            totalUnreadMessages > 0 -> "消息 ($totalUnreadMessages)"
            else -> "消息"
        }
        BotHubTab.INBOX -> "通讯录"
        BotHubTab.STASH -> "收藏"
        BotHubTab.HUB -> "中枢"
    }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .background(
                Brush.verticalGradient(listOf(theme.topBarTop, theme.topBarBottom))
            )
            .statusBarsPadding()
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Text(
                text = headerTitle,
                style = TextStyle(
                    fontSize = 20.sp,
                    fontWeight = FontWeight.Bold,
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
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                when (selectedTab) {
                    BotHubTab.AGENT -> {
                        Box {
                            HeaderIconCircle(
                                icon = Icons.Default.Add,
                                contentDescription = "快捷菜单",
                                accent = showPlusMenu,
                                onClick = { showPlusMenu = true }
                            )
                            FloatingGlassPopover(
                                expanded = showPlusMenu,
                                onDismissRequest = { showPlusMenu = false },
                                offsetY = 42.dp,
                                items = listOf(
                                    GlassMenuAction(
                                        label = "发起群聊",
                                        icon = Icons.Default.GroupAdd,
                                        iconTint = theme.accent,
                                        onClick = onCreateGroup
                                    ),
                                    GlassMenuAction(
                                        label = "添加 AI 好友",
                                        icon = Icons.Default.PersonAddAlt1,
                                        iconTint = Color(0xFF6366F1),
                                        onClick = onCreateFriend
                                    ),
                                    GlassMenuAction(
                                        label = "新建收藏",
                                        icon = Icons.Default.BookmarkBorder,
                                        iconTint = Color(0xFF10B981),
                                        onClick = onQuickCaptureStash
                                    ),
                                    GlassMenuAction(
                                        label = "通知全部已读",
                                        icon = Icons.Default.DoneAll,
                                        iconTint = theme.textMuted,
                                        onClick = onMarkAllRead
                                    ),
                                    GlassMenuAction(
                                        label = "设置与主题",
                                        icon = Icons.Default.Settings,
                                        iconTint = Color(0xFFF59E0B),
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
                        HeaderIconCircle(
                            icon = Icons.Default.Refresh,
                            contentDescription = "刷新",
                            onClick = onRefresh
                        )
                        HeaderIconCircle(
                            icon = Icons.Default.Settings,
                            contentDescription = "设置与主题",
                            accent = true,
                            onClick = onOpenSettings
                        )
                    }
                }
            }
        }

        SpecularHairline()
    }
}

@Composable
private fun HeaderPillAction(
    label: String,
    icon: ImageVector,
    accent: Boolean,
    onClick: () -> Unit
) {
    TactileSecondaryButton(
        text = label,
        leadingIcon = icon,
        accent = accent,
        cornerRadius = 999.dp,
        minHeight = 35.dp,
        contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
        onClick = onClick
    )
}

@Composable
private fun HeaderIconCircle(
    icon: ImageVector,
    contentDescription: String,
    accent: Boolean = false,
    onClick: () -> Unit
) {
    val theme = BotHubThemeTokens
    val shape = CircleShape
    val bgBrush = remember(accent, theme.id) {
        if (accent) {
            Brush.verticalGradient(
                listOf(
                    theme.accent.copy(alpha = 0.20f),
                    theme.accentDeep.copy(alpha = 0.10f)
                )
            )
        } else {
            Brush.verticalGradient(listOf(theme.secondaryBtnTop, theme.secondaryBtnBottom))
        }
    }
    val rim = remember(accent, theme.id) {
        if (accent) {
            specularRimBrush(
                theme.accent.copy(alpha = 0.50f),
                theme.accent.copy(alpha = 0.20f),
                theme.accent.copy(alpha = 0.08f)
            )
        } else {
            specularRimBrush(theme.rimHighlight, theme.rimMid, theme.rimSubtle)
        }
    }

    Box(
        modifier = Modifier
            .size(36.dp)
            .tactileClickable(pressedScale = 0.92f, hapticOnTap = true, onClick = onClick)
            .clip(shape)
            .background(bgBrush, shape)
            .border(BorderStroke(0.85.dp, rim), shape),
        contentAlignment = Alignment.Center
    ) {
        Icon(
            imageVector = icon,
            contentDescription = contentDescription,
            tint = if (accent) (if (theme.isLight) theme.accentDeep else theme.accentLight) else theme.textPrimary,
            modifier = Modifier.size(18.dp)
        )
    }
}

/**
 * Theme-Adaptive Bottom Navigation Dock.
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
            .background(
                Brush.verticalGradient(listOf(theme.bottomBarTop, theme.bottomBarBottom))
            )
            .navigationBarsPadding()
    ) {
        SpecularHairline()
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 8.dp, vertical = 5.dp),
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
                    targetValue = if (isSelected) theme.accent else theme.textMuted,
                    animationSpec = tween(180),
                    label = "navFgColor"
                )
                val iconScale by animateFloatAsState(
                    targetValue = if (isSelected) 1.09f else 1f,
                    animationSpec = spring(dampingRatio = 0.62f, stiffness = 520f),
                    label = "navIconScale"
                )

                Box(
                    modifier = Modifier
                        .weight(1f)
                        .height(50.dp)
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
                    if (isSelected) {
                        Box(
                            modifier = Modifier
                                .size(38.dp)
                                .offset(y = (-5).dp)
                                .background(
                                    Brush.radialGradient(
                                        colors = listOf(theme.accent.copy(alpha = 0.16f), Color.Transparent)
                                    ),
                                    CircleShape
                                )
                        )
                    }

                    Column(
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(3.dp)
                    ) {
                        Box(
                            modifier = Modifier
                                .width(42.dp)
                                .height(23.dp),
                            contentAlignment = Alignment.Center
                        ) {
                            Icon(
                                imageVector = tabIcon,
                                contentDescription = tab.contentDescription,
                                tint = fgColor,
                                modifier = Modifier
                                    .size(21.dp)
                                    .graphicsLayer {
                                        scaleX = iconScale
                                        scaleY = iconScale
                                    }
                            )
                            if (badgeCount > 0) {
                                CrispUnreadBadge(
                                    count = badgeCount,
                                    containerColor = if (tab == BotHubTab.AGENT) Color(0xFFF43F5E) else theme.accent,
                                    contentColor = if (tab == BotHubTab.AGENT) Color.White else theme.onAccent,
                                    modifier = Modifier
                                        .align(Alignment.TopEnd)
                                        .offset(x = 3.dp, y = (-3).dp)
                                )
                            }
                        }
                        Text(
                            text = tab.label,
                            style = TextStyle(
                                fontSize = 10.5.sp,
                                lineHeight = 12.sp,
                                fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Medium,
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
fun MetricMiniCard(
    label: String,
    value: String,
    accentColor: Color = MaterialTheme.colorScheme.primary,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    SpecularGlassCard(
        modifier = modifier,
        cornerRadius = 18.dp,
        accentTint = accentColor,
        elevation = 5.dp
    ) {
        Column(modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp)) {
            Text(
                text = label,
                style = TextStyle(
                    fontSize = 11.sp,
                    fontWeight = FontWeight.Medium,
                    color = theme.textMuted,
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 1,
                softWrap = false,
                overflow = TextOverflow.Ellipsis
            )
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = value,
                style = TextStyle(
                    fontFamily = FontFamily.Monospace,
                    fontSize = 24.sp,
                    fontWeight = FontWeight.Bold,
                    color = accentColor,
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 1,
                softWrap = false
            )
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
    SpecularGlassCard(
        modifier = modifier.fillMaxWidth(),
        cornerRadius = 16.dp,
        onClick = { onCheckedChange(!checked) }
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 14.dp, vertical = if (compact) 10.dp else 12.dp)
                .sizeIn(minHeight = 46.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(
                modifier = Modifier
                    .weight(1f)
                    .padding(end = 12.dp),
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
                    fontSize = 11.5.sp,
                    lineHeight = 16.sp,
                    color = theme.textMuted,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
            TactileToggle(
                checked = checked,
                onCheckedChange = onCheckedChange
            )
        }
    }
}
