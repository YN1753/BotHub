<script setup lang="ts">
import { ref, computed, watch, nextTick, useId, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore, type ChatMessageUI } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { botStateLabel, describeCron, formatTime, formatTimelineDivider, groupModeLabel } from '../utils/format'
import { renderSafeMarkdown } from '../utils/markdown'
import { applySiblingStaggerFallback, isImeComposingEnter, trackSpotlight } from '../utils/topLayer'
import type { Bot } from '../types/bothub'
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
const selectedModel = ref('deepseek-flash')
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

const SLASH_COMMANDS = [
  { cmd: '/assets', title: '资产全面审计', desc: '盘点 VPS、域名与订阅到期风险', prompt: '请调用工具盘点当前全部数字资产与临期风险并给出续费建议。' },
  { cmd: '/nezha', title: '探针节点巡检', desc: '查看哪吒节点在线率与负载情况', prompt: '请调用 query_nezha_nodes 检查哪吒监控面板节点的实时在线与健康状态。' },
  { cmd: '/radar', title: '前沿开源雷达', desc: '汇总近期高价值技术趋势', prompt: '请检索并梳理今天最值得关注的 GitHub 与 Hacker News 开源技术动态。' },
  { cmd: '/tasks', title: '调度队列核查', desc: '检查各自动化例程状态与下次触发时间', prompt: '请核查当前所有自动化调度任务的运行状态与上次执行摘要。' },
  { cmd: '/invite', title: '拉人进群 / 群设置', desc: '打开右侧群成员与协同策略抽屉', action: 'open_drawer' },
  { cmd: '/clear', title: '清空当前对话', desc: '清空当前频道聊天时间线', action: 'clear_timeline' },
]

// Config Editor local state
const cfgName = ref('')
const cfgRole = ref('')
const cfgPrompt = ref('')
const cfgModel = ref('deepseek-flash')
const cfgAvatar = ref('butler:gold')
const cfgThinking = ref(true)
const cfgTools = ref<string[]>([])
const cfgMembers = ref<string[]>([])
const cfgGroupMode = ref('smart')
const savingProfile = ref(false)

const VIEW_TABS = [
  { id: 'stream', label: '对话流' },
  { id: 'routines', label: '例程' },
  { id: 'config', label: '能力与记忆' },
  { id: 'activity', label: '运行脉搏' },
]

const TOOL_CATALOG = [
  { id: 'read_url', label: 'read_url (网页抓取与摘要)' },
  { id: 'web_search', label: 'web_search (开源雷达搜索)' },
  { id: 'query_nezha_nodes', label: 'query_nezha_nodes (哪吒探针直连)' },
  { id: 'manage_assets', label: 'manage_assets (数字资产台账)' },
  { id: 'manage_tasks', label: 'manage_tasks (调度例程管控)' },
  { id: 'remember_fact', label: 'remember_fact (写入长期记忆)' },
  { id: 'recall_memory', label: 'recall_memory (检索长期记忆)' },
]

const STARTER_CARDS = [
  {
    icon: 'assets',
    tag: '资产审计',
    title: '盘点资产与风险',
    desc: '检查服务器与域名到期梯度并输出锐评',
    prompt: '请调用工具盘点我当前的全部数字资产与算力节点，并按到期风险给出锐评。',
  },
  {
    icon: 'pulse',
    tag: '探针巡检',
    title: '探查哪吒节点脉搏',
    desc: '实时读取服务器在线状态、负载与流量健康度',
    prompt: '请调用 query_nezha_nodes 检查哪吒监控面板节点的在线状态与配置详情。',
  },
  {
    icon: 'cron',
    tag: '调度例程',
    title: '核查调度任务队列',
    desc: '汇总定时例程执行状态与下次自动触发窗口',
    prompt: '请查看当前所有自动化调度任务的运行状态，并告诉我下次触发时间。',
  },
]

