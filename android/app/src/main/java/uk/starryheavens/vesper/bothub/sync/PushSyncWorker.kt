package uk.starryheavens.vesper.bothub.sync

import android.content.Context
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.notify.NotificationHelper
import java.util.concurrent.TimeUnit

class PushSyncWorker(
    appContext: Context,
    params: WorkerParameters
) : CoroutineWorker(appContext, params) {

    override suspend fun doWork(): Result {
        val prefs = BotHubPrefs(applicationContext)
        if (prefs.bearerToken.isEmpty()) {
            return Result.success()
        }

        val api = BotHubApiClient(prefs)
        val lastId = prefs.lastEventId

        return api.fetchNotifications(afterId = lastId, limit = 30).fold(
            onSuccess = { items ->
                if (items.isNotEmpty()) {
                    if (lastId == 0L) {
                        val initialUnread = items
                            .filter { it.isUnread }
                            .sortedByDescending { it.id }
                            .take(3)
                            .reversed()
                        for (item in initialUnread) {
                            NotificationHelper.showNotification(applicationContext, item)
                        }
                        for (item in items) {
                            prefs.advanceLastEventId(item.id)
                        }
                    } else {
                        for (item in items) {
                            if (item.isUnread) {
                                NotificationHelper.showNotification(applicationContext, item)
                            }
                            prefs.advanceLastEventId(item.id)
                        }
                    }
                    val existing = prefs.loadCachedNotifications()
                    val byId = existing.associateBy { it.id }.toMutableMap()
                    for (item in items) {
                        byId[item.id] = item
                    }
                    val merged = byId.values.sortedByDescending { it.id }.take(60)
                    prefs.saveCachedNotifications(merged)
                }

                // Warm local snapshot so opening the app cold reflects cross-device edits immediately.
                val prevBotsById = prefs.loadCachedBots().associateBy { it.id }
                api.fetchBootstrap().onSuccess { boot ->
                    if (!RealtimePushService.isAppInForeground && prevBotsById.isNotEmpty()) {
                        for (bot in boot.bots) {
                            if (bot.unreadCount <= 0 || bot.lastSummary.isBlank()) continue
                            val prev = prevBotsById[bot.id] ?: continue
                            val summaryChanged = bot.lastSummary != prev.lastSummary
                            val unreadIncreased = bot.unreadCount > prev.unreadCount
                            if (summaryChanged || unreadIncreased) {
                                NotificationHelper.showBotMessageNotification(
                                    context = applicationContext,
                                    botId = bot.id,
                                    botName = bot.name.ifBlank { "BotHub 助手" },
                                    messageId = 0L,
                                    content = bot.lastSummary,
                                    isRoutine = false
                                )
                            }
                        }
                    }
                    prefs.saveCachedBots(boot.bots)
                    prefs.saveCachedDashboard(boot.dashboard)
                    prefs.saveCachedStash(boot.stashItems)
                    prefs.saveCachedNotifications(boot.notifications)
                    val maxBootNotifId = boot.notifications.maxOfOrNull { it.id } ?: 0L
                    prefs.advanceLastEventId(maxBootNotifId)
                }

                if (prefs.realtimeServiceEnabled && prefs.bearerToken.isNotEmpty()) {
                    RealtimePushService.syncWithPrefs(applicationContext, prefs)
                }

                Result.success()
            },
            onFailure = {
                Result.retry()
            }
        )
    }

    companion object {
        private const val WORK_NAME = "vesper_bothub_periodic_sync"
        private const val IMMEDIATE_WORK_NAME = "vesper_bothub_immediate_sync"

        fun schedulePeriodicSync(context: Context) {
            val constraints = Constraints.Builder()
                .setRequiredNetworkType(NetworkType.CONNECTED)
                .build()

            val request = PeriodicWorkRequestBuilder<PushSyncWorker>(15, TimeUnit.MINUTES)
                .setConstraints(constraints)
                .build()

            WorkManager.getInstance(context.applicationContext).enqueueUniquePeriodicWork(
                WORK_NAME,
                ExistingPeriodicWorkPolicy.KEEP,
                request
            )
        }

        fun enqueueImmediateSync(context: Context) {
            val constraints = Constraints.Builder()
                .setRequiredNetworkType(NetworkType.CONNECTED)
                .build()

            val request = OneTimeWorkRequestBuilder<PushSyncWorker>()
                .setConstraints(constraints)
                .build()

            WorkManager.getInstance(context.applicationContext).enqueueUniqueWork(
                IMMEDIATE_WORK_NAME,
                ExistingWorkPolicy.REPLACE,
                request
            )
        }
    }
}
