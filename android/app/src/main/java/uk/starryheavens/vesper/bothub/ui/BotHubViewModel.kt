package uk.starryheavens.vesper.bothub.ui

import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.Call
import uk.starryheavens.vesper.bothub.data.AgentSessionSummary
import uk.starryheavens.vesper.bothub.data.AssetCategoryItem
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.ChatTurnUi
import uk.starryheavens.vesper.bothub.data.DashboardSnapshot
import uk.starryheavens.vesper.bothub.data.JsonMappers
import uk.starryheavens.vesper.bothub.data.ModelConfigPayload
import uk.starryheavens.vesper.bothub.data.ModelHealthItem
import uk.starryheavens.vesper.bothub.data.ModelOptionItem
import uk.starryheavens.vesper.bothub.data.ModelProviderItem
import uk.starryheavens.vesper.bothub.data.ModelUsageSummaryItem
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.data.PoolStrategiesItem
import uk.starryheavens.vesper.bothub.data.PooledModelItem
import uk.starryheavens.vesper.bothub.data.StashItemUi
import uk.starryheavens.vesper.bothub.data.TaskItem
import uk.starryheavens.vesper.bothub.navigation.BotHubTab
import uk.starryheavens.vesper.bothub.sync.RealtimePushService
import java.util.concurrent.atomic.AtomicLong

