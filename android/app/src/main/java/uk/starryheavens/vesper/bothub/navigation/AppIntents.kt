package uk.starryheavens.vesper.bothub.navigation

import android.content.Context
import android.content.Intent
import uk.starryheavens.vesper.bothub.MainActivity

object AppIntents {
    const val EXTRA_DEEP_CHAT_TEXT = "extra_deep_chat_text"
    const val EXTRA_OPEN_TAB = "open_tab"
    const val EXTRA_NOTIFICATION_ID = "notification_id"
    const val EXTRA_BOT_ID = "open_bot_id"

    fun createMainActivityIntent(
        context: Context,
        deepChatText: String? = null,
        openTab: BotHubTab? = null,
        notificationId: Long? = null,
        botId: String? = null
    ): Intent {
        return Intent(context, MainActivity::class.java).apply {
            addFlags(
                Intent.FLAG_ACTIVITY_NEW_TASK or
                    Intent.FLAG_ACTIVITY_CLEAR_TOP or
                    Intent.FLAG_ACTIVITY_SINGLE_TOP
            )
            if (!deepChatText.isNullOrBlank()) {
                putExtra(EXTRA_DEEP_CHAT_TEXT, deepChatText.trim())
            }
            if (openTab != null) {
                putExtra(EXTRA_OPEN_TAB, openTab.index)
            }
            if (notificationId != null && notificationId > 0L) {
                putExtra(EXTRA_NOTIFICATION_ID, notificationId)
            }
            if (!botId.isNullOrBlank()) {
                putExtra(EXTRA_BOT_ID, botId.trim())
            }
        }
    }

    fun extractSharedText(intent: Intent?): String {
        if (intent == null || intent.action != Intent.ACTION_SEND) return ""
        val mimeType = intent.type
        if (mimeType != null && !mimeType.startsWith("text/")) return ""
        val subject = intent.getStringExtra(Intent.EXTRA_SUBJECT)?.trim().orEmpty()
        val body = intent.getStringExtra(Intent.EXTRA_TEXT)?.trim().orEmpty()
        if (subject.isNotEmpty() && body.isNotEmpty() && !body.contains(subject)) {
            return "$subject $body"
        }
        return body.ifEmpty { subject }
    }

    fun extractPairPayload(intent: Intent?): Pair<String, String>? {
        val data = intent?.takeIf { it.action == Intent.ACTION_VIEW }?.data ?: return null
        if (data.scheme != "bothub" || data.host != "pair") return null
        val pairUrl = data.getQueryParameter("url")?.trim().orEmpty()
        val pairCode = data.getQueryParameter("code")?.trim().orEmpty()
        return if (pairCode.isNotEmpty()) pairUrl to pairCode else null
    }
}
