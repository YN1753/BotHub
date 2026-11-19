package uk.starryheavens.vesper.bothub.ui.components

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Dashboard
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Inventory2
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Schedule
import androidx.compose.material.icons.filled.TaskAlt
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.json.JSONArray
import org.json.JSONObject
import uk.starryheavens.vesper.bothub.data.ChatToolTraceUi
import uk.starryheavens.vesper.bothub.data.TodoItemUi
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens
import java.util.Locale

private val STRUCTURED_TOOL_NAMES = setOf(
    "manage_assets",
    "query_nezha_nodes",
    "manage_todos",
    "manage_tasks",
    "workspace_briefing"
)

private const val MAX_INLINE_ITEMS = 6

@Composable
fun ChatStructuredToolCards(
    traces: List<ChatToolTraceUi>,
    onToggleTodo: (String, String) -> Unit,
    liveTodos: List<TodoItemUi> = emptyList(),
    modifier: Modifier = Modifier
) {
    val parsedCards = remember(traces) {
        traces.mapNotNull { tr ->
            if (tr.name !in STRUCTURED_TOOL_NAMES || tr.result.isBlank()) return@mapNotNull null
            val trimmed = tr.result.trimStart()
            if (!trimmed.startsWith("{")) return@mapNotNull null
            val json = runCatching { JSONObject(trimmed) }.getOrNull() ?: return@mapNotNull null
            tr.id to (tr.name to json)
        }
    }
    if (parsedCards.isEmpty()) return

    val optimisticTodoStatus = remember { mutableStateMapOf<String, String>() }

    Column(
        modifier = modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        parsedCards.forEach { (_, pair) ->
            val (toolName, json) = pair
            when (toolName) {
                "manage_assets" -> ManageAssetsStructuredCard(json = json)
                "query_nezha_nodes" -> NezhaNodesStructuredCard(json = json)
                "manage_todos" -> ManageTodosStructuredCard(
                    json = json,
                    liveTodos = liveTodos,
                    optimisticStatus = optimisticTodoStatus,
                    onToggleTodo = onToggleTodo
                )
                "manage_tasks" -> ManageTasksStructuredCard(json = json)
                "workspace_briefing" -> WorkspaceBriefingStructuredCard(json = json)
            }
        }
    }
}

@Composable
private fun StructuredSectionHeader(
    icon: ImageVector,
    title: String,
    badge: String = "",
    tint: Color
) {
    val theme = BotHubThemeTokens
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(5.dp),
            modifier = Modifier.weight(1f, fill = false)
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = tint,
                modifier = Modifier.size(13.dp)
            )
            Text(
                text = title,
                style = TextStyle(
                    fontSize = 11.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = theme.textSecondary,
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 1,
                softWrap = false,
                overflow = TextOverflow.Ellipsis
            )
        }
        if (badge.isNotBlank()) {
            CrispTagPill(
                text = badge,
                bgColor = tint.copy(alpha = 0.14f),
                borderColor = tint.copy(alpha = 0.34f),
                textColor = tint
            )
        }
    }
}

