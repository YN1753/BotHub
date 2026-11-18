package uk.starryheavens.vesper.bothub.ui.components

import android.util.LruCache
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.ClickableText
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.PlatformTextStyle
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

private sealed interface MdBlock {
    data class Heading(val level: Int, val text: String) : MdBlock
    data class Paragraph(val text: String) : MdBlock
    data class BulletItem(val isOrdered: Boolean, val marker: String, val text: String) : MdBlock
    data class Quote(val text: String) : MdBlock
    data class CodeFence(val lang: String, val code: String) : MdBlock
    data class Table(val headers: List<String>, val rows: List<List<String>>) : MdBlock
    data object Divider : MdBlock
}

private fun isTableSeparatorLine(trimmed: String): Boolean {
    if (!trimmed.contains('|') || !trimmed.contains('-')) return false
    return trimmed.trim('|', ' ').split('|').all { cell ->
        val c = cell.trim()
        c.isNotEmpty() && c.all { it == '-' || it == ':' }
    }
}

private fun splitTableRow(line: String): List<String> =
    line.trim().trim('|').split('|').map { it.trim() }

private fun parseMarkdownBlocks(raw: String): List<MdBlock> {
    if (raw.isBlank()) return emptyList()
    val lines = raw.replace("\r\n", "\n").split('\n')
    val blocks = ArrayList<MdBlock>()
    var i = 0

    while (i < lines.size) {
        val line = lines[i]
        val trimmed = line.trim()

        if (trimmed.isEmpty()) {
            i++
            continue
        }

        // Fenced code block
        if (trimmed.startsWith("```")) {
            val lang = trimmed.removePrefix("```").trim()
            val codeBuf = StringBuilder()
            i++
            while (i < lines.size && !lines[i].trim().startsWith("```")) {
                if (codeBuf.isNotEmpty()) codeBuf.append('\n')
                codeBuf.append(lines[i])
                i++
            }
            if (i < lines.size) i++
            blocks.add(MdBlock.CodeFence(lang, codeBuf.toString()))
            continue
        }

        // Markdown table (Header | ... \n | --- | --- |)
        if (trimmed.contains('|') && i + 1 < lines.size && isTableSeparatorLine(lines[i + 1].trim())) {
            val headers = splitTableRow(trimmed)
            i += 2
            val rows = ArrayList<List<String>>()
            while (i < lines.size && lines[i].trim().let { it.isNotEmpty() && it.contains('|') }) {
                rows.add(splitTableRow(lines[i]))
                i++
            }
            if (headers.isNotEmpty()) {
                blocks.add(MdBlock.Table(headers, rows))
                continue
            }
        }

        // Horizontal rule
        if (trimmed == "---" || trimmed == "***" || trimmed == "___") {
            blocks.add(MdBlock.Divider)
            i++
            continue
        }

        // Heading (# .. ####)
        val headingMatch = HEADING_REGEX.matchEntire(trimmed)
        if (headingMatch != null) {
            val lvl = headingMatch.groupValues[1].length.coerceIn(1, 4)
            blocks.add(MdBlock.Heading(lvl, headingMatch.groupValues[2].trim()))
            i++
            continue
        }

        // Blockquote
        if (trimmed.startsWith(">")) {
            val quoteLines = ArrayList<String>()
            while (i < lines.size && lines[i].trim().startsWith(">")) {
                quoteLines.add(lines[i].trim().removePrefix(">").trimStart())
                i++
            }
            blocks.add(MdBlock.Quote(quoteLines.joinToString("\n")))
            continue
        }

        // Unordered list item
        val ulMatch = UL_REGEX.matchEntire(trimmed)
        if (ulMatch != null) {
            blocks.add(MdBlock.BulletItem(isOrdered = false, marker = "", text = ulMatch.groupValues[1].trim()))
            i++
            continue
        }

        // Ordered list item
        val olMatch = OL_REGEX.matchEntire(trimmed)
        if (olMatch != null) {
            blocks.add(
                MdBlock.BulletItem(
                    isOrdered = true,
                    marker = "${olMatch.groupValues[1]}.",
                    text = olMatch.groupValues[2].trim()
                )
            )
            i++
            continue
        }

        // Paragraph (coalesce consecutive plain lines)
        val paraLines = ArrayList<String>()
        while (i < lines.size) {
            val cur = lines[i]
            val ct = cur.trim()
            if (ct.isEmpty() ||
                ct.startsWith("```") ||
                ct == "---" || ct == "***" ||
                ct.startsWith(">") ||
                (ct.contains('|') && i + 1 < lines.size && isTableSeparatorLine(lines[i + 1].trim())) ||
                HEADING_REGEX.matches(ct) ||
                UL_REGEX.matches(ct) ||
                OL_REGEX.matches(ct)
            ) {
                break
            }
            paraLines.add(ct)
            i++
        }
        if (paraLines.isNotEmpty()) {
            blocks.add(MdBlock.Paragraph(paraLines.joinToString("\n")))
        } else {
            i++
        }
    }
    return blocks
}

