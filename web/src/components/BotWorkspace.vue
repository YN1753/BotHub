<script setup lang="ts">
import { ref, computed, watch, nextTick, useId, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore, mapServerMessages, type ChatMessageUI } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { botStateLabel, describeCron, formatTime, formatTimelineDivider, groupModeLabel } from '../utils/format'
import { renderSafeMarkdown } from '../utils/markdown'
import { applySiblingStaggerFallback, isImeComposingEnter, trackSpotlight } from '../utils/topLayer'
import type { AgentMemory, Bot, BotMode, MemoryPolicy } from '../types/bothub'
import BotAvatar from './BotAvatar.vue'
import BotGroupAvatar from './BotGroupAvatar.vue'
import BotAvatarPicker from './BotAvatarPicker.vue'
import CustomSelect from './CustomSelect.vue'
import SegmentedTabs from './SegmentedTabs.vue'
import ToggleSwitch from './ToggleSwitch.vue'
import GroupDrawer from './GroupDrawer.vue'

defineProps<{
  desktopCollapsed?: boolean
}>()

const emit = defineEmits<{
  openMobileMenu: []
  toggleDesktopSidebar: []
}>()

const store = useAppStore()
const bot = store.activeBot

const moreMenuId = useId()
const composerId = useId()

const inputPrompt = ref('')
const selectedModel = ref('primary')
const enableThinking = ref(true)
const streamStatus = ref('就绪')
const scrollContainerRef = ref<HTMLElement | null>(null)
const starterGridRef = ref<HTMLElement | null>(null)
const routinesGridRef = ref<HTMLElement | null>(null)
const activityListRef = ref<HTMLElement | null>(null)
const textareaRef = ref<HTMLTextAreaElement | null>(null)
const formRef = ref<HTMLFormElement | null>(null)
const userScrolledUp = ref(false)

const moreBtnRef = ref<HTMLButtonElement | null>(null)
const morePopoverRef = ref<HTMLElement | null>(null)
const moreFallbackOpen = ref(false)
const hasNativePopover = typeof HTMLElement !== 'undefined' && Object.hasOwn(HTMLElement.prototype, 'popover')

let lastCompositionEndAt: number | null = null
let abortCtrl: AbortController | null = null
let lastPrompt = ''

// Quote-Reply, @mention & /slash command state
const replyTarget = ref<{ serverId: number; senderName: string; preview: string } | null>(null)
const mentionOpen = ref(false)
const mentionQuery = ref('')
const mentionAnchorIdx = ref(-1)
const mentionHighlight = ref(0)

const slashOpen = ref(false)
const slashHighlight = ref(0)

const announcementDismissed = ref(false)
const relaySteps = ref<Array<{ id: string; name: string; status: 'done' | 'active'; guest?: boolean }>>([])

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
  return lastPrompt
}

function openForwardForMessage(msg: ChatMessageUI) {
  const fromBot = bot.value?.name || msg.senderName || 'BotHub'
  const senderLabel = msg.role === 'user' ? `${fromBot} (管理员提问)` : `${fromBot} · ${msg.senderName}`
  store.state.forwardingMessage = {
    fromName: senderLabel,
    content: msg.content,
  }
}

interface SlashCmdItem {
  cmd: string
  title: string
  desc: string
  prompt?: string
  action?: 'open_drawer' | 'clear_timeline' | 'new_thread' | 'summarize_thread' | 'open_skills'
}

const slashCommands = computed<SlashCmdItem[]>(() => {
  const b = bot.value
  const items: SlashCmdItem[] = []
  let allowedTools: string[] = []
  let boundSkills: string[] = []
  try {
    const parsed = JSON.parse(b?.allowed_tools || '[]')
    if (Array.isArray(parsed)) allowedTools = parsed
  } catch {
    allowedTools = []
  }
  try {
    const parsed = JSON.parse(b?.skill_ids || '[]')
    if (Array.isArray(parsed)) boundSkills = parsed
  } catch {
    boundSkills = []
  }

  // Dynamic bound custom skills first
  for (const sk of store.state.skills || []) {
    if (!sk.enabled || !boundSkills.includes(sk.id)) continue
    items.push({
      cmd: `/${sk.slug}`,
      title: `技能 · ${sk.name}`,
      desc: sk.description || (sk.kind === 'webhook' ? `调用外部技能 skill_${sk.slug}` : '按专属技能规范响应'),
      prompt: sk.kind === 'webhook' ? `请使用 skill_${sk.slug} 工具协助处理当前问题：` : `请遵循「${sk.name}」技能规范回答：`,
    })
  }

  if (allowedTools.includes('manage_assets')) {
    items.push({
      cmd: '/assets',
      title: '资产全面审计',
      desc: '盘点 VPS、域名与订阅到期风险',
      prompt: '请调用工具盘点当前全部数字资产与临期风险并给出续费建议。',
    })
  }
  if (allowedTools.includes('query_nezha_nodes')) {
    items.push({
      cmd: '/nezha',
      title: '探针节点巡检',
      desc: '查看哪吒节点在线率与负载情况',
      prompt: '请调用 query_nezha_nodes 检查哪吒监控面板节点的实时在线与健康状态。',
    })
  }
  if (allowedTools.includes('web_search')) {
    items.push({
      cmd: '/search',
      title: '全网检索',
      desc: '联网搜索最新资讯与公开资料',
      prompt: '请帮我联网搜索：',
    })
  }
  if (allowedTools.includes('manage_tasks')) {
    items.push({
      cmd: '/tasks',
      title: '调度队列核查',
      desc: '检查各自动化例程状态与下次触发时间',
      prompt: '请核查当前所有自动化调度任务的运行状态与上次执行摘要。',
    })
  }
  if (allowedTools.includes('manage_memory') || allowedTools.includes('recall_memory')) {
    items.push({
      cmd: '/memories',
      title: '回忆关键信息',
      desc: '检索与当前会话相关的历史重要记忆',
      prompt: '请先检索我们的历史记忆，梳理目前你记住的关键事实与约定。',
    })
  }

  items.push(
    { cmd: '/thread', title: '新建独立话题', desc: '在当前会话内开启一条干净的新话题线', action: 'new_thread' },
    { cmd: '/summarize', title: '总结话题并存记忆', desc: '提炼当前话题核心结论并沉淀到专属记忆库', action: 'summarize_thread' },
    { cmd: '/skills', title: '打开自定义技能工坊', desc: '管理 Playbook 与 Webhook 外部技能工具', action: 'open_skills' },
    { cmd: '/invite', title: '拉人进群 / 群设置', desc: '打开右侧群成员与协同策略抽屉', action: 'open_drawer' },
    { cmd: '/clear', title: '清空当前对话', desc: '清空当前话题聊天时间线', action: 'clear_timeline' }
  )
  return items
})

// Config Editor local state
const cfgName = ref('')
const cfgRole = ref('')
const cfgPrompt = ref('')
const cfgModel = ref('primary')
const cfgAvatar = ref('butler:gold')
const cfgThinking = ref(true)
const cfgBotMode = ref<BotMode>('hybrid')
const cfgMemoryPolicy = ref<MemoryPolicy>('standard')
const cfgTraits = ref('')
const cfgTools = ref<string[]>([])
const cfgSkillIds = ref<string[]>([])
const cfgMembers = ref<string[]>([])
const cfgGroupMode = ref('smart')
const savingProfile = ref(false)

// Inline Bot-Scoped Memory CRUD
const newMemKey = ref('')
const newMemContent = ref('')
const newMemCategory = ref('preference')
const newMemImportance = ref(3)
const newMemPinned = ref(false)
const savingMem = ref(false)
const editingMemId = ref<number | null>(null)
const editMemContent = ref('')

// Inline Topic Thread rename
const renamingThreadId = ref<string | null>(null)
const renameThreadTitle = ref('')

const BOT_MODE_TABS = [
  { id: 'companion', label: '陪伴聊天型' },
  { id: 'hybrid', label: '综合协同型' },
  { id: 'worker', label: '干活执行型' },
]

const MEMORY_POLICY_OPTIONS = [
  { value: 'isolated', label: '严格私有 (仅 Bot 自身可见)' },
  { value: 'standard', label: '私有 + 全局共享事实' },
]

function botModeLabel(mode?: string): string {
  if (mode === 'companion') return '陪伴'
  if (mode === 'worker') return '执行'
  return '全能'
}

const VIEW_TABS = [
  { id: 'stream', label: '对话流' },
  { id: 'routines', label: '例程' },
  { id: 'config', label: '能力与记忆' },
  { id: 'activity', label: '运行脉搏' },
]

const availableTools = computed(() => store.state.availableTools || [])
const availableSkills = computed(() => (store.state.skills || []).filter((s) => s.enabled !== false))

const botToolBadges = computed(() => {
  if (!bot.value) return []
  let allowed: string[] = []
  try {
    const parsed = JSON.parse(bot.value.allowed_tools || '[]')
    if (Array.isArray(parsed)) allowed = parsed
  } catch {
    allowed = []
  }
  return allowed.map((id) => {
    const found = availableTools.value.find((t) => t.id === id)
    return {
      id,
      desc: found?.description || id,
    }
  })
})

watch(
  bot,
  (b) => {
    if (!b) return
    const normModel = !b.model || b.model === 'deepseek-flash' ? 'primary' : b.model
    selectedModel.value = normModel
    enableThinking.value = b.enable_thinking !== false
    cfgName.value = b.name
    cfgRole.value = b.role_title
    cfgPrompt.value = b.system_prompt
    cfgModel.value = normModel
    cfgAvatar.value = b.avatar_style || 'cyan'
    cfgThinking.value = b.enable_thinking !== false
    cfgBotMode.value = (b.bot_mode as BotMode) || 'hybrid'
    cfgMemoryPolicy.value = (b.memory_policy as MemoryPolicy) || 'standard'
    cfgTraits.value = b.traits || ''
    cfgGroupMode.value = b.group_mode || 'smart'
    announcementDismissed.value = false
    replyTarget.value = null
    renamingThreadId.value = null
    editingMemId.value = null
    try {
      const arr = JSON.parse(b.allowed_tools || '[]')
      cfgTools.value = Array.isArray(arr) ? arr : []
    } catch {
      cfgTools.value = []
    }
    try {
      const sArr = JSON.parse(b.skill_ids || '[]')
      cfgSkillIds.value = Array.isArray(sArr) ? sArr : []
    } catch {
      cfgSkillIds.value = []
    }
    try {
      const mArr = JSON.parse(b.member_ids || '[]')
      cfgMembers.value = Array.isArray(mArr) ? mArr : []
    } catch {
      cfgMembers.value = []
    }
  },
  { immediate: true }
)

