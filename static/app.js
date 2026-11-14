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

    formatTime(str) {
      if (!str) return "--";
      const d = new Date(str);
      if (isNaN(d.getTime())) return str;
      const pad = (n) => String(n).padStart(2, "0");
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
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
        version: "v1.5.0",
        commit: "dev",
        buildTime: "local",
        uptimeSec: 0,
        serverNow: "--",
        stats: { total_assets: 0, sent_pushes: 0, seen_items: 0 },
        assets: [],
        tasks: [],
        templates: [],
        assetTemplates: [],
        configs: {},
        pushLogs: [],
      };
    }

    async checkAuth() {
      try {
        const data = await this.api.request("/api/auth/status");
        this.state.authenticated = !!data.authenticated;
        this.bus.emit("auth:state", { authenticated: this.state.authenticated });
        if (this.state.authenticated) {
          await this.loadDashboard();
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
      return data;
    }

    async logout() {
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
        this.state.configs = data.configs || {};
        this.state.pushLogs = data.push_logs || [];

        this.bus.emit("data:loaded", this.state);
      } catch (err) {
        this.bus.emit("toast", err.message);
      }
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
        if (this.verLabel) this.verLabel.textContent = state.version;
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
      this.statSent = document.getElementById("stat-sent");
      this.statSeen = document.getElementById("stat-seen");
      this.statTime = document.getElementById("stat-time");

      this.bus.on("data:loaded", (state) => {
        if (this.statAssets) this.statAssets.textContent = state.stats.total_assets ?? 0;
        if (this.statSent) this.statSent.textContent = state.stats.sent_pushes ?? 0;
        if (this.statSeen) this.statSeen.textContent = state.stats.seen_items ?? 0;
        if (this.statTime) this.statTime.textContent = state.serverNow || "--";
      });
    }
  }

  // --- Assets Section Component ---
  class AssetsSectionComponent extends BaseComponent {
    init() {
      this.grid = document.getElementById("assets-grid");
      this.addBtn = document.getElementById("add-asset-btn");
      this.syncBtn = document.getElementById("sync-assets-btn");

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
      if (!assets || !assets.length) {
        this.grid.innerHTML = `<div class="empty-state">暂无追踪中的资产</div>`;
        return;
      }

      this.grid.innerHTML = assets.map((a) => {
        const catClass = a.category === "compute" ? "cat-compute" : (a.category === "domain" ? "cat-domain" : "cat-life");
        const catName = a.category === "compute" ? "算力基建" : (a.category === "domain" ? "核心域名" : "生活订阅");
        const daysClass = a.days_left <= 3 ? "days-danger" : (a.days_left <= 7 ? "days-warn" : "days-safe");
        const daysText = a.days_left < 0 ? `逾期 ${Math.abs(a.days_left)} 天` : `剩余 ${a.days_left} 天`;
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
              <div class="meta-box"><span>状态</span><strong>${Utils.esc(a.status || "--")}</strong></div>
              <div class="meta-box" style="grid-column: 1 / -1;"><span>到期时间</span><strong>${Utils.esc(Utils.formatTime(a.due_date))}</strong></div>
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
                <span class="text-muted" style="font-size:10px; display:block; margin-top:2px;">[${Utils.esc(t.cron_expr || "--")}]</span>
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

  // --- Global Config Component ---
  class GlobalConfigComponent extends BaseComponent {
    init() {
      this.form = document.getElementById("config-form");
      this.pushplusInput = document.getElementById("cfg-pushplus-token");
      this.cfInput = document.getElementById("cfg-cf-api-token");
      this.saveBtn = document.getElementById("save-config-btn");

      this.bus.on("data:loaded", (state) => {
        if (this.pushplusInput) this.pushplusInput.value = state.configs["pushplus_token"] || "";
        if (this.cfInput) this.cfInput.value = state.configs["cf_api_token"] || "";
      });

      this.form?.addEventListener("submit", async (e) => {
        e.preventDefault();
        const payload = {
          pushplus_token: (this.pushplusInput?.value || "").trim(),
          cf_api_token: (this.cfInput?.value || "").trim(),
        };
        this.saveBtn.disabled = true;
        try {
          await this.store.saveConfig(payload);
        } finally {
          this.saveBtn.disabled = false;
        }
      });
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

  // --- Push Logs Component ---
  class PushLogsComponent extends BaseComponent {
    init() {
      this.list = document.getElementById("logs-list");
      this.expandedLogIds = new Set();
      this.bus.on("data:loaded", (state) => this.render(state.pushLogs));
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

      // Warn days Presets
      document.querySelectorAll(".warn-pill").forEach((btn) => {
        btn.addEventListener("click", () => {
          document.querySelectorAll(".warn-pill").forEach((b) => b.classList.remove("active"));
          btn.classList.add("active");
          if (this.warnDaysInput) {
            this.warnDaysInput.value = btn.dataset.preset;
            this.updateWarnDaysHint(btn.dataset.preset);
          }
        });
      });
      this.warnDaysInput?.addEventListener("input", () => {
        this.updateWarnDaysHint(this.warnDaysInput.value);
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

        const payload = {
          id: finalID,
          name: this.nameInput.value.trim(),
          category: this.catHidden ? this.catHidden.value : "compute",
          spec: this.specInput.value.trim(),
          price: this.priceHidden ? this.priceHidden.value.trim() : "",
          due_date: this.dueHidden ? this.dueHidden.value.trim() : "",
          status: this.statusHidden ? this.statusHidden.value.trim() : "运行中",
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

    updateWarnDaysHint(val) {
      if (!this.warnDaysHint) return;
      try {
        const arr = JSON.parse(val);
        if (Array.isArray(arr) && arr.length) {
          const formatted = arr.map((x) => (x === 0 ? "当天" : `${x}天`)).join("、");
          this.warnDaysHint.textContent = `将在到期前 ${formatted} 推送提醒`;
          return;
        }
      } catch (e) {}
      this.warnDaysHint.textContent = "将在设定梯度触发微信提醒";
    }

    open(item) {
      const isNew = !item;
      // Reset pills active states
      document.querySelectorAll(".date-pill-group .date-pill").forEach((b) => b.classList.remove("active"));
      document.querySelectorAll(".warn-pill").forEach((b) => b.classList.remove("active"));

      if (isNew) {
        this.titleEl.textContent = "新增数字资产与订阅";
        this.hiddenId.value = "";
        this.idInput.value = "";
        this.idInput.disabled = false;
        this.deleteBtn?.classList.add("hidden");

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

        this.warnDaysInput.value = "[7, 3, 1]";
        this.updateWarnDaysHint("[7, 3, 1]");
        this.extraTextarea.value = "{}";

        this.renderDynamicFields("compute", {});
      } else {
        this.titleEl.textContent = `编辑资产 · ${item.name}`;
        this.hiddenId.value = item.id;
        this.idInput.value = item.id;
        this.idInput.disabled = true;
        this.deleteBtn?.classList.remove("hidden");

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

        this.warnDaysInput.value = item.warn_days || "[7, 3, 1]";
        this.updateWarnDaysHint(this.warnDaysInput.value);

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
            </div>
          `;
        }
        return `
          <div class="form-field">
            <label for="asset-dyn-${Utils.esc(f.key)}">${Utils.esc(f.label)}</label>
            <input id="asset-dyn-${Utils.esc(f.key)}" type="${f.type === "number" ? "number" : "text"}" class="glass-input mono asset-dyn-input" data-key="${Utils.esc(f.key)}" value="${Utils.esc(val)}" placeholder="${Utils.esc(f.placeholder || "")}" />
          </div>
        `;
      }).join("");

      // Bind dynamic custom selects
      tpl.fields.forEach((f) => {
        if (f.type === "select" && f.options && f.options.length) {
          const mount = document.getElementById(`mount-asset-dyn-${f.key}`);
          const hiddenInput = this.dynFieldsContainer.querySelector(`.asset-dyn-input[data-key="${f.key}"]`);
          const val = extraConfigObj[f.key] !== undefined ? extraConfigObj[f.key] : (f.default_val || f.options[0]);
          if (mount) {
            const cs = new CustomSelect(mount, {
              options: f.options.map((opt) => ({ label: opt, value: opt })),
              value: val,
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

        const payload = {
          id: finalID,
          name: this.nameInput.value.trim(),
          type: this.typeHidden ? this.typeHidden.value : "morning",
          cron_expr: this.cronHidden?.value.trim() || "0 8 * * *",
          enabled: this.enabledHidden ? this.enabledHidden.value === "true" : true,
          description: this.descInput.value.trim(),
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

    open(item) {
      const isNew = !item;
      // Reset pills active states
      document.querySelectorAll(".sched-pill").forEach((b) => b.classList.remove("active"));

      if (isNew) {
        this.titleEl.textContent = "创建情报调度任务";
        this.hiddenId.value = "";
        this.idInput.value = "";
        this.idInput.disabled = false;
        this.deleteBtn?.classList.add("hidden");

        this.nameInput.value = "";
        if (this.typeHidden) this.typeHidden.value = "morning";
        this.typeSelect?.setValue("morning", false);

        this.parseCron("0 8 * * *");

        if (this.enabledHidden) this.enabledHidden.value = "true";
        this.enabledSelect?.setValue("true", false);
        this.descInput.value = "汇总目标城市气象建议、每日格言、节点脉搏与临期资产";
        const initialParams = { city: "Shanghai", nezha_url: "https://dash.starryheavens.uk" };
        this.paramsTextarea.value = JSON.stringify(initialParams, null, 2);

        this.renderDynamicParams("morning", initialParams);
      } else {
        this.titleEl.textContent = `编辑任务 · ${item.name}`;
        this.hiddenId.value = item.id;
        this.idInput.value = item.id;
        this.idInput.disabled = true;
        this.deleteBtn?.classList.remove("hidden");

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
        this.dynParamsContainer.innerHTML = `<div class="empty-state" style="padding:6px; font-size:11px;">该模板无需额外业务参数</div>`;
        return;
      }

      this.dynParamsContainer.innerHTML = tpl.params_schema.map((f) => {
        const val = paramsObj[f.key] !== undefined ? paramsObj[f.key] : (f.default_val || "");
        if (f.type === "select" && f.options && f.options.length) {
          return `
            <div class="form-field">
              <label>${Utils.esc(f.label)}</label>
              <div id="mount-task-dyn-${Utils.esc(f.key)}" class="v-select-mount"></div>
              <input type="hidden" class="task-dyn-input" data-key="${Utils.esc(f.key)}" value="${Utils.esc(val)}" />
            </div>
          `;
        }
        return `
          <div class="form-field">
            <label for="task-dyn-${Utils.esc(f.key)}">${Utils.esc(f.label)}</label>
            <input id="task-dyn-${Utils.esc(f.key)}" type="${f.type === "number" ? "number" : "text"}" class="glass-input mono task-dyn-input" data-key="${Utils.esc(f.key)}" value="${Utils.esc(val)}" placeholder="${Utils.esc(f.placeholder || "")}" />
          </div>
        `;
      }).join("");

      // Bind dynamic custom selects
      tpl.params_schema.forEach((f) => {
        if (f.type === "select" && f.options && f.options.length) {
          const mount = document.getElementById(`mount-task-dyn-${f.key}`);
          const hiddenInput = this.dynParamsContainer.querySelector(`.task-dyn-input[data-key="${f.key}"]`);
          const val = paramsObj[f.key] !== undefined ? paramsObj[f.key] : (f.default_val || f.options[0]);
          if (mount) {
            const cs = new CustomSelect(mount, {
              options: f.options.map((opt) => ({ label: opt, value: opt })),
              value: val,
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
      new AssetsSectionComponent("#assets-grid", store, bus),
      new TasksSectionComponent("#tasks-grid", store, bus),
      new GlobalConfigComponent("#config-form", store, bus),
      new PasswordComponent("#password-form", store, bus),
      new PushLogsComponent("#logs-list", store, bus),
      new AssetModalComponent("#asset-modal-overlay", store, bus),
      new TaskModalComponent("#task-modal-overlay", store, bus),
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