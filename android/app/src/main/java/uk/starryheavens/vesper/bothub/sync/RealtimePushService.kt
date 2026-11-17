package uk.starryheavens.vesper.bothub.sync

import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.IBinder
import androidx.core.app.NotificationCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import uk.starryheavens.vesper.bothub.R
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.data.JsonMappers
import uk.starryheavens.vesper.bothub.notify.NotificationHelper

class RealtimePushService : Service() {

    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var streamJob: Job? = null

    override fun onCreate() {
        super.onCreate()
        NotificationHelper.ensureChannels(this)
        val ongoing = NotificationCompat.Builder(this, NotificationHelper.CHANNEL_SERVICE)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle("Vesper BotHub 实时通道已连接")
            .setContentText("正在监听自建 SSE 原生推送流与断线补发")
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setOngoing(true)
            .build()
        startForeground(FOREGROUND_NOTIFICATION_ID, ongoing)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (streamJob?.isActive != true) {
            streamJob = serviceScope.launch {
                val prefs = BotHubPrefs(applicationContext)
                val api = BotHubApiClient(prefs)
                while (isActive) {
                    if (prefs.bearerToken.isEmpty() || !prefs.realtimeServiceEnabled) {
                        stopSelf()
                        break
                    }
                    runCatching {
                        connectSSE(prefs, api)
                    }
                    delay(8000L)
                }
            }
        }
        return START_STICKY
    }

    private suspend fun connectSSE(prefs: BotHubPrefs, api: BotHubApiClient) {
        api.openNotificationEventStream(prefs.lastEventId) { event, obj ->
            when (event) {
                "connected" -> {
                    val serverLatest = obj.optLong("latest_id", 0L)
                    if (prefs.lastEventId == 0L && serverLatest > 0L) {
                        prefs.advanceLastEventId(serverLatest)
                    }
                }
                "notification" -> {
                    val item = JsonMappers.parseNotification(obj)
                    prefs.advanceLastEventId(item.id)
                    NotificationHelper.showNotification(applicationContext, item)
                }
            }
        }
    }

    override fun onDestroy() {
        streamJob?.cancel()
        serviceScope.cancel()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    companion object {
        private const val FOREGROUND_NOTIFICATION_ID = 9001

        fun start(context: Context) {
            runCatching {
                val appContext = context.applicationContext
                appContext.startForegroundService(Intent(appContext, RealtimePushService::class.java))
            }
        }

        fun stop(context: Context) {
            runCatching {
                val appContext = context.applicationContext
                appContext.stopService(Intent(appContext, RealtimePushService::class.java))
            }
        }

        fun syncWithPrefs(context: Context, prefs: BotHubPrefs = BotHubPrefs(context)) {
            if (prefs.realtimeServiceEnabled && prefs.bearerToken.isNotEmpty()) {
                start(context)
            } else {
                stop(context)
            }
        }
    }
}