watch(
  () => store.state.pendingDraftPrompt,
  (draft) => {
    if (!draft) return
    inputPrompt.value = draft
    store.state.pendingDraftPrompt = ''
    nextTick(() => {
      autoResizeTextarea()
      textareaRef.value?.focus()
    })
  },
  { immediate: true }
)

const isGroupBot = computed(() => Boolean(bot.value?.is_group))

const availableSpecialistBots = computed(() =>
  store.state.bots.filter((b) => !b.is_group)
)

const groupMembers = computed(() => {
  if (!isGroupBot.value) return []
  const ids = cfgMembers.value
  const map = new Map(availableSpecialistBots.value.map((b) => [b.id, b]))
  const resolved = ids.map((id) => map.get(id)).filter((x): x is NonNullable<typeof x> => Boolean(x))
  return resolved.length > 0 ? resolved : availableSpecialistBots.value
})

const mentionCandidates = computed(() => {
  const q = mentionQuery.value.toLowerCase()
  const memberIds = new Set(groupMembers.value.map((m) => m.id))
  const sorted = [...availableSpecialistBots.value].sort((a, b) => {
    const aIn = isGroupBot.value ? (memberIds.has(a.id) ? 0 : 1) : a.id === bot.value?.id ? 0 : 1
    const bIn = isGroupBot.value ? (memberIds.has(b.id) ? 0 : 1) : b.id === bot.value?.id ? 0 : 1
    return aIn - bIn
  })
  return sorted
    .filter((sp) => !q || `${sp.name} ${sp.id} ${sp.role_title}`.toLowerCase().includes(q))
    .map((sp) => ({
      bot: sp,
      inRoom: isGroupBot.value ? memberIds.has(sp.id) : sp.id === bot.value?.id,
    }))
})

const summarizingThread = ref(false)

async function handleSummarizeThread() {
  if (summarizingThread.value || !bot.value) return
  summarizingThread.value = true
  try {
    await store.summarizeThread(store.state.activeThreadId || 'main', true)
  } finally {
    summarizingThread.value = false
  }
}

function formatToolLabel(name: string): string {
  if (name.startsWith('skill_')) {
    const slug = name.slice(6)
    const found = (store.state.skills || []).find((s) => s.slug === slug)
    return found ? `${found.name} (${name})` : name
  }
  const builtinNames: Record<string, string> = {
    read_url: '网页正文抓取 (read_url)',
    web_search: '联网检索 (web_search)',
    query_nezha_nodes: '哪吒探针巡检 (query_nezha_nodes)',
    manage_assets: '数字资产台账 (manage_assets)',
    manage_tasks: '调度任务管理 (manage_tasks)',
    remember_fact: '写入长期记忆 (remember_fact)',
    recall_memory: '检索长期记忆 (recall_memory)',
    manage_memory: '整理与遗忘记忆 (manage_memory)',
  }
  return builtinNames[name] || name
}

function formatToolPayload(raw?: string): string {
  if (!raw) return ''
  const trimmed = raw.trim()
  if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2)
    } catch {
      return trimmed
    }
  }
  return trimmed
}

const filteredSlashCommands = computed(() => {
  const val = inputPrompt.value.trim().toLowerCase()
  if (!val.startsWith('/')) return []
  return slashCommands.value.filter(
    (c) => c.cmd.toLowerCase().includes(val) || c.title.toLowerCase().includes(val.slice(1))
  )
})

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
  nextTick(() => textareaRef.value?.focus())
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
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message || '保存记忆失败')
  }
}

function toggleCfgMember(id: string) {
  if (cfgMembers.value.includes(id)) {
    cfgMembers.value = cfgMembers.value.filter((x) => x !== id)
  } else {
    cfgMembers.value = [...cfgMembers.value, id]
  }
}

const botRoutines = computed(() => {
  const bid = bot.value?.id
  if (!bid) return []
  return store.state.tasks.filter((t) => t.bot_id === bid)
})

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

async function addBotMemory() {
  if (!bot.value || !newMemKey.value.trim() || !newMemContent.value.trim() || savingMem.value) return
  savingMem.value = true
  try {
    await BotHubApi.saveMemory({
      scope: `bot:${bot.value.id}`,
      category: newMemCategory.value,
      fact_key: newMemKey.value.trim(),
      content: newMemContent.value.trim(),
      importance: newMemImportance.value,
      pinned: newMemPinned.value,
    })
    newMemKey.value = ''
    newMemContent.value = ''
    newMemPinned.value = false
    store.notify(`已写入「${bot.value.name}」专属记忆`)
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '写入记忆失败')
  } finally {
    savingMem.value = false
  }
}

async function saveInlineMemEdit(mem: AgentMemory) {
  if (!editMemContent.value.trim()) return
  try {
    await BotHubApi.updateMemory(mem.id, { content: editMemContent.value.trim() })
    editingMemId.value = null
    store.notify('记忆条目已更新')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '更新失败')
  }
}

async function toggleBotMemoryPin(mem: AgentMemory) {
  try {
    await BotHubApi.updateMemory(mem.id, { pinned: !mem.pinned })
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '操作失败')
  }
}

async function deleteBotMemory(id: number) {
  try {
    await BotHubApi.deleteMemory(id)
    store.notify('记忆条目已移除')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '移除失败')
  }
}

function openScopedMemoriesManager() {
  if (!bot.value) return
  store.state.memoryModalDefaultScope = `bot:${bot.value.id}`
  store.state.showMemoriesModal = true
}

const activityEntries = computed(() => {
  const cur = bot.value
  if (!cur) return []
  const items: Array<{ title: string; time: string; detail: string; status?: string }> = []

  // 1. Real per-Bot execution audit logs (tools, custom skills, chat/routine completions)
  ;(store.state.botExecutions || []).forEach((ex) => {
    const kindLabel =
      ex.exec_type === 'tool'
        ? `工具调用 · ${ex.name}`
        : ex.exec_type === 'skill'
        ? `技能调用 · ${ex.name}`
        : ex.exec_type === 'routine'
        ? '例程任务'
        : `对话响应 · ${ex.name}`
    items.push({
      title: kindLabel,
      time: `${formatTime(ex.created_at)} · ${ex.duration_ms}ms`,
      detail: ex.output_summary || ex.input_summary || ex.name,
      status: ex.status,
    })
  })

  // 2. Tasks explicitly bound to THIS Bot
  const boundTaskIDs = new Set<string>()
  botRoutines.value.forEach((t) => {
    boundTaskIDs.add(t.id)
    if (t.last_run) {
      items.push({
        title: `绑定例程 · ${t.name}`,
        time: `${formatTime(t.last_run.started_at)} · ${t.last_run.duration_ms}ms`,
        detail: t.last_run.summary,
        status: t.last_run.status,
      })
    }
  })

  // 3. Notifications ONLY if emitted by this Bot or one of its bound tasks (strict isolation!)
  store.state.notifications
    .filter((n) => boundTaskIDs.has(n.source) || n.source === `bot:${cur.id}` || n.source === cur.id)
    .slice(0, 12)
    .forEach((n) => {
      items.push({
        title: `专属推送 · ${n.title}`,
        time: formatTime(n.created_at),
        detail: n.summary,
      })
    })
  return items
})

function onMessagesScroll() {
  const el = scrollContainerRef.value
  if (!el) return
  const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
  userScrolledUp.value = distanceFromBottom > 80
}

function scrollToBottom(force = false) {
  if (force) userScrolledUp.value = false
  if (!force && userScrolledUp.value) return
  nextTick(() => {
    if (scrollContainerRef.value) {
      scrollContainerRef.value.scrollTop = scrollContainerRef.value.scrollHeight
    }
  })
}

const supportsFieldSizing =
  typeof window !== 'undefined' && Boolean(window.CSS?.supports?.('field-sizing', 'content'))

