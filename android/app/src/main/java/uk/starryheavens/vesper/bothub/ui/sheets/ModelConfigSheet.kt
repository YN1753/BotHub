package uk.starryheavens.vesper.bothub.ui.sheets

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
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
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
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
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileToggle
import uk.starryheavens.vesper.bothub.ui.components.tactileClickable
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

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
        tokens >= 1_000_000 -> String.format(java.util.Locale.US, "%.2fM", tokens / 1_000_000.0)
        tokens >= 1_000 -> String.format(java.util.Locale.US, "%.1fK", tokens / 1_000.0)
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
                .padding(horizontal = 16.dp)
                .padding(bottom = 14.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            // Header
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "AI 模型渠道与双池",
                        fontSize = 18.sp,
                        fontWeight = FontWeight.Bold,
                        color = theme.textPrimary
                    )
                    Text(
                        text = "渠道共享密钥 · 免费池与付费池独立调度",
                        fontSize = 11.5.sp,
                        color = theme.textMuted
                    )
                }
                TactilePrimaryButton(
                    text = if (saving) "保存中" else "保存",
                    enabled = !saving,
                    minHeight = 36.dp,
                    contentPadding = PaddingValues(horizontal = 14.dp, vertical = 6.dp),
                    onClick = { saveAll() }
                )
            }

            // 7-Day Telemetry Summary Strip
            SpecularGlassCard(
                cornerRadius = 16.dp,
                onClick = {
                    if (usageSummary.topModels.isNotEmpty() || usageSummary.topCallers.isNotEmpty()) {
                        showUsageDetail = !showUsageDetail
                    }
                },
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 12.dp, vertical = 9.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Column(modifier = Modifier.weight(1f)) {
                            Text("7日免费池", fontSize = 10.5.sp, color = Color(0xFF34D399))
                            Text(
                                text = "${usageSummary.freeCalls}次 · ${formatTokenShort(usageSummary.freeTokens)}",
                                fontSize = 12.sp,
                                fontWeight = FontWeight.SemiBold,
                                fontFamily = FontFamily.Monospace,
                                color = theme.textPrimary
                            )
                        }
                        Column(modifier = Modifier.weight(1f)) {
                            Text("7日付费池", fontSize = 10.5.sp, color = theme.accent)
                            Text(
                                text = "${usageSummary.paidCalls}次 · ${formatTokenShort(usageSummary.paidTokens)}",
                                fontSize = 12.sp,
                                fontWeight = FontWeight.SemiBold,
                                fontFamily = FontFamily.Monospace,
                                color = theme.textPrimary
                            )
                        }
                        Column(horizontalAlignment = Alignment.End) {
                            Text("成功率", fontSize = 10.5.sp, color = theme.textMuted)
                            val rate = if (usageSummary.totalCalls > 0) {
                                "${(usageSummary.successCalls * 100) / usageSummary.totalCalls}%"
                            } else "--"
                            Text(
                                text = rate,
                                fontSize = 12.sp,
                                fontWeight = FontWeight.SemiBold,
                                fontFamily = FontFamily.Monospace,
                                color = theme.textPrimary
                            )
                        }
                    }

                    if (usageSummary.topModels.isNotEmpty() || usageSummary.topCallers.isNotEmpty()) {
                        Text(
                            text = if (showUsageDetail) "收起按模型/按Bot消耗明细" else "点击展开 7日按模型 / 按Bot与任务明细",
                            fontSize = 10.5.sp,
                            color = theme.accent
                        )
                    }

                    if (showUsageDetail) {
                        if (usageSummary.topModels.isNotEmpty()) {
                            HorizontalDivider(thickness = 0.5.dp, color = theme.divider)
                            Text("按模型调用排行", fontSize = 11.sp, fontWeight = FontWeight.SemiBold, color = theme.textSecondary)
                            usageSummary.topModels.take(6).forEach { m ->
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween
                                ) {
                                    Text(
                                        text = m.modelId,
                                        fontSize = 10.5.sp,
                                        fontFamily = FontFamily.Monospace,
                                        color = theme.textPrimary,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        modifier = Modifier.weight(1f).padding(end = 8.dp)
                                    )
                                    Text(
                                        text = "${m.calls}次 · ${formatTokenShort(m.tokens)}",
                                        fontSize = 10.5.sp,
                                        fontFamily = FontFamily.Monospace,
                                        color = if (m.tier == "paid") theme.accent else Color(0xFF34D399)
                                    )
                                }
                            }
                        }
                        if (usageSummary.topCallers.isNotEmpty()) {
                            HorizontalDivider(thickness = 0.5.dp, color = theme.divider)
                            Text("按 Bot / 任务消耗排行", fontSize = 11.sp, fontWeight = FontWeight.SemiBold, color = theme.textSecondary)
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
                                        fontSize = 10.5.sp,
                                        color = theme.textPrimary,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        modifier = Modifier.weight(1f).padding(end = 8.dp)
                                    )
                                    Text(
                                        text = "${c.freeCalls}免/${c.paidCalls}付 · ${formatTokenShort(c.tokens)}",
                                        fontSize = 10.5.sp,
                                        fontFamily = FontFamily.Monospace,
                                        color = if (c.paidCalls > 0) theme.accent else Color(0xFF34D399)
                                    )
                                }
                            }
                        }
                    }
                }
            }

            // 3-Tab Segmented Row
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(14.dp))
                    .background(theme.inputTop, RoundedCornerShape(14.dp))
                    .border(0.8.dp, theme.divider, RoundedCornerShape(14.dp))
                    .padding(4.dp),
                horizontalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                val tabs = listOf(
                    "providers" to "渠道 (${draftProviders.size})",
                    "free" to "免费池 (${freeList.size})",
                    "paid" to "付费池 (${paidList.size})"
                )
                tabs.forEach { (id, label) ->
                    val selected = activeTab == id
                    Box(
                        modifier = Modifier
                            .weight(1f)
                            .clip(RoundedCornerShape(10.dp))
                            .background(
                                if (selected) theme.accent.copy(alpha = 0.2f) else Color.Transparent,
                                RoundedCornerShape(10.dp)
                            )
                            .border(
                                if (selected) 1.dp else 0.dp,
                                if (selected) theme.accent.copy(alpha = 0.6f) else Color.Transparent,
                                RoundedCornerShape(10.dp)
                            )
                            .tactileClickable { activeTab = id }
                            .padding(vertical = 8.dp),
                        contentAlignment = Alignment.Center
                    ) {
                        Text(
                            text = label,
                            fontSize = 12.sp,
                            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                            color = if (selected) theme.accent else theme.textSecondary,
                            maxLines = 1,
                            softWrap = false
                        )
                    }
                }
            }

            // Scrollable Body
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .heightIn(max = 520.dp)
                    .verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                if (activeTab == "providers") {
                    // Quick-Add Public Preset Chips
                    Text(
                        text = "快速添加渠道（仅填入公开标准地址）",
                        fontSize = 11.5.sp,
                        fontWeight = FontWeight.Medium,
                        color = theme.textMuted
                    )
                    LazyRow(
                        horizontalArrangement = Arrangement.spacedBy(7.dp),
                        contentPadding = PaddingValues(vertical = 2.dp)
                    ) {
                        items(ANDROID_PUBLIC_PROVIDER_PRESETS, key = { it.key }) { preset ->
                            val chipShape = RoundedCornerShape(11.dp)
                            Box(
                                modifier = Modifier
                                    .clip(chipShape)
                                    .background(theme.inputTop, chipShape)
                                    .border(0.8.dp, theme.divider, chipShape)
                                    .tactileClickable {
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
                                    }
                                    .padding(horizontal = 11.dp, vertical = 7.dp)
                            ) {
                                Text(
                                    text = "+ ${preset.label}",
                                    fontSize = 12.sp,
                                    color = theme.textPrimary,
                                    maxLines = 1,
                                    softWrap = false
                                )
                            }
                        }
                    }

                    if (draftProviders.isEmpty()) {
                        SpecularGlassCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                            Column(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(16.dp),
                                horizontalAlignment = Alignment.CenterHorizontally,
                                verticalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                Text("尚未添加模型渠道", fontSize = 13.5.sp, fontWeight = FontWeight.SemiBold, color = theme.textPrimary)
                                Text(
                                    "点击上方模板填入 API Key 后，可零消耗拉取全量模型并一键分配至免费池或付费池。",
                                    fontSize = 11.5.sp,
                                    color = theme.textMuted
                                )
                            }
                        }
                    }

                    draftProviders.forEach { prov ->
                        val isExpanded = expandedProviderId == prov.id
                        val freeCount = draftModels.count { it.providerId == prov.id && it.tier != "paid" }
                        val paidCount = draftModels.count { it.providerId == prov.id && it.tier == "paid" }
                        SpecularGlassCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                            Column(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(12.dp),
                                verticalArrangement = Arrangement.spacedBy(8.dp)
                            ) {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    Box(
                                        modifier = Modifier
                                            .size(8.dp)
                                            .background(
                                                if (prov.enabled && prov.apiKey.isNotBlank()) Color(0xFF34D399) else theme.textMuted,
                                                CircleShape
                                            )
                                    )
                                    Column(
                                        modifier = Modifier
                                            .weight(1f)
                                            .tactileClickable {
                                                expandedProviderId = if (isExpanded) "" else prov.id
                                            }
                                    ) {
                                        Text(
                                            text = prov.name.ifBlank { prov.id },
                                            fontSize = 13.5.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            color = theme.textPrimary,
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis
                                        )
                                        Text(
                                            text = "${if (prov.protocol == "gemini") "Gemini" else "OpenAI 兼容"} · 免费 $freeCount / 付费 $paidCount",
                                            fontSize = 11.sp,
                                            color = theme.textMuted
                                        )
                                    }
                                    TactileToggle(
                                        checked = prov.enabled,
                                        onCheckedChange = { checked ->
                                            draftProviders = draftProviders.map {
                                                if (it.id == prov.id) it.copy(enabled = checked) else it
                                            }
                                        }
                                    )
                                }

                                if (isExpanded) {
                                    HorizontalDivider(thickness = 0.5.dp, color = theme.divider)

                                    ObsidianGlassInput(
                                        value = prov.name,
                                        onValueChange = { v ->
                                            draftProviders = draftProviders.map {
                                                if (it.id == prov.id) it.copy(name = v) else it
                                            }
                                        },
                                        label = "渠道名称",
                                        placeholder = "例如 OpenRouter / 硅基流动"
                                    )

                                    Row(
                                        modifier = Modifier.fillMaxWidth(),
                                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                                    ) {
                                        listOf("openai" to "OpenAI 兼容", "gemini" to "Google Gemini").forEach { (proto, label) ->
                                            val active = prov.protocol == proto
                                            val pillShape = RoundedCornerShape(10.dp)
                                            Box(
                                                modifier = Modifier
                                                    .weight(1f)
                                                    .clip(pillShape)
                                                    .background(
                                                        if (active) theme.accent.copy(alpha = 0.16f) else theme.inputTop,
                                                        pillShape
                                                    )
                                                    .border(
                                                        1.dp,
                                                        if (active) theme.accent else theme.divider,
                                                        pillShape
                                                    )
                                                    .tactileClickable {
                                                        draftProviders = draftProviders.map {
                                                            if (it.id == prov.id) it.copy(protocol = proto) else it
                                                        }
                                                    }
                                                    .padding(vertical = 7.dp),
                                                contentAlignment = Alignment.Center
                                            ) {
                                                Text(
                                                    text = label,
                                                    fontSize = 11.5.sp,
                                                    color = if (active) theme.accent else theme.textSecondary,
                                                    fontWeight = if (active) FontWeight.SemiBold else FontWeight.Normal
                                                )
                                            }
                                        }
                                    }

                                    ObsidianGlassInput(
                                        value = prov.baseUrl,
                                        onValueChange = { v ->
                                            draftProviders = draftProviders.map {
                                                if (it.id == prov.id) it.copy(baseUrl = v) else it
                                            }
                                        },
                                        label = "Base URL",
                                        placeholder = "https://api.example.com/v1"
                                    )

                                    ObsidianGlassInput(
                                        value = prov.apiKey,
                                        onValueChange = { v ->
                                            draftProviders = draftProviders.map {
                                                if (it.id == prov.id) it.copy(apiKey = v) else it
                                            }
                                        },
                                        label = "API Key（留原掩码不变）",
                                        placeholder = "sk-... / AIza...",
                                        visualTransformation = PasswordVisualTransformation()
                                    )

                                    Row(
                                        modifier = Modifier.fillMaxWidth(),
                                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                                    ) {
                                        TactileSecondaryButton(
                                            text = if (fetchingProviderId == prov.id) "拉取中..." else "拉取全量模型 (0消耗)",
                                            accent = true,
                                            enabled = fetchingProviderId.isEmpty(),
                                            minHeight = 38.dp,
                                            onClick = {
                                                val cleanUrl = cleanBaseUrlAndroid(prov.baseUrl)
                                                if (cleanUrl.isBlank()) {
                                                    onShowMessage("请先填写 Base URL")
                                                    return@TactileSecondaryButton
                                                }
                                                fetchingProviderId = prov.id
                                                scope.launch {
                                                    api.fetchUpstreamModels(prov.id, prov.protocol, cleanUrl, prov.apiKey)
                                                        .onSuccess { list ->
                                                            fetchedModelsMap[prov.id] = list
                                                            onShowMessage("零消耗拉取成功：共 ${list.size} 个模型")
                                                        }
                                                        .onFailure { err ->
                                                            onShowMessage(err.message ?: "拉取失败")
                                                        }
                                                    fetchingProviderId = ""
                                                }
                                            },
                                            modifier = Modifier.weight(1f)
                                        )
                                        TactileSecondaryButton(
                                            text = "删除渠道",
                                            danger = true,
                                            minHeight = 38.dp,
                                            onClick = {
                                                draftProviders = draftProviders.filterNot { it.id == prov.id }
                                                draftModels = draftModels.filterNot { it.providerId == prov.id }
                                            }
                                        )
                                    }

                                    // Manual model ID quick-add
                                    val manualVal = manualInputMap[prov.id].orEmpty()
                                    ObsidianGlassInput(
                                        value = manualVal,
                                        onValueChange = { manualInputMap[prov.id] = it },
                                        label = "手动输入模型 ID",
                                        placeholder = "如 deepseek-chat（逗号分隔）"
                                    )
                                    Row(
                                        modifier = Modifier.fillMaxWidth(),
                                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                                    ) {
                                        TactileSecondaryButton(
                                            text = "+ 加免费池",
                                            minHeight = 34.dp,
                                            onClick = {
                                                val parts = manualVal.split(",", "，", "\n").map { it.trim() }.filter { it.isNotEmpty() }
                                                parts.forEach { mid -> assignModelToTier(prov.id, mid, "free") }
                                                manualInputMap[prov.id] = ""
                                            },
                                            modifier = Modifier.weight(1f)
                                        )
                                        TactileSecondaryButton(
                                            text = "+ 加付费池",
                                            minHeight = 34.dp,
                                            onClick = {
                                                val parts = manualVal.split(",", "，", "\n").map { it.trim() }.filter { it.isNotEmpty() }
                                                parts.forEach { mid -> assignModelToTier(prov.id, mid, "paid") }
                                                manualInputMap[prov.id] = ""
                                            },
                                            modifier = Modifier.weight(1f)
                                        )
                                    }

                                    // Searchable Fetched Full Model List
                                    val fetched = fetchedModelsMap[prov.id]
                                    if (!fetched.isNullOrEmpty()) {
                                        val query = searchQueryMap[prov.id].orEmpty()
                                        val onlyFreeTag = freeOnlyFilterMap[prov.id] == true
                                        ObsidianGlassInput(
                                            value = query,
                                            onValueChange = { searchQueryMap[prov.id] = it },
                                            placeholder = "搜索模型名（共 ${fetched.size} 个）..."
                                        )
                                        val likelyFreeList = fetched.filter { isLikelyFreeModelAndroid(it) }
                                        if (likelyFreeList.isNotEmpty()) {
                                            Row(
                                                modifier = Modifier.fillMaxWidth(),
                                                horizontalArrangement = Arrangement.spacedBy(8.dp)
                                            ) {
                                                val tagShape = RoundedCornerShape(9.dp)
                                                Box(
                                                    modifier = Modifier
                                                        .clip(tagShape)
                                                        .background(
                                                            if (onlyFreeTag) Color(0xFF34D399).copy(alpha = 0.18f) else theme.inputTop,
                                                            tagShape
                                                        )
                                                        .border(
                                                            0.8.dp,
                                                            if (onlyFreeTag) Color(0xFF34D399) else theme.divider,
                                                            tagShape
                                                        )
                                                        .tactileClickable { freeOnlyFilterMap[prov.id] = !onlyFreeTag }
                                                        .padding(horizontal = 10.dp, vertical = 5.dp)
                                                ) {
                                                    Text(
                                                        text = if (onlyFreeTag) "已筛选免费模型 (${likelyFreeList.size})" else "仅看免费模型 (${likelyFreeList.size})",
                                                        fontSize = 11.sp,
                                                        color = if (onlyFreeTag) Color(0xFF34D399) else theme.textSecondary
                                                    )
                                                }
                                                Box(
                                                    modifier = Modifier
                                                        .clip(tagShape)
                                                        .background(Color(0xFF34D399).copy(alpha = 0.14f), tagShape)
                                                        .border(0.8.dp, Color(0xFF34D399).copy(alpha = 0.45f), tagShape)
                                                        .tactileClickable {
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
                                                        .padding(horizontal = 10.dp, vertical = 5.dp)
                                                ) {
                                                    Text(
                                                        text = "+ 一键免费加池",
                                                        fontSize = 11.sp,
                                                        color = Color(0xFF34D399)
                                                    )
                                                }
                                            }
                                        }

                                        val filtered = fetched.filter { id ->
                                            val qMatch = query.isBlank() || id.contains(query.trim(), ignoreCase = true)
                                            val fMatch = !onlyFreeTag || isLikelyFreeModelAndroid(id)
                                            qMatch && fMatch
                                        }.take(40)

                                        filtered.forEach { mid ->
                                            val curTier = draftModels.find { it.providerId == prov.id && it.modelId == mid }?.tier.orEmpty()
                                            Row(
                                                modifier = Modifier
                                                    .fillMaxWidth()
                                                    .clip(RoundedCornerShape(10.dp))
                                                    .background(theme.inputTop, RoundedCornerShape(10.dp))
                                                    .padding(horizontal = 9.dp, vertical = 6.dp),
                                                verticalAlignment = Alignment.CenterVertically,
                                                horizontalArrangement = Arrangement.spacedBy(6.dp)
                                            ) {
                                                Text(
                                                    text = mid,
                                                    fontSize = 11.sp,
                                                    fontFamily = FontFamily.Monospace,
                                                    color = theme.textPrimary,
                                                    maxLines = 1,
                                                    overflow = TextOverflow.Ellipsis,
                                                    modifier = Modifier.weight(1f)
                                                )
                                                TierAssignPill(
                                                    label = if (curTier == "free") "已选免费" else "免费",
                                                    active = curTier == "free",
                                                    activeColor = Color(0xFF34D399),
                                                    onClick = { assignModelToTier(prov.id, mid, "free") }
                                                )
                                                TierAssignPill(
                                                    label = if (curTier == "paid") "已选付费" else "付费",
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
                } else {
                    val tier = if (activeTab == "paid") "paid" else "free"
                    val list = if (tier == "paid") paidList else freeList
                    val curStrategy = if (tier == "paid") draftStrategies.paid else draftStrategies.free

                    // Pool Strategy Selector Card
                    SpecularGlassCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                        Column(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(12.dp),
                            verticalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Text(
                                    text = if (tier == "paid") "付费池调度策略" else "免费池调度策略",
                                    fontSize = 13.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = theme.textPrimary
                                )
                                val anyCooling = list.any { (modelHealth[it.canonicalTargetId]?.cooldownSec ?: 0L) > 0L }
                                val anyFailedProbe = list.any { it.enabled && probeStatusMap[it.canonicalTargetId]?.first == false }
                                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                    if (anyFailedProbe) {
                                        Text(
                                            text = "停用异常",
                                            fontSize = 11.5.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            color = Color(0xFFF87171),
                                            modifier = Modifier.tactileClickable {
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
                                    if (anyCooling) {
                                        Text(
                                            text = "全部解封",
                                            fontSize = 11.5.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            color = theme.accent,
                                            modifier = Modifier.tactileClickable {
                                                scope.launch {
                                                    api.clearModelCooldown("").onSuccess { nextHealth ->
                                                        onHealthUpdated(nextHealth, null, null)
                                                        onShowMessage("已解除全部冷却状态")
                                                    }
                                                }
                                            }
                                        )
                                    }
                                    if (list.isNotEmpty()) {
                                        Text(
                                            text = if (probingBatchTier == tier) "测速中..." else "并发测速",
                                            fontSize = 11.5.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            color = Color(0xFF34D399),
                                            modifier = Modifier.tactileClickable {
                                                if (probingBatchTier.isNotEmpty()) return@tactileClickable
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
                                }
                            }
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                listOf(
                                    "priority" to "按优先级",
                                    "round_robin" to "轮询均摊",
                                    "lowest_latency" to "最快延迟"
                                ).forEach { (sid, label) ->
                                    val chosen = curStrategy == sid
                                    val pillShape = RoundedCornerShape(10.dp)
                                    Box(
                                        modifier = Modifier
                                            .weight(1f)
                                            .clip(pillShape)
                                            .background(
                                                if (chosen) theme.accent.copy(alpha = 0.18f) else theme.inputTop,
                                                pillShape
                                            )
                                            .border(
                                                1.dp,
                                                if (chosen) theme.accent else theme.divider,
                                                pillShape
                                            )
                                            .tactileClickable {
                                                draftStrategies = if (tier == "paid") {
                                                    draftStrategies.copy(paid = sid)
                                                } else {
                                                    draftStrategies.copy(free = sid)
                                                }
                                            }
                                            .padding(vertical = 7.dp),
                                        contentAlignment = Alignment.Center
                                    ) {
                                        Text(
                                            text = label,
                                            fontSize = 11.5.sp,
                                            fontWeight = if (chosen) FontWeight.SemiBold else FontWeight.Normal,
                                            color = if (chosen) theme.accent else theme.textSecondary
                                        )
                                    }
                                }
                            }
                        }
                    }

                    if (list.isEmpty()) {
                        SpecularGlassCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                            Column(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(16.dp),
                                horizontalAlignment = Alignment.CenterHorizontally,
                                verticalArrangement = Arrangement.spacedBy(8.dp)
                            ) {
                                Text(
                                    text = if (tier == "paid") "付费池暂无模型" else "免费池暂无模型",
                                    fontSize = 13.5.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = theme.textPrimary
                                )
                                TactileSecondaryButton(
                                    text = "去渠道管理添加",
                                    accent = true,
                                    minHeight = 36.dp,
                                    onClick = { activeTab = "providers" }
                                )
                            }
                        }
                    }

                    list.forEachIndexed { idx, m ->
                        val prov = providerById[m.providerId]
                        val health = modelHealth[m.canonicalTargetId]
                        val probeNote = probeStatusMap[m.canonicalTargetId]
                        SpecularGlassCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                            Column(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(12.dp),
                                verticalArrangement = Arrangement.spacedBy(7.dp)
                            ) {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    Column(modifier = Modifier.weight(1f)) {
                                        Text(
                                            text = "#${idx + 1} · ${prov?.name ?: m.providerId}",
                                            fontSize = 11.sp,
                                            color = theme.textMuted
                                        )
                                        Text(
                                            text = m.modelId,
                                            fontSize = 13.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            fontFamily = FontFamily.Monospace,
                                            color = theme.textPrimary,
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis
                                        )
                                    }
                                    TactileToggle(
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

                                // Capability & Health Status Line
                                val statusParts = buildList {
                                    if (m.supportsTools == true) add("Tools可用")
                                    else if (m.supportsTools == false) add("无Tools")
                                    if (m.supportsThinking == true) add("Thinking")
                                    when {
                                        (health?.cooldownSec ?: 0L) > 0L -> add("冷却 ${health?.cooldownSec}s")
                                        (health?.lastLatencyMs ?: 0L) > 0L -> add("正常 ${health?.lastLatencyMs}ms")
                                        m.lastLatencyMs > 0L -> add("上次 ${m.lastLatencyMs}ms")
                                    }
                                }
                                if (statusParts.isNotEmpty()) {
                                    Text(
                                        text = statusParts.joinToString(" · "),
                                        fontSize = 11.sp,
                                        color = if ((health?.cooldownSec ?: 0L) > 0L) theme.accent else Color(0xFF34D399)
                                    )
                                }
                                if (probeNote != null) {
                                    Text(
                                        text = probeNote.second,
                                        fontSize = 11.sp,
                                        color = if (probeNote.first) Color(0xFF34D399) else Color(0xFFF87171)
                                    )
                                }

                                // Row Actions
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                                ) {
                                    SmallPoolActionBtn(
                                        text = if (probingTargetId == m.canonicalTargetId) "测速中" else "测速",
                                        onClick = {
                                            if (prov == null || probingTargetId.isNotEmpty()) return@SmallPoolActionBtn
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
                                    if ((health?.cooldownSec ?: 0L) > 0L) {
                                        SmallPoolActionBtn(
                                            text = "解封",
                                            accent = true,
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
                                    SmallPoolActionBtn(
                                        text = if (tier == "paid") "移至免费" else "移至付费",
                                        onClick = {
                                            val otherTier = if (tier == "paid") "free" else "paid"
                                            assignModelToTier(m.providerId, m.modelId, otherTier)
                                        }
                                    )
                                    if (idx > 0) {
                                        SmallPoolActionBtn(text = "↑", onClick = { moveModelPriority(tier, idx, -1) })
                                    }
                                    if (idx < list.lastIndex) {
                                        SmallPoolActionBtn(text = "↓", onClick = { moveModelPriority(tier, idx, 1) })
                                    }
                                    Spacer(modifier = Modifier.weight(1f))
                                    SmallPoolActionBtn(
                                        text = "移除",
                                        danger = true,
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
            .clip(shape)
            .background(
                if (active) activeColor.copy(alpha = 0.2f) else Color.Transparent,
                shape
            )
            .border(
                BorderStroke(0.8.dp, if (active) activeColor else theme.divider),
                shape
            )
            .tactileClickable { onClick() }
            .padding(horizontal = 8.dp, vertical = 4.dp)
    ) {
        Text(
            text = label,
            fontSize = 10.5.sp,
            fontWeight = if (active) FontWeight.SemiBold else FontWeight.Normal,
            color = if (active) activeColor else theme.textSecondary,
            maxLines = 1,
            softWrap = false
        )
    }
}

@Composable
private fun SmallPoolActionBtn(
    text: String,
    accent: Boolean = false,
    danger: Boolean = false,
    onClick: () -> Unit
) {
    val theme = BotHubThemeTokens
    val shape = RoundedCornerShape(8.dp)
    val fg = when {
        danger -> Color(0xFFF87171)
        accent -> theme.accent
        else -> theme.textSecondary
    }
    Box(
        modifier = Modifier
            .clip(shape)
            .background(theme.inputTop, shape)
            .border(0.8.dp, if (danger) Color(0xFFF87171).copy(alpha = 0.4f) else theme.divider, shape)
            .tactileClickable { onClick() }
            .padding(horizontal = 9.dp, vertical = 5.dp)
    ) {
        Text(
            text = text,
            fontSize = 11.sp,
            fontWeight = FontWeight.Medium,
            color = fg,
            maxLines = 1,
            softWrap = false
        )
    }
}
