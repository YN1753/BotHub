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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import uk.starryheavens.vesper.bothub.ui.BiometricAuthHelper
import uk.starryheavens.vesper.bothub.ui.components.ObsidianGlassInput
import uk.starryheavens.vesper.bothub.ui.components.SpiritSquircleAvatar
import uk.starryheavens.vesper.bothub.ui.components.TactilePrimaryButton
import uk.starryheavens.vesper.bothub.ui.components.TactileSecondaryButton
import uk.starryheavens.vesper.bothub.ui.components.VesperBrandEmblem

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
            Row(
                modifier = Modifier
                    .clip(RoundedCornerShape(999.dp))
                    .background(Color(0xFF131A2E))
                    .border(0.8.dp, Color(0x47F2B880), RoundedCornerShape(999.dp))
                    .padding(start = 7.dp, end = 12.dp, top = 5.dp, bottom = 5.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(7.dp)
            ) {
                VesperBrandEmblem(size = 18.dp)
                Text(
                    text = "BOTHUB",
                    fontFamily = FontFamily.Monospace,
                    fontSize = 11.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = Color(0xFFFFE2BE),
                    letterSpacing = 1.sp
                )
            }
            Spacer(modifier = Modifier.height(20.dp))
            SpiritSquircleAvatar(
                bot = null,
                allBots = emptyList(),
                avatarStyleOverride = "butler:gold",
                state = if (unlockBusy) "working" else "thinking",
                containerSize = 96.dp,
                cornerRadius = 28.dp,
                interactive = true,
                seedKey = "biometric_gate"
            )
            Spacer(modifier = Modifier.height(18.dp))
            Text(
                text = "BotHub 生物识别保护",
                style = MaterialTheme.typography.titleLarge,
                fontWeight = FontWeight.Bold,
                color = Color(0xFFFFF8EE)
            )
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = "后台实时通知常驻运行中，刷脸或按指纹即可进入工作台",
                style = MaterialTheme.typography.bodySmall,
                color = Color(0xFF8C9BB5),
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(24.dp))
            TactilePrimaryButton(
                text = "无感面容 / 指纹解锁",
                minHeight = 48.dp,
                onClick = { triggerBiometricUnlock() },
                modifier = Modifier.fillMaxWidth()
            )
            if (showPassUnlock) {
                Spacer(modifier = Modifier.height(14.dp))
                ObsidianGlassInput(
                    value = fallbackPass,
                    onValueChange = { fallbackPass = it },
                    label = "兜底访问密码",
                    placeholder = "输入控制台访问密钥...",
                    visualTransformation = PasswordVisualTransformation(),
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
                Spacer(modifier = Modifier.height(8.dp))
                TactileSecondaryButton(
                    text = if (unlockBusy) "验证中..." else "使用访问密码进入",
                    enabled = !unlockBusy && fallbackPass.isNotBlank(),
                    minHeight = 48.dp,
                    onClick = {
                        if (fallbackPass.isBlank() || unlockBusy) return@TactileSecondaryButton
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
