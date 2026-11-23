package uk.starryheavens.vesper.bothub.ui.screens.hub

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
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.CalendarMonth
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Refresh
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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.AssetCategoryItem
import uk.starryheavens.vesper.bothub.data.AssetItem
import uk.starryheavens.vesper.bothub.ui.components.ThemeCalendarDialog
import uk.starryheavens.vesper.bothub.ui.components.VesperButtonTone
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperEmptyState
import uk.starryheavens.vesper.bothub.ui.components.VesperFilterChip
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.VesperSwitch
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.VesperTextField
import uk.starryheavens.vesper.bothub.ui.components.assetCategoryIconVector
import uk.starryheavens.vesper.bothub.ui.components.vesperPressable
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

internal fun LazyListScope.hubAssetsItems(
    assets: List<AssetItem>,
    filteredAssets: List<AssetItem>,
    savedCategories: List<AssetCategoryItem>,
    mergedCategories: List<AssetCategoryItem>,
    categoryCounts: Map<String, Int>,
    selectedCategoryFilter: String,
    totalMonthlyCost: String,
    totalYearlyCost: String,
    expiringCount: Int,
    onSelectCategory: (String) -> Unit,
    onAddAsset: () -> Unit,
    onSyncAssets: () -> Unit,
    onEditAsset: (AssetItem) -> Unit,
    onQuickRenewAsset: (AssetItem, Int) -> Unit
) {
    item(key = "__hub_assets_hdr__") {
        val theme = BotHubThemeTokens
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            VesperCard(
                cornerRadius = 15.dp,
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 13.dp, vertical = 11.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column(
                        modifier = Modifier
                            .weight(1f)
                            .padding(end = 8.dp),
                        verticalArrangement = Arrangement.spacedBy(3.dp)
                    ) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(6.dp)
                        ) {
                            Text(
                                text = if (totalMonthlyCost != "--") "月均 $totalMonthlyCost" else "共 ${filteredAssets.size} 项资产",
                                fontSize = 14.5.sp,
                                fontWeight = FontWeight.Bold,
                                fontFamily = if (totalMonthlyCost != "--") FontFamily.Monospace else FontFamily.Default,
                                color = theme.textPrimary,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                            if (expiringCount > 0) {
                                VesperTag(text = "${expiringCount}项临期", toneColor = theme.warning)
                            }
                        }
                        Text(
                            text = if (totalYearlyCost != "--") {
                                "年化 $totalYearlyCost · 当前显示 ${filteredAssets.size} 项"
                            } else {
                                if (expiringCount > 0) "请留意近期到期项目" else "全部资产状态正常"
                            },
                            fontSize = 11.5.sp,
                            fontFamily = FontFamily.Monospace,
                            color = theme.textMuted,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }

                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        VesperSecondaryButton(
                            text = "添加",
                            leadingIcon = Icons.Default.Add,
                            cornerRadius = 999.dp,
                            minHeight = 36.dp,
                            contentPadding = PaddingValues(horizontal = 11.dp, vertical = 6.dp),
                            onClick = onAddAsset
                        )
                        VesperSecondaryButton(
                            text = "同步",
                            leadingIcon = Icons.Default.Refresh,
                            tone = VesperButtonTone.ACCENT,
                            cornerRadius = 999.dp,
                            minHeight = 36.dp,
                            contentPadding = PaddingValues(horizontal = 11.dp, vertical = 6.dp),
                            onClick = onSyncAssets
                        )
                    }
                }
            }

            if (mergedCategories.size > 1) {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    item(key = "__cat_all__") {
                        VesperFilterChip(
                            label = "全部",
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
                title = if (assets.isEmpty()) "暂无数字资产记录" else "当前分类下暂无资产",
                subtitle = if (assets.isEmpty()) {
                    "点击上方「添加」录入服务器、域名或订阅，支持自定义分类与到期提醒"
                } else ""
            )
        }
    }

    items(filteredAssets, key = { it.id }, contentType = { "asset_card" }) { asset ->
        HubAssetCard(
            asset = asset,
            savedCategories = savedCategories,
            showCategoryPrefix = selectedCategoryFilter == "all",
            onClick = { onEditAsset(asset) },
            onQuickRenew = { months -> onQuickRenewAsset(asset, months) }
        )
    }
}

