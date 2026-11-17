<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore, mapServerMessages, type ChatMessageUI } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { formatTime, formatTimelineDivider, triggerBlobDownload } from '../utils/format'
import { lazyPanelChunk } from '../utils/lazyComponent'
import { resolveTabSlideTransition, type DirectionalTransitionName } from '../utils/motion'
import { handleMarkdownCodeCopy, renderSafeMarkdown } from '../utils/markdown'
import type { Bot } from '../types/bothub'
import BotAvatar from './BotAvatar.vue'
import GroupDrawer from './GroupDrawer.vue'
import BotWorkspaceHeader from './workspace/BotWorkspaceHeader.vue'
import BotThreadBar from './workspace/BotThreadBar.vue'
import BotToolTrail from './workspace/BotToolTrail.vue'
import BotChatComposer, { type RelayStepItem, type ReplyQuoteTarget } from './workspace/BotChatComposer.vue'
import BotChatSearchBar from './workspace/BotChatSearchBar.vue'

const BotRoutinesTab = lazyPanelChunk(() => import('./workspace/BotRoutinesTab.vue'))
const BotConfigTab = lazyPanelChunk(() => import('./workspace/BotConfigTab.vue'))
const BotActivityTab = lazyPanelChunk(() => import('./workspace/BotActivityTab.vue'))
const BotStarterPanel = lazyPanelChunk(() => import('./workspace/BotStarterPanel.vue'))

defineProps<{
  desktopCollapsed?: boolean
}>()

const emit = defineEmits<{
  openMobileMenu: []
  toggleDesktopSidebar: []
}>()

const store = useAppStore()
const bot = store.activeBot

const inputPrompt = ref('')
const selectedModel = ref('primary')
const enableThinking = ref(true)
const streamStatus = ref('就绪')
const scrollContainerRef = ref<HTMLElement | null>(null)
const composerRef = ref<InstanceType<typeof BotChatComposer> | null>(null)
const userScrolledUp = ref(false)
const unreadBelowCount = ref(0)
const botSubViewTransition = ref<DirectionalTransitionName>('vesper-slide-forward')
const streamEnterTransition = computed(() =>
  botSubViewTransition.value === 'vesper-slide-backward'
    ? 'vesper-stream-backward'
    : 'vesper-stream-forward'
)

watch(
  () => store.state.botView,
  (next, prev) => {
    botSubViewTransition.value = resolveTabSlideTransition(
      next,
      prev || 'stream',
      ['stream', 'routines', 'config', 'activity']
    )
  }
)

let abortCtrl: AbortController | null = null
const lastPrompt = ref('')

const replyTarget = ref<ReplyQuoteTarget | null>(null)
const announcementDismissed = ref(false)
const relaySteps = ref<RelayStepItem[]>([])

// In-Chat Message Search (QQ/WeChat Cmd+F)
const chatSearchOpen = ref(false)
const chatSearchQuery = ref('')
const chatSearchCursor = ref(0)
const chatSearchInputRef = ref<HTMLInputElement | null>(null)

const chatSearchMatches = computed(() => {
  const q = chatSearchQuery.value.trim().toLowerCase()
  if (!q) return []
  return store.state.timeline
    .filter((m) => m.role !== 'system' && m.content.toLowerCase().includes(q))
    .map((m) => m.localId)
})

const focusedMatchId = computed(() =>
  chatSearchMatches.value[chatSearchCursor.value] || ''
)

function openChatSearch() {
  store.state.botView = 'stream'
  chatSearchOpen.value = true
  nextTick(() => {
    chatSearchInputRef.value?.focus()
    chatSearchInputRef.value?.select()
  })
}

function closeChatSearch() {
  chatSearchOpen.value = false
  chatSearchQuery.value = ''
  chatSearchCursor.value = 0
}

function jumpSearchMatch(delta: number) {
  const list = chatSearchMatches.value
  if (!list.length) return
  chatSearchCursor.value = (chatSearchCursor.value + delta + list.length) % list.length
  const targetId = list[chatSearchCursor.value]
  nextTick(() => {
    const el = scrollContainerRef.value?.querySelector(`[data-msg-id="${CSS.escape(targetId)}"]`)
    el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  })
}

watch(chatSearchMatches, (list) => {
  chatSearchCursor.value = 0
  if (list.length > 0) {
    nextTick(() => {
      const el = scrollContainerRef.value?.querySelector(`[data-msg-id="${CSS.escape(list[0])}"]`)
      el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    })
  }
})

function findPrecedingUserPrompt(idx: number): string {
  for (let i = idx - 1; i >= 0; i--) {
    const item = store.state.timeline[i]
    if (item && item.role === 'user' && item.content.trim()) {
      return item.content.trim()
    }
  }
  return lastPrompt.value
}

function openForwardForMessage(msg: ChatMessageUI) {
  const fromBot = bot.value?.name || msg.senderName || 'BotHub'
  const senderLabel = msg.role === 'user' ? `${fromBot} (管理员提问)` : `${fromBot} · ${msg.senderName}`
  store.state.forwardingMessage = {
    fromName: senderLabel,
    content: msg.content,
  }
}

const expandedUserMsgs = ref<Record<string, boolean>>({})

function isLongUserMessage(content: string): boolean {
  if (!content) return false
  return content.length > 260 || content.split('\n').length > 6
}

function toggleExpandUserMsg(localId: string) {
  expandedUserMsgs.value[localId] = !expandedUserMsgs.value[localId]
}