@Composable
private fun ManageAssetsStructuredCard(json: JSONObject) {
    val theme = BotHubThemeTokens
    val accentTint = if (theme.isLight) theme.accentDeep else theme.accentLight

    // Summary action
    if (json.has("cost_by_currency") || json.has("active_count")) {
        val activeCount = json.optInt("active_count", 0)
        val expiringCount = json.optInt("expiring_30d_count", 0)
        val inactiveCount = json.optInt("inactive_count", 0)
        val currencies = json.optJSONArray("cost_by_currency")
        val cardShape = RoundedCornerShape(12.dp)

        Column(
            modifier = Modifier
                .fillMaxWidth()
                .clip(cardShape)
                .background(theme.codeBlockBg, cardShape)
                .border(0.75.dp, theme.rimMid, cardShape)
                .padding(10.dp),
            verticalArrangement = Arrangement.spacedBy(7.dp)
        ) {
            StructuredSectionHeader(
                icon = Icons.Default.Inventory2,
                title = "资产费用与台账概览",
                badge = "${activeCount}项在用",
                tint = accentTint
            )
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                BriefingKpiChip(
                    label = "在用资产",
                    value = activeCount.toString(),
                    accent = if (theme.isLight) Color(0xFF059669) else Color(0xFF34D399),
                    modifier = Modifier.weight(1f)
                )
                BriefingKpiChip(
                    label = "30天临期",
                    value = expiringCount.toString(),
                    accent = if (expiringCount > 0) {
                        if (theme.isLight) Color(0xFFD97706) else Color(0xFFFBBF24)
                    } else {
                        theme.textMuted
                    },
                    modifier = Modifier.weight(1f)
                )
                BriefingKpiChip(
                    label = "已停用",
                    value = inactiveCount.toString(),
                    accent = theme.textMuted,
                    modifier = Modifier.weight(1f)
                )
            }
            if (currencies != null && currencies.length() > 0) {
                Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    for (i in 0 until currencies.length()) {
                        val c = currencies.optJSONObject(i) ?: continue
                        val curr = c.optString("currency", "USD")
                        val monthly = c.optDouble("monthly_estimated", 0.0)
                        val annual = c.optDouble("annualized_estimated", 0.0)
                        val itemsCount = c.optInt("priced_items", 0)
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clip(RoundedCornerShape(8.dp))
                                .background(theme.cardTop)
                                .border(0.6.dp, theme.rimSubtle, RoundedCornerShape(8.dp))
                                .padding(horizontal = 9.dp, vertical = 6.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.SpaceBetween
                        ) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                CrispTagPill(text = curr)
                                Text(
                                    text = "${itemsCount}项计费",
                                    fontSize = 11.sp,
                                    color = theme.textMuted,
                                    maxLines = 1,
                                    softWrap = false
                                )
                            }
                            Text(
                                text = String.format(
                                    Locale.US,
                                    "月 %.2f · 年 %.0f",
                                    monthly,
                                    annual
                                ),
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textPrimary,
                                maxLines = 1,
                                softWrap = false
                            )
                        }
                    }
                }
            }
        }
        return
    }

    val listArr = json.optJSONArray("assets") ?: json.optJSONArray("expiring")
    val singleAsset = json.optJSONObject("asset")
    val items = buildList {
        if (listArr != null) {
            for (i in 0 until listArr.length()) {
                listArr.optJSONObject(i)?.let { add(it) }
            }
        } else if (singleAsset != null) {
            add(singleAsset)
        }
    }
    if (items.isEmpty()) return

    val headerTitle = if (json.has("expiring")) "近期临期资产" else "数字资产台账"
    val shown = items.take(MAX_INLINE_ITEMS)

    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        StructuredSectionHeader(
            icon = Icons.Default.Inventory2,
            title = headerTitle,
            badge = "${items.size}项",
            tint = accentTint
        )
        shown.forEach { a ->
            AssetMicroCard(a = a)
        }
        if (items.size > MAX_INLINE_ITEMS) {
            OverflowHintText(remaining = items.size - MAX_INLINE_ITEMS)
        }
    }
}

