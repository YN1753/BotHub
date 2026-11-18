package uk.starryheavens.vesper.bothub

import android.graphics.Color as AndroidColor
import android.os.Bundle
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.navigation.AppIntents
import uk.starryheavens.vesper.bothub.navigation.BotHubTab
import uk.starryheavens.vesper.bothub.ui.sheets.QuickCaptureSheet
import uk.starryheavens.vesper.bothub.ui.theme.VesperBotHubTheme

class QuickCaptureActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(AndroidColor.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(AndroidColor.TRANSPARENT)
        )

        val rawShared = AppIntents.extractSharedText(intent)
        if (rawShared.isBlank()) {
            finish()
            return
        }

        val prefs = BotHubPrefs(applicationContext)
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
                        startActivity(
                            AppIntents.createMainActivityIntent(
                                context = this,
                                deepChatText = merged,
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
