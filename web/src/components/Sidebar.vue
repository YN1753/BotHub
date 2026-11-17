<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { botStateLabel, formatChatTime, shortVersion } from '../utils/format'
import { applySiblingStaggerFallback } from '../utils/topLayer'
import type { Bot } from '../types/bothub'
import BotAvatar from './BotAvatar.vue'
import BotGroupAvatar from './BotGroupAvatar.vue'

defineProps<{
  mobileOpen: boolean
  desktopCollapsed: boolean
}>()

const emit = defineEmits<{
  closeMobile: []
  toggleDesktopCollapse: []
}>()

const store = useAppStore()
const searchQuery = ref('')
const rosterScope = ref<'all' | 'groups' | 'companion' | 'worker'>('all')
const rosterRef = ref<HTMLElement | null>(null)

const scopeCounts = computed(() => {
  const bots = store.state.bots
  const groups = bots.filter((b) => isGroup(b)).length
  const companion = bots.filter((b) => !isGroup(b) && b.bot_mode === 'companion').length
  const worker = bots.filter((b) => !isGroup(b) && b.bot_mode !== 'companion').length
  return { all: bots.length, groups, companion, worker }
})

const unreadBotTarget = computed(() =>
  store.state.bots.find((b) => (b.unread_count || 0) > 0)
)
const totalUnreadMessages = computed(() =>
  store.state.bots.reduce((sum, b) => sum + (b.unread_count || 0), 0)
)

const filteredBots = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return store.state.bots.filter((b) => {
    if (rosterScope.value === 'groups' && !isGroup(b)) return false
    if (rosterScope.value === 'companion' && (isGroup(b) || b.bot_mode !== 'companion')) return false
    if (rosterScope.value === 'worker' && (isGroup(b) || (b.bot_mode !== 'worker' && b.bot_mode !== 'hybrid' && Boolean(b.bot_mode)))) return false
    if (!q) return true
    return `${b.name} ${b.role_title} ${b.traits || ''} ${b.last_summary}`.toLowerCase().includes(q)
  })
})

const specialistMap = computed(() => {
  const m = new Map<string, Bot>()
  store.state.bots.forEach((b) => {
    if (!b.is_group) m.set(b.id, b)
  })
  return m
})

function resolveGroupMembers(b: Bot): Bot[] {
  try {
    const ids = JSON.parse(b.member_ids || '[]')
    if (Array.isArray(ids)) {
      const resolved = ids.map((id: string) => specialistMap.value.get(id)).filter((x): x is Bot => Boolean(x))
      if (resolved.length > 0) return resolved
    }
  } catch {
    // ignore
  }
  return Array.from(specialistMap.value.values()).slice(0, 4)
}

function isGroup(b: Bot): boolean {
  return Boolean(b.is_group)
}

function botDraftPreview(b: Bot): string {
  const activeKey = b.active_thread_id ? `${b.id}::${b.active_thread_id}` : b.id
  if (store.state.draftsByBot[activeKey]) return store.state.draftsByBot[activeKey]
  if (store.state.draftsByBot[b.id]) return store.state.draftsByBot[b.id]
  const prefix = `${b.id}::`
  for (const [k, v] of Object.entries(store.state.draftsByBot)) {
    if (k.startsWith(prefix) && v) return v
  }
  return ''
}

function stateDotClass(state: string): string {
  if (state === 'thinking') return 'bg-amber-400 animate-pulse'
  if (state === 'working') return 'bg-[#f2b880] animate-pulse'
  if (state === 'done') return 'bg-emerald-400'
  if (state === 'blocked') return 'bg-rose-400'
  return 'bg-slate-500'
}

async function handlePickBot(id: string) {
  if (store.state.isStreaming) return
  await store.selectBot(id)
  emit('closeMobile')
}

function switchWorkspace(ws: 'bots' | 'ops') {
  store.state.workspace = ws
  emit('closeMobile')
}

