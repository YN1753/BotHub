package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.DeleteOutline
import androidx.compose.material.icons.filled.MarkEmailRead
import androidx.compose.material.icons.outlined.NotificationsNone
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.SwipeToDismissBox
import androidx.compose.material3.SwipeToDismissBoxValue
import androidx.compose.material3.Text
import androidx.compose.material3.rememberSwipeToDismissBoxState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperEmptyState
import uk.starryheavens.vesper.bothub.ui.components.VesperFilterChip
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.formatModuleBadge
import uk.starryheavens.vesper.bothub.ui.components.vesperPressable
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

private fun formatShortTimestamp(raw: String): String {
    val clean = raw.trim()
    return if (clean.length >= 16) clean.substring(5, 16) else clean
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun InboxTabScreen(
    notifications: List<NotificationItem>,
    rerollingNotificationId: Long? = null,
    savedTodoIds: Set<Long> = emptySet(),
    savedNoteIds: Set<Long> = emptySet(),
    onSelectNotification: (NotificationItem) -> Unit,
    onMarkOneRead: (NotificationItem) -> Unit = {},
    onDeleteNotification: (NotificationItem) -> Unit = {},
    onDiagnoseWithAi: (NotificationItem) -> Unit = {},
    onRerollNotification: (NotificationItem) -> Unit = {},
    onConvertToTodo: (NotificationItem) -> Unit = {},
    onSaveToNote: (NotificationItem) -> Unit = {},
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val haptics = LocalHapticFeedback.current
    var filterMode by remember { mutableStateOf("all") }

    val unreadCount = remember(notifications) { notifications.count { it.isUnread } }
    val filtered = remember(notifications, filterMode) {
        when (filterMode) {
            "unread" -> notifications.filter { it.isUnread }
            "high" -> notifications.filter {
                val p = it.priority.lowercase()
                p == "high" || p == "critical" || it.moduleId == "nezha_webhook" || it.moduleId == "asset_watcher"
            }
            else -> notifications
        }
    }

    val filterPills = listOf(
        "all" to if (notifications.isNotEmpty()) "全部 ${notifications.size}" else "全部",
        "unread" to if (unreadCount > 0) "未读 $unreadCount" else "未读",
        "high" to "重要告警"
    )

    Column(modifier = modifier.fillMaxSize()) {
        LazyRow(
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 6.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth()
        ) {
            items(filterPills) { (id, label) ->
                VesperFilterChip(
                    label = label,
                    selected = filterMode == id,
                    onClick = { filterMode = id }
                )
            }
        }

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 24.dp)
        ) {
            if (filtered.isEmpty()) {
                item {
                    VesperEmptyState(
                        icon = Icons.Outlined.NotificationsNone,
                        title = if (filterMode == "unread") "暂无未读通知" else "暂无服务通知",
                        subtitle = "定时简报、资产临期提醒与探针告警会实时汇总于此",
                        modifier = Modifier.padding(top = 16.dp)
                    )
                }
            }

            items(filtered, key = { it.id }) { item ->
                val unread = item.isUnread
                val (badgeLabel, badgeColor) = formatModuleBadge(item.moduleId)
                val isHighPriority = item.priority.equals("high", ignoreCase = true) ||
                    item.priority.equals("critical", ignoreCase = true)

                val dismissState = rememberSwipeToDismissBoxState(
                    confirmValueChange = { value ->
                        when (value) {
                            SwipeToDismissBoxValue.StartToEnd -> {
                                haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                                if (item.isUnread) onMarkOneRead(item)
                                false
                            }
                            SwipeToDismissBoxValue.EndToStart -> {
                                haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                                onDeleteNotification(item)
                                true
                            }
                            SwipeToDismissBoxValue.Settled -> false
                        }
                    }
                )

                SwipeToDismissBox(
                    state = dismissState,
                    enableDismissFromStartToEnd = unread,
                    backgroundContent = {
                        val direction = dismissState.dismissDirection
                        val isReadAction = direction == SwipeToDismissBoxValue.StartToEnd && unread
                        val isDeleteAction = direction == SwipeToDismissBoxValue.EndToStart
                        val bgColor = when {
                            isReadAction -> theme.success
                            isDeleteAction -> theme.danger
                            else -> Color.Transparent
                        }
                        Box(
                            modifier = Modifier
                                .fillMaxSize()
                                .clip(RoundedCornerShape(16.dp))
                                .background(bgColor.copy(alpha = if (isReadAction || isDeleteAction) 0.16f else 0f))
                                .padding(horizontal = 18.dp),
                            contentAlignment = if (isReadAction) Alignment.CenterStart else Alignment.CenterEnd
                        ) {
                            if (isReadAction) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                                ) {
                                    Icon(
                                        imageVector = Icons.Default.MarkEmailRead,
                                        contentDescription = null,
                                        tint = bgColor,
                                        modifier = Modifier.size(18.dp)
                                    )
                                    Text(
                                        text = "标为已读",
                                        fontSize = 12.5.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        color = bgColor
                                    )
                                }
                            } else if (isDeleteAction) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                                ) {
                                    Text(
                                        text = "删除",
                                        fontSize = 12.5.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        color = bgColor
                                    )
                                    Icon(
                                        imageVector = Icons.Default.DeleteOutline,
                                        contentDescription = null,
                                        tint = bgColor,
                                        modifier = Modifier.size(18.dp)
                                    )
                                }
                            }
                        }
                    }
                ) {
                    VesperCard(
                        cornerRadius = 16.dp,
                        elevated = unread,
                        accentTone = when {
                            unread && isHighPriority -> theme.danger
                            unread -> badgeColor
                            else -> null
                        },
                        onClick = { onSelectNotification(item) },
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Column(
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                            verticalArrangement = Arrangement.spacedBy(6.dp)
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
                                    if (unread) {
                                        Box(
                                            modifier = Modifier
                                                .size(6.5.dp)
                                                .background(
                                                    color = if (isHighPriority) theme.danger else theme.accent,
                                                    shape = CircleShape
                                                )
                                        )
                                    }
                                    VesperTag(text = badgeLabel, toneColor = badgeColor)
                                }

                                Text(
                                    text = formatShortTimestamp(item.createdAt),
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 11.sp,
                                    color = theme.textMuted,
                                    maxLines = 1,
                                    softWrap = false
                                )
                            }

                            Text(
                                text = item.title,
                                fontSize = 15.sp,
                                lineHeight = 21.sp,
                                fontWeight = if (unread) FontWeight.SemiBold else FontWeight.Medium,
                                color = if (unread) theme.textPrimary else theme.textSecondary,
                                maxLines = 2,
                                overflow = TextOverflow.Ellipsis
                            )

                            val preview = remember(item.id, item.summary, item.markdownBody) {
                                item.previewText(maxChars = 110)
                            }
                            if (preview.isNotEmpty()) {
                                Text(
                                    text = preview,
                                    fontSize = 12.5.sp,
                                    lineHeight = 18.5.sp,
                                    color = theme.textMuted,
                                    maxLines = 2,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }

                            val isRerolling = rerollingNotificationId == item.id
                            val todoSaved = item.id in savedTodoIds
                            val noteSaved = item.id in savedNoteIds
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(top = 4.dp),
                                horizontalArrangement = Arrangement.spacedBy(6.dp),
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                NotificationMiniActionChip(
                                    text = "找 TA 追问",
                                    accent = true,
                                    onClick = { onDiagnoseWithAi(item) }
                                )
                                NotificationMiniActionChip(
                                    text = if (isRerolling) "重推中..." else "换一条",
                                    enabled = rerollingNotificationId == null,
                                    onClick = { onRerollNotification(item) }
                                )
                                NotificationMiniActionChip(
                                    text = if (todoSaved) "已建待办" else "待办",
                                    done = todoSaved,
                                    enabled = !todoSaved,
                                    onClick = { onConvertToTodo(item) }
                                )
                                NotificationMiniActionChip(
                                    text = if (noteSaved) "已存笔记" else "笔记",
                                    done = noteSaved,
                                    enabled = !noteSaved,
                                    onClick = { onSaveToNote(item) }
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun NotificationMiniActionChip(
    text: String,
    accent: Boolean = false,
    done: Boolean = false,
    enabled: Boolean = true,
    onClick: () -> Unit
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(8.dp)
    val bgColor = when {
        done -> theme.success.copy(alpha = 0.14f)
        accent -> theme.accentSoft
        else -> theme.surfaceInput
    }
    val borderColor = when {
        done -> theme.success.copy(alpha = 0.36f)
        accent -> theme.accentMuted
        else -> theme.border
    }
    val fgColor = when {
        done -> theme.success
        accent -> theme.accentForeground
        else -> theme.textSecondary
    }
    Box(
        modifier = Modifier
            .sizeIn(minHeight = 32.dp)
            .clip(shape)
            .background(bgColor, shape)
            .border(0.8.dp, borderColor, shape)
            .vesperPressable(enabled = enabled, pressedScale = 0.94f, hapticOnTap = true, onClick = onClick)
            .padding(horizontal = 9.dp, vertical = 6.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = text,
            style = TextStyle(
                fontSize = 11.5.sp,
                fontWeight = if (accent || done) FontWeight.SemiBold else FontWeight.Medium,
                color = fgColor,
                platformStyle = NoFontPaddingStyle
            ),
            maxLines = 1,
            softWrap = false
        )
    }
}
