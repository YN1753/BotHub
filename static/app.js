(() => {
  const loginView = document.getElementById("login-view");
  const dashView = document.getElementById("dashboard-view");
  const loginForm = document.getElementById("login-form");
  const loginError = document.getElementById("login-error");
  const toastEl = document.getElementById("toast");

  let toastTimer = null;

  function showToast(msg) {
    toastEl.textContent = msg;
    toastEl.classList.remove("hidden");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => toastEl.classList.add("hidden"), 3200);
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
      renderModules(data.modules || []);
      fillConfig(data.configs || {});
      renderLogs(data.push_logs || []);
    } catch (err) {
      showToast(err.message);
    }
  }

  function renderStats(data) {
    const stats = data.stats || {};
    document.getElementById("stat-sent").textContent = stats.sent_pushes ?? 0;
    document.getElementById("stat-total").textContent = stats.total_pushes ?? 0;
    document.getElementById("stat-seen").textContent = stats.seen_items ?? 0;
    document.getElementById("stat-time").textContent = data.server_now || "--";
    document.getElementById("uptime-label").textContent = `UPTIME ${formatUptime(data.uptime_sec)}`;
  }

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

  document.getElementById("config-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const payload = {
      morning_enabled: document.getElementById("cfg-morning-enabled").value,
      morning_cron: document.getElementById("cfg-morning-cron").value.trim(),
      morning_city: document.getElementById("cfg-morning-city").value.trim(),
      nezha_url: document.getElementById("cfg-nezha-url").value.trim(),
      radar_enabled: document.getElementById("cfg-radar-enabled").value,
      radar_cron: document.getElementById("cfg-radar-cron").value.trim(),
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
