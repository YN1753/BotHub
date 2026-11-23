package uk.starryheavens.vesper.bothub.ui.screens

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
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.CalendarMonth
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
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
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.json.JSONObject
import uk.starryheavens.vesper.bothub.data.AssetCategoryItem
import uk.starryheavens.vesper.bothub.data.AssetItem
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.DashboardSnapshot
import uk.starryheavens.vesper.bothub.data.ModelOptionItem
import uk.starryheavens.vesper.bothub.data.TaskItem
import uk.starryheavens.vesper.bothub.data.TodoItemUi
import uk.starryheavens.vesper.bothub.data.WorkNoteItemUi
import uk.starryheavens.vesper.bothub.ui.components.CrispTagPill
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSegmentedControl
import uk.starryheavens.vesper.bothub.ui.components.TactileToggle
import uk.starryheavens.vesper.bothub.ui.components.ThemeCalendarDialog
import uk.starryheavens.vesper.bothub.ui.components.assetCategoryIconVector
import uk.starryheavens.vesper.bothub.ui.components.tactileClickable
import uk.starryheavens.vesper.bothub.ui.sheets.ModelPickerSheet
import uk.starryheavens.vesper.bothub.ui.sheets.resolveModelOptionDisplayName
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens
import java.time.LocalDate
import java.util.Locale
import kotlin.math.roundToInt

private val LEGACY_CATEGORY_META = mapOf(
    "general" to ("通用资产" to "tag"),
    "other" to ("其他资产" to "tag"),
    "compute" to ("算力节点" to "server"),
    "domain" to ("域名资产" to "globe"),
    "life" to ("订阅服务" to "sparkles"),
    "subscription" to ("会员订阅" to "sparkles"),
    "license" to ("软件授权" to "key"),
    "hardware" to ("硬件设备" to "package"),
    "account" to ("账号凭证" to "shield"),
    "finance" to ("账单合约" to "wallet")
)

private val OPTIONAL_SUGGESTED_CATEGORIES = listOf(
    AssetCategoryItem("general", "通用资产", "tag"),
    AssetCategoryItem("compute", "算力服务", "server"),
    AssetCategoryItem("domain", "域名证书", "globe"),
    AssetCategoryItem("subscription", "会员订阅", "sparkles"),
    AssetCategoryItem("license", "软件授权", "key"),
    AssetCategoryItem("hardware", "硬件设备", "package"),
    AssetCategoryItem("account", "账号凭证", "shield"),
    AssetCategoryItem("finance", "账单合约", "wallet")
)

private val CATEGORY_ICON_CHOICES = listOf(
    "tag" to "标签",
    "server" to "算力",
    "globe" to "域名",
    "sparkles" to "订阅",
    "key" to "授权",
    "shield" to "安全",
    "package" to "硬件",
    "wallet" to "账单"
)

private val ASSET_PRICE_REGEX = Regex("""(HK\$|NT\$|S\$|C\$|A\$|US\$|CN¥|RMB|USD|CNY|EUR|GBP|HKD|JPY|TWD|SGD|CAD|AUD|[¥$€£])?\s*([0-9]+(?:\.[0-9]+)?)""", RegexOption.IGNORE_CASE)

private val DEFAULT_FX_RATES_TO_CNY = mapOf(
    "CNY" to 1.0,
    "USD" to 7.24,
    "EUR" to 7.82,
    "HKD" to 0.93,
    "GBP" to 9.18,
    "JPY" to 0.0475,
    "TWD" to 0.224,
    "SGD" to 5.38,
    "CAD" to 5.16,
    "AUD" to 4.70
)

private fun detectPriceCurrencyCode(rawPrice: String, matchedSymbol: String): String {
    val upper = "${matchedSymbol.trim()} ${rawPrice.trim()}".uppercase(Locale.ROOT)
    return when {
        upper.contains("HK$") || upper.contains("HKD") || upper.contains("港") -> "HKD"
        upper.contains("NT$") || upper.contains("TWD") || upper.contains("台") -> "TWD"
        upper.contains("S$") || upper.contains("SGD") -> "SGD"
        upper.contains("C$") || upper.contains("CAD") -> "CAD"
        upper.contains("A$") || upper.contains("AUD") -> "AUD"
        upper.contains("JPY") || upper.contains("日元") || upper.contains("円") -> "JPY"
        upper.contains("€") || upper.contains("EUR") || upper.contains("欧") -> "EUR"
        upper.contains("£") || upper.contains("GBP") || upper.contains("英镑") -> "GBP"
        upper.contains("$") || upper.contains("USD") || upper.contains("刀") || upper.contains("美元") -> "USD"
        else -> "CNY"
    }
}

private fun rateToCNY(currencyCode: String, fxRates: Map<String, Double>): Double {
    val code = currencyCode.trim().uppercase(Locale.ROOT)
    if (code.isEmpty() || code == "CNY" || code == "RMB") return 1.0
    val live = fxRates[code]
    if (live != null && live > 0.0 && !live.isNaN()) return live
    return DEFAULT_FX_RATES_TO_CNY[code] ?: 1.0
}

private fun summarizeAssetsCost(
    assets: List<AssetItem>,
    yearly: Boolean,
    fxRates: Map<String, Double> = emptyMap()
): String {
    var totalCNY = 0.0
    var hasPaidAsset = false
    assets.forEach { a ->
        if (a.status == "已停用" || a.status == "已归档") return@forEach
        val raw = a.price.trim()
        if (raw.isEmpty()) return@forEach
        val match = ASSET_PRICE_REGEX.find(raw) ?: return@forEach
        val symbol = match.groupValues[1]
        val amount = match.groupValues[2].toDoubleOrNull() ?: return@forEach
        if (amount <= 0.0) return@forEach
        val lower = raw.lowercase(Locale.ROOT)
        val normalized = when {
            lower.contains("年") || lower.contains("yr") || lower.contains("year") ->
                if (yearly) amount else amount / 12.0
            lower.contains("季") || lower.contains("quarter") ->
                if (yearly) amount * 4.0 else amount / 3.0
            else ->
                if (yearly) amount * 12.0 else amount
        }
        val code = detectPriceCurrencyCode(raw, symbol)
        totalCNY += normalized * rateToCNY(code, fxRates)
        hasPaidAsset = true
    }
    if (!hasPaidAsset || totalCNY <= 0.0) return "--"
    return if (yearly) {
        "¥${totalCNY.roundToInt()}"
    } else {
        val rounded = (totalCNY * 10.0).roundToInt() / 10.0
        val numStr = if (rounded % 1.0 == 0.0) rounded.toInt().toString() else rounded.toString()
        "¥$numStr"
    }
}

private fun resolveAssetCategory(
    catId: String,
    saved: List<AssetCategoryItem>
): AssetCategoryItem {
    val clean = catId.trim().ifEmpty { "general" }
    val lower = clean.lowercase(Locale.ROOT)
    saved.firstOrNull { it.id.equals(clean, ignoreCase = true) }?.let { return it }
    LEGACY_CATEGORY_META[lower]?.let { (label, icon) ->
        return AssetCategoryItem(id = clean, label = label, icon = icon)
    }
    return AssetCategoryItem(id = clean, label = clean, icon = "tag")
}

