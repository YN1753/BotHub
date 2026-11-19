package uk.starryheavens.vesper.bothub.ui.sheets

import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
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
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import uk.starryheavens.vesper.bothub.BuildConfig
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.UrlUtils
import uk.starryheavens.vesper.bothub.ui.ApkUpdater
import uk.starryheavens.vesper.bothub.ui.BiometricAuthHelper
import uk.starryheavens.vesper.bothub.ui.OtaUpdateState
import uk.starryheavens.vesper.bothub.ui.PasskeyOutcome
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileToggle
import uk.starryheavens.vesper.bothub.ui.components.tactileClickable
import uk.starryheavens.vesper.bothub.ui.theme.BOTHUB_ACCENT_OPTIONS
import uk.starryheavens.vesper.bothub.ui.theme.BOTHUB_COLOR_MODE_OPTIONS
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens
import uk.starryheavens.vesper.bothub.ui.theme.encodeThemePreference
import uk.starryheavens.vesper.bothub.ui.theme.parseThemePreference
import java.io.File
import java.util.Locale

private const val LATEST_APK_URL =
    "https://github.com/YN1753/BotHub/releases/latest/download/vesper-bothub-android.apk"

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsPairingSheet(
    isPaired: Boolean,
    serverUrl: String,
    biometricAppLockEnabled: Boolean,
    realtimeServiceEnabled: Boolean,
    zeroClickCaptureEnabled: Boolean,
    appThemeId: String = "obsidian",
    api: BotHubApiClient,
    onConnected: (String) -> Unit,
    onServerUrlSaved: (String) -> Unit,
    onBiometricLockChanged: (Boolean) -> Unit,
    onRealtimeServiceChanged: (Boolean) -> Unit,
    onZeroClickChanged: (Boolean) -> Unit,
    onThemeSelected: (String) -> Unit = {},
    onOpenModelConfig: () -> Unit = {},
    onTestHealth: (String) -> Unit,
    onUnbindServer: () -> Unit,
    onShowMessage: (String) -> Unit,
    onDismiss: () -> Unit
) {
    val theme = BotHubThemeTokens
    val context = LocalContext.current
    val activity = remember(context) { BiometricAuthHelper.findFragmentActivity(context) }
    val scope = rememberCoroutineScope()
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)

    var urlInput by remember(serverUrl) {
        mutableStateOf(UrlUtils.normalizeServerUrl(serverUrl, UrlUtils.DEFAULT_SERVER_URL))
    }
    var passwordInput by remember { mutableStateOf("") }
    var showPasswordFallback by remember { mutableStateOf(false) }
    var showCustomUrlField by remember { mutableStateOf(false) }
    var oidcAvailable by remember { mutableStateOf(true) }
    var connecting by remember { mutableStateOf(false) }
    var registeringPasskey by remember { mutableStateOf(false) }
    var sheetHint by remember { mutableStateOf("") }
    var remoteVersion by remember { mutableStateOf("") }
    var remoteApkUrl by remember { mutableStateOf(LATEST_APK_URL) }
    var otaState by remember { mutableStateOf<OtaUpdateState>(OtaUpdateState.Idle) }

    val localVersionClean = remember { BuildConfig.VERSION_NAME.removePrefix("v").trim() }
    val remoteVersionClean = remoteVersion.removePrefix("v").trim()
    val hasNewerRelease = remoteVersionClean.isNotEmpty() &&
        remoteVersionClean != "dev" &&
        remoteVersionClean != localVersionClean

    LaunchedEffect(isPaired, urlInput) {
        if (!isPaired) {
            api.fetchOidcStatus(urlInput).onSuccess { enabled ->
                oidcAvailable = enabled
            }
        } else {
            api.fetchAppVersion(serverUrl).onSuccess { info ->
                remoteVersion = info.version
                if (info.apkUrl.isNotEmpty()) {
                    remoteApkUrl = info.apkUrl
                }
            }
        }
    }

    fun installDownloadedApk(apkFile: File) {
        if (!ApkUpdater.canInstallPackages(context)) {
            otaState = OtaUpdateState.NeedPermission(apkFile)
            ApkUpdater.openInstallPermissionSettings(context)
            return
        }
        ApkUpdater.triggerPackageInstaller(context, apkFile)
            .onSuccess {
                otaState = OtaUpdateState.ReadyToInstall(apkFile)
            }
            .onFailure { err ->
                otaState = OtaUpdateState.Failed(err.message ?: "调起安装器失败")
            }
    }

    fun startInAppUpdate(forceDownloadEvenIfSame: Boolean = false) {
        if (otaState is OtaUpdateState.Downloading || otaState is OtaUpdateState.Checking) return
        scope.launch {
            otaState = OtaUpdateState.Checking
            api.fetchAppVersion(serverUrl).onSuccess { info ->
                remoteVersion = info.version
                if (info.apkUrl.isNotEmpty()) {
                    remoteApkUrl = info.apkUrl
                }
            }
            val latestClean = remoteVersion.removePrefix("v").trim()
            if (!forceDownloadEvenIfSame && latestClean.isNotEmpty() && latestClean == localVersionClean) {
                otaState = OtaUpdateState.UpToDate(localVersionClean)
                return@launch
            }

            otaState = OtaUpdateState.Downloading(percent = 1, downloadedMb = 0f, totalMb = 0f)
            ApkUpdater.downloadUpdateApk(
                context = context.applicationContext,
                serverUrl = serverUrl,
                fallbackApkUrl = remoteApkUrl,
                onProgress = { progress -> otaState = progress }
            ).onSuccess { apkFile ->
                installDownloadedApk(apkFile)
            }.onFailure { err ->
                otaState = OtaUpdateState.Failed(err.message ?: "下载失败")
            }
        }
    }

    fun launchVesperAuthOidcPairing() {
        val base = UrlUtils.normalizeServerUrl(urlInput, UrlUtils.DEFAULT_SERVER_URL)
        urlInput = base
        onServerUrlSaved(base)
        sheetHint = ""
        runCatching {
            context.startActivity(
                Intent(Intent.ACTION_VIEW, Uri.parse("$base/api/auth/oidc/login?platform=android"))
            )
        }.onFailure { err ->
            sheetHint = "无法打开浏览器: ${err.message}"
        }
    }

    fun launchBrowserAutoPairBridge() {
        val base = UrlUtils.normalizeServerUrl(urlInput, UrlUtils.DEFAULT_SERVER_URL)
        urlInput = base
        onServerUrlSaved(base)
        runCatching {
            context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse("$base/?auto_pair_app=1")))
        }
    }

    fun runPasskeyConnect() {
        val act = activity
        if (act == null) {
            showPasswordFallback = true
            return
        }
        scope.launch {
            connecting = true
            sheetHint = ""
            val serverHasPasskey = api.fetchPasskeyStatus(urlInput).getOrDefault(false)
            if (serverHasPasskey) {
                when (BiometricAuthHelper.signInWithPasskey(act, api, urlInput)) {
                    is PasskeyOutcome.Success -> {
                        connecting = false
                        onConnected("已登录")
                        return@launch
                    }
                    is PasskeyOutcome.Cancelled -> {
                        connecting = false
                        return@launch
                    }
                    is PasskeyOutcome.Failed -> Unit
                }
            }
            connecting = false
            if (BiometricAuthHelper.canUseLocalBiometrics(context)) {
                BiometricAuthHelper.authenticateLocal(
                    activity = act,
                    title = "登录 BotHub",
                    subtitle = "验证面容或指纹",
                    onSuccess = {
                        if (oidcAvailable) {
                            launchVesperAuthOidcPairing()
                        } else {
                            launchBrowserAutoPairBridge()
                        }
                    },
                    onFallbackOrCancel = {
                        showPasswordFallback = true
                    }
                )
            } else {
                showPasswordFallback = true
            }
        }
    }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
        containerColor = theme.sheetBg,
        scrimColor = theme.scrim,
        tonalElevation = 0.dp,
        dragHandle = { ObsidianSheetDragHandle() }
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = if (isPaired) "设置" else "连接服务端",
                    fontSize = 18.sp,
                    fontWeight = FontWeight.Bold,
                    color = theme.textPrimary,
                    maxLines = 1,
                    softWrap = false
                )
                Text(
                    text = "v$localVersionClean",
                    fontFamily = FontFamily.Monospace,
                    fontSize = 12.sp,
                    color = theme.textMuted,
                    maxLines = 1,
                    softWrap = false
                )
            }

            ThemeSelectorCard(
                activeThemeId = appThemeId,
                onSelect = onThemeSelected
            )

            if (sheetHint.isNotEmpty()) {
                SpecularGlassCard(
                    cornerRadius = 14.dp,
                    accentTint = theme.accent,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Text(
                        text = sheetHint,
                        fontSize = 12.sp,
                        color = theme.textPrimary,
                        modifier = Modifier.padding(12.dp)
                    )
                }
            }

            if (!isPaired) {
                TactilePrimaryButton(
                    text = "Vesper Auth 登录",
                    enabled = !connecting,
                    minHeight = 48.dp,
                    onClick = { launchVesperAuthOidcPairing() },
                    modifier = Modifier.fillMaxWidth()
                )

                TactileSecondaryButton(
                    text = if (connecting) "验证中..." else "通行密钥登录",
                    enabled = !connecting,
                    minHeight = 46.dp,
                    onClick = { runPasskeyConnect() },
                    modifier = Modifier.fillMaxWidth()
                )

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Text(
                        text = if (showPasswordFallback) "收起密钥登录" else "密钥登录",
                        fontSize = 12.sp,
                        color = theme.accent,
                        modifier = Modifier
                            .clickable { showPasswordFallback = !showPasswordFallback }
                            .padding(vertical = 4.dp)
                    )
                    Text(
                        text = if (showCustomUrlField) "收起服务器地址" else "服务器地址",
                        fontSize = 12.sp,
                        color = theme.textMuted,
                        modifier = Modifier
                            .clickable { showCustomUrlField = !showCustomUrlField }
                            .padding(vertical = 4.dp)
                    )
                }

                if (showCustomUrlField) {
                    ObsidianGlassInput(
                        value = urlInput,
                        onValueChange = { urlInput = it },
                        label = "服务端地址",
                        placeholder = "https://bot.starryheavens.uk",
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )
                }

                if (showPasswordFallback) {
                    ObsidianGlassInput(
                        value = passwordInput,
                        onValueChange = { passwordInput = it },
                        label = "访问密钥",
                        placeholder = "输入密钥",
                        visualTransformation = PasswordVisualTransformation(),
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )
                    TactilePrimaryButton(
                        text = if (connecting) "登录中..." else "登录",
                        enabled = !connecting && passwordInput.isNotBlank(),
                        minHeight = 46.dp,
                        onClick = {
                            if (passwordInput.isBlank() || connecting) return@TactilePrimaryButton
                            scope.launch {
                                connecting = true
                                api.exchangeToken(urlInput, passwordInput).onSuccess {
                                    passwordInput = ""
                                    onConnected("已连接")
                                }.onFailure { err ->
                                    sheetHint = err.message ?: "密钥验证失败"
                                }
                                connecting = false
                            }
                        },
                        modifier = Modifier.fillMaxWidth()
                    )
                }
            } else {
                // 1. Connected Server Row
                SpecularGlassCard(
                    cornerRadius = 17.dp,
                    accentTint = Color(0xFF34D399),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(7.dp)
                                    .background(Color(0xFF34D399), CircleShape)
                            )
                            Text(
                                text = UrlUtils.formatDisplayHost(serverUrl),
                                fontFamily = FontFamily.Monospace,
                                fontSize = 13.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textPrimary,
                                maxLines = 1,
                                softWrap = false,
                                overflow = TextOverflow.Ellipsis
                            )
                        }

                        TactileSecondaryButton(
                            text = if (registeringPasskey) "录入中" else "通行密钥",
                            accent = true,
                            cornerRadius = 999.dp,
                            minHeight = 33.dp,
                            contentPadding = PaddingValues(horizontal = 11.dp, vertical = 5.dp),
                            onClick = {
                                val act = activity ?: return@TactileSecondaryButton
                                if (registeringPasskey) return@TactileSecondaryButton
                                scope.launch {
                                    registeringPasskey = true
                                    sheetHint = ""
                                    when (val out = BiometricAuthHelper.registerPasskey(act, api)) {
                                        is PasskeyOutcome.Success -> {
                                            onShowMessage("已录入通行密钥")
                                        }
                                        is PasskeyOutcome.Cancelled -> Unit
                                        is PasskeyOutcome.Failed -> {
                                            sheetHint = out.message
                                        }
                                    }
                                    registeringPasskey = false
                                }
                            }
                        )
                    }
                }

                // 2. Native In-App OTA Update Card (Zero Browser Redirect!)
                if (hasNewerRelease || otaState !is OtaUpdateState.Idle) {
                    InAppOtaStatusCard(
                        otaState = otaState,
                        remoteVersionClean = remoteVersionClean,
                        onStartDownload = { startInAppUpdate(forceDownloadEvenIfSame = true) },
                        onInstallFile = { file -> installDownloadedApk(file) }
                    )
                }

                // 3. Clean Concise Toggles
                SpecularGlassCard(
                    cornerRadius = 18.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(modifier = Modifier.fillMaxWidth()) {
                        SettingsToggleRow(
                            title = "应用锁",
                            subtitle = "打开应用时验证面容或指纹",
                            checked = biometricAppLockEnabled,
                            onCheckedChange = { checked ->
                                if (!checked) {
                                    onBiometricLockChanged(false)
                                    return@SettingsToggleRow
                                }
                                val act = activity
                                if (act == null || !BiometricAuthHelper.canUseLocalBiometrics(context)) {
                                    sheetHint = "请先在系统设置中录入面容或指纹"
                                    return@SettingsToggleRow
                                }
                                BiometricAuthHelper.authenticateLocal(
                                    activity = act,
                                    title = "开启应用锁",
                                    subtitle = "验证面容或指纹",
                                    onSuccess = { onBiometricLockChanged(true) },
                                    onFallbackOrCancel = { msg -> sheetHint = msg }
                                )
                            }
                        )

                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = theme.divider,
                            modifier = Modifier.padding(horizontal = 14.dp)
                        )

                        SettingsToggleRow(
                            title = "后台消息通知",
                            subtitle = "退至后台时接收新消息提醒",
                            checked = realtimeServiceEnabled,
                            onCheckedChange = onRealtimeServiceChanged
                        )

                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = theme.divider,
                            modifier = Modifier.padding(horizontal = 14.dp)
                        )

                        SettingsToggleRow(
                            title = "系统分享直接收藏",
                            subtitle = "分享内容自动存入收藏",
                            checked = zeroClickCaptureEnabled,
                            onCheckedChange = onZeroClickChanged
                        )
                    }
                }

                // 3.5 AI Model Channels & Dual Pools Entry
                SpecularGlassCard(
                    cornerRadius = 18.dp,
                    modifier = Modifier.fillMaxWidth(),
                    onClick = onOpenModelConfig
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 15.dp, vertical = 13.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = "AI 模型渠道与双池",
                                fontSize = 14.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.textPrimary
                            )
                            Text(
                                text = "管理渠道 Key、免费/付费模型池、调度策略与测速",
                                fontSize = 11.5.sp,
                                color = theme.textMuted
                            )
                        }
                        Text(
                            text = "配置 >",
                            fontSize = 12.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.accent
                        )
                    }
                }

                // 4. Clean Action Buttons
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    TactileSecondaryButton(
                        text = "连接测试",
                        minHeight = 44.dp,
                        onClick = { onTestHealth(urlInput) },
                        modifier = Modifier.weight(1f)
                    )
                    TactileSecondaryButton(
                        text = "省电白名单",
                        minHeight = 44.dp,
                        onClick = {
                            val batIntent = Intent(
                                Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS,
                                Uri.parse("package:${context.packageName}")
                            )
                            runCatching { context.startActivity(batIntent) }.onFailure {
                                runCatching {
                                    context.startActivity(
                                        Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS)
                                    )
                                }
                            }
                        },
                        modifier = Modifier.weight(1f)
                    )
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    TactileSecondaryButton(
                        text = when (otaState) {
                            is OtaUpdateState.Downloading -> "下载中..."
                            is OtaUpdateState.Checking -> "检查中..."
                            else -> if (hasNewerRelease) "立即更新" else "检查更新"
                        },
                        accent = hasNewerRelease,
                        enabled = otaState !is OtaUpdateState.Downloading && otaState !is OtaUpdateState.Checking,
                        minHeight = 44.dp,
                        onClick = { startInAppUpdate(forceDownloadEvenIfSame = false) },
                        modifier = Modifier.weight(1f)
                    )
                    TactileSecondaryButton(
                        text = "退出登录",
                        danger = true,
                        minHeight = 44.dp,
                        onClick = onUnbindServer,
                        modifier = Modifier.weight(1f)
                    )
                }
            }

            Spacer(modifier = Modifier.height(10.dp))
        }
    }
}

