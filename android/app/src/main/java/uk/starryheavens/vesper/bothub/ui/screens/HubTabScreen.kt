package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import uk.starryheavens.vesper.bothub.data.DashboardSnapshot
import uk.starryheavens.vesper.bothub.data.TaskItem
import uk.starryheavens.vesper.bothub.ui.components.MetricMiniCard

@Composable
fun HubTabScreen(
    snapshot: DashboardSnapshot?,
    onRunTask: (TaskItem) -> Unit,
    onToggleTask: (TaskItem, Boolean) -> Unit,
    onSyncAssets: () -> Unit,
    modifier: Modifier = Modifier
) {
    LazyColumn(
        modifier = modifier,
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                MetricMiniCard(
                    modifier = Modifier.weight(1f),
                    label = "数字资产",
                    value = (snapshot?.totalAssets ?: 0).toString()
                )
                MetricMiniCard(
                    modifier = Modifier.weight(1f),
                    label = "7天内临期",
                    value = (snapshot?.expiringCount ?: 0).toString()
                )
                MetricMiniCard(
                    modifier = Modifier.weight(1f),
                    label = "活跃任务",
                    value = (snapshot?.activeTasks ?: 0).toString()
                )
            }
        }

        item {
            Text(
                text = "自动化情报与调度任务",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold
            )
        }

        val tasks = snapshot?.tasks ?: emptyList()
        items(tasks, key = { it.id }) { task ->
            Card(
                shape = RoundedCornerShape(16.dp),
                colors = CardDefaults.cardColors(
                    containerColor = MaterialTheme.colorScheme.surfaceVariant
                ),
                modifier = Modifier.fillMaxWidth()
            ) {
                ListItem(
                    headlineContent = { Text(task.name, fontWeight = FontWeight.SemiBold) },
                    supportingContent = {
                        Text("${task.cronExpr} · 下次: ${task.nextRunAt}\n${task.lastSummary}")
                    },
                    trailingContent = {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            Switch(
                                checked = task.enabled,
                                onCheckedChange = { checked -> onToggleTask(task, checked) }
                            )
                            Button(
                                onClick = { onRunTask(task) },
                                modifier = Modifier.sizeIn(minWidth = 48.dp, minHeight = 48.dp)
                            ) {
                                Icon(Icons.Default.PlayArrow, contentDescription = "触发")
                                Text("触发")
                            }
                        }
                    }
                )
            }
        }

        item {
            Spacer(modifier = Modifier.height(4.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "数字资产与会员守望",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold
                )
                OutlinedButton(
                    onClick = onSyncAssets,
                    modifier = Modifier.sizeIn(minHeight = 48.dp)
                ) {
                    Icon(Icons.Default.Refresh, contentDescription = null)
                    Text(" 嗅探同步", style = MaterialTheme.typography.labelMedium)
                }
            }
        }

        val assets = snapshot?.assets ?: emptyList()
        items(assets, key = { it.id }) { asset ->
            Card(
                shape = RoundedCornerShape(16.dp),
                colors = CardDefaults.cardColors(
                    containerColor = MaterialTheme.colorScheme.surfaceVariant
                ),
                modifier = Modifier.fillMaxWidth()
            ) {
                ListItem(
                    headlineContent = { Text(asset.name, fontWeight = FontWeight.SemiBold) },
                    supportingContent = {
                        Text("${asset.spec} · ${asset.price}\n到期: ${asset.dueDate.take(10)}")
                    },
                    trailingContent = {
                        Text(
                            text = asset.dueBadgeLabel,
                            style = MaterialTheme.typography.labelLarge,
                            color = if (asset.isExpiringSoon) {
                                MaterialTheme.colorScheme.tertiary
                            } else {
                                MaterialTheme.colorScheme.primary
                            }
                        )
                    }
                )
            }
        }
    }
}
