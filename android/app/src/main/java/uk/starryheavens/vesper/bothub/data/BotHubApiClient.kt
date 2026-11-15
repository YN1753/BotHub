package uk.starryheavens.vesper.bothub.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONArray
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.util.concurrent.TimeUnit

data class NotificationItem(
    val id: Long,
    val moduleId: String,
    val title: String,
    val summary: String,
    val markdownBody: String,
    val priority: String,
    val createdAt: String,
    val readAt: String
)

data class AssetItem(
    val id: String,
    val name: String,
    val category: String,
    val spec: String,
    val price: String,
    val dueDate: String,
    val status: String,
    val daysLeft: Int,
    val autoRenew: Boolean
)

data class TaskItem(
    val id: String,
    val name: String,
    val type: String,
    val cronExpr: String,
    val enabled: Boolean,
    val nextRunAt: String,
    val lastSummary: String
)

data class DashboardSnapshot(
    val version: String,
    val totalAssets: Int,
    val expiringCount: Int,
    val activeTasks: Int,
    val unreadNotifications: Int,
    val assets: List<AssetItem>,
    val tasks: List<TaskItem>
)

class BotHubApiClient(private val prefs: BotHubPrefs) {

    private val jsonMedia = "application/json; charset=utf-8".toMediaType()

    val httpClient: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(120, TimeUnit.SECONDS)
        .writeTimeout(30, TimeUnit.SECONDS)
        .build()

    private fun authRequestBuilder(path: String): Request.Builder {
        val url = "${prefs.serverUrl}$path"
        val builder = Request.Builder().url(url)
        val token = prefs.bearerToken
        if (token.isNotEmpty()) {
            builder.header("Authorization", "Bearer $token")
        }
        return builder
    }

