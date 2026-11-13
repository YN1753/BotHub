(() => {
  const loginView = document.getElementById("login-view");
  const dashView = document.getElementById("dashboard-view");
  const loginForm = document.getElementById("login-form");
  const loginError = document.getElementById("login-error");
  const toastEl = document.getElementById("toast");

  // Modal elements
  const modalOverlay = document.getElementById("asset-modal-overlay");
  const closeModalBtn = document.getElementById("close-modal-btn");
  const cancelModalBtn = document.getElementById("cancel-modal-btn");
  const assetEditForm = document.getElementById("asset-edit-form");

  let toastTimer = null;
  let cachedAssets = [];

  function showToast(msg) {
    toastEl.textContent = msg;
    toastEl.classList.remove("hidden");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => toastEl.classList.add("hidden"), 3400);
  }

  function esc(str) {
    return String(str ?? "")
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;")
      .replaceAll("'", "&#39;");
  }

  function formatUptime(sec) {
    const s = Number(sec) || 0;
    if (s < 60) return `${s}s`;
    const m = Math.floor(s / 60);
    if (m < 60) return `${m}m ${s % 60}s`;
    const h = Math.floor(m / 60);
    if (h < 24) return `${h}h ${m % 60}m`;
    const d = Math.floor(h / 24);
    return `${d}d ${h % 24}h`;
  }

  function formatDueDate(str) {
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
      showLogin();
      throw new Error(data.error || "会话已过期，请重新登录");
    }
    if (!res.ok) {
      throw new Error(data.error || `请求失败 (${res.status})`);
    }
    return data;
  }

  function showLogin() {
    dashView.classList.add("hidden");
    loginView.classList.remove("hidden");
  }

  function showDashboard() {
    loginView.classList.add("hidden");
    dashView.classList.remove("hidden");
  }

  async function checkAuth() {
    try {
      const data = await api("/api/auth/status");
      if (data.authenticated) {
        showDashboard();
        await loadDashboard();
      } else {
        showLogin();
      }
    } catch {
      showLogin();
    }
  }

  async function loadDashboard() {
    try {
      const data = await api("/api/dashboard");
      renderStats(data);
      renderAssets(data.assets || []);
      renderModules(data.modules || []);
      fillConfig(data.configs || {});
      renderLogs(data.push_logs || []);
    } catch (err) {
      showToast(err.message);
    }
  }

  function renderStats(data) {
    const stats = data.stats || {};
    document.getElementById("stat-assets").textContent = stats.total_assets ?? 0;
    document.getElementById("stat-sent").textContent = stats.sent_pushes ?? 0;
    document.getElementById("stat-seen").textContent = stats.seen_items ?? 0;
    document.getElementById("stat-time").textContent = data.server_now || "--";
    document.getElementById("uptime-label").textContent = `UPTIME ${formatUptime(data.uptime_sec)}`;
  }

  function renderAssets(assets) {
    cachedAssets = assets;
    const grid = document.getElementById("assets-grid");
    if (!assets.length) {
      grid.innerHTML = `<div class="empty-state">暂无追踪中的资产与会员，可配置或初始化资产表</div>`;
      return;
    }

    grid.innerHTML = assets.map((a) => {
      let catClass = "cat-life";
      let catLabel = "生活订阅";
      if (a.category === "compute") {
        catClass = "cat-compute";
        catLabel = "算力基建";
      } else if (a.category === "domain") {
        catClass = "cat-domain";
        catLabel = "核心域名";
      }

      let daysClass = "days-safe";
      let daysText = `剩余 ${a.days_left} 天`;
      if (a.days_left < 0) {
        daysClass = "days-danger";
        daysText = `已逾期 ${Math.abs(a.days_left)} 天`;
      } else if (a.days_left <= 3) {
        daysClass = "days-danger";
      } else if (a.days_left <= 7) {
        daysClass = "days-warn";
      }

      const isTrial = (a.price && a.price.includes("活动")) ||
                      (a.status && a.status.includes("活动")) ||
                      (a.extra_config && a.extra_config.includes('"is_trial":true')) ||
                      a.id === "qq_music";

      return `
        <article class="glass-card asset-card" data-id="${esc(a.id)}">
          <div class="asset-head">
            <div class="asset-title-block">
              <h3 class="serif-title asset-name">
                <span>${esc(a.name)}</span>
              </h3>
              <p class="asset-spec mono">${esc(a.spec || a.category)}</p>
            </div>
            <span class="category-tag ${catClass}">${esc(catLabel)}</span>
          </div>

          <div class="asset-countdown">
            <span class="countdown-label">扣费 / 到期倒计时</span>
            <span class="days-badge ${daysClass}">${esc(daysText)}</span>
          </div>

          <div class="asset-details mono">
            <div class="detail-row">
              <span>金额与周期</span>
              <strong>${esc(a.price || "--")}</strong>
            </div>
            <div class="detail-row">
              <span>当前状态</span>
              <strong>${esc(a.status || "--")}</strong>
            </div>
            <div class="detail-row" style="grid-column: 1 / -1;">
              <span>到期时间 (CST)</span>
              <strong>${esc(formatDueDate(a.due_date))}</strong>
            </div>
          </div>

          ${isTrial ? `
            <div class="trial-alert-box">
              <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/></svg>
              <span>防自动扣款：活动体验到期前请确认是否已关闭免密续费</span>
            </div>
          ` : ""}

          <div class="asset-footer">
            <span class="auto-renew-tag ${a.auto_renew ? "on" : ""}">
              自动续费: ${a.auto_renew ? "已开启" : "已关闭"}
            </span>
            <button type="button" class="btn btn-ghost btn-sm edit-asset-btn" data-id="${esc(a.id)}">
              <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/></svg>
              <span>编辑 / 凭证</span>
            </button>
          </div>
        </article>
      `;
    }).join("");

    grid.querySelectorAll(".edit-asset-btn").forEach((btn) => {
      btn.addEventListener("click", () => {
        const id = btn.dataset.id;
        const target = cachedAssets.find((item) => item.id === id);
        if (target) {
          openAssetModal(target);
        }
      });
    });
  }

  function openAssetModal(asset) {
    document.getElementById("modal-asset-title").textContent = `编辑资产 · ${asset.name}`;
    document.getElementById("edit-asset-id").value = asset.id;
    document.getElementById("edit-asset-name").value = asset.name || "";
    document.getElementById("edit-asset-category").value = asset.category || "life";
    document.getElementById("edit-asset-spec").value = asset.spec || "";
    document.getElementById("edit-asset-price").value = asset.price || "";
    document.getElementById("edit-asset-due").value = asset.due_date || "";
    document.getElementById("edit-asset-status").value = asset.status || "";
    document.getElementById("edit-asset-autorenew").value = asset.auto_renew ? "true" : "false";
    document.getElementById("edit-asset-warndays").value = asset.warn_days || "[7, 3, 1]";

    let extraStr = "{}";
    try {
      if (asset.extra_config) {
        extraStr = JSON.stringify(JSON.parse(asset.extra_config), null, 2);
      }
    } catch {
      extraStr = asset.extra_config || "{}";
    }
    document.getElementById("edit-asset-extra").value = extraStr;

    modalOverlay.classList.remove("hidden");
  }

  function closeAssetModal() {
    modalOverlay.classList.add("hidden");
  }

  closeModalBtn.addEventListener("click", closeAssetModal);
  cancelModalBtn.addEventListener("click", closeAssetModal);
  modalOverlay.addEventListener("click", (e) => {
    if (e.target === modalOverlay) closeAssetModal();
  });

  assetEditForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    const id = document.getElementById("edit-asset-id").value;
    const payload = {
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
      await api(`/api/assets/${encodeURIComponent(id)}`, {
        method: "POST",
        body: JSON.stringify(payload),
      });
      showToast("资产与凭证更新成功");
      closeAssetModal();
      await loadDashboard();
    } catch (err) {
      showToast(err.message);
    }
  });

  function renderModules(mods) {
    const grid = document.getElementById("modules-grid");
    if (!mods.length) {
      grid.innerHTML = `<div class="empty-state">暂无已注册模块</div>`;
      return;
    }

    grid.innerHTML = mods.map((m) => {
      const last = m.last_run;
      let dotClass = "dot-warn";
      let statusLabel = "STANDBY";
      if (!m.enabled) {
        dotClass = "dot-warn";
        statusLabel = "DISABLED";
      } else if (m.is_running) {
        dotClass = "dot-live";
        statusLabel = "RUNNING";
      } else if (last) {
        dotClass = last.status === "success" ? "dot-live" : "dot-err";
        statusLabel = last.status.toUpperCase();
      } else {
        dotClass = "dot-live";
        statusLabel = "ACTIVE";
      }

      const lastTime = last ? (last.finished_at || last.started_at) : "尚未执行";
      const duration = last ? `${last.duration_ms} ms` : "--";
      const summary = last && last.summary ? last.summary : "等待首次调度或手动触发";

      return `
        <article class="glass-card module-card">
          <div class="module-top">
            <div>
              <h3 class="serif-title module-name">${esc(m.name)}</h3>
              <p class="module-desc">${esc(m.description)}</p>
            </div>
            <span class="status-badge">
              <span class="dot ${dotClass}"></span>
              <span>${esc(statusLabel)}</span>
            </span>
          </div>

          <div class="module-meta mono">
            <div class="meta-item">
              <span>CRON 调度 (CST)</span>
              <strong>${esc(m.cron_expr || "--")}</strong>
            </div>
            <div class="meta-item">
              <span>下次执行</span>
              <strong>${esc(m.enabled ? (m.next_run_at || "--") : "已停用")}</strong>
            </div>
            <div class="meta-item">
              <span>最近运行</span>
              <strong>${esc(lastTime)}</strong>
            </div>
            <div class="meta-item">
              <span>耗时 / 触发</span>
              <strong>${esc(duration)} ${last ? `(${esc(last.trigger_type)})` : ""}</strong>
            </div>
          </div>

          <div class="module-summary">${esc(summary)}</div>

          <div class="module-actions">
            <button class="btn btn-primary run-mod-btn" data-id="${esc(m.id)}" ${m.is_running ? "disabled" : ""}>
              <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><polygon points="6 3 20 12 6 21 6 3"/></svg>
              <span>${m.is_running ? "执行中..." : "立即触发一次"}</span>
            </button>
          </div>
        </article>
      `;
    }).join("");

    grid.querySelectorAll(".run-mod-btn").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const id = btn.dataset.id;
        btn.disabled = true;
        btn.querySelector("span").textContent = "执行中...";
        try {
          const res = await api(`/api/modules/${encodeURIComponent(id)}/run`, { method: "POST" });
          if (res.ok) {
            showToast(res.summary || "模块执行完成");
          } else {
            showToast(`执行异常: ${res.error || res.summary}`);
          }
          await loadDashboard();
        } catch (err) {
          showToast(err.message);
          await loadDashboard();
        }
      });
    });
  }

  function fillConfig(cfg) {
    document.getElementById("cfg-morning-enabled").value = cfg.morning_enabled || "true";
    document.getElementById("cfg-morning-cron").value = cfg.morning_cron || "0 8 * * *";
    document.getElementById("cfg-morning-city").value = cfg.morning_city || "Shanghai";
    document.getElementById("cfg-nezha-url").value = cfg.nezha_url || "";
    document.getElementById("cfg-radar-enabled").value = cfg.radar_enabled || "true";
    document.getElementById("cfg-radar-cron").value = cfg.radar_cron || "0 12,20 * * *";
    document.getElementById("cfg-asset-watcher-enabled").value = cfg.asset_watcher_enabled || "true";
    document.getElementById("cfg-asset-watcher-cron").value = cfg.asset_watcher_cron || "0 3 * * *";
    document.getElementById("cfg-cf-api-token").value = cfg.cf_api_token || "";
    document.getElementById("cfg-hn-score").value = cfg.radar_hn_min_score || "120";
    document.getElementById("cfg-radar-limit").value = cfg.radar_limit || "8";
    document.getElementById("cfg-pushplus-token").value = cfg.pushplus_token || "";
  }

  function renderLogs(logs) {
    const list = document.getElementById("logs-list");
    if (!logs.length) {
      list.innerHTML = `<div class="empty-state">暂无 PushPlus 推送记录，可点击上方「测试推送」或模块触发按钮验证链路。</div>`;
      return;
    }

    list.innerHTML = logs.map((l) => {
      const dotClass = l.status === "sent" ? "dot-live" : "dot-err";
      return `
        <div class="log-item">
          <div class="log-head" data-toggle="log-${l.id}">
            <div class="log-title-wrap">
              <span class="dot ${dotClass}"></span>
              <span class="log-tag">${esc(l.module_id)}</span>
              <span class="log-title">${esc(l.title)}</span>
            </div>
            <div class="log-meta mono">
              <span>${esc(l.status.toUpperCase())}</span>
              <span>${esc(l.created_at)}</span>
            </div>
          </div>
          <div id="log-${l.id}" class="log-body mono hidden">${esc(l.content)}\n\n[PushPlus 响应] ${esc(l.response_msg || "OK")}</div>
        </div>
      `;
    }).join("");

    list.querySelectorAll(".log-head").forEach((head) => {
      head.addEventListener("click", () => {
        const target = document.getElementById(head.dataset.toggle);
        if (target) target.classList.toggle("hidden");
      });
    });
  }

  loginForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    loginError.classList.add("hidden");
    const password = document.getElementById("login-password").value;
    try {
      await api("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ password }),
      });
      document.getElementById("login-password").value = "";
      showDashboard();
      await loadDashboard();
    } catch (err) {
      loginError.textContent = err.message;
      loginError.classList.remove("hidden");
    }
  });

  document.getElementById("logout-btn").addEventListener("click", async () => {
    await api("/api/auth/logout", { method: "POST" }).catch(() => {});
    showLogin();
  });

  document.getElementById("refresh-btn").addEventListener("click", loadDashboard);

  document.getElementById("test-push-btn").addEventListener("click", async (e) => {
    const btn = e.currentTarget;
    btn.disabled = true;
    try {
      const res = await api("/api/test-push", { method: "POST" });
      showToast(res.message || "测试推送已发送");
      await loadDashboard();
    } catch (err) {
      showToast(err.message);
    } finally {
      btn.disabled = false;
    }
  });

  document.getElementById("sync-assets-btn").addEventListener("click", async (e) => {
    const btn = e.currentTarget;
    btn.disabled = true;
    const origHTML = btn.innerHTML;
    btn.innerHTML = `<span class="dot dot-live"></span><span>嗅探同步中...</span>`;
    try {
      const res = await api("/api/modules/asset_watcher/run", { method: "POST" });
      if (res.ok) {
        showToast(res.summary || "资产嗅探同步完成");
      } else {
        showToast(`嗅探提示: ${res.error || res.summary}`);
      }
      await loadDashboard();
    } catch (err) {
      showToast(err.message);
    } finally {
      btn.disabled = false;
      btn.innerHTML = origHTML;
    }
  });

  document.getElementById("config-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const payload = {
      morning_enabled: document.getElementById("cfg-morning-enabled").value,
      morning_cron: document.getElementById("cfg-morning-cron").value.trim(),
      morning_city: document.getElementById("cfg-morning-city").value.trim(),
      nezha_url: document.getElementById("cfg-nezha-url").value.trim(),
      radar_enabled: document.getElementById("cfg-radar-enabled").value,
      radar_cron: document.getElementById("cfg-radar-cron").value.trim(),
      asset_watcher_enabled: document.getElementById("cfg-asset-watcher-enabled").value,
      asset_watcher_cron: document.getElementById("cfg-asset-watcher-cron").value.trim(),
      cf_api_token: document.getElementById("cfg-cf-api-token").value.trim(),
      radar_hn_min_score: document.getElementById("cfg-hn-score").value.trim(),
      radar_limit: document.getElementById("cfg-radar-limit").value.trim(),
      pushplus_token: document.getElementById("cfg-pushplus-token").value.trim(),
    };

    try {
      await api("/api/config", { method: "POST", body: JSON.stringify(payload) });
      showToast("配置已保存并重载调度器");
      await loadDashboard();
    } catch (err) {
      showToast(err.message);
    }
  });

  document.getElementById("password-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const newPassword = document.getElementById("new-password").value;
    try {
      await api("/api/auth/password", {
        method: "POST",
        body: JSON.stringify({ new_password: newPassword }),
      });
      document.getElementById("new-password").value = "";
      showToast("管理密钥已更新");
    } catch (err) {
      showToast(err.message);
    }
  });

  checkAuth();
})();