@Composable
private fun HubAssetCard(
    asset: AssetItem,
    savedCategories: List<AssetCategoryItem>,
    showCategoryPrefix: Boolean,
    onClick: () -> Unit,
    onQuickRenew: (Int) -> Unit
) {
    val theme = BotHubThemeTokens
    val catMeta = remember(asset.category, savedCategories) {
        resolveAssetCategory(asset.category, savedCategories)
    }
    val isPermanent = asset.dueDate.startsWith("2099") || asset.daysLeft > 3650
    val isMultiYear = !isPermanent && asset.daysLeft > 365
    val barColor = when {
        isPermanent || isMultiYear -> theme.success
        asset.daysLeft <= 7 -> theme.danger
        asset.daysLeft <= 30 -> theme.warning
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

    VesperCard(
        cornerRadius = 15.dp,
        accentTone = if (asset.isExpiringSoon && !isPermanent) barColor else null,
        onClick = onClick,
        modifier = Modifier.fillMaxWidth()
    ) {
        Column(
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(7.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(9.dp),
                    modifier = Modifier
                        .weight(1f)
                        .padding(end = 8.dp)
                ) {
                    Box(
                        modifier = Modifier
                            .size(28.dp)
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
                    VesperTag(
                        text = if (isPermanent) "长期" else asset.dueBadgeLabel,
                        toneColor = barColor
                    )
                }
            }

            val subLeftText = when {
                asset.spec.isNotBlank() && showCategoryPrefix -> "${catMeta.label} · ${asset.spec}"
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
                    fontSize = 12.sp,
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
                                .sizeIn(minHeight = 28.dp)
                                .clip(RoundedCornerShape(7.dp))
                                .background(theme.accentSoft)
                                .border(0.8.dp, theme.accentMuted, RoundedCornerShape(7.dp))
                                .vesperPressable(pressedScale = 0.94f, hapticOnTap = true) {
                                    onQuickRenew(recMonths)
                                }
                                .padding(horizontal = 8.dp, vertical = 4.dp),
                            contentAlignment = Alignment.Center
                        ) {
                            Text(
                                text = "续期$recLabel",
                                fontSize = 10.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.accentForeground
                            )
                        }
                    }
                }
            }

            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(3.dp)
                    .clip(RoundedCornerShape(999.dp))
                    .background(theme.surfaceInput)
            ) {
                Box(
                    modifier = Modifier
                        .fillMaxWidth(progressFraction)
                        .height(3.dp)
                        .clip(RoundedCornerShape(999.dp))
                        .background(barColor)
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
    if (cleanNum.isEmpty()) return ""
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
                text = if (asset == null) "添加资产" else "编辑资产",
                fontSize = 18.sp,
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
                    VesperSecondaryButton(
                        text = if (showCategoryManager) "收起管理" else "扩展/管理",
                        leadingIcon = if (showCategoryManager) Icons.Default.Close else Icons.Default.Settings,
                        cornerRadius = 999.dp,
                        minHeight = 34.dp,
                        contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                        onClick = { showCategoryManager = !showCategoryManager }
                    )
                }

                if (availableCategories.isEmpty()) {
                    Text(
                        text = "当前暂无预设分类（将归入「通用资产」）；点击右上角「扩展/管理」可添加任意类型",
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
                                            .size(34.dp)
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
                label = "名称",
                placeholder = "例如：资产或服务名称",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            VesperTextField(
                value = spec,
                onValueChange = { spec = it },
                label = "说明（可选）",
                placeholder = "规格、型号或补充备注",
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )

            // Structured Pricing Card
            VesperCard(
                cornerRadius = 15.dp,
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier.padding(13.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "费用与计费周期",
                            fontSize = 13.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textPrimary
                        )
                        if (formattedPricePreview.isNotEmpty()) {
                            VesperTag(text = formattedPricePreview)
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
                                VesperSecondaryButton(
                                    text = cycleLabel,
                                    tone = if (billingCycle == cycleVal) VesperButtonTone.ACCENT else VesperButtonTone.NEUTRAL,
                                    cornerRadius = 10.dp,
                                    minHeight = 38.dp,
                                    contentPadding = PaddingValues(horizontal = 6.dp, vertical = 6.dp),
                                    onClick = { billingCycle = cycleVal },
                                    modifier = Modifier.weight(1f)
                                )
                            }
                        }
                    }
                }
            }

            // Calendar Date Trigger
            Column(
                verticalArrangement = Arrangement.spacedBy(6.dp),
                modifier = Modifier.fillMaxWidth()
            ) {
                Text(
                    text = "到期日（日历选择）",
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Medium,
                    color = theme.textMuted
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
                            text = when {
                                isPermanent -> "长期有效 / 免续费"
                                dueDate.isNotBlank() -> dueDate
                                else -> "选择日期"
                            },
                            fontFamily = if (!isPermanent && dueDate.isNotBlank()) FontFamily.Monospace else FontFamily.Default,
                            fontSize = 14.sp,
                            color = if (!isPermanent && dueDate.isBlank()) theme.textMuted else theme.textPrimary,
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
            }

            if (!isPermanent) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "基于到期日顺延",
                        fontSize = 12.sp,
                        color = theme.textMuted
                    )
                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        listOf(1 to "+1月", 3 to "+1季", 12 to "+1年").forEach { (months, label) ->
                            VesperFilterChip(
                                label = label,
                                selected = false,
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

            VesperCard(
                cornerRadius = 14.dp,
                onClick = { autoRenew = !autoRenew },
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
                    Text("自动续费", fontSize = 14.5.sp, color = theme.textPrimary)
                    VesperSwitch(
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
                    VesperSecondaryButton(
                        text = "删除",
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