function handleRosterShortcut(e: KeyboardEvent) {
  if (!e.altKey || (e.key !== 'ArrowUp' && e.key !== 'ArrowDown')) return
  if (store.state.isStreaming || document.querySelector('dialog[open]')) return
  const list = filteredBots.value
  if (list.length < 2) return
  e.preventDefault()
  const idx = list.findIndex((b) => b.id === store.state.activeBotId)
  const nextIdx =
    e.key === 'ArrowDown'
      ? (idx + 1 + list.length) % list.length
      : (idx - 1 + list.length) % list.length
  void store.selectBot(list[nextIdx].id)
}

onMounted(() => {
  applySiblingStaggerFallback(rosterRef.value)
  window.addEventListener('keydown', handleRosterShortcut)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleRosterShortcut)
})
watch(() => filteredBots.value.length, () => nextTick(() => applySiblingStaggerFallback(rosterRef.value)))
</script>

<template>
  <!-- Mobile Backdrop -->
  <Transition
    enter-active-class="transition duration-200 ease-out"
    enter-from-class="opacity-0"
    enter-to-class="opacity-100"
    leave-active-class="transition duration-150 ease-in"
    leave-from-class="opacity-100"
    leave-to-class="opacity-0"
  >
    <div
      v-if="mobileOpen"
      class="fixed inset-0 z-30 bg-black/65 backdrop-blur-xs md:hidden"
      aria-hidden="true"
      @click="emit('closeMobile')"
    />
  </Transition>

  <aside
    aria-label="导航与 Bot 名录"
    :class="[
      'fixed md:static inset-y-0 left-0 z-40 shrink-0 flex flex-col min-h-0 bg-[#060913]/94 backdrop-blur-xl overflow-hidden',
      'transition-[transform,width,opacity] duration-[260ms] ease-[var(--ease-snappy)] motion-reduce:transition-none',
      mobileOpen ? 'translate-x-0 w-[min(19rem,84vw)] border-r border-[#fff8ee]/10 shadow-2xl' : '-translate-x-full w-[min(19rem,84vw)] md:translate-x-0',
      desktopCollapsed ? 'md:w-0 md:opacity-0 md:border-r-0 md:pointer-events-none' : 'md:w-72 md:opacity-100 md:border-r md:border-[#fff8ee]/10'
    ]"
  >
    <!-- Fixed-width inner wrapper prevents text wrapping during width collapse animation -->
    <div class="w-[min(19rem,84vw)] md:w-72 flex-1 min-h-0 flex flex-col">
      <!-- Top Section -->
      <div class="p-3.5 pt-[max(0.875rem,env(safe-area-inset-top,0.875rem))] pb-2 flex flex-col gap-2.5">
        <div class="flex items-center justify-between px-1">
          <div class="flex items-center gap-2 min-w-0">
            <span
              :title="store.state.sseConnected ? 'SSE 实时事件通道在线' : 'SSE 通道重连中'"
              :class="[
                'w-2 h-2 rounded-full shrink-0',
                store.state.sseConnected ? 'bg-emerald-400 shadow-[0_0_8px_#34d399]' : 'bg-amber-400 animate-pulse'
              ]"
              aria-hidden="true"
            />
            <strong class="text-base font-semibold tracking-tight text-white truncate">Vesper BotHub</strong>
            <span
              v-if="!store.state.sseConnected"
              class="font-mono text-[9.5px] px-1.5 py-0.5 rounded bg-amber-400/15 text-amber-300 border border-amber-400/30 shrink-0"
            >
              重连中
            </span>
          </div>
          <div class="flex items-center gap-1 shrink-0">
            <span class="font-mono text-[11px] text-slate-500">{{ shortVersion(store.state.version) }}</span>
            <button
              type="button"
              aria-label="收起侧边栏"
              title="收起侧边栏"
              class="hidden md:inline-flex items-center justify-center w-8 h-8 rounded-lg text-slate-400 hover:text-[#f2b880] hover:bg-white/[0.06] cursor-pointer"
              @click="emit('toggleDesktopCollapse')"
            >
              <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <rect x="3" y="3" width="18" height="18" rx="2" />
                <line x1="9" y1="3" x2="9" y2="21" />
                <polyline points="15 9 12 12 15 15" />
              </svg>
            </button>
            <button
              type="button"
              aria-label="关闭侧边栏"
              class="md:hidden min-h-[44px] min-w-[44px] inline-flex items-center justify-center rounded-xl text-slate-400 hover:text-white hover:bg-white/10"
              @click="emit('closeMobile')"
            >
              <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-1.5">
          <button
            type="button"
            @click="store.state.createModalMode = 'bot'; store.state.showCreateBotModal = true; emit('closeMobile')"
            class="min-h-[44px] sm:min-h-[36px] flex items-center justify-center gap-1.5 rounded-xl border border-[#f2b880]/30 bg-[#f2b880]/10 hover:bg-[#f2b880]/18 hover:border-[#f2b880]/55 text-xs font-medium text-[#f7cba0] cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
              <line x1="12" y1="5" x2="12" y2="19" />
              <line x1="5" y1="12" x2="19" y2="12" />
            </svg>
            <span>新建 Bot</span>
          </button>
          <button
            type="button"
            @click="store.state.createModalMode = 'group'; store.state.showCreateBotModal = true; emit('closeMobile')"
            class="min-h-[44px] sm:min-h-[36px] flex items-center justify-center gap-1.5 rounded-xl border border-[#fff8ee]/12 bg-[#fff8ee]/[0.04] hover:border-[#f2b880]/45 hover:text-[#f7cba0] text-xs font-medium text-slate-200 cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
              <circle cx="9" cy="7" r="4" />
              <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
              <path d="M16 3.13a4 4 0 0 1 0 7.75" />
            </svg>
            <span>建群聊</span>
          </button>
        </div>

        <!-- Primary Workspace Navigation (Simplified flat items) -->
        <nav aria-label="主工作区切换" class="flex flex-col gap-0.5 pt-0.5">
          <button
            type="button"
            :aria-current="store.state.workspace === 'bots' ? 'page' : undefined"
            @click="switchWorkspace('bots')"
            :class="[
              'w-full min-h-[44px] sm:min-h-[36px] flex items-center gap-2.5 px-3 py-1.5 rounded-xl text-sm text-left cursor-pointer',
              store.state.workspace === 'bots'
                ? 'bg-[#f2b880]/14 text-[#f2b880] font-medium'
                : 'text-slate-300 hover:bg-white/[0.04] hover:text-white'
            ]"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M23 21v-2a4 4 0 0 0-3-3.87" /><path d="M16 3.13a4 4 0 0 1 0 7.75" /></svg>
            <span>专属 Bot 团队</span>
          </button>

          <button
            type="button"
            :aria-current="store.state.workspace === 'ops' ? 'page' : undefined"
            @click="switchWorkspace('ops')"
            :class="[
              'w-full min-h-[44px] sm:min-h-[36px] flex items-center gap-2.5 px-3 py-1.5 rounded-xl text-sm text-left cursor-pointer',
              store.state.workspace === 'ops'
                ? 'bg-[#f2b880]/14 text-[#f2b880] font-medium'
                : 'text-slate-300 hover:bg-white/[0.04] hover:text-white'
            ]"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><rect x="2" y="3" width="20" height="8" rx="2" /><rect x="2" y="13" width="20" height="8" rx="2" /><line x1="6" y1="7" x2="6.01" y2="7" /><line x1="6" y1="17" x2="6.01" y2="17" /></svg>
            <span>全局资产与调度</span>
          </button>

          <button
            type="button"
            @click="store.state.showWorkhubModal = true; emit('closeMobile')"
            class="w-full min-h-[44px] sm:min-h-[36px] flex items-center gap-2.5 px-3 py-1.5 rounded-xl text-sm text-slate-300 hover:bg-white/[0.04] hover:text-white cursor-pointer text-left"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M9 11l3 3L22 4" /><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" /></svg>
            <span class="flex-1">个人行动台</span>
            <span
              v-if="store.pendingTodoCount.value > 0"
              class="px-1.5 py-0.5 min-w-[20px] text-center rounded-full bg-cyan-400/20 border border-cyan-400/40 text-cyan-200 font-mono text-[10.5px] font-semibold leading-none"
            >
              {{ store.pendingTodoCount.value > 99 ? '99+' : store.pendingTodoCount.value }}
            </span>
          </button>

          <button
            type="button"
            @click="store.state.showInboxModal = true; emit('closeMobile')"
            class="w-full min-h-[44px] sm:min-h-[36px] flex items-center gap-2.5 px-3 py-1.5 rounded-xl text-sm text-slate-300 hover:bg-white/[0.04] hover:text-white cursor-pointer text-left"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" /><path d="M13.73 21a2 2 0 0 1-3.46 0" /></svg>
            <span class="flex-1">情报收件箱</span>
            <span
              v-if="store.unreadInboxCount.value > 0"
              class="px-1.5 py-0.5 min-w-[20px] text-center rounded-full bg-[#f2b880] text-[#070a13] font-mono text-[10.5px] font-bold leading-none"
            >
              {{ store.unreadInboxCount.value > 99 ? '99+' : store.unreadInboxCount.value }}
            </span>
          </button>
        </nav>

        <!-- Roster Header + Quick Search / Cmd+K -->
        <div class="pt-2 border-t border-[#fff8ee]/8 flex flex-col gap-1.5">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-1.5 min-w-0">
              <span class="font-mono text-[10.5px] tracking-wider text-slate-500">会话 ({{ filteredBots.length }})</span>
              <button
                v-if="totalUnreadMessages > 0 && unreadBotTarget"
                type="button"
                @click="handlePickBot(unreadBotTarget.id)"
                title="跳转到有新消息的会话"
                class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full bg-[#f2b880]/18 border border-[#f2b880]/40 text-[#f2b880] font-mono text-[9.5px] cursor-pointer hover:bg-[#f2b880]/28"
              >
                {{ totalUnreadMessages }} 未读
              </button>
              <button
                type="button"
                @click="store.state.showQuickSwitcher = true; emit('closeMobile')"
                title="全局搜索与快速跳转 (Cmd/Ctrl + K)"
                class="hidden sm:inline-flex items-center gap-1 px-1.5 py-0.5 rounded-md border border-[#fff8ee]/10 bg-[#fff8ee]/[0.03] hover:border-[#f2b880]/40 hover:text-[#f2b880] font-mono text-[10px] text-slate-400 cursor-pointer"
              >
                <span>Cmd+K</span>
              </button>
            </div>
            <input
              v-model="searchQuery"
              type="search"
              aria-label="筛选 Bot"
              placeholder="筛选..."
              @keydown.escape="searchQuery = ''"
              class="w-24 focus:w-28 transition-[width,border-color] duration-200 bg-transparent border-b border-[#fff8ee]/12 focus:border-[#f2b880] text-xs text-slate-100 placeholder-slate-500 px-1 py-0.5 focus:outline-none"
            />
          </div>

          <div class="grid grid-cols-4 gap-1 rounded-lg bg-[#050811]/70 p-0.5 border border-[#fff8ee]/8">
            <button
              v-for="tab in ([
                { id: 'all', label: `全部` },
                { id: 'groups', label: `群${scopeCounts.groups ? ' ' + scopeCounts.groups : ''}` },
                { id: 'companion', label: `聊${scopeCounts.companion ? ' ' + scopeCounts.companion : ''}` },
                { id: 'worker', label: `干${scopeCounts.worker ? ' ' + scopeCounts.worker : ''}` },
              ] as const)"
              :key="tab.id"
              type="button"
              @click="rosterScope = tab.id"
              :class="[
                'py-1 rounded-md text-[10.5px] font-medium cursor-pointer transition-colors truncate',
                rosterScope === tab.id
                  ? 'bg-[#f2b880]/16 text-[#f2b880]'
                  : 'text-slate-400 hover:text-slate-200'
              ]"
            >
              {{ tab.label }}
            </button>
          </div>
        </div>
      </div>

      <!-- Scrollable Persistent Bot Roster -->
      <div ref="rosterRef" class="stagger-list flex-1 min-h-0 overflow-y-auto px-2.5 pb-2 flex flex-col gap-1">
        <div
          v-if="filteredBots.length === 0"
          class="py-8 text-center flex flex-col items-center gap-2 text-xs text-slate-500 px-3"
        >
          <span v-if="store.state.bots.length === 0">还没有 Bot，点击上方「新建 Bot」开始</span>
          <span v-else>当前筛选下暂无会话</span>
          <button
            v-if="searchQuery || rosterScope !== 'all'"
            type="button"
            @click="searchQuery = ''; rosterScope = 'all'"
            class="px-2.5 py-1 rounded-full border border-[#fff8ee]/14 text-slate-300 hover:text-[#f2b880] cursor-pointer"
          >
            重置筛选
          </button>
        </div>

        <div
          v-for="b in filteredBots"
          :key="b.id"
          :class="[
            'group stagger-item relative w-full min-h-[56px] flex items-center gap-2.5 px-2.5 py-2 rounded-xl border transition-colors',
            store.state.activeBotId === b.id && store.state.workspace === 'bots'
              ? 'bg-[#f2b880]/12 border-[#f2b880]/25 border-l-[3px] border-l-[#f2b880]'
              : 'border-transparent hover:bg-white/[0.04]'
          ]"
        >
          <button
            type="button"
            @click="handlePickBot(b.id)"
            :aria-current="store.state.activeBotId === b.id && store.state.workspace === 'bots' ? 'true' : undefined"
            class="flex-1 min-w-0 flex items-center gap-2.5 text-left cursor-pointer"
          >
            <BotGroupAvatar
              v-if="isGroup(b)"
              :members="resolveGroupMembers(b)"
              :fallback-style="b.avatar_style"
              :state="b.state"
              size="md"
            />
            <BotAvatar v-else :avatar-style="b.avatar_style" :state="b.state" size="md" />

            <div class="flex-1 min-w-0">
              <div class="flex items-center justify-between gap-1.5">
                <div class="flex items-center gap-1.5 min-w-0">
                  <span class="text-[13px] font-semibold text-slate-100 truncate">{{ b.name }}</span>
                  <span
                    v-if="isGroup(b)"
                    class="px-1.5 py-0.5 rounded bg-[#fff8ee]/8 text-[9.5px] font-mono text-slate-300 leading-none shrink-0"
                  >
                    群聊
                  </span>
                  <span
                    v-else-if="b.bot_mode === 'companion'"
                    class="px-1.5 py-0.5 rounded bg-emerald-500/12 text-[9.5px] font-mono text-emerald-300 leading-none shrink-0"
                  >
                    陪伴
                  </span>
                  <span
                    v-else-if="b.bot_mode === 'worker'"
                    class="px-1.5 py-0.5 rounded bg-cyan-500/12 text-[9.5px] font-mono text-cyan-300 leading-none shrink-0"
                  >
                    执行
                  </span>
                  <span
                    v-if="b.is_pinned"
                    class="w-1.5 h-1.5 rounded-full bg-[#f2b880]/75 shrink-0"
                    title="已置顶"
                    aria-hidden="true"
                  />
                </div>
                <div class="flex items-center gap-1.5 shrink-0">
                  <span
                    v-if="b.unread_count > 0"
                    class="px-1.5 py-0.5 rounded-full bg-[#f2b880] text-[#070a13] font-mono text-[10px] font-bold leading-none"
                  >
                    {{ b.unread_count > 99 ? '99+' : b.unread_count }}
                  </span>
                  <span
                    v-else-if="b.state !== 'idle'"
                    class="inline-flex items-center gap-1 font-mono text-[10px] text-[#f2b880]"
                  >
                    <span :class="['w-1.5 h-1.5 rounded-full', stateDotClass(b.state)]" aria-hidden="true" />
                    <span>{{ botStateLabel(b.state) }}</span>
                  </span>
                  <span v-else class="font-mono text-[10px] text-slate-500">
                    {{ formatChatTime(b.updated_at) }}
                  </span>
                </div>
              </div>
              <div class="text-[11px] text-slate-400/90 truncate mt-0.5">
                <span v-if="botDraftPreview(b) && store.state.activeBotId !== b.id" class="text-rose-400 font-medium">[草稿] {{ botDraftPreview(b) }}</span>
                <template v-else>
                  <span v-if="b.state === 'thinking' || b.state === 'working'" class="text-[#f2b880]">正在输入: </span>
                  <span>{{ b.last_summary || b.role_title }}</span>
                </template>
              </div>
            </div>
          </button>

          <!-- Subtle hover Pin toggle button -->
          <button
            type="button"
            :title="b.is_pinned ? '取消置顶' : '置顶会话'"
            :aria-label="b.is_pinned ? '取消置顶' : '置顶会话'"
            @click.stop="store.togglePinBot(b)"
            :class="[
              'shrink-0 w-7 h-7 rounded-lg inline-flex items-center justify-center transition-opacity cursor-pointer',
              b.is_pinned
                ? 'text-[#f2b880] opacity-75 hover:opacity-100 hover:bg-[#f2b880]/15'
                : 'text-slate-500 opacity-65 md:opacity-0 md:group-hover:opacity-100 md:focus-visible:opacity-100 hover:text-[#f2b880] hover:bg-white/[0.06]'
            ]"
          >
            <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M12 17v5" />
              <path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Minimal 4-Tool Footer -->
      <div class="p-2.5 border-t border-[#fff8ee]/8 grid grid-cols-4 gap-1 pb-[max(0.625rem,env(safe-area-inset-bottom,0.625rem))]">
        <button
          type="button"
          title="记忆仓库与提炼视图"
          @click="store.state.showMemoriesModal = true; emit('closeMobile')"
          class="min-h-[44px] sm:min-h-[36px] flex items-center justify-center gap-1 rounded-xl text-[11.5px] text-slate-400 hover:text-[#f2b880] hover:bg-white/[0.04] cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 2a4 4 0 0 0-4 4v1a3 3 0 0 0-3 3v1a3 3 0 0 0 2 2.83V17a3 3 0 0 0 3 3h4a3 3 0 0 0 3-3v-3.17A3 3 0 0 0 19 11v-1a3 3 0 0 0-3-3V6a4 4 0 0 0-4-4z" /><line x1="12" y1="7" x2="12" y2="15" /></svg>
          <span>记忆</span>
        </button>
        <button
          type="button"
          title="Playbook 与 Webhook 技能市场"
          @click="store.state.showSkillsModal = true; emit('closeMobile')"
          class="min-h-[44px] sm:min-h-[36px] flex items-center justify-center gap-1 rounded-xl text-[11.5px] text-slate-400 hover:text-[#f2b880] hover:bg-white/[0.04] cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" /></svg>
          <span>技能</span>
        </button>
        <button
          type="button"
          title="系统推送与任务运行流水"
          @click="store.state.showLogsModal = true; emit('closeMobile')"
          class="min-h-[44px] sm:min-h-[36px] flex items-center justify-center gap-1 rounded-xl text-[11.5px] text-slate-400 hover:text-[#f2b880] hover:bg-white/[0.04] cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12" /></svg>
          <span>流水</span>
        </button>
        <button
          type="button"
          title="AI 引擎、模型池与安全设置"
          @click="store.state.showSettingsModal = true; emit('closeMobile')"
          class="min-h-[44px] sm:min-h-[36px] flex items-center justify-center gap-1 rounded-xl text-[11.5px] text-slate-400 hover:text-[#f2b880] hover:bg-white/[0.04] cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>
          <span>设置</span>
        </button>
      </div>
    </div>
  </aside>
</template>
