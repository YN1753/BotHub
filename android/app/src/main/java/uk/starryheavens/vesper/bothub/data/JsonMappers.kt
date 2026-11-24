package uk.starryheavens.vesper.bothub.data

import org.json.JSONArray
import org.json.JSONObject

object JsonMappers {

    fun extractApiError(rawBody: String, fallbackMessage: String): String {
        val serverErr = runCatching {
            JSONObject(rawBody).optString("error", "").trim()
        }.getOrDefault("")
        return serverErr.ifEmpty { fallbackMessage }
    }

    fun optStringList(
        obj: JSONObject,
        primaryKey: String,
        fallbackKey: String? = null
    ): List<String> {
        obj.optJSONArray(primaryKey)?.let { return jsonArrayToStrings(it) }
        val rawPrimary = obj.optString(primaryKey, "").trim()
        if (rawPrimary.startsWith("[")) {
            runCatching { JSONArray(rawPrimary) }.getOrNull()?.let { return jsonArrayToStrings(it) }
        }
        if (fallbackKey != null) {
            obj.optJSONArray(fallbackKey)?.let { return jsonArrayToStrings(it) }
            val rawFallback = obj.optString(fallbackKey, "").trim()
            if (rawFallback.startsWith("[")) {
                runCatching { JSONArray(rawFallback) }.getOrNull()?.let { return jsonArrayToStrings(it) }
            }
        }
        return emptyList()
    }

    private fun jsonArrayToStrings(arr: JSONArray): List<String> = buildList {
        for (i in 0 until arr.length()) {
            val v = arr.optString(i, "").trim()
            if (v.isNotEmpty()) add(v)
        }
    }

