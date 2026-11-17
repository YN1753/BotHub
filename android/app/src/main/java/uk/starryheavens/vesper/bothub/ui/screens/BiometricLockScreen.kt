package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
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
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import uk.starryheavens.vesper.bothub.ui.BiometricAuthHelper
import uk.starryheavens.vesper.bothub.ui.GrokBotAvatar

@Composable
fun BiometricLockScreen(
    snackbarHostState: SnackbarHostState,
    onBiometricUnlocked: () -> Unit,
    onPasswordVerify: suspend (String) -> Result<String>,
    modifier: Modifier = Modifier
) {
    val context = LocalContext.current
    val activity = remember(context) { BiometricAuthHelper.findFragmentActivity(context) }
    val scope = rememberCoroutineScope()

    var fallbackPass by remember { mutableStateOf("") }
    var showPassUnlock by remember { mutableStateOf(false) }
    var unlockBusy by remember { mutableStateOf(false) }

    fun triggerBiometricUnlock() {
        val act = activity
        if (act == null || !BiometricAuthHelper.canUseLocalBiometrics(context)) {
            showPassUnlock = true
            return
        }
        BiometricAuthHelper.authenticateLocal(
            activity = act,
            title = "解锁 Vesper BotHub",
            subtitle = "优先无感面容识别，不支持面容自动降级指纹或密码",
            onSuccess = onBiometricUnlocked,
            onFallbackOrCancel = { showPassUnlock = true }
        )
    }

    LaunchedEffect(Unit) {
        triggerBiometricUnlock()
    }

    Scaffold(
        modifier = modifier,
        snackbarHost = { SnackbarHost(snackbarHostState) },
        containerColor = MaterialTheme.colorScheme.background
    ) { pad ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(pad)
                .padding(horizontal = 28.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            GrokBotAvatar(
                avatarStyle = "butler:gold",
                state = if (unlockBusy) "working" else "thinking",
                size = 96.dp,
                interactive = true,
                seedKey = "biometric_gate"
            )
            Spacer(modifier = Modifier.height(16.dp))
            Text(
                text = "Vesper BotHub 生物识别保护",
                style = MaterialTheme.typography.titleLarge,
                fontWeight = FontWeight.Bold
            )
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = "后台实时通知常驻运行中，刷脸或按指纹即可进入工作台",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(24.dp))
            Button(
                onClick = { triggerBiometricUnlock() },
                modifier = Modifier
                    .fillMaxWidth()
                    .sizeIn(minHeight = 48.dp)
            ) {
                Text("无感面容 / 指纹解锁")
            }
            if (showPassUnlock) {
                Spacer(modifier = Modifier.height(14.dp))
                OutlinedTextField(
                    value = fallbackPass,
                    onValueChange = { fallbackPass = it },
                    label = { Text("兜底访问密码") },
                    visualTransformation = PasswordVisualTransformation(),
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedButton(
                    onClick = {
                        if (fallbackPass.isBlank() || unlockBusy) return@OutlinedButton
                        scope.launch {
                            unlockBusy = true
                            onPasswordVerify(fallbackPass).onSuccess {
                                fallbackPass = ""
                                onBiometricUnlocked()
                            }.onFailure { err ->
                                snackbarHostState.showSnackbar(err.message ?: "密码验证失败")
                            }
                            unlockBusy = false
                        }
                    },
                    modifier = Modifier
                        .fillMaxWidth()
                        .sizeIn(minHeight = 48.dp)
                ) {
                    Text(if (unlockBusy) "验证中..." else "使用访问密码进入")
                }
            }
        }
    }
}