function autoResizeTextarea() {
  if (supportsFieldSizing) return
  const el = textareaRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 160)}px`
}

function handleCompositionEnd(e: CompositionEvent) {
  lastCompositionEndAt = e.timeStamp
}

function syncMentionAndSlash() {
  const el = textareaRef.value
  const val = inputPrompt.value
  const pos = el?.selectionStart ?? val.length

  if (val.startsWith('/') && !val.includes('\n') && !val.includes(' ')) {
    slashOpen.value = filteredSlashCommands.value.length > 0
    slashHighlight.value = 0
  } else {
    slashOpen.value = false
  }

  const beforeCaret = val.slice(0, pos)
  const match = beforeCaret.match(/(?:^|\s)@([^\s@]*)$/)
  if (match) {
    mentionQuery.value = match[1] || ''
    mentionAnchorIdx.value = beforeCaret.lastIndexOf('@')
    mentionHighlight.value = 0
    mentionOpen.value = mentionCandidates.value.length > 0
  } else {
    mentionOpen.value = false
  }
}

function handleComposerInput() {
  autoResizeTextarea()
  syncMentionAndSlash()
}

function pickMentionCandidate(sp: Bot) {
  const val = inputPrompt.value
  const pos = textareaRef.value?.selectionStart ?? val.length
  const anchor = mentionAnchorIdx.value >= 0 ? mentionAnchorIdx.value : val.lastIndexOf('@', pos)
  if (anchor >= 0) {
    const before = val.slice(0, anchor)
    const after = val.slice(pos)
    inputPrompt.value = `${before}@${sp.name} ${after}`
  } else {
    appendMention(`@${sp.name} `)
  }
  mentionOpen.value = false
  nextTick(() => {
    autoResizeTextarea()
    textareaRef.value?.focus()
  })
}

async function inviteFromMentionAndPick(sp: Bot) {
  if (isGroupBot.value && bot.value) {
    try {
      await BotHubApi.updateGroupMembers(bot.value.id, 'add', [sp.id])
      await store.loadDashboard(false)
      store.notify(`已将「${sp.name}」正式拉入当前群聊`)
    } catch {
      // fallback to temp mention
    }
  }
  pickMentionCandidate(sp)
}

function pickSlashCommand(item: SlashCmdItem) {
  slashOpen.value = false
  if (item.action === 'open_drawer') {
    inputPrompt.value = ''
    store.state.showGroupDrawer = true
    return
  }
  if (item.action === 'clear_timeline') {
    inputPrompt.value = ''
    clearTimeline()
    return
  }
  if (item.action === 'new_thread') {
    inputPrompt.value = ''
    store.createThread('')
    return
  }
  if (item.action === 'summarize_thread') {
    inputPrompt.value = ''
    handleSummarizeThread()
    return
  }
  if (item.action === 'open_skills') {
    inputPrompt.value = ''
    store.state.showSkillsModal = true
    return
  }
  if (item.prompt) {
    if (item.prompt.endsWith('：')) {
      inputPrompt.value = item.prompt
      nextTick(() => {
        autoResizeTextarea()
        textareaRef.value?.focus()
      })
    } else {
      inputPrompt.value = ''
      sendPrompt(item.prompt)
    }
  }
}

function handleComposerKeydown(e: KeyboardEvent) {
  if (mentionOpen.value && mentionCandidates.value.length > 0) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      mentionHighlight.value = (mentionHighlight.value + 1) % mentionCandidates.value.length
      return
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      mentionHighlight.value = (mentionHighlight.value - 1 + mentionCandidates.value.length) % mentionCandidates.value.length
      return
    }
    if ((e.key === 'Enter' || e.key === 'Tab') && !isImeComposingEnter(e, lastCompositionEndAt)) {
      e.preventDefault()
      const picked = mentionCandidates.value[mentionHighlight.value]
      if (picked) pickMentionCandidate(picked.bot)
      return
    }
    if (e.key === 'Escape') {
      e.preventDefault()
      mentionOpen.value = false
      return
    }
  }

  if (slashOpen.value && filteredSlashCommands.value.length > 0) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      slashHighlight.value = (slashHighlight.value + 1) % filteredSlashCommands.value.length
      return
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      slashHighlight.value = (slashHighlight.value - 1 + filteredSlashCommands.value.length) % filteredSlashCommands.value.length
      return
    }
    if ((e.key === 'Enter' || e.key === 'Tab') && !isImeComposingEnter(e, lastCompositionEndAt)) {
      e.preventDefault()
      const picked = filteredSlashCommands.value[slashHighlight.value]
      if (picked) pickSlashCommand(picked)
      return
    }
    if (e.key === 'Escape') {
      e.preventDefault()
      slashOpen.value = false
      return
    }
  }

  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    if (isImeComposingEnter(e, lastCompositionEndAt)) return
    formRef.value?.requestSubmit()
  }
}

watch(inputPrompt, (val) => {
  if (store.state.activeBotId) {
    store.setBotDraft(store.state.activeBotId, val, store.state.activeThreadId)
  }
  if (!val) {
    mentionOpen.value = false
    slashOpen.value = false
    if (!supportsFieldSizing) {
      nextTick(() => {
        if (textareaRef.value) textareaRef.value.style.height = ''
      })
    }
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
    scrollToBottom(true)
    nextTick(() => {
      autoResizeTextarea()
      applySiblingStaggerFallback(starterGridRef.value)
    })
  }
)

watch(
  () => store.state.timeline.length,
  (len) => {
    scrollToBottom(false)
    if (len === 0) nextTick(() => applySiblingStaggerFallback(starterGridRef.value))
  }
)

watch(
  () => store.state.botView,
  (v) => {
    nextTick(() => {
      if (v === 'routines') applySiblingStaggerFallback(routinesGridRef.value)
      if (v === 'activity') applySiblingStaggerFallback(activityListRef.value)
    })
  }
)

function positionFallbackPopover() {
  const btn = moreBtnRef.value
  const pop = morePopoverRef.value
  if (!btn || !pop) return
  if (window.CSS?.supports?.('position-area', 'block-end')) return
  const rect = btn.getBoundingClientRect()
  const panelWidth = Math.max(pop.offsetWidth, 184)
  const left = Math.max(8, Math.min(rect.right - panelWidth, window.innerWidth - panelWidth - 8))
  const spaceBelow = window.innerHeight - rect.bottom
  const top = spaceBelow < 180 ? Math.max(8, rect.top - 160) : rect.bottom + 8
  pop.style.top = `${top}px`
  pop.style.left = `${left}px`
}

function onMoreBeforeToggle(e: Event) {
  const toggleEvt = e as ToggleEvent
  if (toggleEvt.newState === 'open') {
    nextTick(() => positionFallbackPopover())
  }
}

function handleMoreButtonClick() {
  if (!hasNativePopover) {
    moreFallbackOpen.value = !moreFallbackOpen.value
  }
}

function closeMoreMenu() {
  if (hasNativePopover && morePopoverRef.value?.matches(':popover-open')) {
    morePopoverRef.value.hidePopover()
  }
  moreFallbackOpen.value = false
}

function handleOutsideFallback(e: MouseEvent) {
  if (hasNativePopover || !moreFallbackOpen.value) return
  const t = e.target as Node
  if (moreBtnRef.value?.contains(t) || morePopoverRef.value?.contains(t)) return
  moreFallbackOpen.value = false
}

function handleWorkspaceKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'f') {
    if (store.state.workspace === 'bots' && store.state.botView === 'stream') {
      e.preventDefault()
      openChatSearch()
    }
  } else if (e.key === 'Escape' && chatSearchOpen.value) {
    closeChatSearch()
  }
}

onMounted(() => {
  document.addEventListener('click', handleOutsideFallback)
  window.addEventListener('keydown', handleWorkspaceKeydown)
  const savedDraft = store.getBotDraft(store.state.activeBotId)
  if (savedDraft && !inputPrompt.value) {
    inputPrompt.value = savedDraft
    nextTick(() => autoResizeTextarea())
  }
  applySiblingStaggerFallback(starterGridRef.value)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', handleOutsideFallback)
  window.removeEventListener('keydown', handleWorkspaceKeydown)
})

function appendMention(tag: string) {
  const prefix = inputPrompt.value && !inputPrompt.value.endsWith(' ') ? ' ' : ''
  inputPrompt.value = `${inputPrompt.value}${prefix}${tag}`
  nextTick(() => {
    autoResizeTextarea()
    textareaRef.value?.focus()
  })
}

function handleMarkdownClick(e: MouseEvent) {
  const target = e.target as HTMLElement
  const btn = target.closest('.md-copy-code-btn') as HTMLButtonElement | null
  if (!btn) return
  const code = btn.closest('.md-code-wrap')?.querySelector('pre code')?.textContent || ''
  navigator.clipboard?.writeText(code).then(() => {
    btn.textContent = '已复制'
    btn.classList.add('text-emerald-300', 'border-emerald-400/45')
    window.setTimeout(() => {
      btn.textContent = '复制代码'
      btn.classList.remove('text-emerald-300', 'border-emerald-400/45')
    }, 1500)
  })
}

async function copyMessageContent(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    store.notify('已复制回复全文')
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
  lastPrompt = msg
  if (!overrideText) inputPrompt.value = ''
  mentionOpen.value = false
  slashOpen.value = false
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
    }
    store.state.timeline.push(turn)
    return store.state.timeline[store.state.timeline.length - 1]
  }

  let activeTurn = createTurn(botId, curBot?.name || 'BotHub', curBot?.avatar_style || 'cyan', curBot?.role_title || '')
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
            activeTurn.statusText =
              payload.usage && payload.usage.total_tokens
                ? `${payload.usage.prompt_tokens}+${payload.usage.completion_tokens} Tokens`
                : '已完成'
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
    await store.loadDashboard(false)
    // Reload synced server IDs, threads, memories and execution logs quietly
    if (curBot?.id === store.state.activeBotId) {
      const synced = await BotHubApi.getBotMessages(curBot.id, store.state.activeThreadId).catch(() => null)
      if (synced?.threads) store.state.botThreads = synced.threads
      if (synced?.memories) store.state.botMemories = synced.memories
      if (synced?.executions) store.state.botExecutions = synced.executions
      if (synced?.messages && !store.state.isStreaming) {
        const mappedSynced = mapServerMessages(synced.messages, curBot)
        if (mappedSynced.length >= store.state.timeline.length) {
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
  const parts = [`# Vesper GrokBot · ${bName} 对话流导出\n\n- 导出时间: ${new Date().toISOString()}\n`]
  for (const m of store.state.timeline) {
    parts.push(`\n---\n\n### ${m.role === 'user' ? '管理员指令' : `${m.senderName} (${m.model})`}\n`)
    if (m.reasoning) {
      parts.push(`\n> 深度思考:\n> ${m.reasoning.replace(/\n/g, '\n> ')}\n`)
    }
    if (m.content) parts.push(`\n${m.content}\n`)
  }
  const blob = new Blob([parts.join('')], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `bothub_${bot.value?.id || 'bot'}_${Date.now()}.md`
  a.click()
  URL.revokeObjectURL(url)
  store.notify('已导出为 Markdown')
}

async function clearTimeline() {
  if (store.state.isStreaming || !bot.value) return
  if (!window.confirm(`确定清空 [${bot.value.name}] 的对话时间线吗？`)) return
  await store.clearActiveBotTimeline()
}

async function runRoutine(taskId: string) {
  try {
    const res = await BotHubApi.runTask(taskId)
    store.notify(res.summary || '执行完成')
    await store.loadDashboard(false)
    await store.selectBot(store.state.activeBotId)
  } catch (err: any) {
    store.notify(err.message)
  }
}

async function saveBotProfile() {
  if (!bot.value || savingProfile.value) return
  savingProfile.value = true
  try {
    await BotHubApi.updateBot(bot.value.id, {
      name: cfgName.value.trim(),
      role_title: cfgRole.value.trim(),
      avatar_style: cfgAvatar.value,
      model: cfgModel.value,
      bot_mode: cfgBotMode.value,
      memory_policy: cfgMemoryPolicy.value,
      traits: cfgTraits.value.trim(),
      enable_thinking: cfgThinking.value,
      system_prompt: cfgPrompt.value.trim(),
      allowed_tools: JSON.stringify(cfgTools.value),
      skill_ids: JSON.stringify(cfgSkillIds.value),
      member_ids: JSON.stringify(cfgMembers.value),
      group_mode: cfgGroupMode.value,
      is_pinned: bot.value.is_pinned,
    })
    store.notify('Bot 人设、记忆策略与技能权限已更新')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    savingProfile.value = false
  }
}

