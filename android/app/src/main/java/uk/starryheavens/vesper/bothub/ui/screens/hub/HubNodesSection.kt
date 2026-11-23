package uk.starryheavens.vesper.bothub.ui.screens.hub

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.outlined.Dns
import androidx.compose.material3.Text
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.AssetItem
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperEmptyState
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSectionHeader
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens
import java.util.Locale

private val COMPUTE_CATEGORY_KEYS = setOf("compute", "server", "vps", "node", "cloud", "算力", "服务器", "节点")

internal fun filterComputeNodes(assets: List<AssetItem>): List<AssetItem> {
    return assets.filter { asset ->
        val cat = asset.category.trim().lowercase(Locale.ROOT)
        val combined = "${asset.name} ${asset.spec}".lowercase(Locale.ROOT)
        cat in COMPUTE_CATEGORY_KEYS ||
            combined.contains("vps") ||
            combined.contains("server") ||
            combined.contains("服务器") ||
            combined.contains("探针") ||
            combined.contains("节点")
    }.sortedWith(
        compareBy<AssetItem> { it.status == "已停用" || it.status == "已归档" }
            .thenBy { it.daysLeft }
    )
}

internal fun filterNezhaAlerts(notifications: List<NotificationItem>): List<NotificationItem> {
    return notifications.filter { item ->
        val mod = item.moduleId.trim().lowercase(Locale.ROOT)
        val text = "${item.title} ${item.summary}".lowercase(Locale.ROOT)
        mod == "nezha_webhook" ||
            text.contains("nezha") ||
            text.contains("哪吒") ||
            text.contains("探针") ||
            text.contains("离线")
    }
}

