import { reactive, computed } from 'vue'
import { BotHubApi } from '../api/client'
import type {
  AgentMemory,
  AgentMessage,
  Asset,
  AvailableToolMeta,
  Bot,
  BotExecutionLog,
  BotThread,
  ModelHealthStats,
  ModelPoolEntry,
  NotificationItem,
  Persona,
  PushLog,
  SelectOption,
  Skill,
  Task,
  TaskTemplate,
  TodoItem,
  WorkNote,
} from '../types/bothub'

export type MainWorkspace = 'bots' | 'ops'
export type BotSubView = 'stream' | 'routines' | 'config' | 'activity'

export interface ChatMessageUI {
  localId: string
  serverId?: number
  role: 'user' | 'assistant' | 'system'
  senderBotId: string
  senderName: string
  roleTitle?: string
  avatarStyle: string
  model: string
  msgKind: 'chat' | 'routine' | 'system'
  createdAt: string
  content: string
  reasoning: string
  tools: Array<{
    id: string
    name: string
    arguments: string
    result?: string
    durationMs?: number
    status?: string
    done: boolean
  }>
  replyToId?: number
  replyPreview?: string
  isTempGuest?: boolean
  statusText: string
  usageText?: string
  errorText?: string
  streaming?: boolean
}

const state = reactive({
  authChecked: false,
  authenticated: false,
  version: 'v2.19.0',
  commit: '',
  buildTime: '',
  uptimeSec: 0,
  serverNow: '--',
  stats: {
    total_assets: 0,
    total_pushes: 0,
    sent_pushes: 0,
    seen_items: 0,
    total_memories: 0,
    total_notifications: 0,
    unread_notifications: 0,
  },
  bots: [] as Bot[],
  skills: [] as Skill[],
  todos: [] as TodoItem[],
  notes: [] as WorkNote[],
  modelPool: [] as ModelPoolEntry[],
  availableTools: [] as AvailableToolMeta[],
  personas: [] as Persona[],
  modelHealth: {} as Record<string, ModelHealthStats>,
  assets: [] as Asset[],
  tasks: [] as Task[],
  templates: [] as TaskTemplate[],
  memories: [] as AgentMemory[],
  notifications: [] as NotificationItem[],
  pushLogs: [] as PushLog[],
  configs: {} as Record<string, string>,

  // Navigation & Active Bot Workspace
  workspace: 'bots' as MainWorkspace,
  activeBotId: localStorage.getItem('bothub_active_bot') || '',
  activeThreadId: '',
  botThreads: [] as BotThread[],
  botMemories: [] as AgentMemory[],
  botExecutions: [] as BotExecutionLog[],
  botView: 'stream' as BotSubView,
  timeline: [] as ChatMessageUI[],
  isStreaming: false,
  sseConnected: false,
  soundEnabled: localStorage.getItem('bothub_sound') === '1',
  draftsByBot: (() => {
    try {
      return JSON.parse(sessionStorage.getItem('bothub_drafts') || '{}') as Record<string, string>
    } catch {
      return {} as Record<string, string>
    }
  })(),

  // Modals & Drawers
  showInboxModal: false,
  showWorkhubModal: false,
  workhubTab: 'todos' as 'todos' | 'notes' | 'briefing',
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
  editingAsset: undefined as Asset | null | undefined, // undefined = closed, null = new
  editingTask: undefined as Task | Partial<Task> | null | undefined,

  // Toast
  toastMessage: '',
})

let sseSource: EventSource | null = null
let toastTimer: number | null = null

function notify(msg: string) {
  state.toastMessage = msg
  if (toastTimer !== null) {
    window.clearTimeout(toastTimer)
  }
  toastTimer = window.setTimeout(() => {
    state.toastMessage = ''
  }, 3200)
}

