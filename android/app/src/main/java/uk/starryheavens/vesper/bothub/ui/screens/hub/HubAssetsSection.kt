package uk.starryheavens.vesper.bothub.ui.screens.hub

import android.content.Intent
import android.net.Uri
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
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.OpenInNew
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.CalendarMonth
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.outlined.Inventory2
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.json.JSONObject
import uk.starryheavens.vesper.bothub.data.AssetCategoryItem
import uk.starryheavens.vesper.bothub.data.AssetItem
import uk.starryheavens.vesper.bothub.ui.components.ThemeCalendarDialog
import uk.starryheavens.vesper.bothub.ui.components.VesperButtonTone
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperDivider
import uk.starryheavens.vesper.bothub.ui.components.VesperEmptyState
import uk.starryheavens.vesper.bothub.ui.components.VesperFilterChip
import uk.starryheavens.vesper.bothub.ui.components.VesperIconButton
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSearchField
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSegmentedTabs
import uk.starryheavens.vesper.bothub.ui.components.VesperSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.VesperSwitch
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.VesperTextField
import uk.starryheavens.vesper.bothub.ui.components.assetCategoryIconVector
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens
import java.time.LocalDate
import java.time.temporal.ChronoUnit
import java.util.Locale
import kotlin.math.abs
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

private val ASSET_STATUS_OPTIONS = listOf("生效中", "已到期", "已停用")

private val ASSET_WARN_OPTIONS = listOf(
    "[7, 3, 1]" to "到期前 7天 / 3天 / 1天",
    "[14, 7, 3, 1]" to "到期前 14天 / 7天 / 3天 / 1天",
    "[30, 15, 7, 3, 1]" to "到期前 30天 / 15天 / 7天 / 3天 / 1天"
)

private val EXTRA_PRESET_KEYS = listOf("host", "region", "registrar", "panel_url", "account")

private val ASSET_PRICE_REGEX = Regex(
    """(HK\$|NT\$|S\$|C\$|A\$|US\$|CN¥|RMB|USD|CNY|EUR|GBP|HKD|JPY|TWD|SGD|CAD|AUD|[¥$€£])?\s*([0-9]+(?:\.[0-9]+)?)""",
    RegexOption.IGNORE_CASE
)

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

private fun normalizeAssetStatus(raw: String?): String {
    val s = raw.orEmpty().trim()
    return when {
        s == "已到期" || s.equals("expired", ignoreCase = true) -> "已到期"
        s == "已停用" || s == "待购入" || s.equals("paused", ignoreCase = true) || s.equals("offline", ignoreCase = true) -> "已停用"
        else -> "生效中"
    }
}

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

/**
 * Matches Web's summarizeAssetsMonthlyCost / summarizeAssetsYearlyCost exactly:
 * excludes paused/archived, free, and one-time/permanent buyout assets; handles year/quarter/week/day/month cycles.
 */
internal fun summarizeAssetsCost(
    assets: List<AssetItem>,
    yearly: Boolean,
    fxRates: Map<String, Double> = emptyMap()
): String {
    var totalCNY = 0.0
    var hasPaidAsset = false
    assets.forEach { a ->
        if (a.status == "已停用" || a.status == "已归档") return@forEach
        val raw = a.price.trim()
        if (raw.isEmpty() || raw.contains("免费") || raw.contains("一次") || raw.contains("永久")) return@forEach
        val match = ASSET_PRICE_REGEX.find(raw) ?: return@forEach
        val symbol = match.groupValues[1]
        val amount = match.groupValues[2].toDoubleOrNull() ?: return@forEach
        if (amount <= 0.0) return@forEach
        val cnyAmt = amount * rateToCNY(detectPriceCurrencyCode(raw, symbol), fxRates)
        val lower = raw.lowercase(Locale.ROOT)
        val normalized = when {
            raw.contains("年") || lower.contains("/yr") || lower.contains("year") || lower.contains("/y") ->
                if (yearly) cnyAmt else cnyAmt / 12.0
            raw.contains("季") || lower.contains("/qtr") || lower.contains("quarter") ->
                if (yearly) cnyAmt * 4.0 else cnyAmt / 3.0
            raw.contains("周") || lower.contains("/wk") || lower.contains("week") ->
                if (yearly) cnyAmt * 52.0 else cnyAmt * 4.33
            raw.contains("天") || raw.contains("日") || lower.contains("/day") || lower.contains("/d") ->
                if (yearly) cnyAmt * 365.0 else cnyAmt * 30.0
            else ->
                if (yearly) cnyAmt * 12.0 else cnyAmt
        }
        totalCNY += normalized
        hasPaidAsset = true
    }
    if (!hasPaidAsset || totalCNY <= 0.0) return "--"
    return if (yearly) {
        "¥${totalCNY.roundToInt()}/年"
    } else {
        val rounded = (totalCNY * 10.0).roundToInt() / 10.0
        val numStr = if (rounded % 1.0 == 0.0) rounded.toInt().toString() else rounded.toString()
        "¥$numStr/月"
    }
}

