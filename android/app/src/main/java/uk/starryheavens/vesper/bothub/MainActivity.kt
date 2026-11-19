package uk.starryheavens.vesper.bothub

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Color as AndroidColor
import android.os.Build
import android.os.Bundle
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.navigation.AppIntents
import uk.starryheavens.vesper.bothub.navigation.BotHubTab
import uk.starryheavens.vesper.bothub.notify.NotificationHelper
import uk.starryheavens.vesper.bothub.sync.PushSyncWorker
import uk.starryheavens.vesper.bothub.sync.RealtimePushService
import uk.starryheavens.vesper.bothub.ui.BotHubApp
import uk.starryheavens.vesper.bothub.ui.theme.VesperBotHubTheme

class MainActivity : FragmentActivity() {

    private var sharedTextState by mutableStateOf<String?>(null)
    private var sharedAutoSendState by mutableStateOf(false)
    private var requestedTabState by mutableStateOf<BotHubTab?>(null)
    private var requestedNotificationIdState by mutableStateOf<Long?>(null)
    private var requestedBotIdState by mutableStateOf<String?>(null)
    private var pairPayloadState by mutableStateOf<Pair<String, String>?>(null)

    private val requestPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { _ -> }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(AndroidColor.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(AndroidColor.TRANSPARENT)
        )

        val prefs = BotHubPrefs(applicationContext)
        val api = BotHubApiClient(prefs)

        if (savedInstanceState == null) {
            extractIncomingIntent(intent)
        }

        setContent {
            VesperBotHubTheme(themeId = prefs.appThemeId) {
                BotHubApp(
                    prefs = prefs,
                    api = api,
                    sharedText = sharedTextState,
                    sharedAutoSend = sharedAutoSendState,
                    onSharedTextConsumed = {
                        sharedTextState = null
                        sharedAutoSendState = false
                    },
                    requestedTab = requestedTabState,
                    onRequestedTabConsumed = { requestedTabState = null },
                    requestedNotificationId = requestedNotificationIdState,
                    onRequestedNotificationIdConsumed = { requestedNotificationIdState = null },
                    requestedBotId = requestedBotIdState,
                    onRequestedBotIdConsumed = { requestedBotIdState = null },
                    pairPayload = pairPayloadState,
                    onPairPayloadConsumed = { pairPayloadState = null }
                )
            }
        }

        // Defer Keystore initialization, WorkManager DB setup, and Foreground Service sync off Frame 1
        requestNotificationPermissionIfNeeded()
        lifecycleScope.launch(Dispatchers.IO) {
            NotificationHelper.ensureChannels(applicationContext)
            prefs.warmUpCredentials()
            PushSyncWorker.schedulePeriodicSync(applicationContext)
            RealtimePushService.syncWithPrefs(applicationContext, prefs)
        }
    }

    override fun onResume() {
        super.onResume()
        RealtimePushService.isAppInForeground = true
    }

    override fun onPause() {
        RealtimePushService.isAppInForeground = false
        RealtimePushService.activeViewingBotId = ""
        super.onPause()
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        extractIncomingIntent(intent)
    }

    private fun extractIncomingIntent(intent: Intent?) {
        if (intent == null) return

        AppIntents.extractPairPayload(intent)?.let { payload ->
            pairPayloadState = payload
            return
        }

        val deepChat = intent.getStringExtra(AppIntents.EXTRA_DEEP_CHAT_TEXT)?.trim()
        if (!deepChat.isNullOrEmpty()) {
            sharedTextState = deepChat
            sharedAutoSendState = intent.getBooleanExtra(AppIntents.EXTRA_DEEP_CHAT_AUTO_SEND, false)
            intent.removeExtra(AppIntents.EXTRA_DEEP_CHAT_TEXT)
            intent.removeExtra(AppIntents.EXTRA_DEEP_CHAT_AUTO_SEND)
            return
        }

        val shared = AppIntents.extractSharedText(intent, this)
        if (shared.isNotEmpty()) {
            sharedTextState = shared
            sharedAutoSendState = false
            intent.removeExtra(Intent.EXTRA_TEXT)
            intent.removeExtra(Intent.EXTRA_SUBJECT)
            return
        }

        val botId = intent.getStringExtra(AppIntents.EXTRA_BOT_ID)?.trim()
        if (!botId.isNullOrEmpty()) {
            requestedBotIdState = botId
            intent.removeExtra(AppIntents.EXTRA_BOT_ID)
            intent.removeExtra(AppIntents.EXTRA_OPEN_TAB)
            return
        }

        val tabIdx = intent.getIntExtra(AppIntents.EXTRA_OPEN_TAB, -1)
        BotHubTab.fromIndex(tabIdx)?.let { tab ->
            requestedTabState = tab
            intent.removeExtra(AppIntents.EXTRA_OPEN_TAB)
        }

        val notificationId = intent.getLongExtra(AppIntents.EXTRA_NOTIFICATION_ID, -1L)
        if (notificationId > 0L) {
            requestedNotificationIdState = notificationId
            intent.removeExtra(AppIntents.EXTRA_NOTIFICATION_ID)
        }
    }

    private fun requestNotificationPermissionIfNeeded() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                requestPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
            }
        }
    }
}
