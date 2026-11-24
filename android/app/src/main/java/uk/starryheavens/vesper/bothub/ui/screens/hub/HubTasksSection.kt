package uk.starryheavens.vesper.bothub.ui.screens.hub

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
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
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.CalendarMonth
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.outlined.Schedule
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
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.json.JSONObject
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.ModelOptionItem
import uk.starryheavens.vesper.bothub.data.TaskItem
import uk.starryheavens.vesper.bothub.ui.components.ThemeCalendarDialog
import uk.starryheavens.vesper.bothub.ui.components.VesperButtonTone
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperEmptyState
import uk.starryheavens.vesper.bothub.ui.components.VesperFilterChip
import uk.starryheavens.vesper.bothub.ui.components.VesperIconButton
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSegmentedTabs
import uk.starryheavens.vesper.bothub.ui.components.VesperSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.VesperSwitch
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.VesperTextField
import uk.starryheavens.vesper.bothub.ui.sheets.ModelPickerSheet
import uk.starryheavens.vesper.bothub.ui.sheets.resolveModelOptionDisplayName
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens
import java.time.LocalDate

internal fun LazyListScope.hubTasksItems(
    tasks: List<TaskItem>,
    activeTasksCount: Int,
    onCreateTask: () -> Unit,
    onEditTask: (TaskItem) -> Unit,
    onRunTask: (TaskItem) -> Unit,
    onToggleTask: (TaskItem, Boolean) -> Unit
) {
    item(key = "__hub_tasks_hdr__") {
        val theme = BotHubThemeTokens
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text(
                text = "已启用 $activeTasksCount / ${tasks.size} 项例程",
                fontSize = 12.5.sp,
                fontWeight = FontWeight.Medium,
                color = theme.textMuted
            )
            VesperSecondaryButton(
                text = "新建任务",
                leadingIcon = Icons.Default.Add,
                tone = VesperButtonTone.ACCENT,
                cornerRadius = 999.dp,
                minHeight = 36.dp,
                contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                onClick = onCreateTask
            )
        }
    }

    if (tasks.isEmpty()) {
        item(key = "__hub_tasks_empty__") {
            VesperEmptyState(
                icon = Icons.Outlined.Schedule,
                title = "暂无定时例程任务",
                subtitle = "点击右上角「新建任务」即可创建周期巡检、早报汇总或单次提醒"
            )
        }
    }

    items(tasks, key = { it.id }, contentType = { "task_card" }) { task ->
        val theme = BotHubThemeTokens
        VesperCard(
            cornerRadius = 15.dp,
            onClick = { onEditTask(task) },
            modifier = Modifier.fillMaxWidth()
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .sizeIn(minHeight = 64.dp)
                    .padding(start = 14.dp, end = 6.dp, top = 8.dp, bottom = 8.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(
                    modifier = Modifier
                        .weight(1f)
                        .padding(end = 6.dp),
                    verticalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        Text(
                            text = task.name,
                            fontSize = 14.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = if (task.enabled) theme.textPrimary else theme.textMuted,
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f, fill = false)
                        )
                    }
                    Text(
                        text = task.formattedSchedule(),
                        fontFamily = FontFamily.Monospace,
                        fontSize = 11.5.sp,
                        color = theme.textMuted,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    if (task.lastSummary.isNotBlank()) {
                        Text(
                            text = task.lastSummary,
                            fontSize = 11.5.sp,
                            color = theme.textSecondary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }

                Row(verticalAlignment = Alignment.CenterVertically) {
                    VesperIconButton(
                        icon = Icons.Default.PlayArrow,
                        contentDescription = "立即运行任务",
                        accent = true,
                        visualSize = 34.dp,
                        iconSize = 17.dp,
                        cornerRadius = 10.dp,
                        onClick = { onRunTask(task) }
                    )
                    VesperSwitch(
                        checked = task.enabled,
                        onCheckedChange = { checked -> onToggleTask(task, checked) }
                    )
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
internal fun TaskEditorSheet(
    task: TaskItem?,
    bots: List<BotItem>,
    modelOptions: List<ModelOptionItem>,
    onSave: (
        name: String,
        type: String,
        botId: String,
        cronExpr: String,
        runAt: String,
        description: String,
        model: String,
        enabled: Boolean,
        params: String
    ) -> Unit,
    onDelete: (() -> Unit)?,
    onOptimizePrompt: (name: String, rawPrompt: String, onResult: (String?) -> Unit) -> Unit = { _, _, cb -> cb(null) },
    onDismiss: () -> Unit
) {
    val theme = BotHubThemeTokens
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val workerBots = remember(bots) { bots.filter { !it.isGroup } }
    val defaultBotId = workerBots.firstOrNull { it.id == "concierge" }?.id
        ?: workerBots.firstOrNull()?.id.orEmpty()

    var name by remember(task) { mutableStateOf(task?.name.orEmpty()) }
    var type by remember(task) { mutableStateOf(task?.type?.ifBlank { "bot_prompt" } ?: "bot_prompt") }
    var botId by remember(task) { mutableStateOf(task?.botId?.ifBlank { defaultBotId } ?: defaultBotId) }
    var isOneShot by remember(task) {
        mutableStateOf(task != null && task.cronExpr.isBlank() && task.runAt.isNotBlank())
    }
    var cronExpr by remember(task) { mutableStateOf(task?.cronExpr?.ifBlank { "0 9 * * *" } ?: "0 9 * * *") }
    var oneShotDate by remember(task) {
        val raw = task?.runAt?.take(10).orEmpty()
        mutableStateOf(raw.ifBlank { LocalDate.now().plusDays(1).toString() })
    }
    var oneShotTime by remember(task) {
        val raw = task?.runAt?.trim().orEmpty()
        val timePart = if (raw.length >= 16 && raw.contains(" ")) raw.substring(11, 16) else "09:00"
        mutableStateOf(timePart)
    }
    var promptText by remember(task) {
        val fromParams = runCatching {
            JSONObject(task?.params.orEmpty()).optString("prompt", "")
        }.getOrDefault("")
        mutableStateOf(fromParams.ifBlank { task?.description.orEmpty() })
    }
    var model by remember(task) { mutableStateOf(task?.model.orEmpty()) }
    var enabled by remember(task) { mutableStateOf(task?.enabled ?: true) }
    var showDatePicker by remember { mutableStateOf(false) }
    var showModelPicker by remember { mutableStateOf(false) }

    val templateChoices = listOf(
        "bot_prompt" to "Bot 定时指令",
        "asset_watcher" to "资产到期巡检",
        "morning" to "早间工作简报",
        "radar" to "GitHub 开源雷达"
    )
    val cronPresets = listOf(
        "0 9 * * *" to "每天 09:00",
        "0 21 * * *" to "每天 21:00",
        "0 */6 * * *" to "每 6 小时",
        "0 9 * * 1" to "每周一 09:00"
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
                text = if (task == null) "新建定时任务" else "编辑定时任务",
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )

            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text("任务模板类型", fontSize = 12.sp, color = theme.textMuted)
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    templateChoices.forEach { (tid, label) ->
                        VesperFilterChip(
                            label = label,
                            selected = type == tid,
                            onClick = {
                                type = tid
                                if (name.isBlank()) name = label
                            }
                        )
                    }
                }
            }

            VesperTextField(
                value = name,
                onValueChange = { name = it },
                label = "任务名称",
                placeholder = "例如：每日服务器与资产巡检",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            if (workerBots.isNotEmpty()) {
                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text("负责执行与汇报的 Bot", fontSize = 12.sp, color = theme.textMuted)
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        items(workerBots, key = { it.id }) { b ->
                            VesperFilterChip(
                                label = b.name,
                                selected = botId == b.id,
                                onClick = { botId = b.id }
                            )
                        }
                    }
                }
            }

            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                VesperSegmentedTabs(
                    options = listOf("周期 Cron 例程", "单次定时提醒"),
                    selectedIndex = if (isOneShot) 1 else 0,
                    onSelect = { isOneShot = (it == 1) },
                    modifier = Modifier.fillMaxWidth()
                )

                if (!isOneShot) {
                    FlowRow(
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                        verticalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        cronPresets.forEach { (expr, label) ->
                            VesperFilterChip(
                                label = label,
                                selected = cronExpr.trim() == expr,
                                onClick = { cronExpr = expr }
                            )
                        }
                    }
                    VesperTextField(
                        value = cronExpr,
                        onValueChange = { cronExpr = it },
                        label = "5 段式 Cron 表达式（分 时 日 月 周）",
                        placeholder = "0 9 * * *",
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )
                } else {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                        verticalAlignment = Alignment.Bottom
                    ) {
                        Column(
                            verticalArrangement = Arrangement.spacedBy(6.dp),
                            modifier = Modifier.weight(1.1f)
                        ) {
                            Text("触发日期", fontSize = 12.sp, color = theme.textMuted)
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
                                        .padding(horizontal = 12.dp, vertical = 12.dp),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Text(
                                        text = oneShotDate,
                                        fontFamily = FontFamily.Monospace,
                                        fontSize = 13.5.sp,
                                        color = theme.textPrimary
                                    )
                                    Icon(
                                        imageVector = Icons.Default.CalendarMonth,
                                        contentDescription = "选择日期",
                                        tint = theme.accentForeground,
                                        modifier = Modifier.size(18.dp)
                                    )
                                }
                            }
                        }
                        VesperTextField(
                            value = oneShotTime,
                            onValueChange = { oneShotTime = it },
                            label = "触发时间 (HH:mm)",
                            placeholder = "09:00",
                            singleLine = true,
                            modifier = Modifier.weight(0.9f)
                        )
                    }
                }
            }

            if (type == "bot_prompt") {
                var optimizingPrompt by remember(task) { mutableStateOf(false) }
                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text("定时执行指令 (Prompt)", fontSize = 12.sp, color = theme.textMuted)
                        VesperSecondaryButton(
                            text = if (optimizingPrompt) "AI 精修中..." else "AI 精修指令",
                            tone = VesperButtonTone.ACCENT,
                            enabled = !optimizingPrompt && promptText.isNotBlank(),
                            cornerRadius = 999.dp,
                            minHeight = 34.dp,
                            contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                            onClick = {
                                optimizingPrompt = true
                                onOptimizePrompt(name, promptText) { optimized ->
                                    if (!optimized.isNullOrBlank()) {
                                        promptText = optimized
                                    }
                                    optimizingPrompt = false
                                }
                            }
                        )
                    }
                    VesperTextField(
                        value = promptText,
                        onValueChange = { promptText = it },
                        placeholder = "例如：请巡检所有数字资产与探针节点状态，并汇总今日待办提醒...",
                        singleLine = false,
                        modifier = Modifier.fillMaxWidth()
                    )
                }
            }

            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text("指定执行模型", fontSize = 12.sp, color = theme.textMuted)
                    VesperSecondaryButton(
                        text = if (model.isBlank()) "跟随负责 Bot" else resolveModelOptionDisplayName(model, modelOptions),
                        tone = if (model.isNotBlank()) VesperButtonTone.ACCENT else VesperButtonTone.NEUTRAL,
                        cornerRadius = 999.dp,
                        minHeight = 34.dp,
                        contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                        onClick = { showModelPicker = true }
                    )
                }
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    listOf(
                        "" to "跟随负责 Bot",
                        "auto" to "统一模型池自动路由"
                    ).forEach { (mid, label) ->
                        VesperFilterChip(
                            label = label,
                            selected = model == mid,
                            onClick = { model = mid }
                        )
                    }
                }
            }

            VesperCard(
                cornerRadius = 15.dp,
                onClick = { enabled = !enabled },
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
                    Text("启用此定时任务", fontSize = 14.5.sp, color = theme.textPrimary)
                    VesperSwitch(
                        checked = enabled,
                        onCheckedChange = { enabled = it }
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
                    text = "保存任务",
                    enabled = name.isNotBlank() && (type != "bot_prompt" || promptText.isNotBlank()),
                    minHeight = 48.dp,
                    onClick = {
                        val finalCron = if (isOneShot) "" else cronExpr.trim().ifEmpty { "0 9 * * *" }
                        val finalRunAt = if (isOneShot) "${oneShotDate.trim()} ${oneShotTime.trim().ifEmpty { "09:00" }}" else ""
                        val paramsJson = if (type == "bot_prompt") {
                            JSONObject().put("prompt", promptText.trim()).toString()
                        } else {
                            task?.params?.ifBlank { "{}" } ?: "{}"
                        }
                        onSave(
                            name.trim(),
                            type,
                            botId,
                            finalCron,
                            finalRunAt,
                            promptText.trim(),
                            model.trim(),
                            enabled,
                            paramsJson
                        )
                    },
                    modifier = Modifier.weight(if (onDelete != null) 1.5f else 1f)
                )
            }

            Spacer(modifier = Modifier.height(8.dp))
        }
    }

    if (showDatePicker) {
        ThemeCalendarDialog(
            initialDate = oneShotDate,
            allowPerpetual = false,
            onSelectDate = { picked ->
                oneShotDate = picked
                showDatePicker = false
            },
            onSelectPerpetual = {},
            onDismiss = { showDatePicker = false }
        )
    }

    if (showModelPicker) {
        ModelPickerSheet(
            activeModel = model.ifBlank { "auto" },
            modelOptions = modelOptions,
            onSelect = { chosen ->
                model = chosen
                showModelPicker = false
            },
            onDismiss = { showModelPicker = false }
        )
    }
}