    suspend fun exchangeToken(serverUrl: String, passwordOrToken: String): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanUrl = serverUrl.trim().trimEnd('/')
            val bodyJson = JSONObject().put("password", passwordOrToken.trim()).toString()
            val req = Request.Builder()
                .url("$cleanUrl/api/v2/auth/token")
                .post(bodyJson.toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) {
                    val err = runCatching { JSONObject(raw).optString("error") }.getOrDefault("")
                    throw IllegalStateException(err.ifEmpty { "连接验证失败 (${resp.code})" })
                }
                val token = JSONObject(raw).optString("token", passwordOrToken.trim())
                prefs.serverUrl = cleanUrl
                prefs.bearerToken = token
                token
            }
        }
    }

    suspend fun fetchNotifications(afterId: Long = 0L, limit: Int = 50): Result<List<NotificationItem>> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/v2/notifications?after_id=$afterId&limit=$limit").get().build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) {
                    throw IllegalStateException("拉取情报箱失败 (${resp.code})")
                }
                val root = JSONObject(raw)
                val arr = root.optJSONArray("notifications") ?: JSONArray()
                parseNotifications(arr)
            }
        }
    }

    suspend fun markNotificationRead(id: Long): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/v2/notifications/$id/read")
                .post("{}".toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().close()
        }
    }

    suspend fun markAllNotificationsRead(): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/v2/notifications/read-all")
                .post("{}".toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().close()
        }
    }

    suspend fun fetchDashboard(): Result<DashboardSnapshot> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/dashboard").get().build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) {
                    throw IllegalStateException("获取中枢状态失败 (${resp.code})")
                }
                val obj = JSONObject(raw)
                val stats = obj.optJSONObject("stats") ?: JSONObject()
                val assetsArr = obj.optJSONArray("assets") ?: JSONArray()
                val tasksArr = obj.optJSONArray("tasks") ?: JSONArray()

                val assets = mutableListOf<AssetItem>()
                var expiring = 0
                for (i in 0 until assetsArr.length()) {
                    val a = assetsArr.getJSONObject(i)
                    val days = a.optInt("days_left", 999)
                    val due = a.optString("due_date", "")
                    if (!due.startsWith("2099") && days in 0..7) {
                        expiring++
                    }
                    assets.add(
                        AssetItem(
                            id = a.optString("id"),
                            name = a.optString("name"),
                            category = a.optString("category", "compute"),
                            spec = a.optString("spec"),
                            price = a.optString("price"),
                            dueDate = due,
                            status = a.optString("status"),
                            daysLeft = days,
                            autoRenew = a.optBoolean("auto_renew", false)
                        )
                    )
                }

                val tasks = mutableListOf<TaskItem>()
                var activeTasks = 0
                for (i in 0 until tasksArr.length()) {
                    val t = tasksArr.getJSONObject(i)
                    val enabled = t.optBoolean("enabled", false)
                    if (enabled) activeTasks++
                    val lastRun = t.optJSONObject("last_run")
                    tasks.add(
                        TaskItem(
                            id = t.optString("id"),
                            name = t.optString("name"),
                            type = t.optString("type"),
                            cronExpr = t.optString("cron_expr"),
                            enabled = enabled,
                            nextRunAt = t.optString("next_run_at", "--"),
                            lastSummary = lastRun?.optString("summary") ?: "等待执行"
                        )
                    )
                }

                DashboardSnapshot(
                    version = obj.optString("version", "v2.0.0"),
                    totalAssets = stats.optInt("total_assets", assets.size),
                    expiringCount = expiring,
                    activeTasks = activeTasks,
                    unreadNotifications = stats.optInt("unread_notifications", 0),
                    assets = assets,
                    tasks = tasks
                )
            }
        }
    }

    suspend fun triggerTask(taskId: String): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/tasks/$taskId/run")
                .post("{}".toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                val obj = runCatching { JSONObject(raw) }.getOrNull()
                obj?.optString("summary")?.ifEmpty { "触发完成" } ?: "触发完成"
            }
        }
    }

    suspend fun streamChat(
        sessionId: String,
        message: String,
        persona: String,
        model: String,
        onSession: (String) -> Unit,
        onReasoningDelta: (String) -> Unit,
        onToolCall: (String, String) -> Unit,
        onContentDelta: (String) -> Unit,
        onError: (String) -> Unit
    ) = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
                .put("session_id", sessionId)
                .put("message", message)
                .put("persona", persona)
                .put("model", model)
                .toString()

            val req = authRequestBuilder("/api/agent/chat")
                .header("Accept", "text/event-stream")
                .post(payload.toRequestBody(jsonMedia))
                .build()

            httpClient.newCall(req).execute().use { resp ->
                if (!resp.isSuccessful) {
                    val errText = resp.body?.string().orEmpty()
                    val msg = runCatching { JSONObject(errText).optString("error") }.getOrDefault("")
                    onError(msg.ifEmpty { "请求失败 (${resp.code})" })
                    return@use
                }

                val stream = resp.body?.byteStream() ?: return@use
                val reader = BufferedReader(InputStreamReader(stream, Charsets.UTF_8))
                var currentEvent = "message"
                var line: String?

                while (reader.readLine().also { line = it } != null) {
                    val trimmed = line?.trim().orEmpty()
                    if (trimmed.isEmpty()) continue
                    if (trimmed.startsWith("event:")) {
                        currentEvent = trimmed.removePrefix("event:").trim()
                    } else if (trimmed.startsWith("data:")) {
                        val dataStr = trimmed.removePrefix("data:").trim()
                        val dataObj = runCatching { JSONObject(dataStr) }.getOrNull() ?: continue
                        when (currentEvent) {
                            "session" -> {
                                val sid = dataObj.optString("session_id")
                                if (sid.isNotEmpty()) onSession(sid)
                            }
                            "reasoning" -> {
                                val delta = dataObj.optString("delta")
                                if (delta.isNotEmpty()) onReasoningDelta(delta)
                            }
                            "tool_start" -> {
                                onToolCall(dataObj.optString("name"), "执行中")
                            }
                            "tool_result" -> {
                                onToolCall(dataObj.optString("name"), "已完成")
                            }
                            "content" -> {
                                val delta = dataObj.optString("delta")
                                if (delta.isNotEmpty()) onContentDelta(delta)
                            }
                            "error" -> {
                                onError(dataObj.optString("error", "流式响应异常"))
                            }
                        }
                    }
                }
            }
        }.onFailure { e ->
            onError(e.message ?: "网络连接异常")
        }
    }

    companion object {
        fun parseNotifications(arr: JSONArray): List<NotificationItem> {
            val out = mutableListOf<NotificationItem>()
            for (i in 0 until arr.length()) {
                val n = arr.getJSONObject(i)
                out.add(parseSingleNotification(n))
            }
            return out
        }

        fun parseSingleNotification(n: JSONObject): NotificationItem {
            return NotificationItem(
                id = n.optLong("id"),
                moduleId = n.optString("module_id", "system"),
                title = n.optString("title", "BotHub 通知"),
                summary = n.optString("summary", ""),
                markdownBody = n.optString("markdown_body", ""),
                priority = n.optString("priority", "normal"),
                createdAt = n.optString("created_at", ""),
                readAt = n.optString("read_at", "")
            )
        }
    }
}