function runQuickToolStarter(toolId: string) {
  switch (toolId) {
    case 'workspace_briefing':
      sendPrompt('请调用 workspace_briefing 为我梳理今日重点待办、日程与临期风险，并排出优先次序。')
      break
    case 'manage_todos':
      store.state.workhubTab = 'todos'
      store.state.showWorkhubModal = true
      break
    case 'manage_notes':
      store.state.workhubTab = 'notes'
      store.state.showWorkhubModal = true
      break
    case 'manage_stash':
      store.state.workhubTab = 'stash'
      store.state.showWorkhubModal = true
      break
    case 'manage_assets':
      sendPrompt('请调用 manage_assets(action="summary") 和 expiring 帮我盘点当前数字资产费用与到期风险。')
      break
    case 'query_nezha_nodes':
      sendPrompt('请调用 query_nezha_nodes 帮我巡检全部节点实时在线与负载健康状态。')
      break
    case 'manage_tasks':
      sendPrompt('请调用 manage_tasks 核查当前自动化调度与提醒队列状态。')
      break
    case 'recall_memory':
    case 'manage_memory':
      sendPrompt('请检索当前作用域的长期记忆，确认你目前记得的核心习惯与约定。')
      break
    case 'web_search':
      inputPrompt.value = '请帮我联网检索：'
      nextTick(() => composerRef.value?.focus())
      break
    case 'consult_bot':
      inputPrompt.value = '请帮我协同其他专员一起完成：'
      nextTick(() => composerRef.value?.focus())
      break
    default:
      inputPrompt.value = `请使用 ${toolId} 帮我处理：`
      nextTick(() => composerRef.value?.focus())
  }
}

function continueFromMessage() {
  sendPrompt('请承接你上一条答复继续深入展开可执行步骤、关键细节与具体产出物。')
}

watch(
  bot,
  (b) => {
    if (!b) return
    const normModel = !b.model || b.model === 'deepseek-flash' ? 'primary' : b.model
    selectedModel.value = normModel
    enableThinking.value = b.enable_thinking !== false
    announcementDismissed.value = false
    replyTarget.value = null
  },
  { immediate: true }
)

watch(
  () => store.state.pendingDraftPrompt,
  (draft) => {
    if (!draft) return
    inputPrompt.value = draft
    store.state.pendingDraftPrompt = ''
    nextTick(() => composerRef.value?.focus())
  },
  { immediate: true }
)

const isGroupBot = computed(() => Boolean(bot.value?.is_group))

const availableSpecialistBots = computed(() =>
  store.state.bots.filter((b) => !b.is_group)
)

const groupMembers = computed<Bot[]>(() => {
  if (!isGroupBot.value) return []
  let ids: string[] = []
  try {
    const parsed = JSON.parse(bot.value?.member_ids || '[]')
    if (Array.isArray(parsed)) ids = parsed.map(String)
  } catch {
    ids = []
  }
  const map = new Map(availableSpecialistBots.value.map((b) => [b.id, b]))
  const resolved = ids.map((id) => map.get(id)).filter((x): x is Bot => Boolean(x))
  return resolved.length > 0 ? resolved : availableSpecialistBots.value
})

async function onCarryThreadSummary(summary: string, title: string) {
  const nextTitle = `${title || '延续话题'} · 续`
  await store.createThread(nextTitle)
  inputPrompt.value = `接续上一段话题前情提要继续推进：\n> ${summary}\n\n接下来的重点：`
  nextTick(() => composerRef.value?.focus())
}

async function handleSummarizeThread() {
  if (!bot.value) return
  await store.summarizeThread(store.state.activeThreadId || 'main', true)
}

function shouldShowTimestamp(idx: number): boolean {
  const list = store.state.timeline
  const cur = list[idx]
  if (!cur?.createdAt) return false
  if (idx === 0) return true
  const prev = list[idx - 1]
  if (!prev?.createdAt) return false
  const tCur = new Date(cur.createdAt).getTime()
  const tPrev = new Date(prev.createdAt).getTime()
  if (Number.isNaN(tCur) || Number.isNaN(tPrev)) return false
  return tCur - tPrev > 5 * 60 * 1000
}

function setReplyQuote(msg: ChatMessageUI) {
  const clean = msg.content.replace(/\s+/g, ' ').trim()
  const snippet = clean.length > 72 ? `${clean.slice(0, 72)}...` : clean
  replyTarget.value = {
    serverId: msg.serverId || 0,
    senderName: msg.role === 'user' ? '管理员' : msg.senderName,
    preview: `${msg.role === 'user' ? '管理员' : msg.senderName}: ${snippet}`,
  }
  nextTick(() => composerRef.value?.focus())
}

async function saveMessageAsMemory(msg: ChatMessageUI) {
  const content = msg.content.replace(/\s+/g, ' ').trim().slice(0, 260)
  if (!content || !bot.value) return
  try {
    const key = `${msg.senderBotId || 'bot'}_note_${Date.now().toString(36)}`
    await BotHubApi.saveMemory({
      scope: `bot:${bot.value.id}`,
      category: 'insight',
      fact_key: key,
      content,
    })
    store.notify('已提炼并存入专属长期记忆库')
    await store.refreshWorkspaceSlices(['memories'])
  } catch (err: any) {
    store.notify(err.message || '保存记忆失败')
  }
}

async function convertMessageToTodo(msg: ChatMessageUI) {
  const raw = msg.content.replace(/\s+/g, ' ').trim()
  if (!raw || !bot.value) return
  const title = raw.length > 60 ? `${raw.slice(0, 60)}...` : raw
  try {
    await BotHubApi.createTodo({
      bot_id: bot.value.id,
      thread_id: store.state.activeThreadId || '',
      title,
      notes: raw.length > 60 ? raw : '',
      priority: 'normal',
      status: 'pending',
    })
    await store.refreshWorkspaceSlices(['todos'])
    store.notify('已转入个人待办清单')
  } catch (err: any) {
    store.notify(err.message || '转存待办失败')
  }
}

