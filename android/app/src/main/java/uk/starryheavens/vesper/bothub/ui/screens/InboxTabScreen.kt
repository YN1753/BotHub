package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.NotificationItem

private fun formatModuleBadge(moduleId: String): Pair<String, Color> {
    return when (moduleId.lowercase()) {
        "morning" -> "晨间简报" to Color(0xFFF2B880)
        "radar" -> "技术雷达" to Color(0xFFA5B4FC)
        "asset_watcher" -> "资产预警" to Color(0xFFFBBF24)
        "nezha_webhook" -> "探针告警" to Color(0xFFFB7185)
        "system_test" -> "通道自检" to Color(0xFF34D399)
        else -> moduleId.ifBlank { "系统情报" } to Color(0xFF94A3B8)
    }
}

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
                val selected = filterMode == id
                Surface(
                    onClick = { filterMode = id },
                    shape = RoundedCornerShape(999.dp),
                    color = if (selected) Color(0x26F2B880) else Color(0x990D1326),
                    border = BorderStroke(
                        1.dp,
                        if (selected) Color(0x80F2B880) else Color(0x1FFFF8EE)
                    ),
                    modifier = Modifier.sizeIn(minHeight = 36.dp)
                ) {
                    Box(
                        contentAlignment = Alignment.Center,
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 6.dp)
                    ) {
                        Text(
                            text = label,
                            fontSize = 12.sp,
                            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                            color = if (selected) Color(0xFFFFE2BE) else Color(0xFF94A3B8)
                        )
                    }
                }
            }
        }

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 2.dp, bottom = 20.dp)
        ) {
            if (filtered.isEmpty()) {
                item {
                    Surface(
                        shape = RoundedCornerShape(20.dp),
                        color = Color(0x990D1326),
                        border = BorderStroke(1.dp, Color(0x1FFFF8EE)),
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(top = 24.dp)
                    ) {
                        Column(
                            modifier = Modifier.padding(horizontal = 24.dp, vertical = 32.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(6.dp)
                        ) {
                            Text(
                                text = if (filterMode == "unread") "全部情报均已阅毕" else "暂无推送记录",
                                style = MaterialTheme.typography.titleSmall,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFFFFF8EE)
                            )
                            Text(
                                text = "晨间简报、技术雷达、资产临期与哪吒探针告警将实时同步至此",
                                style = MaterialTheme.typography.bodySmall,
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

                Surface(
                    onClick = { onSelectNotification(item) },
                    shape = RoundedCornerShape(18.dp),
                    color = if (unread) Color(0xE6111932) else Color(0xA60B1020),
                    border = BorderStroke(
                        width = 1.dp,
                        color = when {
                            unread && isHighPriority -> Color(0x73FB7185)
                            unread -> Color(0x5CF2B880)
                            else -> Color(0x1CFFF8EE)
                        }
                    ),
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
                                            .size(7.dp)
                                            .background(
                                                color = if (isHighPriority) Color(0xFFFB7185) else Color(0xFFF2B880),
                                                shape = CircleShape
                                            )
                                    )
                                }
                                Surface(
                                    shape = RoundedCornerShape(6.dp),
                                    color = badgeColor.copy(alpha = 0.14f),
                                    border = BorderStroke(0.5.dp, badgeColor.copy(alpha = 0.38f))
                                ) {
                                    Text(
                                        text = badgeLabel,
                                        fontSize = 10.5.sp,
                                        fontWeight = FontWeight.Medium,
                                        color = badgeColor,
                                        modifier = Modifier.padding(horizontal = 7.dp, vertical = 2.dp)
                                    )
                                }
                            }

                            Text(
                                text = formatShortTimestamp(item.createdAt),
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.sp,
                                color = Color(0xFF64748B)
                            )
                        }

                        Text(
                            text = item.title,
                            style = MaterialTheme.typography.titleSmall,
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
                                style = MaterialTheme.typography.bodySmall,
                                fontSize = 13.sp,
                                lineHeight = 19.sp,
                                color = Color(0xFF94A3B8),
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
