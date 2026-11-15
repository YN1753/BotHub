package uk.starryheavens.vesper.bothub.notify

import android.Manifest
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import uk.starryheavens.vesper.bothub.MainActivity
import uk.starryheavens.vesper.bothub.R
import uk.starryheavens.vesper.bothub.data.NotificationItem

object NotificationHelper {
    const val CHANNEL_ALERTS = "vesper_bothub_alerts"
    const val CHANNEL_DAILY = "vesper_bothub_daily"
    const val CHANNEL_SERVICE = "vesper_bothub_service"

    fun ensureChannels(context: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val manager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager

        val alertsChannel = NotificationChannel(
            CHANNEL_ALERTS,
            "高优基建与资产告警",
            NotificationManager.IMPORTANCE_HIGH
        ).apply {
            description = "哪吒探针告警、VPS 与域名临期防扣款提醒"
            enableVibration(true)
        }

        val dailyChannel = NotificationChannel(
            CHANNEL_DAILY,
            "智能晨报与技术雷达",
            NotificationManager.IMPORTANCE_DEFAULT
        ).apply {
            description = "DeepSeek 晨间参谋简报与全网开源技术雷达"
        }

        val serviceChannel = NotificationChannel(
            CHANNEL_SERVICE,
            "实时推送常驻连接",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "保持与 Vesper BotHub 的低功耗 SSE 实时连接"
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

        val isHigh = item.priority.equals("high", ignoreCase = true) || item.priority.equals("critical", ignoreCase = true)
        val channelId = if (isHigh) CHANNEL_ALERTS else CHANNEL_DAILY

        val intent = Intent(context, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            putExtra("open_tab", 1)
            putExtra("notification_id", item.id)
        }
        val pendingIntent = PendingIntent.getActivity(
            context,
            item.id.toInt(),
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val notification = NotificationCompat.Builder(context, channelId)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(item.title)
            .setContentText(item.summary.ifEmpty { item.markdownBody.take(100) })
            .setStyle(NotificationCompat.BigTextStyle().bigText(item.markdownBody.take(600)))
            .setPriority(if (isHigh) NotificationCompat.PRIORITY_HIGH else NotificationCompat.PRIORITY_DEFAULT)
            .setAutoCancel(true)
            .setContentIntent(pendingIntent)
            .build()

        NotificationManagerCompat.from(context).notify(item.id.toInt(), notification)
    }
}
