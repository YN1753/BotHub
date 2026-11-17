package uk.starryheavens.vesper.bothub

import android.content.Intent
import android.os.Bundle
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
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
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.theme.VesperBotHubTheme

class QuickCaptureActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        val rawShared = extractIncomingText(intent)
        if (rawShared.isBlank()) {
            finish()
            return
        }

        val prefs = BotHubPrefs(this)
        val api = BotHubApiClient(prefs)

        setContent {
            VesperBotHubTheme(dynamicColor = false) {
                QuickCaptureSheet(
                    rawShared = rawShared,
                    prefs = prefs,
                    api = api,
                    onSaved = { msg ->
                        Toast.makeText(applicationContext, msg, Toast.LENGTH_SHORT).show()
                        finish()
                    },
                    onForwardDeepChat = { note ->
                        val merged = if (note.isBlank()) rawShared else "$rawShared\n\n附注：$note"
                        val chatIntent = Intent(this, MainActivity::class.java).apply {
                            putExtra(MainActivity.EXTRA_DEEP_CHAT_TEXT, merged)
                            addFlags(
                                Intent.FLAG_ACTIVITY_NEW_TASK or
                                    Intent.FLAG_ACTIVITY_CLEAR_TOP or
                                    Intent.FLAG_ACTIVITY_SINGLE_TOP
                            )
                        }
                        startActivity(chatIntent)
                        finish()
                    },
                    onOpenSetup = {
                        val mainIntent = Intent(this, MainActivity::class.java).apply {
                            addFlags(
                                Intent.FLAG_ACTIVITY_NEW_TASK or
                                    Intent.FLAG_ACTIVITY_CLEAR_TOP or
                                    Intent.FLAG_ACTIVITY_SINGLE_TOP
                            )
                        }
                        startActivity(mainIntent)
                        finish()
                    },
                    onDismiss = { finish() }
                )
            }
        }
    }

    private fun extractIncomingText(intent: Intent?): String {
        if (intent == null) return ""
        if (intent.action == Intent.ACTION_SEND && intent.type?.startsWith("text/") == true) {
            val subject = intent.getStringExtra(Intent.EXTRA_SUBJECT)?.trim().orEmpty()
            val body = intent.getStringExtra(Intent.EXTRA_TEXT)?.trim().orEmpty()
            if (subject.isNotEmpty() && body.isNotEmpty() && !body.contains(subject)) {
                return "$subject $body"
            }
            return body.ifEmpty { subject }
        }
        return intent.getStringExtra(Intent.EXTRA_TEXT)?.trim().orEmpty()
    }
}

