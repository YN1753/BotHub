<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import { parseJsonRecord } from '../../utils/format'
import type { Skill } from '../../types/bothub'

const store = useAppStore()

const sandboxSkills = computed<Skill[]>(() =>
  (store.state.skills || []).filter(
    (s) =>
      s.kind === 'mcp' &&
      (s.slug.startsWith('sandbox_') ||
        s.id.startsWith('mcp_sandbox_') ||
        (s.http_url || '').includes(':9099/mcp'))
  )
)

const editingId = ref<string | null>(null)
const nodeName = ref('')
const nodeSlug = ref('')
const nodeHost = ref('')
const nodePort = ref('9099')
const nodeToken = ref('')
const masterIp = ref('198.84.85.196')
const memoryMax = ref('1200M')
const nodeDesc = ref('')
const saving = ref(false)
const testingId = ref<string | null>(null)
const showDeployScript = ref(false)

function generateToken() {
  const bytes = new Uint8Array(32)
  window.crypto.getRandomValues(bytes)
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
  nodeToken.value = `vsb_${hex}`
}

function resetForm() {
  editingId.value = null
  nodeName.value = ''
  nodeSlug.value = ''
  nodeHost.value = ''
  nodePort.value = '9099'
  nodeToken.value = ''
  memoryMax.value = '1200M'
  nodeDesc.value = ''
  showDeployScript.value = false
}

function normalizedEndpoint(): string {
  const host = nodeHost.value.trim()
  if (!host) return ''
  if (/^https?:\/\//i.test(host)) {
    return host.endsWith('/mcp') ? host : `${host.replace(/\/+$/, '')}/mcp`
  }
  const port = (nodePort.value || '9099').trim()
  return `http://${host}:${port}/mcp`
}

function normalizedSlug(): string {
  const raw = (nodeSlug.value || nodeName.value)
    .toLowerCase()
    .replace(/[^a-z0-9_]+/g, '_')
    .replace(/^_+|_+$/g, '')
  if (!raw) return `sandbox_${Math.random().toString(16).slice(2, 6)}`
  return raw.startsWith('sandbox_') ? raw : `sandbox_${raw}`
}

function loadNodeForEdit(sk: Skill) {
  editingId.value = sk.id
  nodeName.value = sk.name
  nodeSlug.value = sk.slug
  nodeDesc.value = sk.description || ''
  const rawUrl = (sk.http_url || '').trim()
  try {
    const parsed = new URL(rawUrl)
    if (parsed.protocol === 'http:' && parsed.pathname === '/mcp') {
      nodeHost.value = parsed.hostname
      nodePort.value = parsed.port || '9099'
    } else {
      nodeHost.value = rawUrl
      nodePort.value = parsed.port || '9099'
    }
  } catch {
    nodeHost.value = rawUrl
    nodePort.value = '9099'
  }
  const headers = parseJsonRecord(sk.http_headers)
  const auth = headers.Authorization || headers.authorization || ''
  nodeToken.value = auth.replace(/^Bearer\s+/i, '').trim()
}

const deployScript = computed(() => {
  const tok = nodeToken.value.trim() || '<先点击上方生成64位密钥>'
  const port = (nodePort.value || '9099').trim()
  const allow = (masterIp.value || '198.84.85.196').trim()
  const mem = (memoryMax.value || '1200M').trim()
  const host = nodeHost.value.trim() || '<新虚拟机IP>'
  return [
    '# 1) 在本机交叉编译纯 Go 静态执行节点 (零依赖):',
    'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v2 go build -trimpath -ldflags="-s -w" -o vesper-sandbox-agent ./cmd/sandbox-agent',
    `scp vesper-sandbox-agent root@${host}:/opt/vesper-sandbox/vesper-sandbox-agent`,
    '',
    '# 2) 在目标虚拟机上以 root 执行初始化与强隔离服务安装:',
    'id vesper-sandbox >/dev/null 2>&1 || useradd --create-home --shell /bin/bash vesper-sandbox',
    'mkdir -p /home/vesper-sandbox/workspace /opt/vesper-sandbox /etc/vesper-sandbox',
    'chown -R vesper-sandbox:vesper-sandbox /home/vesper-sandbox/workspace',
    'chmod 755 /opt/vesper-sandbox/vesper-sandbox-agent',
    `cat << 'EOF' > /etc/vesper-sandbox/sandbox.env`,
    `SANDBOX_TOKEN=${tok}`,
    `SANDBOX_PORT=${port}`,
    `SANDBOX_ALLOWED_IPS=${allow},127.0.0.1,::1`,
    'SANDBOX_WORKSPACE=/home/vesper-sandbox/workspace',
    'EOF',
    'chown root:vesper-sandbox /etc/vesper-sandbox/sandbox.env && chmod 640 /etc/vesper-sandbox/sandbox.env',
    `cat << 'EOF' > /etc/systemd/system/vesper-sandbox-mcp.service`,
    '[Unit]',
    'Description=Vesper BotHub Remote Sandbox Agent (Go)',
    'After=network-online.target',
    'Wants=network-online.target',
    '[Service]',
    'Type=simple',
    'User=vesper-sandbox',
    'Group=vesper-sandbox',
    'WorkingDirectory=/home/vesper-sandbox/workspace',
    'EnvironmentFile=/etc/vesper-sandbox/sandbox.env',
    'ExecStart=/opt/vesper-sandbox/vesper-sandbox-agent',
    'Restart=always',
    'RestartSec=3s',
    'NoNewPrivileges=true',
    'PrivateTmp=true',
    'ProtectSystem=strict',
    'ProtectHome=tmpfs',
    'BindPaths=/home/vesper-sandbox/workspace',
    'ReadOnlyPaths=/opt/vesper-sandbox',
    `MemoryMax=${mem}`,
    'CPUQuota=150%',
    'TasksMax=128',
    '[Install]',
    'WantedBy=multi-user.target',
    'EOF',
    `command -v ufw >/dev/null 2>&1 && ufw allow from ${allow} to any port ${port} proto tcp || true`,
    'systemctl daemon-reload && systemctl enable --now vesper-sandbox-mcp && systemctl is-active vesper-sandbox-mcp',
  ].join('\n')
})

