package uk.starryheavens.vesper.bothub.ui.sheets

import android.annotation.SuppressLint
import android.content.Intent
import android.graphics.Color as AndroidColor
import android.net.Uri
import android.view.ViewGroup
import android.webkit.CookieManager
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.OpenInNew
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.PlatformTextStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.UrlUtils

/**
 * Polyglot Hybrid Console Sheet (Kotlin Native Shell + Hardware-Accelerated Vue 3 SPA).
 * Mints a zero-login 24h web session cookie from the paired App Bearer Token, injects it into
 * CookieManager, and mounts the full BotHub Vue 3 Studio with native back-gesture & inset handling.
 */
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun ConsoleWebSheet(
    serverUrl: String,
    entry: String,
    title: String,
    api: BotHubApiClient,
    onDismiss: () -> Unit
) {
    val context = LocalContext.current
    val baseUrl = remember(serverUrl) {
        UrlUtils.normalizeServerUrl(serverUrl, UrlUtils.DEFAULT_SERVER_URL)
    }
    val targetUrl = remember(baseUrl, entry) {
        val encodedEntry = Uri.encode(entry.ifBlank { "ops" })
        "$baseUrl/?app_shell=1&entry=$encodedEntry"
    }
    val baseHost = remember(baseUrl) {
        runCatching { Uri.parse(baseUrl).host.orEmpty().lowercase() }.getOrDefault("")
    }

    var webViewRef by remember { mutableStateOf<WebView?>(null) }
    var loadProgress by remember { mutableIntStateOf(10) }
    var sessionReady by remember { mutableStateOf(false) }
    var authError by remember { mutableStateOf("") }

    LaunchedEffect(baseUrl, entry) {
        authError = ""
        sessionReady = false
        api.mintWebSession().onSuccess { ticket ->
            val cm = CookieManager.getInstance()
            cm.setAcceptCookie(true)
            val secureAttr = if (baseUrl.startsWith("https://", ignoreCase = true)) "; Secure" else ""
            val cookieStr =
                "${ticket.cookieName}=${ticket.cookieValue}; Path=/; Max-Age=${ticket.maxAge}; SameSite=Lax$secureAttr"
            cm.setCookie(baseUrl, cookieStr)
            cm.flush()
            sessionReady = true
        }.onFailure { err ->
            authError = err.message ?: "无法初始化工作台免密会话"
            sessionReady = true
        }
    }

    Dialog(
        onDismissRequest = onDismiss,
        properties = DialogProperties(
            usePlatformDefaultWidth = false,
            decorFitsSystemWindows = false
        )
    ) {
        BackHandler {
            val wv = webViewRef
            if (wv != null && wv.canGoBack()) {
                wv.goBack()
            } else {
                onDismiss()
            }
        }

        Column(
            modifier = Modifier
                .fillMaxSize()
                .background(Color(0xFF050811))
                .imePadding()
        ) {
            // Native Obsidian Top Bar
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Color(0xF2080C1A))
                    .statusBarsPadding()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 12.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                        modifier = Modifier.weight(1f)
                    ) {
                        Surface(
                            onClick = onDismiss,
                            shape = CircleShape,
                            color = Color(0xFF10172C),
                            border = BorderStroke(1.dp, Color(0x24FFF8EE)),
                            modifier = Modifier.size(36.dp)
                        ) {
                            Box(contentAlignment = Alignment.Center) {
                                Icon(
                                    Icons.Default.Close,
                                    contentDescription = "关闭工作台",
                                    tint = Color(0xFFCBD5E1),
                                    modifier = Modifier.size(17.dp)
                                )
                            }
                        }

                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = title.ifBlank { "全功能工作台" },
                                style = TextStyle(
                                    fontSize = 14.5.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = Color(0xFFFFF8EE),
                                    platformStyle = PlatformTextStyle(includeFontPadding = false)
                                ),
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                            Text(
                                text = authError.ifEmpty { UrlUtils.formatDisplayHost(baseUrl) },
                                fontFamily = FontFamily.Monospace,
                                fontSize = 10.5.sp,
                                color = if (authError.isNotEmpty()) Color(0xFFFDA4AF) else Color(0xFF94A3B8),
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                    }

                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        Surface(
                            onClick = { webViewRef?.reload() },
                            shape = CircleShape,
                            color = Color(0xFF10172C),
                            border = BorderStroke(1.dp, Color(0x22FFF8EE)),
                            modifier = Modifier.size(36.dp)
                        ) {
                            Box(contentAlignment = Alignment.Center) {
                                Icon(
                                    Icons.Default.Refresh,
                                    contentDescription = "刷新",
                                    tint = Color(0xFFCBD5E1),
                                    modifier = Modifier.size(16.dp)
                                )
                            }
                        }

                        Surface(
                            onClick = {
                                val current = webViewRef?.url ?: baseUrl
                                runCatching {
                                    context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(current)))
                                }
                            },
                            shape = CircleShape,
                            color = Color(0xFF10172C),
                            border = BorderStroke(1.dp, Color(0x22FFF8EE)),
                            modifier = Modifier.size(36.dp)
                        ) {
                            Box(contentAlignment = Alignment.Center) {
                                Icon(
                                    Icons.Default.OpenInNew,
                                    contentDescription = "在系统浏览器打开",
                                    tint = Color(0xFFCBD5E1),
                                    modifier = Modifier.size(15.dp)
                                )
                            }
                        }
                    }
                }

                if (!sessionReady || loadProgress in 1..99) {
                    LinearProgressIndicator(
                        progress = { (loadProgress / 100f).coerceIn(0.05f, 1f) },
                        color = Color(0xFFF2B880),
                        trackColor = Color(0xFF0E152B),
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(2.dp)
                    )
                } else {
                    HorizontalDivider(thickness = 0.5.dp, color = Color(0x1FFFF8EE))
                }
            }

            // Hardware-Accelerated Vue 3 WebView Viewport
            Box(
                modifier = Modifier
                    .weight(1f)
                    .fillMaxWidth()
                    .navigationBarsPadding()
            ) {
                if (sessionReady) {
                    AndroidView(
                        modifier = Modifier.fillMaxSize(),
                        factory = { ctx ->
                            WebView(ctx).apply {
                                layoutParams = ViewGroup.LayoutParams(
                                    ViewGroup.LayoutParams.MATCH_PARENT,
                                    ViewGroup.LayoutParams.MATCH_PARENT
                                )
                                setBackgroundColor(AndroidColor.parseColor("#050811"))
                                CookieManager.getInstance().setAcceptThirdPartyCookies(this, true)
                                settings.apply {
                                    javaScriptEnabled = true
                                    domStorageEnabled = true
                                    databaseEnabled = true
                                    loadsImagesAutomatically = true
                                    useWideViewPort = true
                                    loadWithOverviewMode = true
                                    cacheMode = WebSettings.LOAD_DEFAULT
                                    mixedContentMode = WebSettings.MIXED_CONTENT_COMPATIBILITY_MODE
                                    textZoom = 100
                                }
                                webChromeClient = object : WebChromeClient() {
                                    override fun onProgressChanged(view: WebView?, newProgress: Int) {
                                        loadProgress = newProgress
                                    }
                                }
                                webViewClient = object : WebViewClient() {
                                    override fun shouldOverrideUrlLoading(
                                        view: WebView?,
                                        request: WebResourceRequest?
                                    ): Boolean {
                                        val uri = request?.url ?: return false
                                        val scheme = uri.scheme?.lowercase().orEmpty()
                                        if (scheme != "http" && scheme != "https") {
                                            runCatching {
                                                ctx.startActivity(Intent(Intent.ACTION_VIEW, uri))
                                            }
                                            return true
                                        }
                                        val reqHost = uri.host?.lowercase().orEmpty()
                                        if (baseHost.isNotEmpty() && reqHost != baseHost && !reqHost.endsWith("starryheavens.uk")) {
                                            runCatching {
                                                ctx.startActivity(Intent(Intent.ACTION_VIEW, uri))
                                            }
                                            return true
                                        }
                                        return false
                                    }
                                }
                                webViewRef = this
                                loadUrl(targetUrl)
                            }
                        }
                    )

                    DisposableEffect(Unit) {
                        onDispose {
                            webViewRef?.stopLoading()
                            webViewRef?.destroy()
                            webViewRef = null
                        }
                    }
                } else {
                    Box(
                        modifier = Modifier.fillMaxSize(),
                        contentAlignment = Alignment.Center
                    ) {
                        Surface(
                            shape = RoundedCornerShape(999.dp),
                            color = Color(0xFF0E1428),
                            border = BorderStroke(1.dp, Color(0x33F2B880))
                        ) {
                            Text(
                                text = "正在建立免密安全工作台会话...",
                                fontSize = 12.5.sp,
                                color = Color(0xFFFFE2BE),
                                modifier = Modifier.padding(horizontal = 16.dp, vertical = 9.dp)
                            )
                        }
                    }
                }
            }
        }
    }
}
