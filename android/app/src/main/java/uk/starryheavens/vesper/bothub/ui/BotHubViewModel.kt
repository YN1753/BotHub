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
import uk.starryheavens.vesper.bothub.data.AgentHistoryMessage
import uk.starryheavens.vesper.bothub.data.AgentSessionSummary
import uk.starryheavens.vesper.bothub.data.AssetCategoryItem
import uk.starryheavens.vesper.bothub.data.AssetItem
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.BotThreadItem
import uk.starryheavens.vesper.bothub.data.ChatToolTraceUi
import uk.starryheavens.vesper.bothub.data.ChatTurnUi
import uk.starryheavens.vesper.bothub.data.ClientSharePrefetcher
import uk.starryheavens.vesper.bothub.data.ShareTextParser
import uk.starryheavens.vesper.bothub.data.formatHumanToolName
import uk.starryheavens.vesper.bothub.data.DashboardSnapshot
import uk.starryheavens.vesper.bothub.data.JsonMappers
import uk.starryheavens.vesper.bothub.data.MemoryItemUi
import uk.starryheavens.vesper.bothub.data.ModelConfigPayload
import uk.starryheavens.vesper.bothub.data.ModelHealthItem
import uk.starryheavens.vesper.bothub.data.ModelOptionItem
import uk.starryheavens.vesper.bothub.data.ModelProviderItem
import uk.starryheavens.vesper.bothub.data.ModelUsageSummaryItem
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.data.PoolStrategiesItem
import uk.starryheavens.vesper.bothub.data.PooledModelItem
import uk.starryheavens.vesper.bothub.data.SkillItemUi
import uk.starryheavens.vesper.bothub.data.StashItemUi
import uk.starryheavens.vesper.bothub.data.TaskItem
import uk.starryheavens.vesper.bothub.data.TodoItemUi
import uk.starryheavens.vesper.bothub.navigation.BotHubTab
import uk.starryheavens.vesper.bothub.notify.NotificationHelper
import uk.starryheavens.vesper.bothub.sync.FloatingCaptureService
import uk.starryheavens.vesper.bothub.sync.RealtimePushService
import java.time.LocalDate
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

    var clipboardBannerEnabled by mutableStateOf(prefs.clipboardBannerEnabled)
        private set

    var floatingBallEnabled by mutableStateOf(prefs.floatingBallEnabled)
        private set

    var appThemeId by mutableStateOf(prefs.appThemeId)
        private set

    var bots by mutableStateOf(prefs.loadCachedBots())
        private set

    var activeBotId by mutableStateOf(prefs.lastActiveBotId)
        private set

    var botThreads by mutableStateOf<List<BotThreadItem>>(emptyList())
        private set

    var activeThreadId by mutableStateOf("")
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

    var stashKindFilter by mutableStateOf("")
        private set

    var selectedStashId by mutableStateOf<String?>(null)
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

    var skills by mutableStateOf<List<SkillItemUi>>(emptyList())
        private set

    var inspectingMemoriesBot by mutableStateOf<BotItem?>(null)
        private set

    var botMemories by mutableStateOf<List<MemoryItemUi>>(emptyList())
        private set

    var botMemoriesLoading by mutableStateOf(false)
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
                            listOf("bots", "tasks", "assets", "todos", "notes", "stash", "models", "notifications", "configs").forEach { k ->
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
                                loadBotTimeline(activeBotId, activeThreadId)
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
                            val updated = bots.map {
                                if (it.id == bId) {
                                    it.copy(
                                        state = st.ifEmpty { it.state },
                                        lastSummary = summary.ifEmpty { it.lastSummary },
                                        unreadCount = if (hasUnread) unreadVal else it.unreadCount
                                    )
                                } else it
                            }
                            if (updated != bots) {
                                bots = updated
                                prefs.saveCachedBots(updated)
                            }
                        }
                    }
                    "thread_updated" -> {
                        val bId = obj.optString("bot_id", "")
                        syncDashboardSlices(listOf("bots"))
                        if (bId == activeBotId && !isStreaming && sessionId.isEmpty()) {
                            loadBotTimeline(bId, activeThreadId)
                        }
                    }
                    "bot_message" -> {
                        val bId = obj.optString("bot_id", "")
                        val thId = obj.optString("thread_id", "")
                        val mId = obj.optLong("id", 0L)
                        val mRole = obj.optString("role", "")
                        val mKind = obj.optString("msg_kind", "chat")
                        if (bId.isNotEmpty() && bId == activeBotId && thId == activeThreadId && !isStreaming && sessionId.isEmpty() && mId > 0L) {
                            val turnId = "botmsg_$mId"
                            if (chatMessages.none { it.id == turnId }) {
                                val target = bots.find { it.id == bId }
                                val parsedTurn = AgentHistoryMessage(
                                    id = mId,
                                    role = mRole,
                                    content = obj.optString("content", ""),
                                    reasoningContent = obj.optString("reasoning_content", ""),
                                    msgKind = mKind,
                                    senderBotId = obj.optString("sender_bot_id", ""),
                                    modelUsed = obj.optString("model_used", ""),
                                    toolTraces = JsonMappers.parseToolCallsJson(obj.optString("tool_calls", "[]"))
                                ).toChatTurnUi(
                                    idPrefix = "botmsg",
                                    defaultPersona = target?.name ?: bId,
                                    defaultModel = target?.model ?: activeModel,
                                    defaultBotId = bId,
                                    defaultAvatarStyle = target?.avatarStyle ?: "butler:gold",
                                    bots = bots
                                )
                                chatMessages.add(parsedTurn)
                                val cacheKey = if (thId.isBlank()) bId else "$bId::$thId"
                                val snapTurns = chatMessages.toList()
                                timelineCache[cacheKey] = snapTurns
                                if (thId.isBlank()) prefs.saveCachedBotMessages(bId, snapTurns)
                            }
                        } else if (mKind == "routine") {
                            scheduleSilentLiveSync()
                        }
                    }
                    "stash_update" -> {
                        syncDashboardSlices(listOf("stash"))
                    }
                    "assistant_update" -> {
                        val rawKinds = obj.optString("kind", "")
                        val kinds = rawKinds
                            .split(",")
                            .map { it.trim() }
                            .filter { it.isNotEmpty() }
                            .flatMap { k ->
                                if (k == "workhub") listOf("todos", "notes") else listOf(k)
                            }
                            .distinct()
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
                slice.skills?.let { skills = it }
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
                if (curDash != null && (slice.assets != null || slice.tasks != null || slice.todos != null || slice.notes != null || slice.assetCategories != null || slice.fxRates != null || slice.modelOptions != null)) {
                    val nextDash = curDash.copy(
                        assets = slice.assets ?: curDash.assets,
                        tasks = slice.tasks ?: curDash.tasks,
                        todos = slice.todos ?: curDash.todos,
                        notes = slice.notes ?: curDash.notes,
                        assetCategories = slice.assetCategories ?: curDash.assetCategories,
                        fxRates = if (!slice.fxRates.isNullOrEmpty()) slice.fxRates else curDash.fxRates,
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
                activeModel = cachedBot.model.ifBlank { "auto:free" }
                enableThinking = cachedBot.enableThinking
            }
            val nextBots = bots.map { if (it.id == botId && it.unreadCount != 0) it.copy(unreadCount = 0) else it }
            if (nextBots != bots) {
                bots = nextBots
                prefs.saveCachedBots(nextBots)
            }

            if (changedBot || chatMessages.isEmpty()) {
                sessionId = ""
                if (changedBot) {
                    activeThreadId = ""
                    botThreads = emptyList()
                }
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

    fun updateClipboardBanner(enabled: Boolean) {
        prefs.clipboardBannerEnabled = enabled
        clipboardBannerEnabled = enabled
    }

    fun updateFloatingBall(enabled: Boolean) {
        prefs.floatingBallEnabled = enabled
        floatingBallEnabled = enabled
        FloatingCaptureService.syncWithPrefs(
            appContext,
            prefs,
            appInForeground = RealtimePushService.isAppInForeground
        )
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

    fun triggerTestPush() {
        if (!NotificationHelper.hasNotificationPermission(appContext)) {
            showMessage("系统通知权限未开启，请先点上方通知权限开启")
            return
        }
        viewModelScope.launch {
            if (prefs.bearerToken.isEmpty()) {
                showMessage("请先连接服务端")
                return@launch
            }
            if (prefs.realtimeServiceEnabled) {
                RealtimePushService.syncWithPrefs(appContext, prefs)
            }
            api.triggerTestPush().onSuccess { msg ->
                showMessage(msg.ifBlank { "测试通知已发送" })
            }.onFailure { err ->
                showMessage("测试推送失败: ${err.message}")
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

    fun handleSharedText(rawText: String, autoSend: Boolean = false) {
        val clean = rawText.trim()
        if (clean.isEmpty()) return
        viewModelScope.launch {
            val token = withContext(Dispatchers.IO) { prefs.warmUpCredentials() }
            if (token.isEmpty()) {
                showSettingsSheet = true
                showMessage("请先连接服务端")
                return@launch
            }
            isPaired = true
            selectedTab = BotHubTab.AGENT
            showServiceFeed = false
            if (activeBotId.isBlank()) {
                val fallbackId = bots.firstOrNull()?.id ?: prefs.lastActiveBotId
                if (fallbackId.isNotBlank()) {
                    activeBotId = fallbackId
                }
            }
            inChatRoom = true
            if (autoSend) {
                sendAgentPrompt(clean)
            } else {
                updateInputText(clean)
                showMessage("已填入输入框，可补充要求后发送")
            }
        }
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
                    if (payload.skills.isNotEmpty() || skills.isEmpty()) {
                        skills = payload.skills
                    }
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
        val targetThreadId = activeThreadId
        chatMessages.clear()
        if (targetId.isNotBlank() && sessionId.isEmpty()) {
            val cacheKey = if (targetThreadId.isBlank()) targetId else "$targetId::$targetThreadId"
            timelineCache.remove(cacheKey)
            if (targetThreadId.isBlank()) {
                prefs.saveCachedBotMessages(targetId, emptyList())
            }
            viewModelScope.launch {
                api.clearBotMessages(targetId, targetThreadId).onSuccess {
                    showMessage(if (targetThreadId.isBlank()) "已清空当前主对话记录" else "已清空当前话题记录")
                    loadBotTimeline(targetId, targetThreadId)
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
        allowPaidFallback: Boolean = false,
        allowedTools: List<String>? = null,
        skillIds: List<String> = emptyList()
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
                allowPaidFallback = allowPaidFallback,
                allowedTools = allowedTools,
                skillIds = skillIds
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
        allowPaidFallback: Boolean? = null,
        allowedTools: List<String>? = null,
        skillIds: List<String>? = null
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
                allowPaidFallback = allowPaidFallback,
                allowedTools = allowedTools,
                skillIds = skillIds
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

    fun openBotMemories(bot: BotItem) {
        inspectingBotProfile = null
        inspectingMemoriesBot = bot
        botMemoriesLoading = true
        val botScope = "bot:${bot.id}"
        viewModelScope.launch {
            val botRes = api.fetchMemories(botScope).getOrDefault(emptyList())
            val globalRes = api.fetchMemories("global").getOrDefault(emptyList())
            botMemories = (botRes + globalRes).distinctBy { it.id }
            botMemoriesLoading = false
        }
    }

    fun dismissBotMemories() {
        inspectingMemoriesBot = null
    }

    fun createBotMemory(bot: BotItem, category: String, factKey: String, content: String, pinned: Boolean) {
        if (content.isBlank()) return
        viewModelScope.launch {
            api.saveMemory("bot:${bot.id}", category, factKey, content, pinned).onSuccess {
                showMessage("已写入专属长期记忆")
                openBotMemories(bot)
            }.onFailure { err ->
                showMessage(err.message ?: "写入记忆失败")
            }
        }
    }

    fun toggleBotMemoryPin(mem: MemoryItemUi) {
        val nextPin = !mem.pinned
        botMemories = botMemories.map { if (it.id == mem.id) it.copy(pinned = nextPin) else it }
        viewModelScope.launch {
            api.toggleMemoryPin(mem.id, nextPin).onFailure { err ->
                botMemories = botMemories.map { if (it.id == mem.id) it.copy(pinned = mem.pinned) else it }
                showMessage(err.message ?: "操作失败")
            }
        }
    }

    fun deleteBotMemory(mem: MemoryItemUi) {
        val prev = botMemories
        botMemories = botMemories.filterNot { it.id == mem.id }
        viewModelScope.launch {
            api.deleteMemory(mem.id).onSuccess {
                showMessage("已遗忘该条记忆")
            }.onFailure { err ->
                botMemories = prev
                showMessage(err.message ?: "删除失败")
            }
        }
    }

    fun loadBotTimeline(botId: String, threadId: String? = null) {
        if (isStreaming || !prefs.isLikelyPaired) return
        viewModelScope.launch {
            if (prefs.bearerToken.isEmpty()) return@launch
            api.fetchBotMessagesPayload(botId, threadId).onSuccess { payload ->
                val target = bots.find { it.id == botId }
                val mapped = payload.messages.map { m ->
                    m.toChatTurnUi(
                        idPrefix = "botmsg",
                        defaultPersona = target?.name ?: botId,
                        defaultModel = target?.model ?: activeModel,
                        defaultBotId = botId,
                        defaultAvatarStyle = target?.avatarStyle ?: "butler:gold",
                        bots = bots
                    )
                }
                val resolvedThreadId = payload.activeThreadId
                val cacheKey = if (resolvedThreadId.isBlank()) botId else "$botId::$resolvedThreadId"
                timelineCache[cacheKey] = mapped
                if (resolvedThreadId.isBlank()) {
                    prefs.saveCachedBotMessages(botId, mapped)
                }

                if (activeBotId == botId && sessionId.isEmpty() && !isStreaming) {
                    botThreads = payload.threads
                    activeThreadId = resolvedThreadId
                    if (target != null) {
                        activeModel = target.model.ifBlank { "auto:free" }
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

    fun selectBotThread(threadId: String) {
        if (isStreaming || activeBotId.isBlank()) return
        val clean = threadId.trim()
        activeThreadId = clean
        val cacheKey = if (clean.isBlank()) activeBotId else "$activeBotId::$clean"
        timelineCache[cacheKey]?.let { cached ->
            chatMessages.clear()
            chatMessages.addAll(cached)
        } ?: chatMessages.clear()
        loadBotTimeline(activeBotId, clean)
    }

    fun createNewBotThread(title: String = "") {
        if (isStreaming || activeBotId.isBlank()) return
        val targetBotId = activeBotId
        viewModelScope.launch {
            api.createBotThread(targetBotId, title).onSuccess { (created, allThreads) ->
                if (activeBotId == targetBotId) {
                    botThreads = allThreads
                    activeThreadId = created.id
                    chatMessages.clear()
                    showMessage("已开启新话题「${created.title}」")
                }
            }.onFailure { err ->
                showMessage(err.message ?: "新建话题失败")
            }
        }
    }

    fun deleteBotThread(threadId: String) {
        if (isStreaming || activeBotId.isBlank() || threadId.isBlank()) return
        val targetBotId = activeBotId
        viewModelScope.launch {
            api.deleteBotThread(targetBotId, threadId).onSuccess { remaining ->
                timelineCache.remove("$targetBotId::$threadId")
                if (activeBotId == targetBotId) {
                    botThreads = remaining
                    if (activeThreadId == threadId) {
                        selectBotThread("")
                    }
                    showMessage("已删除话题")
                }
            }.onFailure { err ->
                showMessage(err.message ?: "删除话题失败")
            }
        }
    }

    fun summarizeActiveThreadToMemory() {
        if (isStreaming || activeBotId.isBlank()) return
        val targetBotId = activeBotId
        val targetThreadId = activeThreadId
        showMessage("正在提炼话题核心摘要并存入记忆...")
        viewModelScope.launch {
            api.summarizeBotThread(targetBotId, targetThreadId, saveToMemory = true).onSuccess { sum ->
                val shortSum = sum.replace('\n', ' ').take(48)
                showMessage(if (shortSum.isNotBlank()) "已提炼存入长期记忆：$shortSum" else "已提炼话题摘要并存入记忆")
                loadBotTimeline(targetBotId, targetThreadId)
            }.onFailure { err ->
                showMessage(err.message ?: "提炼话题摘要失败")
            }
        }
    }

    fun forkThreadFromTurn(turn: ChatTurnUi) {
        if (isStreaming || activeBotId.isBlank()) return
        val targetBotId = activeBotId
        val sourceThreadId = activeThreadId
        val upToMsgId = turn.id.removePrefix("botmsg_").toLongOrNull() ?: 0L
        viewModelScope.launch {
            api.forkBotThread(targetBotId, sourceThreadId, upToMsgId).onSuccess { (created, msgPayload) ->
                if (activeBotId == targetBotId) {
                    botThreads = msgPayload.threads
                    activeThreadId = created.id
                    val target = bots.find { it.id == targetBotId }
                    val mapped = msgPayload.messages.map { m ->
                        m.toChatTurnUi(
                            idPrefix = "botmsg",
                            defaultPersona = target?.name ?: targetBotId,
                            defaultModel = target?.model ?: activeModel,
                            defaultBotId = targetBotId,
                            defaultAvatarStyle = target?.avatarStyle ?: "butler:gold",
                            bots = bots
                        )
                    }
                    timelineCache["$targetBotId::${created.id}"] = mapped
                    chatMessages.clear()
                    chatMessages.addAll(mapped)
                    showMessage("已从该消息分叉开启新话题「${created.title}」")
                }
            }.onFailure { err ->
                showMessage(err.message ?: "分叉话题失败")
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
                    }.onFailure { err ->
                        showMessage("模型切换同步失败：${err.message ?: "网络异常"}")
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

    fun sendAgentPrompt(
        promptText: String,
        overrideModel: String? = null,
        allowPaidOnce: Boolean = false
    ) {
        val clean = promptText.trim()
        if (clean.isEmpty() || isStreaming) return
        if (prefs.bearerToken.isEmpty()) {
            showSettingsSheet = true
            return
        }

        val activeBotObj = bots.find { it.id == activeBotId }
        val botDisplayName = activeBotObj?.name ?: activePersona
        val botStyle = activeBotObj?.avatarStyle ?: "butler:gold"
        val effectiveModel = overrideModel?.trim()?.takeIf { it.isNotEmpty() } ?: activeModel
        val effectiveAllowPaid = if (allowPaidOnce) true else activeBotObj?.allowPaidFallback
        val seq = turnSequence.incrementAndGet()
        val now = System.currentTimeMillis()
        val userTurnId = "u_${now}_$seq"
        val assistantId = "a_${now}_$seq"

        val targetBotId = activeBotId
        val targetThreadId = activeThreadId
        if (overrideModel == null) {
            inputText = ""
            if (sessionId.isEmpty() && targetBotId.isNotBlank()) {
                persistBotDraft(targetBotId, "")
            }
        }
        isStreaming = true
        chatMessages.add(
            ChatTurnUi(
                id = userTurnId,
                role = "user",
                persona = botDisplayName,
                model = effectiveModel,
                content = clean
            )
        )
        chatMessages.add(
            ChatTurnUi(
                id = assistantId,
                role = "assistant",
                persona = botDisplayName,
                model = effectiveModel,
                content = "",
                senderBotId = activeBotId,
                avatarStyle = botStyle
            )
        )

        activeChatJob = viewModelScope.launch {
            val streamedContent = StringBuilder()
            var streamHadError = false
            var modelSwitchedInTurn = false
            try {
                api.streamChat(
                    sessionId = sessionId,
                    botId = activeBotId,
                    threadId = targetThreadId,
                    message = clean,
                    persona = activePersona,
                    model = effectiveModel,
                    enableThinking = enableThinking,
                    allowPaidFallback = effectiveAllowPaid,
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
                    onToolDetail = { tcId, tcName, tcArgs, tcResult, tcStatus ->
                        viewModelScope.launch {
                            updateTurnById(assistantId) { cur ->
                                val humanName = formatHumanToolName(tcName)
                                val existingIdx = cur.toolTraces.indexOfLast { tr ->
                                    (tcId.isNotBlank() && tr.id == tcId) ||
                                        (tcId.isBlank() && tr.name == tcName && tr.status == "进行中")
                                }
                                val nextTraces = if (existingIdx >= 0) {
                                    cur.toolTraces.mapIndexed { idx, existing ->
                                        if (idx == existingIdx) {
                                            existing.copy(
                                                name = tcName.ifBlank { existing.name },
                                                displayName = humanName.ifBlank { existing.displayName },
                                                arguments = tcArgs.ifBlank { existing.arguments },
                                                result = tcResult.ifBlank { existing.result },
                                                status = tcStatus.ifBlank { existing.status }
                                            )
                                        } else {
                                            existing
                                        }
                                    }
                                } else {
                                    cur.toolTraces + ChatToolTraceUi(
                                        id = tcId.ifBlank { "live_${cur.toolTraces.size}_$tcName" },
                                        name = tcName,
                                        displayName = humanName,
                                        arguments = tcArgs,
                                        result = tcResult,
                                        status = tcStatus.ifBlank { "已完成" }
                                    )
                                }
                                cur.copy(toolTraces = nextTraces)
                            }
                        }
                    },
                    onContentDelta = { delta ->
                        synchronized(streamedContent) {
                            streamedContent.append(delta)
                        }
                        viewModelScope.launch {
                            updateTurnById(assistantId) { cur ->
                                cur.copy(content = cur.content + delta)
                            }
                        }
                    },
                    onModelSwitch = { fromLabel, toLabel, toModel, reason ->
                        modelSwitchedInTurn = true
                        val targetDisplay = toLabel.ifBlank { toModel }.ifBlank { "备用模型" }
                        val fromDisplay = fromLabel.ifBlank { "原选模型" }
                        val shortReason = reason.take(48).trim()
                        val notice = if (shortReason.isNotEmpty()) {
                            "$fromDisplay 不可用（$shortReason），已切换至 $targetDisplay"
                        } else {
                            "$fromDisplay 限流或报错，已自动切换至 $targetDisplay"
                        }
                        viewModelScope.launch {
                            showMessage(notice)
                            updateTurnById(assistantId) { cur ->
                                cur.copy(model = "${toModel.ifBlank { targetDisplay }} (已回退)")
                            }
                        }
                    },
                    onDoneModel = { usedModel ->
                        viewModelScope.launch {
                            updateTurnById(assistantId) { cur ->
                                val displayModel = if (modelSwitchedInTurn && !usedModel.contains("已回退")) {
                                    "$usedModel (已回退)"
                                } else {
                                    usedModel
                                }
                                cur.copy(model = displayModel)
                            }
                        }
                    },
                    onError = { err ->
                        streamHadError = true
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
                val replyContent = synchronized(streamedContent) { streamedContent.toString() }.trim()
                    .ifEmpty { chatMessages.find { it.id == assistantId }?.content?.trim().orEmpty() }
                if (!RealtimePushService.isAppInForeground && !streamHadError && replyContent.isNotEmpty()) {
                    val notifyBotId = targetBotId.ifBlank { activeBotId }
                    val notifyBotName = bots.find { it.id == notifyBotId }?.name ?: botDisplayName
                    NotificationHelper.showBotMessageNotification(
                        context = appContext,
                        botId = notifyBotId,
                        botName = notifyBotName,
                        messageId = 0L,
                        content = replyContent,
                        isRoutine = false
                    )
                }
                if (sessionId.isEmpty() && targetBotId.isNotBlank() && activeBotId == targetBotId) {
                    val snapshotTurns = chatMessages.toList()
                    val cacheKey = if (targetThreadId.isBlank()) targetBotId else "$targetBotId::$targetThreadId"
                    timelineCache[cacheKey] = snapshotTurns
                    if (targetThreadId.isBlank()) {
                        prefs.saveCachedBotMessages(targetBotId, snapshotTurns)
                    }
                    loadBotTimeline(targetBotId, targetThreadId)
                }
                refreshAll(silent = true)
            }
        }
    }

    fun retryTurnWithModel(
        assistantTurn: ChatTurnUi,
        targetModel: String,
        allowPaidOnce: Boolean
    ) {
        if (isStreaming) return
        val aIdx = chatMessages.indexOfFirst { it.id == assistantTurn.id }
        if (aIdx < 0) return

        var uIdx = -1
        for (i in (aIdx - 1) downTo 0) {
            if (chatMessages[i].role == "user" && chatMessages[i].content.isNotBlank()) {
                uIdx = i
                break
            }
        }
        if (uIdx < 0) {
            showMessage("未找到对应的用户提问，无法原位重试")
            return
        }
        val userTurn = chatMessages[uIdx]
        val promptToReplay = userTurn.content
        val targetBotId = activeBotId

        chatMessages.removeAll { it.id == assistantTurn.id || it.id == userTurn.id }

        viewModelScope.launch {
            if (sessionId.isEmpty() && targetBotId.isNotBlank()) {
                val aServerId = assistantTurn.id.removePrefix("botmsg_").toLongOrNull() ?: 0L
                val uServerId = userTurn.id.removePrefix("botmsg_").toLongOrNull() ?: 0L
                if (aServerId > 0L) api.deleteBotMessage(targetBotId, aServerId)
                if (uServerId > 0L) api.deleteBotMessage(targetBotId, uServerId)
            }
            sendAgentPrompt(
                promptText = promptToReplay,
                overrideModel = targetModel,
                allowPaidOnce = allowPaidOnce
            )
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
        val prefixBotName = item.title.trim()
            .takeIf { it.startsWith("[") && it.contains("]") }
            ?.substringAfter("[")
            ?.substringBefore("]")
            ?.trim()
            .orEmpty()
        val matchedByPrefix = if (prefixBotName.isNotEmpty()) {
            bots.firstOrNull { !it.isGroup && it.name.equals(prefixBotName, ignoreCase = true) }
        } else null
        val matchedByTask = dashboard?.tasks
            ?.firstOrNull { it.id == item.moduleId && it.botId.isNotBlank() }
            ?.let { task -> bots.firstOrNull { !it.isGroup && it.id == task.botId } }
        val targetBot = matchedByPrefix
            ?: matchedByTask
            ?: bots.firstOrNull { !it.isGroup && it.botMode != "companion" }
            ?: bots.firstOrNull { !it.isGroup }
            ?: bots.firstOrNull()
        if (targetBot != null) {
            openChatRoom(targetBot.id)
        } else {
            inChatRoom = true
        }
        sendAgentPrompt("关于刚才收到的服务通知「${item.title}」：\n${item.previewText(320)}\n\n请帮我进一步分析或处理。")
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

    fun updateStashKindFilter(kind: String) {
        stashKindFilter = if (stashKindFilter == kind) "" else kind
    }

    fun openStashDetail(item: StashItemUi) {
        selectedStashId = item.id
        if (item.isUnread) {
            markStashItemRead(item)
        }
    }

    fun closeStashDetail() {
        selectedStashId = null
    }

    fun captureStashItem(rawInput: String, userNote: String) {
        viewModelScope.launch {
            val preview = ShareTextParser.parse(rawInput)
            val prefetched = if (preview.extractedUrl.isNotBlank()) {
                ClientSharePrefetcher.prefetch(preview.extractedUrl)
            } else {
                null
            }
            api.createStashItem(rawInput, userNote, prefetched = prefetched).onSuccess { (created, dedup) ->
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
        val next = stashItems.map {
            if (it.id == item.id) it.copy(readCount = maxOf(1, it.readCount + 1)) else it
        }
        stashItems = next
        if (stashQuery.isBlank() && stashFilter == "all") prefs.saveCachedStash(next)
        viewModelScope.launch {
            api.updateStashItem(item.id, markRead = true)
            if (stashFilter != "unread") {
                refreshStash(silent = true)
            }
        }
    }

    fun toggleStashRead(item: StashItemUi) {
        val markingRead = item.isUnread
        val prev = stashItems
        val next = stashItems.map {
            if (it.id == item.id) it.copy(readCount = if (markingRead) 1 else 0) else it
        }
        stashItems = next
        if (stashQuery.isBlank() && stashFilter == "all") prefs.saveCachedStash(next)
        showMessage(if (markingRead) "已标为已读" else "已标为未读")
        viewModelScope.launch {
            api.updateStashItem(
                id = item.id,
                markRead = if (markingRead) true else null,
                markUnread = if (!markingRead) true else null
            ).onSuccess {
                refreshStash(silent = true)
            }.onFailure {
                stashItems = prev
            }
        }
    }

    fun markAllStashRead() {
        val prev = stashItems
        val next = stashItems.map { if (it.isUnread) it.copy(readCount = 1) else it }
        stashItems = next
        if (stashQuery.isBlank() && stashFilter == "all") prefs.saveCachedStash(next)
        viewModelScope.launch {
            api.markAllStashRead().onSuccess { count ->
                showMessage(if (count > 0) "已将 $count 条收藏标为已读" else "已全部标为已读")
                refreshStash(silent = true)
            }.onFailure { err ->
                stashItems = prev
                showMessage("操作失败: ${err.message}")
            }
        }
    }

    fun editStashItem(item: StashItemUi, title: String, userNote: String, tags: String) {
        val cleanTitle = title.trim().ifBlank { item.title }
        val cleanNote = userNote.trim()
        val cleanTags = tags.trim()
        val prev = stashItems
        val optimistic = stashItems.map {
            if (it.id == item.id) it.copy(title = cleanTitle, userNote = cleanNote, tags = cleanTags) else it
        }
        stashItems = optimistic
        if (stashQuery.isBlank() && stashFilter == "all") prefs.saveCachedStash(optimistic)
        viewModelScope.launch {
            api.updateStashItem(
                id = item.id,
                title = cleanTitle,
                userNote = cleanNote,
                tags = cleanTags
            ).onSuccess { updated ->
                stashItems = stashItems.map { if (it.id == updated.id) updated else it }
                if (stashQuery.isBlank() && stashFilter == "all") prefs.saveCachedStash(stashItems)
                showMessage("已保存修改")
            }.onFailure { err ->
                stashItems = prev
                showMessage("保存失败: ${err.message}")
            }
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
        if (selectedStashId == item.id) {
            selectedStashId = null
        }
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

    fun convertStashToTodo(item: StashItemUi) {
        val title = item.title.ifEmpty { item.rawInput.take(60) }.ifEmpty { "收藏转化待办" }
        val notes = buildList {
            if (item.aiTldr.isNotBlank()) add("核心结论：${item.aiTldr}")
            if (item.url.isNotBlank()) add("原链接：${item.url}")
            if (item.userNote.isNotBlank()) add("批注：${item.userNote}")
        }.joinToString("\n")
        viewModelScope.launch {
            api.createTodo(title = title, notes = notes, priority = "normal").onSuccess { created ->
                dashboard?.let { cur ->
                    val nextDash = cur.copy(todos = listOf(created) + cur.todos.filterNot { it.id == created.id })
                    dashboard = nextDash
                    prefs.saveCachedDashboard(nextDash)
                }
                markStashItemRead(item)
                showMessage("已一键转化为待办并标为已读")
                syncDashboardSlices(listOf("todos", "stash"))
            }.onFailure { err ->
                showMessage(err.message ?: "转化为待办失败")
            }
        }
    }

    fun convertStashToNote(item: StashItemUi) {
        val title = item.title.ifEmpty { item.rawInput.take(50) }.ifEmpty { "收藏整理笔记" }
        val md = buildList {
            if (item.url.isNotBlank()) add("> 原链接：[${item.domain.ifBlank { item.url }}](${item.url})")
            if (item.aiTldr.isNotBlank()) add("### AI 核心结论\n\n${item.aiTldr}")
            if (item.points.isNotEmpty()) {
                add("### 核心干货要点\n\n" + item.points.joinToString("\n") { "- $it" })
            }
            if (item.userNote.isNotBlank()) add("### 个人批注\n\n${item.userNote}")
            if (isEmpty() && item.rawInput.isNotBlank()) add(item.rawInput)
        }.joinToString("\n\n")
        viewModelScope.launch {
            api.saveNote(existingId = null, title = title, content = md, category = "deliverable", pinned = false)
                .onSuccess {
                    markStashItemRead(item)
                    showMessage("已整理保存至工作笔记并标为已读")
                    syncDashboardSlices(listOf("notes", "stash"))
                }
                .onFailure { err ->
                    showMessage(err.message ?: "存为工作笔记失败")
                }
        }
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

    fun quickRenewAsset(asset: AssetItem, months: Int) {
        if (asset.isPerpetual || months <= 0) return
        val today = LocalDate.now()
        val parsedCur = runCatching { LocalDate.parse(asset.dueDate.take(10)) }.getOrNull()
        val baseDate = if (parsedCur != null && !asset.dueDate.startsWith("2099") && !parsedCur.isBefore(today.minusDays(30))) {
            parsedCur
        } else {
            today
        }
        val nextDue = baseDate.plusMonths(months.toLong()).toString()
        val stepLabel = when (months) {
            1 -> "1 个月"
            3 -> "1 季度"
            12 -> "1 年"
            else -> "$months 个月"
        }
        viewModelScope.launch {
            api.saveAsset(
                existingId = asset.id,
                name = asset.name,
                category = asset.category,
                spec = asset.spec,
                price = asset.price,
                dueDate = nextDue,
                autoRenew = asset.autoRenew
            ).onSuccess {
                showMessage("已将「${asset.name}」顺延 $stepLabel 至 $nextDue")
                refreshAll(silent = true)
            }.onFailure { err ->
                showMessage(err.message ?: "顺延到期日失败")
            }
        }
    }

    fun saveTaskItem(
        existingId: String?,
        name: String,
        type: String,
        botId: String,
        cronExpr: String,
        runAt: String,
        description: String,
        model: String,
        allowPaidFallback: Boolean,
        maxPaidCallsPerDay: Int,
        enabled: Boolean,
        params: String = "{}"
    ) {
        viewModelScope.launch {
            api.saveTask(
                existingId = existingId,
                name = name,
                type = type,
                botId = botId,
                cronExpr = cronExpr,
                runAt = runAt,
                description = description,
                model = model,
                allowPaidFallback = allowPaidFallback,
                maxPaidCallsPerDay = maxPaidCallsPerDay,
                enabled = enabled,
                params = params
            ).onSuccess {
                showMessage("定时任务已保存")
                refreshAll(silent = true)
            }.onFailure { err ->
                showMessage(err.message ?: "保存任务失败")
            }
        }
    }

    fun deleteTaskItem(taskId: String) {
        val prevDash = dashboard
        if (prevDash != null) {
            val nextTasks = prevDash.tasks.filterNot { it.id == taskId }
            val optimisticDash = prevDash.copy(
                activeTasks = nextTasks.count { it.enabled },
                tasks = nextTasks
            )
            dashboard = optimisticDash
            prefs.saveCachedDashboard(optimisticDash)
        }
        viewModelScope.launch {
            api.deleteTask(taskId).onSuccess {
                showMessage("任务已删除")
                refreshAll(silent = true)
            }.onFailure { err ->
                if (prevDash != null) {
                    dashboard = prevDash
                    prefs.saveCachedDashboard(prevDash)
                }
                showMessage(err.message ?: "删除任务失败")
            }
        }
    }

    fun createTodoItem(
        title: String,
        notes: String = "",
        dueAt: String = "",
        priority: String = "normal"
    ) {
        if (title.isBlank()) return
        viewModelScope.launch {
            api.createTodo(title, notes, dueAt, priority).onSuccess { created ->
                dashboard?.let { cur ->
                    val nextDash = cur.copy(todos = listOf(created) + cur.todos.filterNot { it.id == created.id })
                    dashboard = nextDash
                    prefs.saveCachedDashboard(nextDash)
                }
                showMessage("已添加待办")
                syncDashboardSlices(listOf("todos"))
            }.onFailure { err ->
                showMessage(err.message ?: "添加待办失败")
            }
        }
    }

    fun toggleTodoDone(todo: TodoItemUi) {
        val nextStatus = if (todo.isDone) "todo" else "done"
        val prevDash = dashboard
        if (prevDash != null) {
            val optimistic = prevDash.copy(
                todos = prevDash.todos.map { if (it.id == todo.id) it.copy(status = nextStatus) else it }
            )
            dashboard = optimistic
            prefs.saveCachedDashboard(optimistic)
        }
        viewModelScope.launch {
            api.updateTodo(id = todo.id, status = nextStatus).onSuccess {
                syncDashboardSlices(listOf("todos"))
            }.onFailure { err ->
                if (prevDash != null) {
                    dashboard = prevDash
                    prefs.saveCachedDashboard(prevDash)
                }
                showMessage(err.message ?: "更新待办状态失败")
            }
        }
    }

    fun updateTodoItem(
        id: String,
        title: String,
        notes: String,
        dueAt: String,
        priority: String
    ) {
        viewModelScope.launch {
            api.updateTodo(
                id = id,
                title = title,
                notes = notes,
                dueAt = dueAt,
                priority = priority
            ).onSuccess {
                showMessage("待办已更新")
                syncDashboardSlices(listOf("todos"))
            }.onFailure { err ->
                showMessage(err.message ?: "更新待办失败")
            }
        }
    }

    fun deleteTodoItem(todoId: String) {
        val prevDash = dashboard
        if (prevDash != null) {
            val optimistic = prevDash.copy(todos = prevDash.todos.filterNot { it.id == todoId })
            dashboard = optimistic
            prefs.saveCachedDashboard(optimistic)
        }
        viewModelScope.launch {
            api.deleteTodo(todoId).onSuccess {
                showMessage("待办已删除")
                syncDashboardSlices(listOf("todos"))
            }.onFailure { err ->
                if (prevDash != null) {
                    dashboard = prevDash
                    prefs.saveCachedDashboard(prevDash)
                }
                showMessage(err.message ?: "删除待办失败")
            }
        }
    }

    fun saveNoteItem(
        existingId: String?,
        title: String,
        content: String,
        category: String = "deliverable",
        pinned: Boolean = false
    ) {
        viewModelScope.launch {
            api.saveNote(
                existingId = existingId,
                title = title,
                content = content,
                category = category,
                pinned = pinned
            ).onSuccess {
                showMessage("工作笔记已保存")
                syncDashboardSlices(listOf("notes"))
            }.onFailure { err ->
                showMessage(err.message ?: "保存笔记失败")
            }
        }
    }

    fun deleteNoteItem(noteId: String) {
        val prevDash = dashboard
        if (prevDash != null) {
            val optimistic = prevDash.copy(notes = prevDash.notes.filterNot { it.id == noteId })
            dashboard = optimistic
            prefs.saveCachedDashboard(optimistic)
        }
        viewModelScope.launch {
            api.deleteNote(noteId).onSuccess {
                showMessage("工作笔记已删除")
                syncDashboardSlices(listOf("notes"))
            }.onFailure { err ->
                if (prevDash != null) {
                    dashboard = prevDash
                    prefs.saveCachedDashboard(prevDash)
                }
                showMessage(err.message ?: "删除笔记失败")
            }
        }
    }

    fun convertTurnToTodo(turn: ChatTurnUi) {
        val raw = turn.content.trim()
        if (raw.isEmpty()) return
        val firstLine = raw.lineSequence()
            .map { it.trim().removePrefix("#").trim().replace("**", "").replace("`", "") }
            .firstOrNull { it.isNotBlank() }
            .orEmpty()
        val title = firstLine.take(60).ifBlank { "对话转化待办" }
        val notes = raw.take(1500)
        viewModelScope.launch {
            api.createTodo(title = title, notes = notes, priority = "normal").onSuccess { created ->
                dashboard?.let { cur ->
                    val nextDash = cur.copy(todos = listOf(created) + cur.todos.filterNot { it.id == created.id })
                    dashboard = nextDash
                    prefs.saveCachedDashboard(nextDash)
                }
                showMessage("已转为待办事项")
                syncDashboardSlices(listOf("todos"))
            }.onFailure { err ->
                showMessage(err.message ?: "转为待办失败")
            }
        }
    }

    fun saveTurnAsNote(turn: ChatTurnUi) {
        val raw = turn.content.trim()
        if (raw.isEmpty()) return
        val firstLine = raw.lineSequence()
            .map { it.trim().trimStart('#').trim().replace("**", "").replace("`", "") }
            .firstOrNull { it.isNotBlank() }
            .orEmpty()
        val title = firstLine.take(40).ifBlank { "AI 对话文档" }
        viewModelScope.launch {
            api.saveNote(
                existingId = null,
                title = title,
                content = raw,
                category = "deliverable",
                pinned = false
            ).onSuccess {
                showMessage("已存入工作文档")
                syncDashboardSlices(listOf("notes"))
            }.onFailure { err ->
                showMessage(err.message ?: "存入工作文档失败")
            }
        }
    }

    fun quickToggleTodoById(todoId: String, nextStatus: String) {
        val cleanId = todoId.trim()
        val cleanStatus = nextStatus.trim().ifBlank { "done" }
        if (cleanId.isEmpty()) return
        val prevDash = dashboard
        if (prevDash != null && prevDash.todos.any { it.id == cleanId }) {
            val optimistic = prevDash.copy(
                todos = prevDash.todos.map {
                    if (it.id == cleanId) it.copy(status = cleanStatus) else it
                }
            )
            dashboard = optimistic
            prefs.saveCachedDashboard(optimistic)
        }
        viewModelScope.launch {
            api.updateTodo(id = cleanId, status = cleanStatus).onSuccess {
                syncDashboardSlices(listOf("todos"))
            }.onFailure { err ->
                if (prevDash != null) {
                    dashboard = prevDash
                    prefs.saveCachedDashboard(prevDash)
                }
                showMessage(err.message ?: "更新待办状态失败")
            }
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