function startRenameThread(id: string, curTitle: string) {
  renamingThreadId.value = id
  renameThreadTitle.value = curTitle
}

async function commitRenameThread() {
  if (!renamingThreadId.value) return
  const title = renameThreadTitle.value.trim()
  if (title) {
    await store.renameThread(renamingThreadId.value, title)
  }
  renamingThreadId.value = null
}

async function deleteCustomBot() {
  if (!bot.value) return
  if (!window.confirm(`确定删除 [${bot.value.name}] 吗？其专属聊天流将一并移除。`)) return
  await BotHubApi.deleteBot(bot.value.id)
  store.notify(`已删除「${bot.value.name}」`)
  await store.loadDashboard(false)
  if (store.state.bots.length > 0) {
    await store.selectBot(store.state.bots[0].id)
  }
}

function stateBadgeClass(state?: string): string {
  if (state === 'thinking') return 'border-amber-400/40 bg-amber-400/10 text-amber-300'
  if (state === 'working') return 'border-[#f2b880]/45 bg-[#f2b880]/12 text-[#f2b880]'
  if (state === 'done') return 'border-emerald-400/40 bg-emerald-400/10 text-emerald-300'
  if (state === 'blocked') return 'border-rose-400/45 bg-rose-400/10 text-rose-300'
  return 'border-[#fff8ee]/15 text-slate-400'
}
</script>

