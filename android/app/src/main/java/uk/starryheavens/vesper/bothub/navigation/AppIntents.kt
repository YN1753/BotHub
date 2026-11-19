package uk.starryheavens.vesper.bothub.navigation

import android.content.Context
import android.content.Intent
import uk.starryheavens.vesper.bothub.MainActivity

object AppIntents {
    const val EXTRA_DEEP_CHAT_TEXT = "extra_deep_chat_text"
    const val EXTRA_DEEP_CHAT_AUTO_SEND = "extra_deep_chat_auto_send"
    const val EXTRA_OPEN_TAB = "open_tab"
    const val EXTRA_NOTIFICATION_ID = "notification_id"
    const val EXTRA_BOT_ID = "open_bot_id"

    private val htmlTagRegex = Regex("<[^>]+>")

    fun createMainActivityIntent(
        context: Context,
        deepChatText: String? = null,
        autoSendDeepChat: Boolean = false,
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
                putExtra(EXTRA_DEEP_CHAT_AUTO_SEND, autoSendDeepChat)
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

    fun extractSharedText(intent: Intent?, context: Context? = null): String {
        if (intent == null) return ""
        val action = intent.action ?: return ""
        if (action == Intent.ACTION_PROCESS_TEXT) {
            return intent.getCharSequenceExtra(Intent.EXTRA_PROCESS_TEXT)?.toString()?.trim().orEmpty()
        }
        if (action != Intent.ACTION_SEND && action != Intent.ACTION_SEND_MULTIPLE) {
            return ""
        }

        val subject = intent.getCharSequenceExtra(Intent.EXTRA_SUBJECT)?.toString()?.trim().orEmpty()
        var body = intent.getCharSequenceExtra(Intent.EXTRA_TEXT)?.toString()?.trim().orEmpty()

        if (body.isEmpty() && action == Intent.ACTION_SEND_MULTIPLE) {
            val list = runCatching { intent.getCharSequenceArrayListExtra(Intent.EXTRA_TEXT) }.getOrNull()
            if (!list.isNullOrEmpty()) {
                body = list.mapNotNull { it?.toString()?.trim() }.filter { it.isNotEmpty() }.joinToString("\n")
            }
        }

        if (body.isEmpty()) {
            val html = intent.getStringExtra(Intent.EXTRA_HTML_TEXT)?.trim().orEmpty()
            if (html.isNotEmpty()) {
                body = html.replace(htmlTagRegex, " ").replace(Regex("\\s+"), " ").trim()
            }
        }

        if (body.isEmpty()) {
            val clip = intent.clipData
            if (clip != null && clip.itemCount > 0) {
                val parts = mutableListOf<String>()
                for (i in 0 until clip.itemCount) {
                    val item = clip.getItemAt(i) ?: continue
                    val text = item.text?.toString()?.trim().orEmpty()
                        .ifEmpty {
                            item.htmlText?.replace(htmlTagRegex, " ")?.replace(Regex("\\s+"), " ")?.trim().orEmpty()
                        }
                        .ifEmpty {
                            if (context != null) {
                                val coerced = runCatching { item.coerceToText(context)?.toString()?.trim().orEmpty() }.getOrDefault("")
                                if (coerced.startsWith("content://") || coerced.startsWith("file://")) "" else coerced
                            } else {
                                ""
                            }
                        }
                    if (text.isNotEmpty()) {
                        parts.add(text)
                    }
                }
                if (parts.isNotEmpty()) {
                    body = parts.distinct().joinToString("\n")
                }
            }
        }

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
