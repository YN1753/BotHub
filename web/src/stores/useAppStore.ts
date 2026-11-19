import { reactive, computed } from 'vue'
import { BotHubApi } from '../api/client'
import { playSubtleChime as playAudioChime } from '../utils/audio'
import { mapServerMessage, mapServerMessages, type ChatMessageUI } from '../utils/chatMapper'
import { parseJsonRecord } from '../utils/format'
import type {
  AgentMemory,
  AgentMessage,
  Asset,
  AssetCategoryOption,
  AvailableToolMeta,
  Bot,
  BotExecutionLog,
  BotThread,
  ModelHealthStats,
  ModelOptionDTO,
  ModelPoolEntry,
  ModelProvider,
  ModelUsageSummary,
  NotificationItem,
  PoolStrategies,
  PooledModel,
  PushLog,
  SelectOption,
  Skill,
  StashItem,
  StashTagCount,
  Task,
  TaskTemplate,
  TodoItem,
  WorkNote,
} from '../types/bothub'

export { mapServerMessages, type ChatMessageUI }

export type MainWorkspace = 'bots' | 'ops'
export type BotSubView = 'stream' | 'routines' | 'config' | 'activity'

export interface AppDialogRequest {
  id: number
  kind: 'confirm' | 'prompt'
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
  defaultValue?: string
  placeholder?: string
}

export interface ConfirmDialogOptions {
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
}

export interface PromptDialogOptions {
  title?: string
  message: string
  defaultValue?: string
  placeholder?: string
  confirmText?: string
  cancelText?: string
}

const TOAST_DURATION_MS = 3200
const FASTBOOT_STORAGE_KEY = 'bothub_fastboot_v1'

interface FastBootSnapshot {
  version?: string
  bots?: Bot[]
  notifications?: NotificationItem[]
  assets?: Asset[]
  assetCategories?: AssetCategoryOption[]
  tasks?: Task[]
  stash?: StashItem[]
  activeBotId?: string
  timelinesByBot?: Record<string, ChatMessageUI[]>
}