private val HEADING_REGEX = Regex("""^(#{1,4})\s+(.+)$""")
private val UL_REGEX = Regex("""^[-*+]\s+(.+)$""")
private val OL_REGEX = Regex("""^(\d+)\.\s+(.+)$""")

private val markdownBlockCache = LruCache<String, List<MdBlock>>(96)
private val inlineAnnotatedCache = LruCache<String, AnnotatedString>(256)

private fun parseMarkdownBlocksCached(raw: String): List<MdBlock> {
    if (raw.isBlank()) return emptyList()
    if (raw.length <= 16_384) {
        synchronized(markdownBlockCache) {
            markdownBlockCache.get(raw)?.let { return it }
        }
    }
    val parsed = parseMarkdownBlocks(raw)
    if (raw.length <= 16_384) {
        synchronized(markdownBlockCache) {
            markdownBlockCache.put(raw, parsed)
        }
    }
    return parsed
}

private fun parseInlineMarkdownCached(
    text: String,
    codeBg: Color,
    codeFg: Color,
    linkColor: Color,
    boldColor: Color
): AnnotatedString {
    val cacheKey = "${boldColor.value}_${linkColor.value}|$text"
    if (text.length <= 4_096) {
        synchronized(inlineAnnotatedCache) {
            inlineAnnotatedCache.get(cacheKey)?.let { return it }
        }
    }
    val built = parseInlineMarkdown(text, codeBg, codeFg, linkColor, boldColor)
    if (text.length <= 4_096) {
        synchronized(inlineAnnotatedCache) {
            inlineAnnotatedCache.put(cacheKey, built)
        }
    }
    return built
}

/**
 * Order matters:
 * 1. Inline code (`...`)
 * 2. Markdown link `[label](url)` — supports single-level balanced parens inside URLs too
 * 3. Bold `**...**` (recursively parses inner links / code, e.g. `**[repo](https://github.com/...)**`)
 * 4. Italic `*...*`
 * 5. Strikethrough `~~...~~`
 * 6. Bare `https://...` URL
 */
private val INLINE_TOKEN_REGEX = Regex(
    """(`[^`\n]+`)|(\[[^\]\n]+\]\(https?://(?:[^\s()]|\([^\s()]*\))+\))|(\*\*(?!\s).+?(?<!\s)\*\*)|(\*[^*\n]+\*)|(~~[^~\n]+~~)|(https?://[^\s<>()）"']+)"""
)

private fun parseInlineMarkdown(
    text: String,
    codeBg: Color,
    codeFg: Color,
    linkColor: Color,
    boldColor: Color
): AnnotatedString = buildAnnotatedString {
    appendInlineTokens(
        text = text,
        codeBg = codeBg,
        codeFg = codeFg,
        linkColor = linkColor,
        boldColor = boldColor,
        depth = 0
    )
}