@Composable
private fun AssetMicroCard(a: JSONObject) {
    val theme = BotHubThemeTokens
    val name = a.optString("name", "").ifBlank { a.optString("id", "未命名资产") }
    val status = a.optString("status", "生效中").ifBlank { "生效中" }
    val price = a.optString("price", "").trim()
    val spec = a.optString("spec", "").trim()
    val dueDate = a.optString("due_date", "").trim()
    val daysLeft = a.optInt("days_left", 999)
    val isPerpetual = dueDate.startsWith("2099")
    val isInactive = status == "已停用"

    val indicatorColor = when {
        isInactive -> theme.textMuted
        isPerpetual -> if (theme.isLight) Color(0xFF059669) else Color(0xFF34D399)
        daysLeft <= 3 -> if (theme.isLight) Color(0xFFDC2626) else Color(0xFFFB7185)
        daysLeft <= 14 -> if (theme.isLight) Color(0xFFD97706) else Color(0xFFFBBF24)
        else -> if (theme.isLight) Color(0xFF059669) else Color(0xFF34D399)
    }

    val dueFraction = when {
        isInactive -> 0f
        isPerpetual -> 1f
        daysLeft <= 0 -> 0.08f
        else -> (daysLeft.toFloat() / 30f).coerceIn(0.10f, 1f)
    }

    val cardShape = RoundedCornerShape(11.dp)
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(IntrinsicSize.Min)
            .clip(cardShape)
            .background(theme.codeBlockBg, cardShape)
            .border(0.75.dp, theme.rimMid, cardShape)
    ) {
        Box(
            modifier = Modifier
                .width(3.dp)
                .fillMaxHeight()
                .background(indicatorColor)
        )
        Column(
            modifier = Modifier
                .weight(1f)
                .padding(horizontal = 10.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(5.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                Text(
                    text = name,
                    fontSize = 12.5.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = theme.textPrimary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f)
                )
                if (price.isNotEmpty()) {
                    CrispTagPill(
                        text = price,
                        bgColor = theme.accent.copy(alpha = 0.14f),
                        borderColor = theme.accent.copy(alpha = 0.34f),
                        textColor = if (theme.isLight) theme.accentDeep else theme.accentLight
                    )
                }
                CrispTagPill(
                    text = status,
                    bgColor = indicatorColor.copy(alpha = 0.14f),
                    borderColor = indicatorColor.copy(alpha = 0.34f),
                    textColor = indicatorColor
                )
            }

            if (spec.isNotEmpty()) {
                Text(
                    text = spec,
                    fontSize = 11.sp,
                    color = theme.textMuted,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Text(
                    text = when {
                        isPerpetual -> "长期有效"
                        dueDate.isNotEmpty() -> dueDate.take(10)
                        else -> "未设到期日"
                    },
                    fontFamily = FontFamily.Monospace,
                    fontSize = 10.5.sp,
                    color = theme.textSecondary,
                    maxLines = 1,
                    softWrap = false
                )
                Box(
                    modifier = Modifier
                        .weight(1f)
                        .height(4.dp)
                        .clip(RoundedCornerShape(999.dp))
                        .background(theme.divider)
                ) {
                    Box(
                        modifier = Modifier
                            .fillMaxWidth(dueFraction)
                            .fillMaxHeight()
                            .background(indicatorColor, RoundedCornerShape(999.dp))
                    )
                }
                Text(
                    text = if (isPerpetual) "长期" else if (daysLeft < 0) "逾期${-daysLeft}天" else "${daysLeft}天",
                    fontFamily = FontFamily.Monospace,
                    fontSize = 10.5.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = indicatorColor,
                    maxLines = 1,
                    softWrap = false
                )
            }
        }
    }
}

