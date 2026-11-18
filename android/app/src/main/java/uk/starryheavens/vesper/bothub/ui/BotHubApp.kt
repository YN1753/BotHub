package uk.starryheavens.vesper.bothub.ui

import android.content.Intent
import android.net.Uri
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.AnnotatedString
import androidx.lifecycle.viewmodel.compose.viewModel
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.data.BotItem
import uk.starryheavens.vesper.bothub.navigation.BotHubTab
import uk.starryheavens.vesper.bothub.ui.components.BotHubBottomBar
import uk.starryheavens.vesper.bothub.ui.components.BotHubTopBar
import uk.starryheavens.vesper.bothub.ui.components.StarfieldBackdrop
import uk.starryheavens.vesper.bothub.ui.screens.BiometricLockScreen
import uk.starryheavens.vesper.bothub.ui.screens.ChatRoomScreen
import uk.starryheavens.vesper.bothub.ui.screens.ContactsTabScreen
import uk.starryheavens.vesper.bothub.ui.screens.HubTabScreen
import uk.starryheavens.vesper.bothub.ui.screens.ImChatsTabScreen
import uk.starryheavens.vesper.bothub.ui.screens.ServiceFeedScreen
import uk.starryheavens.vesper.bothub.ui.screens.StashTabScreen
import uk.starryheavens.vesper.bothub.ui.sheets.AgentSessionsSheet
import uk.starryheavens.vesper.bothub.ui.sheets.BotProfileSheet
import uk.starryheavens.vesper.bothub.ui.sheets.ChatInfoSheet
import uk.starryheavens.vesper.bothub.ui.sheets.CreateFriendSheet
import uk.starryheavens.vesper.bothub.ui.sheets.CreateGroupSheet
import uk.starryheavens.vesper.bothub.ui.sheets.EditBotSheet
import uk.starryheavens.vesper.bothub.ui.sheets.NotificationDetailSheet
import uk.starryheavens.vesper.bothub.ui.sheets.SettingsPairingSheet

private enum class ImPageLevel {
    ROOT_TABS,
    CHAT_ROOM,
    SERVICE_FEED
}

