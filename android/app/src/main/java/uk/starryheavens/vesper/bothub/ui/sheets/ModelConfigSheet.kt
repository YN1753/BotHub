package uk.starryheavens.vesper.bothub.ui.sheets

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.AutoAwesome
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.ModelConfigPayload
import uk.starryheavens.vesper.bothub.data.ModelHealthItem
import uk.starryheavens.vesper.bothub.data.ModelOptionItem
import uk.starryheavens.vesper.bothub.data.ModelProviderItem
import uk.starryheavens.vesper.bothub.data.ModelUsageSummaryItem
import uk.starryheavens.vesper.bothub.data.PoolStrategiesItem
import uk.starryheavens.vesper.bothub.data.PooledModelItem
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.VesperButtonTone
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperDivider
import uk.starryheavens.vesper.bothub.ui.components.VesperEmptyState
import uk.starryheavens.vesper.bothub.ui.components.VesperIconButton
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSearchField
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.VesperSwitch
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.VesperTextField
import uk.starryheavens.vesper.bothub.ui.components.vesperPressable
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens
import java.util.Locale

private data class PublicProviderPresetItem(
    val key: String,
    val label: String,
    val name: String,
    val protocol: String,
    val baseUrl: String
)

private val ANDROID_PUBLIC_PROVIDER_PRESETS = listOf(
    PublicProviderPresetItem("openrouter", "OpenRouter", "OpenRouter", "openai", "https://openrouter.ai/api/v1"),
    PublicProviderPresetItem("siliconflow", "硅基流动", "硅基流动", "openai", "https://api.siliconflow.cn/v1"),
    PublicProviderPresetItem("deepseek", "DeepSeek", "DeepSeek 官方", "openai", "https://api.deepseek.com/v1"),
    PublicProviderPresetItem("gemini", "Google AI", "Google AI Studio", "gemini", "https://generativelanguage.googleapis.com/v1beta/openai"),
    PublicProviderPresetItem("groq", "Groq", "Groq", "openai", "https://api.groq.com/openai/v1"),
    PublicProviderPresetItem("ollama", "Ollama", "Ollama 本地", "openai", "http://127.0.0.1:11434/v1"),
    PublicProviderPresetItem("custom", "自定义", "自定义中转渠道", "openai", "")
)

private fun cleanBaseUrlAndroid(raw: String): String {
    var u = raw.trim().trimEnd('/')
    val suffixes = listOf("/chat/completions", "/completions", "/models")
    for (s in suffixes) {
        if (u.lowercase().endsWith(s)) {
            u = u.dropLast(s.length).trimEnd('/')
        }
    }
    return u
}

private fun formatTokenShort(tokens: Int): String {
    return when {
        tokens >= 1_000_000 -> String.format(Locale.US, "%.2fM", tokens / 1_000_000.0)
        tokens >= 1_000 -> String.format(Locale.US, "%.1fK", tokens / 1_000.0)
        else -> tokens.toString()
    }
}