@Composable
private fun NezhaNodesStructuredCard(json: JSONObject) {
    val theme = BotHubThemeTokens
    val onlineGreen = if (theme.isLight) Color(0xFF059669) else Color(0xFF34D399)
    val nezhaArr = json.optJSONArray("nezha_nodes")
    val computeArr = json.optJSONArray("compute_assets")
    val nodes = buildList {
        if (nezhaArr != null) {
            for (i in 0 until nezhaArr.length()) {
                nezhaArr.optJSONObject(i)?.let { add(it) }
            }
        }
    }
    val computeAssets = buildList {
        if (computeArr != null) {
            for (i in 0 until computeArr.length()) {
                computeArr.optJSONObject(i)?.let { add(it) }
            }
        }
    }
    if (nodes.isEmpty() && computeAssets.isEmpty()) return

    val totalCount = nodes.size + computeAssets.size
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        StructuredSectionHeader(
            icon = Icons.Default.Dns,
            title = if (nodes.isNotEmpty()) "探针节点巡检" else "计算节点台账",
            badge = "${totalCount}台",
            tint = onlineGreen
        )

        nodes.take(MAX_INLINE_ITEMS).forEach { n ->
            val name = n.optString("name", "").ifBlank { n.optString("id", "节点") }
            val status = n.optString("status", "online").lowercase()
            val isOnline = status == "online" || status == "生效中"
            val dotColor = if (isOnline) onlineGreen else Color(0xFFF59E0B)
            val rawHostInfo = n.optString("host_info", "").trim()
            val hostHighlight = remember(rawHostInfo) { formatHostInfoSummary(rawHostInfo) }
            val lastActive = formatCompactTime(n.optString("last_active", ""))

            val cardShape = RoundedCornerShape(11.dp)
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(cardShape)
                    .background(theme.codeBlockBg, cardShape)
                    .border(0.75.dp, theme.rimMid, cardShape)
                    .padding(horizontal = 10.dp, vertical = 8.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    Box(
                        modifier = Modifier
                            .size(7.dp)
                            .background(dotColor, CircleShape)
                    )
                    Text(
                        text = name,
                        fontSize = 12.5.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = theme.textPrimary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f)
                    )
                    if (lastActive.isNotEmpty()) {
                        Text(
                            text = lastActive,
                            fontFamily = FontFamily.Monospace,
                            fontSize = 10.sp,
                            color = theme.textMuted,
                            maxLines = 1,
                            softWrap = false
                        )
                    }
                }
                if (hostHighlight.isNotEmpty()) {
                    Text(
                        text = hostHighlight,
                        fontSize = 11.sp,
                        color = theme.textSecondary,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
        }

        if (nodes.size < MAX_INLINE_ITEMS && computeAssets.isNotEmpty()) {
            computeAssets.take(MAX_INLINE_ITEMS - nodes.size).forEach { a ->
                AssetMicroCard(a = a)
            }
        }

        if (totalCount > MAX_INLINE_ITEMS) {
            OverflowHintText(remaining = totalCount - MAX_INLINE_ITEMS)
        }
    }
}