async function saveMessageAsNote(msg: ChatMessageUI) {
  const raw = msg.content.trim()
  if (!raw || !bot.value) return
  const firstLine = raw.split('\n')[0].replace(/^#+\s*/, '').trim()
  const title = firstLine.length > 40 ? `${firstLine.slice(0, 40)}...` : firstLine || `${bot.value.name} 对话成果`
  try {
    await BotHubApi.createNote({
      bot_id: bot.value.id,
      title,
      category: 'deliverable',
      content: raw,
    })
    await store.refreshWorkspaceSlices(['notes'])
    store.notify(`已保存至工作文档《${title}》`)
  } catch (err: any) {
    store.notify(err.message || '保存文档失败')
  }
}

const scopedMemories = computed(() => {
  const serverMems = store.state.botMemories || []
  if (serverMems.length > 0) return serverMems
  const cur = bot.value
  if (!cur) return []
  const bScope = `bot:${cur.id}`
  if (cur.memory_policy === 'isolated') {
    return store.state.memories.filter((m) => m.scope === bScope)
  }
  return store.state.memories.filter((m) => !m.scope || m.scope === 'global' || m.scope === bScope)
})

function shouldDeferMessageRender(idx: number): boolean {
  if (chatSearchOpen.value) return false
  return idx < store.state.timeline.length - 10
}

function onMessagesScroll() {
  const el = scrollContainerRef.value
  if (!el) return
  const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
  const scrolledUp = distanceFromBottom > 80
  userScrolledUp.value = scrolledUp
  if (!scrolledUp) {
    unreadBelowCount.value = 0
  }
}

function scrollToBottom(force = false) {
  if (force) {
    userScrolledUp.value = false
    unreadBelowCount.value = 0
  }
  if (!force && userScrolledUp.value) return
  nextTick(() => {
    if (scrollContainerRef.value) {
      scrollContainerRef.value.scrollTop = scrollContainerRef.value.scrollHeight
    }
  })
}

function editResendUserMessage(msg: ChatMessageUI) {
  if (!msg.content) return
  inputPrompt.value = msg.content
  if (msg.replyPreview) {
    replyTarget.value = {
      serverId: msg.replyToId || 0,
      senderName: '',
      preview: msg.replyPreview,
    }
  }
  nextTick(() => composerRef.value?.focus())
  store.notify('已填入编辑框，修改后直接发送')
}

watch(inputPrompt, (val) => {
  if (store.state.activeBotId) {
    store.setBotDraft(store.state.activeBotId, val, store.state.activeThreadId)
  }
})

watch(
  () => [store.state.activeBotId, store.state.activeThreadId] as const,
  ([newId, newThread], [oldId, oldThread]) => {
    if (oldId && (oldId !== newId || oldThread !== newThread)) {
      store.setBotDraft(oldId, inputPrompt.value, oldThread || '')
    }
    const draft = store.state.pendingDraftPrompt ? '' : store.getBotDraft(newId, newThread || '')
    if (!store.state.pendingDraftPrompt) {
      inputPrompt.value = draft
    }
    closeChatSearch()
    userScrolledUp.value = false
    unreadBelowCount.value = 0
    scrollToBottom(true)
  }
)

watch(
  () => store.state.timeline.length,
  (newLen, oldLen) => {
    if (userScrolledUp.value && typeof oldLen === 'number' && newLen > oldLen) {
      unreadBelowCount.value += newLen - oldLen
    } else if (newLen === 0) {
      unreadBelowCount.value = 0
    }
    scrollToBottom(false)
  }
)

function handleWorkspaceKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'f') {
    if (store.state.workspace === 'bots' && store.state.botView === 'stream') {
      e.preventDefault()
      openChatSearch()
    }
  } else if (e.key === 'F3' && chatSearchOpen.value) {
    e.preventDefault()
    jumpSearchMatch(e.shiftKey ? -1 : 1)
  } else if (e.key === 'Escape' && chatSearchOpen.value) {
    closeChatSearch()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleWorkspaceKeydown)
  const savedDraft = store.getBotDraft(store.state.activeBotId)
  if (savedDraft && !inputPrompt.value) {
    inputPrompt.value = savedDraft
  }
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleWorkspaceKeydown)
})

function appendMention(tag: string) {
  const prefix = inputPrompt.value && !inputPrompt.value.endsWith(' ') ? ' ' : ''
  inputPrompt.value = `${inputPrompt.value}${prefix}${tag}`
  nextTick(() => composerRef.value?.focus())
}

function handleMarkdownClick(e: MouseEvent) {
  handleMarkdownCodeCopy(e, () => store.notify('代码块已复制'))
}

async function copyMessageContent(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    store.notify('已复制回复 Markdown 全文')
  } catch {
    store.notify('复制失败')
  }
}

function stripMarkdownFormatting(raw: string): string {
  return String(raw || '')
    .replace(/```[a-zA-Z0-9_-]*\n([\s\S]*?)```/g, '$1')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\*\*([^*]+)\*\*/g, '$1')
    .replace(/(?<!\*)\*([^*]+)\*(?!\*)/g, '$1')
    .replace(/~~([^~]+)~~/g, '$1')
    .replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '$1 ($2)')
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/^>\s?/gm, '')
    .replace(/^[-*]\s+\[([ xX])\]\s+/gm, (_, mark: string) => (mark.toLowerCase() === 'x' ? '[已完成] ' : '[待办] '))
    .trim()
}

async function copyPlainTextContent(text: string) {
  try {
    await navigator.clipboard.writeText(stripMarkdownFormatting(text))
    store.notify('已复制纯文本内容')
  } catch {
    store.notify('复制失败')
  }
}

function stopStream() {
  abortCtrl?.abort()
}

