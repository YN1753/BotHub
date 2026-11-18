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

private fun formatShortTimestamp(raw: String): String {
    val clean = raw.trim()
    return if (clean.length >= 16) clean.substring(5, 16) else clean
}

@Composable
fun InboxTabScreen(
    notifications: List<NotificationItem>,
    onSelectNotification: (NotificationItem) -> Unit,
    onMarkAllRead: () -> Unit,
    modifier: Modifier = Modifier
) {
    var filterMode by remember { mutableStateOf("all") }

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
        "all" to "全部 (${notifications.size})",
        "unread" to "未读 (${notifications.count { it.isUnread }})",
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
            verticalArrangement = Arrangement.spacedBy(10.dp),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 2.dp, bottom = 20.dp)
        ) {
            if (filtered.isEmpty()) {
                item {
                    SpecularGlassCard(
                        cornerRadius = 20.dp,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(top = 24.dp)
                    ) {
                        Column(
                            modifier = Modifier.padding(horizontal = 24.dp, vertical = 32.dp),
                            horizontalAlignment = Alignment.CenterHorizontally
                        ) {
                            Text(
                                text = if (filterMode == "unread") "暂无未读通知" else "暂无通知",
                                style = MaterialTheme.typography.titleSmall,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFF94A3B8),
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

                SpecularGlassCard(
                    cornerRadius = 18.dp,
                    topColor = if (unread) Color(0xF0151F3B) else Color(0xD90E1428),
                    bottomColor = if (unread) Color(0xF00C1224) else Color(0xD9090D1A),
                    accentTint = when {
                        unread && isHighPriority -> Color(0xFFFB7185)
                        unread -> badgeColor
                        else -> null
                    },
                    onClick = { onSelectNotification(item) },
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(horizontal = 15.dp, vertical = 13.dp),
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
                                                color = if (isHighPriority) Color(0xFFFB7185) else Color(0xFFF2B880),
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
                                color = Color(0xFF5D6D89),
                                maxLines = 1,
                                softWrap = false
                            )
                        }

                        Text(
                            text = item.title,
                            fontSize = 15.sp,
                            fontWeight = if (unread) FontWeight.SemiBold else FontWeight.Medium,
                            color = if (unread) Color(0xFFFFF8EE) else Color(0xFFD8E0F0),
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis
                        )

                        val preview = item.previewText(maxChars = 110)
                        if (preview.isNotEmpty()) {
                            Text(
                                text = preview,
                                fontSize = 13.sp,
                                lineHeight = 19.sp,
                                color = Color(0xFF8C9BB5),
                                maxLines = 2,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                    }
                }
            }
        }
    }
}
