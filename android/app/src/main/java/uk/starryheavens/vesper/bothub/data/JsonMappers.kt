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

    fun parseDashboard(obj: JSONObject): DashboardSnapshot {
        val stats = obj.optJSONObject("stats") ?: JSONObject()
        val assetsArr = obj.optJSONArray("assets") ?: JSONArray()
        val tasksArr = obj.optJSONArray("tasks") ?: JSONArray()

        val assets = buildList {
            for (i in 0 until assetsArr.length()) {
                val a = assetsArr.optJSONObject(i) ?: continue
                add(
                    AssetItem(
                        id = a.optString("id"),
                        name = a.optString("name"),
                        category = a.optString("category", "compute"),
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
            tasks = tasks
        )
    }
}