@Composable
private fun ManageTodosStructuredCard(
    json: JSONObject,
    liveTodos: List<TodoItemUi>,
    optimisticStatus: MutableMap<String, String>,
    onToggleTodo: (String, String) -> Unit
) {
    val theme = BotHubThemeTokens
    val haptics = LocalHapticFeedback.current
    val todoArr = json.optJSONArray("todos")
    val singleTodo = json.optJSONObject("todo")
    val items = buildList {
        if (todoArr != null) {
            for (i in 0 until todoArr.length()) {
                todoArr.optJSONObject(i)?.let { add(it) }
            }
        } else if (singleTodo != null) {
            add(singleTodo)
        }
    }
    if (items.isEmpty()) return

    val headerTint = if (theme.isLight) Color(0xFF0284C7) else Color(0xFF38BDF8)
    val doneGreen = if (theme.isLight) Color(0xFF059669) else Color(0xFF34D399)

    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        StructuredSectionHeader(
            icon = Icons.Default.TaskAlt,
            title = "待办事项",
            badge = "${items.size}项",
            tint = headerTint
        )
        items.take(MAX_INLINE_ITEMS).forEach { td ->
            val id = td.optString("id", "").trim()
            val title = td.optString("title", "").ifBlank { "未命名待办" }
            val rawStatus = td.optString("status", "todo")
            val liveMatch = if (id.isNotEmpty()) liveTodos.firstOrNull { it.id == id } else null
            val effectiveStatus = optimisticStatus[id] ?: liveMatch?.status ?: rawStatus
            val isDone = effectiveStatus.equals("done", ignoreCase = true)
            val priority = td.optString("priority", "normal").lowercase()
            val dueAt = td.optString("due_at", "").trim().take(10)

            val prioColor = when (priority) {
                "high" -> if (theme.isLight) Color(0xFFDC2626) else Color(0xFFFB7185)
                "medium" -> if (theme.isLight) Color(0xFFD97706) else Color(0xFFFBBF24)
                else -> theme.textMuted
            }
            val circleColor by animateColorAsState(
                targetValue = if (isDone) doneGreen else theme.rimHighlight,
                animationSpec = tween(150),
                label = "todoCheckCircle"
            )

            val cardShape = RoundedCornerShape(11.dp)
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(cardShape)
                    .background(theme.codeBlockBg, cardShape)
                    .border(0.75.dp, theme.rimMid, cardShape)
                    .clickable(enabled = id.isNotEmpty()) {
                        haptics.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                        val nextStatus = if (isDone) "todo" else "done"
                        optimisticStatus[id] = nextStatus
                        onToggleTodo(id, nextStatus)
                    }
                    .padding(horizontal = 10.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Box(
                    modifier = Modifier
                        .size(19.dp)
                        .clip(CircleShape)
                        .background(if (isDone) doneGreen else Color.Transparent)
                        .border(1.4.dp, circleColor, CircleShape),
                    contentAlignment = Alignment.Center
                ) {
                    if (isDone) {
                        Icon(
                            imageVector = Icons.Default.Check,
                            contentDescription = "已完成",
                            tint = Color.White,
                            modifier = Modifier.size(12.dp)
                        )
                    }
                }

                Box(
                    modifier = Modifier
                        .size(6.dp)
                        .background(prioColor, CircleShape)
                )

                Text(
                    text = title,
                    fontSize = 12.5.sp,
                    fontWeight = FontWeight.Medium,
                    color = if (isDone) theme.textMuted else theme.textPrimary,
                    textDecoration = if (isDone) TextDecoration.LineThrough else TextDecoration.None,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f)
                )

                if (dueAt.isNotEmpty()) {
                    CrispTagPill(
                        text = dueAt,
                        bgColor = theme.cardTop,
                        borderColor = theme.rimMid,
                        textColor = theme.textSecondary
                    )
                }
            }
        }
        if (items.size > MAX_INLINE_ITEMS) {
            OverflowHintText(remaining = items.size - MAX_INLINE_ITEMS)
        }
    }
}

