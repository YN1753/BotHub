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
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.CalendarMonth
import androidx.compose.material.icons.filled.Close
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.AssetCategoryItem
import uk.starryheavens.vesper.bothub.data.AssetItem
import uk.starryheavens.vesper.bothub.data.DashboardSnapshot
import uk.starryheavens.vesper.bothub.data.TaskItem
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
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens
import java.time.LocalDate
import java.util.Locale

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

@Composable
fun HubTabScreen(
    snapshot: DashboardSnapshot?,
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
    onDeleteAsset: (String) -> Unit,
    onSaveAssetCategories: (List<AssetCategoryItem>) -> Unit = {},
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val assets = snapshot?.assets ?: emptyList()
    val savedCategories = snapshot?.assetCategories ?: emptyList()
    val mergedCategories = remember(savedCategories, assets) {
        mergeExtensibleAssetCategories(savedCategories, assets)
    }
    val tasks = snapshot?.tasks ?: emptyList()
    val expiringCount = snapshot?.expiringCount ?: 0
    val activeTasksCount = snapshot?.activeTasks ?: tasks.count { it.enabled }

    var selectedSegment by remember { mutableStateOf(0) }
    var selectedCategoryFilter by remember { mutableStateOf("all") }
    var showAssetSheet by remember { mutableStateOf(false) }
    var editingAsset by remember { mutableStateOf<AssetItem?>(null) }

    val filteredAssets = remember(assets, selectedCategoryFilter) {
        if (selectedCategoryFilter == "all") {
            assets
        } else {
            assets.filter { it.category.equals(selectedCategoryFilter, ignoreCase = true) }
        }
    }

    Column(modifier = modifier) {
        TactileSegmentedControl(
            options = listOf(
                "资产 (${assets.size})",
                "定时任务 ($activeTasksCount/${tasks.size})"
            ),
            selectedIndex = selectedSegment,
            onSelect = { selectedSegment = it },
            modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 4.dp)
        )

        LazyColumn(
            modifier = Modifier.fillMaxWidth().weight(1f),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            if (selectedSegment == 0) {
                item(key = "__hub_assets_hdr__") {
                    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                                modifier = Modifier.weight(1f)
                            ) {
                                Text(
                                    text = "共 ${filteredAssets.size} 项",
                                    fontSize = 12.5.sp,
                                    color = theme.textMuted,
                                    maxLines = 1,
                                    softWrap = false
                                )
                                CrispTagPill(
                                    text = if (expiringCount > 0) "${expiringCount}项临期" else "状态正常",
                                    bgColor = if (expiringCount > 0) Color(0x26F59E0B) else Color(0x2210B981),
                                    borderColor = if (expiringCount > 0) Color(0x52F59E0B) else Color(0x4710B981),
                                    textColor = if (expiringCount > 0) Color(0xFFD97706) else Color(0xFF10B981)
                                )
                            }

                            Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                TactileSecondaryButton(
                                    text = "添加",
                                    leadingIcon = Icons.Default.Add,
                                    cornerRadius = 999.dp,
                                    minHeight = 32.dp,
                                    contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
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
                                    minHeight = 32.dp,
                                    contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                                    onClick = onSyncAssets
                                )
                            }
                        }

                        if (mergedCategories.isNotEmpty()) {
                            LazyRow(
                                horizontalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                item(key = "__cat_all__") {
                                    TactileSecondaryButton(
                                        text = "全部",
                                        accent = selectedCategoryFilter == "all",
                                        cornerRadius = 999.dp,
                                        minHeight = 30.dp,
                                        contentPadding = PaddingValues(horizontal = 11.dp, vertical = 4.dp),
                                        onClick = { selectedCategoryFilter = "all" }
                                    )
                                }
                                items(mergedCategories, key = { it.id }) { cat ->
                                    TactileSecondaryButton(
                                        text = cat.label,
                                        leadingIcon = assetCategoryIconVector(cat.icon),
                                        accent = selectedCategoryFilter.equals(cat.id, ignoreCase = true),
                                        cornerRadius = 999.dp,
                                        minHeight = 30.dp,
                                        contentPadding = PaddingValues(horizontal = 10.dp, vertical = 4.dp),
                                        onClick = { selectedCategoryFilter = cat.id }
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

                    SpecularGlassCard(
                        cornerRadius = 17.dp,
                        accentTint = if (asset.isExpiringSoon && !isPermanent) barColor else null,
                        onClick = {
                            editingAsset = asset
                            showAssetSheet = true
                        },
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Column(
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 11.dp),
                            verticalArrangement = Arrangement.spacedBy(7.dp)
                        ) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                    modifier = Modifier.weight(1f).padding(end = 10.dp)
                                ) {
                                    Box(
                                        modifier = Modifier
                                            .size(28.dp)
                                            .clip(RoundedCornerShape(9.dp))
                                            .background(theme.accent.copy(alpha = 0.14f))
                                            .border(1.dp, theme.accent.copy(alpha = 0.3f), RoundedCornerShape(9.dp)),
                                        contentAlignment = Alignment.Center
                                    ) {
                                        Icon(
                                            imageVector = assetCategoryIconVector(catMeta.icon),
                                            contentDescription = null,
                                            tint = theme.accent,
                                            modifier = Modifier.size(15.dp)
                                        )
                                    }
                                    Column(modifier = Modifier.weight(1f)) {
                                        Text(
                                            text = asset.name,
                                            fontSize = 14.5.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            color = theme.textPrimary,
                                            maxLines = 1,
                                            softWrap = false,
                                            overflow = TextOverflow.Ellipsis
                                        )
                                        Text(
                                            text = catMeta.label,
                                            fontSize = 10.5.sp,
                                            color = theme.textMuted,
                                            maxLines = 1
                                        )
                                    }
                                }
                                CrispTagPill(
                                    text = if (isPermanent) "长期" else asset.dueBadgeLabel,
                                    bgColor = barColor.copy(alpha = 0.14f),
                                    borderColor = barColor.copy(alpha = 0.42f),
                                    textColor = barColor
                                )
                            }

                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Text(
                                    text = listOf(asset.spec, asset.price)
                                        .filter { it.isNotBlank() }
                                        .joinToString(" · ")
                                        .ifEmpty { "点击编辑详情" },
                                    fontSize = 12.sp,
                                    color = theme.textMuted,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                    modifier = Modifier
                                        .weight(1f)
                                        .padding(end = 12.dp)
                                )
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
                            }

                            Box(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .height(3.dp)
                                    .clip(RoundedCornerShape(999.dp))
                                    .background(theme.inputTop)
                            ) {
                                Box(
                                    modifier = Modifier
                                        .fillMaxWidth(progressFraction)
                                        .height(3.dp)
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
            } else {
                if (tasks.isEmpty()) {
                    item(key = "__hub_tasks_empty__") {
                        SpecularGlassCard(
                            cornerRadius = 16.dp,
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            Text(
                                text = "暂无定时任务",
                                fontSize = 13.sp,
                                color = theme.textMuted,
                                modifier = Modifier.padding(16.dp)
                            )
                        }
                    }
                }

                items(tasks, key = { it.id }) { task ->
                    SpecularGlassCard(
                        cornerRadius = 17.dp,
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Column(
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 11.dp),
                            verticalArrangement = Arrangement.spacedBy(7.dp)
                        ) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Text(
                                    text = task.name,
                                    fontSize = 14.5.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = if (task.enabled) theme.textPrimary else theme.textMuted,
                                    maxLines = 1,
                                    softWrap = false,
                                    overflow = TextOverflow.Ellipsis,
                                    modifier = Modifier
                                        .weight(1f)
                                        .padding(end = 10.dp)
                                )
                                TactileToggle(
                                    checked = task.enabled,
                                    onCheckedChange = { checked -> onToggleTask(task, checked) }
                                )
                            }

                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Text(
                                    text = task.formattedSchedule(),
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 11.5.sp,
                                    color = theme.textMuted,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                    modifier = Modifier
                                        .weight(1f)
                                        .padding(end = 10.dp)
                                )

                                TactileSecondaryButton(
                                    text = "运行",
                                    leadingIcon = Icons.Default.PlayArrow,
                                    accent = true,
                                    cornerRadius = 999.dp,
                                    minHeight = 31.dp,
                                    contentPadding = PaddingValues(horizontal = 11.dp, vertical = 4.dp),
                                    onClick = { onRunTask(task) }
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

    var name by remember(asset) { mutableStateOf(asset?.name.orEmpty()) }
    var category by remember(asset) { mutableStateOf(asset?.category?.ifBlank { defaultCat } ?: defaultCat) }
    var spec by remember(asset) { mutableStateOf(asset?.spec.orEmpty()) }
    var price by remember(asset) { mutableStateOf(asset?.price.orEmpty()) }
    var dueDate by remember(asset) {
        val raw = asset?.dueDate?.take(10).orEmpty()
        mutableStateOf(if (asset == null) defaultDate else raw)
    }
    var isPermanent by remember(asset) {
        mutableStateOf(asset?.dueDate?.startsWith("2099") == true)
    }
    var autoRenew by remember(asset) { mutableStateOf(asset?.autoRenew ?: false) }

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

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                verticalAlignment = Alignment.Bottom
            ) {
                ObsidianGlassInput(
                    value = price,
                    onValueChange = { price = it },
                    label = "费用 (可选)",
                    placeholder = "例如：¥99 / 年",
                    singleLine = true,
                    modifier = Modifier.weight(1f)
                )

                // Theme-matched Calendar Date Trigger
                Column(
                    verticalArrangement = Arrangement.spacedBy(6.dp),
                    modifier = Modifier.weight(1.1f)
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
                        border = BorderStroke(1.dp, theme.rimMid),
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
                                    isPermanent -> "长期有效"
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
                                price.trim(),
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
