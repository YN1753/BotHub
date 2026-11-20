<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore, type ChatMessageUI } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { parseJsonStringArray, triggerBlobDownload } from '../utils/format'
import { resolveGroupMemberBots, resolveScopedMemories } from '../utils/botLook'
import { copyToClipboard } from '../utils/clipboard'
import { lazyPanelChunk } from '../utils/lazyComponent'
import { resolveTabSlideTransition, type DirectionalTransitionName } from '../utils/motion'
import { handleMarkdownCodeCopy } from '../utils/markdown'
import { shortToolLabel } from '../utils/toolMeta'
import { useBotChatStream, type ReplyQuoteTarget } from '../composables/useBotChatStream'
import type { Bot } from '../types/bothub'
import BotAvatar from './BotAvatar.vue'
import GroupDrawer from './GroupDrawer.vue'
import BotWorkspaceHeader from './workspace/BotWorkspaceHeader.vue'
import BotThreadBar from './workspace/BotThreadBar.vue'
import BotMessageItem from './workspace/BotMessageItem.vue'
import BotChatComposer, { type StarterChipItem } from './workspace/BotChatComposer.vue'
import BotChatSearchBar from './workspace/BotChatSearchBar.vue'

const BotRoutinesTab = lazyPanelChunk(() => import('./workspace/BotRoutinesTab.vue'))
const BotConfigTab = lazyPanelChunk(() => import('./workspace/BotConfigTab.vue'))
const BotActivityTab = lazyPanelChunk(() => import('./workspace/BotActivityTab.vue'))
const BotStarterPanel = lazyPanelChunk(() => import('./workspace/BotStarterPanel.vue'))
const BotSideInspector = lazyPanelChunk(() => import('./workspace/BotSideInspector.vue'))

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
const scrollContainerRef = ref<HTMLElement | null>(null)
const composerRef = ref<InstanceType<typeof BotChatComposer> | null>(null)
const chatSearchBarRef = ref<InstanceType<typeof BotChatSearchBar> | null>(null)
const userScrolledUp = ref(false)
const unreadBelowCount = ref(0)
const replyTarget = ref<ReplyQuoteTarget | null>(null)
const announcementDismissed = ref(false)

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

const {
  streamStatus,
  lastPrompt,
  relaySteps,
  activeSpeakerBotId,
  sendPrompt,
  stopStream,
} = useBotChatStream({
  store,
  inputPrompt,
  selectedModel,
  enableThinking,
  replyTarget,
  scrollToBottom,
  onClosePopovers: () => composerRef.value?.closePopovers(),
})

// In-Chat Message Search (Cmd/Ctrl + F)
const chatSearchOpen = ref(false)
const chatSearchQuery = ref('')
const chatSearchCursor = ref(0)

const chatSearchMatches = computed(() => {
  const q = chatSearchQuery.value.trim().toLowerCase()
  if (!q) return []
  return store.state.timeline
    .filter((m) => m.role !== 'system' && m.content.toLowerCase().includes(q))
    .map((m) => m.localId)
})

const focusedMatchId = computed(() => chatSearchMatches.value[chatSearchCursor.value] || '')

