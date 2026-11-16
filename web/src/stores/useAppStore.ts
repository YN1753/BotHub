import { reactive, computed } from 'vue'
import { BotHubApi } from '../api/client'
import type {
  AgentMemory,
  AgentMessage,
  Asset,
  AssetTemplate,
  Bot,
  NotificationItem,
  PushLog,
  Task,
  TaskTemplate,
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
  tools: Array<{ id: string; name: string; arguments: string; result?: string; done: boolean }>
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
  version: 'v2.6.0',
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
  assets: [] as Asset[],
  tasks: [] as Task[],
  templates: [] as TaskTemplate[],
  assetTemplates: [] as AssetTemplate[],
  memories: [] as AgentMemory[],
  notifications: [] as NotificationItem[],
  pushLogs: [] as PushLog[],
  configs: {} as Record<string, string>,

  // Navigation & Active Bot Workspace
  workspace: 'bots' as MainWorkspace,
  activeBotId: localStorage.getItem('bothub_active_bot') || 'butler',
  botView: 'stream' as BotSubView,
  timeline: [] as ChatMessageUI[],
  isStreaming: false,

  // Modals & Drawers
  showInboxModal: false,
  showMemoriesModal: false,
  showLogsModal: false,
  showSettingsModal: false,
  showCreateBotModal: false,
  showGroupDrawer: false,
  showQuickSwitcher: false,
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
    senderBotId: m.sender_bot_id || m.bot_id || sender?.id || 'butler',
    senderName: sender?.name || 'BotHub',
    roleTitle: sender?.role_title || '',
    avatarStyle: sender?.avatar_style || 'cyan',
    model: sender?.model || 'deepseek-flash',
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

export function useAppStore() {
  const activeBot = computed(
    () => state.bots.find((b) => b.id === state.activeBotId) || state.bots[0] || null
  )

  const unreadInboxCount = computed(() =>
    state.notifications.filter((n) => !n.read_at).length
  )

  const expiringAssetCount = computed(() =>
    state.assets.filter((a) => {
      if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) return false
      return typeof a.days_left === 'number' && a.days_left >= 0 && a.days_left <= 7
    }).length
  )

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
      state.uptimeSec = Number(data.uptime_sec) || 0
      state.serverNow = data.server_now || '--'
      state.stats = data.stats || state.stats
      state.bots = data.bots || []
      state.assets = data.assets || []
      state.tasks = data.tasks || []
      state.templates = data.templates || []
      state.assetTemplates = data.asset_templates || []
      state.memories = data.agent_memories || []
      state.notifications = data.notifications || []
      state.pushLogs = data.push_logs || []
      state.configs = data.configs || {}

      if (loadInitialBot && state.bots.length > 0) {
        const exists = state.bots.some((b) => b.id === state.activeBotId)
        const targetId = exists ? state.activeBotId : state.bots[0].id
        await selectBot(targetId)
      }
    } catch (err: any) {
      notify(err.message || '加载中枢数据失败')
    }
  }

  async function selectBot(botId: string) {
    state.activeBotId = botId
    state.workspace = 'bots'
    localStorage.setItem('bothub_active_bot', botId)
    const bot = state.bots.find((b) => b.id === botId)
    if (bot) {
      bot.unread_count = 0
      if (bot.state === 'done') bot.state = 'idle'
    }
    try {
      const res = await BotHubApi.getBotMessages(botId)
      state.timeline = (res.messages || [])
        .filter((m) => m.role === 'user' || m.role === 'assistant' || m.role === 'system')
        .map((m) => mapServerMessage(m, res.bot || bot))
    } catch (err: any) {
      notify(err.message || '加载 Bot 对话流失败')
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
    await BotHubApi.clearBotMessages(bot.id)
    state.timeline = []
    notify(`[${bot.name}] 对话时间线已清空`)
  }

  function startSSE() {
    if (sseSource || typeof EventSource === 'undefined') return
    const latestId = state.notifications.length > 0 ? state.notifications[0].id : 0
    const es = new EventSource(`/api/v2/events?last_event_id=${encodeURIComponent(latestId)}`)
    sseSource = es

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
        if (msg.bot_id === state.activeBotId && (!state.isStreaming || msg.role === 'system')) {
          if (!state.timeline.some((t) => msg.id && t.serverId === msg.id)) {
            state.timeline.push(mapServerMessage(msg))
          }
        } else {
          const target = state.bots.find((b) => b.id === msg.bot_id)
          if (target) target.unread_count = (target.unread_count || 0) + 1
        }
      } catch {
        // ignore
      }
    })

    es.onerror = () => {
      if (!state.authenticated) stopSSE()
    }
  }

  function stopSSE() {
    if (sseSource) {
      sseSource.close()
      sseSource = null
    }
  }

  return {
    state,
    activeBot,
    unreadInboxCount,
    expiringAssetCount,
    notify,
    checkAuth,
    login,
    logout,
    loadDashboard,
    selectBot,
    jumpToBot,
    togglePinBot,
    deleteSingleMessage,
    clearActiveBotTimeline,
  }
}
