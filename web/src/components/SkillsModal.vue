<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { parseJsonRecord, parseJsonStringArray } from '../utils/format'
import { clearSearchOnEscape } from '../utils/topLayer'
import type { Skill, SkillParamField, MCPToolMeta } from '../types/bothub'
import BaseModal from './BaseModal.vue'
import SegmentedTabs from './SegmentedTabs.vue'
import SkillWebhookBuilder, { type HeaderRowItem } from './skills/SkillWebhookBuilder.vue'
import SkillTestBench from './skills/SkillTestBench.vue'
import SandboxNodesPanel from './skills/SandboxNodesPanel.vue'

const store = useAppStore()

const KIND_TABS = [
  { id: 'app_connector', label: '连接应用 (Apps)' },
  { id: 'sandbox_vm', label: '执行虚拟机 (VMs)' },
  { id: 'mcp', label: '标准 MCP' },
  { id: 'json_import', label: 'JSON 导入' },
  { id: 'webhook', label: 'Webhook' },
  { id: 'prompt', label: 'Playbook' },
]

const APP_CONNECTOR_TABS = [
  { id: 'mcp_app_github', label: 'GitHub', shortDesc: '仓库 / Issue / PR / Actions / 通知' },
  { id: 'mcp_app_email', label: 'Gmail / 邮箱', shortDesc: 'IMAP 收信 / 搜索邮件 / SMTP 发信' },
  { id: 'mcp_app_cloudflare', label: 'Cloudflare', shortDesc: '域名 Zone / DNS 解析 / 清理缓存' },
  { id: 'mcp_app_rss', label: 'RSS 订阅源', shortDesc: '博客与资讯源并发抓取 / 订阅管理' },
  { id: 'mcp_app_webdav', label: 'WebDAV 云盘', shortDesc: '坚果云 / Alist / Nextcloud 文件读写' },
  { id: 'mcp_app_space', label: 'Vesper Space', shortDesc: '个人画廊 / 5TB 图床直传 / 策展管理' },
]

const selectedAppId = ref('mcp_app_github')
const appConfigForm = ref<Record<string, string>>({})

const editingId = ref<string | null>(null)
const name = ref('')
const slug = ref('')
const kind = ref<'app_connector' | 'sandbox_vm' | 'mcp' | 'json_import' | 'webhook' | 'prompt'>('app_connector')
const description = ref('')
const instruction = ref('')
const httpMethod = ref('POST')
const httpUrl = ref('')
const headerRows = ref<HeaderRowItem[]>([])
const paramRows = ref<SkillParamField[]>([])
const enabled = ref(true)
const saving = ref(false)
const syncingId = ref<string | null>(null)
const mcpJsonInput = ref('')

const testSkillTarget = ref<Skill | null>(null)

const skills = computed(() => store.state.skills || [])
const skillSearch = ref('')

const activeAppSkill = computed(() =>
  skills.value.find((s) => s.id === selectedAppId.value) || null
)

function loadAppConnectorForm(appId: string) {
  selectedAppId.value = appId
  const sk = skills.value.find((s) => s.id === appId)
  if (!sk) {
    appConfigForm.value = {}
    return
  }
  appConfigForm.value = { ...parseJsonRecord(sk.http_headers) }
}

watch(
  [selectedAppId, () => store.state.showSkillsModal],
  ([appId, open]) => {
    if (open) {
      loadAppConnectorForm(appId)
    }
  },
  { immediate: true }
)

function fillSampleRSSFeeds() {
  const cur = (appConfigForm.value.feeds || '').trim()
  const samples = [
    'HackerNews精选|https://hnrss.org/frontpage',
    '阮一峰的网络日志|https://www.ruanyifeng.com/blog/atom.xml',
    'Go官方博客|https://go.dev/blog/feed.atom',
  ].join('\n')
  appConfigForm.value.feeds = cur ? `${cur}\n${samples}` : samples
}

async function saveAppConnector(andTest = false) {
  const appId = selectedAppId.value
  if (!appId || saving.value) return
  saving.value = true
  try {
    const cleanHeaders: Record<string, string> = {}
    for (const [k, v] of Object.entries(appConfigForm.value)) {
      const trimmed = (v || '').trim()
      if (trimmed) {
        cleanHeaders[k] = trimmed
      }
    }
    await BotHubApi.updateSkill(appId, {
      http_headers: JSON.stringify(cleanHeaders),
      enabled: true,
    })
    await store.refreshWorkspaceSlices(['skills'])
    loadAppConnectorForm(appId)
    const appLabel = APP_CONNECTOR_TABS.find((a) => a.id === appId)?.label || appId
    if (andTest) {
      syncingId.value = appId
      const res = await BotHubApi.testSkill(appId, {})
      store.notify(typeof res.result === 'string' ? res.result : `「${appLabel}」已连接并通过验证`)
    } else {
      store.notify(`已保存并启用「${appLabel}」连接器，聊天提到相关需求时将自动永久挂载`)
    }
  } catch (err: any) {
    store.notify(err.message || '连接器保存或验证失败')
  } finally {
    saving.value = false
    syncingId.value = null
  }
}

function addMcpHeaderRow() {
  headerRows.value.push({ key: '', value: '' })
}

function removeMcpHeaderRow(idx: number) {
  headerRows.value.splice(idx, 1)
}

