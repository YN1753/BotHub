package uk.starryheavens.vesper.bothub.data

object UrlUtils {
    const val DEFAULT_SERVER_URL = "https://bot.starryheavens.uk"

    fun normalizeServerUrl(raw: String?, fallback: String = DEFAULT_SERVER_URL): String {
        val cleaned = raw?.trim()?.trimEnd('/').orEmpty()
        if (cleaned.isNotEmpty()) return cleaned
        return fallback.trim().trimEnd('/').ifEmpty { DEFAULT_SERVER_URL }
    }

    fun formatDisplayHost(url: String): String {
        return url.trim()
            .removePrefix("https://")
            .removePrefix("http://")
            .trimEnd('/')
    }
}
