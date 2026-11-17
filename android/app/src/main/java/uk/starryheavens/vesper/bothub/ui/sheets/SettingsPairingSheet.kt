package uk.starryheavens.vesper.bothub.ui.sheets

import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import uk.starryheavens.vesper.bothub.data.BotHubApiClient
import uk.starryheavens.vesper.bothub.data.UrlUtils
import uk.starryheavens.vesper.bothub.ui.BiometricAuthHelper
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar
import uk.starryheavens.vesper.bothub.ui.PasskeyOutcome
import uk.starryheavens.vesper.bothub.ui.components.ZeroClickCaptureRow

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

    var urlInput by remember(serverUrl) { mutableStateOf(serverUrl) }
    var passwordInput by remember { mutableStateOf("") }
    var showPasswordFallback by remember { mutableStateOf(false) }
    var showCustomUrlField by remember { mutableStateOf(false) }
    var connecting by remember { mutableStateOf(false) }
    var registeringPasskey by remember { mutableStateOf(false) }
    var sheetHint by remember { mutableStateOf("") }

    fun launchBrowserAutoPairBridge() {
        val base = UrlUtils.normalizeServerUrl(urlInput, serverUrl)
        onServerUrlSaved(base)
        runCatching {
            context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse("$base/?auto_pair_app=1")))
        }
    }

    fun runPasskeyConnect(autoTriggered: Boolean) {
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
                    is PasskeyOutcome.Failed -> {
                        // Fall through to local passive Face -> Fingerprint + Auto-Pair Bridge
                    }
                }
            }
            connecting = false
            if (BiometricAuthHelper.canUseLocalBiometrics(context)) {
                BiometricAuthHelper.authenticateLocal(
                    activity = act,
                    title = "无感激活 Vesper 实时通知",
                    subtitle = "优先前置无感面容识别，不支持面容自动降级指纹",
                    onSuccess = {
                        launchBrowserAutoPairBridge()
                    },
                    onFallbackOrCancel = {
                        if (!autoTriggered) {
                            showPasswordFallback = true
                        }
                    }
                )
            } else if (!autoTriggered) {
                showPasswordFallback = true
            }
        }
    }

    // Auto-attempt passive Face -> Fingerprint -> Auto-Pair on fresh install
    LaunchedEffect(Unit) {
        if (!isPaired && activity != null) {
            runPasskeyConnect(autoTriggered = true)
        }
    }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        shape = RoundedCornerShape(topStart = 24.dp, topEnd = 24.dp),
        containerColor = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                GrokBotAvatar(
                    avatarStyle = "butler:gold",
                    state = if (connecting || registeringPasskey) "working" else "idle",
                    size = 38.dp,
                    interactive = true,
                    seedKey = "settings_spirit"
                )
                Column {
                    Text(
                        text = if (isPaired) "实时通知与生物识别设置" else "无感激活手机实时通知",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold
                    )
                    Text(
                        text = if (isPaired) {
                            "已连接 ${UrlUtils.formatDisplayHost(serverUrl)} · 无需配置任何 Token"
                        } else {
                            "优先面容无感识别 -> 指纹 -> 网页一键绑定 / 密码兜底"
                        },
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }

            if (sheetHint.isNotEmpty()) {
                Surface(
                    shape = RoundedCornerShape(12.dp),
                    color = MaterialTheme.colorScheme.secondaryContainer,
                    border = BorderStroke(1.dp, MaterialTheme.colorScheme.primary.copy(alpha = 0.35f)),
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Text(
                        text = sheetHint,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        modifier = Modifier.padding(12.dp)
                    )
                }
            }

            if (!isPaired) {
                Button(
                    onClick = { runPasskeyConnect(autoTriggered = false) },
                    enabled = !connecting,
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 48.dp)
                ) {
                    Text(if (connecting) "正在调起面容 / 指纹..." else "面容 / 指纹无感连接")
                }

                OutlinedButton(
                    onClick = { launchBrowserAutoPairBridge() },
                    enabled = !connecting,
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 48.dp)
                ) {
                    Text("从已登录手机浏览器一键自动回跳绑定")
                }

                if (!showPasswordFallback) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Text(
                            text = "改用访问密码连接（兜底）",
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.primary,
                            modifier = Modifier
                                .clickable { showPasswordFallback = true }
                                .padding(vertical = 6.dp)
                        )
                        Text(
                            text = if (showCustomUrlField) "收起服务器地址" else "修改服务器地址",
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier
                                .clickable { showCustomUrlField = !showCustomUrlField }
                                .padding(vertical = 6.dp)
                        )
                    }
                }

                if (showCustomUrlField) {
                    OutlinedTextField(
                        value = urlInput,
                        onValueChange = { urlInput = it },
                        label = { Text("BotHub 服务端地址") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )
                }

                if (showPasswordFallback) {
                    OutlinedTextField(
                        value = passwordInput,
                        onValueChange = { passwordInput = it },
                        label = { Text("控制台访问密码（仅首次兜底，无需填 Token）") },
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
                                    sheetHint = err.message ?: "密码验证失败"
                                }
                                connecting = false
                            }
                        },
                        enabled = !connecting && passwordInput.isNotBlank(),
                        modifier = Modifier
                            .fillMaxWidth()
                            .sizeIn(minHeight = 48.dp)
                    ) {
                        Text(if (connecting) "验证并开启通知中..." else "验证密码并自动开启通知")
                    }
                }
            } else {
                Button(
                    onClick = {
                        val act = activity ?: return@Button
                        scope.launch {
                            registeringPasskey = true
                            sheetHint = ""
                            when (val out = BiometricAuthHelper.registerPasskey(act, api)) {
                                is PasskeyOutcome.Success -> {
                                    onShowMessage("已绑定本机面容 / 指纹通行密钥，重装也可刷脸秒连")
                                }
                                is PasskeyOutcome.Cancelled -> Unit
                                is PasskeyOutcome.Failed -> {
                                    sheetHint = out.message
                                }
                            }
                            registeringPasskey = false
                        }
                    },
                    enabled = !registeringPasskey,
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 48.dp)
                ) {
                    Text(if (registeringPasskey) "正在调起面容 / 指纹..." else "绑定本机面容 / 指纹通行密钥")
                }

                ListItem(
                    headlineContent = { Text("启动 APP 时无感面容 / 指纹验证") },
                    supportingContent = {
                        Text("优先前置无感面容，不支持面容自动降级指纹或密码；不影响后台锁屏收通知")
                    },
                    trailingContent = {
                        Switch(
                            checked = biometricAppLockEnabled,
                            onCheckedChange = { checked ->
                                if (!checked) {
                                    onBiometricLockChanged(false)
                                    return@Switch
                                }
                                val act = activity
                                if (act == null || !BiometricAuthHelper.canUseLocalBiometrics(context)) {
                                    sheetHint = "当前设备尚未在系统设置中录入面容或指纹"
                                    return@Switch
                                }
                                BiometricAuthHelper.authenticateLocal(
                                    activity = act,
                                    title = "开启无感面容 / 指纹保护",
                                    subtitle = "验证一次本机面容或指纹即可启用",
                                    onSuccess = {
                                        onBiometricLockChanged(true)
                                    },
                                    onFallbackOrCancel = { msg ->
                                        sheetHint = msg
                                    }
                                )
                            }
                        )
                    }
                )

                ListItem(
                    headlineContent = { Text("常驻 SSE 实时秒推服务") },
                    supportingContent = { Text("默认开启，锁屏实时直弹服务端通知；关闭则仅使用周期同步") },
                    trailingContent = {
                        Switch(
                            checked = realtimeServiceEnabled,
                            onCheckedChange = onRealtimeServiceChanged
                        )
                    }
                )

                ZeroClickCaptureRow(
                    checked = zeroClickCaptureEnabled,
                    onCheckedChange = onZeroClickChanged
                )

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    OutlinedButton(
                        onClick = { onTestHealth(urlInput) },
                        modifier = Modifier
                            .weight(1f)
                            .sizeIn(minHeight = 48.dp)
                    ) {
                        Text("测试连通性")
                    }
                    OutlinedButton(
                        onClick = {
                            val batIntent = Intent(
                                Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS,
                                Uri.parse("package:${context.packageName}")
                            )
                            runCatching { context.startActivity(batIntent) }.onFailure {
                                runCatching {
                                    context.startActivity(Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS))
                                }
                            }
                        },
                        modifier = Modifier
                            .weight(1f)
                            .sizeIn(minHeight = 48.dp)
                    ) {
                        Text("后台保活白名单")
                    }
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    OutlinedButton(
                        onClick = {
                            runCatching {
                                context.startActivity(
                                    Intent(Intent.ACTION_VIEW, Uri.parse(LATEST_APK_URL))
                                )
                            }
                        },
                        modifier = Modifier
                            .weight(1f)
                            .sizeIn(minHeight = 48.dp)
                    ) {
                        Text("拉取最新 APK")
                    }
                    OutlinedButton(
                        onClick = onUnbindServer,
                        modifier = Modifier
                            .weight(1f)
                            .sizeIn(minHeight = 48.dp)
                    ) {
                        Text("重新绑定服务端")
                    }
                }
            }
            Spacer(modifier = Modifier.height(24.dp))
        }
    }
}
