package uk.starryheavens.vesper.bothub.ui

import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import androidx.compose.animation.animateContentSize
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.BookmarkBorder
import androidx.compose.material.icons.filled.ChatBubbleOutline
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.DeleteOutline
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.DoneAll
import androidx.compose.material.icons.filled.Inbox
import androidx.compose.material.icons.filled.OpenInNew
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Send
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Stop
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import okhttp3.Call
import uk.starryheavens.vesper.bothub.data.AgentSessionSummary
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.data.DashboardSnapshot
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.data.StashItemUi
import uk.starryheavens.vesper.bothub.data.TaskItem
import uk.starryheavens.vesper.bothub.sync.RealtimePushService

data class ChatTurnUi(
    val id: String,
    val role: String,
    val persona: String,
    val model: String,
    val reasoning: String = "",
    val tools: List<String> = emptyList(),
    val content: String = "",
    val msgKind: String = "chat"
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BotHubApp(
    prefs: BotHubPrefs,
    api: BotHubApiClient,
    sharedText: String?,
    onSharedTextConsumed: () -> Unit,
    requestedTab: Int? = null,
    onRequestedTabConsumed: () -> Unit = {}
) {
    val context = LocalContext.current
    val clipboard = LocalClipboardManager.current
    val haptics = LocalHapticFeedback.current
    val scope = rememberCoroutineScope()
    val snackbarHostState = remember { SnackbarHostState() }

    var selectedTab by remember { mutableIntStateOf(0) }
    var showSettingsSheet by remember { mutableStateOf(prefs.bearerToken.isEmpty()) }
    var selectedNotification by remember { mutableStateOf<NotificationItem?>(null) }

    // Persistent GrokBot Team State
    var bots by remember { mutableStateOf<List<BotItem>>(emptyList()) }
    var activeBotId by remember { mutableStateOf("butler") }
    var sessionId by remember { mutableStateOf("") }
    var activePersona by remember { mutableStateOf(prefs.activePersona) }
    var activeModel by remember { mutableStateOf(prefs.activeModel) }
    var enableThinking by remember { mutableStateOf(prefs.enableThinking) }
    var agentSessions by remember { mutableStateOf<List<AgentSessionSummary>>(emptyList()) }
    var showSessionsSheet by remember { mutableStateOf(false) }
    var inputText by remember { mutableStateOf("") }
    var isStreaming by remember { mutableStateOf(false) }
    var activeChatJob by remember { mutableStateOf<Job?>(null) }
    var activeStreamCall by remember { mutableStateOf<Call?>(null) }
    val chatMessages = remember { mutableStateListOf<ChatTurnUi>() }

    // Inbox, Stash & Hub State (Cold-start loads offline cache immediately)
    var notifications by remember { mutableStateOf(prefs.loadCachedNotifications()) }
    var stashItems by remember { mutableStateOf<List<StashItemUi>>(emptyList()) }
    var stashQuery by remember { mutableStateOf("") }
    var stashFilter by remember { mutableStateOf("unread") }
    var stashLoading by remember { mutableStateOf(false) }
    var dashboard by remember { mutableStateOf<DashboardSnapshot?>(null) }
    var isRefreshing by remember { mutableStateOf(false) }

    fun refreshStash(queryOverride: String = stashQuery, filterOverride: String = stashFilter) {
        if (prefs.bearerToken.isEmpty()) return
        scope.launch {
            stashLoading = true
            api.fetchStashItems(query = queryOverride, filter = filterOverride, limit = 80).onSuccess { items ->
                stashItems = items
            }
            stashLoading = false
        }
    }

    fun loadBotTimeline(botId: String) {
        if (isStreaming || prefs.bearerToken.isEmpty()) return
        scope.launch {
            api.fetchBotMessages(botId).onSuccess { msgs ->
                val target = bots.find { it.id == botId }
                if (target != null) {
                    activeModel = target.model
                    enableThinking = target.enableThinking
                }
                chatMessages.clear()
                msgs.forEach { m ->
                    val senderName = bots.find { it.id == m.senderBotId }?.name ?: target?.name ?: botId
                    chatMessages.add(
                        ChatTurnUi(
                            id = "botmsg_${m.id}",
                            role = m.role,
                            persona = senderName,
                            model = target?.model ?: activeModel,
                            reasoning = m.reasoningContent,
                            content = m.content,
                            msgKind = m.msgKind
                        )
                    )
                }
            }
        }
    }

    fun refreshAll() {
        if (prefs.bearerToken.isEmpty()) return
        scope.launch {
            isRefreshing = true
            api.fetchNotifications(afterId = 0L, limit = 50).onSuccess { list ->
                notifications = list
                prefs.saveCachedNotifications(list)
                val maxId = list.maxOfOrNull { it.id } ?: 0L
                if (maxId > prefs.lastEventId) {
                    prefs.lastEventId = maxId
                }
            }
            api.fetchDashboard().onSuccess { snap ->
                dashboard = snap
            }
            api.fetchBots().onSuccess { list ->
                val firstLoad = bots.isEmpty()
                bots = list
                if (firstLoad && list.isNotEmpty() && chatMessages.isEmpty()) {
                    val chosen = list.find { it.id == activeBotId } ?: list.first()
                    activeBotId = chosen.id
                    loadBotTimeline(chosen.id)
                }
            }
            api.fetchAgentSessions().onSuccess { sess ->
                agentSessions = sess
            }
            api.fetchStashItems(query = stashQuery, filter = stashFilter, limit = 80).onSuccess { items ->
                stashItems = items
            }
            isRefreshing = false
        }
    }

    fun loadAgentSession(sess: AgentSessionSummary) {
        if (isStreaming) return
        scope.launch {
            api.fetchAgentSessionMessages(sess.id).onSuccess { msgs ->
                sessionId = sess.id
                activePersona = sess.persona
                activeModel = sess.model
                chatMessages.clear()
                msgs.forEach { m ->
                    chatMessages.add(
                        ChatTurnUi(
                            id = "hist_${m.id}",
                            role = m.role,
                            persona = sess.persona,
                            model = sess.model,
                            reasoning = m.reasoningContent,
                            content = m.content
                        )
                    )
                }
                showSessionsSheet = false
            }.onFailure { err ->
                snackbarHostState.showSnackbar("加载历史会话失败: ${err.message}")
            }
        }
    }

    fun sendAgentPrompt(promptText: String) {
        val clean = promptText.trim()
        if (clean.isEmpty() || isStreaming) return
        if (prefs.bearerToken.isEmpty()) {
            showSettingsSheet = true
            return
        }

        inputText = ""
        isStreaming = true
        chatMessages.add(
            ChatTurnUi(
                id = "u_${System.currentTimeMillis()}",
                role = "user",
                persona = activePersona,
                model = activeModel,
                content = clean
            )
        )
        val assistantIdx = chatMessages.size
        val assistantId = "a_${System.currentTimeMillis()}"
        chatMessages.add(
            ChatTurnUi(
                id = assistantId,
                role = "assistant",
                persona = activePersona,
                model = activeModel,
                content = ""
            )
        )

        activeChatJob = scope.launch {
            api.streamChat(
                sessionId = sessionId,
                botId = activeBotId,
                message = clean,
                persona = activePersona,
                model = activeModel,
                enableThinking = enableThinking,
                onCallCreated = { call -> activeStreamCall = call },
                onSession = { sid -> sessionId = sid },
                onReasoningDelta = { delta ->
                    val cur = chatMessages[assistantIdx]
                    chatMessages[assistantIdx] = cur.copy(reasoning = cur.reasoning + delta)
                },
                onToolCall = { toolName, status ->
                    val cur = chatMessages[assistantIdx]
                    val label = "$toolName ($status)"
                    val updatedTools = cur.tools.filterNot { it.startsWith("$toolName (") } + label
                    chatMessages[assistantIdx] = cur.copy(tools = updatedTools)
                },
                onContentDelta = { delta ->
                    val cur = chatMessages[assistantIdx]
                    chatMessages[assistantIdx] = cur.copy(content = cur.content + delta)
                },
                onError = { err ->
                    val cur = chatMessages[assistantIdx]
                    val fallback = if (cur.content.isEmpty()) "[请求异常] $err" else "${cur.content}\n[中断: $err]"
                    chatMessages[assistantIdx] = cur.copy(content = fallback)
                }
            )
            activeStreamCall = null
            isStreaming = false
            refreshAll()
        }
    }

    fun stopAgentStream() {
        activeStreamCall?.cancel()
        activeChatJob?.cancel()
        activeStreamCall = null
        activeChatJob = null
        isStreaming = false
    }

    LaunchedEffect(Unit) {
        refreshAll()
    }

    // Handle Android system share / deep-chat forwarding intent
    LaunchedEffect(sharedText) {
        if (!sharedText.isNullOrBlank()) {
            selectedTab = 0
            val prompt = "请抓取并深度总结以下分享内容或网页链接的核心要点与技术价值：\n$sharedText"
            onSharedTextConsumed()
            sendAgentPrompt(prompt)
        }
    }

    LaunchedEffect(requestedTab) {
        if (requestedTab != null && requestedTab in 0..3) {
            selectedTab = requestedTab
            onRequestedTabConsumed()
        }
    }

    LaunchedEffect(stashQuery, stashFilter) {
        if (prefs.bearerToken.isEmpty()) return@LaunchedEffect
        if (stashQuery.isNotBlank()) {
            delay(240L)
        }
        refreshStash(stashQuery, stashFilter)
    }

    val unreadCount = notifications.count { it.readAt.isEmpty() }
    val unreadStashCount = stashItems.count { it.status != "archived" && it.readCount == 0 }

    Scaffold(
        modifier = Modifier
            .fillMaxSize()
            .imePadding(),
        snackbarHost = { SnackbarHost(snackbarHostState) },
        topBar = {
            TopAppBar(
                title = {
                    Column {
                        Text(
                            text = "Vesper BotHub",
                            style = MaterialTheme.typography.titleLarge,
                            fontWeight = FontWeight.SemiBold
                        )
                        Text(
                            text = prefs.serverUrl.removePrefix("https://"),
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                },
                actions = {
                    IconButton(
                        onClick = { refreshAll() },
                        modifier = Modifier.sizeIn(minWidth = 48.dp, minHeight = 48.dp)
                    ) {
                        Icon(Icons.Default.Refresh, contentDescription = "刷新状态")
                    }
                    IconButton(
                        onClick = { showSettingsSheet = true },
                        modifier = Modifier.sizeIn(minWidth = 48.dp, minHeight = 48.dp)
                    ) {
                        Icon(Icons.Default.Settings, contentDescription = "连接与推送设置")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.surface
                )
            )
        },
        bottomBar = {
            NavigationBar(
                modifier = Modifier.navigationBarsPadding(),
                containerColor = MaterialTheme.colorScheme.surface
            ) {
                NavigationBarItem(
                    selected = selectedTab == 0,
                    onClick = {
                        haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                        selectedTab = 0
                    },
                    icon = { Icon(Icons.Default.ChatBubbleOutline, contentDescription = "智能中枢对话") },
                    label = { Text("对话") }
                )
                NavigationBarItem(
                    selected = selectedTab == 1,
                    onClick = {
                        haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                        selectedTab = 1
                        refreshAll()
                    },
                    icon = {
                        BadgedBox(
                            badge = {
                                if (unreadCount > 0) {
                                    Badge { Text(unreadCount.toString()) }
                                }
                            }
                        ) {
                            Icon(Icons.Default.Inbox, contentDescription = "原生情报收件箱")
                        }
                    },
                    label = { Text("情报箱") }
                )
                NavigationBarItem(
                    selected = selectedTab == 2,
                    onClick = {
                        haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                        selectedTab = 2
                        refreshStash()
                    },
                    icon = {
                        BadgedBox(
                            badge = {
                                if (unreadStashCount > 0) {
                                    Badge { Text(unreadStashCount.toString()) }
                                }
                            }
                        ) {
                            Icon(Icons.Default.BookmarkBorder, contentDescription = "智能收藏与闪念匣")
                        }
                    },
                    label = { Text("收藏匣") }
                )
                NavigationBarItem(
                    selected = selectedTab == 3,
                    onClick = {
                        haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                        selectedTab = 3
                        refreshAll()
                    },
                    icon = { Icon(Icons.Default.Dns, contentDescription = "资产与调度中枢") },
                    label = { Text("中枢") }
                )
            }
        }
    ) { innerPadding ->
        when (selectedTab) {
            0 -> AgentTabScreen(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(innerPadding),
                activePersona = activePersona,
                activeModel = activeModel,
                enableThinking = enableThinking,
                bots = bots,
                activeBotId = activeBotId,
                onSelectBot = { bid ->
                    activeBotId = bid
                    loadBotTimeline(bid)
                },
                onPersonaChange = {
                    activePersona = it
                    prefs.activePersona = it
                },
                onModelChange = {
                    activeModel = it
                    prefs.activeModel = it
                },
                onThinkingChange = {
                    enableThinking = it
                    prefs.enableThinking = it
                },
                onOpenSessions = {
                    scope.launch {
                        api.fetchAgentSessions().onSuccess { agentSessions = it }
                    }
                    showSessionsSheet = true
                },
                onNewSession = {
                    sessionId = ""
                    chatMessages.clear()
                },
                messages = chatMessages,
                inputText = inputText,
                onInputChange = { inputText = it },
                isStreaming = isStreaming,
                onSend = { sendAgentPrompt(inputText) },
                onStop = {
                    stopAgentStream()
                    scope.launch { snackbarHostState.showSnackbar("已停止生成") }
                },
                onCopyText = { txt ->
                    clipboard.setText(AnnotatedString(txt))
                    scope.launch { snackbarHostState.showSnackbar("已复制全文") }
                },
                onQuickPrompt = { sendAgentPrompt(it) }
            )

            1 -> InboxTabScreen(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(innerPadding),
                notifications = notifications,
                onSelectNotification = { item ->
                    selectedNotification = item
                    if (item.readAt.isEmpty()) {
                        scope.launch {
                            api.markNotificationRead(item.id)
                            refreshAll()
                        }
                    }
                },
                onMarkAllRead = {
                    scope.launch {
                        api.markAllNotificationsRead()
                        refreshAll()
                        snackbarHostState.showSnackbar("全部情报已标记为已读")
                    }
                }
            )

            2 -> StashTabScreen(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(innerPadding),
                items = stashItems,
                query = stashQuery,
                activeFilter = stashFilter,
                loading = stashLoading,
                onQueryChange = { stashQuery = it },
                onFilterChange = { stashFilter = it },
                onCapture = { rawInput, userNote ->
                    scope.launch {
                        api.createStashItem(rawInput, userNote).onSuccess { (_, dedup) ->
                            snackbarHostState.showSnackbar(
                                if (dedup) "该链接已在收藏匣，已合并批注" else "已收入收藏匣，后台正提炼要点"
                            )
                            refreshStash()
                        }.onFailure { err ->
                            snackbarHostState.showSnackbar("收藏失败: ${err.message}")
                        }
                    }
                },
                onOpenUrl = { item ->
                    if (item.url.isNotBlank()) {
                        runCatching {
                            context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(item.url)))
                        }
                    }
                    scope.launch {
                        api.updateStashItem(item.id, markRead = true)
                        refreshStash()
                    }
                },
                onTogglePin = { item ->
                    scope.launch {
                        api.updateStashItem(item.id, pinned = !item.pinned).onSuccess {
                            refreshStash()
                        }
                    }
                },
                onReprocess = { item ->
                    scope.launch {
                        api.reprocessStashItem(item.id).onSuccess {
                            snackbarHostState.showSnackbar("已重新排入 AI 提炼队列")
                            refreshStash()
                        }.onFailure { err ->
                            snackbarHostState.showSnackbar("重提炼失败: ${err.message}")
                        }
                    }
                },
                onDelete = { item ->
                    scope.launch {
                        api.deleteStashItem(item.id).onSuccess {
                            snackbarHostState.showSnackbar("已删除收藏条目")
                            refreshStash()
                        }
                    }
                },
                onAskAi = { item ->
                    selectedTab = 0
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
            )

            3 -> HubTabScreen(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(innerPadding),
                snapshot = dashboard,
                onRunTask = { task ->
                    scope.launch {
                        snackbarHostState.showSnackbar("正在触发: ${task.name}")
                        api.triggerTask(task.id).onSuccess { msg ->
                            snackbarHostState.showSnackbar(msg)
                            refreshAll()
                        }.onFailure { err ->
                            snackbarHostState.showSnackbar("执行失败: ${err.message}")
                        }
                    }
                },
                onToggleTask = { task, enabled ->
                    scope.launch {
                        api.toggleTaskEnabled(task.id, enabled).onSuccess {
                            snackbarHostState.showSnackbar(if (enabled) "已启用: ${task.name}" else "已暂停: ${task.name}")
                            refreshAll()
                        }.onFailure { err ->
                            snackbarHostState.showSnackbar("更新失败: ${err.message}")
                        }
                    }
                },
                onSyncAssets = {
                    scope.launch {
                        snackbarHostState.showSnackbar("正在嗅探同步云端资产...")
                        api.syncAssetWatcher().onSuccess { msg ->
                            snackbarHostState.showSnackbar(msg)
                            refreshAll()
                        }.onFailure { err ->
                            snackbarHostState.showSnackbar("同步失败: ${err.message}")
                        }
                    }
                }
            )
        }
    }

    if (selectedNotification != null) {
        val item = selectedNotification!!
        ModalBottomSheet(
            onDismissRequest = { selectedNotification = null },
            shape = RoundedCornerShape(topStart = 24.dp, topEnd = 24.dp),
            containerColor = MaterialTheme.colorScheme.surface
        ) {
            LazyColumn(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 20.dp, vertical = 12.dp),
                contentPadding = PaddingValues(bottom = 32.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                item {
                    Text(
                        text = item.title,
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Bold
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = "模块: ${item.moduleId}  |  时间: ${item.createdAt}",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                item {
                    Card(
                        shape = RoundedCornerShape(16.dp),
                        colors = CardDefaults.cardColors(
                            containerColor = MaterialTheme.colorScheme.surfaceVariant
                        ),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Text(
                            text = item.markdownBody,
                            style = MaterialTheme.typography.bodyMedium,
                            modifier = Modifier.padding(16.dp)
                        )
                    }
                }
                item {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        OutlinedButton(
                            onClick = {
                                clipboard.setText(AnnotatedString(item.markdownBody))
                                scope.launch { snackbarHostState.showSnackbar("已复制通知正文") }
                            },
                            modifier = Modifier
                                .weight(1f)
                                .sizeIn(minHeight = 48.dp)
                        ) {
                            Icon(Icons.Default.ContentCopy, contentDescription = null)
                            Text(" 复制全文")
                        }
                        Button(
                            onClick = {
                                selectedNotification = null
                                selectedTab = 0
                                sendAgentPrompt("请帮我分析这条通知并给出排查或处置建议：\n\n标题：${item.title}\n\n${item.markdownBody}")
                            },
                            modifier = Modifier
                                .weight(1f)
                                .sizeIn(minHeight = 48.dp)
                        ) {
                            Text("交给 AI 诊断")
                        }
                    }
                }
            }
        }
    }

    if (showSettingsSheet) {
        var urlInput by remember { mutableStateOf(prefs.serverUrl) }
        var passOrTokenInput by remember { mutableStateOf(prefs.bearerToken) }
        var serviceOn by remember { mutableStateOf(prefs.realtimeServiceEnabled) }
        var zeroClickOn by remember { mutableStateOf(prefs.zeroClickCaptureEnabled) }
        var connecting by remember { mutableStateOf(false) }

        ModalBottomSheet(
            onDismissRequest = { showSettingsSheet = false },
            shape = RoundedCornerShape(topStart = 24.dp, topEnd = 24.dp),
            containerColor = MaterialTheme.colorScheme.surface
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 20.dp, vertical = 12.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Text(
                    text = "中枢服务器与原生推送设置",
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.SemiBold
                )
                OutlinedTextField(
                    value = urlInput,
                    onValueChange = { urlInput = it },
                    label = { Text("BotHub 服务端地址") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
                OutlinedTextField(
                    value = passOrTokenInput,
                    onValueChange = { passOrTokenInput = it },
                    label = { Text("管理密钥 或 App Bearer Token") },
                    visualTransformation = PasswordVisualTransformation(),
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
                ListItem(
                    headlineContent = { Text("常驻 SSE 实时秒推服务") },
                    supportingContent = { Text("开启后锁屏实时直弹通知；关闭则仅使用 WorkManager 周期省电同步") },
                    trailingContent = {
                        Switch(
                            checked = serviceOn,
                            onCheckedChange = { checked ->
                                serviceOn = checked
                                prefs.realtimeServiceEnabled = checked
                                val svcIntent = Intent(context, RealtimePushService::class.java)
                                if (checked) {
                                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                                        context.startForegroundService(svcIntent)
                                    } else {
                                        context.startService(svcIntent)
                                    }
                                } else {
                                    context.stopService(svcIntent)
                                }
                            }
                        )
                    }
                )
                ListItem(
                    headlineContent = { Text("系统分享零点击秒收") },
                    supportingContent = { Text("开启后在任意应用点击系统分享将自动收入收藏匣并立即返回原应用") },
                    trailingContent = {
                        Switch(
                            checked = zeroClickOn,
                            onCheckedChange = { checked ->
                                zeroClickOn = checked
                                prefs.zeroClickCaptureEnabled = checked
                            }
                        )
                    }
                )
                Button(
                    onClick = {
                        scope.launch {
                            connecting = true
                            api.exchangeToken(urlInput, passOrTokenInput).onSuccess {
                                showSettingsSheet = false
                                refreshAll()
                                snackbarHostState.showSnackbar("已连接至 ${prefs.serverUrl}")
                            }.onFailure { err ->
                                snackbarHostState.showSnackbar(err.message ?: "连接失败")
                            }
                            connecting = false
                        }
                    },
                    enabled = !connecting,
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 48.dp)
                ) {
                    Text(if (connecting) "验证并同步中..." else "保存并验证连接")
                }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    OutlinedButton(
                        onClick = {
                            scope.launch {
                                api.testHealth(urlInput).onSuccess { (ms, ver) ->
                                    snackbarHostState.showSnackbar("连通正常 (${ms}ms · $ver)")
                                }.onFailure { err ->
                                    snackbarHostState.showSnackbar("连通测试失败: ${err.message}")
                                }
                            }
                        },
                        modifier = Modifier
                            .weight(1f)
                            .sizeIn(minHeight = 48.dp)
                    ) {
                        Text("测试连通性")
                    }
                    OutlinedButton(
                        onClick = {
                            val batIntent = Intent(
                                Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS,
                                Uri.parse("package:${context.packageName}")
                            )
                            runCatching { context.startActivity(batIntent) }.onFailure {
                                context.startActivity(Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS))
                            }
                        },
                        modifier = Modifier
                            .weight(1f)
                            .sizeIn(minHeight = 48.dp)
                    ) {
                        Text("后台保活白名单")
                    }
                }
                OutlinedButton(
                    onClick = {
                        val apkIntent = Intent(
                            Intent.ACTION_VIEW,
                            Uri.parse("https://github.com/YN1753/BotHub/releases/latest/download/vesper-bothub-android.apk")
                        )
                        context.startActivity(apkIntent)
                    },
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 48.dp)
                ) {
                    Text("拉取云端最新编译 APK 安装包")
                }
                Spacer(modifier = Modifier.height(24.dp))
            }
        }
    }

    if (showSessionsSheet) {
        ModalBottomSheet(
            onDismissRequest = { showSessionsSheet = false },
            shape = RoundedCornerShape(topStart = 24.dp, topEnd = 24.dp),
            containerColor = MaterialTheme.colorScheme.surface
        ) {
            LazyColumn(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 20.dp, vertical = 12.dp),
                contentPadding = PaddingValues(bottom = 32.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                item {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "跨端历史会话 (${agentSessions.size})",
                            style = MaterialTheme.typography.titleLarge,
                            fontWeight = FontWeight.SemiBold
                        )
                        OutlinedButton(
                            onClick = {
                                sessionId = ""
                                chatMessages.clear()
                                showSessionsSheet = false
                            },
                            modifier = Modifier.sizeIn(minHeight = 48.dp)
                        ) {
                            Text("+ 新对话")
                        }
                    }
                }
                if (agentSessions.isEmpty()) {
                    item {
                        Text(
                            text = "暂无历史会话记录",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(vertical = 12.dp)
                        )
                    }
                }
                items(agentSessions, key = { it.id }) { sess ->
                    Card(
                        shape = RoundedCornerShape(14.dp),
                        colors = CardDefaults.cardColors(
                            containerColor = if (sess.id == sessionId) {
                                MaterialTheme.colorScheme.secondaryContainer
                            } else {
                                MaterialTheme.colorScheme.surfaceVariant
                            }
                        ),
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable { loadAgentSession(sess) }
                    ) {
                        ListItem(
                            headlineContent = { Text(sess.title, fontWeight = FontWeight.Medium) },
                            supportingContent = { Text("${sess.persona} · ${sess.model} · ${sess.updatedAt.take(16)}") }
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun AgentTabScreen(
    modifier: Modifier = Modifier,
    activePersona: String,
    activeModel: String,
    enableThinking: Boolean,
    bots: List<BotItem>,
    activeBotId: String,
    onSelectBot: (String) -> Unit,
    onPersonaChange: (String) -> Unit,
    onModelChange: (String) -> Unit,
    onThinkingChange: (Boolean) -> Unit,
    onOpenSessions: () -> Unit,
    onNewSession: () -> Unit,
    messages: List<ChatTurnUi>,
    inputText: String,
    onInputChange: (String) -> Unit,
    isStreaming: Boolean,
    onSend: () -> Unit,
    onStop: () -> Unit,
    onCopyText: (String) -> Unit,
    onQuickPrompt: (String) -> Unit
) {
    Column(modifier = modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
        // Persistent GrokBot Team Roster Strip
        if (bots.isNotEmpty()) {
            LazyRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.fillMaxWidth()
            ) {
                items(bots, key = { it.id }) { b ->
                    val stateSuffix = when (b.state) {
                        "thinking" -> " [思考]"
                        "working" -> " [执行]"
                        "blocked" -> " [告警]"
                        "done" -> if (b.unreadCount > 0) " (${b.unreadCount})" else " [新]"
                        else -> if (b.unreadCount > 0) " (${b.unreadCount})" else ""
                    }
                    FilterChip(
                        selected = activeBotId == b.id,
                        onClick = { onSelectBot(b.id) },
                        label = { Text("${b.name}$stateSuffix") },
                        modifier = Modifier.sizeIn(minHeight = 48.dp)
                    )
                }
            }
            Spacer(modifier = Modifier.height(6.dp))
        }

        // Model & Thinking Toggle Chips
        LazyRow(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth()
        ) {
            val personas = listOf(
                "grok" to "Grok 锐评",
                "butler" to "执更管家",
                "architect" to "架构师"
            )
            items(personas) { (id, label) ->
                FilterChip(
                    selected = activePersona == id,
                    onClick = { onPersonaChange(id) },
                    label = { Text(label) },
                    modifier = Modifier.sizeIn(minHeight = 48.dp)
                )
            }
            val models = listOf(
                "deepseek-flash" to "主力引擎",
                "free-pool" to "免费模型池"
            )
            items(models) { (id, label) ->
                FilterChip(
                    selected = activeModel == id,
                    onClick = { onModelChange(id) },
                    label = { Text(label) },
                    modifier = Modifier.sizeIn(minHeight = 48.dp)
                )
            }
            item {
                FilterChip(
                    selected = enableThinking,
                    onClick = { onThinkingChange(!enableThinking) },
                    label = { Text(if (enableThinking) "深度思考: 开" else "深度思考: 关") },
                    modifier = Modifier.sizeIn(minHeight = 48.dp)
                )
            }
            item {
                AssistChip(
                    onClick = onOpenSessions,
                    label = { Text("历史会话") },
                    modifier = Modifier.sizeIn(minHeight = 48.dp)
                )
            }
            item {
                AssistChip(
                    onClick = onNewSession,
                    label = { Text("+ 新对话") },
                    modifier = Modifier.sizeIn(minHeight = 48.dp)
                )
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        // Quick Action Prompts
        LazyRow(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth()
        ) {
            val prompts = listOf(
                "盘点资产与临期风险" to "请调用工具盘点全部数字资产与算力节点，并按到期风险给出锐评。",
                "探查哪吒节点脉搏" to "请调用 query_nezha_nodes 检查哪吒监控面板节点的实时在线状态。",
                "检查调度任务" to "请列出当前所有定时调度任务的状态与下次触发时间。"
            )
            items(prompts) { (title, prompt) ->
                AssistChip(
                    onClick = { onQuickPrompt(prompt) },
                    label = { Text(title) },
                    modifier = Modifier.sizeIn(minHeight = 48.dp)
                )
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        LazyColumn(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth(),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            contentPadding = PaddingValues(vertical = 8.dp)
        ) {
            if (messages.isEmpty()) {
                item {
                    Card(
                        shape = RoundedCornerShape(16.dp),
                        colors = CardDefaults.cardColors(
                            containerColor = MaterialTheme.colorScheme.surfaceVariant
                        ),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Column(modifier = Modifier.padding(20.dp)) {
                            Text(
                                text = "长庚执更 · DeepSeek 智能中枢",
                                style = MaterialTheme.typography.titleMedium,
                                fontWeight = FontWeight.SemiBold
                            )
                            Spacer(modifier = Modifier.height(6.dp))
                            Text(
                                text = "支持流式对话、哪吒探针直连查询、资产与任务管控，以及从系统浏览器「分享 -> Vesper BotHub」直达总结网页。",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }
            }

            items(messages, key = { it.id }) { msg ->
                val isUser = msg.role == "user"
                var expandReasoning by remember { mutableStateOf(false) }

                Card(
                    shape = RoundedCornerShape(16.dp),
                    colors = CardDefaults.cardColors(
                        containerColor = if (isUser) {
                            MaterialTheme.colorScheme.primaryContainer
                        } else {
                            MaterialTheme.colorScheme.surfaceVariant
                        }
                    ),
                    modifier = Modifier
                        .fillMaxWidth()
                        .animateContentSize()
                ) {
                    Column(
                        modifier = Modifier.padding(14.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        Text(
                            text = if (isUser) "管理员指令" else "DeepSeek (${msg.model})",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )

                        if (msg.reasoning.isNotEmpty()) {
                            Card(
                                shape = RoundedCornerShape(12.dp),
                                colors = CardDefaults.cardColors(
                                    containerColor = MaterialTheme.colorScheme.surface
                                ),
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .clickable { expandReasoning = !expandReasoning }
                            ) {
                                Column(modifier = Modifier.padding(10.dp)) {
                                    Text(
                                        text = if (expandReasoning) "深度思考过程 (点击收起)" else "深度思考过程 (点击展开)",
                                        style = MaterialTheme.typography.labelMedium,
                                        color = MaterialTheme.colorScheme.tertiary
                                    )
                                    if (expandReasoning) {
                                        Spacer(modifier = Modifier.height(6.dp))
                                        Text(
                                            text = msg.reasoning,
                                            style = MaterialTheme.typography.bodySmall,
                                            color = MaterialTheme.colorScheme.onSurfaceVariant
                                        )
                                    }
                                }
                            }
                        }

                        if (msg.tools.isNotEmpty()) {
                            Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                msg.tools.forEach { tLabel ->
                                    Text(
                                        text = "[工具: $tLabel]",
                                        style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.primary
                                    )
                                }
                            }
                        }

                        Text(
                            text = msg.content.ifEmpty { if (isStreaming) "思考与生成中..." else "" },
                            style = MaterialTheme.typography.bodyMedium
                        )

                        if (!isUser && msg.content.isNotEmpty()) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.End
                            ) {
                                OutlinedButton(
                                    onClick = { onCopyText(msg.content) },
                                    modifier = Modifier.sizeIn(minHeight = 48.dp)
                                ) {
                                    Icon(Icons.Default.ContentCopy, contentDescription = "复制")
                                    Text(" 复制全文", style = MaterialTheme.typography.labelMedium)
                                }
                            }
                        }
                    }
                }
            }
        }

        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(top = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            OutlinedTextField(
                value = inputText,
                onValueChange = onInputChange,
                placeholder = { Text("输入指令或粘贴链接...") },
                modifier = Modifier.weight(1f),
                maxLines = 4,
                shape = RoundedCornerShape(16.dp)
            )
            if (isStreaming) {
                Button(
                    onClick = onStop,
                    shape = RoundedCornerShape(16.dp),
                    modifier = Modifier.sizeIn(minWidth = 48.dp, minHeight = 52.dp)
                ) {
                    Icon(Icons.Default.Stop, contentDescription = "停止")
                }
            } else {
                Button(
                    onClick = onSend,
                    enabled = inputText.isNotBlank(),
                    shape = RoundedCornerShape(16.dp),
                    modifier = Modifier.sizeIn(minWidth = 48.dp, minHeight = 52.dp)
                ) {
                    Icon(Icons.Default.Send, contentDescription = "发送")
                }
            }
        }
    }
}

@Composable
private fun InboxTabScreen(
    modifier: Modifier = Modifier,
    notifications: List<NotificationItem>,
    onSelectNotification: (NotificationItem) -> Unit,
    onMarkAllRead: () -> Unit
) {
    Column(modifier = modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text(
                text = "原生推送收件箱 (${notifications.size})",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold
            )
            OutlinedButton(
                onClick = onMarkAllRead,
                modifier = Modifier.sizeIn(minHeight = 48.dp)
            ) {
                Icon(Icons.Default.DoneAll, contentDescription = null)
                Text(text = " 全部已读", style = MaterialTheme.typography.labelMedium)
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            contentPadding = PaddingValues(bottom = 16.dp)
        ) {
            if (notifications.isEmpty()) {
                item {
                    Card(
                        shape = RoundedCornerShape(16.dp),
                        colors = CardDefaults.cardColors(
                            containerColor = MaterialTheme.colorScheme.surfaceVariant
                        ),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Text(
                            text = "收件箱暂无历史情报记录",
                            style = MaterialTheme.typography.bodyMedium,
                            modifier = Modifier.padding(20.dp)
                        )
                    }
                }
            }

            items(notifications, key = { it.id }) { item ->
                val unread = item.readAt.isEmpty()
                Card(
                    shape = RoundedCornerShape(16.dp),
                    colors = CardDefaults.cardColors(
                        containerColor = if (unread) {
                            MaterialTheme.colorScheme.secondaryContainer
                        } else {
                            MaterialTheme.colorScheme.surfaceVariant
                        }
                    ),
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { onSelectNotification(item) }
                ) {
                    ListItem(
                        headlineContent = {
                            Text(
                                text = item.title,
                                style = MaterialTheme.typography.titleMedium,
                                fontWeight = if (unread) FontWeight.Bold else FontWeight.Normal
                            )
                        },
                        supportingContent = {
                            Column {
                                Text(
                                    text = item.summary.ifEmpty { item.markdownBody.take(90) },
                                    style = MaterialTheme.typography.bodyMedium,
                                    maxLines = 2
                                )
                                Spacer(modifier = Modifier.height(4.dp))
                                Text(
                                    text = "${item.moduleId} · ${item.createdAt}",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        }
                    )
                }
            }
        }
    }
}

@Composable
private fun HubTabScreen(
    modifier: Modifier = Modifier,
    snapshot: DashboardSnapshot?,
    onRunTask: (TaskItem) -> Unit,
    onToggleTask: (TaskItem, Boolean) -> Unit,
    onSyncAssets: () -> Unit
) {
    LazyColumn(
        modifier = modifier,
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                MetricMiniCard(
                    modifier = Modifier.weight(1f),
                    label = "数字资产",
                    value = (snapshot?.totalAssets ?: 0).toString()
                )
                MetricMiniCard(
                    modifier = Modifier.weight(1f),
                    label = "7天内临期",
                    value = (snapshot?.expiringCount ?: 0).toString()
                )
                MetricMiniCard(
                    modifier = Modifier.weight(1f),
                    label = "活跃任务",
                    value = (snapshot?.activeTasks ?: 0).toString()
                )
            }
        }

        item {
            Text(
                text = "自动化情报与调度任务",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold
            )
        }

        val tasks = snapshot?.tasks ?: emptyList()
        items(tasks, key = { it.id }) { task ->
            Card(
                shape = RoundedCornerShape(16.dp),
                colors = CardDefaults.cardColors(
                    containerColor = MaterialTheme.colorScheme.surfaceVariant
                ),
                modifier = Modifier.fillMaxWidth()
            ) {
                ListItem(
                    headlineContent = { Text(task.name, fontWeight = FontWeight.SemiBold) },
                    supportingContent = {
                        Text("${task.cronExpr} · 下次: ${task.nextRunAt}\n${task.lastSummary}")
                    },
                    trailingContent = {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            Switch(
                                checked = task.enabled,
                                onCheckedChange = { checked -> onToggleTask(task, checked) }
                            )
                            Button(
                                onClick = { onRunTask(task) },
                                modifier = Modifier.sizeIn(minWidth = 48.dp, minHeight = 48.dp)
                            ) {
                                Icon(Icons.Default.PlayArrow, contentDescription = "触发")
                                Text("触发")
                            }
                        }
                    }
                )
            }
        }

        item {
            Spacer(modifier = Modifier.height(4.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "数字资产与会员守望",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold
                )
                OutlinedButton(
                    onClick = onSyncAssets,
                    modifier = Modifier.sizeIn(minHeight = 48.dp)
                ) {
                    Icon(Icons.Default.Refresh, contentDescription = null)
                    Text(" 嗅探同步", style = MaterialTheme.typography.labelMedium)
                }
            }
        }

        val assets = snapshot?.assets ?: emptyList()
        items(assets, key = { it.id }) { asset ->
            Card(
                shape = RoundedCornerShape(16.dp),
                colors = CardDefaults.cardColors(
                    containerColor = MaterialTheme.colorScheme.surfaceVariant
                ),
                modifier = Modifier.fillMaxWidth()
            ) {
                ListItem(
                    headlineContent = { Text(asset.name, fontWeight = FontWeight.SemiBold) },
                    supportingContent = {
                        Text("${asset.spec} · ${asset.price}\n到期: ${asset.dueDate.take(10)}")
                    },
                    trailingContent = {
                        val daysLabel = if (asset.dueDate.startsWith("2099")) "长期" else "${asset.daysLeft}天"
                        Text(
                            text = daysLabel,
                            style = MaterialTheme.typography.labelLarge,
                            color = if (asset.daysLeft <= 7 && !asset.dueDate.startsWith("2099")) {
                                MaterialTheme.colorScheme.tertiary
                            } else {
                                MaterialTheme.colorScheme.primary
                            }
                        )
                    }
                )
            }
        }
    }
}

@Composable
private fun MetricMiniCard(
    modifier: Modifier = Modifier,
    label: String,
    value: String
) {
    Card(
        modifier = modifier,
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceVariant
        )
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            Text(
                text = label,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = value,
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.primary
            )
        }
    }
}

@Composable
private fun StashTabScreen(
    modifier: Modifier = Modifier,
    items: List<StashItemUi>,
    query: String,
    activeFilter: String,
    loading: Boolean,
    onQueryChange: (String) -> Unit,
    onFilterChange: (String) -> Unit,
    onCapture: (String, String) -> Unit,
    onOpenUrl: (StashItemUi) -> Unit,
    onTogglePin: (StashItemUi) -> Unit,
    onReprocess: (StashItemUi) -> Unit,
    onDelete: (StashItemUi) -> Unit,
    onAskAi: (StashItemUi) -> Unit
) {
    var captureInput by remember { mutableStateOf("") }
    var captureNote by remember { mutableStateOf("") }
    var showCaptureBar by remember { mutableStateOf(false) }

    val filters = listOf(
        "unread" to "未读干货",
        "all" to "全部",
        "pinned" to "置顶",
        "note" to "纯闪念",
        "archived" to "已归档"
    )

    Column(modifier = modifier.padding(horizontal = 16.dp, vertical = 8.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            OutlinedTextField(
                value = query,
                onValueChange = onQueryChange,
                leadingIcon = { Icon(Icons.Default.Search, contentDescription = null) },
                placeholder = { Text("检索收藏标题 / AI 结论 / 正文...") },
                singleLine = true,
                shape = RoundedCornerShape(14.dp),
                modifier = Modifier.weight(1f)
            )
            OutlinedButton(
                onClick = { showCaptureBar = !showCaptureBar },
                shape = RoundedCornerShape(14.dp),
                modifier = Modifier.sizeIn(minHeight = 48.dp)
            ) {
                Text(if (showCaptureBar) "收起" else "+ 秒收")
            }
        }

        if (showCaptureBar) {
            Spacer(modifier = Modifier.height(8.dp))
            Card(
                shape = RoundedCornerShape(16.dp),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
                modifier = Modifier.fillMaxWidth()
            ) {
                Column(
                    modifier = Modifier.padding(12.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    OutlinedTextField(
                        value = captureInput,
                        onValueChange = { captureInput = it },
                        placeholder = { Text("粘贴链接、B站分享文案或输入闪念...") },
                        maxLines = 3,
                        modifier = Modifier.fillMaxWidth()
                    )
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        OutlinedTextField(
                            value = captureNote,
                            onValueChange = { captureNote = it },
                            placeholder = { Text("附注 (可选)") },
                            singleLine = true,
                            modifier = Modifier.weight(1f)
                        )
                        Button(
                            onClick = {
                                val clean = captureInput.trim()
                                if (clean.isNotEmpty()) {
                                    onCapture(clean, captureNote.trim())
                                    captureInput = ""
                                    captureNote = ""
                                    showCaptureBar = false
                                }
                            },
                            enabled = captureInput.isNotBlank(),
                            modifier = Modifier.sizeIn(minHeight = 48.dp)
                        ) {
                            Text("投喂")
                        }
                    }
                }
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        LazyRow(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth()
        ) {
            items(filters) { (fid, label) ->
                FilterChip(
                    selected = activeFilter == fid,
                    onClick = { onFilterChange(fid) },
                    label = { Text(label) },
                    modifier = Modifier.sizeIn(minHeight = 48.dp)
                )
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            contentPadding = PaddingValues(bottom = 16.dp)
        ) {
            if (items.isEmpty()) {
                item {
                    Card(
                        shape = RoundedCornerShape(16.dp),
                        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Text(
                            text = if (loading) {
                                "正在同步收藏与闪念匣..."
                            } else if (query.isNotBlank()) {
                                "未检索到匹配的收藏条目"
                            } else {
                                "当前视图暂无收藏条目。在任意 App 点击「系统分享 -> Vesper BotHub」即可半屏无感秒收。"
                            },
                            style = MaterialTheme.typography.bodyMedium,
                            modifier = Modifier.padding(20.dp)
                        )
                    }
                }
            }

            items(items, key = { it.id }) { item ->
                var expanded by remember { mutableStateOf(false) }
                val sourceBadge = when (item.kind) {
                    "bilibili" -> "Bilibili"
                    "github" -> "GitHub"
                    "note" -> "闪念"
                    else -> "Web"
                }

                Card(
                    shape = RoundedCornerShape(16.dp),
                    colors = CardDefaults.cardColors(
                        containerColor = if (item.readCount == 0 && item.status != "archived") {
                            MaterialTheme.colorScheme.secondaryContainer
                        } else {
                            MaterialTheme.colorScheme.surfaceVariant
                        }
                    ),
                    modifier = Modifier
                        .fillMaxWidth()
                        .animateContentSize()
                        .clickable { expanded = !expanded }
                ) {
                    Column(
                        modifier = Modifier.padding(14.dp),
                        verticalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                text = buildString {
                                    append("[$sourceBadge]")
                                    if (item.pinned) append(" [置顶]")
                                    if (item.status == "processing") append(" [AI 提炼中]")
                                    if (item.domain.isNotEmpty()) append(" · ${item.domain}")
                                },
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.primary
                            )
                            Text(
                                text = item.createdAt.take(16),
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }

                        Text(
                            text = item.title.ifEmpty { item.rawInput.take(72) },
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.SemiBold
                        )

                        if (item.aiTldr.isNotEmpty()) {
                            Text(
                                text = item.aiTldr,
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                        } else if (item.status == "processing") {
                            Text(
                                text = "后台正在抓取网页快照并提炼核心结论...",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        } else if (item.errorMsg.isNotEmpty()) {
                            Text(
                                text = item.errorMsg,
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.tertiary
                            )
                        }

                        if (item.tags.isNotEmpty()) {
                            Text(
                                text = "标签: ${item.tags}",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }

                        if (expanded) {
                            if (item.points.isNotEmpty()) {
                                Spacer(modifier = Modifier.height(2.dp))
                                Text(
                                    text = "AI 干货要点",
                                    style = MaterialTheme.typography.labelMedium,
                                    color = MaterialTheme.colorScheme.tertiary
                                )
                                item.points.forEach { pt ->
                                    Text(
                                        text = "- $pt",
                                        style = MaterialTheme.typography.bodySmall
                                    )
                                }
                            }
                            if (item.userNote.isNotEmpty()) {
                                Spacer(modifier = Modifier.height(2.dp))
                                Text(
                                    text = "个人批注: ${item.userNote}",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }

                            Spacer(modifier = Modifier.height(4.dp))
                            LazyRow(
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                                modifier = Modifier.fillMaxWidth()
                            ) {
                                item {
                                    Button(
                                        onClick = { onOpenUrl(item) },
                                        modifier = Modifier.sizeIn(minHeight = 48.dp)
                                    ) {
                                        if (item.url.isNotEmpty()) {
                                            Icon(Icons.Default.OpenInNew, contentDescription = null)
                                            Text(" 打开原链")
                                        } else {
                                            Text("标为已阅")
                                        }
                                    }
                                }
                                item {
                                    OutlinedButton(
                                        onClick = { onAskAi(item) },
                                        modifier = Modifier.sizeIn(minHeight = 48.dp)
                                    ) {
                                        Text("AI 深聊")
                                    }
                                }
                                item {
                                    OutlinedButton(
                                        onClick = { onTogglePin(item) },
                                        modifier = Modifier.sizeIn(minHeight = 48.dp)
                                    ) {
                                        Text(if (item.pinned) "取消置顶" else "置顶")
                                    }
                                }
                                item {
                                    OutlinedButton(
                                        onClick = { onReprocess(item) },
                                        modifier = Modifier.sizeIn(minHeight = 48.dp)
                                    ) {
                                        Text("重提炼")
                                    }
                                }
                                item {
                                    OutlinedButton(
                                        onClick = { onDelete(item) },
                                        modifier = Modifier.sizeIn(minHeight = 48.dp)
                                    ) {
                                        Icon(Icons.Default.DeleteOutline, contentDescription = "删除")
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
