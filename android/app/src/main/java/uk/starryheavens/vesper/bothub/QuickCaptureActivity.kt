package uk.starryheavens.vesper.bothub

import android.content.Intent
import android.content.res.Configuration
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
import uk.starryheavens.vesper.bothub.sync.ScreenCaptureAccessibilityService
import uk.starryheavens.vesper.bothub.sync.ScreenCaptureHolder
import uk.starryheavens.vesper.bothub.sync.ScreenCaptureSnapshot
import uk.starryheavens.vesper.bothub.ui.sheets.QuickCaptureSheet
import uk.starryheavens.vesper.bothub.ui.theme.VesperBotHubTheme
import uk.starryheavens.vesper.bothub.ui.theme.resolveBotHubThemeColors

class QuickCaptureActivity : ComponentActivity() {

    private var rawSharedState by mutableStateOf("")
    private var forcePreviewState by mutableStateOf(false)
    private var screenSnapshotState by mutableStateOf<ScreenCaptureSnapshot?>(null)
    private var accessibilityEnabledState by mutableStateOf(false)
    private var pendingClipboardRead = false
    private var pendingClipboardUrlMerge = false
    private lateinit var prefs: BotHubPrefs

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        prefs = BotHubPrefs(applicationContext)
        val nightMask = resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK
        val isLightTheme = resolveBotHubThemeColors(
            prefs.appThemeId,
            systemDark = nightMask == Configuration.UI_MODE_NIGHT_YES
        ).isLight
        val barStyle = if (isLightTheme) {
            SystemBarStyle.light(AndroidColor.TRANSPARENT, AndroidColor.TRANSPARENT)
        } else {
            SystemBarStyle.dark(AndroidColor.TRANSPARENT)
        }
        enableEdgeToEdge(
            statusBarStyle = barStyle,
            navigationBarStyle = barStyle
        )

        accessibilityEnabledState = ScreenCaptureAccessibilityService.isServiceEnabled(this)
        val api = BotHubApiClient(prefs)

        if (!applyIncomingIntent(intent)) {
            return
        }

        setContent {
            VesperBotHubTheme(themeId = prefs.appThemeId, dynamicColor = false) {
                if (rawSharedState.isNotBlank() || screenSnapshotState != null) {
                    QuickCaptureSheet(
                        rawShared = rawSharedState,
                        prefs = prefs,
                        api = api,
                        forcePreview = forcePreviewState,
                        screenSnapshot = screenSnapshotState,
                        accessibilityEnabled = accessibilityEnabledState,
                        onOpenAccessibilitySettings = {
                            ScreenCaptureAccessibilityService.openAccessibilitySettings(this)
                            finish()
                        },
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
        accessibilityEnabledState = ScreenCaptureAccessibilityService.isServiceEnabled(this)
        FloatingCaptureService.setTemporarilyHidden(this, true)
    }

    override fun onPause() {
        FloatingCaptureService.setTemporarilyHidden(this, RealtimePushService.isAppInForeground)
        super.onPause()
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        if (!hasFocus) return

        if (pendingClipboardRead) {
            pendingClipboardRead = false
            window.decorView.post {
                val clipText = AppIntents.readClipboardText(this)
                if (clipText.isBlank()) {
                    Toast.makeText(
                        applicationContext,
                        "剪贴板为空，可先开启「悬浮球识屏」直接识别当前页面",
                        Toast.LENGTH_LONG
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
            return
        }

        if (pendingClipboardUrlMerge) {
            pendingClipboardUrlMerge = false
            window.decorView.post {
                val snap = screenSnapshotState ?: return@post
                if (snap.extractedUrl.isNotBlank()) return@post
                val clipText = AppIntents.readClipboardText(this)
                if (clipText.isBlank()) return@post
                val clipPreview = ShareTextParser.parse(clipText)
                if (clipPreview.extractedUrl.isBlank()) return@post
                val key = AppIntents.clipboardDedupKey(clipPreview.extractedUrl, clipText)
                if (snap.autoCopiedLink) {
                    // Auto-clicked "Share -> Copy Link": trust the newly copied URL directly
                    prefs.lastPromptedClipboardHash = key
                    val mergedSnap = snap.copy(
                        extractedUrl = clipPreview.extractedUrl,
                        candidateTitle = clipPreview.candidateTitle.ifBlank { snap.candidateTitle }
                    )
                    screenSnapshotState = mergedSnap
                    rawSharedState = clipText
                    return@post
                }
                // Only merge a brand-new URL matching the foreground platform so stale clipboard URLs don't pollute screen capture
                if (key == prefs.lastPromptedClipboardHash) return@post
                if (isClipboardMatchingApp(snap.sourceAppLabel, clipPreview.extractedUrl)) {
                    prefs.lastPromptedClipboardHash = key
                    val mergedSnap = snap.copy(extractedUrl = clipPreview.extractedUrl)
                    screenSnapshotState = mergedSnap
                    rawSharedState = mergedSnap.buildRawInput()
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        applyIncomingIntent(intent)
    }

    private fun isClipboardMatchingApp(appLabel: String, url: String): Boolean {
        val lower = url.lowercase()
        return when (appLabel) {
            "抖音" -> lower.contains("douyin.com") || lower.contains("iesdouyin.com")
            "小红书" -> lower.contains("xhslink.com") || lower.contains("xiaohongshu.com")
            "Bilibili" -> lower.contains("b23.tv") || lower.contains("bilibili.com")
            "微信" -> lower.contains("mp.weixin.qq.com")
            "知乎" -> lower.contains("zhihu.com")
            else -> false
        }
    }

    private fun applyIncomingIntent(intent: Intent?): Boolean {
        if (intent?.action == AppIntents.ACTION_CAPTURE_SCREEN) {
            val snap = ScreenCaptureHolder.consumePending()
            if (snap == null) {
                finish()
                return false
            }
            forcePreviewState = true
            pendingClipboardRead = false
            screenSnapshotState = snap
            val built = snap.buildRawInput()
            rawSharedState = built.ifBlank {
                if (snap.sourceAppLabel.isNotBlank()) "[${snap.sourceAppLabel}识屏] 当前页面截图" else "屏幕截图收藏"
            }
            pendingClipboardUrlMerge = snap.extractedUrl.isBlank()
            if (hasWindowFocus() && pendingClipboardUrlMerge) {
                onWindowFocusChanged(true)
            }
            return true
        }

        if (intent?.action == AppIntents.ACTION_CAPTURE_CLIPBOARD) {
            screenSnapshotState = null
            forcePreviewState = true
            pendingClipboardRead = true
            pendingClipboardUrlMerge = false
            rawSharedState = ""
            if (hasWindowFocus()) {
                onWindowFocusChanged(true)
            }
            return true
        }

        screenSnapshotState = null
        pendingClipboardRead = false
        pendingClipboardUrlMerge = false
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