private fun AnnotatedString.Builder.appendInlineTokens(
    text: String,
    codeBg: Color,
    codeFg: Color,
    linkColor: Color,
    boldColor: Color,
    depth: Int
) {
    if (depth > 3 || text.isEmpty()) {
        append(text)
        return
    }
    var cursor = 0
    for (match in INLINE_TOKEN_REGEX.findAll(text)) {
        if (match.range.first > cursor) {
            append(text.substring(cursor, match.range.first))
        }
        val token = match.value
        when {
            token.startsWith("`") && token.endsWith("`") && token.length >= 2 -> {
                withStyle(
                    SpanStyle(
                        fontFamily = FontFamily.Monospace,
                        fontSize = 12.5.sp,
                        background = codeBg,
                        color = codeFg
                    )
                ) {
                    append(" ${token.substring(1, token.length - 1)} ")
                }
            }
            token.startsWith("[") -> {
                val splitIdx = token.indexOf("](")
                if (splitIdx > 1 && token.endsWith(")")) {
                    val rawLabel = token.substring(1, splitIdx)
                        .removeSurrounding("**")
                        .trim()
                    val url = token.substring(splitIdx + 2, token.length - 1).trim()
                    pushStringAnnotation(tag = "URL", annotation = url)
                    withStyle(
                        SpanStyle(
                            color = linkColor,
                            fontWeight = FontWeight.Medium,
                            textDecoration = TextDecoration.Underline
                        )
                    ) {
                        append(rawLabel)
                    }
                    pop()
                } else {
                    append(token)
                }
            }
            token.startsWith("**") && token.endsWith("**") && token.length >= 4 -> {
                val inner = token.substring(2, token.length - 2)
                withStyle(SpanStyle(fontWeight = FontWeight.SemiBold, color = boldColor)) {
                    appendInlineTokens(inner, codeBg, codeFg, linkColor, boldColor, depth + 1)
                }
            }
            token.startsWith("*") && token.endsWith("*") && token.length >= 2 -> {
                val inner = token.substring(1, token.length - 1)
                withStyle(SpanStyle(fontStyle = FontStyle.Italic)) {
                    appendInlineTokens(inner, codeBg, codeFg, linkColor, boldColor, depth + 1)
                }
            }
            token.startsWith("~~") && token.endsWith("~~") && token.length >= 4 -> {
                withStyle(
                    SpanStyle(
                        textDecoration = TextDecoration.LineThrough,
                        color = Color(0xFF94A3B8)
                    )
                ) {
                    append(token.substring(2, token.length - 2))
                }
            }
            token.startsWith("http://") || token.startsWith("https://") -> {
                pushStringAnnotation(tag = "URL", annotation = token)
                withStyle(
                    SpanStyle(
                        color = linkColor,
                        textDecoration = TextDecoration.Underline
                    )
                ) {
                    append(token)
                }
                pop()
            }
            else -> append(token)
        }
        cursor = match.range.last + 1
    }
    if (cursor < text.length) {
        append(text.substring(cursor))
    }
}