private fun isLikelyFreeModelAndroid(id: String): Boolean {
    val lower = id.trim().lowercase()
    return lower.contains(":free") ||
        Regex("""(^|[-_/])free($|[-_/])""").containsMatchIn(lower) ||
        Regex("""(^|[-_/])zero($|[-_/])""").containsMatchIn(lower)
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ModelConfigSheet(
    providers: List<ModelProviderItem>,
    pooledModels: List<PooledModelItem>,
    strategies: PoolStrategiesItem,
    modelHealth: Map<String, ModelHealthItem>,
    usageSummary: ModelUsageSummaryItem,
    api: BotHubApiClient,
    onApplied: (ModelConfigPayload) -> Unit,
    onHealthUpdated: (Map<String, ModelHealthItem>, List<PooledModelItem>?, List<ModelOptionItem>?) -> Unit,
    onShowMessage: (String) -> Unit,
    onDismiss: () -> Unit
) {
    val theme = BotHubThemeTokens
    val scope = rememberCoroutineScope()
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)

    var activeTab by remember { mutableStateOf("providers") } // providers | free | paid
    var draftProviders by remember(providers) { mutableStateOf(providers) }
    var draftModels by remember(pooledModels) { mutableStateOf(pooledModels) }
    var draftStrategies by remember(strategies) { mutableStateOf(strategies) }
    var expandedProviderId by remember { mutableStateOf(providers.firstOrNull()?.id ?: "") }

    val fetchedModelsMap = remember { mutableStateMapOf<String, List<String>>() }
    val searchQueryMap = remember { mutableStateMapOf<String, String>() }
    val freeOnlyFilterMap = remember { mutableStateMapOf<String, Boolean>() }
    val manualInputMap = remember { mutableStateMapOf<String, String>() }
    val probeStatusMap = remember { mutableStateMapOf<String, Pair<Boolean, String>>() }

    var fetchingProviderId by remember { mutableStateOf("") }
    var probingTargetId by remember { mutableStateOf("") }
    var probingBatchTier by remember { mutableStateOf("") }
    var showUsageDetail by remember { mutableStateOf(false) }
    var saving by remember { mutableStateOf(false) }

    LaunchedEffect(Unit) {
        api.fetchModelConfig().onSuccess { payload ->
            onApplied(payload)
            draftProviders = payload.providers
            draftModels = payload.models
            draftStrategies = payload.strategies
            if (expandedProviderId.isBlank() && payload.providers.isNotEmpty()) {
                expandedProviderId = payload.providers.first().id
            }
        }
    }

    val freeList = remember(draftModels) {
        draftModels.filter { it.tier != "paid" }.sortedBy { it.priority }
    }
    val paidList = remember(draftModels) {
        draftModels.filter { it.tier == "paid" }.sortedBy { it.priority }
    }
    val providerById = remember(draftProviders) {
        draftProviders.associateBy { it.id }
    }

    fun assignModelToTier(providerId: String, modelId: String, targetTier: String) {
        val clean = modelId.trim()
        if (clean.isEmpty()) return
        val existing = draftModels.find { it.providerId == providerId && it.modelId == clean }
        draftModels = if (existing != null) {
            if (existing.tier == targetTier) {
                draftModels.filterNot { it.providerId == providerId && it.modelId == clean }
            } else {
                val nextPrio = draftModels.count { it.tier == targetTier } * 10 + 10
                draftModels.map {
                    if (it.providerId == providerId && it.modelId == clean) {
                        it.copy(tier = targetTier, priority = nextPrio)
                    } else it
                }
            }
        } else {
            val nextPrio = draftModels.count { it.tier == targetTier } * 10 + 10
            draftModels + PooledModelItem(
                providerId = providerId,
                modelId = clean,
                tier = targetTier,
                enabled = true,
                priority = nextPrio
            )
        }
    }

    fun moveModelPriority(tier: String, index: Int, delta: Int) {
        val current = (if (tier == "paid") paidList else freeList).toMutableList()
        val nextIdx = index + delta
        if (nextIdx !in current.indices) return
        val item = current.removeAt(index)
        current.add(nextIdx, item)
        val renumbered = current.mapIndexed { i, m -> m.copy(priority = (i + 1) * 10) }
        val others = draftModels.filter { it.tier != tier }
        draftModels = if (tier == "free") renumbered + others else others + renumbered
    }

    fun saveAll() {
        if (saving) return
        saving = true
        scope.launch {
            val cleanedProviders = draftProviders.map {
                it.copy(baseUrl = cleanBaseUrlAndroid(it.baseUrl))
            }
            api.saveModelConfig(cleanedProviders, draftModels, draftStrategies)
                .onSuccess { payload ->
                    onApplied(payload)
                    draftProviders = payload.providers
                    draftModels = payload.models
                    draftStrategies = payload.strategies
                    onShowMessage("模型渠道与双池配置已同步保存")
                }
                .onFailure { err ->
                    onShowMessage(err.message ?: "保存失败")
                }
            saving = false
        }
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
                .navigationBarsPadding()
                .padding(horizontal = 16.dp)
                .padding(bottom = 14.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            // Top Header
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "AI 模型渠道与双池",
                        fontSize = 18.sp,
                        fontWeight = FontWeight.Bold,
                        color = theme.textPrimary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = "渠道共享密钥 · 免费池与付费池独立调度",
                        fontSize = 11.5.sp,
                        color = theme.textMuted,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
                VesperPrimaryButton(
                    text = if (saving) "保存中" else "保存",
                    enabled = !saving,
                    minHeight = 48.dp,
                    contentPadding = PaddingValues(horizontal = 16.dp, vertical = 10.dp),
                    onClick = { saveAll() }
                )
                VesperIconButton(
                    icon = Icons.Default.Close,
                    contentDescription = "关闭模型配置",
                    onClick = onDismiss
                )
            }

            // 7-Day Telemetry Summary Card
            val hasUsageDetail = usageSummary.topModels.isNotEmpty() || usageSummary.topCallers.isNotEmpty()
            VesperCard(
                cornerRadius = 14.dp,
                onClick = if (hasUsageDetail) {
                    { showUsageDetail = !showUsageDetail }
                } else null,
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 14.dp, vertical = 11.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Column(modifier = Modifier.weight(1f)) {
                            Text("7日免费池", fontSize = 11.sp, color = theme.success)
                            Text(
                                text = "${usageSummary.freeCalls}次 · ${formatTokenShort(usageSummary.freeTokens)}",
                                fontSize = 12.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                fontFamily = FontFamily.Monospace,
                                color = theme.textPrimary
                            )
                        }
                        Column(modifier = Modifier.weight(1f)) {
                            Text("7日付费池", fontSize = 11.sp, color = theme.accentForeground)
                            Text(
                                text = "${usageSummary.paidCalls}次 · ${formatTokenShort(usageSummary.paidTokens)}",
                                fontSize = 12.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                fontFamily = FontFamily.Monospace,
                                color = theme.textPrimary
                            )
                        }
                        Column(horizontalAlignment = Alignment.End) {
                            Text("成功率", fontSize = 11.sp, color = theme.textMuted)
                            val rate = if (usageSummary.totalCalls > 0) {
                                "${(usageSummary.successCalls * 100) / usageSummary.totalCalls}%"
                            } else "--"
                            Text(
                                text = rate,
                                fontSize = 12.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                fontFamily = FontFamily.Monospace,
                                color = theme.textPrimary
                            )
                        }
                    }

                    if (hasUsageDetail) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(4.dp)
                        ) {
                            Text(
                                text = if (showUsageDetail) "收起按模型/按Bot消耗明细" else "展开 7日按模型 / 按Bot与任务明细",
                                fontSize = 11.sp,
                                fontWeight = FontWeight.Medium,
                                color = theme.accentForeground
                            )
                            Icon(
                                imageVector = if (showUsageDetail) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown,
                                contentDescription = null,
                                tint = theme.accentForeground,
                                modifier = Modifier.size(16.dp)
                            )
                        }
                    }

                    if (showUsageDetail) {
                        if (usageSummary.topModels.isNotEmpty()) {
                            VesperDivider()
                            Text(
                                text = "按模型调用排行",
                                fontSize = 11.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textSecondary
                            )
                            usageSummary.topModels.take(6).forEach { m ->
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween
                                ) {
                                    Text(
                                        text = m.modelId,
                                        fontSize = 11.sp,
                                        fontFamily = FontFamily.Monospace,
                                        color = theme.textPrimary,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        modifier = Modifier
                                            .weight(1f)
                                            .padding(end = 8.dp)
                                    )
                                    Text(
                                        text = "${m.calls}次 · ${formatTokenShort(m.tokens)}",
                                        fontSize = 11.sp,
                                        fontFamily = FontFamily.Monospace,
                                        color = if (m.tier == "paid") theme.accentForeground else theme.success
                                    )
                                }
                            }
                        }
                        if (usageSummary.topCallers.isNotEmpty()) {
                            VesperDivider()
                            Text(
                                text = "按 Bot / 任务消耗排行",
                                fontSize = 11.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textSecondary
                            )
                            usageSummary.topCallers.take(6).forEach { c ->
                                val callerLabel = when {
                                    c.taskId.isNotBlank() -> "任务:${c.taskId}"
                                    c.botId.isNotBlank() -> "Bot:${c.botId}"
                                    else -> if (c.source == "routine") "后台例程" else "系统轻模型"
                                }
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween
                                ) {
                                    Text(
                                        text = callerLabel,
                                        fontSize = 11.sp,
                                        color = theme.textPrimary,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        modifier = Modifier
                                            .weight(1f)
                                            .padding(end = 8.dp)
                                    )
                                    Text(
                                        text = "${c.freeCalls}免/${c.paidCalls}付 · ${formatTokenShort(c.tokens)}",
                                        fontSize = 11.sp,
                                        fontFamily = FontFamily.Monospace,
                                        color = if (c.paidCalls > 0) theme.accentForeground else theme.success
                                    )
                                }
                            }
                        }
                    }
                }
            }

            // 3-Tab Segmented Row (48dp touch height)
            PoolSegmentedControl(
                options = listOf(
                    "providers" to "渠道 (${draftProviders.size})",
                    "free" to "免费池 (${freeList.size})",
                    "paid" to "付费池 (${paidList.size})"
                ),
                selectedId = activeTab,
                onSelect = { activeTab = it }
            )

            // Scrollable Body
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .heightIn(max = 520.dp)
                    .verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                if (activeTab == "providers") {
                    Text(
                        text = "快速添加渠道（仅填入公开标准地址）",
                        fontSize = 11.5.sp,
                        fontWeight = FontWeight.Medium,
                        color = theme.textMuted,
                        modifier = Modifier.padding(start = 2.dp)
                    )
                    LazyRow(
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        items(ANDROID_PUBLIC_PROVIDER_PRESETS, key = { it.key }) { preset ->
                            val chipShape = RoundedCornerShape(10.dp)
                            Box(
                                modifier = Modifier
                                    .sizeIn(minWidth = 48.dp, minHeight = 48.dp)
                                    .vesperPressable(pressedScale = 0.96f, hapticOnTap = true) {
                                        val suffix = (System.currentTimeMillis() % 10000).toString(36)
                                        val newId = "${preset.key}_$suffix"
                                        draftProviders = draftProviders + ModelProviderItem(
                                            id = newId,
                                            name = preset.name,
                                            protocol = preset.protocol,
                                            baseUrl = preset.baseUrl,
                                            apiKey = "",
                                            enabled = true
                                        )
                                        expandedProviderId = newId
                                    },
                                contentAlignment = Alignment.Center
                            ) {
                                Row(
                                    modifier = Modifier
                                        .clip(chipShape)
                                        .background(theme.surfaceControl, chipShape)
                                        .border(1.dp, theme.border, chipShape)
                                        .padding(horizontal = 11.dp, vertical = 8.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                                ) {
                                    Icon(
                                        imageVector = Icons.Default.Add,
                                        contentDescription = null,
                                        tint = theme.accentForeground,
                                        modifier = Modifier.size(14.dp)
                                    )
                                    Text(
                                        text = preset.label,
                                        style = TextStyle(
                                            fontSize = 12.sp,
                                            fontWeight = FontWeight.Medium,
                                            color = theme.textPrimary,
                                            platformStyle = NoFontPaddingStyle
                                        ),
                                        maxLines = 1,
                                        softWrap = false
                                    )
                                }
                            }
                        }
                    }

                    if (draftProviders.isEmpty()) {
                        VesperEmptyState(
                            icon = Icons.Default.Dns,
                            title = "尚未添加模型渠道",
                            subtitle = "点击上方模板填入 API Key 后，可实测拉取可用模型并一键分配至免费池或付费池。"
                        )
                    }

                    draftProviders.forEach { prov ->
                        val isExpanded = expandedProviderId == prov.id
                        val freeCount = draftModels.count { it.providerId == prov.id && it.tier != "paid" }
                        val paidCount = draftModels.count { it.providerId == prov.id && it.tier == "paid" }

                        VesperCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                            Column(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(horizontal = 14.dp, vertical = 10.dp),
                                verticalArrangement = Arrangement.spacedBy(10.dp)
                            ) {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    Row(
                                        modifier = Modifier
                                            .weight(1f)
                                            .sizeIn(minHeight = 48.dp)
                                            .vesperPressable {
                                                expandedProviderId = if (isExpanded) "" else prov.id
                                            },
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                                    ) {
                                        Box(
                                            modifier = Modifier
                                                .size(8.dp)
                                                .background(
                                                    if (prov.enabled && prov.apiKey.isNotBlank()) theme.success else theme.textMuted,
                                                    CircleShape
                                                )
                                        )
                                        Column(
                                            modifier = Modifier.weight(1f),
                                            verticalArrangement = Arrangement.spacedBy(2.dp)
                                        ) {
                                            Text(
                                                text = prov.name.ifBlank { prov.id },
                                                fontSize = 14.sp,
                                                fontWeight = FontWeight.SemiBold,
                                                color = theme.textPrimary,
                                                maxLines = 1,
                                                overflow = TextOverflow.Ellipsis
                                            )
                                            val keyCount = prov.effectiveApiKeys().size
                                            val keyTag = if (keyCount > 1) " · $keyCount 密钥" else ""
                                            Text(
                                                text = "${if (prov.protocol == "gemini") "Gemini" else "OpenAI 兼容"}$keyTag · 免费 $freeCount / 付费 $paidCount",
                                                fontSize = 11.5.sp,
                                                color = theme.textMuted,
                                                maxLines = 1,
                                                overflow = TextOverflow.Ellipsis
                                            )
                                        }
                                        Icon(
                                            imageVector = if (isExpanded) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown,
                                            contentDescription = if (isExpanded) "收起渠道" else "展开渠道",
                                            tint = theme.textMuted,
                                            modifier = Modifier.size(18.dp)
                                        )
                                    }

                                    VesperSwitch(
                                        checked = prov.enabled,
                                        onCheckedChange = { checked ->
                                            draftProviders = draftProviders.map {
                                                if (it.id == prov.id) it.copy(enabled = checked) else it
                                            }
                                        }
                                    )
                                }

                                if (isExpanded) {
                                    VesperDivider()

                                    VesperTextField(
                                        value = prov.name,
                                        onValueChange = { v ->
                                            draftProviders = draftProviders.map {
                                                if (it.id == prov.id) it.copy(name = v) else it
                                            }
                                        },
                                        label = "渠道名称",
                                        placeholder = "例如 OpenRouter / 硅基流动"
                                    )

                                    PoolSegmentedControl(
                                        options = listOf(
                                            "openai" to "OpenAI 兼容",
                                            "gemini" to "Google Gemini"
                                        ),
                                        selectedId = prov.protocol,
                                        onSelect = { proto ->
                                            draftProviders = draftProviders.map {
                                                if (it.id == prov.id) it.copy(protocol = proto) else it
                                            }
                                        }
                                    )

                                    VesperTextField(
                                        value = prov.baseUrl,
                                        onValueChange = { v ->
                                            draftProviders = draftProviders.map {
                                                if (it.id == prov.id) it.copy(baseUrl = v) else it
                                            }
                                        },
                                        label = "Base URL",
                                        placeholder = "https://api.example.com/v1"
                                    )

                                    val rawKeysText = remember(prov.id, prov.apiKey, prov.apiKeys) {
                                        mutableStateOf(prov.effectiveApiKeys().joinToString("\n"))
                                    }
                                    VesperTextField(
                                        value = rawKeysText.value,
                                        onValueChange = { v ->
                                            rawKeysText.value = v
                                            val parsed = v.split('\n', '\r', ',', ';')
                                                .map { it.trim() }
                                                .filter { it.isNotEmpty() }
                                                .distinct()
                                            draftProviders = draftProviders.map {
                                                if (it.id == prov.id) {
                                                    it.copy(
                                                        apiKey = parsed.firstOrNull().orEmpty(),
                                                        apiKeys = parsed
                                                    )
                                                } else it
                                            }
                                        },
                                        label = "API Keys（每行一个，失败自动切换下一个）",
                                        placeholder = "sk-... / AIza...\n可换行粘贴多个密钥",
                                        singleLine = false,
                                        maxLines = 4
                                    )

                                    Row(
                                        modifier = Modifier.fillMaxWidth(),
                                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                                    ) {
                                        VesperSecondaryButton(
                                            text = if (fetchingProviderId == prov.id) "实测拉取中..." else "拉取可用模型",
                                            tone = VesperButtonTone.ACCENT,
                                            enabled = fetchingProviderId.isEmpty(),
                                            minHeight = 48.dp,
                                            onClick = {
                                                val cleanUrl = cleanBaseUrlAndroid(prov.baseUrl)
                                                val effKeys = prov.effectiveApiKeys()
                                                if (cleanUrl.isBlank() || effKeys.isEmpty()) {
                                                    onShowMessage("请先填写 Base URL 与至少一个 API Key")
                                                    return@VesperSecondaryButton
                                                }
                                                fetchingProviderId = prov.id
                                                scope.launch {
                                                    api.fetchUpstreamModels(prov.id, prov.protocol, cleanUrl, effKeys, verify = true)
                                                        .onSuccess { res ->
                                                            fetchedModelsMap[prov.id] = res.models
                                                            val filteredTotal = res.filteredOutCount + res.failedVerifyCount
                                                            val note = if (filteredTotal > 0) "（已拦截 $filteredTotal 个不可用/非对话模型）" else ""
                                                            onShowMessage("实测拉取成功：共 ${res.models.size} 个可用模型$note")
                                                        }
                                                        .onFailure { err ->
                                                            onShowMessage(err.message ?: "拉取失败")
                                                        }
                                                    fetchingProviderId = ""
                                                }
                                            },
                                            modifier = Modifier.weight(1f)
                                        )
                                        VesperSecondaryButton(
                                            text = "删除渠道",
                                            tone = VesperButtonTone.DANGER,
                                            minHeight = 48.dp,
                                            onClick = {
                                                draftProviders = draftProviders.filterNot { it.id == prov.id }
                                                draftModels = draftModels.filterNot { it.providerId == prov.id }
                                            }
                                        )
                                    }

                                    // Manual model ID quick-add
                                    val manualVal = manualInputMap[prov.id].orEmpty()
                                    VesperTextField(
                                        value = manualVal,
                                        onValueChange = { manualInputMap[prov.id] = it },
                                        label = "手动输入模型 ID",
                                        placeholder = "如 deepseek-chat（逗号分隔）"
                                    )
                                    Row(
                                        modifier = Modifier.fillMaxWidth(),
                                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                                    ) {
                                        VesperSecondaryButton(
                                            text = "加入免费池",
                                            leadingIcon = Icons.Default.Add,
                                            minHeight = 48.dp,
                                            onClick = {
                                                val parts = manualVal.split(",", "，", "\n").map { it.trim() }.filter { it.isNotEmpty() }
                                                parts.forEach { mid -> assignModelToTier(prov.id, mid, "free") }
                                                manualInputMap[prov.id] = ""
                                            },
                                            modifier = Modifier.weight(1f)
                                        )
                                        VesperSecondaryButton(
                                            text = "加入付费池",
                                            leadingIcon = Icons.Default.Add,
                                            minHeight = 48.dp,
                                            onClick = {
                                                val parts = manualVal.split(",", "，", "\n").map { it.trim() }.filter { it.isNotEmpty() }
                                                parts.forEach { mid -> assignModelToTier(prov.id, mid, "paid") }
                                                manualInputMap[prov.id] = ""
                                            },
                                            modifier = Modifier.weight(1f)
                                        )
                                    }

                                    // Searchable Fetched Model List
                                    val fetched = fetchedModelsMap[prov.id]
                                    if (!fetched.isNullOrEmpty()) {
                                        val query = searchQueryMap[prov.id].orEmpty()
                                        val onlyFreeTag = freeOnlyFilterMap[prov.id] == true
                                        VesperSearchField(
                                            value = query,
                                            onValueChange = { searchQueryMap[prov.id] = it },
                                            placeholder = "搜索模型名（共 ${fetched.size} 个）..."
                                        )
                                        val likelyFreeList = fetched.filter { isLikelyFreeModelAndroid(it) }
                                        if (likelyFreeList.isNotEmpty()) {
                                            Row(
                                                modifier = Modifier
                                                    .fillMaxWidth()
                                                    .horizontalScroll(rememberScrollState()),
                                                horizontalArrangement = Arrangement.spacedBy(8.dp)
                                            ) {
                                                PoolActionPill(
                                                    text = if (onlyFreeTag) "已筛选免费 (${likelyFreeList.size})" else "仅看免费 (${likelyFreeList.size})",
                                                    active = onlyFreeTag,
                                                    toneColor = theme.success,
                                                    onClick = { freeOnlyFilterMap[prov.id] = !onlyFreeTag }
                                                )
                                                PoolActionPill(
                                                    text = "+ 一键免费加池",
                                                    active = true,
                                                    toneColor = theme.success,
                                                    onClick = {
                                                        var addedCount = 0
                                                        likelyFreeList.take(40).forEach { mid ->
                                                            if (draftModels.none { it.providerId == prov.id && it.modelId == mid }) {
                                                                assignModelToTier(prov.id, mid, "free")
                                                                addedCount += 1
                                                            }
                                                        }
                                                        onShowMessage(
                                                            if (addedCount > 0) "已将 $addedCount 个免费模型加入免费池" else "免费模型已在池中"
                                                        )
                                                    }
                                                )
                                            }
                                        }

                                        val filtered = fetched.filter { id ->
                                            val qMatch = query.isBlank() || id.contains(query.trim(), ignoreCase = true)
                                            val fMatch = !onlyFreeTag || isLikelyFreeModelAndroid(id)
                                            qMatch && fMatch
                                        }.take(40)

                                        Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                                            filtered.forEach { mid ->
                                                val curTier = draftModels.find { it.providerId == prov.id && it.modelId == mid }?.tier.orEmpty()
                                                Row(
                                                    modifier = Modifier
                                                        .fillMaxWidth()
                                                        .clip(RoundedCornerShape(10.dp))
                                                        .background(theme.surfaceInput, RoundedCornerShape(10.dp))
                                                        .border(1.dp, theme.border, RoundedCornerShape(10.dp))
                                                        .padding(start = 10.dp, end = 4.dp),
                                                    verticalAlignment = Alignment.CenterVertically,
                                                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                                                ) {
                                                    Text(
                                                        text = mid,
                                                        fontSize = 11.5.sp,
                                                        fontFamily = FontFamily.Monospace,
                                                        color = theme.textPrimary,
                                                        maxLines = 1,
                                                        overflow = TextOverflow.Ellipsis,
                                                        modifier = Modifier.weight(1f)
                                                    )
                                                    TierAssignPill(
                                                        label = if (curTier == "free") "已选免" else "免费",
                                                        active = curTier == "free",
                                                        activeColor = theme.success,
                                                        onClick = { assignModelToTier(prov.id, mid, "free") }
                                                    )
                                                    TierAssignPill(
                                                        label = if (curTier == "paid") "已选付" else "付费",
                                                        active = curTier == "paid",
                                                        activeColor = theme.accent,
                                                        onClick = { assignModelToTier(prov.id, mid, "paid") }
                                                    )
                                                }
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                } else {
                    val tier = if (activeTab == "paid") "paid" else "free"
                    val list = if (tier == "paid") paidList else freeList
                    val curStrategy = if (tier == "paid") draftStrategies.paid else draftStrategies.free

                    // Pool Strategy & Batch Actions Card
                    VesperCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                        Column(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(horizontal = 14.dp, vertical = 12.dp),
                            verticalArrangement = Arrangement.spacedBy(10.dp)
                        ) {
                            Text(
                                text = if (tier == "paid") "付费池调度策略" else "免费池调度策略",
                                fontSize = 13.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textPrimary
                            )

                            PoolSegmentedControl(
                                options = listOf(
                                    "priority" to "按优先级",
                                    "round_robin" to "轮询均摊",
                                    "lowest_latency" to "最快延迟"
                                ),
                                selectedId = curStrategy,
                                onSelect = { sid ->
                                    draftStrategies = if (tier == "paid") {
                                        draftStrategies.copy(paid = sid)
                                    } else {
                                        draftStrategies.copy(free = sid)
                                    }
                                }
                            )

                            val anyCooling = list.any { (modelHealth[it.canonicalTargetId]?.cooldownSec ?: 0L) > 0L }
                            val anyFailedProbe = list.any { it.enabled && probeStatusMap[it.canonicalTargetId]?.first == false }
                            if (list.isNotEmpty() || anyCooling || anyFailedProbe) {
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .horizontalScroll(rememberScrollState()),
                                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    if (list.isNotEmpty()) {
                                        PoolActionPill(
                                            text = if (probingBatchTier == tier) "测速中..." else "并发测速",
                                            active = true,
                                            toneColor = theme.success,
                                            enabled = probingBatchTier.isEmpty(),
                                            onClick = {
                                                if (probingBatchTier.isNotEmpty()) return@PoolActionPill
                                                probingBatchTier = tier
                                                scope.launch {
                                                    api.testModelBatch(tier = tier, disableFailed = false)
                                                        .onSuccess { batch ->
                                                            var okCnt = 0
                                                            var failCnt = 0
                                                            batch.results.forEach { r ->
                                                                if (r.ok) {
                                                                    okCnt += 1
                                                                    val cap = buildString {
                                                                        append("${r.latencyMs}ms OK")
                                                                        if (r.supportsTools == true) append(" · Tools可用")
                                                                        else if (r.supportsTools == false) append(" · 无Tools")
                                                                    }
                                                                    probeStatusMap[r.targetId] = true to cap
                                                                    draftModels = draftModels.map { m ->
                                                                        if (m.providerId == r.providerId && m.modelId == r.modelId) {
                                                                            m.copy(
                                                                                supportsTools = r.supportsTools ?: m.supportsTools,
                                                                                supportsThinking = r.supportsThinking ?: m.supportsThinking,
                                                                                lastLatencyMs = if (r.latencyMs > 0) r.latencyMs else m.lastLatencyMs
                                                                            )
                                                                        } else m
                                                                    }
                                                                } else {
                                                                    failCnt += 1
                                                                    probeStatusMap[r.targetId] = false to r.error.ifBlank { "探测失败" }
                                                                }
                                                            }
                                                            onHealthUpdated(batch.modelHealth, batch.pooledModels, batch.modelOptions)
                                                            onShowMessage("并发测速完成：$okCnt 个可用" + if (failCnt > 0) "，$failCnt 个异常" else "")
                                                        }
                                                        .onFailure { err ->
                                                            onShowMessage(err.message ?: "批量测速失败")
                                                        }
                                                    probingBatchTier = ""
                                                }
                                            }
                                        )
                                    }
                                    if (anyCooling) {
                                        PoolActionPill(
                                            text = "全部解封",
                                            active = true,
                                            toneColor = theme.accent,
                                            onClick = {
                                                scope.launch {
                                                    api.clearModelCooldown("").onSuccess { nextHealth ->
                                                        onHealthUpdated(nextHealth, null, null)
                                                        onShowMessage("已解除全部冷却状态")
                                                    }
                                                }
                                            }
                                        )
                                    }
                                    if (anyFailedProbe) {
                                        PoolActionPill(
                                            text = "停用异常",
                                            active = true,
                                            toneColor = theme.danger,
                                            onClick = {
                                                var disabledCount = 0
                                                draftModels = draftModels.map { m ->
                                                    if (m.tier == tier && m.enabled && probeStatusMap[m.canonicalTargetId]?.first == false) {
                                                        disabledCount += 1
                                                        m.copy(enabled = false)
                                                    } else m
                                                }
                                                onShowMessage("已停用 $disabledCount 个异常模型，记得点击右上角「保存」")
                                            }
                                        )
                                    }
                                }
                            }
                        }
                    }

                    if (list.isEmpty()) {
                        VesperEmptyState(
                            icon = Icons.Default.AutoAwesome,
                            title = if (tier == "paid") "付费池暂无模型" else "免费池暂无模型",
                            subtitle = "前往「渠道」页拉取或手动输入模型 ID 后即可一键分配。",
                            actions = {
                                VesperSecondaryButton(
                                    text = "去渠道管理添加",
                                    tone = VesperButtonTone.ACCENT,
                                    minHeight = 48.dp,
                                    onClick = { activeTab = "providers" }
                                )
                            }
                        )
                    }

                    list.forEachIndexed { idx, m ->
                        val prov = providerById[m.providerId]
                        val health = modelHealth[m.canonicalTargetId]
                        val probeNote = probeStatusMap[m.canonicalTargetId]
                        val isCooling = (health?.cooldownSec ?: 0L) > 0L

                        VesperCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                            Column(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(horizontal = 14.dp, vertical = 10.dp),
                                verticalArrangement = Arrangement.spacedBy(8.dp)
                            ) {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    Column(
                                        modifier = Modifier.weight(1f),
                                        verticalArrangement = Arrangement.spacedBy(2.dp)
                                    ) {
                                        Row(
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(6.dp)
                                        ) {
                                            VesperTag(
                                                text = "#${idx + 1}",
                                                bgColor = theme.surfaceInput,
                                                borderColor = theme.border,
                                                textColor = theme.textSecondary
                                            )
                                            Text(
                                                text = prov?.name ?: m.providerId,
                                                fontSize = 11.5.sp,
                                                color = theme.textMuted,
                                                maxLines = 1,
                                                overflow = TextOverflow.Ellipsis
                                            )
                                        }
                                        Text(
                                            text = m.modelId,
                                            fontSize = 13.5.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            fontFamily = FontFamily.Monospace,
                                            color = theme.textPrimary,
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis
                                        )
                                    }
                                    VesperSwitch(
                                        checked = m.enabled,
                                        onCheckedChange = { checked ->
                                            draftModels = draftModels.map {
                                                if (it.providerId == m.providerId && it.modelId == m.modelId) {
                                                    it.copy(enabled = checked)
                                                } else it
                                            }
                                        }
                                    )
                                }

                                // Capability & Health Tag Strip
                                val hasAnyBadge = m.supportsTools != null ||
                                    m.supportsThinking == true ||
                                    isCooling ||
                                    (health?.lastLatencyMs ?: 0L) > 0L ||
                                    m.lastLatencyMs > 0L

                                if (hasAnyBadge) {
                                    Row(
                                        modifier = Modifier
                                            .fillMaxWidth()
                                            .horizontalScroll(rememberScrollState()),
                                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                                        verticalAlignment = Alignment.CenterVertically
                                    ) {
                                        if (m.supportsTools == true) {
                                            VesperTag(text = "Tools可用", toneColor = theme.success)
                                        } else if (m.supportsTools == false) {
                                            VesperTag(text = "无Tools", toneColor = theme.textMuted)
                                        }
                                        if (m.supportsThinking == true) {
                                            VesperTag(text = "Thinking", toneColor = theme.info)
                                        }
                                        when {
                                            isCooling -> VesperTag(
                                                text = "冷却 ${health?.cooldownSec}s",
                                                toneColor = theme.warning
                                            )
                                            (health?.lastLatencyMs ?: 0L) > 0L -> VesperTag(
                                                text = "正常 ${health?.lastLatencyMs}ms",
                                                toneColor = theme.success
                                            )
                                            m.lastLatencyMs > 0L -> VesperTag(
                                                text = "上次 ${m.lastLatencyMs}ms",
                                                toneColor = theme.textSecondary
                                            )
                                        }
                                    }
                                }

                                if (probeNote != null) {
                                    Text(
                                        text = probeNote.second,
                                        fontSize = 11.5.sp,
                                        color = if (probeNote.first) theme.success else theme.danger
                                    )
                                }

                                VesperDivider()

                                // Row Actions (all >= 48dp touch targets)
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .horizontalScroll(rememberScrollState()),
                                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    PoolActionPill(
                                        text = if (probingTargetId == m.canonicalTargetId) "测速中" else "测速",
                                        enabled = probingTargetId.isEmpty(),
                                        onClick = {
                                            if (prov == null || probingTargetId.isNotEmpty()) return@PoolActionPill
                                            probingTargetId = m.canonicalTargetId
                                            scope.launch {
                                                api.probePooledModel(prov, m.modelId)
                                                    .onSuccess { res ->
                                                        if (res.ok) {
                                                            val capStr = buildString {
                                                                append("${res.latencyMs}ms OK")
                                                                if (res.supportsTools == true) append(" · Tools可用")
                                                                else if (res.supportsTools == false) append(" · 无Tools")
                                                            }
                                                            probeStatusMap[m.canonicalTargetId] = true to capStr
                                                            draftModels = draftModels.map {
                                                                if (it.providerId == m.providerId && it.modelId == m.modelId) {
                                                                    it.copy(
                                                                        supportsTools = res.supportsTools ?: it.supportsTools,
                                                                        supportsThinking = res.supportsThinking ?: it.supportsThinking,
                                                                        lastLatencyMs = if (res.latencyMs > 0) res.latencyMs else it.lastLatencyMs
                                                                    )
                                                                } else it
                                                            }
                                                        } else {
                                                            probeStatusMap[m.canonicalTargetId] = false to res.error.ifBlank { "探测失败" }
                                                        }
                                                        onHealthUpdated(res.modelHealth, res.pooledModels, res.modelOptions)
                                                    }
                                                    .onFailure { err ->
                                                        probeStatusMap[m.canonicalTargetId] = false to (err.message ?: "连接失败")
                                                    }
                                                probingTargetId = ""
                                            }
                                        }
                                    )
                                    if (isCooling) {
                                        PoolActionPill(
                                            text = "解封",
                                            active = true,
                                            toneColor = theme.accent,
                                            onClick = {
                                                scope.launch {
                                                    api.clearModelCooldown(m.canonicalTargetId).onSuccess { nextH ->
                                                        onHealthUpdated(nextH, null, null)
                                                        onShowMessage("已解封 ${m.modelId}")
                                                    }
                                                }
                                            }
                                        )
                                    }
                                    PoolActionPill(
                                        text = if (tier == "paid") "移至免费" else "移至付费",
                                        onClick = {
                                            val otherTier = if (tier == "paid") "free" else "paid"
                                            assignModelToTier(m.providerId, m.modelId, otherTier)
                                        }
                                    )
                                    if (idx > 0) {
                                        PoolIconActionPill(
                                            icon = Icons.Default.KeyboardArrowUp,
                                            contentDescription = "上移优先级",
                                            onClick = { moveModelPriority(tier, idx, -1) }
                                        )
                                    }
                                    if (idx < list.lastIndex) {
                                        PoolIconActionPill(
                                            icon = Icons.Default.KeyboardArrowDown,
                                            contentDescription = "下移优先级",
                                            onClick = { moveModelPriority(tier, idx, 1) }
                                        )
                                    }
                                    PoolActionPill(
                                        text = "移除",
                                        active = true,
                                        toneColor = theme.danger,
                                        onClick = {
                                            draftModels = draftModels.filterNot {
                                                it.providerId == m.providerId && it.modelId == m.modelId
                                            }
                                        }
                                    )
                                }
                            }
                        }
                    }
                }
                Spacer(modifier = Modifier.height(6.dp))
            }
        }
    }
}

