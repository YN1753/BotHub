<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore, type ChatMessageUI } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { botStateLabel, describeCron, formatTime } from '../utils/format'
import { renderSafeMarkdown } from '../utils/markdown'
import BotAvatar from './BotAvatar.vue'
import CustomSelect from './CustomSelect.vue'

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
const selectedModel = ref('deepseek-flash')
const enableThinking = ref(true)
const streamStatus = ref('就绪')
const scrollContainerRef = ref<HTMLElement | null>(null)
const textareaRef = ref<HTMLTextAreaElement | null>(null)
const userScrolledUp = ref(false)
const moreMenuOpen = ref(false)
const moreMenuRef = ref<HTMLElement | null>(null)

let abortCtrl: AbortController | null = null
let lastPrompt = ''

// Config Editor local state
const cfgName = ref('')
const cfgRole = ref('')
const cfgPrompt = ref('')
const cfgModel = ref('deepseek-flash')
const cfgAvatar = ref('cyan')
const cfgThinking = ref(true)
const cfgTools = ref<string[]>([])
const savingProfile = ref(false)

const TOOL_CATALOG = [
  { id: 'read_url', label: 'read_url (网页抓取与摘要)' },
  { id: 'web_search', label: 'web_search (开源雷达搜索)' },
  { id: 'query_nezha_nodes', label: 'query_nezha_nodes (哪吒探针直连)' },
  { id: 'manage_assets', label: 'manage_assets (数字资产台账)' },
  { id: 'manage_tasks', label: 'manage_tasks (调度例程管控)' },
  { id: 'remember_fact', label: 'remember_fact (写入长期记忆)' },
  { id: 'recall_memory', label: 'recall_memory (检索长期记忆)' },
]

const MODEL_OPTIONS = [
  { value: 'deepseek-flash', label: 'DeepSeek Flash (主力)' },
  { value: 'deepseek-v4-pro', label: 'DeepSeek V4 Pro (深度)' },
]

const AVATAR_OPTIONS = [
  { value: 'cyan', label: '暮光暖金 (Vesper Gold)' },
  { value: 'indigo', label: '深空靛蓝 (Deep Indigo)' },
  { value: 'amber', label: '炽焰琥珀 (Warm Amber)' },
  { value: 'emerald', label: '星野翠绿 (Emerald)' },
  { value: 'violet', label: '星云暮紫 (Nebula Violet)' },
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
    try {
      const arr = JSON.parse(b.allowed_tools || '[]')
      cfgTools.value = Array.isArray(arr) ? arr : []
    } catch {
      cfgTools.value = []
    }
  },
  { immediate: true }
)

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

