package uk.starryheavens.vesper.bothub.sync

import android.accessibilityservice.AccessibilityService
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import android.util.Base64
import android.view.Display
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityNodeInfo
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.FileOutputStream

data class ScreenCaptureSnapshot(
    val sourcePackage: String = "",
    val sourceAppLabel: String = "",
    val extractedUrl: String = "",
    val candidateTitle: String = "",
    val visibleText: String = "",
    val screenshotFilePath: String = "",
    val imageDataUrl: String = "",
    val autoCopiedLink: Boolean = false,
    val capturedAtMs: Long = System.currentTimeMillis()
) {
    fun buildRawInput(): String {
        val parts = mutableListOf<String>()
        if (sourceAppLabel.isNotBlank() && extractedUrl.isBlank()) {
            parts.add("[${sourceAppLabel}识屏]")
        }
        if (candidateTitle.isNotBlank()) {
            parts.add(candidateTitle)
        }
        if (extractedUrl.isNotBlank()) {
            parts.add(extractedUrl)
        }
        if (visibleText.isNotBlank() && visibleText != candidateTitle) {
            parts.add(visibleText.take(1800))
        }
        return parts.joinToString("\n").trim()
    }

    fun buildClientExcerpt(): String {
        val sb = StringBuilder()
        if (sourceAppLabel.isNotBlank()) {
            sb.append("来源应用：").append(sourceAppLabel).append('\n')
        }
        if (extractedUrl.isNotBlank()) {
            sb.append("页面链接：").append(extractedUrl).append('\n')
        }
        if (visibleText.isNotBlank()) {
            sb.append("页面可见文字：\n").append(visibleText.take(4000))
        }
        return sb.toString().trim()
    }
}

object ScreenCaptureHolder {
    @Volatile
    private var pending: ScreenCaptureSnapshot? = null

    fun setPending(snapshot: ScreenCaptureSnapshot) {
        pending = snapshot
    }

    fun consumePending(): ScreenCaptureSnapshot? {
        val current = pending
        pending = null
        return current
    }
}

class ScreenCaptureAccessibilityService : AccessibilityService() {

    private val mainHandler = Handler(Looper.getMainLooper())

    override fun onServiceConnected() {
        super.onServiceConnected()
        activeService = this
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        if (event == null) return
        if (event.eventType == AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED) {
            val pkg = event.packageName?.toString()?.trim().orEmpty()
            if (pkg.isNotEmpty() && pkg != packageName && pkg != "com.android.systemui") {
                lastForegroundPackage = pkg
            }
        }
    }

    override fun onInterrupt() {
        // Only reads screen when user explicitly taps the floating capture pill.
    }

    override fun onUnbind(intent: Intent?): Boolean {
        if (activeService === this) {
            activeService = null
        }
        return super.onUnbind(intent)
    }

    override fun onDestroy() {
        if (activeService === this) {
            activeService = null
        }
        super.onDestroy()
    }

    fun captureCurrentScreen(onComplete: (ScreenCaptureSnapshot) -> Unit) {
        val treeResult = runCatching { extractScreenTree() }.getOrElse { ScreenTreeData() }

        // 1. If we already found a real URL on screen (e.g. browser address bar), return immediately with screenshot
        if (treeResult.extractedUrl.isNotBlank()) {
            finishWithOptionalScreenshot(treeResult, autoCopiedLink = false, onComplete = onComplete)
            return
        }

        // 2. For native content apps (Douyin / Xiaohongshu / Bilibili / WeChat / Zhihu / Weibo),
        //    auto-tap "Share -> Copy Link" to capture the real URL first; fall back to screen text + screenshot.
        if (shouldAttemptAutoCopyLink(treeResult.appLabel)) {
            attemptAutoCopyShareLink(treeResult.appLabel) { copied ->
                if (copied) {
                    val snapshot = ScreenCaptureSnapshot(
                        sourcePackage = treeResult.packageName,
                        sourceAppLabel = treeResult.appLabel,
                        candidateTitle = treeResult.candidateTitle,
                        visibleText = treeResult.visibleText,
                        autoCopiedLink = true
                    )
                    mainHandler.post { onComplete(snapshot) }
                } else {
                    // Fallback: wait briefly in case a share drawer just closed, then capture screen text + screenshot
                    mainHandler.postDelayed({
                        finishWithOptionalScreenshot(treeResult, autoCopiedLink = false, onComplete = onComplete)
                    }, 180L)
                }
            }
            return
        }

        // 3. Other apps: capture screen text + silent screenshot directly
        finishWithOptionalScreenshot(treeResult, autoCopiedLink = false, onComplete = onComplete)
    }