@Composable
private fun PoolSegmentedControl(
    options: List<Pair<String, String>>,
    selectedId: String,
    onSelect: (String) -> Unit
) {
    val theme = BotHubThemeTokens
    val outerShape = RoundedCornerShape(12.dp)
    val tabShape = RoundedCornerShape(9.dp)
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(outerShape)
            .background(theme.surfaceInput, outerShape)
            .border(1.dp, theme.border, outerShape)
            .padding(3.dp),
        horizontalArrangement = Arrangement.spacedBy(3.dp)
    ) {
        options.forEach { (id, label) ->
            val selected = selectedId == id
            Box(
                modifier = Modifier
                    .weight(1f)
                    .sizeIn(minWidth = 48.dp, minHeight = 48.dp)
                    .vesperPressable(pressedScale = 0.97f, hapticOnTap = true) {
                        if (!selected) onSelect(id)
                    }
                    .clip(tabShape)
                    .then(
                        if (selected) {
                            Modifier
                                .background(theme.surfaceCard, tabShape)
                                .border(1.dp, theme.borderStrong, tabShape)
                        } else {
                            Modifier
                        }
                    )
                    .padding(horizontal = 6.dp, vertical = 10.dp),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = label,
                    style = TextStyle(
                        fontSize = 12.sp,
                        fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                        color = if (selected) theme.textPrimary else theme.textMuted,
                        platformStyle = NoFontPaddingStyle
                    ),
                    maxLines = 1,
                    softWrap = false,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Composable
private fun TierAssignPill(
    label: String,
    active: Boolean,
    activeColor: Color,
    onClick: () -> Unit
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(8.dp)
    Box(
        modifier = Modifier
            .sizeIn(minWidth = 48.dp, minHeight = 48.dp)
            .vesperPressable(pressedScale = 0.94f, hapticOnTap = true, onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Box(
            modifier = Modifier
                .clip(shape)
                .background(
                    if (active) activeColor.copy(alpha = if (theme.isLight) 0.14f else 0.20f) else theme.surfaceControl,
                    shape
                )
                .border(
                    BorderStroke(1.dp, if (active) activeColor else theme.border),
                    shape
                )
                .padding(horizontal = 9.dp, vertical = 6.dp),
            contentAlignment = Alignment.Center
        ) {
            Text(
                text = label,
                style = TextStyle(
                    fontSize = 11.sp,
                    fontWeight = if (active) FontWeight.SemiBold else FontWeight.Medium,
                    color = if (active) activeColor else theme.textSecondary,
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 1,
                softWrap = false
            )
        }
    }
}

@Composable
private fun PoolActionPill(
    text: String,
    active: Boolean = false,
    toneColor: Color = Color.Unspecified,
    enabled: Boolean = true,
    onClick: () -> Unit
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(9.dp)
    val resolvedTone = if (toneColor != Color.Unspecified) toneColor else theme.accent
    val bgColor = if (active) {
        resolvedTone.copy(alpha = if (theme.isLight) 0.11f else 0.16f)
    } else {
        theme.surfaceControl
    }
    val borderColor = if (active) {
        resolvedTone.copy(alpha = if (theme.isLight) 0.34f else 0.44f)
    } else {
        theme.border
    }
    val fgColor = when {
        !enabled -> theme.textMuted
        active -> resolvedTone
        else -> theme.textSecondary
    }

    Box(
        modifier = Modifier
            .sizeIn(minWidth = 48.dp, minHeight = 48.dp)
            .vesperPressable(enabled = enabled, pressedScale = 0.95f, hapticOnTap = true, onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Box(
            modifier = Modifier
                .clip(shape)
                .background(bgColor, shape)
                .border(1.dp, borderColor, shape)
                .padding(horizontal = 11.dp, vertical = 7.dp),
            contentAlignment = Alignment.Center
        ) {
            Text(
                text = text,
                style = TextStyle(
                    fontSize = 11.5.sp,
                    fontWeight = FontWeight.Medium,
                    color = fgColor,
                    platformStyle = NoFontPaddingStyle
                ),
                maxLines = 1,
                softWrap = false
            )
        }
    }
}

@Composable
private fun PoolIconActionPill(
    icon: ImageVector,
    contentDescription: String,
    onClick: () -> Unit
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(9.dp)
    Box(
        modifier = Modifier
            .sizeIn(minWidth = 48.dp, minHeight = 48.dp)
            .vesperPressable(pressedScale = 0.92f, hapticOnTap = true, onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Box(
            modifier = Modifier
                .size(34.dp)
                .clip(shape)
                .background(theme.surfaceControl, shape)
                .border(1.dp, theme.border, shape),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = icon,
                contentDescription = contentDescription,
                tint = theme.textSecondary,
                modifier = Modifier.size(18.dp)
            )
        }
    }
}
