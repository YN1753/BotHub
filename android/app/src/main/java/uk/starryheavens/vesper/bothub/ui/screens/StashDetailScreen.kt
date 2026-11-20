package uk.starryheavens.vesper.bothub.ui.screens

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material.icons.filled.BookmarkBorder
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material.icons.filled.ExpandLess
import androidx.compose.material.icons.filled.ExpandMore
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.OpenInNew
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.StashItemUi
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun StashDetailScreen(
    item: StashItemUi,
    onBack: () -> Unit,
    onOpenUrl: (StashItemUi) -> Unit,
    onCopyText: (String) -> Unit,
    onTogglePin: (StashItemUi) -> Unit,
    onToggleRead: (StashItemUi) -> Unit,
    onReprocess: (StashItemUi) -> Unit,
    onSaveEdit: (StashItemUi, String, String, String) -> Unit,
    onAskAi: (StashItemUi) -> Unit,
    onConvertToTodo: (StashItemUi) -> Unit,
    onConvertToNote: (StashItemUi) -> Unit,
    onDelete: (StashItemUi) -> Unit,
    modifier: Modifier = Modifier
) {
    BackHandler(onBack = onBack)

    val theme = BotHubThemeTokens
    var menuExpanded by remember { mutableStateOf(false) }
    var editing by remember(item.id) { mutableStateOf(false) }
    var editTitle by remember(item.id, item.title) { mutableStateOf(item.title) }
    var editNote by remember(item.id, item.userNote) { mutableStateOf(item.userNote) }
    var editTags by remember(item.id, item.tags) { mutableStateOf(item.tags) }
    var excerptExpanded by remember(item.id) { mutableStateOf(item.aiTldr.isBlank()) }

    val tagList = remember(item.tags) {
        item.tags.split(',', '，').map { it.trim() }.filter { it.isNotEmpty() }
    }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(theme.bgBase)
            .imePadding()
    ) {
        // Top Navigation Bar
        Surface(
            color = theme.sheetBg,
            tonalElevation = 0.dp,
            modifier = Modifier.fillMaxWidth()
        ) {
            Column {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .statusBarsPadding()
                        .padding(horizontal = 10.dp, vertical = 10.dp),
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
                                .size(36.dp)
                                .clip(RoundedCornerShape(10.dp))
                                .clickable { onBack() },
                            contentAlignment = Alignment.Center
                        ) {
                            Icon(
                                imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                                contentDescription = "返回收藏列表",
                                tint = theme.textPrimary,
                                modifier = Modifier.size(20.dp)
                            )
                        }

                        Box(
                            modifier = Modifier
                                .clip(RoundedCornerShape(6.dp))
                                .background(theme.accent.copy(alpha = 0.14f))
                                .border(0.5.dp, theme.accent.copy(alpha = 0.34f), RoundedCornerShape(6.dp))
                                .padding(horizontal = 8.dp, vertical = 3.dp)
                        ) {
                            Text(
                                text = item.shareKind.badgeLabel,
                                style = TextStyle(
                                    fontSize = 11.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = if (theme.isLight) theme.accentDeep else theme.accentLight,
                                    platformStyle = NoFontPaddingStyle
                                ),
                                maxLines = 1
                            )
                        }

                        if (item.domain.isNotBlank()) {
                            Text(
                                text = item.domain,
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.5.sp,
                                color = theme.textMuted,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                                modifier = Modifier.weight(1f, fill = false)
                            )
                        }
                    }

                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(4.dp)
                    ) {
                        Box(
                            modifier = Modifier
                                .size(36.dp)
                                .clip(RoundedCornerShape(10.dp))
                                .clickable { onTogglePin(item) },
                            contentAlignment = Alignment.Center
                        ) {
                            Icon(
                                imageVector = if (item.pinned) Icons.Default.Bookmark else Icons.Default.BookmarkBorder,
                                contentDescription = if (item.pinned) "取消置顶" else "置顶",
                                tint = if (item.pinned) Color(0xFFF59E0B) else theme.textSecondary,
                                modifier = Modifier.size(20.dp)
                            )
                        }

                        Box {
                            Box(
                                modifier = Modifier
                                    .size(36.dp)
                                    .clip(RoundedCornerShape(10.dp))
                                    .clickable { menuExpanded = true },
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = Icons.Default.MoreVert,
                                    contentDescription = "更多操作",
                                    tint = theme.textPrimary,
                                    modifier = Modifier.size(20.dp)
                                )
                            }

                            DropdownMenu(
                                expanded = menuExpanded,
                                onDismissRequest = { menuExpanded = false }
                            ) {
                                DropdownMenuItem(
                                    text = { Text("编辑标题与批注") },
                                    onClick = {
                                        menuExpanded = false
                                        editing = true
                                    }
                                )
                                DropdownMenuItem(
                                    text = { Text(if (item.url.isNotBlank()) "复制原文链接" else "复制内容") },
                                    onClick = {
                                        menuExpanded = false
                                        onCopyText(item.url.ifBlank { item.rawInput })
                                    }
                                )
                                DropdownMenuItem(
                                    text = { Text("重新 AI 提炼") },
                                    onClick = {
                                        menuExpanded = false
                                        onReprocess(item)
                                    }
                                )
                                DropdownMenuItem(
                                    text = { Text(if (item.isUnread) "标为已读" else "标为未读") },
                                    onClick = {
                                        menuExpanded = false
                                        onToggleRead(item)
                                    }
                                )
                                DropdownMenuItem(
                                    text = {
                                        Text(
                                            text = "删除收藏",
                                            color = if (theme.isLight) Color(0xFFBE123C) else Color(0xFFFB7185)
                                        )
                                    },
                                    onClick = {
                                        menuExpanded = false
                                        onDelete(item)
                                    }
                                )
                            }
                        }
                    }
                }
                HorizontalDivider(thickness = 0.5.dp, color = theme.divider)
            }
        }

        // Scrollable Reading Content
        Column(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 18.dp, vertical = 16.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            // Inline Edit Panel
            AnimatedVisibility(visible = editing) {
                SpecularGlassCard(
                    cornerRadius = 14.dp,
                    accentTint = theme.accent,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(14.dp),
                        verticalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        Text(
                            text = "编辑收藏信息",
                            fontSize = 13.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textPrimary
                        )
                        ObsidianGlassInput(
                            value = editTitle,
                            onValueChange = { editTitle = it },
                            placeholder = "标题",
                            singleLine = false,
                            maxLines = 3,
                            modifier = Modifier.fillMaxWidth()
                        )
                        ObsidianGlassInput(
                            value = editNote,
                            onValueChange = { editNote = it },
                            placeholder = "我的批注或思考...",
                            singleLine = false,
                            maxLines = 4,
                            modifier = Modifier.fillMaxWidth()
                        )
                        ObsidianGlassInput(
                            value = editTags,
                            onValueChange = { editTags = it },
                            placeholder = "标签（逗号分隔，如 AI,架构,Bilibili）",
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth()
                        )
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            TactileSecondaryButton(
                                text = "取消",
                                minHeight = 38.dp,
                                onClick = {
                                    editTitle = item.title
                                    editNote = item.userNote
                                    editTags = item.tags
                                    editing = false
                                },
                                modifier = Modifier.weight(1f)
                            )
                            TactilePrimaryButton(
                                text = "保存修改",
                                minHeight = 38.dp,
                                onClick = {
                                    onSaveEdit(item, editTitle, editNote, editTags)
                                    editing = false
                                },
                                modifier = Modifier.weight(1f)
                            )
                        }
                    }
                }
            }

            // Title & Meta
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text(
                    text = item.title.ifBlank { item.rawInput.take(96) },
                    fontSize = 20.sp,
                    lineHeight = 28.sp,
                    fontWeight = FontWeight.Bold,
                    color = theme.textPrimary
                )
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    Text(
                        text = if (item.createdAt.length >= 16) item.createdAt.substring(0, 16) else item.createdAt,
                        fontFamily = FontFamily.Monospace,
                        fontSize = 11.5.sp,
                        color = theme.textMuted
                    )
                    Text(
                        text = if (item.isUnread) "未读" else "已读",
                        fontSize = 11.5.sp,
                        color = if (item.isUnread) theme.accent else theme.textMuted
                    )
                }
            }

            // Clickable Source Link Strip
            if (item.url.isNotBlank()) {
                Surface(
                    shape = RoundedCornerShape(12.dp),
                    color = theme.cardTop,
                    border = BorderStroke(1.dp, theme.divider),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable { onOpenUrl(item) }
                            .padding(horizontal = 12.dp, vertical = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Icon(
                            imageVector = Icons.Default.OpenInNew,
                            contentDescription = "打开链接",
                            tint = theme.accent,
                            modifier = Modifier.size(16.dp)
                        )
                        Text(
                            text = item.url,
                            fontFamily = FontFamily.Monospace,
                            fontSize = 12.sp,
                            color = theme.accent,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f)
                        )
                        Box(
                            modifier = Modifier
                                .size(30.dp)
                                .clip(RoundedCornerShape(8.dp))
                                .clickable { onCopyText(item.url) },
                            contentAlignment = Alignment.Center
                        ) {
                            Icon(
                                imageVector = Icons.Default.ContentCopy,
                                contentDescription = "复制链接",
                                tint = theme.textSecondary,
                                modifier = Modifier.size(15.dp)
                            )
                        }
                    }
                }
            }

            // AI TLDR Highlight Card
            if (item.aiTldr.isNotBlank()) {
                Surface(
                    shape = RoundedCornerShape(12.dp),
                    color = theme.accent.copy(alpha = if (theme.isLight) 0.08f else 0.12f),
                    border = BorderStroke(1.dp, theme.accent.copy(alpha = 0.24f)),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                        verticalArrangement = Arrangement.spacedBy(5.dp)
                    ) {
                        Text(
                            text = "核心结论",
                            fontSize = 11.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = if (theme.isLight) theme.accentDeep else theme.accentLight
                        )
                        Text(
                            text = item.aiTldr,
                            fontSize = 14.5.sp,
                            lineHeight = 22.sp,
                            color = theme.textPrimary
                        )
                    }
                }
            } else if (item.status == "processing") {
                SpecularGlassCard(
                    cornerRadius = 12.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Text(
                        text = "后台正在抓取页面并提炼核心结论，稍后自动刷新...",
                        fontSize = 13.sp,
                        color = theme.textSecondary,
                        modifier = Modifier.padding(14.dp)
                    )
                }
            }

            // Status / Degradation Notice + Retry
            if (item.errorMsg.isNotBlank()) {
                val isHardFailure = item.status == "failed"
                Surface(
                    shape = RoundedCornerShape(10.dp),
                    color = if (isHardFailure) {
                        Color(0xFFE11D48).copy(alpha = if (theme.isLight) 0.08f else 0.14f)
                    } else {
                        theme.inputTop
                    },
                    border = BorderStroke(
                        1.dp,
                        if (isHardFailure) Color(0xFFE11D48).copy(alpha = 0.3f) else theme.divider
                    ),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 12.dp, vertical = 9.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Text(
                            text = item.errorMsg,
                            fontSize = 12.sp,
                            lineHeight = 17.sp,
                            color = if (isHardFailure) {
                                if (theme.isLight) Color(0xFFBE123C) else Color(0xFFFB7185)
                            } else {
                                theme.textMuted
                            },
                            modifier = Modifier.weight(1f)
                        )
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(3.dp),
                            modifier = Modifier
                                .clip(RoundedCornerShape(8.dp))
                                .clickable { onReprocess(item) }
                                .padding(horizontal = 6.dp, vertical = 4.dp)
                        ) {
                            Icon(
                                imageVector = Icons.Default.Refresh,
                                contentDescription = "重试",
                                tint = theme.accent,
                                modifier = Modifier.size(14.dp)
                            )
                            Text(
                                text = "重试",
                                fontSize = 12.sp,
                                fontWeight = FontWeight.Medium,
                                color = theme.accent
                            )
                        }
                    }
                }
            }

            // Key Points List
            if (item.points.isNotEmpty()) {
                SpecularGlassCard(
                    cornerRadius = 14.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 13.dp),
                        verticalArrangement = Arrangement.spacedBy(9.dp)
                    ) {
                        Text(
                            text = "核心干货要点",
                            fontSize = 12.5.sp,
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
                                        .width(16.dp)
                                        .padding(top = 8.dp),
                                    contentAlignment = Alignment.TopStart
                                ) {
                                    Box(
                                        modifier = Modifier
                                            .size(5.dp)
                                            .background(theme.accent, CircleShape)
                                    )
                                }
                                Text(
                                    text = pt,
                                    fontSize = 14.sp,
                                    lineHeight = 21.sp,
                                    color = theme.textPrimary,
                                    modifier = Modifier.weight(1f)
                                )
                            }
                        }
                    }
                }
            }

            // Tags
            if (tagList.isNotEmpty()) {
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    tagList.forEach { tag ->
                        Box(
                            modifier = Modifier
                                .clip(RoundedCornerShape(999.dp))
                                .background(theme.inputTop)
                                .border(0.5.dp, theme.divider, RoundedCornerShape(999.dp))
                                .padding(horizontal = 10.dp, vertical = 4.dp)
                        ) {
                            Text(
                                text = "#$tag",
                                fontSize = 11.5.sp,
                                color = theme.textSecondary
                            )
                        }
                    }
                }
            }

            // Personal Note Card (always editable with 1 tap)
            SpecularGlassCard(
                cornerRadius = 14.dp,
                onClick = { editing = true },
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "我的批注",
                            fontSize = 12.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textSecondary
                        )
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(3.dp)
                        ) {
                            Icon(
                                imageVector = Icons.Default.Edit,
                                contentDescription = "编辑批注",
                                tint = theme.accent,
                                modifier = Modifier.size(13.dp)
                            )
                            Text(
                                text = if (item.userNote.isBlank()) "添加" else "编辑",
                                fontSize = 12.sp,
                                color = theme.accent
                            )
                        }
                    }
                    Text(
                        text = item.userNote.ifBlank { "点击记录你对这条内容的想法、行动项或疑问..." },
                        fontSize = 13.5.sp,
                        lineHeight = 20.sp,
                        color = if (item.userNote.isBlank()) theme.textMuted else theme.textPrimary
                    )
                }
            }

            // Collapsible Full Excerpt / Captured Snapshot
            val readableExcerpt = item.contentExcerpt.ifBlank {
                if (item.kind == "note") item.rawInput else ""
            }
            if (readableExcerpt.isNotBlank()) {
                SpecularGlassCard(
                    cornerRadius = 14.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable { excerptExpanded = !excerptExpanded },
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = if (item.kind == "note") "完整正文" else "原文 / 字幕快照",
                                fontSize = 12.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textSecondary
                            )
                            Icon(
                                imageVector = if (excerptExpanded) Icons.Default.ExpandLess else Icons.Default.ExpandMore,
                                contentDescription = if (excerptExpanded) "收起快照" else "展开快照",
                                tint = theme.textMuted,
                                modifier = Modifier.size(18.dp)
                            )
                        }
                        if (excerptExpanded) {
                            Text(
                                text = readableExcerpt,
                                fontSize = 13.5.sp,
                                lineHeight = 21.sp,
                                color = theme.textSecondary
                            )
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(12.dp))
        }

        // Fixed Bottom Action Bar
        Surface(
            color = theme.sheetBg,
            tonalElevation = 0.dp,
            modifier = Modifier.fillMaxWidth()
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .navigationBarsPadding()
            ) {
                HorizontalDivider(thickness = 0.5.dp, color = theme.divider)
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 16.dp, vertical = 10.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        TactileSecondaryButton(
                            text = "转为待办",
                            accent = true,
                            cornerRadius = 10.dp,
                            minHeight = 36.dp,
                            contentPadding = PaddingValues(horizontal = 10.dp, vertical = 5.dp),
                            onClick = { onConvertToTodo(item) },
                            modifier = Modifier.weight(1f)
                        )
                        TactileSecondaryButton(
                            text = "存为笔记",
                            cornerRadius = 10.dp,
                            minHeight = 36.dp,
                            contentPadding = PaddingValues(horizontal = 10.dp, vertical = 5.dp),
                            onClick = { onConvertToNote(item) },
                            modifier = Modifier.weight(1f)
                        )
                    }
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        TactilePrimaryButton(
                            text = if (item.url.isNotBlank()) "打开原文" else "复制内容",
                            leadingIcon = if (item.url.isNotBlank()) Icons.Default.OpenInNew else Icons.Default.ContentCopy,
                            cornerRadius = 11.dp,
                            minHeight = 42.dp,
                            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                            onClick = {
                                if (item.url.isNotBlank()) {
                                    onOpenUrl(item)
                                } else {
                                    onCopyText(item.rawInput)
                                }
                            },
                            modifier = Modifier.weight(1.05f)
                        )
                        TactileSecondaryButton(
                            text = "发给 AI 聊",
                            cornerRadius = 11.dp,
                            minHeight = 42.dp,
                            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                            onClick = { onAskAi(item) },
                            modifier = Modifier.weight(1f)
                        )
                    }
                }
            }
        }
    }
}