@Composable
fun BotHubApp(
    prefs: BotHubPrefs,
    api: BotHubApiClient,
    sharedText: String? = null,
    onSharedTextConsumed: () -> Unit = {},
    requestedTab: BotHubTab? = null,
    onRequestedTabConsumed: () -> Unit = {},
    requestedNotificationId: Long? = null,
    onRequestedNotificationIdConsumed: () -> Unit = {},
    pairPayload: Pair<String, String>? = null,
    onPairPayloadConsumed: () -> Unit = {}
) {
    val context = LocalContext.current
    val clipboard = LocalClipboardManager.current
    val snackbarHostState = remember { SnackbarHostState() }
    var showStashCaptureInline by remember { mutableStateOf(false) }
    var showChatInfoSheet by remember { mutableStateOf(false) }
    var editingBot by remember { mutableStateOf<BotItem?>(null) }

    val viewModel: BotHubViewModel = viewModel(
        factory = remember(context, prefs, api) {
            BotHubViewModel.factory(context.applicationContext, prefs, api)
        }
    )

    LaunchedEffect(viewModel) {
        viewModel.uiEvents.collect { msg ->
            snackbarHostState.showSnackbar(msg)
        }
    }

    LaunchedEffect(pairPayload) {
        val payload = pairPayload ?: return@LaunchedEffect
        onPairPayloadConsumed()
        viewModel.handlePairPayload(payload.first, payload.second)
    }

    LaunchedEffect(sharedText) {
        if (!sharedText.isNullOrBlank()) {
            val captured = sharedText
            onSharedTextConsumed()
            viewModel.handleSharedText(captured)
        }
    }

    LaunchedEffect(requestedTab) {
        val tab = requestedTab ?: return@LaunchedEffect
        onRequestedTabConsumed()
        viewModel.selectTab(tab)
    }

    LaunchedEffect(requestedNotificationId) {
        val notifId = requestedNotificationId ?: return@LaunchedEffect
        onRequestedNotificationIdConsumed()
        viewModel.handleOpenNotificationFromTray(notifId)
    }

    if (!viewModel.biometricUnlocked && viewModel.isPaired) {
        BiometricLockScreen(
            snackbarHostState = snackbarHostState,
            onBiometricUnlocked = viewModel::markBiometricUnlocked,
            onPasswordVerify = viewModel::verifyFallbackPassword
        )
        return
    }

    val unreadInbox = viewModel.notifications.count { it.isUnread }
    val unreadBots = viewModel.bots.sumOf { it.unreadCount }
    val totalUnreadMessages = unreadInbox + unreadBots
    val unreadStash = viewModel.stashItems.count { it.isUnread }
    val activeBot = viewModel.bots.find { it.id == viewModel.activeBotId }
    val otherUnreadWhileInChat = unreadInbox + viewModel.bots
        .filter { it.id != viewModel.activeBotId }
        .sumOf { it.unreadCount }

    val pageLevel = when {
        viewModel.inChatRoom -> ImPageLevel.CHAT_ROOM
        viewModel.showServiceFeed -> ImPageLevel.SERVICE_FEED
        else -> ImPageLevel.ROOT_TABS
    }

    Box(modifier = Modifier.fillMaxSize()) {
        StarfieldBackdrop()

        AnimatedContent(
            targetState = pageLevel,
            transitionSpec = {
                val pushing = targetState != ImPageLevel.ROOT_TABS
                if (pushing) {
                    (slideInHorizontally(tween(240)) { it } + fadeIn(tween(180))) togetherWith
                        (slideOutHorizontally(tween(240)) { -it / 4 } + fadeOut(tween(180)))
                } else {
                    (slideInHorizontally(tween(220)) { -it / 4 } + fadeIn(tween(160))) togetherWith
                        (slideOutHorizontally(tween(220)) { it } + fadeOut(tween(160)))
                }
            },
            label = "imRootPushTransition"
        ) { level ->
            when (level) {
                ImPageLevel.CHAT_ROOM -> {
                    Scaffold(
                        modifier = Modifier.fillMaxSize(),
                        containerColor = Color.Transparent,
                        snackbarHost = { SnackbarHost(snackbarHostState) }
                    ) { _ ->
                        ChatRoomScreen(
                            activeBot = activeBot,
                            allBots = viewModel.bots,
                            otherUnreadCount = otherUnreadWhileInChat,
                            activeModel = viewModel.activeModel,
                            enableThinking = viewModel.enableThinking,
                            messages = viewModel.messages,
                            inputText = viewModel.inputText,
                            isStreaming = viewModel.isStreaming,
                            onBack = viewModel::closeChatRoom,
                            onInputChange = viewModel::updateInputText,
                            onSendPrompt = viewModel::sendAgentPrompt,
                            onStop = { viewModel.stopAgentStream(notifyUser = true) },
                            onModelChange = viewModel::updateModel,
                            onThinkingChange = viewModel::updateThinking,
                            onCopyText = { txt ->
                                clipboard.setText(AnnotatedString(txt))
                                viewModel.showMessage("已复制")
                            },
                            onOpenChatInfo = { showChatInfoSheet = true },
                            onOpenBotProfile = viewModel::openBotProfile,
                            onInviteToGroup = {
                                viewModel.openCreateGroup(
                                    sourceBotId = if (activeBot?.isGroup == true) "" else viewModel.activeBotId
                                )
                            },
                            onClearHistory = viewModel::clearActiveChatHistory
                        )
                    }
                }

                ImPageLevel.SERVICE_FEED -> {
                    Scaffold(
                        modifier = Modifier.fillMaxSize(),
                        containerColor = Color.Transparent,
                        snackbarHost = { SnackbarHost(snackbarHostState) }
                    ) { _ ->
                        ServiceFeedScreen(
                            notifications = viewModel.notifications,
                            onBack = viewModel::closeServiceFeed,
                            onSelectNotification = viewModel::selectNotification,
                            onMarkAllRead = viewModel::markAllNotificationsRead,
                            onRefresh = { viewModel.refreshAll(silent = false) }
                        )
                    }
                }

                ImPageLevel.ROOT_TABS -> {
                    Scaffold(
                        modifier = Modifier
                            .fillMaxSize()
                            .imePadding(),
                        containerColor = Color.Transparent,
                        snackbarHost = { SnackbarHost(snackbarHostState) },
                        topBar = {
                            BotHubTopBar(
                                selectedTab = viewModel.selectedTab,
                                isRefreshing = viewModel.isRefreshing,
                                totalUnreadMessages = totalUnreadMessages,
                                showStashCapture = showStashCaptureInline,
                                onCreateFriend = viewModel::openCreateFriend,
                                onCreateGroup = { viewModel.openCreateGroup("") },
                                onQuickCaptureStash = {
                                    viewModel.selectTab(BotHubTab.STASH)
                                    showStashCaptureInline = true
                                },
                                onMarkAllRead = viewModel::markAllNotificationsRead,
                                onToggleStashCapture = { showStashCaptureInline = !showStashCaptureInline },
                                onRefresh = { viewModel.refreshAll(silent = false) },
                                onOpenSettings = viewModel::openSettings
                            )
                        },
                        bottomBar = {
                            BotHubBottomBar(
                                selectedTab = viewModel.selectedTab,
                                unreadMessagesCount = totalUnreadMessages,
                                unreadStashCount = unreadStash,
                                onSelectTab = { tab -> viewModel.selectTab(tab, triggerRefresh = true) }
                            )
                        }
                    ) { innerPadding ->
                        val contentModifier = Modifier
                            .fillMaxSize()
                            .padding(innerPadding)

                        AnimatedContent(
                            targetState = viewModel.selectedTab,
                            transitionSpec = {
                                val forward = targetState.ordinal >= initialState.ordinal
                                val enterOffset = { fullWidth: Int -> if (forward) fullWidth / 14 else -fullWidth / 14 }
                                val exitOffset = { fullWidth: Int -> if (forward) -fullWidth / 14 else fullWidth / 14 }
                                (fadeIn(tween(170)) + slideInHorizontally(tween(190), enterOffset)) togetherWith
                                    (fadeOut(tween(130)) + slideOutHorizontally(tween(170), exitOffset))
                            },
                            label = "imRootTabTransition"
                        ) { currentTab ->
                            when (currentTab) {
                                BotHubTab.AGENT -> ImChatsTabScreen(
                                    modifier = contentModifier,
                                    bots = viewModel.bots,
                                    notifications = viewModel.notifications,
                                    draftsByBot = viewModel.draftsByBot,
                                    activeBotId = viewModel.activeBotId,
                                    isStreaming = viewModel.isStreaming,
                                    searchQuery = viewModel.imSearchQuery,
                                    onSearchQueryChange = viewModel::updateImSearchQuery,
                                    onOpenChat = viewModel::openChatRoom,
                                    onOpenServiceFeed = viewModel::openServiceFeed,
                                    onTogglePinBot = viewModel::togglePinBot,
                                    onMarkBotRead = viewModel::markBotRead,
                                    onClearBotHistory = viewModel::clearBotHistoryById,
                                    onOpenBotProfile = viewModel::openBotProfile,
                                    onCreateFriend = viewModel::openCreateFriend,
                                    onCreateGroup = { viewModel.openCreateGroup("") }
                                )

                                BotHubTab.INBOX -> ContactsTabScreen(
                                    modifier = contentModifier,
                                    bots = viewModel.bots,
                                    unreadInboxCount = unreadInbox,
                                    onOpenProfile = viewModel::openBotProfile,
                                    onOpenChat = viewModel::openChatRoom,
                                    onCreateFriend = viewModel::openCreateFriend,
                                    onCreateGroup = { viewModel.openCreateGroup("") },
                                    onOpenServiceFeed = viewModel::openServiceFeed
                                )

                                BotHubTab.STASH -> StashTabScreen(
                                    modifier = contentModifier,
                                    items = viewModel.stashItems,
                                    query = viewModel.stashQuery,
                                    activeFilter = viewModel.stashFilter,
                                    loading = viewModel.stashLoading,
                                    showCaptureBar = showStashCaptureInline,
                                    onDismissCaptureBar = { showStashCaptureInline = false },
                                    onQueryChange = viewModel::updateStashQuery,
                                    onFilterChange = viewModel::updateStashFilter,
                                    onCapture = viewModel::captureStashItem,
                                    onOpenUrl = { item ->
                                        if (item.url.isNotBlank()) {
                                            runCatching {
                                                context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(item.url)))
                                            }
                                        }
                                        viewModel.markStashItemRead(item)
                                    },
                                    onTogglePin = viewModel::toggleStashPin,
                                    onReprocess = viewModel::reprocessStashItem,
                                    onDelete = viewModel::deleteStashItem,
                                    onAskAi = viewModel::askAiAboutStashItem
                                )

                                BotHubTab.HUB -> HubTabScreen(
                                    modifier = contentModifier,
                                    snapshot = viewModel.dashboard,
                                    onRunTask = viewModel::runTask,
                                    onToggleTask = viewModel::toggleTask,
                                    onSyncAssets = viewModel::syncAssets,
                                    onSaveAsset = viewModel::saveAssetItem,
                                    onDeleteAsset = viewModel::deleteAssetItem
                                )
                            }
                        }
                    }
                }
            }
        }
    }

    viewModel.inspectingBotProfile?.let { profileBot ->
        val freshBot = viewModel.bots.find { it.id == profileBot.id } ?: profileBot
        BotProfileSheet(
            bot = freshBot,
            allBots = viewModel.bots,
            onSendMessage = { viewModel.openChatRoom(freshBot.id) },
            onTogglePin = { viewModel.togglePinBot(freshBot) },
            onStartGroupWithBot = { viewModel.openCreateGroup(freshBot.id) },
            onEditBot = {
                viewModel.dismissBotProfile()
                editingBot = freshBot
            },
            onDeleteBot = { viewModel.deleteFriendOrGroup(freshBot) },
            onDismiss = viewModel::dismissBotProfile
        )
    }

    editingBot?.let { targetBot ->
        val freshBot = viewModel.bots.find { it.id == targetBot.id } ?: targetBot
        EditBotSheet(
            bot = freshBot,
            onSave = { name, roleTitle, avatarStyle, traits, systemPrompt, groupMode, announcement ->
                viewModel.saveBotEdits(
                    botId = freshBot.id,
                    name = name,
                    roleTitle = roleTitle,
                    avatarStyle = avatarStyle,
                    traits = traits,
                    systemPrompt = systemPrompt,
                    groupMode = groupMode,
                    announcement = announcement
                )
                editingBot = null
            },
            onDismiss = { editingBot = null }
        )
    }

    if (showChatInfoSheet) {
        ChatInfoSheet(
            activeBot = activeBot,
            allBots = viewModel.bots,
            activeModel = viewModel.activeModel,
            enableThinking = viewModel.enableThinking,
            onModelChange = viewModel::updateModel,
            onThinkingChange = viewModel::updateThinking,
            onTogglePin = {
                if (activeBot != null) viewModel.togglePinBot(activeBot)
            },
            onInviteToGroup = {
                viewModel.openCreateGroup(
                    sourceBotId = if (activeBot?.isGroup == true) "" else viewModel.activeBotId
                )
            },
            onOpenMemberProfile = viewModel::openBotProfile,
            onEditBot = {
                if (activeBot != null) editingBot = activeBot
            },
            onClearHistory = viewModel::clearActiveChatHistory,
            onDismiss = { showChatInfoSheet = false }
        )
    }

    if (viewModel.showCreateFriendSheet) {
        CreateFriendSheet(
            onSubmit = viewModel::createNewFriend,
            onDismiss = viewModel::dismissCreateFriend
        )
    }

    viewModel.createGroupSourceBotId?.let { srcId ->
        CreateGroupSheet(
            allBots = viewModel.bots,
            sourceBotId = srcId,
            onSubmit = viewModel::createNewGroup,
            onDismiss = viewModel::dismissCreateGroup
        )
    }

    if (viewModel.showSessionsSheet) {
        AgentSessionsSheet(
            sessions = viewModel.agentSessions,
            currentSessionId = viewModel.sessionId,
            onSelectSession = viewModel::loadAgentSession,
            onNewSession = viewModel::startNewSession,
            onDismiss = viewModel::dismissSessions
        )
    }

    viewModel.selectedNotification?.let { item ->
        NotificationDetailSheet(
            item = item,
            onCopyBody = { body ->
                clipboard.setText(AnnotatedString(body))
                viewModel.showMessage("已复制")
            },
            onDiagnoseWithAi = viewModel::diagnoseNotificationWithAi,
            onDismiss = viewModel::dismissNotificationDetail
        )
    }

    if (viewModel.showSettingsSheet) {
        SettingsPairingSheet(
            isPaired = viewModel.isPaired,
            serverUrl = viewModel.serverUrl,
            biometricAppLockEnabled = viewModel.biometricAppLockEnabled,
            realtimeServiceEnabled = viewModel.realtimeServiceEnabled,
            zeroClickCaptureEnabled = viewModel.zeroClickCaptureEnabled,
            api = api,
            onConnected = viewModel::onConnectedSuccessfully,
            onServerUrlSaved = viewModel::updateServerUrl,
            onBiometricLockChanged = viewModel::setBiometricAppLock,
            onRealtimeServiceChanged = viewModel::setRealtimeService,
            onZeroClickChanged = viewModel::setZeroClickCapture,
            onTestHealth = viewModel::testConnectivity,
            onUnbindServer = viewModel::unbindServer,
            onShowMessage = viewModel::showMessage,
            onDismiss = viewModel::dismissSettings
        )
    }
}
