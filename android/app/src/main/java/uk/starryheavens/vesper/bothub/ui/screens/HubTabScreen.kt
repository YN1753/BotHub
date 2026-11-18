package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.DashboardSnapshot
import uk.starryheavens.vesper.bothub.data.TaskItem
import uk.starryheavens.vesper.bothub.data.UrlUtils
import uk.starryheavens.vesper.bothub.ui.components.MetricMiniCard
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileToggle
import uk.starryheavens.vesper.bothub.ui.components.VesperBrandEmblem
import uk.starryheavens.vesper.bothub.ui.components.specularRimBrush

@Composable
fun HubTabScreen(
    snapshot: DashboardSnapshot?,
    serverUrl: String = "",
    onRunTask: (TaskItem) -> Unit,
    onToggleTask: (TaskItem, Boolean) -> Unit,
    onSyncAssets: () -> Unit,
    onOpenSettings: () -> Unit = {},
    onOpenConsole: (String, String) -> Unit = { _, _ -> },
    modifier: Modifier = Modifier
) {
    val expiringCount = snapshot?.expiringCount ?: 0

    LazyColumn(
        modifier = modifier,
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 10.dp, bottom = 24.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        // 1. WeChat-"我" Style Node Identity & Guardian Hero Card
        if (serverUrl.isNotBlank()) {
            item(key = "__hub_identity__") {
                SpecularGlassCard(
                    cornerRadius = 20.dp,
                    accentTint = Color(0xFFF2B880),
                    onClick = onOpenSettings,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 15.dp, vertical = 14.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        val sq = RoundedCornerShape(15.dp)
                        Box(
                            modifier = Modifier
                                .size(48.dp)
                                .clip(sq)
                                .background(
                                    Brush.verticalGradient(listOf(Color(0xFF1A2346), Color(0xFF0E1328))),
                                    sq
                                )
                                .border(
                                    BorderStroke(
                                        0.8.dp,
                                        specularRimBrush(Color(0x66F2B880), Color(0x24A5B4FC), Color.Transparent)
                                    ),
                                    sq
                                ),
                            contentAlignment = Alignment.Center
                        ) {
                            VesperBrandEmblem(size = 32.dp)
                        }

                        Column(
                            modifier = Modifier.weight(1f),
                            verticalArrangement = Arrangement.spacedBy(3.dp)
                        ) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                Text(
                                    text = "BotHub 私人中枢",
                                    fontSize = 15.5.sp,
                                    fontWeight = FontWeight.Bold,
                                    color = Color(0xFFFFF8EE),
                                    maxLines = 1,
                                    softWrap = false
                                )
                                Row(
                                    modifier = Modifier
                                        .clip(RoundedCornerShape(999.dp))
                                        .background(Color(0x1F34D399))
                                        .border(0.6.dp, Color(0x4734D399), RoundedCornerShape(999.dp))
                                        .padding(horizontal = 7.dp, vertical = 2.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                                ) {
                                    Box(
                                        modifier = Modifier
                                            .size(5.dp)
                                            .background(Color(0xFF34D399), CircleShape)
                                    )
                                    Text(
                                        text = "实时守护",
                                        style = TextStyle(
                                            fontSize = 9.5.sp,
                                            fontWeight = FontWeight.Medium,
                                            color = Color(0xFFA7F3D0),
                                            platformStyle = NoFontPaddingStyle
                                        ),
                                        maxLines = 1,
                                        softWrap = false
                                    )
                                }
                            }

                            Text(
                                text = "${UrlUtils.formatDisplayHost(serverUrl)} · 无异常静默巡检",
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.sp,
                                color = Color(0xFF8C9BB5),
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                        }

                        Icon(
                            imageVector = Icons.Default.KeyboardArrowRight,
                            contentDescription = "打开连接设置",
                            tint = Color(0xFFF2B880),
                            modifier = Modifier.size(20.dp)
                        )
                    }
                }
            }
        }

        // 2. Hero Telemetry Tiles
        item(key = "__hub_metrics__") {
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

        // 3. Polyglot Hybrid Studio Bridge Card
        item(key = "__hub_studio__") {
            SpecularGlassCard(
                cornerRadius = 17.dp,
                accentTint = Color(0xFF34D399),
                onClick = { onOpenConsole("ops", "全量资产与哪吒实时工作台") },
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Column(
                        modifier = Modifier
                            .weight(1f)
                            .padding(end = 8.dp),
                        verticalArrangement = Arrangement.spacedBy(2.dp)
                    ) {
                        Text(
                            text = "打开全功能资产与哪吒监控工作台",
                            fontSize = 13.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFE2BE),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        Text(
                            text = "免密直达 · 节点实时负载 · 资产与任务增删改",
                            fontSize = 11.sp,
                            color = Color(0xFF8C9BB5),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                    Icon(
                        imageVector = Icons.Default.KeyboardArrowRight,
                        contentDescription = null,
                        tint = Color(0xFFF2B880),
                        modifier = Modifier.size(18.dp)
                    )
                }
            }
        }

        // 4. Section 1: Digital Assets with Annual Runway Progress Scale
        item(key = "__hub_assets_hdr__") {
            Spacer(modifier = Modifier.height(2.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "数字资产与续费守望",
                        fontSize = 15.sp,
                        fontWeight = FontWeight.Bold,
                        color = Color(0xFFFFF8EE)
                    )
                    Text(
                        text = "平时静默巡检 · 临期主动提醒",
                        fontSize = 11.5.sp,
                        color = Color(0xFF8C9BB5),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }

                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    TactileSecondaryButton(
                        text = "新增",
                        leadingIcon = Icons.Default.Add,
                        cornerRadius = 999.dp,
                        minHeight = 33.dp,
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 5.dp),
                        onClick = { onOpenConsole("create_asset", "新增数字资产") }
                    )
                    TactileSecondaryButton(
                        text = "嗅探",
                        leadingIcon = Icons.Default.Refresh,
                        accent = true,
                        cornerRadius = 999.dp,
                        minHeight = 33.dp,
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 5.dp),
                        onClick = onSyncAssets
                    )
                }
            }
        }

        val assets = snapshot?.assets ?: emptyList()
        if (assets.isEmpty()) {
            item(key = "__hub_assets_empty__") {
                SpecularGlassCard(
                    cornerRadius = 16.dp,
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

            SpecularGlassCard(
                cornerRadius = 18.dp,
                accentTint = if (asset.isExpiringSoon && !isPermanent) barColor else null,
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
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFF8EE),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        )
                        Box(
                            modifier = Modifier
                                .clip(RoundedCornerShape(6.dp))
                                .background(barColor.copy(alpha = 0.14f))
                                .border(0.5.dp, barColor.copy(alpha = 0.42f), RoundedCornerShape(6.dp))
                                .padding(horizontal = 8.dp, vertical = 2.5.dp)
                        ) {
                            Text(
                                text = if (isPermanent) "长期有效" else asset.dueBadgeLabel,
                                style = TextStyle(
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 11.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = barColor,
                                    platformStyle = NoFontPaddingStyle
                                )
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
                            color = Color(0xFF8C9BB5),
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
                                color = Color(0xFF5D6D89)
                            )
                        }
                    }

                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(3.5.dp)
                            .clip(RoundedCornerShape(999.dp))
                            .background(Color(0xFF141C34))
                    ) {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth(progressFraction)
                                .height(3.5.dp)
                                .clip(RoundedCornerShape(999.dp))
                                .background(
                                    Brush.horizontalGradient(
                                        listOf(barColor.copy(alpha = 0.72f), barColor)
                                    )
                                )
                        )
                    }
                }
            }
        }

        // 5. Section 2: Automation & Scheduled Tasks
        item(key = "__hub_tasks_hdr__") {
            Spacer(modifier = Modifier.height(6.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "自动化情报与静默巡检",
                        fontSize = 15.sp,
                        fontWeight = FontWeight.Bold,
                        color = Color(0xFFFFF8EE)
                    )
                    Text(
                        text = "无异常不打扰 · 有告警即时推送",
                        fontSize = 11.5.sp,
                        color = Color(0xFF8C9BB5),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }

                TactileSecondaryButton(
                    text = "新建任务",
                    leadingIcon = Icons.Default.Add,
                    cornerRadius = 999.dp,
                    minHeight = 33.dp,
                    contentPadding = PaddingValues(horizontal = 10.dp, vertical = 5.dp),
                    onClick = { onOpenConsole("create_task", "创建自动化调度任务") }
                )
            }
        }

        val tasks = snapshot?.tasks ?: emptyList()
        items(tasks, key = { it.id }) { task ->
            SpecularGlassCard(
                cornerRadius = 18.dp,
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
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = if (task.enabled) Color(0xFFFFF8EE) else Color(0xFF8C9BB5),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        )
                        TactileToggle(
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
                                color = Color(0xFF8C9BB5),
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                            if (task.lastSummary.isNotBlank()) {
                                Text(
                                    text = task.lastSummary,
                                    fontSize = 12.sp,
                                    color = Color(0xFF5D6D89),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                        }

                        TactileSecondaryButton(
                            text = "运行",
                            leadingIcon = Icons.Default.PlayArrow,
                            accent = true,
                            cornerRadius = 999.dp,
                            minHeight = 34.dp,
                            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 5.dp),
                            onClick = { onRunTask(task) }
                        )
                    }
                }
            }
        }
    }
}