watch(
  bot,
  (b) => {
    if (!b) return
    selectedModel.value = b.model || 'deepseek-flash'
    enableThinking.value = b.enable_thinking !== false
    cfgName.value = b.name
    cfgRole.value = b.role_title
    cfgPrompt.value = b.system_prompt
    cfgModel.value = b.model || 'deepseek-flash'
    cfgAvatar.value = b.avatar_style || 'cyan'
    cfgThinking.value = b.enable_thinking !== false
    cfgGroupMode.value = b.group_mode || 'smart'
    announcementDismissed.value = false
    replyTarget.value = null
    try {
      const arr = JSON.parse(b.allowed_tools || '[]')
      cfgTools.value = Array.isArray(arr) ? arr : []
    } catch {
      cfgTools.value = []
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

const isGroupBot = computed(() => Boolean(bot.value?.is_group || bot.value?.id === 'team_room'))

const availableSpecialistBots = computed(() =>
  store.state.bots.filter((b) => !b.is_group && b.id !== 'team_room')
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

const filteredSlashCommands = computed(() => {
  const val = inputPrompt.value.trim().toLowerCase()
  if (!val.startsWith('/')) return []
  return SLASH_COMMANDS.filter((c) => c.cmd.includes(val) || c.title.toLowerCase().includes(val.slice(1)))
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
  if (!content) return
  try {
    const key = `${msg.senderBotId || 'bot'}_note_${Date.now().toString(36)}`
    await BotHubApi.saveMemory({
      scope: `bot:${bot.value?.id || 'butler'}`,
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

const botRoutines = computed(() =>
  store.state.tasks.filter((t) => (t.bot_id || 'butler') === (bot.value?.id || 'butler'))
)

const scopedMemories = computed(() => {
  const bScope = `bot:${bot.value?.id || ''}`
  return store.state.memories.filter((m) => !m.scope || m.scope === 'global' || m.scope === bScope)
})

const activityEntries = computed(() => {
  const bid = bot.value?.id || ''
  const items: Array<{ title: string; time: string; detail: string }> = []
  botRoutines.value.forEach((t) => {
    if (t.last_run) {
      items.push({
        title: `例程执行 · ${t.name}`,
        time: `${formatTime(t.last_run.started_at)} · ${t.last_run.duration_ms}ms`,
        detail: t.last_run.summary,
      })
    }
  })
  store.state.notifications
    .filter((n) => {
      if (bid === 'sentinel') return n.source.includes('nezha')
      if (bid === 'radar') return n.source.includes('radar')
      if (bid === 'butler') return n.source.includes('morning') || n.source.includes('asset')
      return false
    })
    .slice(0, 10)
    .forEach((n) => {
      items.push({
        title: `推送投递 · ${n.title}`,
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

function pickSlashCommand(item: (typeof SLASH_COMMANDS)[number]) {
  slashOpen.value = false
  inputPrompt.value = ''
  if (item.action === 'open_drawer') {
    store.state.showGroupDrawer = true
    return
  }
  if (item.action === 'clear_timeline') {
    clearTimeline()
    return
  }
  if (item.prompt) {
    sendPrompt(item.prompt)
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
    store.setBotDraft(store.state.activeBotId, val)
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
  () => store.state.activeBotId,
  (newId, oldId) => {
    if (oldId && oldId !== newId) {
      store.setBotDraft(oldId, inputPrompt.value)
    }
    const draft = store.state.pendingDraftPrompt ? '' : store.getBotDraft(newId)
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
  const botId = curBot?.id || 'butler'
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
    // Reload synced server IDs quietly so reply/delete have accurate IDs
    if (curBot?.id === store.state.activeBotId) {
      const synced = await BotHubApi.getBotMessages(curBot.id).catch(() => null)
      if (synced?.messages && !store.state.isStreaming) {
        const srvMsgs = synced.messages.filter((m) => m.role === 'user' || m.role === 'assistant' || m.role === 'system')
        if (srvMsgs.length >= store.state.timeline.length) {
          store.state.timeline = srvMsgs.map((m) => {
            const sender = store.state.bots.find((b) => b.id === (m.sender_bot_id || m.bot_id)) || curBot
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
            return {
              localId: `srv_${m.id}`,
              serverId: m.id,
              role: kind === 'system' ? 'system' : m.role === 'user' ? 'user' : 'assistant',
              senderBotId: m.sender_bot_id || m.bot_id || sender?.id || 'butler',
              senderName: sender?.name || 'BotHub',
              roleTitle: sender?.role_title || '',
              avatarStyle: sender?.avatar_style || 'cyan',
              model: m.model_used || sender?.model || 'deepseek-flash',
              msgKind: kind,
              createdAt: m.created_at,
              content: m.content || '',
              reasoning: m.reasoning_content || '',
              tools: parsedTools,
              replyToId: m.reply_to_id || undefined,
              replyPreview: m.reply_preview || undefined,
              statusText: kind === 'routine' ? '例程主动汇报' : '已完成',
            }
          })
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
      enable_thinking: cfgThinking.value,
      system_prompt: cfgPrompt.value.trim(),
      allowed_tools: JSON.stringify(cfgTools.value),
      member_ids: JSON.stringify(cfgMembers.value),
      group_mode: cfgGroupMode.value,
      is_pinned: bot.value.is_pinned,
    })
    store.notify('Bot 人设与工具权限已更新')
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    savingProfile.value = false
  }
}

async function deleteCustomBot() {
  if (!bot.value || bot.value.is_builtin) return
  if (!window.confirm(`确定注销自定义 Bot [${bot.value.name}] 吗？其专属聊天流将一并移除。`)) return
  await BotHubApi.deleteBot(bot.value.id)
  store.notify('已注销自定义 Bot')
  await store.loadDashboard(false)
  await store.selectBot('butler')
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
            <h1 class="text-sm md:text-base font-semibold text-white truncate">{{ bot?.name || '长庚执更' }}</h1>
            <span
              v-if="isGroupBot"
              class="hidden sm:inline-flex font-mono text-[10px] px-2 py-0.5 rounded-full border border-[#fff8ee]/14 bg-[#fff8ee]/[0.04] text-slate-300 leading-none shrink-0"
            >
              {{ groupModeLabel(bot?.group_mode) }}
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

      <div class="flex items-center gap-2 max-w-full ml-auto">
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

    <!-- View 1: Persistent Stream -->
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0 translate-y-1.5"
      enter-to-class="opacity-100 translate-y-0"
    >
      <div v-show="store.state.botView === 'stream'" class="flex-1 min-h-0 flex flex-col">
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
                <h2 class="text-xl sm:text-2xl font-semibold text-white tracking-tight">{{ bot?.name }} 随时待命</h2>
                <p class="text-xs sm:text-sm text-slate-400 max-w-md">
                  {{ bot?.role_title }} · 定时产出与探针告警将实时投递至此持久时间线
                </p>
              </div>

              <div ref="starterGridRef" class="starter-grid stagger-list mt-3 text-left">
                <button
                  v-for="card in STARTER_CARDS"
                  :key="card.title"
                  type="button"
                  @pointermove="trackSpotlight"
                  @click="sendPrompt(card.prompt)"
                  class="group stagger-item luxury-card vesper-card-interactive rounded-2xl p-4 flex flex-col gap-3 cursor-pointer text-left"
                >
                  <div class="flex items-center justify-between gap-2">
                    <span class="w-8 h-8 rounded-xl border border-[#f2b880]/25 bg-[#f2b880]/10 text-[#f2b880] inline-flex items-center justify-center shrink-0">
                      <svg v-if="card.icon === 'assets'" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                        <rect x="2" y="3" width="20" height="8" rx="2" />
                        <rect x="2" y="13" width="20" height="8" rx="2" />
                        <line x1="6" y1="7" x2="6.01" y2="7" />
                        <line x1="6" y1="17" x2="6.01" y2="17" />
                      </svg>
                      <svg v-else-if="card.icon === 'pulse'" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                        <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
                      </svg>
                      <svg v-else viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                        <circle cx="12" cy="12" r="9" />
                        <polyline points="12 7 12 12 15 14" />
                      </svg>
                    </span>
                    <span class="font-mono text-[10px] tracking-wider uppercase text-slate-500 group-hover:text-[#f2b880]/80 transition-colors">
                      {{ card.tag }}
                    </span>
                  </div>
                  <div class="flex-1">
                    <div class="flex items-center justify-between gap-2">
                      <strong class="text-sm font-semibold text-white group-hover:text-[#f7cba0] transition-colors">
                        {{ card.title }}
                      </strong>
                      <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="text-slate-600 group-hover:text-[#f2b880] group-hover:translate-x-0.5 group-hover:-translate-y-0.5 transition-transform shrink-0">
                        <line x1="7" y1="17" x2="17" y2="7" />
                        <polyline points="7 7 17 7 17 17" />
                      </svg>
                    </div>
                    <p class="text-[11.5px] text-slate-400/90 leading-relaxed mt-1">
                      {{ card.desc }}
                    </p>
                  </div>
                </button>
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
                          <span class="text-[#f2b880] font-semibold">#{{ idx + 1 }} {{ tc.name }}</span>
                          <span :class="tc.done ? 'text-emerald-400' : 'text-amber-300'">{{ tc.done ? '已完成' : '执行中' }}</span>
                        </div>
                        <div v-if="tc.arguments" class="mt-1 text-slate-400 whitespace-pre-wrap break-all">入参: {{ tc.arguments }}</div>
                        <div v-if="tc.result" class="mt-1 text-slate-300 whitespace-pre-wrap break-all">出参: {{ tc.result }}</div>
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
              @click="store.state.editingTask = { bot_id: bot?.id || 'butler' }"
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

            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <label class="flex flex-col gap-1.5 text-xs text-slate-300">
                <span>{{ isGroupBot ? '群聊名称' : 'Bot 名称' }}</span>
                <input v-model="cfgName" type="text" required class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60" />
              </label>
              <label class="flex flex-col gap-1.5 text-xs text-slate-300">
                <span>职责定位</span>
                <input v-model="cfgRole" type="text" required class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60" />
              </label>
              <div class="flex flex-col gap-1.5 text-xs text-slate-300">
                <span>默认推理模型</span>
                <CustomSelect v-model="cfgModel" :options="store.modelOptions.value" aria-label="默认推理模型" />
              </div>
              <div class="flex flex-col justify-end">
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
                <span>绑定可用工具集 (最小权限白名单)</span>
                <div class="flex flex-wrap gap-2">
                  <label
                    v-for="tool in TOOL_CATALOG"
                    :key="tool.id"
                    class="min-h-[44px] sm:min-h-[34px] inline-flex items-center gap-2 px-3 py-1.5 rounded-xl border border-[#fff8ee]/12 bg-[#060a16]/75 font-mono text-xs text-slate-300 cursor-pointer hover:border-[#f2b880]/40 has-[:checked]:border-[#f2b880]/55 has-[:checked]:bg-[#f2b880]/14 has-[:checked]:text-[#f7cba0] transition-colors"
                  >
                    <input v-model="cfgTools" type="checkbox" :value="tool.id" class="accent-[#f2b880]" />
                    <span>{{ tool.label }}</span>
                  </label>
                </div>
              </div>
            </div>

            <div class="flex items-center justify-between gap-3 flex-wrap pt-3 border-t border-[#fff8ee]/10">
              <button
                v-if="bot && !bot.is_builtin"
                type="button"
                @click="deleteCustomBot"
                class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-rose-400/40 bg-rose-500/15 text-rose-300 hover:bg-rose-500/25 text-xs cursor-pointer"
              >
                {{ isGroupBot ? '解散此群聊' : '注销此自定义 Bot' }}
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

          <div class="vesper-card rounded-2xl p-5">
            <h3 class="font-semibold text-sm text-white mb-3">关联记忆背景 (全局 + 专属域)</h3>
            <div v-if="scopedMemories.length === 0" class="text-xs text-slate-500">暂无关联长期记忆</div>
            <div class="flex flex-col gap-2">
              <article
                v-for="mem in scopedMemories"
                :key="mem.id"
                class="rounded-xl border border-[#fff8ee]/10 bg-[#070a16]/75 px-3.5 py-2.5"
              >
                <div class="flex items-center justify-between gap-2 flex-wrap">
                  <strong class="font-mono text-xs text-[#f2b880]">{{ mem.fact_key }}</strong>
                  <span class="font-mono text-[11px] text-slate-500">{{ mem.scope || 'global' }} · {{ mem.category }}</span>
                </div>
                <p class="text-xs text-slate-200 mt-1">{{ mem.content }}</p>
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
          <h2 class="text-base sm:text-lg font-semibold text-white">运行脉搏与最近汇报摘要</h2>
          <div v-if="activityEntries.length === 0" class="vesper-card rounded-2xl p-8 text-center text-sm text-slate-400">
            暂无后台运行脉搏记录
          </div>
          <div ref="activityListRef" class="stagger-list flex flex-col gap-2.5">
            <article
              v-for="(act, idx) in activityEntries"
              :key="idx"
              class="stagger-item vesper-card rounded-xl px-4 py-3"
            >
              <div class="flex items-center justify-between gap-2 flex-wrap">
                <strong class="text-sm text-white">{{ act.title }}</strong>
                <span class="font-mono text-[11px] text-slate-400">{{ act.time }}</span>
              </div>
              <p class="text-xs text-slate-300 mt-1">{{ act.detail }}</p>
            </article>
          </div>
        </div>
      </div>
    </Transition>

    <GroupDrawer />
  </section>
</template>
