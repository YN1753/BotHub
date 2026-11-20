package uk.starryheavens.vesper.bothub.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.withContext
import okhttp3.Call
import okhttp3.ConnectionPool
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONArray
import org.json.JSONObject
import java.io.IOException
import java.io.InterruptedIOException
import java.net.URLEncoder
import java.util.concurrent.TimeUnit

class BotHubApiClient(private val prefs: BotHubPrefs) {

    private val jsonMedia = "application/json; charset=utf-8".toMediaType()
    private val emptyJsonBody: RequestBody = "{}".toRequestBody(jsonMedia)

    private val httpClient: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(90, TimeUnit.SECONDS)
        .writeTimeout(25, TimeUnit.SECONDS)
        .retryOnConnectionFailure(true)
        .connectionPool(ConnectionPool(6, 3, TimeUnit.MINUTES))
        .build()

    private val sseHttpClient: OkHttpClient = httpClient.newBuilder()
        .readTimeout(45, TimeUnit.SECONDS)
        .pingInterval(20, TimeUnit.SECONDS)
        .retryOnConnectionFailure(true)
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
        val isIdempotentGet = request.method.equals("GET", ignoreCase = true)
        var lastIoError: IOException? = null
        val maxAttempts = if (isIdempotentGet) 2 else 1
        for (attempt in 1..maxAttempts) {
            try {
                httpClient.newCall(request).execute().use { resp ->
                    val raw = resp.body?.string().orEmpty()
                    if (!resp.isSuccessful) {
                        throw IllegalStateException(
                            JsonMappers.extractApiError(raw, "$errorPrefix (${resp.code})")
                        )
                    }
                    return if (raw.isBlank()) JSONObject() else JSONObject(raw)
                }
            } catch (ioe: IOException) {
                if (ioe is InterruptedIOException || attempt >= maxAttempts) {
                    throw ioe
                }
                lastIoError = ioe
            }
        }
        throw (lastIoError ?: IllegalStateException(errorPrefix))
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