async function copyDeployScript() {
  if (!nodeToken.value.trim()) {
    generateToken()
  }
  try {
    await navigator.clipboard.writeText(deployScript.value)
    store.notify('已复制纯 Go 节点隔离部署脚本')
  } catch {
    store.notify('复制失败，请手动选中脚本复制')
  }
}

async function saveNode(andTest = true) {
  const endpoint = normalizedEndpoint()
  if (!nodeName.value.trim() || !endpoint) {
    store.notify('请填写节点名称与服务器 IP / Endpoint 地址')
    return
  }
  if (!nodeToken.value.trim() && !editingId.value) {
    store.notify('请先填写或生成 64 位 Bearer 通信密钥')
    return
  }
  saving.value = true
  try {
    const headersObj: Record<string, string> = {}
    if (nodeToken.value.trim()) {
      headersObj.Authorization = `Bearer ${nodeToken.value.trim()}`
    }
    const payload: Partial<Skill> = {
      name: nodeName.value.trim(),
      slug: normalizedSlug(),
      kind: 'mcp',
      description:
        nodeDesc.value.trim() ||
        '纯 Go 静态非 root 隔离执行虚拟机节点，支持安全运行 Shell、读写隔离工作区文件与节点状态探测。',
      instruction:
        '当用户需要执行远程命令、脚本、数据处理或隔离沙盒操作时，优先调用此远程执行节点工具。',
      http_method: 'POST',
      http_url: endpoint,
      http_headers: JSON.stringify(headersObj),
      enabled: true,
    }
    let savedId = editingId.value
    if (savedId) {
      await BotHubApi.updateSkill(savedId, payload)
    } else {
      const created = await BotHubApi.createSkill(payload)
      savedId = created?.skill?.id || null
    }
    await store.refreshWorkspaceSlices(['skills'])
    if (andTest && savedId) {
      testingId.value = savedId
      try {
        const res = await BotHubApi.testSkill(savedId, {})
        await store.refreshWorkspaceSlices(['skills'])
        store.notify(typeof res.result === 'string' ? res.result : `节点「${payload.name}」已保存并连通`)
      } catch (err: any) {
        store.notify(`节点已保存，但连通测试失败：${err.message || '请确认目标机 Agent 已启动且防火墙已放行'}`)
      } finally {
        testingId.value = null
      }
    } else {
      store.notify(`已保存执行节点「${payload.name}」`)
    }
    resetForm()
  } catch (err: any) {
    store.notify(err.message || '保存执行节点失败')
  } finally {
    saving.value = false
  }
}

async function pingNode(sk: Skill) {
  if (testingId.value) return
  testingId.value = sk.id
  try {
    const res = await BotHubApi.testSkill(sk.id, {})
    await store.refreshWorkspaceSlices(['skills'])
    store.notify(typeof res.result === 'string' ? res.result : `节点「${sk.name}」握手正常`)
  } catch (err: any) {
    store.notify(err.message || '节点连通探测失败')
  } finally {
    testingId.value = null
  }
}

