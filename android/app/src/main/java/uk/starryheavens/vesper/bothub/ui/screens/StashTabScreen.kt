package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material.icons.filled.ContentPaste
import androidx.compose.material.icons.filled.DeleteOutline
import androidx.compose.material.icons.filled.DoneAll
import androidx.compose.material.icons.filled.MarkEmailRead
import androidx.compose.material.icons.filled.MarkEmailUnread
import androidx.compose.material.icons.outlined.BookmarkBorder
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.SwipeToDismissBox
import androidx.compose.material3.SwipeToDismissBoxValue
import androidx.compose.material3.Text
import androidx.compose.material3.rememberSwipeToDismissBoxState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.ShareKind
import uk.starryheavens.vesper.bothub.data.StashItemUi
import uk.starryheavens.vesper.bothub.ui.components.VesperButtonTone
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperEmptyState
import uk.starryheavens.vesper.bothub.ui.components.VesperFilterChip
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSearchField
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.VesperTextField
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun StashTabScreen(
    items: List<StashItemUi>,
    query: String,
    activeFilter: String,
    activeKindFilter: String = "",
    loading: Boolean,
    showCaptureBar: Boolean = false,
    onDismissCaptureBar: () -> Unit = {},
    onQueryChange: (String) -> Unit,
    onFilterChange: (String) -> Unit,
    onKindFilterChange: (String) -> Unit = {},
    onMarkAllRead: () -> Unit = {},
    onCapture: (String, String) -> Unit,
    onOpenDetail: (StashItemUi) -> Unit,
    onToggleRead: (StashItemUi) -> Unit,
    onDelete: (StashItemUi) -> Unit,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val clipboard = LocalClipboardManager.current
    val haptics = LocalHapticFeedback.current
    var captureInput by remember { mutableStateOf("") }
    var captureNote by remember { mutableStateOf("") }

    val filters = listOf(
        "all" to "全部",
        "unread" to "未读",
        "pinned" to "置顶",
        "note" to "闪念"
    )

    val unreadCount = remember(items) { items.count { it.isUnread } }

    val availablePlatforms = remember(items) {
        val present = items.map { it.shareKind }.distinct().filter { it != ShareKind.PLAIN_NOTE }
        if (present.size >= 2) present else emptyList()
    }

    val visibleItems = remember(items, activeKindFilter) {
        if (activeKindFilter.isBlank()) {
            items
        } else {
            items.filter { it.kind.equals(activeKindFilter, ignoreCase = true) }
        }
    }

    Column(modifier = modifier.fillMaxSize()) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(start = 16.dp, end = 16.dp, top = 10.dp, bottom = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            VesperSearchField(
                value = query,
                onValueChange = onQueryChange,
                placeholder = "搜索收藏、摘要或标签",
                modifier = Modifier.weight(1f)
            )

            if (unreadCount > 0) {
                VesperSecondaryButton(
                    text = "全读",
                    leadingIcon = Icons.Default.DoneAll,
                    tone = VesperButtonTone.ACCENT,
                    cornerRadius = 12.dp,
                    minHeight = 44.dp,
                    contentPadding = PaddingValues(horizontal = 11.dp, vertical = 8.dp),
                    onClick = onMarkAllRead
                )
            }
        }

        AnimatedVisibility(
            visible = showCaptureBar,
            enter = fadeIn() + expandVertically(),
            exit = fadeOut() + shrinkVertically()
        ) {
            VesperCard(
                cornerRadius = 16.dp,
                accentTone = theme.accent,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 6.dp)
            ) {
                Column(
                    modifier = Modifier.padding(14.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "快速收纳链接或闪念",
                            fontSize = 13.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textPrimary
                        )
                        VesperSecondaryButton(
                            text = "粘贴剪贴板",
                            leadingIcon = Icons.Default.ContentPaste,
                            tone = VesperButtonTone.ACCENT,
                            cornerRadius = 999.dp,
                            minHeight = 34.dp,
                            contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                            onClick = {
                                val clip = clipboard.getText()?.text?.trim().orEmpty()
                                if (clip.isNotEmpty()) {
                                    captureInput = clip
                                }
                            }
                        )
                    }
                    VesperTextField(
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
                        VesperTextField(
                            value = captureNote,
                            onValueChange = { captureNote = it },
                            placeholder = "批注（可选）",
                            singleLine = true,
                            modifier = Modifier.weight(1f)
                        )
                        VesperPrimaryButton(
                            text = "保存",
                            enabled = captureInput.isNotBlank(),
                            minHeight = 48.dp,
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

        // Primary Status Filter + Dynamic Platform Filter Strip
        LazyRow(
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 6.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth()
        ) {
            items(filters) { (fid, label) ->
                val chipLabel = if (fid == "unread" && unreadCount > 0) "$label $unreadCount" else label
                VesperFilterChip(
                    label = chipLabel,
                    selected = activeFilter == fid && activeKindFilter.isBlank(),
                    onClick = {
                        if (activeKindFilter.isNotBlank()) onKindFilterChange("")
                        onFilterChange(fid)
                    }
                )
            }
            if (availablePlatforms.isNotEmpty()) {
                items(availablePlatforms, key = { it.wireKind }) { platform ->
                    VesperFilterChip(
                        label = platform.badgeLabel,
                        selected = activeKindFilter.equals(platform.wireKind, ignoreCase = true),
                        onClick = { onKindFilterChange(platform.wireKind) }
                    )
                }
            }
        }

        // High-Density Editorial Stash Stream
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 24.dp)
        ) {
            if (visibleItems.isEmpty()) {
                item(key = "__stash_empty__") {
                    VesperEmptyState(
                        icon = Icons.Outlined.BookmarkBorder,
                        title = when {
                            loading -> "正在同步收藏与闪念匣..."
                            query.isNotBlank() || activeKindFilter.isNotBlank() -> "当前筛选下暂无匹配收藏"
                            activeFilter == "unread" -> "所有收藏都已阅读完毕"
                            else -> "当前视图暂无收藏条目"
                        },
                        subtitle = if (!loading && query.isBlank() && activeKindFilter.isBlank() && activeFilter != "unread") {
                            "通过边缘悬浮球一键收链接与识屏，或复制链接切回即可自动提示收纳"
                        } else "",
                        actions = if (!loading && query.isBlank() && activeKindFilter.isBlank() && activeFilter != "unread") {
                            {
                                VesperSecondaryButton(
                                    text = "粘贴剪贴板秒收",
                                    leadingIcon = Icons.Default.ContentPaste,
                                    tone = VesperButtonTone.ACCENT,
                                    cornerRadius = 999.dp,
                                    minHeight = 44.dp,
                                    contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
                                    onClick = {
                                        val clip = clipboard.getText()?.text?.trim().orEmpty()
                                        if (clip.isNotEmpty()) {
                                            onCapture(clip, "")
                                        }
                                    }
                                )
                            }
                        } else null,
                        modifier = Modifier.padding(top = 16.dp)
                    )
                }
            }

            items(visibleItems, key = { it.id }, contentType = { "stash_card" }) { item ->
                val dismissState = rememberSwipeToDismissBoxState(
                    confirmValueChange = { value ->
                        when (value) {
                            SwipeToDismissBoxValue.StartToEnd -> {
                                haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                                onToggleRead(item)
                                false
                            }
                            SwipeToDismissBoxValue.EndToStart -> {
                                haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                                onDelete(item)
                                true
                            }
                            SwipeToDismissBoxValue.Settled -> false
                        }
                    }
                )

                SwipeToDismissBox(
                    state = dismissState,
                    backgroundContent = {
                        val direction = dismissState.dismissDirection
                        val isReadAction = direction == SwipeToDismissBoxValue.StartToEnd
                        val isDeleteAction = direction == SwipeToDismissBoxValue.EndToStart
                        val bgColor = when {
                            isReadAction -> if (item.isUnread) theme.success else theme.accent
                            isDeleteAction -> theme.danger
                            else -> Color.Transparent
                        }
                        Box(
                            modifier = Modifier
                                .fillMaxSize()
                                .clip(RoundedCornerShape(16.dp))
                                .background(bgColor.copy(alpha = if (isReadAction || isDeleteAction) 0.16f else 0f))
                                .padding(horizontal = 18.dp),
                            contentAlignment = if (isReadAction) Alignment.CenterStart else Alignment.CenterEnd
                        ) {
                            if (isReadAction) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                                ) {
                                    Icon(
                                        imageVector = if (item.isUnread) Icons.Default.MarkEmailRead else Icons.Default.MarkEmailUnread,
                                        contentDescription = null,
                                        tint = bgColor,
                                        modifier = Modifier.size(18.dp)
                                    )
                                    Text(
                                        text = if (item.isUnread) "标为已读" else "标为未读",
                                        fontSize = 12.5.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        color = bgColor
                                    )
                                }
                            } else if (isDeleteAction) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                                ) {
                                    Text(
                                        text = "删除",
                                        fontSize = 12.5.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        color = bgColor
                                    )
                                    Icon(
                                        imageVector = Icons.Default.DeleteOutline,
                                        contentDescription = null,
                                        tint = bgColor,
                                        modifier = Modifier.size(18.dp)
                                    )
                                }
                            }
                        }
                    }
                ) {
                    EditorialStashCard(
                        item = item,
                        onClick = { onOpenDetail(item) }
                    )
                }
            }
        }
    }
}

