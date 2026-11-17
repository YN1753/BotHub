package uk.starryheavens.vesper.bothub.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.Call
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONArray
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.URLEncoder
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

data class AgentSessionSummary(
    val id: String,
    val title: String,
    val persona: String,
    val model: String,
    val updatedAt: String
)

data class BotItem(
    val id: String,
    val name: String,
    val roleTitle: String,
    val avatarStyle: String,
    val model: String,
    val enableThinking: Boolean,
    val state: String,
    val unreadCount: Int,
    val lastSummary: String,
    val isGroup: Boolean = false,
    val memberIds: List<String> = emptyList(),
    val botMode: String = "worker"
)

data class AgentHistoryMessage(
    val id: Long,
    val role: String,
    val content: String,
    val reasoningContent: String,
    val msgKind: String = "chat",
    val senderBotId: String = ""
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

data class StashItemUi(
    val id: String,
    val kind: String,
    val url: String,
    val domain: String,
    val title: String,
    val rawInput: String,
    val userNote: String,
    val aiTldr: String,
    val points: List<String>,
    val tags: String,
    val status: String,
    val errorMsg: String,
    val pinned: Boolean,
    val readCount: Int,
    val createdAt: String
)

data class SharePreviewInfo(
    val kindLabel: String,
    val candidateTitle: String,
    val extractedUrl: String
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

    suspend fun exchangePairCode(serverUrl: String, code: String): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanUrl = serverUrl.trim().trimEnd('/').ifEmpty { prefs.serverUrl }
            val bodyJson = JSONObject().put("code", code.trim()).toString()
            val req = Request.Builder()
                .url("$cleanUrl/api/v2/auth/pair/exchange")
                .post(bodyJson.toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) {
                    val err = runCatching { JSONObject(raw).optString("error") }.getOrDefault("")
                    throw IllegalStateException(err.ifEmpty { "免密配对失败 (${resp.code})" })
                }
                val token = JSONObject(raw).optString("token", "")
                if (token.isEmpty()) throw IllegalStateException("服务端未返回有效凭证")
                prefs.serverUrl = cleanUrl
                prefs.bearerToken = token
                token
            }
        }
    }

    suspend fun fetchPasskeyStatus(serverUrl: String): Result<Boolean> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanUrl = serverUrl.trim().trimEnd('/').ifEmpty { prefs.serverUrl }
            val req = Request.Builder()
                .url("$cleanUrl/api/auth/passkey/status")
                .get()
                .build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) return@use false
                JSONObject(raw).optBoolean("has_passkeys", false)
            }
        }
    }

    suspend fun beginPasskeyLogin(serverUrl: String): Result<Pair<String, String>> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanUrl = serverUrl.trim().trimEnd('/').ifEmpty { prefs.serverUrl }
            val req = Request.Builder()
                .url("$cleanUrl/api/auth/passkey/login/options")
                .post("{}".toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) {
                    val err = runCatching { JSONObject(raw).optString("error") }.getOrDefault("")
                    throw IllegalStateException(err.ifEmpty { "请求生物识别挑战失败 (${resp.code})" })
                }
                val root = JSONObject(raw)
                val ceremonyId = root.optString("ceremony_id", "")
                val pubKeyObj = root.optJSONObject("publicKey")
                    ?: throw IllegalStateException("挑战参数缺失")
                ceremonyId to pubKeyObj.toString()
            }
        }
    }

    suspend fun finishPasskeyLogin(serverUrl: String, ceremonyId: String, responseJson: String): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanUrl = serverUrl.trim().trimEnd('/').ifEmpty { prefs.serverUrl }
            val encId = URLEncoder.encode(ceremonyId, "UTF-8")
            val req = Request.Builder()
                .url("$cleanUrl/api/auth/passkey/login/verify?ceremony_id=$encId")
                .post(responseJson.toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) {
                    val err = runCatching { JSONObject(raw).optString("error") }.getOrDefault("")
                    throw IllegalStateException(err.ifEmpty { "通行密钥签名校验未通过 (${resp.code})" })
                }
                val token = JSONObject(raw).optString("token", "")
                if (token.isEmpty()) throw IllegalStateException("服务端未返回推送凭证")
                prefs.serverUrl = cleanUrl
                prefs.bearerToken = token
                token
            }
        }
    }

    suspend fun beginPasskeyRegister(): Result<Pair<String, String>> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/auth/passkey/register/options")
                .post("""{"promotion":true}""".toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) {
                    val err = runCatching { JSONObject(raw).optString("error") }.getOrDefault("")
                    throw IllegalStateException(err.ifEmpty { "获取通行密钥注册参数失败 (${resp.code})" })
                }
                val root = JSONObject(raw)
                val ceremonyId = root.optString("ceremony_id", "")
                val pubKeyObj = root.optJSONObject("publicKey")
                    ?: throw IllegalStateException("注册参数格式无效")
                ceremonyId to pubKeyObj.toString()
            }
        }
    }

    suspend fun finishPasskeyRegister(ceremonyId: String, responseJson: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val encId = URLEncoder.encode(ceremonyId, "UTF-8")
            val req = authRequestBuilder("/api/auth/passkey/register/verify?ceremony_id=$encId")
                .post(responseJson.toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) {
                    val err = runCatching { JSONObject(raw).optString("error") }.getOrDefault("")
                    throw IllegalStateException(err.ifEmpty { "保存本机通行密钥失败 (${resp.code})" })
                }
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

    suspend fun toggleTaskEnabled(taskId: String, enabled: Boolean): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject().put("enabled", enabled).toString()
            val req = authRequestBuilder("/api/tasks/$taskId")
                .post(payload.toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().use { resp ->
                if (!resp.isSuccessful) error("更新任务状态失败 (${resp.code})")
            }
        }
    }

    suspend fun syncAssetWatcher(): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/modules/asset_watcher/run")
                .post("{}".toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().use { resp ->
                if (!resp.isSuccessful) error("资产嗅探同步失败 (${resp.code})")
                val raw = resp.body?.string().orEmpty()
                val obj = runCatching { JSONObject(raw) }.getOrNull()
                obj?.optString("summary")?.ifEmpty { "资产嗅探同步完成" } ?: "资产嗅探同步完成"
            }
        }
    }

    suspend fun fetchAgentSessions(): Result<List<AgentSessionSummary>> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/agent/sessions").get().build()
            httpClient.newCall(req).execute().use { resp ->
                if (!resp.isSuccessful) error("获取会话列表失败 (${resp.code})")
                val raw = resp.body?.string().orEmpty()
                val arr = JSONObject(raw).optJSONArray("sessions") ?: JSONArray()
                val list = mutableListOf<AgentSessionSummary>()
                for (i in 0 until arr.length()) {
                    val s = arr.getJSONObject(i)
                    list.add(
                        AgentSessionSummary(
                            id = s.optString("id"),
                            title = s.optString("title", "未命名会话"),
                            persona = s.optString("persona", "grok"),
                            model = s.optString("model", "deepseek-flash"),
                            updatedAt = s.optString("updated_at", "")
                        )
                    )
                }
                list
            }
        }
    }

    suspend fun fetchAgentSessionMessages(sessionId: String): Result<List<AgentHistoryMessage>> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/agent/sessions/$sessionId").get().build()
            httpClient.newCall(req).execute().use { resp ->
                if (!resp.isSuccessful) error("加载历史会话失败 (${resp.code})")
                val raw = resp.body?.string().orEmpty()
                val arr = JSONObject(raw).optJSONArray("messages") ?: JSONArray()
                val list = mutableListOf<AgentHistoryMessage>()
                for (i in 0 until arr.length()) {
                    val m = arr.getJSONObject(i)
                    val role = m.optString("role")
                    if (role == "user" || role == "assistant") {
                        list.add(
                            AgentHistoryMessage(
                                id = m.optLong("id"),
                                role = role,
                                content = m.optString("content", ""),
                                reasoningContent = m.optString("reasoning_content", "")
                            )
                        )
                    }
                }
                list
            }
        }
    }

    suspend fun fetchBots(): Result<List<BotItem>> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/v2/bots").get().build()
            httpClient.newCall(req).execute().use { resp ->
                if (!resp.isSuccessful) error("获取 Bot 名录失败 (${resp.code})")
                val raw = resp.body?.string().orEmpty()
                val arr = JSONObject(raw).optJSONArray("bots") ?: JSONArray()
                val list = mutableListOf<BotItem>()
                for (i in 0 until arr.length()) {
                    val b = arr.getJSONObject(i)
                    val memberIdsRaw = b.optString("member_ids", "[]")
                    val parsedMembers = runCatching {
                        val mArr = JSONArray(memberIdsRaw)
                        buildList {
                            for (mi in 0 until mArr.length()) {
                                val mid = mArr.optString(mi).trim()
                                if (mid.isNotEmpty()) add(mid)
                            }
                        }
                    }.getOrDefault(emptyList())
                    list.add(
                        BotItem(
                            id = b.optString("id"),
                            name = b.optString("name"),
                            roleTitle = b.optString("role_title"),
                            avatarStyle = b.optString("avatar_style", "cyan"),
                            model = b.optString("model", "deepseek-flash"),
                            enableThinking = b.optBoolean("enable_thinking", true),
                            state = b.optString("state", "idle"),
                            unreadCount = b.optInt("unread_count", 0),
                            lastSummary = b.optString("last_summary", ""),
                            isGroup = b.optBoolean("is_group", false),
                            memberIds = parsedMembers,
                            botMode = b.optString("bot_mode", "worker")
                        )
                    )
                }
                list
            }
        }
    }

    suspend fun fetchBotMessages(botId: String): Result<List<AgentHistoryMessage>> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/v2/bots/$botId/messages").get().build()
            httpClient.newCall(req).execute().use { resp ->
                if (!resp.isSuccessful) error("加载 Bot 对话流失败 (${resp.code})")
                val raw = resp.body?.string().orEmpty()
                val arr = JSONObject(raw).optJSONArray("messages") ?: JSONArray()
                val list = mutableListOf<AgentHistoryMessage>()
                for (i in 0 until arr.length()) {
                    val m = arr.getJSONObject(i)
                    val role = m.optString("role")
                    if (role == "user" || role == "assistant") {
                        list.add(
                            AgentHistoryMessage(
                                id = m.optLong("id"),
                                role = role,
                                content = m.optString("content", ""),
                                reasoningContent = m.optString("reasoning_content", ""),
                                msgKind = m.optString("msg_kind", "chat"),
                                senderBotId = m.optString("sender_bot_id", "")
                            )
                        )
                    }
                }
                list
            }
        }
    }

    suspend fun testHealth(serverUrlOverride: String? = null): Result<Pair<Long, String>> = withContext(Dispatchers.IO) {
        runCatching {
            val base = (serverUrlOverride ?: prefs.serverUrl).trim().trimEnd('/')
            val started = System.currentTimeMillis()
            val req = Request.Builder().url("$base/healthz").get().build()
            httpClient.newCall(req).execute().use { resp ->
                if (!resp.isSuccessful) error("HTTP ${resp.code}")
                val elapsed = System.currentTimeMillis() - started
                val obj = JSONObject(resp.body?.string().orEmpty())
                val ver = obj.optString("version", "unknown")
                elapsed to ver
            }
        }
    }

    suspend fun createStashItem(
        rawInput: String,
        userNote: String = "",
        tags: String = ""
    ): Result<Pair<StashItemUi, Boolean>> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
                .put("raw_input", rawInput.trim())
                .put("user_note", userNote.trim())
                .put("tags", tags.trim())
                .toString()
            val req = authRequestBuilder("/api/v2/stash")
                .post(payload.toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) {
                    val err = runCatching { JSONObject(raw).optString("error") }.getOrDefault("")
                    throw IllegalStateException(err.ifEmpty { "收入收藏匣失败 (${resp.code})" })
                }
                val root = JSONObject(raw)
                val dedup = root.optBoolean("deduplicated", false)
                val itemObj = root.optJSONObject("item") ?: JSONObject()
                parseSingleStashItem(itemObj) to dedup
            }
        }
    }

    suspend fun fetchStashItems(
        query: String = "",
        filter: String = "all",
        limit: Int = 80
    ): Result<List<StashItemUi>> = withContext(Dispatchers.IO) {
        runCatching {
            val encodedQ = URLEncoder.encode(query.trim(), "UTF-8")
            val encodedF = URLEncoder.encode(filter.trim().ifEmpty { "all" }, "UTF-8")
            val req = authRequestBuilder("/api/v2/stash?q=$encodedQ&filter=$encodedF&limit=$limit")
                .get()
                .build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) {
                    throw IllegalStateException("加载收藏匣失败 (${resp.code})")
                }
                val arr = JSONObject(raw).optJSONArray("items") ?: JSONArray()
                val list = mutableListOf<StashItemUi>()
                for (i in 0 until arr.length()) {
                    list.add(parseSingleStashItem(arr.getJSONObject(i)))
                }
                list
            }
        }
    }

    suspend fun updateStashItem(
        id: String,
        pinned: Boolean? = null,
        markRead: Boolean? = null,
        status: String? = null
    ): Result<StashItemUi> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
            if (pinned != null) payload.put("pinned", pinned)
            if (markRead != null) payload.put("mark_read", markRead)
            if (status != null) payload.put("status", status)
            val req = authRequestBuilder("/api/v2/stash/$id")
                .post(payload.toString().toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().use { resp ->
                val raw = resp.body?.string().orEmpty()
                if (!resp.isSuccessful) error("更新收藏失败 (${resp.code})")
                val obj = JSONObject(raw).optJSONObject("item") ?: JSONObject()
                parseSingleStashItem(obj)
            }
        }
    }

    suspend fun reprocessStashItem(id: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/v2/stash/$id/reprocess")
                .post("{}".toRequestBody(jsonMedia))
                .build()
            httpClient.newCall(req).execute().use { resp ->
                if (!resp.isSuccessful) error("触发重提炼失败 (${resp.code})")
            }
        }
    }

    suspend fun deleteStashItem(id: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = authRequestBuilder("/api/v2/stash/$id").delete().build()
            httpClient.newCall(req).execute().use { resp ->
                if (!resp.isSuccessful) error("删除收藏失败 (${resp.code})")
            }
        }
    }

    suspend fun streamChat(
        sessionId: String,
        botId: String = "",
        message: String,
        persona: String,
        model: String,
        enableThinking: Boolean = true,
        onCallCreated: (Call) -> Unit = {},
        onSession: (String) -> Unit,
        onReasoningDelta: (String) -> Unit,
        onToolCall: (String, String) -> Unit,
        onContentDelta: (String) -> Unit,
        onError: (String) -> Unit
    ) = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
                .put("session_id", sessionId)
                .put("bot_id", botId)
                .put("message", message)
                .put("persona", persona)
                .put("model", model)
                .put("enable_thinking", enableThinking)
                .toString()

            val req = authRequestBuilder("/api/agent/chat")
                .header("Accept", "text/event-stream")
                .post(payload.toRequestBody(jsonMedia))
                .build()

            val call = httpClient.newCall(req)
            onCallCreated(call)
            call.execute().use { resp ->
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

        fun parseSingleStashItem(s: JSONObject): StashItemUi {
            val ptsArr = s.optJSONArray("points")
            val pts = mutableListOf<String>()
            if (ptsArr != null) {
                for (i in 0 until ptsArr.length()) {
                    val v = ptsArr.optString(i, "").trim()
                    if (v.isNotEmpty()) pts.add(v)
                }
            } else {
                val rawPts = s.optString("ai_points", "[]")
                runCatching {
                    val fallbackArr = JSONArray(rawPts)
                    for (i in 0 until fallbackArr.length()) {
                        val v = fallbackArr.optString(i, "").trim()
                        if (v.isNotEmpty()) pts.add(v)
                    }
                }
            }
            return StashItemUi(
                id = s.optString("id"),
                kind = s.optString("kind", "link"),
                url = s.optString("url", ""),
                domain = s.optString("domain", ""),
                title = s.optString("title", ""),
                rawInput = s.optString("raw_input", ""),
                userNote = s.optString("user_note", ""),
                aiTldr = s.optString("ai_tldr", ""),
                points = pts,
                tags = s.optString("tags", ""),
                status = s.optString("status", "ready"),
                errorMsg = s.optString("error_msg", ""),
                pinned = s.optBoolean("pinned", false),
                readCount = s.optInt("read_count", 0),
                createdAt = s.optString("created_at", "")
            )
        }

        private val urlRegex = Regex("""https?://[^\s<>"'\]）】》]+""", RegexOption.IGNORE_CASE)

        fun parseSharePreview(rawText: String): SharePreviewInfo {
            val trimmed = rawText.trim()
            val match = urlRegex.find(trimmed)
            if (match == null) {
                val firstLine = trimmed.lineSequence().firstOrNull()?.trim().orEmpty().take(72)
                return SharePreviewInfo(
                    kindLabel = "纯文字闪念",
                    candidateTitle = firstLine.ifEmpty { "随手灵感记录" },
                    extractedUrl = ""
                )
            }
            val rawUrl = match.value.trimEnd(',', '.', ';', ':', '!', '?', '，', '。', '；', '：', '！', '？', '、', ')', '）', '】', '》')
            val prefix = trimmed.substring(0, match.range.first).trim().trim('-', '|', '—', '·', ':', '：')
            val suffix = trimmed.substring(match.range.last + 1).trim().trim('-', '|', '—', '·', ':', '：')
            val cand = prefix.ifEmpty { suffix }.replace(Regex("""\s+"""), " ").take(96)
            val lower = rawUrl.lowercase()
            val kindLabel = when {
                lower.contains("bilibili.com") || lower.contains("b23.tv") -> "Bilibili 视频"
                lower.contains("github.com") -> "GitHub 仓库"
                else -> "网页外链"
            }
            return SharePreviewInfo(
                kindLabel = kindLabel,
                candidateTitle = cand.ifEmpty { rawUrl.removePrefix("https://").removePrefix("http://").take(64) },
                extractedUrl = rawUrl
            )
        }
    }
}