async function toggleNode(sk: Skill) {
  try {
    await BotHubApi.updateSkill(sk.id, { enabled: !sk.enabled })
    await store.refreshWorkspaceSlices(['skills'])
    store.notify(sk.enabled ? `已停用节点「${sk.name}」` : `已启用节点「${sk.name}」`)
  } catch (err: any) {
    store.notify(err.message || '切换节点状态失败')
  }
}

async function removeNode(sk: Skill) {
  try {
    await BotHubApi.deleteSkill(sk.id)
    if (editingId.value === sk.id) resetForm()
    await store.refreshWorkspaceSlices(['skills'])
    store.notify(`已移除执行节点「${sk.name}」`)
  } catch (err: any) {
    store.notify(err.message || '移除节点失败')
  }
}

function countTools(sk: Skill): number {
  try {
    const parsed = JSON.parse(sk.params_schema || '[]')
    return Array.isArray(parsed) ? parsed.length : 0
  } catch {
    return 0
  }
}
</script>

<template>
  <div class="flex flex-col gap-3">
    <!-- Existing Remote VM Nodes List -->
    <div class="rounded-xl border border-line bg-surface/30 p-3.5 flex flex-col gap-2.5">
      <div class="flex items-center justify-between gap-2">
        <div>
          <strong class="text-xs font-semibold text-fg">已接入远程执行虚拟机 (支持多台并存)</strong>
          <p class="text-[11px] text-fg-muted mt-0.5">
            每台节点运行独立的纯 Go 非 root 沙盒 (`cmd/sandbox-agent`)，可在任意 Bot 的 MCP 挂载列表里自由选择。
          </p>
        </div>
        <span class="text-[11px] px-2 py-0.5 rounded-full border border-line text-fg-sub shrink-0">
          {{ sandboxSkills.length }} 台节点
        </span>
      </div>

      <div v-if="sandboxSkills.length === 0" class="rounded-lg border border-dashed border-line px-3 py-4 text-center text-xs text-fg-muted">
        暂无远程执行虚拟机，可在下方添加任意 Linux VPS / 测试机作为 BotHub 隔离执行节点。
      </div>

      <div v-else class="flex flex-col gap-2">
        <div
          v-for="sk in sandboxSkills"
          :key="sk.id"
          class="rounded-xl border border-line bg-canvas px-3 py-2.5 flex flex-col sm:flex-row sm:items-center justify-between gap-2"
        >
          <div class="min-w-0 flex flex-col gap-0.5">
            <div class="flex items-center gap-2 flex-wrap">
              <span
                :class="[
                  'w-2 h-2 rounded-full shrink-0',
                  sk.enabled ? 'bg-ok' : 'bg-line-strong',
                ]"
              />
              <span class="text-xs font-semibold text-fg truncate">{{ sk.name }}</span>
              <span class="font-mono text-[10px] px-1.5 py-0.5 rounded bg-surface text-fg-muted">
                {{ sk.slug }}
              </span>
              <span class="text-[10px] px-1.5 py-0.5 rounded border border-line text-fg-sub">
                {{ countTools(sk) }} 工具
              </span>
            </div>
            <span class="font-mono text-[11px] text-fg-muted truncate">{{ sk.http_url }}</span>
          </div>

          <div class="flex items-center gap-1.5 shrink-0">
            <button
              type="button"
              :disabled="testingId === sk.id"
              @click="pingNode(sk)"
              class="px-2.5 py-1 rounded-lg border border-line bg-surface/60 hover:border-accent text-[11px] text-fg-sub cursor-pointer disabled:opacity-50"
            >
              {{ testingId === sk.id ? '探测中...' : '连通测试' }}
            </button>
            <button
              type="button"
              @click="loadNodeForEdit(sk)"
              class="px-2.5 py-1 rounded-lg border border-line bg-surface/60 hover:border-accent text-[11px] text-fg-sub cursor-pointer"
            >
              编辑
            </button>
            <button
              type="button"
              @click="toggleNode(sk)"
              class="px-2.5 py-1 rounded-lg border border-line bg-surface/60 hover:border-accent text-[11px] text-fg-sub cursor-pointer"
            >
              {{ sk.enabled ? '停用' : '启用' }}
            </button>
            <button
              type="button"
              @click="removeNode(sk)"
              class="px-2.5 py-1 rounded-lg border border-line bg-surface/60 hover:border-danger text-[11px] text-danger cursor-pointer"
            >
              移除
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Add / Edit VM Form -->
    <div class="rounded-xl border border-line bg-surface/30 p-3.5 flex flex-col gap-2.5">
      <div class="flex items-center justify-between">
        <strong class="text-xs font-semibold text-fg">
          {{ editingId ? '编辑远程执行节点' : '新增通用执行虚拟机' }}
        </strong>
        <button
          v-if="editingId"
          type="button"
          @click="resetForm"
          class="text-[11px] text-fg-muted hover:text-fg cursor-pointer"
        >
          取消编辑
        </button>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
        <label class="flex flex-col gap-1 text-xs text-fg-sub">
          <span>节点显示名称</span>
          <input
            v-model="nodeName"
            type="text"
            placeholder="如 远程执行节点 · CloudCone LA"
            class="min-h-[36px] rounded-xl border border-line bg-canvas px-3 py-1.5 text-xs text-fg focus:border-accent"
          />
        </label>
        <label class="flex flex-col gap-1 text-xs text-fg-sub">
          <span>节点标识 Slug (自动加 sandbox_ 前缀)</span>
          <input
            v-model="nodeSlug"
            type="text"
            placeholder="如 sandbox_cc_la / sandbox_sg"
            class="min-h-[36px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
          />
        </label>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
        <label class="flex flex-col gap-1 text-xs text-fg-sub sm:col-span-2">
          <span>目标虚拟机 IP 或完整 Endpoint URL</span>
          <input
            v-model="nodeHost"
            type="text"
            placeholder="103.232.94.2 或 https://vm.example.com/mcp"
            class="min-h-[36px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
          />
        </label>
        <label class="flex flex-col gap-1 text-xs text-fg-sub">
          <span>端口 (默认 9099)</span>
          <input
            v-model="nodePort"
            type="text"
            placeholder="9099"
            class="min-h-[36px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
          />
        </label>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
        <label class="flex flex-col gap-1 text-xs text-fg-sub sm:col-span-2">
          <div class="flex items-center justify-between">
            <span>通信密钥 Bearer Token</span>
            <button
              type="button"
              @click="generateToken"
              class="text-[11px] text-accent hover:underline cursor-pointer"
            >
              随机生成 64 位密钥
            </button>
          </div>
          <input
            v-model="nodeToken"
            type="password"
            placeholder="编辑时留空保持原密钥不变"
            class="min-h-[36px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
          />
        </label>
        <label class="flex flex-col gap-1 text-xs text-fg-sub">
          <span>允许访问的主站 IP</span>
          <input
            v-model="masterIp"
            type="text"
            placeholder="198.84.85.196"
            class="min-h-[36px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
          />
        </label>
      </div>

      <label class="flex flex-col gap-1 text-xs text-fg-sub">
        <span>节点用途说明 (可选，供 AI 判断何时调用此节点)</span>
        <input
          v-model="nodeDesc"
          type="text"
          placeholder="如：美西 2C/2G/110G 隔离沙盒，适合跑脚本、数据处理和抓取任务"
          class="min-h-[36px] rounded-xl border border-line bg-canvas px-3 py-1.5 text-xs text-fg focus:border-accent"
        />
      </label>

      <!-- Collapsible One-Click Go Agent Deploy Script -->
      <div class="rounded-xl border border-line bg-canvas p-2.5 flex flex-col gap-2">
        <div class="flex items-center justify-between gap-2">
          <button
            type="button"
            @click="showDeployScript = !showDeployScript"
            class="text-xs font-medium text-fg-sub hover:text-fg cursor-pointer text-left"
          >
            {{ showDeployScript ? '收起纯 Go 节点一键部署脚本' : '展开纯 Go 节点一键部署脚本 (换机 / 加新机通用)' }}
          </button>
          <button
            type="button"
            @click="copyDeployScript"
            class="px-2.5 py-1 rounded-lg border border-line bg-surface hover:border-accent text-[11px] text-fg-sub cursor-pointer shrink-0"
          >
            复制脚本
          </button>
        </div>
        <pre
          v-if="showDeployScript"
          class="max-h-48 overflow-auto rounded-lg bg-surface/60 p-2.5 font-mono text-[11px] text-fg-sub leading-relaxed whitespace-pre-wrap break-all"
        >{{ deployScript }}</pre>
      </div>

      <div class="flex items-center justify-end gap-2 pt-1">
        <button
          type="button"
          :disabled="saving"
          @click="saveNode(false)"
          class="min-h-[36px] px-3.5 py-1.5 rounded-xl border border-line bg-canvas hover:border-line-strong text-xs text-fg-sub cursor-pointer disabled:opacity-50"
        >
          仅保存
        </button>
        <button
          type="button"
          :disabled="saving"
          @click="saveNode(true)"
          class="min-h-[36px] px-4 py-1.5 rounded-xl bg-accent hover:bg-accent-hover text-on-accent font-semibold text-xs cursor-pointer disabled:opacity-50"
        >
          {{ saving ? '保存并探测中...' : (editingId ? '更新并连通测试' : '添加节点并连通测试') }}
        </button>
      </div>
    </div>
  </div>
</template>
