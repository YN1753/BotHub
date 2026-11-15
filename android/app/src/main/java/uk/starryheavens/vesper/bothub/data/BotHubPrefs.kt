package uk.starryheavens.vesper.bothub.data

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import org.json.JSONArray
import org.json.JSONObject

class BotHubPrefs(context: Context) {
    private val prefs: SharedPreferences =
        context.getSharedPreferences("vesper_bothub_prefs", Context.MODE_PRIVATE)

    private val securePrefs: SharedPreferences by lazy {
        runCatching {
            val masterKey = MasterKey.Builder(context)
                .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
                .build()
            EncryptedSharedPreferences.create(
                context,
                "vesper_bothub_secure_prefs",
                masterKey,
                EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
            )
        }.getOrElse { prefs }
    }

    var serverUrl: String
        get() = prefs.getString("server_url", "https://bot.starryheavens.uk")?.trimEnd('/') ?: "https://bot.starryheavens.uk"
        set(value) = prefs.edit().putString("server_url", value.trim().trimEnd('/')).apply()

    var bearerToken: String
        get() {
            val sec = securePrefs.getString("bearer_token", "")?.trim().orEmpty()
            if (sec.isNotEmpty()) return sec
            // One-time transparent migration from legacy plaintext SharedPreferences
            val legacy = prefs.getString("bearer_token", "")?.trim().orEmpty()
            if (legacy.isNotEmpty()) {
                securePrefs.edit().putString("bearer_token", legacy).apply()
                prefs.edit().remove("bearer_token").apply()
            }
            return legacy
        }
        set(value) {
            securePrefs.edit().putString("bearer_token", value.trim()).apply()
            prefs.edit().remove("bearer_token").apply()
        }

    var lastEventId: Long
        get() = prefs.getLong("last_event_id", 0L)
        set(value) = prefs.edit().putLong("last_event_id", value).apply()

    var activePersona: String
        get() = prefs.getString("active_persona", "grok") ?: "grok"
        set(value) = prefs.edit().putString("active_persona", value).apply()

    var activeModel: String
        get() = prefs.getString("active_model", "deepseek-flash") ?: "deepseek-flash"
        set(value) = prefs.edit().putString("active_model", value).apply()

    var realtimeServiceEnabled: Boolean
        get() = prefs.getBoolean("realtime_service_enabled", false)
        set(value) = prefs.edit().putBoolean("realtime_service_enabled", value).apply()

    var enableThinking: Boolean
        get() = prefs.getBoolean("enable_thinking", true)
        set(value) = prefs.edit().putBoolean("enable_thinking", value).apply()

    fun saveCachedNotifications(items: List<NotificationItem>) {
        val arr = JSONArray()
        items.take(50).forEach { n ->
            arr.put(
                JSONObject()
                    .put("id", n.id)
                    .put("module_id", n.moduleId)
                    .put("title", n.title)
                    .put("summary", n.summary)
                    .put("markdown_body", n.markdownBody)
                    .put("priority", n.priority)
                    .put("created_at", n.createdAt)
                    .put("read_at", n.readAt)
            )
        }
        prefs.edit().putString("cached_notifications", arr.toString()).apply()
    }

    fun loadCachedNotifications(): List<NotificationItem> {
        val raw = prefs.getString("cached_notifications", "") ?: return emptyList()
        if (raw.isBlank()) return emptyList()
        return runCatching {
            val arr = JSONArray(raw)
            val list = mutableListOf<NotificationItem>()
            for (i in 0 until arr.length()) {
                val o = arr.getJSONObject(i)
                list.add(
                    NotificationItem(
                        id = o.optLong("id"),
                        moduleId = o.optString("module_id", "system"),
                        title = o.optString("title", "情报通知"),
                        summary = o.optString("summary", ""),
                        markdownBody = o.optString("markdown_body", ""),
                        priority = o.optString("priority", "normal"),
                        createdAt = o.optString("created_at", ""),
                        readAt = o.optString("read_at", "")
                    )
                )
            }
            list
        }.getOrDefault(emptyList())
    }
}
