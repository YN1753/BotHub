package uk.starryheavens.vesper.bothub.ui.sheets

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Extension
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.SkillItemUi
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperDivider
import uk.starryheavens.vesper.bothub.ui.components.VesperEmptyState
import uk.starryheavens.vesper.bothub.ui.components.VesperIconButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSectionHeader
import uk.starryheavens.vesper.bothub.ui.components.VesperSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.VesperSwitch
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.vesperPressable
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun McpAppsSheet(
    skills: List<SkillItemUi>,
    onRefresh: () -> Unit,
    onToggleEnabled: (SkillItemUi, Boolean) -> Unit,
    onTestSkill: (SkillItemUi, (Boolean, String) -> Unit) -> Unit,
    onDismiss: () -> Unit
) {
    val theme = BotHubThemeTokens
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var testingSkillId by remember { mutableStateOf("") }
    val testResultMap = remember { mutableStateMapOf<String, Pair<Boolean, String>>() }

    val appConnectors = remember(skills) { skills.filter { it.isAppConnector } }
    val builtinMcps = remember(skills) { skills.filter { it.kind == "builtin_mcp" && !it.isAppConnector } }
    val customExtensions = remember(skills) { skills.filter { !it.isAppConnector && it.kind != "builtin_mcp" } }
    val enabledCount = remember(skills) { skills.count { it.enabled } }

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
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "MCP 服务与已连接应用",
                        fontSize = 18.sp,
                        fontWeight = FontWeight.Bold,
                        color = theme.textPrimary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = "已启用 $enabledCount / ${skills.size} 项能力 · 凭据与远程 MCP 详情可在 Web 端精细配置",
                        fontSize = 11.5.sp,
                        color = theme.textMuted,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
                VesperIconButton(
                    icon = Icons.Default.Refresh,
                    contentDescription = "刷新列表",
                    onClick = onRefresh
                )
                VesperIconButton(
                    icon = Icons.Default.Close,
                    contentDescription = "关闭",
                    onClick = onDismiss
                )
            }

            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .heightIn(max = 520.dp)
                    .verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                if (skills.isEmpty()) {
                    VesperEmptyState(
                        icon = Icons.Default.Extension,
                        title = "暂无 MCP 服务或已连接应用",
                        subtitle = "请点击右上角刷新同步服务端已注册的内置连接器与远程 MCP 服务。"
                    )
                }

                if (appConnectors.isNotEmpty()) {
                    McpSkillSection(
                        title = "已连接应用（GitHub / 邮箱 / Cloudflare 等）",
                        items = appConnectors,
                        testingSkillId = testingSkillId,
                        testResultMap = testResultMap,
                        onToggleEnabled = onToggleEnabled,
                        onRunTest = { sk ->
                            if (testingSkillId.isNotEmpty()) return@McpSkillSection
                            testingSkillId = sk.id
                            onTestSkill(sk) { ok, msg ->
                                testResultMap[sk.id] = ok to msg
                                testingSkillId = ""
                            }
                        }
                    )
                }

                if (builtinMcps.isNotEmpty()) {
                    McpSkillSection(
                        title = "内置工作区 MCP 服务",
                        items = builtinMcps,
                        testingSkillId = testingSkillId,
                        testResultMap = testResultMap,
                        onToggleEnabled = onToggleEnabled,
                        onRunTest = { sk ->
                            if (testingSkillId.isNotEmpty()) return@McpSkillSection
                            testingSkillId = sk.id
                            onTestSkill(sk) { ok, msg ->
                                testResultMap[sk.id] = ok to msg
                                testingSkillId = ""
                            }
                        }
                    )
                }

                if (customExtensions.isNotEmpty()) {
                    McpSkillSection(
                        title = "远程 MCP 与自定义技能",
                        items = customExtensions,
                        testingSkillId = testingSkillId,
                        testResultMap = testResultMap,
                        onToggleEnabled = onToggleEnabled,
                        onRunTest = { sk ->
                            if (testingSkillId.isNotEmpty()) return@McpSkillSection
                            testingSkillId = sk.id
                            onTestSkill(sk) { ok, msg ->
                                testResultMap[sk.id] = ok to msg
                                testingSkillId = ""
                            }
                        }
                    )
                }

                Spacer(modifier = Modifier.height(6.dp))
            }
        }
    }
}

