package uk.starryheavens.vesper.bothub.ui.sheets

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.data.ShareTextParser
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.SpiritSquircleAvatar
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.ZeroClickCaptureRow
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

@Composable
fun QuickCaptureSheet(
    rawShared: String,
    prefs: BotHubPrefs,
    api: BotHubApiClient,
    onSaved: (String) -> Unit,
    onForwardDeepChat: (String) -> Unit,
    onOpenSetup: () -> Unit,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val scope = rememberCoroutineScope()
    val preview = remember(rawShared) { ShareTextParser.parse(rawShared) }
    var noteInput by remember(rawShared) { mutableStateOf("") }
    var zeroClick by remember { mutableStateOf(prefs.zeroClickCaptureEnabled) }
    var saving by remember(rawShared) { mutableStateOf(false) }
    var savingTodo by remember(rawShared) { mutableStateOf(false) }
    var savedMsg by remember(rawShared) { mutableStateOf("") }
    var errorMsg by remember(rawShared) { mutableStateOf("") }

    var hasToken by remember { mutableStateOf(prefs.isLikelyPaired) }

    fun submitCapture() {
        if (saving || savingTodo || !hasToken) return
        saving = true
        errorMsg = ""
        scope.launch {
            api.createStashItem(rawInput = rawShared, userNote = noteInput).fold(
                onSuccess = { (_, deduplicated) ->
                    val toastText = if (deduplicated) {
                        "该内容已在 BotHub 收藏匣，已合并批注"
                    } else {
                        "已收入 BotHub 收藏匣，后台正在提炼要点"
                    }
                    savedMsg = toastText
                    delay(320L)
                    onSaved(toastText)
                },
                onFailure = { err ->
                    saving = false
                    errorMsg = err.message ?: "网络异常，请重试"
                }
            )
        }
    }

    fun submitTodo() {
        if (saving || savingTodo || !hasToken) return
        savingTodo = true
        errorMsg = ""
        val cleanTitle = preview.candidateTitle.ifBlank { rawShared.take(60) }.take(72)
        val notesParts = buildList {
            if (noteInput.isNotBlank()) add(noteInput.trim())
            if (preview.extractedUrl.isNotBlank()) add(preview.extractedUrl)
            if (rawShared.length > cleanTitle.length && preview.extractedUrl.isBlank()) {
                add(rawShared.take(600))
            }
        }
        scope.launch {
            api.createTodo(
                title = cleanTitle,
                notes = notesParts.joinToString("\n\n"),
                priority = "normal"
            ).fold(
                onSuccess = {
                    val toastText = "已添加至 BotHub 个人待办"
                    savedMsg = toastText
                    delay(320L)
                    onSaved(toastText)
                },
                onFailure = { err ->
                    savingTodo = false
                    errorMsg = err.message ?: "添加待办失败，请重试"
                }
            )
        }
    }

    LaunchedEffect(rawShared) {
        val warmToken = withContext(Dispatchers.IO) { prefs.warmUpCredentials() }
        hasToken = warmToken.isNotEmpty()
        if (hasToken && prefs.zeroClickCaptureEnabled) {
            submitCapture()
        }
    }

    val scrimInteraction = remember { MutableInteractionSource() }
    val sheetInteraction = remember { MutableInteractionSource() }
    val busy = saving || savingTodo

    Box(
        modifier = modifier
            .fillMaxSize()
            .clickable(
                interactionSource = scrimInteraction,
                indication = null,
                onClick = onDismiss
            ),
        contentAlignment = Alignment.BottomCenter
    ) {
        Surface(
            modifier = Modifier
                .fillMaxWidth()
                .clickable(
                    interactionSource = sheetInteraction,
                    indication = null,
                    onClick = {}
                ),
            shape = RoundedCornerShape(topStart = 20.dp, topEnd = 20.dp),
            color = theme.sheetBg,
            border = BorderStroke(1.dp, theme.divider),
            tonalElevation = 0.dp
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .navigationBarsPadding()
                    .imePadding()
                    .padding(horizontal = 18.dp, vertical = 14.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Box(
                    modifier = Modifier
                        .width(36.dp)
                        .height(4.dp)
                        .background(theme.textMuted.copy(alpha = 0.36f), RoundedCornerShape(999.dp))
                        .align(Alignment.CenterHorizontally)
                )

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                        modifier = Modifier.weight(1f)
                    ) {
                        SpiritSquircleAvatar(
                            bot = null,
                            allBots = emptyList(),
                            avatarStyleOverride = preview.kind.avatarStyle,
                            state = when {
                                busy -> "working"
                                errorMsg.isNotEmpty() -> "blocked"
                                noteInput.isNotEmpty() -> "thinking"
                                else -> "idle"
                            },
                            containerSize = 36.dp,
                            cornerRadius = 10.dp,
                            interactive = true,
                            seedKey = "quick_capture"
                        )
                        Text(
                            text = "快速收藏到 BotHub",
                            fontSize = 16.sp,
                            fontWeight = FontWeight.Bold,
                            color = theme.textPrimary,
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f, fill = false)
                        )
                    }
                    Box(
                        modifier = Modifier
                            .clip(RoundedCornerShape(8.dp))
                            .background(theme.accent.copy(alpha = 0.12f))
                            .border(1.dp, theme.accent.copy(alpha = 0.30f), RoundedCornerShape(8.dp))
                            .padding(horizontal = 8.dp, vertical = 4.dp)
                    ) {
                        Text(
                            text = preview.kind.shareHeaderLabel,
                            style = TextStyle(
                                fontSize = 11.sp,
                                fontWeight = FontWeight.Medium,
                                color = if (theme.isLight) theme.accentDeep else theme.accentLight,
                                platformStyle = NoFontPaddingStyle
                            ),
                            maxLines = 1,
                            softWrap = false
                        )
                    }
                }

                if (savedMsg.isNotEmpty()) {
                    SpecularGlassCard(
                        cornerRadius = 12.dp,
                        accentTint = Color(0xFF10B981),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 11.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(7.dp)
                                    .background(Color(0xFF10B981), CircleShape)
                            )
                            Text(
                                text = savedMsg,
                                fontSize = 13.sp,
                                fontWeight = FontWeight.Medium,
                                color = theme.textPrimary
                            )
                        }
                    }
                }

                SpecularGlassCard(
                    cornerRadius = 12.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                        verticalArrangement = Arrangement.spacedBy(4.dp)
                    ) {
                        Text(
                            text = preview.candidateTitle,
                            fontSize = 14.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textPrimary,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis
                        )
                        if (preview.extractedUrl.isNotEmpty()) {
                            Text(
                                text = preview.extractedUrl,
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.5.sp,
                                color = theme.accent,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                    }
                }

                if (!hasToken) {
                    Text(
                        text = "尚未配置服务端连接，请先打开主程序一键绑定。",
                        fontSize = 13.sp,
                        color = theme.textSecondary
                    )
                    TactilePrimaryButton(
                        text = "打开 BotHub 一键绑定",
                        minHeight = 46.dp,
                        onClick = onOpenSetup,
                        modifier = Modifier.fillMaxWidth()
                    )
                } else {
                    ObsidianGlassInput(
                        value = noteInput,
                        onValueChange = { noteInput = it },
                        placeholder = "随手批注一句想法或提问 (可选)...",
                        singleLine = false,
                        maxLines = 3,
                        modifier = Modifier.fillMaxWidth()
                    )

                    ZeroClickCaptureRow(
                        checked = zeroClick,
                        onCheckedChange = { checked ->
                            zeroClick = checked
                            prefs.zeroClickCaptureEnabled = checked
                        },
                        title = "开启零点击秒收",
                        subtitle = "下次系统分享自动入库并弹 Toast 提示",
                        compact = true
                    )

                    if (errorMsg.isNotEmpty()) {
                        Text(
                            text = errorMsg,
                            fontSize = 12.sp,
                            color = if (theme.isLight) Color(0xFFBE123C) else Color(0xFFFB7185)
                        )
                    }

                    TactilePrimaryButton(
                        text = if (saving) "正在收入收藏匣..." else "收入收藏匣",
                        enabled = !busy,
                        minHeight = 46.dp,
                        onClick = { submitCapture() },
                        modifier = Modifier.fillMaxWidth()
                    )

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        TactileSecondaryButton(
                            text = if (savingTodo) "添加中..." else "转为待办",
                            enabled = !busy,
                            minHeight = 44.dp,
                            onClick = { submitTodo() },
                            modifier = Modifier.weight(1f)
                        )

                        TactileSecondaryButton(
                            text = "发给 AI 深聊",
                            enabled = !busy,
                            minHeight = 44.dp,
                            onClick = { onForwardDeepChat(noteInput.trim()) },
                            modifier = Modifier.weight(1f)
                        )
                    }
                }
                Spacer(modifier = Modifier.height(2.dp))
            }
        }
    }
}