function mapServerMessage(m: AgentMessage, fallbackBot?: Bot): ChatMessageUI {
  const sender =
    state.bots.find((b) => b.id === (m.sender_bot_id || m.bot_id)) || fallbackBot
  let parsedTools: ChatMessageUI['tools'] = []
  try {
    const raw = JSON.parse(m.tool_calls || '[]')
    if (Array.isArray(raw)) {
      parsedTools = raw.map((tc: any, idx: number) => ({
        id: tc.id || `tool_${idx}`,
        name: tc.function?.name || 'tool',
        arguments: tc.function?.arguments || '{}',
        done: true,
      }))
    }
  } catch {
    parsedTools = []
  }
  const kind = m.msg_kind === 'system' || m.role === 'system' ? 'system' : m.msg_kind === 'routine' ? 'routine' : 'chat'
  const role: ChatMessageUI['role'] =
    kind === 'system' ? 'system' : m.role === 'user' ? 'user' : 'assistant'

  return {
    localId: `srv_${m.id}`,
    serverId: m.id,
    role,
    senderBotId: m.sender_bot_id || m.bot_id || sender?.id || '',
    senderName: sender?.name || 'BotHub',
    roleTitle: sender?.role_title || '',
    avatarStyle: sender?.avatar_style || 'cyan',
    model: m.model_used || sender?.model || state.configs.ai_model || '',
    msgKind: kind,
    createdAt: m.created_at,
    content: m.content || '',
    reasoning: m.reasoning_content || '',
    tools: parsedTools,
    replyToId: m.reply_to_id || undefined,
    replyPreview: m.reply_preview || undefined,
    statusText: kind === 'routine' ? '例程主动汇报' : '已完成',
  }
}

