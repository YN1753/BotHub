package uk.starryheavens.vesper.bothub.notify

import android.Manifest
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.PowerManager
import android.provider.Settings
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import uk.starryheavens.vesper.bothub.R
import uk.starryheavens.vesper.bothub.data.NotificationItem
import uk.starryheavens.vesper.bothub.navigation.AppIntents
import uk.starryheavens.vesper.bothub.navigation.BotHubTab

object NotificationHelper {
    const val CHANNEL_ALERTS = "vesper_bothub_alerts"
    const val CHANNEL_BOT_CHAT = "vesper_bothub_bot_chat"
    const val CHANNEL_DAILY = "vesper_bothub_daily"
    const val CHANNEL_SERVICE = "bothub_bg_sync_silent"
    private const val LEGACY_CHANNEL_SERVICE = "vesper_bothub_service"
    private const val DEDUP_WINDOW_MS = 15_000L

    private val dedupLock = Any()
    private val recentNotifContentTimes = LinkedHashMap<String, Long>()

    private val MD_LINK_REGEX = Regex("""\[([^\]\n]+)\]\(https?://[^\s)]+\)""")
    private val MD_HEADING_REGEX = Regex("""(?m)^#{1,6}\s+""")
    private val MD_DECORATION_REGEX = Regex("""(\*\*|__|~~|`+|>)""")
    private val WHITESPACE_COLLAPSE_REGEX = Regex("""\s+""")

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

        val botChatChannel = NotificationChannel(
            CHANNEL_BOT_CHAT,
            "Bot 聊天与例程",
            NotificationManager.IMPORTANCE_HIGH
        ).apply {
            description = "Bot 定时汇报与后台对话回复"
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

        manager.createNotificationChannels(
            listOf(alertsChannel, botChatChannel, dailyChannel, serviceChannel)
        )
    }

    fun hasNotificationPermission(context: Context): Boolean {
        val appContext = context.applicationContext
        val runtimeGranted = Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
            ContextCompat.checkSelfPermission(
                appContext,
                Manifest.permission.POST_NOTIFICATIONS
            ) == PackageManager.PERMISSION_GRANTED
        if (!runtimeGranted) return false
        return runCatching {
            NotificationManagerCompat.from(appContext).areNotificationsEnabled()
        }.getOrDefault(true)
    }

    fun isIgnoringBatteryOptimizations(context: Context): Boolean {
        return runCatching {
            val appContext = context.applicationContext
            val pm = appContext.getSystemService(Context.POWER_SERVICE) as? PowerManager
            pm?.isIgnoringBatteryOptimizations(appContext.packageName) ?: false
        }.getOrDefault(false)
    }

