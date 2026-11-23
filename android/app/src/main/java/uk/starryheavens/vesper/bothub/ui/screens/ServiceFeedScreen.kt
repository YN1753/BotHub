package uk.starryheavens.vesper.bothub.ui.screens

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ChevronLeft
import androidx.compose.material.icons.filled.DeleteSweep
import androidx.compose.material.icons.filled.DoneAll
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.VesperButtonTone
import uk.starryheavens.vesper.bothub.ui.components.VesperDivider
import uk.starryheavens.vesper.bothub.ui.components.VesperIconButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.vesperPressable
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

/**
 * Full-screen Service Notification Feed ("服务通知").
 */
@Composable
fun ServiceFeedScreen(
    notifications: List<NotificationItem>,
    rerollingNotificationId: Long? = null,
    savedTodoIds: Set<Long> = emptySet(),
    savedNoteIds: Set<Long> = emptySet(),
    onBack: () -> Unit,
    onSelectNotification: (NotificationItem) -> Unit,
    onMarkOneRead: (NotificationItem) -> Unit = {},
    onDeleteNotification: (NotificationItem) -> Unit = {},
    onDiagnoseWithAi: (NotificationItem) -> Unit = {},
    onRerollNotification: (NotificationItem) -> Unit = {},
    onConvertToTodo: (NotificationItem) -> Unit = {},
    onSaveToNote: (NotificationItem) -> Unit = {},
    onMarkAllRead: () -> Unit,
    onClearRead: () -> Unit = {},
    onRefresh: () -> Unit,
    modifier: Modifier = Modifier
) {
    BackHandler(onBack = onBack)

    val theme = BotHubThemeTokens
    val unreadCount = remember(notifications) { notifications.count { it.isUnread } }
    val hasReadItems = remember(notifications) { notifications.any { !it.isUnread } }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(theme.bgCanvas)
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(theme.surfaceBar)
                .statusBarsPadding()
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 6.dp, vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Row(
                    modifier = Modifier
                        .sizeIn(minWidth = 48.dp, minHeight = 48.dp)
                        .vesperPressable(pressedScale = 0.94f, onClick = onBack)
                        .padding(horizontal = 6.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(2.dp)
                ) {
                    Icon(
                        imageVector = Icons.Default.ChevronLeft,
                        contentDescription = "返回消息",
                        tint = theme.textPrimary,
                        modifier = Modifier.size(22.dp)
                    )
                    Text(
                        text = "消息",
                        style = TextStyle(
                            fontSize = 14.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textPrimary,
                            platformStyle = NoFontPaddingStyle
                        )
                    )
                }

                Text(
                    text = "服务通知",
                    style = TextStyle(
                        fontSize = 16.sp,
                        fontWeight = FontWeight.Bold,
                        color = theme.textPrimary,
                        platformStyle = NoFontPaddingStyle
                    ),
                    maxLines = 1,
                    softWrap = false
                )

                if (unreadCount > 0) {
                    VesperSecondaryButton(
                        text = "全部已读",
                        leadingIcon = Icons.Default.DoneAll,
                        tone = VesperButtonTone.ACCENT,
                        cornerRadius = 999.dp,
                        minHeight = 36.dp,
                        contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                        onClick = onMarkAllRead,
                        modifier = Modifier.padding(end = 6.dp)
                    )
                } else if (hasReadItems) {
                    VesperSecondaryButton(
                        text = "清理已读",
                        leadingIcon = Icons.Default.DeleteSweep,
                        cornerRadius = 999.dp,
                        minHeight = 36.dp,
                        contentPadding = PaddingValues(horizontal = 12.dp, vertical = 6.dp),
                        onClick = onClearRead,
                        modifier = Modifier.padding(end = 6.dp)
                    )
                } else {
                    VesperIconButton(
                        icon = Icons.Default.Refresh,
                        contentDescription = "刷新通知",
                        onClick = onRefresh
                    )
                }
            }

            VesperDivider()
        }

        InboxTabScreen(
            modifier = Modifier.fillMaxSize(),
            notifications = notifications,
            rerollingNotificationId = rerollingNotificationId,
            savedTodoIds = savedTodoIds,
            savedNoteIds = savedNoteIds,
            onSelectNotification = onSelectNotification,
            onMarkOneRead = onMarkOneRead,
            onDeleteNotification = onDeleteNotification,
            onDiagnoseWithAi = onDiagnoseWithAi,
            onRerollNotification = onRerollNotification,
            onConvertToTodo = onConvertToTodo,
            onSaveToNote = onSaveToNote
        )
    }
}