@Composable
private fun ThemeSelectorCard(
    activeThemeId: String,
    onSelect: (String) -> Unit
) {
    val theme = BotHubThemeTokens
    val (activeMode, activeAccent) = remember(activeThemeId) {
        parseThemePreference(activeThemeId)
    }

    SpecularGlassCard(
        cornerRadius = 12.dp,
        modifier = Modifier.fillMaxWidth()
    ) {
        Column(
            modifier = Modifier.padding(top = 12.dp, bottom = 14.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 14.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "外观主题",
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = theme.textPrimary,
                    maxLines = 1,
                    softWrap = false
                )
                Text(
                    text = theme.label,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Medium,
                    color = theme.textSecondary,
                    maxLines = 1,
                    softWrap = false
                )
            }

            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 14.dp),
                verticalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                Text(
                    text = "外观模式",
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Medium,
                    color = theme.textMuted,
                    maxLines = 1,
                    softWrap = false
                )
                val segOuterShape = RoundedCornerShape(10.dp)
                val segTabShape = RoundedCornerShape(8.dp)
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clip(segOuterShape)
                        .background(theme.inputTop, segOuterShape)
                        .border(1.dp, theme.divider, segOuterShape)
                        .padding(3.dp),
                    horizontalArrangement = Arrangement.spacedBy(3.dp)
                ) {
                    BOTHUB_COLOR_MODE_OPTIONS.forEach { modeOpt ->
                        val selected = modeOpt.id == activeMode
                        Box(
                            modifier = Modifier
                                .weight(1f)
                                .sizeIn(minHeight = 34.dp)
                                .tactileClickable(pressedScale = 0.96f, hapticOnTap = true) {
                                    if (!selected) {
                                        onSelect(encodeThemePreference(modeOpt.id, activeAccent))
                                    }
                                }
                                .clip(segTabShape)
                                .then(
                                    if (selected) {
                                        Modifier
                                            .background(theme.cardTop, segTabShape)
                                            .border(1.dp, theme.rimHighlight, segTabShape)
                                    } else {
                                        Modifier
                                    }
                                )
                                .padding(horizontal = 8.dp, vertical = 6.dp),
                            contentAlignment = Alignment.Center
                        ) {
                            Text(
                                text = modeOpt.label,
                                fontSize = 12.5.sp,
                                fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                                color = if (selected) theme.textPrimary else theme.textMuted,
                                maxLines = 1,
                                softWrap = false
                            )
                        }
                    }
                }
            }

            Column(
                modifier = Modifier.fillMaxWidth(),
                verticalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                Text(
                    text = "强调色",
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Medium,
                    color = theme.textMuted,
                    maxLines = 1,
                    softWrap = false,
                    modifier = Modifier.padding(horizontal = 14.dp)
                )
                LazyRow(
                    contentPadding = PaddingValues(horizontal = 14.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    items(BOTHUB_ACCENT_OPTIONS, key = { it.id }) { opt ->
                        val chosen = opt.id == activeAccent
                        val swatchColor = if (theme.isLight) opt.lightSwatch else opt.darkSwatch
                        val pillShape = RoundedCornerShape(10.dp)
                        Row(
                            modifier = Modifier
                                .tactileClickable(pressedScale = 0.95f, hapticOnTap = true) {
                                    if (!chosen) {
                                        onSelect(encodeThemePreference(activeMode, opt.id))
                                    }
                                }
                                .clip(pillShape)
                                .background(
                                    if (chosen) {
                                        swatchColor.copy(alpha = if (theme.isLight) 0.12f else 0.16f)
                                    } else {
                                        theme.inputTop
                                    },
                                    pillShape
                                )
                                .border(
                                    BorderStroke(
                                        if (chosen) 1.2.dp else 1.dp,
                                        if (chosen) swatchColor else theme.divider
                                    ),
                                    pillShape
                                )
                                .padding(horizontal = 11.dp, vertical = 7.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(6.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(13.dp)
                                    .clip(CircleShape)
                                    .background(swatchColor, CircleShape)
                                    .border(
                                        1.dp,
                                        if (theme.isLight) Color(0x26000000) else Color(0x33FFFFFF),
                                        CircleShape
                                    )
                            )
                            Text(
                                text = opt.label,
                                fontSize = 12.sp,
                                fontWeight = if (chosen) FontWeight.SemiBold else FontWeight.Medium,
                                color = if (chosen) theme.textPrimary else theme.textSecondary,
                                maxLines = 1,
                                softWrap = false
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun InAppOtaStatusCard(
    otaState: OtaUpdateState,
    remoteVersionClean: String,
    onStartDownload: () -> Unit,
    onInstallFile: (File) -> Unit
) {
    val theme = BotHubThemeTokens
    val accent = when (otaState) {
        is OtaUpdateState.Failed -> Color(0xFFFB7185)
        is OtaUpdateState.UpToDate -> Color(0xFF10B981)
        else -> theme.accent
    }

    SpecularGlassCard(
        cornerRadius = 16.dp,
        accentTint = accent,
        modifier = Modifier.fillMaxWidth()
    ) {
        Column(
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            when (otaState) {
                is OtaUpdateState.Idle -> {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "发现新版本 v$remoteVersionClean",
                            fontSize = 13.5.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textPrimary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        )
                        TactilePrimaryButton(
                            text = "立即更新",
                            cornerRadius = 999.dp,
                            minHeight = 33.dp,
                            contentPadding = PaddingValues(horizontal = 13.dp, vertical = 5.dp),
                            onClick = onStartDownload
                        )
                    }
                }

                is OtaUpdateState.Checking -> {
                    Text(
                        text = "正在检查新版本...",
                        fontSize = 13.sp,
                        color = theme.textPrimary
                    )
                }

                is OtaUpdateState.Downloading -> {
                    val fraction by animateFloatAsState(
                        targetValue = if (otaState.percent > 0) {
                            (otaState.percent / 100f).coerceIn(0.04f, 1f)
                        } else {
                            0.35f
                        },
                        animationSpec = tween(120),
                        label = "otaProgress"
                    )
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = if (otaState.percent > 0) "正在下载更新 ${otaState.percent}%" else "正在下载更新...",
                            fontSize = 13.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = theme.textPrimary,
                            maxLines = 1,
                            softWrap = false
                        )
                        val sizeLabel = if (otaState.totalMb > 0f) {
                            String.format(Locale.US, "%.1f / %.1f MB", otaState.downloadedMb, otaState.totalMb)
                        } else {
                            String.format(Locale.US, "%.1f MB", otaState.downloadedMb)
                        }
                        Text(
                            text = sizeLabel,
                            fontFamily = FontFamily.Monospace,
                            fontSize = 11.sp,
                            color = theme.textMuted,
                            maxLines = 1,
                            softWrap = false
                        )
                    }
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(5.dp)
                            .clip(RoundedCornerShape(999.dp))
                            .background(theme.inputTop)
                    ) {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth(fraction)
                                .height(5.dp)
                                .clip(RoundedCornerShape(999.dp))
                                .background(
                                    Brush.horizontalGradient(
                                        listOf(theme.accentDeep, theme.accent)
                                    )
                                )
                        )
                    }
                }

                is OtaUpdateState.NeedPermission -> {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "授权安装应用后点击继续",
                            fontSize = 13.sp,
                            color = theme.textPrimary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        )
                        TactilePrimaryButton(
                            text = "继续安装",
                            cornerRadius = 999.dp,
                            minHeight = 33.dp,
                            contentPadding = PaddingValues(horizontal = 13.dp, vertical = 5.dp),
                            onClick = { onInstallFile(otaState.apkFile) }
                        )
                    }
                }

                is OtaUpdateState.ReadyToInstall -> {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "下载完成",
                            fontSize = 13.sp,
                            fontWeight = FontWeight.Medium,
                            color = theme.textPrimary
                        )
                        TactilePrimaryButton(
                            text = "立即安装",
                            cornerRadius = 999.dp,
                            minHeight = 33.dp,
                            contentPadding = PaddingValues(horizontal = 13.dp, vertical = 5.dp),
                            onClick = { onInstallFile(otaState.apkFile) }
                        )
                    }
                }

                is OtaUpdateState.UpToDate -> {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "当前已是最新版本 (v${otaState.version})",
                            fontSize = 12.5.sp,
                            color = Color(0xFF10B981),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.weight(1f)
                        )
                        Text(
                            text = "重下覆盖",
                            fontSize = 11.5.sp,
                            color = theme.textMuted,
                            maxLines = 1,
                            softWrap = false,
                            modifier = Modifier.clickable { onStartDownload() }
                        )
                    }
                }

                is OtaUpdateState.Failed -> {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "更新失败: ${otaState.message}",
                            fontSize = 12.5.sp,
                            color = if (theme.isLight) Color(0xFFE11D48) else Color(0xFFFDA4AF),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 10.dp)
                        )
                        TactileSecondaryButton(
                            text = "重试",
                            cornerRadius = 999.dp,
                            minHeight = 32.dp,
                            contentPadding = PaddingValues(horizontal = 12.dp, vertical = 5.dp),
                            onClick = onStartDownload
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun SettingsToggleRow(
    title: String,
    subtitle: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit
) {
    val theme = BotHubThemeTokens
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onCheckedChange(!checked) }
            .padding(horizontal = 15.dp, vertical = 12.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically
    ) {
        Column(
            modifier = Modifier
                .weight(1f)
                .padding(end = 12.dp),
            verticalArrangement = Arrangement.spacedBy(2.dp)
        ) {
            Text(
                text = title,
                fontSize = 14.5.sp,
                fontWeight = FontWeight.Medium,
                color = theme.textPrimary,
                maxLines = 1,
                softWrap = false,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = subtitle,
                fontSize = 11.5.sp,
                color = theme.textMuted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }
        TactileToggle(
            checked = checked,
            onCheckedChange = onCheckedChange
        )
    }
}
