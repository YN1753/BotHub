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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.DeleteOutline
import androidx.compose.material.icons.filled.MarkEmailRead
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
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
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.TactileFilterChip
import uk.starryheavens.vesper.bothub.ui.components.formatModuleBadge
import uk.starryheavens.vesper.bothub.ui.components.tactileClickable
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
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth()
        ) {
            items(filterPills) { (id, label) ->
                TactileFilterChip(
                    label = label,
                    selected = filterMode == id,
                    onClick = { filterMode = id }
                )
            }
        }

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(9.dp),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 2.dp, bottom = 20.dp)
        ) {
            if (filtered.isEmpty()) {
                item {
                    SpecularGlassCard(
                        cornerRadius = 18.dp,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(top = 20.dp)
                    ) {
                        Column(
                            modifier = Modifier.padding(horizontal = 24.dp, vertical = 28.dp),
                            horizontalAlignment = Alignment.CenterHorizontally
                        ) {
                            Text(
                                text = if (filterMode == "unread") "暂无未读通知" else "暂无通知",
                                style = MaterialTheme.typography.titleSmall,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textMuted,
                                textAlign = TextAlign.Center
                            )
                        }
                    }
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
                                if (item.isUnread) onMarkOneRead(item)
                                false
                            }
                            SwipeToDismissBoxValue.EndToStart -> {
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
                            isReadAction -> Color(0xFF10B981)
                            isDeleteAction -> Color(0xFFE11D48)
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
                    SpecularGlassCard(
                        cornerRadius = 16.dp,
                        topColor = if (unread) theme.cardUnreadTop else theme.cardTop,
                        bottomColor = if (unread) theme.cardUnreadBottom else theme.cardBottom,
                        accentTint = when {
                            unread && isHighPriority -> Color(0xFFFB7185)
                            unread -> badgeColor
                            else -> null
                        },
                        onClick = { onSelectNotification(item) },
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Column(
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                            verticalArrangement = Arrangement.spacedBy(5.dp)
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
                                                    color = if (isHighPriority) Color(0xFFFB7185) else theme.accent,
                                                    shape = CircleShape
                                                )
                                        )
                                    }
                                    Box(
                                        modifier = Modifier
                                            .clip(RoundedCornerShape(6.dp))
                                            .background(badgeColor.copy(alpha = 0.14f))
                                            .border(0.5.dp, badgeColor.copy(alpha = 0.38f), RoundedCornerShape(6.dp))
                                            .padding(horizontal = 7.dp, vertical = 2.dp)
                                    ) {
                                        Text(
                                            text = badgeLabel,
                                            style = TextStyle(
                                                fontSize = 10.sp,
                                                fontWeight = FontWeight.Medium,
                                                color = badgeColor,
                                                platformStyle = NoFontPaddingStyle
                                            ),
                                            maxLines = 1,
                                            softWrap = false
                                        )
                                    }
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
                                fontSize = 14.5.sp,
                                fontWeight = if (unread) FontWeight.SemiBold else FontWeight.Medium,
                                color = if (unread) theme.textPrimary else theme.textSecondary,
                                maxLines = 2,
                                overflow = TextOverflow.Ellipsis
                            )

                            val preview = item.previewText(maxChars = 110)
                            if (preview.isNotEmpty()) {
                                Text(
                                    text = preview,
                                    fontSize = 12.5.sp,
                                    lineHeight = 18.sp,
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
                                val chipShape = RoundedCornerShape(8.dp)
                                Box(
                                    modifier = Modifier
                                        .clip(chipShape)
                                        .background(theme.accent.copy(alpha = 0.14f), chipShape)
                                        .border(0.6.dp, theme.accent.copy(alpha = 0.42f), chipShape)
                                        .tactileClickable(pressedScale = 0.94f, hapticOnTap = true) {
                                            onDiagnoseWithAi(item)
                                        }
                                        .padding(horizontal = 8.dp, vertical = 4.dp)
                                ) {
                                    Text(
                                        text = "找 TA 追问",
                                        style = TextStyle(
                                            fontSize = 11.sp,
                                            fontWeight = FontWeight.Medium,
                                            color = theme.accent,
                                            platformStyle = NoFontPaddingStyle
                                        )
                                    )
                                }
                                Box(
                                    modifier = Modifier
                                        .clip(chipShape)
                                        .background(theme.inputTop, chipShape)
                                        .border(0.6.dp, theme.divider, chipShape)
                                        .tactileClickable(
                                            enabled = rerollingNotificationId == null,
                                            pressedScale = 0.94f,
                                            hapticOnTap = true
                                        ) {
                                            onRerollNotification(item)
                                        }
                                        .padding(horizontal = 8.dp, vertical = 4.dp)
                                ) {
                                    Text(
                                        text = if (isRerolling) "重推中..." else "换一条",
                                        style = TextStyle(
                                            fontSize = 11.sp,
                                            color = theme.textSecondary,
                                            platformStyle = NoFontPaddingStyle
                                        )
                                    )
                                }
                                Box(
                                    modifier = Modifier
                                        .clip(chipShape)
                                        .background(
                                            if (todoSaved) Color(0x1F10B981) else theme.inputTop,
                                            chipShape
                                        )
                                        .border(
                                            0.6.dp,
                                            if (todoSaved) Color(0x4A10B981) else theme.divider,
                                            chipShape
                                        )
                                        .tactileClickable(
                                            enabled = !todoSaved,
                                            pressedScale = 0.94f,
                                            hapticOnTap = true
                                        ) {
                                            onConvertToTodo(item)
                                        }
                                        .padding(horizontal = 8.dp, vertical = 4.dp)
                                ) {
                                    Text(
                                        text = if (todoSaved) "已建待办" else "待办",
                                        style = TextStyle(
                                            fontSize = 11.sp,
                                            color = if (todoSaved) Color(0xFF10B981) else theme.textSecondary,
                                            platformStyle = NoFontPaddingStyle
                                        )
                                    )
                                }
                                Box(
                                    modifier = Modifier
                                        .clip(chipShape)
                                        .background(
                                            if (noteSaved) Color(0x1F10B981) else theme.inputTop,
                                            chipShape
                                        )
                                        .border(
                                            0.6.dp,
                                            if (noteSaved) Color(0x4A10B981) else theme.divider,
                                            chipShape
                                        )
                                        .tactileClickable(
                                            enabled = !noteSaved,
                                            pressedScale = 0.94f,
                                            hapticOnTap = true
                                        ) {
                                            onSaveToNote(item)
                                        }
                                        .padding(horizontal = 8.dp, vertical = 4.dp)
                                ) {
                                    Text(
                                        text = if (noteSaved) "已存笔记" else "笔记",
                                        style = TextStyle(
                                            fontSize = 11.sp,
                                            color = if (noteSaved) Color(0xFF10B981) else theme.textSecondary,
                                            platformStyle = NoFontPaddingStyle
                                        )
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