@Composable
private fun EditorialStashCard(
    item: StashItemUi,
    onClick: () -> Unit
) {
    val theme = BotHubThemeTokens
    val sourceBadge = item.shareKind.badgeLabel
    val tagList = remember(item.tags) {
        item.tags.split(',', '，').map { it.trim() }.filter { it.isNotEmpty() }.take(3)
    }

    VesperCard(
        cornerRadius = 16.dp,
        elevated = item.isUnread,
        accentTone = when {
            item.pinned -> theme.warning
            item.isUnread -> theme.accent
            else -> null
        },
        onClick = onClick,
        modifier = Modifier.fillMaxWidth()
    ) {
        Column(
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 13.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp)
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
                                .size(6.5.dp)
                                .background(theme.accent, CircleShape)
                        )
                    }
                    VesperTag(
                        text = sourceBadge,
                        bgColor = if (item.isUnread) theme.accentSoft else theme.surfaceInput,
                        borderColor = if (item.isUnread) theme.accentMuted else theme.border,
                        textColor = if (item.isUnread) theme.accentForeground else theme.textSecondary
                    )
                    if (item.pinned) {
                        Icon(
                            imageVector = Icons.Default.Bookmark,
                            contentDescription = "已置顶",
                            tint = theme.warning,
                            modifier = Modifier.size(13.dp)
                        )
                    }
                    if (item.status == "processing") {
                        VesperTag(text = "AI 提炼中", toneColor = theme.success)
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
                    fontSize = 10.5.sp,
                    color = theme.textMuted,
                    maxLines = 1,
                    softWrap = false
                )
            }

            Text(
                text = item.title.ifEmpty { item.rawInput.take(72) },
                fontSize = 15.5.sp,
                lineHeight = 22.sp,
                fontWeight = if (item.isUnread) FontWeight.SemiBold else FontWeight.Medium,
                color = if (item.isUnread) theme.textPrimary else theme.textPrimary.copy(alpha = 0.86f),
                maxLines = 2,
                overflow = TextOverflow.Ellipsis
            )

            when {
                item.aiTldr.isNotEmpty() -> {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(IntrinsicSize.Min)
                            .clip(RoundedCornerShape(10.dp))
                            .background(theme.codeBlockBg)
                            .padding(end = 10.dp, top = 7.dp, bottom = 7.dp)
                    ) {
                        Box(
                            modifier = Modifier
                                .width(2.5.dp)
                                .fillMaxHeight()
                                .background(theme.accent, RoundedCornerShape(999.dp))
                        )
                        Column(
                            modifier = Modifier
                                .weight(1f)
                                .padding(start = 9.dp),
                            verticalArrangement = Arrangement.spacedBy(2.dp)
                        ) {
                            Text(
                                text = "AI 摘要",
                                fontSize = 10.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.accentForeground
                            )
                            Text(
                                text = item.aiTldr,
                                fontSize = 12.5.sp,
                                lineHeight = 18.5.sp,
                                color = theme.textSecondary,
                                maxLines = 2,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                    }
                }
                item.status == "processing" -> {
                    Text(
                        text = "后台正在抓取页面并提炼要点...",
                        fontSize = 12.5.sp,
                        color = theme.textMuted,
                        maxLines = 1
                    )
                }
                item.errorMsg.isNotEmpty() -> {
                    Text(
                        text = item.errorMsg,
                        fontSize = 12.sp,
                        color = if (item.status == "failed") theme.danger else theme.textMuted,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }

            if (tagList.isNotEmpty() || item.userNote.isNotEmpty()) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    tagList.forEach { tag ->
                        Box(
                            modifier = Modifier
                                .clip(RoundedCornerShape(999.dp))
                                .background(theme.surfaceInput)
                                .border(0.6.dp, theme.border, RoundedCornerShape(999.dp))
                                .padding(horizontal = 8.dp, vertical = 2.5.dp)
                        ) {
                            Text(
                                text = "#$tag",
                                fontSize = 10.5.sp,
                                color = theme.textMuted,
                                maxLines = 1
                            )
                        }
                    }
                    if (item.userNote.isNotEmpty()) {
                        Text(
                            text = "· 含批注",
                            fontSize = 10.5.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.accentForeground,
                            maxLines = 1
                        )
                    }
                }
            }
        }
    }
}