@Composable
private fun McpSkillSection(
    title: String,
    items: List<SkillItemUi>,
    testingSkillId: String,
    testResultMap: Map<String, Pair<Boolean, String>>,
    onToggleEnabled: (SkillItemUi, Boolean) -> Unit,
    onRunTest: (SkillItemUi) -> Unit
) {
    val theme = BotHubThemeTokens
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        VesperSectionHeader(title = title, count = items.size)
        items.forEach { sk ->
            val testOutcome = testResultMap[sk.id]
            val isTesting = testingSkillId == sk.id
            val kindLabel = when {
                sk.isAppConnector -> "应用连接器"
                sk.kind == "builtin_mcp" -> "内置 MCP"
                sk.kind == "mcp" -> "远程 MCP"
                sk.kind == "webhook" -> "Webhook"
                else -> "指令技能"
            }

            VesperCard(cornerRadius = 16.dp, modifier = Modifier.fillMaxWidth()) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 14.dp, vertical = 11.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Box(
                            modifier = Modifier
                                .size(8.dp)
                                .background(if (sk.enabled) theme.success else theme.textMuted, CircleShape)
                        )
                        Column(
                            modifier = Modifier.weight(1f),
                            verticalArrangement = Arrangement.spacedBy(3.dp)
                        ) {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                Text(
                                    text = sk.name,
                                    fontSize = 14.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = theme.textPrimary,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                    modifier = Modifier.weight(1f, fill = false)
                                )
                                VesperTag(
                                    text = if (sk.toolCount > 0) "$kindLabel · ${sk.toolCount}工具" else kindLabel,
                                    bgColor = theme.surfaceInput,
                                    borderColor = theme.border,
                                    textColor = theme.textSecondary
                                )
                            }
                            if (sk.description.isNotBlank()) {
                                Text(
                                    text = sk.description,
                                    fontSize = 12.sp,
                                    lineHeight = 17.sp,
                                    color = theme.textMuted,
                                    maxLines = 2,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                            if (sk.httpUrl.isNotBlank()) {
                                Text(
                                    text = sk.httpUrl,
                                    fontSize = 11.sp,
                                    fontFamily = FontFamily.Monospace,
                                    color = theme.textMuted,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                        }
                        VesperSwitch(
                            checked = sk.enabled,
                            onCheckedChange = { checked -> onToggleEnabled(sk, checked) }
                        )
                    }

                    if (testOutcome != null) {
                        Text(
                            text = testOutcome.second,
                            fontSize = 11.5.sp,
                            lineHeight = 16.sp,
                            color = if (testOutcome.first) theme.success else theme.danger
                        )
                    }

                    VesperDivider()

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = if (sk.enabled) "已向 Bot 开放调用" else "当前已停用",
                            fontSize = 11.5.sp,
                            color = if (sk.enabled) theme.success else theme.textMuted
                        )
                        val btnShape = RoundedCornerShape(9.dp)
                        Box(
                            modifier = Modifier
                                .sizeIn(minWidth = 48.dp, minHeight = 48.dp)
                                .vesperPressable(
                                    enabled = testingSkillId.isEmpty(),
                                    pressedScale = 0.95f,
                                    hapticOnTap = true,
                                    onClick = { onRunTest(sk) }
                                ),
                            contentAlignment = Alignment.Center
                        ) {
                            Box(
                                modifier = Modifier
                                    .clip(btnShape)
                                    .background(theme.accentSoft, btnShape)
                                    .border(1.dp, theme.accentMuted, btnShape)
                                    .padding(horizontal = 12.dp, vertical = 7.dp),
                                contentAlignment = Alignment.Center
                            ) {
                                Text(
                                    text = if (isTesting) "测试中..." else "连通测试",
                                    style = TextStyle(
                                        fontSize = 11.5.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        color = theme.accentForeground,
                                        platformStyle = NoFontPaddingStyle
                                    ),
                                    maxLines = 1,
                                    softWrap = false
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}
