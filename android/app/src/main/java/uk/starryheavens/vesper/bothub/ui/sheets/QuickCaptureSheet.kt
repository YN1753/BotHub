package uk.starryheavens.vesper.bothub.ui.sheets

import android.graphics.BitmapFactory
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.BotHubPrefs
import uk.starryheavens.vesper.bothub.data.ClientSharePrefetcher
import uk.starryheavens.vesper.bothub.data.PrefetchedShareMeta
import uk.starryheavens.vesper.bothub.data.ShareTextParser
import uk.starryheavens.vesper.bothub.sync.ScreenCaptureSnapshot
import uk.starryheavens.vesper.bothub.ui.components.BotAvatar
import uk.starryheavens.vesper.bothub.ui.components.VesperCard
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.VesperTag
import uk.starryheavens.vesper.bothub.ui.components.VesperTextField
import uk.starryheavens.vesper.bothub.ui.components.ZeroClickCaptureRow
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

@Composable
fun QuickCaptureSheet(
    rawShared: String,
    prefs: BotHubPrefs,
    api: BotHubApiClient,
    forcePreview: Boolean = false,
    screenSnapshot: ScreenCaptureSnapshot? = null,
    accessibilityEnabled: Boolean = true,
    onOpenAccessibilitySettings: () -> Unit = {},
    onSaved: (String) -> Unit,
    onForwardDeepChat: (String) -> Unit,
    onOpenSetup: () -> Unit,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier
) {
    val theme = BotHubThemeTokens
    val scope = rememberCoroutineScope()
    val preview = remember(rawShared) { ShareTextParser.parse(rawShared) }
    var noteInput by remember(rawShared) { mutableStateOf("") }
    var zeroClick by remember { mutableStateOf(prefs.zeroClickCaptureEnabled) }
    var attachScreenshot by remember(screenSnapshot?.capturedAtMs) {
        mutableStateOf(screenSnapshot?.imageDataUrl?.isNotBlank() == true)
    }
    val screenshotThumb = remember(screenSnapshot?.screenshotFilePath) {
        val path = screenSnapshot?.screenshotFilePath.orEmpty()
        if (path.isBlank()) null else runCatching { BitmapFactory.decodeFile(path)?.asImageBitmap() }.getOrNull()
    }
    var saving by remember(rawShared) { mutableStateOf(false) }
    var savingTodo by remember(rawShared) { mutableStateOf(false) }
    var savedMsg by remember(rawShared) { mutableStateOf("") }
    var errorMsg by remember(rawShared) { mutableStateOf("") }

    var hasToken by remember { mutableStateOf(prefs.isLikelyPaired) }
    var prefetchDeferred by remember(rawShared) { mutableStateOf<Deferred<PrefetchedShareMeta?>?>(null) }

    fun submitCapture() {
        if (saving || savingTodo || !hasToken) return
        saving = true
        errorMsg = ""
        scope.launch {
            val urlPrefetched = runCatching {
                val active = prefetchDeferred
                if (active != null) {
                    withTimeoutOrNull(2800L) { active.await() }
                } else if (preview.extractedUrl.isNotBlank()) {
                    ClientSharePrefetcher.prefetch(preview.extractedUrl)
                } else {
                    null
                }
            }.getOrNull()

            val snap = screenSnapshot
            val mergedPrefetch = if (snap != null) {
                val snapExcerpt = snap.buildClientExcerpt()
                val snapTag = snap.sourceAppLabel.takeIf { it.isNotBlank() && it != "浏览器" }.orEmpty()
                val mergedTags = listOf(urlPrefetched?.tags.orEmpty(), snapTag)
                    .filter { it.isNotBlank() }
                    .joinToString(",")
                PrefetchedShareMeta(
                    canonicalUrl = urlPrefetched?.canonicalUrl.orEmpty().ifBlank { snap.extractedUrl },
                    title = urlPrefetched?.title.orEmpty().ifBlank { snap.candidateTitle },
                    excerpt = urlPrefetched?.excerpt.orEmpty().ifBlank { snapExcerpt },
                    tags = mergedTags,
                    imageDataUrl = if (attachScreenshot) snap.imageDataUrl else ""
                ).takeUnless { it.isEmpty }
            } else {
                urlPrefetched
            }

            api.createStashItem(
                rawInput = rawShared,
                userNote = noteInput,
                prefetched = mergedPrefetch
            ).fold(
                onSuccess = { (_, deduplicated) ->
                    val toastText = if (deduplicated) {
                        "该内容已在 BotHub 收藏匣，已合并批注"
                    } else {
                        "已收入 BotHub 收藏匣，后台正在提炼要点"
                    }
                    savedMsg = toastText
                    delay(320L)
                    onSaved(toastText)
                },
                onFailure = { err ->
                    saving = false
                    errorMsg = err.message ?: "网络异常，请重试"
                }
            )
        }
    }

    fun submitTodo() {
        if (saving || savingTodo || !hasToken) return
        savingTodo = true
        errorMsg = ""
        val cleanTitle = preview.candidateTitle.ifBlank { rawShared.take(60) }.take(72)
        val notesParts = buildList {
            if (noteInput.isNotBlank()) add(noteInput.trim())
            if (preview.extractedUrl.isNotBlank()) add(preview.extractedUrl)
            if (rawShared.length > cleanTitle.length && preview.extractedUrl.isBlank()) {
                add(rawShared.take(600))
            }
        }
        scope.launch {
            api.createTodo(
                title = cleanTitle,
                notes = notesParts.joinToString("\n\n"),
                priority = "normal"
            ).fold(
                onSuccess = {
                    val toastText = "已添加至 BotHub 个人待办"
                    savedMsg = toastText
                    delay(320L)
                    onSaved(toastText)
                },
                onFailure = { err ->
                    savingTodo = false
                    errorMsg = err.message ?: "添加待办失败，请重试"
                }
            )
        }
    }

    LaunchedEffect(rawShared, forcePreview) {
        if (preview.extractedUrl.isNotBlank() && ClientSharePrefetcher.shouldPrefetch(preview.extractedUrl)) {
            prefetchDeferred = async(Dispatchers.IO) {
                ClientSharePrefetcher.prefetch(preview.extractedUrl)
            }
        }
        val warmToken = withContext(Dispatchers.IO) { prefs.warmUpCredentials() }
        hasToken = warmToken.isNotEmpty()
        if (hasToken && !forcePreview && prefs.zeroClickCaptureEnabled) {
            submitCapture()
        }
    }

    val scrimInteraction = remember { MutableInteractionSource() }
    val sheetInteraction = remember { MutableInteractionSource() }
    val busy = saving || savingTodo

    Box(
        modifier = modifier
            .fillMaxSize()
            .clickable(
                interactionSource = scrimInteraction,
                indication = null,
                onClick = onDismiss
            ),
        contentAlignment = Alignment.BottomCenter
    ) {
        Surface(
            modifier = Modifier
                .fillMaxWidth()
                .clickable(
                    interactionSource = sheetInteraction,
                    indication = null,
                    onClick = {}
                ),
            shape = RoundedCornerShape(topStart = 24.dp, topEnd = 24.dp),
            color = theme.surfaceSheet,
            border = BorderStroke(1.dp, theme.border),
            tonalElevation = 0.dp
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .verticalScroll(rememberScrollState())
                    .navigationBarsPadding()
                    .imePadding()
                    .padding(horizontal = 18.dp, vertical = 10.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                VesperSheetDragHandle()

                val headerBadgeLabel = remember(screenSnapshot, preview.kind) {
                    when {
                        screenSnapshot != null && screenSnapshot.sourceAppLabel.isNotBlank() ->
                            "${screenSnapshot.sourceAppLabel}识屏"
                        screenSnapshot != null -> "屏幕截取"
                        else -> preview.kind.shareHeaderLabel
                    }
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                        modifier = Modifier.weight(1f)
                    ) {
                        BotAvatar(
                            bot = null,
                            allBots = emptyList(),
                            avatarStyleOverride = preview.kind.avatarStyle,
                            state = when {
                                busy -> "working"
                                errorMsg.isNotEmpty() -> "blocked"
                                noteInput.isNotEmpty() -> "thinking"
                                else -> "idle"
                            },
                            containerSize = 38.dp,
                            interactive = true,
                            seedKey = "quick_capture"
                        )
                        Text(
                            text = if (screenSnapshot != null) "识屏收藏到 BotHub" else "快速收藏到 BotHub",
                            fontSize = 16.5.sp,
                            fontWeight = FontWeight.Bold,
                            color = theme.textPrimary,
                            maxLines = 1,
                            softWrap = false,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f, fill = false)
                        )
                    }
                    VesperTag(text = headerBadgeLabel)
                }

                if (savedMsg.isNotEmpty()) {
                    VesperCard(
                        cornerRadius = 12.dp,
                        accentTone = theme.success,
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 11.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(7.dp)
                                    .background(theme.success, CircleShape)
                            )
                            Text(
                                text = savedMsg,
                                fontSize = 13.sp,
                                fontWeight = FontWeight.Medium,
                                color = theme.textPrimary
                            )
                        }
                    }
                }

                VesperCard(
                    cornerRadius = 14.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        if (screenshotThumb != null) {
                            Box(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .height(148.dp)
                                    .clip(RoundedCornerShape(10.dp))
                                    .border(1.dp, theme.border, RoundedCornerShape(10.dp))
                            ) {
                                Image(
                                    bitmap = screenshotThumb,
                                    contentDescription = "当前页面截图预览",
                                    contentScale = ContentScale.Crop,
                                    modifier = Modifier.fillMaxSize()
                                )
                                Box(
                                    modifier = Modifier
                                        .align(Alignment.BottomEnd)
                                        .padding(8.dp)
                                        .sizeIn(minHeight = 36.dp)
                                        .clip(RoundedCornerShape(9.dp))
                                        .background(
                                            if (attachScreenshot) theme.accent.copy(alpha = 0.92f)
                                            else Color(0xCC14171F)
                                        )
                                        .clickable { attachScreenshot = !attachScreenshot }
                                        .padding(horizontal = 10.dp, vertical = 6.dp),
                                    contentAlignment = Alignment.Center
                                ) {
                                    Text(
                                        text = if (attachScreenshot) "已附带截图供 AI 识图" else "仅文字（点此附带截图）",
                                        fontSize = 11.sp,
                                        fontWeight = FontWeight.Medium,
                                        color = if (attachScreenshot) theme.onAccent else Color.White,
                                        maxLines = 1
                                    )
                                }
                            }
                        }

                        val displayTitle = screenSnapshot?.candidateTitle
                            ?.takeIf { it.isNotBlank() }
                            ?: preview.candidateTitle
                        Text(
                            text = displayTitle,
                            fontSize = 14.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textPrimary,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis
                        )

                        val displayUrl = screenSnapshot?.extractedUrl
                            ?.takeIf { it.isNotBlank() }
                            ?: preview.extractedUrl
                        if (displayUrl.isNotEmpty()) {
                            Text(
                                text = displayUrl,
                                fontFamily = FontFamily.Monospace,
                                fontSize = 11.5.sp,
                                color = theme.accentForeground,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                        } else if (screenSnapshot != null && screenSnapshot.visibleText.isNotBlank()) {
                            Text(
                                text = screenSnapshot.visibleText.replace("\n", " · "),
                                fontSize = 12.sp,
                                lineHeight = 17.sp,
                                color = theme.textSecondary,
                                maxLines = 2,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                    }
                }

                if (forcePreview && screenSnapshot == null && !accessibilityEnabled) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 48.dp)
                            .clip(RoundedCornerShape(12.dp))
                            .background(theme.surfaceInput)
                            .border(1.dp, theme.border, RoundedCornerShape(12.dp))
                            .clickable { onOpenAccessibilitySettings() }
                            .padding(horizontal = 12.dp, vertical = 10.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "开启「悬浮球识屏」后，无需复制链接即可自动识别页面与截图",
                            fontSize = 12.sp,
                            lineHeight = 17.sp,
                            color = theme.textSecondary,
                            modifier = Modifier.weight(1f)
                        )
                        Spacer(modifier = Modifier.width(8.dp))
                        Text(
                            text = "去开启",
                            fontSize = 12.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.accentForeground,
                            maxLines = 1
                        )
                    }
                }

                if (!hasToken) {
                    Text(
                        text = "尚未配置服务端连接，请先打开主程序一键绑定。",
                        fontSize = 13.sp,
                        color = theme.textSecondary
                    )
                    VesperPrimaryButton(
                        text = "打开 BotHub 一键绑定",
                        minHeight = 48.dp,
                        onClick = onOpenSetup,
                        modifier = Modifier.fillMaxWidth()
                    )
                } else {
                    VesperTextField(
                        value = noteInput,
                        onValueChange = { noteInput = it },
                        placeholder = "随手批注一句想法或提问（可选）...",
                        singleLine = false,
                        maxLines = 3,
                        modifier = Modifier.fillMaxWidth()
                    )

                    if (!forcePreview) {
                        ZeroClickCaptureRow(
                            checked = zeroClick,
                            onCheckedChange = { checked ->
                                zeroClick = checked
                                prefs.zeroClickCaptureEnabled = checked
                            },
                            title = "开启零点击秒收",
                            subtitle = "下次系统分享自动入库并弹 Toast 提示",
                            compact = true
                        )
                    }

                    if (errorMsg.isNotEmpty()) {
                        Text(
                            text = errorMsg,
                            fontSize = 12.sp,
                            color = theme.danger
                        )
                    }

                    VesperPrimaryButton(
                        text = if (saving) "正在收入收藏匣..." else "收入收藏匣",
                        enabled = !busy,
                        minHeight = 48.dp,
                        onClick = { submitCapture() },
                        modifier = Modifier.fillMaxWidth()
                    )

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        VesperSecondaryButton(
                            text = if (savingTodo) "添加中..." else "转为待办",
                            enabled = !busy,
                            minHeight = 46.dp,
                            onClick = { submitTodo() },
                            modifier = Modifier.weight(1f)
                        )

                        VesperSecondaryButton(
                            text = "发给 AI 深聊",
                            enabled = !busy,
                            minHeight = 46.dp,
                            onClick = { onForwardDeepChat(noteInput.trim()) },
                            modifier = Modifier.weight(1f)
                        )
                    }
                }
                Spacer(modifier = Modifier.height(4.dp))
            }
        }
    }
}