@Composable
fun RichMarkdownText(
    markdown: String,
    modifier: Modifier = Modifier,
    textColor: Color = BotHubThemeTokens.textPrimary
) {
    val theme = BotHubThemeTokens
    val blocks = remember(markdown) { parseMarkdownBlocksCached(markdown) }
    val uriHandler = LocalUriHandler.current
    val clipboard = LocalClipboardManager.current
    val codeBg = theme.codeInlineBg
    val codeFg = if (theme.isLight) theme.accentDeep else theme.accentLight
    val linkColor = theme.accent
    val boldColor = theme.textPrimary

    Column(
        modifier = modifier,
        verticalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        blocks.forEach { block ->
            when (block) {
                is MdBlock.Heading -> {
                    val style = when (block.level) {
                        1 -> MaterialTheme.typography.titleMedium.copy(
                            fontSize = 17.sp,
                            fontWeight = FontWeight.Bold,
                            lineHeight = 24.sp
                        )
                        2 -> MaterialTheme.typography.titleSmall.copy(
                            fontSize = 15.5.sp,
                            fontWeight = FontWeight.Bold,
                            lineHeight = 22.sp
                        )
                        else -> MaterialTheme.typography.labelLarge.copy(
                            fontSize = 14.sp,
                            fontWeight = FontWeight.SemiBold,
                            lineHeight = 20.sp
                        )
                    }
                    val annotated = remember(block.text, theme.id) {
                        parseInlineMarkdownCached(block.text, codeBg, codeFg, linkColor, boldColor)
                    }
                    InlineLinkedText(
                        annotated = annotated,
                        style = style.copy(color = theme.textPrimary),
                        onOpenUrl = { url -> runCatching { uriHandler.openUri(url) } },
                        modifier = Modifier.padding(top = 2.dp)
                    )
                }

                is MdBlock.Paragraph -> {
                    val annotated = remember(block.text, theme.id) {
                        parseInlineMarkdownCached(block.text, codeBg, codeFg, linkColor, boldColor)
                    }
                    InlineLinkedText(
                        annotated = annotated,
                        style = MaterialTheme.typography.bodyMedium.copy(
                            fontSize = 14.5.sp,
                            lineHeight = 22.sp,
                            color = textColor
                        ),
                        onOpenUrl = { url -> runCatching { uriHandler.openUri(url) } }
                    )
                }

                is MdBlock.BulletItem -> {
                    val annotated = remember(block.text, theme.id) {
                        parseInlineMarkdownCached(block.text, codeBg, codeFg, linkColor, boldColor)
                    }
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        verticalAlignment = Alignment.Top
                    ) {
                        if (block.isOrdered) {
                            Text(
                                text = block.marker,
                                style = TextStyle(
                                    fontFamily = FontFamily.Monospace,
                                    fontWeight = FontWeight.SemiBold,
                                    fontSize = 13.sp,
                                    lineHeight = 22.sp,
                                    color = theme.accent,
                                    platformStyle = PlatformTextStyle(includeFontPadding = false)
                                ),
                                modifier = Modifier
                                    .width(24.dp)
                                    .padding(top = 2.dp)
                            )
                        } else {
                            // Geometric dot immune to custom OEM font vertical padding shifts
                            Box(
                                modifier = Modifier
                                    .width(16.dp)
                                    .padding(top = 8.5.dp),
                                contentAlignment = Alignment.TopStart
                            ) {
                                Box(
                                    modifier = Modifier
                                        .size(5.dp)
                                        .background(theme.accent, CircleShape)
                                )
                            }
                        }
                        InlineLinkedText(
                            annotated = annotated,
                            style = MaterialTheme.typography.bodyMedium.copy(
                                fontSize = 14.5.sp,
                                lineHeight = 22.sp,
                                color = textColor
                            ),
                            onOpenUrl = { url -> runCatching { uriHandler.openUri(url) } },
                            modifier = Modifier.weight(1f)
                        )
                    }
                }

                is MdBlock.Quote -> {
                    val annotated = remember(block.text, theme.id) {
                        parseInlineMarkdownCached(block.text, codeBg, codeFg, linkColor, boldColor)
                    }
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(IntrinsicSize.Min)
                            .background(
                                color = theme.accent.copy(alpha = 0.11f),
                                shape = RoundedCornerShape(topEnd = 10.dp, bottomEnd = 10.dp)
                            )
                            .padding(end = 12.dp, top = 8.dp, bottom = 8.dp)
                    ) {
                        Box(
                            modifier = Modifier
                                .width(3.dp)
                                .fillMaxHeight()
                                .background(
                                    color = theme.accent,
                                    shape = RoundedCornerShape(2.dp)
                                )
                        )
                        Spacer(modifier = Modifier.width(10.dp))
                        InlineLinkedText(
                            annotated = annotated,
                            style = MaterialTheme.typography.bodySmall.copy(
                                fontSize = 13.5.sp,
                                lineHeight = 20.sp,
                                color = theme.textSecondary
                            ),
                            onOpenUrl = { url -> runCatching { uriHandler.openUri(url) } },
                            modifier = Modifier.weight(1f)
                        )
                    }
                }

                is MdBlock.CodeFence -> {
                    var copied by remember { mutableStateOf(false) }
                    Surface(
                        shape = RoundedCornerShape(14.dp),
                        color = theme.codeBlockBg,
                        border = BorderStroke(1.dp, theme.divider),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Column(modifier = Modifier.padding(12.dp)) {
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(bottom = 6.dp),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Text(
                                    text = block.lang.ifEmpty { "code" }.uppercase(),
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 10.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = theme.accent
                                )
                                Surface(
                                    onClick = {
                                        clipboard.setText(AnnotatedString(block.code))
                                        copied = true
                                    },
                                    shape = RoundedCornerShape(999.dp),
                                    color = theme.secondaryBtnTop,
                                    border = BorderStroke(0.5.dp, theme.divider)
                                ) {
                                    Row(
                                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp),
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(4.dp)
                                    ) {
                                        Icon(
                                            Icons.Default.ContentCopy,
                                            contentDescription = "复制代码",
                                            tint = theme.textMuted,
                                            modifier = Modifier.size(10.dp)
                                        )
                                        Text(
                                            text = if (copied) "已复制" else "复制",
                                            fontSize = 10.sp,
                                            color = if (copied) theme.accent else theme.textMuted
                                        )
                                    }
                                }
                            }
                            Box(modifier = Modifier.horizontalScroll(rememberScrollState())) {
                                Text(
                                    text = block.code,
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 12.5.sp,
                                    lineHeight = 18.5.sp,
                                    color = theme.codeText
                                )
                            }
                        }
                    }
                }

                is MdBlock.Table -> {
                    Surface(
                        shape = RoundedCornerShape(12.dp),
                        color = theme.codeBlockBg,
                        border = BorderStroke(1.dp, theme.divider),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Column(
                            modifier = Modifier
                                .horizontalScroll(rememberScrollState())
                                .padding(vertical = 6.dp)
                        ) {
                            Row(modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp)) {
                                block.headers.forEach { h ->
                                    Text(
                                        text = h,
                                        fontSize = 12.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        color = theme.accent,
                                        modifier = Modifier
                                            .widthIn(min = 84.dp, max = 200.dp)
                                            .padding(end = 12.dp)
                                    )
                                }
                            }
                            HorizontalDivider(thickness = 0.5.dp, color = theme.divider)
                            block.rows.forEach { row ->
                                Row(modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp)) {
                                    block.headers.indices.forEach { colIdx ->
                                        val cellText = row.getOrNull(colIdx).orEmpty()
                                        val cellAnnotated = remember(cellText, theme.id) {
                                            parseInlineMarkdownCached(cellText, codeBg, codeFg, linkColor, boldColor)
                                        }
                                        InlineLinkedText(
                                            annotated = cellAnnotated,
                                            style = MaterialTheme.typography.bodySmall.copy(
                                                fontSize = 12.sp,
                                                lineHeight = 17.sp,
                                                color = theme.textSecondary
                                            ),
                                            onOpenUrl = { url -> runCatching { uriHandler.openUri(url) } },
                                            modifier = Modifier
                                                .widthIn(min = 84.dp, max = 200.dp)
                                                .padding(end = 12.dp)
                                        )
                                    }
                                }
                            }
                        }
                    }
                }

                MdBlock.Divider -> {
                    HorizontalDivider(
                        modifier = Modifier.padding(vertical = 4.dp),
                        thickness = 1.dp,
                        color = theme.divider
                    )
                }
            }
        }
    }
}

@Suppress("DEPRECATION")
@Composable
private fun InlineLinkedText(
    annotated: AnnotatedString,
    style: TextStyle,
    onOpenUrl: (String) -> Unit,
    modifier: Modifier = Modifier
) {
    val hasUrl = remember(annotated) {
        annotated.getStringAnnotations("URL", 0, annotated.length).isNotEmpty()
    }
    if (!hasUrl) {
        Text(text = annotated, style = style, modifier = modifier)
    } else {
        ClickableText(
            text = annotated,
            style = style,
            modifier = modifier,
            onClick = { offset ->
                annotated.getStringAnnotations("URL", offset, offset)
                    .firstOrNull()
                    ?.let { onOpenUrl(it.item) }
            }
        )
    }
}
