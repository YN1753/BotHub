package uk.starryheavens.vesper.bothub.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.withContext
import okhttp3.Call
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONArray
import org.json.JSONObject
import java.net.URLEncoder
import java.util.concurrent.TimeUnit

class BotHubApiClient(private val prefs: BotHubPrefs) {

    private val jsonMedia = "application/json; charset=utf-8".toMediaType()
    private val emptyJsonBody: RequestBody = "{}".toRequestBody(jsonMedia)

    private val httpClient: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(120, TimeUnit.SECONDS)
        .writeTimeout(30, TimeUnit.SECONDS)
        .build()

    private val sseHttpClient: OkHttpClient = httpClient.newBuilder()
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .build()

    private fun jsonBody(rawJson: String): RequestBody = rawJson.toRequestBody(jsonMedia)

    private fun requestBuilder(
        path: String,
        baseUrlOverride: String? = null,
        includeAuth: Boolean = true
    ): Request.Builder {
        val baseUrl = if (baseUrlOverride != null) {
            UrlUtils.normalizeServerUrl(baseUrlOverride, prefs.serverUrl)
        } else {
            prefs.serverUrl
        }
        val builder = Request.Builder().url("$baseUrl$path")
        if (includeAuth) {
            val token = prefs.bearerToken
            if (token.isNotEmpty()) {
                builder.header("Authorization", "Bearer $token")
            }
        }
        return builder
    }

    private fun executeJson(request: Request, errorPrefix: String): JSONObject {
        httpClient.newCall(request).execute().use { resp ->
            val raw = resp.body?.string().orEmpty()
            if (!resp.isSuccessful) {
                throw IllegalStateException(
                    JsonMappers.extractApiError(raw, "$errorPrefix (${resp.code})")
                )
            }
            return if (raw.isBlank()) JSONObject() else JSONObject(raw)
        }
    }

    private fun executeUnit(request: Request, errorPrefix: String) {
        httpClient.newCall(request).execute().use { resp ->
            val raw = resp.body?.string().orEmpty()
            if (!resp.isSuccessful) {
                throw IllegalStateException(
                    JsonMappers.extractApiError(raw, "$errorPrefix (${resp.code})")
                )
            }
        }
    }

    private fun saveAuthResult(
        cleanUrl: String,
        json: JSONObject,
        fallbackToken: String = "",
        missingTokenMessage: String = "服务端未返回有效凭证"
    ): String {
        val token = json.optString("token", fallbackToken).trim()
        if (token.isEmpty()) {
            throw IllegalStateException(missingTokenMessage)
        }
        prefs.serverUrl = cleanUrl
        prefs.bearerToken = token
        return token
    }

    private fun parsePasskeyCeremony(json: JSONObject, missingMessage: String): Pair<String, String> {
        val ceremonyId = json.optString("ceremony_id", "")
        val pubKeyObj = json.optJSONObject("publicKey")
            ?: throw IllegalStateException(missingMessage)
        return ceremonyId to pubKeyObj.toString()
    }