    fun openSystemNotificationSettings(context: Context) {
        val intent = Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).apply {
            putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName)
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        runCatching { context.startActivity(intent) }.onFailure {
            val fallback = Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS).apply {
                data = Uri.parse("package:${context.packageName}")
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            runCatching { context.startActivity(fallback) }
        }
    }

    private fun stripMarkdown(raw: String): String {
        if (raw.isBlank()) return ""
        return raw
            .replace(MD_LINK_REGEX, "$1")
            .replace(MD_HEADING_REGEX, "")
            .replace(MD_DECORATION_REGEX, "")
            .replace(WHITESPACE_COLLAPSE_REGEX, " ")
            .trim()
    }

    private fun normalizedDedupKey(raw: String): String {
        return stripMarkdown(raw).lowercase().take(48)
    }

    private fun checkAndRecordDedup(keys: List<String>): Boolean {
        val validKeys = keys.map { it.trim() }.filter { it.isNotEmpty() }.distinct()
        if (validKeys.isEmpty()) return false
        val now = System.currentTimeMillis()
        synchronized(dedupLock) {
            val iter = recentNotifContentTimes.entries.iterator()
            while (iter.hasNext()) {
                val entry = iter.next()
                if (now - entry.value > DEDUP_WINDOW_MS) {
                    iter.remove()
                }
            }
            for (k in validKeys) {
                val prev = recentNotifContentTimes[k]
                if (prev != null && now - prev <= DEDUP_WINDOW_MS) {
                    return true
                }
            }
            for (k in validKeys) {
                recentNotifContentTimes[k] = now
            }
            while (recentNotifContentTimes.size > 64) {
                val oldestKey = recentNotifContentTimes.keys.firstOrNull() ?: break
                recentNotifContentTimes.remove(oldestKey)
            }
            return false
        }
    }

    fun showBotMessageNotification(
        context: Context,
        botId: String,
        botName: String,
        messageId: Long,
        content: String,
        isRoutine: Boolean
    ) {
        val appContext = context.applicationContext
        ensureChannels(appContext)
        if (!hasNotificationPermission(appContext)) return

        val cleanedBody = stripMarkdown(content)
        if (cleanedBody.isEmpty()) return

        val dedupKey = normalizedDedupKey(cleanedBody)
        if (checkAndRecordDedup(listOf(dedupKey))) return

        val displayName = botName.trim().ifEmpty { "BotHub 助手" }
        val title = if (isRoutine) "$displayName · 定时汇报" else displayName
        val preview = cleanedBody.take(120)
        val expanded = cleanedBody.take(600)

        val notifId = if (messageId > 0L) {
            200000 + (messageId % 100000L).toInt()
        } else {
            200000 + (botId.hashCode() and 0x7FFF)
        }

        val intent = AppIntents.createMainActivityIntent(
            context = appContext,
            openTab = BotHubTab.AGENT,
            botId = botId
        )
        val pendingIntent = PendingIntent.getActivity(
            appContext,
            notifId,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val notification = NotificationCompat.Builder(appContext, CHANNEL_BOT_CHAT)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(title)
            .setContentText(preview)
            .setStyle(NotificationCompat.BigTextStyle().bigText(expanded))
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setCategory(NotificationCompat.CATEGORY_MESSAGE)
            .setAutoCancel(true)
            .setContentIntent(pendingIntent)
            .build()

        runCatching {
            NotificationManagerCompat.from(appContext).notify(notifId, notification)
        }
    }

    fun showNotification(context: Context, item: NotificationItem) {
        val appContext = context.applicationContext
        ensureChannels(appContext)
        if (!hasNotificationPermission(appContext)) return

        val isSystemTest = item.moduleId.equals("system_test", ignoreCase = true)
        if (!isSystemTest) {
            val bodyKey = normalizedDedupKey(item.markdownBody)
            val summaryKey = normalizedDedupKey(item.summary)
            if (checkAndRecordDedup(listOf(bodyKey, summaryKey))) {
                return
            }
        }

        val isHigh = isSystemTest || item.isHighPriority
        val channelId = if (isHigh) CHANNEL_ALERTS else CHANNEL_DAILY

        val notifId = if (isSystemTest) {
            item.id.toInt().takeIf { it > 0 }
                ?: ((System.currentTimeMillis() and 0x7FFFFFFFL).toInt().coerceAtLeast(1))
        } else {
            item.id.toInt().takeIf { it > 0 }
                ?: (100000 + (item.title.hashCode() and 0x7FFF))
        }

        val intent = AppIntents.createMainActivityIntent(
            context = appContext,
            openTab = BotHubTab.INBOX,
            notificationId = item.id
        )
        val pendingIntent = PendingIntent.getActivity(
            appContext,
            notifId,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val cleanedExpanded = stripMarkdown(item.markdownBody.ifBlank { item.summary }).take(600)
        val preview = item.previewText(maxChars = 100).ifBlank { cleanedExpanded.take(100) }

        val notification = NotificationCompat.Builder(appContext, channelId)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(item.title)
            .setContentText(preview)
            .setStyle(
                NotificationCompat.BigTextStyle().bigText(
                    cleanedExpanded.ifBlank { item.markdownBody.take(600) }
                )
            )
            .setPriority(if (isHigh) NotificationCompat.PRIORITY_HIGH else NotificationCompat.PRIORITY_DEFAULT)
            .setAutoCancel(true)
            .setContentIntent(pendingIntent)
            .build()

        runCatching {
            NotificationManagerCompat.from(appContext).notify(notifId, notification)
        }
    }
}
