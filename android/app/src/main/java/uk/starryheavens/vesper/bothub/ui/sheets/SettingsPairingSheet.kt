package uk.starryheavens.vesper.bothub.ui.sheets

import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Surface
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
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.PasskeyOutcome
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSheetDragHandle
import uk.starryheavens.vesper.bothub.ui.components.ObsidianSwitch

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
    onOpenConsole: (String, String) -> Unit = { _, _ -> },
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
            sheetHint = "无法拉起系统浏览器: ${err.message}"
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
                        onConnected("无感面容 / 指纹验证通过，实时通知已开启")
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
                    title = "无感激活 BotHub 实时通知",
                    subtitle = "优先前置无感面容识别，不支持面容自动降级指纹",
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
        shape = RoundedCornerShape(topStart = 26.dp, topEnd = 26.dp),
        containerColor = Color(0xFF090E1D),
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
            // Sheet Title Header
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                modifier = Modifier.fillMaxWidth()
            ) {
                GrokBotAvatar(
                    avatarStyle = "butler:gold",
                    state = if (connecting || registeringPasskey) "working" else "idle",
                    size = 36.dp,
                    interactive = true,
                    seedKey = "settings_spirit"
                )
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = if (isPaired) "实时守护与连接设置" else "一键激活 BotHub 手机端",
                        style = MaterialTheme.typography.titleMedium,
                        fontSize = 16.5.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = Color(0xFFFFF8EE)
                    )
                    Text(
                        text = if (isPaired) {
                            "客户端 v$localVersionClean" +
                                if (remoteVersionClean.isNotEmpty()) " · 服务端 v$remoteVersionClean" else ""
                        } else {
                            "首选 Vesper Auth 单点免密绑定 · 目标 ${UrlUtils.formatDisplayHost(urlInput)}"
                        },
                        fontFamily = FontFamily.Monospace,
                        fontSize = 11.sp,
                        color = Color(0xFF94A3B8)
                    )
                }
            }

            if (sheetHint.isNotEmpty()) {
                Surface(
                    shape = RoundedCornerShape(12.dp),
                    color = Color(0xFF141D38),
                    border = BorderStroke(1.dp, Color(0x5CF2B880)),
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
                Button(
                    onClick = { launchVesperAuthOidcPairing() },
                    enabled = !connecting,
                    shape = RoundedCornerShape(14.dp),
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 48.dp)
                ) {
                    Text(
                        text = "使用 Vesper Auth 通行证一键绑定",
                        fontSize = 14.sp,
                        fontWeight = FontWeight.SemiBold
                    )
                }

                Surface(
                    onClick = { if (!connecting) runPasskeyConnect() },
                    shape = RoundedCornerShape(14.dp),
                    color = Color(0xFF11182E),
                    border = BorderStroke(1.dp, Color(0x2EFFF8EE)),
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 46.dp)
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        Text(
                            text = if (connecting) "正在调起面容 / 指纹..." else "面容 / 指纹通行密钥直连",
                            fontSize = 13.5.sp,
                            fontWeight = FontWeight.Medium,
                            color = Color(0xFFE2E8F0)
                        )
                    }
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Text(
                        text = if (showPasswordFallback) "收起访问密钥表单" else "改用访问密钥绑定",
                        fontSize = 12.sp,
                        color = Color(0xFFFFE2BE),
                        modifier = Modifier
                            .clickable { showPasswordFallback = !showPasswordFallback }
                            .padding(vertical = 4.dp)
                    )
                    Text(
                        text = if (showCustomUrlField) "收起服务器地址" else "修改服务器地址",
                        fontSize = 12.sp,
                        color = Color(0xFF94A3B8),
                        modifier = Modifier
                            .clickable { showCustomUrlField = !showCustomUrlField }
                            .padding(vertical = 4.dp)
                    )
                }

                if (showCustomUrlField) {
                    ObsidianGlassInput(
                        value = urlInput,
                        onValueChange = { urlInput = it },
                        label = "BotHub 服务端地址",
                        placeholder = "https://bot.starryheavens.uk",
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )
                }

                if (showPasswordFallback) {
                    ObsidianGlassInput(
                        value = passwordInput,
                        onValueChange = { passwordInput = it },
                        label = "控制台访问密钥（备用兜底）",
                        placeholder = "输入访问密钥...",
                        visualTransformation = PasswordVisualTransformation(),
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )
                    Button(
                        onClick = {
                            if (passwordInput.isBlank() || connecting) return@Button
                            scope.launch {
                                connecting = true
                                api.exchangeToken(urlInput, passwordInput).onSuccess {
                                    passwordInput = ""
                                    onConnected("已连接并自动开启锁屏实时通知")
                                }.onFailure { err ->
                                    sheetHint = err.message ?: "密钥验证失败"
                                }
                                connecting = false
                            }
                        },
                        enabled = !connecting && passwordInput.isNotBlank(),
                        shape = RoundedCornerShape(14.dp),
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 46.dp)
                    ) {
                        Text(if (connecting) "验证并开启通知中..." else "验证密钥并开启通知")
                    }
                }
            } else {
                // 1. Connected Server Node + Compact Passkey Enrollment Pill
                Surface(
                    shape = RoundedCornerShape(16.dp),
                    color = Color(0xCC0D1326),
                    border = BorderStroke(1.dp, Color(0x26FFF8EE)),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 11.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                            modifier = Modifier
                                .weight(1f)
                                .padding(end = 8.dp)
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(7.dp)
                                    .background(Color(0xFF34D399), CircleShape)
                            )
                            Column {
                                Text(
                                    text = UrlUtils.formatDisplayHost(serverUrl),
                                    fontFamily = FontFamily.Monospace,
                                    fontSize = 13.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = Color(0xFFFFF8EE),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                                Text(
                                    text = "免密令牌已绑定 · 锁屏推送就绪",
                                    fontSize = 11.sp,
                                    color = Color(0xFF94A3B8)
                                )
                            }
                        }

                        Surface(
                            onClick = {
                                val act = activity ?: return@Surface
                                if (registeringPasskey) return@Surface
                                scope.launch {
                                    registeringPasskey = true
                                    sheetHint = ""
                                    when (val out = BiometricAuthHelper.registerPasskey(act, api)) {
                                        is PasskeyOutcome.Success -> {
                                            onShowMessage("已录入本机通行密钥，重装也可刷脸秒连")
                                        }
                                        is PasskeyOutcome.Cancelled -> Unit
                                        is PasskeyOutcome.Failed -> {
                                            sheetHint = out.message
                                        }
                                    }
                                    registeringPasskey = false
                                }
                            },
                            shape = RoundedCornerShape(999.dp),
                            color = Color(0x22F2B880),
                            border = BorderStroke(1.dp, Color(0x5CF2B880)),
                            modifier = Modifier.sizeIn(minHeight = 34.dp)
                        ) {
                            Box(
                                contentAlignment = Alignment.Center,
                                modifier = Modifier.padding(horizontal = 11.dp, vertical = 6.dp)
                            ) {
                                Text(
                                    text = if (registeringPasskey) "录入中..." else "录入通行密钥",
                                    fontSize = 11.5.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = Color(0xFFFFE2BE)
                                )
                            }
                        }
                    }
                }

                // Optional New Version Banner
                if (hasNewerRelease) {
                    Surface(
                        onClick = {
                            runCatching {
                                context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(remoteApkUrl)))
                            }
                        },
                        shape = RoundedCornerShape(14.dp),
                        color = Color(0x24F2B880),
                        border = BorderStroke(1.dp, Color(0x73F2B880)),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.SpaceBetween
                        ) {
                            Text(
                                text = "发现新版本 v$remoteVersionClean · 点击拉取安装包",
                                fontSize = 12.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFFFFE2BE)
                            )
                            Icon(
                                Icons.Default.KeyboardArrowRight,
                                contentDescription = null,
                                tint = Color(0xFFFFE2BE),
                                modifier = Modifier.size(16.dp)
                            )
                        }
                    }
                }

                // 2. Unified Grouped Obsidian Glass Toggles (Zero disjointed raw ListItems)
                Surface(
                    shape = RoundedCornerShape(18.dp),
                    color = Color(0xCC0D1326),
                    border = BorderStroke(1.dp, Color(0x22FFF8EE)),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Column(modifier = Modifier.fillMaxWidth()) {
                        SettingsToggleRow(
                            title = "启动无感面容 / 指纹锁",
                            subtitle = "优先前置面容解锁，不影响后台锁屏收通知",
                            checked = biometricAppLockEnabled,
                            onCheckedChange = { checked ->
                                if (!checked) {
                                    onBiometricLockChanged(false)
                                    return@SettingsToggleRow
                                }
                                val act = activity
                                if (act == null || !BiometricAuthHelper.canUseLocalBiometrics(context)) {
                                    sheetHint = "当前设备尚未在系统设置中录入面容或指纹"
                                    return@SettingsToggleRow
                                }
                                BiometricAuthHelper.authenticateLocal(
                                    activity = act,
                                    title = "开启无感面容 / 指纹保护",
                                    subtitle = "验证一次本机面容或指纹即可启用",
                                    onSuccess = { onBiometricLockChanged(true) },
                                    onFallbackOrCancel = { msg -> sheetHint = msg }
                                )
                            }
                        )

                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = Color(0x18FFF8EE),
                            modifier = Modifier.padding(horizontal = 14.dp)
                        )

                        SettingsToggleRow(
                            title = "常驻 SSE 实时秒推服务",
                            subtitle = "默认开启，锁屏毫秒级直弹服务端情报与告警",
                            checked = realtimeServiceEnabled,
                            onCheckedChange = onRealtimeServiceChanged
                        )

                        HorizontalDivider(
                            thickness = 0.5.dp,
                            color = Color(0x18FFF8EE),
                            modifier = Modifier.padding(horizontal = 14.dp)
                        )

                        SettingsToggleRow(
                            title = "系统分享零点击秒收",
                            subtitle = "任意应用点系统分享即自动入库并秒回原应用",
                            checked = zeroClickCaptureEnabled,
                            onCheckedChange = onZeroClickChanged
                        )
                    }
                }

                // 3. Full-Featured Hybrid Web Console Entry
                Surface(
                    onClick = {
                        onDismiss()
                        onOpenConsole("settings", "全功能系统控制台")
                    },
                    shape = RoundedCornerShape(16.dp),
                    color = Color(0xCC0E152B),
                    border = BorderStroke(1.dp, Color(0x33F2B880)),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 11.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = "打开全功能系统控制台",
                                fontSize = 13.5.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = Color(0xFFFFE2BE)
                            )
                            Text(
                                text = "模型池路由 · 技能工坊 · 长期记忆 · 备份与安全",
                                fontSize = 11.sp,
                                color = Color(0xFF94A3B8)
                            )
                        }
                        Icon(
                            Icons.Default.KeyboardArrowRight,
                            contentDescription = null,
                            tint = Color(0xFFF2B880),
                            modifier = Modifier.size(18.dp)
                        )
                    }
                }

                // 4. Compact 2x2 Tactile Maintenance Grid
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    SettingsGridActionBtn(
                        label = "测试连通性",
                        modifier = Modifier.weight(1f),
                        onClick = { onTestHealth(urlInput) }
                    )
                    SettingsGridActionBtn(
                        label = "保活白名单",
                        modifier = Modifier.weight(1f),
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
                        }
                    )
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    SettingsGridActionBtn(
                        label = "拉取最新 APK",
                        modifier = Modifier.weight(1f),
                        onClick = {
                            runCatching {
                                context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(remoteApkUrl)))
                            }
                        }
                    )
                    SettingsGridActionBtn(
                        label = "重新绑定服务端",
                        danger = true,
                        modifier = Modifier.weight(1f),
                        onClick = onUnbindServer
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
            .padding(horizontal = 14.dp, vertical = 11.dp),
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
                fontSize = 14.sp,
                fontWeight = FontWeight.Medium,
                color = Color(0xFFFFF8EE),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = subtitle,
                fontSize = 11.5.sp,
                lineHeight = 16.sp,
                color = Color(0xFF94A3B8),
                maxLines = 2,
                overflow = TextOverflow.Ellipsis
            )
        }
        ObsidianSwitch(
            checked = checked,
            onCheckedChange = onCheckedChange
        )
    }
}

@Composable
private fun SettingsGridActionBtn(
    label: String,
    modifier: Modifier = Modifier,
    danger: Boolean = false,
    onClick: () -> Unit
) {
    Surface(
        onClick = onClick,
        shape = RoundedCornerShape(14.dp),
        color = if (danger) Color(0x1AF43F5E) else Color(0xFF10172C),
        border = BorderStroke(
            1.dp,
            if (danger) Color(0x4DFB7185) else Color(0x24FFF8EE)
        ),
        modifier = modifier.sizeIn(minHeight = 44.dp)
    ) {
        Box(
            contentAlignment = Alignment.Center,
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 10.dp)
        ) {
            Text(
                text = label,
                fontSize = 12.5.sp,
                fontWeight = FontWeight.Medium,
                color = if (danger) Color(0xFFFDA4AF) else Color(0xFFE2E8F0),
                maxLines = 1
            )
        }
    }
}