function applyWebhookTemplate(mode: 'get' | 'post') {
  kind.value = 'webhook'
  if (mode === 'get') {
    httpMethod.value = 'GET'
    httpUrl.value = 'https://api.example.com/v1/lookup?q={{query}}'
    instruction.value = '按关键词查询外部 REST 接口并返回结果'
    paramRows.value = [{ key: 'query', description: '查询关键词', type: 'string', required: true }]
    return
  }
  httpMethod.value = 'POST'
  httpUrl.value = 'https://api.example.com/v1/webhook'
  instruction.value = '{"title": "{{title}}", "content": "{{content}}"}'
  headerRows.value = [{ key: 'Content-Type', value: 'application/json' }]
  paramRows.value = [
    { key: 'title', description: '标题或事件名', type: 'string', required: true },
    { key: 'content', description: '正文负载内容', type: 'string', required: false },
  ]
}

const filteredSkills = computed(() => {
  const q = skillSearch.value.trim().toLowerCase()
  if (!q) return skills.value
  return skills.value.filter(
    (sk) =>
      sk.name.toLowerCase().includes(q) ||
      sk.slug.toLowerCase().includes(q) ||
      (sk.description || '').toLowerCase().includes(q) ||
      (sk.http_url || '').toLowerCase().includes(q)
  )
})

function isAppConnectorSkill(sk: Skill): boolean {
  return sk.id.startsWith('mcp_app_')
}

function isAppConnectorConfigured(sk: Skill): boolean {
  const hdrs = parseJsonRecord(sk.http_headers)
  return Object.values(hdrs).some((v) => String(v || '').trim() !== '')
}

function mountedBotCount(skillId: string): number {
  return store.state.bots.filter((b) => parseJsonStringArray(b.skill_ids).includes(skillId)).length
}

function parseMCPTools(sk: Skill): MCPToolMeta[] {
  if (sk.kind !== 'builtin_mcp' && sk.kind !== 'mcp') return []
  try {
    const arr = JSON.parse(sk.params_schema || '[]')
    if (!Array.isArray(arr)) return []
    return arr.filter((t: any) => t && typeof t.name === 'string' && t.name.trim() !== '')
  } catch {
    return []
  }
}

function kindBadgeLabel(sk: Skill): string {
  if (isAppConnectorSkill(sk)) {
    return sk.enabled ? '已连接应用 MCP' : '待连接应用'
  }
  switch (sk.kind) {
    case 'builtin_mcp':
      return '内置业务 MCP'
    case 'mcp':
      return `标准 MCP · mcp_${sk.slug}`
    case 'webhook':
      return `${sk.http_method || 'GET'} skill_${sk.slug}`
    default:
      return 'Playbook'
  }
}

function resetForm() {
  editingId.value = null
  name.value = ''
  slug.value = ''
  kind.value = 'mcp'
  description.value = ''
  instruction.value = ''
  httpMethod.value = 'POST'
  httpUrl.value = ''
  headerRows.value = []
  paramRows.value = []
  enabled.value = true
}

function loadSkillToEdit(sk: Skill) {
  if (isAppConnectorSkill(sk)) {
    kind.value = 'app_connector'
    loadAppConnectorForm(sk.id)
    return
  }
  if (sk.kind === 'builtin_mcp') {
    store.notify('系统内置业务 MCP 无需修改连接参数，可直接启停或挂载给 Bot')
    return
  }
  editingId.value = sk.id
  name.value = sk.name
  slug.value = sk.slug
  kind.value = sk.kind === 'mcp' ? 'mcp' : sk.kind === 'webhook' ? 'webhook' : 'prompt'
  description.value = sk.description || ''
  instruction.value = sk.instruction || ''
  httpMethod.value = sk.http_method || (sk.kind === 'mcp' ? 'POST' : 'GET')
  httpUrl.value = sk.http_url || ''
  enabled.value = sk.enabled !== false

  headerRows.value = Object.entries(parseJsonRecord(sk.http_headers)).map(([k, v]) => ({
    key: k,
    value: v,
  }))

  if (sk.kind === 'webhook') {
    try {
      const pArr = JSON.parse(sk.params_schema || '[]')
      paramRows.value = Array.isArray(pArr)
        ? pArr.map((f: any) => ({
            key: String(f.key || ''),
            type: String(f.type || 'string'),
            description: String(f.description || ''),
            required: Boolean(f.required),
            default: String(f.default || ''),
          }))
        : []
    } catch {
      paramRows.value = []
    }
  } else {
    paramRows.value = []
  }
}

function cloneSkillToForm(sk: Skill) {
  if (sk.kind === 'builtin_mcp') return
  loadSkillToEdit(sk)
  editingId.value = null
  name.value = `${sk.name} (副本)`
  slug.value = `${sk.slug}_copy`
}

