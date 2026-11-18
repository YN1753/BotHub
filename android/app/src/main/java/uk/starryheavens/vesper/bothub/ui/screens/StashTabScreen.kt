package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material.icons.filled.BookmarkBorder
import androidx.compose.material.icons.filled.ContentPaste
import androidx.compose.material.icons.filled.DeleteOutline
import androidx.compose.material.icons.filled.OpenInNew
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
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
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.StashItemUi
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.RecessedSearchBar
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.TactileFilterChip
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.specularRimBrush
import uk.starryheavens.vesper.bothub.ui.components.tactileClickable
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

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
    val theme = BotHubThemeTokens
    val clipboard = LocalClipboardManager.current
    var captureInput by remember { mutableStateOf("") }
    var captureNote by remember { mutableStateOf("") }

    val filters = listOf(
        "all" to "全部",
        "unread" to "未读",
        "pinned" to "置顶",
        "note" to "闪念"
    )

    Column(modifier = modifier.fillMaxSize()) {
        RecessedSearchBar(
            value = query,
            onValueChange = onQueryChange,
            placeholder = "搜索收藏",
            modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 4.dp)
        )

        AnimatedVisibility(
            visible = showCaptureBar,
            enter = fadeIn() + expandVertically(),
            exit = fadeOut() + shrinkVertically()
        ) {
            SpecularGlassCard(
                cornerRadius = 18.dp,
                accentTint = theme.accent,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 6.dp)
            ) {
                Column(
                    modifier = Modifier.padding(14.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "快速添加收藏",
                            fontSize = 13.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textPrimary
                        )
                        TactileSecondaryButton(
                            text = "粘贴剪贴板",
                            leadingIcon = Icons.Default.ContentPaste,
                            accent = true,
                            cornerRadius = 999.dp,
                            minHeight = 29.dp,
                            contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
                            onClick = {
                                val clip = clipboard.getText()?.text?.trim().orEmpty()
                                if (clip.isNotEmpty()) {
                                    captureInput = clip
                                }
                            }
                        )
                    }
                    ObsidianGlassInput(
                        value = captureInput,
                        onValueChange = { captureInput = it },
                        placeholder = "输入链接、分享文案或随手笔记...",
                        singleLine = false,
                        maxLines = 3,
                        modifier = Modifier.fillMaxWidth()
                    )
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        ObsidianGlassInput(
                            value = captureNote,
                            onValueChange = { captureNote = it },
                            placeholder = "备注 (可选)",
                            singleLine = true,
                            modifier = Modifier.weight(1f)
                        )
                        TactilePrimaryButton(
                            text = "保存",
                            enabled = captureInput.isNotBlank(),
                            minHeight = 46.dp,
                            onClick = {
                                val clean = captureInput.trim()
                                if (clean.isNotEmpty()) {
                                    onCapture(clean, captureNote.trim())
                                    captureInput = ""
                                    captureNote = ""
                                    onDismissCaptureBar()
                                }
                            }
                        )
                    }
                }
            }
        }

        // Tactile Filter Pills
        LazyRow(
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 6.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth()
        ) {
            items(filters) { (fid, label) ->
                TactileFilterChip(
                    label = label,
                    selected = activeFilter == fid,
                    onClick = { onFilterChange(fid) }
                )
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
                    SpecularGlassCard(
                        cornerRadius = 20.dp,
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
                                color = theme.textPrimary
                            )
                            if (!loading && query.isBlank()) {
                                Text(
                                    text = "在任意 App 分享链接至 BotHub，或一键粘贴剪贴板秒收",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = theme.textMuted,
                                    textAlign = TextAlign.Center
                                )
                                Spacer(modifier = Modifier.height(6.dp))
                                TactileSecondaryButton(
                                    text = "粘贴剪贴板秒收",
                                    leadingIcon = Icons.Default.ContentPaste,
                                    accent = true,
                                    cornerRadius = 999.dp,
                                    minHeight = 36.dp,
                                    contentPadding = PaddingValues(horizontal = 16.dp, vertical = 7.dp),
                                    onClick = {
                                        val clip = clipboard.getText()?.text?.trim().orEmpty()
                                        if (clip.isNotEmpty()) {
                                            onCapture(clip, "")
                                        }
                                    }
                                )
                            }
                        }
                    }
                }
            }

            items(items, key = { it.id }) { item ->
                var expanded by remember { mutableStateOf(false) }
                val sourceBadge = item.shareKind.badgeLabel

                SpecularGlassCard(
                    cornerRadius = 18.dp,
                    topColor = if (item.isUnread) theme.cardUnreadTop else theme.cardTop,
                    bottomColor = if (item.isUnread) theme.cardUnreadBottom else theme.cardBottom,
                    accentTint = when {
                        item.pinned -> Color(0xFFF59E0B)
                        item.isUnread -> theme.accent
                        else -> null
                    },
                    onClick = { expanded = !expanded },
                    modifier = Modifier
                        .fillMaxWidth()
                        .animateContentSize()
                ) {
                    Column(
                        modifier = Modifier.padding(horizontal = 15.dp, vertical = 13.dp),
                        verticalArrangement = Arrangement.spacedBy(7.dp)
                    ) {
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
                                            .background(theme.accent, CircleShape)
                                    )
                                }
                                Box(
                                    modifier = Modifier
                                        .clip(RoundedCornerShape(6.dp))
                                        .background(theme.accent.copy(alpha = 0.14f))
                                        .border(0.5.dp, theme.accent.copy(alpha = 0.36f), RoundedCornerShape(6.dp))
                                        .padding(horizontal = 7.dp, vertical = 2.dp)
                                ) {
                                    Text(
                                        text = sourceBadge,
                                        style = TextStyle(
                                            fontSize = 10.sp,
                                            fontWeight = FontWeight.Medium,
                                            color = if (theme.isLight) theme.accentDeep else theme.accentLight,
                                            platformStyle = NoFontPaddingStyle
                                        ),
                                        maxLines = 1,
                                        softWrap = false
                                    )
                                }
                                if (item.pinned) {
                                    Icon(
                                        imageVector = Icons.Default.Bookmark,
                                        contentDescription = "已置顶",
                                        tint = Color(0xFFF59E0B),
                                        modifier = Modifier.size(12.dp)
                                    )
                                }
                                if (item.status == "processing") {
                                    Text(
                                        text = "处理中",
                                        fontSize = 10.5.sp,
                                        color = Color(0xFF10B981),
                                        maxLines = 1,
                                        softWrap = false
                                    )
                                } else if (item.domain.isNotEmpty()) {
                                    Text(
                                        text = item.domain,
                                        fontFamily = FontFamily.Monospace,
                                        fontSize = 11.sp,
                                        color = theme.textMuted,
                                        maxLines = 1,
                                        softWrap = false,
                                        overflow = TextOverflow.Ellipsis,
                                        modifier = Modifier.weight(1f, fill = false)
                                    )
                                }
                            }

                            Text(
                                text = if (item.createdAt.length >= 16) item.createdAt.substring(5, 16) else item.createdAt,
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.sp,
                                color = theme.textMuted,
                                maxLines = 1,
                                softWrap = false
                            )
                        }

                        Text(
                            text = item.title.ifEmpty { item.rawInput.take(72) },
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textPrimary,
                            maxLines = if (expanded) 4 else 2,
                            overflow = TextOverflow.Ellipsis
                        )

                        if (item.aiTldr.isNotEmpty()) {
                            Text(
                                text = item.aiTldr,
                                fontSize = 13.5.sp,
                                lineHeight = 20.sp,
                                color = theme.textSecondary,
                                maxLines = if (expanded) 6 else 3,
                                overflow = TextOverflow.Ellipsis
                            )
                        } else if (item.status == "processing") {
                            Text(
                                text = "后台正在抓取网页快照并提炼核心结论...",
                                fontSize = 12.5.sp,
                                color = theme.textMuted
                            )
                        } else if (item.errorMsg.isNotEmpty()) {
                            Text(
                                text = item.errorMsg,
                                fontSize = 12.5.sp,
                                color = if (theme.isLight) Color(0xFFE11D48) else Color(0xFFFDA4AF)
                            )
                        }

                        val tagList = remember(item.tags) {
                            item.tags.split(',', '，').map { it.trim() }.filter { it.isNotEmpty() }.take(4)
                        }
                        if (tagList.isNotEmpty()) {
                            Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                tagList.forEach { tag ->
                                    Box(
                                        modifier = Modifier
                                            .clip(RoundedCornerShape(999.dp))
                                            .background(theme.inputTop)
                                            .padding(horizontal = 8.dp, vertical = 2.5.dp)
                                    ) {
                                        Text(
                                            text = "#$tag",
                                            fontSize = 10.5.sp,
                                            color = theme.textMuted
                                        )
                                    }
                                }
                            }
                        }

                        if (expanded) {
                            HorizontalDivider(
                                modifier = Modifier.padding(vertical = 3.dp),
                                thickness = 0.5.dp,
                                color = theme.divider
                            )

                            if (item.points.isNotEmpty()) {
                                Column(verticalArrangement = Arrangement.spacedBy(5.dp)) {
                                    Text(
                                        text = "核心干货要点",
                                        fontSize = 11.5.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        color = Color(0xFF10B981)
                                    )
                                    item.points.forEach { pt ->
                                        Row(
                                            modifier = Modifier.fillMaxWidth(),
                                            verticalAlignment = Alignment.Top
                                        ) {
                                            Box(
                                                modifier = Modifier
                                                    .width(14.dp)
                                                    .padding(top = 7.5.dp),
                                                contentAlignment = Alignment.TopStart
                                            ) {
                                                Box(
                                                    modifier = Modifier
                                                        .size(4.5.dp)
                                                        .background(theme.accent, CircleShape)
                                                )
                                            }
                                            Text(
                                                text = pt,
                                                fontSize = 13.sp,
                                                lineHeight = 19.sp,
                                                color = theme.textSecondary,
                                                modifier = Modifier.weight(1f)
                                            )
                                        }
                                    }
                                }
                            }

                            if (item.userNote.isNotEmpty()) {
                                Text(
                                    text = "附注: ${item.userNote}",
                                    fontSize = 12.5.sp,
                                    color = theme.accent
                                )
                            }

                            Spacer(modifier = Modifier.height(4.dp))

                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                TactilePrimaryButton(
                                    text = if (item.url.isNotEmpty()) "打开链接" else "标为已读",
                                    leadingIcon = if (item.url.isNotEmpty()) Icons.Default.OpenInNew else null,
                                    cornerRadius = 12.dp,
                                    minHeight = 38.dp,
                                    contentPadding = PaddingValues(horizontal = 10.dp, vertical = 6.dp),
                                    onClick = { onOpenUrl(item) },
                                    modifier = Modifier.weight(1f)
                                )

                                TactileSecondaryButton(
                                    text = "发到聊天",
                                    cornerRadius = 12.dp,
                                    minHeight = 38.dp,
                                    contentPadding = PaddingValues(horizontal = 10.dp, vertical = 6.dp),
                                    onClick = { onAskAi(item) },
                                    modifier = Modifier.weight(1f)
                                )

                                CompactStashIconBtn(
                                    icon = if (item.pinned) Icons.Default.Bookmark else Icons.Default.BookmarkBorder,
                                    tint = if (item.pinned) theme.accent else theme.textSecondary,
                                    contentDescription = if (item.pinned) "取消置顶" else "置顶",
                                    onClick = { onTogglePin(item) }
                                )

                                if (item.status == "failed" || item.aiTldr.isEmpty()) {
                                    CompactStashIconBtn(
                                        icon = Icons.Default.Refresh,
                                        tint = theme.textSecondary,
                                        contentDescription = "重新提炼",
                                        onClick = { onReprocess(item) }
                                    )
                                }

                                CompactStashIconBtn(
                                    icon = Icons.Default.DeleteOutline,
                                    tint = if (theme.isLight) Color(0xFFE11D48) else Color(0xFFFDA4AF),
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
    icon: ImageVector,
    tint: Color,
    contentDescription: String,
    onClick: () -> Unit
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(12.dp)
    Box(
        modifier = Modifier
            .size(40.dp)
            .tactileClickable(pressedScale = 0.90f, hapticOnTap = true, onClick = onClick)
            .clip(shape)
            .background(
                Brush.verticalGradient(listOf(theme.secondaryBtnTop, theme.secondaryBtnBottom)),
                shape
            )
            .border(
                BorderStroke(
                    0.8.dp,
                    specularRimBrush(theme.rimHighlight, theme.rimMid, Color.Transparent)
                ),
                shape
            ),
        contentAlignment = Alignment.Center
    ) {
        Icon(
            imageVector = icon,
            contentDescription = contentDescription,
            tint = tint,
            modifier = Modifier.size(17.dp)
        )
    }
}
