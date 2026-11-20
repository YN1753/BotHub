package uk.starryheavens.vesper.bothub.sync

import android.app.AlarmManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.os.Build
import android.os.IBinder
import android.os.SystemClock
import androidx.core.app.NotificationCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull
import org.json.JSONObject
import uk.starryheavens.vesper.bothub.R
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.data.JsonMappers
import uk.starryheavens.vesper.bothub.navigation.AppIntents
import uk.starryheavens.vesper.bothub.notify.NotificationHelper
import java.time.LocalDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter

enum class PushConnectionStatus {
    DISCONNECTED,
    CONNECTING,
    CONNECTED
}

class RealtimePushService : Service() {

    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val reconnectSignal = MutableSharedFlow<Unit>(extraBufferCapacity = 4)
    private var streamJob: Job? = null

    @Volatile
    private var activeSseJob: Job? = null

    @Volatile
    private var immediateReconnectRequested: Boolean = false

    private var connectivityManager: ConnectivityManager? = null
    private var networkCallback: ConnectivityManager.NetworkCallback? = null

    @Volatile
    private var currentNetworkHandle: Long = -1L

    @Volatile
    private var networkWasLost: Boolean = false

    override fun onCreate() {
        super.onCreate()
        NotificationHelper.ensureChannels(this)
        val openMainPending = PendingIntent.getActivity(
            this,
            9200,
            AppIntents.createMainActivityIntent(this),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val captureClipboardPending = AppIntents.createClipboardCapturePendingIntent(this, 9201)
        val ongoing = NotificationCompat.Builder(this, NotificationHelper.CHANNEL_SERVICE)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle("BotHub")
            .setContentText("点击下方按钮可随时收藏剪贴板内容")
            .setContentIntent(openMainPending)
            .addAction(R.drawable.ic_notification, "收藏剪贴板", captureClipboardPending)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setSilent(true)
            .setShowWhen(false)
            .setVisibility(NotificationCompat.VISIBILITY_PUBLIC)
            .setOngoing(true)
            .build()

        val startedForeground = runCatching {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                startForeground(
                    FOREGROUND_NOTIFICATION_ID,
                    ongoing,
                    ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC
                )
            } else {
                startForeground(FOREGROUND_NOTIFICATION_ID, ongoing)
            }
        }.isSuccess

        if (!startedForeground) {
            _connectionStatus.value = PushConnectionStatus.DISCONNECTED
            stopSelf()
            return
        }

        registerNetworkObserver()
    }

    private fun registerNetworkObserver() {
        val cm = getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager ?: return
        connectivityManager = cm
        currentNetworkHandle = runCatching { cm.activeNetwork?.networkHandle ?: -1L }.getOrDefault(-1L)

        val callback = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                val handle = runCatching { network.networkHandle }.getOrDefault(-1L)
                val switchedOrRecovered = networkWasLost ||
                    (currentNetworkHandle != -1L && handle != currentNetworkHandle)
                currentNetworkHandle = handle
                networkWasLost = false
                if (switchedOrRecovered || _connectionStatus.value != PushConnectionStatus.CONNECTED) {
                    requestImmediateReconnect(cancelActiveStream = switchedOrRecovered)
                }
            }

            override fun onLost(network: Network) {
                val handle = runCatching { network.networkHandle }.getOrDefault(-1L)
                if (currentNetworkHandle == -1L || handle == currentNetworkHandle) {
                    networkWasLost = true
                    _connectionStatus.value = PushConnectionStatus.DISCONNECTED
                    activeSseJob?.cancel()
                }
            }