    private fun shouldAttemptAutoCopyLink(appLabel: String): Boolean {
        return appLabel in AUTO_COPY_APPS
    }

    private fun attemptAutoCopyShareLink(appLabel: String, onDone: (Boolean) -> Unit) {
        val clickedShare = runCatching { findAndClickShareTrigger(appLabel) }.getOrDefault(false)
        if (!clickedShare) {
            onDone(false)
            return
        }

        var attempt = 0
        var scrolledOnce = false
        val maxAttempts = 7

        fun pollForCopyLink() {
            attempt++
            val clickedCopy = runCatching { findAndClickCopyLinkNode() }.getOrDefault(false)
            if (clickedCopy) {
                mainHandler.postDelayed({
                    // Close share drawer only if it remained visible after clicking copy link
                    if (isShareDrawerStillVisible()) {
                        runCatching { performGlobalAction(GLOBAL_ACTION_BACK) }
                    }
                    onDone(true)
                }, 200L)
                return
            }

            if (attempt == 3 && !scrolledOnce) {
                scrolledOnce = true
                runCatching { scrollHorizontalContainersForward() }
            }

            if (attempt < maxAttempts) {
                mainHandler.postDelayed({ pollForCopyLink() }, 150L)
            } else {
                // Clean up opened share drawer before falling back to screenshot
                if (isShareDrawerStillVisible()) {
                    runCatching { performGlobalAction(GLOBAL_ACTION_BACK) }
                }
                onDone(false)
            }
        }

        mainHandler.postDelayed({ pollForCopyLink() }, 220L)
    }

    private fun activeAppRoots(): List<AccessibilityNodeInfo> {
        val roots = mutableListOf<AccessibilityNodeInfo>()
        runCatching {
            for (win in windows.orEmpty()) {
                val r = win.root ?: continue
                val pkg = r.packageName?.toString().orEmpty()
                if (pkg.isNotEmpty() && pkg != packageName && pkg != "com.android.systemui") {
                    roots.add(r)
                }
            }
        }
        val activeRoot = runCatching { rootInActiveWindow }.getOrNull()
        if (activeRoot != null) {
            val pkg = activeRoot.packageName?.toString().orEmpty()
            if (pkg != packageName && pkg != "com.android.systemui") {
                roots.add(0, activeRoot)
            }
        }
        return roots
    }

    private fun clickNearestClickable(node: AccessibilityNodeInfo): Boolean {
        var cur: AccessibilityNodeInfo? = node
        var hops = 0
        while (cur != null && hops <= 4) {
            if (cur.isClickable && cur.isEnabled) {
                if (cur.performAction(AccessibilityNodeInfo.ACTION_CLICK)) {
                    return true
                }
            }
            cur = cur.parent
            hops++
        }
        return node.performAction(AccessibilityNodeInfo.ACTION_CLICK)
    }

    private fun findAndClickShareTrigger(appLabel: String): Boolean {
        val roots = activeAppRoots()
        val matchers: List<(String, String, String) -> Boolean> = when (appLabel) {
            "微信" -> listOf(
                { _, desc, vid ->
                    desc == "更多" || desc == "更多功能按钮" || desc.contains("更多") && desc.length <= 8 ||
                        vid.endsWith(":id/coz") || vid.contains("more")
                },
                { txt, desc, _ -> txt == "分享" || desc.contains("分享") }
            )
            else -> listOf(
                { txt, desc, _ ->
                    txt == "分享" || txt == "转发" ||
                        desc == "分享" || desc == "转发" ||
                        desc.startsWith("分享") || desc.startsWith("转发")
                },
                { txt, desc, vid ->
                    (txt.contains("分享") && txt.length <= 8) ||
                        (desc.contains("分享") && desc.length <= 14) ||
                        vid.contains("share_btn") || vid.contains("share_icon") || vid.contains("iv_share")
                }
            )
        }

        for (matcher in matchers) {
            for (root in roots) {
                val hit = searchNode(root, maxNodes = 260) { node ->
                    if (!node.isVisibleToUser) return@searchNode false
                    val txt = node.text?.toString()?.trim().orEmpty()
                    val desc = node.contentDescription?.toString()?.trim().orEmpty()
                    val vid = node.viewIdResourceName?.lowercase().orEmpty()
                    matcher(txt, desc, vid)
                }
                if (hit != null && clickNearestClickable(hit)) {
                    return true
                }
            }
        }
        return false
    }