private fun mergeExtensibleAssetCategories(
    saved: List<AssetCategoryItem>,
    assets: List<AssetItem>
): List<AssetCategoryItem> {
    val result = mutableListOf<AssetCategoryItem>()
    val seen = mutableSetOf<String>()
    fun push(item: AssetCategoryItem) {
        val cleanId = item.id.trim()
        if (cleanId.isEmpty()) return
        val key = cleanId.lowercase(Locale.ROOT)
        if (seen.add(key)) {
            result.add(
                item.copy(
                    id = cleanId,
                    label = item.label.trim().ifEmpty { cleanId },
                    icon = item.icon.ifBlank { LEGACY_CATEGORY_META[key]?.second ?: "tag" }
                )
            )
        }
    }
    saved.forEach(::push)
    assets.forEach { a ->
        val raw = a.category.trim()
        if (raw.isNotEmpty()) push(resolveAssetCategory(raw, saved))
    }
    return result
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun HubTabScreen(
    snapshot: DashboardSnapshot?,
    bots: List<BotItem> = emptyList(),
    modelOptions: List<ModelOptionItem> = emptyList(),
    onRunTask: (TaskItem) -> Unit,
    onToggleTask: (TaskItem, Boolean) -> Unit,
    onSyncAssets: () -> Unit,
    onSaveAsset: (
        existingId: String?,
        name: String,
        category: String,
        spec: String,
        price: String,
        dueDate: String,
        autoRenew: Boolean
    ) -> Unit,
    onQuickRenewAsset: (AssetItem, Int) -> Unit = { _, _ -> },
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
        allowPaidFallback: Boolean,
        maxPaidCallsPerDay: Int,
        enabled: Boolean,
        params: String
    ) -> Unit = { _, _, _, _, _, _, _, _, _, _, _, _ -> },
    onDeleteTask: (String) -> Unit = {},
    onOptimizeTaskPrompt: (name: String, rawPrompt: String, onResult: (String?) -> Unit) -> Unit = { _, _, cb -> cb(null) },
    onCreateTodo: (title: String, notes: String, dueAt: String, priority: String) -> Unit = { _, _, _, _ -> },
    onToggleTodoDone: (TodoItemUi) -> Unit = {},
    onUpdateTodo: (id: String, title: String, notes: String, dueAt: String, priority: String) -> Unit = { _, _, _, _, _ -> },
    onDeleteTodo: (String) -> Unit = {},
    onClearDoneTodos: () -> Unit = {},
    onSaveNote: (existingId: String?, title: String, content: String, category: String, pinned: Boolean) -> Unit = { _, _, _, _, _ -> },
    onDeleteNote: (String) -> Unit = {},
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
    val pendingTodosCount = openTodos.size

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
    var selectedCategoryFilter by remember { mutableStateOf("all") }
    var showDoneTodos by remember { mutableStateOf(false) }

    var showAssetSheet by remember { mutableStateOf(false) }
    var editingAsset by remember { mutableStateOf<AssetItem?>(null) }

    var showTaskSheet by remember { mutableStateOf(false) }
    var editingTask by remember { mutableStateOf<TaskItem?>(null) }

    var showTodoSheet by remember { mutableStateOf(false) }
    var editingTodo by remember { mutableStateOf<TodoItemUi?>(null) }

    var showNoteSheet by remember { mutableStateOf(false) }
    var editingNote by remember { mutableStateOf<WorkNoteItemUi?>(null) }

    val filteredAssets = remember(assets, selectedCategoryFilter) {
        if (selectedCategoryFilter == "all") {
            assets
        } else {
            assets.filter { it.category.equals(selectedCategoryFilter, ignoreCase = true) }
        }
    }

    val fxRates = snapshot?.fxRates ?: emptyMap()
    val totalMonthlyCost = remember(filteredAssets, fxRates) { summarizeAssetsCost(filteredAssets, yearly = false, fxRates = fxRates) }
    val totalYearlyCost = remember(filteredAssets, fxRates) { summarizeAssetsCost(filteredAssets, yearly = true, fxRates = fxRates) }
    val categoryCounts = remember(assets) {
        assets.groupingBy { it.category.trim().lowercase(Locale.ROOT) }.eachCount()
    }

    val segmentOptions = listOf(
        if (assets.isNotEmpty()) "资产 ${assets.size}" else "资产",
        if (tasks.isNotEmpty()) "任务 ${tasks.size}" else "任务",
        if (pendingTodosCount > 0) "待办与手记 $pendingTodosCount" else "待办与手记"
    )

    Column(modifier = modifier) {
        // Dynamic Today Focus Strip: appears ONLY when there are <=14d expiring assets or today/overdue/high-priority todos
        if (focusAssets.isNotEmpty() || focusTodos.isNotEmpty()) {
            val totalFocusCount = focusAssets.size + focusTodos.size
            SpecularGlassCard(
                cornerRadius = 14.dp,
                accentTint = if (focusAssets.any { it.daysLeft <= 3 } || focusTodos.any { it.dueAt.take(10).let { d -> d.length == 10 && d < todayPrefix } }) {
                    Color(0xFFF43F5E)
                } else {
                    Color(0xFFF59E0B)
                },
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 2.dp)
            ) {
                Column(
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "今日焦点",
                            fontSize = 11.5.sp,
                            fontWeight = FontWeight.Bold,
                            color = Color(0xFFD97706)
                        )
                        Text(
                            text = "待处理 $totalFocusCount 项",
                            fontFamily = FontFamily.Monospace,
                            fontSize = 11.sp,
                            color = theme.textMuted
                        )
                    }

                    focusAssets.take(3).forEach { asset ->
                        val recMonths = remember(asset.price) { asset.inferRecommendedRenewMonths() }
                        val recLabel = when (recMonths) {
                            12 -> "+1年"
                            3 -> "+1季"
                            else -> "+1月"
                        }
                        val urgentColor = if (asset.daysLeft <= 3) Color(0xFFF43F5E) else Color(0xFFD97706)
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clip(RoundedCornerShape(9.dp))
                                .background(theme.inputTop.copy(alpha = 0.65f))
                                .clickable {
                                    editingAsset = asset
                                    showAssetSheet = true
                                }
                                .padding(horizontal = 9.dp, vertical = 6.dp),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Row(
                                modifier = Modifier.weight(1f).padding(end = 8.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                Text(
                                    text = asset.name,
                                    fontSize = 12.5.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = theme.textPrimary,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                    modifier = Modifier.weight(1f, fill = false)
                                )
                                Text(
                                    text = listOfNotNull(
                                        asset.dueBadgeLabel,
                                        asset.price.takeIf { it.isNotBlank() }?.replace(" / ", "/")
                                    ).joinToString(" · "),
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 11.sp,
                                    color = urgentColor,
                                    maxLines = 1,
                                    softWrap = false
                                )
                            }
                            Box(
                                modifier = Modifier
                                    .clip(RoundedCornerShape(7.dp))
                                    .background(theme.accent.copy(alpha = 0.16f))
                                    .border(0.8.dp, theme.accent.copy(alpha = 0.45f), RoundedCornerShape(7.dp))
                                    .tactileClickable(pressedScale = 0.94f, hapticOnTap = true) {
                                        onQuickRenewAsset(asset, recMonths)
                                    }
                                    .padding(horizontal = 8.dp, vertical = 3.dp)
                            ) {
                                Text(
                                    text = "续期$recLabel",
                                    fontSize = 10.5.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = if (theme.isLight) theme.accentDeep else theme.accentLight
                                )
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
                        val tagColor = if (tagText == "已逾期" || tagText == "高优") Color(0xFFF43F5E) else Color(0xFFD97706)
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clip(RoundedCornerShape(9.dp))
                                .background(theme.inputTop.copy(alpha = 0.65f))
                                .clickable {
                                    editingTodo = todo
                                    showTodoSheet = true
                                }
                                .padding(horizontal = 9.dp, vertical = 6.dp),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Row(
                                modifier = Modifier.weight(1f).padding(end = 8.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp)
                            ) {
                                Box(
                                    modifier = Modifier
                                        .size(18.dp)
                                        .clip(RoundedCornerShape(5.dp))
                                        .background(theme.sheetBg)
                                        .border(1.2.dp, theme.accent.copy(alpha = 0.65f), RoundedCornerShape(5.dp))
                                        .tactileClickable(pressedScale = 0.9f, hapticOnTap = true) {
                                            onToggleTodoDone(todo)
                                        }
                                )
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
                            CrispTagPill(
                                text = tagText,
                                bgColor = tagColor.copy(alpha = 0.14f),
                                borderColor = tagColor.copy(alpha = 0.4f),
                                textColor = tagColor
                            )
                        }
                    }
                }
            }
        }

        TactileSegmentedControl(
            options = segmentOptions,
            selectedIndex = selectedSegment,
            onSelect = { selectedSegment = it },
            modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 4.dp)
        )

        LazyColumn(
            modifier = Modifier.fillMaxWidth().weight(1f),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            when (selectedSegment) {
                0 -> {
                    item(key = "__hub_assets_hdr__") {
                        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            // Unified single-card summary header: merges cost total, status badge, and add/sync actions
                            SpecularGlassCard(
                                cornerRadius = 14.dp,
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .padding(horizontal = 12.dp, vertical = 9.dp),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Column(
                                        modifier = Modifier.weight(1f).padding(end = 8.dp),
                                        verticalArrangement = Arrangement.spacedBy(3.dp)
                                    ) {
                                        Row(
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(6.dp)
                                        ) {
                                            Text(
                                                text = if (totalMonthlyCost != "--") "月均 $totalMonthlyCost" else "共 ${filteredAssets.size} 项资产",
                                                fontSize = 13.5.sp,
                                                fontWeight = FontWeight.Bold,
                                                fontFamily = if (totalMonthlyCost != "--") FontFamily.Monospace else FontFamily.Default,
                                                color = theme.textPrimary,
                                                maxLines = 1,
                                                overflow = TextOverflow.Ellipsis
                                            )
                                            if (expiringCount > 0) {
                                                CrispTagPill(
                                                    text = "${expiringCount}项临期",
                                                    bgColor = Color(0x26F59E0B),
                                                    borderColor = Color(0x52F59E0B),
                                                    textColor = Color(0xFFD97706)
                                                )
                                            }
                                        }
                                        Text(
                                            text = if (totalYearlyCost != "--") {
                                                "年化 $totalYearlyCost · 当前显示 ${filteredAssets.size} 项"
                                            } else {
                                                if (expiringCount > 0) "请留意近期到期项目" else "全部资产状态正常"
                                            },
                                            fontSize = 11.sp,
                                            fontFamily = FontFamily.Monospace,
                                            color = theme.textMuted,
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis
                                        )
                                    }

                                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                        TactileSecondaryButton(
                                            text = "添加",
                                            leadingIcon = Icons.Default.Add,
                                            cornerRadius = 999.dp,
                                            minHeight = 31.dp,
                                            contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
                                            onClick = {
                                                editingAsset = null
                                                showAssetSheet = true
                                            }
                                        )
                                        TactileSecondaryButton(
                                            text = "同步",
                                            leadingIcon = Icons.Default.Refresh,
                                            accent = true,
                                            cornerRadius = 999.dp,
                                            minHeight = 31.dp,
                                            contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
                                            onClick = onSyncAssets
                                        )
                                    }
                                }
                            }

                            if (mergedCategories.size > 1) {
                                LazyRow(
                                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                                ) {
                                    item(key = "__cat_all__") {
                                        TactileSecondaryButton(
                                            text = "全部",
                                            accent = selectedCategoryFilter == "all",
                                            cornerRadius = 999.dp,
                                            minHeight = 29.dp,
                                            contentPadding = PaddingValues(horizontal = 11.dp, vertical = 4.dp),
                                            onClick = { selectedCategoryFilter = "all" }
                                        )
                                    }
                                    items(mergedCategories, key = { it.id }) { cat ->
                                        val cnt = categoryCounts[cat.id.lowercase(Locale.ROOT)] ?: 0
                                        TactileSecondaryButton(
                                            text = if (cnt > 0) "${cat.label} $cnt" else cat.label,
                                            leadingIcon = assetCategoryIconVector(cat.icon),
                                            accent = selectedCategoryFilter.equals(cat.id, ignoreCase = true),
                                            cornerRadius = 999.dp,
                                            minHeight = 29.dp,
                                            contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
                                            onClick = {
                                                selectedCategoryFilter = if (selectedCategoryFilter.equals(cat.id, ignoreCase = true)) {
                                                    "all"
                                                } else {
                                                    cat.id
                                                }
                                            }
                                        )
                                    }
                                }
                            }
                        }
                    }

                    if (filteredAssets.isEmpty()) {
                        item(key = "__hub_assets_empty__") {
                            SpecularGlassCard(
                                cornerRadius = 16.dp,
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Text(
                                    text = if (assets.isEmpty()) {
                                        "暂无资产记录，点击右上角「添加」即可自由录入与扩展分类"
                                    } else {
                                        "当前分类下暂无资产记录"
                                    },
                                    fontSize = 13.sp,
                                    color = theme.textMuted,
                                    modifier = Modifier.padding(16.dp)
                                )
                            }
                        }
                    }

                    items(filteredAssets, key = { it.id }) { asset ->
                        val catMeta = remember(asset.category, savedCategories) {
                            resolveAssetCategory(asset.category, savedCategories)
                        }
                        val isPermanent = asset.dueDate.startsWith("2099") || asset.daysLeft > 3650
                        val isMultiYear = !isPermanent && asset.daysLeft > 365
                        val barColor = when {
                            isPermanent || isMultiYear -> Color(0xFF10B981)
                            asset.daysLeft <= 7 -> Color(0xFFF43F5E)
                            asset.daysLeft <= 30 -> Color(0xFFF59E0B)
                            else -> theme.accent
                        }
                        val progressFraction = when {
                            isPermanent -> 1f
                            asset.daysLeft <= 0 -> 0.04f
                            else -> (asset.daysLeft.toFloat() / 365f).coerceIn(0.06f, 1f)
                        }
                        val recMonths = remember(asset.price) { asset.inferRecommendedRenewMonths() }
                        val recLabel = when (recMonths) {
                            12 -> "+1年"
                            3 -> "+1季"
                            else -> "+1月"
                        }
                        val showQuickRenew = !isPermanent && asset.daysLeft <= 30

                        SpecularGlassCard(
                            cornerRadius = 15.dp,
                            accentTint = if (asset.isExpiringSoon && !isPermanent) barColor else null,
                            onClick = {
                                editingAsset = asset
                                showAssetSheet = true
                            },
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            Column(
                                modifier = Modifier.padding(horizontal = 13.dp, vertical = 10.dp),
                                verticalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                // Row 1: Icon + Name + Price + Due Days Pill
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Row(
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                                        modifier = Modifier.weight(1f).padding(end = 8.dp)
                                    ) {
                                        Box(
                                            modifier = Modifier
                                                .size(26.dp)
                                                .clip(RoundedCornerShape(8.dp))
                                                .background(theme.accent.copy(alpha = 0.14f))
                                                .border(0.8.dp, theme.accent.copy(alpha = 0.3f), RoundedCornerShape(8.dp)),
                                            contentAlignment = Alignment.Center
                                        ) {
                                            Icon(
                                                imageVector = assetCategoryIconVector(catMeta.icon),
                                                contentDescription = null,
                                                tint = theme.accent,
                                                modifier = Modifier.size(14.dp)
                                            )
                                        }
                                        Text(
                                            text = asset.name,
                                            fontSize = 14.5.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            color = theme.textPrimary,
                                            maxLines = 1,
                                            softWrap = false,
                                            overflow = TextOverflow.Ellipsis,
                                            modifier = Modifier.weight(1f, fill = false)
                                        )
                                    }

                                    Row(
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(6.dp)
                                    ) {
                                        if (asset.price.isNotBlank()) {
                                            Text(
                                                text = asset.price.replace(" / ", "/"),
                                                fontFamily = FontFamily.Monospace,
                                                fontSize = 12.sp,
                                                fontWeight = FontWeight.SemiBold,
                                                color = theme.textSecondary,
                                                maxLines = 1,
                                                softWrap = false
                                            )
                                        }
                                        CrispTagPill(
                                            text = if (isPermanent) "长期" else asset.dueBadgeLabel,
                                            bgColor = barColor.copy(alpha = 0.14f),
                                            borderColor = barColor.copy(alpha = 0.42f),
                                            textColor = barColor
                                        )
                                    }
                                }

                                // Row 2: Spec (or category fallback) on left + Due Date & expiring quick-renew chip on right
                                val subLeftText = when {
                                    asset.spec.isNotBlank() && selectedCategoryFilter == "all" -> "${catMeta.label} · ${asset.spec}"
                                    asset.spec.isNotBlank() -> asset.spec
                                    else -> catMeta.label
                                }
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Text(
                                        text = subLeftText,
                                        fontSize = 11.5.sp,
                                        color = theme.textMuted,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        modifier = Modifier
                                            .weight(1f)
                                            .padding(end = 10.dp)
                                    )
                                    Row(
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(6.dp)
                                    ) {
                                        if (!isPermanent && asset.dueDate.isNotEmpty()) {
                                            Text(
                                                text = asset.dueDate.take(10),
                                                fontFamily = FontFamily.Monospace,
                                                fontSize = 11.sp,
                                                color = theme.textMuted,
                                                maxLines = 1,
                                                softWrap = false
                                            )
                                        }
                                        if (showQuickRenew) {
                                            Box(
                                                modifier = Modifier
                                                    .clip(RoundedCornerShape(7.dp))
                                                    .background(theme.accent.copy(alpha = 0.16f))
                                                    .border(0.8.dp, theme.accent.copy(alpha = 0.48f), RoundedCornerShape(7.dp))
                                                    .tactileClickable(pressedScale = 0.94f, hapticOnTap = true) {
                                                        onQuickRenewAsset(asset, recMonths)
                                                    }
                                                    .padding(horizontal = 7.dp, vertical = 2.dp)
                                            ) {
                                                Text(
                                                    text = "续期$recLabel",
                                                    fontSize = 10.5.sp,
                                                    fontWeight = FontWeight.SemiBold,
                                                    color = if (theme.isLight) theme.accentDeep else theme.accentLight
                                                )
                                            }
                                        }
                                    }
                                }

                                Box(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .height(2.5.dp)
                                        .clip(RoundedCornerShape(999.dp))
                                        .background(theme.inputTop)
                                ) {
                                    Box(
                                        modifier = Modifier
                                            .fillMaxWidth(progressFraction)
                                            .height(2.5.dp)
                                            .clip(RoundedCornerShape(999.dp))
                                            .background(
                                                Brush.horizontalGradient(
                                                    listOf(barColor.copy(alpha = 0.72f), barColor)
                                                )
                                            )
                                    )
                                }
                            }
                        }
                    }
                }

                1 -> {
                    item(key = "__hub_tasks_hdr__") {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = "已启用 $activeTasksCount / ${tasks.size} 项例程",
                                fontSize = 12.sp,
                                color = theme.textMuted
                            )
                            TactileSecondaryButton(
                                text = "新建任务",
                                leadingIcon = Icons.Default.Add,
                                accent = true,
                                cornerRadius = 999.dp,
                                minHeight = 31.dp,
                                contentPadding = PaddingValues(horizontal = 11.dp, vertical = 4.dp),
                                onClick = {
                                    editingTask = null
                                    showTaskSheet = true
                                }
                            )
                        }
                    }

                    if (tasks.isEmpty()) {
                        item(key = "__hub_tasks_empty__") {
                            SpecularGlassCard(
                                cornerRadius = 16.dp,
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Text(
                                    text = "暂无定时任务，点击右上角「新建任务」即可在手机端直接创建定时巡检或提醒",
                                    fontSize = 13.sp,
                                    color = theme.textMuted,
                                    modifier = Modifier.padding(16.dp)
                                )
                            }
                        }
                    }

                    items(tasks, key = { it.id }) { task ->
                        SpecularGlassCard(
                            cornerRadius = 15.dp,
                            onClick = {
                                editingTask = task
                                showTaskSheet = true
                            },
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(horizontal = 13.dp, vertical = 10.dp),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Column(
                                    modifier = Modifier.weight(1f).padding(end = 10.dp),
                                    verticalArrangement = Arrangement.spacedBy(3.dp)
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
                                        if (task.allowPaidFallback) {
                                            CrispTagPill(
                                                text = if (task.maxPaidCallsPerDay > 0) "付费<=${task.maxPaidCallsPerDay}/日" else "允许付费",
                                                bgColor = Color(0x24F59E0B),
                                                borderColor = Color(0x4DF59E0B),
                                                textColor = Color(0xFFD97706)
                                            )
                                        }
                                    }
                                    Text(
                                        text = task.formattedSchedule(),
                                        fontFamily = FontFamily.Monospace,
                                        fontSize = 11.5.sp,
                                        color = theme.textMuted,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis
                                    )
                                }

                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    Box(
                                        modifier = Modifier
                                            .size(30.dp)
                                            .clip(RoundedCornerShape(9.dp))
                                            .background(theme.accent.copy(alpha = 0.14f))
                                            .border(0.8.dp, theme.accent.copy(alpha = 0.35f), RoundedCornerShape(9.dp))
                                            .tactileClickable(pressedScale = 0.92f, hapticOnTap = true) {
                                                onRunTask(task)
                                            },
                                        contentAlignment = Alignment.Center
                                    ) {
                                        Icon(
                                            imageVector = Icons.Default.PlayArrow,
                                            contentDescription = "立即运行",
                                            tint = if (theme.isLight) theme.accentDeep else theme.accentLight,
                                            modifier = Modifier.size(16.dp)
                                        )
                                    }
                                    TactileToggle(
                                        checked = task.enabled,
                                        onCheckedChange = { checked -> onToggleTask(task, checked) }
                                    )
                                }
                            }
                        }
                    }
                }

                else -> {
                    // Section 1: Open Todos (+ collapsible Done Todos)
                    item(key = "__hub_todos_hdr__") {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = if (openTodos.isNotEmpty()) "待办清单 · ${openTodos.size} 项进行中" else "待办清单",
                                fontSize = 12.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textSecondary
                            )
                            TactileSecondaryButton(
                                text = "新建待办",
                                leadingIcon = Icons.Default.Add,
                                accent = true,
                                cornerRadius = 999.dp,
                                minHeight = 30.dp,
                                contentPadding = PaddingValues(horizontal = 11.dp, vertical = 4.dp),
                                onClick = {
                                    editingTodo = null
                                    showTodoSheet = true
                                }
                            )
                        }
                    }

                    if (openTodos.isEmpty() && doneTodos.isEmpty()) {
                        item(key = "__hub_todos_empty__") {
                            SpecularGlassCard(
                                cornerRadius = 15.dp,
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Text(
                                    text = "暂无待办事项，点击「新建待办」或让 AI 助理帮你记录",
                                    fontSize = 12.5.sp,
                                    color = theme.textMuted,
                                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp)
                                )
                            }
                        }
                    } else if (openTodos.isEmpty()) {
                        item(key = "__hub_todos_all_done__") {
                            SpecularGlassCard(
                                cornerRadius = 15.dp,
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Text(
                                    text = "当前待办已全部完成",
                                    fontSize = 12.5.sp,
                                    color = theme.textMuted,
                                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 11.dp)
                                )
                            }
                        }
                    }

                    items(openTodos, key = { "open_${it.id}" }) { todo ->
                        val isHigh = todo.priority.equals("high", ignoreCase = true)
                        SpecularGlassCard(
                            cornerRadius = 15.dp,
                            onClick = {
                                editingTodo = todo
                                showTodoSheet = true
                            },
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(horizontal = 13.dp, vertical = 10.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(10.dp)
                            ) {
                                Box(
                                    modifier = Modifier
                                        .size(21.dp)
                                        .clip(RoundedCornerShape(6.dp))
                                        .background(theme.inputTop)
                                        .border(1.2.dp, theme.accent.copy(alpha = 0.5f), RoundedCornerShape(6.dp))
                                        .tactileClickable(pressedScale = 0.9f, hapticOnTap = true) {
                                            onToggleTodoDone(todo)
                                        }
                                )

                                Column(
                                    modifier = Modifier.weight(1f),
                                    verticalArrangement = Arrangement.spacedBy(2.dp)
                                ) {
                                    Text(
                                        text = todo.title,
                                        fontSize = 14.sp,
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
                                    CrispTagPill(
                                        text = "高优",
                                        bgColor = Color(0x26F43F5E),
                                        borderColor = Color(0x52F43F5E),
                                        textColor = Color(0xFFF43F5E)
                                    )
                                }
                            }
                        }
                    }

                    if (doneTodos.isNotEmpty()) {
                        item(key = "__hub_done_todos_toggle__") {
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(top = 2.dp),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Row(
                                    modifier = Modifier
                                        .clip(RoundedCornerShape(8.dp))
                                        .clickable { showDoneTodos = !showDoneTodos }
                                        .padding(horizontal = 4.dp, vertical = 4.dp),
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
                                Text(
                                    text = "清理已完成",
                                    fontSize = 12.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = theme.textSecondary,
                                    modifier = Modifier
                                        .clip(RoundedCornerShape(8.dp))
                                        .clickable {
                                            showDoneTodos = false
                                            onClearDoneTodos()
                                        }
                                        .padding(horizontal = 8.dp, vertical = 4.dp)
                                )
                            }
                        }

                        if (showDoneTodos) {
                            items(doneTodos, key = { "done_${it.id}" }) { todo ->
                                SpecularGlassCard(
                                    cornerRadius = 14.dp,
                                    onClick = {
                                        editingTodo = todo
                                        showTodoSheet = true
                                    },
                                    modifier = Modifier.fillMaxWidth()
                                ) {
                                    Row(
                                        modifier = Modifier
                                            .fillMaxWidth()
                                            .padding(horizontal = 13.dp, vertical = 9.dp),
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                                    ) {
                                        Box(
                                            modifier = Modifier
                                                .size(20.dp)
                                                .clip(RoundedCornerShape(6.dp))
                                                .background(Color(0xFF10B981).copy(alpha = 0.22f))
                                                .border(1.2.dp, Color(0xFF10B981), RoundedCornerShape(6.dp))
                                                .tactileClickable(pressedScale = 0.9f, hapticOnTap = true) {
                                                    onToggleTodoDone(todo)
                                                },
                                            contentAlignment = Alignment.Center
                                        ) {
                                            Box(
                                                modifier = Modifier
                                                    .size(9.dp)
                                                    .clip(RoundedCornerShape(3.dp))
                                                    .background(Color(0xFF10B981))
                                            )
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

                    // Section 2: Work Notes (Pinned first, compact cards)
                    item(key = "__hub_notes_hdr__") {
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(top = 8.dp),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = if (sortedNotes.isNotEmpty()) "工作手记 · ${sortedNotes.size} 篇" else "工作手记",
                                fontSize = 12.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textSecondary
                            )
                            TactileSecondaryButton(
                                text = "新建手记",
                                leadingIcon = Icons.Default.Add,
                                cornerRadius = 999.dp,
                                minHeight = 30.dp,
                                contentPadding = PaddingValues(horizontal = 11.dp, vertical = 4.dp),
                                onClick = {
                                    editingNote = null
                                    showNoteSheet = true
                                }
                            )
                        }
                    }

                    if (sortedNotes.isEmpty()) {
                        item(key = "__hub_notes_empty__") {
                            SpecularGlassCard(
                                cornerRadius = 15.dp,
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Text(
                                    text = "暂无工作手记，点击「新建手记」或让 Bot 整理产出文档，双端实时同步",
                                    fontSize = 12.5.sp,
                                    color = theme.textMuted,
                                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp)
                                )
                            }
                        }
                    }

                    items(sortedNotes, key = { "note_${it.id}" }) { note ->
                        val catLabel = when (note.category.lowercase(Locale.ROOT)) {
                            "sop" -> "运维手册"
                            "memo" -> "随手备忘"
                            else -> "工作产出"
                        }
                        SpecularGlassCard(
                            cornerRadius = 15.dp,
                            onClick = {
                                editingNote = note
                                showNoteSheet = true
                            },
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            Column(
                                modifier = Modifier.padding(horizontal = 13.dp, vertical = 10.dp),
                                verticalArrangement = Arrangement.spacedBy(4.dp)
                            ) {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Text(
                                        text = note.title,
                                        fontSize = 14.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        color = theme.textPrimary,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        modifier = Modifier.weight(1f).padding(end = 8.dp)
                                    )
                                    Row(horizontalArrangement = Arrangement.spacedBy(5.dp)) {
                                        if (note.pinned) {
                                            CrispTagPill(
                                                text = "置顶",
                                                bgColor = theme.accent.copy(alpha = 0.16f),
                                                borderColor = theme.accent.copy(alpha = 0.42f),
                                                textColor = if (theme.isLight) theme.accentDeep else theme.accentLight
                                            )
                                        }
                                        CrispTagPill(
                                            text = catLabel,
                                            bgColor = theme.inputTop,
                                            borderColor = theme.rimSubtle,
                                            textColor = theme.textSecondary
                                        )
                                    }
                                }
                                Text(
                                    text = note.previewText(100).ifEmpty { "点击查看或编辑正文" },
                                    fontSize = 12.sp,
                                    color = theme.textSecondary,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                        }
                    }
                }
            }
        }
    }

    if (showAssetSheet) {
        AssetEditorSheet(
            asset = editingAsset,
            savedCategories = savedCategories,
            availableCategories = mergedCategories,
            onSaveCategories = onSaveAssetCategories,
            onSave = { name, category, spec, price, dueDate, autoRenew ->
                onSaveAsset(editingAsset?.id, name, category, spec, price, dueDate, autoRenew)
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
            onSave = { name, type, botId, cronExpr, runAt, description, model, allowPaid, maxPaid, enabled, params ->
                onSaveTask(
                    editingTask?.id,
                    name,
                    type,
                    botId,
                    cronExpr,
                    runAt,
                    description,
                    model,
                    allowPaid,
                    maxPaid,
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

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
private fun TaskEditorSheet(
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
        allowPaidFallback: Boolean,
        maxPaidCallsPerDay: Int,
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
    var allowPaidFallback by remember(task) { mutableStateOf(task?.allowPaidFallback ?: true) }
    var maxPaidCallsText by remember(task) {
        val v = task?.maxPaidCallsPerDay ?: 0
        mutableStateOf(if (v > 0) v.toString() else "0")
    }
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
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        containerColor = theme.sheetBg,
        scrimColor = theme.scrim,
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Text(
                text = if (task == null) "新建定时任务" else "编辑定时任务",
                fontSize = 17.5.sp,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )

            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text("任务模板类型", fontSize = 12.sp, color = theme.textMuted)
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    templateChoices.forEach { (tid, label) ->
                        TactileSecondaryButton(
                            text = label,
                            accent = type == tid,
                            cornerRadius = 10.dp,
                            minHeight = 32.dp,
                            contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
                            onClick = {
                                type = tid
                                if (name.isBlank()) name = label
                            }
                        )
                    }
                }
            }

            ObsidianGlassInput(
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
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        items(workerBots, key = { it.id }) { b ->
                            TactileSecondaryButton(
                                text = b.name,
                                accent = botId == b.id,
                                cornerRadius = 10.dp,
                                minHeight = 32.dp,
                                contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
                                onClick = { botId = b.id }
                            )
                        }
                    }
                }
            }

            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                TactileSegmentedControl(
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
                            TactileSecondaryButton(
                                text = label,
                                accent = cronExpr.trim() == expr,
                                cornerRadius = 999.dp,
                                minHeight = 28.dp,
                                contentPadding = PaddingValues(horizontal = 10.dp, vertical = 3.dp),
                                onClick = { cronExpr = expr }
                            )
                        }
                    }
                    ObsidianGlassInput(
                        value = cronExpr,
                        onValueChange = { cronExpr = it },
                        label = "5 段式 Cron 表达式 (分 时 日 月 周)",
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
                                shape = RoundedCornerShape(14.dp),
                                color = theme.inputTop,
                                border = BorderStroke(1.dp, theme.rimSubtle),
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Row(
                                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 13.dp),
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
                                        tint = theme.accent,
                                        modifier = Modifier.size(18.dp)
                                    )
                                }
                            }
                        }
                        ObsidianGlassInput(
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
                        Text("定时执行的指令内容 (Prompt)", fontSize = 12.sp, color = theme.textMuted)
                        TactileSecondaryButton(
                            text = if (optimizingPrompt) "AI 精修中..." else "AI 精修优化指令",
                            accent = true,
                            enabled = !optimizingPrompt && promptText.isNotBlank(),
                            cornerRadius = 999.dp,
                            minHeight = 28.dp,
                            contentPadding = PaddingValues(horizontal = 10.dp, vertical = 3.dp),
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
                    ObsidianGlassInput(
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
                    TactileSecondaryButton(
                        text = if (model.isBlank()) "跟随负责 Bot" else resolveModelOptionDisplayName(model, modelOptions),
                        accent = model.isNotBlank(),
                        cornerRadius = 999.dp,
                        minHeight = 30.dp,
                        contentPadding = PaddingValues(horizontal = 11.dp, vertical = 4.dp),
                        onClick = { showModelPicker = true }
                    )
                }
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    listOf(
                        "" to "跟随负责 Bot",
                        "auto:paid" to "付费主力池 (auto:paid)",
                        "auto:free" to "免费模型池 (auto:free)"
                    ).forEach { (mid, label) ->
                        TactileSecondaryButton(
                            text = label,
                            accent = model == mid,
                            cornerRadius = 999.dp,
                            minHeight = 28.dp,
                            contentPadding = PaddingValues(horizontal = 10.dp, vertical = 3.dp),
                            onClick = {
                                model = mid
                                if (mid == "auto:paid") allowPaidFallback = true
                            }
                        )
                    }
                }
            }

            SpecularGlassCard(
                cornerRadius = 15.dp,
                onClick = { allowPaidFallback = !allowPaidFallback },
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Column(modifier = Modifier.weight(1f).padding(end = 10.dp)) {
                            Text(
                                text = "允许本任务调用付费模型池",
                                fontSize = 13.5.sp,
                                fontWeight = FontWeight.Medium,
                                color = theme.textPrimary
                            )
                            Text(
                                text = if (allowPaidFallback) "默认开启：优先使用付费主力池保障执行质量" else "已关闭：严格锁定零成本免费池，拦截付费调用",
                                fontSize = 11.sp,
                                color = theme.textMuted
                            )
                        }
                        TactileToggle(
                            checked = allowPaidFallback,
                            onCheckedChange = { allowPaidFallback = it }
                        )
                    }

                    if (allowPaidFallback) {
                        ObsidianGlassInput(
                            value = maxPaidCallsText,
                            onValueChange = { maxPaidCallsText = it.filter { ch -> ch.isDigit() } },
                            label = "单日最大付费调用次数上限 (0 表示不限)",
                            placeholder = "0",
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                }
            }

            SpecularGlassCard(
                cornerRadius = 15.dp,
                onClick = { enabled = !enabled },
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 14.dp, vertical = 10.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text("启用此定时任务", fontSize = 14.sp, color = theme.textPrimary)
                    TactileToggle(
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
                    TactileSecondaryButton(
                        text = "删除",
                        danger = true,
                        minHeight = 46.dp,
                        onClick = onDelete,
                        modifier = Modifier.weight(1f)
                    )
                }
                TactilePrimaryButton(
                    text = "保存任务",
                    enabled = name.isNotBlank() && (type != "bot_prompt" || promptText.isNotBlank()),
                    minHeight = 46.dp,
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
                            allowPaidFallback,
                            maxPaidCallsText.toIntOrNull()?.coerceAtLeast(0) ?: 0,
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
            activeModel = model.ifBlank { "auto:paid" },
            modelOptions = modelOptions,
            onSelect = { chosen ->
                model = chosen
                if (chosen == "auto:paid" || modelOptions.find { it.id == chosen }?.tier == "paid") {
                    allowPaidFallback = true
                }
                showModelPicker = false
            },
            onDismiss = { showModelPicker = false }
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun TodoEditorSheet(
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

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        containerColor = theme.sheetBg,
        scrimColor = theme.scrim,
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Text(
                text = if (todo == null) "新建待办事项" else "编辑待办事项",
                fontSize = 17.5.sp,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )

            ObsidianGlassInput(
                value = title,
                onValueChange = { title = it },
                label = "待办标题",
                placeholder = "例如：续费香港服务器并核对备份",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text("优先级", fontSize = 12.sp, color = theme.textMuted)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    listOf("normal" to "普通", "high" to "高优紧急", "low" to "低优").forEach { (pid, label) ->
                        TactileSecondaryButton(
                            text = label,
                            accent = priority == pid,
                            cornerRadius = 10.dp,
                            minHeight = 32.dp,
                            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 4.dp),
                            onClick = { priority = pid }
                        )
                    }
                }
            }

            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text("截止日期 (可选)", fontSize = 12.sp, color = theme.textMuted)
                    if (dueAt.isNotBlank()) {
                        Text(
                            text = "清除日期",
                            fontSize = 11.5.sp,
                            color = theme.accent,
                            modifier = Modifier.clickable { dueAt = "" }
                        )
                    }
                }
                Surface(
                    onClick = { showDatePicker = true },
                    shape = RoundedCornerShape(14.dp),
                    color = theme.inputTop,
                    border = BorderStroke(1.dp, theme.rimSubtle),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 12.dp, vertical = 13.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = dueAt.ifBlank { "点击选择截止日期" },
                            fontFamily = if (dueAt.isNotBlank()) FontFamily.Monospace else FontFamily.Default,
                            fontSize = 13.5.sp,
                            color = if (dueAt.isBlank()) theme.textMuted else theme.textPrimary
                        )
                        Icon(
                            imageVector = Icons.Default.CalendarMonth,
                            contentDescription = "选择截止日期",
                            tint = theme.accent,
                            modifier = Modifier.size(18.dp)
                        )
                    }
                }
            }

            ObsidianGlassInput(
                value = notes,
                onValueChange = { notes = it },
                label = "补充备注 (可选)",
                placeholder = "补充说明或上下文细节",
                singleLine = false,
                modifier = Modifier.fillMaxWidth()
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                if (onDelete != null) {
                    TactileSecondaryButton(
                        text = "删除",
                        danger = true,
                        minHeight = 46.dp,
                        onClick = onDelete,
                        modifier = Modifier.weight(1f)
                    )
                }
                TactilePrimaryButton(
                    text = "保存待办",
                    enabled = title.isNotBlank(),
                    minHeight = 46.dp,
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
private fun NoteEditorSheet(
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

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        containerColor = theme.sheetBg,
        scrimColor = theme.scrim,
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Text(
                text = if (note == null) "新建工作笔记" else "编辑工作笔记",
                fontSize = 17.5.sp,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )

            ObsidianGlassInput(
                value = title,
                onValueChange = { title = it },
                label = "笔记标题",
                placeholder = "例如：2026 Q4 服务器架构与续期复盘",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Text("文档分类", fontSize = 12.sp, color = theme.textMuted)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    listOf(
                        "deliverable" to "工作产出",
                        "sop" to "运维手册",
                        "memo" to "随手备忘"
                    ).forEach { (cid, label) ->
                        TactileSecondaryButton(
                            text = label,
                            accent = category == cid,
                            cornerRadius = 10.dp,
                            minHeight = 32.dp,
                            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 4.dp),
                            onClick = { category = cid }
                        )
                    }
                }
            }

            ObsidianGlassInput(
                value = content,
                onValueChange = { content = it },
                label = "Markdown 正文内容",
                placeholder = "支持 Markdown 格式排版，双端实时同步...",
                singleLine = false,
                modifier = Modifier.fillMaxWidth()
            )

            SpecularGlassCard(
                cornerRadius = 15.dp,
                onClick = { pinned = !pinned },
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 14.dp, vertical = 10.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text("置顶此篇工作笔记", fontSize = 14.sp, color = theme.textPrimary)
                    TactileToggle(
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
                    TactileSecondaryButton(
                        text = "删除",
                        danger = true,
                        minHeight = 46.dp,
                        onClick = onDelete,
                        modifier = Modifier.weight(1f)
                    )
                }
                TactilePrimaryButton(
                    text = "保存笔记",
                    enabled = title.isNotBlank(),
                    minHeight = 46.dp,
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

private data class AssetCurrencySpec(
    val code: String,
    val chipLabel: String,
    val fullLabel: String,
    val prefix: String
)

private val ASSET_CURRENCY_SPECS = listOf(
    AssetCurrencySpec("CNY", "CNY ¥", "人民币 (CNY ¥)", "¥"),
    AssetCurrencySpec("USD", "USD $", "美元 (USD $)", "$"),
    AssetCurrencySpec("EUR", "EUR €", "欧元 (EUR €)", "€"),
    AssetCurrencySpec("HKD", "HKD HK$", "港元 (HKD HK$)", "HK$"),
    AssetCurrencySpec("JPY", "JPY ¥", "日元 (JPY)", "JPY "),
    AssetCurrencySpec("GBP", "GBP £", "英镑 (GBP £)", "£"),
    AssetCurrencySpec("FREE", "免费", "免费 / 赠金", "免费")
)

private val ASSET_BILLING_CYCLES = listOf(
    "年" to "年付",
    "月" to "月付",
    "季" to "季付",
    "一次性" to "一次性"
)

private data class ParsedAssetPrice(
    val currency: String,
    val amount: String,
    val cycle: String
)

private fun parseAssetStructuredPrice(raw: String?): ParsedAssetPrice {
    val txt = raw.orEmpty().trim()
    val cycle = when {
        txt.contains("月") -> "月"
        txt.contains("季") -> "季"
        txt.contains("一次") || txt.contains("永久") -> "一次性"
        else -> "年"
    }
    val moneyPart = txt.substringBefore("/").trim()
    if (
        moneyPart.isEmpty() ||
        moneyPart.contains("免费") ||
        moneyPart.contains("0元") ||
        moneyPart.equals("free", ignoreCase = true)
    ) {
        val isExplicitFree = moneyPart.contains("免费") || moneyPart.equals("free", ignoreCase = true)
        return ParsedAssetPrice(
            currency = if (isExplicitFree) "FREE" else "CNY",
            amount = "",
            cycle = cycle
        )
    }

    val upper = moneyPart.uppercase(Locale.ROOT)
    val currency = when {
        upper.contains("HK$") || upper.contains("HKD") -> "HKD"
        upper.contains("JPY") -> "JPY"
        upper.contains("GBP") || moneyPart.contains("£") -> "GBP"
        upper.contains("EUR") || moneyPart.contains("€") -> "EUR"
        upper.contains("USD") || moneyPart.contains("$") -> "USD"
        upper.contains("CNY") || moneyPart.contains("¥") || moneyPart.contains("￥") || moneyPart.contains("元") -> "CNY"
        else -> "CNY"
    }
    val numMatch = Regex("""\d+(?:\.\d+)?""").find(moneyPart)?.value.orEmpty()
    return ParsedAssetPrice(currency = currency, amount = numMatch, cycle = cycle)
}

private fun formatAssetStructuredPrice(currency: String, amount: String, cycle: String): String {
    if (currency == "FREE") {
        return if (cycle == "一次性") "免费 (一次性)" else "免费 / $cycle"
    }
    val cleanNum = amount.trim()
    if (cleanNum.isEmpty()) {
        return ""
    }
    val prefix = ASSET_CURRENCY_SPECS.firstOrNull { it.code == currency }?.prefix ?: "¥"
    val numText = "$prefix$cleanNum"
    return if (cycle == "一次性") "$numText (一次性)" else "$numText / $cycle"
}

private fun sanitizeAssetAmountInput(raw: String): String {
    val out = StringBuilder()
    var seenDot = false
    for (ch in raw) {
        when {
            ch.isDigit() -> out.append(ch)
            ch == '.' && !seenDot -> {
                if (out.isEmpty()) out.append('0')
                out.append('.')
                seenDot = true
            }
        }
    }
    return out.toString()
}

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
private fun AssetEditorSheet(
    asset: AssetItem?,
    savedCategories: List<AssetCategoryItem>,
    availableCategories: List<AssetCategoryItem>,
    onSaveCategories: (List<AssetCategoryItem>) -> Unit,
    onSave: (
        name: String,
        category: String,
        spec: String,
        price: String,
        dueDate: String,
        autoRenew: Boolean
    ) -> Unit,
    onDelete: (() -> Unit)?,
    onDismiss: () -> Unit
) {
    val theme = BotHubThemeTokens
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val defaultCat = availableCategories.firstOrNull()?.id ?: "general"
    val defaultDate = remember { LocalDate.now().plusYears(1).toString() }
    val initialPrice = remember(asset) { parseAssetStructuredPrice(asset?.price) }

    var name by remember(asset) { mutableStateOf(asset?.name.orEmpty()) }
    var category by remember(asset) { mutableStateOf(asset?.category?.ifBlank { defaultCat } ?: defaultCat) }
    var spec by remember(asset) { mutableStateOf(asset?.spec.orEmpty()) }
    var currency by remember(asset) { mutableStateOf(initialPrice.currency) }
    var amountNum by remember(asset) { mutableStateOf(initialPrice.amount) }
    var billingCycle by remember(asset) { mutableStateOf(initialPrice.cycle) }
    var dueDate by remember(asset) {
        val raw = asset?.dueDate?.take(10).orEmpty()
        mutableStateOf(if (asset == null) defaultDate else raw)
    }
    var isPermanent by remember(asset) {
        mutableStateOf(asset?.dueDate?.startsWith("2099") == true)
    }
    var autoRenew by remember(asset) { mutableStateOf(asset?.autoRenew ?: false) }

    val formattedPricePreview = remember(currency, amountNum, billingCycle) {
        formatAssetStructuredPrice(currency, amountNum, billingCycle)
    }
    val selectedCurrencySpec = remember(currency) {
        ASSET_CURRENCY_SPECS.firstOrNull { it.code == currency } ?: ASSET_CURRENCY_SPECS.first()
    }

    var showCalendarDialog by remember { mutableStateOf(false) }
    var showCategoryManager by remember { mutableStateOf(false) }
    var newCategoryLabel by remember { mutableStateOf("") }
    var newCategoryIcon by remember { mutableStateOf("tag") }

    val customIds = remember(savedCategories) {
        savedCategories.map { it.id.lowercase(Locale.ROOT) }.toSet()
    }
    val unusedSuggestions = remember(availableCategories) {
        val curIds = availableCategories.map { it.id.lowercase(Locale.ROOT) }.toSet()
        OPTIONAL_SUGGESTED_CATEGORIES.filter { it.id.lowercase(Locale.ROOT) !in curIds }
    }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        containerColor = theme.sheetBg,
        scrimColor = theme.scrim,
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Text(
                text = if (asset == null) "添加资产" else "编辑资产",
                fontSize = 17.5.sp,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )

            // Extensible Category Selection
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "资产类别（可自由扩展）",
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Medium,
                        color = theme.textMuted
                    )
                    TactileSecondaryButton(
                        text = if (showCategoryManager) "收起管理" else "+ 扩展/管理",
                        leadingIcon = if (showCategoryManager) Icons.Default.Close else Icons.Default.Settings,
                        cornerRadius = 999.dp,
                        minHeight = 28.dp,
                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 3.dp),
                        onClick = { showCategoryManager = !showCategoryManager }
                    )
                }

                if (availableCategories.isEmpty()) {
                    Text(
                        text = "当前暂无预设分类（将归入「通用资产」）；点击右上角「+ 扩展/管理」可添加任意类型",
                        fontSize = 11.5.sp,
                        color = theme.textMuted
                    )
                } else {
                    FlowRow(
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        availableCategories.forEach { cat ->
                            val isSelected = category.equals(cat.id, ignoreCase = true)
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(4.dp)
                            ) {
                                TactileSecondaryButton(
                                    text = cat.label,
                                    leadingIcon = assetCategoryIconVector(cat.icon),
                                    accent = isSelected,
                                    cornerRadius = 11.dp,
                                    minHeight = 34.dp,
                                    contentPadding = PaddingValues(horizontal = 10.dp, vertical = 5.dp),
                                    onClick = { category = cat.id }
                                )
                                if (showCategoryManager && cat.id.lowercase(Locale.ROOT) in customIds) {
                                    Box(
                                        modifier = Modifier
                                            .size(26.dp)
                                            .clip(RoundedCornerShape(8.dp))
                                            .background(Color(0x24F43F5E))
                                            .clickable {
                                                val next = savedCategories.filterNot {
                                                    it.id.equals(cat.id, ignoreCase = true)
                                                }
                                                onSaveCategories(next)
                                                if (category.equals(cat.id, ignoreCase = true)) {
                                                    category = next.firstOrNull()?.id ?: "general"
                                                }
                                            },
                                        contentAlignment = Alignment.Center
                                    ) {
                                        Icon(
                                            imageVector = Icons.Default.Close,
                                            contentDescription = "删除分类",
                                            tint = Color(0xFFF43F5E),
                                            modifier = Modifier.size(14.dp)
                                        )
                                    }
                                }
                            }
                        }
                    }
                }

                if (showCategoryManager) {
                    Surface(
                        shape = RoundedCornerShape(15.dp),
                        color = theme.inputTop,
                        border = BorderStroke(1.dp, theme.accent.copy(alpha = 0.35f)),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Column(
                            modifier = Modifier.padding(12.dp),
                            verticalArrangement = Arrangement.spacedBy(10.dp)
                        ) {
                            ObsidianGlassInput(
                                value = newCategoryLabel,
                                onValueChange = { newCategoryLabel = it },
                                label = "新建类型名称",
                                placeholder = "例如：软件授权 / 硬件 / 保险 / 证件...",
                                singleLine = true,
                                modifier = Modifier.fillMaxWidth()
                            )

                            FlowRow(
                                horizontalArrangement = Arrangement.spacedBy(6.dp),
                                verticalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                CATEGORY_ICON_CHOICES.forEach { (iconId, iconLabel) ->
                                    TactileSecondaryButton(
                                        text = iconLabel,
                                        leadingIcon = assetCategoryIconVector(iconId),
                                        accent = newCategoryIcon == iconId,
                                        cornerRadius = 9.dp,
                                        minHeight = 30.dp,
                                        contentPadding = PaddingValues(horizontal = 8.dp, vertical = 4.dp),
                                        onClick = { newCategoryIcon = iconId }
                                    )
                                }
                            }

                            TactilePrimaryButton(
                                text = "保存并使用此类型",
                                enabled = newCategoryLabel.isNotBlank(),
                                minHeight = 38.dp,
                                onClick = {
                                    val cleanLabel = newCategoryLabel.trim()
                                    if (cleanLabel.isNotEmpty()) {
                                        val cleanId = cleanLabel.replace(Regex("\\s+"), "_")
                                        val next = savedCategories.toMutableList()
                                        if (next.none { it.id.equals(cleanId, ignoreCase = true) }) {
                                            next.add(AssetCategoryItem(cleanId, cleanLabel, newCategoryIcon))
                                            onSaveCategories(next)
                                        }
                                        category = cleanId
                                        newCategoryLabel = ""
                                    }
                                },
                                modifier = Modifier.fillMaxWidth()
                            )

                            if (unusedSuggestions.isNotEmpty()) {
                                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                                    Text(
                                        text = "可选快捷添加：",
                                        fontSize = 11.sp,
                                        color = theme.textMuted
                                    )
                                    FlowRow(
                                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                                        verticalArrangement = Arrangement.spacedBy(6.dp)
                                    ) {
                                        unusedSuggestions.forEach { sug ->
                                            TactileSecondaryButton(
                                                text = "+ ${sug.label}",
                                                leadingIcon = assetCategoryIconVector(sug.icon),
                                                cornerRadius = 999.dp,
                                                minHeight = 28.dp,
                                                contentPadding = PaddingValues(horizontal = 9.dp, vertical = 3.dp),
                                                onClick = {
                                                    val next = savedCategories + sug
                                                    onSaveCategories(next)
                                                    category = sug.id
                                                }
                                            )
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }

            ObsidianGlassInput(
                value = name,
                onValueChange = { name = it },
                label = "名称",
                placeholder = "例如：资产或服务名称",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            ObsidianGlassInput(
                value = spec,
                onValueChange = { spec = it },
                label = "说明 (可选)",
                placeholder = "规格、型号或补充备注",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            // Structured Pricing (aligned with Web AssetModal: currency + numeric amount + billing cycle)
            Surface(
                shape = RoundedCornerShape(15.dp),
                color = theme.cardTop,
                border = BorderStroke(1.dp, theme.rimSubtle),
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier.padding(12.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "费用与计费周期",
                            fontSize = 12.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textPrimary
                        )
                        if (formattedPricePreview.isNotEmpty()) {
                            Surface(
                                shape = RoundedCornerShape(999.dp),
                                color = theme.accent.copy(alpha = 0.14f),
                                border = BorderStroke(1.dp, theme.accent.copy(alpha = 0.35f))
                            ) {
                                Text(
                                    text = formattedPricePreview,
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 11.5.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = theme.accent,
                                    modifier = Modifier.padding(horizontal = 9.dp, vertical = 3.dp)
                                )
                            }
                        } else {
                            Text(
                                text = "可选",
                                fontSize = 11.sp,
                                color = theme.textMuted
                            )
                        }
                    }

                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(
                            text = "币种",
                            fontSize = 11.5.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textMuted
                        )
                        FlowRow(
                            horizontalArrangement = Arrangement.spacedBy(6.dp),
                            verticalArrangement = Arrangement.spacedBy(6.dp)
                        ) {
                            ASSET_CURRENCY_SPECS.forEach { cur ->
                                TactileSecondaryButton(
                                    text = cur.chipLabel,
                                    accent = currency == cur.code,
                                    cornerRadius = 9.dp,
                                    minHeight = 31.dp,
                                    contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
                                    onClick = {
                                        currency = cur.code
                                        if (cur.code == "FREE") {
                                            amountNum = ""
                                        }
                                    }
                                )
                            }
                        }
                    }

                    if (currency != "FREE") {
                        ObsidianGlassInput(
                            value = amountNum,
                            onValueChange = { amountNum = sanitizeAssetAmountInput(it) },
                            label = "金额 · ${selectedCurrencySpec.fullLabel}",
                            placeholder = "输入数字金额，例如 99 或 12.99",
                            singleLine = true,
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                            modifier = Modifier.fillMaxWidth()
                        )
                    } else {
                        Text(
                            text = "当前已选「免费 / 赠金」，无需填写金额。",
                            fontSize = 11.5.sp,
                            color = theme.textMuted
                        )
                    }

                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(
                            text = "计费周期",
                            fontSize = 11.5.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textMuted
                        )
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.spacedBy(6.dp)
                        ) {
                            ASSET_BILLING_CYCLES.forEach { (cycleVal, cycleLabel) ->
                                TactileSecondaryButton(
                                    text = cycleLabel,
                                    accent = billingCycle == cycleVal,
                                    cornerRadius = 9.dp,
                                    minHeight = 32.dp,
                                    contentPadding = PaddingValues(horizontal = 6.dp, vertical = 4.dp),
                                    onClick = { billingCycle = cycleVal },
                                    modifier = Modifier.weight(1f)
                                )
                            }
                        }
                    }
                }
            }

            // Theme-matched Calendar Date Trigger
            Column(
                verticalArrangement = Arrangement.spacedBy(6.dp),
                modifier = Modifier.fillMaxWidth()
            ) {
                Text(
                    text = "到期日 (日历选择)",
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Medium,
                    color = theme.textMuted
                )
                Surface(
                    onClick = { showCalendarDialog = true },
                    shape = RoundedCornerShape(14.dp),
                    color = theme.inputTop,
                    border = BorderStroke(1.dp, theme.rimSubtle),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 12.dp, vertical = 13.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = when {
                                isPermanent -> "长期有效 / 免续费"
                                dueDate.isNotBlank() -> dueDate
                                else -> "选择日期"
                            },
                            fontFamily = if (!isPermanent && dueDate.isNotBlank()) FontFamily.Monospace else FontFamily.Default,
                            fontSize = 13.5.sp,
                            color = if (!isPermanent && dueDate.isBlank()) theme.textMuted else theme.textPrimary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f)
                        )
                        Icon(
                            imageVector = Icons.Default.CalendarMonth,
                            contentDescription = "打开日历选择日期",
                            tint = theme.accent,
                            modifier = Modifier.size(18.dp)
                        )
                    }
                }
            }

            if (!isPermanent) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "基于当前到期日顺延：",
                        fontSize = 11.5.sp,
                        color = theme.textMuted
                    )
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        listOf(1 to "+1月", 3 to "+1季", 12 to "+1年").forEach { (months, label) ->
                            TactileSecondaryButton(
                                text = label,
                                cornerRadius = 999.dp,
                                minHeight = 28.dp,
                                contentPadding = PaddingValues(horizontal = 10.dp, vertical = 3.dp),
                                onClick = {
                                    val today = LocalDate.now()
                                    val parsed = runCatching { LocalDate.parse(dueDate.take(10)) }.getOrNull()
                                    val base = if (parsed != null && !dueDate.startsWith("2099") && !parsed.isBefore(today.minusDays(30))) {
                                        parsed
                                    } else {
                                        today
                                    }
                                    dueDate = base.plusMonths(months.toLong()).toString()
                                }
                            )
                        }
                    }
                }
            }

            SpecularGlassCard(
                cornerRadius = 15.dp,
                onClick = { autoRenew = !autoRenew },
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { autoRenew = !autoRenew }
                        .padding(horizontal = 14.dp, vertical = 10.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text("自动续费", fontSize = 14.sp, color = theme.textPrimary)
                    TactileToggle(
                        checked = autoRenew,
                        onCheckedChange = { autoRenew = it }
                    )
                }
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                if (onDelete != null) {
                    TactileSecondaryButton(
                        text = "删除",
                        danger = true,
                        minHeight = 46.dp,
                        onClick = onDelete,
                        modifier = Modifier.weight(1f)
                    )
                }
                TactilePrimaryButton(
                    text = "保存",
                    enabled = name.isNotBlank(),
                    minHeight = 46.dp,
                    onClick = {
                        if (name.isNotBlank()) {
                            onSave(
                                name.trim(),
                                category.ifBlank { "general" },
                                spec.trim(),
                                formatAssetStructuredPrice(currency, amountNum, billingCycle),
                                if (isPermanent) "2099-12-31" else dueDate.trim().ifEmpty { "2099-12-31" },
                                autoRenew
                            )
                        }
                    },
                    modifier = Modifier.weight(if (onDelete != null) 1.5f else 1f)
                )
            }

            Spacer(modifier = Modifier.height(8.dp))
        }
    }

    if (showCalendarDialog) {
        ThemeCalendarDialog(
            initialDate = if (isPermanent) defaultDate else dueDate.ifBlank { defaultDate },
            allowPerpetual = true,
            onSelectDate = { picked ->
                isPermanent = false
                dueDate = picked
                showCalendarDialog = false
            },
            onSelectPerpetual = {
                isPermanent = true
                dueDate = "2099-12-31"
                showCalendarDialog = false
            },
            onDismiss = { showCalendarDialog = false }
        )
    }
}