internal fun LazyListScope.hubNodesItems(
    computeNodes: List<AssetItem>,
    nezhaAlerts: List<NotificationItem>,
    fxRates: Map<String, Double>,
    onInspectNezhaWithAi: () -> Unit,
    onSyncAssets: () -> Unit,
    onEditNodeAsset: (AssetItem) -> Unit,
    onOpenServiceFeed: () -> Unit
) {
    item(key = "__hub_nodes_hero__") {
        val theme = BotHubThemeTokens
        val activeNodes = computeNodes.count { it.status != "已停用" && it.status != "已归档" }
        val alertUnread = nezhaAlerts.count { it.isUnread }
        val computeMonthly = remember(computeNodes, fxRates) {
            summarizeAssetsCost(computeNodes, yearly = false, fxRates = fxRates)
        }

        VesperCard(
            cornerRadius = 16.dp,
            accentTone = if (alertUnread > 0) theme.danger else theme.success,
            modifier = Modifier.fillMaxWidth()
        ) {
            Column(
                modifier = Modifier.padding(14.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column(verticalArrangement = Arrangement.spacedBy(3.dp)) {
                        Text(
                            text = "哪吒探针与算力节点",
                            fontSize = 15.sp,
                            fontWeight = FontWeight.Bold,
                            color = theme.textPrimary
                        )
                        Text(
                            text = buildString {
                                append("在用节点 $activeNodes 台")
                                if (computeMonthly != "--") append(" · 算力月均 $computeMonthly")
                                if (alertUnread > 0) append(" · $alertUnread 条未读告警")
                            },
                            fontFamily = FontFamily.Monospace,
                            fontSize = 11.5.sp,
                            color = theme.textSecondary
                        )
                    }
                    VesperTag(
                        text = if (alertUnread > 0) "待巡检" else "正常",
                        toneColor = if (alertUnread > 0) theme.danger else theme.success
                    )
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    VesperPrimaryButton(
                        text = "AI 实时巡检探针",
                        leadingIcon = Icons.Default.AutoAwesome,
                        minHeight = 44.dp,
                        contentPadding = PaddingValues(horizontal = 12.dp, vertical = 8.dp),
                        onClick = onInspectNezhaWithAi,
                        modifier = Modifier.weight(1.35f)
                    )
                    VesperSecondaryButton(
                        text = "同步台账",
                        leadingIcon = Icons.Default.Refresh,
                        minHeight = 44.dp,
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 8.dp),
                        onClick = onSyncAssets,
                        modifier = Modifier.weight(0.85f)
                    )
                }
            }
        }
    }

    if (nezhaAlerts.isNotEmpty()) {
        item(key = "__hub_nodes_alerts_hdr__") {
            VesperSectionHeader(
                title = "哪吒探针动态与告警",
                count = nezhaAlerts.size,
                trailing = {
                    VesperSecondaryButton(
                        text = "全部通知",
                        cornerRadius = 999.dp,
                        minHeight = 32.dp,
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
                        onClick = onOpenServiceFeed
                    )
                },
                modifier = Modifier.padding(top = 4.dp)
            )
        }

        items(nezhaAlerts.take(5), key = { "nezha_alert_${it.id}" }, contentType = { "node_alert" }) { alert ->
            val theme = BotHubThemeTokens
            val tone = if (alert.isUnread || alert.isHighPriority) theme.danger else theme.warning
            VesperCard(
                cornerRadius = 14.dp,
                elevated = alert.isUnread,
                accentTone = if (alert.isUnread) tone else null,
                onClick = onOpenServiceFeed,
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier.padding(horizontal = 13.dp, vertical = 11.dp),
                    verticalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(6.dp),
                            modifier = Modifier.weight(1f).padding(end = 8.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(7.dp)
                                    .background(tone, CircleShape)
                            )
                            Text(
                                text = alert.title,
                                fontSize = 13.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textPrimary,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                        Text(
                            text = if (alert.createdAt.length >= 16) alert.createdAt.substring(5, 16) else alert.createdAt,
                            fontFamily = FontFamily.Monospace,
                            fontSize = 10.5.sp,
                            color = theme.textMuted
                        )
                    }
                    val snippet = alert.previewText(90)
                    if (snippet.isNotBlank()) {
                        Text(
                            text = snippet,
                            fontSize = 12.sp,
                            color = theme.textSecondary,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }
            }
        }
    }

    item(key = "__hub_nodes_fleet_hdr__") {
        VesperSectionHeader(
            title = "计算与服务器台账",
            count = computeNodes.size,
            modifier = Modifier.padding(top = 4.dp)
        )
    }

    if (computeNodes.isEmpty()) {
        item(key = "__hub_nodes_empty__") {
            VesperEmptyState(
                icon = Icons.Outlined.Dns,
                title = "暂无算力节点资产",
                subtitle = "点击「AI 实时巡检探针」实时拉取哪吒面板状态，或在「资产」中添加算力服务条目"
            )
        }
    }

    items(computeNodes, key = { "node_${it.id}" }, contentType = { "compute_node" }) { node ->
        val theme = BotHubThemeTokens
        val isInactive = node.status == "已停用" || node.status == "已归档"
        val isPermanent = node.dueDate.startsWith("2099") || node.daysLeft > 3650
        val statusColor = when {
            isInactive -> theme.textMuted
            !isPermanent && node.daysLeft <= 7 -> theme.danger
            !isPermanent && node.daysLeft <= 30 -> theme.warning
            else -> theme.success
        }

        VesperCard(
            cornerRadius = 15.dp,
            onClick = { onEditNodeAsset(node) },
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
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        modifier = Modifier.weight(1f).padding(end = 8.dp)
                    ) {
                        Box(
                            modifier = Modifier
                                .size(8.dp)
                                .background(statusColor, CircleShape)
                        )
                        Text(
                            text = node.name,
                            fontSize = 14.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = if (isInactive) theme.textMuted else theme.textPrimary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                    VesperTag(
                        text = if (isInactive) node.status else if (isPermanent) "长期在线" else node.dueBadgeLabel,
                        toneColor = statusColor
                    )
                }

                if (node.spec.isNotBlank()) {
                    Text(
                        text = node.spec,
                        fontSize = 12.sp,
                        color = theme.textSecondary,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis
                    )
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = node.price.replace(" / ", "/").ifBlank { "未标价" },
                        fontFamily = FontFamily.Monospace,
                        fontSize = 11.5.sp,
                        color = theme.textMuted
                    )
                    if (!isPermanent && node.dueDate.isNotBlank()) {
                        Text(
                            text = "到期 ${node.dueDate.take(10)}",
                            fontFamily = FontFamily.Monospace,
                            fontSize = 11.sp,
                            color = theme.textMuted
                        )
                    }
                }
            }
        }
    }
}
