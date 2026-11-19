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
import uk.starryheavens.vesper.bothub.navigation.AppIntents
import uk.starryheavens.vesper.bothub.navigation.BotHubTab
import uk.starryheavens.vesper.bothub.ui.sheets.QuickCaptureSheet
import uk.starryheavens.vesper.bothub.ui.theme.VesperBotHubTheme

class QuickCaptureActivity : ComponentActivity() {

    private var rawSharedState by mutableStateOf("")

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(AndroidColor.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(AndroidColor.TRANSPARENT)
        )

        val extracted = AppIntents.extractSharedText(intent, this)
        if (extracted.isBlank()) {
            Toast.makeText(
                applicationContext,
                "未检测到文字或链接，请复制链接或附带文字后分享",
                Toast.LENGTH_LONG
            ).show()
            finish()
            return
        }
        rawSharedState = extracted

        val prefs = BotHubPrefs(applicationContext)
        val api = BotHubApiClient(prefs)

        setContent {
            VesperBotHubTheme(themeId = prefs.appThemeId, dynamicColor = false) {
                QuickCaptureSheet(
                    rawShared = rawSharedState,
                    prefs = prefs,
                    api = api,
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

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        val extracted = AppIntents.extractSharedText(intent, this)
        if (extracted.isNotBlank()) {
            rawSharedState = extracted
        }
    }
}