internal fun resolveAssetCategory(
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

internal fun mergeExtensibleAssetCategories(
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

private fun parseExtraConfigRows(rawJson: String): List<Pair<String, String>> {
    val trimmed = rawJson.trim()
    if (trimmed.isEmpty() || trimmed == "{}") return emptyList()
    return runCatching {
        val obj = JSONObject(trimmed)
        buildList {
            val iter = obj.keys()
            while (iter.hasNext()) {
                val k = iter.next().trim()
                if (k.isNotEmpty()) {
                    add(k to obj.optString(k, ""))
                }
            }
        }
    }.getOrDefault(emptyList())
}

private fun serializeExtraConfigRows(rows: List<Pair<String, String>>): String {
    val obj = JSONObject()
    rows.forEach { (rawKey, rawVal) ->
        val k = rawKey.trim()
        if (k.isNotEmpty()) {
            obj.put(k, rawVal.trim())
        }
    }
    return obj.toString()
}

internal fun LazyListScope.hubAssetsItems(
    assets: List<AssetItem>,
    filteredAssets: List<AssetItem>,
    savedCategories: List<AssetCategoryItem>,
    mergedCategories: List<AssetCategoryItem>,
    categoryCounts: Map<String, Int>,
    searchQuery: String,
    selectedCategoryFilter: String,
    expiringOnly: Boolean,
    sortByUrgency: Boolean,
    totalMonthlyCost: String,
    totalYearlyCost: String,
    expiringCount: Int,
    onSearchQueryChange: (String) -> Unit,
    onSelectCategory: (String) -> Unit,
    onToggleExpiringOnly: () -> Unit,
    onToggleSortByUrgency: () -> Unit,
    onResetFilters: () -> Unit,
    onAddAsset: () -> Unit,
    onEditAsset: (AssetItem) -> Unit
) {
    item(key = "__hub_assets_hdr__") {
        val theme = BotHubThemeTokens
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            // Compact Summary + Core Add Action + Sort / Expiring Filter Controls (aligned with Web)
            VesperCard(
                cornerRadius = 15.dp,
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 13.dp, vertical = 11.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Column(
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp),
                            verticalArrangement = Arrangement.spacedBy(3.dp)
                        ) {
                            Text(
                                text = if (totalMonthlyCost != "--") "月均 $totalMonthlyCost" else "月均 ¥0/月",
                                fontSize = 15.sp,
                                fontWeight = FontWeight.Bold,
                                fontFamily = FontFamily.Monospace,
                                color = theme.accentForeground,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                            Text(
                                text = buildString {
                                    if (totalYearlyCost != "--") append("年化 $totalYearlyCost · ")
                                    append("显示 ${filteredAssets.size} 项")
                                },
                                fontSize = 11.5.sp,
                                fontFamily = FontFamily.Monospace,
                                color = theme.textMuted,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                        }

                        VesperPrimaryButton(
                            text = "添加",
                            leadingIcon = Icons.Default.Add,
                            cornerRadius = 999.dp,
                            minHeight = 44.dp,
                            contentPadding = PaddingValues(horizontal = 14.dp, vertical = 8.dp),
                            onClick = onAddAsset
                        )
                    }

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        VesperFilterChip(
                            label = "按紧迫度",
                            selected = sortByUrgency,
                            onClick = onToggleSortByUrgency
                        )
                        if (expiringCount > 0) {
                            VesperFilterChip(
                                label = "${expiringCount} 项临期",
                                selected = expiringOnly,
                                onClick = onToggleExpiringOnly
                            )
                        }
                    }
                }
            }

            // Search bar matching Web (searches name, spec, status, extra_config)
            if (assets.isNotEmpty()) {
                VesperSearchField(
                    value = searchQuery,
                    onValueChange = onSearchQueryChange,
                    placeholder = "搜索资产名称、规格、属性..."
                )
            }

            // Category filter tabs
            if (mergedCategories.isNotEmpty()) {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    item(key = "__cat_all__") {
                        VesperFilterChip(
                            label = "全部 ${assets.size}",
                            selected = selectedCategoryFilter == "all",
                            onClick = { onSelectCategory("all") }
                        )
                    }
                    items(mergedCategories, key = { it.id }) { cat ->
                        val cnt = categoryCounts[cat.id.lowercase(Locale.ROOT)] ?: 0
                        VesperFilterChip(
                            label = if (cnt > 0) "${cat.label} $cnt" else cat.label,
                            leadingIcon = assetCategoryIconVector(cat.icon),
                            selected = selectedCategoryFilter.equals(cat.id, ignoreCase = true),
                            onClick = {
                                onSelectCategory(
                                    if (selectedCategoryFilter.equals(cat.id, ignoreCase = true)) "all" else cat.id
                                )
                            }
                        )
                    }
                }
            }
        }
    }

    if (filteredAssets.isEmpty()) {
        item(key = "__hub_assets_empty__") {
            VesperEmptyState(
                icon = Icons.Outlined.Inventory2,
                title = if (assets.isEmpty()) "暂无追踪中的资产" else "当前筛选条件下无匹配资产",
                subtitle = if (assets.isEmpty()) {
                    "点击上方「添加」登记数字资产，支持自定义分类、预警梯度与扩展属性"
                } else "",
                actions = if (assets.isNotEmpty()) {
                    {
                        VesperSecondaryButton(
                            text = "重置筛选条件",
                            minHeight = 44.dp,
                            onClick = onResetFilters
                        )
                    }
                } else null
            )
        }
    }

    items(filteredAssets, key = { it.id }, contentType = { "asset_card" }) { asset ->
        HubAssetCard(
            asset = asset,
            savedCategories = savedCategories,
            showCategoryPrefix = selectedCategoryFilter == "all",
            onClick = { onEditAsset(asset) }
        )
    }
}

