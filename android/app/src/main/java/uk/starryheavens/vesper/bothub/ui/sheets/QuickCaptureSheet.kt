package uk.starryheavens.vesper.bothub.ui.sheets

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
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
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.data.ShareTextParser
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.ZeroClickCaptureRow

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
            shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
            color = Color(0xFF090E1D),
            border = BorderStroke(1.dp, Color(0x28FFF8EE)),
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
                        .width(36.dp)
                        .height(4.dp)
                        .background(Color(0x52CBD5E1), RoundedCornerShape(999.dp))
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
                        GrokBotAvatar(
                            avatarStyle = preview.kind.avatarStyle,
                            state = when {
                                saving -> "working"
                                errorMsg.isNotEmpty() -> "blocked"
                                noteInput.isNotEmpty() -> "thinking"
                                else -> "idle"
                            },
                            size = 34.dp,
                            interactive = true,
                            isListening = noteInput.isNotEmpty(),
                            seedKey = "quick_capture"
                        )
                        Text(
                            text = "BotHub 智能收藏与闪念匣",
                            style = MaterialTheme.typography.titleMedium,
                            fontSize = 16.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFF8EE),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                    Surface(
                        shape = RoundedCornerShape(8.dp),
                        color = Color(0x24F2B880),
                        border = BorderStroke(0.5.dp, Color(0x5CF2B880))
                    ) {
                        Text(
                            text = preview.kind.shareHeaderLabel,
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Medium,
                            color = Color(0xFFFFE2BE),
                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp)
                        )
                    }
                }

                // Parsed Title & URL Preview Card
                Surface(
                    shape = RoundedCornerShape(16.dp),
                    color = Color(0xCC0D1326),
                    border = BorderStroke(1.dp, Color(0x22FFF8EE)),
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
                    Button(
                        onClick = onOpenSetup,
                        shape = RoundedCornerShape(14.dp),
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 46.dp)
                    ) {
                        Text("打开 BotHub 一键绑定", fontWeight = FontWeight.SemiBold)
                    }
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
                        Surface(
                            onClick = { if (!saving) onForwardDeepChat(noteInput.trim()) },
                            shape = RoundedCornerShape(14.dp),
                            color = Color(0xFF11182E),
                            border = BorderStroke(1.dp, Color(0x29FFF8EE)),
                            modifier = Modifier
                                .weight(1f)
                                .sizeIn(minHeight = 46.dp)
                        ) {
                            Box(contentAlignment = Alignment.Center) {
                                Text(
                                    text = "发给 AI 深聊",
                                    fontSize = 13.5.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = Color(0xFFE2E8F0)
                                )
                            }
                        }

                        Button(
                            onClick = { submitCapture() },
                            enabled = !saving,
                            shape = RoundedCornerShape(14.dp),
                            colors = ButtonDefaults.buttonColors(
                                containerColor = MaterialTheme.colorScheme.primary,
                                contentColor = MaterialTheme.colorScheme.onPrimary
                            ),
                            modifier = Modifier
                                .weight(1.25f)
                                .sizeIn(minHeight = 46.dp)
                        ) {
                            Text(
                                text = if (saving) "秒收中..." else "收入收藏匣",
                                fontSize = 13.5.sp,
                                fontWeight = FontWeight.SemiBold
                            )
                        }
                    }
                }
                Spacer(modifier = Modifier.height(2.dp))
            }
        }
    }
}
