package uk.starryheavens.vesper.bothub.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import uk.starryheavens.vesper.bothub.ui.BiometricAuthHelper
import uk.starryheavens.vesper.bothub.ui.components.BotAvatar
import uk.starryheavens.vesper.bothub.ui.components.BotHubSnackbarHost
import uk.starryheavens.vesper.bothub.ui.components.VesperPrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperTextField
import uk.starryheavens.vesper.bothub.ui.components.VesperBrandEmblem
import uk.starryheavens.vesper.bothub.ui.theme.BotHubThemeTokens

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
            title = "解锁 BotHub",
            subtitle = "验证面容或指纹以进入工作区",
            onSuccess = onBiometricUnlocked,
            onFallbackOrCancel = { showPassUnlock = true }
        )
    }

    LaunchedEffect(Unit) {
        triggerBiometricUnlock()
    }

    val theme = BotHubThemeTokens
    Scaffold(
        modifier = modifier,
        snackbarHost = { BotHubSnackbarHost(snackbarHostState) },
        containerColor = theme.bgCanvas
    ) { pad ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(pad)
                .imePadding()
                .padding(horizontal = 28.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            Row(
                modifier = Modifier
                    .clip(RoundedCornerShape(999.dp))
                    .background(theme.surfaceCard)
                    .border(1.dp, theme.border, RoundedCornerShape(999.dp))
                    .padding(start = 8.dp, end = 12.dp, top = 5.dp, bottom = 5.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(7.dp)
            ) {
                VesperBrandEmblem(size = 18.dp)
                Text(
                    text = "BOTHUB",
                    fontFamily = FontFamily.Monospace,
                    fontSize = 11.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = theme.accentForeground,
                    letterSpacing = 1.sp
                )
            }
            Spacer(modifier = Modifier.height(22.dp))
            BotAvatar(
                bot = null,
                allBots = emptyList(),
                avatarStyleOverride = "butler:gold",
                state = if (unlockBusy) "working" else "thinking",
                containerSize = 92.dp,
                interactive = true,
                seedKey = "biometric_gate"
            )
            Spacer(modifier = Modifier.height(18.dp))
            Text(
                text = "应用已锁定",
                style = MaterialTheme.typography.titleLarge,
                fontWeight = FontWeight.Bold,
                color = theme.textPrimary
            )
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = "请验证面容或指纹以继续",
                style = MaterialTheme.typography.bodySmall,
                color = theme.textMuted,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(26.dp))
            VesperPrimaryButton(
                text = "面容或指纹解锁",
                minHeight = 48.dp,
                onClick = { triggerBiometricUnlock() },
                modifier = Modifier.fillMaxWidth()
            )
            if (showPassUnlock) {
                Spacer(modifier = Modifier.height(16.dp))
                VesperTextField(
                    value = fallbackPass,
                    onValueChange = { fallbackPass = it },
                    label = "访问密钥",
                    placeholder = "输入控制台访问密钥",
                    visualTransformation = PasswordVisualTransformation(),
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
                Spacer(modifier = Modifier.height(10.dp))
                VesperSecondaryButton(
                    text = if (unlockBusy) "验证中..." else "使用访问密钥进入",
                    enabled = !unlockBusy && fallbackPass.isNotBlank(),
                    minHeight = 48.dp,
                    onClick = {
                        if (fallbackPass.isBlank() || unlockBusy) return@VesperSecondaryButton
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
                    modifier = Modifier.fillMaxWidth()
                )
            }
        }
    }
}
