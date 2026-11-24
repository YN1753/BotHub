package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
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
import uk.starryheavens.vesper.bothub.data.AssetCategoryItem
import uk.starryheavens.vesper.bothub.data.AssetItem
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.DashboardSnapshot
import uk.starryheavens.vesper.bothub.data.ModelOptionItem
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.data.TaskItem
import uk.starryheavens.vesper.bothub.data.TodoItemUi
import uk.starryheavens.vesper.bothub.data.WorkNoteItemUi
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperSegmentedTabs
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.vesperPressable
import uk.starryheavens.vesper.bothub.ui.screens.hub.AssetEditorSheet
import uk.starryheavens.vesper.bothub.ui.screens.hub.NoteEditorSheet
import uk.starryheavens.vesper.bothub.ui.screens.hub.TaskEditorSheet
import uk.starryheavens.vesper.bothub.ui.screens.hub.TodoEditorSheet
import uk.starryheavens.vesper.bothub.ui.screens.hub.filterComputeNodes
import uk.starryheavens.vesper.bothub.ui.screens.hub.filterNezhaAlerts
import uk.starryheavens.vesper.bothub.ui.screens.hub.hubAssetsItems
import uk.starryheavens.vesper.bothub.ui.screens.hub.hubNodesItems
import uk.starryheavens.vesper.bothub.ui.screens.hub.hubTasksItems
import uk.starryheavens.vesper.bothub.ui.screens.hub.hubWorkspaceItems
import uk.starryheavens.vesper.bothub.ui.screens.hub.mergeExtensibleAssetCategories
import uk.starryheavens.vesper.bothub.ui.screens.hub.summarizeAssetsCost
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens
import java.time.LocalDate
import java.util.Locale

