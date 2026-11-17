package uk.starryheavens.vesper.bothub.sync

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import uk.starryheavens.vesper.bothub.data.BotHubPrefs

class BootCompletedReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action == Intent.ACTION_BOOT_COMPLETED) {
            val prefs = BotHubPrefs(context)
            if (prefs.bearerToken.isNotBlank()) {
                PushSyncWorker.schedulePeriodicSync(context)
                if (prefs.realtimeServiceEnabled) {
                    RealtimePushService.start(context)
                }
            }
        }
    }
}