function loadFastBootSnapshot(): FastBootSnapshot | null {
  try {
    const raw = localStorage.getItem(FASTBOOT_STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as FastBootSnapshot
    if (!parsed || !Array.isArray(parsed.bots)) return null
    return parsed
  } catch {
    return null
  }
}

const initialSnap = typeof window !== 'undefined' ? loadFastBootSnapshot() : null
const timelinesByBot: Record<string, ChatMessageUI[]> = initialSnap?.timelinesByBot
  ? { ...initialSnap.timelinesByBot }
  : {}
const initialActiveBotId =
  (typeof window !== 'undefined' && localStorage.getItem('bothub_active_bot')) ||
  initialSnap?.activeBotId ||
  initialSnap?.bots?.[0]?.id ||
  ''

const state = reactive({
  authChecked: Boolean(initialSnap),
  authenticated: Boolean(initialSnap),
  oidcEnabled: false,
  oidcProviderName: 'Vesper Auth',
  version: initialSnap?.version || 'v2.46.0',
  commit: '',
  buildTime: '',
  uptimeSec: 0,
  stats: {
    sent_pushes: 0,
  },
  bots: (initialSnap?.bots || []) as Bot[],
  skills: [] as Skill[],
  todos: [] as TodoItem[],
  notes: [] as WorkNote[],
  stash: (initialSnap?.stash || []) as StashItem[],
  stashTags: [] as StashTagCount[],
  modelPool: [] as ModelPoolEntry[],
  modelProviders: [] as ModelProvider[],
  pooledModels: [] as PooledModel[],
  poolStrategies: { free: 'priority', paid: 'priority' } as PoolStrategies,
  modelUsageSummary: {
    days: 7,
    total_calls: 0,
    success_calls: 0,
    free_calls: 0,
    paid_calls: 0,
    free_tokens: 0,
    paid_tokens: 0,
    top_models: [],
  } as ModelUsageSummary,
  modelOptions: [] as ModelOptionDTO[],
  revisions: {} as Record<string, number>,
  availableTools: [] as AvailableToolMeta[],
  modelHealth: {} as Record<string, ModelHealthStats>,
  assets: (initialSnap?.assets || []) as Asset[],
  assetCategories: (initialSnap?.assetCategories || []) as AssetCategoryOption[],
  tasks: (initialSnap?.tasks || []) as Task[],
  templates: [] as TaskTemplate[],
  memories: [] as AgentMemory[],
  notifications: (initialSnap?.notifications || []) as NotificationItem[],
  pushLogs: [] as PushLog[],
  configs: {} as Record<string, string>,

  // Navigation & Active Bot Workspace
  workspace: 'bots' as MainWorkspace,
  activeBotId: initialActiveBotId,
  activeThreadId: '',
  botThreads: [] as BotThread[],
  botMemories: [] as AgentMemory[],
  botExecutions: [] as BotExecutionLog[],
  botView: 'stream' as BotSubView,
  timeline: (initialActiveBotId && timelinesByBot[initialActiveBotId]
    ? [...timelinesByBot[initialActiveBotId]]
    : []) as ChatMessageUI[],
  isStreaming: false,
  sseConnected: false,
  soundEnabled: localStorage.getItem('bothub_sound') === '1',
  ambientMotionPaused: localStorage.getItem('bothub_ambient_paused') === '1',
  draftsByBot: (() => {
    try {
      const persisted = localStorage.getItem('bothub_drafts_v2') || sessionStorage.getItem('bothub_drafts')
      return parseJsonRecord(persisted)
    } catch {
      return {} as Record<string, string>
    }
  })(),

  // Modals & Drawers
  showInboxModal: false,
  showWorkhubModal: false,
  workhubTab: 'todos' as 'todos' | 'notes' | 'stash' | 'briefing',
  showMemoriesModal: false,
  showSkillsModal: false,
  showLogsModal: false,
  showSettingsModal: false,
  showCreateBotModal: false,
  showGroupDrawer: false,
  showQuickSwitcher: false,
  memoryModalDefaultScope: '',
  forwardingMessage: null as { fromName: string; content: string } | null,
  createModalMode: 'bot' as 'bot' | 'group',
  pendingDraftPrompt: '',
  pendingNoteId: '',
  editingAsset: undefined as Asset | null | undefined, // undefined = closed, null = new
  editingTask: undefined as Task | Partial<Task> | null | undefined,

  // Toast & Dialog
  toastMessage: '',
  toastSeq: 0,
  dialogRequest: null as AppDialogRequest | null,
})

let dialogSeq = 0
let activeDialogResolve: ((result: boolean | string | null) => void) | null = null

function openConfirm(opts: string | ConfirmDialogOptions): Promise<boolean> {
  const normalized: ConfirmDialogOptions = typeof opts === 'string' ? { message: opts } : opts
  if (activeDialogResolve) {
    activeDialogResolve(false)
    activeDialogResolve = null
  }
  return new Promise<boolean>((resolve) => {
    activeDialogResolve = (res) => resolve(Boolean(res))
    state.dialogRequest = {
      id: ++dialogSeq,
      kind: 'confirm',
      title: normalized.title || '请确认操作',
      message: normalized.message,
      confirmText: normalized.confirmText || '确认',
      cancelText: normalized.cancelText || '取消',
      danger: normalized.danger ?? true,
    }
  })
}

function openPrompt(opts: PromptDialogOptions): Promise<string | null> {
  if (activeDialogResolve) {
    activeDialogResolve(null)
    activeDialogResolve = null
  }
  return new Promise<string | null>((resolve) => {
    activeDialogResolve = (res) => resolve(typeof res === 'string' ? res : null)
    state.dialogRequest = {
      id: ++dialogSeq,
      kind: 'prompt',
      title: opts.title || '请输入内容',
      message: opts.message,
      defaultValue: opts.defaultValue || '',
      placeholder: opts.placeholder || '',
      confirmText: opts.confirmText || '保存',
      cancelText: opts.cancelText || '取消',
    }
  })
}

function resolveDialog(result: boolean | string | null) {
  const resolver = activeDialogResolve
  activeDialogResolve = null
  state.dialogRequest = null
  if (resolver) resolver(result)
}

let sseSource: EventSource | null = null
let toastTimer: number | null = null
let toastStartedAt = 0
let toastRemainingMs = TOAST_DURATION_MS
let toastPaused = false

function scheduleToastDismiss(delayMs: number) {
  if (toastTimer !== null) {
    window.clearTimeout(toastTimer)
  }
  toastStartedAt = Date.now()
  toastRemainingMs = delayMs
  toastTimer = window.setTimeout(() => {
    state.toastMessage = ''
    toastTimer = null
  }, delayMs)
}

function notify(msg: string) {
  state.toastMessage = msg
  state.toastSeq += 1
  toastPaused = false
  scheduleToastDismiss(TOAST_DURATION_MS)
}

function pauseToast() {
  if (!state.toastMessage || toastPaused) return
  toastPaused = true
  if (toastTimer !== null) {
    window.clearTimeout(toastTimer)
    toastTimer = null
  }
  const elapsed = Date.now() - toastStartedAt
  toastRemainingMs = Math.max(400, toastRemainingMs - elapsed)
}

function resumeToast() {
  if (!state.toastMessage || !toastPaused) return
  toastPaused = false
  scheduleToastDismiss(toastRemainingMs)
}

function mapOptions() {
  return {
    bots: state.bots,
    defaultModel: state.configs.ai_model,
  }
}

function saveFastBootSnapshot() {
  if (typeof window === 'undefined' || !state.authenticated) return
  try {
    const trimmedTimelines: Record<string, ChatMessageUI[]> = {}
    const candidateIds = Array.from(
      new Set([state.activeBotId, ...state.bots.slice(0, 6).map((b) => b.id)].filter(Boolean))
    ).slice(0, 6)
    for (const id of candidateIds) {
      const list = timelinesByBot[id]
      if (Array.isArray(list) && list.length > 0) {
        trimmedTimelines[id] = list.slice(-30)
      }
    }
    const payload: FastBootSnapshot = {
      version: state.version,
      bots: state.bots.slice(0, 30),
      notifications: state.notifications.slice(0, 30),
      assets: state.assets.slice(0, 30),
      assetCategories: state.assetCategories.slice(0, 40),
      tasks: state.tasks.slice(0, 30),
      stash: state.stash.slice(0, 40),
      activeBotId: state.activeBotId,
      timelinesByBot: trimmedTimelines,
    }
    localStorage.setItem(FASTBOOT_STORAGE_KEY, JSON.stringify(payload))
  } catch {
    // Ignore storage quota errors
  }
}

function clearFastBootSnapshot() {
  if (typeof window === 'undefined') return
  try {
    localStorage.removeItem(FASTBOOT_STORAGE_KEY)
  } catch {
    // Ignore
  }
}

export function useAppStore() {
  const activeBot = computed(
    () => state.bots.find((b) => b.id === state.activeBotId) || state.bots[0] || null
  )

  const unreadInboxCount = computed(() =>
    state.notifications.filter((n) => !n.read_at).length
  )

  const pendingTodoCount = computed(() =>
    state.todos.filter((t) => t.status !== 'done').length
  )

  const unreadStashCount = computed(() =>
    state.stash.filter((s) => s.status !== 'archived' && s.read_count === 0).length
  )

  const expiringAssetCount = computed(() =>
    state.assets.filter((a) => {
      if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) return false
      return typeof a.days_left === 'number' && a.days_left >= 0 && a.days_left <= 7
    }).length
  )

  const modelOptions = computed<SelectOption[]>(() => {
    if (state.modelOptions.length > 0) {
      return state.modelOptions.map((o) => ({
        value: o.id,
        label: o.label,
      }))
    }
    const enabledNodes = state.modelPool.filter((m) => m.enabled)
    const freeOpts: SelectOption[] = []
    const paidOpts: SelectOption[] = []
    let freeReady = 0
    let paidReady = 0

    for (const node of enabledNodes) {
      const tier = node.tier === 'paid' ? 'paid' : 'free'
      const models = Array.from(
        new Set([...(node.models || []), node.model || ''].map((x) => x.trim()).filter(Boolean))
      )
      const list = models.length > 0 ? models : ['']
      list.forEach((m, idx) => {
        const ready = Boolean(node.api_key && m)
        if (ready) {
          if (tier === 'paid') paidReady++
          else freeReady++
        }
        const optId = idx === 0 || !m ? `pool:${node.id}` : `pool:${node.id}::${m}`
        const tag = tier === 'paid' ? '[付费]' : '[免费]'
        const opt = {
          value: optId,
          label: `${tag} ${node.name} · ${m || '待选择模型'} (${ready ? '可用' : '待配置'})`,
        }
        if (tier === 'paid') paidOpts.push(opt)
        else freeOpts.push(opt)
      })
    }

    const out: SelectOption[] = [
      {
        value: 'auto:free',
        label: freeReady > 0 ? `自动 · 免费模型池 (${freeReady} 可用)` : '自动 · 免费模型池 (未配置)',
      },
    ]
    if (paidReady > 0 || paidOpts.length > 0) {
      out.push({
        value: 'auto:paid',
        label: `自动 · 付费模型池 (${paidReady} 可用)`,
      })
    }
    return [...out, ...freeOpts, ...paidOpts]
  })

  async function checkAuth() {
    try {
      const res = await BotHubApi.authStatus()
      state.authenticated = !!res.authenticated
      state.oidcEnabled = Boolean(res.oidc_enabled)
      if (res.oidc_provider_name) {
        state.oidcProviderName = res.oidc_provider_name
      }
      state.authChecked = true
      if (state.authenticated) {
        await loadDashboard(true)
        startSSE()
      } else {
        clearFastBootSnapshot()
      }
    } catch {
      if (!initialSnap) {
        state.authenticated = false
      }
      state.authChecked = true
    }
  }

  async function completeSessionActivation() {
    state.authenticated = true
    await loadDashboard(true)
    startSSE()
  }

  async function login(password: string, rememberDevice = false) {
    await BotHubApi.login(password, rememberDevice)
    await completeSessionActivation()
  }

  async function logout() {
    stopSSE()
    clearFastBootSnapshot()
    await BotHubApi.logout().catch(() => {})
    state.authenticated = false
  }

  async function logoutAllOtherDevices() {
    const res = await BotHubApi.logoutAllDevices(true)
    stopSSE()
    startSSE()
    notify(res.message || '已吊销所有其他已登录设备的访问会话')
    return res
  }

  let inflightDashboard: Promise<void> | null = null
  let pendingDashboard = false
  let pendingLoadInitial = false
  let dashboardRefreshTimer: number | null = null
  const pendingSliceKinds = new Set<string>()
  let sliceRefreshTimer: number | null = null

  const SLICEABLE_KINDS = new Set([
    'todos',
    'notes',
    'stash',
    'memories',
    'assets',
    'tasks',
    'bots',
    'skills',
    'notifications',
    'push_logs',
    'stats',
    'configs',
    'models',
    'model_pool',
    'model_health',
    'workhub',
    'ops',
    'agents',
  ])

  async function refreshWorkspaceSlices(kinds: string[]) {
    const sliceBatch: string[] = []
    let needFullReload = false

    for (const rawKind of kinds) {
      const kind = rawKind.trim().toLowerCase()
      if (!kind) continue
      if (kind === 'notifications_cleared') {
        state.notifications = state.notifications.filter((n) => !n.read_at)
        continue
      }
      if (kind === 'backup_restored' || kind === 'dashboard') {
        needFullReload = true
        break
      }
      if (SLICEABLE_KINDS.has(kind)) {
        sliceBatch.push(kind)
      } else {
        needFullReload = true
        break
      }
    }

    if (needFullReload) {
      await loadDashboard(false)
      return
    }
    if (sliceBatch.length === 0) return

    const jobs: Promise<void>[] = [
      BotHubApi.dashboardSlice(sliceBatch)
        .then((data) => {
          if (data.todos !== undefined) state.todos = data.todos || []
          if (data.notes !== undefined) state.notes = data.notes || []
          if (data.stash !== undefined) state.stash = data.stash || []
          if (data.stash_tags !== undefined) state.stashTags = data.stash_tags || []
          if (data.agent_memories !== undefined) state.memories = data.agent_memories || []
          if (data.assets !== undefined) state.assets = data.assets || []
          if (data.asset_categories !== undefined) state.assetCategories = data.asset_categories || []
          if (data.tasks !== undefined) state.tasks = data.tasks || []
          if (data.bots !== undefined) {
            state.bots = data.bots || []
            if (state.bots.length > 0 && !state.bots.some((b) => b.id === state.activeBotId)) {
              void selectBot(state.bots[0].id)
            }
          }
          if (data.skills !== undefined) state.skills = data.skills || []
          if (data.notifications !== undefined) state.notifications = data.notifications || []
          if (data.push_logs !== undefined) state.pushLogs = data.push_logs || []
          if (data.configs !== undefined && data.configs) state.configs = data.configs
          if (data.model_pool !== undefined) state.modelPool = data.model_pool || []
          if (data.model_providers !== undefined) state.modelProviders = data.model_providers || []
          if (data.pooled_models !== undefined) state.pooledModels = data.pooled_models || []
          if (data.pool_strategies !== undefined && data.pool_strategies) state.poolStrategies = data.pool_strategies
          if (data.model_usage_summary !== undefined && data.model_usage_summary) {
            state.modelUsageSummary = data.model_usage_summary
          }
          if (data.model_options !== undefined) state.modelOptions = data.model_options || []
          if (data.revisions !== undefined && data.revisions) state.revisions = { ...state.revisions, ...data.revisions }
          if (data.stats?.sent_pushes !== undefined) {
            state.stats.sent_pushes = Number(data.stats.sent_pushes) || 0
          }
          if (data.model_health !== undefined && data.model_health) state.modelHealth = data.model_health
          saveFastBootSnapshot()
        })
        .catch(() => {
          return loadDashboard(false)
        }),
    ]

    if ((sliceBatch.includes('memories') || sliceBatch.includes('workhub')) && state.activeBotId) {
      const memScope =
        activeBot.value?.memory_policy === 'isolated'
          ? `bot:${state.activeBotId}`
          : `bot:${state.activeBotId},global`
      jobs.push(
        BotHubApi.listMemories(memScope).then((res) => {
          state.botMemories = res.memories || []
        })
      )
    }

    await Promise.allSettled(jobs)
  }

  function scheduleSliceRefresh(kind?: string, delayMs = 300) {
    const normalized = (kind || '').trim().toLowerCase()
    if (!normalized) {
      scheduleDashboardRefresh(delayMs)
      return
    }
    pendingSliceKinds.add(normalized)
    if (sliceRefreshTimer !== null) {
      window.clearTimeout(sliceRefreshTimer)
    }
    sliceRefreshTimer = window.setTimeout(() => {
      sliceRefreshTimer = null
      const batch = Array.from(pendingSliceKinds)
      pendingSliceKinds.clear()
      void refreshWorkspaceSlices(batch)
    }, delayMs)
  }

  async function loadDashboard(loadInitialBot = false): Promise<void> {
    if (inflightDashboard) {
      pendingDashboard = true
      if (loadInitialBot) pendingLoadInitial = true
      return inflightDashboard
    }
    const run = (async () => {
      try {
        // Fire active bot timeline in parallel with dashboard when activeBotId is already known!
        const preflightBotId = loadInitialBot && state.activeBotId ? state.activeBotId : ''
        const preflightPromise = preflightBotId ? selectBot(preflightBotId) : Promise.resolve()

        const [data] = await Promise.all([BotHubApi.dashboard(), preflightPromise])
        state.version = data.version || state.version
        state.commit = data.commit || state.commit
        state.buildTime = data.build_time || state.buildTime
        state.uptimeSec = Number(data.uptime_sec) || 0
        if (data.stats?.sent_pushes !== undefined) {
          state.stats.sent_pushes = Number(data.stats.sent_pushes) || 0
        }
        state.bots = data.bots || []
        state.skills = data.skills || []
        state.todos = data.todos || []
        state.notes = data.notes || []
        state.stash = data.stash || []
        state.stashTags = data.stash_tags || []
        state.modelPool = data.model_pool || []
        state.modelProviders = data.model_providers || []
        state.pooledModels = data.pooled_models || []
        if (data.pool_strategies) state.poolStrategies = data.pool_strategies
        if (data.model_usage_summary) state.modelUsageSummary = data.model_usage_summary
        state.modelOptions = data.model_options || []
        if (data.revisions) state.revisions = data.revisions
        state.availableTools = data.available_tools || []
        state.modelHealth = data.model_health || {}
        state.assets = data.assets || []
        state.assetCategories = data.asset_categories || []
        state.tasks = data.tasks || []
        state.templates = data.templates || []
        state.memories = data.agent_memories || []
        state.notifications = data.notifications || []
        state.pushLogs = data.push_logs || []
        state.configs = data.configs || {}

        if (state.bots.length === 0) {
          state.activeBotId = ''
          state.activeThreadId = ''
          state.botThreads = []
          state.botMemories = []
          state.botExecutions = []
          state.timeline = []
          localStorage.removeItem('bothub_active_bot')
        } else if (!state.bots.some((b) => b.id === state.activeBotId)) {
          await selectBot(state.bots[0].id)
        } else if (loadInitialBot && !preflightBotId) {
          await selectBot(state.activeBotId)
        }
        saveFastBootSnapshot()
      } catch (err: any) {
        notify(err.message || '加载中枢数据失败')
      } finally {
        inflightDashboard = null
        if (pendingDashboard) {
          const nextInit = pendingLoadInitial
          pendingDashboard = false
          pendingLoadInitial = false
          void loadDashboard(nextInit)
        }
      }
    })()
    inflightDashboard = run
    return run
  }

  function scheduleDashboardRefresh(delayMs = 450) {
    if (dashboardRefreshTimer !== null) {
      window.clearTimeout(dashboardRefreshTimer)
    }
    dashboardRefreshTimer = window.setTimeout(() => {
      dashboardRefreshTimer = null
      void loadDashboard(false)
    }, delayMs)
  }

  let latestSelectSeq = 0

  async function selectBot(botId: string, threadId?: string) {
    const seq = ++latestSelectSeq
    const changedBot = state.activeBotId !== botId
    state.activeBotId = botId
    state.workspace = 'bots'
    localStorage.setItem('bothub_active_bot', botId)
    const bot = state.bots.find((b) => b.id === botId)
    if (bot) {
      bot.unread_count = 0
      if (bot.state === 'done') bot.state = 'idle'
    }
    const targetThread = threadId !== undefined ? threadId : (bot?.active_thread_id || '')
    const cacheKey = targetThread ? `${botId}::${targetThread}` : botId
    if (changedBot || threadId !== undefined) {
      state.timeline = timelinesByBot[cacheKey] ? [...timelinesByBot[cacheKey]] : []
    }
    try {
      const res = await BotHubApi.getBotMessages(botId, targetThread)
      if (seq !== latestSelectSeq) return
      state.activeThreadId = res.thread_id || ''
      state.botThreads = res.threads || []
      state.botMemories = res.memories || []
      state.botExecutions = res.executions || []
      if (res.bot) {
        const idx = state.bots.findIndex((b) => b.id === botId)
        if (idx >= 0) state.bots[idx] = { ...state.bots[idx], ...res.bot }
      }
      const mapped = mapServerMessages(res.messages || [], res.bot || bot, mapOptions())
      state.timeline = mapped
      const resolvedKey = state.activeThreadId ? `${botId}::${state.activeThreadId}` : botId
      timelinesByBot[resolvedKey] = mapped
      if (!state.activeThreadId) {
        timelinesByBot[botId] = mapped
      }
      saveFastBootSnapshot()
    } catch (err: any) {
      if (seq !== latestSelectSeq) return
      notify(err.message || '加载 Bot 对话流失败')
    }
  }

  async function refreshActiveBot() {
    if (!state.activeBotId) return
    await selectBot(state.activeBotId, state.activeThreadId)
  }

  async function switchThread(threadId: string) {
    if (!state.activeBotId) return
    await selectBot(state.activeBotId, threadId)
  }

  async function createThread(title?: string) {
    if (!state.activeBotId) return null
    try {
      const res = await BotHubApi.createBotThread(state.activeBotId, title)
      state.botThreads = res.threads || []
      await selectBot(state.activeBotId, res.thread.id)
      notify(`已新建话题「${res.thread.title}」`)
      return res.thread
    } catch (err: any) {
      notify(err.message || '新建话题失败')
      return null
    }
  }

  async function forkThread(upToMsgId?: number, title?: string) {
    if (!state.activeBotId) return null
    try {
      const res = await BotHubApi.forkBotThread(state.activeBotId, {
        source_thread_id: state.activeThreadId || '',
        up_to_msg_id: upToMsgId || 0,
        title: title || '',
      })
      state.botThreads = res.threads || []
      state.activeThreadId = res.thread.id
      state.botView = 'stream'
      const bot = state.bots.find((b) => b.id === state.activeBotId)
      state.timeline = mapServerMessages(res.messages || [], bot, mapOptions())
      notify(`已从该节点分支出新话题「${res.thread.title}」`)
      return res.thread
    } catch (err: any) {
      notify(err.message || '创建分支话题失败')
      return null
    }
  }

  async function renameThread(threadId: string, title: string) {
    if (!state.activeBotId) return
    try {
      const res = await BotHubApi.updateBotThread(state.activeBotId, threadId, { title })
      state.botThreads = res.threads || []
      notify('话题名称已更新')
    } catch (err: any) {
      notify(err.message || '重命名话题失败')
    }
  }

  async function deleteThread(threadId: string) {
    if (!state.activeBotId) return
    try {
      const res = await BotHubApi.deleteBotThread(state.activeBotId, threadId)
      state.botThreads = res.threads || []
      if (state.activeThreadId === threadId) {
        await selectBot(state.activeBotId, '')
      } else {
        await refreshActiveBot()
      }
      notify('已删除该话题记录')
    } catch (err: any) {
      notify(err.message || '删除话题失败')
    }
  }

  async function summarizeThread(threadId?: string, saveToMemory = true) {
    if (!state.activeBotId) return null
    const tid = threadId !== undefined ? threadId : (state.activeThreadId || 'main')
    try {
      const res = await BotHubApi.summarizeBotThread(state.activeBotId, tid || 'main', saveToMemory)
      if (res.threads) state.botThreads = res.threads
      if (res.memories) state.botMemories = res.memories
      await refreshWorkspaceSlices(['memories'])
      notify(
        res.saved_fact_key
          ? `已生成话题摘要并沉淀为记忆「${res.saved_fact_key}」`
          : '已生成话题摘要'
      )
      return res
    } catch (err: any) {
      notify(err.message || '提炼话题摘要失败')
      return null
    }
  }

  async function jumpToBot(botId: string, draftPrompt?: string) {
    state.botView = 'stream'
    state.showInboxModal = false
    state.showGroupDrawer = false
    state.showQuickSwitcher = false
    await selectBot(botId)
    if (draftPrompt) {
      state.pendingDraftPrompt = draftPrompt
    }
  }

  async function togglePinBot(targetBot: Bot) {
    try {
      const nextPinned = !targetBot.is_pinned
      await BotHubApi.updateBot(targetBot.id, { is_pinned: nextPinned })
      targetBot.is_pinned = nextPinned
      notify(nextPinned ? `已置顶「${targetBot.name}」` : `已取消置顶「${targetBot.name}」`)
      await refreshWorkspaceSlices(['bots'])
    } catch (err: any) {
      notify(err.message || '更新置顶状态失败')
    }
  }

  async function deleteSingleMessage(msg: ChatMessageUI) {
    try {
      if (msg.serverId && activeBot.value) {
        await BotHubApi.deleteBotMessage(activeBot.value.id, msg.serverId)
      }
      state.timeline = state.timeline.filter((x) => x.localId !== msg.localId)
      notify('已删除该条消息')
    } catch (err: any) {
      notify(err.message || '删除消息失败')
    }
  }

  async function clearActiveBotTimeline() {
    const bot = activeBot.value
    if (!bot) return
    try {
      await BotHubApi.clearBotMessages(bot.id, state.activeThreadId)
      state.timeline = []
      await refreshActiveBot()
      notify(state.activeThreadId ? `[${bot.name}] 当前话题记录已清空` : `[${bot.name}] 主对话时间线已清空`)
    } catch (err: any) {
      notify(err.message || '清空对话记录失败')
    }
  }

  function draftStorageKey(botId: string, threadId = ''): string {
    return threadId ? `${botId}::${threadId}` : botId
  }

  function setBotDraft(botId: string, text: string, threadId = '') {
    if (!botId) return
    const key = draftStorageKey(botId, threadId)
    if (text) state.draftsByBot[key] = text
    else delete state.draftsByBot[key]
    try {
      localStorage.setItem('bothub_drafts_v2', JSON.stringify(state.draftsByBot))
    } catch {
      // ignore storage quota errors
    }
  }

  function getBotDraft(botId: string, threadId = ''): string {
    if (!botId) return ''
    return state.draftsByBot[draftStorageKey(botId, threadId)] || ''
  }

  function playSubtleChime() {
    if (!state.soundEnabled) return
    playAudioChime()
  }

  function toggleSound() {
    state.soundEnabled = !state.soundEnabled
    localStorage.setItem('bothub_sound', state.soundEnabled ? '1' : '0')
    if (state.soundEnabled) playSubtleChime()
    notify(state.soundEnabled ? '已开启完成轻提示音' : '已关闭提示音')
  }

  function toggleAmbientMotion() {
    state.ambientMotionPaused = !state.ambientMotionPaused
    localStorage.setItem('bothub_ambient_paused', state.ambientMotionPaused ? '1' : '0')
    notify(state.ambientMotionPaused ? '已暂停背景漂浮与待机动效' : '已恢复背景漂浮与待机动效')
  }

  function startSSE() {
    if (sseSource || typeof EventSource === 'undefined') return
    let hadDisconnect = false
    const latestId = state.notifications.length > 0 ? state.notifications[0].id : 0
    const es = new EventSource(`/api/v2/events?last_event_id=${encodeURIComponent(latestId)}`)
    sseSource = es
    es.onopen = () => {
      const reconnected = hadDisconnect
      hadDisconnect = false
      state.sseConnected = true
      if (reconnected) {
        void loadDashboard(false)
        void refreshActiveBot()
      }
    }

    es.addEventListener('connected', (ev: MessageEvent) => {
      try {
        const payload = JSON.parse(ev.data || '{}') as { revisions?: Record<string, number> }
        if (!payload.revisions) return
        const prev = state.revisions || {}
        const changedKinds: string[] = []
        const revKeys = ['bots', 'tasks', 'assets', 'todos', 'notes', 'stash', 'memories', 'skills', 'models', 'configs', 'notifications']
        for (const k of revKeys) {
          if ((payload.revisions[k] || 0) > (prev[k] || 0)) {
            changedKinds.push(k)
          }
        }
        const msgBumped = (payload.revisions.messages || 0) > (prev.messages || 0)
        state.revisions = { ...prev, ...payload.revisions }
        if (changedKinds.length > 0) {
          void refreshWorkspaceSlices(changedKinds)
        }
        if (msgBumped && state.activeBotId) {
          void refreshActiveBot()
        }
      } catch {
        // ignore
      }
    })

    es.addEventListener('notification', (ev: MessageEvent) => {
      try {
        const item = JSON.parse(ev.data) as NotificationItem
        if (!item?.id) return
        if (!state.notifications.some((n) => n.id === item.id)) {
          state.notifications = [item, ...state.notifications].slice(0, 60)
          notify(`新情报到达: ${item.title}`)
          if (
            localStorage.getItem('bothub_desktop_notify') === '1' &&
            typeof Notification !== 'undefined' &&
            Notification.permission === 'granted' &&
            (document.hidden || item.priority === 'high')
          ) {
            try {
              new Notification(`BotHub · ${item.title}`, {
                body: item.summary || '收到新的情报推送',
              })
            } catch {
              // ignore browser notification error
            }
          }
        }
      } catch {
        // ignore
      }
    })

    es.addEventListener('bot_state', (ev: MessageEvent) => {
      try {
        const patch = JSON.parse(ev.data) as {
          bot_id: string
          state?: string
          last_summary?: string
          unread_count?: number
        }
        const target = state.bots.find((b) => b.id === patch.bot_id)
        if (target) {
          if (patch.state) target.state = patch.state
          if (patch.last_summary) target.last_summary = patch.last_summary
          if (patch.unread_count !== undefined) target.unread_count = patch.unread_count
        }
      } catch {
        // ignore
      }
    })

    es.addEventListener('bot_message', (ev: MessageEvent) => {
      try {
        const msg = JSON.parse(ev.data) as AgentMessage
        if (!msg?.bot_id) return
        const sameThread = (msg.thread_id || '') === (state.activeThreadId || '')
        if (msg.bot_id === state.activeBotId && sameThread && (!state.isStreaming || msg.role === 'system')) {
          if (!state.timeline.some((t) => msg.id && t.serverId === msg.id)) {
            state.timeline.push({ ...mapServerMessage(msg, undefined, mapOptions()), isLiveNew: true })
          }
        } else if (msg.bot_id !== state.activeBotId && msg.msg_kind === 'routine') {
          // Smart DND: only background routines/alerts increment unread badges; active interactive chat on another device does not
          const target = state.bots.find((b) => b.id === msg.bot_id)
          if (target) target.unread_count = (target.unread_count || 0) + 1
        }
      } catch {
        // ignore
      }
    })

    es.addEventListener('assistant_update', (ev: MessageEvent) => {
      try {
        const payload = JSON.parse(ev.data || '{}') as { kind?: string }
        scheduleSliceRefresh(payload.kind, 300)
      } catch {
        scheduleDashboardRefresh(450)
      }
    })

    es.addEventListener('thread_updated', (ev: MessageEvent) => {
      try {
        const payload = JSON.parse(ev.data || '{}') as {
          bot_id?: string
          thread_id?: string
          action?: string
          summary?: string
        }
        if (payload.bot_id === state.activeBotId) {
          if (payload.action && payload.action !== 'summarized') {
            void refreshActiveBot()
          } else if (payload.thread_id) {
            const target = state.botThreads.find((t) => t.id === payload.thread_id)
            if (target && payload.summary !== undefined) {
              target.summary = payload.summary
            }
          }
        }
        void refreshWorkspaceSlices(['bots'])
      } catch {
        // ignore
      }
    })

    es.onerror = () => {
      hadDisconnect = true
      state.sseConnected = false
      if (!state.authenticated) stopSSE()
    }
  }

  function stopSSE() {
    if (dashboardRefreshTimer !== null) {
      window.clearTimeout(dashboardRefreshTimer)
      dashboardRefreshTimer = null
    }
    if (sliceRefreshTimer !== null) {
      window.clearTimeout(sliceRefreshTimer)
      sliceRefreshTimer = null
    }
    pendingSliceKinds.clear()
    state.sseConnected = false
    if (sseSource) {
      sseSource.close()
      sseSource = null
    }
  }

  async function saveAssetCategories(next: AssetCategoryOption[]): Promise<AssetCategoryOption[]> {
    const prev = [...state.assetCategories]
    state.assetCategories = next
    try {
      const res = await BotHubApi.saveAssetCategories(next)
      state.assetCategories = res.categories || next
      saveFastBootSnapshot()
      return state.assetCategories
    } catch (err) {
      state.assetCategories = prev
      throw err
    }
  }

  return {
    state,
    activeBot,
    modelOptions,
    unreadInboxCount,
    pendingTodoCount,
    unreadStashCount,
    expiringAssetCount,
    notify,
    saveAssetCategories,
    pauseToast,
    resumeToast,
    openConfirm,
    openPrompt,
    resolveDialog,
    checkAuth,
    login,
    completeSessionActivation,
    logout,
    logoutAllOtherDevices,
    loadDashboard,
    refreshWorkspaceSlices,
    selectBot,
    refreshActiveBot,
    switchThread,
    createThread,
    forkThread,
    renameThread,
    deleteThread,
    summarizeThread,
    jumpToBot,
    togglePinBot,
    deleteSingleMessage,
    clearActiveBotTimeline,
    setBotDraft,
    getBotDraft,
    toggleSound,
    toggleAmbientMotion,
    playSubtleChime,
    startSSE,
    stopSSE,
  }
}