@Composable
fun HubTabScreen(
    snapshot: DashboardSnapshot?,
    bots: List<BotItem> = emptyList(),
    modelOptions: List<ModelOptionItem> = emptyList(),
    notifications: List<NotificationItem> = emptyList(),
    onRunTask: (TaskItem) -> Unit,
    onToggleTask: (TaskItem, Boolean) -> Unit,
    onSaveAsset: (
        existingId: String?,
        name: String,
        category: String,
        spec: String,
        price: String,
        status: String,
        dueDate: String,
        autoRenew: Boolean,
        warnDays: String,
        extraConfig: String
    ) -> Unit,
    onDeleteAsset: (String) -> Unit,
    onSaveAssetCategories: (List<AssetCategoryItem>) -> Unit = {},
    onSaveTask: (
        existingId: String?,
        name: String,
        type: String,
        botId: String,
        cronExpr: String,
        runAt: String,
        description: String,
        model: String,
        enabled: Boolean,
        params: String
    ) -> Unit = { _, _, _, _, _, _, _, _, _, _ -> },
    onDeleteTask: (String) -> Unit = {},
    onOptimizeTaskPrompt: (name: String, rawPrompt: String, onResult: (String?) -> Unit) -> Unit = { _, _, cb -> cb(null) },
    onCreateTodo: (title: String, notes: String, dueAt: String, priority: String) -> Unit = { _, _, _, _ -> },
    onToggleTodoDone: (TodoItemUi) -> Unit = {},
    onUpdateTodo: (id: String, title: String, notes: String, dueAt: String, priority: String) -> Unit = { _, _, _, _, _ -> },
    onDeleteTodo: (String) -> Unit = {},
    onClearDoneTodos: () -> Unit = {},
    onSaveNote: (existingId: String?, title: String, content: String, category: String, pinned: Boolean) -> Unit = { _, _, _, _, _ -> },
    onDeleteNote: (String) -> Unit = {},
    onOpenServiceFeed: () -> Unit = {},
    onInspectNezhaWithAi: () -> Unit = {},
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val assets = snapshot?.assets ?: emptyList()
    val savedCategories = snapshot?.assetCategories ?: emptyList()
    val mergedCategories = remember(savedCategories, assets) {
        mergeExtensibleAssetCategories(savedCategories, assets)
    }
    val tasks = snapshot?.tasks ?: emptyList()
    val todos = snapshot?.todos ?: emptyList()
    val notes = snapshot?.notes ?: emptyList()
    val expiringCount = snapshot?.expiringCount ?: 0
    val activeTasksCount = snapshot?.activeTasks ?: tasks.count { it.enabled }

    val openTodos = remember(todos) { todos.filter { !it.isDone } }
    val doneTodos = remember(todos) { todos.filter { it.isDone } }
    val computeNodes = remember(assets) { filterComputeNodes(assets) }
    val nezhaAlerts = remember(notifications) { filterNezhaAlerts(notifications) }
    val unreadAlertsCount = remember(nezhaAlerts) { nezhaAlerts.count { it.isUnread } }

    val todayPrefix = remember { LocalDate.now().toString() }
    val focusAssets = remember(assets) {
        assets.filter { a ->
            val isPerm = a.dueDate.startsWith("2099") || a.daysLeft > 3650
            !isPerm && a.status != "已停用" && a.status != "已归档" && a.daysLeft <= 14
        }.sortedBy { it.daysLeft }
    }
    val focusTodos = remember(openTodos, todayPrefix) {
        openTodos.filter { t ->
            val due = t.dueAt.trim().take(10)
            val isUrgentDate = due.length == 10 && due <= todayPrefix
            isUrgentDate || t.priority.equals("high", ignoreCase = true)
        }.sortedWith(
            compareBy<TodoItemUi>(
                {
                    val due = it.dueAt.trim().take(10)
                    when {
                        due.length == 10 && due < todayPrefix -> 0
                        due.length == 10 && due == todayPrefix -> 1
                        it.priority.equals("high", ignoreCase = true) -> 2
                        else -> 3
                    }
                },
                { it.dueAt.ifBlank { "9999-12-31" } }
            )
        )
    }
    val sortedNotes = remember(notes) {
        notes.sortedWith(compareByDescending<WorkNoteItemUi> { it.pinned }.thenByDescending { it.updatedAt })
    }

    var selectedSegment by remember { mutableStateOf(0) }
    var assetSearchQuery by remember { mutableStateOf("") }
    var selectedCategoryFilter by remember { mutableStateOf("all") }
    var assetExpiringOnly by remember { mutableStateOf(false) }
    var assetSortByUrgency by remember { mutableStateOf(false) }
    var showDoneTodos by remember { mutableStateOf(false) }

    var showAssetSheet by remember { mutableStateOf(false) }
    var editingAsset by remember { mutableStateOf<AssetItem?>(null) }

    var showTaskSheet by remember { mutableStateOf(false) }
    var editingTask by remember { mutableStateOf<TaskItem?>(null) }

    var showTodoSheet by remember { mutableStateOf(false) }
    var editingTodo by remember { mutableStateOf<TodoItemUi?>(null) }

    var showNoteSheet by remember { mutableStateOf(false) }
    var editingNote by remember { mutableStateOf<WorkNoteItemUi?>(null) }

    val filteredAssets = remember(
        assets,
        assetSearchQuery,
        selectedCategoryFilter,
        assetExpiringOnly,
        assetSortByUrgency
    ) {
        val q = assetSearchQuery.trim().lowercase(Locale.ROOT)
        val matched = assets.filter { a ->
            if (selectedCategoryFilter != "all" && !a.category.equals(selectedCategoryFilter, ignoreCase = true)) {
                return@filter false
            }
            if (assetExpiringOnly && !a.isExpiringSoon) {
                return@filter false
            }
            if (q.isEmpty()) true
            else "${a.name} ${a.spec} ${a.status} ${a.extraConfig}".lowercase(Locale.ROOT).contains(q)
        }
        if (!assetSortByUrgency) {
            matched
        } else {
            matched.sortedWith(
                compareBy<AssetItem> { it.isPerpetual }
                    .thenBy { it.daysLeft }
            )
        }
    }

    val fxRates = snapshot?.fxRates ?: emptyMap()
    val allMonthlyCost = remember(assets, fxRates) { summarizeAssetsCost(assets, yearly = false, fxRates = fxRates) }
    val totalMonthlyCost = remember(filteredAssets, fxRates) { summarizeAssetsCost(filteredAssets, yearly = false, fxRates = fxRates) }
    val totalYearlyCost = remember(filteredAssets, fxRates) { summarizeAssetsCost(filteredAssets, yearly = true, fxRates = fxRates) }
    val categoryCounts = remember(assets) {
        assets.groupingBy { it.category.trim().lowercase(Locale.ROOT) }.eachCount()
    }

    val segmentOptions = listOf("资产", "任务", "工作台", "节点")

    Column(modifier = modifier) {
        // High-density 4-KPI Dashboard Pulse Strip (tapping jumps directly to section)
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(start = 16.dp, end = 16.dp, top = 10.dp, bottom = 4.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            HubKpiTile(
                label = "资产月均",
                value = if (allMonthlyCost != "--") allMonthlyCost else "${assets.size}项",
                accent = if (expiringCount > 0) theme.warning else theme.textPrimary,
                active = selectedSegment == 0,
                onClick = { selectedSegment = 0 },
                modifier = Modifier.weight(1f)
            )
            HubKpiTile(
                label = "运行例程",
                value = "$activeTasksCount/${tasks.size}",
                accent = if (activeTasksCount > 0) theme.success else theme.textMuted,
                active = selectedSegment == 1,
                onClick = { selectedSegment = 1 },
                modifier = Modifier.weight(1f)
            )
            HubKpiTile(
                label = "待办事项",
                value = openTodos.size.toString(),
                accent = if (focusTodos.isNotEmpty()) theme.info else theme.textPrimary,
                active = selectedSegment == 2,
                onClick = { selectedSegment = 2 },
                modifier = Modifier.weight(1f)
            )
            HubKpiTile(
                label = "探针节点",
                value = if (unreadAlertsCount > 0) "${unreadAlertsCount}警" else "${computeNodes.size}台",
                accent = if (unreadAlertsCount > 0) theme.danger else theme.success,
                active = selectedSegment == 3,
                onClick = { selectedSegment = 3 },
                modifier = Modifier.weight(1f)
            )
        }

        // Dynamic Today Focus Card (shown only when urgent assets or todos need action)
        if (focusAssets.isNotEmpty() || focusTodos.isNotEmpty()) {
            HubTodayFocusStrip(
                focusAssets = focusAssets,
                focusTodos = focusTodos,
                todayPrefix = todayPrefix,
                onClickAsset = { asset ->
                    editingAsset = asset
                    showAssetSheet = true
                },
                onClickTodo = { todo ->
                    editingTodo = todo
                    showTodoSheet = true
                },
                onToggleTodoDone = onToggleTodoDone,
                modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 6.dp, bottom = 2.dp)
            )
        }

        VesperSegmentedTabs(
            options = segmentOptions,
            selectedIndex = selectedSegment,
            onSelect = { selectedSegment = it },
            modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 4.dp)
        )

        LazyColumn(
            modifier = Modifier
                .fillMaxWidth()
                .weight(1f),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            when (selectedSegment) {
                0 -> hubAssetsItems(
                    assets = assets,
                    filteredAssets = filteredAssets,
                    savedCategories = savedCategories,
                    mergedCategories = mergedCategories,
                    categoryCounts = categoryCounts,
                    searchQuery = assetSearchQuery,
                    selectedCategoryFilter = selectedCategoryFilter,
                    expiringOnly = assetExpiringOnly,
                    sortByUrgency = assetSortByUrgency,
                    totalMonthlyCost = totalMonthlyCost,
                    totalYearlyCost = totalYearlyCost,
                    expiringCount = expiringCount,
                    onSearchQueryChange = { assetSearchQuery = it },
                    onSelectCategory = { selectedCategoryFilter = it },
                    onToggleExpiringOnly = { assetExpiringOnly = !assetExpiringOnly },
                    onToggleSortByUrgency = { assetSortByUrgency = !assetSortByUrgency },
                    onResetFilters = {
                        assetSearchQuery = ""
                        selectedCategoryFilter = "all"
                        assetExpiringOnly = false
                    },
                    onAddAsset = {
                        editingAsset = null
                        showAssetSheet = true
                    },
                    onEditAsset = { asset ->
                        editingAsset = asset
                        showAssetSheet = true
                    }
                )

                1 -> hubTasksItems(
                    tasks = tasks,
                    activeTasksCount = activeTasksCount,
                    onCreateTask = {
                        editingTask = null
                        showTaskSheet = true
                    },
                    onEditTask = { task ->
                        editingTask = task
                        showTaskSheet = true
                    },
                    onRunTask = onRunTask,
                    onToggleTask = onToggleTask
                )

                2 -> hubWorkspaceItems(
                    openTodos = openTodos,
                    doneTodos = doneTodos,
                    sortedNotes = sortedNotes,
                    showDoneTodos = showDoneTodos,
                    onToggleShowDoneTodos = { showDoneTodos = !showDoneTodos },
                    onCreateTodo = {
                        editingTodo = null
                        showTodoSheet = true
                    },
                    onEditTodo = { todo ->
                        editingTodo = todo
                        showTodoSheet = true
                    },
                    onToggleTodoDone = onToggleTodoDone,
                    onClearDoneTodos = {
                        showDoneTodos = false
                        onClearDoneTodos()
                    },
                    onCreateNote = {
                        editingNote = null
                        showNoteSheet = true
                    },
                    onEditNote = { note ->
                        editingNote = note
                        showNoteSheet = true
                    }
                )

                else -> hubNodesItems(
                    computeNodes = computeNodes,
                    nezhaAlerts = nezhaAlerts,
                    fxRates = fxRates,
                    onInspectNezhaWithAi = onInspectNezhaWithAi,
                    onEditNodeAsset = { asset ->
                        editingAsset = asset
                        showAssetSheet = true
                    },
                    onOpenServiceFeed = onOpenServiceFeed
                )
            }
        }
    }

    if (showAssetSheet) {
        AssetEditorSheet(
            asset = editingAsset,
            savedCategories = savedCategories,
            availableCategories = mergedCategories,
            onSaveCategories = onSaveAssetCategories,
            onSave = { name, category, spec, price, status, dueDate, autoRenew, warnDays, extraConfig ->
                onSaveAsset(
                    editingAsset?.id,
                    name,
                    category,
                    spec,
                    price,
                    status,
                    dueDate,
                    autoRenew,
                    warnDays,
                    extraConfig
                )
                showAssetSheet = false
            },
            onDelete = editingAsset?.let { target ->
                {
                    onDeleteAsset(target.id)
                    showAssetSheet = false
                }
            },
            onDismiss = { showAssetSheet = false }
        )
    }

    if (showTaskSheet) {
        TaskEditorSheet(
            task = editingTask,
            bots = bots,
            modelOptions = modelOptions,
            onSave = { name, type, botId, cronExpr, runAt, description, model, enabled, params ->
                onSaveTask(
                    editingTask?.id,
                    name,
                    type,
                    botId,
                    cronExpr,
                    runAt,
                    description,
                    model,
                    enabled,
                    params
                )
                showTaskSheet = false
            },
            onDelete = editingTask?.let { target ->
                {
                    onDeleteTask(target.id)
                    showTaskSheet = false
                }
            },
            onOptimizePrompt = onOptimizeTaskPrompt,
            onDismiss = { showTaskSheet = false }
        )
    }

    if (showTodoSheet) {
        TodoEditorSheet(
            todo = editingTodo,
            onSave = { title, notesText, dueAt, priority ->
                val cur = editingTodo
                if (cur == null) {
                    onCreateTodo(title, notesText, dueAt, priority)
                } else {
                    onUpdateTodo(cur.id, title, notesText, dueAt, priority)
                }
                showTodoSheet = false
            },
            onDelete = editingTodo?.let { target ->
                {
                    onDeleteTodo(target.id)
                    showTodoSheet = false
                }
            },
            onDismiss = { showTodoSheet = false }
        )
    }

    if (showNoteSheet) {
        NoteEditorSheet(
            note = editingNote,
            onSave = { title, content, category, pinned ->
                onSaveNote(editingNote?.id, title, content, category, pinned)
                showNoteSheet = false
            },
            onDelete = editingNote?.let { target ->
                {
                    onDeleteNote(target.id)
                    showNoteSheet = false
                }
            },
            onDismiss = { showNoteSheet = false }
        )
    }
}

