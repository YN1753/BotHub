package uk.starryheavens.vesper.bothub.ui.sheets

import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.compose.foundation.background
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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
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
import uk.starryheavens.vesper.bothub.ui.BiometricAuthHelper
import uk.starryheavens.vesper.bothub.ui.PasskeyOutcome
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.SpecularGlassCard
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileToggle

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
    api: BotHubApiClient,
    onConnected: (String) -> Unit,
    onServerUrlSaved: (String) -> Unit,
    onBiometricLockChanged: (Boolean) -> Unit,
    onRealtimeServiceChanged: (Boolean) -> Unit,
    onZeroClickChanged: (Boolean) -> Unit,
    onTestHealth: (String) -> Unit,
    onUnbindServer: () -> Unit,
    onShowMessage: (String) -> Unit,
    onDismiss: () -> Unit
) {
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
        containerColor = Color(0xFF080D1B),
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
                    color = Color(0xFFFFF8EE),
                    maxLines = 1,
                    softWrap = false
                )
                Text(
                    text = "v$localVersionClean",
                    fontFamily = FontFamily.Monospace,
                    fontSize = 12.sp,
                    color = Color(0xFF64748B),
                    maxLines = 1,
                    softWrap = false
                )
            }

            if (sheetHint.isNotEmpty()) {
                SpecularGlassCard(
                    cornerRadius = 14.dp,
                    accentTint = Color(0xFFF2B880),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Text(
                        text = sheetHint,
                        fontSize = 12.sp,
                        color = Color(0xFFFFF8EE),
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
                        color = Color(0xFFFFE2BE),
                        modifier = Modifier
                            .clickable { showPasswordFallback = !showPasswordFallback }
                            .padding(vertical = 4.dp)
                    )
                    Text(
                        text = if (showCustomUrlField) "收起服务器地址" else "服务器地址",
                        fontSize = 12.sp,
                        color = Color(0xFF8C9BB5),
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
                // 1. Connected Server Row (Clean, zero wrapping!)
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
                                color = Color(0xFFFFF8EE),
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

                if (hasNewerRelease) {
                    SpecularGlassCard(
                        cornerRadius = 15.dp,
                        accentTint = Color(0xFFF2B880),
                        onClick = {
                            runCatching {
                                context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(remoteApkUrl)))
                            }
                        },
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 11.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.SpaceBetween
                        ) {
                            Text(
                                text = "发现新版本 v$remoteVersionClean",
                                fontSize = 13.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFFFFE2BE),
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                                modifier = Modifier.weight(1f)
                            )
                            Icon(
                                imageVector = Icons.Default.KeyboardArrowRight,
                                contentDescription = null,
                                tint = Color(0xFFFFE2BE),
                                modifier = Modifier.size(16.dp)
                            )
                        }
                    }
                }

                // 2. Clean Concise Toggles (Zero technical jargon!)
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
                            color = Color(0x14FFF8EE),
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
                            color = Color(0x14FFF8EE),
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

                // 3. Clean Action Buttons
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
                        text = "检查更新",
                        minHeight = 44.dp,
                        onClick = {
                            runCatching {
                                context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(remoteApkUrl)))
                            }
                        },
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
private fun SettingsToggleRow(
    title: String,
    subtitle: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit
) {
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
                color = Color(0xFFFFF8EE),
                maxLines = 1,
                softWrap = false,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = subtitle,
                fontSize = 11.5.sp,
                color = Color(0xFF8C9BB5),
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
