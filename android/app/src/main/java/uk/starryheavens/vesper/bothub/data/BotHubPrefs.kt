package uk.starryheavens.vesper.bothub.data

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

class BotHubPrefs(context: Context) {
    private val appContext: Context = context.applicationContext

    private val prefs: SharedPreferences =
        appContext.getSharedPreferences("vesper_bothub_prefs", Context.MODE_PRIVATE)

    private val securePrefs: SharedPreferences by lazy {
        runCatching {
            val masterKey = MasterKey.Builder(appContext)
                .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
                .build()
            EncryptedSharedPreferences.create(
                appContext,
                "vesper_bothub_secure_prefs",
                masterKey,
                EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
            )
        }.getOrElse { prefs }
    }

    @Volatile
    private var memoryBearerToken: String? = null

    val isLikelyPaired: Boolean
        get() {
            val mem = memoryBearerToken
            if (mem != null) return mem.isNotEmpty()
            if (prefs.contains("has_paired_hint")) {
                return prefs.getBoolean("has_paired_hint", false)
            }
            return bearerToken.isNotEmpty()
        }

    fun warmUpCredentials(): String = bearerToken

    var serverUrl: String
        get() = UrlUtils.normalizeServerUrl(prefs.getString("server_url", UrlUtils.DEFAULT_SERVER_URL))
        set(value) = prefs.edit().putString("server_url", UrlUtils.normalizeServerUrl(value)).apply()

    var bearerToken: String
        get() {
            memoryBearerToken?.let { return it }
            val sec = securePrefs.getString("bearer_token", "")?.trim().orEmpty()
            if (sec.isNotEmpty()) {
                memoryBearerToken = sec
                if (!prefs.getBoolean("has_paired_hint", false)) {
                    prefs.edit().putBoolean("has_paired_hint", true).apply()
                }
                return sec
            }
            // One-time transparent migration from legacy plaintext SharedPreferences
            val legacy = prefs.getString("bearer_token", "")?.trim().orEmpty()
            if (legacy.isNotEmpty()) {
                securePrefs.edit().putString("bearer_token", legacy).apply()
                prefs.edit()
                    .remove("bearer_token")
                    .putBoolean("has_paired_hint", true)
                    .apply()
            } else if (!prefs.contains("has_paired_hint")) {
                prefs.edit().putBoolean("has_paired_hint", false).apply()
            }
            memoryBearerToken = legacy
            return legacy
        }
        set(value) {
            val clean = value.trim()
            memoryBearerToken = clean
            if (clean.isEmpty()) {
                securePrefs.edit().remove("bearer_token").apply()
                prefs.edit()
                    .remove("bearer_token")
                    .putBoolean("has_paired_hint", false)
                    .apply()
            } else {
                securePrefs.edit().putString("bearer_token", clean).apply()
                prefs.edit()
                    .remove("bearer_token")
                    .putBoolean("has_paired_hint", true)
                    .apply()
            }
        }

    var lastEventId: Long
        get() = prefs.getLong("last_event_id", 0L)
        set(value) = prefs.edit().putLong("last_event_id", value).apply()

    var lastNotifiedBotMsgId: Long
        get() = prefs.getLong("last_notified_bot_msg_id", 0L)
        set(value) = prefs.edit().putLong("last_notified_bot_msg_id", value).apply()

    var activePersona: String
        get() = prefs.getString("active_persona", "grok") ?: "grok"
        set(value) = prefs.edit().putString("active_persona", value).apply()

    var activeModel: String
        get() = prefs.getString("active_model", "deepseek-flash") ?: "deepseek-flash"
        set(value) = prefs.edit().putString("active_model", value).apply()

    var realtimeServiceEnabled: Boolean
        get() = prefs.getBoolean("realtime_service_enabled", true)
        set(value) = prefs.edit().putBoolean("realtime_service_enabled", value).apply()

