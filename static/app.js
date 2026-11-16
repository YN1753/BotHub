/**
 * Vesper BotHub · Frontend Architecture
 * Micro-Component System (EventBus + Reactive AppStore + High-Cohesion Components)
 * Includes Custom Twilight Select Component (STRICT NO SYSTEM DROPDOWNS)
 * STRICT NO EMOJI · Single Binary Embedded
 */
(() => {
  "use strict";

  // ==========================================
  // 1. Utils & Formatting
  // ==========================================
  const Utils = {
    esc(s) {
      return String(s ?? "")
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;");
    },

    renderMarkdown(raw) {
      const src = String(raw || "").replace(/\r\n/g, "\n");
      if (!src.trim()) return "";

      // Extract fenced code blocks first to keep them untouched
      const codeBlocks = [];
      const withPlaceholders = src.replace(/```([a-zA-Z0-9_-]*)\n([\s\S]*?)```/g, (_, lang, code) => {
        const idx = codeBlocks.length;
        const safeLang = Utils.esc(lang || "text");
        const safeCode = Utils.esc(code.replace(/\n$/, ""));
        codeBlocks.push(
          `<div class="md-code-wrap"><div class="md-code-head"><span class="mono">${safeLang}</span><button type="button" class="agent-msg-action-btn md-copy-code-btn">复制代码</button></div><pre><code>${safeCode}</code></pre></div>`
        );
        return `\n@@CODEBLOCK_${idx}@@\n`;
      });

      const formatInline = (text) => {
        let out = Utils.esc(text);
        out = out.replace(/`([^`\n]+)`/g, '<code class="md-inline-code">$1</code>');
        out = out.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
        out = out.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
        return out;
      };

      const lines = withPlaceholders.split("\n");
      const html = [];
      let inList = false;
      let listType = "";

      const closeList = () => {
        if (inList) {
          html.push(listType === "ol" ? "</ol>" : "</ul>");
          inList = false;
          listType = "";
        }
      };

      const isTableRow = (l) => /^\|(.+)\|$/.test(l.trim());
      const isTableSep = (l) => /^\|[\s:|-]+\|$/.test(l.trim());

      for (let i = 0; i < lines.length; i++) {
        const trimmed = lines[i].trim();
        const cbMatch = trimmed.match(/^@@CODEBLOCK_(\d+)@@$/);
        if (cbMatch) {
          closeList();
          html.push(codeBlocks[Number(cbMatch[1])] || "");
          continue;
        }
        if (!trimmed) {
          closeList();
          continue;
        }
        if (isTableRow(trimmed) && i + 1 < lines.length && isTableSep(lines[i + 1])) {
          closeList();
          const splitCells = (row) => row.trim().slice(1, -1).split("|").map((c) => formatInline(c.trim()));
          const headers = splitCells(trimmed);
          i += 2;
          const rows = [];
          while (i < lines.length && isTableRow(lines[i])) {
            rows.push(splitCells(lines[i]));
            i++;
          }
          i--;
          const thead = `<thead><tr>${headers.map((h) => `<th>${h}</th>`).join("")}</tr></thead>`;
          const tbody = `<tbody>${rows.map((r) => `<tr>${r.map((c) => `<td>${c}</td>`).join("")}</tr>`).join("")}</tbody>`;
          html.push(`<table>${thead}${tbody}</table>`);
          continue;
        }
        if (/^---+$/.test(trimmed)) {
          closeList();
          html.push('<div class="divider"></div>');
          continue;
        }
        if (trimmed.startsWith("> ")) {
          closeList();
          html.push(`<blockquote>${formatInline(trimmed.slice(2))}</blockquote>`);
          continue;
        }
        const hMatch = trimmed.match(/^(#{1,4})\s+(.+)$/);
        if (hMatch) {
          closeList();
          const lvl = hMatch[1].length;
          html.push(`<h${lvl}>${formatInline(hMatch[2])}</h${lvl}>`);
          continue;
        }
        const ulMatch = trimmed.match(/^[-*]\s+(.+)$/);
        if (ulMatch) {
          if (!inList || listType !== "ul") {
            closeList();
            html.push("<ul>");
            inList = true;
            listType = "ul";
          }
          html.push(`<li>${formatInline(ulMatch[1])}</li>`);
          continue;
        }
        const olMatch = trimmed.match(/^\d+\.\s+(.+)$/);
        if (olMatch) {
          if (!inList || listType !== "ol") {
            closeList();
            html.push("<ol>");
            inList = true;
            listType = "ol";
          }
          html.push(`<li>${formatInline(olMatch[1])}</li>`);
          continue;
        }
        closeList();
        html.push(`<p>${formatInline(trimmed)}</p>`);
      }
      closeList();
      return `<div class="md-body">${html.join("")}</div>`;
    },

    formatTime(str) {
      if (!str) return "--";
      const d = new Date(str);
      if (isNaN(d.getTime())) return str;
      const pad = (n) => String(n).padStart(2, "0");
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
    },

    formatDateOnly(str) {
      if (!str || String(str).startsWith("2099")) return "长期有效";
      const s = String(str).trim();
      const m = s.match(/^(\d{4}-\d{2}-\d{2})/);
      if (m) return m[1];
      const d = new Date(s);
      if (isNaN(d.getTime())) return s;
      const pad = (n) => String(n).padStart(2, "0");
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
    },

    generateSlug(text, defaultPrefix = "item") {
      const clean = String(text || "")
        .trim()
        .toLowerCase()
        .replace(/[^a-z0-9_-]+/g, "_")
        .replace(/^_+|_+$/g, "");
      if (clean) return clean;
      return `${defaultPrefix}_${Date.now().toString(36).slice(-4)}`;
    },

    describeCron(expr) {
      const trimmed = String(expr || "").trim();
      if (!trimmed) return "未指定调度策略";
      const parts = trimmed.split(/\s+/);
      if (parts.length !== 5) return `表达式: ${trimmed}`;

      const [min, hour, dom, mon, dow] = parts;
      const pad = (n) => String(n).padStart(2, "0");

      if (min === "0" && hour === "*" && dom === "*" && mon === "*" && dow === "*") {
        return "每小时整点执行一次";
      }
      if (min.startsWith("*/") && hour === "*" && dom === "*" && mon === "*" && dow === "*") {
        return `每隔 ${min.slice(2)} 分钟轮询一次`;
      }
      if (min === "0" && hour.startsWith("*/") && dom === "*" && mon === "*" && dow === "*") {
        return `每隔 ${hour.slice(2)} 小时整点执行`;
      }
      if (!isNaN(min) && !isNaN(hour) && dom === "*" && mon === "*" && dow === "*") {
        const h = parseInt(hour, 10);
        const period = h < 6 ? "凌晨" : (h < 12 ? "早间" : (h < 14 ? "午间" : (h < 18 ? "下午" : "晚间")));
        return `每天 ${period} ${pad(hour)}:${pad(min)} 执行`;
      }
      if (!isNaN(min) && !isNaN(hour) && dom === "*" && mon === "*" && (dow === "1-5" || dow === "1,2,3,4,5")) {
        return `工作日 (周一至五) ${pad(hour)}:${pad(min)} 执行`;
      }
      if (!isNaN(min) && !isNaN(hour) && dom === "*" && mon === "*" && (dow === "6,0" || dow === "0,6" || dow === "6,7")) {
        return `周末 (周六日) ${pad(hour)}:${pad(min)} 执行`;
      }
      const weekMap = { "0": "周日", "1": "周一", "2": "周二", "3": "周三", "4": "周四", "5": "周五", "6": "周六", "7": "周日" };
      if (!isNaN(min) && !isNaN(hour) && dom === "*" && mon === "*" && weekMap[dow]) {
        return `每${weekMap[dow]} ${pad(hour)}:${pad(min)} 执行`;
      }
      if (!isNaN(min) && !isNaN(hour) && !isNaN(dom) && mon === "*" && dow === "*") {
        return `每月 ${pad(dom)} 日 ${pad(hour)}:${pad(min)} 执行`;
      }
      if (!isNaN(min) && hour.includes(",") && dom === "*" && mon === "*" && dow === "*") {
        return `每天 ${hour.split(",").map((h) => `${pad(h)}:${pad(min)}`).join(" 与 ")} 各执行一次`;
      }
      return `定时 Cron: ${trimmed}`;
    },

    safeJSONParse(str, fallback = {}) {
      try {
        return JSON.parse(str || "{}");
      } catch {
        return fallback;
      }
    },

    parseOptionItem(opt) {
      const s = String(opt || "");
      const idx = s.indexOf("|");
      if (idx > 0) {
        return { value: s.slice(0, idx), label: s.slice(idx + 1) };
      }
      return { value: s, label: s };
    },

    normalizeWarnDays(val) {
      try {
        const arr = JSON.parse(val || "[7, 3, 1]");
        if (Array.isArray(arr)) {
          return `[${arr.join(", ")}]`;
        }
      } catch {
        // ignore
      }
      return String(val || "[7, 3, 1]").trim();
    },
  };

  // ==========================================
  // 2. Custom Select Component (Vesper Twilight Theme)
  // ==========================================
  class CustomSelect {
    constructor(mountEl, { options, value, onChange, placeholder = "请选择..." }) {
      this.mount = typeof mountEl === "string" ? document.querySelector(mountEl) : mountEl;
      this.options = options || [];
      this.value = value !== undefined ? String(value) : (this.options[0]?.value ?? "");
      this.onChange = onChange || (() => {});
      this.placeholder = placeholder;
      this.isOpen = false;
      this.root = null;
      this.labelSpan = null;

      this.handleDocClick = (e) => {
        if (this.root && !this.root.contains(e.target)) {
          this.close();
        }
      };
      document.addEventListener("click", this.handleDocClick);

      this.render();
    }

    destroy() {
      document.removeEventListener("click", this.handleDocClick);
      if (this.root && this.root.parentNode) {
        this.root.parentNode.removeChild(this.root);
      }
    }

    setOptions(options, newValue) {
      this.options = options || [];
      if (newValue !== undefined) {
        this.value = String(newValue);
      } else if (!this.options.some((o) => String(o.value) === String(this.value))) {
        this.value = this.options[0]?.value ?? "";
      }
      this.render();
    }

    setValue(val, triggerChange = false) {
      this.value = String(val);
      this.updateUI();
      if (triggerChange) {
        this.onChange(this.value);
      }
    }

    getValue() {
      return this.value;
    }

    toggle() {
      if (this.isOpen) {
        this.close();
      } else {
        this.open();
      }
    }

    open() {
      document.querySelectorAll(".v-select.is-open").forEach((el) => {
        if (el !== this.root) el.classList.remove("is-open");
      });
      this.isOpen = true;
      this.root?.classList.add("is-open");
    }

    close() {
      this.isOpen = false;
      this.root?.classList.remove("is-open");
    }

    select(val) {
      this.value = String(val);
      this.updateUI();
      this.close();
      this.onChange(this.value);
    }

    updateUI() {
      if (!this.root) return;
      const currentOpt = this.options.find((o) => String(o.value) === String(this.value));
      const labelText = currentOpt ? currentOpt.label : this.placeholder;
      if (this.labelSpan) this.labelSpan.textContent = labelText;

      this.root.querySelectorAll(".v-select-option").forEach((optEl) => {
        if (optEl.dataset.value === String(this.value)) {
          optEl.classList.add("is-selected");
        } else {
          optEl.classList.remove("is-selected");
        }
      });
    }

    render() {
      if (!this.mount) return;
      this.mount.innerHTML = "";

      const wrapper = document.createElement("div");
      wrapper.className = "v-select";
      this.root = wrapper;

      const currentOpt = this.options.find((o) => String(o.value) === String(this.value));
      const labelText = currentOpt ? currentOpt.label : this.placeholder;

      wrapper.innerHTML = `
        <div class="v-select-trigger" tabindex="0" role="button" aria-haspopup="listbox">
          <span class="v-select-label">${Utils.esc(labelText)}</span>
          <span class="v-select-arrow" aria-hidden="true"></span>
        </div>
        <div class="v-select-dropdown" role="listbox">
          ${this.options.map((opt) => `
            <div class="v-select-option ${String(opt.value) === String(this.value) ? "is-selected" : ""}" data-value="${Utils.esc(opt.value)}" role="option">
              <span>${Utils.esc(opt.label)}</span>
            </div>
          `).join("")}
        </div>
      `;

      this.labelSpan = wrapper.querySelector(".v-select-label");
      const trigger = wrapper.querySelector(".v-select-trigger");

      trigger.addEventListener("click", (e) => {
        e.stopPropagation();
        this.toggle();
      });

      trigger.addEventListener("keydown", (e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          this.toggle();
        } else if (e.key === "Escape") {
          this.close();
        }
      });

      wrapper.querySelectorAll(".v-select-option").forEach((optEl) => {
        optEl.addEventListener("click", (e) => {
          e.stopPropagation();
          this.select(optEl.dataset.value);
        });
      });

      this.mount.appendChild(wrapper);
    }
  }

  // ==========================================
  // 3. EventBus (Decoupled Pub/Sub)
  // ==========================================
  class EventBus {
    constructor() {
      this.listeners = new Map();
    }

    on(event, handler) {
      if (!this.listeners.has(event)) {
        this.listeners.set(event, new Set());
      }
      this.listeners.get(event).add(handler);
      return () => this.off(event, handler);
    }

    off(event, handler) {
      if (this.listeners.has(event)) {
        this.listeners.get(event).delete(handler);
      }
    }

    emit(event, payload) {
      if (this.listeners.has(event)) {
        this.listeners.get(event).forEach((handler) => {
          try {
            handler(payload);
          } catch (e) {
            console.error(`[EventBus Error on "${event}"]:`, e);
          }
        });
      }
    }
  }

  // ==========================================
  // 4. ApiClient
  // ==========================================
  class ApiClient {
    constructor(bus) {
      this.bus = bus;
    }

    async request(path, opts = {}) {
      const res = await fetch(path, {
        credentials: "same-origin",
        headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
        ...opts,
      });

      const data = await res.json().catch(() => ({}));
      if (res.status === 401) {
        this.bus.emit("auth:state", { authenticated: false });
        throw new Error(data.error || "未登录或会话已过期");
      }
      if (!res.ok) {
        throw new Error(data.error || `请求失败 (${res.status})`);
      }
      return data;
    }
  }

  // ==========================================
  // 5. AppStore (Reactive Central State)
  // ==========================================
  class AppStore {
    constructor(bus) {
      this.bus = bus;
      this.api = new ApiClient(bus);
      this.state = {
        authenticated: false,
        version: "v1.6.0",
        commit: "dev",
        buildTime: "local",
        uptimeSec: 0,
        serverNow: "--",
        stats: { total_assets: 0, sent_pushes: 0, seen_items: 0 },
        assets: [],
        tasks: [],
        templates: [],
        assetTemplates: [],
        personas: [],
        agentSessions: [],
        agentMemories: [],
        notifications: [],
        unreadCount: 0,
        configs: {},
        pushLogs: [],
      };
      this.sseSource = null;
    }

    async checkAuth() {
      try {
        const data = await this.api.request("/api/auth/status");
        this.state.authenticated = !!data.authenticated;
        this.bus.emit("auth:state", { authenticated: this.state.authenticated });
        if (this.state.authenticated) {
          await this.loadDashboard();
          this.startNotificationStream();
        }
      } catch {
        this.state.authenticated = false;
        this.bus.emit("auth:state", { authenticated: false });
      }
    }

    async login(password) {
      const data = await this.api.request("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ password }),
      });
      this.state.authenticated = true;
      this.bus.emit("auth:state", { authenticated: true });
      await this.loadDashboard();
      this.startNotificationStream();
      return data;
    }

    async logout() {
      this.stopNotificationStream();
      await this.api.request("/api/auth/logout", { method: "POST" }).catch(() => {});
      this.state.authenticated = false;
      this.bus.emit("auth:state", { authenticated: false });
    }

    async loadDashboard() {
      try {
        const data = await this.api.request("/api/dashboard");
        this.state.version = data.version || this.state.version;
        this.state.commit = data.commit || this.state.commit;
        this.state.buildTime = data.build_time || this.state.buildTime;
        this.state.uptimeSec = Number(data.uptime_sec) || 0;
        this.state.serverNow = data.server_now || "--";
        this.state.stats = data.stats || this.state.stats;
        this.state.assets = data.assets || [];
        this.state.tasks = data.tasks || data.modules || [];
        this.state.templates = data.templates || [];
        this.state.assetTemplates = data.asset_templates || [];
        this.state.personas = data.personas || [];
        this.state.agentSessions = data.agent_sessions || [];
        this.state.agentMemories = data.agent_memories || [];
        this.state.notifications = data.notifications || [];
        this.state.unreadCount = Number(data.stats?.unread_notifications) || this.state.notifications.filter((n) => !n.read_at).length;
        this.state.configs = data.configs || {};
        this.state.pushLogs = data.push_logs || [];

        this.bus.emit("data:loaded", this.state);
      } catch (err) {
        this.bus.emit("toast", err.message);
      }
    }

    startNotificationStream() {
      if (this.sseSource || typeof EventSource === "undefined") return;
      const latestId = this.state.notifications.length > 0 ? this.state.notifications[0].id : 0;
      const url = `/api/v2/events?last_event_id=${encodeURIComponent(latestId)}`;
      const es = new EventSource(url);
      this.sseSource = es;

      es.addEventListener("notification", (ev) => {
        try {
          const item = JSON.parse(ev.data);
          if (!item || !item.id) return;
          const exists = this.state.notifications.some((n) => n.id === item.id);
          if (!exists) {
            this.state.notifications = [item, ...this.state.notifications].slice(0, 60);
            if (!item.read_at) {
              this.state.unreadCount += 1;
            }
            this.bus.emit("notifications:updated", this.state);
            this.bus.emit("toast", `新情报到达: ${item.title}`);
            if (
              localStorage.getItem("bothub_desktop_notify") === "1" &&
              typeof Notification !== "undefined" &&
              Notification.permission === "granted" &&
              (document.hidden || item.priority === "high")
            ) {
              try {
                new Notification(`Vesper BotHub · ${item.title}`, {
                  body: item.summary || "收到新的情报推送",
                });
              } catch {
                // ignore browser notification errors
              }
            }
          }
        } catch {
          // ignore malformed SSE frame
        }
      });

      es.onerror = () => {
        // EventSource automatically reconnects; close only if logged out
        if (!this.state.authenticated) {
          this.stopNotificationStream();
        }
      };
    }

    stopNotificationStream() {
      if (this.sseSource) {
        this.sseSource.close();
        this.sseSource = null;
      }
    }

    async markNotificationRead(id) {
      const target = this.state.notifications.find((n) => n.id === id);
      if (target && target.read_at) return;
      await this.api.request(`/api/v2/notifications/${encodeURIComponent(id)}/read`, { method: "POST" });
      if (target) target.read_at = new Date().toISOString();
      this.state.unreadCount = this.state.notifications.filter((n) => !n.read_at).length;
      this.bus.emit("notifications:updated", this.state);
    }

    async markAllNotificationsRead() {
      await this.api.request("/api/v2/notifications/read-all", { method: "POST" });
      const now = new Date().toISOString();
      this.state.notifications.forEach((n) => {
        if (!n.read_at) n.read_at = now;
      });
      this.state.unreadCount = 0;
      this.bus.emit("notifications:updated", this.state);
      this.bus.emit("toast", "全部情报已标记为已读");
    }

    async rotateAppToken() {
      const res = await this.api.request("/api/v2/auth/token/rotate", { method: "POST" });
      this.bus.emit("toast", res.message || "已生成全新 App Bearer Token");
      await this.loadDashboard();
      return res;
    }

    async saveAsset(payload, isNew) {
      if (isNew) {
        await this.api.request("/api/assets", { method: "POST", body: JSON.stringify(payload) });
        this.bus.emit("toast", "新资产创建成功");
      } else {
        await this.api.request(`/api/assets/${encodeURIComponent(payload.id)}`, { method: "POST", body: JSON.stringify(payload) });
        this.bus.emit("toast", "资产已更新");
      }
      await this.loadDashboard();
    }

    async deleteAsset(id) {
      await this.api.request(`/api/assets/${encodeURIComponent(id)}`, { method: "DELETE" });
      this.bus.emit("toast", "资产已成功下架删除");
      await this.loadDashboard();
    }

    async saveTask(payload, isNew) {
      if (isNew) {
        await this.api.request("/api/tasks", { method: "POST", body: JSON.stringify(payload) });
        this.bus.emit("toast", "新情报任务创建成功");
      } else {
        await this.api.request(`/api/tasks/${encodeURIComponent(payload.id)}`, { method: "POST", body: JSON.stringify(payload) });
        this.bus.emit("toast", "任务配置已更新并生效");
      }
      await this.loadDashboard();
    }

    async deleteTask(id) {
      await this.api.request(`/api/tasks/${encodeURIComponent(id)}`, { method: "DELETE" });
      this.bus.emit("toast", "任务已注销删除");
      await this.loadDashboard();
    }

    async runTask(id) {
      const res = await this.api.request(`/api/tasks/${encodeURIComponent(id)}/run`, { method: "POST" });
      this.bus.emit("toast", res.summary || "执行完成");
      await this.loadDashboard();
    }

    async saveConfig(payload) {
      await this.api.request("/api/config", { method: "POST", body: JSON.stringify(payload) });
      this.bus.emit("toast", "全局系统凭据已安全保存");
      await this.loadDashboard();
    }

    async updatePassword(newPassword) {
      await this.api.request("/api/auth/password", { method: "POST", body: JSON.stringify({ new_password: newPassword }) });
      this.bus.emit("toast", "控制台访问密钥修改成功");
    }

    async testPush() {
      const res = await this.api.request("/api/test-push", { method: "POST" });
      this.bus.emit("toast", res.message || "测试消息已推送");
      await this.loadDashboard();
    }

    async syncAssets() {
      const res = await this.api.request("/api/modules/asset_watcher/run", { method: "POST" });
      this.bus.emit("toast", res.summary || "资产嗅探同步完成");
      await this.loadDashboard();
    }

    async exportBackup() {
      try {
        const res = await fetch("/api/system/export", {
          headers: { "X-Admin-Token": this.api.token },
        });
        if (!res.ok) throw new Error("导出失败");
        const blob = await res.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        const now = new Date();
        const pad = (n) => String(n).padStart(2, "0");
        const dateStr = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}_${pad(now.getHours())}${pad(now.getMinutes())}`;
        a.download = `bothub_backup_${dateStr}.json`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        window.URL.revokeObjectURL(url);
        this.bus.emit("toast", "备份文件已成功导出下载");
      } catch (err) {
        this.bus.emit("toast", "导出备份异常: " + err.message);
      }
    }

    async importBackup(backupObj) {
      const res = await this.api.request("/api/system/import", {
        method: "POST",
        body: JSON.stringify(backupObj),
      });
      this.bus.emit("toast", res.message || "备份已成功恢复");
      await this.loadDashboard();
    }

    async loadAgentSession(id) {
      return await this.api.request(`/api/agent/sessions/${encodeURIComponent(id)}`);
    }

    async deleteAgentSession(id) {
      await this.api.request(`/api/agent/sessions/${encodeURIComponent(id)}`, { method: "DELETE" });
      this.bus.emit("toast", "对话会话已清空");
      await this.loadDashboard();
    }

    async saveAgentMemory(payload) {
      await this.api.request("/api/agent/memories", {
        method: "POST",
        body: JSON.stringify(payload),
      });
      this.bus.emit("toast", "长期记忆已写入 SQLite");
      await this.loadDashboard();
    }

    async deleteAgentMemory(id) {
      await this.api.request(`/api/agent/memories/${encodeURIComponent(id)}`, { method: "DELETE" });
      this.bus.emit("toast", "记忆条目已移除");
      await this.loadDashboard();
    }
  }

  // ==========================================
  // 6. Component Base Class
  // ==========================================
  class BaseComponent {
    constructor(rootEl, store, bus) {
      this.root = typeof rootEl === "string" ? document.querySelector(rootEl) : rootEl;
      this.store = store;
      this.bus = bus;
    }
    init() {}
  }

  // ==========================================
  // 7. UI Components Implementation
  // ==========================================

  // --- Toast Component ---
  class ToastComponent extends BaseComponent {
    init() {
      this.timer = null;
      this.bus.on("toast", (msg) => this.show(msg));
    }

    show(msg) {
      if (!this.root) return;
      this.root.textContent = msg;
      this.root.classList.remove("hidden");
      clearTimeout(this.timer);
      this.timer = setTimeout(() => this.root.classList.add("hidden"), 3200);
    }
  }

  // --- Auth View Component ---
  class AuthComponent extends BaseComponent {
    init() {
      this.loginView = document.getElementById("login-view");
      this.dashView = document.getElementById("dashboard-view");
      this.form = document.getElementById("login-form");
      this.passInput = document.getElementById("login-password");
      this.errText = document.getElementById("login-error");
      this.btn = document.getElementById("login-btn");

      this.bus.on("auth:state", ({ authenticated }) => {
        if (authenticated) {
          this.loginView?.classList.add("hidden");
          this.dashView?.classList.remove("hidden");
        } else {
          this.dashView?.classList.add("hidden");
          this.loginView?.classList.remove("hidden");
        }
      });

      this.form?.addEventListener("submit", async (e) => {
        e.preventDefault();
        this.errText?.classList.add("hidden");
        this.btn.disabled = true;
        this.btn.textContent = "验证中...";
        try {
          await this.store.login(this.passInput.value);
          this.passInput.value = "";
        } catch (err) {
          if (this.errText) {
            this.errText.textContent = err.message;
            this.errText.classList.remove("hidden");
          }
        } finally {
          this.btn.disabled = false;
          this.btn.textContent = "解锁控制台";
        }
      });
    }
  }

  // --- TopBar Component ---
  class TopbarComponent extends BaseComponent {
    init() {
      this.verLabel = document.getElementById("version-label");
      this.uptimeLabel = document.getElementById("uptime-label");
      this.testPushBtn = document.getElementById("test-push-btn");
      this.refreshBtn = document.getElementById("refresh-btn");
      this.logoutBtn = document.getElementById("logout-btn");

      this.bus.on("data:loaded", (state) => {
        if (this.verLabel) {
          const shortVer = String(state.version || "v2.2.0").replace(/-[0-9a-f]{12,40}$/i, "");
          this.verLabel.textContent = shortVer;
        }
        this.renderUptime(state.uptimeSec);
      });

      this.testPushBtn?.addEventListener("click", async () => {
        this.testPushBtn.disabled = true;
        try {
          await this.store.testPush();
        } finally {
          this.testPushBtn.disabled = false;
        }
      });

      this.refreshBtn?.addEventListener("click", () => this.store.loadDashboard());
      this.logoutBtn?.addEventListener("click", () => this.store.logout());
    }

    renderUptime(sec) {
      if (!this.uptimeLabel) return;
      const s = Number(sec) || 0;
      this.uptimeLabel.textContent = s < 60 ? `UPTIME ${s}s` : `UPTIME ${Math.floor(s / 60)}m`;
    }
  }

  // --- Metrics Component ---
  class MetricsComponent extends BaseComponent {
    init() {
      this.statAssets = document.getElementById("stat-assets");
      this.statExpiring = document.getElementById("stat-expiring");
      this.statTasks = document.getElementById("stat-tasks");
      this.statSent = document.getElementById("stat-sent");

      this.bus.on("data:loaded", (state) => {
        const assets = state.assets || [];
        const tasks = state.tasks || [];

        const expiringCount = assets.filter((a) => {
          if (a.status === "长期有效" || String(a.due_date || "").startsWith("2099")) return false;
          return typeof a.days_left === "number" && a.days_left >= 0 && a.days_left <= 7;
        }).length;

        const activeTasksCount = tasks.filter((t) => Boolean(t.enabled)).length;

        if (this.statAssets) this.statAssets.textContent = state.stats?.total_assets ?? assets.length;
        if (this.statExpiring) {
          this.statExpiring.textContent = expiringCount;
          this.statExpiring.className = `metric-value mono ${expiringCount > 0 ? "text-amber" : "text-cyan"}`;
        }
        if (this.statTasks) this.statTasks.textContent = activeTasksCount;
        if (this.statSent) this.statSent.textContent = state.stats?.sent_pushes ?? 0;
      });
    }
  }

  // --- Assets Section Component ---
  class AssetsSectionComponent extends BaseComponent {
    init() {
      this.grid = document.getElementById("assets-grid");
      this.addBtn = document.getElementById("add-asset-btn");
      this.syncBtn = document.getElementById("sync-assets-btn");
      this.searchInput = document.getElementById("asset-search-input");
      this.catPills = document.querySelectorAll("#asset-cat-pills .pill-btn");
      this.activeCat = "all";
      this.searchQuery = "";

      this.searchInput?.addEventListener("input", () => {
        this.searchQuery = (this.searchInput.value || "").trim().toLowerCase();
        this.render(this.store.state.assets);
      });

      this.catPills.forEach((btn) => {
        btn.addEventListener("click", () => {
          this.activeCat = btn.dataset.cat || "all";
          this.catPills.forEach((b) => b.classList.toggle("active", b === btn));
          this.render(this.store.state.assets);
        });
      });

      this.addBtn?.addEventListener("click", () => {
        this.bus.emit("modal:open-asset", null);
      });

      this.syncBtn?.addEventListener("click", async () => {
        this.syncBtn.disabled = true;
        this.syncBtn.textContent = "嗅探同步中...";
        try {
          await this.store.syncAssets();
        } finally {
          this.syncBtn.disabled = false;
          this.syncBtn.textContent = "立即嗅探同步";
        }
      });

      this.bus.on("data:loaded", (state) => this.render(state.assets));
    }

    render(assets) {
      if (!this.grid) return;
      const filtered = (assets || []).filter((a) => {
        if (this.activeCat !== "all" && a.category !== this.activeCat) return false;
        if (this.searchQuery) {
          const hay = `${a.name || ""} ${a.spec || ""} ${a.status || ""} ${a.extra_config || ""}`.toLowerCase();
          if (!hay.includes(this.searchQuery)) return false;
        }
        return true;
      });

      if (!filtered.length) {
        this.grid.innerHTML = `<div class="empty-state">${(assets || []).length ? "当前筛选条件下无匹配资产" : "暂无追踪中的资产"}</div>`;
        return;
      }

      this.grid.innerHTML = filtered.map((a) => {
        const catClass = a.category === "compute" ? "cat-compute" : (a.category === "domain" ? "cat-domain" : "cat-life");
        const catName = a.category === "compute" ? "算力基建" : (a.category === "domain" ? "核心域名" : "生活订阅");
        const isPermanent = a.status === "长期有效" || String(a.due_date || "").startsWith("2099");
        const daysClass = isPermanent ? "days-safe" : (a.days_left <= 3 ? "days-danger" : (a.days_left <= 7 ? "days-warn" : "days-safe"));
        const daysText = isPermanent ? "长期有效" : (a.days_left < 0 ? `逾期 ${Math.abs(a.days_left)} 天` : `剩余 ${a.days_left} 天`);
        const isTrial = (a.price && a.price.includes("活动")) || (a.status && a.status.includes("活动")) || a.id === "qq_music";

        return `
          <article class="glass-card asset-card">
            <div class="asset-head">
              <div>
                <h3 class="serif-title asset-name">${Utils.esc(a.name)}</h3>
                <p class="asset-spec mono">${Utils.esc(a.spec || a.category)}</p>
              </div>
              <span class="cat-badge ${catClass}">${Utils.esc(catName)}</span>
            </div>
            <div class="asset-bar">
              <span>扣费/到期倒计时</span>
              <span class="days-pill ${daysClass}">${Utils.esc(daysText)}</span>
            </div>
            <div class="asset-meta-grid mono">
              <div class="meta-box"><span>周期金额</span><strong>${Utils.esc(a.price || "--")}</strong></div>
              <div class="meta-box"><span>到期日期</span><strong>${Utils.esc(isPermanent ? "长期有效" : Utils.formatDateOnly(a.due_date))}</strong></div>
            </div>
            ${isTrial ? `<div class="trial-alert">[防免密扣款] 活动体验到期前请确认是否已取消自动扣款</div>` : ""}
            <div class="asset-foot">
              <span class="${a.auto_renew ? "renew-on" : ""}">自动续费: ${a.auto_renew ? "开启" : "关闭"}</span>
              <button class="btn btn-ghost btn-sm edit-asset-btn" data-id="${Utils.esc(a.id)}">配置/凭证</button>
            </div>
          </article>
        `;
      }).join("");

      this.grid.querySelectorAll(".edit-asset-btn").forEach((btn) => {
        btn.addEventListener("click", () => {
          const item = assets.find((x) => x.id === btn.dataset.id);
          if (item) this.bus.emit("modal:open-asset", item);
        });
      });
    }
  }

  // --- Tasks Section Component ---
  class TasksSectionComponent extends BaseComponent {
    init() {
      this.grid = document.getElementById("tasks-grid");
      this.addBtn = document.getElementById("add-task-btn");

      this.addBtn?.addEventListener("click", () => {
        this.bus.emit("modal:open-task", null);
      });

      this.bus.on("data:loaded", (state) => this.render(state.tasks));
    }

    render(tasks) {
      if (!this.grid) return;
      if (!tasks || !tasks.length) {
        this.grid.innerHTML = `<div class="empty-state">暂无已注册的任务模板与实例</div>`;
        return;
      }

      this.grid.innerHTML = tasks.map((t) => {
        const last = t.last_run;
        const dot = !t.enabled ? "dot-warn" : (t.is_running ? "dot-live" : (last && last.status === "error" ? "dot-err" : "dot-live"));
        const typeLabel = t.type === "morning" ? "早报小秘书" : (t.type === "radar" ? "开源技术雷达" : (t.type === "asset_watcher" ? "数字资产守望" : Utils.esc(t.type)));
        const typeClass = t.type === "morning" ? "cat-domain" : (t.type === "radar" ? "cat-compute" : "cat-life");

        let badgeHTML = "";
        const p = Utils.safeJSONParse(t.params);
        if (p.city) badgeHTML += `<span class="pill-btn mono">城市: ${Utils.esc(p.city)}</span>`;
        if (p.hn_min_score) badgeHTML += `<span class="pill-btn mono">HN分 >= ${Utils.esc(p.hn_min_score)}</span>`;
        if (p.limit) badgeHTML += `<span class="pill-btn mono">最多 ${Utils.esc(p.limit)} 条</span>`;

        return `
          <article class="glass-card module-card">
            <div class="module-top">
              <div>
                <div style="display:flex; align-items:center; gap:8px; margin-bottom:4px; flex-wrap:wrap;">
                  <h3 class="serif-title module-name">${Utils.esc(t.name)}</h3>
                  <span class="cat-badge ${typeClass}">${typeLabel}</span>
                </div>
                <p class="module-desc">${Utils.esc(t.description || "无详细描述")}</p>
              </div>
              <span class="stat-pill mono"><span class="dot ${dot}"></span>${t.enabled ? (t.is_running ? "RUN" : "ON") : "OFF"}</span>
            </div>
            <div class="module-cron-box mono">
              <div>
                <span class="text-sub">调度策略</span><br>
                <strong class="text-cyan">${Utils.esc(Utils.describeCron(t.cron_expr))}</strong>
              </div>
              <div>
                <span class="text-sub">下次触发 (CST)</span><br>
                <strong>${Utils.esc(t.enabled ? (t.next_run_at || "--") : "已暂停调度")}</strong>
              </div>
            </div>
            ${badgeHTML ? `<div class="preset-pill-group" style="margin-top:-4px;">${badgeHTML}</div>` : ""}
            <div class="module-summary">${Utils.esc(last ? last.summary : "等待执行流水")}</div>
            <div class="task-foot">
              <button class="btn btn-primary btn-sm run-task-btn" data-id="${Utils.esc(t.id)}" ${t.is_running ? "disabled" : ""}>
                ${t.is_running ? "执行中..." : "立即触发"}
              </button>
              <div class="task-actions-right">
                <button class="btn btn-ghost btn-sm edit-task-btn" data-id="${Utils.esc(t.id)}">配置/编辑</button>
                <button class="btn btn-danger btn-sm del-task-btn" data-id="${Utils.esc(t.id)}">删除</button>
              </div>
            </div>
          </article>
        `;
      }).join("");

      this.grid.querySelectorAll(".run-task-btn").forEach((btn) => {
        btn.addEventListener("click", async () => {
          btn.disabled = true;
          btn.textContent = "执行中...";
          try {
            await this.store.runTask(btn.dataset.id);
          } finally {
            btn.disabled = false;
          }
        });
      });

      this.grid.querySelectorAll(".edit-task-btn").forEach((btn) => {
        btn.addEventListener("click", () => {
          const item = tasks.find((x) => x.id === btn.dataset.id);
          if (item) this.bus.emit("modal:open-task", item);
        });
      });

      this.grid.querySelectorAll(".del-task-btn").forEach((btn) => {
        btn.addEventListener("click", async () => {
          const item = tasks.find((x) => x.id === btn.dataset.id);
          if (!item) return;
          if (!confirm(`确定删除任务 [${item.name || item.id}] 吗？`)) return;
          await this.store.deleteTask(item.id);
        });
      });
    }
  }

  // --- Global Config & Settings Modal Component ---
  class GlobalConfigComponent extends BaseComponent {
    init() {
      this.overlay = document.getElementById("settings-modal-overlay");
      this.openBtn = document.getElementById("open-settings-btn");
      this.closeBtn = document.getElementById("close-settings-modal-btn");

      this.form = document.getElementById("config-form");
      this.channelInput = document.getElementById("cfg-push-channel");
      this.channelMount = document.getElementById("mount-push-channel");
      this.wecomGroup = document.getElementById("wecom-fields-group");
      this.pushplusGroup = document.getElementById("pushplus-fields-group");

      this.wecomCorpInput = document.getElementById("cfg-wecom-corpid");
      this.wecomAgentInput = document.getElementById("cfg-wecom-agentid");
      this.wecomSecretInput = document.getElementById("cfg-wecom-secret");
      this.wecomToUserInput = document.getElementById("cfg-wecom-touser");
      this.wecomToUserMount = document.getElementById("mount-wecom-touser");
      this.pushplusInput = document.getElementById("cfg-pushplus-token");

      this.nezhaUrlInput = document.getElementById("cfg-nezha-url");
      this.nezhaDbPathInput = document.getElementById("cfg-nezha-db-path");
      this.appTokenInput = document.getElementById("cfg-app-bearer-token");
      this.cfInput = document.getElementById("cfg-cf-api-token");
      this.biliSessdataInput = document.getElementById("cfg-bili-sessdata");

      this.deepseekKeyInput = document.getElementById("cfg-deepseek-api-key");
      this.deepseekBaseInput = document.getElementById("cfg-deepseek-base-url");
      this.aiModelInput = document.getElementById("cfg-ai-model");
      this.aiModelMount = document.getElementById("mount-cfg-ai-model");
      this.activePersonaInput = document.getElementById("cfg-active-persona");
      this.activePersonaMount = document.getElementById("mount-cfg-active-persona");

      this.lightBaseInput = document.getElementById("cfg-light-base-url");
      this.lightKeyInput = document.getElementById("cfg-light-api-key");
      this.lightModelInput = document.getElementById("cfg-light-model");

      this.saveAiBtn = document.getElementById("save-ai-config-btn");
      this.saveBtn = document.getElementById("save-config-btn");
      this.saveIntegrationsBtn = document.getElementById("save-integrations-btn");
      this.testBtn = document.getElementById("cfg-test-push-btn");

      this.openBtn?.addEventListener("click", () => this.open());
      this.closeBtn?.addEventListener("click", () => this.close());
      this.overlay?.addEventListener("click", (e) => {
        if (e.target === this.overlay) this.close();
      });

      // Tabs switching
      this.tabBtns = document.querySelectorAll(".settings-tab-btn");
      this.tabPanes = {
        ai: document.getElementById("tab-pane-ai"),
        push: document.getElementById("tab-pane-push"),
        integrations: document.getElementById("tab-pane-integrations"),
        security: document.getElementById("tab-pane-security"),
      };
      this.tabBtns.forEach((btn) => {
        btn.addEventListener("click", () => {
          const target = btn.dataset.tab;
          this.tabBtns.forEach((b) => b.classList.toggle("active", b === btn));
          Object.entries(this.tabPanes).forEach(([key, pane]) => {
            if (!pane) return;
            pane.classList.toggle("hidden", key !== target);
          });
        });
      });

      if (this.aiModelMount) {
        this.aiModelSelect = new CustomSelect(this.aiModelMount, {
          options: [
            { value: "deepseek-flash", label: "DeepSeek Flash (默认主力 · 支持思考开关)" },
            { value: "deepseek-v4-pro", label: "DeepSeek V4 Pro (深度主力推理)" },
          ],
          value: "deepseek-flash",
          onChange: (val) => {
            if (this.aiModelInput) this.aiModelInput.value = val;
          },
        });
      }

      if (this.activePersonaMount) {
        this.activePersonaSelect = new CustomSelect(this.activePersonaMount, {
          options: [
            { value: "grok", label: "Grok 锐评模式 (开门见山、高信息密度)" },
            { value: "butler", label: "执更管家模式 (严谨细致、运维与账单盘点)" },
            { value: "architect", label: "架构师模式 (根因剖析、系统权衡与方案)" },
          ],
          value: "grok",
          onChange: (val) => {
            if (this.activePersonaInput) this.activePersonaInput.value = val;
          },
        });
      }

      if (this.channelMount) {
        this.channelSelect = new CustomSelect(this.channelMount, {
          options: [
            { value: "native", label: "自建原生推送流 (Android 客户端 + 控制台收件箱)" },
            { value: "wecom", label: "企业微信应用消息 (原生收件箱 + 微信插件直达)" },
            { value: "pushplus", label: "PushPlus 个人公众号 (原生收件箱 + 备用通道)" },
          ],
          value: "wecom",
          onChange: (val) => {
            if (this.channelInput) this.channelInput.value = val;
            this.toggleChannelFields(val);
          },
        });
      }

      this.baseToUserOptions = [
        { value: "@all", label: "全员推送 (@all - 默认)" },
        { value: "Starry", label: "仅推送给管理员 (Starry)" },
      ];
      if (this.wecomToUserMount) {
        this.toUserSelect = new CustomSelect(this.wecomToUserMount, {
          options: this.baseToUserOptions,
          value: "@all",
          onChange: (val) => {
            if (this.wecomToUserInput) this.wecomToUserInput.value = val;
          },
        });
      }

      this.bus.on("data:loaded", (state) => {
        const cfgs = state.configs || {};
        const chan = cfgs["push_channel"] || "wecom";
        if (this.channelInput) this.channelInput.value = chan;
        if (this.channelSelect) this.channelSelect.setValue(chan);
        this.toggleChannelFields(chan);

        if (this.wecomCorpInput) this.wecomCorpInput.value = cfgs["wecom_corpid"] || "";
        if (this.wecomAgentInput) this.wecomAgentInput.value = cfgs["wecom_agentid"] || "";
        if (this.wecomSecretInput) this.wecomSecretInput.value = cfgs["wecom_secret"] || "";

        const toUser = cfgs["wecom_touser"] || "@all";
        if (this.wecomToUserInput) this.wecomToUserInput.value = toUser;
        if (this.toUserSelect) {
          const exists = this.baseToUserOptions.some((o) => o.value === toUser);
          const opts = exists
            ? this.baseToUserOptions
            : [...this.baseToUserOptions, { value: toUser, label: `指定账号 (${toUser})` }];
          this.toUserSelect.setOptions(opts, toUser);
        }

        if (this.pushplusInput) this.pushplusInput.value = cfgs["pushplus_token"] || "";
        if (this.nezhaUrlInput) this.nezhaUrlInput.value = cfgs["nezha_url"] || "https://dash.starryheavens.uk";
        if (this.nezhaDbPathInput) this.nezhaDbPathInput.value = cfgs["nezha_db_path"] || "/opt/nezha/dashboard/data/sqlite.db";
        if (this.appTokenInput) this.appTokenInput.value = cfgs["app_bearer_token"] || "";
        if (this.cfInput) this.cfInput.value = cfgs["cf_api_token"] || "";
        if (this.biliSessdataInput) this.biliSessdataInput.value = cfgs["bili_sessdata"] || "";

        if (this.deepseekKeyInput) this.deepseekKeyInput.value = cfgs["deepseek_api_key"] || "";
        if (this.deepseekBaseInput) this.deepseekBaseInput.value = cfgs["deepseek_base_url"] || "https://api.deepseek.com/v1";
        const aiModel = cfgs["ai_model"] || "deepseek-flash";
        if (this.aiModelInput) this.aiModelInput.value = aiModel;
        this.aiModelSelect?.setValue(aiModel, false);

        const persona = cfgs["active_persona"] || "grok";
        if (this.activePersonaInput) this.activePersonaInput.value = persona;
        this.activePersonaSelect?.setValue(persona, false);

        if (this.lightBaseInput) this.lightBaseInput.value = cfgs["light_base_url"] || "";
        if (this.lightKeyInput) this.lightKeyInput.value = cfgs["light_api_key"] || "";
        if (this.lightModelInput) this.lightModelInput.value = cfgs["light_model"] || "";
      });

      this.form?.addEventListener("submit", async (e) => {
        e.preventDefault();
        const payload = {
          push_channel: (this.channelInput?.value || "wecom").trim(),
          wecom_corpid: (this.wecomCorpInput?.value || "").trim(),
          wecom_agentid: (this.wecomAgentInput?.value || "").trim(),
          wecom_secret: (this.wecomSecretInput?.value || "").trim(),
          wecom_touser: (this.wecomToUserInput?.value || "@all").trim(),
          pushplus_token: (this.pushplusInput?.value || "").trim(),
          nezha_url: (this.nezhaUrlInput?.value || "").trim(),
          nezha_db_path: (this.nezhaDbPathInput?.value || "").trim(),
          app_bearer_token: (this.appTokenInput?.value || "").trim(),
          cf_api_token: (this.cfInput?.value || "").trim(),
          bili_sessdata: (this.biliSessdataInput?.value || "").trim(),
          deepseek_api_key: (this.deepseekKeyInput?.value || "").trim(),
          deepseek_base_url: (this.deepseekBaseInput?.value || "https://api.deepseek.com/v1").trim(),
          ai_model: (this.aiModelInput?.value || "deepseek-flash").trim(),
          active_persona: (this.activePersonaInput?.value || "grok").trim(),
          light_base_url: (this.lightBaseInput?.value || "").trim(),
          light_api_key: (this.lightKeyInput?.value || "").trim(),
          light_model: (this.lightModelInput?.value || "").trim(),
        };
        if (this.saveAiBtn) this.saveAiBtn.disabled = true;
        if (this.saveBtn) this.saveBtn.disabled = true;
        if (this.saveIntegrationsBtn) this.saveIntegrationsBtn.disabled = true;
        try {
          await this.store.saveConfig(payload);
        } finally {
          if (this.saveAiBtn) this.saveAiBtn.disabled = false;
          if (this.saveBtn) this.saveBtn.disabled = false;
          if (this.saveIntegrationsBtn) this.saveIntegrationsBtn.disabled = false;
        }
      });

      this.testBtn?.addEventListener("click", async () => {
        this.testBtn.disabled = true;
        try {
          await this.store.testPush();
        } finally {
          this.testBtn.disabled = false;
        }
      });
    }

    toggleChannelFields(channel) {
      const isNative = channel === "native";
      const isPushPlus = channel === "pushplus";
      this.wecomGroup?.classList.toggle("hidden", isPushPlus || isNative);
      this.pushplusGroup?.classList.toggle("hidden", !isPushPlus || isNative);
    }

    open() {
      this.overlay?.classList.remove("hidden");
    }

    close() {
      this.overlay?.classList.add("hidden");
    }
  }

  // --- Password Form Component ---
  class PasswordComponent extends BaseComponent {
    init() {
      this.form = document.getElementById("password-form");
      this.passInput = document.getElementById("new-password");

      this.form?.addEventListener("submit", async (e) => {
        e.preventDefault();
        const val = this.passInput.value.trim();
        if (val.length < 6) {
          this.bus.emit("toast", "新访问密钥至少需要 6 位");
          return;
        }
        await this.store.updatePassword(val);
        this.passInput.value = "";
      });
    }
  }

  // --- Push Logs Modal Component ---
  class PushLogsComponent extends BaseComponent {
    init() {
      this.overlay = document.getElementById("logs-modal-overlay");
      this.openBtn = document.getElementById("open-logs-btn");
      this.cardTrigger = document.getElementById("card-stat-sent");
      this.closeBtn = document.getElementById("close-logs-modal-btn");
      this.list = document.getElementById("logs-list");
      this.expandedLogIds = new Set();

      this.openBtn?.addEventListener("click", () => this.open());
      this.cardTrigger?.addEventListener("click", () => this.open());
      this.closeBtn?.addEventListener("click", () => this.close());
      this.overlay?.addEventListener("click", (e) => {
        if (e.target === this.overlay) this.close();
      });

      this.bus.on("data:loaded", (state) => this.render(state.pushLogs));
    }

    open() {
      this.overlay?.classList.remove("hidden");
    }

    close() {
      this.overlay?.classList.add("hidden");
    }

    render(logs) {
      if (!this.list) return;
      if (!logs || !logs.length) {
        this.list.innerHTML = `<div class="empty-state">暂无推送记录</div>`;
        return;
      }

      this.list.innerHTML = logs.map((l) => {
        const isExp = this.expandedLogIds.has(String(l.id));
        return `
          <div class="log-item">
            <div class="log-head" data-id="${l.id}">
              <div class="log-title-wrap">
                <span class="dot ${l.status === "sent" ? "dot-live" : "dot-err"}"></span>
                <span class="log-tag mono">${Utils.esc(l.module_id)}</span>
                <span class="log-title">${Utils.esc(l.title)}</span>
              </div>
              <span class="log-meta mono">${Utils.esc(l.created_at.slice(5, 16))}</span>
            </div>
            <div id="log-body-${l.id}" class="log-body mono ${isExp ? "" : "hidden"}">${Utils.esc(l.content)}\n\n[响应] ${Utils.esc(l.response_msg || "OK")}</div>
          </div>
        `;
      }).join("");

      this.list.querySelectorAll(".log-head").forEach((h) => {
        h.addEventListener("click", () => {
          const id = h.dataset.id;
          const body = document.getElementById(`log-body-${id}`);
          if (body) {
            body.classList.toggle("hidden");
            if (body.classList.contains("hidden")) {
              this.expandedLogIds.delete(String(id));
            } else {
              this.expandedLogIds.add(String(id));
            }
          }
        });
      });
    }
  }

  // --- Asset Modal Component (Powered by CustomSelect & Fool-proof Split Controls) ---
  class AssetModalComponent extends BaseComponent {
    init() {
      this.overlay = document.getElementById("asset-modal-overlay");
      this.form = document.getElementById("asset-edit-form");
      this.titleEl = document.getElementById("modal-asset-title");
      this.hiddenId = document.getElementById("edit-asset-id");
      this.idInput = document.getElementById("edit-asset-id-input");
      this.nameInput = document.getElementById("edit-asset-name");
      this.catHidden = document.getElementById("edit-asset-category");
      this.specInput = document.getElementById("edit-asset-spec");

      // Composite Price Elements
      this.priceHidden = document.getElementById("edit-asset-price");
      this.currencyHidden = document.getElementById("edit-asset-currency");
      this.amountInput = document.getElementById("edit-asset-amount");
      this.cycleHidden = document.getElementById("edit-asset-cycle");
      this.pricePreview = document.getElementById("preview-asset-price");

      // Composite Due Date Elements
      this.dueHidden = document.getElementById("edit-asset-due");
      this.dueYearHidden = document.getElementById("edit-asset-due-year");
      this.dueMonthHidden = document.getElementById("edit-asset-due-month");
      this.dueDayHidden = document.getElementById("edit-asset-due-day");
      this.duePreview = document.getElementById("preview-asset-due-relative");

      // Status & AutoRenew Elements
      this.statusHidden = document.getElementById("edit-asset-status");
      this.renewHidden = document.getElementById("edit-asset-autorenew");
      this.warnDaysInput = document.getElementById("edit-asset-warndays");
      this.warnDaysHint = document.getElementById("preview-asset-warndays-hint");

      this.dynFieldsContainer = document.getElementById("asset-dynamic-fields");
      this.extraTextarea = document.getElementById("edit-asset-extra");
      this.deleteBtn = document.getElementById("delete-asset-btn");
      this.closeBtn = document.getElementById("close-modal-btn");
      this.cancelBtn = document.getElementById("cancel-modal-btn");

      this.dynamicSelects = [];

      // 1. Custom Select: Category
      this.categorySelect = new CustomSelect("#mount-asset-category", {
        options: [
          { label: "算力与云主机 (compute)", value: "compute" },
          { label: "核心域名资产 (domain)", value: "domain" },
          { label: "数字会员与生活订阅 (life)", value: "life" },
        ],
        value: "compute",
        onChange: (cat) => {
          if (this.catHidden) this.catHidden.value = cat;
          const extraObj = Utils.safeJSONParse(this.extraTextarea.value);
          this.renderDynamicFields(cat, extraObj);
          if (!this.hiddenId.value) {
            this.idInput.value = Utils.generateSlug(this.nameInput.value, cat);
          }
        },
      });

      // 2. Custom Select: Currency
      this.currencySelect = new CustomSelect("#mount-asset-currency", {
        options: [
          { label: "¥ 人民币 (CNY)", value: "CNY" },
          { label: "$ 美元 (USD)", value: "USD" },
          { label: "€ 欧元 (EUR)", value: "EUR" },
          { label: "£ 英镑 (GBP)", value: "GBP" },
          { label: "HK$ 港币 (HKD)", value: "HKD" },
        ],
        value: "CNY",
        onChange: (curr) => {
          if (this.currencyHidden) this.currencyHidden.value = curr;
          this.syncPrice();
        },
      });

      // 3. Custom Select: Billing Cycle
      this.cycleSelect = new CustomSelect("#mount-asset-cycle", {
        options: [
          { label: "按月续费 (/月)", value: "/月" },
          { label: "按季度 (/季)", value: "/季" },
          { label: "半年付 (/半年)", value: "/半年" },
          { label: "按年续费 (/年)", value: "/年" },
          { label: "每两年 (/2年)", value: "/2年" },
          { label: "一次性买断 (/终身)", value: "/终身" },
          { label: "免费 / 赞助 (免续费)", value: "/免费" },
        ],
        value: "/月",
        onChange: (cycle) => {
          if (this.cycleHidden) this.cycleHidden.value = cycle;
          this.syncPrice();
        },
      });

      // Amount input sync
      this.amountInput?.addEventListener("input", () => this.syncPrice());

      // 4. Custom Select: Due Date (Year, Month, Day)
      const currentYear = new Date().getFullYear();
      const yearOptions = [];
      for (let y = currentYear - 1; y <= currentYear + 10; y++) {
        yearOptions.push({ label: `${y} 年`, value: String(y) });
      }
      this.yearSelect = new CustomSelect("#mount-asset-due-year", {
        options: yearOptions,
        value: String(currentYear),
        onChange: (y) => {
          if (this.dueYearHidden) this.dueYearHidden.value = y;
          this.refreshDayOptions();
          this.syncDueDate();
        },
      });

      const monthOptions = [];
      for (let m = 1; m <= 12; m++) {
        const val = String(m).padStart(2, "0");
        monthOptions.push({ label: `${val} 月`, value: val });
      }
      this.monthSelect = new CustomSelect("#mount-asset-due-month", {
        options: monthOptions,
        value: "01",
        onChange: (m) => {
          if (this.dueMonthHidden) this.dueMonthHidden.value = m;
          this.refreshDayOptions();
          this.syncDueDate();
        },
      });

      this.daySelect = new CustomSelect("#mount-asset-due-day", {
        options: [{ label: "01 日", value: "01" }],
        value: "01",
        onChange: (d) => {
          if (this.dueDayHidden) this.dueDayHidden.value = d;
          this.syncDueDate();
        },
      });

      // 5. Custom Select: Running Status
      this.statusSelect = new CustomSelect("#mount-asset-status", {
        options: [
          { label: "正常运行 (active)", value: "运行中" },
          { label: "即将到期 (expiring)", value: "即将到期" },
          { label: "已欠费暂停 (suspended)", value: "已暂停" },
          { label: "长期有效 (permanent)", value: "长期有效" },
        ],
        value: "运行中",
        onChange: (st) => {
          if (this.statusHidden) this.statusHidden.value = st;
        },
      });

      // 6. Custom Select: AutoRenew
      this.autorenewSelect = new CustomSelect("#mount-asset-autorenew", {
        options: [
          { label: "开启自动续费 (到期日前防扣款提醒)", value: "true" },
          { label: "关闭自动续费 (到期需人工手动处理)", value: "false" },
        ],
        value: "true",
        onChange: (val) => {
          if (this.renewHidden) this.renewHidden.value = val;
        },
      });

      // Date Presets Pills
      document.querySelectorAll(".date-pill-group .date-pill").forEach((btn) => {
        btn.addEventListener("click", () => {
          document.querySelectorAll(".date-pill-group .date-pill").forEach((b) => b.classList.remove("active"));
          btn.classList.add("active");

          if (btn.dataset.permanent === "true") {
            this.setDueValues("2099", "12", "31");
            return;
          }

          const addM = parseInt(btn.dataset.addMonths, 10) || 1;
          const target = new Date();
          target.setMonth(target.getMonth() + addM);
          const y = String(target.getFullYear());
          const m = String(target.getMonth() + 1).padStart(2, "0");
          const d = String(target.getDate()).padStart(2, "0");
          this.setDueValues(y, m, d);
        });
      });

      // 7. Custom Select: Warn Days Strategy
      this.baseWarnOptions = [
        { label: "标准提醒 (提前 7天 / 3天 / 1天)", value: "[7, 3, 1]" },
        { label: "密集防漏 (提前 15天 / 7天 / 3天 / 1天 / 当天)", value: "[15, 7, 3, 1, 0]" },
        { label: "长周期预留 (提前 30天 / 7天 / 1天)", value: "[30, 7, 1]" },
        { label: "极简冲刺 (仅提前 3天 / 1天)", value: "[3, 1]" },
        { label: "临期前一天 (仅提前 1天)", value: "[1]" },
        { label: "静默追踪 (不主动推送提醒)", value: "[]" },
      ];
      this.warnDaysSelect = new CustomSelect("#mount-asset-warndays", {
        options: this.baseWarnOptions,
        value: "[7, 3, 1]",
        onChange: (val) => {
          if (this.warnDaysInput) this.warnDaysInput.value = val;
          this.updateWarnDaysHint(val);
        },
      });

      // Spec Quick Pills
      document.querySelectorAll(".spec-pill-group .spec-pill").forEach((btn) => {
        btn.addEventListener("click", () => {
          document.querySelectorAll(".spec-pill-group .spec-pill").forEach((b) => b.classList.remove("active"));
          btn.classList.add("active");
          if (this.specInput) this.specInput.value = btn.dataset.spec || "";
        });
      });

      // Novice Service Presets (One-click Instant Starter)
      const servicePresets = {
        bandwagon: {
          name: "搬瓦工 GIA VPS",
          cat: "compute",
          spec: "2C2G / 洛杉矶 DC9 CN2-GIA",
          curr: "USD",
          amt: "49.99",
          cycle: "/年",
          addMonths: 12,
          autoRenew: "true",
          warnDays: "[7, 3, 1]",
          extra: { provider: "BandwagonHost" },
        },
        cloudflare: {
          name: "starryheavens.uk",
          cat: "domain",
          spec: "COM/UK 顶级域名 / 权威托管",
          curr: "USD",
          amt: "9.77",
          cycle: "/年",
          addMonths: 12,
          autoRenew: "true",
          warnDays: "[30, 7, 1]",
          extra: { registrar: "Cloudflare" },
        },
        bilibili: {
          name: "哔哩哔哩大会员",
          cat: "life",
          spec: "年度大会员 / 4K 高码率画质",
          curr: "CNY",
          amt: "148.00",
          cycle: "/年",
          addMonths: 12,
          autoRenew: "true",
          warnDays: "[7, 3, 1]",
          extra: { platform: "bilibili", billing_note: "微信免密扣款" },
        },
        qqmusic: {
          name: "QQ音乐豪华绿钻",
          cat: "life",
          spec: "绿钻豪华版 / 臻品母带音质",
          curr: "CNY",
          amt: "15.00",
          cycle: "/月",
          addMonths: 1,
          autoRenew: "false",
          warnDays: "[3, 1]",
          extra: { platform: "qqmusic", billing_note: "微信免密扣款" },
        },
        chatgpt: {
          name: "ChatGPT Plus 订阅",
          cat: "life",
          spec: "OpenAI Plus 个人会员",
          curr: "USD",
          amt: "20.00",
          cycle: "/月",
          addMonths: 1,
          autoRenew: "true",
          warnDays: "[3, 1]",
          extra: { platform: "chatgpt", billing_note: "Visa外币信用卡" },
        },
        netflix: {
          name: "Netflix 奈飞高级车位",
          cat: "life",
          spec: "Ultra HD 4K / 独立车位",
          curr: "USD",
          amt: "19.99",
          cycle: "/月",
          addMonths: 1,
          autoRenew: "true",
          warnDays: "[3, 1]",
          extra: { platform: "netflix", billing_note: "Visa外币信用卡" },
        },
        spotify: {
          name: "Spotify 音乐家庭组",
          cat: "life",
          spec: "Premium 6人家庭组主账号",
          curr: "HKD",
          amt: "88.00",
          cycle: "/月",
          addMonths: 1,
          autoRenew: "true",
          warnDays: "[7, 3, 1]",
          extra: { platform: "spotify", billing_note: "Visa外币信用卡" },
        },
      };

      document.querySelectorAll(".service-preset").forEach((btn) => {
        btn.addEventListener("click", () => {
          const key = btn.dataset.service;
          const p = servicePresets[key];
          if (!p) return;
          this.applyServicePreset(p);
        });
      });

      this.bus.on("modal:open-asset", (item) => this.open(item));

      // Close handlers
      this.closeBtn?.addEventListener("click", () => this.close());
      this.cancelBtn?.addEventListener("click", () => this.close());
      this.overlay?.addEventListener("click", (e) => {
        if (e.target === this.overlay) this.close();
      });

      // Name -> Slug
      this.nameInput?.addEventListener("input", () => {
        if (!this.hiddenId.value) {
          const cat = this.catHidden ? this.catHidden.value : "compute";
          this.idInput.value = Utils.generateSlug(this.nameInput.value, cat);
        }
      });

      // Extra Textarea Manual Edit -> Sync to inputs
      this.extraTextarea?.addEventListener("input", () => {
        const parsed = Utils.safeJSONParse(this.extraTextarea.value, null);
        if (!parsed) return;
        this.dynFieldsContainer?.querySelectorAll(".asset-dyn-input").forEach((el) => {
          if (parsed[el.dataset.key] !== undefined) el.value = parsed[el.dataset.key];
        });
      });

      // Delete
      this.deleteBtn?.addEventListener("click", async () => {
        const id = this.hiddenId.value;
        const name = this.nameInput.value;
        if (!id) return;
        if (!confirm(`确定下架并删除资产 [${name || id}] 吗？`)) return;
        await this.store.deleteAsset(id);
        this.close();
      });

      // Form Submit
      this.form?.addEventListener("submit", async (e) => {
        e.preventDefault();
        const existingID = this.hiddenId.value.trim();
        const idInputVal = this.idInput.value.trim();
        const isNew = !existingID;
        const finalID = isNew ? idInputVal : existingID;

        if (!finalID) {
          this.bus.emit("toast", "资产 ID 不能为空");
          return;
        }

        this.syncInputsToJSON();

        const cat = this.catHidden ? this.catHidden.value : "compute";
        const extraObj = Utils.safeJSONParse(this.extraTextarea.value);
        let derivedSpec = this.specInput.value.trim();
        if (!derivedSpec) {
          const catFallback = cat === "compute" ? "云主机算力" : (cat === "domain" ? "核心域名" : "数字订阅");
          derivedSpec = extraObj.provider || extraObj.registrar || extraObj.platform || catFallback;
        }

        const dueStr = this.dueHidden ? this.dueHidden.value.trim() : "";
        const cycleVal = this.cycleHidden ? this.cycleHidden.value : "/月";
        let derivedStatus = "运行中";
        if (dueStr.startsWith("2099") || cycleVal === "/终身" || cycleVal === "/免费") {
          derivedStatus = "长期有效";
        } else if (dueStr) {
          const diffDays = Math.ceil((new Date(dueStr).getTime() - Date.now()) / 86400000);
          if (!isNaN(diffDays) && diffDays <= 7) {
            derivedStatus = "即将到期";
          }
        }
        if (this.statusHidden) this.statusHidden.value = derivedStatus;

        const payload = {
          id: finalID,
          name: this.nameInput.value.trim(),
          category: cat,
          spec: derivedSpec,
          price: this.priceHidden ? this.priceHidden.value.trim() : "",
          due_date: dueStr,
          status: derivedStatus,
          auto_renew: this.renewHidden ? this.renewHidden.value === "true" : true,
          warn_days: this.warnDaysInput.value.trim(),
          extra_config: this.extraTextarea.value.trim(),
        };

        try {
          await this.store.saveAsset(payload, isNew);
          this.close();
        } catch (err) {
          this.bus.emit("toast", err.message);
        }
      });
    }

    // Helper: Price Calculation & Formatting
    syncPrice() {
      const curr = this.currencyHidden?.value || "CNY";
      const cycle = this.cycleHidden?.value || "/月";
      const amtVal = parseFloat(this.amountInput?.value || "0");
      const isFree = cycle === "/免费" || (amtVal === 0 && cycle === "/免费");

      const currSymbols = { CNY: "¥", USD: "$", EUR: "€", GBP: "£", HKD: "HK$" };
      const symbol = currSymbols[curr] || "¥";

      let finalPriceStr = "";
      if (isFree) {
        finalPriceStr = "免费";
      } else {
        const amtFormatted = isNaN(amtVal) ? "0.00" : amtVal.toFixed(2);
        finalPriceStr = `${symbol}${amtFormatted}${cycle}`;
      }

      if (this.priceHidden) this.priceHidden.value = finalPriceStr;
      if (this.pricePreview) {
        this.pricePreview.textContent = isFree ? "免费 (免续费)" : `${symbol} ${isNaN(amtVal) ? "0.00" : amtVal.toFixed(2)} ${cycle}`;
      }
    }

    parsePrice(priceStr) {
      if (!priceStr || priceStr === "免费" || priceStr.includes("免续费")) {
        if (this.amountInput) this.amountInput.value = "0";
        this.cycleSelect?.setValue("/免费", false);
        if (this.cycleHidden) this.cycleHidden.value = "/免费";
        this.syncPrice();
        return;
      }

      let curr = "CNY";
      if (priceStr.includes("$") && !priceStr.includes("HK$")) curr = "USD";
      else if (priceStr.includes("€")) curr = "EUR";
      else if (priceStr.includes("£")) curr = "GBP";
      else if (priceStr.includes("HK$")) curr = "HKD";
      else if (priceStr.includes("¥") || priceStr.includes("元") || priceStr.includes("CNY")) curr = "CNY";

      let cycle = "/月";
      if (priceStr.includes("2年")) cycle = "/2年";
      else if (priceStr.includes("年")) cycle = "/年";
      else if (priceStr.includes("半年")) cycle = "/半年";
      else if (priceStr.includes("季")) cycle = "/季";
      else if (priceStr.includes("终身") || priceStr.includes("买断")) cycle = "/终身";
      else if (priceStr.includes("免费") || priceStr.includes("活动")) cycle = "/免费";

      const numMatch = priceStr.match(/([0-9]+(?:\.[0-9]+)?)/);
      const amt = numMatch ? numMatch[1] : "0";

      this.currencySelect?.setValue(curr, false);
      if (this.currencyHidden) this.currencyHidden.value = curr;
      if (this.amountInput) this.amountInput.value = amt;
      this.cycleSelect?.setValue(cycle, false);
      if (this.cycleHidden) this.cycleHidden.value = cycle;

      this.syncPrice();
    }

    // Helper: Due Date Calculation & Dynamic Days
    refreshDayOptions() {
      const year = parseInt(this.dueYearHidden?.value || new Date().getFullYear(), 10);
      const month = parseInt(this.dueMonthHidden?.value || (new Date().getMonth() + 1), 10);
      const daysInMonth = new Date(year, month, 0).getDate();

      const dayOpts = [];
      for (let d = 1; d <= daysInMonth; d++) {
        const val = String(d).padStart(2, "0");
        dayOpts.push({ label: `${val} 日`, value: val });
      }
      this.daySelect?.setOptions(dayOpts);

      const currentDayVal = parseInt(this.dueDayHidden?.value || "1", 10);
      if (currentDayVal > daysInMonth) {
        const safeDay = String(daysInMonth).padStart(2, "0");
        this.daySelect?.setValue(safeDay, false);
        if (this.dueDayHidden) this.dueDayHidden.value = safeDay;
      }
    }

    setDueValues(year, month, day) {
      if (this.dueYearHidden) this.dueYearHidden.value = year;
      if (this.dueMonthHidden) this.dueMonthHidden.value = month;
      if (this.dueDayHidden) this.dueDayHidden.value = day;

      this.yearSelect?.setValue(year, false);
      this.monthSelect?.setValue(month, false);
      this.refreshDayOptions();
      this.daySelect?.setValue(day, false);
      this.syncDueDate();
    }

    syncDueDate() {
      const y = this.dueYearHidden?.value || String(new Date().getFullYear());
      const m = this.dueMonthHidden?.value || "01";
      const d = this.dueDayHidden?.value || "01";

      const rfcStr = `${y}-${m}-${d}T23:59:59+08:00`;
      if (this.dueHidden) this.dueHidden.value = rfcStr;

      if (this.duePreview) {
        if (y === "2099") {
          this.duePreview.textContent = "永久免续费";
          return;
        }
        const target = new Date(`${y}-${m}-${d}T23:59:59`);
        const now = new Date();
        const diffMs = target.getTime() - now.getTime();
        const diffDays = Math.ceil(diffMs / (1000 * 60 * 60 * 24));

        if (diffDays < 0) {
          this.duePreview.textContent = `已逾期 ${Math.abs(diffDays)} 天`;
        } else if (diffDays === 0) {
          this.duePreview.textContent = "今日即将到期";
        } else if (diffDays > 365) {
          this.duePreview.textContent = `距今约 ${(diffDays / 365).toFixed(1)} 年 (剩余 ${diffDays} 天)`;
        } else {
          this.duePreview.textContent = `距今剩余 ${diffDays} 天`;
        }
      }
    }

    parseDueDate(dueStr) {
      if (!dueStr) {
        const nextMonth = new Date();
        nextMonth.setMonth(nextMonth.getMonth() + 1);
        const y = String(nextMonth.getFullYear());
        const m = String(nextMonth.getMonth() + 1).padStart(2, "0");
        const d = String(nextMonth.getDate()).padStart(2, "0");
        this.setDueValues(y, m, d);
        return;
      }

      const parsed = new Date(dueStr);
      if (isNaN(parsed.getTime())) {
        const y = String(new Date().getFullYear());
        this.setDueValues(y, "01", "01");
        return;
      }

      const y = String(parsed.getFullYear());
      const m = String(parsed.getMonth() + 1).padStart(2, "0");
      const d = String(parsed.getDate()).padStart(2, "0");
      this.setDueValues(y, m, d);
    }

    setWarnDaysValue(rawVal) {
      const normalized = Utils.normalizeWarnDays(rawVal || "[7, 3, 1]");
      if (this.warnDaysSelect && this.baseWarnOptions) {
        const exists = this.baseWarnOptions.some((o) => o.value === normalized);
        if (!exists) {
          this.warnDaysSelect.setOptions(
            [...this.baseWarnOptions, { label: `自定义梯度 (${normalized})`, value: normalized }],
            normalized
          );
        } else {
          this.warnDaysSelect.setOptions(this.baseWarnOptions, normalized);
        }
      }
      if (this.warnDaysInput) this.warnDaysInput.value = normalized;
      this.updateWarnDaysHint(normalized);
    }

    updateWarnDaysHint(val) {
      if (!this.warnDaysHint) return;
      try {
        const arr = JSON.parse(val);
        if (Array.isArray(arr)) {
          if (arr.length === 0) {
            this.warnDaysHint.textContent = "已开启静默追踪，到期前不主动推送";
            return;
          }
          const formatted = arr.map((x) => (x === 0 ? "当天" : `${x}天`)).join("、");
          this.warnDaysHint.textContent = `将在到期前 ${formatted} 推送提醒`;
          return;
        }
      } catch (e) {}
      this.warnDaysHint.textContent = "将在设定梯度触发微信提醒";
    }

    applyServicePreset(p) {
      this.nameInput.value = p.name;
      this.idInput.value = Utils.generateSlug(p.name, p.cat);
      if (this.catHidden) this.catHidden.value = p.cat;
      this.categorySelect?.setValue(p.cat, false);
      this.specInput.value = p.spec;

      // Price
      this.currencySelect?.setValue(p.curr, false);
      if (this.currencyHidden) this.currencyHidden.value = p.curr;
      if (this.amountInput) this.amountInput.value = p.amt;
      this.cycleSelect?.setValue(p.cycle, false);
      if (this.cycleHidden) this.cycleHidden.value = p.cycle;
      this.syncPrice();

      // Due Date
      const target = new Date();
      target.setMonth(target.getMonth() + p.addMonths);
      const y = String(target.getFullYear());
      const m = String(target.getMonth() + 1).padStart(2, "0");
      const d = String(target.getDate()).padStart(2, "0");
      this.setDueValues(y, m, d);

      // Auto renew & warn days
      if (this.renewHidden) this.renewHidden.value = p.autoRenew;
      this.autorenewSelect?.setValue(p.autoRenew, false);
      this.setWarnDaysValue(p.warnDays);

      // Extra config
      this.extraTextarea.value = JSON.stringify(p.extra, null, 2);
      this.renderDynamicFields(p.cat, p.extra);

      this.bus.emit("toast", `已套用 [${p.name}] 常用配置预设`);
    }

    open(item) {
      const isNew = !item;
      // Reset pills active states
      document.querySelectorAll(".date-pill-group .date-pill").forEach((b) => b.classList.remove("active"));
      document.querySelectorAll(".spec-pill-group .spec-pill").forEach((b) => b.classList.remove("active"));

      const quickBanner = document.getElementById("asset-quick-presets");

      if (isNew) {
        this.titleEl.textContent = "新增数字资产与订阅";
        this.hiddenId.value = "";
        this.idInput.value = "";
        this.idInput.disabled = false;
        this.deleteBtn?.classList.add("hidden");
        quickBanner?.classList.remove("hidden");

        this.nameInput.value = "";
        if (this.catHidden) this.catHidden.value = "compute";
        this.categorySelect?.setValue("compute", false);
        this.specInput.value = "";

        // Reset Price
        if (this.amountInput) this.amountInput.value = "0";
        this.currencySelect?.setValue("CNY", false);
        this.cycleSelect?.setValue("/月", false);
        this.syncPrice();

        // Default Due Date: +1 month
        const nextMonth = new Date();
        nextMonth.setMonth(nextMonth.getMonth() + 1);
        const y = String(nextMonth.getFullYear());
        const m = String(nextMonth.getMonth() + 1).padStart(2, "0");
        const d = String(nextMonth.getDate()).padStart(2, "0");
        this.setDueValues(y, m, d);

        // Status & AutoRenew
        if (this.statusHidden) this.statusHidden.value = "运行中";
        this.statusSelect?.setValue("运行中", false);
        if (this.renewHidden) this.renewHidden.value = "true";
        this.autorenewSelect?.setValue("true", false);

        this.setWarnDaysValue("[7, 3, 1]");
        this.extraTextarea.value = "{}";

        this.renderDynamicFields("compute", {});
      } else {
        this.titleEl.textContent = `编辑资产 · ${item.name}`;
        this.hiddenId.value = item.id;
        this.idInput.value = item.id;
        this.idInput.disabled = true;
        this.deleteBtn?.classList.remove("hidden");
        quickBanner?.classList.add("hidden");

        this.nameInput.value = item.name;
        const cat = item.category || "compute";
        if (this.catHidden) this.catHidden.value = cat;
        this.categorySelect?.setValue(cat, false);

        this.specInput.value = item.spec || "";

        // Parse Price & Due Date
        this.parsePrice(item.price || "");
        this.parseDueDate(item.due_date || "");

        const currentStatus = item.status || "运行中";
        if (this.statusHidden) this.statusHidden.value = currentStatus;
        this.statusSelect?.setValue(currentStatus, false);

        const autoRenewStr = item.auto_renew ? "true" : "false";
        if (this.renewHidden) this.renewHidden.value = autoRenewStr;
        this.autorenewSelect?.setValue(autoRenewStr, false);

        this.setWarnDaysValue(item.warn_days || "[7, 3, 1]");

        const extraObj = Utils.safeJSONParse(item.extra_config);
        this.extraTextarea.value = JSON.stringify(extraObj, null, 2);
        this.renderDynamicFields(cat, extraObj);
      }

      this.overlay?.classList.remove("hidden");
    }

    close() {
      this.overlay?.classList.add("hidden");
    }

    renderDynamicFields(category, extraConfigObj = {}) {
      if (!this.dynFieldsContainer) return;
      this.dynamicSelects.forEach((s) => s.destroy());
      this.dynamicSelects = [];

      const tpl = (this.store.state.assetTemplates || []).find((t) => t.category === category);
      if (!tpl || !tpl.fields || !tpl.fields.length) {
        this.dynFieldsContainer.innerHTML = `<div class="empty-state" style="padding:6px; font-size:11px;">该分类暂无特定扩展参数</div>`;
        return;
      }

      this.dynFieldsContainer.innerHTML = tpl.fields.map((f) => {
        const val = extraConfigObj[f.key] !== undefined ? extraConfigObj[f.key] : (f.default_val || "");
        if (f.type === "select" && f.options && f.options.length) {
          return `
            <div class="form-field">
              <label>${Utils.esc(f.label)}</label>
              <div id="mount-asset-dyn-${Utils.esc(f.key)}" class="v-select-mount"></div>
              <input type="hidden" class="asset-dyn-input" data-key="${Utils.esc(f.key)}" value="${Utils.esc(val)}" />
              ${f.hint ? `<span class="field-hint">${Utils.esc(f.hint)}</span>` : ""}
            </div>
          `;
        }
        return `
          <div class="form-field">
            <label for="asset-dyn-${Utils.esc(f.key)}">${Utils.esc(f.label)}</label>
            <input id="asset-dyn-${Utils.esc(f.key)}" type="${f.type === "number" ? "number" : "text"}" class="glass-input mono asset-dyn-input" data-key="${Utils.esc(f.key)}" value="${Utils.esc(val)}" placeholder="${Utils.esc(f.placeholder || "")}" />
            ${f.hint ? `<span class="field-hint">${Utils.esc(f.hint)}</span>` : ""}
          </div>
        `;
      }).join("");

      // Bind dynamic custom selects
      tpl.fields.forEach((f) => {
        if (f.type === "select" && f.options && f.options.length) {
          const mount = document.getElementById(`mount-asset-dyn-${f.key}`);
          const hiddenInput = this.dynFieldsContainer.querySelector(`.asset-dyn-input[data-key="${f.key}"]`);
          const parsedOptions = f.options.map((opt) => Utils.parseOptionItem(opt));
          const rawVal = extraConfigObj[f.key] !== undefined ? String(extraConfigObj[f.key]) : (f.default_val || parsedOptions[0]?.value || "");
          if (rawVal && !parsedOptions.some((o) => String(o.value) === rawVal)) {
            parsedOptions.push({ label: `${rawVal} (当前值)`, value: rawVal });
          }
          if (hiddenInput) hiddenInput.value = rawVal;
          if (mount) {
            const cs = new CustomSelect(mount, {
              options: parsedOptions,
              value: rawVal,
              onChange: (newVal) => {
                if (hiddenInput) hiddenInput.value = newVal;
                this.syncInputsToJSON();
              },
            });
            this.dynamicSelects.push(cs);
          }
        }
      });

      this.dynFieldsContainer.querySelectorAll("input.asset-dyn-input:not([type=hidden])").forEach((input) => {
        input.addEventListener("input", () => this.syncInputsToJSON());
        input.addEventListener("change", () => this.syncInputsToJSON());
      });
    }

    syncInputsToJSON() {
      const currentExtra = Utils.safeJSONParse(this.extraTextarea.value);
      this.dynFieldsContainer?.querySelectorAll(".asset-dyn-input").forEach((el) => {
        const key = el.dataset.key;
        let val = el.value.trim();
        if (el.type === "number") {
          const num = Number(val);
          val = isNaN(num) ? val : num;
        }
        if (val !== "") {
          currentExtra[key] = val;
        } else {
          delete currentExtra[key];
        }
      });
      this.extraTextarea.value = JSON.stringify(currentExtra, null, 2);
    }
  }

  // --- Task Modal Component (Powered by CustomSelect & Natural Language Scheduler) ---
  class TaskModalComponent extends BaseComponent {
    init() {
      this.overlay = document.getElementById("task-modal-overlay");
      this.form = document.getElementById("task-edit-form");
      this.titleEl = document.getElementById("modal-task-title");
      this.hiddenId = document.getElementById("edit-task-id-hidden");
      this.idInput = document.getElementById("edit-task-id");
      this.nameInput = document.getElementById("edit-task-name");
      this.typeHidden = document.getElementById("edit-task-type");

      // Schedule Elements
      this.cronHidden = document.getElementById("edit-task-cron");
      this.cronCustomInput = document.getElementById("edit-task-cron-custom");
      this.wrapCustom = document.getElementById("wrap-schedule-custom");
      this.wrapSub = document.getElementById("wrap-schedule-sub");
      this.wrapHour = document.getElementById("wrap-schedule-hour");
      this.wrapMinute = document.getElementById("wrap-schedule-minute");
      this.schedPreview = document.getElementById("preview-schedule-desc");

      this.enabledHidden = document.getElementById("edit-task-enabled");
      this.descInput = document.getElementById("edit-task-desc");
      this.wrapDynParams = document.getElementById("wrap-task-dynamic-params");
      this.dynParamsContainer = document.getElementById("task-dynamic-params");
      this.paramsTextarea = document.getElementById("edit-task-params-json");
      this.deleteBtn = document.getElementById("delete-task-btn");
      this.closeBtn = document.getElementById("close-task-modal-btn");
      this.cancelBtn = document.getElementById("cancel-task-modal-btn");

      this.dynamicSelects = [];

      // 1. Custom Select: Task Type
      this.typeSelect = new CustomSelect("#mount-task-type", {
        options: [
          { label: "晨报小秘书模板 (morning)", value: "morning" },
          { label: "开源技术雷达模板 (radar)", value: "radar" },
          { label: "数字资产守望模板 (asset_watcher)", value: "asset_watcher" },
        ],
        value: "morning",
        onChange: (type) => {
          if (this.typeHidden) this.typeHidden.value = type;
          const tpl = (this.store.state.templates || []).find((t) => t.type === type);
          const isNew = !this.hiddenId.value;

          if (isNew && tpl) {
            this.nameInput.value = tpl.name || "";
            this.parseCron(tpl.default_cron || "0 8 * * *");
            this.descInput.value = tpl.description || "";
            this.idInput.value = Utils.generateSlug(tpl.name, type);
          }

          const paramsObj = Utils.safeJSONParse(this.paramsTextarea.value);
          this.renderDynamicParams(type, paramsObj);
        },
      });

      // 2. Custom Select: Task Enabled
      this.enabledSelect = new CustomSelect("#mount-task-enabled", {
        options: [
          { label: "开启常态调度", value: "true" },
          { label: "暂停任务调度", value: "false" },
        ],
        value: "true",
        onChange: (val) => {
          if (this.enabledHidden) this.enabledHidden.value = val;
        },
      });

      // 3. Natural Language Schedule Selectors
      this.freqSelect = new CustomSelect("#mount-schedule-freq", {
        options: [
          { label: "每天执行 (Daily)", value: "daily" },
          { label: "工作日 (周一至五)", value: "workday" },
          { label: "周末 (周六日)", value: "weekend" },
          { label: "每周固定日 (Weekly)", value: "weekly" },
          { label: "每月固定日 (Monthly)", value: "monthly" },
          { label: "每隔若干小时 (Interval)", value: "interval" },
          { label: "极客自定义 (Cron)", value: "custom" },
        ],
        value: "daily",
        onChange: (freq) => {
          this.updateScheduleVisibility(freq);
          this.syncSchedule();
        },
      });

      this.subSelect = new CustomSelect("#mount-schedule-sub", {
        options: this.getSubOptions("weekly"),
        value: "1",
        onChange: () => this.syncSchedule(),
      });

      const hourOptions = [];
      for (let h = 0; h < 24; h++) {
        const val = String(h).padStart(2, "0");
        const period = h < 6 ? "凌晨" : (h < 12 ? "早间" : (h < 14 ? "午间" : (h < 18 ? "下午" : "晚间")));
        hourOptions.push({ label: `${val} 时 (${period})`, value: val });
      }
      this.hourSelect = new CustomSelect("#mount-schedule-hour", {
        options: hourOptions,
        value: "08",
        onChange: () => this.syncSchedule(),
      });

      const minuteOptions = [
        { label: "00 分 (整点)", value: "00" },
        { label: "05 分", value: "05" },
        { label: "10 分", value: "10" },
        { label: "15 分 (一刻)", value: "15" },
        { label: "20 分", value: "20" },
        { label: "30 分 (半点)", value: "30" },
        { label: "45 分", value: "45" },
      ];
      this.minuteSelect = new CustomSelect("#mount-schedule-minute", {
        options: minuteOptions,
        value: "00",
        onChange: () => this.syncSchedule(),
      });

      // Custom Cron input listener
      this.cronCustomInput?.addEventListener("input", () => {
        if (this.freqSelect?.value === "custom") {
          this.syncSchedule();
        }
      });

      // Quick Schedule Preset Pills
      document.querySelectorAll(".sched-pill").forEach((btn) => {
        btn.addEventListener("click", () => {
          document.querySelectorAll(".sched-pill").forEach((b) => b.classList.remove("active"));
          btn.classList.add("active");
          const cron = btn.dataset.cron;
          this.parseCron(cron);
        });
      });

      // Novice Task Presets
      const taskPresets = {
        morning_sh: {
          name: "上海晨间早报",
          type: "morning",
          cron: "0 8 * * *",
          desc: "每日早08:00定时汇总上海天气预报、出行建议、每日格言、节点脉搏与临期资产",
          params: { city: "Shanghai" },
        },
        morning_bj: {
          name: "北京晨间早报",
          type: "morning",
          cron: "0 8 * * *",
          desc: "每日早08:00定时汇总北京天气预报、穿衣与空气质量建议及节点脉搏",
          params: { city: "Beijing" },
        },
        radar_daily: {
          name: "全网开源技术风向标",
          type: "radar",
          cron: "0 20 * * *",
          desc: "每天晚20:00精选抓取 GitHub Trending 热门项目与 Hacker News 高分热榜",
          params: { hn_min_score: 150, limit: 8 },
        },
        asset_watcher: {
          name: "深夜资产临期巡检",
          type: "asset_watcher",
          cron: "0 3 * * *",
          desc: "每日凌晨03:00自动巡检所有主机、域名与会员到期状态，触发防扣款强提醒",
          params: {},
        },
      };

      document.querySelectorAll(".task-preset").forEach((btn) => {
        btn.addEventListener("click", () => {
          const key = btn.dataset.taskPreset;
          const p = taskPresets[key];
          if (!p) return;
          this.applyTaskPreset(p);
        });
      });

      this.bus.on("modal:open-task", (item) => this.open(item));

      // Close handlers
      this.closeBtn?.addEventListener("click", () => this.close());
      this.cancelBtn?.addEventListener("click", () => this.close());
      this.overlay?.addEventListener("click", (e) => {
        if (e.target === this.overlay) this.close();
      });

      // Name -> Slug
      this.nameInput?.addEventListener("input", () => {
        if (!this.hiddenId.value) {
          const type = this.typeHidden ? this.typeHidden.value : "morning";
          this.idInput.value = Utils.generateSlug(this.nameInput.value, type);
        }
      });

      // Params Textarea Manual Edit -> Sync to inputs
      this.paramsTextarea?.addEventListener("input", () => {
        const parsed = Utils.safeJSONParse(this.paramsTextarea.value, null);
        if (!parsed) return;
        this.dynParamsContainer?.querySelectorAll(".task-dyn-input").forEach((el) => {
          if (parsed[el.dataset.key] !== undefined) el.value = parsed[el.dataset.key];
        });
      });

      // Delete Task
      this.deleteBtn?.addEventListener("click", async () => {
        const id = this.hiddenId.value;
        const name = this.nameInput.value;
        if (!id) return;
        if (!confirm(`确定删除任务 [${name || id}] 吗？`)) return;
        await this.store.deleteTask(id);
        this.close();
      });

      // Form Submit
      this.form?.addEventListener("submit", async (e) => {
        e.preventDefault();
        const existingID = this.hiddenId.value.trim();
        const idInputVal = this.idInput.value.trim();
        const isNew = !existingID;
        const finalID = isNew ? idInputVal : existingID;

        if (!finalID) {
          this.bus.emit("toast", "任务 ID 不能为空");
          return;
        }

        this.syncInputsToJSON();

        const taskType = this.typeHidden ? this.typeHidden.value : "morning";
        const tpl = (this.store.state.templates || []).find((t) => t.type === taskType);
        const derivedDesc = this.descInput.value.trim() || tpl?.description || "";

        const payload = {
          id: finalID,
          name: this.nameInput.value.trim(),
          type: taskType,
          cron_expr: this.cronHidden?.value.trim() || "0 8 * * *",
          enabled: this.enabledHidden ? this.enabledHidden.value === "true" : true,
          description: derivedDesc,
          params: this.paramsTextarea.value.trim(),
        };

        try {
          await this.store.saveTask(payload, isNew);
          this.close();
        } catch (err) {
          this.bus.emit("toast", err.message);
        }
      });
    }

    getSubOptions(freq) {
      if (freq === "weekly") {
        return [
          { label: "每周一 (Monday)", value: "1" },
          { label: "每周二 (Tuesday)", value: "2" },
          { label: "每周三 (Wednesday)", value: "3" },
          { label: "每周四 (Thursday)", value: "4" },
          { label: "每周五 (Friday)", value: "5" },
          { label: "每周六 (Saturday)", value: "6" },
          { label: "每周日 (Sunday)", value: "0" },
        ];
      }
      if (freq === "monthly") {
        const opts = [];
        for (let d = 1; d <= 28; d++) {
          const val = String(d).padStart(2, "0");
          opts.push({ label: `每月 ${val} 号`, value: String(d) });
        }
        return opts;
      }
      if (freq === "interval") {
        return [
          { label: "每 1 小时", value: "1" },
          { label: "每 2 小时", value: "2" },
          { label: "每 3 小时", value: "3" },
          { label: "每 4 小时", value: "4" },
          { label: "每 6 小时", value: "6" },
          { label: "每 12 小时", value: "12" },
        ];
      }
      return [];
    }

    updateScheduleVisibility(freq) {
      if (freq === "custom") {
        this.wrapCustom?.classList.remove("hidden");
        if (this.wrapSub) this.wrapSub.style.display = "none";
        if (this.wrapHour) this.wrapHour.style.display = "none";
        if (this.wrapMinute) this.wrapMinute.style.display = "none";
      } else if (freq === "interval") {
        this.wrapCustom?.classList.add("hidden");
        if (this.wrapSub) {
          this.wrapSub.style.display = "block";
          this.subSelect?.setOptions(this.getSubOptions("interval"));
          this.subSelect?.setValue("1", false);
        }
        if (this.wrapHour) this.wrapHour.style.display = "none";
        if (this.wrapMinute) this.wrapMinute.style.display = "none";
      } else if (freq === "weekly") {
        this.wrapCustom?.classList.add("hidden");
        if (this.wrapSub) {
          this.wrapSub.style.display = "block";
          this.subSelect?.setOptions(this.getSubOptions("weekly"));
          this.subSelect?.setValue("1", false);
        }
        if (this.wrapHour) this.wrapHour.style.display = "block";
        if (this.wrapMinute) this.wrapMinute.style.display = "block";
      } else if (freq === "monthly") {
        this.wrapCustom?.classList.add("hidden");
        if (this.wrapSub) {
          this.wrapSub.style.display = "block";
          this.subSelect?.setOptions(this.getSubOptions("monthly"));
          this.subSelect?.setValue("1", false);
        }
        if (this.wrapHour) this.wrapHour.style.display = "block";
        if (this.wrapMinute) this.wrapMinute.style.display = "block";
      } else {
        // daily, workday, weekend
        this.wrapCustom?.classList.add("hidden");
        if (this.wrapSub) this.wrapSub.style.display = "none";
        if (this.wrapHour) this.wrapHour.style.display = "block";
        if (this.wrapMinute) this.wrapMinute.style.display = "block";
      }
    }

    syncSchedule() {
      const freq = this.freqSelect?.value || "daily";
      const hour = this.hourSelect?.value || "08";
      const min = this.minuteSelect?.value || "00";
      const sub = this.subSelect?.value || "1";

      const minNum = parseInt(min, 10);
      const hourNum = parseInt(hour, 10);

      let cron = "0 8 * * *";
      if (freq === "daily") {
        cron = `${minNum} ${hourNum} * * *`;
      } else if (freq === "workday") {
        cron = `${minNum} ${hourNum} * * 1-5`;
      } else if (freq === "weekend") {
        cron = `${minNum} ${hourNum} * * 6,0`;
      } else if (freq === "weekly") {
        cron = `${minNum} ${hourNum} * * ${sub}`;
      } else if (freq === "monthly") {
        cron = `${minNum} ${hourNum} ${sub} * *`;
      } else if (freq === "interval") {
        cron = sub === "1" ? "0 * * * *" : `0 */${sub} * * *`;
      } else if (freq === "custom") {
        cron = this.cronCustomInput?.value.trim() || "* * * * *";
      }

      if (this.cronHidden) this.cronHidden.value = cron;
      if (this.schedPreview) {
        this.schedPreview.textContent = Utils.describeCron(cron);
      }
    }

    parseCron(cronExpr) {
      const trimmed = String(cronExpr || "").trim();
      const pad = (n) => String(n).padStart(2, "0");

      if (trimmed === "0 * * * *") {
        this.freqSelect?.setValue("interval", false);
        this.updateScheduleVisibility("interval");
        this.subSelect?.setValue("1", false);
        this.syncSchedule();
        return;
      }

      const intMatch = trimmed.match(/^0\s+\*\/([0-9]+)\s+\*\s+\*\s+\*$/);
      if (intMatch) {
        this.freqSelect?.setValue("interval", false);
        this.updateScheduleVisibility("interval");
        this.subSelect?.setValue(intMatch[1], false);
        this.syncSchedule();
        return;
      }

      const dailyMatch = trimmed.match(/^([0-9]+)\s+([0-9]+)\s+\*\s+\*\s+\*$/);
      if (dailyMatch) {
        this.freqSelect?.setValue("daily", false);
        this.updateScheduleVisibility("daily");
        this.hourSelect?.setValue(pad(dailyMatch[2]), false);
        this.minuteSelect?.setValue(pad(dailyMatch[1]), false);
        this.syncSchedule();
        return;
      }

      const workdayMatch = trimmed.match(/^([0-9]+)\s+([0-9]+)\s+\*\s+\*\s+(?:1-5|1,2,3,4,5)$/);
      if (workdayMatch) {
        this.freqSelect?.setValue("workday", false);
        this.updateScheduleVisibility("workday");
        this.hourSelect?.setValue(pad(workdayMatch[2]), false);
        this.minuteSelect?.setValue(pad(workdayMatch[1]), false);
        this.syncSchedule();
        return;
      }

      const weekendMatch = trimmed.match(/^([0-9]+)\s+([0-9]+)\s+\*\s+\*\s+(?:6,0|0,6|6,7)$/);
      if (weekendMatch) {
        this.freqSelect?.setValue("weekend", false);
        this.updateScheduleVisibility("weekend");
        this.hourSelect?.setValue(pad(weekendMatch[2]), false);
        this.minuteSelect?.setValue(pad(weekendMatch[1]), false);
        this.syncSchedule();
        return;
      }

      const weekMatch = trimmed.match(/^([0-9]+)\s+([0-9]+)\s+\*\s+\*\s+([0-7])$/);
      if (weekMatch) {
        this.freqSelect?.setValue("weekly", false);
        this.updateScheduleVisibility("weekly");
        const dow = weekMatch[3] === "7" ? "0" : weekMatch[3];
        this.subSelect?.setValue(dow, false);
        this.hourSelect?.setValue(pad(weekMatch[2]), false);
        this.minuteSelect?.setValue(pad(weekMatch[1]), false);
        this.syncSchedule();
        return;
      }

      const monthMatch = trimmed.match(/^([0-9]+)\s+([0-9]+)\s+([0-9]+)\s+\*\s+\*$/);
      if (monthMatch) {
        this.freqSelect?.setValue("monthly", false);
        this.updateScheduleVisibility("monthly");
        this.subSelect?.setValue(monthMatch[3], false);
        this.hourSelect?.setValue(pad(monthMatch[2]), false);
        this.minuteSelect?.setValue(pad(monthMatch[1]), false);
        this.syncSchedule();
        return;
      }

      // Fallback: custom
      this.freqSelect?.setValue("custom", false);
      this.updateScheduleVisibility("custom");
      if (this.cronCustomInput) this.cronCustomInput.value = trimmed;
      this.syncSchedule();
    }

    applyTaskPreset(p) {
      this.nameInput.value = p.name;
      this.idInput.value = Utils.generateSlug(p.name, p.type);
      if (this.typeHidden) this.typeHidden.value = p.type;
      this.typeSelect?.setValue(p.type, false);
      this.descInput.value = p.desc;
      this.parseCron(p.cron);

      this.paramsTextarea.value = JSON.stringify(p.params, null, 2);
      this.renderDynamicParams(p.type, p.params);

      this.bus.emit("toast", `已套用 [${p.name}] 常用任务预设`);
    }

    open(item) {
      const isNew = !item;
      // Reset pills active states
      document.querySelectorAll(".sched-pill").forEach((b) => b.classList.remove("active"));

      const quickBanner = document.getElementById("task-quick-presets");

      if (isNew) {
        this.titleEl.textContent = "创建情报调度任务";
        this.hiddenId.value = "";
        this.idInput.value = "";
        this.idInput.disabled = false;
        this.deleteBtn?.classList.add("hidden");
        quickBanner?.classList.remove("hidden");

        this.nameInput.value = "";
        if (this.typeHidden) this.typeHidden.value = "morning";
        this.typeSelect?.setValue("morning", false);

        this.parseCron("0 8 * * *");

        if (this.enabledHidden) this.enabledHidden.value = "true";
        this.enabledSelect?.setValue("true", false);
        this.descInput.value = "汇总目标城市气象建议、每日格言、节点脉搏与临期资产";
        const initialParams = { city: "Shanghai" };
        this.paramsTextarea.value = JSON.stringify(initialParams, null, 2);

        this.renderDynamicParams("morning", initialParams);
      } else {
        this.titleEl.textContent = `编辑任务 · ${item.name}`;
        this.hiddenId.value = item.id;
        this.idInput.value = item.id;
        this.idInput.disabled = true;
        this.deleteBtn?.classList.remove("hidden");
        quickBanner?.classList.add("hidden");

        this.nameInput.value = item.name;
        if (this.typeHidden) this.typeHidden.value = item.type;
        this.typeSelect?.setValue(item.type, false);

        this.parseCron(item.cron_expr || "0 8 * * *");

        const enStr = item.enabled ? "true" : "false";
        if (this.enabledHidden) this.enabledHidden.value = enStr;
        this.enabledSelect?.setValue(enStr, false);
        this.descInput.value = item.description || "";

        const paramsObj = Utils.safeJSONParse(item.params);
        this.paramsTextarea.value = JSON.stringify(paramsObj, null, 2);
        this.renderDynamicParams(item.type, paramsObj);
      }

      this.overlay?.classList.remove("hidden");
    }

    close() {
      this.overlay?.classList.add("hidden");
    }

    renderDynamicParams(type, paramsObj = {}) {
      if (!this.dynParamsContainer) return;
      this.dynamicSelects.forEach((s) => s.destroy());
      this.dynamicSelects = [];

      const tpl = (this.store.state.templates || []).find((t) => t.type === type);
      if (!tpl || !tpl.params_schema || !tpl.params_schema.length) {
        if (this.wrapDynParams) this.wrapDynParams.style.display = "none";
        this.dynParamsContainer.innerHTML = "";
        return;
      }
      if (this.wrapDynParams) this.wrapDynParams.style.display = "";

      this.dynParamsContainer.innerHTML = tpl.params_schema.map((f) => {
        const val = paramsObj[f.key] !== undefined ? paramsObj[f.key] : (f.default_val || "");
        if (f.type === "select" && f.options && f.options.length) {
          return `
            <div class="form-field">
              <label>${Utils.esc(f.label)}</label>
              <div id="mount-task-dyn-${Utils.esc(f.key)}" class="v-select-mount"></div>
              <input type="hidden" class="task-dyn-input" data-key="${Utils.esc(f.key)}" value="${Utils.esc(val)}" />
              ${f.hint ? `<span class="field-hint">${Utils.esc(f.hint)}</span>` : ""}
            </div>
          `;
        }
        return `
          <div class="form-field">
            <label for="task-dyn-${Utils.esc(f.key)}">${Utils.esc(f.label)}</label>
            <input id="task-dyn-${Utils.esc(f.key)}" type="${f.type === "number" ? "number" : "text"}" class="glass-input mono task-dyn-input" data-key="${Utils.esc(f.key)}" value="${Utils.esc(val)}" placeholder="${Utils.esc(f.placeholder || "")}" />
            ${f.hint ? `<span class="field-hint">${Utils.esc(f.hint)}</span>` : ""}
          </div>
        `;
      }).join("");

      // Bind dynamic custom selects
      tpl.params_schema.forEach((f) => {
        if (f.type === "select" && f.options && f.options.length) {
          const mount = document.getElementById(`mount-task-dyn-${f.key}`);
          const hiddenInput = this.dynParamsContainer.querySelector(`.task-dyn-input[data-key="${f.key}"]`);
          const parsedOptions = f.options.map((opt) => Utils.parseOptionItem(opt));
          const rawVal = paramsObj[f.key] !== undefined ? String(paramsObj[f.key]) : (f.default_val || parsedOptions[0]?.value || "");
          if (rawVal && !parsedOptions.some((o) => String(o.value) === rawVal)) {
            parsedOptions.push({ label: `${rawVal} (当前值)`, value: rawVal });
          }
          if (hiddenInput) hiddenInput.value = rawVal;
          if (mount) {
            const cs = new CustomSelect(mount, {
              options: parsedOptions,
              value: rawVal,
              onChange: (newVal) => {
                if (hiddenInput) hiddenInput.value = newVal;
                this.syncInputsToJSON();
              },
            });
            this.dynamicSelects.push(cs);
          }
        }
      });

      this.dynParamsContainer.querySelectorAll("input.task-dyn-input:not([type=hidden])").forEach((input) => {
        input.addEventListener("input", () => this.syncInputsToJSON());
        input.addEventListener("change", () => this.syncInputsToJSON());
      });
    }

    syncInputsToJSON() {
      const currentParams = Utils.safeJSONParse(this.paramsTextarea.value);
      this.dynParamsContainer?.querySelectorAll(".task-dyn-input").forEach((el) => {
        const key = el.dataset.key;
        let val = el.value.trim();
        if (el.type === "number") {
          const num = Number(val);
          val = isNaN(num) ? val : num;
        }
        if (val !== "") {
          currentParams[key] = val;
        } else {
          delete currentParams[key];
        }
      });
      this.paramsTextarea.value = JSON.stringify(currentParams, null, 2);
    }
  }

  // --- System Backup Component ---
  class SystemBackupComponent extends BaseComponent {
    init() {
      this.exportBtn = document.getElementById("export-backup-btn");
      this.fileInput = document.getElementById("import-backup-file");
      this.rotateTokenBtn = document.getElementById("rotate-app-token-btn");

      this.rotateTokenBtn?.addEventListener("click", async () => {
        if (!confirm("确定重置移动端 App Bearer Token 吗？重置后手机端需重新配置新令牌。")) return;
        this.rotateTokenBtn.disabled = true;
        try {
          const res = await this.store.rotateAppToken();
          if (res?.token) {
            await navigator.clipboard?.writeText(res.token).catch(() => {});
            this.bus.emit("toast", "新 App Token 已生成并尝试复制到剪贴板");
          }
        } finally {
          this.rotateTokenBtn.disabled = false;
        }
      });

      this.exportBtn?.addEventListener("click", () => {
        this.store.exportBackup();
      });

      this.fileInput?.addEventListener("change", (e) => {
        const file = e.target.files?.[0];
        if (!file) return;

        const reader = new FileReader();
        reader.onload = async (evt) => {
          try {
            const parsed = JSON.parse(evt.target.result);
            if (!parsed || (!parsed.assets && !parsed.tasks && !parsed.configs)) {
              throw new Error("文件格式不合法，未检测到有效数据");
            }
            const countAssets = (parsed.assets || []).length;
            const countTasks = (parsed.tasks || []).length;
            if (!confirm(`检测到备份文件 (包含 ${countAssets} 个资产、${countTasks} 个任务)，确定导入并恢复覆盖吗？`)) {
              this.fileInput.value = "";
              return;
            }
            await this.store.importBackup(parsed);
          } catch (err) {
            this.bus.emit("toast", "导入解析失败: " + err.message);
          } finally {
            this.fileInput.value = "";
          }
        };
        reader.readAsText(file);
      });
    }
  }

  // --- Workspace Switcher Component ---
  class WorkspaceSwitcherComponent extends BaseComponent {
    init() {
      this.btns = document.querySelectorAll(".workspace-tab-btn");
      this.agentView = document.getElementById("workspace-agent");
      this.opsView = document.getElementById("workspace-ops");

      this.btns.forEach((btn) => {
        btn.addEventListener("click", () => {
          const target = btn.dataset.workspace;
          this.btns.forEach((b) => b.classList.toggle("active", b === btn));
          if (target === "ops") {
            this.agentView?.classList.add("hidden");
            this.opsView?.classList.remove("hidden");
          } else {
            this.agentView?.classList.remove("hidden");
            this.opsView?.classList.add("hidden");
          }
        });
      });
    }
  }

  // --- DeepSeek Agent Hub Component ---
  class AgentHubComponent extends BaseComponent {
    init() {
      this.sessionListEl = document.getElementById("agent-session-list");
      this.sessionSearchEl = document.getElementById("agent-session-search");
      this.activeTitleEl = document.getElementById("agent-active-title");
      this.activeSubEl = document.getElementById("agent-active-sub");
      this.sessionHidden = document.getElementById("agent-session-id");
      this.personaMount = document.getElementById("mount-agent-persona");
      this.personaHidden = document.getElementById("agent-persona");
      this.modelMount = document.getElementById("mount-agent-model");
      this.modelHidden = document.getElementById("agent-model");

      this.sessionSearchQuery = "";
      this.sessionSearchEl?.addEventListener("input", () => {
        this.sessionSearchQuery = (this.sessionSearchEl.value || "").trim().toLowerCase();
        this.syncSessionSelect(this.store.state.agentSessions || []);
      });

      this.exportSessionBtn = document.getElementById("agent-export-session-btn");
      this.newSessionBtn = document.getElementById("agent-new-session-btn");
      this.deleteSessionBtn = document.getElementById("agent-delete-session-btn");

      this.exportSessionBtn?.addEventListener("click", () => {
        const msgs = this.messagesEl?.querySelectorAll(".agent-msg");
        if (!msgs || !msgs.length) {
          this.bus.emit("toast", "当前会话暂无内容可导出");
          return;
        }
        const parts = [`# Vesper BotHub 会话导出\n\n- 导出时间: ${new Date().toISOString()}\n`];
        msgs.forEach((node) => {
          const isUser = node.classList.contains("agent-msg-user");
          const meta = node.querySelector(".agent-msg-meta")?.innerText?.replace(/\s+/g, " ").trim() || "";
          const content = node.querySelector(".agent-msg-content")?.innerText?.trim() || "";
          const reasoning = node.querySelector(".agent-reasoning-body")?.innerText?.trim() || "";
          parts.push(`\n---\n\n### ${isUser ? "管理员指令" : meta}\n`);
          if (reasoning) parts.push(`\n> 深度思考:\n> ${reasoning.replace(/\n/g, "\n> ")}\n`);
          if (content) parts.push(`\n${content}\n`);
        });
        const blob = new Blob([parts.join("")], { type: "text/markdown;charset=utf-8" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `bothub_chat_${Date.now()}.md`;
        a.click();
        URL.revokeObjectURL(url);
        this.bus.emit("toast", "会话已导出为 Markdown");
      });
      this.messagesEl = document.getElementById("agent-chat-messages");
      this.emptyEl = document.getElementById("agent-empty-state");
      this.form = document.getElementById("agent-chat-form");
      this.input = document.getElementById("agent-chat-input");
      this.sendBtn = document.getElementById("agent-send-btn");
      this.stopBtn = document.getElementById("agent-stop-btn");
      this.thinkingBtn = document.getElementById("agent-thinking-toggle");
      this.statusEl = document.getElementById("agent-stream-status");

      this.isStreaming = false;
      this.enableThinking = true;
      this.abortController = null;
      this.lastUserPrompt = "";
      this.initializedDefaults = false;

      this.thinkingBtn?.addEventListener("click", () => {
        this.enableThinking = !this.enableThinking;
        this.thinkingBtn.classList.toggle("active", this.enableThinking);
        this.thinkingBtn.textContent = this.enableThinking ? "深度思考: 开" : "深度思考: 关";
      });

      this.stopBtn?.addEventListener("click", () => {
        if (this.abortController) {
          this.abortController.abort();
        }
      });

      if (this.sessionMount) {
        this.sessionSelect = new CustomSelect(this.sessionMount, {
          options: [{ value: "", label: "新对话 (尚未开始)" }],
          value: "",
          onChange: (sid) => {
            if (this.sessionHidden) this.sessionHidden.value = sid;
            if (sid) {
              this.loadHistory(sid);
            } else {
              this.clearMessagesUI();
            }
          },
        });
      }

      if (this.personaMount) {
        this.personaSelect = new CustomSelect(this.personaMount, {
          options: [
            { value: "grok", label: "Grok 锐评模式 (高密度犀利结论)" },
            { value: "butler", label: "执更管家模式 (运维巡检与账单)" },
            { value: "architect", label: "架构师模式 (系统设计与根因分析)" },
          ],
          value: "grok",
          onChange: (val) => {
            if (this.personaHidden) this.personaHidden.value = val;
          },
        });
      }

      if (this.modelMount) {
        this.modelSelect = new CustomSelect(this.modelMount, {
          options: [
            { value: "deepseek-flash", label: "DeepSeek Flash (默认主力 · 支持思考)" },
            { value: "deepseek-v4-pro", label: "DeepSeek V4 Pro (深度主力推理)" },
          ],
          value: "deepseek-flash",
          onChange: (val) => {
            if (this.modelHidden) this.modelHidden.value = val;
          },
        });
      }

      this.newSessionBtn?.addEventListener("click", () => {
        if (this.isStreaming) return;
        if (this.sessionHidden) this.sessionHidden.value = "";
        this.sessionSelect?.setValue("", false);
        this.clearMessagesUI();
        this.input?.focus();
      });

      this.deleteSessionBtn?.addEventListener("click", async () => {
        if (this.isStreaming) return;
        const sid = this.sessionHidden?.value || "";
        if (!sid) {
          this.clearMessagesUI();
          return;
        }
        await this.store.deleteAgentSession(sid);
        if (this.sessionHidden) this.sessionHidden.value = "";
        this.clearMessagesUI();
      });

      document.querySelectorAll(".agent-prompt-pill").forEach((btn) => {
        btn.addEventListener("click", () => {
          if (this.isStreaming) return;
          const prompt = btn.dataset.prompt || "";
          if (prompt && this.input) {
            this.input.value = prompt;
            this.sendMessage();
          }
        });
      });

      this.input?.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.shiftKey) {
          e.preventDefault();
          this.sendMessage();
        }
      });

      this.form?.addEventListener("submit", (e) => {
        e.preventDefault();
        this.sendMessage();
      });

      this.bus.on("data:loaded", (state) => {
        const cfgs = state.configs || {};
        if (!this.initializedDefaults) {
          const defPersona = cfgs["active_persona"] || "grok";
          const defModel = cfgs["ai_model"] || "deepseek-flash";
          if (this.personaHidden) this.personaHidden.value = defPersona;
          this.personaSelect?.setValue(defPersona, false);
          if (this.modelHidden) this.modelHidden.value = defModel;
          this.modelSelect?.setValue(defModel, false);
          this.initializedDefaults = true;
        }
        this.syncSessionSelect(state.agentSessions || []);
      });
    }

    syncSessionSelect(sessions) {
      if (!this.sessionListEl) return;
      const cur = this.sessionHidden?.value || "";
      const filtered = (sessions || []).filter((s) => {
        if (!this.sessionSearchQuery) return true;
        return `${s.title || ""} ${s.persona || ""} ${s.model || ""}`.toLowerCase().includes(this.sessionSearchQuery);
      });

      if (!filtered.length) {
        this.sessionListEl.innerHTML = `<div class="empty-state" style="padding:20px 10px; font-size:12px;">${(sessions || []).length ? "无匹配会话" : "暂无历史会话，直接发消息开始"}</div>`;
        return;
      }

      this.sessionListEl.innerHTML = filtered.map((s) => {
        const isActive = s.id === cur;
        const avatarChar = s.persona === "butler" ? "执" : (s.persona === "architect" ? "架" : "G");
        const title = s.title || s.id;
        return `
          <div class="im-session-item ${isActive ? "active" : ""}" data-sid="${Utils.esc(s.id)}">
            <div class="im-session-avatar">${Utils.esc(avatarChar)}</div>
            <div class="im-session-info">
              <div class="im-session-title">${Utils.esc(title)}</div>
              <div class="im-session-meta mono">${Utils.esc(s.model || "deepseek-flash")} · ${Utils.esc(Utils.formatTime(s.updated_at))}</div>
            </div>
            <button type="button" class="im-session-del" data-del-sid="${Utils.esc(s.id)}" title="删除会话">x</button>
          </div>
        `;
      }).join("");

      this.sessionListEl.querySelectorAll(".im-session-item").forEach((row) => {
        row.addEventListener("click", (e) => {
          if (e.target.closest(".im-session-del") || this.isStreaming) return;
          const sid = row.dataset.sid || "";
          if (!sid) return;
          if (this.sessionHidden) this.sessionHidden.value = sid;
          this.syncSessionSelect(this.store.state.agentSessions || []);
          this.loadHistory(sid);
        });
      });

      this.sessionListEl.querySelectorAll(".im-session-del").forEach((btn) => {
        btn.addEventListener("click", async (e) => {
          e.stopPropagation();
          if (this.isStreaming) return;
          const sid = btn.dataset.delSid || "";
          if (!sid) return;
          await this.store.deleteAgentSession(sid);
          if (this.sessionHidden && this.sessionHidden.value === sid) {
            this.sessionHidden.value = "";
            this.clearMessagesUI();
          }
        });
      });
    }

    clearMessagesUI() {
      if (this.activeTitleEl) this.activeTitleEl.textContent = "新对话";
      if (this.activeSubEl) this.activeSubEl.textContent = "长庚执更 · DeepSeek 智能中枢";
      if (!this.messagesEl) return;
      this.messagesEl.innerHTML = "";
      if (this.emptyEl) {
        this.emptyEl.classList.remove("hidden");
        this.messagesEl.appendChild(this.emptyEl);
      }
      this.syncSessionSelect(this.store.state.agentSessions || []);
    }

    async loadHistory(sessionId) {
      try {
        const data = await this.store.loadAgentSession(sessionId);
        const sess = data.session;
        const msgs = data.messages || [];
        if (sess) {
          if (this.activeTitleEl) this.activeTitleEl.textContent = sess.title || "历史会话";
          if (this.activeSubEl) this.activeSubEl.textContent = `${sess.persona || "grok"} · ${sess.model || "deepseek-flash"}`;
          if (this.personaHidden) this.personaHidden.value = sess.persona || "grok";
          this.personaSelect?.setValue(sess.persona || "grok", false);
          if (this.modelHidden) this.modelHidden.value = sess.model || "deepseek-flash";
          this.modelSelect?.setValue(sess.model || "deepseek-flash", false);
        }

        if (!this.messagesEl) return;
        this.messagesEl.innerHTML = "";
        if (!msgs.length) {
          this.clearMessagesUI();
          return;
        }

        msgs.forEach((m) => {
          if (m.role === "user") {
            this.appendUserBubble(m.content, m.created_at);
          } else if (m.role === "assistant") {
            let tcs = [];
            try {
              tcs = JSON.parse(m.tool_calls || "[]");
            } catch {
              tcs = [];
            }
            if (m.content || m.reasoning_content || tcs.length) {
              const bubble = this.createAssistantBubble(sess?.persona || "grok", sess?.model || "deepseek-flash");
              if (m.reasoning_content) {
                bubble.setReasoning(m.reasoning_content);
              }
              tcs.forEach((tc) => {
                bubble.addToolCall(tc.id || "tool", tc.function?.name || "tool", tc.function?.arguments || "{}");
              });
              if (m.content) {
                bubble.setContent(m.content);
              }
            }
          }
        });
        this.scrollToBottom();
      } catch (err) {
        this.bus.emit("toast", "加载会话历史失败: " + err.message);
      }
    }

    appendUserBubble(text, timeStr) {
      if (this.emptyEl && this.emptyEl.parentNode === this.messagesEl) {
        this.emptyEl.remove();
      }
      const row = document.createElement("div");
      row.className = "agent-msg-row agent-msg-row-user";
      row.innerHTML = `
        <div class="im-avatar im-avatar-user">我</div>
        <div class="agent-msg agent-msg-user">
          <div class="agent-msg-meta mono">
            <span>管理员</span>
            <span>${Utils.esc(timeStr ? Utils.formatTime(timeStr) : "刚刚")}</span>
          </div>
          <div class="agent-msg-content">${Utils.esc(text)}</div>
        </div>
      `;
      this.messagesEl?.appendChild(row);
      this.scrollToBottom();
    }

    createAssistantBubble(persona, model) {
      if (this.emptyEl && this.emptyEl.parentNode === this.messagesEl) {
        this.emptyEl.remove();
      }
      const personaLabel = persona === "butler" ? "长庚执更管家" : (persona === "architect" ? "系统架构师" : "Grok 锐评中枢");
      const avatarChar = persona === "butler" ? "执" : (persona === "architect" ? "架" : "G");
      const row = document.createElement("div");
      row.className = "agent-msg-row agent-msg-row-assistant";
      row.innerHTML = `
        <div class="im-avatar im-avatar-ai">${Utils.esc(avatarChar)}</div>
        <div class="agent-msg agent-msg-assistant">
          <div class="agent-msg-meta mono">
            <span class="text-cyan">${Utils.esc(personaLabel)} · ${Utils.esc(model)}</span>
            <span class="agent-msg-state">生成中...</span>
          </div>
          <details class="agent-reasoning-box hidden" open>
            <summary class="mono">深度思考过程 (Reasoning Chain)</summary>
            <div class="agent-reasoning-body mono"></div>
          </details>
          <div class="agent-tools-list"></div>
          <div class="agent-msg-content"></div>
          <div class="agent-msg-actions hidden">
            <button type="button" class="agent-msg-action-btn agent-copy-btn">复制全文</button>
            <button type="button" class="agent-msg-action-btn agent-retry-btn">重新生成</button>
          </div>
        </div>
      `;
      this.messagesEl?.appendChild(row);
      const div = row.querySelector(".agent-msg");

      const stateSpan = div.querySelector(".agent-msg-state");
      const reasonDetails = div.querySelector(".agent-reasoning-box");
      const reasonBody = div.querySelector(".agent-reasoning-body");
      const toolsList = div.querySelector(".agent-tools-list");
      const contentBody = div.querySelector(".agent-msg-content");
      const actionsBar = div.querySelector(".agent-msg-actions");
      const copyBtn = div.querySelector(".agent-copy-btn");
      const retryBtn = div.querySelector(".agent-retry-btn");
      const toolEls = new Map();

      let reasonBuf = "";
      let contentBuf = "";

      const renderContentMarkdown = (text) => {
        if (!contentBody) return;
        contentBody.innerHTML = Utils.renderMarkdown(text);
        contentBody.querySelectorAll(".md-copy-code-btn").forEach((btn) => {
          btn.addEventListener("click", async () => {
            const code = btn.closest(".md-code-wrap")?.querySelector("pre code")?.textContent || "";
            try {
              await navigator.clipboard.writeText(code);
              btn.textContent = "已复制";
              setTimeout(() => { btn.textContent = "复制代码"; }, 1500);
            } catch {
              this.bus.emit("toast", "复制失败，请手动复制");
            }
          });
        });
      };

      copyBtn?.addEventListener("click", async () => {
        try {
          await navigator.clipboard.writeText(contentBuf);
          this.bus.emit("toast", "已复制回复全文");
        } catch {
          this.bus.emit("toast", "复制失败");
        }
      });

      retryBtn?.addEventListener("click", () => {
        if (this.isStreaming || !this.lastUserPrompt) return;
        if (this.input) {
          this.input.value = this.lastUserPrompt;
          this.sendMessage();
        }
      });

      return {
        appendReasoning: (delta) => {
          reasonBuf += delta;
          reasonDetails?.classList.remove("hidden");
          if (reasonBody) reasonBody.textContent = reasonBuf;
          this.scrollToBottom();
        },
        setReasoning: (full) => {
          reasonBuf = full;
          reasonDetails?.classList.remove("hidden");
          reasonDetails?.removeAttribute("open");
          if (reasonBody) reasonBody.textContent = full;
        },
        addToolCall: (id, name, args) => {
          const item = document.createElement("details");
          item.className = "agent-tool-card mono";
          item.innerHTML = `
            <summary>
              <span>调用工具: ${Utils.esc(name)}</span>
              <span class="tool-status-tag">执行中</span>
            </summary>
            <div class="agent-tool-detail">参数: ${Utils.esc(args)}</div>
          `;
          toolsList?.appendChild(item);
          toolEls.set(id, item);
          this.scrollToBottom();
        },
        updateToolResult: (id, name, result) => {
          const item = toolEls.get(id);
          if (!item) return;
          const statusTag = item.querySelector(".tool-status-tag");
          if (statusTag) statusTag.textContent = "已完成";
          const detail = item.querySelector(".agent-tool-detail");
          if (detail) {
            detail.textContent += `\n返回: ${result}`;
          }
        },
        appendContent: (delta) => {
          contentBuf += delta;
          renderContentMarkdown(contentBuf);
          this.scrollToBottom();
        },
        setContent: (full) => {
          contentBuf = full;
          renderContentMarkdown(full);
          if (stateSpan) stateSpan.textContent = "已完成";
          actionsBar?.classList.remove("hidden");
        },
        markDone: (usage) => {
          if (stateSpan) {
            stateSpan.textContent = usage && usage.total_tokens
              ? `已完成 (${usage.prompt_tokens}+${usage.completion_tokens} Tokens)`
              : "已完成";
          }
          if (reasonBuf) {
            reasonDetails?.removeAttribute("open");
          }
          if (contentBuf) {
            actionsBar?.classList.remove("hidden");
          }
        },
        markAborted: () => {
          if (stateSpan) stateSpan.textContent = "已手动停止";
          if (contentBuf) {
            actionsBar?.classList.remove("hidden");
          }
        },
        markError: (errMsg) => {
          if (stateSpan) {
            stateSpan.textContent = "异常中断";
            stateSpan.className = "agent-msg-state error-text";
          }
          if (!contentBuf && contentBody) {
            contentBody.textContent = `[调用失败] ${errMsg}`;
            contentBody.classList.add("error-text");
          }
        },
      };
    }

    scrollToBottom() {
      if (!this.messagesEl) return;
      this.messagesEl.scrollTop = this.messagesEl.scrollHeight;
    }

    async sendMessage() {
      if (this.isStreaming) return;
      const msg = (this.input?.value || "").trim();
      if (!msg) return;

      const sessionId = this.sessionHidden?.value || "";
      const persona = this.personaHidden?.value || "grok";
      const model = this.modelHidden?.value || "deepseek-flash";

      this.lastUserPrompt = msg;
      this.input.value = "";
      this.isStreaming = true;
      this.abortController = new AbortController();
      if (this.sendBtn) {
        this.sendBtn.disabled = true;
        this.sendBtn.textContent = "推理中...";
      }
      this.stopBtn?.classList.remove("hidden");
      if (this.statusEl) this.statusEl.textContent = "正在连接 DeepSeek 流式通道...";

      this.appendUserBubble(msg);
      const assistantUI = this.createAssistantBubble(persona, model);

      try {
        const res = await fetch("/api/agent/chat", {
          method: "POST",
          credentials: "same-origin",
          signal: this.abortController.signal,
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            session_id: sessionId,
            message: msg,
            persona,
            model,
            enable_thinking: this.enableThinking,
          }),
        });

        if (!res.ok) {
          const errData = await res.json().catch(() => ({}));
          throw new Error(errData.error || `HTTP ${res.status}`);
        }

        const reader = res.body.getReader();
        const decoder = new TextDecoder("utf-8");
        let buffer = "";
        let currentEvent = "message";

        while (true) {
          const { value, done } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });

          const lines = buffer.split("\n");
          buffer = lines.pop() || "";

          for (const rawLine of lines) {
            const line = rawLine.trim();
            if (!line) continue;
            if (line.startsWith("event:")) {
              currentEvent = line.slice(6).trim();
            } else if (line.startsWith("data:")) {
              const jsonStr = line.slice(5).trim();
              let payload = {};
              try {
                payload = JSON.parse(jsonStr);
              } catch {
                continue;
              }
              this.handleSSEEvent(currentEvent, payload, assistantUI);
            }
          }
        }
        assistantUI.markDone();
        if (this.statusEl) this.statusEl.textContent = "就绪";
        await this.store.loadDashboard();
      } catch (err) {
        if (err.name === "AbortError") {
          assistantUI.markAborted();
          if (this.statusEl) this.statusEl.textContent = "已手动停止";
          this.bus.emit("toast", "已停止生成");
        } else {
          assistantUI.markError(err.message);
          if (this.statusEl) this.statusEl.textContent = "发生错误";
          this.bus.emit("toast", err.message);
        }
      } finally {
        this.isStreaming = false;
        this.abortController = null;
        this.stopBtn?.classList.add("hidden");
        if (this.sendBtn) {
          this.sendBtn.disabled = false;
          this.sendBtn.textContent = "发送指令";
        }
      }
    }

    handleSSEEvent(event, payload, assistantUI) {
      if (event === "session") {
        if (payload.session_id && this.sessionHidden) {
          this.sessionHidden.value = payload.session_id;
        }
        if (payload.title && this.activeTitleEl) {
          this.activeTitleEl.textContent = payload.title;
        }
      } else if (event === "reasoning") {
        if (this.statusEl) this.statusEl.textContent = "深度思考中 (Reasoner)...";
        assistantUI.appendReasoning(payload.delta || "");
      } else if (event === "tool_start") {
        if (this.statusEl) this.statusEl.textContent = `执行工具: ${payload.name}...`;
        assistantUI.addToolCall(payload.id, payload.name, payload.arguments);
      } else if (event === "tool_result") {
        assistantUI.updateToolResult(payload.id, payload.name, payload.result);
      } else if (event === "content") {
        if (this.statusEl) this.statusEl.textContent = "正在生成回复...";
        assistantUI.appendContent(payload.delta || "");
      } else if (event === "error") {
        assistantUI.markError(payload.error || "未知流式错误");
      } else if (event === "done") {
        assistantUI.markDone(payload.usage);
      }
    }
  }

  // --- Long-Term Memories Modal Component ---
  class AgentMemoriesModalComponent extends BaseComponent {
    init() {
      this.overlay = document.getElementById("memories-modal-overlay");
      this.openBtn = document.getElementById("open-memories-btn");
      this.closeBtn = document.getElementById("close-memories-modal-btn");
      this.listEl = document.getElementById("memories-list");
      this.form = document.getElementById("memory-add-form");
      this.catMount = document.getElementById("mount-memory-category");
      this.catHidden = document.getElementById("mem-category");
      this.keyInput = document.getElementById("mem-fact-key");
      this.contentInput = document.getElementById("mem-content");

      if (this.catMount) {
        this.catSelect = new CustomSelect(this.catMount, {
          options: [
            { value: "infra", label: "基础设施与节点 (infra)" },
            { value: "preference", label: "用户偏好与交互习惯 (preference)" },
            { value: "schedule", label: "日程与运维约定 (schedule)" },
            { value: "general", label: "通用背景事实 (general)" },
          ],
          value: "infra",
          onChange: (val) => {
            if (this.catHidden) this.catHidden.value = val;
          },
        });
      }

      this.openBtn?.addEventListener("click", () => this.open());
      this.closeBtn?.addEventListener("click", () => this.close());
      this.overlay?.addEventListener("click", (e) => {
        if (e.target === this.overlay) this.close();
      });

      this.form?.addEventListener("submit", async (e) => {
        e.preventDefault();
        const category = (this.catHidden?.value || "infra").trim();
        const fact_key = (this.keyInput?.value || "").trim();
        const content = (this.contentInput?.value || "").trim();
        if (!fact_key || !content) return;
        await this.store.saveAgentMemory({ category, fact_key, content });
        if (this.keyInput) this.keyInput.value = "";
        if (this.contentInput) this.contentInput.value = "";
      });

      this.bus.on("data:loaded", (state) => {
        this.render(state.agentMemories || []);
      });
    }

    open() {
      this.overlay?.classList.remove("hidden");
    }

    close() {
      this.overlay?.classList.add("hidden");
    }

    render(memories) {
      if (!this.listEl) return;
      if (!memories || !memories.length) {
        this.listEl.innerHTML = `<div class="empty-state">尚未写入长期记忆条目，可在上方手动添加或在对话中要求 Agent 记住关键事实</div>`;
        return;
      }

      this.listEl.innerHTML = memories.map((m) => `
        <article class="log-item">
          <div class="log-top">
            <div style="display:flex; align-items:center; gap:8px; flex-wrap:wrap;">
              <span class="cat-badge cat-compute mono">${Utils.esc(m.category)}</span>
              <strong class="mono text-cyan">${Utils.esc(m.fact_key)}</strong>
            </div>
            <div style="display:flex; align-items:center; gap:8px;">
              <span class="mono" style="font-size:11px; color:var(--text-muted);">${Utils.esc(Utils.formatTime(m.updated_at))}</span>
              <button type="button" class="btn btn-danger btn-sm del-mem-btn" data-id="${Utils.esc(m.id)}">移除</button>
            </div>
          </div>
          <div style="font-size:13px; color:var(--text-main); margin-top:4px;">${Utils.esc(m.content)}</div>
        </article>
      `).join("");

      this.listEl.querySelectorAll(".del-mem-btn").forEach((btn) => {
        btn.addEventListener("click", async () => {
          const id = btn.dataset.id;
          if (!id) return;
          await this.store.deleteAgentMemory(id);
        });
      });
    }
  }

  // --- Native Notifications Inbox Modal Component ---
  class InboxModalComponent extends BaseComponent {
    init() {
      this.overlay = document.getElementById("inbox-modal-overlay");
      this.openBtn = document.getElementById("open-inbox-btn");
      this.closeBtn = document.getElementById("close-inbox-modal-btn");
      this.readAllBtn = document.getElementById("inbox-read-all-btn");
      this.desktopNotifyBtn = document.getElementById("inbox-desktop-notify-btn");
      this.badgeEl = document.getElementById("inbox-unread-badge");

      const syncNotifyBtn = () => {
        if (!this.desktopNotifyBtn) return;
        const enabled = localStorage.getItem("bothub_desktop_notify") === "1" && typeof Notification !== "undefined" && Notification.permission === "granted";
        this.desktopNotifyBtn.textContent = enabled ? "桌面弹窗: 开" : "桌面弹窗: 关";
      };
      syncNotifyBtn();

      this.desktopNotifyBtn?.addEventListener("click", async () => {
        if (typeof Notification === "undefined") {
          this.bus.emit("toast", "当前浏览器不支持桌面通知 API");
          return;
        }
        const cur = localStorage.getItem("bothub_desktop_notify") === "1";
        if (cur) {
          localStorage.setItem("bothub_desktop_notify", "0");
          syncNotifyBtn();
          this.bus.emit("toast", "已关闭桌面告警弹窗");
          return;
        }
        const perm = Notification.permission === "granted" ? "granted" : await Notification.requestPermission();
        if (perm === "granted") {
          localStorage.setItem("bothub_desktop_notify", "1");
          this.bus.emit("toast", "已开启后台高优告警桌面弹窗");
        } else {
          this.bus.emit("toast", "浏览器未授予通知权限");
        }
        syncNotifyBtn();
      });
      this.listEl = document.getElementById("inbox-list");
      this.filterBtns = document.querySelectorAll("#inbox-filter-bar .pill-btn");

      this.activeFilter = "all";

      this.openBtn?.addEventListener("click", () => this.open());
      this.closeBtn?.addEventListener("click", () => this.close());
      this.overlay?.addEventListener("click", (e) => {
        if (e.target === this.overlay) this.close();
      });

      this.readAllBtn?.addEventListener("click", async () => {
        await this.store.markAllNotificationsRead();
      });

      this.filterBtns.forEach((btn) => {
        btn.addEventListener("click", () => {
          this.activeFilter = btn.dataset.filter || "all";
          this.filterBtns.forEach((b) => b.classList.toggle("active", b === btn));
          this.render(this.store.state.notifications || [], this.store.state.unreadCount || 0);
        });
      });

      this.bus.on("data:loaded", (state) => {
        this.render(state.notifications || [], state.unreadCount || 0);
      });
      this.bus.on("notifications:updated", (state) => {
        this.render(state.notifications || [], state.unreadCount || 0);
      });
    }

    open() {
      this.overlay?.classList.remove("hidden");
    }

    close() {
      this.overlay?.classList.add("hidden");
    }

    matchesFilter(source) {
      if (this.activeFilter === "all") return true;
      const s = String(source || "").toLowerCase();
      if (this.activeFilter === "morning") return s.includes("morning");
      if (this.activeFilter === "radar") return s.includes("radar") || s.includes("bili");
      if (this.activeFilter === "asset") return s.includes("asset");
      if (this.activeFilter === "alert") return s.includes("nezha") || s.includes("alert") || s.includes("webhook");
      return true;
    }

    render(notifications, unreadCount) {
      if (this.badgeEl) {
        if (unreadCount > 0) {
          this.badgeEl.textContent = unreadCount > 99 ? "99+" : String(unreadCount);
          this.badgeEl.classList.remove("hidden");
        } else {
          this.badgeEl.classList.add("hidden");
        }
      }
      if (!this.listEl) return;

      const filtered = (notifications || []).filter((n) => this.matchesFilter(n.source));
      if (!filtered.length) {
        this.listEl.innerHTML = `<div class="empty-state">当前分类下暂无情报消息</div>`;
        return;
      }

      this.listEl.innerHTML = filtered.map((n) => {
        const isUnread = !n.read_at;
        const prioClass = n.priority === "high" ? "cat-vps" : "cat-compute";
        return `
          <article class="log-item ${isUnread ? "inbox-item-unread" : ""}" data-nid="${Utils.esc(n.id)}">
            <div class="log-top">
              <div style="display:flex; align-items:center; gap:8px; flex-wrap:wrap;">
                <span class="cat-badge ${prioClass} mono">${Utils.esc(n.source)}</span>
                <strong>${Utils.esc(n.title)}</strong>
                ${isUnread ? '<span class="mono text-amber" style="font-size:11px;">[未读]</span>' : ""}
              </div>
              <span class="mono" style="font-size:11px; color:var(--text-muted);">${Utils.esc(Utils.formatTime(n.created_at))}</span>
            </div>
            <div style="font-size:12px; color:var(--text-muted); margin-top:4px;">${Utils.esc(n.summary)}</div>
            <details class="inbox-details" style="margin-top:6px;">
              <summary class="mono" style="cursor:pointer; font-size:11px; color:var(--accent-cyan);">展开全文</summary>
              <div class="inbox-item-body">${Utils.renderMarkdown(n.markdown || n.summary)}</div>
            </details>
          </article>
        `;
      }).join("");

      this.listEl.querySelectorAll(".inbox-details").forEach((det) => {
        det.addEventListener("toggle", () => {
          if (det.open) {
            const nid = Number(det.closest(".log-item")?.dataset.nid || 0);
            if (nid > 0) {
              this.store.markNotificationRead(nid).catch(() => {});
            }
          }
        });
      });
    }
  }

  // ==========================================
  // 8. Application Bootstrap
  // ==========================================
  function bootstrap() {
    const bus = new EventBus();
    const store = new AppStore(bus);

    const components = [
      new ToastComponent("#toast", store, bus),
      new AuthComponent("#login-view", store, bus),
      new TopbarComponent("header.topbar", store, bus),
      new MetricsComponent(".metrics-grid", store, bus),
      new WorkspaceSwitcherComponent(".workspace-tabs", store, bus),
      new AgentHubComponent("#workspace-agent", store, bus),
      new AgentMemoriesModalComponent("#memories-modal-overlay", store, bus),
      new InboxModalComponent("#inbox-modal-overlay", store, bus),
      new AssetsSectionComponent("#assets-grid", store, bus),
      new TasksSectionComponent("#tasks-grid", store, bus),
      new GlobalConfigComponent("#config-form", store, bus),
      new PasswordComponent("#password-form", store, bus),
      new PushLogsComponent("#logs-list", store, bus),
      new AssetModalComponent("#asset-modal-overlay", store, bus),
      new TaskModalComponent("#task-modal-overlay", store, bus),
      new SystemBackupComponent(".backup-actions-row", store, bus),
    ];

    components.forEach((c) => c.init());
    store.checkAuth();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", bootstrap);
  } else {
    bootstrap();
  }
})();