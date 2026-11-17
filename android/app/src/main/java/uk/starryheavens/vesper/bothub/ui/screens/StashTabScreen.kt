package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.animation.animateContentSize
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.DeleteOutline
import androidx.compose.material.icons.filled.OpenInNew
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import uk.starryheavens.vesper.bothub.data.StashItemUi

@Composable
fun StashTabScreen(
    items: List<StashItemUi>,
    query: String,
    activeFilter: String,
    loading: Boolean,
    onQueryChange: (String) -> Unit,
    onFilterChange: (String) -> Unit,
    onCapture: (String, String) -> Unit,
    onOpenUrl: (StashItemUi) -> Unit,
    onTogglePin: (StashItemUi) -> Unit,
    onReprocess: (StashItemUi) -> Unit,
    onDelete: (StashItemUi) -> Unit,
    onAskAi: (StashItemUi) -> Unit,
    modifier: Modifier = Modifier
) {
    var captureInput by remember { mutableStateOf("") }
    var captureNote by remember { mutableStateOf("") }
    var showCaptureBar by remember { mutableStateOf(false) }

    val filters = listOf(
        "unread" to "未读干货",
        "all" to "全部",
        "pinned" to "置顶",
        "note" to "纯闪念",
        "archived" to "已归档"
    )

    Column(modifier = modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            OutlinedTextField(
                value = query,
                onValueChange = onQueryChange,
                leadingIcon = { Icon(Icons.Default.Search, contentDescription = null) },
                placeholder = { Text("检索收藏标题 / AI 结论 / 正文...") },
                singleLine = true,
                shape = RoundedCornerShape(14.dp),
                modifier = Modifier.weight(1f)
            )
            OutlinedButton(
                onClick = { showCaptureBar = !showCaptureBar },
                shape = RoundedCornerShape(14.dp),
                modifier = Modifier.sizeIn(minHeight = 48.dp)
            ) {
                Text(if (showCaptureBar) "收起" else "+ 秒收")
            }
        }

        if (showCaptureBar) {
            Spacer(modifier = Modifier.height(8.dp))
            Card(
                shape = RoundedCornerShape(16.dp),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier.padding(12.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    OutlinedTextField(
                        value = captureInput,
                        onValueChange = { captureInput = it },
                        placeholder = { Text("粘贴链接、B站分享文案或输入闪念...") },
                        maxLines = 3,
                        modifier = Modifier.fillMaxWidth()
                    )
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        OutlinedTextField(
                            value = captureNote,
                            onValueChange = { captureNote = it },
                            placeholder = { Text("附注 (可选)") },
                            singleLine = true,
                            modifier = Modifier.weight(1f)
                        )
                        Button(
                            onClick = {
                                val clean = captureInput.trim()
                                if (clean.isNotEmpty()) {
                                    onCapture(clean, captureNote.trim())
                                    captureInput = ""
                                    captureNote = ""
                                    showCaptureBar = false
                                }
                            },
                            enabled = captureInput.isNotBlank(),
                            modifier = Modifier.sizeIn(minHeight = 48.dp)
                        ) {
                            Text("投喂")
                        }
                    }
                }
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        LazyRow(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth()
        ) {
            items(filters) { (fid, label) ->
                FilterChip(
                    selected = activeFilter == fid,
                    onClick = { onFilterChange(fid) },
                    label = { Text(label) },
                    modifier = Modifier.sizeIn(minHeight = 48.dp)
                )
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            contentPadding = PaddingValues(bottom = 16.dp)
        ) {
            if (items.isEmpty()) {
                item {
                    Card(
                        shape = RoundedCornerShape(16.dp),
                        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Text(
                            text = if (loading) {
                                "正在同步收藏与闪念匣..."
                            } else if (query.isNotBlank()) {
                                "未检索到匹配的收藏条目"
                            } else {
                                "当前视图暂无收藏条目。在任意 App 点击「系统分享 -> Vesper BotHub」即可半屏无感秒收。"
                            },
                            style = MaterialTheme.typography.bodyMedium,
                            modifier = Modifier.padding(20.dp)
                        )
                    }
                }
            }

            items(items, key = { it.id }) { item ->
                var expanded by remember { mutableStateOf(false) }
                val sourceBadge = item.shareKind.badgeLabel

                Card(
                    shape = RoundedCornerShape(16.dp),
                    colors = CardDefaults.cardColors(
                        containerColor = if (item.isUnread) {
                            MaterialTheme.colorScheme.secondaryContainer
                        } else {
                            MaterialTheme.colorScheme.surfaceVariant
                        }
                    ),
                    modifier = Modifier
                        .fillMaxWidth()
                        .animateContentSize()
                        .clickable { expanded = !expanded }
                ) {
                    Column(
                        modifier = Modifier.padding(14.dp),
                        verticalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = buildString {
                                    append("[$sourceBadge]")
                                    if (item.pinned) append(" [置顶]")
                                    if (item.status == "processing") append(" [AI 提炼中]")
                                    if (item.domain.isNotEmpty()) append(" · ${item.domain}")
                                },
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.primary
                            )
                            Text(
                                text = item.createdAt.take(16),
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }

                        Text(
                            text = item.title.ifEmpty { item.rawInput.take(72) },
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.SemiBold
                        )

                        if (item.aiTldr.isNotEmpty()) {
                            Text(
                                text = item.aiTldr,
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                        } else if (item.status == "processing") {
                            Text(
                                text = "后台正在抓取网页快照并提炼核心结论...",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        } else if (item.errorMsg.isNotEmpty()) {
                            Text(
                                text = item.errorMsg,
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.tertiary
                            )
                        }

                        if (item.tags.isNotEmpty()) {
                            Text(
                                text = "标签: ${item.tags}",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }

                        if (expanded) {
                            if (item.points.isNotEmpty()) {
                                Spacer(modifier = Modifier.height(2.dp))
                                Text(
                                    text = "AI 干货要点",
                                    style = MaterialTheme.typography.labelMedium,
                                    color = MaterialTheme.colorScheme.tertiary
                                )
                                item.points.forEach { pt ->
                                    Text(
                                        text = "- $pt",
                                        style = MaterialTheme.typography.bodySmall
                                    )
                                }
                            }
                            if (item.userNote.isNotEmpty()) {
                                Spacer(modifier = Modifier.height(2.dp))
                                Text(
                                    text = "个人批注: ${item.userNote}",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }

                            Spacer(modifier = Modifier.height(4.dp))
                            LazyRow(
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                item {
                                    Button(
                                        onClick = { onOpenUrl(item) },
                                        modifier = Modifier.sizeIn(minHeight = 48.dp)
                                    ) {
                                        if (item.url.isNotEmpty()) {
                                            Icon(Icons.Default.OpenInNew, contentDescription = null)
                                            Text(" 打开原链")
                                        } else {
                                            Text("标为已阅")
                                        }
                                    }
                                }
                                item {
                                    OutlinedButton(
                                        onClick = { onAskAi(item) },
                                        modifier = Modifier.sizeIn(minHeight = 48.dp)
                                    ) {
                                        Text("AI 深聊")
                                    }
                                }
                                item {
                                    OutlinedButton(
                                        onClick = { onTogglePin(item) },
                                        modifier = Modifier.sizeIn(minHeight = 48.dp)
                                    ) {
                                        Text(if (item.pinned) "取消置顶" else "置顶")
                                    }
                                }
                                item {
                                    OutlinedButton(
                                        onClick = { onReprocess(item) },
                                        modifier = Modifier.sizeIn(minHeight = 48.dp)
                                    ) {
                                        Text("重提炼")
                                    }
                                }
                                item {
                                    OutlinedButton(
                                        onClick = { onDelete(item) },
                                        modifier = Modifier.sizeIn(minHeight = 48.dp)
                                    ) {
                                        Icon(Icons.Default.DeleteOutline, contentDescription = "删除")
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