    var biometricAppLockEnabled: Boolean
        get() = prefs.getBoolean("biometric_app_lock_enabled", false)
        set(value) = prefs.edit().putBoolean("biometric_app_lock_enabled", value).apply()

    var enableThinking: Boolean
        get() = prefs.getBoolean("enable_thinking", true)
        set(value) = prefs.edit().putBoolean("enable_thinking", value).apply()

    var zeroClickCaptureEnabled: Boolean
        get() = prefs.getBoolean("zero_click_capture_enabled", false)
        set(value) = prefs.edit().putBoolean("zero_click_capture_enabled", value).apply()

    var appThemeId: String
        get() = prefs.getString("app_theme_id", "obsidian") ?: "obsidian"
        set(value) = prefs.edit().putString("app_theme_id", value.trim().ifEmpty { "obsidian" }).apply()

    fun advanceLastEventId(candidateId: Long) {
        if (candidateId > lastEventId) {
            lastEventId = candidateId
        }
    }

    fun advanceLastNotifiedBotMsgId(candidateId: Long) {
        if (candidateId > lastNotifiedBotMsgId) {
            lastNotifiedBotMsgId = candidateId
        }
    }

    fun clearCredentials() {
        bearerToken = ""
    }

    var lastActiveBotId: String
        get() = prefs.getString("last_active_bot_id", "butler") ?: "butler"
        set(value) = prefs.edit().putString("last_active_bot_id", value.ifBlank { "butler" }).apply()

    fun saveCachedNotifications(items: List<NotificationItem>) {
        prefs.edit().putString("cached_notifications", JsonMappers.serializeNotifications(items)).apply()
    }

    fun loadCachedNotifications(): List<NotificationItem> {
        val raw = prefs.getString("cached_notifications", "") ?: return emptyList()
        return JsonMappers.deserializeNotifications(raw)
    }

    fun saveCachedBots(items: List<BotItem>) {
        prefs.edit().putString("cached_bots_v1", JsonMappers.serializeBots(items)).apply()
    }

    fun loadCachedBots(): List<BotItem> {
        val raw = prefs.getString("cached_bots_v1", "") ?: return emptyList()
        return JsonMappers.deserializeBots(raw)
    }

    fun saveCachedDashboard(snap: DashboardSnapshot) {
        prefs.edit().putString("cached_dashboard_v1", JsonMappers.serializeDashboard(snap)).apply()
    }

    fun loadCachedDashboard(): DashboardSnapshot? {
        val raw = prefs.getString("cached_dashboard_v1", "") ?: return null
        return JsonMappers.deserializeDashboard(raw)
    }

    fun saveCachedStash(items: List<StashItemUi>) {
        prefs.edit().putString("cached_stash_v1", JsonMappers.serializeStashItems(items)).apply()
    }

    fun loadCachedStash(): List<StashItemUi> {
        val raw = prefs.getString("cached_stash_v1", "") ?: return emptyList()
        return JsonMappers.deserializeStashItems(raw)
    }

    fun saveCachedBotMessages(botId: String, items: List<ChatTurnUi>) {
        if (botId.isBlank()) return
        prefs.edit()
            .putString("cached_chat_turns_${botId.trim()}", JsonMappers.serializeChatTurns(items, maxItems = 50))
            .apply()
    }

    fun loadCachedBotMessages(botId: String): List<ChatTurnUi> {
        if (botId.isBlank()) return emptyList()
        val raw = prefs.getString("cached_chat_turns_${botId.trim()}", "") ?: return emptyList()
        return JsonMappers.deserializeChatTurns(raw)
    }

    fun saveBotDrafts(drafts: Map<String, String>) {
        prefs.edit().putString("cached_bot_drafts_v1", JsonMappers.serializeDrafts(drafts)).apply()
    }

    fun loadBotDrafts(): Map<String, String> {
        val raw = prefs.getString("cached_bot_drafts_v1", "") ?: return emptyMap()
        return JsonMappers.deserializeDrafts(raw)
    }
}
