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
import androidx.compose.foundation.layout.sizeIn
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

internal val STRUCTURED_TOOL_NAMES = setOf(
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
            horizontalArrangement = Arrangement.spacedBy(6.dp),
            modifier = Modifier.weight(1f, fill = false)
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = tint,
                modifier = Modifier.size(14.dp)
            )
            Text(
                text = title,
                style = TextStyle(
                    fontSize = 11.5.sp,
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
            VesperTag(text = badge, toneColor = tint)
        }
    }
}

@Composable
private fun ManageAssetsStructuredCard(json: JSONObject) {
    val theme = BotHubThemeTokens
    val accentTint = theme.accentForeground

    if (json.has("cost_by_currency") || json.has("active_count")) {
        val activeCount = json.optInt("active_count", 0)
        val expiringCount = json.optInt("expiring_30d_count", 0)
        val inactiveCount = json.optInt("inactive_count", 0)
        val currencies = json.optJSONArray("cost_by_currency")
        val cardShape = RoundedCornerShape(12.dp)

        val monthlyParts = mutableListOf<String>()
        val annualParts = mutableListOf<String>()
        if (currencies != null) {
            for (i in 0 until currencies.length()) {
                val c = currencies.optJSONObject(i) ?: continue
                val curr = c.optString("currency", "USD").uppercase(Locale.ROOT)
                val sym = when (curr) {
                    "USD" -> "$"
                    "CNY", "RMB" -> "¥"
                    "EUR" -> "€"
                    "GBP" -> "£"
                    "HKD" -> "HK$"
                    else -> "$curr "
                }
                val monthly = c.optDouble("monthly_estimated", 0.0)
                val annual = c.optDouble("annualized_estimated", 0.0)
                if (monthly > 0.0) monthlyParts.add(String.format(Locale.US, "%s%.1f", sym, monthly).removeSuffix(".0"))
                if (annual > 0.0) annualParts.add(String.format(Locale.US, "%s%.0f", sym, annual))
            }
        }

        Column(
            modifier = Modifier
                .fillMaxWidth()
                .clip(cardShape)
                .background(theme.codeBlockBg, cardShape)
                .border(1.dp, theme.border, cardShape)
                .padding(horizontal = 12.dp, vertical = 10.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp)
        ) {
            val badgeText = buildString {
                append("在用 $activeCount")
                if (expiringCount > 0) append(" · 临期 $expiringCount")
                if (inactiveCount > 0) append(" · 停用 $inactiveCount")
            }
            StructuredSectionHeader(
                icon = Icons.Default.Inventory2,
                title = "资产费用概览",
                badge = badgeText,
                tint = accentTint
            )
            if (monthlyParts.isNotEmpty() || annualParts.isNotEmpty()) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "月均 ${monthlyParts.joinToString(" + ").ifEmpty { "--" }}",
                        fontFamily = FontFamily.Monospace,
                        fontSize = 12.5.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = theme.textPrimary
                    )
                    if (annualParts.isNotEmpty()) {
                        Text(
                            text = "年均 ${annualParts.joinToString(" + ")}",
                            fontFamily = FontFamily.Monospace,
                            fontSize = 11.5.sp,
                            color = theme.textSecondary
                        )
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
        isPerpetual -> theme.success
        daysLeft <= 3 -> theme.danger
        daysLeft <= 14 -> theme.warning
        else -> theme.success
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
            .border(1.dp, theme.border, cardShape)
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
                .padding(horizontal = 10.dp, vertical = 9.dp),
            verticalArrangement = Arrangement.spacedBy(5.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                Text(
                    text = name,
                    fontSize = 13.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = theme.textPrimary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f)
                )
                if (price.isNotEmpty()) {
                    VesperTag(text = price.replace(" / ", "/"))
                }
                if (isInactive || status == "已到期") {
                    VesperTag(text = status, toneColor = indicatorColor)
                }
            }

            if (spec.isNotEmpty()) {
                Text(
                    text = spec,
                    fontSize = 11.5.sp,
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
                    fontSize = 11.sp,
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
                    fontSize = 11.sp,
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
            tint = theme.success
        )

        nodes.take(MAX_INLINE_ITEMS).forEach { n ->
            val name = n.optString("name", "").ifBlank { n.optString("id", "节点") }
            val status = n.optString("status", "online").lowercase()
            val isOnline = status == "online" || status == "生效中"
            val dotColor = if (isOnline) theme.success else theme.warning
            val rawHostInfo = n.optString("host_info", "").trim()
            val hostHighlight = remember(rawHostInfo) { formatHostInfoSummary(rawHostInfo) }
            val lastActive = formatCompactTime(n.optString("last_active", ""))

            val cardShape = RoundedCornerShape(11.dp)
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(cardShape)
                    .background(theme.codeBlockBg, cardShape)
                    .border(1.dp, theme.border, cardShape)
                    .padding(horizontal = 11.dp, vertical = 9.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(7.dp)
                ) {
                    Box(
                        modifier = Modifier
                            .size(7.dp)
                            .background(dotColor, CircleShape)
                    )
                    Text(
                        text = name,
                        fontSize = 13.sp,
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
                            fontSize = 11.sp,
                            color = theme.textMuted,
                            maxLines = 1,
                            softWrap = false
                        )
                    }
                }
                if (hostHighlight.isNotEmpty()) {
                    Text(
                        text = hostHighlight,
                        fontSize = 11.5.sp,
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

    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        StructuredSectionHeader(
            icon = Icons.Default.TaskAlt,
            title = "待办事项",
            badge = "${items.size}项",
            tint = theme.info
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
                "high" -> theme.danger
                "medium" -> theme.warning
                else -> theme.textMuted
            }
            val circleColor by animateColorAsState(
                targetValue = if (isDone) theme.success else theme.borderStrong,
                animationSpec = tween(150),
                label = "todoCheckCircle"
            )

            val cardShape = RoundedCornerShape(11.dp)
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .sizeIn(minHeight = 48.dp)
                    .clip(cardShape)
                    .background(theme.codeBlockBg, cardShape)
                    .border(1.dp, theme.border, cardShape)
                    .clickable(enabled = id.isNotEmpty()) {
                        haptics.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                        val nextStatus = if (isDone) "todo" else "done"
                        optimisticStatus[id] = nextStatus
                        onToggleTodo(id, nextStatus)
                    }
                    .padding(horizontal = 11.dp, vertical = 9.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(9.dp)
            ) {
                Box(
                    modifier = Modifier
                        .size(20.dp)
                        .clip(CircleShape)
                        .background(if (isDone) theme.success else Color.Transparent)
                        .border(1.5.dp, circleColor, CircleShape),
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
                    fontSize = 13.sp,
                    fontWeight = FontWeight.Medium,
                    color = if (isDone) theme.textMuted else theme.textPrimary,
                    textDecoration = if (isDone) TextDecoration.LineThrough else TextDecoration.None,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f)
                )

                if (dueAt.isNotEmpty()) {
                    VesperTag(
                        text = dueAt,
                        bgColor = theme.surfaceCard,
                        borderColor = theme.border,
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

    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        StructuredSectionHeader(
            icon = Icons.Default.Schedule,
            title = "定时与调度任务",
            badge = "${items.size}项",
            tint = theme.accentForeground
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
                isRunning -> theme.accentForeground
                enabled -> theme.success
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
                    .border(1.dp, theme.border, cardShape)
                    .padding(horizontal = 11.dp, vertical = 9.dp),
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
                        fontSize = 13.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = theme.textPrimary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f)
                    )
                    VesperTag(text = stateLabel, toneColor = stateColor)
                }
                if (scheduleLabel.isNotEmpty()) {
                    Text(
                        text = scheduleLabel,
                        fontFamily = FontFamily.Monospace,
                        fontSize = 11.sp,
                        color = theme.textSecondary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
                if (lastSummary.isNotEmpty()) {
                    Text(
                        text = lastSummary,
                        fontSize = 11.5.sp,
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

    val cardShape = RoundedCornerShape(12.dp)
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clip(cardShape)
            .background(theme.codeBlockBg, cardShape)
            .border(1.dp, theme.border, cardShape)
            .padding(11.dp),
        verticalArrangement = Arrangement.spacedBy(9.dp)
    ) {
        StructuredSectionHeader(
            icon = Icons.Default.Dashboard,
            title = "今日工作简报",
            badge = listOf(today, weekday).filter { it.isNotEmpty() }.joinToString(" "),
            tint = theme.accentForeground
        )
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(6.dp)
        ) {
            BriefingKpiChip(
                label = "逾期",
                value = overdueCount.toString(),
                accent = if (overdueCount > 0) theme.danger else theme.textMuted,
                modifier = Modifier.weight(1f)
            )
            BriefingKpiChip(
                label = "今日待办",
                value = dueTodayCount.toString(),
                accent = theme.info,
                modifier = Modifier.weight(1f)
            )
            BriefingKpiChip(
                label = "临期资产",
                value = expiringCount.toString(),
                accent = if (expiringCount > 0) theme.warning else theme.textMuted,
                modifier = Modifier.weight(1f)
            )
            BriefingKpiChip(
                label = "未读通知",
                value = unreadCount.toString(),
                accent = if (unreadCount > 0) theme.accentForeground else theme.textMuted,
                modifier = Modifier.weight(1f)
            )
        }
        if (hints != null && hints.length() > 0) {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                val limit = minOf(hints.length(), 2)
                for (i in 0 until limit) {
                    val hint = hints.optString(i, "").trim()
                    if (hint.isNotEmpty()) {
                        Text(
                            text = hint,
                            fontSize = 11.5.sp,
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
            .background(theme.surfaceCard, shape)
            .border(0.75.dp, theme.border, shape)
            .padding(horizontal = 6.dp, vertical = 7.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(2.dp)
    ) {
        Text(
            text = value,
            style = TextStyle(
                fontSize = 14.5.sp,
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
                fontSize = 10.5.sp,
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
        fontSize = 11.sp,
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
