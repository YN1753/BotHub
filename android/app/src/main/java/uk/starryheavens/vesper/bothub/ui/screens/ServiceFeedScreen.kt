package uk.starryheavens.vesper.bothub.ui.screens

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ChevronLeft
import androidx.compose.material.icons.filled.DoneAll
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.PlatformTextStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.data.NotificationItem

/**
 * Full-Screen Level-2 Official Service Account Feed ("服务通知").
 * Entered from the pinned "服务通知" row in the IM conversation list or lock-screen push tap.
 */
@Composable
fun ServiceFeedScreen(
    notifications: List<NotificationItem>,
    onBack: () -> Unit,
    onSelectNotification: (NotificationItem) -> Unit,
    onMarkAllRead: () -> Unit,
    onRefresh: () -> Unit,
    modifier: Modifier = Modifier
) {
    BackHandler(onBack = onBack)

    val unreadCount = remember(notifications) { notifications.count { it.isUnread } }
    val noPadStyle = remember { PlatformTextStyle(includeFontPadding = false) }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(Color(0xFF050811))
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(Color(0xF0080C1A))
                .statusBarsPadding()
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 10.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Surface(
                    onClick = onBack,
                    shape = RoundedCornerShape(999.dp),
                    color = Color.Transparent,
                    modifier = Modifier.sizeIn(minHeight = 38.dp)
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 6.dp, vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(2.dp)
                    ) {
                        Icon(
                            Icons.Default.ChevronLeft,
                            contentDescription = "返回消息",
                            tint = Color(0xFFFFF8EE),
                            modifier = Modifier.size(22.dp)
                        )
                        Text(
                            text = "消息",
                            style = TextStyle(
                                fontSize = 14.sp,
                                fontWeight = FontWeight.Medium,
                                color = Color(0xFFFFF8EE),
                                platformStyle = noPadStyle
                            )
                        )
                    }
                }

                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(
                        text = "服务通知",
                        style = TextStyle(
                            fontSize = 16.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = Color(0xFFFFF8EE),
                            platformStyle = noPadStyle
                        )
                    )
                    Text(
                        text = if (unreadCount > 0) "$unreadCount 条未读情报" else "晨报 · 雷达 · 探针告警订阅号",
                        style = TextStyle(
                            fontSize = 10.5.sp,
                            color = Color(0xFF94A3B8),
                            platformStyle = noPadStyle
                        )
                    )
                }

                if (unreadCount > 0) {
                    Surface(
                        onClick = onMarkAllRead,
                        shape = RoundedCornerShape(999.dp),
                        color = Color(0x22F2B880),
                        border = BorderStroke(1.dp, Color(0x5CF2B880)),
                        modifier = Modifier.sizeIn(minHeight = 36.dp)
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 11.dp, vertical = 6.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(4.dp)
                        ) {
                            Icon(
                                Icons.Default.DoneAll,
                                contentDescription = null,
                                tint = Color(0xFFFFE2BE),
                                modifier = Modifier.size(14.dp)
                            )
                            Text(
                                text = "已读",
                                fontSize = 12.sp,
                                fontWeight = FontWeight.Medium,
                                color = Color(0xFFFFE2BE)
                            )
                        }
                    }
                } else {
                    Surface(
                        onClick = onRefresh,
                        shape = CircleShape,
                        color = Color(0xFF10172C),
                        border = BorderStroke(1.dp, Color(0x22FFF8EE)),
                        modifier = Modifier.size(38.dp)
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Icon(
                                Icons.Default.Refresh,
                                contentDescription = "刷新通知",
                                tint = Color(0xFFCBD5E1),
                                modifier = Modifier.size(17.dp)
                            )
                        }
                    }
                }
            }

            HorizontalDivider(thickness = 0.5.dp, color = Color(0x1AFFF8EE))
        }

        InboxTabScreen(
            modifier = Modifier.fillMaxSize(),
            notifications = notifications,
            onSelectNotification = onSelectNotification,
            onMarkAllRead = onMarkAllRead
        )
    }
}