async function sendPrompt(overrideText?: string) {
  if (store.state.isStreaming) return
  const msg = (overrideText ?? inputPrompt.value).trim()
  if (!msg) return

  const curBot = bot.value
  if (!curBot) {
    store.notify('请先创建一个 Bot 再开始对话')
    return
  }
  const botId = curBot.id
  const quote = replyTarget.value
  lastPrompt.value = msg
  if (!overrideText) inputPrompt.value = ''
  composerRef.value?.closePopovers()
  replyTarget.value = null
  relaySteps.value = []

  store.state.isStreaming = true
  abortCtrl = new AbortController()
  streamStatus.value = `连线 [${curBot?.name || botId}]...`

  store.state.timeline.push({
    localId: `u_${Date.now()}`,
    role: 'user',
    senderBotId: 'user',
    senderName: '管理员',
    avatarStyle: 'cyan',
    model: selectedModel.value,
    msgKind: 'chat',
    createdAt: new Date().toISOString(),
    content: msg,
    reasoning: '',
    tools: [],
    replyToId: quote?.serverId,
    replyPreview: quote?.preview,
    statusText: '已发送',
    isLiveNew: true,
  })

  const createTurn = (bId: string, bName: string, style: string, roleTitle = '', isTempGuest = false): ChatMessageUI => {
    const turn: ChatMessageUI = {
      localId: `a_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`,
      role: 'assistant',
      senderBotId: bId,
      senderName: bName,
      roleTitle,
      avatarStyle: style,
      model: selectedModel.value,
      msgKind: 'chat',
      createdAt: new Date().toISOString(),
      content: '',
      reasoning: '',
      tools: [],
      isTempGuest,
      statusText: '思考与生成中...',
      streaming: true,
      isLiveNew: true,
    }
    store.state.timeline.push(turn)
    return store.state.timeline[store.state.timeline.length - 1]
  }

  let activeTurn = createTurn(botId, curBot?.name || 'BotHub', curBot?.avatar_style || 'cyan', curBot?.role_title || '')
  const turnStartedAt = Date.now()
  scrollToBottom(true)

  try {
    const res = await fetch('/api/agent/chat', {
      method: 'POST',
      credentials: 'same-origin',
      signal: abortCtrl.signal,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        bot_id: botId,
        thread_id: store.state.activeThreadId || '',
        message: msg,
        model: selectedModel.value,
        enable_thinking: enableThinking.value,
        reply_to_id: quote?.serverId || 0,
        reply_preview: quote?.preview || '',
      }),
    })
    if (!res.ok) {
      const errData = await res.json().catch(() => ({}))
      throw new Error(errData.error || `HTTP ${res.status}`)
    }

    const reader = res.body!.getReader()
    const decoder = new TextDecoder('utf-8')
    let buffer = ''
    let currentEvent = 'message'

    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const rawLine of lines) {
        const line = rawLine.trim()
        if (!line) continue
        if (line.startsWith('event:')) {
          currentEvent = line.slice(6).trim()
        } else if (line.startsWith('data:')) {
          let payload: any = {}
          try {
            payload = JSON.parse(line.slice(5).trim())
          } catch {
            continue
          }

          if (currentEvent === 'system_notice') {
            const idx = store.state.timeline.indexOf(activeTurn)
            const sysItem: ChatMessageUI = {
              localId: `srv_${payload.id || Date.now()}`,
              serverId: payload.id,
              role: 'system',
              senderBotId: botId,
              senderName: '系统动态',
              avatarStyle: curBot?.avatar_style || 'cyan',
              model: selectedModel.value,
              msgKind: 'system',
              createdAt: payload.created_at || new Date().toISOString(),
              content: payload.content || '',
              reasoning: '',
              tools: [],
              statusText: '',
              isLiveNew: true,
            }
            if (idx >= 0) store.state.timeline.splice(idx, 0, sysItem)
            else store.state.timeline.push(sysItem)
          } else if (currentEvent === 'bot_turn') {
            relaySteps.value.forEach((s) => {
              s.status = 'done'
            })
            relaySteps.value.push({
              id: payload.sender_bot_id,
              name: payload.sender_name,
              status: 'active',
              guest: Boolean(payload.is_temp_guest),
            })
            if (Number(payload.step) > 1) {
              activeTurn.statusText = '已完成'
              activeTurn.streaming = false
              activeTurn = createTurn(
                payload.sender_bot_id,
                payload.sender_name,
                payload.avatar_style || 'cyan',
                payload.role_title || '',
                Boolean(payload.is_temp_guest)
              )
            } else {
              activeTurn.senderBotId = payload.sender_bot_id
              activeTurn.senderName = payload.sender_name
              activeTurn.roleTitle = payload.role_title || ''
              activeTurn.avatarStyle = payload.avatar_style || 'cyan'
              activeTurn.isTempGuest = Boolean(payload.is_temp_guest)
            }
            streamStatus.value = `协同接力 (${payload.step}/${payload.total_steps}): ${payload.sender_name}`
          } else if (currentEvent === 'reasoning') {
            streamStatus.value = '深度思考中...'
            activeTurn.reasoning += payload.delta || ''
            scrollToBottom()
          } else if (currentEvent === 'tool_start') {
            streamStatus.value = `执行工具: ${payload.name}...`
            activeTurn.tools.push({
              id: payload.id,
              name: payload.name,
              arguments: payload.arguments,
              done: false,
            })
            scrollToBottom()
          } else if (currentEvent === 'tool_result') {
            const found = activeTurn.tools.find((t) => t.id === payload.id)
            if (found) {
              found.done = true
              found.result = payload.result
              if (payload.duration_ms !== undefined) found.durationMs = Number(payload.duration_ms)
              if (payload.status) found.status = String(payload.status)
            }
          } else if (currentEvent === 'content') {
            streamStatus.value = '正在生成回复...'
            activeTurn.content += payload.delta || ''
            scrollToBottom()
          } else if (currentEvent === 'model_switch') {
            streamStatus.value = `${payload.from_label || '当前节点'} 限流或异常，已自动切换至 ${payload.to_label || payload.to_model}`
          } else if (currentEvent === 'error') {
            activeTurn.statusText = '异常中断'
            activeTurn.errorText = payload.error || '未知流式错误'
            activeTurn.streaming = false
          } else if (currentEvent === 'done') {
            activeTurn.streaming = false
            if (payload.model) activeTurn.model = payload.model
            const elapsedSec = ((Date.now() - turnStartedAt) / 1000).toFixed(1)
            activeTurn.statusText =
              payload.usage && payload.usage.total_tokens
                ? `${payload.usage.prompt_tokens}+${payload.usage.completion_tokens} Tokens · ${elapsedSec}s`
                : `已完成 · ${elapsedSec}s`
          }
        }
      }
    }

    activeTurn.streaming = false
    relaySteps.value.forEach((s) => {
      s.status = 'done'
    })
    window.setTimeout(() => {
      if (!store.state.isStreaming) relaySteps.value = []
    }, 1600)
    if (activeTurn.statusText === '思考与生成中...') activeTurn.statusText = '已完成'
    streamStatus.value = '就绪'
    store.playSubtleChime()
    await store.refreshWorkspaceSlices(['bots', 'memories', 'todos', 'notes', 'model_health'])
    // Reload synced server IDs, threads, memories and execution logs quietly
    if (curBot?.id === store.state.activeBotId) {
      const synced = await BotHubApi.getBotMessages(curBot.id, store.state.activeThreadId).catch(() => null)
      if (synced?.threads) store.state.botThreads = synced.threads
      if (synced?.memories) store.state.botMemories = synced.memories
      if (synced?.executions) store.state.botExecutions = synced.executions
      if (synced?.messages && !store.state.isStreaming) {
        const finalTurnStatus = activeTurn.statusText
        const mappedSynced = mapServerMessages(synced.messages, curBot)
        if (mappedSynced.length >= store.state.timeline.length) {
          if (finalTurnStatus && mappedSynced.length > 0 && mappedSynced[mappedSynced.length - 1].role === 'assistant') {
            mappedSynced[mappedSynced.length - 1].statusText = finalTurnStatus
          }
          store.state.timeline = mappedSynced
        }
      }
    }
  } catch (err: any) {
    activeTurn.streaming = false
    if (err.name === 'AbortError') {
      activeTurn.statusText = '已手动停止'
      streamStatus.value = '已手动停止'
      store.notify('已停止生成')
    } else {
      activeTurn.statusText = '异常中断'
      activeTurn.errorText = err.message
      streamStatus.value = '发生错误'
      store.notify(err.message)
    }
  } finally {
    store.state.isStreaming = false
    abortCtrl = null
  }
}

