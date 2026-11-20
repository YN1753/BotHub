package uk.starryheavens.vesper.bothub.data

object ShareTextParser {
    private val urlRegex = Regex("""https?://[^\s<>"'\]）】》]+""", RegexOption.IGNORE_CASE)
    private val whitespaceRegex = Regex("""\s+""")
    private val douyinPrefixRegex = Regex("""^\s*\d+(?:\.\d+)?\s*(?:复制打开抖音|长按复制此条消息|打开抖音搜索)[^【\n]*?(?:看看)?\s*""")
    private val douyinPromptRegex = Regex("""(?:复制此链接[，,]?\s*打开\s*(?:Dou音|抖音)\s*搜索[，,]?\s*直接观看视频[!！]?|长按复制此条消息[，,]?\s*打开抖音搜索[，,]?\s*查看TA的更多作品[!！]?|复制打开抖音[，,]?\s*看看)""")
    private val douyinTailCodeRegex = Regex("""\s+\d{2}/\d{2}\s+[A-Za-z0-9@._/:+-]+(?:\s+[A-Za-z0-9@._/:+-]+)*\s*$""")
    private val xhsBoilerplateRegex = Regex("""(?:复制本条信息[，,]?\s*打开\s*【?小红书】?\s*App\s*查看精彩内容[!！]?|打开\s*【?小红书】?\s*App\s*查看精彩内容[!！]?|小红书\s*-\s*你的生活指南)""")
    private val trailingUrlPunctuation = charArrayOf(
        ',', '.', ';', ':', '!', '?',
        '，', '。', '；', '：', '！', '？', '、',
        ')', '）', '】', '》'
    )
    private val separatorChars = charArrayOf('-', '|', '—', '·', ':', '：')

    private fun cleanShareCopy(raw: String): String {
        var s = raw.trim()
        if (s.isEmpty()) return ""
        s = douyinPrefixRegex.replace(s, "")
        s = douyinPromptRegex.replace(s, " ")
        s = xhsBoilerplateRegex.replace(s, " ")
        s = douyinTailCodeRegex.replace(s, "")
        s = whitespaceRegex.replace(s, " ")
        return s.trim().trim(*separatorChars)
    }

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
        val prefix = cleanShareCopy(trimmed.substring(0, match.range.first))
        val suffix = cleanShareCopy(trimmed.substring(match.range.last + 1))
        val candidate = prefix.ifEmpty { suffix }.take(96)
        val lower = rawUrl.lowercase()
        val kind = when {
            lower.contains("bilibili.com") || lower.contains("b23.tv") -> ShareKind.BILIBILI
            lower.contains("github.com") -> ShareKind.GITHUB
            lower.contains("xhslink.com") || lower.contains("xiaohongshu.com") -> ShareKind.XIAOHONGSHU
            lower.contains("douyin.com") || lower.contains("iesdouyin.com") -> ShareKind.DOUYIN
            lower.contains("mp.weixin.qq.com") -> ShareKind.WECHAT
            else -> ShareKind.WEB_LINK
        }
        return SharePreviewInfo(
            kind = kind,
            candidateTitle = candidate.ifEmpty { UrlUtils.formatDisplayHost(rawUrl).take(64) },
            extractedUrl = rawUrl
        )
    }
}
