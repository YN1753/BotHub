package uk.starryheavens.vesper.bothub

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.core.content.ContextCompat
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.notify.NotificationHelper
import uk.starryheavens.vesper.bothub.sync.PushSyncWorker
import uk.starryheavens.vesper.bothub.sync.RealtimePushService
import uk.starryheavens.vesper.bothub.ui.BotHubApp
import uk.starryheavens.vesper.bothub.ui.theme.VesperBotHubTheme

class MainActivity : ComponentActivity() {

    private var sharedTextState by mutableStateOf<String?>(null)
    private var requestedTabState by mutableStateOf<Int?>(null)

    private val requestPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { _ -> }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        NotificationHelper.ensureChannels(this)
        requestNotificationPermissionIfNeeded()
        PushSyncWorker.schedulePeriodicSync(this)

        val prefs = BotHubPrefs(this)
        if (prefs.realtimeServiceEnabled && prefs.bearerToken.isNotEmpty()) {
            RealtimePushService.start(this)
        }

        extractIncomingIntent(intent)

        val api = BotHubApiClient(prefs)
        setContent {
            VesperBotHubTheme {
                BotHubApp(
                    prefs = prefs,
                    api = api,
                    sharedText = sharedTextState,
                    onSharedTextConsumed = { sharedTextState = null },
                    requestedTab = requestedTabState,
                    onRequestedTabConsumed = { requestedTabState = null }
                )
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        extractIncomingIntent(intent)
    }

    private fun extractIncomingIntent(intent: Intent?) {
        if (intent == null) return
        val deepChat = intent.getStringExtra(EXTRA_DEEP_CHAT_TEXT)?.trim()
        if (!deepChat.isNullOrEmpty()) {
            sharedTextState = deepChat
            intent.removeExtra(EXTRA_DEEP_CHAT_TEXT)
            return
        }
        if (intent.action == Intent.ACTION_SEND && intent.type == "text/plain") {
            val text = intent.getStringExtra(Intent.EXTRA_TEXT)?.trim()
            if (!text.isNullOrEmpty()) {
                sharedTextState = text
            }
            return
        }
        val tabIdx = intent.getIntExtra("open_tab", -1)
        if (tabIdx >= 0) {
            requestedTabState = tabIdx
            intent.removeExtra("open_tab")
        }
    }

    companion object {
        const val EXTRA_DEEP_CHAT_TEXT = "extra_deep_chat_text"
    }

    private fun requestNotificationPermissionIfNeeded() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                requestPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
            }
        }
    }
}
