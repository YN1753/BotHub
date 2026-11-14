(() => {
  const loginView = document.getElementById("login-view");
  const dashView = document.getElementById("dashboard-view");
  const toastEl = document.getElementById("toast");
  const modalOverlay = document.getElementById("asset-modal-overlay");
  const taskModalOverlay = document.getElementById("task-modal-overlay");
  let toastTimer = null;
  let cachedAssets = [];
  let cachedTasks = [];
  let cachedTemplates = [];

  function showToast(msg) {
    toastEl.textContent = msg;
    toastEl.classList.remove("hidden");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => toastEl.classList.add("hidden"), 3000);
  }

  function esc(s) {
    return String(s ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;");
  }

  function formatTime(str) {
    if (!str) return "--";
    const d = new Date(str);
    if (isNaN(d.getTime())) return str;
    const pad = (n) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }

  async function api(path, opts = {}) {
    const res = await fetch(path, {
      credentials: "same-origin",
      headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
      ...opts,
    });
    const data = await res.json().catch(() => ({}));
    if (res.status === 401) {
      dashView.classList.add("hidden");
      loginView.classList.remove("hidden");
      throw new Error(data.error || "未登录或会话已过期");
    }
    if (!res.ok) throw new Error(data.error || `请求失败 (${res.status})`);
    return data;
  }

  async function checkAuth() {
    try {
      const data = await api("/api/auth/status");
      if (data.authenticated) {
        loginView.classList.add("hidden");
        dashView.classList.remove("hidden");
        await loadDashboard();
      } else {
        dashView.classList.add("hidden");
        loginView.classList.remove("hidden");
      }
    } catch {
      dashView.classList.add("hidden");
      loginView.classList.remove("hidden");
    }
  }

  async function loadDashboard() {
    try {
      const data = await api("/api/dashboard");
      cachedTemplates = data.templates || [];
      renderStats(data);
      renderAssets(data.assets || []);
      renderTasks(data.tasks || data.modules || []);
      fillConfig(data.configs || {});
      renderLogs(data.push_logs || []);
    } catch (err) {
      showToast(err.message);
    }
  }

  function renderStats(data) {
    if (data.version) document.getElementById("version-label").textContent = data.version;
    const s = data.stats || {};
    document.getElementById("stat-assets").textContent = s.total_assets ?? 0;
    document.getElementById("stat-sent").textContent = s.sent_pushes ?? 0;
    document.getElementById("stat-seen").textContent = s.seen_items ?? 0;
    document.getElementById("stat-time").textContent = data.server_now || "--";
    const u = Number(data.uptime_sec) || 0;
    document.getElementById("uptime-label").textContent = u < 60 ? `UPTIME ${u}s` : `UPTIME ${Math.floor(u / 60)}m`;
  }

  function renderAssets(assets) {
    cachedAssets = assets;
    const grid = document.getElementById("assets-grid");
    if (!assets.length) {
      grid.innerHTML = `<div class="empty-state">暂无追踪中的资产</div>`;
      return;
    }

    grid.innerHTML = assets.map((a) => {
      const catClass = a.category === "compute" ? "cat-compute" : (a.category === "domain" ? "cat-domain" : "cat-life");
      const catName = a.category === "compute" ? "算力基建" : (a.category === "domain" ? "核心域名" : "生活订阅");
      const daysClass = a.days_left <= 3 ? "days-danger" : (a.days_left <= 7 ? "days-warn" : "days-safe");
      const daysText = a.days_left < 0 ? `逾期 ${Math.abs(a.days_left)} 天` : `剩余 ${a.days_left} 天`;
      const isTrial = (a.price && a.price.includes("活动")) || (a.status && a.status.includes("活动")) || a.id === "qq_music";

      return `
        <article class="glass-card asset-card">
          <div class="asset-head">
            <div>
              <h3 class="serif-title asset-name">${esc(a.name)}</h3>
              <p class="asset-spec mono">${esc(a.spec || a.category)}</p>
            </div>
            <span class="cat-badge ${catClass}">${esc(catName)}</span>
          </div>
          <div class="asset-bar">
            <span>扣费/到期倒计时</span>
            <span class="days-pill ${daysClass}">${esc(daysText)}</span>
          </div>
          <div class="asset-meta-grid mono">
            <div class="meta-box"><span>周期金额</span><strong>${esc(a.price || "--")}</strong></div>
            <div class="meta-box"><span>状态</span><strong>${esc(a.status || "--")}</strong></div>
            <div class="meta-box" style="grid-column: 1 / -1;"><span>到期时间</span><strong>${esc(formatTime(a.due_date))}</strong></div>
          </div>
          ${isTrial ? `<div class="trial-alert">[防免密扣款] 活动体验到期前请确认是否已取消自动扣款</div>` : ""}
          <div class="asset-foot">
            <span class="${a.auto_renew ? "renew-on" : ""}">自动续费: ${a.auto_renew ? "开启" : "关闭"}</span>
            <button class="btn btn-ghost btn-sm edit-btn" data-id="${esc(a.id)}">配置/凭证</button>
          </div>
        </article>
      `;
    }).join("");

    grid.querySelectorAll(".edit-btn").forEach((btn) => {
      btn.onclick = () => {
        const item = cachedAssets.find((x) => x.id === btn.dataset.id);
        if (!item) return;
        document.getElementById("modal-asset-title").textContent = `编辑资产 · ${item.name}`;
        document.getElementById("edit-asset-id").value = item.id;
        const idInput = document.getElementById("edit-asset-id-input");
        idInput.value = item.id;
        idInput.disabled = true;
        document.getElementById("delete-asset-btn").classList.remove("hidden");

        document.getElementById("edit-asset-name").value = item.name;
        document.getElementById("edit-asset-category").value = item.category || "life";
        document.getElementById("edit-asset-spec").value = item.spec || "";
        document.getElementById("edit-asset-price").value = item.price || "";
        document.getElementById("edit-asset-due").value = item.due_date || "";
        document.getElementById("edit-asset-status").value = item.status || "";
        document.getElementById("edit-asset-autorenew").value = item.auto_renew ? "true" : "false";
        document.getElementById("edit-asset-warndays").value = item.warn_days || "[7, 3, 1]";
        try {
          document.getElementById("edit-asset-extra").value = JSON.stringify(JSON.parse(item.extra_config || "{}"), null, 2);
        } catch {
          document.getElementById("edit-asset-extra").value = item.extra_config || "{}";
        }
        modalOverlay.classList.remove("hidden");
      };
    });
  }

  function renderTasks(tasks) {
    cachedTasks = tasks;
    const grid = document.getElementById("tasks-grid") || document.getElementById("modules-grid");
    if (!grid) return;
    if (!tasks.length) {
      grid.innerHTML = `<div class="empty-state">暂无已注册的任务模板与实例</div>`;
      return;
    }

    grid.innerHTML = tasks.map((t) => {
      const last = t.last_run;
      const dot = !t.enabled ? "dot-warn" : (t.is_running ? "dot-live" : (last && last.status === "error" ? "dot-err" : "dot-live"));
      const typeLabel = t.type === "morning" ? "早报小秘书" : (t.type === "radar" ? "开源技术雷达" : (t.type === "asset_watcher" ? "数字资产守望" : esc(t.type)));
      const typeClass = t.type === "morning" ? "cat-domain" : (t.type === "radar" ? "cat-compute" : "cat-life");

      return `
        <article class="glass-card module-card">
          <div class="module-top">
            <div>
              <div style="display:flex; align-items:center; gap:8px; margin-bottom:4px;">
                <h3 class="serif-title module-name">${esc(t.name)}</h3>
                <span class="cat-badge ${typeClass}">${typeLabel}</span>
              </div>
              <p class="module-desc">${esc(t.description || "无详细描述")}</p>
            </div>
            <span class="stat-pill mono"><span class="dot ${dot}"></span>${t.enabled ? (t.is_running ? "RUN" : "ON") : "OFF"}</span>
          </div>
          <div class="module-cron-box mono">
            <div><span>Cron 表达式</span><br><strong>${esc(t.cron_expr || "--")}</strong></div>
            <div><span>下次执行 (CST)</span><br><strong>${esc(t.enabled ? (t.next_run_at || "--") : "停用")}</strong></div>
          </div>
          <div class="module-summary">${esc(last ? last.summary : "等待执行流水")}</div>
          <div class="task-foot">
            <button class="btn btn-primary btn-sm run-btn" data-id="${esc(t.id)}" ${t.is_running ? "disabled" : ""}>
              ${t.is_running ? "执行中..." : "立即触发"}
            </button>
            <div class="task-actions-right">
              <button class="btn btn-ghost btn-sm edit-task-btn" data-id="${esc(t.id)}">配置/编辑</button>
              <button class="btn btn-danger btn-sm del-task-btn" data-id="${esc(t.id)}">删除</button>
            </div>
          </div>
        </article>
      `;
    }).join("");

    // Bind Run
    grid.querySelectorAll(".run-btn").forEach((btn) => {
      btn.onclick = async () => {
        btn.disabled = true;
        btn.textContent = "执行中...";
        try {
          const res = await api(`/api/tasks/${encodeURIComponent(btn.dataset.id)}/run`, { method: "POST" });
          showToast(res.summary || "执行完成");
          await loadDashboard();
        } catch (err) {
          showToast(err.message);
          await loadDashboard();
        }
      };
    });

    // Bind Edit Task
    grid.querySelectorAll(".edit-task-btn").forEach((btn) => {
      btn.onclick = () => {
        const item = cachedTasks.find((x) => x.id === btn.dataset.id);
        if (!item) return;
        document.getElementById("modal-task-title").textContent = `编辑任务 · ${item.name}`;
        document.getElementById("edit-task-id-hidden").value = item.id;
        const idInput = document.getElementById("edit-task-id");
        idInput.value = item.id;
        idInput.disabled = true;
        document.getElementById("delete-task-btn").classList.remove("hidden");

        document.getElementById("edit-task-name").value = item.name;
        document.getElementById("edit-task-type").value = item.type;
        document.getElementById("edit-task-cron").value = item.cron_expr || "";
        document.getElementById("edit-task-enabled").value = item.enabled ? "true" : "false";
        document.getElementById("edit-task-desc").value = item.description || "";
        try {
          document.getElementById("edit-task-params-json").value = JSON.stringify(JSON.parse(item.params || "{}"), null, 2);
        } catch {
          document.getElementById("edit-task-params-json").value = item.params || "{}";
        }
        taskModalOverlay.classList.remove("hidden");
      };
    });

    // Bind Delete Task
    grid.querySelectorAll(".del-task-btn").forEach((btn) => {
      btn.onclick = async () => {
        const item = cachedTasks.find((x) => x.id === btn.dataset.id);
        if (!item) return;
        if (!confirm(`确定删除任务 [${item.name || item.id}] 吗？`)) return;
        try {
          await api(`/api/tasks/${encodeURIComponent(item.id)}`, { method: "DELETE" });
          showToast("任务已删除");
          await loadDashboard();
        } catch (err) {
          showToast(err.message);
        }
      };
    });
  }

  function renderLogs(logs) {
    const list = document.getElementById("logs-list");
    if (!logs.length) {
      list.innerHTML = `<div class="empty-state">暂无推送记录</div>`;
      return;
    }
    list.innerHTML = logs.map((l) => `
      <div class="log-item">
        <div class="log-head" data-id="${l.id}">
          <div class="log-title-wrap">
            <span class="dot ${l.status === "sent" ? "dot-live" : "dot-err"}"></span>
            <span class="log-tag mono">${esc(l.module_id)}</span>
            <span class="log-title">${esc(l.title)}</span>
          </div>
          <span class="log-meta mono">${esc(l.created_at.slice(5, 16))}</span>
        </div>
        <div id="log-body-${l.id}" class="log-body mono hidden">${esc(l.content)}\n\n[响应] ${esc(l.response_msg || "OK")}</div>
      </div>
    `).join("");

    list.querySelectorAll(".log-head").forEach((h) => {
      h.onclick = () => document.getElementById(`log-body-${h.dataset.id}`)?.classList.toggle("hidden");
    });
  }

  function fillConfig(cfg) {
    const keys = ["morning_enabled", "morning_cron", "morning_city", "nezha_url", "radar_enabled", "radar_cron", "asset_watcher_enabled", "asset_watcher_cron", "cf_api_token", "pushplus_token"];
    keys.forEach((k) => {
      const el = document.getElementById("cfg-" + k.replaceAll("_", "-"));
      if (el) el.value = cfg[k] || "";
    });
  }

  // Event Listeners
  document.getElementById("login-form").onsubmit = async (e) => {
    e.preventDefault();
    const errEl = document.getElementById("login-error");
    errEl.classList.add("hidden");
    try {
      await api("/api/auth/login", { method: "POST", body: JSON.stringify({ password: document.getElementById("login-password").value }) });
      document.getElementById("login-password").value = "";
      loginView.classList.add("hidden");
      dashView.classList.remove("hidden");
      await loadDashboard();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove("hidden");
    }
  };

  document.getElementById("logout-btn").onclick = async () => {
    await api("/api/auth/logout", { method: "POST" }).catch(() => {});
    dashView.classList.add("hidden");
    loginView.classList.remove("hidden");
  };

  document.getElementById("refresh-btn").onclick = loadDashboard;

  document.getElementById("test-push-btn").onclick = async (e) => {
    e.currentTarget.disabled = true;
    try {
      const res = await api("/api/test-push", { method: "POST" });
      showToast(res.message || "测试推送已发出");
      await loadDashboard();
    } catch (err) {
      showToast(err.message);
    } finally {
      e.currentTarget.disabled = false;
    }
  };

  document.getElementById("sync-assets-btn").onclick = async (e) => {
    const btn = e.currentTarget;
    btn.disabled = true;
    btn.textContent = "嗅探同步中...";
    try {
      const res = await api("/api/modules/asset_watcher/run", { method: "POST" });
      showToast(res.summary || "嗅探完成");
      await loadDashboard();
    } catch (err) {
      showToast(err.message);
    } finally {
      btn.disabled = false;
      btn.textContent = "立即嗅探同步";
    }
  };

  document.getElementById("config-form").onsubmit = async (e) => {
    e.preventDefault();
    const keys = ["morning_enabled", "morning_cron", "morning_city", "nezha_url", "radar_enabled", "radar_cron", "asset_watcher_enabled", "asset_watcher_cron", "cf_api_token", "pushplus_token"];
    const payload = {};
    keys.forEach((k) => {
      const el = document.getElementById("cfg-" + k.replaceAll("_", "-"));
      if (el) payload[k] = el.value.trim();
    });
    try {
      await api("/api/config", { method: "POST", body: JSON.stringify(payload) });
      showToast("配置已更新并重载调度");
      await loadDashboard();
    } catch (err) {
      showToast(err.message);
    }
  };

  document.getElementById("password-form").onsubmit = async (e) => {
    e.preventDefault();
    const pw = document.getElementById("new-password").value;
    try {
      await api("/api/auth/password", { method: "POST", body: JSON.stringify({ new_password: pw }) });
      document.getElementById("new-password").value = "";
      showToast("密钥已修改");
    } catch (err) {
      showToast(err.message);
    }
  };

  document.getElementById("close-modal-btn").onclick = () => modalOverlay.classList.add("hidden");
  document.getElementById("cancel-modal-btn").onclick = () => modalOverlay.classList.add("hidden");

  // Add Asset Button
  const addAssetBtn = document.getElementById("add-asset-btn");
  if (addAssetBtn) {
    addAssetBtn.onclick = () => {
      document.getElementById("modal-asset-title").textContent = "新增数字资产与订阅";
      document.getElementById("edit-asset-id").value = "";
      const idInput = document.getElementById("edit-asset-id-input");
      idInput.value = "";
      idInput.disabled = false;
      document.getElementById("delete-asset-btn").classList.add("hidden");

      document.getElementById("edit-asset-name").value = "";
      document.getElementById("edit-asset-category").value = "compute";
      document.getElementById("edit-asset-spec").value = "";
      document.getElementById("edit-asset-price").value = "";
      document.getElementById("edit-asset-due").value = "";
      document.getElementById("edit-asset-status").value = "运行中";
      document.getElementById("edit-asset-autorenew").value = "true";
      document.getElementById("edit-asset-warndays").value = "[7, 3, 1]";
      document.getElementById("edit-asset-extra").value = "{}";
      modalOverlay.classList.remove("hidden");
    };
  }

  // Delete Asset Button
  const deleteAssetBtn = document.getElementById("delete-asset-btn");
  if (deleteAssetBtn) {
    deleteAssetBtn.onclick = async () => {
      const id = document.getElementById("edit-asset-id").value;
      const name = document.getElementById("edit-asset-name").value;
      if (!id) return;
      if (!confirm(`确定下架并删除资产 [${name || id}] 吗？`)) return;

      try {
        await api(`/api/assets/${encodeURIComponent(id)}`, { method: "DELETE" });
        modalOverlay.classList.add("hidden");
        showToast("资产已删除");
        await loadDashboard();
      } catch (err) {
        showToast(err.message);
      }
    };
  }

  document.getElementById("asset-edit-form").onsubmit = async (e) => {
    e.preventDefault();
    const existingID = document.getElementById("edit-asset-id").value.trim();
    const idInputVal = document.getElementById("edit-asset-id-input").value.trim();
    const isNew = !existingID;
    const finalID = isNew ? idInputVal : existingID;

    if (!finalID) {
      showToast("资产 ID 不能为空");
      return;
    }

    const payload = {
      id: finalID,
      name: document.getElementById("edit-asset-name").value.trim(),
      category: document.getElementById("edit-asset-category").value,
      spec: document.getElementById("edit-asset-spec").value.trim(),
      price: document.getElementById("edit-asset-price").value.trim(),
      due_date: document.getElementById("edit-asset-due").value.trim(),
      status: document.getElementById("edit-asset-status").value.trim(),
      auto_renew: document.getElementById("edit-asset-autorenew").value === "true",
      warn_days: document.getElementById("edit-asset-warndays").value.trim(),
      extra_config: document.getElementById("edit-asset-extra").value.trim(),
    };

    try {
      if (isNew) {
        await api("/api/assets", { method: "POST", body: JSON.stringify(payload) });
        showToast("新资产创建成功");
      } else {
        await api(`/api/assets/${encodeURIComponent(finalID)}`, { method: "POST", body: JSON.stringify(payload) });
        showToast("资产更新成功");
      }
      modalOverlay.classList.add("hidden");
      await loadDashboard();
    } catch (err) {
      showToast(err.message);
    }
  };

  // Task Modal Handlers
  const addTaskBtn = document.getElementById("add-task-btn");
  if (addTaskBtn) {
    addTaskBtn.onclick = () => {
      document.getElementById("modal-task-title").textContent = "创建情报调度任务";
      document.getElementById("edit-task-id-hidden").value = "";
      const idInput = document.getElementById("edit-task-id");
      idInput.value = "";
      idInput.disabled = false;
      document.getElementById("delete-task-btn").classList.add("hidden");

      document.getElementById("edit-task-name").value = "";
      document.getElementById("edit-task-type").value = "morning";
      document.getElementById("edit-task-cron").value = "0 8 * * *";
      document.getElementById("edit-task-enabled").value = "true";
      document.getElementById("edit-task-desc").value = "汇总目标城市气象建议、每日格言、节点脉搏与临期资产";
      document.getElementById("edit-task-params-json").value = JSON.stringify({ city: "Shanghai", nezha_url: "https://dash.starryheavens.uk" }, null, 2);
      taskModalOverlay.classList.remove("hidden");
    };
  }

  // Type change template presets
  const taskTypeSelect = document.getElementById("edit-task-type");
  if (taskTypeSelect) {
    taskTypeSelect.onchange = () => {
      const type = taskTypeSelect.value;
      const isNew = !document.getElementById("edit-task-id-hidden").value;
      if (isNew) {
        if (type === "morning") {
          document.getElementById("edit-task-name").value = "每日晨报小秘书";
          document.getElementById("edit-task-cron").value = "0 8 * * *";
          document.getElementById("edit-task-desc").value = "汇总目标城市气象建议、每日格言、节点脉搏与临期资产";
          document.getElementById("edit-task-params-json").value = JSON.stringify({ city: "Shanghai", nezha_url: "https://dash.starryheavens.uk" }, null, 2);
        } else if (type === "radar") {
          document.getElementById("edit-task-name").value = "全网开源技术雷达";
          document.getElementById("edit-task-cron").value = "0 12,20 * * *";
          document.getElementById("edit-task-desc").value = "抓取 GitHub Trending 与 Hacker News 热门高分条目";
          document.getElementById("edit-task-params-json").value = JSON.stringify({ hn_min_score: 120, limit: 8 }, null, 2);
        } else if (type === "asset_watcher") {
          document.getElementById("edit-task-name").value = "数字资产与会员守望";
          document.getElementById("edit-task-cron").value = "0 3 * * *";
          document.getElementById("edit-task-desc").value = "自动巡检资产到期与自动扣费防坑预警";
          document.getElementById("edit-task-params-json").value = "{}";
        }
      }
    };
  }

  const closeTaskModalBtn = document.getElementById("close-task-modal-btn");
  if (closeTaskModalBtn) closeTaskModalBtn.onclick = () => taskModalOverlay.classList.add("hidden");
  const cancelTaskModalBtn = document.getElementById("cancel-task-modal-btn");
  if (cancelTaskModalBtn) cancelTaskModalBtn.onclick = () => taskModalOverlay.classList.add("hidden");

  const deleteTaskModalBtn = document.getElementById("delete-task-btn");
  if (deleteTaskModalBtn) {
    deleteTaskModalBtn.onclick = async () => {
      const id = document.getElementById("edit-task-id-hidden").value;
      const name = document.getElementById("edit-task-name").value;
      if (!id) return;
      if (!confirm(`确定删除任务 [${name || id}] 吗？`)) return;
      try {
        await api(`/api/tasks/${encodeURIComponent(id)}`, { method: "DELETE" });
        taskModalOverlay.classList.add("hidden");
        showToast("任务已删除");
        await loadDashboard();
      } catch (err) {
        showToast(err.message);
      }
    };
  }

  const taskEditForm = document.getElementById("task-edit-form");
  if (taskEditForm) {
    taskEditForm.onsubmit = async (e) => {
      e.preventDefault();
      const existingID = document.getElementById("edit-task-id-hidden").value.trim();
      const idInputVal = document.getElementById("edit-task-id").value.trim();
      const isNew = !existingID;
      const finalID = isNew ? idInputVal : existingID;

      if (!finalID) {
        showToast("任务 ID 不能为空");
        return;
      }

      const payload = {
        id: finalID,
        name: document.getElementById("edit-task-name").value.trim(),
        type: document.getElementById("edit-task-type").value,
        cron_expr: document.getElementById("edit-task-cron").value.trim(),
        enabled: document.getElementById("edit-task-enabled").value === "true",
        description: document.getElementById("edit-task-desc").value.trim(),
        params: document.getElementById("edit-task-params-json").value.trim(),
      };

      try {
        if (isNew) {
          await api("/api/tasks", { method: "POST", body: JSON.stringify(payload) });
          showToast("新任务创建成功");
        } else {
          await api(`/api/tasks/${encodeURIComponent(finalID)}`, { method: "POST", body: JSON.stringify(payload) });
          showToast("任务已更新并重载调度");
        }
        taskModalOverlay.classList.add("hidden");
        await loadDashboard();
      } catch (err) {
        showToast(err.message);
      }
    };
  }

  checkAuth();
})();
