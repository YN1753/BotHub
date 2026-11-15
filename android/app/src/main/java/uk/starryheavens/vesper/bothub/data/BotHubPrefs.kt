package uk.starryheavens.vesper.bothub.data

import android.content.Context
import android.content.SharedPreferences

class BotHubPrefs(context: Context) {
    private val prefs: SharedPreferences =
        context.getSharedPreferences("vesper_bothub_prefs", Context.MODE_PRIVATE)

    var serverUrl: String
        get() = prefs.getString("server_url", "https://bot.starryheavens.uk")?.trimEnd('/') ?: "https://bot.starryheavens.uk"
        set(value) = prefs.edit().putString("server_url", value.trim().trimEnd('/')).apply()

    var bearerToken: String
        get() = prefs.getString("bearer_token", "")?.trim() ?: ""
        set(value) = prefs.edit().putString("bearer_token", value.trim()).apply()

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
}
