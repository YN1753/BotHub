package uk.starryheavens.vesper.bothub.data

import android.text.Html
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import okhttp3.OkHttpClient
import okhttp3.Request
import org.json.JSONObject
import java.util.concurrent.TimeUnit

object ClientSharePrefetcher {

    private const val DESKTOP_UA =
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
    private const val MOBILE_UA =
        "Mozilla/5.0 (iPhone; CPU iPhone OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Mobile/15E148 Safari/604.1"

    private val client: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(2500, TimeUnit.MILLISECONDS)
        .readTimeout(2500, TimeUnit.MILLISECONDS)
        .callTimeout(3200, TimeUnit.MILLISECONDS)
        .followRedirects(true)
        .followSslRedirects(true)
        .build()

    private val bvidRegex = Regex("""(BV[0-9A-Za-z]{10})""", RegexOption.IGNORE_CASE)
    private val douyinIdRegex = Regex("""(?:/video/|/share/video/|/note/)(\d{10,25})""", RegexOption.IGNORE_CASE)
    private val jsUndefinedRegex = Regex(""":\s*undefined\b""")
    private val metaOgTitle1 = Regex(
        """<meta[^>]+(?:name|property)=["'](?:og:title|twitter:title)["'][^>]+content=["']([^"']{1,400})["']""",
        setOf(RegexOption.IGNORE_CASE, RegexOption.DOT_MATCHES_ALL)
    )
    private val metaOgTitle2 = Regex(
        """<meta[^>]+content=["']([^"']{1,400})["'][^>]+(?:name|property)=["'](?:og:title|twitter:title)["']""",
        setOf(RegexOption.IGNORE_CASE, RegexOption.DOT_MATCHES_ALL)
    )
    private val metaDesc1 = Regex(
        """<meta[^>]+(?:name|property)=["'](?:description|og:description|twitter:description)["'][^>]+content=["']([^"']{1,800})["']""",
        setOf(RegexOption.IGNORE_CASE, RegexOption.DOT_MATCHES_ALL)
    )
    private val metaDesc2 = Regex(
        """<meta[^>]+content=["']([^"']{1,800})["'][^>]+(?:name|property)=["'](?:description|og:description|twitter:description)["']""",
        setOf(RegexOption.IGNORE_CASE, RegexOption.DOT_MATCHES_ALL)
    )
    private val htmlTitleRegex = Regex(
        """<title[^>]*>(.*?)</title>""",
        setOf(RegexOption.IGNORE_CASE, RegexOption.DOT_MATCHES_ALL)
    )
    private val wechatNickRegex = Regex("var\\s+nickname\\s*=\\s*htmlDecode\\(\"([^\"]+)\"\\)|\"nick_name\"\\s*:\\s*\"([^\"]+)\"")

    fun shouldPrefetch(url: String): Boolean {
        val lower = url.trim().lowercase()
        if (!lower.startsWith("http://") && !lower.startsWith("https://")) return false
        return lower.contains("b23.tv") ||
            lower.contains("bilibili.com") ||
            lower.contains("xhslink.com") ||
            lower.contains("xiaohongshu.com") ||
            lower.contains("douyin.com") ||
            lower.contains("iesdouyin.com") ||
            lower.contains("mp.weixin.qq.com")
    }

    suspend fun prefetch(url: String): PrefetchedShareMeta? {
        val cleanUrl = url.trim()
        if (!shouldPrefetch(cleanUrl)) return null
        return withTimeoutOrNull(3400L) {
            withContext(Dispatchers.IO) {
                runCatching {
                    val lower = cleanUrl.lowercase()
                    when {
                        lower.contains("b23.tv") || lower.contains("bilibili.com") -> prefetchBilibili(cleanUrl)
                        lower.contains("xhslink.com") || lower.contains("xiaohongshu.com") -> prefetchXiaohongshu(cleanUrl)
                        lower.contains("douyin.com") || lower.contains("iesdouyin.com") -> prefetchDouyin(cleanUrl)
                        lower.contains("mp.weixin.qq.com") -> prefetchWeChat(cleanUrl)
                        else -> null
                    }
                }.getOrNull()?.takeUnless { it.isEmpty }
            }
        }
    }

    private data class HttpFetchResult(val code: Int, val finalUrl: String, val body: String)