    suspend fun clearReadNotifications(): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/notifications/clear-read")
                .post(emptyJsonBody)
                .build()
            executeUnit(req, "清理已读通知失败")
        }
    }

    suspend fun fetchDashboard(): Result<DashboardSnapshot> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/dashboard").get().build()
            val json = executeJson(req, "获取中枢状态失败")
            JsonMappers.parseDashboard(json)
        }
    }

    suspend fun fetchBootstrap(): Result<BootstrapPayload> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/app/bootstrap").get().build()
            val json = executeJson(req, "同步失败")
            val dash = JsonMappers.parseDashboard(json)
            BootstrapPayload(
                bots = JsonMappers.parseBots(json.optJSONArray("bots")),
                skills = JsonMappers.parseSkills(json.optJSONArray("skills")),
                notifications = JsonMappers.parseNotifications(json.optJSONArray("notifications")),
                dashboard = dash,
                stashItems = JsonMappers.parseStashItems(json.optJSONArray("stash")),
                todos = dash.todos,
                notes = dash.notes,
                modelOptions = dash.modelOptions,
                modelProviders = JsonMappers.parseModelProviders(json.optJSONArray("model_providers")),
                pooledModels = JsonMappers.parsePooledModels(json.optJSONArray("pooled_models")),
                poolStrategies = JsonMappers.parsePoolStrategies(json.optJSONObject("pool_strategies")),
                modelHealth = JsonMappers.parseModelHealth(json.optJSONObject("model_health")),
                modelUsageSummary = JsonMappers.parseModelUsageSummary(json.optJSONObject("model_usage_summary")),
                revisions = dash.revisions
            )
        }
    }

    suspend fun fetchDashboardSlice(kinds: List<String>): Result<DashboardSlicePayload> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanKinds = kinds.map { it.trim() }.filter { it.isNotEmpty() }.distinct()
            val encoded = URLEncoder.encode(cleanKinds.joinToString(","), "UTF-8")
            val req = requestBuilder("/api/v2/dashboard/slice?kinds=$encoded").get().build()
            val json = executeJson(req, "局部同步失败")
            DashboardSlicePayload(
                bots = if (json.has("bots")) JsonMappers.parseBots(json.optJSONArray("bots")) else null,
                skills = if (json.has("skills")) JsonMappers.parseSkills(json.optJSONArray("skills")) else null,
                assets = if (json.has("assets")) JsonMappers.parseAssets(json.optJSONArray("assets")) else null,
                assetCategories = if (json.has("asset_categories")) JsonMappers.parseAssetCategories(json.optJSONArray("asset_categories")) else null,
                tasks = if (json.has("tasks")) JsonMappers.parseTasks(json.optJSONArray("tasks")) else null,
                todos = if (json.has("todos")) JsonMappers.parseTodos(json.optJSONArray("todos")) else null,
                notes = if (json.has("notes")) JsonMappers.parseNotes(json.optJSONArray("notes")) else null,
                notifications = if (json.has("notifications")) JsonMappers.parseNotifications(json.optJSONArray("notifications")) else null,
                stashItems = if (json.has("stash")) JsonMappers.parseStashItems(json.optJSONArray("stash")) else null,
                modelOptions = if (json.has("model_options")) JsonMappers.parseModelOptions(json.optJSONArray("model_options")) else null,
                modelProviders = if (json.has("model_providers")) JsonMappers.parseModelProviders(json.optJSONArray("model_providers")) else null,
                pooledModels = if (json.has("pooled_models")) JsonMappers.parsePooledModels(json.optJSONArray("pooled_models")) else null,
                poolStrategies = if (json.has("pool_strategies")) JsonMappers.parsePoolStrategies(json.optJSONObject("pool_strategies")) else null,
                modelHealth = if (json.has("model_health")) JsonMappers.parseModelHealth(json.optJSONObject("model_health")) else null,
                modelUsageSummary = if (json.has("model_usage_summary")) JsonMappers.parseModelUsageSummary(json.optJSONObject("model_usage_summary")) else null,
                revisions = JsonMappers.parseRevisions(json.optJSONObject("revisions"))
            )
        }
    }

    suspend fun fetchModelConfig(): Result<ModelConfigPayload> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/model-config").get().build()
            val json = executeJson(req, "读取模型配置失败")
            JsonMappers.parseModelConfigPayload(json)
        }
    }

    suspend fun saveModelConfig(
        providers: List<ModelProviderItem>,
        models: List<PooledModelItem>,
        strategies: PoolStrategiesItem
    ): Result<ModelConfigPayload> = withContext(Dispatchers.IO) {
        runCatching {
            val provArr = JSONArray()
            providers.forEach { p ->
                provArr.put(
                    JSONObject()
                        .put("id", p.id.trim())
                        .put("name", p.name.trim())
                        .put("protocol", p.protocol.ifBlank { "openai" })
                        .put("base_url", p.baseUrl.trim())
                        .put("api_key", p.apiKey.trim())
                        .put("enabled", p.enabled)
                        .put("note", p.note.trim())
                )
            }
            val modArr = JSONArray()
            models.forEach { m ->
                val obj = JSONObject()
                    .put("provider_id", m.providerId.trim())
                    .put("model_id", m.modelId.trim())
                    .put("tier", m.tier.ifBlank { "free" })
                    .put("enabled", m.enabled)
                    .put("priority", m.priority)
                    .put("last_latency_ms", m.lastLatencyMs)
                if (m.supportsTools != null) obj.put("supports_tools", m.supportsTools)
                if (m.supportsThinking != null) obj.put("supports_thinking", m.supportsThinking)
                modArr.put(obj)
            }
            val payload = JSONObject()
                .put("providers", provArr)
                .put("models", modArr)
                .put(
                    "strategies",
                    JSONObject()
                        .put("free", strategies.free.ifBlank { "priority" })
                        .put("paid", strategies.paid.ifBlank { "priority" })
                )
                .toString()
            val req = requestBuilder("/api/v2/model-config")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "保存模型配置失败")
            JsonMappers.parseModelConfigPayload(json)
        }
    }

    suspend fun fetchUpstreamModels(
        providerId: String,
        protocol: String,
        baseUrl: String,
        apiKey: String
    ): Result<List<String>> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
                .put("provider_id", providerId.trim())
                .put("protocol", protocol.ifBlank { "openai" })
                .put("base_url", baseUrl.trim())
                .put("api_key", apiKey.trim())
                .put("verify", false)
                .toString()
            val req = requestBuilder("/api/v2/model-pool/models")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "拉取模型列表失败")
            JsonMappers.optStringList(json, "models")
        }
    }

    suspend fun probePooledModel(
        provider: ModelProviderItem,
        modelId: String
    ): Result<ModelProbeResultItem> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
                .put("provider_id", provider.id.trim())
                .put("protocol", provider.protocol.ifBlank { "openai" })
                .put("base_url", provider.baseUrl.trim())
                .put("api_key", provider.apiKey.trim())
                .put("model", modelId.trim())
                .put("probe_tools", true)
                .toString()
            val req = requestBuilder("/api/v2/model-pool/test")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "模型探测失败")
            ModelProbeResultItem(
                ok = json.optBoolean("ok", false),
                latencyMs = json.optLong("latency_ms", 0L),
                error = json.optString("error", ""),
                supportsTools = if (json.has("supports_tools") && !json.isNull("supports_tools")) {
                    json.optBoolean("supports_tools")
                } else null,
                supportsThinking = if (json.has("supports_thinking") && !json.isNull("supports_thinking")) {
                    json.optBoolean("supports_thinking")
                } else null,
                modelHealth = JsonMappers.parseModelHealth(json.optJSONObject("model_health")),
                pooledModels = if (json.has("pooled_models")) JsonMappers.parsePooledModels(json.optJSONArray("pooled_models")) else null,
                modelOptions = if (json.has("model_options")) JsonMappers.parseModelOptions(json.optJSONArray("model_options")) else null
            )
        }
    }

    suspend fun clearModelCooldown(targetId: String = ""): Result<Map<String, ModelHealthItem>> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject().put("target_id", targetId.trim()).toString()
            val req = requestBuilder("/api/v2/model-pool/cooldown/clear")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "解除冷却失败")
            JsonMappers.parseModelHealth(json.optJSONObject("model_health"))
        }
    }

    suspend fun testModelBatch(
        tier: String = "free",
        disableFailed: Boolean = false
    ): Result<BatchModelProbePayload> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
                .put("tier", tier.trim())
                .put("disable_failed", disableFailed)
                .put("probe_tools", true)
                .toString()
            val req = requestBuilder("/api/v2/model-pool/test-batch")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "批量测速失败")
            val resArr = json.optJSONArray("results")
            val results = buildList {
                if (resArr != null) {
                    for (i in 0 until resArr.length()) {
                        val o = resArr.optJSONObject(i) ?: continue
                        add(
                            BatchModelProbeEntryItem(
                                targetId = o.optString("target_id", ""),
                                providerId = o.optString("provider_id", ""),
                                modelId = o.optString("model_id", ""),
                                tier = o.optString("tier", "free"),
                                ok = o.optBoolean("ok", false),
                                latencyMs = o.optLong("latency_ms", 0L),
                                error = o.optString("error", ""),
                                supportsTools = if (o.has("supports_tools") && !o.isNull("supports_tools")) {
                                    o.optBoolean("supports_tools")
                                } else null,
                                supportsThinking = if (o.has("supports_thinking") && !o.isNull("supports_thinking")) {
                                    o.optBoolean("supports_thinking")
                                } else null
                            )
                        )
                    }
                }
            }
            BatchModelProbePayload(
                results = results,
                modelHealth = JsonMappers.parseModelHealth(json.optJSONObject("model_health")),
                pooledModels = if (json.has("pooled_models")) JsonMappers.parsePooledModels(json.optJSONArray("pooled_models")) else null,
                modelOptions = if (json.has("model_options")) JsonMappers.parseModelOptions(json.optJSONArray("model_options")) else null
            )
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

    suspend fun saveTask(
        existingId: String?,
        name: String,
        type: String,
        botId: String,
        cronExpr: String,
        runAt: String,
        description: String,
        model: String,
        allowPaidFallback: Boolean,
        maxPaidCallsPerDay: Int,
        enabled: Boolean,
        params: String = "{}"
    ): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val id = existingId?.trim().takeUnless { it.isNullOrEmpty() }
                ?: "task_${System.currentTimeMillis() % 100000000}"
            val payload = JSONObject()
                .put("id", id)
                .put("name", name.trim())
                .put("type", type.ifBlank { "bot_prompt" })
                .put("bot_id", botId.trim())
                .put("cron_expr", cronExpr.trim())
                .put("run_at", runAt.trim())
                .put("description", description.trim())
                .put("model", model.trim())
                .put("allow_paid_fallback", allowPaidFallback)
                .put("max_paid_calls_per_day", maxPaidCallsPerDay.coerceAtLeast(0))
                .put("enabled", enabled)
                .put("params", params.ifBlank { "{}" })
                .toString()
            val path = if (existingId.isNullOrBlank()) "/api/tasks" else "/api/tasks/${existingId.trim()}"
            val req = requestBuilder(path)
                .post(jsonBody(payload))
                .build()
            executeUnit(req, "保存定时任务失败")
        }
    }

    suspend fun deleteTask(taskId: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/tasks/${taskId.trim()}")
                .delete()
                .build()
            executeUnit(req, "删除定时任务失败")
        }
    }

    suspend fun createTodo(
        title: String,
        notes: String = "",
        dueAt: String = "",
        priority: String = "normal"
    ): Result<TodoItemUi> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
                .put("title", title.trim())
                .put("notes", notes.trim())
                .put("due_at", dueAt.trim())
                .put("priority", priority.ifBlank { "normal" })
                .toString()
            val req = requestBuilder("/api/v2/todos")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "创建待办失败")
            JsonMappers.parseTodo(json.optJSONObject("todo") ?: JSONObject())
        }
    }

    suspend fun updateTodo(
        id: String,
        title: String? = null,
        notes: String? = null,
        status: String? = null,
        dueAt: String? = null,
        priority: String? = null
    ): Result<TodoItemUi> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
            if (title != null) payload.put("title", title.trim())
            if (notes != null) payload.put("notes", notes.trim())
            if (status != null) payload.put("status", status.trim())
            if (dueAt != null) payload.put("due_at", dueAt.trim())
            if (priority != null) payload.put("priority", priority.trim())
            val req = requestBuilder("/api/v2/todos/${id.trim()}")
                .post(jsonBody(payload.toString()))
                .build()
            val json = executeJson(req, "更新待办失败")
            JsonMappers.parseTodo(json.optJSONObject("todo") ?: JSONObject())
        }
    }

    suspend fun deleteTodo(id: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/todos/${id.trim()}")
                .delete()
                .build()
            executeUnit(req, "删除待办失败")
        }
    }

    suspend fun saveNote(
        existingId: String?,
        title: String,
        content: String,
        category: String = "deliverable",
        pinned: Boolean = false
    ): Result<WorkNoteItemUi> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
                .put("title", title.trim())
                .put("content", content.trim())
                .put("category", category.ifBlank { "deliverable" })
                .put("pinned", pinned)
                .toString()
            val path = if (existingId.isNullOrBlank()) "/api/v2/notes" else "/api/v2/notes/${existingId.trim()}"
            val req = requestBuilder(path)
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "保存笔记失败")
            JsonMappers.parseNote(json.optJSONObject("note") ?: JSONObject())
        }
    }

    suspend fun deleteNote(id: String): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/notes/${id.trim()}")
                .delete()
                .build()
            executeUnit(req, "删除笔记失败")
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
                .put("category", category.ifBlank { "general" })
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

    suspend fun saveAssetCategories(categories: List<AssetCategoryItem>): Result<List<AssetCategoryItem>> =
        withContext(Dispatchers.IO) {
            runCatching {
                val arr = JSONArray()
                categories.forEach { cat ->
                    arr.put(
                        JSONObject()
                            .put("id", cat.id.trim())
                            .put("label", cat.label.trim())
                            .put("icon", cat.icon.ifBlank { "tag" })
                    )
                }
                val payload = JSONObject().put("categories", arr).toString()
                val req = requestBuilder("/api/v2/asset-categories")
                    .put(jsonBody(payload))
                    .build()
                val json = executeJson(req, "保存资产分类失败")
                val savedArr = json.optJSONArray("categories") ?: arr
                JsonMappers.parseAssetCategories(savedArr)
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

    suspend fun fetchBotMessagesPayload(
        botId: String,
        threadId: String? = null
    ): Result<BotMessagesPayload> = withContext(Dispatchers.IO) {
        runCatching {
            val threadParam = if (threadId != null) {
                "&thread_id=${URLEncoder.encode(threadId, "UTF-8")}"
            } else {
                ""
            }
            val req = requestBuilder("/api/v2/bots/$botId/messages?compact=1&include_threads=1&limit=60$threadParam").get().build()
            val json = executeJson(req, "加载 Bot 对话流失败")
            BotMessagesPayload(
                messages = JsonMappers.parseHistoryMessages(json.optJSONArray("messages")),
                threads = JsonMappers.parseBotThreads(json.optJSONArray("threads")),
                activeThreadId = json.optString("thread_id", "")
            )
        }
    }

    suspend fun fetchBotMessages(botId: String): Result<List<AgentHistoryMessage>> =
        fetchBotMessagesPayload(botId, null).map { it.messages }

    suspend fun createBotThread(
        botId: String,
        title: String = ""
    ): Result<Pair<BotThreadItem, List<BotThreadItem>>> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject().put("title", title.trim()).toString()
            val req = requestBuilder("/api/v2/bots/$botId/threads")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "创建话题失败")
            val created = JsonMappers.parseBotThread(json.optJSONObject("thread") ?: JSONObject())
            val list = JsonMappers.parseBotThreads(json.optJSONArray("threads"))
            created to list
        }
    }

    suspend fun deleteBotThread(
        botId: String,
        threadId: String
    ): Result<List<BotThreadItem>> = withContext(Dispatchers.IO) {
        runCatching {
            val encTid = URLEncoder.encode(threadId.trim(), "UTF-8")
            val req = requestBuilder("/api/v2/bots/$botId/threads/$encTid")
                .delete()
                .build()
            val json = executeJson(req, "删除话题失败")
            JsonMappers.parseBotThreads(json.optJSONArray("threads"))
        }
    }

    suspend fun summarizeBotThread(
        botId: String,
        threadId: String,
        saveToMemory: Boolean = true
    ): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            val cleanTid = threadId.trim().ifEmpty { "main" }
            val encTid = URLEncoder.encode(cleanTid, "UTF-8")
            val payload = JSONObject().put("save_to_memory", saveToMemory).toString()
            val req = requestBuilder("/api/v2/bots/$botId/threads/$encTid/summarize")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "提炼话题摘要失败")
            json.optString("summary", "").trim()
        }
    }

    suspend fun forkBotThread(
        botId: String,
        sourceThreadId: String,
        upToMsgId: Long = 0L,
        title: String = ""
    ): Result<Pair<BotThreadItem, BotMessagesPayload>> = withContext(Dispatchers.IO) {
        runCatching {
            val payloadObj = JSONObject()
                .put("source_thread_id", sourceThreadId.trim())
                .put("title", title.trim())
            if (upToMsgId > 0L) payloadObj.put("up_to_msg_id", upToMsgId)
            val req = requestBuilder("/api/v2/bots/$botId/threads/fork")
                .post(jsonBody(payloadObj.toString()))
                .build()
            val json = executeJson(req, "分叉新话题失败")
            val created = JsonMappers.parseBotThread(json.optJSONObject("thread") ?: JSONObject())
            val msgPayload = BotMessagesPayload(
                messages = JsonMappers.parseHistoryMessages(json.optJSONArray("messages")),
                threads = JsonMappers.parseBotThreads(json.optJSONArray("threads")),
                activeThreadId = created.id
            )
            created to msgPayload
        }
    }

    suspend fun fetchMemories(scope: String): Result<List<MemoryItemUi>> = withContext(Dispatchers.IO) {
        runCatching {
            val encScope = URLEncoder.encode(scope.trim(), "UTF-8")
            val req = requestBuilder("/api/agent/memories?scope=$encScope").get().build()
            val json = executeJson(req, "读取长期记忆失败")
            JsonMappers.parseMemories(json.optJSONArray("memories"))
        }
    }

    suspend fun saveMemory(
        scope: String,
        category: String,
        factKey: String,
        content: String,
        pinned: Boolean = false
    ): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
                .put("scope", scope.trim())
                .put("category", category.ifBlank { "fact" })
                .put("fact_key", factKey.trim())
                .put("content", content.trim())
                .put("pinned", pinned)
                .toString()
            val req = requestBuilder("/api/agent/memories")
                .post(jsonBody(payload))
                .build()
            executeUnit(req, "写入长期记忆失败")
        }
    }

    suspend fun toggleMemoryPin(id: Long, pinned: Boolean): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject().put("pinned", pinned).toString()
            val req = requestBuilder("/api/agent/memories/$id")
                .post(jsonBody(payload))
                .build()
            executeUnit(req, "更新记忆置顶状态失败")
        }
    }

    suspend fun deleteMemory(id: Long): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/agent/memories/$id")
                .delete()
                .build()
            executeUnit(req, "删除长期记忆失败")
        }
    }

    suspend fun clearBotMessages(botId: String, threadId: String? = null): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val qs = if (threadId != null) "?thread_id=${URLEncoder.encode(threadId, "UTF-8")}" else ""
            val req = requestBuilder("/api/v2/bots/$botId/messages$qs")
                .delete()
                .build()
            executeUnit(req, "清空聊天记录失败")
        }
    }

    suspend fun deleteBotMessage(botId: String, msgId: Long): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/bots/$botId/messages/$msgId")
                .delete()
                .build()
            executeUnit(req, "删除消息失败")
        }
    }

    suspend fun forwardBotMessage(
        targetBotId: String,
        fromName: String,
        content: String,
        note: String = ""
    ): Result<Unit> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
                .put("from_name", fromName.trim())
                .put("content", content.trim())
                .put("note", note.trim())
                .toString()
            val req = requestBuilder("/api/v2/bots/$targetBotId/forward")
                .post(jsonBody(payload))
                .build()
            executeUnit(req, "转发消息失败")
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

    suspend fun updateBotPreferences(
        botId: String,
        model: String? = null,
        enableThinking: Boolean? = null,
        allowPaidFallback: Boolean? = null
    ): Result<BotItem> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
            if (!model.isNullOrBlank()) payload.put("model", model.trim())
            if (enableThinking != null) payload.put("enable_thinking", enableThinking)
            if (allowPaidFallback != null) payload.put("allow_paid_fallback", allowPaidFallback)
            val req = requestBuilder("/api/v2/bots/$botId")
                .post(jsonBody(payload.toString()))
                .build()
            val json = executeJson(req, "同步模型偏好失败")
            JsonMappers.parseBot(json.optJSONObject("bot") ?: JSONObject())
        }
    }

    suspend fun updateGroupMembers(
        groupId: String,
        memberIds: List<String>
    ): Result<BotItem> = withContext(Dispatchers.IO) {
        runCatching {
            val arr = JSONArray()
            memberIds.forEach { arr.put(it) }
            val payload = JSONObject()
                .put("action", "set")
                .put("bot_ids", arr)
                .toString()
            val req = requestBuilder("/api/v2/bots/$groupId/members")
                .post(jsonBody(payload))
                .build()
            val json = executeJson(req, "更新群成员失败")
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
        announcement: String,
        allowPaidFallback: Boolean? = null,
        allowedTools: List<String>? = null,
        skillIds: List<String>? = null
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
            if (allowPaidFallback != null) {
                payload.put("allow_paid_fallback", allowPaidFallback)
            }
            if (allowedTools != null) {
                val arr = JSONArray()
                allowedTools.forEach { arr.put(it) }
                payload.put("allowed_tools", arr.toString())
            }
            if (skillIds != null) {
                val arr = JSONArray()
                skillIds.forEach { arr.put(it) }
                payload.put("skill_ids", arr.toString())
            }
            val req = requestBuilder("/api/v2/bots/$botId")
                .post(jsonBody(payload.toString()))
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
        systemPrompt: String,
        model: String = "auto:free",
        allowPaidFallback: Boolean = false,
        allowedTools: List<String>? = null,
        skillIds: List<String> = emptyList()
    ): Result<BotItem> = withContext(Dispatchers.IO) {
        runCatching {
            val toolsArr = JSONArray()
            if (allowedTools != null) {
                allowedTools.forEach { toolsArr.put(it) }
            } else if (botMode == "companion") {
                toolsArr.put("remember_fact").put("recall_memory")
            } else {
                listOf(
                    "manage_todos", "manage_notes", "manage_stash", "manage_tasks",
                    "manage_assets", "query_nezha_nodes", "web_search", "read_url",
                    "calculate_eval", "datetime_calc", "remember_fact", "recall_memory"
                ).forEach { toolsArr.put(it) }
            }
            val skillArr = JSONArray()
            skillIds.forEach { skillArr.put(it) }
            val payload = JSONObject()
                .put("name", name.trim())
                .put("role_title", roleTitle.trim())
                .put("avatar_style", avatarStyle)
                .put("bot_mode", botMode)
                .put("model", model.ifBlank { "auto:free" })
                .put("allow_paid_fallback", allowPaidFallback)
                .put("traits", traits.trim())
                .put("system_prompt", systemPrompt.trim())
                .put("allowed_tools", toolsArr.toString())
                .put("skill_ids", skillArr.toString())
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

    suspend fun triggerTestPush(): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/test-push")
                .post(emptyJsonBody)
                .build()
            val json = executeJson(req, "发送测试推送失败")
            json.optString("message", "测试通知已发送").ifBlank { "测试通知已发送" }
        }
    }

    suspend fun createStashItem(
        rawInput: String,
        userNote: String = "",
        tags: String = "",
        prefetched: PrefetchedShareMeta? = null
    ): Result<Pair<StashItemUi, Boolean>> = withContext(Dispatchers.IO) {
        runCatching {
            val payloadObj = JSONObject()
                .put("raw_input", rawInput.trim())
                .put("user_note", userNote.trim())
                .put("tags", tags.trim())
            if (prefetched != null && !prefetched.isEmpty) {
                if (prefetched.canonicalUrl.isNotBlank()) payloadObj.put("client_url", prefetched.canonicalUrl.trim())
                if (prefetched.title.isNotBlank()) payloadObj.put("client_title", prefetched.title.trim())
                if (prefetched.excerpt.isNotBlank()) payloadObj.put("client_excerpt", prefetched.excerpt.trim())
                if (prefetched.tags.isNotBlank()) payloadObj.put("client_tags", prefetched.tags.trim())
                if (prefetched.imageDataUrl.isNotBlank()) payloadObj.put("client_image_data_url", prefetched.imageDataUrl.trim())
            }
            val payload = payloadObj.toString()
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
        title: String? = null,
        userNote: String? = null,
        tags: String? = null,
        pinned: Boolean? = null,
        markRead: Boolean? = null,
        markUnread: Boolean? = null,
        status: String? = null
    ): Result<StashItemUi> = withContext(Dispatchers.IO) {
        runCatching {
            val payload = JSONObject()
            if (title != null) payload.put("title", title.trim())
            if (userNote != null) payload.put("user_note", userNote.trim())
            if (tags != null) payload.put("tags", tags.trim())
            if (pinned != null) payload.put("pinned", pinned)
            if (markRead != null) payload.put("mark_read", markRead)
            if (markUnread != null) payload.put("mark_unread", markUnread)
            if (status != null) payload.put("status", status)
            val req = requestBuilder("/api/v2/stash/$id")
                .post(jsonBody(payload.toString()))
                .build()
            val json = executeJson(req, "更新收藏失败")
            val itemObj = json.optJSONObject("item") ?: JSONObject()
            JsonMappers.parseStashItem(itemObj)
        }
    }

    suspend fun markAllStashRead(): Result<Int> = withContext(Dispatchers.IO) {
        runCatching {
            val req = requestBuilder("/api/v2/stash/mark-all-read")
                .post(emptyJsonBody)
                .build()
            val json = executeJson(req, "全部标为已读失败")
            json.optInt("updated", 0)
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
        threadId: String = "",
        message: String,
        persona: String,
        model: String,
        enableThinking: Boolean = true,
        allowPaidFallback: Boolean? = null,
        onCallCreated: (Call) -> Unit = {},
        onSession: (String) -> Unit,
        onReasoningDelta: (String) -> Unit,
        onToolCall: (String, String) -> Unit,
        onToolDetail: (id: String, name: String, arguments: String, result: String, status: String) -> Unit = { _, _, _, _, _ -> },
        onContentDelta: (String) -> Unit,
        onDoneModel: (String) -> Unit = {},
        onError: (String) -> Unit
    ) = withContext(Dispatchers.IO) {
        runCatching {
            val payloadObj = JSONObject()
                .put("session_id", sessionId)
                .put("bot_id", botId)
                .put("thread_id", threadId)
                .put("message", message)
                .put("persona", persona)
                .put("model", model)
                .put("enable_thinking", enableThinking)
            if (allowPaidFallback != null) {
                payloadObj.put("allow_paid_fallback", allowPaidFallback)
            }
            val payload = payloadObj.toString()

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
                            val tcId = dataObj.optString("id")
                            val tcName = dataObj.optString("name")
                            val tcArgs = dataObj.optString("arguments")
                            onToolCall(tcName, "执行中")
                            onToolDetail(tcId, tcName, tcArgs, "", "进行中")
                        }
                        "tool_result" -> {
                            val tcId = dataObj.optString("id")
                            val tcName = dataObj.optString("name")
                            val tcResult = dataObj.optString("result")
                            val tcStatus = dataObj.optString("status", "已完成").ifBlank { "已完成" }
                            onToolCall(tcName, "已完成")
                            onToolDetail(tcId, tcName, "", tcResult, tcStatus)
                        }
                        "content" -> {
                            val delta = dataObj.optString("delta")
                            if (delta.isNotEmpty()) onContentDelta(delta)
                        }
                        "done" -> {
                            val usedModel = dataObj.optString("model", "").trim()
                            if (usedModel.isNotEmpty()) onDoneModel(usedModel)
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
