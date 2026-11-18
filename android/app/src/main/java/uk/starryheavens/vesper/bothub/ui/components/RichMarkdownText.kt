package uk.starryheavens.vesper.bothub.ui.components

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.ClickableText
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.text.AnnotatedString
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

private sealed interface MdBlock {
    data class Heading(val level: Int, val text: String) : MdBlock
    data class Paragraph(val text: String) : MdBlock
    data class BulletItem(val marker: String, val text: String) : MdBlock
    data class Quote(val text: String) : MdBlock
    data class CodeFence(val lang: String, val code: String) : MdBlock
    data object Divider : MdBlock
}

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
            blocks.add(MdBlock.BulletItem("•", ulMatch.groupValues[1].trim()))
            i++
            continue
        }

        // Ordered list item
        val olMatch = OL_REGEX.matchEntire(trimmed)
        if (olMatch != null) {
            blocks.add(MdBlock.BulletItem("${olMatch.groupValues[1]}.", olMatch.groupValues[2].trim()))
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
private val INLINE_TOKEN_REGEX = Regex(
    """(`[^`\n]+`)|(\*\*[^*\n]+\*\*)|(\*[^*\n]+\*)|(\[[^\]\n]+\]\(https?://[^\s)]+\))"""
)

private fun parseInlineMarkdown(
    text: String,
    codeBg: Color,
    codeFg: Color,
    linkColor: Color,
    boldColor: Color
): AnnotatedString = buildAnnotatedString {
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
            token.startsWith("**") && token.endsWith("**") && token.length >= 4 -> {
                withStyle(SpanStyle(fontWeight = FontWeight.SemiBold, color = boldColor)) {
                    append(token.substring(2, token.length - 2))
                }
            }
            token.startsWith("*") && token.endsWith("*") && token.length >= 2 -> {
                withStyle(SpanStyle(fontStyle = FontStyle.Italic)) {
                    append(token.substring(1, token.length - 1))
                }
            }
            token.startsWith("[") -> {
                val splitIdx = token.indexOf("](")
                if (splitIdx > 1 && token.endsWith(")")) {
                    val label = token.substring(1, splitIdx)
                    val url = token.substring(splitIdx + 2, token.length - 1)
                    pushStringAnnotation(tag = "URL", annotation = url)
                    withStyle(
                        SpanStyle(
                            color = linkColor,
                            fontWeight = FontWeight.Medium,
                            textDecoration = TextDecoration.Underline
                        )
                    ) {
                        append(label)
                    }
                    pop()
                } else {
                    append(token)
                }
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
    textColor: Color = MaterialTheme.colorScheme.onSurface
) {
    val blocks = remember(markdown) { parseMarkdownBlocks(markdown) }
    val uriHandler = LocalUriHandler.current
    val codeBg = Color(0xFF151E38)
    val codeFg = Color(0xFFFFE2BE)
    val linkColor = MaterialTheme.colorScheme.primary
    val boldColor = Color(0xFFFFF8EE)

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
                    val annotated = remember(block.text) {
                        parseInlineMarkdown(block.text, codeBg, codeFg, linkColor, boldColor)
                    }
                    Text(
                        text = annotated,
                        style = style,
                        color = Color(0xFFFFF8EE),
                        modifier = Modifier.padding(top = 2.dp)
                    )
                }

                is MdBlock.Paragraph -> {
                    val annotated = remember(block.text) {
                        parseInlineMarkdown(block.text, codeBg, codeFg, linkColor, boldColor)
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
                    val annotated = remember(block.text) {
                        parseInlineMarkdown(block.text, codeBg, codeFg, linkColor, boldColor)
                    }
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        verticalAlignment = Alignment.Top
                    ) {
                        Text(
                            text = block.marker,
                            style = MaterialTheme.typography.bodyMedium.copy(
                                fontWeight = FontWeight.SemiBold,
                                fontSize = 14.sp,
                                lineHeight = 22.sp
                            ),
                            color = MaterialTheme.colorScheme.primary,
                            modifier = Modifier.width(if (block.marker.length > 1) 24.dp else 16.dp)
                        )
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
                    val annotated = remember(block.text) {
                        parseInlineMarkdown(block.text, codeBg, codeFg, linkColor, boldColor)
                    }
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(
                                color = Color(0x1AF2B880),
                                shape = RoundedCornerShape(topEnd = 10.dp, bottomEnd = 10.dp)
                            )
                            .padding(end = 12.dp, top = 8.dp, bottom = 8.dp)
                    ) {
                        Box(
                            modifier = Modifier
                                .width(3.dp)
                                .height(20.dp)
                                .background(
                                    color = MaterialTheme.colorScheme.primary,
                                    shape = RoundedCornerShape(2.dp)
                                )
                        )
                        Spacer(modifier = Modifier.width(10.dp))
                        Text(
                            text = annotated,
                            style = MaterialTheme.typography.bodySmall.copy(
                                fontSize = 13.5.sp,
                                lineHeight = 20.sp
                            ),
                            color = Color(0xFFD8E0F0)
                        )
                    }
                }

                is MdBlock.CodeFence -> {
                    Surface(
                        shape = RoundedCornerShape(12.dp),
                        color = Color(0xFF050812),
                        border = BorderStroke(1.dp, Color(0x26FFF8EE)),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Column(modifier = Modifier.padding(12.dp)) {
                            if (block.lang.isNotEmpty()) {
                                Text(
                                    text = block.lang.uppercase(),
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 10.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = MaterialTheme.colorScheme.primary,
                                    modifier = Modifier.padding(bottom = 6.dp)
                                )
                            }
                            Box(modifier = Modifier.horizontalScroll(rememberScrollState())) {
                                Text(
                                    text = block.code,
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 12.5.sp,
                                    lineHeight = 18.5.sp,
                                    color = Color(0xFFE2E8F0)
                                )
                            }
                        }
                    }
                }

                MdBlock.Divider -> {
                    HorizontalDivider(
                        modifier = Modifier.padding(vertical = 4.dp),
                        thickness = 1.dp,
                        color = Color(0x1FFFF8EE)
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
