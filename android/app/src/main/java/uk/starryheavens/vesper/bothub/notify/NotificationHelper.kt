package uk.starryheavens.vesper.bothub.notify

import android.Manifest
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import uk.starryheavens.vesper.bothub.R
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.navigation.AppIntents
import uk.starryheavens.vesper.bothub.navigation.BotHubTab

object NotificationHelper {
    const val CHANNEL_ALERTS = "vesper_bothub_alerts"
    const val CHANNEL_DAILY = "vesper_bothub_daily"
    const val CHANNEL_SERVICE = "bothub_bg_sync_silent"
    private const val LEGACY_CHANNEL_SERVICE = "vesper_bothub_service"

    fun ensureChannels(context: Context) {
        val manager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        runCatching { manager.deleteNotificationChannel(LEGACY_CHANNEL_SERVICE) }

        val alertsChannel = NotificationChannel(
            CHANNEL_ALERTS,
            "重要提醒",
            NotificationManager.IMPORTANCE_HIGH
        ).apply {
            description = "异常告警与到期提醒"
            enableVibration(true)
        }

        val dailyChannel = NotificationChannel(
            CHANNEL_DAILY,
            "日常推送",
            NotificationManager.IMPORTANCE_DEFAULT
        ).apply {
            description = "简报与订阅通知"
        }

        val serviceChannel = NotificationChannel(
            CHANNEL_SERVICE,
            "后台连接",
            NotificationManager.IMPORTANCE_MIN
        ).apply {
            description = "保持后台消息接收"
            setShowBadge(false)
            enableLights(false)
            enableVibration(false)
            setSound(null, null)
        }

        manager.createNotificationChannels(listOf(alertsChannel, dailyChannel, serviceChannel))
    }

    fun showNotification(context: Context, item: NotificationItem) {
        ensureChannels(context)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                return
            }
        }

        val isHigh = item.isHighPriority
        val channelId = if (isHigh) CHANNEL_ALERTS else CHANNEL_DAILY

        val intent = AppIntents.createMainActivityIntent(
            context = context,
            openTab = BotHubTab.INBOX,
            notificationId = item.id
        )
        val pendingIntent = PendingIntent.getActivity(
            context,
            item.id.toInt(),
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val notification = NotificationCompat.Builder(context, channelId)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(item.title)
            .setContentText(item.previewText(maxChars = 100))
            .setStyle(NotificationCompat.BigTextStyle().bigText(item.markdownBody.take(600)))
            .setPriority(if (isHigh) NotificationCompat.PRIORITY_HIGH else NotificationCompat.PRIORITY_DEFAULT)
            .setAutoCancel(true)
            .setContentIntent(pendingIntent)
            .build()

        NotificationManagerCompat.from(context).notify(item.id.toInt(), notification)
    }
}