    private fun findAndClickCopyLinkNode(): Boolean {
        val roots = activeAppRoots()
        for (root in roots) {
            val hit = searchNode(root, maxNodes = 320) { node ->
                if (!node.isVisibleToUser) return@searchNode false
                val txt = node.text?.toString()?.trim().orEmpty()
                val desc = node.contentDescription?.toString()?.trim().orEmpty()
                COPY_LINK_KEYWORDS.any { kw ->
                    txt.equals(kw, ignoreCase = true) ||
                        desc.equals(kw, ignoreCase = true) ||
                        (txt.contains(kw) && txt.length <= 10) ||
                        (desc.contains(kw) && desc.length <= 12)
                }
            }
            if (hit != null && clickNearestClickable(hit)) {
                return true
            }
        }
        return false
    }

    private fun isShareDrawerStillVisible(): Boolean {
        val roots = activeAppRoots()
        for (root in roots) {
            val hit = searchNode(root, maxNodes = 200) { node ->
                if (!node.isVisibleToUser) return@searchNode false
                val txt = node.text?.toString()?.trim().orEmpty()
                val desc = node.contentDescription?.toString()?.trim().orEmpty()
                txt == "复制链接" || desc == "复制链接" || txt == "生成长图" || txt == "保存图片" ||
                    (txt == "取消" && node.isClickable)
            }
            if (hit != null) return true
        }
        return false
    }

    private fun scrollHorizontalContainersForward() {
        val roots = activeAppRoots()
        for (root in roots) {
            val scroller = searchNode(root, maxNodes = 220) { node ->
                if (!node.isVisibleToUser || !node.isScrollable) return@searchNode false
                val cls = node.className?.toString()?.lowercase().orEmpty()
                cls.contains("horizontal") || cls.contains("recyclerview")
            }
            if (scroller != null) {
                scroller.performAction(AccessibilityNodeInfo.ACTION_SCROLL_FORWARD)
                return
            }
        }
    }

    private fun searchNode(
        root: AccessibilityNodeInfo,
        maxNodes: Int,
        predicate: (AccessibilityNodeInfo) -> Boolean
    ): AccessibilityNodeInfo? {
        val queue = ArrayDeque<AccessibilityNodeInfo>()
        queue.add(root)
        var visited = 0
        while (queue.isNotEmpty() && visited < maxNodes) {
            val cur = queue.removeFirst()
            visited++
            if (predicate(cur)) {
                return cur
            }
            val count = cur.childCount
            for (i in 0 until count) {
                val child = cur.getChild(i) ?: continue
                queue.add(child)
            }
        }
        return null
    }