function buildPayload(): Partial<Skill> {
  const headersObj: Record<string, string> = {}
  for (const h of headerRows.value) {
    const k = h.key.trim()
    if (k) headersObj[k] = h.value.trim()
  }
  const actualKind = kind.value === 'json_import' || kind.value === 'app_connector' || kind.value === 'sandbox_vm' ? 'mcp' : kind.value
  const payload: Partial<Skill> = {
    name: name.value.trim(),
    slug: slug.value.trim(),
    kind: actualKind,
    description: description.value.trim(),
    instruction: instruction.value.trim(),
    http_method: actualKind === 'mcp' ? 'POST' : httpMethod.value,
    http_url: httpUrl.value.trim(),
    http_headers: JSON.stringify(headersObj),
    enabled: enabled.value,
  }
  if (actualKind === 'webhook') {
    const cleanParams = paramRows.value
      .map((p) => ({
        key: p.key.trim(),
        type: p.type || 'string',
        description: p.description.trim(),
        required: Boolean(p.required),
        default: (p.default || '').trim(),
      }))
      .filter((p) => p.key !== '')
    payload.params_schema = JSON.stringify(cleanParams)
  }
  return payload
}

async function handleImportJSON() {
  const raw = mcpJsonInput.value.trim()
  if (!raw || saving.value) return
  saving.value = true
  try {
    const res = await BotHubApi.importMCPServers(raw)
    mcpJsonInput.value = ''
    await store.refreshWorkspaceSlices(['skills'])
    store.notify(`已成功导入并探测 ${res.count || 0} 个 MCP 服务`)
  } catch (err: any) {
    store.notify(err.message || '导入 MCP JSON 配置失败')
  } finally {
    saving.value = false
  }
}

