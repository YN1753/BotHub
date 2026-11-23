package uk.starryheavens.vesper.bothub.ui.screens.hub

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.CalendarMonth
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material.icons.outlined.TaskAlt
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.TodoItemUi
import uk.starryheavens.vesper.bothub.data.WorkNoteItemUi
import uk.starryheavens.vesper.bothub.ui.components.ThemeCalendarDialog
import uk.starryheavens.vesper.bothub.ui.components.VesperButtonTone
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperEmptyState
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSectionHeader
import uk.starryheavens.vesper.bothub.ui.components.VesperSegmentedTabs
import uk.starryheavens.vesper.bothub.ui.components.VesperSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.VesperSwitch
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.VesperTextField
import uk.starryheavens.vesper.bothub.ui.components.vesperPressable
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens
import java.time.LocalDate
import java.util.Locale

internal fun LazyListScope.hubWorkspaceItems(
    openTodos: List<TodoItemUi>,
    doneTodos: List<TodoItemUi>,
    sortedNotes: List<WorkNoteItemUi>,
    showDoneTodos: Boolean,
    onToggleShowDoneTodos: () -> Unit,
    onCreateTodo: () -> Unit,
    onEditTodo: (TodoItemUi) -> Unit,
    onToggleTodoDone: (TodoItemUi) -> Unit,
    onClearDoneTodos: () -> Unit,
    onCreateNote: () -> Unit,
    onEditNote: (WorkNoteItemUi) -> Unit
) {
    // Section 1: Open Todos
    item(key = "__hub_todos_hdr__") {
        VesperSectionHeader(
            title = "待办清单",
            count = openTodos.size,
            trailing = {
                VesperSecondaryButton(
                    text = "新建待办",
                    leadingIcon = Icons.Default.Add,
                    tone = VesperButtonTone.ACCENT,
                    cornerRadius = 999.dp,
                    minHeight = 36.dp,
                    contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                    onClick = onCreateTodo
                )
            }
        )
    }

    if (openTodos.isEmpty() && doneTodos.isEmpty()) {
        item(key = "__hub_todos_empty__") {
            VesperEmptyState(
                icon = Icons.Outlined.TaskAlt,
                title = "暂无待办事项",
                subtitle = "点击右上角「新建待办」或在聊天中输入 /todo 让 AI 助理自动记账"
            )
        }
    } else if (openTodos.isEmpty()) {
        item(key = "__hub_todos_all_done__") {
            val theme = BotHubThemeTokens
            VesperCard(cornerRadius = 14.dp, modifier = Modifier.fillMaxWidth()) {
                Text(
                    text = "当前进行中的待办已全部完成",
                    fontSize = 13.sp,
                    color = theme.textMuted,
                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp)
                )
            }
        }
    }

    items(openTodos, key = { "open_${it.id}" }, contentType = { "todo_open" }) { todo ->
        val theme = BotHubThemeTokens
        val isHigh = todo.priority.equals("high", ignoreCase = true)
        VesperCard(
            cornerRadius = 15.dp,
            onClick = { onEditTodo(todo) },
            modifier = Modifier.fillMaxWidth()
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .sizeIn(minHeight = 58.dp)
                    .padding(start = 6.dp, end = 13.dp, top = 6.dp, bottom = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                Box(
                    modifier = Modifier
                        .size(44.dp)
                        .vesperPressable(pressedScale = 0.9f, hapticOnTap = true) {
                            onToggleTodoDone(todo)
                        },
                    contentAlignment = Alignment.Center
                ) {
                    Box(
                        modifier = Modifier
                            .size(21.dp)
                            .clip(RoundedCornerShape(6.dp))
                            .background(theme.surfaceInput)
                            .border(1.4.dp, if (isHigh) theme.danger else theme.accent, RoundedCornerShape(6.dp))
                    )
                }

                Column(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(3.dp)
                ) {
                    Text(
                        text = todo.title,
                        fontSize = 14.5.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = theme.textPrimary,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis
                    )
                    if (todo.notes.isNotBlank() || todo.dueAt.isNotBlank()) {
                        Text(
                            text = listOfNotNull(
                                todo.dueAt.takeIf { it.isNotBlank() }?.let { "截止 ${it.take(16)}" },
                                todo.notes.takeIf { it.isNotBlank() }
                            ).joinToString(" · "),
                            fontSize = 11.5.sp,
                            color = theme.textMuted,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }

                if (isHigh) {
                    VesperTag(text = "高优", toneColor = theme.danger)
                }
            }
        }
    }

    if (doneTodos.isNotEmpty()) {
        item(key = "__hub_done_todos_toggle__") {
            val theme = BotHubThemeTokens
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Row(
                    modifier = Modifier
                        .sizeIn(minHeight = 40.dp)
                        .clip(RoundedCornerShape(8.dp))
                        .clickable(onClick = onToggleShowDoneTodos)
                        .padding(horizontal = 6.dp, vertical = 6.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    Icon(
                        imageVector = if (showDoneTodos) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown,
                        contentDescription = null,
                        tint = theme.textMuted,
                        modifier = Modifier.size(16.dp)
                    )
                    Text(
                        text = if (showDoneTodos) "收起已完成 (${doneTodos.size})" else "展开已完成 (${doneTodos.size})",
                        fontSize = 12.sp,
                        color = theme.textMuted
                    )
                }
                Box(
                    modifier = Modifier
                        .sizeIn(minHeight = 40.dp)
                        .clip(RoundedCornerShape(8.dp))
                        .clickable(onClick = onClearDoneTodos)
                        .padding(horizontal = 10.dp, vertical = 6.dp),
                    contentAlignment = Alignment.Center
                ) {
                    Text(
                        text = "清理已完成",
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Medium,
                        color = theme.textSecondary
                    )
                }
            }
        }

        if (showDoneTodos) {
            items(doneTodos, key = { "done_${it.id}" }, contentType = { "todo_done" }) { todo ->
                val theme = BotHubThemeTokens
                VesperCard(
                    cornerRadius = 14.dp,
                    onClick = { onEditTodo(todo) },
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 52.dp)
                            .padding(start = 6.dp, end = 13.dp, top = 4.dp, bottom = 4.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(4.dp)
                    ) {
                        Box(
                            modifier = Modifier
                                .size(44.dp)
                                .vesperPressable(pressedScale = 0.9f, hapticOnTap = true) {
                                    onToggleTodoDone(todo)
                                },
                            contentAlignment = Alignment.Center
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(21.dp)
                                    .clip(RoundedCornerShape(6.dp))
                                    .background(theme.success)
                                    .border(1.2.dp, theme.success, RoundedCornerShape(6.dp)),
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = Icons.Default.Check,
                                    contentDescription = "已完成",
                                    tint = Color.White,
                                    modifier = Modifier.size(13.dp)
                                )
                            }
                        }
                        Text(
                            text = todo.title,
                            fontSize = 13.5.sp,
                            color = theme.textMuted,
                            textDecoration = TextDecoration.LineThrough,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f)
                        )
                    }
                }
            }
        }
    }

    // Section 2: Work Notes
    item(key = "__hub_notes_hdr__") {
        VesperSectionHeader(
            title = "工作手记",
            count = sortedNotes.size,
            trailing = {
                VesperSecondaryButton(
                    text = "新建手记",
                    leadingIcon = Icons.Default.Add,
                    cornerRadius = 999.dp,
                    minHeight = 36.dp,
                    contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                    onClick = onCreateNote
                )
            },
            modifier = Modifier.padding(top = 8.dp)
        )
    }

    if (sortedNotes.isEmpty()) {
        item(key = "__hub_notes_empty__") {
            VesperEmptyState(
                icon = Icons.Outlined.Description,
                title = "暂无工作手记",
                subtitle = "点击「新建手记」或让 AI 助理整理产出 Markdown 文档，双端实时同步"
            )
        }
    }

    items(sortedNotes, key = { "note_${it.id}" }, contentType = { "note_card" }) { note ->
        val theme = BotHubThemeTokens
        val catLabel = when (note.category.lowercase(Locale.ROOT)) {
            "sop" -> "运维手册"
            "memo" -> "随手备忘"
            else -> "工作产出"
        }
        VesperCard(
            cornerRadius = 15.dp,
            onClick = { onEditNote(note) },
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
                    Text(
                        text = note.title,
                        fontSize = 14.5.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = theme.textPrimary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier
                            .weight(1f)
                            .padding(end = 8.dp)
                    )
                    Row(horizontalArrangement = Arrangement.spacedBy(5.dp)) {
                        if (note.pinned) {
                            VesperTag(text = "置顶")
                        }
                        VesperTag(
                            text = catLabel,
                            bgColor = theme.surfaceInput,
                            borderColor = theme.border,
                            textColor = theme.textSecondary
                        )
                    }
                }
                Text(
                    text = note.previewText(110).ifEmpty { "点击查看或编辑正文" },
                    fontSize = 12.5.sp,
                    lineHeight = 18.sp,
                    color = theme.textSecondary,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun TodoEditorSheet(
    todo: TodoItemUi?,
    onSave: (title: String, notes: String, dueAt: String, priority: String) -> Unit,
    onDelete: (() -> Unit)?,
    onDismiss: () -> Unit
) {
    val theme = BotHubThemeTokens
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var title by remember(todo) { mutableStateOf(todo?.title.orEmpty()) }
    var notes by remember(todo) { mutableStateOf(todo?.notes.orEmpty()) }
    var dueAt by remember(todo) { mutableStateOf(todo?.dueAt?.take(10).orEmpty()) }
    var priority by remember(todo) { mutableStateOf(todo?.priority?.ifBlank { "normal" } ?: "normal") }
    var showDatePicker by remember { mutableStateOf(false) }

    val priorities = listOf("normal" to "普通", "high" to "高优紧急", "low" to "低优")

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 24.dp, topEnd = 24.dp),
        containerColor = theme.surfaceSheet,
        scrimColor = theme.scrim,
        tonalElevation = 0.dp,
        dragHandle = { VesperSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .imePadding()
                .navigationBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(13.dp)
        ) {
            Text(
                text = if (todo == null) "新建待办事项" else "编辑待办事项",
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )

            VesperTextField(
                value = title,
                onValueChange = { title = it },
                label = "待办标题",
                placeholder = "例如：续费香港服务器并核对备份",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text("优先级", fontSize = 12.sp, color = theme.textMuted)
                VesperSegmentedTabs(
                    options = priorities.map { it.second },
                    selectedIndex = priorities.indexOfFirst { it.first == priority }.coerceAtLeast(0),
                    onSelect = { idx -> priority = priorities[idx].first }
                )
            }

            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text("截止日期（可选）", fontSize = 12.sp, color = theme.textMuted)
                    if (dueAt.isNotBlank()) {
                        Text(
                            text = "清除日期",
                            fontSize = 12.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.accentForeground,
                            modifier = Modifier
                                .clickable { dueAt = "" }
                                .padding(horizontal = 4.dp, vertical = 2.dp)
                        )
                    }
                }
                Surface(
                    onClick = { showDatePicker = true },
                    shape = RoundedCornerShape(12.dp),
                    color = theme.surfaceInput,
                    border = BorderStroke(1.dp, theme.border),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier
                            .sizeIn(minHeight = 48.dp)
                            .padding(horizontal = 14.dp, vertical = 12.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = dueAt.ifBlank { "点击选择截止日期" },
                            fontFamily = if (dueAt.isNotBlank()) FontFamily.Monospace else FontFamily.Default,
                            fontSize = 14.sp,
                            color = if (dueAt.isBlank()) theme.textMuted else theme.textPrimary
                        )
                        Icon(
                            imageVector = Icons.Default.CalendarMonth,
                            contentDescription = "选择截止日期",
                            tint = theme.accentForeground,
                            modifier = Modifier.size(18.dp)
                        )
                    }
                }
            }

            VesperTextField(
                value = notes,
                onValueChange = { notes = it },
                label = "补充备注（可选）",
                placeholder = "补充说明或上下文细节",
                singleLine = false,
                modifier = Modifier.fillMaxWidth()
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                if (onDelete != null) {
                    VesperSecondaryButton(
                        text = "删除",
                        tone = VesperButtonTone.DANGER,
                        minHeight = 48.dp,
                        onClick = onDelete,
                        modifier = Modifier.weight(1f)
                    )
                }
                VesperPrimaryButton(
                    text = "保存待办",
                    enabled = title.isNotBlank(),
                    minHeight = 48.dp,
                    onClick = {
                        onSave(title.trim(), notes.trim(), dueAt.trim(), priority)
                    },
                    modifier = Modifier.weight(if (onDelete != null) 1.5f else 1f)
                )
            }

            Spacer(modifier = Modifier.height(8.dp))
        }
    }

    if (showDatePicker) {
        ThemeCalendarDialog(
            initialDate = dueAt.ifBlank { LocalDate.now().toString() },
            allowPerpetual = false,
            onSelectDate = { picked ->
                dueAt = picked
                showDatePicker = false
            },
            onSelectPerpetual = {},
            onDismiss = { showDatePicker = false }
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun NoteEditorSheet(
    note: WorkNoteItemUi?,
    onSave: (title: String, content: String, category: String, pinned: Boolean) -> Unit,
    onDelete: (() -> Unit)?,
    onDismiss: () -> Unit
) {
    val theme = BotHubThemeTokens
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var title by remember(note) { mutableStateOf(note?.title.orEmpty()) }
    var content by remember(note) { mutableStateOf(note?.content.orEmpty()) }
    var category by remember(note) { mutableStateOf(note?.category?.ifBlank { "deliverable" } ?: "deliverable") }
    var pinned by remember(note) { mutableStateOf(note?.pinned ?: false) }

    val categories = listOf(
        "deliverable" to "工作产出",
        "sop" to "运维手册",
        "memo" to "随手备忘"
    )

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 24.dp, topEnd = 24.dp),
        containerColor = theme.surfaceSheet,
        scrimColor = theme.scrim,
        tonalElevation = 0.dp,
        dragHandle = { VesperSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .imePadding()
                .navigationBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(13.dp)
        ) {
            Text(
                text = if (note == null) "新建工作手记" else "编辑工作手记",
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )

            VesperTextField(
                value = title,
                onValueChange = { title = it },
                label = "手记标题",
                placeholder = "例如：服务器架构与续期复盘",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text("文档分类", fontSize = 12.sp, color = theme.textMuted)
                VesperSegmentedTabs(
                    options = categories.map { it.second },
                    selectedIndex = categories.indexOfFirst { it.first == category }.coerceAtLeast(0),
                    onSelect = { idx -> category = categories[idx].first }
                )
            }

            VesperTextField(
                value = content,
                onValueChange = { content = it },
                label = "Markdown 正文内容",
                placeholder = "支持 Markdown 格式排版，双端实时同步...",
                singleLine = false,
                maxLines = 6,
                modifier = Modifier.fillMaxWidth()
            )

            VesperCard(
                cornerRadius = 15.dp,
                onClick = { pinned = !pinned },
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 54.dp)
                        .padding(start = 14.dp, end = 6.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text("置顶此篇工作手记", fontSize = 14.5.sp, color = theme.textPrimary)
                    VesperSwitch(
                        checked = pinned,
                        onCheckedChange = { pinned = it }
                    )
                }
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                if (onDelete != null) {
                    VesperSecondaryButton(
                        text = "删除",
                        tone = VesperButtonTone.DANGER,
                        minHeight = 48.dp,
                        onClick = onDelete,
                        modifier = Modifier.weight(1f)
                    )
                }
                VesperPrimaryButton(
                    text = "保存手记",
                    enabled = title.isNotBlank(),
                    minHeight = 48.dp,
                    onClick = {
                        onSave(title.trim(), content.trim(), category, pinned)
                    },
                    modifier = Modifier.weight(if (onDelete != null) 1.5f else 1f)
                )
            }

            Spacer(modifier = Modifier.height(8.dp))
        }
    }
}
