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
import androidx.compose.foundation.layout.width
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
import kotlinx.coroutines.launch
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
import uk.starryheavens.vesper.bothub.ui.components.specularRimBrush

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
    val scope = rememberCoroutineScope()
    val preview = remember(rawShared) { ShareTextParser.parse(rawShared) }
    var noteInput by remember { mutableStateOf("") }
    var zeroClick by remember { mutableStateOf(prefs.zeroClickCaptureEnabled) }
    var saving by remember { mutableStateOf(false) }
    var errorMsg by remember { mutableStateOf("") }

    val hasToken = prefs.bearerToken.isNotEmpty()

    fun submitCapture() {
        if (saving || !hasToken) return
        saving = true
        errorMsg = ""
        scope.launch {
            api.createStashItem(rawInput = rawShared, userNote = noteInput).fold(
                onSuccess = { (_, deduplicated) ->
                    val toastText = if (deduplicated) {
                        "该链接已在收藏匣，已合并最新批注"
                    } else {
                        "已秒收入收藏匣，后台正异步提炼要点"
                    }
                    onSaved(toastText)
                },
                onFailure = { err ->
                    saving = false
                    errorMsg = err.message ?: "网络异常，请重试"
                }
            )
        }
    }

    LaunchedEffect(Unit) {
        if (hasToken && prefs.zeroClickCaptureEnabled) {
            submitCapture()
        }
    }

    val scrimInteraction = remember { MutableInteractionSource() }
    val sheetInteraction = remember { MutableInteractionSource() }

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
            shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
            color = Color(0xFF080D1B),
            border = BorderStroke(0.9.dp, specularRimBrush(Color(0x38FFF8EE), Color(0x14FFF8EE), Color.Transparent)),
            tonalElevation = 0.dp
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .navigationBarsPadding()
                    .imePadding()
                    .padding(horizontal = 20.dp, vertical = 14.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Box(
                    modifier = Modifier
                        .width(38.dp)
                        .height(4.dp)
                        .background(Color(0x5CCBD5E1), RoundedCornerShape(999.dp))
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
                                saving -> "working"
                                errorMsg.isNotEmpty() -> "blocked"
                                noteInput.isNotEmpty() -> "thinking"
                                else -> "idle"
                            },
                            containerSize = 38.dp,
                            cornerRadius = 12.dp,
                            interactive = true,
                            seedKey = "quick_capture"
                        )
                        Text(
                            text = "快速收藏",
                            fontSize = 16.sp,
                            fontWeight = FontWeight.Bold,
                            color = Color(0xFFFFF8EE),
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f, fill = false)
                        )
                    }
                    Box(
                        modifier = Modifier
                            .clip(RoundedCornerShape(8.dp))
                            .background(Color(0x24F2B880))
                            .border(0.5.dp, Color(0x5CF2B880), RoundedCornerShape(8.dp))
                            .padding(horizontal = 8.dp, vertical = 3.dp)
                    ) {
                        Text(
                            text = preview.kind.shareHeaderLabel,
                            style = TextStyle(
                                fontSize = 10.5.sp,
                                fontWeight = FontWeight.Medium,
                                color = Color(0xFFFFE2BE),
                                platformStyle = NoFontPaddingStyle
                            ),
                            maxLines = 1,
                            softWrap = false
                        )
                    }
                }

                SpecularGlassCard(
                    cornerRadius = 16.dp,
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
                            color = Color(0xFFFFF8EE),
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis
                        )
                        if (preview.extractedUrl.isNotEmpty()) {
                            Text(
                                text = preview.extractedUrl,
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.sp,
                                color = Color(0xFFA5B4FC),
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
                        color = Color(0xFFFFE2BE)
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
                        placeholder = "随手批注一句想法，AI 提炼时一并参考 (可选)...",
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
                        subtitle = "下次系统分享自动入库并秒回原应用",
                        compact = true
                    )

                    if (errorMsg.isNotEmpty()) {
                        Text(
                            text = errorMsg,
                            fontSize = 12.sp,
                            color = Color(0xFFFDA4AF)
                        )
                    }

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        TactileSecondaryButton(
                            text = "发给 AI 深聊",
                            enabled = !saving,
                            minHeight = 46.dp,
                            onClick = { onForwardDeepChat(noteInput.trim()) },
                            modifier = Modifier.weight(1f)
                        )

                        TactilePrimaryButton(
                            text = if (saving) "秒收中..." else "收入收藏匣",
                            enabled = !saving,
                            minHeight = 46.dp,
                            onClick = { submitCapture() },
                            modifier = Modifier.weight(1.25f)
                        )
                    }
                }
                Spacer(modifier = Modifier.height(2.dp))
            }
        }
    }
}
