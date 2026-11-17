package uk.starryheavens.vesper.bothub.ui

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.AnnotatedString
import androidx.lifecycle.viewmodel.compose.viewModel
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.navigation.BotHubTab
import uk.starryheavens.vesper.bothub.ui.components.BotHubBottomBar
import uk.starryheavens.vesper.bothub.ui.components.BotHubTopBar
import uk.starryheavens.vesper.bothub.ui.screens.AgentTabScreen
import uk.starryheavens.vesper.bothub.ui.screens.BiometricLockScreen
import uk.starryheavens.vesper.bothub.ui.screens.HubTabScreen
import uk.starryheavens.vesper.bothub.ui.screens.InboxTabScreen
import uk.starryheavens.vesper.bothub.ui.screens.StashTabScreen
import uk.starryheavens.vesper.bothub.ui.sheets.AgentSessionsSheet
import uk.starryheavens.vesper.bothub.ui.sheets.NotificationDetailSheet
import uk.starryheavens.vesper.bothub.ui.sheets.SettingsPairingSheet

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

    Scaffold(
        modifier = Modifier
            .fillMaxSize()
            .imePadding(),
        snackbarHost = { SnackbarHost(snackbarHostState) },
        topBar = {
            BotHubTopBar(
                activeBot = viewModel.bots.find { it.id == viewModel.activeBotId },
                allBots = viewModel.bots,
                isStreaming = viewModel.isStreaming,
                serverUrl = viewModel.serverUrl,
                onRefresh = viewModel::refreshAll,
                onOpenSettings = viewModel::openSettings
            )
        },
        bottomBar = {
            BotHubBottomBar(
                selectedTab = viewModel.selectedTab,
                unreadInboxCount = viewModel.notifications.count { it.isUnread },
                unreadStashCount = viewModel.stashItems.count { it.isUnread },
                onSelectTab = { tab -> viewModel.selectTab(tab, triggerRefresh = true) }
            )
        }
    ) { innerPadding ->
        val contentModifier = Modifier
            .fillMaxSize()
            .padding(innerPadding)

        when (viewModel.selectedTab) {
            BotHubTab.AGENT -> AgentTabScreen(
                modifier = contentModifier,
                activePersona = viewModel.activePersona,
                activeModel = viewModel.activeModel,
                enableThinking = viewModel.enableThinking,
                bots = viewModel.bots,
                activeBotId = viewModel.activeBotId,
                onSelectBot = viewModel::selectBot,
                onPersonaChange = viewModel::updatePersona,
                onModelChange = viewModel::updateModel,
                onThinkingChange = viewModel::updateThinking,
                onOpenSessions = viewModel::openSessions,
                onNewSession = viewModel::startNewSession,
                messages = viewModel.messages,
                inputText = viewModel.inputText,
                onInputChange = viewModel::updateInputText,
                isStreaming = viewModel.isStreaming,
                onSend = { viewModel.sendAgentPrompt(viewModel.inputText) },
                onStop = { viewModel.stopAgentStream(notifyUser = true) },
                onCopyText = { txt ->
                    clipboard.setText(AnnotatedString(txt))
                    viewModel.showMessage("已复制全文")
                },
                onQuickPrompt = viewModel::sendAgentPrompt
            )

            BotHubTab.INBOX -> InboxTabScreen(
                modifier = contentModifier,
                notifications = viewModel.notifications,
                onSelectNotification = viewModel::selectNotification,
                onMarkAllRead = viewModel::markAllNotificationsRead
            )

            BotHubTab.STASH -> StashTabScreen(
                modifier = contentModifier,
                items = viewModel.stashItems,
                query = viewModel.stashQuery,
                activeFilter = viewModel.stashFilter,
                loading = viewModel.stashLoading,
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
                onSyncAssets = viewModel::syncAssets
            )
        }
    }

    viewModel.selectedNotification?.let { item ->
        NotificationDetailSheet(
            item = item,
            onCopyBody = { body ->
                clipboard.setText(AnnotatedString(body))
                viewModel.showMessage("已复制通知正文")
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

    if (viewModel.showSessionsSheet) {
        AgentSessionsSheet(
            sessions = viewModel.agentSessions,
            currentSessionId = viewModel.sessionId,
            onSelectSession = viewModel::loadAgentSession,
            onNewSession = viewModel::startNewSession,
            onDismiss = viewModel::dismissSessions
        )
    }
}
