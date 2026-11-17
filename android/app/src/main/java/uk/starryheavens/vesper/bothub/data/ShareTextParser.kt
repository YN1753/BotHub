package uk.starryheavens.vesper.bothub.data

object ShareTextParser {
    private val urlRegex = Regex("""https?://[^\s<>"'\]）】》]+""", RegexOption.IGNORE_CASE)
    private val whitespaceRegex = Regex("""\s+""")
    private val trailingUrlPunctuation = charArrayOf(
        ',', '.', ';', ':', '!', '?',
        '，', '。', '；', '：', '！', '？', '、',
        ')', '）', '】', '》'
    )
    private val separatorChars = charArrayOf('-', '|', '—', '·', ':', '：')

    fun parse(rawText: String): SharePreviewInfo {
        val trimmed = rawText.trim()
        val match = urlRegex.find(trimmed)
        if (match == null) {
            val firstLine = trimmed.lineSequence().firstOrNull()?.trim().orEmpty().take(72)
            return SharePreviewInfo(
                kind = ShareKind.PLAIN_NOTE,
                candidateTitle = firstLine.ifEmpty { "随手灵感记录" },
                extractedUrl = ""
            )
        }

        val rawUrl = match.value.trimEnd(*trailingUrlPunctuation)
        val prefix = trimmed.substring(0, match.range.first).trim().trim(*separatorChars)
        val suffix = trimmed.substring(match.range.last + 1).trim().trim(*separatorChars)
        val candidate = prefix.ifEmpty { suffix }.replace(whitespaceRegex, " ").take(96)
        val lower = rawUrl.lowercase()
        val kind = when {
            lower.contains("bilibili.com") || lower.contains("b23.tv") -> ShareKind.BILIBILI
            lower.contains("github.com") -> ShareKind.GITHUB
            else -> ShareKind.WEB_LINK
        }
        return SharePreviewInfo(
            kind = kind,
            candidateTitle = candidate.ifEmpty { UrlUtils.formatDisplayHost(rawUrl).take(64) },
            extractedUrl = rawUrl
        )
    }
}