function openChatSearch() {
  store.state.botView = 'stream'
  chatSearchOpen.value = true
  nextTick(() => {
    chatSearchBarRef.value?.focus()
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
  if (list.length > 0) {
    const latestIdx = list.length - 1
    chatSearchCursor.value = latestIdx
    nextTick(() => {
      const el = scrollContainerRef.value?.querySelector(`[data-msg-id="${CSS.escape(list[latestIdx])}"]`)
      el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    })
  } else {
    chatSearchCursor.value = 0
  }
})

function handleSelectedModelChange(nextModel: string) {
  selectedModel.value = nextModel
  const b = bot.value
  if (b) {
    b.model = nextModel
    void BotHubApi.updateBot(b.id, { model: nextModel }).catch(() => {})
  }
}

function handleEnableThinkingChange(nextEnabled: boolean) {
  enableThinking.value = nextEnabled
  const b = bot.value
  if (b) {
    b.enable_thinking = nextEnabled
    void BotHubApi.updateBot(b.id, { enable_thinking: nextEnabled }).catch(() => {})
  }
}

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

function runQuickToolStarter(toolId: string) {
  switch (toolId) {
    case 'workspace_briefing':
      void sendPrompt('请调用 workspace_briefing 为我梳理今日重点待办、日程与临期风险，并排出优先次序。')
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
      void sendPrompt('请调用 manage_assets(action="summary") 和 expiring 帮我盘点当前数字资产费用与到期风险。')
      break
    case 'query_nezha_nodes':
      void sendPrompt('请调用 query_nezha_nodes 帮我巡检全部节点实时在线与负载健康状态。')
      break
    case 'manage_tasks':
      void sendPrompt('请调用 manage_tasks 核查当前自动化调度与提醒队列状态。')
      break
    case 'recall_memory':
    case 'manage_memory':
      void sendPrompt('请检索当前作用域的长期记忆，确认你目前记得的核心习惯与约定。')
      break
    case 'web_search':
      inputPrompt.value = '请帮我联网检索：'
      nextTick(() => composerRef.value?.focus())
      break
    case 'manage_bots':
      inputPrompt.value = '请帮我创建一个专属 Bot，或把几位 Bot 拉进一个协同群聊：'
      nextTick(() => composerRef.value?.focus())
      break
    case 'manage_models':
      void sendPrompt('请调用 manage_models(action="status") 巡检当前免费模型池与付费模型池的健康度及近 7 日调用量。')
      break
    case 'manage_skills':
      inputPrompt.value = '请调用 manage_skills(action="upsert") 帮我创建一个自定义技能：'
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
  void sendPrompt('请承接你上一条答复继续深入展开可执行步骤、关键细节与具体产出物。')
}

watch(
  bot,
  (b) => {
    if (!b) return
    const normModel = !b.model || b.model === 'free-pool' ? 'auto:free' : b.model
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
const isStreamEmpty = computed(() => store.state.timeline.length === 0)

const starterChips = computed<StarterChipItem[]>(() => {
  const b = bot.value
  if (!b) return []
  const chips: StarterChipItem[] = []
  const todos = store.pendingTodoCount.value
  const expiring = store.expiringAssetCount.value
  if (b.bot_mode !== 'companion' && (todos > 0 || expiring > 0)) {
    const parts: string[] = []
    if (todos > 0) parts.push(`${todos} 项待办`)
    if (expiring > 0) parts.push(`${expiring} 项临期`)
    chips.push({
      id: 'workspace_briefing',
      label: `梳理今日安排 · ${parts.join(' / ')}`,
      highlight: true,
    })
  }
  const allowed = parseJsonStringArray(b.allowed_tools)
  for (const id of allowed) {
    if (chips.some((c) => c.id === id)) continue
    const found = (store.state.availableTools || []).find((t) => t.id === id)
    chips.push({
      id,
      label: shortToolLabel({ id, description: found?.description || id }),
    })
    if (chips.length >= 5) break
  }
  return chips
})

const groupMembers = computed<Bot[]>(() => {
  if (!isGroupBot.value || !bot.value) return []
  return resolveGroupMemberBots(bot.value, store.state.bots)
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

const scopedMemories = computed(() =>
  resolveScopedMemories(bot.value, store.state.botMemories || [], store.state.memories)
)

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

function handleInspectorQuote(text: string) {
  const clean = text.trim()
  if (!clean) return
  inputPrompt.value = inputPrompt.value.trim()
    ? `${inputPrompt.value.trimEnd()}\n\n${clean}`
    : clean
  store.state.botView = 'stream'
  nextTick(() => composerRef.value?.focus())
}

function handleMarkdownClick(e: MouseEvent) {
  handleMarkdownCodeCopy(e, () => store.notify('代码块已复制'))
}

async function copyMessageContent(text: string) {
  const ok = await copyToClipboard(text)
  store.notify(ok ? '已复制回复 Markdown 全文' : '复制失败')
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
  const ok = await copyToClipboard(stripMarkdownFormatting(text))
  store.notify(ok ? '已复制纯文本内容' : '复制失败')
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
    `# BotHub · ${bName}（${thTitle}）\n\n- 导出时间: ${new Date().toISOString()}\n`,
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
  const ok = await store.openConfirm({
    title: '清空对话时间线',
    message: `确定清空 [${bot.value.name}] 的对话时间线吗？`,
    confirmText: '确认清空',
    danger: true,
  })
  if (!ok) return
  await store.clearActiveBotTimeline()
}
</script>

<template>
  <section class="flex-1 min-h-0 flex flex-col">
    <BotWorkspaceHeader
      :bot="bot"
      :is-group-bot="isGroupBot"
      :group-members="groupMembers"
      :active-speaker-id="activeSpeakerBotId"
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
        <div class="vesper-card rounded-xl p-6 sm:p-8 flex flex-col gap-4 text-center items-center">
          <BotAvatar avatar-style="butler:gold" state="idle" size="lg" />
          <div>
            <h2 class="text-lg font-semibold text-fg">工作台准备就绪</h2>
            <p class="text-xs text-fg-muted mt-1.5 leading-relaxed max-w-md mx-auto">
              当前尚未添加 Bot 或协同群聊。你可以先在设置中配置模型并实时探测可用性，再按需创建专属 Bot。
            </p>
          </div>
          <div class="flex flex-wrap items-center justify-center gap-2.5 pt-1">
            <button
              type="button"
              @click="store.state.createModalMode = 'bot'; store.state.showCreateBotModal = true"
              class="min-h-[42px] px-4 py-2 rounded-xl bg-accent hover:bg-accent-hover text-on-accent font-semibold text-xs cursor-pointer"
            >
              + 新建 Bot
            </button>
            <button
              type="button"
              @click="store.state.showSettingsModal = true"
              class="min-h-[42px] px-4 py-2 rounded-xl border border-accent-border bg-accent-soft hover:bg-accent-soft text-xs font-medium text-accent-fg cursor-pointer"
            >
              配置模型 Key
            </button>
          </div>
        </div>
      </div>
    </div>

    <template v-else>
      <Transition :name="streamEnterTransition">
        <div
          v-show="store.state.botView === 'stream'"
          class="flex-1 min-h-0 flex min-w-0"
        >
          <div :class="['flex-1 min-w-0 min-h-0 flex flex-col', isStreamEmpty ? 'overflow-y-auto py-2 sm:py-4' : '']">
            <BotThreadBar v-if="bot" :bot="bot" @carry-summary="onCarryThreadSummary" />

          <!-- Slide-Down In-Chat Search Bar (Cmd/Ctrl + F) -->
          <BotChatSearchBar
            ref="chatSearchBarRef"
            :open="chatSearchOpen"
            v-model:query="chatSearchQuery"
            :match-count="chatSearchMatches.length"
            :cursor="chatSearchCursor"
            @jump="jumpSearchMatch"
            @close="closeChatSearch"
          />

          <!-- Pinned Group Announcement Banner -->
          <Transition
            enter-active-class="transition-[opacity,transform] duration-180 ease-[var(--ease-snappy)] motion-reduce:duration-100"
            enter-from-class="opacity-0 -translate-y-1.5 motion-reduce:translate-y-0"
            enter-to-class="opacity-100 translate-y-0"
            leave-active-class="transition-[opacity,transform] duration-130 ease-[var(--ease-in-snappy)] motion-reduce:duration-75"
            leave-from-class="opacity-100 translate-y-0"
            leave-to-class="opacity-0 -translate-y-1.5 motion-reduce:translate-y-0"
          >
            <div
              v-if="isGroupBot && bot?.announcement && !announcementDismissed && !isStreamEmpty"
              class="mx-4 md:mx-6 mb-1 px-3.5 py-1.5 rounded-xl border border-accent-border bg-accent-soft flex items-center justify-between gap-3 shrink-0"
            >
              <button
                type="button"
                @click="store.state.showGroupDrawer = true"
                class="flex items-center gap-2 min-w-0 text-left text-xs text-fg-sub hover:text-fg cursor-pointer"
              >
                <span class="px-1.5 py-0.5 rounded bg-accent-soft text-accent-fg font-mono text-xs font-semibold shrink-0">
                  群公告
                </span>
                <span class="truncate">{{ bot.announcement }}</span>
              </button>
              <button
                type="button"
                aria-label="暂时收起群公告"
                @click="announcementDismissed = true"
                class="w-6 h-6 rounded-full inline-flex items-center justify-center text-fg-muted hover:text-fg cursor-pointer shrink-0"
              >
                <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </svg>
              </button>
            </div>
          </Transition>

          <div :class="['relative flex flex-col', isStreamEmpty ? 'shrink-0 mt-auto' : 'flex-1 min-h-0']">
            <div
              ref="scrollContainerRef"
              @scroll="onMessagesScroll"
              @click="handleMarkdownClick"
              :class="[
                'cq-container stream-scroller px-4 md:px-6 flex flex-col gap-6',
                isStreamEmpty ? 'shrink-0 py-1' : 'stream-edge-fade flex-1 min-h-0 overflow-y-auto py-5',
              ]"
            >
              <BotStarterPanel
                v-if="isStreamEmpty"
                :bot="bot"
                :is-group-bot="isGroupBot"
                :group-members="groupMembers"
              />

              <BotMessageItem
                v-for="(m, idx) in store.state.timeline"
                :key="m.localId"
                :message="m"
                :show-timestamp="shouldShowTimestamp(idx)"
                :deferred="shouldDeferMessageRender(idx)"
                :focused-match="focusedMatchId === m.localId"
                :is-group-bot="isGroupBot"
                :active-bot-id="bot?.id"
                :is-last="idx === store.state.timeline.length - 1"
                :can-retry="Boolean(findPrecedingUserPrompt(idx))"
                @quote="setReplyQuote"
                @edit-resend="editResendUserMessage"
                @copy-markdown="copyMessageContent"
                @copy-plain="copyPlainTextContent"
                @fork="store.forkThread($event)"
                @delete="store.deleteSingleMessage($event)"
                @mention="appendMention"
                @jump-bot="store.jumpToBot($event)"
                @continue="continueFromMessage"
                @retry="sendPrompt(findPrecedingUserPrompt(idx), { replaceAssistantMsg: m })"
                @retry-model="(mid, paidOnce) => sendPrompt(findPrecedingUserPrompt(idx), { overrideModel: mid, allowPaidOnce: paidOnce, replaceAssistantMsg: m })"
                @save-memory="saveMessageAsMemory"
                @convert-todo="convertMessageToTodo"
                @save-note="saveMessageAsNote"
                @forward="openForwardForMessage"
              />
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
                v-if="userScrolledUp && !isStreamEmpty"
                type="button"
                @click="scrollToBottom(true)"
                class="btn-tactile-gold absolute bottom-3 left-1/2 -translate-x-1/2 z-10 inline-flex items-center gap-1.5 min-h-[36px] px-4 rounded-full text-xs font-medium shadow-xl cursor-pointer"
              >
                <span
                  v-if="store.state.isStreaming || unreadBelowCount > 0"
                  :class="['w-1.5 h-1.5 rounded-full shrink-0', store.state.isStreaming ? 'bg-accent animate-pulse' : 'bg-ok']"
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

          <!-- Unified Spotlight / Bottom-Docked Composer -->
          <BotChatComposer
            ref="composerRef"
            :class="isStreamEmpty ? 'mb-auto' : ''"
            v-model:input-prompt="inputPrompt"
            :selected-model="selectedModel"
            :enable-thinking="enableThinking"
            v-model:reply-target="replyTarget"
            :stream-status="streamStatus"
            :relay-steps="relaySteps"
            :is-group-bot="isGroupBot"
            :group-members="groupMembers"
            :last-prompt="lastPrompt"
            :centered-stage="isStreamEmpty"
            :starter-chips="starterChips"
            @update:selected-model="handleSelectedModelChange"
            @update:enable-thinking="handleEnableThinkingChange"
            @submit-prompt="sendPrompt"
            @stop-stream="stopStream"
            @clear-timeline="clearTimeline"
            @summarize-thread="handleSummarizeThread"
            @pick-starter="runQuickToolStarter"
          />
          </div>

          <BotSideInspector
            v-if="store.state.sideInspectorOpen"
            @quote-to-chat="handleInspectorQuote"
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