@Composable
private fun ManageTasksStructuredCard(json: JSONObject) {
    val theme = BotHubThemeTokens
    val tasksArr = json.optJSONArray("tasks")
    val singleTask = json.optJSONObject("task")
    val items = buildList {
        if (tasksArr != null) {
            for (i in 0 until tasksArr.length()) {
                tasksArr.optJSONObject(i)?.let { add(it) }
            }
        } else if (singleTask != null) {
            add(singleTask)
        }
    }
    if (items.isEmpty()) return

    val accentTint = if (theme.isLight) Color(0xFF7C3AED) else Color(0xFFC084FC)
    val enabledGreen = if (theme.isLight) Color(0xFF059669) else Color(0xFF34D399)

    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        StructuredSectionHeader(
            icon = Icons.Default.Schedule,
            title = "定时与调度任务",
            badge = "${items.size}项",
            tint = accentTint
        )
        items.take(MAX_INLINE_ITEMS).forEach { tk ->
            val name = tk.optString("name", "").ifBlank { tk.optString("id", "定时任务") }
            val enabled = tk.optBoolean("enabled", false)
            val isRunning = tk.optBoolean("is_running", false)
            val cronExpr = tk.optString("cron_expr", "").trim()
            val runAt = tk.optString("run_at", "").trim()
            val nextRunAt = formatCompactTime(tk.optString("next_run_at", ""))
            val lastSummary = tk.optJSONObject("last_run")
                ?.optString("summary", "")
                ?.trim()
                .orEmpty()
                .ifBlank { tk.optString("description", "").trim() }

            val stateColor = when {
                isRunning -> theme.accent
                enabled -> enabledGreen
                else -> theme.textMuted
            }
            val stateLabel = when {
                isRunning -> "执行中"
                enabled -> "已启用"
                else -> "已暂停"
            }
            val scheduleLabel = buildString {
                if (cronExpr.isNotEmpty()) {
                    append(cronExpr)
                } else if (runAt.isNotEmpty()) {
                    append("单次 ").append(formatCompactTime(runAt))
                }
                if (nextRunAt.isNotEmpty()) {
                    if (isNotEmpty()) append(" · 下次 ")
                    append(nextRunAt)
                }
            }

            val cardShape = RoundedCornerShape(11.dp)
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(cardShape)
                    .background(theme.codeBlockBg, cardShape)
                    .border(0.75.dp, theme.rimMid, cardShape)
                    .padding(horizontal = 10.dp, vertical = 8.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    Icon(
                        imageVector = if (enabled) Icons.Default.PlayArrow else Icons.Default.Pause,
                        contentDescription = null,
                        tint = stateColor,
                        modifier = Modifier.size(14.dp)
                    )
                    Text(
                        text = name,
                        fontSize = 12.5.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = theme.textPrimary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f)
                    )
                    CrispTagPill(
                        text = stateLabel,
                        bgColor = stateColor.copy(alpha = 0.14f),
                        borderColor = stateColor.copy(alpha = 0.34f),
                        textColor = stateColor
                    )
                }
                if (scheduleLabel.isNotEmpty()) {
                    Text(
                        text = scheduleLabel,
                        fontFamily = FontFamily.Monospace,
                        fontSize = 10.5.sp,
                        color = theme.textSecondary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
                if (lastSummary.isNotEmpty()) {
                    Text(
                        text = lastSummary,
                        fontSize = 11.sp,
                        color = theme.textMuted,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
        }
        if (items.size > MAX_INLINE_ITEMS) {
            OverflowHintText(remaining = items.size - MAX_INLINE_ITEMS)
        }
    }
}

@Composable
private fun WorkspaceBriefingStructuredCard(json: JSONObject) {
    val theme = BotHubThemeTokens
    val today = json.optString("today", "").trim()
    val weekday = json.optString("weekday", "").trim()
    val overdueCount = json.optJSONArray("overdue_todos")?.length() ?: 0
    val dueTodayCount = json.optJSONArray("due_today_todos")?.length() ?: 0
    val expiringCount = json.optJSONArray("expiring_assets")?.length() ?: 0
    val unreadCount = json.optInt(
        "unread_count",
        json.optJSONArray("unread_alerts")?.length() ?: 0
    )
    val hints = json.optJSONArray("focus_hints")

    val accentTint = if (theme.isLight) theme.accentDeep else theme.accentLight
    val cardShape = RoundedCornerShape(12.dp)

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clip(cardShape)
            .background(theme.codeBlockBg, cardShape)
            .border(0.75.dp, theme.rimMid, cardShape)
            .padding(10.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        StructuredSectionHeader(
            icon = Icons.Default.Dashboard,
            title = "今日工作简报",
            badge = listOf(today, weekday).filter { it.isNotEmpty() }.joinToString(" "),
            tint = accentTint
        )
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(6.dp)
        ) {
            BriefingKpiChip(
                label = "逾期",
                value = overdueCount.toString(),
                accent = if (overdueCount > 0) {
                    if (theme.isLight) Color(0xFFDC2626) else Color(0xFFFB7185)
                } else {
                    theme.textMuted
                },
                modifier = Modifier.weight(1f)
            )
            BriefingKpiChip(
                label = "今日待办",
                value = dueTodayCount.toString(),
                accent = if (theme.isLight) Color(0xFF0284C7) else Color(0xFF38BDF8),
                modifier = Modifier.weight(1f)
            )
            BriefingKpiChip(
                label = "临期资产",
                value = expiringCount.toString(),
                accent = if (expiringCount > 0) {
                    if (theme.isLight) Color(0xFFD97706) else Color(0xFFFBBF24)
                } else {
                    theme.textMuted
                },
                modifier = Modifier.weight(1f)
            )
            BriefingKpiChip(
                label = "未读通知",
                value = unreadCount.toString(),
                accent = if (unreadCount > 0) accentTint else theme.textMuted,
                modifier = Modifier.weight(1f)
            )
        }
        if (hints != null && hints.length() > 0) {
            Column(verticalArrangement = Arrangement.spacedBy(3.dp)) {
                val limit = minOf(hints.length(), 2)
                for (i in 0 until limit) {
                    val hint = hints.optString(i, "").trim()
                    if (hint.isNotEmpty()) {
                        Text(
                            text = hint,
                            fontSize = 11.sp,
                            color = theme.textSecondary,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun BriefingKpiChip(
    label: String,
    value: String,
    accent: Color,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(9.dp)
    Column(
        modifier = modifier
            .clip(shape)
            .background(theme.cardTop, shape)
            .border(0.6.dp, accent.copy(alpha = 0.28f), shape)
            .padding(horizontal = 6.dp, vertical = 6.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(2.dp)
    ) {
        Text(
            text = value,
            style = TextStyle(
                fontSize = 14.sp,
                fontFamily = FontFamily.Monospace,
                fontWeight = FontWeight.Bold,
                color = accent,
                platformStyle = NoFontPaddingStyle
            ),
            maxLines = 1,
            softWrap = false
        )
        Text(
            text = label,
            style = TextStyle(
                fontSize = 9.5.sp,
                fontWeight = FontWeight.Medium,
                color = theme.textMuted,
                platformStyle = NoFontPaddingStyle
            ),
            maxLines = 1,
            softWrap = false,
            overflow = TextOverflow.Ellipsis
        )
    }
}

@Composable
private fun OverflowHintText(remaining: Int) {
    val theme = BotHubThemeTokens
    Text(
        text = "另有 $remaining 项未展开，详情见下方总结",
        fontSize = 10.5.sp,
        color = theme.textMuted,
        maxLines = 1,
        softWrap = false,
        modifier = Modifier.padding(start = 2.dp)
    )
}

private fun formatCompactTime(raw: String): String {
    val clean = raw.trim()
    if (clean.isEmpty()) return ""
    if (clean.length >= 16 && clean[4] == '-' && (clean[10] == ' ' || clean[10] == 'T')) {
        return clean.substring(5, 16).replace('T', ' ')
    }
    return clean.take(16)
}

private fun formatHostInfoSummary(raw: String): String {
    val clean = raw.trim()
    if (clean.isEmpty()) return ""
    if (clean.startsWith("{")) {
        val obj = runCatching { JSONObject(clean) }.getOrNull()
        if (obj != null) {
            val parts = buildList {
                val platform = obj.optString("Platform", obj.optString("platform", "")).trim()
                val version = obj.optString("PlatformVersion", obj.optString("platform_version", "")).trim()
                val arch = obj.optString("Arch", obj.optString("arch", "")).trim()
                if (platform.isNotEmpty()) {
                    add(if (version.isNotEmpty()) "$platform $version" else platform)
                }
                if (arch.isNotEmpty()) add(arch)
                val cpuArr: JSONArray? = obj.optJSONArray("CPU") ?: obj.optJSONArray("cpu")
                if (cpuArr != null && cpuArr.length() > 0) {
                    val cpuStr = cpuArr.optString(0, "").trim()
                    if (cpuStr.isNotEmpty()) add(cpuStr.take(28))
                }
                val memBytes = obj.optLong("MemTotal", obj.optLong("mem_total", 0L))
                if (memBytes > 0L) {
                    val gb = memBytes.toDouble() / (1024.0 * 1024.0 * 1024.0)
                    add(String.format(Locale.US, "RAM %.1fG", gb))
                }
            }
            if (parts.isNotEmpty()) return parts.joinToString(" · ")
        }
    }
    return clean
}