async function handleSave() {
  if (kind.value === 'sandbox_vm') {
    return
  }
  if (kind.value === 'app_connector') {
    await saveAppConnector(false)
    return
  }
  if (kind.value === 'json_import') {
    await handleImportJSON()
    return
  }
  if (!name.value.trim() || saving.value) return
  if ((kind.value === 'mcp' || kind.value === 'webhook') && !httpUrl.value.trim()) {
    store.notify(kind.value === 'mcp' ? '标准 MCP 服务必须填写 Endpoint URL' : 'Webhook 外部工具必须填写请求 URL')
    return
  }
  saving.value = true
  try {
    const payload = buildPayload()
    if (editingId.value) {
      await BotHubApi.updateSkill(editingId.value, payload)
      store.notify(`已更新 MCP / 扩展「${payload.name}」`)
    } else {
      await BotHubApi.createSkill(payload)
      store.notify(`已创建 MCP / 扩展「${payload.name}」`)
    }
    resetForm()
    await store.refreshWorkspaceSlices(['skills'])
  } catch (err: any) {
    store.notify(err.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function syncMCPTools(sk: Skill) {
  if (syncingId.value) return
  syncingId.value = sk.id
  try {
    const res = await BotHubApi.testSkill(sk.id, {})
    await store.refreshWorkspaceSlices(['skills'])
    store.notify(typeof res.result === 'string' ? res.result : `已验证「${sk.name}」`)
  } catch (err: any) {
    store.notify(err.message || '连接验证或同步 tools/list 失败')
  } finally {
    syncingId.value = null
  }
}

async function toggleSkillEnabled(sk: Skill) {
  if (isAppConnectorSkill(sk) && !sk.enabled && !isAppConnectorConfigured(sk)) {
    kind.value = 'app_connector'
    loadAppConnectorForm(sk.id)
    store.notify(`请先在左侧填写「${sk.name}」凭证后再启用`)
    return
  }
  try {
    await BotHubApi.updateSkill(sk.id, { enabled: !sk.enabled })
    await store.refreshWorkspaceSlices(['skills'])
    store.notify(!sk.enabled ? `已启用「${sk.name}」` : `已停用「${sk.name}」`)
  } catch (err: any) {
    store.notify(err.message || '切换状态失败')
  }
}

async function handleDelete(sk: Skill) {
  if (sk.kind === 'builtin_mcp') return
  try {
    await BotHubApi.deleteSkill(sk.id)
    if (editingId.value === sk.id) resetForm()
    if (testSkillTarget.value?.id === sk.id) {
      testSkillTarget.value = null
    }
    await store.refreshWorkspaceSlices(['skills'])
    store.notify(`已删除「${sk.name}」`)
  } catch (err: any) {
    store.notify(err.message || '删除失败')
  }
}

function closeModal() {
  store.state.showSkillsModal = false
}

watch(
  () => store.state.showSkillsModal,
  (open) => {
    if (!open) {
      testSkillTarget.value = null
    }
  }
)
</script>

<template>
  <BaseModal
    :open="store.state.showSkillsModal"
    title="MCP 服务与应用连接器中心 (MCP & Connected Apps)"
    subtitle="原生底座（联网搜索 / 网页读取 / 记忆 / 时间计算）所有 Bot 默认自带；在此连接 GitHub / Gmail / Cloudflare / RSS / WebDAV 或接入远程 MCP，聊天提及相关需求时 Bot 自动挂载并永久保留"
    max-width="3xl"
    @close="closeModal"
  >
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-4 min-h-0 flex-1 overflow-y-auto pr-0.5">
      <!-- Left: App Connector Configurator / MCP Creation Form -->
      <form
        @submit.prevent="handleSave"
        class="lg:col-span-6 rounded-xl border border-line bg-canvas p-4 flex flex-col gap-3 h-fit"
      >
        <div class="flex items-center justify-between gap-2">
          <h4 class="text-xs font-semibold text-fg">
            {{
              kind === 'app_connector'
                ? '一键连接常用外部应用 (全读写能力)'
                : editingId
                  ? '编辑 MCP / 扩展服务'
                  : '接入新 MCP 服务或扩展'
            }}
          </h4>
          <button
            v-if="editingId && kind !== 'app_connector'"
            type="button"
            @click="resetForm"
            class="text-xs text-accent-fg hover:underline cursor-pointer"
          >
            取消编辑
          </button>
        </div>

        <SegmentedTabs v-model="kind" :items="KIND_TABS" aria-label="MCP 与连接器模式" />

        <!-- Mode 1: Built-in Connected Apps (GitHub / Gmail / Cloudflare / RSS / WebDAV) -->
        <template v-if="kind === 'app_connector'">
          <div class="grid grid-cols-2 sm:grid-cols-3 gap-1.5">
            <button
              v-for="app in APP_CONNECTOR_TABS"
              :key="app.id"
              type="button"
              @click="loadAppConnectorForm(app.id)"
              :class="[
                'rounded-xl border px-2.5 py-2 text-left transition-colors cursor-pointer flex flex-col gap-0.5',
                selectedAppId === app.id
                  ? 'border-accent bg-accent-soft text-accent-fg'
                  : 'border-line bg-canvas hover:border-line-strong text-fg-sub',
              ]"
            >
              <div class="flex items-center justify-between gap-1">
                <span class="text-xs font-semibold truncate">{{ app.label }}</span>
                <span
                  :class="[
                    'w-2 h-2 rounded-full shrink-0',
                    skills.find((s) => s.id === app.id)?.enabled ? 'bg-ok' : 'bg-line-strong',
                  ]"
                />
              </div>
              <span class="text-[10px] text-fg-muted truncate">{{ app.shortDesc }}</span>
            </button>
          </div>

          <!-- 1A. GitHub Connector Form -->
          <div v-if="selectedAppId === 'mcp_app_github'" class="rounded-xl border border-line bg-surface/30 p-3.5 flex flex-col gap-2.5">
            <div class="flex items-center justify-between">
              <strong class="text-xs font-semibold text-fg">GitHub 账号与仓库连接配置</strong>
              <span
                :class="[
                  'text-[11px] px-2 py-0.5 rounded-full border',
                  activeAppSkill?.enabled ? 'border-ok-border bg-ok-soft text-ok' : 'border-line text-fg-muted',
                ]"
              >
                {{ activeAppSkill?.enabled ? '已连接启用' : '未连接' }}
              </span>
            </div>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>GitHub Personal Access Token (ghp_... / github_pat_...)</span>
              <input
                v-model="appConfigForm.token"
                type="password"
                placeholder="ghp_xxxxxxxxxxxxxxxxxxxx"
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
              />
            </label>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>默认 GitHub 用户名 / 组织 (可选，如 YN1753)</span>
              <input
                v-model="appConfigForm.default_owner"
                type="text"
                placeholder="YN1753"
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
              />
            </label>
            <p class="text-[11px] text-fg-muted leading-relaxed">
              支持查询代码仓库、查看或新建 Issue/评论、检索 PR、查看并一键重跑失败的 GitHub Actions 流水线以及读取未读通知。
            </p>
          </div>

          <!-- 1B. Gmail / Generic Email (IMAP + SMTP) Connector Form -->
          <div v-else-if="selectedAppId === 'mcp_app_email'" class="rounded-xl border border-line bg-surface/30 p-3.5 flex flex-col gap-2.5">
            <div class="flex items-center justify-between">
              <strong class="text-xs font-semibold text-fg">Gmail 与通用 IMAP / SMTP 邮箱配置</strong>
              <span
                :class="[
                  'text-[11px] px-2 py-0.5 rounded-full border',
                  activeAppSkill?.enabled ? 'border-ok-border bg-ok-soft text-ok' : 'border-line text-fg-muted',
                ]"
              >
                {{ activeAppSkill?.enabled ? '已连接启用' : '未连接' }}
              </span>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <label class="flex flex-col gap-1 text-xs text-fg-sub">
                <span>邮箱地址 (Email)</span>
                <input
                  v-model="appConfigForm.email"
                  type="email"
                  placeholder="yourname@gmail.com"
                  class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
                />
              </label>
              <label class="flex flex-col gap-1 text-xs text-fg-sub">
                <span>应用专用密码 / 授权码</span>
                <input
                  v-model="appConfigForm.password"
                  type="password"
                  placeholder="Gmail 16 位应用专用密码"
                  class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
                />
              </label>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <label class="flex flex-col gap-1 text-xs text-fg-sub">
                <span>发件人名称 (可选)</span>
                <input
                  v-model="appConfigForm.sender_name"
                  type="text"
                  placeholder="Vesper Assistant"
                  class="min-h-[36px] rounded-xl border border-line bg-canvas px-2.5 py-1 text-xs text-fg focus:border-accent"
                />
              </label>
              <label class="flex flex-col gap-1 text-xs text-fg-sub">
                <span>IMAP 服务器 (留空自动识别)</span>
                <input
                  v-model="appConfigForm.imap_host"
                  type="text"
                  placeholder="imap.gmail.com:993"
                  class="min-h-[36px] rounded-xl border border-line bg-canvas px-2.5 py-1 font-mono text-xs text-fg focus:border-accent"
                />
              </label>
              <label class="flex flex-col gap-1 text-xs text-fg-sub">
                <span>SMTP 服务器 (留空自动识别)</span>
                <input
                  v-model="appConfigForm.smtp_host"
                  type="text"
                  placeholder="smtp.gmail.com:465"
                  class="min-h-[36px] rounded-xl border border-line bg-canvas px-2.5 py-1 font-mono text-xs text-fg focus:border-accent"
                />
              </label>
            </div>
            <p class="text-[11px] text-fg-muted leading-relaxed">
              自动识别 Gmail、Outlook、QQ、163 及自定义企业域名邮箱；支持读取未读邮件、按发件人/主题搜索邮件、查看邮件全文及通过 SMTP 发送邮件。
            </p>
          </div>

          <!-- 1C. Cloudflare Connector Form -->
          <div v-else-if="selectedAppId === 'mcp_app_cloudflare'" class="rounded-xl border border-line bg-surface/30 p-3.5 flex flex-col gap-2.5">
            <div class="flex items-center justify-between">
              <strong class="text-xs font-semibold text-fg">Cloudflare 域名与 DNS 连接配置</strong>
              <span
                :class="[
                  'text-[11px] px-2 py-0.5 rounded-full border',
                  activeAppSkill?.enabled ? 'border-ok-border bg-ok-soft text-ok' : 'border-line text-fg-muted',
                ]"
              >
                {{ activeAppSkill?.enabled ? '已连接启用' : '未连接' }}
              </span>
            </div>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>Cloudflare API Token (需包含 Zone 与 DNS 权限)</span>
              <input
                v-model="appConfigForm.api_token"
                type="password"
                placeholder="填入 Cloudflare API Token"
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
              />
            </label>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>默认主域名或 Zone ID (可选，如 starryheavens.uk)</span>
              <input
                v-model="appConfigForm.default_zone"
                type="text"
                placeholder="starryheavens.uk"
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
              />
            </label>
            <p class="text-[11px] text-fg-muted leading-relaxed">
              支持列出所有域名 Zone、查询/新增/修改/删除 A/CNAME/TXT 等 DNS 解析记录以及一键清理域名 CDN 缓存。
            </p>
          </div>

          <!-- 1D. RSS Feed Subscription Connector Form -->
          <div v-else-if="selectedAppId === 'mcp_app_rss'" class="rounded-xl border border-line bg-surface/30 p-3.5 flex flex-col gap-2.5">
            <div class="flex items-center justify-between gap-2">
              <strong class="text-xs font-semibold text-fg">RSS / Atom 资讯订阅源列表</strong>
              <button
                type="button"
                @click="fillSampleRSSFeeds"
                class="text-xs text-accent-fg hover:underline cursor-pointer"
              >
                + 填入常用科技源示例
              </button>
            </div>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>每行一个订阅源（支持「名称|https://...」或直接填 URL）</span>
              <textarea
                v-model="appConfigForm.feeds"
                rows="5"
                placeholder="HackerNews精选|https://hnrss.org/frontpage&#10;Go官方博客|https://go.dev/blog/feed.atom"
                class="rounded-xl border border-line bg-canvas px-3 py-2 font-mono text-xs text-fg focus:border-accent leading-relaxed"
              />
            </label>
            <p class="text-[11px] text-fg-muted leading-relaxed">
              绑定后可在聊天或定时早报推送中让 Bot 直接并发抓取你订阅源的最新文章，也可以随时在对话中让 Bot 帮你动态增删订阅源。
            </p>
          </div>

          <!-- 1E. WebDAV / Alist Connector Form -->
          <div v-else-if="selectedAppId === 'mcp_app_webdav'" class="rounded-xl border border-line bg-surface/30 p-3.5 flex flex-col gap-2.5">
            <div class="flex items-center justify-between">
              <strong class="text-xs font-semibold text-fg">WebDAV / Alist / 坚果云 / Nextcloud 配置</strong>
              <span
                :class="[
                  'text-[11px] px-2 py-0.5 rounded-full border',
                  activeAppSkill?.enabled ? 'border-ok-border bg-ok-soft text-ok' : 'border-line text-fg-muted',
                ]"
              >
                {{ activeAppSkill?.enabled ? '已连接启用' : '未连接' }}
              </span>
            </div>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>WebDAV 服务端根目录 URL (Base URL)</span>
              <input
                v-model="appConfigForm.base_url"
                type="url"
                placeholder="https://dav.jianguoyun.com/dav/ 或 https://alist.example.com/dav/"
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
              />
            </label>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <label class="flex flex-col gap-1 text-xs text-fg-sub">
                <span>账号 (Username)</span>
                <input
                  v-model="appConfigForm.username"
                  type="text"
                  placeholder="WebDAV 用户名"
                  class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
                />
              </label>
              <label class="flex flex-col gap-1 text-xs text-fg-sub">
                <span>密码 / 应用密码 (Password)</span>
                <input
                  v-model="appConfigForm.password"
                  type="password"
                  placeholder="WebDAV 密码"
                  class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
                />
              </label>
            </div>
            <p class="text-[11px] text-fg-muted leading-relaxed">
              支持浏览云端目录、读取云盘内的 Markdown/文本文档，以及让 Bot 将整理好的报告或笔记直接写入云盘保存。
            </p>
          </div>

          <!-- 1F. Vesper Space Gallery & 5TB Imagebed Connector Form -->
          <div v-else-if="selectedAppId === 'mcp_app_space'" class="rounded-xl border border-line bg-surface/30 p-3.5 flex flex-col gap-2.5">
            <div class="flex items-center justify-between">
              <strong class="text-xs font-semibold text-fg">Vesper Space 个人美术馆与 5TB 图床配置</strong>
              <span
                :class="[
                  'text-[11px] px-2 py-0.5 rounded-full border',
                  activeAppSkill?.enabled ? 'border-ok-border bg-ok-soft text-ok' : 'border-line text-fg-muted',
                ]"
              >
                {{ activeAppSkill?.enabled ? '已连接启用' : '未连接' }}
              </span>
            </div>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>Vesper Space 站点地址 (Base URL)</span>
              <input
                v-model="appConfigForm.base_url"
                type="url"
                placeholder="https://space.starryheavens.uk"
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
              />
            </label>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <label class="flex flex-col gap-1 text-xs text-fg-sub">
                <span>Internal Bridge Token (INTERNAL_BRIDGE_TOKEN)</span>
                <input
                  v-model="appConfigForm.bridge_token"
                  type="password"
                  placeholder="vs_bridge_..."
                  class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
                />
              </label>
              <label class="flex flex-col gap-1 text-xs text-fg-sub">
                <span>默认创作者署名 (可选，如 迟暮)</span>
                <input
                  v-model="appConfigForm.default_artist"
                  type="text"
                  placeholder="迟暮"
                  class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 text-xs text-fg focus:border-accent"
                />
              </label>
            </div>
            <p class="text-[11px] text-fg-muted leading-relaxed">
              支持查询画廊展览统计、检索作品、将网络图片或 Base64 图像一键归档至 5TB Google Drive (/space) 图床并返回永久直链与 Markdown，以及修订策展手记与管理评论。
            </p>
          </div>

          <div class="flex items-center justify-end gap-2 pt-1">
            <button
              type="button"
              :disabled="saving"
              @click="saveAppConnector(true)"
              class="min-h-[38px] px-3.5 py-1.5 rounded-xl border border-info-border bg-info-soft text-info font-medium text-xs cursor-pointer disabled:opacity-50"
            >
              {{ syncingId === selectedAppId ? '正在验证连接...' : '保存并测试连通性' }}
            </button>
            <button
              type="submit"
              :disabled="saving"
              class="min-h-[38px] px-4 py-1.5 rounded-xl bg-accent hover:bg-accent-hover text-on-accent font-semibold text-xs cursor-pointer disabled:opacity-50"
            >
              {{ saving ? '保存中...' : '保存并连接应用' }}
            </button>
          </div>
        </template>

        <!-- Mode 1.5: Multi-VM Remote Execution Sandbox Nodes (Pure Go Agent) -->
        <template v-else-if="kind === 'sandbox_vm'">
          <SandboxNodesPanel />
        </template>

        <!-- Mode 2: JSON Import Mode -->
        <template v-else-if="kind === 'json_import'">
          <label class="flex flex-col gap-1.5 text-xs text-fg-sub">
            <span>粘贴标准 mcpServers JSON 配置 (支持 Claude / Cursor / Muse 格式)</span>
            <textarea
              v-model="mcpJsonInput"
              rows="8"
              placeholder='{
  "mcpServers": {
    "deepwiki": {
      "url": "https://mcp.deepwiki.com/mcp",
      "description": "开源仓库架构知识库检索",
      "headers": {
        "Authorization": "Bearer YOUR_TOKEN"
      }
    }
  }
}'
              class="rounded-xl border border-line bg-canvas px-3 py-2 font-mono text-xs text-fg focus:border-accent leading-relaxed"
            />
          </label>
          <p class="text-[11px] text-fg-muted leading-relaxed">
            支持一次性粘贴多个远程 HTTP/SSE MCP Server。导入时系统将自动发起 JSON-RPC 2.0 握手并同步每个 MCP 暴露的 Tools 列表。
          </p>
          <div class="flex justify-end pt-1">
            <button
              type="submit"
              :disabled="saving || !mcpJsonInput.trim()"
              class="min-h-[38px] px-4 py-1.5 rounded-xl bg-accent hover:bg-accent-hover text-on-accent font-semibold text-xs cursor-pointer disabled:opacity-50"
            >
              {{ saving ? '正在握手导入...' : '一键解析并导入 MCP' }}
            </button>
          </div>
        </template>

        <!-- Mode 3: Form Mode (Standard Remote MCP / Webhook / Prompt) -->
        <template v-else>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>{{ kind === 'mcp' ? 'MCP 服务名称' : '扩展名称' }}</span>
              <input
                v-model="name"
                type="text"
                :placeholder="kind === 'mcp' ? '如 DeepWiki / 搜索 MCP' : '如 天气查询 / 晨间复盘'"
                required
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 text-xs text-fg focus:border-accent"
              />
            </label>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>标识符 Slug (可选)</span>
              <input
                v-model="slug"
                type="text"
                placeholder="自动生成如 deepwiki"
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
              />
            </label>
          </div>

          <label class="flex flex-col gap-1 text-xs text-fg-sub">
            <span>功能与触发场景描述 (Bot 聊天时靠它自动发现并挂载给自己)</span>
            <input
              v-model="description"
              type="text"
              placeholder="简洁说明该 MCP 提供什么能力、聊到什么话题时应该自动挂载"
              class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 text-xs text-fg focus:border-accent"
            />
          </label>

          <!-- Standard Remote MCP Server Builder -->
          <div v-if="kind === 'mcp'" class="flex flex-col gap-2.5">
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>MCP Server Endpoint URL (Streamable HTTP / JSON-RPC 2.0 / SSE)</span>
              <input
                v-model="httpUrl"
                type="url"
                placeholder="https://mcp.example.com/mcp"
                required
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
              />
            </label>

            <div class="rounded-xl border border-line bg-surface/40 p-3 flex flex-col gap-2">
              <div class="flex items-center justify-between">
                <span class="text-xs font-medium text-fg-sub">自定义鉴权请求头 Headers (可选)</span>
                <button
                  type="button"
                  @click="addMcpHeaderRow"
                  class="text-xs text-accent-fg hover:underline cursor-pointer"
                >
                  + 添加 Header
                </button>
              </div>
              <div v-if="headerRows.length === 0" class="text-[11px] text-fg-muted">
                无需鉴权的公开 MCP 服务可留空；如需 Bearer Token 可添加 Authorization 请求头。
              </div>
              <div
                v-for="(row, idx) in headerRows"
                :key="idx"
                class="grid grid-cols-12 gap-1.5 items-center"
              >
                <input
                  v-model="row.key"
                  type="text"
                  placeholder="Authorization"
                  class="col-span-5 min-h-[32px] rounded-lg border border-line bg-canvas px-2 font-mono text-xs text-fg"
                />
                <input
                  v-model="row.value"
                  type="text"
                  placeholder="Bearer sk-..."
                  class="col-span-6 min-h-[32px] rounded-lg border border-line bg-canvas px-2 font-mono text-xs text-fg"
                />
                <button
                  type="button"
                  @click="removeMcpHeaderRow(idx)"
                  class="col-span-1 text-xs text-danger hover:underline cursor-pointer text-center"
                >
                  删
                </button>
              </div>
            </div>
          </div>

          <!-- Prompt Playbook Content -->
          <label v-else-if="kind === 'prompt'" class="flex flex-col gap-1 text-xs text-fg-sub">
            <span>指令正文 / 行动准则</span>
            <textarea
              v-model="instruction"
              rows="4"
              placeholder="写入具体的工作流步骤、说话风格、输出要求或判断原则；挂载后将注入该 Bot 的能力上下文。"
              class="auto-size-textarea rounded-xl border border-line bg-canvas px-3 py-2 text-xs text-fg focus:border-accent leading-relaxed"
            />
          </label>

          <!-- Webhook / API Tool Builder -->
          <SkillWebhookBuilder
            v-else
            v-model:http-method="httpMethod"
            v-model:http-url="httpUrl"
            v-model:instruction="instruction"
            v-model:param-rows="paramRows"
            v-model:header-rows="headerRows"
            @apply-template="applyWebhookTemplate"
          />

          <div class="flex items-center justify-between pt-1">
            <label class="inline-flex items-center gap-2 text-xs text-fg-sub cursor-pointer">
              <input v-model="enabled" type="checkbox" class="accent-accent" />
              <span>启用此服务</span>
            </label>
            <button
              type="submit"
              :disabled="saving"
              class="min-h-[38px] px-4 py-1.5 rounded-xl bg-accent hover:bg-accent-hover text-on-accent font-semibold text-xs cursor-pointer disabled:opacity-50"
            >
              {{ saving ? '保存并探测中...' : editingId ? '保存修改' : kind === 'mcp' ? '接入并同步 MCP' : '创建扩展' }}
            </button>
          </div>
        </template>
      </form>

      <!-- Right: MCP Registry & Live Discovery -->
      <div class="lg:col-span-6 flex flex-col gap-3">
        <div class="rounded-xl border border-line bg-canvas p-4 flex flex-col gap-2.5">
          <div class="flex items-center justify-between gap-2 flex-wrap">
            <h4 class="text-xs font-semibold text-fg">
              全部 MCP 服务与应用连接器 ({{ filteredSkills.length }}<span v-if="skillSearch.trim()"> / {{ skills.length }}</span>)
            </h4>
            <span class="text-xs text-fg-muted">聊天提及相关需求时 Bot 自动挂载并永久保留</span>
          </div>

          <input
            v-if="skills.length > 0"
            v-model="skillSearch"
            type="search"
            aria-label="筛选 MCP 服务"
            placeholder="搜索应用名称、MCP、工具或描述..."
            @keydown.escape="clearSearchOnEscape($event, skillSearch, () => { skillSearch = '' })"
            class="w-full min-h-[34px] rounded-lg border border-line bg-canvas px-2.5 text-xs text-fg placeholder:text-fg-muted focus:border-accent focus:outline-none"
          />

          <div v-if="skills.length === 0" class="py-8 text-center text-xs text-fg-muted leading-relaxed">
            当前暂无已注册的 MCP 服务。你可以在左侧连接 GitHub / Gmail / Cloudflare 等应用，或接入标准远程 MCP Server。
          </div>
          <div v-else-if="filteredSkills.length === 0" class="py-6 text-center text-xs text-fg-muted">
            没有匹配「{{ skillSearch }}」的 MCP 服务
          </div>

          <article
            v-for="sk in filteredSkills"
            :key="sk.id"
            class="rounded-xl border border-line bg-canvas p-3 flex flex-col gap-2"
          >
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2 flex-wrap">
                  <strong class="text-xs font-semibold text-fg">{{ sk.name }}</strong>
                  <span
                    :class="[
                      'px-1.5 py-0.5 rounded font-mono text-[11px] border',
                      isAppConnectorSkill(sk)
                        ? sk.enabled
                          ? 'border-ok-border bg-ok-soft text-ok'
                          : 'border-line bg-surface text-fg-muted'
                        : sk.kind === 'builtin_mcp'
                          ? 'border-accent-border bg-accent-soft text-accent-fg'
                          : sk.kind === 'mcp' || sk.kind === 'webhook'
                            ? 'border-info-border bg-info-soft text-info'
                            : 'border-line bg-surface text-fg-sub',
                    ]"
                  >
                    {{ kindBadgeLabel(sk) }}
                  </span>
                  <span
                    v-if="mountedBotCount(sk.id) > 0"
                    class="px-1.5 py-0.5 rounded border border-ok-border bg-ok-soft text-[11px] text-ok"
                  >
                    {{ mountedBotCount(sk.id) }} 个 Bot 已挂载
                  </span>
                  <span v-if="!sk.enabled && !isAppConnectorSkill(sk)" class="text-xs text-fg-muted">已停用</span>
                </div>
                <p v-if="sk.description" class="text-xs text-fg-muted mt-1 leading-relaxed">
                  {{ sk.description }}
                </p>
                <div v-if="sk.kind === 'mcp' || sk.kind === 'webhook'" class="text-[11px] font-mono text-fg-muted mt-1 truncate">
                  {{ sk.http_url }}
                </div>

                <!-- Discovered / Bundled MCP Tools Chips -->
                <div v-if="parseMCPTools(sk).length > 0" class="flex items-center gap-1.5 flex-wrap mt-2">
                  <span class="text-[11px] text-fg-muted">Tools ({{ parseMCPTools(sk).length }}):</span>
                  <span
                    v-for="t in parseMCPTools(sk)"
                    :key="t.name"
                    :title="t.description"
                    class="px-1.5 py-0.5 rounded bg-surface border border-line font-mono text-[11px] text-fg-sub"
                  >
                    {{ t.name }}
                  </span>
                </div>
              </div>
            </div>

            <div class="flex items-center justify-end gap-1.5 flex-wrap pt-1 border-t border-line">
              <button
                v-if="isAppConnectorSkill(sk)"
                type="button"
                @click="loadSkillToEdit(sk)"
                class="px-2.5 py-1 rounded-lg border border-accent-border text-xs text-accent-fg hover:bg-accent-soft cursor-pointer"
              >
                配置凭证
              </button>
              <button
                v-if="sk.kind === 'mcp' || sk.kind === 'builtin_mcp'"
                type="button"
                :disabled="syncingId === sk.id"
                @click="syncMCPTools(sk)"
                class="px-2.5 py-1 rounded-lg border border-info-border text-xs text-info hover:bg-info-soft cursor-pointer disabled:opacity-50"
              >
                {{
                  syncingId === sk.id
                    ? '验证中...'
                    : isAppConnectorSkill(sk)
                      ? '测试连接'
                      : sk.kind === 'mcp'
                        ? '探测同步 Tools'
                        : '校验状态'
                }}
              </button>
              <button
                v-if="sk.kind === 'webhook'"
                type="button"
                @click="testSkillTarget = sk"
                class="px-2.5 py-1 rounded-lg border border-info-border text-xs text-info hover:bg-info-soft cursor-pointer"
              >
                实时调试
              </button>
              <button
                type="button"
                @click="toggleSkillEnabled(sk)"
                class="px-2.5 py-1 rounded-lg border border-line text-xs text-fg-sub hover:text-fg cursor-pointer"
              >
                {{ sk.enabled ? '停用' : '启用' }}
              </button>
              <template v-if="sk.kind !== 'builtin_mcp'">
                <button
                  type="button"
                  @click="cloneSkillToForm(sk)"
                  class="px-2.5 py-1 rounded-lg border border-line text-xs text-fg-sub hover:text-info cursor-pointer"
                >
                  复用新建
                </button>
                <button
                  type="button"
                  @click="loadSkillToEdit(sk)"
                  class="px-2.5 py-1 rounded-lg border border-line text-xs text-fg-sub hover:text-accent-fg cursor-pointer"
                >
                  编辑
                </button>
                <button
                  type="button"
                  @click="handleDelete(sk)"
                  class="px-2.5 py-1 rounded-lg border border-danger-border text-xs text-danger hover:bg-danger-soft cursor-pointer"
                >
                  删除
                </button>
              </template>
            </div>
          </article>
        </div>

        <!-- Webhook Live Test Box -->
        <SkillTestBench
          v-if="testSkillTarget"
          :skill="testSkillTarget"
          @close="testSkillTarget = null"
        />
      </div>
    </div>
  </BaseModal>
</template>