class BotHubViewModel(
    private val appContext: Context,
    private val prefs: BotHubPrefs,
    private val api: BotHubApiClient
) : ViewModel() {

    private val _uiEvents = MutableSharedFlow<String>(extraBufferCapacity = 16)
    val uiEvents: SharedFlow<String> = _uiEvents.asSharedFlow()

    var selectedTab by mutableStateOf(BotHubTab.AGENT)
        private set

    var isPaired by mutableStateOf(prefs.isLikelyPaired)
        private set

    var showSettingsSheet by mutableStateOf(!prefs.isLikelyPaired)
        private set

    var showModelConfigSheet by mutableStateOf(false)
        private set

    var biometricUnlocked by mutableStateOf(!prefs.biometricAppLockEnabled || !prefs.isLikelyPaired)
        private set

    var serverUrl by mutableStateOf(prefs.serverUrl)
        private set

    var realtimeServiceEnabled by mutableStateOf(prefs.realtimeServiceEnabled)
        private set

    var biometricAppLockEnabled by mutableStateOf(prefs.biometricAppLockEnabled)
        private set

    var zeroClickCaptureEnabled by mutableStateOf(prefs.zeroClickCaptureEnabled)
        private set

    var appThemeId by mutableStateOf(prefs.appThemeId)
        private set

    var bots by mutableStateOf(prefs.loadCachedBots())
        private set

    var activeBotId by mutableStateOf(prefs.lastActiveBotId)
        private set

    var draftsByBot by mutableStateOf(prefs.loadBotDrafts())
        private set

    var inChatRoom by mutableStateOf(false)
        private set

    var showServiceFeed by mutableStateOf(false)
        private set

    var imSearchQuery by mutableStateOf("")
        private set

    var inspectingBotProfile by mutableStateOf<BotItem?>(null)
        private set

    var showCreateFriendSheet by mutableStateOf(false)
        private set

    var createGroupSourceBotId by mutableStateOf<String?>(null)
        private set

    var managingGroupBot by mutableStateOf<BotItem?>(null)
        private set

    var sessionId by mutableStateOf("")
        private set

    var activePersona by mutableStateOf(prefs.activePersona)
        private set

    var activeModel by mutableStateOf(prefs.activeModel)
        private set

    var enableThinking by mutableStateOf(prefs.enableThinking)
        private set

    var agentSessions by mutableStateOf<List<AgentSessionSummary>>(emptyList())
        private set

    var showSessionsSheet by mutableStateOf(false)
        private set

    var inputText by mutableStateOf("")
        private set

    var isStreaming by mutableStateOf(false)
        private set

    private val chatMessages = mutableStateListOf<ChatTurnUi>()
    val messages: List<ChatTurnUi>
        get() = chatMessages

    var notifications by mutableStateOf(prefs.loadCachedNotifications())
        private set

    var selectedNotification by mutableStateOf<NotificationItem?>(null)
        private set

    var stashItems by mutableStateOf(prefs.loadCachedStash())
        private set

    var stashQuery by mutableStateOf("")
        private set

    var stashFilter by mutableStateOf("all")
        private set

    var stashLoading by mutableStateOf(false)
        private set

    var dashboard by mutableStateOf<DashboardSnapshot?>(prefs.loadCachedDashboard())
        private set

    var modelOptions by mutableStateOf(prefs.loadCachedDashboard()?.modelOptions ?: emptyList())
        private set

    var modelProviders by mutableStateOf<List<ModelProviderItem>>(emptyList())
        private set

    var pooledModels by mutableStateOf<List<PooledModelItem>>(emptyList())
        private set

    var poolStrategies by mutableStateOf(PoolStrategiesItem())
        private set

    var modelHealth by mutableStateOf<Map<String, ModelHealthItem>>(emptyMap())
        private set

    var modelUsageSummary by mutableStateOf(ModelUsageSummaryItem())
        private set

    var isRefreshing by mutableStateOf(false)
        private set

    private val timelineCache = mutableMapOf<String, List<ChatTurnUi>>()
    private var lastRevisions: Map<String, Long> = prefs.loadCachedDashboard()?.revisions ?: emptyMap()
    private var lastFullRefreshAtMs: Long = 0L
    private var lastForegroundSyncMs: Long = 0L
    private var pendingNotificationId: Long? = null
    private var stashSearchJob: Job? = null
    private var liveSyncDebounceJob: Job? = null
    private var activeChatJob: Job? = null

    @Volatile
    private var activeStreamCall: Call? = null
    private val turnSequence = AtomicLong(0L)

    init {
        val initialTurns = prefs.loadCachedBotMessages(activeBotId)
        if (initialTurns.isNotEmpty()) {
            timelineCache[activeBotId] = initialTurns
            chatMessages.addAll(initialTurns)
        }
        inputText = draftsByBot[activeBotId].orEmpty()

        viewModelScope.launch {
            val token = withContext(Dispatchers.IO) { prefs.warmUpCredentials() }
            val actuallyPaired = token.isNotEmpty()
            isPaired = actuallyPaired
            if (!actuallyPaired) {
                showSettingsSheet = true
                biometricUnlocked = true
            } else {
                showSettingsSheet = false
                refreshAll(silent = bots.isNotEmpty())
            }
        }

        viewModelScope.launch {
            RealtimePushService.liveEvents.collect { (event, obj) ->
                when (event) {
                    "connected" -> {
                        val revs = JsonMappers.parseRevisions(obj.optJSONObject("revisions"))
                        if (revs.isEmpty() || lastRevisions.isEmpty()) {
                            scheduleSilentLiveSync()
                        } else {
                            val changed = mutableListOf<String>()
                            listOf("bots", "tasks", "assets", "stash", "models", "notifications", "configs").forEach { k ->
                                if ((revs[k] ?: 0L) > (lastRevisions[k] ?: 0L)) {
                                    changed.add(k)
                                }
                            }
                            val msgBumped = (revs["messages"] ?: 0L) > (lastRevisions["messages"] ?: 0L)
                            lastRevisions = lastRevisions + revs
                            if (changed.isNotEmpty()) {
                                syncDashboardSlices(changed)
                            }
                            if (msgBumped && activeBotId.isNotBlank() && !isStreaming && sessionId.isEmpty()) {
                                loadBotTimeline(activeBotId)
                            }
                        }
                    }
                    "notification" -> {
                        val item = JsonMappers.parseNotification(obj)
                        if (notifications.none { it.id == item.id }) {
                            val updated = (listOf(item) + notifications).take(60)
                            notifications = updated
                            prefs.saveCachedNotifications(updated)
                        }
                        scheduleSilentLiveSync()
                    }
                    "bot_state" -> {
                        val bId = obj.optString("bot_id", "")
                        val st = obj.optString("state", "")
                        val summary = obj.optString("last_summary", "")
                        val hasUnread = obj.has("unread_count")
                        val unreadVal = obj.optInt("unread_count", 0)
                        if (bId.isNotEmpty()) {
                            bots = bots.map {
                                if (it.id == bId) {
                                    it.copy(
                                        state = st.ifEmpty { it.state },
                                        lastSummary = summary.ifEmpty { it.lastSummary },
                                        unreadCount = if (hasUnread) unreadVal else it.unreadCount
                                    )
                                } else it
                            }
                        }
                    }
                    "thread_updated" -> {
                        val bId = obj.optString("bot_id", "")
                        syncDashboardSlices(listOf("bots"))
                        if (bId == activeBotId && !isStreaming && sessionId.isEmpty()) {
                            loadBotTimeline(bId)
                        }
                    }
                    "bot_message" -> {
                        scheduleSilentLiveSync()
                    }
                    "stash_update" -> {
                        syncDashboardSlices(listOf("stash"))
                    }
                    "assistant_update" -> {
                        val rawKinds = obj.optString("kind", "")
                        val kinds = rawKinds.split(",").map { it.trim() }.filter { it.isNotEmpty() }
                        if (kinds.any { it == "backup_restored" || it == "dashboard" } || kinds.isEmpty()) {
                            scheduleSilentLiveSync()
                        } else {
                            syncDashboardSlices(kinds)
                        }
                    }
                }
            }
        }
    }

    fun onAppForegrounded() {
        if (!prefs.isLikelyPaired || isStreaming) return
        val now = System.currentTimeMillis()
        if (now - lastForegroundSyncMs < 4000L) return
        lastForegroundSyncMs = now
        refreshAll(silent = true)
        if (inChatRoom && sessionId.isEmpty() && activeBotId.isNotBlank()) {
            loadBotTimeline(activeBotId)
        }
    }

    private fun syncDashboardSlices(kinds: List<String>) {
        if (!prefs.isLikelyPaired || kinds.isEmpty()) return
        viewModelScope.launch {
            if (prefs.bearerToken.isEmpty()) return@launch
            api.fetchDashboardSlice(kinds).onSuccess { slice ->
                slice.bots?.let { nextBots ->
                    bots = nextBots
                    prefs.saveCachedBots(nextBots)
                }
                slice.notifications?.let { nextNotifs ->
                    notifications = nextNotifs
                    prefs.saveCachedNotifications(nextNotifs)
                    val maxId = nextNotifs.maxOfOrNull { it.id } ?: 0L
                    prefs.advanceLastEventId(maxId)
                }
                slice.stashItems?.let { nextStash ->
                    if (stashQuery.isBlank() && stashFilter == "all") {
                        stashItems = nextStash
                        prefs.saveCachedStash(nextStash)
                    }
                }
                slice.modelOptions?.let { nextOpts ->
                    modelOptions = nextOpts
                }
                slice.modelProviders?.let { modelProviders = it }
                slice.pooledModels?.let { pooledModels = it }
                slice.poolStrategies?.let { poolStrategies = it }
                slice.modelHealth?.let { modelHealth = it }
                slice.modelUsageSummary?.let { modelUsageSummary = it }
                val curDash = dashboard
                if (curDash != null && (slice.assets != null || slice.tasks != null || slice.assetCategories != null || slice.modelOptions != null)) {
                    val nextDash = curDash.copy(
                        assets = slice.assets ?: curDash.assets,
                        tasks = slice.tasks ?: curDash.tasks,
                        assetCategories = slice.assetCategories ?: curDash.assetCategories,
                        modelOptions = slice.modelOptions ?: curDash.modelOptions,
                        totalAssets = slice.assets?.size ?: curDash.totalAssets,
                        expiringCount = (slice.assets ?: curDash.assets).count { it.isExpiringSoon },
                        activeTasks = (slice.tasks ?: curDash.tasks).count { it.enabled },
                        revisions = slice.revisions.ifEmpty { curDash.revisions }
                    )
                    dashboard = nextDash
                    prefs.saveCachedDashboard(nextDash)
                }
                if (slice.revisions.isNotEmpty()) {
                    lastRevisions = lastRevisions + slice.revisions
                }
            }
        }
    }

    private fun scheduleSilentLiveSync() {
        liveSyncDebounceJob?.cancel()
        liveSyncDebounceJob = viewModelScope.launch {
            delay(380L)
            refreshAll(silent = true)
            if (inChatRoom && !isStreaming && sessionId.isEmpty() && activeBotId.isNotBlank()) {
                loadBotTimeline(activeBotId)
            }
        }
    }

    fun openModelConfig() {
        showSettingsSheet = false
        showModelConfigSheet = true
        refreshModelConfig()
    }

    fun dismissModelConfig() {
        showModelConfigSheet = false
    }

    fun applyModelConfigPayload(payload: ModelConfigPayload) {
        modelProviders = payload.providers
        pooledModels = payload.models
        poolStrategies = payload.strategies
        if (payload.modelOptions.isNotEmpty()) {
            modelOptions = payload.modelOptions
        }
        modelHealth = payload.modelHealth
        modelUsageSummary = payload.usageSummary
    }

    fun refreshModelConfig() {
        if (!prefs.isLikelyPaired || prefs.bearerToken.isEmpty()) return
        viewModelScope.launch {
            api.fetchModelConfig().onSuccess { payload ->
                applyModelConfigPayload(payload)
            }
        }
    }

    fun updateModelHealth(next: Map<String, ModelHealthItem>, nextModels: List<PooledModelItem>? = null, nextOptions: List<ModelOptionItem>? = null) {
        modelHealth = next
        if (nextModels != null) pooledModels = nextModels
        if (nextOptions != null) modelOptions = nextOptions
    }

    fun showMessage(message: String) {
        if (message.isNotBlank()) {
            _uiEvents.tryEmit(message)
        }
    }

    fun selectTab(tab: BotHubTab, triggerRefresh: Boolean = false) {
        inChatRoom = false
        showServiceFeed = false
        selectedTab = tab
        if (triggerRefresh) {
            val now = System.currentTimeMillis()
            if (now - lastFullRefreshAtMs > 15_000L) {
                when (tab) {
                    BotHubTab.AGENT, BotHubTab.INBOX, BotHubTab.HUB -> refreshAll(silent = true)
                    BotHubTab.STASH -> {
                        if (stashItems.isEmpty() || stashQuery.isNotBlank() || stashFilter != "all") {
                            refreshStash(silent = stashItems.isNotEmpty())
                        } else {
                            refreshAll(silent = true)
                        }
                    }
                }
            }
        }
    }

    fun updateImSearchQuery(q: String) {
        imSearchQuery = q
    }

    private fun persistBotDraft(botId: String, text: String) {
        if (botId.isBlank()) return
        val trimmed = text
        val current = draftsByBot[botId].orEmpty()
        if (current == trimmed) return
        val next = draftsByBot.toMutableMap()
        if (trimmed.isBlank()) {
            next.remove(botId)
        } else {
            next[botId] = trimmed
        }
        draftsByBot = next
        prefs.saveBotDrafts(next)
    }

    fun openChatRoom(botId: String) {
        inspectingBotProfile = null
        showServiceFeed = false
        if (botId.isNotBlank()) {
            val changedBot = activeBotId != botId
            if (changedBot && sessionId.isEmpty() && activeBotId.isNotBlank()) {
                persistBotDraft(activeBotId, inputText)
            }
            activeBotId = botId
            prefs.lastActiveBotId = botId
            bots.find { it.id == botId }?.let { cachedBot ->
                if (cachedBot.model.isNotBlank()) {
                    activeModel = cachedBot.model
                }
                enableThinking = cachedBot.enableThinking
            }
            val nextBots = bots.map { if (it.id == botId && it.unreadCount != 0) it.copy(unreadCount = 0) else it }
            if (nextBots != bots) {
                bots = nextBots
                prefs.saveCachedBots(nextBots)
            }

            if (changedBot || chatMessages.isEmpty()) {
                sessionId = ""
                inputText = draftsByBot[botId].orEmpty()
                val cachedTurns = timelineCache[botId] ?: prefs.loadCachedBotMessages(botId).also { loaded ->
                    if (loaded.isNotEmpty()) timelineCache[botId] = loaded
                }
                chatMessages.clear()
                if (cachedTurns.isNotEmpty()) {
                    chatMessages.addAll(cachedTurns)
                }
                loadBotTimeline(botId)
            }
        }
        inChatRoom = true
    }

    fun closeChatRoom() {
        if (sessionId.isEmpty() && activeBotId.isNotBlank()) {
            persistBotDraft(activeBotId, inputText)
        }
        inChatRoom = false
        if (prefs.isLikelyPaired) {
            viewModelScope.launch {
                api.fetchBots().onSuccess { list ->
                    bots = list
                    prefs.saveCachedBots(list)
                }
            }
        }
    }

    fun openServiceFeed() {
        inChatRoom = false
        showServiceFeed = true
        if (System.currentTimeMillis() - lastFullRefreshAtMs > 10_000L) {
            refreshAll(silent = notifications.isNotEmpty())
        }
    }

    fun closeServiceFeed() {
        showServiceFeed = false
    }

    fun openBotProfile(bot: BotItem) {
        inspectingBotProfile = bot
    }

    fun dismissBotProfile() {
        inspectingBotProfile = null
    }

    fun openCreateFriend() {
        showCreateFriendSheet = true
    }

    fun dismissCreateFriend() {
        showCreateFriendSheet = false
    }

    fun openCreateGroup(sourceBotId: String = "") {
        inspectingBotProfile = null
        createGroupSourceBotId = sourceBotId
    }

    fun dismissCreateGroup() {
        createGroupSourceBotId = null
    }

    fun openManageGroupMembers(groupBot: BotItem) {
        inspectingBotProfile = null
        managingGroupBot = groupBot
    }

    fun dismissManageGroupMembers() {
        managingGroupBot = null
    }

    fun saveGroupMembers(groupId: String, memberIds: List<String>) {
        if (groupId.isBlank() || memberIds.isEmpty()) return
        val optimistic = bots.map {
            if (it.id == groupId) it.copy(memberIds = memberIds) else it
        }
        bots = optimistic
        prefs.saveCachedBots(optimistic)
        managingGroupBot = null
        viewModelScope.launch {
            api.updateGroupMembers(groupId, memberIds).onSuccess { updated ->
                val synced = bots.map { if (it.id == groupId) updated else it }
                bots = synced
                prefs.saveCachedBots(synced)
                showMessage("已更新群成员 (${updated.memberIds.size})")
                if (activeBotId == groupId) {
                    loadBotTimeline(groupId)
                }
            }.onFailure { err ->
                showMessage(err.message ?: "更新群成员失败")
                refreshAll(silent = true)
            }
        }
    }

    fun openSettings() {
        showSettingsSheet = true
    }

    fun dismissSettings() {
        showSettingsSheet = false
    }

    fun markBiometricUnlocked() {
        biometricUnlocked = true
    }

    suspend fun verifyFallbackPassword(password: String): Result<String> {
        return api.exchangeToken(prefs.serverUrl, password).onSuccess {
            serverUrl = prefs.serverUrl
            biometricUnlocked = true
        }
    }

    fun onConnectedSuccessfully(toastMessage: String) {
        serverUrl = prefs.serverUrl
        isPaired = true
        biometricUnlocked = true
        showSettingsSheet = false
        RealtimePushService.syncWithPrefs(appContext, prefs)
        refreshAll()
        showMessage(toastMessage)
    }

    fun updateServerUrl(newUrl: String) {
        prefs.serverUrl = newUrl
        serverUrl = prefs.serverUrl
    }

    fun setBiometricAppLock(enabled: Boolean) {
        prefs.biometricAppLockEnabled = enabled
        biometricAppLockEnabled = enabled
        if (enabled) {
            biometricUnlocked = true
        }
    }

    fun setRealtimeService(enabled: Boolean) {
        prefs.realtimeServiceEnabled = enabled
        realtimeServiceEnabled = enabled
        RealtimePushService.syncWithPrefs(appContext, prefs)
    }

    fun setZeroClickCapture(enabled: Boolean) {
        prefs.zeroClickCaptureEnabled = enabled
        zeroClickCaptureEnabled = enabled
    }

    fun setAppTheme(themeId: String) {
        val clean = themeId.trim().ifEmpty { "obsidian" }
        prefs.appThemeId = clean
        appThemeId = clean
    }

    fun testConnectivity(urlOverride: String) {
        viewModelScope.launch {
            api.testHealth(urlOverride).onSuccess { (ms, ver) ->
                showMessage("连通正常 (${ms}ms · $ver)")
            }.onFailure { err ->
                showMessage("连通测试失败: ${err.message}")
            }
        }
    }

    fun unbindServer() {
        stopAgentStream()
        prefs.clearCredentials()
        isPaired = false
        RealtimePushService.syncWithPrefs(appContext, prefs)
    }

    fun handlePairPayload(targetUrl: String, pairCode: String) {
        viewModelScope.launch {
            api.exchangePairCode(targetUrl, pairCode).onSuccess {
                onConnectedSuccessfully("已免密绑定成功，手机锁屏实时通知已自动开启")
            }.onFailure { err ->
                showMessage(err.message ?: "免密配对失败")
            }
        }
    }

    fun handleSharedText(rawText: String) {
        if (rawText.isBlank()) return
        selectedTab = BotHubTab.AGENT
        showServiceFeed = false
        inChatRoom = true
        sendAgentPrompt("请抓取并深度总结以下分享内容或网页链接的核心要点与技术价值：\n$rawText")
    }

    fun handleOpenNotificationFromTray(notificationId: Long) {
        if (notificationId <= 0L) return
        inChatRoom = false
        selectedTab = BotHubTab.AGENT
        showServiceFeed = true
        val existing = notifications.find { it.id == notificationId }
        if (existing != null) {
            pendingNotificationId = null
            selectNotification(existing)
        } else {
            pendingNotificationId = notificationId
            refreshAll()
        }
    }

    private fun resolvePendingNotificationIfPresent(list: List<NotificationItem>) {
        val targetId = pendingNotificationId ?: return
        val match = list.find { it.id == targetId } ?: return
        pendingNotificationId = null
        selectedNotification = match
        if (match.isUnread) {
            viewModelScope.launch {
                api.markNotificationRead(match.id)
                api.fetchNotifications(afterId = 0L, limit = 50).onSuccess { updated ->
                    notifications = updated
                    prefs.saveCachedNotifications(updated)
                }
            }
        }
    }

    fun refreshAll(silent: Boolean = false) {
        if (!prefs.isLikelyPaired) return
        viewModelScope.launch {
            if (prefs.bearerToken.isEmpty()) return@launch
            if (!silent) {
                isRefreshing = true
            }
            try {
                val bootRes = api.fetchBootstrap()
                if (bootRes.isSuccess) {
                    val payload = bootRes.getOrThrow()
                    lastFullRefreshAtMs = System.currentTimeMillis()

                    notifications = payload.notifications
                    prefs.saveCachedNotifications(payload.notifications)
                    val maxId = payload.notifications.maxOfOrNull { it.id } ?: 0L
                    prefs.advanceLastEventId(maxId)
                    resolvePendingNotificationIfPresent(payload.notifications)

                    dashboard = payload.dashboard
                    prefs.saveCachedDashboard(payload.dashboard)
                    modelOptions = payload.modelOptions.ifEmpty { payload.dashboard.modelOptions }
                    modelProviders = payload.modelProviders
                    pooledModels = payload.pooledModels
                    poolStrategies = payload.poolStrategies
                    modelHealth = payload.modelHealth
                    modelUsageSummary = payload.modelUsageSummary
                    if (payload.revisions.isNotEmpty()) {
                        lastRevisions = lastRevisions + payload.revisions
                    }

                    val firstLoad = bots.isEmpty()
                    bots = payload.bots
                    prefs.saveCachedBots(payload.bots)
                    if (firstLoad && payload.bots.isNotEmpty()) {
                        val chosen = payload.bots.find { it.id == activeBotId } ?: payload.bots.first()
                        activeBotId = chosen.id
                        prefs.lastActiveBotId = chosen.id
                    }

                    if (stashQuery.isBlank() && stashFilter == "all") {
                        stashItems = payload.stashItems
                        prefs.saveCachedStash(payload.stashItems)
                    } else if (selectedTab == BotHubTab.STASH) {
                        api.fetchStashItems(query = stashQuery, filter = stashFilter, limit = 80).onSuccess { items ->
                            stashItems = items
                        }
                    }
                } else {
                    coroutineScope {
                        launch {
                            api.fetchNotifications(afterId = 0L, limit = 50).onSuccess { list ->
                                notifications = list
                                prefs.saveCachedNotifications(list)
                                val maxId = list.maxOfOrNull { it.id } ?: 0L
                                prefs.advanceLastEventId(maxId)
                                resolvePendingNotificationIfPresent(list)
                            }
                        }
                        launch {
                            api.fetchDashboard().onSuccess { snap ->
                                dashboard = snap
                                prefs.saveCachedDashboard(snap)
                            }
                        }
                        launch {
                            api.fetchBots().onSuccess { list ->
                                bots = list
                                prefs.saveCachedBots(list)
                            }
                        }
                        launch {
                            api.fetchStashItems(query = stashQuery, filter = stashFilter, limit = 80).onSuccess { items ->
                                stashItems = items
                                if (stashQuery.isBlank() && stashFilter == "all") {
                                    prefs.saveCachedStash(items)
                                }
                            }
                        }
                    }
                    lastFullRefreshAtMs = System.currentTimeMillis()
                }
            } finally {
                if (!silent) {
                    isRefreshing = false
                }
            }
        }
    }

    fun refreshStash(
        queryOverride: String = stashQuery,
        filterOverride: String = stashFilter,
        silent: Boolean = false
    ) {
        if (!prefs.isLikelyPaired) return
        viewModelScope.launch {
            if (prefs.bearerToken.isEmpty()) return@launch
            if (!silent) {
                stashLoading = true
            }
            try {
                api.fetchStashItems(query = queryOverride, filter = filterOverride, limit = 80).onSuccess { items ->
                    stashItems = items
                    if (queryOverride.isBlank() && filterOverride == "all") {
                        prefs.saveCachedStash(items)
                    }
                }
            } finally {
                if (!silent) {
                    stashLoading = false
                }
            }
        }
    }

    fun selectBot(botId: String) {
        openChatRoom(botId)
    }

    fun togglePinBot(bot: BotItem) {
        val nextPinned = !bot.isPinned
        val optimistic = bots.map { if (it.id == bot.id) it.copy(isPinned = nextPinned) else it }
        bots = optimistic
        prefs.saveCachedBots(optimistic)
        viewModelScope.launch {
            api.updateBotPin(bot.id, nextPinned).onSuccess { updated ->
                val synced = bots.map { if (it.id == bot.id) updated else it }
                bots = synced
                prefs.saveCachedBots(synced)
                showMessage(if (nextPinned) "已置顶聊天" else "已取消置顶")
            }.onFailure { err ->
                val rolledBack = bots.map { if (it.id == bot.id) bot else it }
                bots = rolledBack
                prefs.saveCachedBots(rolledBack)
                showMessage(err.message ?: "置顶操作失败")
            }
        }
    }

    fun markBotRead(bot: BotItem) {
        if (bot.unreadCount <= 0) return
        val updated = bots.map { if (it.id == bot.id) it.copy(unreadCount = 0) else it }
        bots = updated
        prefs.saveCachedBots(updated)
        viewModelScope.launch {
            // Fetching bot messages clears server-side unread count cleanly
            api.fetchBotMessages(bot.id)
        }
    }

    fun clearBotHistoryById(bot: BotItem) {
        if (isStreaming && activeBotId == bot.id) return
        timelineCache.remove(bot.id)
        prefs.saveCachedBotMessages(bot.id, emptyList())
        if (activeBotId == bot.id && sessionId.isEmpty()) {
            chatMessages.clear()
        }
        val clearedBots = bots.map {
            if (it.id == bot.id) it.copy(lastSummary = "", unreadCount = 0) else it
        }
        bots = clearedBots
        prefs.saveCachedBots(clearedBots)
        viewModelScope.launch {
            api.clearBotMessages(bot.id).onSuccess {
                showMessage("已清空「${bot.name}」的聊天记录")
                refreshAll(silent = true)
            }.onFailure { err ->
                showMessage(err.message ?: "清空记录失败")
                refreshAll(silent = true)
            }
        }
    }

    fun clearActiveChatHistory() {
        if (isStreaming) return
        val targetId = activeBotId
        chatMessages.clear()
        if (targetId.isNotBlank() && sessionId.isEmpty()) {
            timelineCache.remove(targetId)
            prefs.saveCachedBotMessages(targetId, emptyList())
            viewModelScope.launch {
                api.clearBotMessages(targetId).onSuccess {
                    showMessage("已清空当前聊天记录")
                    refreshAll(silent = true)
                }.onFailure { err ->
                    showMessage(err.message ?: "清空记录失败")
                }
            }
        } else {
            sessionId = ""
            showMessage("已开启全新对话")
        }
    }

    fun deleteChatTurn(turn: ChatTurnUi) {
        if (isStreaming) return
        val targetBotId = activeBotId
        chatMessages.removeAll { it.id == turn.id }
        if (sessionId.isEmpty() && targetBotId.isNotBlank()) {
            val updated = chatMessages.toList()
            timelineCache[targetBotId] = updated
            prefs.saveCachedBotMessages(targetBotId, updated)
        }
        showMessage("已删除")
        if (turn.id.startsWith("botmsg_") && targetBotId.isNotBlank()) {
            val serverMsgId = turn.id.removePrefix("botmsg_").toLongOrNull() ?: 0L
            if (serverMsgId > 0L) {
                viewModelScope.launch {
                    api.deleteBotMessage(targetBotId, serverMsgId)
                }
            }
        }
    }

    fun forwardMessageToBot(
        targetBot: BotItem,
        fromName: String,
        content: String,
        note: String,
        jumpToTarget: Boolean
    ) {
        viewModelScope.launch {
            api.forwardBotMessage(
                targetBotId = targetBot.id,
                fromName = fromName,
                content = content,
                note = note
            ).onSuccess {
                timelineCache.remove(targetBot.id)
                showMessage("已转发至「${targetBot.name}」")
                if (jumpToTarget) {
                    openChatRoom(targetBot.id)
                } else {
                    refreshAll(silent = true)
                }
            }.onFailure { err ->
                showMessage(err.message ?: "转发失败")
            }
        }
    }

    fun createNewFriend(
        name: String,
        roleTitle: String,
        avatarStyle: String,
        botMode: String,
        traits: String,
        systemPrompt: String,
        model: String = "auto:free",
        allowPaidFallback: Boolean = false
    ) {
        viewModelScope.launch {
            api.createAiFriend(
                name = name,
                roleTitle = roleTitle,
                avatarStyle = avatarStyle,
                botMode = botMode,
                traits = traits,
                systemPrompt = systemPrompt,
                model = model,
                allowPaidFallback = allowPaidFallback
            ).onSuccess { created ->
                showCreateFriendSheet = false
                bots = listOf(created) + bots.filterNot { it.id == created.id }
                showMessage("已添加 AI 好友「${created.name}」")
                openChatRoom(created.id)
            }.onFailure { err ->
                showMessage(err.message ?: "添加 AI 好友失败")
            }
        }
    }

    fun createNewGroup(
        sourceBotId: String,
        memberIds: List<String>,
        name: String,
        groupMode: String,
        carryContext: Boolean
    ) {
        viewModelScope.launch {
            api.createGroupChat(
                sourceBotId = sourceBotId,
                memberIds = memberIds,
                name = name,
                groupMode = groupMode,
                carryContext = carryContext
            ).onSuccess { created ->
                createGroupSourceBotId = null
                bots = listOf(created) + bots.filterNot { it.id == created.id }
                showMessage("已发起群聊「${created.name}」")
                openChatRoom(created.id)
            }.onFailure { err ->
                showMessage(err.message ?: "发起群聊失败")
            }
        }
    }

    fun deleteFriendOrGroup(bot: BotItem) {
        if (bot.isBuiltin) {
            showMessage("内置核心好友不支持删除")
            return
        }
        viewModelScope.launch {
            api.deleteBot(bot.id).onSuccess {
                inspectingBotProfile = null
                if (activeBotId == bot.id) {
                    inChatRoom = false
                }
                bots = bots.filterNot { it.id == bot.id }
                showMessage("已移除「${bot.name}」")
            }.onFailure { err ->
                showMessage(err.message ?: "移除失败")
            }
        }
    }

    fun saveBotEdits(
        botId: String,
        name: String,
        roleTitle: String,
        avatarStyle: String,
        traits: String,
        systemPrompt: String,
        groupMode: String,
        announcement: String,
        allowPaidFallback: Boolean? = null
    ) {
        viewModelScope.launch {
            api.updateBotDetails(
                botId = botId,
                name = name,
                roleTitle = roleTitle,
                avatarStyle = avatarStyle,
                traits = traits,
                systemPrompt = systemPrompt,
                groupMode = groupMode,
                announcement = announcement,
                allowPaidFallback = allowPaidFallback
            ).onSuccess { updated ->
                bots = bots.map { if (it.id == botId) updated else it }
                if (inspectingBotProfile?.id == botId) {
                    inspectingBotProfile = updated
                }
                showMessage("已保存")
            }.onFailure { err ->
                showMessage(err.message ?: "保存失败")
            }
        }
    }

    fun loadBotTimeline(botId: String) {
        if (isStreaming || !prefs.isLikelyPaired) return
        viewModelScope.launch {
            if (prefs.bearerToken.isEmpty()) return@launch
            api.fetchBotMessages(botId).onSuccess { msgs ->
                val target = bots.find { it.id == botId }
                val mapped = msgs.map { m ->
                    m.toChatTurnUi(
                        idPrefix = "botmsg",
                        defaultPersona = target?.name ?: botId,
                        defaultModel = target?.model ?: activeModel,
                        defaultBotId = botId,
                        defaultAvatarStyle = target?.avatarStyle ?: "butler:gold",
                        bots = bots
                    )
                }
                timelineCache[botId] = mapped
                prefs.saveCachedBotMessages(botId, mapped)

                if (activeBotId == botId && sessionId.isEmpty() && !isStreaming) {
                    if (target != null) {
                        activeModel = target.model
                        enableThinking = target.enableThinking
                    }
                    if (chatMessages.toList() != mapped) {
                        chatMessages.clear()
                        chatMessages.addAll(mapped)
                    }
                }
            }
        }
    }

    fun updatePersona(persona: String) {
        activePersona = persona
        prefs.activePersona = persona
    }

    fun updateModel(model: String) {
        activeModel = model
        prefs.activeModel = model
        val targetId = activeBotId
        if (sessionId.isEmpty() && targetId.isNotBlank()) {
            val optimistic = bots.map { if (it.id == targetId) it.copy(model = model) else it }
            bots = optimistic
            prefs.saveCachedBots(optimistic)
            if (prefs.isLikelyPaired) {
                viewModelScope.launch {
                    api.updateBotPreferences(botId = targetId, model = model).onSuccess { updated ->
                        val synced = bots.map { if (it.id == targetId) updated else it }
                        bots = synced
                        prefs.saveCachedBots(synced)
                    }
                }
            }
        }
    }

    fun updateThinking(enabled: Boolean) {
        enableThinking = enabled
        prefs.enableThinking = enabled
        val targetId = activeBotId
        if (sessionId.isEmpty() && targetId.isNotBlank()) {
            val optimistic = bots.map { if (it.id == targetId) it.copy(enableThinking = enabled) else it }
            bots = optimistic
            prefs.saveCachedBots(optimistic)
            if (prefs.isLikelyPaired) {
                viewModelScope.launch {
                    api.updateBotPreferences(botId = targetId, enableThinking = enabled).onSuccess { updated ->
                        val synced = bots.map { if (it.id == targetId) updated else it }
                        bots = synced
                        prefs.saveCachedBots(synced)
                    }
                }
            }
        }
    }

    fun updateAllowPaidFallback(allow: Boolean) {
        val targetId = activeBotId
        if (sessionId.isEmpty() && targetId.isNotBlank()) {
            val optimistic = bots.map { if (it.id == targetId) it.copy(allowPaidFallback = allow) else it }
            bots = optimistic
            prefs.saveCachedBots(optimistic)
            if (prefs.isLikelyPaired) {
                viewModelScope.launch {
                    api.updateBotPreferences(botId = targetId, allowPaidFallback = allow).onSuccess { updated ->
                        val synced = bots.map { if (it.id == targetId) updated else it }
                        bots = synced
                        prefs.saveCachedBots(synced)
                    }
                }
            }
        }
    }

    fun updateInputText(text: String) {
        inputText = text
        if (sessionId.isEmpty() && activeBotId.isNotBlank()) {
            persistBotDraft(activeBotId, text)
        }
    }

    fun openSessions() {
        viewModelScope.launch {
            api.fetchAgentSessions().onSuccess { agentSessions = it }
        }
        showSessionsSheet = true
    }

    fun dismissSessions() {
        showSessionsSheet = false
    }

    fun startNewSession() {
        sessionId = ""
        chatMessages.clear()
        showSessionsSheet = false
    }

    fun loadAgentSession(sess: AgentSessionSummary) {
        if (isStreaming) return
        viewModelScope.launch {
            api.fetchAgentSessionMessages(sess.id).onSuccess { msgs ->
                sessionId = sess.id
                activePersona = sess.persona
                activeModel = sess.model
                chatMessages.clear()
                msgs.forEach { m ->
                    chatMessages.add(
                        m.toChatTurnUi(
                            idPrefix = "hist",
                            defaultPersona = sess.persona,
                            defaultModel = sess.model
                        )
                    )
                }
                showSessionsSheet = false
            }.onFailure { err ->
                showMessage("加载历史会话失败: ${err.message}")
            }
        }
    }

    private fun updateTurnById(turnId: String, transform: (ChatTurnUi) -> ChatTurnUi) {
        val idx = chatMessages.indexOfFirst { it.id == turnId }
        if (idx >= 0) {
            chatMessages[idx] = transform(chatMessages[idx])
        }
    }

    fun sendAgentPrompt(promptText: String) {
        val clean = promptText.trim()
        if (clean.isEmpty() || isStreaming) return
        if (prefs.bearerToken.isEmpty()) {
            showSettingsSheet = true
            return
        }

        val activeBotObj = bots.find { it.id == activeBotId }
        val botDisplayName = activeBotObj?.name ?: activePersona
        val botStyle = activeBotObj?.avatarStyle ?: "butler:gold"
        val seq = turnSequence.incrementAndGet()
        val now = System.currentTimeMillis()
        val userTurnId = "u_${now}_$seq"
        val assistantId = "a_${now}_$seq"

        val targetBotId = activeBotId
        inputText = ""
        if (sessionId.isEmpty() && targetBotId.isNotBlank()) {
            persistBotDraft(targetBotId, "")
        }
        isStreaming = true
        chatMessages.add(
            ChatTurnUi(
                id = userTurnId,
                role = "user",
                persona = botDisplayName,
                model = activeModel,
                content = clean
            )
        )
        chatMessages.add(
            ChatTurnUi(
                id = assistantId,
                role = "assistant",
                persona = botDisplayName,
                model = activeModel,
                content = "",
                senderBotId = activeBotId,
                avatarStyle = botStyle
            )
        )

        activeChatJob = viewModelScope.launch {
            try {
                api.streamChat(
                    sessionId = sessionId,
                    botId = activeBotId,
                    message = clean,
                    persona = activePersona,
                    model = activeModel,
                    enableThinking = enableThinking,
                    allowPaidFallback = activeBotObj?.allowPaidFallback,
                    onCallCreated = { call -> activeStreamCall = call },
                    onSession = { sid ->
                        viewModelScope.launch { sessionId = sid }
                    },
                    onReasoningDelta = { delta ->
                        viewModelScope.launch {
                            updateTurnById(assistantId) { cur ->
                                cur.copy(reasoning = cur.reasoning + delta)
                            }
                        }
                    },
                    onToolCall = { toolName, status ->
                        viewModelScope.launch {
                            updateTurnById(assistantId) { cur ->
                                val displayName = formatHumanToolName(toolName)
                                val label = if (status == "执行中") "$displayName · 进行中" else displayName
                                val updatedTools = cur.tools.filterNot {
                                    it == displayName || it.startsWith("$displayName ·")
                                } + label
                                cur.copy(tools = updatedTools)
                            }
                        }
                    },
                    onContentDelta = { delta ->
                        viewModelScope.launch {
                            updateTurnById(assistantId) { cur ->
                                cur.copy(content = cur.content + delta)
                            }
                        }
                    },
                    onError = { err ->
                        viewModelScope.launch {
                            updateTurnById(assistantId) { cur ->
                                val fallback = if (cur.content.isEmpty()) {
                                    "[请求异常] $err"
                                } else {
                                    "${cur.content}\n[中断: $err]"
                                }
                                cur.copy(content = fallback)
                            }
                        }
                    }
                )
            } finally {
                activeStreamCall = null
                isStreaming = false
                if (sessionId.isEmpty() && targetBotId.isNotBlank() && activeBotId == targetBotId) {
                    val snapshotTurns = chatMessages.toList()
                    timelineCache[targetBotId] = snapshotTurns
                    prefs.saveCachedBotMessages(targetBotId, snapshotTurns)
                }
                refreshAll(silent = true)
            }
        }
    }

    fun stopAgentStream(notifyUser: Boolean = false) {
        activeStreamCall?.cancel()
        activeChatJob?.cancel()
        activeStreamCall = null
        activeChatJob = null
        isStreaming = false
        if (notifyUser) {
            showMessage("已停止生成")
        }
    }

    fun selectNotification(item: NotificationItem) {
        selectedNotification = item
        if (item.isUnread) {
            val optimistic = notifications.map {
                if (it.id == item.id) it.copy(readAt = "read") else it
            }
            notifications = optimistic
            prefs.saveCachedNotifications(optimistic)
            viewModelScope.launch {
                api.markNotificationRead(item.id)
                refreshAll(silent = true)
            }
        }
    }

    fun dismissNotificationDetail() {
        selectedNotification = null
    }

    fun diagnoseNotificationWithAi(item: NotificationItem) {
        selectedNotification = null
        showServiceFeed = false
        selectedTab = BotHubTab.AGENT
        val butler = bots.firstOrNull { !it.isGroup } ?: bots.firstOrNull()
        if (butler != null) {
            openChatRoom(butler.id)
        } else {
            inChatRoom = true
        }
        sendAgentPrompt("请帮我分析这条通知并给出排查或处置建议：\n\n标题：${item.title}\n\n${item.markdownBody}")
    }

    fun markAllNotificationsRead() {
        val optimistic = notifications.map { if (it.isUnread) it.copy(readAt = "read") else it }
        notifications = optimistic
        prefs.saveCachedNotifications(optimistic)
        showMessage("已全部标为已读")
        viewModelScope.launch {
            api.markAllNotificationsRead()
            refreshAll(silent = true)
        }
    }

    fun clearReadNotifications() {
        val prev = notifications
        val remaining = notifications.filter { it.isUnread }
        notifications = remaining
        prefs.saveCachedNotifications(remaining)
        showMessage("已清理已读通知")
        viewModelScope.launch {
            api.clearReadNotifications().onSuccess {
                refreshAll(silent = true)
            }.onFailure { err ->
                notifications = prev
                prefs.saveCachedNotifications(prev)
                showMessage(err.message ?: "清理失败")
            }
        }
    }

    fun updateStashQuery(query: String) {
        stashQuery = query
        stashSearchJob?.cancel()
        stashSearchJob = viewModelScope.launch {
            if (query.isNotBlank()) {
                delay(240L)
            }
            refreshStash(query, stashFilter)
        }
    }

    fun updateStashFilter(filter: String) {
        stashFilter = filter
        stashSearchJob?.cancel()
        refreshStash(stashQuery, filter, silent = stashItems.isNotEmpty())
    }

    fun captureStashItem(rawInput: String, userNote: String) {
        viewModelScope.launch {
            api.createStashItem(rawInput, userNote).onSuccess { (created, dedup) ->
                if (!dedup && stashQuery.isBlank()) {
                    val prepended = listOf(created) + stashItems.filterNot { it.id == created.id }
                    stashItems = prepended
                    if (stashFilter == "all") prefs.saveCachedStash(prepended)
                }
                showMessage(if (dedup) "该链接已在收藏匣，已合并批注" else "已收入收藏，后台提炼中")
                refreshStash(silent = true)
            }.onFailure { err ->
                showMessage("收藏失败: ${err.message}")
            }
        }
    }

    fun markStashItemRead(item: StashItemUi) {
        val next = stashItems.map { if (it.id == item.id) it.copy(readCount = it.readCount + 1) else it }
        stashItems = next
        if (stashQuery.isBlank() && stashFilter == "all") prefs.saveCachedStash(next)
        viewModelScope.launch {
            api.updateStashItem(item.id, markRead = true)
            refreshStash(silent = true)
        }
    }

    fun toggleStashPin(item: StashItemUi) {
        val nextPinned = !item.pinned
        val prev = stashItems
        val optimistic = stashItems.map { if (it.id == item.id) it.copy(pinned = nextPinned) else it }
        stashItems = optimistic
        if (stashQuery.isBlank() && stashFilter == "all") prefs.saveCachedStash(optimistic)
        viewModelScope.launch {
            api.updateStashItem(item.id, pinned = nextPinned).onSuccess {
                refreshStash(silent = true)
            }.onFailure {
                stashItems = prev
            }
        }
    }

    fun reprocessStashItem(item: StashItemUi) {
        viewModelScope.launch {
            api.reprocessStashItem(item.id).onSuccess {
                showMessage("已重新排入 AI 提炼队列")
                refreshStash(silent = true)
            }.onFailure { err ->
                showMessage("重提炼失败: ${err.message}")
            }
        }
    }

    fun deleteStashItem(item: StashItemUi) {
        val prev = stashItems
        val optimistic = stashItems.filterNot { it.id == item.id }
        stashItems = optimistic
        if (stashQuery.isBlank() && stashFilter == "all") prefs.saveCachedStash(optimistic)
        showMessage("已删除")
        viewModelScope.launch {
            api.deleteStashItem(item.id).onFailure { err ->
                stashItems = prev
                showMessage("删除失败: ${err.message}")
            }
        }
    }

    fun askAiAboutStashItem(item: StashItemUi) {
        selectedTab = BotHubTab.AGENT
        showServiceFeed = false
        val target = bots.find { it.id == activeBotId } ?: bots.firstOrNull { !it.isGroup } ?: bots.firstOrNull()
        if (target != null) {
            openChatRoom(target.id)
        } else {
            inChatRoom = true
        }
        val summaryBlock = buildString {
            append("请结合我收藏匣中的这条内容继续深入分析：\n")
            append("标题：${item.title.ifEmpty { item.rawInput.take(60) }}\n")
            if (item.url.isNotEmpty()) append("链接：${item.url}\n")
            if (item.aiTldr.isNotEmpty()) append("AI 结论：${item.aiTldr}\n")
            if (item.points.isNotEmpty()) {
                append("核心要点：\n")
                item.points.forEach { pt -> append("- $pt\n") }
            }
        }
        sendAgentPrompt(summaryBlock)
    }

    fun runTask(task: TaskItem) {
        viewModelScope.launch {
            showMessage("正在执行: ${task.name}")
            api.triggerTask(task.id).onSuccess { msg ->
                showMessage(msg)
                refreshAll(silent = true)
            }.onFailure { err ->
                showMessage("执行失败: ${err.message}")
            }
        }
    }

    fun toggleTask(task: TaskItem, enabled: Boolean) {
        val prevDash = dashboard
        if (prevDash != null) {
            val nextTasks = prevDash.tasks.map { if (it.id == task.id) it.copy(enabled = enabled) else it }
            val optimisticDash = prevDash.copy(
                activeTasks = nextTasks.count { it.enabled },
                tasks = nextTasks
            )
            dashboard = optimisticDash
            prefs.saveCachedDashboard(optimisticDash)
        }
        viewModelScope.launch {
            api.toggleTaskEnabled(task.id, enabled).onSuccess {
                showMessage(if (enabled) "已启用: ${task.name}" else "已暂停: ${task.name}")
                refreshAll(silent = true)
            }.onFailure { err ->
                if (prevDash != null) {
                    dashboard = prevDash
                    prefs.saveCachedDashboard(prevDash)
                }
                showMessage("更新失败: ${err.message}")
            }
        }
    }

    fun syncAssets() {
        viewModelScope.launch {
            api.syncAssetWatcher().onSuccess {
                showMessage("资产状态已刷新")
                refreshAll(silent = true)
            }.onFailure { err ->
                showMessage("同步失败: ${err.message}")
            }
        }
    }

    fun saveAssetItem(
        existingId: String?,
        name: String,
        category: String,
        spec: String,
        price: String,
        dueDate: String,
        autoRenew: Boolean
    ) {
        viewModelScope.launch {
            api.saveAsset(
                existingId = existingId,
                name = name,
                category = category,
                spec = spec,
                price = price,
                dueDate = dueDate,
                autoRenew = autoRenew
            ).onSuccess {
                showMessage("资产已保存")
                refreshAll(silent = true)
            }.onFailure { err ->
                showMessage(err.message ?: "保存失败")
            }
        }
    }

    fun deleteAssetItem(assetId: String) {
        val prevDash = dashboard
        if (prevDash != null) {
            val nextAssets = prevDash.assets.filterNot { it.id == assetId }
            val optimisticDash = prevDash.copy(
                totalAssets = nextAssets.size,
                expiringCount = nextAssets.count { it.isExpiringSoon },
                assets = nextAssets
            )
            dashboard = optimisticDash
            prefs.saveCachedDashboard(optimisticDash)
        }
        viewModelScope.launch {
            api.deleteAsset(assetId).onSuccess {
                showMessage("资产已删除")
                refreshAll(silent = true)
            }.onFailure { err ->
                if (prevDash != null) {
                    dashboard = prevDash
                    prefs.saveCachedDashboard(prevDash)
                }
                showMessage(err.message ?: "删除失败")
            }
        }
    }

    fun saveAssetCategories(nextCategories: List<AssetCategoryItem>) {
        val prevDash = dashboard
        if (prevDash != null) {
            val optimisticDash = prevDash.copy(assetCategories = nextCategories)
            dashboard = optimisticDash
            prefs.saveCachedDashboard(optimisticDash)
        }
        viewModelScope.launch {
            api.saveAssetCategories(nextCategories).onSuccess { saved ->
                dashboard?.let { cur ->
                    val updated = cur.copy(assetCategories = saved)
                    dashboard = updated
                    prefs.saveCachedDashboard(updated)
                }
                showMessage("资产分类已更新")
            }.onFailure { err ->
                if (prevDash != null) {
                    dashboard = prevDash
                    prefs.saveCachedDashboard(prevDash)
                }
                showMessage(err.message ?: "保存分类失败")
            }
        }
    }

    private fun formatHumanToolName(toolName: String): String {
        val clean = toolName.trim()
        return when {
            clean == "web_search" -> "联网搜索"
            clean == "read_url" -> "阅读网页"
            clean == "manage_bots" -> "Bot 与群聊管理"
            clean == "consult_bot" -> "协同成员"
            clean == "manage_todos" -> "待办管理"
            clean == "manage_notes" -> "工作文档"
            clean == "manage_stash" -> "收藏闪念"
            clean == "manage_tasks" -> "定时提醒"
            clean == "workspace_briefing" -> "今日简报"
            clean == "send_notification" -> "主动推送"
            clean == "calculate_eval" -> "精确计算"
            clean == "datetime_calc" -> "日期推算"
            clean == "remember_fact" -> "写入记忆"
            clean == "recall_memory" -> "检索记忆"
            clean == "manage_memory" -> "整理记忆"
            clean == "manage_assets" -> "资产台账"
            clean == "query_nezha_nodes" -> "探针巡检"
            clean.startsWith("skill_") -> clean.removePrefix("skill_")
            clean.isBlank() -> "扩展能力"
            else -> clean
        }
    }

    override fun onCleared() {
        activeStreamCall?.cancel()
        activeChatJob?.cancel()
        super.onCleared()
    }

    companion object {
        fun factory(
            appContext: Context,
            prefs: BotHubPrefs,
            api: BotHubApiClient
        ): ViewModelProvider.Factory = object : ViewModelProvider.Factory {
            @Suppress("UNCHECKED_CAST")
            override fun <T : ViewModel> create(modelClass: Class<T>): T {
                return BotHubViewModel(
                    appContext = appContext.applicationContext,
                    prefs = prefs,
                    api = api
                ) as T
            }
        }
    }
}