    private fun finishWithOptionalScreenshot(
        treeResult: ScreenTreeData,
        autoCopiedLink: Boolean,
        onComplete: (ScreenCaptureSnapshot) -> Unit
    ) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            val invoked = runCatching {
                takeScreenshot(
                    Display.DEFAULT_DISPLAY,
                    mainExecutor,
                    object : TakeScreenshotCallback {
                        override fun onSuccess(screenshot: ScreenshotResult) {
                            Thread {
                                val (filePath, dataUrl) = encodeScreenshotResult(screenshot)
                                val snapshot = ScreenCaptureSnapshot(
                                    sourcePackage = treeResult.packageName,
                                    sourceAppLabel = treeResult.appLabel,
                                    extractedUrl = treeResult.extractedUrl,
                                    candidateTitle = treeResult.candidateTitle,
                                    visibleText = treeResult.visibleText,
                                    screenshotFilePath = filePath,
                                    imageDataUrl = dataUrl,
                                    autoCopiedLink = autoCopiedLink
                                )
                                mainHandler.post { onComplete(snapshot) }
                            }.start()
                        }

                        override fun onFailure(errorCode: Int) {
                            val snapshot = ScreenCaptureSnapshot(
                                sourcePackage = treeResult.packageName,
                                sourceAppLabel = treeResult.appLabel,
                                extractedUrl = treeResult.extractedUrl,
                                candidateTitle = treeResult.candidateTitle,
                                visibleText = treeResult.visibleText,
                                autoCopiedLink = autoCopiedLink
                            )
                            mainHandler.post { onComplete(snapshot) }
                        }
                    }
                )
                true
            }.getOrDefault(false)
            if (invoked) return
        }

        val snapshot = ScreenCaptureSnapshot(
            sourcePackage = treeResult.packageName,
            sourceAppLabel = treeResult.appLabel,
            extractedUrl = treeResult.extractedUrl,
            candidateTitle = treeResult.candidateTitle,
            visibleText = treeResult.visibleText,
            autoCopiedLink = autoCopiedLink
        )
        mainHandler.post { onComplete(snapshot) }
    }

    private fun encodeScreenshotResult(screenshot: ScreenshotResult): Pair<String, String> {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.R) return "" to ""
        return runCatching {
            val buffer = screenshot.hardwareBuffer
            val colorSpace = screenshot.colorSpace
            var softBitmap: Bitmap? = null
            var scaledBitmap: Bitmap? = null
            try {
                val hwBitmap = Bitmap.wrapHardwareBuffer(buffer, colorSpace) ?: return "" to ""
                softBitmap = hwBitmap.copy(Bitmap.Config.ARGB_8888, false)
                hwBitmap.recycle()
                if (softBitmap == null) return "" to ""

                val maxSide = 1080
                val srcW = softBitmap.width
                val srcH = softBitmap.height
                val outBitmap = if (srcW > maxSide || srcH > maxSide) {
                    val ratio = minOf(maxSide.toFloat() / srcW.toFloat(), maxSide.toFloat() / srcH.toFloat())
                    val dstW = (srcW * ratio).toInt().coerceAtLeast(1)
                    val dstH = (srcH * ratio).toInt().coerceAtLeast(1)
                    scaledBitmap = Bitmap.createScaledBitmap(softBitmap, dstW, dstH, true)
                    scaledBitmap
                } else {
                    softBitmap
                }

                val baos = ByteArrayOutputStream()
                outBitmap.compress(Bitmap.CompressFormat.JPEG, 72, baos)
                val jpegBytes = baos.toByteArray()

                val outFile = File(cacheDir, "last_screen_capture.jpg")
                FileOutputStream(outFile).use { fos -> fos.write(jpegBytes) }

                val b64 = Base64.encodeToString(jpegBytes, Base64.NO_WRAP)
                outFile.absolutePath to "data:image/jpeg;base64,$b64"
            } finally {
                scaledBitmap?.recycle()
                softBitmap?.recycle()
                runCatching { buffer.close() }
            }
        }.getOrDefault("" to "")
    }

    private data class ScreenTreeData(
        val packageName: String = "",
        val appLabel: String = "",
        val extractedUrl: String = "",
        val candidateTitle: String = "",
        val visibleText: String = ""
    )

    private fun extractScreenTree(): ScreenTreeData {
        val roots = activeAppRoots()
        val detectedPkg = roots.firstOrNull()?.packageName?.toString()?.trim().orEmpty()
            .ifEmpty { lastForegroundPackage }
        val appLabel = resolveAppLabel(detectedPkg)

        var foundUrl = ""
        val lines = LinkedHashSet<String>()
        var visited = 0
        val maxNodes = 260
        var totalChars = 0

        fun visit(node: AccessibilityNodeInfo?, depth: Int) {
            if (node == null || visited >= maxNodes || depth > 22 || totalChars >= 2600) return
            visited++
            val viewId = node.viewIdResourceName?.lowercase().orEmpty()
            val textVal = node.text?.toString()?.trim().orEmpty()
            val descVal = node.contentDescription?.toString()?.trim().orEmpty()

            if (foundUrl.isEmpty()) {
                val urlCandidate = findUrlInNode(viewId, textVal, descVal)
                if (urlCandidate.isNotEmpty()) {
                    foundUrl = urlCandidate
                }
            }

            val candidates = listOf(textVal, descVal)
            for (raw in candidates) {
                val clean = raw.replace(WHITESPACE_REGEX, " ").trim()
                if (clean.length < 3) continue
                if (isUiNoise(clean)) continue
                if (lines.add(clean)) {
                    totalChars += clean.length
                }
            }

            val count = node.childCount
            for (i in 0 until count) {
                if (visited >= maxNodes || totalChars >= 2600) break
                visit(node.getChild(i), depth + 1)
            }
        }

        for (r in roots.take(2)) {
            visit(r, 0)
        }

        val lineList = lines.toList()
        val candidateTitle = lineList
            .firstOrNull { it.length in 6..96 && !it.startsWith("http://") && !it.startsWith("https://") }
            ?: lineList.firstOrNull()?.take(80).orEmpty()
        val visibleText = lineList.joinToString("\n").take(2600)

        return ScreenTreeData(
            packageName = detectedPkg,
            appLabel = appLabel,
            extractedUrl = foundUrl,
            candidateTitle = candidateTitle,
            visibleText = visibleText
        )
    }

    private fun findUrlInNode(viewId: String, textVal: String, descVal: String): String {
        for (raw in listOf(textVal, descVal)) {
            val match = HTTP_URL_REGEX.find(raw)
            if (match != null) {
                return match.value.trimEnd(',', '.', ';', ')', '）', '】', '》')
            }
        }
        val isAddressBar = viewId.contains("url_bar") ||
            viewId.contains("location_bar") ||
            viewId.contains("address_bar") ||
            viewId.contains("url_field") ||
            viewId.contains("omnibox") ||
            viewId.contains("search_box_text") ||
            viewId.contains("title_url")
        if (isAddressBar && textVal.isNotBlank() && !textVal.contains(" ")) {
            if (BARE_DOMAIN_REGEX.matches(textVal)) {
                return "https://$textVal"
            }
        }
        return ""
    }

    private fun isUiNoise(text: String): Boolean {
        if (text in NOISE_LABELS) return true
        if (NUMERIC_COUNTER_REGEX.matches(text)) return true
        return false
    }

    private fun resolveAppLabel(pkg: String): String {
        val lower = pkg.lowercase()
        return when {
            lower.contains("aweme") || lower.contains("douyin") -> "抖音"
            lower.contains("xingin.xhs") || lower.contains("xiaohongshu") -> "小红书"
            lower.contains("bilibili") || lower.contains("danmaku.bili") -> "Bilibili"
            lower == "com.tencent.mm" -> "微信"
            lower.contains("zhihu") -> "知乎"
            lower.contains("weibo") -> "微博"
            lower.contains("chrome") || lower.contains("emmx") || lower.contains("quark") ||
                lower.contains("firefox") || lower.contains("via") || lower.contains("browser") -> "浏览器"
            pkg.isNotBlank() -> runCatching {
                val ai = packageManager.getApplicationInfo(pkg, 0)
                packageManager.getApplicationLabel(ai).toString().trim()
            }.getOrDefault("")
            else -> ""
        }
    }

    companion object {
        private val WHITESPACE_REGEX = Regex("""\s+""")
        private val HTTP_URL_REGEX = Regex("""https?://[^\s<>"'\]）】》]+""", RegexOption.IGNORE_CASE)
        private val BARE_DOMAIN_REGEX = Regex("""^[a-zA-Z0-9-]+(?:\.[a-zA-Z0-9-]+)+(?:/[^\s]*)?$""")
        private val NUMERIC_COUNTER_REGEX = Regex("""^\d+(?:\.\d+)?[万千wWkK+]?$""")
        private val AUTO_COPY_APPS = setOf("抖音", "小红书", "Bilibili", "微信", "知乎", "微博")
        private val COPY_LINK_KEYWORDS = listOf("复制链接", "复制网址", "复制视频链接", "复制笔记链接", "复制口令")
        private val NOISE_LABELS = setOf(
            "首页", "朋友", "消息", "我的", "商城", "推荐", "关注", "同城", "发现",
            "点赞", "评论", "转发", "收藏", "分享", "搜索", "返回", "更多", "关闭",
            "拍照", "视频", "直播", "购物", "团购", "经验", "热点", "刷新", "主页"
        )

        @Volatile
        var activeService: ScreenCaptureAccessibilityService? = null
            private set

        @Volatile
        var lastForegroundPackage: String = ""
            private set

        fun isServiceEnabled(context: Context): Boolean {
            if (activeService != null) return true
            val expected = ComponentName(context.applicationContext, ScreenCaptureAccessibilityService::class.java)
            val enabled = runCatching {
                Settings.Secure.getString(
                    context.contentResolver,
                    Settings.Secure.ENABLED_ACCESSIBILITY_SERVICES
                ).orEmpty()
            }.getOrDefault("")
            if (enabled.isBlank()) return false
            val full = expected.flattenToString()
            val shortName = expected.flattenToShortString()
            return enabled.split(':').any { item ->
                item.equals(full, ignoreCase = true) || item.equals(shortName, ignoreCase = true)
            }
        }

        fun openAccessibilitySettings(context: Context) {
            val intent = Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            runCatching { context.startActivity(intent) }
        }
    }
}
