package uk.starryheavens.vesper.bothub.ui

import android.content.Context
import android.content.ContextWrapper
import android.os.Build
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricManager.Authenticators
import androidx.biometric.BiometricPrompt
import androidx.core.content.ContextCompat
import androidx.credentials.CreatePublicKeyCredentialRequest
import androidx.credentials.CreatePublicKeyCredentialResponse
import androidx.credentials.CredentialManager
import androidx.credentials.GetCredentialRequest
import androidx.credentials.GetPublicKeyCredentialOption
import androidx.credentials.PublicKeyCredential
import androidx.credentials.exceptions.CreateCredentialCancellationException
import androidx.credentials.exceptions.GetCredentialCancellationException
import androidx.fragment.app.FragmentActivity
import uk.starryheavens.vesper.bothub.data.BotHubApiClient

sealed class PasskeyOutcome {
    data object Success : PasskeyOutcome()
    data object Cancelled : PasskeyOutcome()
    data class Failed(val message: String) : PasskeyOutcome()
}

object BiometricAuthHelper {

    fun findFragmentActivity(context: Context): FragmentActivity? {
        var cur: Context? = context
        while (cur is ContextWrapper) {
            if (cur is FragmentActivity) return cur
            cur = cur.baseContext
        }
        return null
    }

    fun canUseLocalBiometrics(context: Context): Boolean {
        val bm = BiometricManager.from(context)
        val authenticators = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            Authenticators.BIOMETRIC_STRONG or Authenticators.BIOMETRIC_WEAK or Authenticators.DEVICE_CREDENTIAL
        } else {
            Authenticators.BIOMETRIC_WEAK
        }
        return bm.canAuthenticate(authenticators) == BiometricManager.BIOMETRIC_SUCCESS
    }

    /**
     * Triggers Android system biometrics with passive Face Unlock prioritized:
     * - Enables BIOMETRIC_WEAK + setConfirmationRequired(false) so 2D/3D Face Unlock passes immediately
     *   with zero confirmation tap.
     * - Automatically degrades to Fingerprint when Face is unavailable/unsupported, and to Device PIN/Password
     *   when neither Face nor Fingerprint is enrolled.
     */
    fun authenticateLocal(
        activity: FragmentActivity,
        title: String = "验证身份",
        subtitle: String = "优先无感面容识别，自动降级指纹或锁屏密码",
        onSuccess: () -> Unit,
        onFallbackOrCancel: (String) -> Unit
    ) {
        val executor = ContextCompat.getMainExecutor(activity)
        val prompt = BiometricPrompt(
            activity,
            executor,
            object : BiometricPrompt.AuthenticationCallback() {
                override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) {
                    onSuccess()
                }

                override fun onAuthenticationError(errorCode: Int, errString: CharSequence) {
                    onFallbackOrCancel(errString.toString())
                }
            }
        )

        val builder = BiometricPrompt.PromptInfo.Builder()
            .setTitle(title)
            .setSubtitle(subtitle)
            .setConfirmationRequired(false)

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            builder.setAllowedAuthenticators(
                Authenticators.BIOMETRIC_STRONG or
                    Authenticators.BIOMETRIC_WEAK or
                    Authenticators.DEVICE_CREDENTIAL
            )
        } else {
            builder
                .setAllowedAuthenticators(Authenticators.BIOMETRIC_WEAK)
                .setNegativeButtonText("使用访问密码")
        }

        runCatching {
            prompt.authenticate(builder.build())
        }.onFailure { err ->
            onFallbackOrCancel(err.message ?: "无法调起系统生物识别")
        }
    }

    suspend fun signInWithPasskey(
        activity: FragmentActivity,
        api: BotHubApiClient,
        serverUrl: String
    ): PasskeyOutcome {
        val (ceremonyId, requestJson) = api.beginPasskeyLogin(serverUrl).getOrElse { err ->
            return PasskeyOutcome.Failed(err.message ?: "无法连接服务器获取验证参数")
        }

        return try {
            val credentialManager = CredentialManager.create(activity)
            val option = GetPublicKeyCredentialOption(requestJson)
            val req = GetCredentialRequest.Builder()
                .addCredentialOption(option)
                .build()
            val result = credentialManager.getCredential(activity, req)
            val pkCred = result.credential as? PublicKeyCredential
                ?: return PasskeyOutcome.Failed("未获取到有效通行密钥签名")
            api.finishPasskeyLogin(serverUrl, ceremonyId, pkCred.authenticationResponseJson).fold(
                onSuccess = { PasskeyOutcome.Success },
                onFailure = { err -> PasskeyOutcome.Failed(err.message ?: "通行密钥校验失败") }
            )
        } catch (_: GetCredentialCancellationException) {
            PasskeyOutcome.Cancelled
        } catch (err: Exception) {
            PasskeyOutcome.Failed(
                err.localizedMessage?.takeIf { it.isNotBlank() }
                    ?: "当前系统未找到匹配的通行密钥，可从已登录浏览器一键绑定或输入一次密码"
            )
        }
    }

    suspend fun registerPasskey(
        activity: FragmentActivity,
        api: BotHubApiClient
    ): PasskeyOutcome {
        val (ceremonyId, requestJson) = api.beginPasskeyRegister().getOrElse { err ->
            return PasskeyOutcome.Failed(err.message ?: "请求注册挑战失败")
        }

        return try {
            val credentialManager = CredentialManager.create(activity)
            val createReq = CreatePublicKeyCredentialRequest(requestJson)
            val createRes = credentialManager.createCredential(activity, createReq)
            val pkRes = createRes as? CreatePublicKeyCredentialResponse
                ?: return PasskeyOutcome.Failed("系统未返回通行密钥公钥")
            api.finishPasskeyRegister(ceremonyId, pkRes.registrationResponseJson).fold(
                onSuccess = { PasskeyOutcome.Success },
                onFailure = { err -> PasskeyOutcome.Failed(err.message ?: "保存通行密钥失败") }
            )
        } catch (_: CreateCredentialCancellationException) {
            PasskeyOutcome.Cancelled
        } catch (err: Exception) {
            PasskeyOutcome.Failed(
                err.localizedMessage?.takeIf { it.isNotBlank() }
                    ?: "当前 ROM 未启用系统级 Passkey 服务，已为您保留本机无感面容/指纹支持"
            )
        }
    }
}