    fun parseNotification(n: JSONObject): NotificationItem {
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

    fun parseNotifications(arr: JSONArray?): List<NotificationItem> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val n = arr.optJSONObject(i) ?: continue
                add(parseNotification(n))
            }
        }
    }

    fun serializeNotifications(items: List<NotificationItem>, maxItems: Int = 50): String {
        val arr = JSONArray()
        items.take(maxItems).forEach { n ->
            arr.put(
                JSONObject()
                    .put("id", n.id)
                    .put("module_id", n.moduleId)
                    .put("title", n.title)
                    .put("summary", n.summary)
                    .put("markdown_body", n.markdownBody)
                    .put("priority", n.priority)
                    .put("created_at", n.createdAt)
                    .put("read_at", n.readAt)
            )
        }
        return arr.toString()
    }

    fun deserializeNotifications(rawJson: String): List<NotificationItem> {
        if (rawJson.isBlank()) return emptyList()
        return runCatching {
            parseNotifications(JSONArray(rawJson))
        }.getOrDefault(emptyList())
    }

    fun parseBot(b: JSONObject): BotItem {
        val rawModel = normalizeModelIdForSinglePool(b.optString("model", ""))
        return BotItem(
            id = b.optString("id"),
            name = b.optString("name"),
            roleTitle = b.optString("role_title"),
            avatarStyle = b.optString("avatar_style", "butler:gold"),
            model = rawModel,
            state = b.optString("state", "idle"),
            unreadCount = b.optInt("unread_count", 0),
            lastSummary = b.optString("last_summary", ""),
            isGroup = b.optBoolean("is_group", false),
            memberIds = optStringList(b, "member_ids"),
            botMode = b.optString("bot_mode", "worker"),
            isPinned = b.optBoolean("is_pinned", false),
            isBuiltin = b.optBoolean("is_builtin", false),
            traits = b.optString("traits", ""),
            systemPrompt = b.optString("system_prompt", ""),
            groupMode = b.optString("group_mode", "smart"),
            announcement = b.optString("announcement", ""),
            updatedAt = b.optString("updated_at", ""),
            allowedTools = optStringList(b, "allowed_tools"),
            skillIds = optStringList(b, "skill_ids")
        )
    }

    fun parseSkill(s: JSONObject): SkillItemUi? {
        val id = s.optString("id", "").trim()
        if (id.isEmpty()) return null
        val paramsRaw = s.optString("params_schema", "").trim()
        val toolCount = if (paramsRaw.startsWith("[")) {
            runCatching { JSONArray(paramsRaw).length() }.getOrDefault(0)
        } else {
            s.optJSONArray("params_schema")?.length() ?: 0
        }
        return SkillItemUi(
            id = id,
            slug = s.optString("slug", ""),
            name = s.optString("name", id).ifBlank { id },
            description = s.optString("description", ""),
            kind = s.optString("kind", "prompt").ifBlank { "prompt" },
            httpUrl = s.optString("http_url", ""),
            toolCount = toolCount,
            enabled = s.optBoolean("enabled", true)
        )
    }

    fun parseSkills(arr: JSONArray?): List<SkillItemUi> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val s = arr.optJSONObject(i) ?: continue
                parseSkill(s)?.let { add(it) }
            }
        }
    }

    fun parseMemories(arr: JSONArray?): List<MemoryItemUi> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val m = arr.optJSONObject(i) ?: continue
                val id = m.optLong("id", 0L)
                if (id > 0L) {
                    add(
                        MemoryItemUi(
                            id = id,
                            scope = m.optString("scope", "global"),
                            category = m.optString("category", "fact"),
                            factKey = m.optString("fact_key", ""),
                            content = m.optString("content", ""),
                            importance = m.optInt("importance", 3),
                            pinned = m.optBoolean("pinned", false),
                            updatedAt = m.optString("updated_at", "")
                        )
                    )
                }
            }
        }
    }

    fun parseBots(arr: JSONArray?): List<BotItem> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val b = arr.optJSONObject(i) ?: continue
                add(parseBot(b))
            }
        }
    }

    fun serializeBots(items: List<BotItem>): String {
        val arr = JSONArray()
        items.forEach { b ->
            val membersArr = JSONArray()
            b.memberIds.forEach { membersArr.put(it) }
            arr.put(
                JSONObject()
                    .put("id", b.id)
                    .put("name", b.name)
                    .put("role_title", b.roleTitle)
                    .put("avatar_style", b.avatarStyle)
                    .put("model", b.model)
                    .put("state", b.state)
                    .put("unread_count", b.unreadCount)
                    .put("last_summary", b.lastSummary)
                    .put("is_group", b.isGroup)
                    .put("member_ids", membersArr)
                    .put("bot_mode", b.botMode)
                    .put("is_pinned", b.isPinned)
                    .put("is_builtin", b.isBuiltin)
                    .put("traits", b.traits)
                    .put("system_prompt", b.systemPrompt)
                    .put("group_mode", b.groupMode)
                    .put("announcement", b.announcement)
                    .put("updated_at", b.updatedAt)
                    .put("allowed_tools", JSONArray().apply { b.allowedTools.forEach { put(it) } })
                    .put("skill_ids", JSONArray().apply { b.skillIds.forEach { put(it) } })
            )
        }
        return arr.toString()
    }

    fun deserializeBots(rawJson: String): List<BotItem> {
        if (rawJson.isBlank()) return emptyList()
        return runCatching { parseBots(JSONArray(rawJson)) }.getOrDefault(emptyList())
    }

    fun parseStashItem(s: JSONObject): StashItemUi {
        return StashItemUi(
            id = s.optString("id"),
            kind = s.optString("kind", "link"),
            url = s.optString("url", ""),
            domain = s.optString("domain", ""),
            title = s.optString("title", ""),
            rawInput = s.optString("raw_input", ""),
            userNote = s.optString("user_note", ""),
            aiTldr = s.optString("ai_tldr", ""),
            points = optStringList(s, "points", fallbackKey = "ai_points"),
            contentExcerpt = s.optString("content_excerpt", ""),
            tags = s.optString("tags", ""),
            status = s.optString("status", "ready"),
            errorMsg = s.optString("error_msg", ""),
            pinned = s.optBoolean("pinned", false),
            readCount = s.optInt("read_count", 0),
            createdAt = s.optString("created_at", "")
        )
    }

    fun parseStashItems(arr: JSONArray?): List<StashItemUi> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val s = arr.optJSONObject(i) ?: continue
                add(parseStashItem(s))
            }
        }
    }

    fun serializeStashItems(items: List<StashItemUi>, maxItems: Int = 80): String {
        val arr = JSONArray()
        items.take(maxItems).forEach { s ->
            val pts = JSONArray()
            s.points.forEach { pts.put(it) }
            arr.put(
                JSONObject()
                    .put("id", s.id)
                    .put("kind", s.kind)
                    .put("url", s.url)
                    .put("domain", s.domain)
                    .put("title", s.title)
                    .put("raw_input", s.rawInput)
                    .put("user_note", s.userNote)
                    .put("ai_tldr", s.aiTldr)
                    .put("points", pts)
                    .put("content_excerpt", s.contentExcerpt.take(2400))
                    .put("tags", s.tags)
                    .put("status", s.status)
                    .put("error_msg", s.errorMsg)
                    .put("pinned", s.pinned)
                    .put("read_count", s.readCount)
                    .put("created_at", s.createdAt)
            )
        }
        return arr.toString()
    }

    fun deserializeStashItems(rawJson: String): List<StashItemUi> {
        if (rawJson.isBlank()) return emptyList()
        return runCatching { parseStashItems(JSONArray(rawJson)) }.getOrDefault(emptyList())
    }

    fun parseToolCallsJson(
        rawToolCalls: String,
        toolResultsByCallId: Map<String, String> = emptyMap()
    ): List<ChatToolTraceUi> {
        val clean = rawToolCalls.trim()
        if (clean.isEmpty() || clean == "[]" || clean == "null") return emptyList()
        val arr = runCatching { JSONArray(clean) }.getOrNull() ?: return emptyList()
        return parseToolCallsArray(arr, toolResultsByCallId)
    }

    private fun parseToolCallsArray(
        arr: JSONArray,
        toolResultsByCallId: Map<String, String>
    ): List<ChatToolTraceUi> = buildList {
        for (i in 0 until arr.length()) {
            val tc = arr.optJSONObject(i) ?: continue
            val tcId = tc.optString("id", "").trim().ifEmpty { "tool_$i" }
            val fn = tc.optJSONObject("function")
            val name = (fn?.optString("name", "") ?: tc.optString("name", ""))
                .trim()
                .ifEmpty { "tool" }
            val arguments = fn?.optString("arguments", "{}")
                ?: tc.optString("arguments", "{}")
            val result = toolResultsByCallId[tcId].orEmpty()
            add(
                ChatToolTraceUi(
                    id = tcId,
                    name = name,
                    displayName = formatHumanToolName(name),
                    arguments = arguments,
                    result = result,
                    status = "已完成"
                )
            )
        }
    }

    fun parseHistoryMessages(arr: JSONArray?): List<AgentHistoryMessage> {
        if (arr == null) return emptyList()

        // First pass: collect tool execution results keyed by tool_call_id.
        val toolResultsByCallId = mutableMapOf<String, String>()
        for (i in 0 until arr.length()) {
            val m = arr.optJSONObject(i) ?: continue
            if (m.optString("role") == "tool") {
                val callId = m.optString("tool_call_id", "").trim()
                if (callId.isNotEmpty()) {
                    toolResultsByCallId[callId] = m.optString("content", "")
                }
            }
        }

        // Second pass: reconstruct user/system/assistant messages and merge multi-round assistant tool turns.
        val out = mutableListOf<AgentHistoryMessage>()
        for (i in 0 until arr.length()) {
            val m = arr.optJSONObject(i) ?: continue
            val role = m.optString("role")
            val msgKind = m.optString("msg_kind", "chat")
            if (role != "user" && role != "assistant" && !(role == "system" && msgKind == "system")) {
                continue
            }

            val traces = if (role == "assistant") {
                m.optJSONArray("tool_calls")?.let { parseToolCallsArray(it, toolResultsByCallId) }
                    ?: parseToolCallsJson(m.optString("tool_calls", "[]"), toolResultsByCallId)
            } else {
                emptyList()
            }

            val mapped = AgentHistoryMessage(
                id = m.optLong("id"),
                role = role,
                content = m.optString("content", ""),
                reasoningContent = m.optString("reasoning_content", ""),
                msgKind = msgKind,
                senderBotId = m.optString("sender_bot_id", ""),
                modelUsed = m.optString("model_used", ""),
                modelTargetId = m.optString("model_target_id", ""),
                attachments = optStringList(m, "attachments"),
                toolTraces = traces
            )

            val prev = out.lastOrNull()
            if (
                prev != null &&
                prev.role == "assistant" &&
                mapped.role == "assistant" &&
                prev.senderBotId == mapped.senderBotId &&
                prev.toolTraces.isNotEmpty()
            ) {
                val mergedReasoning = when {
                    mapped.reasoningContent.isBlank() -> prev.reasoningContent
                    prev.reasoningContent.isBlank() -> mapped.reasoningContent
                    prev.reasoningContent.contains(mapped.reasoningContent) -> prev.reasoningContent
                    else -> "${prev.reasoningContent}\n${mapped.reasoningContent}"
                }
                out[out.lastIndex] = prev.copy(
                    id = mapped.id,
                    reasoningContent = mergedReasoning,
                    toolTraces = prev.toolTraces + mapped.toolTraces,
                    content = mapped.content.ifEmpty { prev.content },
                    modelUsed = mapped.modelUsed.ifBlank { prev.modelUsed },
                    modelTargetId = mapped.modelTargetId.ifBlank { prev.modelTargetId }
                )
                continue
            }

            out.add(mapped)
        }
        return out
    }

    fun parseAssetCategories(catArr: JSONArray?): List<AssetCategoryItem> {
        if (catArr == null) return emptyList()
        return buildList {
            for (i in 0 until catArr.length()) {
                val c = catArr.optJSONObject(i) ?: continue
                val cid = c.optString("id", "").trim()
                val label = c.optString("label", "").trim().ifEmpty { cid }
                if (cid.isNotEmpty() || label.isNotEmpty()) {
                    add(
                        AssetCategoryItem(
                            id = cid.ifEmpty { label },
                            label = label,
                            icon = c.optString("icon", "tag").ifBlank { "tag" }
                        )
                    )
                }
            }
        }
    }

    fun parseModelOptions(arr: JSONArray?): List<ModelOptionItem> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val o = arr.optJSONObject(i) ?: continue
                val id = o.optString("id", "").trim()
                if (id.isNotEmpty()) {
                    add(
                        ModelOptionItem(
                            id = id,
                            label = o.optString("label", id).ifBlank { id },
                            isAuto = o.optBoolean("is_auto", id == "auto"),
                            isLight = o.optBoolean("is_light", false),
                            provider = o.optString("provider", ""),
                            model = o.optString("model", ""),
                            available = o.optBoolean("available", true)
                        )
                    )
                }
            }
        }
    }

    fun parseModelProviders(arr: JSONArray?): List<ModelProviderItem> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val o = arr.optJSONObject(i) ?: continue
                val id = o.optString("id", "").trim()
                if (id.isNotEmpty()) {
                    val keysList = optStringList(o, "api_keys")
                    val primaryKey = o.optString("api_key", "").ifBlank { keysList.firstOrNull().orEmpty() }
                    add(
                        ModelProviderItem(
                            id = id,
                            name = o.optString("name", id).ifBlank { id },
                            protocol = o.optString("protocol", "openai").ifBlank { "openai" },
                            baseUrl = o.optString("base_url", ""),
                            apiKey = primaryKey,
                            apiKeys = keysList.ifEmpty { listOfNotNull(primaryKey.takeIf { it.isNotBlank() }) },
                            enabled = o.optBoolean("enabled", true),
                            note = o.optString("note", "")
                        )
                    )
                }
            }
        }
    }

    fun parsePooledModels(arr: JSONArray?): List<PooledModelItem> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val o = arr.optJSONObject(i) ?: continue
                val pid = o.optString("provider_id", "").trim()
                val mid = o.optString("model_id", "").trim()
                if (pid.isNotEmpty() && mid.isNotEmpty()) {
                    add(
                        PooledModelItem(
                            providerId = pid,
                            modelId = mid,
                            displayName = o.optString("display_name", ""),
                            enabled = o.optBoolean("enabled", true),
                            priority = o.optInt("priority", 10),
                            isLight = o.optBoolean("is_light", false),
                            inputPricePerM = o.optDouble("input_price_per_m", 0.0).takeIf { !it.isNaN() && it > 0.0 } ?: 0.0,
                            outputPricePerM = o.optDouble("output_price_per_m", 0.0).takeIf { !it.isNaN() && it > 0.0 } ?: 0.0,
                            supportsTools = if (o.has("supports_tools") && !o.isNull("supports_tools")) {
                                o.optBoolean("supports_tools")
                            } else null,
                            supportsThinking = if (o.has("supports_thinking") && !o.isNull("supports_thinking")) {
                                o.optBoolean("supports_thinking")
                            } else null,
                            lastLatencyMs = o.optLong("last_latency_ms", 0L)
                        )
                    )
                }
            }
        }
    }

    fun parsePoolStrategy(raw: String?): String {
        return when (raw?.trim()?.lowercase().orEmpty()) {
            "round_robin" -> "round_robin"
            "lowest_latency" -> "lowest_latency"
            else -> "priority"
        }
    }

    fun parseModelHealth(obj: JSONObject?): Map<String, ModelHealthItem> {
        if (obj == null) return emptyMap()
        return buildMap {
            val keys = obj.keys()
            while (keys.hasNext()) {
                val k = keys.next()
                val item = obj.optJSONObject(k) ?: continue
                put(
                    k,
                    ModelHealthItem(
                        targetId = item.optString("target_id", k).ifBlank { k },
                        status = item.optString("status", "healthy"),
                        successCount = item.optLong("success_count", 0L),
                        failCount = item.optLong("fail_count", 0L),
                        lastLatencyMs = item.optLong("last_latency_ms", 0L),
                        lastError = item.optString("last_error", ""),
                        cooldownSec = item.optLong("cooldown_sec", 0L)
                    )
                )
            }
        }
    }

    fun parseModelUsageSummary(obj: JSONObject?, defaultDays: Int = 7): ModelUsageSummaryItem {
        if (obj == null) return ModelUsageSummaryItem(days = defaultDays)
        val topModelsArr = obj.optJSONArray("top_models")
        val topModels = buildList {
            if (topModelsArr != null) {
                for (i in 0 until topModelsArr.length()) {
                    val m = topModelsArr.optJSONObject(i) ?: continue
                    add(
                        ModelUsageModelStatItem(
                            targetId = m.optString("target_id", ""),
                            modelId = m.optString("model_id", ""),
                            calls = m.optInt("calls", 0),
                            tokens = m.optInt("tokens", 0),
                            cachedTokens = m.optInt("cached_tokens", 0),
                            costUsd = m.optDouble("cost_usd", 0.0).takeIf { !it.isNaN() } ?: 0.0,
                            avgLatencyMs = m.optLong("avg_latency_ms", 0L)
                        )
                    )
                }
            }
        }
        val topCallersArr = obj.optJSONArray("top_callers")
        val topCallers = buildList {
            if (topCallersArr != null) {
                for (i in 0 until topCallersArr.length()) {
                    val c = topCallersArr.optJSONObject(i) ?: continue
                    add(
                        ModelUsageCallerStatItem(
                            botId = c.optString("bot_id", ""),
                            taskId = c.optString("task_id", ""),
                            source = c.optString("source", ""),
                            calls = c.optInt("calls", 0),
                            tokens = c.optInt("tokens", 0),
                            costUsd = c.optDouble("cost_usd", 0.0).takeIf { !it.isNaN() } ?: 0.0
                        )
                    )
                }
            }
        }
        return ModelUsageSummaryItem(
            days = obj.optInt("days", defaultDays),
            totalCalls = obj.optInt("total_calls", 0),
            successCalls = obj.optInt("success_calls", 0),
            totalPromptTokens = obj.optInt("total_prompt_tokens", 0),
            totalCachedTokens = obj.optInt("total_cached_tokens", 0),
            totalCompletionTokens = obj.optInt("total_completion_tokens", 0),
            totalTokens = obj.optInt("total_tokens", 0),
            cacheHitRate = obj.optDouble("cache_hit_rate", 0.0).takeIf { !it.isNaN() } ?: 0.0,
            totalCostUsd = obj.optDouble("total_cost_usd", 0.0).takeIf { !it.isNaN() } ?: 0.0,
            totalCostCny = obj.optDouble("total_cost_cny", 0.0).takeIf { !it.isNaN() } ?: 0.0,
            topModels = topModels,
            topCallers = topCallers
        )
    }

    fun parseModelConfigPayload(json: JSONObject): ModelConfigPayload {
        val provArr = json.optJSONArray("providers") ?: json.optJSONArray("model_providers")
        val modArr = json.optJSONArray("models") ?: json.optJSONArray("pooled_models")
        val rawStrat = json.optString("strategy", "").ifBlank { json.optString("pool_strategy", "priority") }
        return ModelConfigPayload(
            providers = parseModelProviders(provArr),
            models = parsePooledModels(modArr),
            strategy = parsePoolStrategy(rawStrat),
            modelOptions = parseModelOptions(json.optJSONArray("model_options")),
            modelHealth = parseModelHealth(json.optJSONObject("model_health")),
            usageSummary = parseModelUsageSummary(json.optJSONObject("model_usage_summary"), 7),
            usage30d = parseModelUsageSummary(json.optJSONObject("model_usage_30d"), 30)
        )
    }

    fun parseRevisions(obj: JSONObject?): Map<String, Long> {
        if (obj == null) return emptyMap()
        return buildMap {
            val keys = obj.keys()
            while (keys.hasNext()) {
                val k = keys.next()
                val v = obj.optLong(k, -1L)
                if (k.isNotBlank() && v >= 0L) {
                    put(k, v)
                }
            }
        }
    }

    fun parseBotThread(t: JSONObject): BotThreadItem {
        return BotThreadItem(
            id = t.optString("id", "").trim(),
            botId = t.optString("bot_id", "").trim(),
            title = t.optString("title", "新话题").ifBlank { "新话题" },
            summary = t.optString("summary", ""),
            msgCount = t.optInt("msg_count", 0),
            archived = t.optBoolean("archived", false),
            updatedAt = t.optString("updated_at", "")
        )
    }

    fun parseBotThreads(arr: JSONArray?): List<BotThreadItem> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val t = arr.optJSONObject(i) ?: continue
                val id = t.optString("id", "").trim()
                if (id.isNotEmpty()) {
                    add(parseBotThread(t))
                }
            }
        }
    }

    fun parseTodo(t: JSONObject): TodoItemUi {
        return TodoItemUi(
            id = t.optString("id", "").trim(),
            title = t.optString("title", "").trim(),
            notes = t.optString("notes", ""),
            status = t.optString("status", "todo").ifBlank { "todo" },
            priority = t.optString("priority", "normal").ifBlank { "normal" },
            dueAt = t.optString("due_at", ""),
            botId = t.optString("bot_id", ""),
            updatedAt = t.optString("updated_at", "")
        )
    }

    fun parseTodos(arr: JSONArray?): List<TodoItemUi> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val t = arr.optJSONObject(i) ?: continue
                val id = t.optString("id", "").trim()
                if (id.isNotEmpty()) {
                    add(parseTodo(t))
                }
            }
        }
    }

    fun parseNote(n: JSONObject): WorkNoteItemUi {
        return WorkNoteItemUi(
            id = n.optString("id", "").trim(),
            title = n.optString("title", "未命名笔记").ifBlank { "未命名笔记" },
            content = n.optString("content", ""),
            category = n.optString("category", "deliverable").ifBlank { "deliverable" },
            pinned = n.optBoolean("pinned", false),
            botId = n.optString("bot_id", ""),
            updatedAt = n.optString("updated_at", "")
        )
    }

    fun parseNotes(arr: JSONArray?): List<WorkNoteItemUi> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val n = arr.optJSONObject(i) ?: continue
                val id = n.optString("id", "").trim()
                if (id.isNotEmpty()) {
                    add(parseNote(n))
                }
            }
        }
    }

    fun parseTasks(tasksArr: JSONArray?): List<TaskItem> {
        if (tasksArr == null) return emptyList()
        return buildList {
            for (i in 0 until tasksArr.length()) {
                val t = tasksArr.optJSONObject(i) ?: continue
                val lastRun = t.optJSONObject("last_run")
                add(
                    TaskItem(
                        id = t.optString("id"),
                        name = t.optString("name"),
                        type = t.optString("type"),
                        cronExpr = t.optString("cron_expr"),
                        enabled = t.optBoolean("enabled", false),
                        nextRunAt = t.optString("next_run_at", "--"),
                        lastSummary = lastRun?.optString("summary")?.ifEmpty { "等待执行" } ?: "等待执行",
                        model = t.optString("model", ""),
                        botId = t.optString("bot_id", ""),
                        description = t.optString("description", ""),
                        runAt = t.optString("run_at", ""),
                        params = t.optString("params", "{}")
                    )
                )
            }
        }
    }

    fun parseAssets(assetsArr: JSONArray?): List<AssetItem> {
        if (assetsArr == null) return emptyList()
        return buildList {
            for (i in 0 until assetsArr.length()) {
                val a = assetsArr.optJSONObject(i) ?: continue
                add(
                    AssetItem(
                        id = a.optString("id"),
                        name = a.optString("name"),
                        category = a.optString("category", "general").ifBlank { "general" },
                        spec = a.optString("spec"),
                        price = a.optString("price"),
                        dueDate = a.optString("due_date", ""),
                        status = a.optString("status", "生效中").ifBlank { "生效中" },
                        daysLeft = a.optInt("days_left", 999),
                        autoRenew = a.optBoolean("auto_renew", false),
                        warnDays = a.optString("warn_days", "[7, 3, 1]").ifBlank { "[7, 3, 1]" },
                        extraConfig = a.optString("extra_config", "{}").ifBlank { "{}" }
                    )
                )
            }
        }
    }

    fun parseFXRates(fxObj: JSONObject?): Map<String, Double> {
        if (fxObj == null) return emptyMap()
        val ratesObj = fxObj.optJSONObject("rates") ?: fxObj
        return buildMap {
            val keys = ratesObj.keys()
            while (keys.hasNext()) {
                val k = keys.next().trim().uppercase()
                val v = ratesObj.optDouble(k, -1.0)
                if (k.isNotEmpty() && v > 0.0 && !v.isNaN()) {
                    put(k, v)
                }
            }
        }
    }

    fun parseDashboard(obj: JSONObject): DashboardSnapshot {
        val stats = obj.optJSONObject("stats") ?: JSONObject()
        val assetCategories = parseAssetCategories(obj.optJSONArray("asset_categories"))
        val fxRates = parseFXRates(obj.optJSONObject("fx_rates"))
        val assets = parseAssets(obj.optJSONArray("assets"))
        val tasks = parseTasks(obj.optJSONArray("tasks"))
        val todos = parseTodos(obj.optJSONArray("todos"))
        val notes = parseNotes(obj.optJSONArray("notes"))
        val modelOptions = parseModelOptions(obj.optJSONArray("model_options"))
        val revisions = parseRevisions(obj.optJSONObject("revisions"))

        return DashboardSnapshot(
            version = obj.optString("version", "v2.0.0"),
            totalAssets = stats.optInt("total_assets", assets.size),
            expiringCount = assets.count { it.isExpiringSoon },
            activeTasks = tasks.count { it.enabled },
            unreadNotifications = stats.optInt("unread_notifications", 0),
            assets = assets,
            tasks = tasks,
            todos = todos,
            notes = notes,
            assetCategories = assetCategories,
            fxRates = fxRates,
            modelOptions = modelOptions,
            revisions = revisions
        )
    }

    fun serializeDashboard(snap: DashboardSnapshot): String {
        val assetsArr = JSONArray()
        snap.assets.forEach { a ->
            assetsArr.put(
                JSONObject()
                    .put("id", a.id)
                    .put("name", a.name)
                    .put("category", a.category)
                    .put("spec", a.spec)
                    .put("price", a.price)
                    .put("due_date", a.dueDate)
                    .put("status", a.status)
                    .put("days_left", a.daysLeft)
                    .put("auto_renew", a.autoRenew)
                    .put("warn_days", a.warnDays)
                    .put("extra_config", a.extraConfig)
            )
        }
        val tasksArr = JSONArray()
        snap.tasks.forEach { t ->
            tasksArr.put(
                JSONObject()
                    .put("id", t.id)
                    .put("name", t.name)
                    .put("type", t.type)
                    .put("cron_expr", t.cronExpr)
                    .put("enabled", t.enabled)
                    .put("next_run_at", t.nextRunAt)
                    .put("model", t.model)
                    .put("bot_id", t.botId)
                    .put("description", t.description)
                    .put("run_at", t.runAt)
                    .put("params", t.params)
                    .put("last_run", JSONObject().put("summary", t.lastSummary))
            )
        }
        val todosArr = JSONArray()
        snap.todos.forEach { td ->
            todosArr.put(
                JSONObject()
                    .put("id", td.id)
                    .put("title", td.title)
                    .put("notes", td.notes)
                    .put("status", td.status)
                    .put("priority", td.priority)
                    .put("due_at", td.dueAt)
                    .put("bot_id", td.botId)
                    .put("updated_at", td.updatedAt)
            )
        }
        val notesArr = JSONArray()
        snap.notes.forEach { nt ->
            notesArr.put(
                JSONObject()
                    .put("id", nt.id)
                    .put("title", nt.title)
                    .put("content", nt.content)
                    .put("category", nt.category)
                    .put("pinned", nt.pinned)
                    .put("bot_id", nt.botId)
                    .put("updated_at", nt.updatedAt)
            )
        }
        val catArr = JSONArray()
        snap.assetCategories.forEach { c ->
            catArr.put(
                JSONObject()
                    .put("id", c.id)
                    .put("label", c.label)
                    .put("icon", c.icon)
            )
        }
        val modelOptArr = JSONArray()
        snap.modelOptions.forEach { m ->
            modelOptArr.put(
                JSONObject()
                    .put("id", m.id)
                    .put("label", m.label)
                    .put("is_auto", m.isAuto)
                    .put("is_light", m.isLight)
                    .put("provider", m.provider)
                    .put("model", m.model)
                    .put("available", m.available)
            )
        }
        val fxRatesObj = JSONObject()
        snap.fxRates.forEach { (k, v) ->
            if (k.isNotBlank() && v > 0.0) {
                fxRatesObj.put(k, v)
            }
        }
        return JSONObject()
            .put("version", snap.version)
            .put("assets", assetsArr)
            .put("asset_categories", catArr)
            .put("fx_rates", JSONObject().put("rates", fxRatesObj))
            .put("model_options", modelOptArr)
            .put("tasks", tasksArr)
            .put("todos", todosArr)
            .put("notes", notesArr)
            .put(
                "stats",
                JSONObject()
                    .put("total_assets", snap.totalAssets)
                    .put("unread_notifications", snap.unreadNotifications)
            )
            .toString()
    }

    fun deserializeDashboard(rawJson: String): DashboardSnapshot? {
        if (rawJson.isBlank()) return null
        return runCatching { parseDashboard(JSONObject(rawJson)) }.getOrNull()
    }

    fun serializeChatTurns(items: List<ChatTurnUi>, maxItems: Int = 50): String {
        val arr = JSONArray()
        items.takeLast(maxItems).forEach { t ->
            val toolsArr = JSONArray()
            t.tools.forEach { toolsArr.put(it) }
            val tracesArr = JSONArray()
            t.toolTraces.forEach { tr ->
                tracesArr.put(
                    JSONObject()
                        .put("id", tr.id)
                        .put("name", tr.name)
                        .put("display_name", tr.displayName)
                        .put("arguments", tr.arguments)
                        .put("result", tr.result)
                        .put("status", tr.status)
                )
            }
            arr.put(
                JSONObject()
                    .put("id", t.id)
                    .put("role", t.role)
                    .put("persona", t.persona)
                    .put("model", t.model)
                    .put("reasoning", t.reasoning)
                    .put("tools", toolsArr)
                    .put("tool_traces", tracesArr)
                    .put("content", t.content)
                    .put("attachments", JSONArray().apply { t.attachments.forEach { put(it) } })
                    .put("msg_kind", t.msgKind)
                    .put("sender_bot_id", t.senderBotId)
                    .put("avatar_style", t.avatarStyle)
            )
        }
        return arr.toString()
    }

    fun deserializeChatTurns(rawJson: String): List<ChatTurnUi> {
        if (rawJson.isBlank()) return emptyList()
        return runCatching {
            val arr = JSONArray(rawJson)
            buildList {
                for (i in 0 until arr.length()) {
                    val o = arr.optJSONObject(i) ?: continue
                    val traces = buildList {
                        val tracesArr = o.optJSONArray("tool_traces")
                        if (tracesArr != null) {
                            for (j in 0 until tracesArr.length()) {
                                val tr = tracesArr.optJSONObject(j) ?: continue
                                val name = tr.optString("name", "").ifBlank { "tool" }
                                val displayName = tr.optString("display_name", "")
                                    .ifBlank { formatHumanToolName(name) }
                                add(
                                    ChatToolTraceUi(
                                        id = tr.optString("id", "tool_$j"),
                                        name = name,
                                        displayName = displayName,
                                        arguments = tr.optString("arguments", ""),
                                        result = tr.optString("result", ""),
                                        status = tr.optString("status", "已完成")
                                    )
                                )
                            }
                        }
                    }
                    add(
                        ChatTurnUi(
                            id = o.optString("id"),
                            role = o.optString("role", "assistant"),
                            persona = o.optString("persona", ""),
                            model = o.optString("model", "deepseek-flash"),
                            reasoning = o.optString("reasoning", ""),
                            tools = optStringList(o, "tools"),
                            toolTraces = traces,
                            content = o.optString("content", ""),
                            attachments = optStringList(o, "attachments"),
                            msgKind = o.optString("msg_kind", "chat"),
                            senderBotId = o.optString("sender_bot_id", ""),
                            avatarStyle = o.optString("avatar_style", "butler:gold")
                        )
                    )
                }
            }
        }.getOrDefault(emptyList())
    }

    fun serializeDrafts(drafts: Map<String, String>): String {
        val obj = JSONObject()
        drafts.forEach { (k, v) ->
            if (k.isNotBlank() && v.isNotBlank()) {
                obj.put(k, v)
            }
        }
        return obj.toString()
    }

    fun deserializeDrafts(rawJson: String): Map<String, String> {
        if (rawJson.isBlank()) return emptyMap()
        return runCatching {
            val obj = JSONObject(rawJson)
            buildMap {
                val keys = obj.keys()
                while (keys.hasNext()) {
                    val k = keys.next()
                    val v = obj.optString(k, "")
                    if (k.isNotBlank() && v.isNotBlank()) {
                        put(k, v)
                    }
                }
            }
        }.getOrDefault(emptyMap())
    }
}
