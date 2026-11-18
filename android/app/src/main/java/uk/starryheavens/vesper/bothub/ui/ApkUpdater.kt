package uk.starryheavens.vesper.bothub.ui

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import androidx.core.content.FileProvider
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import uk.starryheavens.vesper.bothub.data.UrlUtils
import java.io.File
import java.io.FileInputStream
import java.io.FileOutputStream
import java.util.concurrent.TimeUnit

sealed interface OtaUpdateState {
    data object Idle : OtaUpdateState
    data object Checking : OtaUpdateState
    data class Downloading(
        val percent: Int,
        val downloadedMb: Float,
        val totalMb: Float
    ) : OtaUpdateState
    data class NeedPermission(val apkFile: File) : OtaUpdateState
    data class ReadyToInstall(val apkFile: File) : OtaUpdateState
    data class UpToDate(val version: String) : OtaUpdateState
    data class Failed(val message: String) : OtaUpdateState
}

/**
 * Native In-App OTA Updater.
 * Streams the signed release APK via the self-hosted /api/v2/app/apk mirror first (bypassing
 * slow or blocked GitHub downloads on cellular networks), verifies ZIP/APK magic bytes, and
 * invokes the system package installer overlay directly via FileProvider without opening a browser.
 */
object ApkUpdater {

    private val downloadClient: OkHttpClient by lazy {
        OkHttpClient.Builder()
            .connectTimeout(15, TimeUnit.SECONDS)
            .readTimeout(180, TimeUnit.SECONDS)
            .followRedirects(true)
            .followSslRedirects(true)
            .build()
    }

    suspend fun downloadUpdateApk(
        context: Context,
        serverUrl: String,
        fallbackApkUrl: String,
        onProgress: (OtaUpdateState.Downloading) -> Unit
    ): Result<File> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanBase = UrlUtils.normalizeServerUrl(serverUrl, UrlUtils.DEFAULT_SERVER_URL)
            val candidates = buildList {
                if (cleanBase.isNotEmpty()) {
                    add("$cleanBase/api/v2/app/apk")
                }
                if (fallbackApkUrl.isNotBlank()) {
                    add(fallbackApkUrl.trim())
                }
            }.distinct()

            val updateDir = File(context.cacheDir, "updates").apply { mkdirs() }
            val tempFile = File(updateDir, "bothub-update.apk.part")
            val finalFile = File(updateDir, "bothub-update.apk")

            var lastError: Throwable? = null
            var succeeded = false

            for (url in candidates) {
                try {
                    ensureActive()
                    if (tempFile.exists()) tempFile.delete()
                    streamUrlToFile(url, tempFile, onProgress)
                    if (isValidApkFile(tempFile)) {
                        if (finalFile.exists()) finalFile.delete()
                        if (!tempFile.renameTo(finalFile)) {
                            tempFile.copyTo(finalFile, overwrite = true)
                            tempFile.delete()
                        }
                        succeeded = true
                        break
                    } else {
                        tempFile.delete()
                        lastError = IllegalStateException("安装包校验未通过")
                    }
                } catch (e: Throwable) {
                    tempFile.delete()
                    lastError = e
                }
            }

            if (!succeeded || !finalFile.exists()) {
                throw (lastError ?: IllegalStateException("下载安装包失败"))
            }
            finalFile
        }
    }

    private suspend fun streamUrlToFile(
        url: String,
        dest: File,
        onProgress: (OtaUpdateState.Downloading) -> Unit
    ) = withContext(Dispatchers.IO) {
        val req = Request.Builder()
            .url(url)
            .header("Accept-Encoding", "identity")
            .get()
            .build()

        downloadClient.newCall(req).execute().use { resp ->
            if (!resp.isSuccessful) {
                throw IllegalStateException("HTTP ${resp.code}")
            }
            val body = resp.body ?: throw IllegalStateException("空响应体")
            val totalBytes = body.contentLength()
            val totalMb = if (totalBytes > 0) totalBytes / (1024f * 1024f) else 0f

            body.byteStream().use { input ->
                FileOutputStream(dest).use { output ->
                    val buffer = ByteArray(32 * 1024)
                    var downloaded = 0L
                    var lastEmitMs = 0L

                    while (true) {
                        coroutineContext.ensureActive()
                        val read = input.read(buffer)
                        if (read == -1) break
                        output.write(buffer, 0, read)
                        downloaded += read

                        val now = System.currentTimeMillis()
                        if (now - lastEmitMs >= 80L || (totalBytes > 0 && downloaded == totalBytes)) {
                            lastEmitMs = now
                            val dlMb = downloaded / (1024f * 1024f)
                            val pct = if (totalBytes > 0) {
                                ((downloaded * 100L) / totalBytes).toInt().coerceIn(1, 100)
                            } else {
                                -1
                            }
                            onProgress(
                                OtaUpdateState.Downloading(
                                    percent = pct,
                                    downloadedMb = dlMb,
                                    totalMb = totalMb
                                )
                            )
                        }
                    }
                    output.flush()
                }
            }
        }
    }

    private fun isValidApkFile(file: File): Boolean {
        if (!file.exists() || file.length() < 512 * 1024L) return false
        return runCatching {
            FileInputStream(file).use { fis ->
                val magic = ByteArray(4)
                val n = fis.read(magic)
                n == 4 &&
                    magic[0] == 0x50.toByte() &&
                    magic[1] == 0x4B.toByte() &&
                    magic[2] == 0x03.toByte() &&
                    magic[3] == 0x04.toByte()
            }
        }.getOrDefault(false)
    }

    fun canInstallPackages(context: Context): Boolean {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.packageManager.canRequestPackageInstalls()
        } else {
            true
        }
    }

    fun openInstallPermissionSettings(context: Context) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val intent = Intent(
                Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                Uri.parse("package:${context.packageName}")
            ).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            runCatching { context.startActivity(intent) }
        }
    }

    fun triggerPackageInstaller(context: Context, apkFile: File): Result<Unit> {
        return runCatching {
            val uri = FileProvider.getUriForFile(
                context,
                "${context.packageName}.fileprovider",
                apkFile
            )
            val installIntent = Intent(Intent.ACTION_VIEW).apply {
                setDataAndType(uri, "application/vnd.android.package-archive")
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            context.startActivity(installIntent)
        }
    }
}
