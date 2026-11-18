package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.DashboardSnapshot
import uk.starryheavens.vesper.bothub.data.TaskItem
import uk.starryheavens.vesper.bothub.ui.components.MetricMiniCard
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSwitch

@Composable
fun HubTabScreen(
    snapshot: DashboardSnapshot?,
    onRunTask: (TaskItem) -> Unit,
    onToggleTask: (TaskItem, Boolean) -> Unit,
    onSyncAssets: () -> Unit,
    onOpenConsole: (String, String) -> Unit = { _, _ -> },
    modifier: Modifier = Modifier
) {
    val expiringCount = snapshot?.expiringCount ?: 0

    LazyColumn(
        modifier = modifier,
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 10.dp, bottom = 24.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        // Hero Telemetry Tiles
        item {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                MetricMiniCard(
                    modifier = Modifier.weight(1f),
                    label = "数字资产",
                    value = (snapshot?.totalAssets ?: 0).toString(),
                    accentColor = Color(0xFFFFE2BE)
                )
                MetricMiniCard(
                    modifier = Modifier.weight(1f),
                    label = "7天临期",
                    value = expiringCount.toString(),
                    accentColor = if (expiringCount > 0) Color(0xFFFBBF24) else Color(0xFF34D399)
                )
                MetricMiniCard(
                    modifier = Modifier.weight(1f),
                    label = "活跃任务",
                    value = (snapshot?.activeTasks ?: 0).toString(),
                    accentColor = Color(0xFFA5B4FC)
                )
            }
        }

        // Polyglot Hybrid Studio Bridge Card (Full Nezha Telemetry + Asset/Task CRUD)
        item {
            Surface(
                onClick = { onOpenConsole("ops", "全量资产与哪吒实时工作台") },
                shape = RoundedCornerShape(16.dp),
                color = Color(0xCC0E152B),
                border = BorderStroke(1.dp, Color(0x38F2B880)),
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 11.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        modifier = Modifier.weight(1f)
                    ) {
                        Box(
                            modifier = Modifier
                                .size(7.dp)
                                .background(Color(0xFF34D399), CircleShape)
                        )
                        Column {
                            Text(
                                text = "打开全功能资产与哪吒监控工作台",
                                fontSize = 13.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFFFFE2BE)
                            )
                            Text(
                                text = "免密直达 · 节点负载监控 · 资产与调度完整增删改",
                                fontSize = 11.sp,
                                color = Color(0xFF94A3B8)
                            )
                        }
                    }
                    Icon(
                        Icons.Default.KeyboardArrowRight,
                        contentDescription = null,
                        tint = Color(0xFFF2B880),
                        modifier = Modifier.size(18.dp)
                    )
                }
            }
        }

        // Section 1: Digital Assets with Annual Runway Progress Scale
        item {
            Spacer(modifier = Modifier.height(2.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "数字资产与续费守望",
                        style = MaterialTheme.typography.titleSmall,
                        fontSize = 15.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = Color(0xFFFFF8EE)
                    )
                    Text(
                        text = "VPS · 域名 · 订阅按年周期比例显示剩余额度",
                        fontSize = 11.5.sp,
                        color = Color(0xFF94A3B8)
                    )
                }

                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Surface(
                        onClick = { onOpenConsole("create_asset", "新增数字资产") },
                        shape = RoundedCornerShape(999.dp),
                        color = Color(0xFF10172C),
                        border = BorderStroke(1.dp, Color(0x26FFF8EE)),
                        modifier = Modifier.sizeIn(minHeight = 34.dp)
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(3.dp)
                        ) {
                            Icon(
                                Icons.Default.Add,
                                contentDescription = null,
                                tint = Color(0xFFCBD5E1),
                                modifier = Modifier.size(13.dp)
                            )
                            Text(
                                text = "新增",
                                fontSize = 11.5.sp,
                                fontWeight = FontWeight.Medium,
                                color = Color(0xFFE2E8F0)
                            )
                        }
                    }

                    Surface(
                        onClick = onSyncAssets,
                        shape = RoundedCornerShape(999.dp),
                        color = Color(0x22F2B880),
                        border = BorderStroke(1.dp, Color(0x52F2B880)),
                        modifier = Modifier.sizeIn(minHeight = 34.dp)
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 11.dp, vertical = 6.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(4.dp)
                        ) {
                            Icon(
                                Icons.Default.Refresh,
                                contentDescription = null,
                                tint = Color(0xFFFFE2BE),
                                modifier = Modifier.size(13.dp)
                            )
                            Text(
                                text = "嗅探",
                                fontSize = 11.5.sp,
                                fontWeight = FontWeight.Medium,
                                color = Color(0xFFFFE2BE)
                            )
                        }
                    }
                }
            }
        }

        val assets = snapshot?.assets ?: emptyList()
        if (assets.isEmpty()) {
            item {
                Surface(
                    shape = RoundedCornerShape(16.dp),
                    color = Color(0x990D1326),
                    border = BorderStroke(1.dp, Color(0x1FFFF8EE)),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Text(
                        text = "暂无已登记的数字资产",
                        fontSize = 13.sp,
                        color = Color(0xFF94A3B8),
                        modifier = Modifier.padding(16.dp)
                    )
                }
            }
        }

        items(assets, key = { it.id }) { asset ->
            val isPermanent = asset.dueDate.startsWith("2099") || asset.daysLeft > 3650
            val isMultiYear = !isPermanent && asset.daysLeft > 365
            val barColor = when {
                isPermanent || isMultiYear -> Color(0xFF34D399)
                asset.daysLeft <= 7 -> Color(0xFFFB7185)
                asset.daysLeft <= 30 -> Color(0xFFFBBF24)
                else -> Color(0xFFF2B880)
            }
            val progressFraction = when {
                isPermanent -> 1f
                asset.daysLeft <= 0 -> 0.04f
                else -> (asset.daysLeft.toFloat() / 365f).coerceIn(0.06f, 1f)
            }

            Surface(
                shape = RoundedCornerShape(18.dp),
                color = Color(0xBF0C1224),
                border = BorderStroke(
                    1.dp,
                    if (asset.isExpiringSoon && !isPermanent) barColor.copy(alpha = 0.55f) else Color(0x20FFF8EE)
                ),
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier.padding(horizontal = 15.dp, vertical = 13.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = asset.name,
                            style = MaterialTheme.typography.titleSmall,
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFF8EE),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        )
                        Surface(
                            shape = RoundedCornerShape(6.dp),
                            color = barColor.copy(alpha = 0.14f),
                            border = BorderStroke(0.5.dp, barColor.copy(alpha = 0.42f))
                        ) {
                            Text(
                                text = if (isPermanent) "长期有效" else asset.dueBadgeLabel,
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = barColor,
                                modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.5.dp)
                            )
                        }
                    }

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = listOf(asset.spec, asset.price)
                                .filter { it.isNotBlank() }
                                .joinToString(" · ")
                                .ifEmpty { "常规资产" },
                            fontSize = 12.5.sp,
                            color = Color(0xFF94A3B8),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 12.dp)
                        )
                        if (!isPermanent && asset.dueDate.isNotEmpty()) {
                            Text(
                                text = asset.dueDate.take(10),
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.sp,
                                color = Color(0xFF64748B)
                            )
                        }
                    }

                    // Annual Runway Progress Bar (365-day scale)
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(3.dp)
                            .clip(RoundedCornerShape(999.dp))
                            .background(Color(0xFF1A223B))
                    ) {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth(progressFraction)
                                .height(3.dp)
                                .clip(RoundedCornerShape(999.dp))
                                .background(barColor)
                        )
                    }
                }
            }
        }

        // Section 2: Automation & Scheduled Tasks
        item {
            Spacer(modifier = Modifier.height(6.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "自动化情报与调度引擎",
                        style = MaterialTheme.typography.titleSmall,
                        fontSize = 15.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = Color(0xFFFFF8EE)
                    )
                    Text(
                        text = "可随时启停周期任务或立即手动触发执行",
                        fontSize = 11.5.sp,
                        color = Color(0xFF94A3B8)
                    )
                }

                Surface(
                    onClick = { onOpenConsole("create_task", "创建自动化调度任务") },
                    shape = RoundedCornerShape(999.dp),
                    color = Color(0xFF10172C),
                    border = BorderStroke(1.dp, Color(0x26FFF8EE)),
                    modifier = Modifier.sizeIn(minHeight = 34.dp)
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(3.dp)
                    ) {
                        Icon(
                            Icons.Default.Add,
                            contentDescription = null,
                            tint = Color(0xFFCBD5E1),
                            modifier = Modifier.size(13.dp)
                        )
                        Text(
                            text = "新建任务",
                            fontSize = 11.5.sp,
                            fontWeight = FontWeight.Medium,
                            color = Color(0xFFE2E8F0)
                        )
                    }
                }
            }
        }

        val tasks = snapshot?.tasks ?: emptyList()
        items(tasks, key = { it.id }) { task ->
            Surface(
                shape = RoundedCornerShape(18.dp),
                color = Color(0xBF0C1224),
                border = BorderStroke(1.dp, Color(0x20FFF8EE)),
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier.padding(horizontal = 15.dp, vertical = 12.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = task.name,
                            style = MaterialTheme.typography.titleSmall,
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = if (task.enabled) Color(0xFFFFF8EE) else Color(0xFF94A3B8),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        )
                        ObsidianSwitch(
                            checked = task.enabled,
                            onCheckedChange = { checked -> onToggleTask(task, checked) }
                        )
                    }

                    Row(
                        modifier = Modifier.fillMaxWidth(),
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
                                text = task.formattedSchedule(),
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.5.sp,
                                color = Color(0xFF94A3B8),
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                            if (task.lastSummary.isNotBlank()) {
                                Text(
                                    text = task.lastSummary,
                                    fontSize = 12.sp,
                                    color = Color(0xFF64748B),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                        }

                        Surface(
                            onClick = { onRunTask(task) },
                            shape = RoundedCornerShape(999.dp),
                            color = Color(0x22F2B880),
                            border = BorderStroke(1.dp, Color(0x52F2B880)),
                            modifier = Modifier.sizeIn(minHeight = 36.dp)
                        ) {
                            Row(
                                modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(4.dp)
                            ) {
                                Icon(
                                    Icons.Default.PlayArrow,
                                    contentDescription = "立即运行",
                                    tint = Color(0xFFFFE2BE),
                                    modifier = Modifier.size(15.dp)
                                )
                                Text(
                                    text = "运行",
                                    fontSize = 12.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = Color(0xFFFFE2BE)
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}
