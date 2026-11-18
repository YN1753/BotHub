package uk.starryheavens.vesper.bothub.sync

import android.content.Context
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
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
                    for (item in items) {
                        if (lastId > 0L && item.isUnread) {
                            NotificationHelper.showNotification(applicationContext, item)
                        }
                        prefs.advanceLastEventId(item.id)
                    }
                    val existing = prefs.loadCachedNotifications()
                    val byId = existing.associateBy { it.id }.toMutableMap()
                    for (item in items) {
                        byId[item.id] = item
                    }
                    val merged = byId.values.sortedByDescending { it.id }.take(60)
                    prefs.saveCachedNotifications(merged)
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

        fun schedulePeriodicSync(context: Context) {
            val constraints = Constraints.Builder()
                .setRequiredNetworkType(NetworkType.CONNECTED)
                .build()

            val request = PeriodicWorkRequestBuilder<PushSyncWorker>(15, TimeUnit.MINUTES)
                .setConstraints(constraints)
                .build()

            WorkManager.getInstance(context).enqueueUniquePeriodicWork(
                WORK_NAME,
                ExistingPeriodicWorkPolicy.KEEP,
                request
            )
        }
    }
}