<template>
  <section class="flex-1 min-h-0 flex flex-col">
    <!-- Minimal Bot Top Header -->
    <header class="px-3.5 sm:px-6 pt-[max(0.75rem,env(safe-area-inset-top,0.75rem))] pb-3 border-b border-[#fff8ee]/8 bg-[#080c18]/50 backdrop-blur-md flex flex-wrap items-center justify-between gap-2.5">
      <div class="flex items-center gap-2.5 min-w-0">
        <button
          type="button"
          aria-label="打开菜单"
          class="md:hidden min-h-[44px] min-w-[44px] inline-flex items-center justify-center rounded-xl border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
          @click="emit('openMobileMenu')"
        >
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <line x1="3" y1="6" x2="21" y2="6" />
            <line x1="3" y1="12" x2="21" y2="12" />
            <line x1="3" y1="18" x2="21" y2="18" />
          </svg>
        </button>
        <button
          v-if="desktopCollapsed"
          type="button"
          aria-label="展开侧边栏"
          title="展开侧边栏"
          class="hidden md:inline-flex items-center justify-center w-9 h-9 rounded-xl border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
          @click="emit('toggleDesktopSidebar')"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <rect x="3" y="3" width="18" height="18" rx="2" />
            <line x1="9" y1="3" x2="9" y2="21" />
            <polyline points="13 9 16 12 13 15" />
          </svg>
        </button>
        <BotGroupAvatar
          v-if="isGroupBot"
          :members="groupMembers"
          :fallback-style="bot?.avatar_style"
          :state="bot?.state"
          size="md"
        />
        <BotAvatar v-else :avatar-style="bot?.avatar_style" :state="bot?.state" size="md" />
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <h1 class="text-sm md:text-base font-semibold text-white truncate">{{ bot?.name || 'BotHub 工作台' }}</h1>
            <span
              v-if="isGroupBot"
              class="hidden sm:inline-flex font-mono text-[10px] px-2 py-0.5 rounded-full border border-[#fff8ee]/14 bg-[#fff8ee]/[0.04] text-slate-300 leading-none shrink-0"
            >
              {{ groupModeLabel(bot?.group_mode) }}
            </span>
            <span
              v-else-if="bot"
              :class="[
                'hidden sm:inline-flex font-mono text-[10px] px-2 py-0.5 rounded-full border leading-none shrink-0',
                bot.bot_mode === 'companion'
                  ? 'border-emerald-400/35 bg-emerald-500/10 text-emerald-200'
                  : bot.bot_mode === 'worker'
                  ? 'border-cyan-400/35 bg-cyan-500/10 text-cyan-200'
                  : 'border-[#fff8ee]/14 bg-[#fff8ee]/[0.04] text-slate-300',
              ]"
            >
              {{ botModeLabel(bot.bot_mode) }}
            </span>
            <span
              v-if="bot?.state && bot.state !== 'idle'"
              :class="['font-mono text-[10px] px-2 py-0.5 rounded-full border leading-none shrink-0', stateBadgeClass(bot?.state)]"
            >
              {{ botStateLabel(bot?.state) }}
            </span>
          </div>
          <p class="hidden sm:block text-xs text-slate-400/90 truncate">{{ bot?.role_title }}</p>
        </div>
      </div>

      <div v-if="bot" class="flex items-center gap-2 max-w-full ml-auto">
        <!-- Gliding Physics Segmented Control -->
        <SegmentedTabs v-model="store.state.botView" :items="VIEW_TABS" aria-label="Bot 子视图" />

        <!-- QQ/WeChat Member / Invite Pill -->
        <button
          type="button"
          @click="store.state.showGroupDrawer = true"
          :title="isGroupBot ? '查看群成员与拉人管理' : '临时拉人发起协同群聊'"
          class="min-h-[44px] sm:min-h-[36px] inline-flex items-center gap-1.5 px-3 rounded-full border border-[#f2b880]/30 bg-[#f2b880]/10 hover:bg-[#f2b880]/18 hover:border-[#f2b880]/55 text-xs font-medium text-[#f7cba0] cursor-pointer shrink-0"
        >
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
            <circle cx="8.5" cy="7" r="4" />
            <line x1="20" y1="8" x2="20" y2="14" />
            <line x1="23" y1="11" x2="17" y2="11" />
          </svg>
          <span class="hidden sm:inline">{{ isGroupBot ? `群成员 (${groupMembers.length})` : '拉人建群' }}</span>
        </button>

        <!-- Native Top-Layer Popover + Anchor Positioned More Actions -->
        <div class="relative shrink-0">
          <button
            ref="moreBtnRef"
            type="button"
            :popovertarget="hasNativePopover ? moreMenuId : undefined"
            aria-label="更多操作"
            title="更多操作"
            @click="handleMoreButtonClick"
            class="min-h-[44px] min-w-[44px] sm:min-h-[36px] sm:min-w-[36px] inline-flex items-center justify-center rounded-full border border-[#fff8ee]/10 bg-[#060913]/85 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor" aria-hidden="true">
              <circle cx="5" cy="12" r="1.75" />
              <circle cx="12" cy="12" r="1.75" />
              <circle cx="19" cy="12" r="1.75" />
            </svg>
          </button>

          <div
            ref="morePopoverRef"
            :id="moreMenuId"
            :popover="hasNativePopover ? 'auto' : undefined"
            @beforetoggle="onMoreBeforeToggle"
            :class="[
              'vesper-popover anchored-end',
              !hasNativePopover ? (moreFallbackOpen ? 'fixed right-4 top-16 flex flex-col gap-0.5 z-50' : 'hidden') : ''
            ]"
          >
            <button
              type="button"
              @click="openChatSearch(); closeMoreMenu()"
              class="min-h-[44px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
            >
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" /></svg>
              <span>查找聊天记录</span>
            </button>
            <button
              type="button"
              @click="store.toggleSound(); closeMoreMenu()"
              class="min-h-[44px] sm:min-h-[34px] w-full flex items-center justify-between gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
            >
              <span class="inline-flex items-center gap-2.5">
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" /><path d="M15.54 8.46a5 5 0 0 1 0 7.07" /></svg>
                <span>回复提示音</span>
              </span>
              <span class="font-mono text-[10px]" :class="store.state.soundEnabled ? 'text-[#f2b880]' : 'text-slate-500'">
                {{ store.state.soundEnabled ? '开' : '关' }}
              </span>
            </button>
            <button
              type="button"
              @click="store.loadDashboard(false); closeMoreMenu()"
              class="min-h-[44px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
            >
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><polyline points="23 4 23 10 17 10" /><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" /></svg>
              <span>刷新状态</span>
            </button>
            <button
              type="button"
              @click="exportTimelineMarkdown(); closeMoreMenu()"
              class="min-h-[44px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
            >
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="7 10 12 15 17 10" /><line x1="12" y1="15" x2="12" y2="3" /></svg>
              <span>导出 Markdown</span>
            </button>
            <div class="my-1 border-t border-[#fff8ee]/10" />
            <button
              type="button"
              @click="clearTimeline(); closeMoreMenu()"
              class="min-h-[44px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-rose-300 hover:bg-rose-500/15 cursor-pointer text-left"
            >
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /></svg>
              <span>清空对话流</span>
            </button>
          </div>
        </div>
      </div>
    </header>

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
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0 translate-y-1.5"
      enter-to-class="opacity-100 translate-y-0"
    >
      <div v-show="store.state.botView === 'stream'" class="flex-1 min-h-0 flex flex-col">
        <!-- Per-Bot / Per-Group Scoped Topic Threads Bar -->
        <div
          v-if="bot"
          class="px-4 md:px-6 py-1.5 border-b border-[#fff8ee]/8 bg-[#050811]/75 flex items-center gap-1.5 overflow-x-auto shrink-0 no-scrollbar"
        >
          <span class="text-[10.5px] font-mono text-slate-500 shrink-0 mr-1">话题</span>
          <button
            type="button"
            @click="store.switchThread('')"
            :class="[
              'px-2.5 py-1 rounded-lg text-[11px] whitespace-nowrap cursor-pointer transition-colors border',
              !store.state.activeThreadId
                ? 'border-[#f2b880]/55 bg-[#f2b880]/15 text-[#f7cba0] font-medium'
                : 'border-[#fff8ee]/10 bg-transparent text-slate-400 hover:text-slate-200',
            ]"
          >
            主时间线
          </button>

          <div
            v-for="th in store.state.botThreads"
            :key="th.id"
            class="inline-flex items-center gap-1 shrink-0"
          >
            <input
              v-if="renamingThreadId === th.id"
              v-model="renameThreadTitle"
              type="text"
              @keydown.enter.prevent="commitRenameThread"
              @keydown.escape="renamingThreadId = null"
              @blur="commitRenameThread"
              class="w-32 px-2 py-0.5 rounded-lg border border-[#f2b880]/60 bg-[#070a16] text-[11px] text-white"
            />
            <div
              v-else
              :class="[
                'inline-flex items-center gap-1 rounded-lg border transition-colors',
                store.state.activeThreadId === th.id
                  ? 'border-[#f2b880]/55 bg-[#f2b880]/15 text-[#f7cba0]'
                  : 'border-[#fff8ee]/10 bg-transparent text-slate-400 hover:text-slate-200',
              ]"
            >
              <button
                type="button"
                @click="store.switchThread(th.id)"
                @dblclick="startRenameThread(th.id, th.title)"
                class="px-2.5 py-1 text-[11px] whitespace-nowrap cursor-pointer font-medium"
              >
                {{ th.title }}
                <span v-if="th.msg_count > 0" class="font-mono text-[10px] opacity-65 ml-0.5">
                  ({{ th.msg_count }})
                </span>
              </button>
              <button
                v-if="store.state.activeThreadId === th.id"
                type="button"
                title="重命名话题"
                @click.stop="startRenameThread(th.id, th.title)"
                class="pr-1 text-[10px] text-slate-400 hover:text-[#f2b880] cursor-pointer"
              >
                改
              </button>
              <button
                v-if="store.state.activeThreadId === th.id"
                type="button"
                title="删除此话题"
                @click.stop="store.deleteThread(th.id)"
                class="pr-2 text-[10px] text-slate-400 hover:text-rose-300 cursor-pointer"
              >
                ×
              </button>
            </div>
          </div>

          <div class="ml-auto flex items-center gap-1.5 shrink-0">
            <button
              v-if="store.state.timeline.length >= 2"
              type="button"
              :disabled="summarizingThread"
              @click="handleSummarizeThread"
              title="提炼当前话题核心结论并存为专属记忆"
              class="px-2.5 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:border-emerald-400/45 hover:text-emerald-300 cursor-pointer whitespace-nowrap disabled:opacity-50"
            >
              {{ summarizingThread ? '提炼中...' : '总结存记忆' }}
            </button>
            <button
              type="button"
              @click="store.createThread('')"
              class="px-2.5 py-1 rounded-lg border border-dashed border-[#fff8ee]/16 text-[11px] text-slate-300 hover:border-[#f2b880]/50 hover:text-[#f2b880] cursor-pointer whitespace-nowrap"
            >
              + 新话题
            </button>
          </div>
        </div>

        <!-- Slide-Down In-Chat Search Bar (Cmd/Ctrl + F) -->
        <Transition
          enter-active-class="transition duration-160 ease-out"
          enter-from-class="opacity-0 -translate-y-2"
          enter-to-class="opacity-100 translate-y-0"
        >
          <div
            v-if="chatSearchOpen"
            class="px-4 md:px-6 py-2 border-b border-[#fff8ee]/10 bg-[#070b17]/95 backdrop-blur-lg flex items-center gap-2 shrink-0"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="text-[#f2b880] shrink-0">
              <circle cx="11" cy="11" r="8" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
            <input
              ref="chatSearchInputRef"
              v-model="chatSearchQuery"
              type="search"
              placeholder="在当前会话中搜索聊天记录..."
              @keydown.enter.exact.prevent="jumpSearchMatch(1)"
              @keydown.shift.enter.prevent="jumpSearchMatch(-1)"
              @keydown.escape="closeChatSearch"
              class="flex-1 min-w-0 bg-transparent text-xs text-white placeholder-slate-500 py-1"
            />
            <span v-if="chatSearchQuery.trim()" class="font-mono text-[11px] text-slate-400 shrink-0">
              {{ chatSearchMatches.length ? `${chatSearchCursor + 1}/${chatSearchMatches.length}` : '0/0' }}
            </span>
            <button
              type="button"
              :disabled="!chatSearchMatches.length"
              @click="jumpSearchMatch(-1)"
              aria-label="上一条匹配"
              class="w-7 h-7 rounded-lg border border-[#fff8ee]/10 text-slate-300 hover:text-[#f2b880] disabled:opacity-30 inline-flex items-center justify-center cursor-pointer"
            >
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.2"><polyline points="18 15 12 9 6 15" /></svg>
            </button>
            <button
              type="button"
              :disabled="!chatSearchMatches.length"
              @click="jumpSearchMatch(1)"
              aria-label="下一条匹配"
              class="w-7 h-7 rounded-lg border border-[#fff8ee]/10 text-slate-300 hover:text-[#f2b880] disabled:opacity-30 inline-flex items-center justify-center cursor-pointer"
            >
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.2"><polyline points="6 9 12 15 18 9" /></svg>
            </button>
            <button
              type="button"
              @click="closeChatSearch"
              aria-label="关闭查找"
              class="w-7 h-7 rounded-lg text-slate-400 hover:text-white inline-flex items-center justify-center cursor-pointer"
            >
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.2"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
            </button>
          </div>
        </Transition>

        <!-- Pinned Group Announcement Banner (QQ/WeChat style) -->
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

        <div class="relative flex-1 min-h-0 flex flex-col">
          <div
            ref="scrollContainerRef"
            @scroll="onMessagesScroll"
            @click="handleMarkdownClick"
            class="cq-container stream-scroller flex-1 min-h-0 overflow-y-auto px-4 md:px-6 py-5 flex flex-col gap-5"
          >
            <!-- Empty State Hero + Living Companion Avatar + Spotlight Bento Starter Cards -->
            <div
              v-if="store.state.timeline.length === 0"
              class="m-auto max-w-[780px] w-full text-center py-6 px-2 flex flex-col items-center gap-3.5"
            >
              <div class="relative inline-flex items-center justify-center mb-1">
                <BotGroupAvatar
                  v-if="isGroupBot"
                  :members="groupMembers"
                  :fallback-style="bot?.avatar_style"
                  :state="bot?.state || 'idle'"
                  size="xl"
                />
                <BotAvatar
                  v-else
                  :avatar-style="bot?.avatar_style"
                  :state="bot?.state || 'idle'"
                  size="xl"
                />
              </div>
              <div class="flex flex-col items-center gap-1.5">
                <h2 class="text-xl sm:text-2xl font-semibold text-white tracking-tight">{{ bot?.name }}</h2>
                <p class="text-xs sm:text-sm text-slate-400 max-w-md">
                  {{ bot?.role_title }} · 输入消息开始对话，键入 <code class="font-mono text-[#f2b880]">@</code> 协同成员或 <code class="font-mono text-[#f2b880]">/</code> 触发指令
                </p>
              </div>

              <div
                v-if="botToolBadges.length > 0"
                ref="starterGridRef"
                class="stagger-list mt-2 flex flex-wrap items-center justify-center gap-2 max-w-2xl"
              >
                <span
                  v-for="tool in botToolBadges"
                  :key="tool.id"
                  :title="tool.desc"
                  class="stagger-item inline-flex items-center gap-1.5 px-3 py-1 rounded-full border border-[#fff8ee]/10 bg-[#070b18]/80 font-mono text-[11px] text-slate-300"
                >
                  <span class="w-1.5 h-1.5 rounded-full bg-[#f2b880]/75" aria-hidden="true" />
                  <span>{{ tool.id }}</span>
                </span>
              </div>
            </div>

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
                class="msg-enter max-w-[680px] mx-auto flex justify-center my-0.5"
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
                  'msg-enter reveal-host max-w-[820px] w-full mx-auto flex flex-col items-end gap-1 rounded-2xl transition-shadow',
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
                  {{ m.content }}
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
                  'msg-enter reveal-host max-w-[820px] w-full mx-auto rounded-2xl transition-shadow',
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
                    </summary>
                    <div class="mt-2 font-mono whitespace-pre-wrap leading-relaxed text-slate-300/90">
                      {{ m.reasoning }}
                    </div>
                  </details>

                  <!-- Smooth Collapsible Merged Tool Call Trajectory -->
                  <details
                    v-if="m.tools.length"
                    class="vesper-details rounded-xl border border-[#fff8ee]/10 bg-[#070a16]/75 px-3 py-2 font-mono text-[11.5px]"
                  >
                    <summary class="cursor-pointer flex items-center justify-between gap-2 text-slate-300 select-none">
                      <span class="inline-flex items-center gap-1.5 min-w-0">
                        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="details-chevron text-slate-400 shrink-0">
                          <polyline points="9 18 15 12 9 6" />
                        </svg>
                        <span class="truncate">工具执行轨迹 · 共 {{ m.tools.length }} 步</span>
                      </span>
                      <span :class="m.tools.some((t) => !t.done) ? 'text-[#f2b880] shrink-0' : 'text-emerald-400 shrink-0'">
                        {{ m.tools.some((t) => !t.done) ? '执行中' : '全部完成' }}
                      </span>
                    </summary>
                    <div class="mt-2 pt-2 border-t border-[#fff8ee]/8 flex flex-col gap-2">
                      <div
                        v-for="(tc, idx) in m.tools"
                        :key="tc.id"
                        class="rounded-lg border border-[#fff8ee]/8 bg-[#050811]/75 px-2.5 py-2"
                      >
                        <div class="flex items-center justify-between gap-2 text-[11px]">
                          <span class="text-[#f2b880] font-semibold truncate">#{{ idx + 1 }} {{ formatToolLabel(tc.name) }}</span>
                          <div class="flex items-center gap-2 shrink-0">
                            <span v-if="tc.durationMs !== undefined" class="text-[10px] text-slate-400">{{ tc.durationMs }}ms</span>
                            <button
                              v-if="tc.result || tc.arguments"
                              type="button"
                              @click="copyMessageContent(tc.result || tc.arguments)"
                              class="text-[10px] text-slate-500 hover:text-[#f2b880] cursor-pointer"
                            >
                              复制
                            </button>
                            <span
                              :class="
                                !tc.done
                                  ? 'text-amber-300'
                                  : tc.status === 'error'
                                  ? 'text-rose-400'
                                  : 'text-emerald-400'
                              "
                            >
                              {{ !tc.done ? '执行中' : tc.status === 'error' ? '异常' : '已完成' }}
                            </span>
                          </div>
                        </div>
                        <div v-if="tc.arguments && tc.arguments !== '{}'" class="mt-1 text-slate-400 whitespace-pre-wrap break-all max-h-32 overflow-y-auto">入参: {{ formatToolPayload(tc.arguments) }}</div>
                        <div v-if="tc.result" class="mt-1 text-slate-300 whitespace-pre-wrap break-all max-h-48 overflow-y-auto border-t border-[#fff8ee]/6 pt-1">出参: {{ formatToolPayload(tc.result) }}</div>
                      </div>
                    </div>
                  </details>

                  <!-- Markdown Body -->
                  <div v-if="m.content" v-html="renderSafeMarkdown(m.content)" />
                  <div v-else-if="m.errorText" class="text-sm text-rose-400">[调用失败] {{ m.errorText }}</div>

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
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <polyline points="6 9 12 15 18 9" />
              </svg>
              <span>{{ store.state.isStreaming ? '正在生成 · 回到底部' : '回到底部' }}</span>
            </button>
          </Transition>
        </div>

        <!-- Floating Composer Dock + Popovers -->
        <div class="relative px-3 sm:px-6 pt-1 pb-[max(0.75rem,env(safe-area-inset-bottom,0.75rem))] sm:pb-5">
          <!-- Live Multi-Bot Relay Strip during Group Collaboration -->
          <div
            v-if="relaySteps.length > 0"
            class="max-w-[820px] w-full mx-auto mb-2 px-3 py-1.5 rounded-full border border-[#f2b880]/25 bg-[#080d1c]/90 backdrop-blur-md flex items-center gap-2 overflow-x-auto no-scrollbar text-[11px]"
          >
            <span class="font-mono text-slate-400 shrink-0">协同接力:</span>
            <template v-for="(step, sidx) in relaySteps" :key="step.id + sidx">
              <span v-if="sidx > 0" class="text-slate-600 shrink-0">-></span>
              <span
                :class="[
                  'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full shrink-0',
                  step.status === 'active'
                    ? 'bg-[#f2b880]/15 text-[#f2b880] font-medium'
                    : 'bg-emerald-400/10 text-emerald-300'
                ]"
              >
                <span :class="['w-1.5 h-1.5 rounded-full', step.status === 'active' ? 'bg-[#f2b880] animate-pulse' : 'bg-emerald-400']" />
                <span>{{ step.name }}</span>
                <span v-if="step.guest" class="text-[9.5px] opacity-80">(外援)</span>
              </span>
            </template>
          </div>

          <!-- Interactive @Mention Autocomplete Popover -->
          <Transition
            enter-active-class="transition duration-150 ease-out"
            enter-from-class="opacity-0 translate-y-2 scale-95"
            enter-to-class="opacity-100 translate-y-0 scale-100"
          >
            <div
              v-if="mentionOpen && mentionCandidates.length > 0"
              class="max-w-[820px] w-full mx-auto mb-2 rounded-2xl border border-[#f2b880]/35 bg-[#090e1e]/96 backdrop-blur-2xl shadow-2xl p-1.5 flex flex-col gap-0.5 max-h-60 overflow-y-auto"
            >
              <div class="px-2.5 py-1 flex items-center justify-between text-[10.5px] font-mono text-slate-400">
                <span>选择要 @点名 或临时支援的 Bot</span>
                <span>Enter 确认 · Esc 关闭</span>
              </div>
              <div
                v-for="(item, idx) in mentionCandidates"
                :key="item.bot.id"
                :class="[
                  'flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-xl transition-colors',
                  idx === mentionHighlight ? 'bg-[#f2b880]/16' : 'hover:bg-white/[0.04]'
                ]"
              >
                <button
                  type="button"
                  @click="pickMentionCandidate(item.bot)"
                  class="flex-1 min-w-0 flex items-center gap-2.5 text-left cursor-pointer"
                >
                  <BotAvatar :avatar-style="item.bot.avatar_style" :state="item.bot.state" size="sm" :interactive="false" />
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-1.5">
                      <span class="text-xs font-semibold text-white truncate">@{{ item.bot.name }}</span>
                      <span
                        :class="[
                          'px-1.5 py-0.5 rounded font-mono text-[9.5px] leading-none',
                          item.inRoom ? 'bg-[#f2b880]/15 text-[#f2b880]' : 'bg-emerald-400/12 text-emerald-300'
                        ]"
                      >
                        {{ item.inRoom ? (isGroupBot ? '群内成员' : '当前Bot') : '临时召唤外援' }}
                      </span>
                    </div>
                    <div class="text-[10.5px] text-slate-400 truncate">{{ item.bot.role_title }}</div>
                  </div>
                </button>
                <button
                  v-if="isGroupBot && !item.inRoom"
                  type="button"
                  @click="inviteFromMentionAndPick(item.bot)"
                  class="px-2 py-1 rounded-lg border border-[#f2b880]/40 bg-[#f2b880]/12 text-[10.5px] font-medium text-[#f2b880] hover:bg-[#f2b880]/22 shrink-0 cursor-pointer"
                >
                  拉群并@
                </button>
              </div>
            </div>
          </Transition>

          <!-- Interactive /Slash Command Palette -->
          <Transition
            enter-active-class="transition duration-150 ease-out"
            enter-from-class="opacity-0 translate-y-2 scale-95"
            enter-to-class="opacity-100 translate-y-0 scale-100"
          >
            <div
              v-if="slashOpen && filteredSlashCommands.length > 0"
              class="max-w-[820px] w-full mx-auto mb-2 rounded-2xl border border-[#f2b880]/35 bg-[#090e1e]/96 backdrop-blur-2xl shadow-2xl p-1.5 flex flex-col gap-0.5"
            >
              <div class="px-2.5 py-1 text-[10.5px] font-mono text-slate-400">快捷斜杠指令 (Enter 执行)</div>
              <button
                v-for="(cmd, idx) in filteredSlashCommands"
                :key="cmd.cmd"
                type="button"
                @click="pickSlashCommand(cmd)"
                :class="[
                  'w-full flex items-center justify-between gap-3 px-3 py-2 rounded-xl text-left cursor-pointer transition-colors',
                  idx === slashHighlight ? 'bg-[#f2b880]/16 text-white' : 'text-slate-300 hover:bg-white/[0.04]'
                ]"
              >
                <div class="flex items-center gap-2.5 min-w-0">
                  <span class="font-mono text-xs font-semibold text-[#f2b880] w-16 shrink-0">{{ cmd.cmd }}</span>
                  <span class="text-xs font-medium truncate">{{ cmd.title }}</span>
                </div>
                <span class="text-[11px] text-slate-400 truncate">{{ cmd.desc }}</span>
              </button>
            </div>
          </Transition>

          <div
            v-if="isGroupBot && groupMembers.length > 0"
            class="max-w-[820px] w-full mx-auto mb-2 flex items-center gap-1.5 flex-wrap"
          >
            <span class="font-mono text-[11px] text-slate-400">@点名协同:</span>
            <button
              v-for="member in groupMembers"
              :key="member.id"
              type="button"
              @click="appendMention(`@${member.name} `)"
              class="min-h-[36px] sm:min-h-[28px] inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs border border-[#fff8ee]/12 bg-[#fff8ee]/[0.04] hover:border-[#f2b880]/50 hover:text-[#f2b880] text-slate-200 cursor-pointer"
            >
              <BotAvatar :avatar-style="member.avatar_style" :state="member.state" size="sm" :interactive="false" />
              <span>@{{ member.name }}</span>
            </button>
            <button
              type="button"
              @click="store.state.showGroupDrawer = true"
              class="min-h-[36px] sm:min-h-[28px] inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs border border-dashed border-[#f2b880]/40 text-[#f2b880] hover:bg-[#f2b880]/10 cursor-pointer"
            >
              <span>+ 拉人</span>
            </button>
          </div>

          <form
            ref="formRef"
            @submit.prevent="sendPrompt()"
            :class="[
              'composer-dock max-w-[820px] w-full mx-auto rounded-[22px] border border-[#fff8ee]/14 focus-within:border-[#f2b880]/55 bg-[#0b101e]/92 backdrop-blur-xl shadow-[inset_0_1px_0_rgba(255,248,238,0.08),0_20px_46px_rgba(0,0,0,0.52)] px-3.5 sm:px-4 pt-3 pb-2.5 flex flex-col gap-2.5',
              store.state.isStreaming ? 'is-streaming' : ''
            ]"
          >
            <!-- Quote-Reply Preview Bar inside Composer -->
            <div
              v-if="replyTarget"
              class="flex items-center justify-between gap-2 px-3 py-1.5 rounded-xl border border-[#f2b880]/25 bg-[#f2b880]/[0.06] text-xs"
            >
              <div class="flex items-center gap-2 min-w-0">
                <span class="w-0.5 h-4 rounded-full bg-[#f2b880] shrink-0" />
                <span class="text-slate-300 truncate">{{ replyTarget.preview }}</span>
              </div>
              <button
                type="button"
                aria-label="取消引用"
                @click="replyTarget = null"
                class="w-5 h-5 rounded-full inline-flex items-center justify-center text-slate-400 hover:text-white cursor-pointer shrink-0"
              >
                <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </svg>
              </button>
            </div>

            <label :for="composerId" class="sr-only">向当前专属 Bot 下达指令</label>
            <textarea
              :id="composerId"
              ref="textareaRef"
              v-model="inputPrompt"
              rows="1"
              placeholder="输入指令，@ 召唤任何 Bot，/ 呼出快捷指令（Enter 发送）..."
              @input="handleComposerInput"
              @compositionend="handleCompositionEnd"
              @keydown="handleComposerKeydown"
              class="auto-size-textarea w-full bg-transparent border-0 text-sm text-slate-100 placeholder-slate-500 resize-none focus:outline-none min-h-[24px] max-h-[160px] overflow-y-auto leading-relaxed"
            />

            <div class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-2 min-w-0 flex-1 flex-wrap sm:flex-nowrap">
                <div class="flex-1 min-w-[140px] max-w-[230px] sm:flex-none sm:w-56 shrink-0">
                  <CustomSelect v-model="selectedModel" :options="store.modelOptions.value" compact aria-label="选择推理模型" />
                </div>
                <button
                  type="button"
                  :aria-pressed="enableThinking"
                  @click="enableThinking = !enableThinking"
                  :title="selectedModel.startsWith('pool:') || selectedModel === 'free-pool' ? '深度思考开关主要作用于 DeepSeek Flash / 支持 thinking 的中转站' : '开启或关闭 Flash 深度推理'"
                  :class="[
                    'min-h-[44px] sm:min-h-[32px] px-3 rounded-full text-xs border cursor-pointer whitespace-nowrap shrink-0',
                    enableThinking
                      ? 'border-[#f2b880]/50 bg-[#f2b880]/15 text-[#f2b880]'
                      : 'border-[#fff8ee]/12 text-slate-400 hover:text-white'
                  ]"
                >
                  深度思考: {{ enableThinking ? '开' : '关' }}
                </button>
                <span class="hidden sm:inline-flex items-center gap-1.5 font-mono text-[11px] text-slate-400 truncate" aria-live="polite">
                  <span
                    v-if="store.state.isStreaming"
                    class="w-1.5 h-1.5 rounded-full bg-[#f2b880] animate-pulse shrink-0"
                    aria-hidden="true"
                  />
                  <span class="truncate">{{ streamStatus }}</span>
                </span>
              </div>

              <div class="flex items-center gap-2 shrink-0">
                <button
                  v-if="store.state.isStreaming"
                  type="button"
                  @click="stopStream"
                  class="min-h-[44px] sm:min-h-[34px] px-3.5 rounded-full text-xs bg-rose-500/20 border border-rose-400/45 text-rose-300 hover:bg-rose-500/30 cursor-pointer"
                >
                  停止
                </button>
                <button
                  type="submit"
                  :disabled="store.state.isStreaming || !inputPrompt.trim()"
                  aria-label="发送指令"
                  title="发送 (Enter)"
                  class="w-11 h-11 sm:w-10 sm:h-10 rounded-full bg-[#f2b880] hover:bg-[#f7cba0] hover:scale-105 text-[#070a13] flex items-center justify-center cursor-pointer disabled:opacity-35 disabled:hover:scale-100 disabled:cursor-not-allowed shadow-md"
                >
                  <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.4" aria-hidden="true">
                    <line x1="12" y1="19" x2="12" y2="5" />
                    <polyline points="5 12 12 5 19 12" />
                  </svg>
                </button>
              </div>
            </div>
          </form>
        </div>
      </div>
    </Transition>

    <!-- View 2: Bot Routines -->
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0 translate-y-1.5"
      enter-to-class="opacity-100 translate-y-0"
    >
      <div v-show="store.state.botView === 'routines'" class="cq-container flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
        <div class="max-w-4xl mx-auto flex flex-col gap-4">
          <div class="flex items-center justify-between gap-3 flex-wrap">
            <div>
              <h2 class="text-base sm:text-lg font-semibold text-white">专属定时例程</h2>
              <p class="text-xs text-slate-400">触发后由当前 Bot 自主执行并将总结汇报至对话流</p>
            </div>
            <button
              type="button"
              @click="store.state.editingTask = { bot_id: bot?.id || '' }"
              class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer"
            >
              + 添加例程
            </button>
          </div>

          <div v-if="botRoutines.length === 0" class="vesper-card rounded-2xl p-8 text-center text-sm text-slate-400">
            当前 Bot 尚未绑定自动化例程，可点击右上角新增
          </div>

          <div ref="routinesGridRef" class="ops-task-grid stagger-list">
            <article
              v-for="t in botRoutines"
              :key="t.id"
              @pointermove="trackSpotlight"
              class="stagger-item luxury-card vesper-card-interactive rounded-2xl p-4 flex flex-col gap-3"
            >
              <div class="flex items-start justify-between gap-2.5">
                <div class="flex items-center gap-2.5 min-w-0">
                  <span class="w-8 h-8 rounded-xl border border-[#f2b880]/22 bg-[#f2b880]/10 text-[#f2b880] inline-flex items-center justify-center shrink-0">
                    <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                      <circle cx="12" cy="12" r="9" />
                      <polyline points="12 7 12 12 15 14" />
                    </svg>
                  </span>
                  <div class="min-w-0">
                    <h3 class="font-semibold text-sm text-white truncate">{{ t.name }}</h3>
                    <div class="font-mono text-[11px] text-slate-400 mt-0.5 truncate">{{ describeCron(t.cron_expr) }}</div>
                  </div>
                </div>
                <span
                  :class="[
                    'inline-flex items-center gap-1.5 font-mono text-[11px] shrink-0 px-2 py-0.5 rounded-full border',
                    t.enabled ? 'border-emerald-400/25 bg-emerald-400/10 text-emerald-300' : 'border-[#fff8ee]/10 text-slate-500'
                  ]"
                >
                  <span :class="['w-1.5 h-1.5 rounded-full', t.enabled ? 'bg-emerald-400 animate-pulse' : 'bg-slate-600']" aria-hidden="true" />
                  {{ t.enabled ? '运行中' : '已暂停' }}
                </span>
              </div>
              <p class="text-xs text-slate-300/90 leading-relaxed">{{ t.description }}</p>
              <div class="font-mono text-[11px] text-slate-400 truncate rounded-lg bg-[#050811]/60 border border-[#fff8ee]/6 px-2.5 py-1.5">
                最近结果: {{ t.last_run?.summary || '等待首次执行' }}
              </div>
              <div class="flex items-center gap-2 pt-1">
                <button
                  type="button"
                  @click="runRoutine(t.id)"
                  class="min-h-[44px] sm:min-h-[32px] px-3 py-1.5 rounded-lg bg-[#f2b880]/15 border border-[#f2b880]/40 text-[#f2b880] hover:bg-[#f2b880]/25 text-xs cursor-pointer"
                >
                  立即执行
                </button>
                <button
                  type="button"
                  @click="store.state.editingTask = t"
                  class="min-h-[44px] sm:min-h-[32px] px-3 py-1.5 rounded-lg border border-[#fff8ee]/12 text-slate-300 hover:bg-white/5 text-xs cursor-pointer"
                >
                  配置参数
                </button>
              </div>
            </article>
          </div>
        </div>
      </div>
    </Transition>

    <!-- View 3: Bot Profile, Tools & Scoped Memories -->
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0 translate-y-1.5"
      enter-to-class="opacity-100 translate-y-0"
    >
      <div v-show="store.state.botView === 'config'" class="flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
        <div class="max-w-4xl mx-auto flex flex-col gap-5">
          <form @submit.prevent="saveBotProfile" class="luxury-card rounded-2xl p-5 flex flex-col gap-4">
            <BotAvatarPicker v-model="cfgAvatar" :preview-name="cfgName || bot?.name" />

            <div class="flex flex-col gap-2 text-xs text-slate-300">
              <span>交流定位 / 角色模式</span>
              <SegmentedTabs v-model="cfgBotMode" :items="BOT_MODE_TABS" variant="subtle" aria-label="交流定位" />
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <label class="flex flex-col gap-1.5 text-xs text-slate-300">
                <span>{{ isGroupBot ? '群聊名称' : 'Bot 名称' }}</span>
                <input v-model="cfgName" type="text" required class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60" />
              </label>
              <label class="flex flex-col gap-1.5 text-xs text-slate-300">
                <span>职责定位 / 关系称谓</span>
                <input v-model="cfgRole" type="text" required class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60" />
              </label>
              <label class="md:col-span-2 flex flex-col gap-1.5 text-xs text-slate-300">
                <span>性格特质与沟通口吻</span>
                <input
                  v-model="cfgTraits"
                  type="text"
                  placeholder="如：说话像真人发消息一样简短有温度 / 严谨务实直击根因"
                  class="min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-xs text-white focus:border-[#f2b880]/60"
                />
              </label>
              <div class="flex flex-col gap-1.5 text-xs text-slate-300">
                <span>默认推理模型</span>
                <CustomSelect v-model="cfgModel" :options="store.modelOptions.value" aria-label="默认推理模型" />
              </div>
              <div class="flex flex-col gap-1.5 text-xs text-slate-300">
                <span>记忆隔离策略</span>
                <CustomSelect v-model="cfgMemoryPolicy" :options="MEMORY_POLICY_OPTIONS" aria-label="记忆隔离策略" />
              </div>
              <div class="md:col-span-2 flex items-center justify-between">
                <ToggleSwitch v-model="cfgThinking" label="默认开启深度思考" />
              </div>

              <!-- Group Member Selector when viewing a group chat -->
              <div v-if="isGroupBot" class="md:col-span-2 flex flex-col gap-2.5 text-xs text-slate-300">
                <div class="flex items-center justify-between gap-2">
                  <span>群协同策略</span>
                </div>
                <SegmentedTabs
                  v-model="cfgGroupMode"
                  :items="[
                    { id: 'smart', label: '智能协同' },
                    { id: 'sequential', label: '全员接力' },
                    { id: 'mention_only', label: '仅 @点名' },
                  ]"
                  variant="subtle"
                  aria-label="群协同策略"
                />

                <span>群聊协同成员 ({{ cfgMembers.length }} 位)</span>
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <button
                    v-for="sp in availableSpecialistBots"
                    :key="sp.id"
                    type="button"
                    @click="toggleCfgMember(sp.id)"
                    :aria-pressed="cfgMembers.includes(sp.id)"
                    :class="[
                      'group flex items-center gap-2.5 px-3 py-2 rounded-xl border text-left cursor-pointer',
                      cfgMembers.includes(sp.id)
                        ? 'border-[#f2b880]/60 bg-[#f2b880]/12 text-white'
                        : 'border-[#fff8ee]/10 bg-[#070a16]/70 text-slate-400 hover:border-[#fff8ee]/25'
                    ]"
                  >
                    <BotAvatar :avatar-style="sp.avatar_style" :state="sp.state" size="sm" />
                    <span class="text-xs font-medium flex-1 truncate">{{ sp.name }}</span>
                    <span class="text-[11px] font-mono">{{ cfgMembers.includes(sp.id) ? '在群' : '未选' }}</span>
                  </button>
                </div>
              </div>

              <label class="md:col-span-2 flex flex-col gap-1.5 text-xs text-slate-300">
                <span>专属人设与行为约束 (System Prompt)</span>
                <textarea v-model="cfgPrompt" rows="4" class="auto-size-textarea rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 resize-y" />
              </label>
              <div class="md:col-span-2 flex flex-col gap-2 text-xs text-slate-300">
                <span>绑定内置工具集 ({{ cfgTools.length }}/{{ availableTools.length }})</span>
                <div class="flex flex-wrap gap-2">
                  <label
                    v-for="tool in availableTools"
                    :key="tool.id"
                    class="min-h-[36px] inline-flex items-center gap-2 px-3 py-1 rounded-xl border border-[#fff8ee]/12 bg-[#060a16]/75 font-mono text-xs text-slate-300 cursor-pointer hover:border-[#f2b880]/40 has-[:checked]:border-[#f2b880]/55 has-[:checked]:bg-[#f2b880]/14 has-[:checked]:text-[#f7cba0] transition-colors"
                  >
                    <input v-model="cfgTools" type="checkbox" :value="tool.id" class="accent-[#f2b880]" />
                    <span>{{ tool.description }}</span>
                  </label>
                </div>
              </div>

              <div class="md:col-span-2 flex flex-col gap-2 text-xs text-slate-300 pt-2 border-t border-[#fff8ee]/8">
                <div class="flex items-center justify-between">
                  <span>挂载自定义技能 ({{ cfgSkillIds.length }}/{{ availableSkills.length }})</span>
                  <button
                    type="button"
                    @click="store.state.showSkillsModal = true"
                    class="text-[11px] text-[#f2b880] hover:underline cursor-pointer"
                  >
                    打开技能工坊
                  </button>
                </div>
                <div v-if="availableSkills.length === 0" class="text-[11px] text-slate-500">
                  暂无已启用的自定义技能，点击右上「打开技能工坊」创建自己的 Playbook 或 Webhook 工具。
                </div>
                <div v-else class="flex flex-wrap gap-2">
                  <label
                    v-for="sk in availableSkills"
                    :key="sk.id"
                    class="min-h-[36px] inline-flex items-center gap-2 px-3 py-1 rounded-xl border border-[#fff8ee]/12 bg-[#060a16]/75 text-xs text-slate-300 cursor-pointer hover:border-cyan-400/40 has-[:checked]:border-cyan-400/55 has-[:checked]:bg-cyan-500/14 has-[:checked]:text-cyan-200 transition-colors"
                  >
                    <input v-model="cfgSkillIds" type="checkbox" :value="sk.id" class="accent-cyan-400" />
                    <span>{{ sk.name }}</span>
                    <span class="font-mono text-[10px] opacity-70">{{ sk.kind === 'webhook' ? 'API' : 'Doc' }}</span>
                  </label>
                </div>
              </div>
            </div>

            <div class="flex items-center justify-between gap-3 flex-wrap pt-3 border-t border-[#fff8ee]/10">
              <button
                v-if="bot"
                type="button"
                @click="deleteCustomBot"
                class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-rose-400/40 bg-rose-500/15 text-rose-300 hover:bg-rose-500/25 text-xs cursor-pointer"
              >
                {{ isGroupBot ? '解散此群聊' : '删除此 Bot' }}
              </button>
              <span v-else />
              <button
                type="submit"
                :disabled="savingProfile"
                class="min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50"
              >
                {{ savingProfile ? '保存中...' : '保存配置' }}
              </button>
            </div>
          </form>

          <!-- Scoped Memory CRUD inline inside Bot/Group Config -->
          <div class="vesper-card rounded-2xl p-5 flex flex-col gap-3">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <div>
                <h3 class="font-semibold text-sm text-white">
                  {{ isGroupBot ? '当前群聊专属记忆' : '当前 Bot 记忆域' }} ({{ scopedMemories.length }})
                </h3>
                <p class="text-[11px] text-slate-400 mt-0.5">
                  {{
                    cfgMemoryPolicy === 'isolated'
                      ? '当前策略为严格私有：仅读取当前 Bot/群聊的独立记忆，绝不与其他 Bot 混淆。'
                      : '当前策略为私有优先 + 全局共享：当前 Bot 专属记忆权重更高，并保留全局基础设施事实。'
                  }}
                </p>
              </div>
              <button
                type="button"
                @click="openScopedMemoriesManager"
                class="px-3 py-1.5 rounded-xl border border-[#fff8ee]/12 text-xs text-slate-300 hover:text-[#f2b880] cursor-pointer"
              >
                在记忆库管理
              </button>
            </div>

            <form @submit.prevent="addBotMemory" class="grid grid-cols-1 sm:grid-cols-12 gap-2">
              <input
                v-model="newMemKey"
                type="text"
                placeholder="记忆键名 (如 preferred_tone)"
                required
                class="sm:col-span-3 min-h-[36px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-2.5 font-mono text-xs text-white"
              />
              <input
                v-model="newMemContent"
                type="text"
                placeholder="写给它记住的专属事实或偏好..."
                required
                class="sm:col-span-6 min-h-[36px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-2.5 text-xs text-white"
              />
              <label class="sm:col-span-1 inline-flex items-center justify-center gap-1 text-[11px] text-slate-300 cursor-pointer">
                <input v-model="newMemPinned" type="checkbox" class="accent-[#f2b880]" />
                <span>置顶</span>
              </label>
              <button
                type="submit"
                :disabled="savingMem"
                class="sm:col-span-2 min-h-[36px] rounded-xl bg-[#f2b880]/18 border border-[#f2b880]/45 text-[#f7cba0] hover:bg-[#f2b880]/28 text-xs font-medium cursor-pointer disabled:opacity-50"
              >
                {{ savingMem ? '写入中' : '写入专属记忆' }}
              </button>
            </form>

            <div v-if="scopedMemories.length === 0" class="text-xs text-slate-500 py-3">
              暂无关联长期记忆；对话中会自动沉淀偏好与重要背景，也可在上方直接添加。
            </div>
            <div class="flex flex-col gap-2">
              <article
                v-for="mem in scopedMemories"
                :key="mem.id"
                class="rounded-xl border border-[#fff8ee]/10 bg-[#070a16]/75 px-3.5 py-2.5"
              >
                <div class="flex items-center justify-between gap-2 flex-wrap">
                  <div class="flex items-center gap-1.5 min-w-0">
                    <span
                      v-if="mem.pinned"
                      class="px-1.5 py-0.5 rounded border border-amber-400/50 bg-amber-400/15 font-mono text-[10px] text-amber-200"
                    >
                      置顶
                    </span>
                    <span class="px-1.5 py-0.5 rounded border border-[#fff8ee]/10 font-mono text-[10px] text-slate-400">
                      W{{ mem.importance || 3 }}
                    </span>
                    <strong class="font-mono text-xs text-[#f2b880]">{{ mem.fact_key }}</strong>
                    <span class="font-mono text-[10.5px] text-slate-500">
                      · {{ mem.scope?.startsWith('bot:') ? '本域私有' : '全局共享' }}
                      <template v-if="mem.recall_count"> · 召回 {{ mem.recall_count }} 次</template>
                    </span>
                  </div>
                  <div class="flex items-center gap-1.5">
                    <button
                      type="button"
                      @click="toggleBotMemoryPin(mem)"
                      class="px-2 py-0.5 rounded border border-[#fff8ee]/10 text-[10.5px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
                    >
                      {{ mem.pinned ? '取消置顶' : '置顶' }}
                    </button>
                    <button
                      type="button"
                      @click="editingMemId === mem.id ? (editingMemId = null) : (editingMemId = mem.id, editMemContent = mem.content)"
                      class="px-2 py-0.5 rounded border border-[#fff8ee]/10 text-[10.5px] text-slate-300 hover:text-white cursor-pointer"
                    >
                      {{ editingMemId === mem.id ? '取消' : '编辑' }}
                    </button>
                    <button
                      type="button"
                      @click="deleteBotMemory(mem.id)"
                      class="px-2 py-0.5 rounded border border-rose-400/30 text-[10.5px] text-rose-300 hover:bg-rose-500/15 cursor-pointer"
                    >
                      移除
                    </button>
                  </div>
                </div>
                <div v-if="editingMemId === mem.id" class="mt-2 flex gap-2">
                  <input
                    v-model="editMemContent"
                    type="text"
                    class="flex-1 min-h-[32px] rounded-lg border border-[#f2b880]/45 bg-[#050811] px-2.5 text-xs text-white"
                  />
                  <button
                    type="button"
                    @click="saveInlineMemEdit(mem)"
                    class="px-3 py-1 rounded-lg bg-[#f2b880] text-[#070a13] font-semibold text-xs cursor-pointer"
                  >
                    保存
                  </button>
                </div>
                <p v-else class="text-xs text-slate-200 mt-1">{{ mem.content }}</p>
              </article>
            </div>
          </div>
        </div>
      </div>
    </Transition>

    <!-- View 4: Activity Pulse -->
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0 translate-y-1.5"
      enter-to-class="opacity-100 translate-y-0"
    >
      <div v-show="store.state.botView === 'activity'" class="flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
        <div class="max-w-4xl mx-auto flex flex-col gap-3">
          <div class="flex items-center justify-between gap-2 flex-wrap">
            <h2 class="text-base sm:text-lg font-semibold text-white">
              {{ bot?.name || '当前 Bot' }} 的专属运行脉搏
            </h2>
            <span class="text-[11px] font-mono text-slate-400">
              仅显示该会话自身的工具/技能调用、绑定例程与专属推送
            </span>
          </div>
          <div v-if="activityEntries.length === 0" class="vesper-card rounded-2xl p-8 text-center text-sm text-slate-400">
            暂无该 Bot 的专属运行记录
          </div>
          <div ref="activityListRef" class="stagger-list flex flex-col gap-2.5">
            <article
              v-for="(act, idx) in activityEntries"
              :key="idx"
              class="stagger-item vesper-card rounded-xl px-4 py-3"
            >
              <div class="flex items-center justify-between gap-2 flex-wrap">
                <div class="flex items-center gap-2 min-w-0">
                  <span
                    v-if="act.status"
                    :class="[
                      'px-1.5 py-0.5 rounded font-mono text-[10px] border shrink-0',
                      act.status === 'error'
                        ? 'border-rose-400/40 bg-rose-500/10 text-rose-300'
                        : 'border-emerald-400/40 bg-emerald-500/10 text-emerald-300',
                    ]"
                  >
                    {{ act.status }}
                  </span>
                  <strong class="text-sm text-white truncate">{{ act.title }}</strong>
                </div>
                <span class="font-mono text-[11px] text-slate-400 shrink-0">{{ act.time }}</span>
              </div>
              <p class="text-xs text-slate-300 mt-1">{{ act.detail }}</p>
            </article>
          </div>
        </div>
      </div>
    </Transition>
    </template>

    <GroupDrawer />
  </section>
</template>
