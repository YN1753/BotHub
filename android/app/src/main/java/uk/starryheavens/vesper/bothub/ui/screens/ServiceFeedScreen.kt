package uk.starryheavens.vesper.bothub.ui.screens

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.shape.CircleShape
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.ui.components.NoFontPaddingStyle
import uk.starryheavens.vesper.bothub.ui.components.SpecularHairline
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.specularRimBrush
import uk.starryheavens.vesper.bothub.ui.components.tactileClickable
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

/**
 * Full-Screen Level-2 Official Service Account Feed ("服务通知").
 */
@Composable
fun ServiceFeedScreen(
    notifications: List<NotificationItem>,
    onBack: () -> Unit,
    onSelectNotification: (NotificationItem) -> Unit,
    onMarkOneRead: (NotificationItem) -> Unit = {},
    onDeleteNotification: (NotificationItem) -> Unit = {},
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
            .background(theme.bgBase)
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(
                    Brush.verticalGradient(listOf(theme.topBarTop, theme.topBarBottom))
                )
                .statusBarsPadding()
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 10.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Row(
                    modifier = Modifier
                        .sizeIn(minHeight = 36.dp)
                        .tactileClickable(pressedScale = 0.94f, onClick = onBack)
                        .padding(horizontal = 6.dp, vertical = 6.dp),
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
                    TactileSecondaryButton(
                        text = "已读",
                        leadingIcon = Icons.Default.DoneAll,
                        accent = true,
                        cornerRadius = 999.dp,
                        minHeight = 34.dp,
                        contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                        onClick = onMarkAllRead
                    )
                } else if (hasReadItems) {
                    TactileSecondaryButton(
                        text = "清理已读",
                        leadingIcon = Icons.Default.DeleteSweep,
                        cornerRadius = 999.dp,
                        minHeight = 34.dp,
                        contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                        onClick = onClearRead
                    )
                } else {
                    Box(
                        modifier = Modifier
                            .size(36.dp)
                            .tactileClickable(pressedScale = 0.92f, hapticOnTap = true, onClick = onRefresh)
                            .clip(CircleShape)
                            .background(
                                Brush.verticalGradient(listOf(theme.secondaryBtnTop, theme.secondaryBtnBottom)),
                                CircleShape
                            )
                            .border(
                                BorderStroke(
                                    0.85.dp,
                                    specularRimBrush(theme.rimHighlight, theme.rimMid, theme.rimSubtle)
                                ),
                                CircleShape
                            ),
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = Icons.Default.Refresh,
                            contentDescription = "刷新通知",
                            tint = theme.textSecondary,
                            modifier = Modifier.size(17.dp)
                        )
                    }
                }
            }

            SpecularHairline()
        }

        InboxTabScreen(
            modifier = Modifier.fillMaxSize(),
            notifications = notifications,
            onSelectNotification = onSelectNotification,
            onMarkOneRead = onMarkOneRead,
            onDeleteNotification = onDeleteNotification,
            onMarkAllRead = onMarkAllRead
        )
    }
}