    suspend fun exchangeToken(serverUrl: String, passwordOrToken: String): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanUrl = UrlUtils.normalizeServerUrl(serverUrl, prefs.serverUrl)
            val cleanSecret = passwordOrToken.trim()
            val payload = JSONObject().put("password", cleanSecret).toString()
            val req = requestBuilder("/api/v2/auth/token", baseUrlOverride = cleanUrl, includeAuth = false)
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "连接验证失败")
            saveAuthResult(cleanUrl, json, fallbackToken = cleanSecret)
        }
    }

    suspend fun exchangePairCode(serverUrl: String, code: String): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanUrl = UrlUtils.normalizeServerUrl(serverUrl, prefs.serverUrl)
            val payload = JSONObject().put("code", code.trim()).toString()
            val req = requestBuilder("/api/v2/auth/pair/exchange", baseUrlOverride = cleanUrl, includeAuth = false)
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "免密配对失败")
            saveAuthResult(cleanUrl, json)
        }
    }

    suspend fun fetchOidcStatus(serverUrl: String): Result<Boolean> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanUrl = UrlUtils.normalizeServerUrl(serverUrl, prefs.serverUrl)
            val req = requestBuilder("/api/v2/auth/oidc/status", baseUrlOverride = cleanUrl, includeAuth = false)
                .get()
                .build()
            val json = executeJson(req, "查询单点登录状态失败")
            json.optBoolean("oidc_enabled", false)
        }
    }

    suspend fun mintWebSession(): Result<WebSessionTicket> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/auth/web-session")
                .post(emptyJsonBody)
                .build()
            val json = executeJson(req, "签发混合工作台会话失败")
            val cookieName = json.optString("cookie_name", "bothub_session").trim()
            val cookieValue = json.optString("cookie_value", "").trim()
            val maxAge = json.optInt("max_age", 86400)
            if (cookieValue.isEmpty()) {
                throw IllegalStateException("服务端未返回有效工作台会话")
            }
            WebSessionTicket(cookieName = cookieName, cookieValue = cookieValue, maxAge = maxAge)
        }
    }

    suspend fun fetchAppVersion(serverUrlOverride: String? = null): Result<AppVersionInfo> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder(
                "/api/v2/app/version",
                baseUrlOverride = serverUrlOverride,
                includeAuth = false
            ).get().build()
            val json = executeJson(req, "查询版本信息失败")
            AppVersionInfo(
                version = json.optString("version", "").trim(),
                apkUrl = json.optString(
                    "apk_url",
                    "https://github.com/YN1753/BotHub/releases/latest/download/vesper-bothub-android.apk"
                ).trim()
            )
        }
    }

    suspend fun fetchPasskeyStatus(serverUrl: String): Result<Boolean> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanUrl = UrlUtils.normalizeServerUrl(serverUrl, prefs.serverUrl)
            val req = requestBuilder("/api/v2/auth/passkey/status", baseUrlOverride = cleanUrl, includeAuth = false)
                .get()
                .build()
            val json = executeJson(req, "查询通行密钥状态失败")
            json.optBoolean("has_passkeys", false)
        }
    }

    suspend fun beginPasskeyLogin(serverUrl: String): Result<Pair<String, String>> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanUrl = UrlUtils.normalizeServerUrl(serverUrl, prefs.serverUrl)
            val req = requestBuilder("/api/v2/auth/passkey/login/options", baseUrlOverride = cleanUrl, includeAuth = false)
                .post(emptyJsonBody)
                .build()
            val json = executeJson(req, "请求生物识别挑战失败")
            parsePasskeyCeremony(json, "挑战参数缺失")
        }
    }

    suspend fun finishPasskeyLogin(
        serverUrl: String,
        ceremonyId: String,
        responseJson: String
    ): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanUrl = UrlUtils.normalizeServerUrl(serverUrl, prefs.serverUrl)
            val encId = URLEncoder.encode(ceremonyId, "UTF-8")
            val req = requestBuilder(
                "/api/v2/auth/passkey/login/verify?ceremony_id=$encId",
                baseUrlOverride = cleanUrl,
                includeAuth = false
            )
                .post(jsonBody(responseJson))
                .build()
            val json = executeJson(req, "通行密钥签名校验未通过")
            saveAuthResult(cleanUrl, json, missingTokenMessage = "服务端未返回推送凭证")
        }
    }

    suspend fun beginPasskeyRegister(): Result<Pair<String, String>> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject().put("promotion", true).toString()
            val req = requestBuilder("/api/v2/auth/passkey/register/options")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "获取通行密钥注册参数失败")
            parsePasskeyCeremony(json, "注册参数格式无效")
        }
    }

    suspend fun finishPasskeyRegister(ceremonyId: String, responseJson: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val encId = URLEncoder.encode(ceremonyId, "UTF-8")
            val req = requestBuilder("/api/v2/auth/passkey/register/verify?ceremony_id=$encId")
                .post(jsonBody(responseJson))
                .build()
            executeUnit(req, "保存本机通行密钥失败")
        }
    }

    suspend fun fetchNotifications(afterId: Long = 0L, limit: Int = 50): Result<List<NotificationItem>> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/notifications?after_id=$afterId&limit=$limit").get().build()
            val json = executeJson(req, "拉取情报箱失败")
            JsonMappers.parseNotifications(json.optJSONArray("notifications"))
        }
    }

    suspend fun markNotificationRead(id: Long): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/notifications/$id/read")
                .post(emptyJsonBody)
                .build()
            executeUnit(req, "标记情报已读失败")
        }
    }

    suspend fun markAllNotificationsRead(): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/notifications/read-all")
                .post(emptyJsonBody)
                .build()
            executeUnit(req, "全部标记已读失败")
        }
    }

    suspend fun fetchDashboard(): Result<DashboardSnapshot> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/dashboard").get().build()
            val json = executeJson(req, "获取中枢状态失败")
            JsonMappers.parseDashboard(json)
        }
    }

    suspend fun triggerTask(taskId: String): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/tasks/$taskId/run")
                .post(emptyJsonBody)
                .build()
            val json = executeJson(req, "触发任务失败")
            json.optString("summary", "").ifEmpty { "触发完成" }
        }
    }

    suspend fun toggleTaskEnabled(taskId: String, enabled: Boolean): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject().put("enabled", enabled).toString()
            val req = requestBuilder("/api/tasks/$taskId")
                .post(jsonBody(payload))
                .build()
            executeUnit(req, "更新任务状态失败")
        }
    }

    suspend fun syncAssetWatcher(): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/modules/asset_watcher/run")
                .post(emptyJsonBody)
                .build()
            val json = executeJson(req, "资产同步失败")
            json.optString("summary", "").ifEmpty { "同步完成" }
        }
    }

    suspend fun saveAsset(
        existingId: String?,
        name: String,
        category: String,
        spec: String,
        price: String,
        dueDate: String,
        autoRenew: Boolean
    ): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val id = existingId?.trim().takeUnless { it.isNullOrEmpty() }
                ?: "asset_${System.currentTimeMillis() % 100000000}"
            val payload = JSONObject()
                .put("id", id)
                .put("name", name.trim())
                .put("category", category.ifBlank { "compute" })
                .put("spec", spec.trim())
                .put("price", price.trim())
                .put("due_date", dueDate.trim())
                .put("auto_renew", autoRenew)
                .toString()
            val path = if (existingId.isNullOrBlank()) "/api/assets" else "/api/assets/${existingId.trim()}"
            val req = requestBuilder(path)
                .post(jsonBody(payload))
                .build()
            executeUnit(req, "保存资产失败")
        }
    }

    suspend fun deleteAsset(assetId: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/assets/${assetId.trim()}")
                .delete()
                .build()
            executeUnit(req, "删除资产失败")
        }
    }

    suspend fun fetchAgentSessions(): Result<List<AgentSessionSummary>> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/agent/sessions").get().build()
            val json = executeJson(req, "获取会话列表失败")
            JsonMappers.parseAgentSessions(json.optJSONArray("sessions"))
        }
    }

    suspend fun fetchAgentSessionMessages(sessionId: String): Result<List<AgentHistoryMessage>> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/agent/sessions/$sessionId").get().build()
            val json = executeJson(req, "加载历史会话失败")
            JsonMappers.parseHistoryMessages(json.optJSONArray("messages"))
        }
    }

    suspend fun fetchBots(): Result<List<BotItem>> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/bots").get().build()
            val json = executeJson(req, "获取 Bot 名录失败")
            JsonMappers.parseBots(json.optJSONArray("bots"))
        }
    }

    suspend fun fetchBotMessages(botId: String): Result<List<AgentHistoryMessage>> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/bots/$botId/messages").get().build()
            val json = executeJson(req, "加载 Bot 对话流失败")
            JsonMappers.parseHistoryMessages(json.optJSONArray("messages"))
        }
    }

    suspend fun clearBotMessages(botId: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/bots/$botId/messages")
                .delete()
                .build()
            executeUnit(req, "清空聊天记录失败")
        }
    }

    suspend fun updateBotPin(botId: String, pinned: Boolean): Result<BotItem> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject().put("is_pinned", pinned).toString()
            val req = requestBuilder("/api/v2/bots/$botId")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "更新置顶状态失败")
            JsonMappers.parseBot(json.optJSONObject("bot") ?: JSONObject())
        }
    }

    suspend fun updateBotDetails(
        botId: String,
        name: String,
        roleTitle: String,
        avatarStyle: String,
        traits: String,
        systemPrompt: String,
        groupMode: String,
        announcement: String
    ): Result<BotItem> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
                .put("name", name.trim())
                .put("role_title", roleTitle.trim())
                .put("avatar_style", avatarStyle)
                .put("traits", traits.trim())
                .put("system_prompt", systemPrompt.trim())
                .put("group_mode", groupMode)
                .put("announcement", announcement.trim())
                .toString()
            val req = requestBuilder("/api/v2/bots/$botId")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "保存资料失败")
            JsonMappers.parseBot(json.optJSONObject("bot") ?: JSONObject())
        }
    }

    suspend fun createAiFriend(
        name: String,
        roleTitle: String,
        avatarStyle: String,
        botMode: String,
        traits: String,
        systemPrompt: String
    ): Result<BotItem> = withContext(Dispatchers.IO) {
        runCatching {
            val defaultTools = if (botMode == "companion") {
                JSONArray()
            } else {
                JSONArray()
                    .put("list_assets")
                    .put("query_nezha_nodes")
                    .put("list_tasks")
                    .put("manage_stash")
                    .put("fetch_webpage")
            }
            val payload = JSONObject()
                .put("name", name.trim())
                .put("role_title", roleTitle.trim())
                .put("avatar_style", avatarStyle)
                .put("bot_mode", botMode)
                .put("traits", traits.trim())
                .put("system_prompt", systemPrompt.trim())
                .put("allowed_tools", defaultTools.toString())
                .put("enable_thinking", botMode != "companion")
                .toString()
            val req = requestBuilder("/api/v2/bots")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "创建 AI 好友失败")
            JsonMappers.parseBot(json.optJSONObject("bot") ?: JSONObject())
        }
    }

    suspend fun createGroupChat(
        sourceBotId: String = "",
        memberIds: List<String>,
        name: String = "",
        groupMode: String = "smart",
        carryContext: Boolean = false
    ): Result<BotItem> = withContext(Dispatchers.IO) {
        runCatching {
            val arr = JSONArray()
            memberIds.forEach { arr.put(it) }
            val payload = JSONObject()
                .put("source_bot_id", sourceBotId)
                .put("member_ids", arr)
                .put("name", name.trim())
                .put("group_mode", groupMode)
                .put("carry_context", carryContext)
                .toString()
            val req = requestBuilder("/api/v2/bots/adhoc-group")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "发起群聊失败")
            JsonMappers.parseBot(json.optJSONObject("bot") ?: JSONObject())
        }
    }

    suspend fun deleteBot(botId: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/bots/$botId")
                .delete()
                .build()
            executeUnit(req, "删除好友或群聊失败")
        }
    }

    suspend fun testHealth(serverUrlOverride: String? = null): Result<Pair<Long, String>> = withContext(Dispatchers.IO) {
        runCatching {
            val started = System.currentTimeMillis()
            val req = requestBuilder(
                "/healthz",
                baseUrlOverride = serverUrlOverride ?: prefs.serverUrl,
                includeAuth = false
            ).get().build()
            val json = executeJson(req, "连通性检测失败")
            val elapsed = System.currentTimeMillis() - started
            elapsed to json.optString("version", "unknown")
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
            val req = requestBuilder("/api/v2/stash")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "收入收藏匣失败")
            val dedup = json.optBoolean("deduplicated", false)
            val itemObj = json.optJSONObject("item") ?: JSONObject()
            JsonMappers.parseStashItem(itemObj) to dedup
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
            val req = requestBuilder("/api/v2/stash?q=$encodedQ&filter=$encodedF&limit=$limit")
                .get()
                .build()
            val json = executeJson(req, "加载收藏匣失败")
            JsonMappers.parseStashItems(json.optJSONArray("items"))
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
            val req = requestBuilder("/api/v2/stash/$id")
                .post(jsonBody(payload.toString()))
                .build()
            val json = executeJson(req, "更新收藏失败")
            val itemObj = json.optJSONObject("item") ?: JSONObject()
            JsonMappers.parseStashItem(itemObj)
        }
    }

    suspend fun reprocessStashItem(id: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/stash/$id/reprocess")
                .post(emptyJsonBody)
                .build()
            executeUnit(req, "触发重提炼失败")
        }
    }

    suspend fun deleteStashItem(id: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/stash/$id").delete().build()
            executeUnit(req, "删除收藏失败")
        }
    }

    suspend fun openNotificationEventStream(
        lastEventId: Long,
        onEvent: (String, JSONObject) -> Unit
    ) = withContext(Dispatchers.IO) {
        val req = requestBuilder("/api/v2/events?last_event_id=$lastEventId")
            .header("Accept", "text/event-stream")
            .get()
            .build()
        val call = sseHttpClient.newCall(req)
        val cancelHandle = coroutineContext[Job]?.invokeOnCompletion { cause ->
            if (cause != null) {
                call.cancel()
            }
        }
        try {
            call.execute().use { resp ->
                if (!resp.isSuccessful) {
                    val errRaw = resp.body?.string().orEmpty()
                    throw IllegalStateException(
                        JsonMappers.extractApiError(errRaw, "实时推送流连接失败 (${resp.code})")
                    )
                }
                val stream = resp.body?.byteStream() ?: return@withContext
                SseReader.readJsonEvents(stream, onEvent)
            }
        } finally {
            cancelHandle?.dispose()
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

            val req = requestBuilder("/api/agent/chat")
                .header("Accept", "text/event-stream")
                .post(jsonBody(payload))
                .build()

            val call = httpClient.newCall(req)
            onCallCreated(call)
            call.execute().use { resp ->
                if (!resp.isSuccessful) {
                    val errText = resp.body?.string().orEmpty()
                    onError(JsonMappers.extractApiError(errText, "请求失败 (${resp.code})"))
                    return@use
                }

                val stream = resp.body?.byteStream() ?: return@use
                SseReader.readJsonEvents(stream) { event, dataObj ->
                    when (event) {
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
        }.onFailure { e ->
            onError(e.message ?: "网络连接异常")
        }
    }
}