    private fun fetch(url: String, ua: String, referer: String? = null, origin: String? = null): HttpFetchResult? {
        val builder = Request.Builder()
            .url(url)
            .header("User-Agent", ua)
            .header("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
            .header("Accept", "text/html,application/xhtml+xml,application/json,text/plain;q=0.9,*/*;q=0.8")
        if (!referer.isNullOrBlank()) builder.header("Referer", referer)
        if (!origin.isNullOrBlank()) builder.header("Origin", origin)
        return runCatching {
            client.newCall(builder.build()).execute().use { resp ->
                val finalUrl = resp.request.url.toString()
                val raw = resp.body?.byteStream()?.use { stream ->
                    val maxBytes = 1024 * 1024
                    val buf = ByteArray(maxBytes)
                    var offset = 0
                    while (offset < maxBytes) {
                        val read = stream.read(buf, offset, maxBytes - offset)
                        if (read <= 0) break
                        offset += read
                    }
                    String(buf, 0, offset, Charsets.UTF_8)
                }.orEmpty()
                HttpFetchResult(resp.code, finalUrl, raw)
            }
        }.getOrNull()
    }

    private fun prefetchBilibili(targetUrl: String): PrefetchedShareMeta? {
        var resolvedUrl = targetUrl
        var landingHtml = ""
        var bvid = extractBvid(resolvedUrl)
        if (bvid.isEmpty()) {
            val res = fetch(targetUrl, DESKTOP_UA, referer = "https://www.bilibili.com/")
            if (res != null) {
                resolvedUrl = res.finalUrl
                landingHtml = res.body
                bvid = extractBvid(resolvedUrl)
            }
        }
        if (bvid.isEmpty()) return null
        val canonical = "https://www.bilibili.com/video/$bvid"

        val apiRes = fetch(
            url = "https://api.bilibili.com/x/web-interface/view?bvid=$bvid",
            ua = DESKTOP_UA,
            referer = canonical,
            origin = "https://www.bilibili.com"
        )
        if (apiRes != null && apiRes.code in 200..299 && apiRes.body.startsWith("{")) {
            val root = runCatching { JSONObject(apiRes.body) }.getOrNull()
            if (root != null && root.optInt("code", -1) == 0) {
                val data = root.optJSONObject("data")
                if (data != null) {
                    val title = data.optString("title").trim()
                    val desc = data.optString("desc").trim()
                    val tname = data.optString("tname").trim()
                    val upName = data.optJSONObject("owner")?.optString("name")?.trim().orEmpty()
                    val pagesArr = data.optJSONArray("pages")
                    val sb = StringBuilder()
                    if (upName.isNotEmpty()) sb.append("UP主：").append(upName).append('\n')
                    if (tname.isNotEmpty()) sb.append("分区频道：").append(tname).append('\n')
                    if (desc.isNotEmpty() && desc != "-") {
                        sb.append("视频简介：\n").append(desc).append('\n')
                    }
                    if (pagesArr != null && pagesArr.length() > 1) {
                        sb.append("\n分P目录：\n")
                        val limit = minOf(pagesArr.length(), 20)
                        for (i in 0 until limit) {
                            val p = pagesArr.optJSONObject(i) ?: continue
                            val part = p.optString("part").trim()
                            if (part.isNotEmpty()) {
                                sb.append("- P").append(p.optInt("page", i + 1)).append(": ").append(part).append('\n')
                            }
                        }
                    }
                    val displayTitle = if (upName.isNotEmpty() && title.isNotEmpty() && !title.startsWith("【") && !title.contains(upName)) {
                        "[$upName] $title"
                    } else {
                        title
                    }
                    val tags = buildList {
                        add("Bilibili")
                        if (tname.isNotEmpty()) add(tname)
                    }.joinToString(",")
                    if (displayTitle.isNotEmpty() || sb.isNotEmpty()) {
                        return PrefetchedShareMeta(
                            canonicalUrl = canonical,
                            title = displayTitle,
                            excerpt = sb.toString().trim().take(6000),
                            tags = tags
                        )
                    }
                }
            }
        }

        if (landingHtml.isNotEmpty()) {
            val title = extractMetaTitle(landingHtml).removeSuffix("_哔哩哔哩_bilibili").removeSuffix("-哔哩哔哩").trim()
            val desc = extractMetaDesc(landingHtml)
            if (title.isNotEmpty() || desc.isNotEmpty()) {
                return PrefetchedShareMeta(
                    canonicalUrl = canonical,
                    title = title,
                    excerpt = listOf(title, desc).filter { it.isNotEmpty() }.joinToString("\n\n").take(4000),
                    tags = "Bilibili"
                )
            }
        }
        return PrefetchedShareMeta(canonicalUrl = canonical)
    }

    private fun prefetchXiaohongshu(targetUrl: String): PrefetchedShareMeta? {
        val res = fetch(targetUrl, DESKTOP_UA, referer = "https://www.xiaohongshu.com/")
            ?: fetch(targetUrl, MOBILE_UA, referer = "https://www.xiaohongshu.com/")
            ?: return null
        val canonical = res.finalUrl.ifBlank { targetUrl }
        val body = res.body
        if (body.isBlank()) return PrefetchedShareMeta(canonicalUrl = canonical)

        var title = ""
        var desc = ""
        var author = ""
        val tags = mutableListOf("小红书")

        val stateJson = extractAssignedJsonObject(body, "window.__INITIAL_STATE__=", "window.__INITIAL_STATE__ =")
        if (stateJson.isNotEmpty()) {
            val cleaned = jsUndefinedRegex.replace(stateJson, ":null")
            val root = runCatching { JSONObject(cleaned) }.getOrNull()
            val detailMap = root?.optJSONObject("note")?.optJSONObject("noteDetailMap")
            if (detailMap != null) {
                val keys = detailMap.keys()
                while (keys.hasNext()) {
                    val k = keys.next()
                    val note = detailMap.optJSONObject(k)?.optJSONObject("note") ?: continue
                    val nTitle = note.optString("title").trim()
                    val nDesc = note.optString("desc").trim()
                    if (nTitle.isNotEmpty() || nDesc.isNotEmpty()) {
                        title = nTitle
                        desc = nDesc
                        author = note.optJSONObject("user")?.optString("nickname")?.trim().orEmpty()
                        val tagList = note.optJSONArray("tagList")
                        if (tagList != null) {
                            val lim = minOf(tagList.length(), 4)
                            for (i in 0 until lim) {
                                val tn = tagList.optJSONObject(i)?.optString("name")?.trim().orEmpty()
                                if (tn.isNotEmpty()) tags.add(tn)
                            }
                        }
                        break
                    }
                }
            }
        }

        if (title.isEmpty()) {
            val t = extractMetaTitle(body).removeSuffix(" - 小红书").trim()
            if (t.isNotEmpty() && !isLoginWall(t) && t != "小红书") title = t
        }
        if (desc.isEmpty()) {
            val d = extractMetaDesc(body)
            if (d.isNotEmpty() && !isLoginWall(d)) desc = d
        }
        if (title.isEmpty() && desc.isEmpty()) {
            return PrefetchedShareMeta(canonicalUrl = canonical)
        }
        val sb = StringBuilder()
        if (author.isNotEmpty()) sb.append("作者：").append(author).append('\n')
        if (title.isNotEmpty()) sb.append("标题：").append(title).append('\n')
        if (desc.isNotEmpty()) sb.append("正文：\n").append(desc)
        return PrefetchedShareMeta(
            canonicalUrl = canonical,
            title = title.ifEmpty { desc.take(48) },
            excerpt = sb.toString().trim().take(6000),
            tags = tags.distinct().joinToString(",")
        )
    }

    private fun prefetchDouyin(targetUrl: String): PrefetchedShareMeta? {
        var res = fetch(targetUrl, MOBILE_UA, referer = "https://www.douyin.com/") ?: return null
        val canonical = res.finalUrl.ifBlank { targetUrl }
        var parsed = parseDouyinHtml(res.body, canonical)
        if (parsed != null) return parsed

        val idMatch = douyinIdRegex.find(canonical)
        if (idMatch != null) {
            val shareUrl = "https://www.iesdouyin.com/share/video/${idMatch.groupValues[1]}/"
            if (!shareUrl.equals(canonical, ignoreCase = true)) {
                res = fetch(shareUrl, MOBILE_UA, referer = "https://www.iesdouyin.com/") ?: return PrefetchedShareMeta(canonicalUrl = canonical)
                parsed = parseDouyinHtml(res.body, canonical)
                if (parsed != null) return parsed
            }
        }
        return PrefetchedShareMeta(canonicalUrl = canonical)
    }

    private fun parseDouyinHtml(body: String, canonical: String): PrefetchedShareMeta? {
        if (body.isBlank()) return null
        var desc = ""
        var author = ""
        val tags = mutableListOf("抖音")

        val routerJson = extractAssignedJsonObject(body, "window._ROUTER_DATA =", "window._ROUTER_DATA=")
        if (routerJson.isNotEmpty()) {
            val root = runCatching { JSONObject(routerJson) }.getOrNull()
            val loaderData = root?.optJSONObject("loaderData")
            if (loaderData != null) {
                val keys = loaderData.keys()
                while (keys.hasNext()) {
                    val k = keys.next()
                    val page = loaderData.optJSONObject(k) ?: continue
                    val items = page.optJSONObject("videoInfoRes")?.optJSONArray("item_list") ?: continue
                    if (items.length() > 0) {
                        val it = items.optJSONObject(0) ?: continue
                        desc = it.optString("desc").trim()
                        author = it.optJSONObject("author")?.optString("nickname")?.trim().orEmpty()
                        val textExtra = it.optJSONArray("text_extra")
                        if (textExtra != null) {
                            val lim = minOf(textExtra.length(), 4)
                            for (i in 0 until lim) {
                                val ht = textExtra.optJSONObject(i)?.optString("hashtag_name")?.trim().orEmpty()
                                if (ht.isNotEmpty()) tags.add(ht)
                            }
                        }
                        break
                    }
                }
            }
        }
        if (desc.isEmpty()) {
            val d = extractMetaDesc(body)
            if (d.isNotEmpty() && !isLoginWall(d) && !d.contains("抖音为您提供短视频")) desc = d
        }
        if (desc.isEmpty()) return null
        val sb = StringBuilder()
        if (author.isNotEmpty()) sb.append("作者：").append(author).append('\n')
        sb.append("视频文案：\n").append(desc)
        val displayTitle = if (author.isNotEmpty() && !desc.contains(author)) {
            "[$author] ${desc.take(56)}"
        } else {
            desc.take(64)
        }
        return PrefetchedShareMeta(
            canonicalUrl = canonical,
            title = displayTitle,
            excerpt = sb.toString().trim().take(6000),
            tags = tags.distinct().joinToString(",")
        )
    }

    private fun prefetchWeChat(targetUrl: String): PrefetchedShareMeta? {
        val res = fetch(targetUrl, MOBILE_UA, referer = "https://mp.weixin.qq.com/") ?: return null
        val title = extractMetaTitle(res.body)
        val desc = extractMetaDesc(res.body)
        val nickMatch = wechatNickRegex.find(res.body)
        val author = nickMatch?.groupValues?.drop(1)?.firstOrNull { it.isNotBlank() }?.trim().orEmpty()
        if (title.isEmpty() && desc.isEmpty()) return null
        val sb = StringBuilder()
        if (author.isNotEmpty()) sb.append("公众号：").append(author).append('\n')
        if (title.isNotEmpty()) sb.append("标题：").append(title).append('\n')
        if (desc.isNotEmpty()) sb.append("摘要：\n").append(desc)
        return PrefetchedShareMeta(
            canonicalUrl = res.finalUrl.ifBlank { targetUrl },
            title = title,
            excerpt = sb.toString().trim().take(6000),
            tags = "公众号"
        )
    }

    private fun extractBvid(url: String): String {
        val m = bvidRegex.find(url) ?: return ""
        val raw = m.groupValues[1]
        return if (raw.length > 2) "BV" + raw.substring(2) else raw
    }

    private fun extractMetaTitle(html: String): String {
        val og = metaOgTitle1.find(html)?.groupValues?.getOrNull(1)
            ?: metaOgTitle2.find(html)?.groupValues?.getOrNull(1)
        if (!og.isNullOrBlank()) return unescapeHtml(og)
        val raw = htmlTitleRegex.find(html)?.groupValues?.getOrNull(1).orEmpty()
        return unescapeHtml(raw)
    }

    private fun extractMetaDesc(html: String): String {
        val raw = metaDesc1.find(html)?.groupValues?.getOrNull(1)
            ?: metaDesc2.find(html)?.groupValues?.getOrNull(1)
            ?: return ""
        return unescapeHtml(raw)
    }

    private fun unescapeHtml(raw: String): String {
        if (raw.isBlank()) return ""
        return runCatching {
            Html.fromHtml(raw, Html.FROM_HTML_MODE_LEGACY).toString().replace(Regex("""\s+"""), " ").trim()
        }.getOrDefault(raw.trim())
    }

    private fun isLoginWall(text: String): Boolean {
        val lower = text.lowercase()
        return lower.contains("你的生活指南") ||
            lower.contains("请登录") ||
            lower.contains("扫码登录") ||
            lower.contains("安全验证") ||
            lower.contains("验证码") ||
            lower.contains("access denied")
    }

    private fun extractAssignedJsonObject(rawHtml: String, vararg markers: String): String {
        for (marker in markers) {
            val idx = rawHtml.indexOf(marker)
            if (idx < 0) continue
            val startRel = rawHtml.indexOf('{', idx + marker.length)
            if (startRel < 0) continue
            var depth = 0
            var inStr = false
            var escaped = false
            val limit = minOf(rawHtml.length, startRel + 512 * 1024)
            for (i in startRel until limit) {
                val ch = rawHtml[i]
                if (inStr) {
                    if (escaped) {
                        escaped = false
                    } else if (ch == '\\') {
                        escaped = true
                    } else if (ch == '"') {
                        inStr = false
                    }
                    continue
                }
                when (ch) {
                    '"' -> inStr = true
                    '{' -> depth++
                    '}' -> {
                        depth--
                        if (depth == 0) {
                            return rawHtml.substring(startRel, i + 1)
                        }
                    }
                }
            }
        }
        return ""
    }
}