function exportTimelineMarkdown() {
  if (!store.state.timeline.length) {
    store.notify('当前 Bot 时间线暂无内容可导出')
    return
  }
  const bName = bot.value?.name || 'BotHub'
  const activeTh = store.state.botThreads.find((t) => t.id === store.state.activeThreadId)
  const thTitle = activeTh ? activeTh.title : '主时间线'
  const parts = [
    `# Vesper BotHub · ${bName}（${thTitle}）\n\n- 导出时间: ${new Date().toISOString()}\n`,
  ]
  if (scopedMemories.value.length > 0) {
    parts.push('\n## 核心记忆上下文\n\n')
    scopedMemories.value.slice(0, 10).forEach((mem) => {
      parts.push(`- **${mem.fact_key}** (${mem.category}): ${mem.content}\n`)
    })
  }
  for (const m of store.state.timeline) {
    parts.push(`\n---\n\n### ${m.role === 'user' ? '管理员指令' : m.role === 'system' ? '系统动态' : `${m.senderName} (${m.model})`}\n`)
    if (m.reasoning) {
      parts.push(`\n> 深度思考:\n> ${m.reasoning.replace(/\n/g, '\n> ')}\n`)
    }
    if (m.tools && m.tools.length > 0) {
      parts.push('\n<details><summary>工具调用轨迹</summary>\n\n')
      m.tools.forEach((t, ti) => {
        parts.push(`- #${ti + 1} \`${t.name}\` (${t.durationMs !== undefined ? t.durationMs + 'ms' : '完成'})\n`)
      })
      parts.push('\n</details>\n')
    }
    if (m.content) parts.push(`\n${m.content}\n`)
  }
  const blob = new Blob([parts.join('')], { type: 'text/markdown;charset=utf-8' })
  triggerBlobDownload(blob, `bothub_${bot.value?.id || 'bot'}_${Date.now()}.md`)
  store.notify('已导出为结构化 Markdown')
}

async function clearTimeline() {
  if (store.state.isStreaming || !bot.value) return
  if (!window.confirm(`确定清空 [${bot.value.name}] 的对话时间线吗？`)) return
  await store.clearActiveBotTimeline()
}
</script>