            override fun onCapabilitiesChanged(network: Network, capabilities: NetworkCapabilities) {
                if (!capabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)) {
                    return
                }
                val handle = runCatching { network.networkHandle }.getOrDefault(-1L)
                val switchedNetwork = currentNetworkHandle != -1L && handle != currentNetworkHandle
                if (switchedNetwork || networkWasLost) {
                    currentNetworkHandle = handle
                    networkWasLost = false
                    requestImmediateReconnect(cancelActiveStream = true)
                } else if (_connectionStatus.value == PushConnectionStatus.DISCONNECTED) {
                    requestImmediateReconnect(cancelActiveStream = false)
                }
            }
        }

        val registered = runCatching {
            cm.registerDefaultNetworkCallback(callback)
            true
        }.getOrDefault(false)
        if (registered) {
            networkCallback = callback
        }
    }

    private fun requestImmediateReconnect(cancelActiveStream: Boolean) {
        immediateReconnectRequested = true
        if (cancelActiveStream) {
            activeSseJob?.cancel()
        }
        reconnectSignal.tryEmit(Unit)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_FORCE_RECONNECT) {
            requestImmediateReconnect(cancelActiveStream = true)
        }
        if (streamJob?.isActive != true) {
            streamJob = serviceScope.launch {
                val prefs = BotHubPrefs(applicationContext)
                val api = BotHubApiClient(prefs)
                var backoffMs = INITIAL_BACKOFF_MS

                while (isActive) {
                    if (prefs.bearerToken.isEmpty() || !prefs.realtimeServiceEnabled) {
                        _connectionStatus.value = PushConnectionStatus.DISCONNECTED
                        stopSelf()
                        break
                    }

                    _connectionStatus.value = PushConnectionStatus.CONNECTING
                    var connectedThisAttempt = false

                    val sseJob = serviceScope.launch {
                        runCatching {
                            connectSSE(prefs, api) {
                                connectedThisAttempt = true
                                backoffMs = INITIAL_BACKOFF_MS
                            }
                        }
                    }
                    activeSseJob = sseJob
                    sseJob.join()
                    activeSseJob = null

                    if (!isActive) break
                    _connectionStatus.value = PushConnectionStatus.DISCONNECTED

                    if (prefs.bearerToken.isEmpty() || !prefs.realtimeServiceEnabled) {
                        stopSelf()
                        break
                    }

                    if (immediateReconnectRequested) {
                        immediateReconnectRequested = false
                        backoffMs = INITIAL_BACKOFF_MS
                        continue
                    }

                    val waitMs = if (connectedThisAttempt) INITIAL_BACKOFF_MS else backoffMs
                    if (!connectedThisAttempt) {
                        backoffMs = (backoffMs * 2L).coerceAtMost(MAX_BACKOFF_MS)
                    }
                    withTimeoutOrNull(waitMs) {
                        reconnectSignal.first()
                    }
                    immediateReconnectRequested = false
                }
            }
        }
        return START_STICKY
    }

    private suspend fun connectSSE(
        prefs: BotHubPrefs,
        api: BotHubApiClient,
        onConnected: () -> Unit
    ) {
        api.openNotificationEventStream(prefs.lastEventId) { event, obj ->
            val nowMs = System.currentTimeMillis()
            when (event) {
                "connected" -> {
                    onConnected()
                    _connectionStatus.value = PushConnectionStatus.CONNECTED
                    _lastHeartbeatAtMs.value = nowMs
                    val serverLatest = obj.optLong("latest_id", 0L)
                    val unreadCount = obj.optInt("unread_count", 0)
                    if (prefs.lastEventId == 0L && serverLatest > 0L) {
                        if (unreadCount == 0) {
                            prefs.advanceLastEventId(serverLatest)
                        } else {
                            serviceScope.launch {
                                catchUpInitialUnreadNotifications(prefs, api, serverLatest)
                            }
                        }
                    }
                    _liveEvents.tryEmit(event to obj)
                }

                "heartbeat" -> {
                    _connectionStatus.value = PushConnectionStatus.CONNECTED
                    _lastHeartbeatAtMs.value = nowMs
                }

                "notification" -> {
                    _lastHeartbeatAtMs.value = nowMs
                    val item = JsonMappers.parseNotification(obj)
                    prefs.advanceLastEventId(item.id)
                    NotificationHelper.showNotification(applicationContext, item)
                    _liveEvents.tryEmit(event to obj)
                }

                "bot_message" -> {
                    _lastHeartbeatAtMs.value = nowMs
                    _liveEvents.tryEmit(event to obj)
                    handleIncomingBotMessageNotification(prefs, obj)
                }

                "bot_state", "stash_update", "assistant_update", "thread_updated" -> {
                    _lastHeartbeatAtMs.value = nowMs
                    _liveEvents.tryEmit(event to obj)
                }
            }
        }
    }

    private suspend fun catchUpInitialUnreadNotifications(
        prefs: BotHubPrefs,
        api: BotHubApiClient,
        serverLatest: Long
    ) {
        api.fetchNotifications(afterId = 0L, limit = 5).onSuccess { items ->
            val recentUnread = items
                .filter { it.isUnread && isCreatedWithinMinutes(it.createdAt, 10L) }
                .sortedBy { it.id }
            for (item in recentUnread) {
                NotificationHelper.showNotification(applicationContext, item)
            }
            val maxId = maxOf(serverLatest, items.maxOfOrNull { it.id } ?: 0L)
            prefs.advanceLastEventId(maxId)
        }.onFailure {
            prefs.advanceLastEventId(serverLatest)
        }
    }

    private fun isCreatedWithinMinutes(rawCreatedAt: String, windowMinutes: Long): Boolean {
        val clean = rawCreatedAt.trim()
        if (clean.isEmpty()) return false
        return runCatching {
            val parsed = LocalDateTime.parse(clean.take(19), SHANGHAI_TIME_FORMAT)
            val createdMs = parsed.atZone(SHANGHAI_ZONE).toInstant().toEpochMilli()
            val deltaMs = System.currentTimeMillis() - createdMs
            deltaMs in -60_000L..(windowMinutes * 60_000L)
        }.getOrDefault(false)
    }

    private fun handleIncomingBotMessageNotification(prefs: BotHubPrefs, obj: JSONObject) {
        val role = obj.optString("role", "")
        if (role != "assistant") return

        val msgId = obj.optLong("id", 0L)
        if (msgId <= 0L || msgId <= prefs.lastNotifiedBotMsgId) return

        val botId = obj.optString("bot_id", "").trim()
        val content = obj.optString("content", "")
        if (botId.isEmpty() || content.isBlank()) return

        val msgKind = obj.optString("msg_kind", "chat").trim()
        val isRoutine = msgKind.equals("routine", ignoreCase = true)
        val viewingThisBot = isAppInForeground && activeViewingBotId == botId
        val shouldNotify = if (isRoutine) {
            !viewingThisBot
        } else {
            !isAppInForeground
        }

        if (shouldNotify) {
            val botName = prefs.loadCachedBots()
                .find { it.id == botId }
                ?.name
                ?.takeIf { it.isNotBlank() }
                ?: "BotHub 助手"
            NotificationHelper.showBotMessageNotification(
                context = applicationContext,
                botId = botId,
                botName = botName,
                messageId = msgId,
                content = content,
                isRoutine = isRoutine
            )
        }
        prefs.advanceLastNotifiedBotMsgId(msgId)
    }

    override fun onTaskRemoved(rootIntent: Intent?) {
        super.onTaskRemoved(rootIntent)
        val prefs = BotHubPrefs(applicationContext)
        if (prefs.realtimeServiceEnabled && prefs.bearerToken.isNotEmpty()) {
            runCatching {
                val restartIntent = Intent(applicationContext, RealtimePushService::class.java)
                val pendingIntent = PendingIntent.getForegroundService(
                    applicationContext,
                    RESTART_REQUEST_CODE,
                    restartIntent,
                    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
                )
                val alarmManager = getSystemService(Context.ALARM_SERVICE) as? AlarmManager
                val triggerAt = SystemClock.elapsedRealtime() + 1500L
                alarmManager?.setAndAllowWhileIdle(
                    AlarmManager.ELAPSED_REALTIME_WAKEUP,
                    triggerAt,
                    pendingIntent
                )
            }
            PushSyncWorker.enqueueImmediateSync(applicationContext)
        }
    }

    override fun onDestroy() {
        networkCallback?.let { cb ->
            runCatching { connectivityManager?.unregisterNetworkCallback(cb) }
        }
        networkCallback = null
        connectivityManager = null
        activeSseJob?.cancel()
        streamJob?.cancel()
        _connectionStatus.value = PushConnectionStatus.DISCONNECTED
        serviceScope.cancel()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    companion object {
        private const val FOREGROUND_NOTIFICATION_ID = 9001
        private const val RESTART_REQUEST_CODE = 9002
        private const val INITIAL_BACKOFF_MS = 2000L
        private const val MAX_BACKOFF_MS = 15_000L
        const val ACTION_FORCE_RECONNECT = "uk.starryheavens.vesper.bothub.ACTION_FORCE_RECONNECT"

        private val SHANGHAI_ZONE: ZoneId = ZoneId.of("Asia/Shanghai")
        private val SHANGHAI_TIME_FORMAT: DateTimeFormatter =
            DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss")

        @Volatile
        var isAppInForeground: Boolean = false

        @Volatile
        var activeViewingBotId: String = ""

        private val _connectionStatus = MutableStateFlow(PushConnectionStatus.DISCONNECTED)
        val connectionStatus: StateFlow<PushConnectionStatus> = _connectionStatus.asStateFlow()

        private val _lastHeartbeatAtMs = MutableStateFlow(0L)
        val lastHeartbeatAtMs: StateFlow<Long> = _lastHeartbeatAtMs.asStateFlow()

        private val _liveEvents = MutableSharedFlow<Pair<String, JSONObject>>(extraBufferCapacity = 32)
        val liveEvents: SharedFlow<Pair<String, JSONObject>> = _liveEvents.asSharedFlow()

        fun start(context: Context) {
            runCatching {
                val appContext = context.applicationContext
                appContext.startForegroundService(Intent(appContext, RealtimePushService::class.java))
            }
        }

        fun forceReconnect(context: Context) {
            runCatching {
                val appContext = context.applicationContext
                val intent = Intent(appContext, RealtimePushService::class.java).apply {
                    action = ACTION_FORCE_RECONNECT
                }
                appContext.startForegroundService(intent)
            }
        }

        fun stop(context: Context) {
            _connectionStatus.value = PushConnectionStatus.DISCONNECTED
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