function autoResizeTextarea() {
  const el = textareaRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 160)}px`
}

watch(inputPrompt, (val) => {
  if (!val) {
    nextTick(() => {
      if (textareaRef.value) textareaRef.value.style.height = ''
    })
  }
})

watch(
  () => store.state.activeBotId,
  () => {
    userScrolledUp.value = false
    scrollToBottom(true)
  }
)

watch(
  () => store.state.timeline.length,
  () => scrollToBottom(false)
)

function handleDocClick(e: MouseEvent) {
  if (!moreMenuOpen.value) return
  if (moreMenuRef.value && !moreMenuRef.value.contains(e.target as Node)) {
    moreMenuOpen.value = false
  }
}

onMounted(() => document.addEventListener('click', handleDocClick))
onBeforeUnmount(() => document.removeEventListener('click', handleDocClick))

function appendMention(tag: string) {
  const prefix = inputPrompt.value && !inputPrompt.value.endsWith(' ') ? ' ' : ''
  inputPrompt.value = `${inputPrompt.value}${prefix}${tag}`
  nextTick(() => autoResizeTextarea())
}

function handleMarkdownClick(e: MouseEvent) {
  const target = e.target as HTMLElement
  const btn = target.closest('.md-copy-code-btn') as HTMLButtonElement | null
  if (!btn) return
  const code = btn.closest('.md-code-wrap')?.querySelector('pre code')?.textContent || ''
  navigator.clipboard?.writeText(code).then(() => {
    btn.textContent = '已复制'
    window.setTimeout(() => {
      btn.textContent = '复制代码'
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
  lastPrompt = msg
  if (!overrideText) inputPrompt.value = ''

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
    statusText: '已发送',
  })

  const createTurn = (bId: string, bName: string, style: string): ChatMessageUI => {
    const turn: ChatMessageUI = {
      localId: `a_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`,
      role: 'assistant',
      senderBotId: bId,
      senderName: bName,
      avatarStyle: style,
      model: selectedModel.value,
      msgKind: 'chat',
      createdAt: new Date().toISOString(),
      content: '',
      reasoning: '',
      tools: [],
      statusText: '思考与生成中...',
      streaming: true,
    }
    store.state.timeline.push(turn)
    return store.state.timeline[store.state.timeline.length - 1]
  }

  let activeTurn = createTurn(botId, curBot?.name || 'BotHub', curBot?.avatar_style || 'cyan')
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

          if (currentEvent === 'bot_turn') {
            if (Number(payload.step) > 1) {
              activeTurn.statusText = '已完成'
              activeTurn.streaming = false
              activeTurn = createTurn(
                payload.sender_bot_id,
                payload.sender_name,
                payload.avatar_style || 'cyan'
              )
            } else {
              activeTurn.senderBotId = payload.sender_bot_id
              activeTurn.senderName = payload.sender_name
              activeTurn.avatarStyle = payload.avatar_style || 'cyan'
            }
            streamStatus.value = `群聊接力 (${payload.step}/${payload.total_steps}): ${payload.sender_name}`
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
          } else if (currentEvent === 'error') {
            activeTurn.statusText = '异常中断'
            activeTurn.errorText = payload.error || '未知流式错误'
            activeTurn.streaming = false
          } else if (currentEvent === 'done') {
            activeTurn.streaming = false
            activeTurn.statusText =
              payload.usage && payload.usage.total_tokens
                ? `已完成 (${payload.usage.prompt_tokens}+${payload.usage.completion_tokens} Tokens)`
                : '已完成'
          }
        }
      }
    }

    activeTurn.streaming = false
    if (activeTurn.statusText === '思考与生成中...') activeTurn.statusText = '已完成'
    streamStatus.value = '就绪'
    await store.loadDashboard(false)
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
    <!-- Active Bot Top Header -->
    <header class="px-3.5 sm:px-6 py-3 border-b border-[#fff8ee]/10 bg-[#080c18]/55 flex flex-wrap items-center justify-between gap-2.5">
      <div class="flex items-center gap-2.5 min-w-0">
        <button
          type="button"
          aria-label="打开菜单"
          class="md:hidden min-h-[40px] min-w-[40px] inline-flex items-center justify-center rounded-xl border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer transition"
          @click="emit('openMobileMenu')"
        >
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="6" x2="21" y2="6" /><line x1="3" y1="12" x2="21" y2="12" /><line x1="3" y1="18" x2="21" y2="18" /></svg>
        </button>
        <button
          v-if="desktopCollapsed"
          type="button"
          aria-label="展开侧边栏"
          title="展开侧边栏"
          class="hidden md:inline-flex items-center justify-center w-9 h-9 rounded-xl border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer transition"
          @click="emit('toggleDesktopSidebar')"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="3" width="18" height="18" rx="2" />
            <line x1="9" y1="3" x2="9" y2="21" />
            <polyline points="13 9 16 12 13 15" />
          </svg>
        </button>
        <BotAvatar :avatar-style="bot?.avatar_style" :state="bot?.state" size="lg" />
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <strong class="text-sm md:text-base font-semibold text-white truncate">{{ bot?.name || '长庚执更' }}</strong>
            <span :class="['font-mono text-[10.5px] px-2 py-0.5 rounded-full border leading-none shrink-0', stateBadgeClass(bot?.state)]">
              {{ botStateLabel(bot?.state) }}
            </span>
          </div>
          <p class="text-xs text-slate-400 truncate">{{ bot?.role_title }}</p>
        </div>
      </div>

      <div class="flex items-center gap-2 max-w-full ml-auto">
        <!-- Segmented Control Capsule -->
        <div class="inline-flex items-center p-1 rounded-full bg-[#060913]/90 border border-[#fff8ee]/11 overflow-x-auto no-scrollbar max-w-full">
          <button
            v-for="tab in [
              { id: 'stream', label: '对话流' },
              { id: 'routines', label: '例程' },
              { id: 'config', label: '能力与记忆' },
              { id: 'activity', label: '运行脉搏' },
            ]"
            :key="tab.id"
            type="button"
            @click="store.state.botView = tab.id as any"
            :class="[
              'min-h-[34px] px-3.5 rounded-full text-xs transition cursor-pointer whitespace-nowrap shrink-0',
              store.state.botView === tab.id
                ? 'bg-[#f2b880] text-[#070a13] font-semibold shadow-sm'
                : 'text-slate-300 hover:text-white'
            ]"
          >
            {{ tab.label }}
          </button>
        </div>

        <!-- Collapsed More Actions Dropdown -->
        <div ref="moreMenuRef" class="relative shrink-0">
          <button
            type="button"
            aria-label="更多操作"
            title="更多操作"
            @click.stop="moreMenuOpen = !moreMenuOpen"
            class="min-h-[40px] min-w-[40px] sm:min-h-[36px] sm:min-w-[36px] inline-flex items-center justify-center rounded-full border border-[#fff8ee]/12 bg-[#060913]/80 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 transition cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
              <circle cx="5" cy="12" r="1.75" />
              <circle cx="12" cy="12" r="1.75" />
              <circle cx="19" cy="12" r="1.75" />
            </svg>
          </button>

          <Transition
            enter-active-class="transition duration-150 ease-out"
            enter-from-class="opacity-0 -translate-y-1 scale-95"
            enter-to-class="opacity-100 translate-y-0 scale-100"
            leave-active-class="transition duration-100 ease-in"
            leave-from-class="opacity-100"
            leave-to-class="opacity-0"
          >
            <div
              v-if="moreMenuOpen"
              class="absolute right-0 mt-2 w-44 rounded-xl border border-[#fff8ee]/12 bg-[#0a0f1f]/98 backdrop-blur-xl shadow-2xl p-1.5 z-50 flex flex-col gap-0.5"
            >
              <button
                type="button"
                @click="store.loadDashboard(false); moreMenuOpen = false"
                class="min-h-[40px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/12 hover:text-[#f2b880] transition cursor-pointer text-left"
              >
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8"><polyline points="23 4 23 10 17 10" /><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" /></svg>
                <span>刷新状态</span>
              </button>
              <button
                type="button"
                @click="exportTimelineMarkdown(); moreMenuOpen = false"
                class="min-h-[40px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/12 hover:text-[#f2b880] transition cursor-pointer text-left"
              >
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="7 10 12 15 17 10" /><line x1="12" y1="15" x2="12" y2="3" /></svg>
                <span>导出 Markdown</span>
              </button>
              <div class="my-1 border-t border-[#fff8ee]/10" />
              <button
                type="button"
                @click="clearTimeline(); moreMenuOpen = false"
                class="min-h-[40px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-rose-300 hover:bg-rose-500/15 transition cursor-pointer text-left"
              >
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /></svg>
                <span>清空对话流</span>
              </button>
            </div>
          </Transition>
        </div>
      </div>
    </header>

    <!-- View 1: Persistent Stream -->
    <div v-show="store.state.botView === 'stream'" class="flex-1 min-h-0 flex flex-col">
      <div class="relative flex-1 min-h-0 flex flex-col">
        <div
          ref="scrollContainerRef"
          @scroll="onMessagesScroll"
          @click="handleMarkdownClick"
          class="flex-1 min-h-0 overflow-y-auto px-4 md:px-6 py-4 flex flex-col gap-5"
        >
          <!-- Empty State Hero -->
          <div
            v-if="store.state.timeline.length === 0"
            class="m-auto max-w-xl text-center py-8 px-4 flex flex-col items-center gap-3"
          >
            <span class="px-2.5 py-1 rounded-full border border-[#f2b880]/30 bg-[#f2b880]/10 text-[#f2b880] font-mono text-[11px]">
              PERSISTENT GROKBOT
            </span>
            <h2 class="text-2xl font-semibold text-white">{{ bot?.name }} 随时待命</h2>
            <p class="text-xs sm:text-sm text-slate-400">
              {{ bot?.role_title }} · 后台例程产出与告警均会主动投递至此时间线
            </p>
            <div class="flex flex-wrap justify-center gap-2 mt-2">
              <button
                type="button"
                @click="sendPrompt('请调用工具盘点我当前的全部数字资产与算力节点，并按到期风险给出锐评。')"
                class="min-h-[40px] sm:min-h-[34px] px-3.5 py-1.5 rounded-full text-xs border border-[#fff8ee]/14 bg-[#fff8ee]/[0.04] hover:border-[#f2b880]/50 hover:text-[#f2b880] text-slate-200 cursor-pointer transition"
              >
                盘点全部资产与到期风险
              </button>
              <button
                type="button"
                @click="sendPrompt('请调用 query_nezha_nodes 检查哪吒监控面板节点的在线状态与配置详情。')"
                class="min-h-[40px] sm:min-h-[34px] px-3.5 py-1.5 rounded-full text-xs border border-[#fff8ee]/14 bg-[#fff8ee]/[0.04] hover:border-[#f2b880]/50 hover:text-[#f2b880] text-slate-200 cursor-pointer transition"
              >
                探查哪吒节点脉搏
              </button>
              <button
                type="button"
                @click="sendPrompt('请查看当前所有自动化调度任务的运行状态，并告诉我下次触发时间。')"
                class="min-h-[40px] sm:min-h-[34px] px-3.5 py-1.5 rounded-full text-xs border border-[#fff8ee]/14 bg-[#fff8ee]/[0.04] hover:border-[#f2b880]/50 hover:text-[#f2b880] text-slate-200 cursor-pointer transition"
              >
                检查调度任务状态
              </button>
            </div>
          </div>

          <!-- Messages -->
          <template v-for="m in store.state.timeline" :key="m.localId">
            <!-- User Message Pill -->
            <div v-if="m.role === 'user'" class="max-w-[820px] w-full mx-auto flex justify-end">
              <div
                :title="formatTime(m.createdAt)"
                class="max-w-[82%] sm:max-w-[74%] rounded-2xl px-4 py-2.5 bg-[#f2b880]/14 border border-[#f2b880]/28 text-sm text-slate-100 leading-relaxed whitespace-pre-wrap"
              >
                {{ m.content }}
              </div>
            </div>

            <!-- Bot Message (Chat or Proactive Routine Report) -->
            <div v-else class="max-w-[820px] w-full mx-auto">
              <div
                :class="[
                  'flex flex-col gap-2.5',
                  m.msgKind === 'routine'
                    ? 'rounded-2xl border border-[#f2b880]/30 bg-[#0d1222]/80 p-4'
                    : 'py-1'
                ]"
              >
                <div class="flex items-center justify-between gap-2 font-mono text-[11px] text-slate-400">
                  <div class="flex items-center gap-2 flex-wrap">
                    <BotAvatar :avatar-style="m.avatarStyle" :state="m.streaming ? 'working' : (m.msgKind === 'routine' ? 'done' : 'idle')" size="sm" />
                    <span class="text-[#f2b880] font-semibold">{{ m.senderName }}</span>
                    <span>· {{ m.model }}</span>
                    <span
                      v-if="m.msgKind === 'routine'"
                      class="px-2 py-0.5 rounded-md border border-[#f2b880]/35 bg-[#f2b880]/15 text-[#f2b880] text-[10px]"
                    >
                      例程主动汇报
                    </span>
                  </div>
                  <span :class="m.errorText ? 'text-rose-400' : ''">{{ m.statusText }}</span>
                </div>

                <!-- Collapsible Reasoning -->
                <details
                  v-if="m.reasoning"
                  :open="m.streaming && !m.content"
                  class="rounded-xl border border-amber-400/25 bg-[#060a16]/75 px-3 py-2 text-xs text-slate-400"
                >
                  <summary class="cursor-pointer font-mono text-amber-300/90 select-none">
                    深度思考过程 (Reasoning Chain)
                  </summary>
                  <div class="mt-2 font-mono whitespace-pre-wrap leading-relaxed text-slate-300/90">
                    {{ m.reasoning }}
                  </div>
                </details>

                <!-- Merged Tool Call Trajectory Summary Bar -->
                <details
                  v-if="m.tools.length"
                  class="rounded-xl border border-[#fff8ee]/11 bg-[#070a16]/80 px-3 py-2 font-mono text-[11.5px]"
                >
                  <summary class="cursor-pointer flex items-center justify-between gap-2 text-slate-300 select-none">
                    <span class="inline-flex items-center gap-2 min-w-0">
                      <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" class="text-[#f2b880] shrink-0">
                        <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
                      </svg>
                      <span class="truncate">工具执行轨迹 · 共 {{ m.tools.length }} 步</span>
                    </span>
                    <span :class="m.tools.some((t) => !t.done) ? 'text-[#f2b880] shrink-0' : 'text-emerald-400 shrink-0'">
                      {{ m.tools.some((t) => !t.done) ? '执行中' : '全部完成' }}
                    </span>
                  </summary>
                  <div class="mt-2 pt-2 border-t border-[#fff8ee]/10 flex flex-col gap-2">
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

                <!-- Message Footer Actions -->
                <div v-if="!m.streaming && m.content" class="flex items-center gap-2 pt-1.5 border-t border-[#fff8ee]/8">
                  <button
                    type="button"
                    @click="copyMessageContent(m.content)"
                    class="min-h-[36px] sm:min-h-[28px] px-2.5 py-1 rounded-md border border-[#fff8ee]/12 text-[11px] text-slate-400 hover:text-[#f2b880] hover:border-[#f2b880]/40 transition cursor-pointer"
                  >
                    复制全文
                  </button>
                  <button
                    v-if="lastPrompt"
                    type="button"
                    @click="sendPrompt(lastPrompt)"
                    class="min-h-[36px] sm:min-h-[28px] px-2.5 py-1 rounded-md border border-[#fff8ee]/12 text-[11px] text-slate-400 hover:text-[#f2b880] hover:border-[#f2b880]/40 transition cursor-pointer"
                  >
                    重新生成
                  </button>
                </div>
              </div>
            </div>
          </template>
        </div>

        <!-- Floating Scroll-to-Bottom Button -->
        <Transition
          enter-active-class="transition duration-150 ease-out"
          enter-from-class="opacity-0 translate-y-2"
          enter-to-class="opacity-100 translate-y-0"
          leave-active-class="transition duration-150 ease-in"
          leave-from-class="opacity-100"
          leave-to-class="opacity-0 translate-y-2"
        >
          <button
            v-if="userScrolledUp"
            type="button"
            @click="scrollToBottom(true)"
            class="absolute bottom-3 left-1/2 -translate-x-1/2 z-10 inline-flex items-center gap-1.5 min-h-[36px] px-3.5 rounded-full border border-[#f2b880]/45 bg-[#0a0f1f]/95 text-xs font-medium text-[#f2b880] shadow-xl backdrop-blur-md hover:bg-[#f2b880]/15 transition cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9" /></svg>
            <span>回到底部</span>
          </button>
        </Transition>
      </div>

      <!-- Floating Composer Dock -->
      <div class="px-3 sm:px-6 pt-1 pb-[max(0.75rem,env(safe-area-inset-bottom))] sm:pb-5">
        <div
          v-if="bot?.id === 'team_room'"
          class="max-w-[820px] w-full mx-auto mb-2 flex items-center gap-1.5 flex-wrap"
        >
          <span class="font-mono text-[11px] text-slate-400">@点名协同:</span>
          <button
            v-for="tag in ['@探针哨兵 ', '@长庚执更 ', '@开源雷达 ', '@Grok 锐评官 ']"
            :key="tag"
            type="button"
            @click="appendMention(tag)"
            class="min-h-[34px] sm:min-h-[28px] px-2.5 py-1 rounded-full text-xs border border-[#fff8ee]/14 bg-[#fff8ee]/[0.04] hover:border-[#f2b880]/50 hover:text-[#f2b880] text-slate-200 cursor-pointer transition"
          >
            {{ tag.trim() }}
          </button>
        </div>

        <form
          @submit.prevent="sendPrompt()"
          class="max-w-[820px] w-full mx-auto rounded-[22px] border border-[#fff8ee]/14 focus-within:border-[#f2b880]/55 bg-[#0b101e]/92 shadow-2xl px-3.5 sm:px-4 pt-3 pb-2.5 flex flex-col gap-2.5 transition-colors"
        >
          <textarea
            ref="textareaRef"
            v-model="inputPrompt"
            rows="1"
            placeholder="向当前专属 Bot 下达指令（Enter 发送，Shift+Enter 换行）..."
            @input="autoResizeTextarea"
            @keydown.enter.exact.prevent="sendPrompt()"
            class="w-full bg-transparent border-0 text-sm text-slate-100 placeholder-slate-500 resize-none focus:outline-none min-h-[24px] max-h-[160px] overflow-y-auto leading-relaxed"
          />
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2 min-w-0 flex-1 flex-wrap sm:flex-nowrap">
              <div class="w-36 sm:w-44 shrink-0">
                <CustomSelect v-model="selectedModel" :options="MODEL_OPTIONS" compact />
              </div>
              <button
                type="button"
                @click="enableThinking = !enableThinking"
                :class="[
                  'min-h-[40px] sm:min-h-[32px] px-3 rounded-full text-xs border transition cursor-pointer whitespace-nowrap shrink-0',
                  enableThinking
                    ? 'border-[#f2b880]/50 bg-[#f2b880]/15 text-[#f2b880]'
                    : 'border-[#fff8ee]/14 text-slate-400 hover:text-white'
                ]"
              >
                深度思考: {{ enableThinking ? '开' : '关' }}
              </button>
              <span class="hidden sm:inline font-mono text-[11px] text-slate-400 truncate">{{ streamStatus }}</span>
            </div>

            <div class="flex items-center gap-2 shrink-0">
              <button
                v-if="store.state.isStreaming"
                type="button"
                @click="stopStream"
                class="min-h-[40px] sm:min-h-[34px] px-3.5 rounded-full text-xs bg-rose-500/20 border border-rose-400/45 text-rose-300 hover:bg-rose-500/30 cursor-pointer"
              >
                停止
              </button>
              <button
                type="submit"
                :disabled="store.state.isStreaming || !inputPrompt.trim()"
                title="发送 (Enter)"
                class="w-10 h-10 rounded-full bg-[#f2b880] hover:bg-[#f7cba0] hover:scale-105 text-[#070a13] flex items-center justify-center transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
              >
                <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.4"><line x1="12" y1="19" x2="12" y2="5" /><polyline points="5 12 12 5 19 12" /></svg>
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>

    <!-- View 2: Bot Routines -->
    <div v-show="store.state.botView === 'routines'" class="flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
      <div class="max-w-4xl mx-auto flex flex-col gap-4">
        <div class="flex items-center justify-between gap-3 flex-wrap">
          <div>
            <h3 class="text-lg font-semibold text-white">该 Bot 专属定时例程 (Routines)</h3>
            <p class="text-xs text-slate-400">例程触发后将由当前 Bot 自主运行并将结果主动汇报进对话流</p>
          </div>
          <button
            type="button"
            @click="store.state.editingTask = { bot_id: bot?.id || 'butler' }"
            class="min-h-[40px] sm:min-h-[34px] px-3.5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer transition"
          >
            + 为该 Bot 添加例程
          </button>
        </div>

        <div v-if="botRoutines.length === 0" class="aero-glass rounded-2xl p-8 text-center text-sm text-slate-400">
          当前 Bot 尚未绑定自动化例程，可点击右上角新增
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <article
            v-for="t in botRoutines"
            :key="t.id"
            class="aero-glass rounded-2xl p-4 flex flex-col gap-2.5"
          >
            <div class="flex items-start justify-between gap-2">
              <div>
                <h4 class="font-semibold text-sm text-white">{{ t.name }}</h4>
                <div class="font-mono text-[11px] text-slate-400 mt-0.5">{{ describeCron(t.cron_expr) }}</div>
              </div>
              <span
                :class="[
                  'font-mono text-[11px] px-2 py-0.5 rounded-full border',
                  t.enabled ? 'border-emerald-400/35 bg-emerald-400/10 text-emerald-300' : 'border-[#fff8ee]/15 text-slate-400'
                ]"
              >
                {{ t.enabled ? '运行中' : '已暂停' }}
              </span>
            </div>
            <p class="text-xs text-slate-300">{{ t.description }}</p>
            <div class="font-mono text-[11px] text-slate-400">最近结果: {{ t.last_run?.summary || '等待首次执行' }}</div>
            <div class="flex items-center gap-2 pt-1">
              <button
                type="button"
                @click="runRoutine(t.id)"
                class="min-h-[40px] sm:min-h-[32px] px-3 py-1.5 rounded-lg bg-[#f2b880]/15 border border-[#f2b880]/40 text-[#f2b880] hover:bg-[#f2b880]/25 text-xs cursor-pointer transition"
              >
                立即执行一次
              </button>
              <button
                type="button"
                @click="store.state.editingTask = t"
                class="min-h-[40px] sm:min-h-[32px] px-3 py-1.5 rounded-lg border border-[#fff8ee]/14 text-slate-300 hover:bg-white/5 text-xs cursor-pointer transition"
              >
                配置参数
              </button>
            </div>
          </article>
        </div>
      </div>
    </div>

    <!-- View 3: Bot Profile, Tools & Scoped Memories -->
    <div v-show="store.state.botView === 'config'" class="flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
      <div class="max-w-4xl mx-auto flex flex-col gap-5">
        <form @submit.prevent="saveBotProfile" class="aero-glass rounded-2xl p-5 flex flex-col gap-4">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <label class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>Bot 名称</span>
              <input v-model="cfgName" type="text" required class="rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
            </label>
            <label class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>职责定位</span>
              <input v-model="cfgRole" type="text" required class="rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
            </label>
            <div class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>默认推理模型</span>
              <CustomSelect v-model="cfgModel" :options="MODEL_OPTIONS" />
            </div>
            <div class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>头像主题色</span>
              <CustomSelect v-model="cfgAvatar" :options="AVATAR_OPTIONS" />
            </div>
            <label class="md:col-span-2 flex flex-col gap-1.5 text-xs text-slate-300">
              <span>专属人设与行为约束 (System Prompt)</span>
              <textarea v-model="cfgPrompt" rows="4" class="rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
            </label>
            <div class="md:col-span-2 flex flex-col gap-1.5 text-xs text-slate-300">
              <span>绑定可用工具集 (最小权限白名单)</span>
              <div class="flex flex-wrap gap-3 p-3 rounded-xl border border-[#fff8ee]/11 bg-[#060a16]/60">
                <label
                  v-for="tool in TOOL_CATALOG"
                  :key="tool.id"
                  class="inline-flex items-center gap-2 font-mono text-xs text-slate-300 cursor-pointer"
                >
                  <input v-model="cfgTools" type="checkbox" :value="tool.id" class="accent-[#f2b880]" />
                  <span>{{ tool.label }}</span>
                </label>
              </div>
            </div>
          </div>

          <div class="flex items-center justify-between gap-3 flex-wrap pt-2 border-t border-[#fff8ee]/10">
            <button
              v-if="bot && !bot.is_builtin"
              type="button"
              @click="deleteCustomBot"
              class="min-h-[40px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-rose-400/40 bg-rose-500/15 text-rose-300 hover:bg-rose-500/25 text-xs cursor-pointer"
            >
              注销此自定义 Bot
            </button>
            <span v-else />
            <div class="flex items-center gap-4">
              <label class="inline-flex items-center gap-2 text-xs text-slate-300 cursor-pointer">
                <input v-model="cfgThinking" type="checkbox" class="accent-[#f2b880]" />
                <span>默认开启深度思考</span>
              </label>
              <button
                type="submit"
                :disabled="savingProfile"
                class="min-h-[40px] sm:min-h-[34px] px-4 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50 transition"
              >
                {{ savingProfile ? '保存中...' : '保存 Bot 人设与能力' }}
              </button>
            </div>
          </div>
        </form>

        <div class="aero-glass rounded-2xl p-5">
          <h4 class="font-semibold text-sm text-white mb-3">关联记忆背景 (全局 + 专属域)</h4>
          <div v-if="scopedMemories.length === 0" class="text-xs text-slate-500">暂无关联长期记忆</div>
          <div class="flex flex-col gap-2">
            <article
              v-for="mem in scopedMemories"
              :key="mem.id"
              class="rounded-xl border border-[#fff8ee]/11 bg-[#070a16]/75 px-3.5 py-2.5"
            >
              <div class="flex items-center justify-between gap-2">
                <strong class="font-mono text-xs text-[#f2b880]">{{ mem.fact_key }}</strong>
                <span class="font-mono text-[11px] text-slate-500">{{ mem.scope || 'global' }} · {{ mem.category }}</span>
              </div>
              <p class="text-xs text-slate-200 mt-1">{{ mem.content }}</p>
            </article>
          </div>
        </div>
      </div>
    </div>

    <!-- View 4: Activity Pulse -->
    <div v-show="store.state.botView === 'activity'" class="flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
      <div class="max-w-4xl mx-auto flex flex-col gap-3">
        <h3 class="text-lg font-semibold text-white">运行脉搏与最近汇报摘要</h3>
        <div v-if="activityEntries.length === 0" class="aero-glass rounded-2xl p-8 text-center text-sm text-slate-400">
          暂无后台运行脉搏记录
        </div>
        <article
          v-for="(act, idx) in activityEntries"
          :key="idx"
          class="aero-glass rounded-xl px-4 py-3"
        >
          <div class="flex items-center justify-between gap-2">
            <strong class="text-sm text-white">{{ act.title }}</strong>
            <span class="font-mono text-[11px] text-slate-400">{{ act.time }}</span>
          </div>
          <p class="text-xs text-slate-300 mt-1">{{ act.detail }}</p>
        </article>
      </div>
    </div>
  </section>
</template>
