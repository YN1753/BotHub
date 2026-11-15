package uk.starryheavens.vesper.bothub.sync

import android.app.Service
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
import okhttp3.Request
import org.json.JSONObject
import uk.starryheavens.vesper.bothub.R
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.notify.NotificationHelper
import java.io.BufferedReader
import java.io.InputStreamReader

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
        startForeground(9001, ongoing)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (streamJob?.isActive != true) {
            streamJob = serviceScope.launch {
                val prefs = BotHubPrefs(applicationContext)
                val api = BotHubApiClient(prefs)
                while (isActive) {
                    if (prefs.bearerToken.isNotEmpty()) {
                        runCatching {
                            connectSSE(prefs, api)
                        }
                    }
                    delay(8000L)
                }
            }
        }
        return START_STICKY
    }

    private fun connectSSE(prefs: BotHubPrefs, api: BotHubApiClient) {
        val lastId = prefs.lastEventId
        val url = "${prefs.serverUrl}/api/v2/events?last_event_id=$lastId"
        val req = Request.Builder()
            .url(url)
            .header("Accept", "text/event-stream")
            .header("Authorization", "Bearer ${prefs.bearerToken}")
            .get()
            .build()

        api.httpClient.newCall(req).execute().use { resp ->
            if (!resp.isSuccessful) return
            val stream = resp.body?.byteStream() ?: return
            val reader = BufferedReader(InputStreamReader(stream, Charsets.UTF_8))
            var currentEvent = "message"
            var line: String?

            while (reader.readLine().also { line = it } != null) {
                val trimmed = line?.trim().orEmpty()
                if (trimmed.isEmpty()) continue
                if (trimmed.startsWith("event:")) {
                    currentEvent = trimmed.removePrefix("event:").trim()
                } else if (trimmed.startsWith("data:")) {
                    val dataStr = trimmed.removePrefix("data:").trim()
                    val obj = runCatching { JSONObject(dataStr) }.getOrNull() ?: continue
                    if (currentEvent == "connected") {
                        val serverLatest = obj.optLong("latest_id", 0L)
                        if (prefs.lastEventId == 0L && serverLatest > 0L) {
                            prefs.lastEventId = serverLatest
                        }
                    } else if (currentEvent == "notification") {
                        val item = BotHubApiClient.parseSingleNotification(obj)
                        if (item.id > prefs.lastEventId) {
                            prefs.lastEventId = item.id
                        }
                        NotificationHelper.showNotification(applicationContext, item)
                    }
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
}
