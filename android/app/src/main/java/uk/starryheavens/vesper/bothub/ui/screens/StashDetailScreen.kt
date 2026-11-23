package uk.starryheavens.vesper.bothub.ui.screens

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateContentSize
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.OpenInNew
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material.icons.filled.BookmarkBorder
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.DeleteOutline
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material.icons.filled.ExpandLess
import androidx.compose.material.icons.filled.ExpandMore
import androidx.compose.material.icons.filled.MarkEmailRead
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.Refresh
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
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.StashItemUi
import uk.starryheavens.vesper.bothub.ui.components.VesperButtonTone
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperDivider
import uk.starryheavens.vesper.bothub.ui.components.VesperIconButton
import uk.starryheavens.vesper.bothub.ui.components.VesperMenuAction
import uk.starryheavens.vesper.bothub.ui.components.VesperPopoverMenu
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.VesperTextField
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
            .background(theme.bgCanvas)
            .imePadding()
    ) {
        // Top Navigation Bar
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(theme.surfaceBar)
                .statusBarsPadding()
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 4.dp, vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    modifier = Modifier.weight(1f)
                ) {
                    VesperIconButton(
                        icon = Icons.AutoMirrored.Filled.ArrowBack,
                        contentDescription = "返回收藏列表",
                        filled = false,
                        iconSize = 21.dp,
                        onClick = onBack
                    )

                    VesperTag(text = item.shareKind.badgeLabel)

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

                Row(verticalAlignment = Alignment.CenterVertically) {
                    VesperIconButton(
                        icon = if (item.pinned) Icons.Default.Bookmark else Icons.Default.BookmarkBorder,
                        contentDescription = if (item.pinned) "取消置顶" else "置顶",
                        accent = item.pinned,
                        filled = false,
                        iconSize = 20.dp,
                        onClick = { onTogglePin(item) }
                    )

                    Box {
                        VesperIconButton(
                            icon = Icons.Default.MoreVert,
                            contentDescription = "更多操作",
                            filled = false,
                            iconSize = 20.dp,
                            onClick = { menuExpanded = true }
                        )

                        VesperPopoverMenu(
                            expanded = menuExpanded,
                            onDismissRequest = { menuExpanded = false },
                            offsetY = 48.dp,
                            items = listOf(
                                VesperMenuAction(
                                    label = "编辑标题与批注",
                                    icon = Icons.Default.Edit,
                                    iconTint = theme.accentForeground,
                                    onClick = { editing = true }
                                ),
                                VesperMenuAction(
                                    label = if (item.url.isNotBlank()) "复制原文链接" else "复制内容",
                                    icon = Icons.Default.ContentCopy,
                                    iconTint = theme.info,
                                    onClick = { onCopyText(item.url.ifBlank { item.rawInput }) }
                                ),
                                VesperMenuAction(
                                    label = "重新 AI 提炼",
                                    icon = Icons.Default.Refresh,
                                    iconTint = theme.success,
                                    onClick = { onReprocess(item) }
                                ),
                                VesperMenuAction(
                                    label = if (item.isUnread) "标为已读" else "标为未读",
                                    icon = Icons.Default.MarkEmailRead,
                                    iconTint = theme.textSecondary,
                                    onClick = { onToggleRead(item) }
                                ),
                                VesperMenuAction(
                                    label = "删除收藏",
                                    icon = Icons.Default.DeleteOutline,
                                    danger = true,
                                    onClick = { onDelete(item) }
                                )
                            )
                        )
                    }
                }
            }
            VesperDivider()
        }

        // Scrollable Editorial Reading Content
        Column(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 18.dp, vertical = 18.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            AnimatedVisibility(visible = editing) {
                VesperCard(
                    cornerRadius = 16.dp,
                    accentTone = theme.accent,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(14.dp),
                        verticalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        Text(
                            text = "编辑收藏信息",
                            fontSize = 14.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textPrimary
                        )
                        VesperTextField(
                            value = editTitle,
                            onValueChange = { editTitle = it },
                            placeholder = "标题",
                            singleLine = false,
                            maxLines = 3,
                            modifier = Modifier.fillMaxWidth()
                        )
                        VesperTextField(
                            value = editNote,
                            onValueChange = { editNote = it },
                            placeholder = "我的批注或思考...",
                            singleLine = false,
                            maxLines = 4,
                            modifier = Modifier.fillMaxWidth()
                        )
                        VesperTextField(
                            value = editTags,
                            onValueChange = { editTags = it },
                            placeholder = "标签（逗号分隔，如 AI,架构,Bilibili）",
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth()
                        )
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.spacedBy(10.dp)
                        ) {
                            VesperSecondaryButton(
                                text = "取消",
                                minHeight = 44.dp,
                                onClick = {
                                    editTitle = item.title
                                    editNote = item.userNote
                                    editTags = item.tags
                                    editing = false
                                },
                                modifier = Modifier.weight(1f)
                            )
                            VesperPrimaryButton(
                                text = "保存修改",
                                minHeight = 44.dp,
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
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(
                    text = item.title.ifBlank { item.rawInput.take(96) },
                    fontSize = 21.sp,
                    lineHeight = 29.sp,
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
                    VesperTag(
                        text = if (item.isUnread) "未读" else "已读",
                        bgColor = if (item.isUnread) theme.accentSoft else theme.surfaceInput,
                        borderColor = if (item.isUnread) theme.accentMuted else theme.border,
                        textColor = if (item.isUnread) theme.accentForeground else theme.textMuted
                    )
                }
            }

            // Source Link Strip
            if (item.url.isNotBlank()) {
                VesperCard(
                    cornerRadius = 14.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 50.dp)
                            .clickable { onOpenUrl(item) }
                            .padding(start = 13.dp, end = 4.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Icon(
                            imageVector = Icons.AutoMirrored.Filled.OpenInNew,
                            contentDescription = "打开链接",
                            tint = theme.accentForeground,
                            modifier = Modifier.size(16.dp)
                        )
                        Text(
                            text = item.url,
                            fontFamily = FontFamily.Monospace,
                            fontSize = 12.sp,
                            color = theme.accentForeground,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f)
                        )
                        VesperIconButton(
                            icon = Icons.Default.ContentCopy,
                            contentDescription = "复制链接",
                            filled = false,
                            iconSize = 16.dp,
                            onClick = { onCopyText(item.url) }
                        )
                    }
                }
            }

            // AI TL;DR Insight Callout
            if (item.aiTldr.isNotBlank()) {
                Surface(
                    shape = RoundedCornerShape(14.dp),
                    color = theme.accentSoft,
                    border = BorderStroke(1.dp, theme.accentMuted),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(IntrinsicSize.Min)
                            .padding(horizontal = 14.dp, vertical = 13.dp),
                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        Box(
                            modifier = Modifier
                                .width(3.dp)
                                .fillMaxHeight()
                                .background(theme.accent, RoundedCornerShape(999.dp))
                        )
                        Column(
                            modifier = Modifier.weight(1f),
                            verticalArrangement = Arrangement.spacedBy(5.dp)
                        ) {
                            Text(
                                text = "AI 核心结论",
                                fontSize = 11.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.accentForeground
                            )
                            Text(
                                text = item.aiTldr,
                                fontSize = 15.sp,
                                lineHeight = 23.sp,
                                color = theme.textPrimary
                            )
                        }
                    }
                }
            } else if (item.status == "processing") {
                VesperCard(
                    cornerRadius = 14.dp,
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
                    shape = RoundedCornerShape(12.dp),
                    color = if (isHardFailure) theme.danger.copy(alpha = 0.12f) else theme.surfaceInput,
                    border = BorderStroke(
                        1.dp,
                        if (isHardFailure) theme.danger.copy(alpha = 0.34f) else theme.border
                    ),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(start = 13.dp, end = 6.dp, top = 6.dp, bottom = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Text(
                            text = item.errorMsg,
                            fontSize = 12.sp,
                            lineHeight = 17.sp,
                            color = if (isHardFailure) theme.danger else theme.textMuted,
                            modifier = Modifier.weight(1f)
                        )
                        VesperSecondaryButton(
                            text = "重试",
                            leadingIcon = Icons.Default.Refresh,
                            tone = VesperButtonTone.ACCENT,
                            cornerRadius = 9.dp,
                            minHeight = 36.dp,
                            contentPadding = PaddingValues(horizontal = 10.dp, vertical = 5.dp),
                            onClick = { onReprocess(item) }
                        )
                    }
                }
            }

            // Key Points List (Structured Numbered Insights)
            if (item.points.isNotEmpty()) {
                VesperCard(
                    cornerRadius = 16.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(horizontal = 15.dp, vertical = 14.dp),
                        verticalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        Text(
                            text = "核心干货要点",
                            fontSize = 12.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.success
                        )
                        item.points.forEachIndexed { idx, pt ->
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                verticalAlignment = Alignment.Top,
                                horizontalArrangement = Arrangement.spacedBy(9.dp)
                            ) {
                                Text(
                                    text = String.format("%02d", idx + 1),
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 11.5.sp,
                                    fontWeight = FontWeight.Bold,
                                    color = theme.accentForeground,
                                    modifier = Modifier.padding(top = 3.dp)
                                )
                                Text(
                                    text = pt,
                                    fontSize = 14.sp,
                                    lineHeight = 22.sp,
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
                                .background(theme.surfaceInput)
                                .border(0.75.dp, theme.border, RoundedCornerShape(999.dp))
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

            // Personal Note Card
            VesperCard(
                cornerRadius = 16.dp,
                onClick = { editing = true },
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
                        Text(
                            text = "我的批注",
                            fontSize = 12.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textSecondary
                        )
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(4.dp)
                        ) {
                            Icon(
                                imageVector = Icons.Default.Edit,
                                contentDescription = "编辑批注",
                                tint = theme.accentForeground,
                                modifier = Modifier.size(13.dp)
                            )
                            Text(
                                text = if (item.userNote.isBlank()) "添加" else "编辑",
                                fontSize = 12.sp,
                                fontWeight = FontWeight.Medium,
                                color = theme.accentForeground
                            )
                        }
                    }
                    Text(
                        text = item.userNote.ifBlank { "点击记录你对这条内容的想法、行动项或疑问..." },
                        fontSize = 13.5.sp,
                        lineHeight = 21.sp,
                        color = if (item.userNote.isBlank()) theme.textMuted else theme.textPrimary
                    )
                }
            }

            // Collapsible Full Excerpt / Captured Snapshot
            val readableExcerpt = item.contentExcerpt.ifBlank {
                if (item.kind == "note") item.rawInput else ""
            }
            if (readableExcerpt.isNotBlank()) {
                VesperCard(
                    cornerRadius = 16.dp,
                    modifier = Modifier
                        .fillMaxWidth()
                        .animateContentSize()
                ) {
                    Column(modifier = Modifier.fillMaxWidth()) {
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .sizeIn(minHeight = 48.dp)
                                .clickable { excerptExpanded = !excerptExpanded }
                                .padding(horizontal = 15.dp, vertical = 12.dp),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = if (item.kind == "note") "完整正文" else "原文 / 字幕快照",
                                fontSize = 13.sp,
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
                            VesperDivider(startIndent = 15.dp, endIndent = 15.dp)
                            Text(
                                text = readableExcerpt,
                                fontSize = 14.sp,
                                lineHeight = 22.5.sp,
                                color = theme.textSecondary,
                                modifier = Modifier.padding(horizontal = 15.dp, vertical = 13.dp)
                            )
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(12.dp))
        }

        // Fixed Bottom Action Dock
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(theme.surfaceBar)
                .navigationBarsPadding()
        ) {
            VesperDivider()
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
                    VesperSecondaryButton(
                        text = "转为待办",
                        tone = VesperButtonTone.ACCENT,
                        cornerRadius = 11.dp,
                        minHeight = 42.dp,
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 7.dp),
                        onClick = { onConvertToTodo(item) },
                        modifier = Modifier.weight(1f)
                    )
                    VesperSecondaryButton(
                        text = "存为笔记",
                        cornerRadius = 11.dp,
                        minHeight = 42.dp,
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 7.dp),
                        onClick = { onConvertToNote(item) },
                        modifier = Modifier.weight(1f)
                    )
                }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    VesperPrimaryButton(
                        text = if (item.url.isNotBlank()) "打开原文" else "复制内容",
                        leadingIcon = if (item.url.isNotBlank()) Icons.AutoMirrored.Filled.OpenInNew else Icons.Default.ContentCopy,
                        cornerRadius = 12.dp,
                        minHeight = 48.dp,
                        contentPadding = PaddingValues(horizontal = 12.dp, vertical = 8.dp),
                        onClick = {
                            if (item.url.isNotBlank()) {
                                onOpenUrl(item)
                            } else {
                                onCopyText(item.rawInput)
                            }
                        },
                        modifier = Modifier.weight(1.05f)
                    )
                    VesperSecondaryButton(
                        text = "发给 AI 聊",
                        cornerRadius = 12.dp,
                        minHeight = 48.dp,
                        contentPadding = PaddingValues(horizontal = 12.dp, vertical = 8.dp),
                        onClick = { onAskAi(item) },
                        modifier = Modifier.weight(1f)
                    )
                }
            }
        }
    }
}