@Composable
private fun HubAssetCard(
    asset: AssetItem,
    savedCategories: List<AssetCategoryItem>,
    showCategoryPrefix: Boolean,
    onClick: () -> Unit
) {
    val theme = BotHubThemeTokens
    val catMeta = remember(asset.category, savedCategories) {
        resolveAssetCategory(asset.category, savedCategories)
    }
    val isPermanent = asset.isPerpetual
    val barColor = when {
        isPermanent -> theme.success
        asset.daysLeft <= 3 -> theme.danger
        asset.daysLeft <= 7 -> theme.warning
        else -> theme.accent
    }
    val daysColor = when {
        isPermanent -> theme.success
        asset.daysLeft <= 3 -> theme.danger
        asset.daysLeft <= 7 -> theme.warning
        else -> theme.success
    }
    // Exact Web 90-day runway formula
    val progressFraction = when {
        isPermanent -> 1f
        asset.daysLeft <= 0 -> 0.06f
        else -> (asset.daysLeft.toFloat() / 90f).coerceIn(0.08f, 1f)
    }

    VesperCard(
        cornerRadius = 15.dp,
        accentTone = if (asset.isExpiringSoon && !isPermanent) barColor else null,
        onClick = onClick,
        modifier = Modifier.fillMaxWidth()
    ) {
        Column(
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.Top
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                    modifier = Modifier
                        .weight(1f)
                        .padding(end = 8.dp)
                ) {
                    Box(
                        modifier = Modifier
                            .size(30.dp)
                            .clip(RoundedCornerShape(8.dp))
                            .background(theme.accentSoft)
                            .border(0.75.dp, theme.accentMuted, RoundedCornerShape(8.dp)),
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = assetCategoryIconVector(catMeta.icon),
                            contentDescription = null,
                            tint = theme.accentForeground,
                            modifier = Modifier.size(15.dp)
                        )
                    }
                    Column(
                        modifier = Modifier.weight(1f),
                        verticalArrangement = Arrangement.spacedBy(2.dp)
                    ) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(6.dp)
                        ) {
                            Text(
                                text = asset.name,
                                fontSize = 14.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textPrimary,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                                modifier = Modifier.weight(1f, fill = false)
                            )
                            if (asset.status == "已到期" || asset.status == "已停用") {
                                VesperTag(
                                    text = asset.status,
                                    toneColor = if (asset.status == "已到期") theme.danger else theme.textMuted
                                )
                            }
                        }
                        val subText = when {
                            asset.spec.isNotBlank() && showCategoryPrefix -> "${catMeta.label} · ${asset.spec}"
                            asset.spec.isNotBlank() -> asset.spec
                            else -> catMeta.label
                        }
                        Text(
                            text = subText,
                            fontSize = 12.sp,
                            color = theme.textMuted,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }

                VesperTag(
                    text = asset.price.ifBlank { "--" },
                    toneColor = theme.accent
                )
            }

            // 90-day runway progress bar
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(3.5.dp)
                    .clip(RoundedCornerShape(999.dp))
                    .background(theme.surfaceInput)
            ) {
                Box(
                    modifier = Modifier
                        .fillMaxWidth(progressFraction)
                        .height(3.5.dp)
                        .clip(RoundedCornerShape(999.dp))
                        .background(barColor)
                )
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                val dateAndRenew = buildString {
                    append(if (isPermanent) "长期有效" else asset.dueDate.take(10).ifBlank { "--" })
                    if (asset.autoRenew) append(" · 自动续费")
                }
                Text(
                    text = dateAndRenew,
                    fontFamily = FontFamily.Monospace,
                    fontSize = 11.5.sp,
                    color = theme.textMuted,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier
                        .weight(1f)
                        .padding(end = 8.dp)
                )
                Text(
                    text = asset.dueBadgeLabel,
                    fontFamily = FontFamily.Monospace,
                    fontSize = 11.5.sp,
                    fontWeight = if (!isPermanent && asset.daysLeft <= 7) FontWeight.SemiBold else FontWeight.Medium,
                    color = daysColor,
                    maxLines = 1,
                    softWrap = false
                )
            }
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
    if (cleanNum.isEmpty()) return cycle
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
internal fun AssetEditorSheet(
    asset: AssetItem?,
    savedCategories: List<AssetCategoryItem>,
    availableCategories: List<AssetCategoryItem>,
    onSaveCategories: (List<AssetCategoryItem>) -> Unit,
    onSave: (
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
    onDelete: (() -> Unit)?,
    onDismiss: () -> Unit
) {
    val theme = BotHubThemeTokens
    val context = LocalContext.current
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val defaultCat = availableCategories.firstOrNull()?.id ?: "general"
    val defaultDate = remember { LocalDate.now().plusYears(1).toString() }
    val initialPrice = remember(asset) { parseAssetStructuredPrice(asset?.price) }

    var name by remember(asset) { mutableStateOf(asset?.name.orEmpty()) }
    var category by remember(asset) { mutableStateOf(asset?.category?.ifBlank { defaultCat } ?: defaultCat) }
    var spec by remember(asset) { mutableStateOf(asset?.spec.orEmpty()) }
    var status by remember(asset) { mutableStateOf(normalizeAssetStatus(asset?.status)) }
    var currency by remember(asset) { mutableStateOf(initialPrice.currency) }
    var amountNum by remember(asset) { mutableStateOf(initialPrice.amount) }
    var billingCycle by remember(asset) { mutableStateOf(initialPrice.cycle) }
    var isPermanent by remember(asset) {
        mutableStateOf(
            asset != null && (asset.dueDate.isBlank() || asset.dueDate.startsWith("2099"))
        )
    }
    var dueDate by remember(asset) {
        val raw = asset?.dueDate?.take(10).orEmpty()
        mutableStateOf(
            if (asset == null || raw.isBlank() || raw.startsWith("2099")) defaultDate else raw
        )
    }
    var autoRenew by remember(asset) { mutableStateOf(asset?.autoRenew ?: false) }
    var warnDays by remember(asset) {
        val rawWarn = asset?.warnDays?.trim().orEmpty()
        mutableStateOf(
            ASSET_WARN_OPTIONS.firstOrNull { it.first == rawWarn }?.first ?: "[7, 3, 1]"
        )
    }
    val extraRows = remember(asset) {
        mutableStateListOf<Pair<String, String>>().apply {
            addAll(parseExtraConfigRows(asset?.extraConfig.orEmpty()))
        }
    }

    val formattedPricePreview = remember(currency, amountNum, billingCycle) {
        formatAssetStructuredPrice(currency, amountNum, billingCycle)
    }
    val selectedCurrencySpec = remember(currency) {
        ASSET_CURRENCY_SPECS.firstOrNull { it.code == currency } ?: ASSET_CURRENCY_SPECS.first()
    }
    val previewDaysLeft = remember(isPermanent, dueDate) {
        if (isPermanent) {
            null
        } else {
            runCatching {
                ChronoUnit.DAYS.between(LocalDate.now(), LocalDate.parse(dueDate.take(10))).toInt()
            }.getOrNull()
        }
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
            verticalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(3.dp)) {
                Text(
                    text = if (asset == null) "登记数字资产" else "编辑数字资产",
                    fontSize = 18.sp,
                    fontWeight = FontWeight.Bold,
                    color = theme.textPrimary
                )
                Text(
                    text = "结构化管理资产周期、币种成本、状态、预警规则与扩展属性",
                    fontSize = 12.sp,
                    color = theme.textMuted
                )
            }

            // =========================================================================
            // Section 1: Basic Info, Lifecycle Status & Structured Billing
            // =========================================================================
            VesperCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                Column(
                    modifier = Modifier.padding(14.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    Text(
                        text = "基础信息与结构化费用",
                        fontSize = 12.5.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = theme.accentForeground
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
                                color = theme.textSecondary
                            )
                            VesperSecondaryButton(
                                text = if (showCategoryManager) "收起管理" else "扩展/管理",
                                leadingIcon = if (showCategoryManager) Icons.Default.Close else Icons.Default.Settings,
                                cornerRadius = 999.dp,
                                minHeight = 36.dp,
                                contentPadding = PaddingValues(horizontal = 11.dp, vertical = 6.dp),
                                onClick = { showCategoryManager = !showCategoryManager }
                            )
                        }

                        if (availableCategories.isEmpty()) {
                            Text(
                                text = "当前暂无自定义分类（将归入「通用资产」）；点击右上角「扩展/管理」可添加任意类型",
                                fontSize = 11.5.sp,
                                color = theme.textMuted
                            )
                        } else {
                            FlowRow(
                                horizontalArrangement = Arrangement.spacedBy(6.dp),
                                verticalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                availableCategories.forEach { cat ->
                                    val isSelected = category.equals(cat.id, ignoreCase = true)
                                    Row(
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(4.dp)
                                    ) {
                                        VesperFilterChip(
                                            label = cat.label,
                                            leadingIcon = assetCategoryIconVector(cat.icon),
                                            selected = isSelected,
                                            onClick = { category = cat.id }
                                        )
                                        if (showCategoryManager && cat.id.lowercase(Locale.ROOT) in customIds) {
                                            Box(
                                                modifier = Modifier
                                                    .size(36.dp)
                                                    .clip(RoundedCornerShape(9.dp))
                                                    .background(theme.danger.copy(alpha = 0.14f))
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
                                                    tint = theme.danger,
                                                    modifier = Modifier.size(15.dp)
                                                )
                                            }
                                        }
                                    }
                                }
                            }
                        }

                        if (showCategoryManager) {
                            VesperCard(
                                cornerRadius = 14.dp,
                                accentTone = theme.accent,
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Column(
                                    modifier = Modifier.padding(12.dp),
                                    verticalArrangement = Arrangement.spacedBy(10.dp)
                                ) {
                                    VesperTextField(
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
                                            VesperFilterChip(
                                                label = iconLabel,
                                                leadingIcon = assetCategoryIconVector(iconId),
                                                selected = newCategoryIcon == iconId,
                                                onClick = { newCategoryIcon = iconId }
                                            )
                                        }
                                    }

                                    VesperPrimaryButton(
                                        text = "保存并使用此类型",
                                        enabled = newCategoryLabel.isNotBlank(),
                                        minHeight = 44.dp,
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
                                                fontSize = 11.5.sp,
                                                color = theme.textMuted
                                            )
                                            FlowRow(
                                                horizontalArrangement = Arrangement.spacedBy(6.dp),
                                                verticalArrangement = Arrangement.spacedBy(6.dp)
                                            ) {
                                                unusedSuggestions.forEach { sug ->
                                                    VesperFilterChip(
                                                        label = "+ ${sug.label}",
                                                        leadingIcon = assetCategoryIconVector(sug.icon),
                                                        selected = false,
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

                    VesperTextField(
                        value = name,
                        onValueChange = { name = it },
                        label = "资产名称",
                        placeholder = "如：项目或服务名称",
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )

                    VesperTextField(
                        value = spec,
                        onValueChange = { spec = it },
                        label = "规格与备注描述",
                        placeholder = "如：版本、配置或用途备注",
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )

                    // Lifecycle Status Selector (aligned with Web)
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(
                            text = "生命周期状态",
                            fontSize = 12.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textSecondary
                        )
                        VesperSegmentedTabs(
                            options = ASSET_STATUS_OPTIONS,
                            selectedIndex = ASSET_STATUS_OPTIONS.indexOf(status).coerceAtLeast(0),
                            onSelect = { idx -> status = ASSET_STATUS_OPTIONS[idx] }
                        )
                    }

                    VesperDivider()

                    // Structured Pricing
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
                        VesperTag(text = formattedPricePreview)
                    }

                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(
                            text = "币种 / 费用类型",
                            fontSize = 11.5.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textMuted
                        )
                        FlowRow(
                            horizontalArrangement = Arrangement.spacedBy(6.dp),
                            verticalArrangement = Arrangement.spacedBy(6.dp)
                        ) {
                            ASSET_CURRENCY_SPECS.forEach { cur ->
                                VesperFilterChip(
                                    label = cur.chipLabel,
                                    selected = currency == cur.code,
                                    onClick = {
                                        currency = cur.code
                                        if (cur.code == "FREE") amountNum = ""
                                    }
                                )
                            }
                        }
                    }

                    if (currency != "FREE") {
                        VesperTextField(
                            value = amountNum,
                            onValueChange = { amountNum = sanitizeAssetAmountInput(it) },
                            label = "金额数值 · ${selectedCurrencySpec.fullLabel}",
                            placeholder = "如 21.66",
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
                                VesperSecondaryButton(
                                    text = cycleLabel,
                                    tone = if (billingCycle == cycleVal) VesperButtonTone.ACCENT else VesperButtonTone.NEUTRAL,
                                    cornerRadius = 10.dp,
                                    minHeight = 42.dp,
                                    contentPadding = PaddingValues(horizontal = 6.dp, vertical = 6.dp),
                                    onClick = { billingCycle = cycleVal },
                                    modifier = Modifier.weight(1f)
                                )
                            }
                        }
                    }
                }
            }

            // =========================================================================
            // Section 2: Due Date, Permanent Mode, Alert Gradient & Auto Renew
            // =========================================================================
            VesperCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                Column(
                    modifier = Modifier.padding(14.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "到期守望与续费规则",
                            fontSize = 12.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.accentForeground
                        )
                        if (isPermanent) {
                            VesperTag(text = "长期有效", toneColor = theme.success)
                        } else if (previewDaysLeft != null) {
                            val badgeColor = when {
                                previewDaysLeft <= 0 -> theme.danger
                                previewDaysLeft <= 14 -> theme.warning
                                else -> theme.success
                            }
                            VesperTag(
                                text = if (previewDaysLeft <= 0) {
                                    "已逾期 ${abs(previewDaysLeft)} 天"
                                } else {
                                    "预计距今剩余 $previewDaysLeft 天"
                                },
                                toneColor = badgeColor
                            )
                        }
                    }

                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(
                            text = "到期类型",
                            fontSize = 12.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textSecondary
                        )
                        VesperSegmentedTabs(
                            options = listOf("指定具体到期日", "长期有效"),
                            selectedIndex = if (isPermanent) 1 else 0,
                            onSelect = { idx -> isPermanent = idx == 1 }
                        )
                    }

                    if (!isPermanent) {
                        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            Text(
                                text = "到期日（日历选择）",
                                fontSize = 12.sp,
                                fontWeight = FontWeight.Medium,
                                color = theme.textSecondary
                            )
                            Surface(
                                onClick = { showCalendarDialog = true },
                                shape = RoundedCornerShape(12.dp),
                                color = theme.surfaceInput,
                                border = BorderStroke(1.dp, theme.border),
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .sizeIn(minHeight = 48.dp)
                                        .padding(horizontal = 14.dp, vertical = 12.dp),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Text(
                                        text = dueDate.ifBlank { "选择日期" },
                                        fontFamily = FontFamily.Monospace,
                                        fontSize = 14.sp,
                                        color = if (dueDate.isBlank()) theme.textMuted else theme.textPrimary,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        modifier = Modifier.weight(1f)
                                    )
                                    Icon(
                                        imageVector = Icons.Default.CalendarMonth,
                                        contentDescription = "打开日历选择日期",
                                        tint = theme.accentForeground,
                                        modifier = Modifier.size(18.dp)
                                    )
                                }
                            }

                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Text(
                                    text = "一键顺延",
                                    fontSize = 12.sp,
                                    color = theme.textMuted
                                )
                                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                    listOf(1 to "+1月", 3 to "+1季", 12 to "+1年").forEach { (months, label) ->
                                        VesperFilterChip(
                                            label = label,
                                            selected = false,
                                            onClick = {
                                                val base = runCatching { LocalDate.parse(dueDate.take(10)) }
                                                    .getOrDefault(LocalDate.now())
                                                dueDate = base.plusMonths(months.toLong()).toString()
                                            }
                                        )
                                    }
                                }
                            }
                        }
                    }

                    // Alert Warning Days Gradient (aligned with Web)
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text(
                            text = "临期预警梯度",
                            fontSize = 12.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textSecondary
                        )
                        Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                            ASSET_WARN_OPTIONS.forEach { (value, label) ->
                                val chosen = warnDays == value
                                Surface(
                                    onClick = { warnDays = value },
                                    shape = RoundedCornerShape(11.dp),
                                    color = if (chosen) theme.accentSoft else theme.surfaceInput,
                                    border = BorderStroke(1.dp, if (chosen) theme.accentMuted else theme.border),
                                    modifier = Modifier.fillMaxWidth()
                                ) {
                                    Row(
                                        modifier = Modifier
                                            .fillMaxWidth()
                                            .sizeIn(minHeight = 46.dp)
                                            .padding(horizontal = 12.dp, vertical = 10.dp),
                                        horizontalArrangement = Arrangement.SpaceBetween,
                                        verticalAlignment = Alignment.CenterVertically
                                    ) {
                                        Text(
                                            text = label,
                                            fontSize = 13.sp,
                                            fontWeight = if (chosen) FontWeight.SemiBold else FontWeight.Normal,
                                            color = if (chosen) theme.accentForeground else theme.textPrimary
                                        )
                                        if (chosen) {
                                            VesperTag(text = "当前", toneColor = theme.accent)
                                        }
                                    }
                                }
                            }
                        }
                    }

                    VesperDivider()

                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 54.dp)
                            .clickable { autoRenew = !autoRenew }
                            .padding(vertical = 2.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Column(
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp),
                            verticalArrangement = Arrangement.spacedBy(2.dp)
                        ) {
                            Text(
                                text = "已开启自动续费扣款",
                                fontSize = 14.sp,
                                fontWeight = FontWeight.Medium,
                                color = theme.textPrimary
                            )
                            Text(
                                text = "开启后临期预警将提示核对自动扣款授权",
                                fontSize = 11.5.sp,
                                color = theme.textMuted
                            )
                        }
                        VesperSwitch(
                            checked = autoRenew,
                            onCheckedChange = { autoRenew = it }
                        )
                    }
                }
            }

            // =========================================================================
            // Section 3: Extensible Custom Key-Value Properties (extra_config)
            // =========================================================================
            VesperCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                Column(
                    modifier = Modifier.padding(14.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Column(
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 8.dp),
                            verticalArrangement = Arrangement.spacedBy(2.dp)
                        ) {
                            Text(
                                text = "可扩展自定义属性 (Key / Value)",
                                fontSize = 12.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.accentForeground
                            )
                            Text(
                                text = "记录元信息（如 host、region、registrar、panel_url）",
                                fontSize = 11.5.sp,
                                color = theme.textMuted
                            )
                        }
                        VesperSecondaryButton(
                            text = "添加属性",
                            leadingIcon = Icons.Default.Add,
                            tone = VesperButtonTone.ACCENT,
                            cornerRadius = 999.dp,
                            minHeight = 36.dp,
                            contentPadding = PaddingValues(horizontal = 11.dp, vertical = 6.dp),
                            onClick = { extraRows.add("" to "") }
                        )
                    }

                    if (extraRows.isEmpty()) {
                        Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                            Text(
                                text = "点击右上角「添加属性」或点下方常用键名快速添加：",
                                fontSize = 11.5.sp,
                                color = theme.textMuted
                            )
                            FlowRow(
                                horizontalArrangement = Arrangement.spacedBy(6.dp),
                                verticalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                EXTRA_PRESET_KEYS.forEach { presetKey ->
                                    VesperFilterChip(
                                        label = "+ $presetKey",
                                        selected = false,
                                        onClick = { extraRows.add(presetKey to "") }
                                    )
                                }
                            }
                        }
                    } else {
                        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                            extraRows.forEachIndexed { idx, (rowKey, rowVal) ->
                                val isUrl = rowVal.trim().startsWith("http://", ignoreCase = true) ||
                                    rowVal.trim().startsWith("https://", ignoreCase = true)
                                Column(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .clip(RoundedCornerShape(12.dp))
                                        .background(theme.surfaceControl)
                                        .border(1.dp, theme.border, RoundedCornerShape(12.dp))
                                        .padding(10.dp),
                                    verticalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    Row(
                                        modifier = Modifier.fillMaxWidth(),
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(6.dp)
                                    ) {
                                        VesperTextField(
                                            value = rowKey,
                                            onValueChange = { extraRows[idx] = it to rowVal },
                                            placeholder = "键名 (如 region)",
                                            singleLine = true,
                                            modifier = Modifier.weight(1f)
                                        )
                                        if (isUrl) {
                                            VesperIconButton(
                                                icon = Icons.AutoMirrored.Filled.OpenInNew,
                                                contentDescription = "打开链接",
                                                accent = true,
                                                onClick = {
                                                    runCatching {
                                                        context.startActivity(
                                                            Intent(Intent.ACTION_VIEW, Uri.parse(rowVal.trim()))
                                                        )
                                                    }
                                                }
                                            )
                                        }
                                        VesperIconButton(
                                            icon = Icons.Default.Close,
                                            contentDescription = "移除该属性",
                                            danger = true,
                                            onClick = { extraRows.removeAt(idx) }
                                        )
                                    }
                                    VesperTextField(
                                        value = rowVal,
                                        onValueChange = { extraRows[idx] = rowKey to it },
                                        placeholder = "属性值",
                                        singleLine = true,
                                        modifier = Modifier.fillMaxWidth()
                                    )
                                }
                            }
                        }
                    }
                }
            }

            // Action Footer
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                if (onDelete != null) {
                    VesperSecondaryButton(
                        text = "删除资产",
                        tone = VesperButtonTone.DANGER,
                        minHeight = 48.dp,
                        onClick = onDelete,
                        modifier = Modifier.weight(1f)
                    )
                }
                VesperPrimaryButton(
                    text = "保存资产",
                    enabled = name.isNotBlank(),
                    minHeight = 48.dp,
                    onClick = {
                        if (name.isNotBlank()) {
                            onSave(
                                name.trim(),
                                category.ifBlank { "general" },
                                spec.trim(),
                                formatAssetStructuredPrice(currency, amountNum, billingCycle),
                                status,
                                if (isPermanent) "2099-12-31" else dueDate.trim().ifEmpty { defaultDate },
                                autoRenew,
                                warnDays,
                                serializeExtraConfigRows(extraRows)
                            )
                        }
                    },
                    modifier = Modifier.weight(if (onDelete != null) 1.5f else 1f)
                )
            }

            Spacer(modifier = Modifier.height(10.dp))
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