<template>
  <section class="flex-1 min-h-0 flex flex-col">
    <BotWorkspaceHeader
      :bot="bot"
      :is-group-bot="isGroupBot"
      :group-members="groupMembers"
      :desktop-collapsed="desktopCollapsed"
      @open-mobile-menu="emit('openMobileMenu')"
      @toggle-desktop-sidebar="emit('toggleDesktopSidebar')"
      @open-chat-search="openChatSearch"
      @export-markdown="exportTimelineMarkdown"
      @clear-timeline="clearTimeline"
    />

    <!-- Empty Team Onboarding View when 0 Bots exist -->
    <div v-if="!bot" class="flex-1 min-h-0 overflow-y-auto px-4 sm:px-8 py-10 flex items-center justify-center">
      <div class="max-w-lg w-full">
        <div class="luxury-card rounded-2xl p-6 sm:p-8 flex flex-col gap-4 text-center items-center">
          <BotAvatar avatar-style="butler:gold" state="idle" size="lg" />
          <div>
            <h2 class="text-lg font-semibold text-white">工作台准备就绪</h2>
            <p class="text-xs text-slate-400 mt-1.5 leading-relaxed max-w-md mx-auto">
              当前尚未添加 Bot 或协同群聊。你可以先在设置中配置模型并实时探测可用性，再按需创建专属 Bot。
            </p>
          </div>
          <div class="flex flex-wrap items-center justify-center gap-2.5 pt-1">
            <button
              type="button"
              @click="store.state.createModalMode = 'bot'; store.state.showCreateBotModal = true"
              class="min-h-[42px] px-4 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer"
            >
              + 新建 Bot
            </button>
            <button
              type="button"
              @click="store.state.showSettingsModal = true"
              class="min-h-[42px] px-4 py-2 rounded-xl border border-[#f2b880]/35 bg-[#f2b880]/10 hover:bg-[#f2b880]/20 text-xs font-medium text-[#f2b880] cursor-pointer"
            >
              配置模型 Key
            </button>
          </div>
        </div>
      </div>
    </div>

    <template v-else>
    <!-- View 1: Persistent Stream -->
    <Transition :name="streamEnterTransition">
      <div v-show="store.state.botView === 'stream'" class="flex-1 min-h-0 flex flex-col">
        <BotThreadBar v-if="bot" :bot="bot" @carry-summary="onCarryThreadSummary" />

        <!-- Slide-Down In-Chat Search Bar (Cmd/Ctrl + F) -->
        <BotChatSearchBar
          :open="chatSearchOpen"
          v-model:query="chatSearchQuery"
          :match-count="chatSearchMatches.length"
          :cursor="chatSearchCursor"
          @jump="jumpSearchMatch"
          @close="closeChatSearch"
        />

        <!-- Pinned Group Announcement Banner (QQ/WeChat style) -->
        <Transition
          enter-active-class="transition-[opacity,transform] duration-180 ease-[var(--ease-snappy)] motion-reduce:duration-100"
          enter-from-class="opacity-0 -translate-y-1.5 motion-reduce:translate-y-0"
          enter-to-class="opacity-100 translate-y-0"
          leave-active-class="transition-[opacity,transform] duration-130 ease-[var(--ease-in-snappy)] motion-reduce:duration-75"
          leave-from-class="opacity-100 translate-y-0"
          leave-to-class="opacity-0 -translate-y-1.5 motion-reduce:translate-y-0"
        >
          <div
            v-if="isGroupBot && bot?.announcement && !announcementDismissed"
            class="px-4 md:px-6 py-2 border-b border-[#f2b880]/15 bg-[#f2b880]/[0.05] flex items-center justify-between gap-3 shrink-0"
          >
            <button
              type="button"
              @click="store.state.showGroupDrawer = true"
              class="flex items-center gap-2 min-w-0 text-left text-xs text-slate-300 hover:text-white cursor-pointer"
            >
              <span class="px-1.5 py-0.5 rounded bg-[#f2b880]/20 text-[#f2b880] font-mono text-[10px] font-semibold shrink-0">
                群公告
              </span>
              <span class="truncate">{{ bot.announcement }}</span>
            </button>
            <button
              type="button"
              aria-label="暂时收起群公告"
              @click="announcementDismissed = true"
              class="w-6 h-6 rounded-full inline-flex items-center justify-center text-slate-500 hover:text-white cursor-pointer shrink-0"
            >
              <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>
        </Transition>

        <div class="relative flex-1 min-h-0 flex flex-col">
          <div
            ref="scrollContainerRef"
            @scroll="onMessagesScroll"
            @click="handleMarkdownClick"
            class="cq-container stream-scroller flex-1 min-h-0 overflow-y-auto px-4 md:px-6 py-5 flex flex-col gap-5"
          >
            <BotStarterPanel
              v-if="store.state.timeline.length === 0"
              :bot="bot"
              :is-group-bot="isGroupBot"
              :group-members="groupMembers"
              @starter="runQuickToolStarter"
            />

            <!-- Messages -->
            <template v-for="(m, idx) in store.state.timeline" :key="m.localId">
              <!-- QQ/WeChat 5-Minute Time Cluster Divider -->
              <div
                v-if="shouldShowTimestamp(idx)"
                class="flex justify-center py-0.5 select-none"
              >
                <span class="px-2.5 py-0.5 rounded-full bg-[#fff8ee]/[0.03] text-[10.5px] font-mono text-slate-500">
                  {{ formatTimelineDivider(m.createdAt) }}
                </span>
              </div>

              <!-- QQ/WeChat Centered System Dynamic Pill (Invite / Remove / Adhoc / Context Sync) -->
              <div
                v-if="m.role === 'system' || m.msgKind === 'system'"
                :class="['max-w-[680px] mx-auto flex justify-center my-0.5', m.isLiveNew ? 'msg-enter' : '']"
              >
                <div class="px-3.5 py-1 rounded-2xl border border-[#fff8ee]/8 bg-[#fff8ee]/[0.04] text-[11.5px] text-slate-400 text-center whitespace-pre-line leading-relaxed">
                  {{ m.content }}
                </div>
              </div>

              <!-- User Message Pill -->
              <div
                v-else-if="m.role === 'user'"
                :data-msg-id="m.localId"
                :class="[
                  'reveal-host max-w-[820px] w-full mx-auto flex flex-col items-end gap-1 rounded-2xl transition-shadow',
                  m.isLiveNew ? 'msg-enter-user' : '',
                  shouldDeferMessageRender(idx) ? 'msg-deferred-user' : '',
                  focusedMatchId === m.localId ? 'ring-1 ring-[#f2b880]/70 p-1' : ''
                ]"
              >
                <div
                  :title="formatTime(m.createdAt)"
                  class="max-w-[85%] sm:max-w-[74%] rounded-2xl rounded-tr-sm px-4 py-2.5 bg-[#f2b880]/14 border border-[#f2b880]/28 text-sm text-slate-100 leading-relaxed whitespace-pre-wrap"
                >
                  <div
                    v-if="m.replyPreview"
                    class="mb-2 pl-2.5 border-l-2 border-[#f2b880]/70 text-xs text-slate-300/85 bg-[#050811]/35 rounded-r-lg py-1 pr-2 line-clamp-2"
                  >
                    {{ m.replyPreview }}
                  </div>
                  <div :class="isLongUserMessage(m.content) && !expandedUserMsgs[m.localId] ? 'line-clamp-5' : ''">
                    {{ m.content }}
                  </div>
                  <button
                    v-if="isLongUserMessage(m.content)"
                    type="button"
                    @click="toggleExpandUserMsg(m.localId)"
                    class="mt-1.5 font-mono text-[11px] text-[#f2b880] hover:underline cursor-pointer"
                  >
                    {{ expandedUserMsgs[m.localId] ? '收起长消息' : '展开全文' }}
                  </button>
                </div>
                <div class="reveal-actions flex items-center gap-1.5 px-1">
                  <button
                    type="button"
                    @click="setReplyQuote(m)"
                    class="px-2 py-0.5 rounded-md text-[10.5px] text-slate-500 hover:text-[#f2b880] cursor-pointer"
                  >
                    引用
                  </button>
                  <button
                    type="button"
                    @click="editResendUserMessage(m)"
                    class="px-2 py-0.5 rounded-md text-[10.5px] text-slate-500 hover:text-[#f2b880] cursor-pointer"
                  >
                    编辑重发
                  </button>
                  <button
                    v-if="m.serverId"
                    type="button"
                    @click="store.forkThread(m.serverId)"
                    class="px-2 py-0.5 rounded-md text-[10.5px] text-slate-500 hover:text-emerald-300 cursor-pointer"
                  >
                    分支话题
                  </button>
                  <button
                    type="button"
                    @click="copyMessageContent(m.content)"
                    class="px-2 py-0.5 rounded-md text-[10.5px] text-slate-500 hover:text-[#f2b880] cursor-pointer"
                  >
                    复制
                  </button>
                  <button
                    type="button"
                    @click="openForwardForMessage(m)"
                    class="px-2 py-0.5 rounded-md text-[10.5px] text-slate-500 hover:text-[#f2b880] cursor-pointer"
                  >
                    转发
                  </button>
                  <button
                    v-if="m.serverId"
                    type="button"
                    @click="store.deleteSingleMessage(m)"
                    class="px-2 py-0.5 rounded-md text-[10.5px] text-slate-500 hover:text-rose-300 cursor-pointer"
                  >
                    删除
                  </button>
                </div>
              </div>

              <!-- Bot Message -->
              <article
                v-else
                :data-msg-id="m.localId"
                :class="[
                  'reveal-host max-w-[820px] w-full mx-auto rounded-2xl transition-shadow',
                  m.isLiveNew ? 'msg-enter-bot' : '',
                  shouldDeferMessageRender(idx) ? 'msg-deferred-bot' : '',
                  focusedMatchId === m.localId ? 'ring-1 ring-[#f2b880]/70 bg-[#f2b880]/[0.04] px-2.5 py-1.5' : ''
                ]"
              >
                <div
                  :class="[
                    'flex flex-col gap-2.5',
                    m.msgKind === 'routine'
                      ? 'vesper-card rounded-2xl border-[#f2b880]/28 p-4'
                      : 'py-1'
                  ]"
                >
                  <!-- Quiet editorial sender header with clickable @Bot & Role Tag -->
                  <div class="flex items-center justify-between gap-2 text-xs text-slate-400">
                    <div class="flex items-center gap-2 min-w-0">
                      <button
                        type="button"
                        @click="appendMention(`@${m.senderName} `)"
                        :title="`点击 @${m.senderName}`"
                        class="inline-flex items-center gap-2 min-w-0 cursor-pointer group/sender"
                      >
                        <BotAvatar :avatar-style="m.avatarStyle" :state="m.streaming ? 'working' : (m.msgKind === 'routine' ? 'done' : 'idle')" size="sm" :interactive="false" />
                        <span class="text-[#f2b880] group-hover/sender:underline font-medium truncate">{{ m.senderName }}</span>
                      </button>
                      <span
                        v-if="m.roleTitle && isGroupBot"
                        class="hidden sm:inline-flex font-mono text-[10px] px-1.5 py-0.5 rounded bg-[#fff8ee]/[0.05] text-slate-400 truncate max-w-[160px]"
                      >
                        {{ m.roleTitle }}
                      </span>
                      <span
                        v-if="m.isTempGuest"
                        class="font-mono text-[10px] px-1.5 py-0.5 rounded border border-emerald-400/30 bg-emerald-400/10 text-emerald-300 shrink-0"
                      >
                        临时支援
                      </span>
                      <span v-if="m.msgKind === 'routine'" class="font-mono text-[10px] px-1.5 py-0.5 rounded bg-[#f2b880]/12 text-[#f2b880] shrink-0">
                        例程汇报
                      </span>
                    </div>
                    <div class="flex items-center gap-2 shrink-0">
                      <span
                        v-if="m.model && !m.streaming"
                        class="hidden sm:inline-flex font-mono text-[10px] px-1.5 py-0.5 rounded bg-[#fff8ee]/[0.04] text-slate-400"
                      >
                        {{ m.model }}
                      </span>
                      <button
                        v-if="isGroupBot && m.senderBotId && m.senderBotId !== bot?.id"
                        type="button"
                        @click="store.jumpToBot(m.senderBotId)"
                        class="reveal-actions font-mono text-[10px] px-1.5 py-0.5 rounded border border-[#fff8ee]/12 text-slate-400 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
                      >
                        私聊TA
                      </button>
                      <span :class="['font-mono text-[11px]', m.errorText ? 'text-rose-400' : 'text-slate-500']">
                        {{ m.statusText }}
                      </span>
                    </div>
                  </div>

                  <!-- Calm Thinking Waveform (before first token arrives) -->
                  <div
                    v-if="m.streaming && !m.content && !m.reasoning && !m.tools.length"
                    class="py-1.5 flex items-center gap-2.5 text-xs text-slate-400"
                    aria-live="polite"
                  >
                    <span class="thinking-dots inline-flex items-center gap-1" aria-hidden="true">
                      <span /><span /><span />
                    </span>
                    <span>思考中</span>
                  </div>

                  <!-- Smooth Interpolate-Size Collapsible Reasoning -->
                  <details
                    v-if="m.reasoning"
                    :open="m.streaming && !m.content"
                    class="vesper-details rounded-xl border border-amber-400/20 bg-[#060a16]/65 px-3 py-2 text-xs text-slate-400"
                  >
                    <summary class="cursor-pointer flex items-center gap-1.5 font-mono text-amber-300/90 select-none">
                      <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="details-chevron shrink-0">
                        <polyline points="9 18 15 12 9 6" />
                      </svg>
                      <span>深度思考过程</span>
                      <span class="text-[10px] text-amber-300/60">({{ m.reasoning.length }} 字)</span>
                    </summary>
                    <div class="mt-2 font-mono whitespace-pre-wrap leading-relaxed text-slate-300/90">
                      {{ m.reasoning }}
                    </div>
                  </details>

                  <!-- Smooth Collapsible Merged Tool Call Trajectory -->
                  <BotToolTrail :tools="m.tools" @copy="copyMessageContent" />

                  <!-- Markdown Body -->
                  <div v-if="m.content" :class="m.streaming ? 'md-streaming' : ''" v-html="renderSafeMarkdown(m.content)" />
                  <div v-else-if="m.errorText" class="flex items-center justify-between gap-2 flex-wrap text-sm text-rose-400">
                    <span>[调用失败] {{ m.errorText }}</span>
                    <button
                      v-if="findPrecedingUserPrompt(idx)"
                      type="button"
                      @click="sendPrompt(findPrecedingUserPrompt(idx))"
                      class="px-2.5 py-1 rounded-lg border border-rose-400/45 bg-rose-500/15 hover:bg-rose-500/25 text-xs text-rose-200 cursor-pointer shrink-0"
                    >
                      立即重试
                    </button>
                  </div>

                  <!-- Quiet Footer Actions (hover / keyboard / touch) -->
                  <div v-if="!m.streaming && m.content" class="reveal-actions flex items-center flex-wrap gap-1.5 pt-1">
                    <button
                      type="button"
                      @click="setReplyQuote(m)"
                      class="min-h-[36px] sm:min-h-[26px] px-2.5 py-0.5 rounded-md border border-[#fff8ee]/10 text-[11px] text-slate-400 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
                    >
                      引用
                    </button>
                    <button
                      type="button"
                      @click="saveMessageAsMemory(m)"
                      class="min-h-[36px] sm:min-h-[26px] px-2.5 py-0.5 rounded-md border border-[#fff8ee]/10 text-[11px] text-slate-400 hover:text-emerald-300 hover:border-emerald-400/40 cursor-pointer"
                    >
                      存为记忆
                    </button>
                    <button
                      type="button"
                      @click="convertMessageToTodo(m)"
                      class="min-h-[36px] sm:min-h-[26px] px-2.5 py-0.5 rounded-md border border-[#fff8ee]/10 text-[11px] text-slate-400 hover:text-cyan-300 hover:border-cyan-400/40 cursor-pointer"
                    >
                      转待办
                    </button>
                    <button
                      type="button"
                      @click="saveMessageAsNote(m)"
                      class="min-h-[36px] sm:min-h-[26px] px-2.5 py-0.5 rounded-md border border-[#fff8ee]/10 text-[11px] text-slate-400 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
                    >
                      存文档
                    </button>
                    <button
                      type="button"
                      @click="openForwardForMessage(m)"
                      class="min-h-[36px] sm:min-h-[26px] px-2.5 py-0.5 rounded-md border border-[#fff8ee]/10 text-[11px] text-slate-400 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
                    >
                      转发
                    </button>
                    <button
                      type="button"
                      @click="copyMessageContent(m.content)"
                      class="min-h-[36px] sm:min-h-[26px] px-2.5 py-0.5 rounded-md border border-[#fff8ee]/10 text-[11px] text-slate-400 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
                    >
                      复制全文
                    </button>
                    <button
                      type="button"
                      @click="copyPlainTextContent(m.content)"
                      class="min-h-[36px] sm:min-h-[26px] px-2.5 py-0.5 rounded-md border border-[#fff8ee]/10 text-[11px] text-slate-400 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
                    >
                      复制纯文本
                    </button>
                    <button
                      v-if="m.serverId"
                      type="button"
                      @click="store.forkThread(m.serverId)"
                      class="min-h-[36px] sm:min-h-[26px] px-2.5 py-0.5 rounded-md border border-[#fff8ee]/10 text-[11px] text-slate-400 hover:text-emerald-300 hover:border-emerald-400/40 cursor-pointer"
                    >
                      分支话题
                    </button>
                    <button
                      v-if="idx === store.state.timeline.length - 1"
                      type="button"
                      @click="continueFromMessage()"
                      class="min-h-[36px] sm:min-h-[26px] px-2.5 py-0.5 rounded-md border border-[#fff8ee]/10 text-[11px] text-slate-400 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
                    >
                      继续补充
                    </button>
                    <button
                      v-if="findPrecedingUserPrompt(idx)"
                      type="button"
                      @click="sendPrompt(findPrecedingUserPrompt(idx))"
                      class="min-h-[36px] sm:min-h-[26px] px-2.5 py-0.5 rounded-md border border-[#fff8ee]/10 text-[11px] text-slate-400 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
                    >
                      重试该轮
                    </button>
                    <button
                      v-if="m.serverId"
                      type="button"
                      @click="store.deleteSingleMessage(m)"
                      class="min-h-[36px] sm:min-h-[26px] px-2.5 py-0.5 rounded-md border border-[#fff8ee]/10 text-[11px] text-slate-500 hover:text-rose-300 hover:border-rose-400/40 cursor-pointer"
                    >
                      删除
                    </button>
                  </div>
                </div>
              </article>
            </template>
          </div>

          <!-- Spring Floating Scroll-to-Bottom Button -->
          <Transition
            enter-active-class="transition duration-250 ease-[var(--ease-spring)]"
            enter-from-class="opacity-0 translate-y-3 scale-90"
            enter-to-class="opacity-100 translate-y-0 scale-100"
            leave-active-class="transition duration-150 ease-in"
            leave-from-class="opacity-100 translate-y-0 scale-100"
            leave-to-class="opacity-0 translate-y-2 scale-95"
          >
            <button
              v-if="userScrolledUp"
              type="button"
              @click="scrollToBottom(true)"
              class="absolute bottom-3 left-1/2 -translate-x-1/2 z-10 inline-flex items-center gap-1.5 min-h-[40px] sm:min-h-[34px] px-4 rounded-full border border-[#f2b880]/45 bg-[#0a0f1f]/95 text-xs font-medium text-[#f2b880] shadow-xl backdrop-blur-md hover:bg-[#f2b880]/15 cursor-pointer"
            >
              <span
                v-if="store.state.isStreaming || unreadBelowCount > 0"
                :class="['w-1.5 h-1.5 rounded-full shrink-0', store.state.isStreaming ? 'bg-[#f2b880] animate-pulse' : 'bg-emerald-400']"
                aria-hidden="true"
              />
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <polyline points="6 9 12 15 18 9" />
              </svg>
              <span>
                {{
                  store.state.isStreaming
                    ? '正在生成 · 回到底部'
                    : unreadBelowCount > 0
                      ? `${unreadBelowCount} 条新消息 · 回到底部`
                      : '回到底部'
                }}
              </span>
            </button>
          </Transition>
        </div>

        <!-- Floating Composer Dock + Popovers -->
        <BotChatComposer
          ref="composerRef"
          v-model:input-prompt="inputPrompt"
          v-model:selected-model="selectedModel"
          v-model:enable-thinking="enableThinking"
          v-model:reply-target="replyTarget"
          :stream-status="streamStatus"
          :relay-steps="relaySteps"
          :is-group-bot="isGroupBot"
          :group-members="groupMembers"
          :last-prompt="lastPrompt"
          @submit-prompt="sendPrompt"
          @stop-stream="stopStream"
          @clear-timeline="clearTimeline"
          @summarize-thread="handleSummarizeThread"
        />
      </div>
    </Transition>

    <!-- Secondary Views: Directional Spatial Transitions -->
    <Transition :name="botSubViewTransition" mode="out-in">
      <BotRoutinesTab v-if="store.state.botView === 'routines'" key="routines" />
      <BotConfigTab v-else-if="store.state.botView === 'config'" key="config" />
      <BotActivityTab v-else-if="store.state.botView === 'activity'" key="activity" />
    </Transition>
    </template>

    <GroupDrawer />
  </section>
</template>