// Reconstructs full assistant turn by joining tool_calls with their corresponding role="tool" outputs
// and merging multi-round tool turns from the same sender into a single clean timeline bubble.
export function mapServerMessages(msgs: AgentMessage[], fallbackBot?: Bot): ChatMessageUI[] {
  const toolResultByCallID = new Map<string, string>()
  for (const m of msgs) {
    if (m.role === 'tool' && m.tool_call_id) {
      toolResultByCallID.set(m.tool_call_id, m.content || '')
    }
  }

  const out: ChatMessageUI[] = []
  for (const m of msgs) {
    if (m.role !== 'user' && m.role !== 'assistant' && m.role !== 'system') {
      continue
    }
    const mapped = mapServerMessage(m, fallbackBot)
    if (mapped.tools.length > 0) {
      for (const tc of mapped.tools) {
        if (toolResultByCallID.has(tc.id)) {
          tc.result = toolResultByCallID.get(tc.id)
        }
      }
    }

    const prev = out.length > 0 ? out[out.length - 1] : null
    if (
      prev &&
      prev.role === 'assistant' &&
      mapped.role === 'assistant' &&
      prev.senderBotId === mapped.senderBotId &&
      prev.tools.length > 0
    ) {
      // Merge subsequent round (tool trace / final answer) into the ongoing assistant turn
      prev.serverId = mapped.serverId
      prev.localId = mapped.localId
      if (mapped.reasoning && !prev.reasoning.includes(mapped.reasoning)) {
        prev.reasoning = prev.reasoning ? `${prev.reasoning}\n${mapped.reasoning}` : mapped.reasoning
      }
      if (mapped.tools.length > 0) {
        prev.tools.push(...mapped.tools)
      }
      if (mapped.content) {
        prev.content = mapped.content
      }
      if (mapped.model) {
        prev.model = mapped.model
      }
      prev.createdAt = mapped.createdAt
      continue
    }
    out.push(mapped)
  }
  return out
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

  const expiringAssetCount = computed(() =>
    state.assets.filter((a) => {
      if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) return false
      return typeof a.days_left === 'number' && a.days_left >= 0 && a.days_left <= 7
    }).length
  )

  const modelOptions = computed<SelectOption[]>(() => {
    const enabledNodes = state.modelPool.filter((m) => m.enabled)
    const readyCount = enabledNodes.filter((m) => Boolean(m.api_key && m.model)).length
    const hasPrimaryKey = Boolean((state.configs.deepseek_api_key || '').trim())
    const primaryModel = (state.configs.ai_model || '').trim()
    const primaryOpt: SelectOption = {
      value: 'primary',
      label: hasPrimaryKey
        ? `主力引擎 (${primaryModel || '待拉取模型'})`
        : '主力引擎 (需先配置 Key)',
    }
    const baseOpts: SelectOption[] = []

    // If user only set up Free Pool (without Primary Key), put free-pool first as smart default
    if (!hasPrimaryKey && enabledNodes.length > 0) {
      baseOpts.push({
        value: 'free-pool',
        label:
          readyCount > 0
            ? `免费模型池 (自动容灾 · ${readyCount} 已就绪)`
            : '免费模型池 (待配 Key 与模型)',
      })
      baseOpts.push(primaryOpt)
    } else {
      baseOpts.push(primaryOpt)
      if (enabledNodes.length > 0) {
        baseOpts.push({
          value: 'free-pool',
          label:
            readyCount > 0
              ? `免费模型池 (自动容灾 · ${readyCount} 已就绪)`
              : '免费模型池 (待配 Key 与模型)',
        })
      }
    }

    for (const node of enabledNodes) {
      const modelTag = node.model ? ` · ${node.model}` : ' · 待拉取模型'
      baseOpts.push({
        value: `pool:${node.id}`,
        label: `${node.name}${modelTag} [${node.api_key && node.model ? '可用' : '待配置'}]`,
      })
    }
    return baseOpts
  })

  async function checkAuth() {
    try {
      const res = await BotHubApi.authStatus()
      state.authenticated = !!res.authenticated
      state.authChecked = true
      if (state.authenticated) {
        await loadDashboard(true)
        startSSE()
      }
    } catch {
      state.authenticated = false
      state.authChecked = true
    }
  }

  async function login(password: string) {
    await BotHubApi.login(password)
    state.authenticated = true
    await loadDashboard(true)
    startSSE()
  }

  async function logout() {
    stopSSE()
    await BotHubApi.logout().catch(() => {})
    state.authenticated = false
  }

  async function loadDashboard(loadInitialBot = false) {
    try {
      const data = await BotHubApi.dashboard()
      state.version = data.version || state.version
      state.commit = data.commit || state.commit
      state.buildTime = data.build_time || state.buildTime
      state.uptimeSec = Number(data.uptime_sec) || 0
      state.serverNow = data.server_now || '--'
      state.stats = data.stats || state.stats
      state.bots = data.bots || []
      state.skills = data.skills || []
      state.todos = data.todos || []
      state.notes = data.notes || []
      state.modelPool = data.model_pool || []
      state.availableTools = data.available_tools || []
      state.personas = data.personas || []
      state.modelHealth = data.model_health || {}
      state.assets = data.assets || []
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
      } else if (loadInitialBot || !state.bots.some((b) => b.id === state.activeBotId)) {
        const exists = state.bots.some((b) => b.id === state.activeBotId)
        const targetId = exists ? state.activeBotId : state.bots[0].id
        await selectBot(targetId)
      }
    } catch (err: any) {
      notify(err.message || '加载中枢数据失败')
    }
  }

  async function selectBot(botId: string, threadId?: string) {
    state.activeBotId = botId
    state.workspace = 'bots'
    localStorage.setItem('bothub_active_bot', botId)
    const bot = state.bots.find((b) => b.id === botId)
    if (bot) {
      bot.unread_count = 0
      if (bot.state === 'done') bot.state = 'idle'
    }
    try {
      const targetThread = threadId !== undefined ? threadId : (bot?.active_thread_id || '')
      const res = await BotHubApi.getBotMessages(botId, targetThread)
      state.activeThreadId = res.thread_id || ''
      state.botThreads = res.threads || []
      state.botMemories = res.memories || []
      state.botExecutions = res.executions || []
      if (res.bot) {
        const idx = state.bots.findIndex((b) => b.id === botId)
        if (idx >= 0) state.bots[idx] = { ...state.bots[idx], ...res.bot }
      }
      state.timeline = mapServerMessages(res.messages || [], res.bot || bot)
    } catch (err: any) {
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
      state.timeline = mapServerMessages(res.messages || [], bot)
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
      await loadDashboard(false)
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
      await loadDashboard(false)
    } catch (err: any) {
      notify(err.message || '更新置顶状态失败')
    }
  }

  async function deleteSingleMessage(msg: ChatMessageUI) {
    if (msg.serverId && activeBot.value) {
      await BotHubApi.deleteBotMessage(activeBot.value.id, msg.serverId)
    }
    state.timeline = state.timeline.filter((x) => x.localId !== msg.localId)
    notify('已删除该条消息')
  }

  async function clearActiveBotTimeline() {
    const bot = activeBot.value
    if (!bot) return
    await BotHubApi.clearBotMessages(bot.id, state.activeThreadId)
    state.timeline = []
    await refreshActiveBot()
    notify(state.activeThreadId ? `[${bot.name}] 当前话题记录已清空` : `[${bot.name}] 主对话时间线已清空`)
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
      sessionStorage.setItem('bothub_drafts', JSON.stringify(state.draftsByBot))
    } catch {
      // ignore storage quota errors
    }
  }

  function getBotDraft(botId: string, threadId = ''): string {
    if (!botId) return ''
    return state.draftsByBot[draftStorageKey(botId, threadId)] || ''
  }

  function toggleSound() {
    state.soundEnabled = !state.soundEnabled
    localStorage.setItem('bothub_sound', state.soundEnabled ? '1' : '0')
    if (state.soundEnabled) playSubtleChime()
    notify(state.soundEnabled ? '已开启完成轻提示音' : '已关闭提示音')
  }

  function playSubtleChime() {
    if (!state.soundEnabled || typeof window === 'undefined') return
    try {
      const AudioCtx = window.AudioContext || (window as any).webkitAudioContext
      if (!AudioCtx) return
      const ctx = new AudioCtx()
      const now = ctx.currentTime
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()
      osc.type = 'sine'
      osc.frequency.setValueAtTime(587.33, now)
      osc.frequency.exponentialRampToValueAtTime(880, now + 0.09)
      gain.gain.setValueAtTime(0.035, now)
      gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.22)
      osc.connect(gain)
      gain.connect(ctx.destination)
      osc.start(now)
      osc.stop(now + 0.24)
      window.setTimeout(() => ctx.close().catch(() => {}), 300)
    } catch {
      // ignore autoplay block
    }
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
        loadDashboard(false)
      }
    }

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
              new Notification(`Vesper BotHub · ${item.title}`, {
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
            state.timeline.push(mapServerMessage(msg))
          }
        } else if (msg.bot_id !== state.activeBotId) {
          const target = state.bots.find((b) => b.id === msg.bot_id)
          if (target) target.unread_count = (target.unread_count || 0) + 1
        }
      } catch {
        // ignore
      }
    })

    es.addEventListener('assistant_update', () => {
      loadDashboard(false)
    })

    es.onerror = () => {
      hadDisconnect = true
      state.sseConnected = false
      if (!state.authenticated) stopSSE()
    }
  }

  function stopSSE() {
    state.sseConnected = false
    if (sseSource) {
      sseSource.close()
      sseSource = null
    }
  }

  return {
    state,
    activeBot,
    modelOptions,
    unreadInboxCount,
    pendingTodoCount,
    expiringAssetCount,
    notify,
    checkAuth,
    login,
    logout,
    loadDashboard,
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
    playSubtleChime,
    startSSE,
  }
}
