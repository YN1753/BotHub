<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { botStateDotClass, botStateLabel, formatChatTime } from '../utils/format'
import { resolveGroupMemberBots } from '../utils/botLook'
import { applySiblingStaggerFallback } from '../utils/topLayer'
import type { Bot } from '../types/bothub'
import BotAvatar from './BotAvatar.vue'
import BotGroupAvatar from './BotGroupAvatar.vue'
import SegmentedTabs from './SegmentedTabs.vue'
import VesperBrandIcon from './ui/VesperBrandIcon.vue'

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
const isOnline = ref(typeof navigator === 'undefined' ? true : navigator.onLine)

const WORKSPACE_MODES = [
  { id: 'bots', label: '对话' },
  { id: 'ops', label: '资产' },
]

function updateOnlineFlag() {
  isOnline.value = typeof navigator === 'undefined' ? true : navigator.onLine
}

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
    if (rosterScope.value === 'worker' && (isGroup(b) || b.bot_mode === 'companion')) return false
    if (!q) return true
    return `${b.name} ${b.role_title} ${b.traits || ''} ${b.last_summary}`.toLowerCase().includes(q)
  })
})

function resolveGroupMembers(b: Bot): Bot[] {
  return resolveGroupMemberBots(b, store.state.bots)
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

function handlePickBot(id: string) {
  if (store.state.isStreaming) return
  void store.selectBot(id)
  emit('closeMobile')
}

function onModeChange(val: string) {
  store.state.workspace = val === 'ops' ? 'ops' : 'bots'
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
  window.addEventListener('online', updateOnlineFlag)
  window.addEventListener('offline', updateOnlineFlag)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleRosterShortcut)
  window.removeEventListener('online', updateOnlineFlag)
  window.removeEventListener('offline', updateOnlineFlag)
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
      'sidebar-console fixed md:static inset-y-0 left-0 z-40 shrink-0 flex flex-col min-h-0 overflow-hidden md:rounded-[24px]',
      'transition-[transform,width,opacity] duration-[260ms] ease-[var(--ease-snappy)] motion-reduce:transition-none',
      mobileOpen ? 'translate-x-0 w-[min(19.5rem,85vw)] border-r border-[#fff8ee]/14 shadow-2xl' : '-translate-x-full w-[min(19.5rem,85vw)] md:translate-x-0',
      desktopCollapsed ? 'md:w-0 md:opacity-0 md:border-0 md:pointer-events-none' : 'md:w-[17.5rem] md:opacity-100 md:border md:border-[#fff8ee]/12'
    ]"
  >
    <div class="w-[min(19.5rem,85vw)] md:w-[17.5rem] flex-1 min-h-0 flex flex-col">
      <!-- Zone 1: Minimal Studio Header + Inline Workspace Switch + Unified Search/Create Bar -->
      <div class="p-3 pt-[max(0.875rem,env(safe-area-inset-top,0.875rem))] pb-2 flex flex-col gap-2.5">
        <div class="flex items-center justify-between gap-2">
          <div class="flex items-center gap-2 min-w-0">
            <span
              class="relative w-7 h-7 rounded-xl inline-flex items-center justify-center shrink-0 shadow-[0_3px_10px_rgba(0,0,0,0.45)]"
              :title="!isOnline ? '设备已离线' : store.state.sseConnected ? '实时通道在线' : '通道重连中'"
              aria-hidden="true"
            >
              <VesperBrandIcon class="w-7 h-7" />
              <span
                :class="[
                  'absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full ring-2 ring-[#090d1b]',
                  !isOnline
                    ? 'bg-rose-400'
                    : store.state.sseConnected
                    ? 'bg-emerald-400'
                    : 'bg-amber-400 animate-pulse'
                ]"
              />
            </span>
            <strong class="text-sm font-semibold tracking-tight text-white truncate">BotHub</strong>
          </div>

          <div class="flex items-center gap-1 shrink-0">
            <SegmentedTabs
              :model-value="store.state.workspace"
              :items="WORKSPACE_MODES"
              variant="subtle"
              aria-label="主模式切换"
              @update:model-value="onModeChange"
            />
            <button
              type="button"
              aria-label="收起侧边栏"
              title="收起侧边栏"
              class="hidden md:inline-flex items-center justify-center w-7 h-7 rounded-lg text-slate-400 hover:text-[#f2b880] hover:bg-white/[0.06] cursor-pointer"
              @click="emit('toggleDesktopCollapse')"
            >
              <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <rect x="3" y="3" width="18" height="18" rx="2" />
                <line x1="9" y1="3" x2="9" y2="21" />
                <polyline points="15 9 12 12 15 15" />
              </svg>
            </button>
            <button
              type="button"
              aria-label="关闭侧边栏"
              class="md:hidden min-h-[40px] min-w-[40px] inline-flex items-center justify-center rounded-xl text-slate-400 hover:text-white hover:bg-white/10"
              @click="emit('closeMobile')"
            >
              <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>
        </div>

        <!-- Unified Omnibar Search + Compact Bot / Group Creators -->
        <div class="flex items-center gap-1.5">
          <div class="relative flex-1 min-w-0">
            <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-500">
              <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
            <input
              v-model="searchQuery"
              type="search"
              aria-label="筛选会话"
              placeholder="筛选会话..."
              @keydown.escape="searchQuery = ''"
              class="w-full min-h-[36px] rounded-xl border border-[#fff8ee]/10 pl-8 pr-12 py-1.5 text-xs text-slate-100 placeholder-slate-500 focus:border-[#f2b880]/55 focus:outline-none"
            />
            <button
              v-if="!searchQuery"
              type="button"
              @click="store.state.showQuickSwitcher = true; emit('closeMobile')"
              title="全局快速跳转 (Cmd/Ctrl + K)"
              class="hidden sm:inline-flex items-center px-1.5 py-0.5 rounded-md bg-white/[0.06] hover:bg-[#f2b880]/20 hover:text-[#ffe2be] font-mono text-[9.5px] text-slate-400 absolute right-1.5 top-1/2 -translate-y-1/2 cursor-pointer"
            >
              Cmd+K
            </button>
          </div>

          <button
            type="button"
            title="新建专属 Bot"
            aria-label="新建专属 Bot"
            @click="store.state.createModalMode = 'bot'; store.state.showCreateBotModal = true; emit('closeMobile')"
            class="btn-jewel w-9 h-9 rounded-xl inline-flex items-center justify-center shrink-0 cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2.4" aria-hidden="true">
              <line x1="12" y1="5" x2="12" y2="19" />
              <line x1="5" y1="12" x2="19" y2="12" />
            </svg>
          </button>
          <button
            type="button"
            title="新建多人协同群聊"
            aria-label="新建多人协同群聊"
            @click="store.state.createModalMode = 'group'; store.state.showCreateBotModal = true; emit('closeMobile')"
            class="btn-tactile w-9 h-9 rounded-xl inline-flex items-center justify-center shrink-0 cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="14.5" height="14.5" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true">
              <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
              <circle cx="9" cy="7" r="4" />
              <line x1="20" y1="8" x2="20" y2="14" />
              <line x1="23" y1="11" x2="17" y2="11" />
            </svg>
          </button>
        </div>

        <!-- Subtle Scope Filter Pills (only shown when >2 bots exist, zero clutter for small rosters) -->
        <div v-if="store.state.bots.length > 2" class="flex items-center justify-between gap-1">
          <div class="flex items-center gap-1 flex-1 min-w-0">
            <button
              v-for="tab in ([
                { id: 'all', label: '全部' },
                { id: 'groups', label: `群聊${scopeCounts.groups ? ' ' + scopeCounts.groups : ''}` },
                { id: 'companion', label: '陪伴' },
                { id: 'worker', label: '执行' },
              ] as const)"
              :key="tab.id"
              type="button"
              @click="rosterScope = tab.id"
              :class="[
                'px-2 py-0.5 rounded-full text-[10.5px] cursor-pointer transition-all truncate',
                rosterScope === tab.id
                  ? 'bg-[#f2b880]/18 text-[#ffe2be] font-medium'
                  : 'text-slate-500 hover:text-slate-300'
              ]"
            >
              {{ tab.label }}
            </button>
          </div>
          <button
            v-if="totalUnreadMessages > 0 && unreadBotTarget"
            type="button"
            @click="handlePickBot(unreadBotTarget.id)"
            class="btn-tactile-gold px-2 py-0.5 rounded-full font-mono text-[9.5px] cursor-pointer shrink-0"
          >
            {{ totalUnreadMessages }} 未读
          </button>
        </div>
      </div>

      <!-- Zone 2: Spacious, Breathable Bot Roster (Hero Centerpiece of the Sidebar) -->
      <div ref="rosterRef" class="stagger-list flex-1 min-h-0 overflow-y-auto px-2.5 pb-2 flex flex-col gap-1">
        <div
          v-if="filteredBots.length === 0"
          class="py-10 text-center flex flex-col items-center gap-2.5 text-xs text-slate-500 px-3"
        >
          <span v-if="store.state.bots.length === 0">点击右上方「+」创建首位专属伙伴</span>
          <span v-else>当前筛选下暂无会话</span>
          <button
            v-if="searchQuery || rosterScope !== 'all'"
            type="button"
            @click="searchQuery = ''; rosterScope = 'all'"
            class="btn-tactile px-3 py-1 rounded-full text-[11px] cursor-pointer"
          >
            重置筛选
          </button>
        </div>

        <div
          v-for="b in filteredBots"
          :key="b.id"
          :class="[
            'group stagger-item relative w-full min-h-[58px] flex items-center gap-2.5 px-2.5 py-2 rounded-2xl border transition-all duration-200',
            store.state.activeBotId === b.id && store.state.workspace === 'bots'
              ? 'roster-card-active'
              : 'border-transparent hover:bg-white/[0.045] hover:border-[#fff8ee]/10'
          ]"
        >
          <span
            aria-hidden="true"
            :class="[
              'pointer-events-none absolute left-0 top-3.5 bottom-3.5 w-[3px] rounded-r-full bg-gradient-to-b from-[#ffe2be] to-[#d99252] shadow-[0_0_10px_#f2b880] origin-center transition-[transform,opacity] duration-220 ease-[var(--ease-spring)] motion-reduce:transition-none',
              store.state.activeBotId === b.id && store.state.workspace === 'bots'
                ? 'scale-y-100 opacity-100'
                : 'scale-y-0 opacity-0'
            ]"
          />
          <button
            type="button"
            @click="handlePickBot(b.id)"
            :aria-current="store.state.activeBotId === b.id && store.state.workspace === 'bots' ? 'true' : undefined"
            class="flex-1 min-w-0 flex items-center gap-2.5 text-left cursor-pointer rounded-xl focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[#f2b880]/70"
          >
            <BotGroupAvatar
              v-if="isGroup(b)"
              :members="resolveGroupMembers(b)"
              :fallback-style="b.avatar_style"
              :state="b.state"
              size="md"
            />
            <BotAvatar v-else :avatar-style="b.avatar_style" :state="b.state" size="md" :interactive="false" />

            <div class="flex-1 min-w-0">
              <div class="flex items-center justify-between gap-1.5">
                <div class="flex items-center gap-1.5 min-w-0">
                  <span class="text-[13px] font-semibold text-slate-100 truncate">{{ b.name }}</span>
                  <span
                    v-if="isGroup(b)"
                    class="px-1.5 py-0.5 rounded-md bg-white/[0.06] text-[9.5px] font-mono text-slate-400 leading-none shrink-0"
                  >
                    群
                  </span>
                  <span
                    v-if="b.is_pinned"
                    class="w-1.5 h-1.5 rounded-full bg-[#f2b880]/80 shrink-0"
                    title="已置顶"
                    aria-hidden="true"
                  />
                </div>
                <div class="flex items-center gap-1.5 shrink-0">
                  <span
                    v-if="b.unread_count > 0"
                    class="badge-pop btn-jewel px-1.5 py-0.5 rounded-full font-mono text-[10px] leading-none"
                  >
                    {{ b.unread_count > 99 ? '99+' : b.unread_count }}
                  </span>
                  <span
                    v-else-if="b.state !== 'idle'"
                    class="inline-flex items-center gap-1 font-mono text-[10px] text-[#f2b880]"
                  >
                    <span :class="['w-1.5 h-1.5 rounded-full', botStateDotClass(b.state)]" aria-hidden="true" />
                    <span>{{ botStateLabel(b.state) }}</span>
                  </span>
                  <span v-else class="font-mono text-[10px] text-slate-500">
                    {{ formatChatTime(b.updated_at) }}
                  </span>
                </div>
              </div>
              <div class="text-[11px] text-slate-400/85 truncate mt-0.5">
                <span v-if="botDraftPreview(b) && store.state.activeBotId !== b.id" class="text-rose-400 font-medium">[草稿] {{ botDraftPreview(b) }}</span>
                <template v-else>
                  <span v-if="b.state === 'thinking' || b.state === 'working'" class="text-[#f2b880]">正在处理 · </span>
                  <span>{{ b.last_summary || b.role_title }}</span>
                </template>
              </div>
            </div>
          </button>

          <button
            type="button"
            :title="b.is_pinned ? '取消置顶' : '置顶会话'"
            :aria-label="b.is_pinned ? '取消置顶' : '置顶会话'"
            @click.stop="store.togglePinBot(b)"
            :class="[
              'shrink-0 w-6 h-6 rounded-lg inline-flex items-center justify-center transition-opacity cursor-pointer',
              b.is_pinned
                ? 'text-[#f2b880] opacity-80 hover:opacity-100 hover:bg-[#f2b880]/15'
                : 'text-slate-500 opacity-0 group-hover:opacity-100 focus-visible:opacity-100 hover:text-[#f2b880] hover:bg-white/[0.06]'
            ]"
          >
            <svg viewBox="0 0 24 24" width="12.5" height="12.5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M12 17v5" />
              <path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Zone 3: Unified Floating Launcher Dock (Consolidates All 6 Global Tools into One Sleek Dock) -->
      <div class="px-2.5 pt-1.5 pb-[max(0.625rem,env(safe-area-inset-bottom,0.625rem))]">
        <div class="well-recessed rounded-2xl p-1 grid grid-cols-6 gap-0.5" role="toolbar" aria-label="快捷工具坞">
          <button
            type="button"
            title="个人行动台（待办与文档）"
            aria-label="个人行动台"
            @click="store.state.workhubTab = 'todos'; store.state.showWorkhubModal = true; emit('closeMobile')"
            class="relative min-h-[44px] sm:min-h-[40px] flex flex-col items-center justify-center gap-0.5 rounded-xl text-slate-400 hover:text-cyan-200 hover:bg-white/[0.07] cursor-pointer"
          >
            <span
              v-if="store.pendingTodoCount.value > 0"
              class="absolute top-1 right-1.5 w-2 h-2 rounded-full bg-cyan-400 shadow-[0_0_6px_#22d3ee]"
              aria-hidden="true"
            />
            <svg viewBox="0 0 24 24" width="14.5" height="14.5" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M9 11l3 3L22 4" /><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" /></svg>
            <span class="text-[9.5px] leading-none">行动</span>
          </button>

          <button
            type="button"
            title="智能收藏与闪念匣"
            aria-label="智能收藏与闪念匣"
            @click="store.state.workhubTab = 'stash'; store.state.showWorkhubModal = true; emit('closeMobile')"
            class="relative min-h-[44px] sm:min-h-[40px] flex flex-col items-center justify-center gap-0.5 rounded-xl text-slate-400 hover:text-rose-200 hover:bg-white/[0.07] cursor-pointer"
          >
            <span
              v-if="store.unreadStashCount.value > 0"
              class="absolute top-1 right-1.5 w-2 h-2 rounded-full bg-rose-400 shadow-[0_0_6px_#fb7185]"
              aria-hidden="true"
            />
            <svg viewBox="0 0 24 24" width="14.5" height="14.5" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M19 21l-7-5-7 5V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z" /></svg>
            <span class="text-[9.5px] leading-none">闪念</span>
          </button>

          <button
            type="button"
            title="情报收件箱"
            aria-label="情报收件箱"
            @click="store.state.showInboxModal = true; emit('closeMobile')"
            class="relative min-h-[44px] sm:min-h-[40px] flex flex-col items-center justify-center gap-0.5 rounded-xl text-slate-400 hover:text-[#ffe2be] hover:bg-white/[0.07] cursor-pointer"
          >
            <span
              v-if="store.unreadInboxCount.value > 0"
              class="absolute top-1 right-1.5 w-2 h-2 rounded-full bg-[#f2b880] shadow-[0_0_6px_#f2b880]"
              aria-hidden="true"
            />
            <svg viewBox="0 0 24 24" width="14.5" height="14.5" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" /><path d="M13.73 21a2 2 0 0 1-3.46 0" /></svg>
            <span class="text-[9.5px] leading-none">收件</span>
          </button>

          <button
            type="button"
            title="长期记忆仓库"
            aria-label="长期记忆仓库"
            @click="store.state.showMemoriesModal = true; emit('closeMobile')"
            class="min-h-[44px] sm:min-h-[40px] flex flex-col items-center justify-center gap-0.5 rounded-xl text-slate-400 hover:text-emerald-200 hover:bg-white/[0.07] cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="14.5" height="14.5" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M12 2a4 4 0 0 0-4 4v1a3 3 0 0 0-3 3v1a3 3 0 0 0 2 2.83V17a3 3 0 0 0 3 3h4a3 3 0 0 0 3-3v-3.17A3 3 0 0 0 19 11v-1a3 3 0 0 0-3-3V6a4 4 0 0 0-4-4z" /><line x1="12" y1="7" x2="12" y2="15" /></svg>
            <span class="text-[9.5px] leading-none">记忆</span>
          </button>

          <button
            type="button"
            title="技能工坊与 Webhook"
            aria-label="技能工坊"
            @click="store.state.showSkillsModal = true; emit('closeMobile')"
            class="min-h-[44px] sm:min-h-[40px] flex flex-col items-center justify-center gap-0.5 rounded-xl text-slate-400 hover:text-[#ffe2be] hover:bg-white/[0.07] cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="14.5" height="14.5" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" /></svg>
            <span class="text-[9.5px] leading-none">技能</span>
          </button>

          <button
            type="button"
            title="AI 引擎、模型池与安全设置 (Cmd+,)"
            aria-label="系统设置"
            @click="store.state.showSettingsModal = true; emit('closeMobile')"
            class="min-h-[44px] sm:min-h-[40px] flex flex-col items-center justify-center gap-0.5 rounded-xl text-slate-400 hover:text-[#ffe2be] hover:bg-white/[0.07] cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="14.5" height="14.5" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>
            <span class="text-[9.5px] leading-none">设置</span>
          </button>
        </div>
      </div>
    </div>
  </aside>
</template>
