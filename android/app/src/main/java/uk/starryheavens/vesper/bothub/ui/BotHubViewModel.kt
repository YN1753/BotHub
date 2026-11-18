package uk.starryheavens.vesper.bothub.ui

import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.launch
import okhttp3.Call
import uk.starryheavens.vesper.bothub.data.AgentSessionSummary
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.ChatTurnUi
import uk.starryheavens.vesper.bothub.data.DashboardSnapshot
import uk.starryheavens.vesper.bothub.data.NotificationItem
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

    var isPaired by mutableStateOf(prefs.bearerToken.isNotEmpty())
        private set

    var showSettingsSheet by mutableStateOf(prefs.bearerToken.isEmpty())
        private set

    var biometricUnlocked by mutableStateOf(!prefs.biometricAppLockEnabled || prefs.bearerToken.isEmpty())
        private set

    var serverUrl by mutableStateOf(prefs.serverUrl)
        private set

    var realtimeServiceEnabled by mutableStateOf(prefs.realtimeServiceEnabled)
        private set

    var biometricAppLockEnabled by mutableStateOf(prefs.biometricAppLockEnabled)
        private set

    var zeroClickCaptureEnabled by mutableStateOf(prefs.zeroClickCaptureEnabled)
        private set

    var bots by mutableStateOf<List<BotItem>>(emptyList())
        private set

    var activeBotId by mutableStateOf("butler")
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

    var stashItems by mutableStateOf<List<StashItemUi>>(emptyList())
        private set

    var stashQuery by mutableStateOf("")
        private set

    var stashFilter by mutableStateOf("unread")
        private set

    var stashLoading by mutableStateOf(false)
        private set

    var dashboard by mutableStateOf<DashboardSnapshot?>(null)
        private set

    var isRefreshing by mutableStateOf(false)
        private set

    private var pendingNotificationId: Long? = null
    private var stashSearchJob: Job? = null
    private var activeChatJob: Job? = null

    @Volatile
    private var activeStreamCall: Call? = null
    private val turnSequence = AtomicLong(0L)

    init {
        if (prefs.bearerToken.isNotEmpty()) {
            RealtimePushService.syncWithPrefs(appContext, prefs)
            refreshAll()
        }
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
            when (tab) {
                BotHubTab.AGENT, BotHubTab.INBOX, BotHubTab.HUB -> refreshAll()
                BotHubTab.STASH -> refreshStash()
            }
        }
    }

    fun updateImSearchQuery(q: String) {
        imSearchQuery = q
    }

    fun openChatRoom(botId: String) {
        inspectingBotProfile = null
        showServiceFeed = false
        if (botId.isNotBlank()) {
            val changedBot = activeBotId != botId
            activeBotId = botId
            bots = bots.map { if (it.id == botId) it.copy(unreadCount = 0) else it }
            if (changedBot || chatMessages.isEmpty()) {
                sessionId = ""
                loadBotTimeline(botId)
            }
        }
        inChatRoom = true
    }

    fun closeChatRoom() {
        inChatRoom = false
        if (prefs.bearerToken.isNotEmpty()) {
            viewModelScope.launch {
                api.fetchBots().onSuccess { bots = it }
            }
        }
    }

    fun openServiceFeed() {
        inChatRoom = false
        showServiceFeed = true
        refreshAll()
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

    fun refreshAll() {
        if (prefs.bearerToken.isEmpty()) return
        viewModelScope.launch {
            isRefreshing = true
            try {
                val bootRes = api.fetchBootstrap()
                if (bootRes.isSuccess) {
                    val payload = bootRes.getOrThrow()
                    notifications = payload.notifications
                    prefs.saveCachedNotifications(payload.notifications)
                    val maxId = payload.notifications.maxOfOrNull { it.id } ?: 0L
                    prefs.advanceLastEventId(maxId)
                    resolvePendingNotificationIfPresent(payload.notifications)

                    dashboard = payload.dashboard

                    val firstLoad = bots.isEmpty()
                    bots = payload.bots
                    if (firstLoad && payload.bots.isNotEmpty()) {
                        val chosen = payload.bots.find { it.id == activeBotId } ?: payload.bots.first()
                        activeBotId = chosen.id
                    }

                    if (stashQuery.isBlank() && stashFilter == "all") {
                        stashItems = payload.stashItems
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
                            api.fetchDashboard().onSuccess { snap -> dashboard = snap }
                        }
                        launch {
                            api.fetchBots().onSuccess { list -> bots = list }
                        }
                        launch {
                            api.fetchStashItems(query = stashQuery, filter = stashFilter, limit = 80).onSuccess { items ->
                                stashItems = items
                            }
                        }
                    }
                }
            } finally {
                isRefreshing = false
            }
        }
    }

    fun refreshStash(queryOverride: String = stashQuery, filterOverride: String = stashFilter) {
        if (prefs.bearerToken.isEmpty()) return
        viewModelScope.launch {
            stashLoading = true
            try {
                api.fetchStashItems(query = queryOverride, filter = filterOverride, limit = 80).onSuccess { items ->
                    stashItems = items
                }
            } finally {
                stashLoading = false
            }
        }
    }

    fun selectBot(botId: String) {
        openChatRoom(botId)
    }

    fun togglePinBot(bot: BotItem) {
        val nextPinned = !bot.isPinned
        bots = bots.map { if (it.id == bot.id) it.copy(isPinned = nextPinned) else it }
        viewModelScope.launch {
            api.updateBotPin(bot.id, nextPinned).onSuccess { updated ->
                bots = bots.map { if (it.id == bot.id) updated else it }
                showMessage(if (nextPinned) "已置顶聊天" else "已取消置顶")
            }.onFailure { err ->
                bots = bots.map { if (it.id == bot.id) bot else it }
                showMessage(err.message ?: "置顶操作失败")
            }
        }
    }

    fun clearActiveChatHistory() {
        if (isStreaming) return
        val targetId = activeBotId
        chatMessages.clear()
        if (targetId.isNotBlank() && sessionId.isEmpty()) {
            viewModelScope.launch {
                api.clearBotMessages(targetId).onSuccess {
                    showMessage("已清空当前聊天记录")
                    refreshAll()
                }.onFailure { err ->
                    showMessage(err.message ?: "清空记录失败")
                }
            }
        } else {
            sessionId = ""
            showMessage("已开启全新对话")
        }
    }

    fun createNewFriend(
        name: String,
        roleTitle: String,
        avatarStyle: String,
        botMode: String,
        traits: String,
        systemPrompt: String
    ) {
        viewModelScope.launch {
            api.createAiFriend(
                name = name,
                roleTitle = roleTitle,
                avatarStyle = avatarStyle,
                botMode = botMode,
                traits = traits,
                systemPrompt = systemPrompt
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
        announcement: String
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
                announcement = announcement
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
        if (isStreaming || prefs.bearerToken.isEmpty()) return
        viewModelScope.launch {
            api.fetchBotMessages(botId).onSuccess { msgs ->
                val target = bots.find { it.id == botId }
                if (target != null) {
                    activeModel = target.model
                    enableThinking = target.enableThinking
                }
                chatMessages.clear()
                msgs.forEach { m ->
                    chatMessages.add(
                        m.toChatTurnUi(
                            idPrefix = "botmsg",
                            defaultPersona = target?.name ?: botId,
                            defaultModel = target?.model ?: activeModel,
                            defaultBotId = botId,
                            defaultAvatarStyle = target?.avatarStyle ?: "butler:gold",
                            bots = bots
                        )
                    )
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
    }

    fun updateThinking(enabled: Boolean) {
        enableThinking = enabled
        prefs.enableThinking = enabled
    }

    fun updateInputText(text: String) {
        inputText = text
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

        inputText = ""
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
                                val label = "$toolName ($status)"
                                val updatedTools = cur.tools.filterNot { it.startsWith("$toolName (") } + label
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
                refreshAll()
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
            viewModelScope.launch {
                api.markNotificationRead(item.id)
                refreshAll()
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
            activeBotId = butler.id
        }
        inChatRoom = true
        sendAgentPrompt("请帮我分析这条通知并给出排查或处置建议：\n\n标题：${item.title}\n\n${item.markdownBody}")
    }

    fun markAllNotificationsRead() {
        viewModelScope.launch {
            api.markAllNotificationsRead()
            refreshAll()
            showMessage("全部情报已标记为已读")
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
        refreshStash(stashQuery, filter)
    }

    fun captureStashItem(rawInput: String, userNote: String) {
        viewModelScope.launch {
            api.createStashItem(rawInput, userNote).onSuccess { (_, dedup) ->
                showMessage(if (dedup) "该链接已在收藏匣，已合并批注" else "已收入收藏匣，后台正提炼要点")
                refreshStash()
            }.onFailure { err ->
                showMessage("收藏失败: ${err.message}")
            }
        }
    }

    fun markStashItemRead(item: StashItemUi) {
        viewModelScope.launch {
            api.updateStashItem(item.id, markRead = true)
            refreshStash()
        }
    }

    fun toggleStashPin(item: StashItemUi) {
        viewModelScope.launch {
            api.updateStashItem(item.id, pinned = !item.pinned).onSuccess {
                refreshStash()
            }
        }
    }

    fun reprocessStashItem(item: StashItemUi) {
        viewModelScope.launch {
            api.reprocessStashItem(item.id).onSuccess {
                showMessage("已重新排入 AI 提炼队列")
                refreshStash()
            }.onFailure { err ->
                showMessage("重提炼失败: ${err.message}")
            }
        }
    }

    fun deleteStashItem(item: StashItemUi) {
        viewModelScope.launch {
            api.deleteStashItem(item.id).onSuccess {
                showMessage("已删除收藏条目")
                refreshStash()
            }
        }
    }

    fun askAiAboutStashItem(item: StashItemUi) {
        selectedTab = BotHubTab.AGENT
        showServiceFeed = false
        val target = bots.find { it.id == activeBotId } ?: bots.firstOrNull { !it.isGroup } ?: bots.firstOrNull()
        if (target != null) {
            activeBotId = target.id
        }
        inChatRoom = true
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
            showMessage("正在触发: ${task.name}")
            api.triggerTask(task.id).onSuccess { msg ->
                showMessage(msg)
                refreshAll()
            }.onFailure { err ->
                showMessage("执行失败: ${err.message}")
            }
        }
    }

    fun toggleTask(task: TaskItem, enabled: Boolean) {
        viewModelScope.launch {
            api.toggleTaskEnabled(task.id, enabled).onSuccess {
                showMessage(if (enabled) "已启用: ${task.name}" else "已暂停: ${task.name}")
                refreshAll()
            }.onFailure { err ->
                showMessage("更新失败: ${err.message}")
            }
        }
    }

    fun syncAssets() {
        viewModelScope.launch {
            api.syncAssetWatcher().onSuccess {
                showMessage("资产状态已刷新")
                refreshAll()
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
                refreshAll()
            }.onFailure { err ->
                showMessage(err.message ?: "保存失败")
            }
        }
    }

    fun deleteAssetItem(assetId: String) {
        viewModelScope.launch {
            api.deleteAsset(assetId).onSuccess {
                showMessage("资产已删除")
                refreshAll()
            }.onFailure { err ->
                showMessage(err.message ?: "删除失败")
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