@Composable
private fun HubKpiTile(
    label: String,
    value: String,
    accent: Color,
    active: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(12.dp)
    Column(
        modifier = modifier
            .sizeIn(minHeight = 54.dp)
            .vesperPressable(pressedScale = 0.96f, hapticOnTap = true, onClick = onClick)
            .clip(shape)
            .background(if (active) theme.accentSoft else theme.surfaceCard, shape)
            .border(1.dp, if (active) theme.accentMuted else theme.border, shape)
            .padding(horizontal = 8.dp, vertical = 9.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(3.dp, Alignment.CenterVertically)
    ) {
        Text(
            text = value,
            style = TextStyle(
                fontFamily = FontFamily.Monospace,
                fontSize = 13.5.sp,
                fontWeight = FontWeight.Bold,
                color = accent,
                platformStyle = NoFontPaddingStyle
            ),
            maxLines = 1,
            softWrap = false,
            overflow = TextOverflow.Ellipsis
        )
        Text(
            text = label,
            style = TextStyle(
                fontSize = 10.5.sp,
                fontWeight = if (active) FontWeight.SemiBold else FontWeight.Medium,
                color = if (active) theme.accentForeground else theme.textMuted,
                platformStyle = NoFontPaddingStyle
            ),
            maxLines = 1,
            softWrap = false,
            overflow = TextOverflow.Ellipsis
        )
    }
}

@Composable
private fun HubTodayFocusStrip(
    focusAssets: List<AssetItem>,
    focusTodos: List<TodoItemUi>,
    todayPrefix: String,
    onClickAsset: (AssetItem) -> Unit,
    onClickTodo: (TodoItemUi) -> Unit,
    onToggleTodoDone: (TodoItemUi) -> Unit,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val totalFocusCount = focusAssets.size + focusTodos.size
    val hasCritical = focusAssets.any { it.daysLeft <= 3 } ||
        focusTodos.any { it.dueAt.take(10).let { d -> d.length == 10 && d < todayPrefix } }

    VesperCard(
        cornerRadius = 14.dp,
        accentTone = if (hasCritical) theme.danger else theme.warning,
        modifier = modifier.fillMaxWidth()
    ) {
        Column(
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "今日焦点",
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Bold,
                    color = if (hasCritical) theme.danger else theme.warning
                )
                Text(
                    text = "待处理 $totalFocusCount 项",
                    fontFamily = FontFamily.Monospace,
                    fontSize = 11.sp,
                    color = theme.textMuted
                )
            }

            focusAssets.take(3).forEach { asset ->
                val urgentColor = if (asset.daysLeft <= 3) theme.danger else theme.warning
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 42.dp)
                        .clip(RoundedCornerShape(10.dp))
                        .background(theme.surfaceInput)
                        .clickable { onClickAsset(asset) }
                        .padding(horizontal = 10.dp, vertical = 6.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = asset.name,
                        fontSize = 12.5.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = theme.textPrimary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier
                            .weight(1f)
                            .padding(end = 8.dp)
                    )
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        if (asset.price.isNotBlank()) {
                            Text(
                                text = asset.price.replace(" / ", "/"),
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.sp,
                                color = theme.textMuted,
                                maxLines = 1,
                                softWrap = false
                            )
                        }
                        VesperTag(text = asset.dueBadgeLabel, toneColor = urgentColor)
                    }
                }
            }

            focusTodos.take(3).forEach { todo ->
                val due = todo.dueAt.trim().take(10)
                val tagText = when {
                    due.length == 10 && due < todayPrefix -> "已逾期"
                    due.length == 10 && due == todayPrefix -> "今日"
                    else -> "高优"
                }
                val tagColor = if (tagText == "已逾期" || tagText == "高优") theme.danger else theme.warning
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 42.dp)
                        .clip(RoundedCornerShape(10.dp))
                        .background(theme.surfaceInput)
                        .clickable { onClickTodo(todo) }
                        .padding(start = 6.dp, end = 10.dp, top = 4.dp, bottom = 4.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Row(
                        modifier = Modifier
                            .weight(1f)
                            .padding(end = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(4.dp)
                    ) {
                        Box(
                            modifier = Modifier
                                .size(34.dp)
                                .vesperPressable(pressedScale = 0.9f, hapticOnTap = true) {
                                    onToggleTodoDone(todo)
                                },
                            contentAlignment = Alignment.Center
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(18.dp)
                                    .clip(RoundedCornerShape(5.dp))
                                    .background(theme.surfaceCard)
                                    .border(1.3.dp, theme.accent, RoundedCornerShape(5.dp))
                            )
                        }
                        Text(
                            text = todo.title,
                            fontSize = 12.5.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textPrimary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f)
                        )
                    }
                    VesperTag(text = tagText, toneColor = tagColor)
                }
            }
        }
    }
}
