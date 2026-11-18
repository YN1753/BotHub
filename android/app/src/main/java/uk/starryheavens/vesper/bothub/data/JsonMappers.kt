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
        return BotItem(
            id = b.optString("id"),
            name = b.optString("name"),
            roleTitle = b.optString("role_title"),
            avatarStyle = b.optString("avatar_style", "butler:gold"),
            model = b.optString("model", "deepseek-flash"),
            enableThinking = b.optBoolean("enable_thinking", true),
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
            updatedAt = b.optString("updated_at", "")
        )
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
                    .put("enable_thinking", b.enableThinking)
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

    fun parseHistoryMessages(arr: JSONArray?): List<AgentHistoryMessage> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val m = arr.optJSONObject(i) ?: continue
                val role = m.optString("role")
                val msgKind = m.optString("msg_kind", "chat")
                if (role == "user" || role == "assistant" || (role == "system" && msgKind == "system")) {
                    add(
                        AgentHistoryMessage(
                            id = m.optLong("id"),
                            role = role,
                            content = m.optString("content", ""),
                            reasoningContent = m.optString("reasoning_content", ""),
                            msgKind = msgKind,
                            senderBotId = m.optString("sender_bot_id", "")
                        )
                    )
                }
            }
        }
    }

    fun parseAgentSessions(arr: JSONArray?): List<AgentSessionSummary> {
        if (arr == null) return emptyList()
        return buildList {
            for (i in 0 until arr.length()) {
                val s = arr.optJSONObject(i) ?: continue
                add(
                    AgentSessionSummary(
                        id = s.optString("id"),
                        title = s.optString("title", "未命名会话"),
                        persona = s.optString("persona", "grok"),
                        model = s.optString("model", "deepseek-flash"),
                        updatedAt = s.optString("updated_at", "")
                    )
                )
            }
        }
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

    fun parseDashboard(obj: JSONObject): DashboardSnapshot {
        val stats = obj.optJSONObject("stats") ?: JSONObject()
        val assetsArr = obj.optJSONArray("assets") ?: JSONArray()
        val tasksArr = obj.optJSONArray("tasks") ?: JSONArray()
        val assetCategories = parseAssetCategories(obj.optJSONArray("asset_categories"))

        val assets = buildList {
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
                        status = a.optString("status"),
                        daysLeft = a.optInt("days_left", 999),
                        autoRenew = a.optBoolean("auto_renew", false)
                    )
                )
            }
        }

        val tasks = buildList {
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
                        lastSummary = lastRun?.optString("summary")?.ifEmpty { "等待执行" } ?: "等待执行"
                    )
                )
            }
        }

        return DashboardSnapshot(
            version = obj.optString("version", "v2.0.0"),
            totalAssets = stats.optInt("total_assets", assets.size),
            expiringCount = assets.count { it.isExpiringSoon },
            activeTasks = tasks.count { it.enabled },
            unreadNotifications = stats.optInt("unread_notifications", 0),
            assets = assets,
            tasks = tasks,
            assetCategories = assetCategories
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
                    .put("last_run", JSONObject().put("summary", t.lastSummary))
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
        return JSONObject()
            .put("version", snap.version)
            .put("assets", assetsArr)
            .put("asset_categories", catArr)
            .put("tasks", tasksArr)
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
            arr.put(
                JSONObject()
                    .put("id", t.id)
                    .put("role", t.role)
                    .put("persona", t.persona)
                    .put("model", t.model)
                    .put("reasoning", t.reasoning)
                    .put("tools", toolsArr)
                    .put("content", t.content)
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
                    add(
                        ChatTurnUi(
                            id = o.optString("id"),
                            role = o.optString("role", "assistant"),
                            persona = o.optString("persona", ""),
                            model = o.optString("model", "deepseek-flash"),
                            reasoning = o.optString("reasoning", ""),
                            tools = optStringList(o, "tools"),
                            content = o.optString("content", ""),
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
