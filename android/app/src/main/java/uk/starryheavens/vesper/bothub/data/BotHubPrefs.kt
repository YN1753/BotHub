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

    var serverUrl: String
        get() = UrlUtils.normalizeServerUrl(prefs.getString("server_url", UrlUtils.DEFAULT_SERVER_URL))
        set(value) = prefs.edit().putString("server_url", UrlUtils.normalizeServerUrl(value)).apply()

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
            val clean = value.trim()
            if (clean.isEmpty()) {
                securePrefs.edit().remove("bearer_token").apply()
            } else {
                securePrefs.edit().putString("bearer_token", clean).apply()
            }
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

    fun advanceLastEventId(candidateId: Long) {
        if (candidateId > lastEventId) {
            lastEventId = candidateId
        }
    }

    fun clearCredentials() {
        bearerToken = ""
    }

    fun saveCachedNotifications(items: List<NotificationItem>) {
        prefs.edit().putString("cached_notifications", JsonMappers.serializeNotifications(items)).apply()
    }

    fun loadCachedNotifications(): List<NotificationItem> {
        val raw = prefs.getString("cached_notifications", "") ?: return emptyList()
        return JsonMappers.deserializeNotifications(raw)
    }
}