@Composable
private fun QuickCaptureSheet(
    rawShared: String,
    prefs: BotHubPrefs,
    api: BotHubApiClient,
    onSaved: (String) -> Unit,
    onForwardDeepChat: (String) -> Unit,
    onOpenSetup: () -> Unit,
    onDismiss: () -> Unit
) {
    val scope = rememberCoroutineScope()
    val preview = remember(rawShared) { BotHubApiClient.parseSharePreview(rawShared) }
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

    // Zero-click fast path: auto-submit immediately upon share sheet launch and return to source app
    LaunchedEffect(Unit) {
        if (hasToken && prefs.zeroClickCaptureEnabled) {
            submitCapture()
        }
    }

    val scrimInteraction = remember { MutableInteractionSource() }
    val sheetInteraction = remember { MutableInteractionSource() }

    Box(
        modifier = Modifier
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
            shape = RoundedCornerShape(topStart = 24.dp, topEnd = 24.dp),
            color = Color(0xFF0A0F1F),
            border = BorderStroke(1.dp, Color(0xFF223052)),
            tonalElevation = 8.dp
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .navigationBarsPadding()
                    .imePadding()
                    .padding(horizontal = 20.dp, vertical = 16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                // Drag Handle Indicator
                Box(
                    modifier = Modifier
                        .width(36.dp)
                        .height(4.dp)
                        .background(Color(0xFF334268), RoundedCornerShape(2.dp))
                        .align(Alignment.CenterHorizontally)
                )

                val captureSpiritStyle = when {
                    preview.kindLabel.contains("Bilibili", ignoreCase = true) -> "radar:emerald"
                    preview.kindLabel.contains("GitHub", ignoreCase = true) -> "sentinel:indigo"
                    preview.kindLabel.contains("闪念") -> "swarm:violet"
                    else -> "butler:gold"
                }

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
                            avatarStyle = captureSpiritStyle,
                            state = when {
                                saving -> "working"
                                errorMsg.isNotEmpty() -> "blocked"
                                noteInput.isNotEmpty() -> "thinking"
                                else -> "idle"
                            },
                            size = 36.dp,
                            interactive = true,
                            isListening = noteInput.isNotEmpty(),
                            seedKey = "quick_capture"
                        )
                        Text(
                            text = "Vesper 智能收藏与闪念匣",
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFEAF0FF),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                    Surface(
                        shape = RoundedCornerShape(8.dp),
                        color = Color(0xFF172342),
                        border = BorderStroke(1.dp, Color(0xFFF2B880).copy(alpha = 0.45f))
                    ) {
                        Text(
                            text = preview.kindLabel,
                            style = MaterialTheme.typography.labelSmall,
                            color = Color(0xFFF2B880),
                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp)
                        )
                    }
                }

                // Parsed Title & URL Preview Card
                Card(
                    shape = RoundedCornerShape(14.dp),
                    colors = CardDefaults.cardColors(containerColor = Color(0xFF121A30)),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(12.dp),
                        verticalArrangement = Arrangement.spacedBy(4.dp)
                    ) {
                        Text(
                            text = preview.candidateTitle,
                            style = MaterialTheme.typography.bodyMedium,
                            fontWeight = FontWeight.Medium,
                            color = Color(0xFFEAF0FF),
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis
                        )
                        if (preview.extractedUrl.isNotEmpty()) {
                            Text(
                                text = preview.extractedUrl,
                                style = MaterialTheme.typography.labelSmall,
                                color = Color(0xFF8CA5D0),
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                    }
                }

                if (!hasToken) {
                    Text(
                        text = "尚未配置服务端连接密钥，请先打开主程序完成连接。",
                        style = MaterialTheme.typography.bodySmall,
                        color = Color(0xFFFFB74D)
                    )
                    Button(
                        onClick = onOpenSetup,
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 48.dp)
                    ) {
                        Text("打开 BotHub 配置连接")
                    }
                } else {
                    OutlinedTextField(
                        value = noteInput,
                        onValueChange = { noteInput = it },
                        label = { Text("随手批注 (可选)") },
                        placeholder = { Text("记录一句想法，AI 提炼时一并参考...") },
                        maxLines = 3,
                        shape = RoundedCornerShape(12.dp),
                        modifier = Modifier.fillMaxWidth()
                    )

                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 48.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = "开启零点击秒收",
                                style = MaterialTheme.typography.bodySmall,
                                fontWeight = FontWeight.Medium,
                                color = Color(0xFFD6E1F8)
                            )
                            Text(
                                text = "下次系统分享时自动入库并秒回原应用（可在主程序设置关闭）",
                                style = MaterialTheme.typography.labelSmall,
                                color = Color(0xFF8393B8)
                            )
                        }
                        Switch(
                            checked = zeroClick,
                            onCheckedChange = { checked ->
                                zeroClick = checked
                                prefs.zeroClickCaptureEnabled = checked
                            }
                        )
                    }

                    if (errorMsg.isNotEmpty()) {
                        Text(
                            text = errorMsg,
                            style = MaterialTheme.typography.labelMedium,
                            color = Color(0xFFFF8A80)
                        )
                    }

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        OutlinedButton(
                            onClick = { onForwardDeepChat(noteInput.trim()) },
                            enabled = !saving,
                            shape = RoundedCornerShape(12.dp),
                            modifier = Modifier
                                .weight(1f)
                                .sizeIn(minHeight = 48.dp)
                        ) {
                            Text("发给 AI 深聊")
                        }
                        Button(
                            onClick = { submitCapture() },
                            enabled = !saving,
                            shape = RoundedCornerShape(12.dp),
                            colors = ButtonDefaults.buttonColors(
                                containerColor = Color(0xFFF2B880),
                                contentColor = Color(0xFF070A13)
                            ),
                            modifier = Modifier
                                .weight(1.25f)
                                .sizeIn(minHeight = 48.dp)
                        ) {
                            Text(
                                text = if (saving) "秒收中..." else "收入收藏匣",
                                fontWeight = FontWeight.SemiBold
                            )
                        }
                    }
                }
                Spacer(modifier = Modifier.height(4.dp))
            }
        }
    }
}
