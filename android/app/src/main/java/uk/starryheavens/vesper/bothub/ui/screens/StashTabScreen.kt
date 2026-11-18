package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
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
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material.icons.filled.BookmarkBorder
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.DeleteOutline
import androidx.compose.material.icons.filled.OpenInNew
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Button
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
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
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.StashItemUi

@Composable
fun StashTabScreen(
    items: List<StashItemUi>,
    query: String,
    activeFilter: String,
    loading: Boolean,
    showCaptureBar: Boolean = false,
    onDismissCaptureBar: () -> Unit = {},
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

    val filters = listOf(
        "unread" to "未读干货",
        "all" to "全部",
        "pinned" to "置顶",
        "note" to "闪念",
        "archived" to "归档"
    )

    Column(modifier = modifier.fillMaxSize()) {
        // Recessed Glass Search Capsule
        Surface(
            shape = RoundedCornerShape(999.dp),
            color = Color(0xCC0A0F1E),
            border = BorderStroke(1.dp, Color(0x24FFF8EE)),
            modifier = Modifier
                .fillMaxWidth()
                .padding(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 4.dp)
                .sizeIn(minHeight = 42.dp)
        ) {
            Row(
                modifier = Modifier.padding(horizontal = 14.dp, vertical = 9.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Icon(
                    Icons.Default.Search,
                    contentDescription = null,
                    tint = Color(0xFF64748B),
                    modifier = Modifier.size(17.dp)
                )
                BasicTextField(
                    value = query,
                    onValueChange = onQueryChange,
                    singleLine = true,
                    textStyle = TextStyle(
                        color = Color(0xFFFFF8EE),
                        fontSize = 14.sp
                    ),
                    cursorBrush = SolidColor(Color(0xFFF2B880)),
                    modifier = Modifier.weight(1f),
                    decorationBox = { inner ->
                        Box(contentAlignment = Alignment.CenterStart) {
                            if (query.isEmpty()) {
                                Text(
                                    text = "检索收藏标题、AI 结论或标签...",
                                    fontSize = 13.5.sp,
                                    color = Color(0xFF64748B)
                                )
                            }
                            inner()
                        }
                    }
                )
                if (query.isNotEmpty()) {
                    Surface(
                        onClick = { onQueryChange("") },
                        shape = CircleShape,
                        color = Color(0x22FFFFFF),
                        modifier = Modifier.size(20.dp)
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Icon(
                                Icons.Default.Close,
                                contentDescription = "清空搜索",
                                tint = Color(0xFFCBD5E1),
                                modifier = Modifier.size(12.dp)
                            )
                        }
                    }
                }
            }
        }

        // Slide-Down Quick Capture Panel (controlled from TopBar "+ 秒收")
        AnimatedVisibility(
            visible = showCaptureBar,
            enter = fadeIn() + expandVertically(),
            exit = fadeOut() + shrinkVertically()
        ) {
            Surface(
                shape = RoundedCornerShape(18.dp),
                color = Color(0xE60E152A),
                border = BorderStroke(1.dp, Color(0x52F2B880)),
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 6.dp)
            ) {
                Column(
                    modifier = Modifier.padding(14.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    OutlinedTextField(
                        value = captureInput,
                        onValueChange = { captureInput = it },
                        placeholder = { Text("粘贴链接、B站分享文案或输入随手闪念...") },
                        maxLines = 3,
                        shape = RoundedCornerShape(14.dp),
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
                            placeholder = { Text("个人附注 (可选)") },
                            singleLine = true,
                            shape = RoundedCornerShape(14.dp),
                            modifier = Modifier.weight(1f)
                        )
                        Button(
                            onClick = {
                                val clean = captureInput.trim()
                                if (clean.isNotEmpty()) {
                                    onCapture(clean, captureNote.trim())
                                    captureInput = ""
                                    captureNote = ""
                                    onDismissCaptureBar()
                                }
                            },
                            enabled = captureInput.isNotBlank(),
                            shape = RoundedCornerShape(14.dp),
                            modifier = Modifier.sizeIn(minHeight = 48.dp)
                        ) {
                            Text("投喂提炼", fontWeight = FontWeight.SemiBold)
                        }
                    }
                }
            }
        }

        // Filter Pills
        LazyRow(
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 6.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth()
        ) {
            items(filters) { (fid, label) ->
                val selected = activeFilter == fid
                Surface(
                    onClick = { onFilterChange(fid) },
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

        // Stash Feed
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 20.dp)
        ) {
            if (items.isEmpty()) {
                item {
                    Surface(
                        shape = RoundedCornerShape(20.dp),
                        color = Color(0x990D1326),
                        border = BorderStroke(1.dp, Color(0x1FFFF8EE)),
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(top = 20.dp)
                    ) {
                        Column(
                            modifier = Modifier.padding(horizontal = 24.dp, vertical = 30.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(6.dp)
                        ) {
                            Text(
                                text = when {
                                    loading -> "正在同步收藏与闪念匣..."
                                    query.isNotBlank() -> "未检索到匹配的收藏条目"
                                    else -> "当前视图暂无收藏条目"
                                },
                                style = MaterialTheme.typography.titleSmall,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFFFFF8EE)
                            )
                            if (!loading && query.isBlank()) {
                                Text(
                                    text = "在任意 App 点击「系统分享 -> BotHub」或右上角「+ 秒收」即可自动抓取并提炼干货",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = Color(0xFF94A3B8),
                                    textAlign = TextAlign.Center
                                )
                            }
                        }
                    }
                }
            }

            items(items, key = { it.id }) { item ->
                var expanded by remember { mutableStateOf(false) }
                val sourceBadge = item.shareKind.badgeLabel

                Surface(
                    onClick = { expanded = !expanded },
                    shape = RoundedCornerShape(18.dp),
                    color = if (item.isUnread) Color(0xE6111932) else Color(0xA60B1020),
                    border = BorderStroke(
                        width = 1.dp,
                        color = when {
                            item.pinned -> Color(0x73F2B880)
                            item.isUnread -> Color(0x47F2B880)
                            else -> Color(0x1CFFF8EE)
                        }
                    ),
                    modifier = Modifier
                        .fillMaxWidth()
                        .animateContentSize()
                ) {
                    Column(
                        modifier = Modifier.padding(horizontal = 15.dp, vertical = 13.dp),
                        verticalArrangement = Arrangement.spacedBy(7.dp)
                    ) {
                        // Top Metadata Badges
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(6.dp),
                                modifier = Modifier.weight(1f)
                            ) {
                                if (item.isUnread) {
                                    Box(
                                        modifier = Modifier
                                            .size(6.dp)
                                            .background(Color(0xFFF2B880), CircleShape)
                                    )
                                }
                                Surface(
                                    shape = RoundedCornerShape(6.dp),
                                    color = Color(0x24F2B880),
                                    border = BorderStroke(0.5.dp, Color(0x52F2B880))
                                ) {
                                    Text(
                                        text = sourceBadge,
                                        fontSize = 10.5.sp,
                                        fontWeight = FontWeight.Medium,
                                        color = Color(0xFFFFE2BE),
                                        modifier = Modifier.padding(horizontal = 7.dp, vertical = 2.dp)
                                    )
                                }
                                if (item.pinned) {
                                    Text(
                                        text = "置顶",
                                        fontSize = 10.5.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        color = Color(0xFFFBBF24)
                                    )
                                }
                                if (item.status == "processing") {
                                    Text(
                                        text = "提炼中",
                                        fontSize = 10.5.sp,
                                        color = Color(0xFF34D399)
                                    )
                                } else if (item.domain.isNotEmpty()) {
                                    Text(
                                        text = item.domain,
                                        fontFamily = FontFamily.Monospace,
                                        fontSize = 11.sp,
                                        color = Color(0xFF64748B),
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis
                                    )
                                }
                            }

                            Text(
                                text = if (item.createdAt.length >= 16) item.createdAt.substring(5, 16) else item.createdAt,
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.sp,
                                color = Color(0xFF64748B)
                            )
                        }

                        // Title
                        Text(
                            text = item.title.ifEmpty { item.rawInput.take(72) },
                            style = MaterialTheme.typography.titleSmall,
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFF8EE),
                            maxLines = if (expanded) 4 else 2,
                            overflow = TextOverflow.Ellipsis
                        )

                        // AI TL;DR Summary
                        if (item.aiTldr.isNotEmpty()) {
                            Text(
                                text = item.aiTldr,
                                style = MaterialTheme.typography.bodySmall,
                                fontSize = 13.5.sp,
                                lineHeight = 20.sp,
                                color = Color(0xFFD8E0F0),
                                maxLines = if (expanded) 6 else 3,
                                overflow = TextOverflow.Ellipsis
                            )
                        } else if (item.status == "processing") {
                            Text(
                                text = "后台正在抓取网页快照并提炼核心结论...",
                                fontSize = 12.5.sp,
                                color = Color(0xFF94A3B8)
                            )
                        } else if (item.errorMsg.isNotEmpty()) {
                            Text(
                                text = item.errorMsg,
                                fontSize = 12.5.sp,
                                color = Color(0xFFFDA4AF)
                            )
                        }

                        // Tags Row
                        val tagList = remember(item.tags) {
                            item.tags.split(',', '，').map { it.trim() }.filter { it.isNotEmpty() }.take(4)
                        }
                        if (tagList.isNotEmpty()) {
                            Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                tagList.forEach { tag ->
                                    Surface(
                                        shape = RoundedCornerShape(999.dp),
                                        color = Color(0xFF141C34)
                                    ) {
                                        Text(
                                            text = "#$tag",
                                            fontSize = 10.5.sp,
                                            color = Color(0xFF94A3B8),
                                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.5.dp)
                                        )
                                    }
                                }
                            }
                        }

                        // Expanded Key Points + Compact Action Bar
                        if (expanded) {
                            HorizontalDivider(
                                modifier = Modifier.padding(vertical = 3.dp),
                                thickness = 0.5.dp,
                                color = Color(0x1FFFF8EE)
                            )

                            if (item.points.isNotEmpty()) {
                                Column(verticalArrangement = Arrangement.spacedBy(5.dp)) {
                                    Text(
                                        text = "核心干货要点",
                                        fontSize = 11.5.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        color = Color(0xFF34D399)
                                    )
                                    item.points.forEach { pt ->
                                        Row(
                                            modifier = Modifier.fillMaxWidth(),
                                            verticalAlignment = Alignment.Top
                                        ) {
                                            Text(
                                                text = "• ",
                                                color = Color(0xFFF2B880),
                                                fontSize = 13.sp,
                                                fontWeight = FontWeight.Bold
                                            )
                                            Text(
                                                text = pt,
                                                fontSize = 13.sp,
                                                lineHeight = 19.sp,
                                                color = Color(0xFFCBD5E1)
                                            )
                                        }
                                    }
                                }
                            }

                            if (item.userNote.isNotEmpty()) {
                                Text(
                                    text = "附注: ${item.userNote}",
                                    fontSize = 12.5.sp,
                                    color = Color(0xFFFFE2BE)
                                )
                            }

                            Spacer(modifier = Modifier.height(4.dp))

                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Button(
                                    onClick = { onOpenUrl(item) },
                                    shape = RoundedCornerShape(12.dp),
                                    contentPadding = PaddingValues(horizontal = 12.dp, vertical = 8.dp),
                                    modifier = Modifier
                                        .weight(1f)
                                        .sizeIn(minHeight = 42.dp)
                                ) {
                                    if (item.url.isNotEmpty()) {
                                        Icon(
                                            Icons.Default.OpenInNew,
                                            contentDescription = null,
                                            modifier = Modifier.size(15.dp)
                                        )
                                        Text(" 原链", fontSize = 12.5.sp, fontWeight = FontWeight.SemiBold)
                                    } else {
                                        Text("标为已阅", fontSize = 12.5.sp, fontWeight = FontWeight.SemiBold)
                                    }
                                }

                                OutlinedButton(
                                    onClick = { onAskAi(item) },
                                    shape = RoundedCornerShape(12.dp),
                                    contentPadding = PaddingValues(horizontal = 12.dp, vertical = 8.dp),
                                    modifier = Modifier
                                        .weight(1f)
                                        .sizeIn(minHeight = 42.dp)
                                ) {
                                    Text("AI 深聊", fontSize = 12.5.sp)
                                }

                                CompactStashIconBtn(
                                    icon = if (item.pinned) Icons.Default.Bookmark else Icons.Default.BookmarkBorder,
                                    tint = if (item.pinned) Color(0xFFF2B880) else Color(0xFFCBD5E1),
                                    contentDescription = if (item.pinned) "取消置顶" else "置顶",
                                    onClick = { onTogglePin(item) }
                                )

                                CompactStashIconBtn(
                                    icon = Icons.Default.Refresh,
                                    tint = Color(0xFFCBD5E1),
                                    contentDescription = "重新提炼",
                                    onClick = { onReprocess(item) }
                                )

                                CompactStashIconBtn(
                                    icon = Icons.Default.DeleteOutline,
                                    tint = Color(0xFFFDA4AF),
                                    contentDescription = "删除",
                                    onClick = { onDelete(item) }
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
private fun CompactStashIconBtn(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    tint: Color,
    contentDescription: String,
    onClick: () -> Unit
) {
    Surface(
        onClick = onClick,
        shape = RoundedCornerShape(12.dp),
        color = Color(0xFF131B33),
        border = BorderStroke(1.dp, Color(0x24FFF8EE)),
        modifier = Modifier.size(42.dp)
    ) {
        Box(contentAlignment = Alignment.Center) {
            Icon(
                icon,
                contentDescription = contentDescription,
                tint = tint,
                modifier = Modifier.size(17.dp)
            )
        }
    }
}
