package uk.starryheavens.vesper.bothub

import android.content.Intent
import android.graphics.Color as AndroidColor
import android.os.Bundle
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.data.ShareTextParser
import uk.starryheavens.vesper.bothub.navigation.AppIntents
import uk.starryheavens.vesper.bothub.navigation.BotHubTab
import uk.starryheavens.vesper.bothub.sync.FloatingCaptureService
import uk.starryheavens.vesper.bothub.sync.RealtimePushService
import uk.starryheavens.vesper.bothub.ui.sheets.QuickCaptureSheet
import uk.starryheavens.vesper.bothub.ui.theme.VesperBotHubTheme

class QuickCaptureActivity : ComponentActivity() {

    private var rawSharedState by mutableStateOf("")
    private var forcePreviewState by mutableStateOf(false)
    private var pendingClipboardRead = false
    private lateinit var prefs: BotHubPrefs

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(AndroidColor.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(AndroidColor.TRANSPARENT)
        )

        prefs = BotHubPrefs(applicationContext)
        val api = BotHubApiClient(prefs)

        if (!applyIncomingIntent(intent)) {
            return
        }

        setContent {
            VesperBotHubTheme(themeId = prefs.appThemeId, dynamicColor = false) {
                if (rawSharedState.isNotBlank()) {
                    QuickCaptureSheet(
                        rawShared = rawSharedState,
                        prefs = prefs,
                        api = api,
                        forcePreview = forcePreviewState,
                        onSaved = { msg ->
                            if (msg.isNotBlank()) {
                                Toast.makeText(applicationContext, msg, Toast.LENGTH_SHORT).show()
                            }
                            finish()
                        },
                        onForwardDeepChat = { note ->
                            val hasNote = note.isNotBlank()
                            val merged = if (hasNote) {
                                "${note.trim()}\n\n$rawSharedState"
                            } else {
                                rawSharedState
                            }
                            startActivity(
                                AppIntents.createMainActivityIntent(
                                    context = this,
                                    deepChatText = merged,
                                    autoSendDeepChat = hasNote,
                                    openTab = BotHubTab.AGENT
                                )
                            )
                            finish()
                        },
                        onOpenSetup = {
                            startActivity(AppIntents.createMainActivityIntent(context = this))
                            finish()
                        },
                        onDismiss = { finish() }
                    )
                }
            }
        }
    }

    override fun onResume() {
        super.onResume()
        FloatingCaptureService.setTemporarilyHidden(this, true)
    }

    override fun onPause() {
        FloatingCaptureService.setTemporarilyHidden(this, RealtimePushService.isAppInForeground)
        super.onPause()
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        if (hasFocus && pendingClipboardRead) {
            pendingClipboardRead = false
            window.decorView.post {
                val clipText = AppIntents.readClipboardText(this)
                if (clipText.isBlank()) {
                    Toast.makeText(
                        applicationContext,
                        "剪贴板为空，请先复制链接或文字",
                        Toast.LENGTH_SHORT
                    ).show()
                    finish()
                    return@post
                }
                val preview = ShareTextParser.parse(clipText)
                if (preview.extractedUrl.isNotBlank()) {
                    prefs.lastPromptedClipboardHash = AppIntents.clipboardDedupKey(preview.extractedUrl, clipText)
                }
                rawSharedState = clipText
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        applyIncomingIntent(intent)
    }

    private fun applyIncomingIntent(intent: Intent?): Boolean {
        if (intent?.action == AppIntents.ACTION_CAPTURE_CLIPBOARD) {
            forcePreviewState = true
            pendingClipboardRead = true
            rawSharedState = ""
            if (hasWindowFocus()) {
                onWindowFocusChanged(true)
            }
            return true
        }

        pendingClipboardRead = false
        forcePreviewState = intent?.getBooleanExtra(AppIntents.EXTRA_FORCE_PREVIEW, false) ?: false
        val extracted = AppIntents.extractSharedText(intent, this)
        if (extracted.isBlank()) {
            Toast.makeText(
                applicationContext,
                "未检测到文字或链接，请复制链接或附带文字后分享",
                Toast.LENGTH_LONG
            ).show()
            finish()
            return false
        }
        rawSharedState = extracted
        return true
    }
}
