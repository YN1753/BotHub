<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { botStateLabel, shortVersion } from '../utils/format'
import { applySiblingStaggerFallback } from '../utils/topLayer'
import BotAvatar from './BotAvatar.vue'

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
const rosterRef = ref<HTMLElement | null>(null)

const filteredBots = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return store.state.bots
  return store.state.bots.filter((b) =>
    `${b.name} ${b.role_title} ${b.last_summary}`.toLowerCase().includes(q)
  )
})

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

onMounted(() => applySiblingStaggerFallback(rosterRef.value))
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
            <span class="w-2 h-2 rounded-full bg-emerald-400 shadow-[0_0_8px_#34d399] shrink-0" aria-hidden="true" />
            <strong class="text-base font-semibold tracking-tight text-white truncate">Vesper BotHub</strong>
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

        <button
          type="button"
          @click="store.state.showCreateBotModal = true; emit('closeMobile')"
          class="w-full min-h-[44px] sm:min-h-[38px] flex items-center justify-center gap-2 rounded-xl border border-[#f2b880]/28 bg-[#f2b880]/10 hover:bg-[#f2b880]/18 hover:border-[#f2b880]/55 text-xs sm:text-sm font-medium text-[#f7cba0] cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
            <line x1="12" y1="5" x2="12" y2="19" />
            <line x1="5" y1="12" x2="19" y2="12" />
          </svg>
          <span>新建专属 Bot</span>
        </button>

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

        <!-- Roster Header -->
        <div class="pt-2 border-t border-[#fff8ee]/8 flex items-center justify-between gap-2">
          <span class="font-mono text-[10.5px] tracking-wider text-slate-500">BOT 名录 ({{ filteredBots.length }})</span>
          <input
            v-model="searchQuery"
            type="search"
            aria-label="筛选 Bot"
            placeholder="筛选..."
            @keydown.escape="searchQuery = ''"
            class="w-24 focus:w-28 transition-[width,border-color] duration-200 bg-transparent border-b border-[#fff8ee]/12 focus:border-[#f2b880] text-xs text-slate-100 placeholder-slate-500 px-1 py-0.5 focus:outline-none"
          />
        </div>
      </div>

      <!-- Scrollable Persistent Bot Roster -->
      <div ref="rosterRef" class="stagger-list flex-1 min-h-0 overflow-y-auto px-2.5 pb-2 flex flex-col gap-1">
        <div
          v-if="filteredBots.length === 0"
          class="py-8 text-center flex flex-col items-center gap-2 text-xs text-slate-500"
        >
          <span>无匹配的专属 Bot</span>
          <button
            v-if="searchQuery"
            type="button"
            @click="searchQuery = ''"
            class="px-2.5 py-1 rounded-full border border-[#fff8ee]/14 text-slate-300 hover:text-[#f2b880] cursor-pointer"
          >
            清除筛选
          </button>
        </div>

        <button
          v-for="b in filteredBots"
          :key="b.id"
          type="button"
          @click="handlePickBot(b.id)"
          :aria-current="store.state.activeBotId === b.id && store.state.workspace === 'bots' ? 'true' : undefined"
          :class="[
            'group stagger-item relative w-full min-h-[52px] flex items-center gap-2.5 px-2.5 py-2 rounded-xl border text-left cursor-pointer',
            store.state.activeBotId === b.id && store.state.workspace === 'bots'
              ? 'bg-[#f2b880]/12 border-[#f2b880]/25 border-l-[3px] border-l-[#f2b880]'
              : 'border-transparent hover:bg-white/[0.04]'
          ]"
        >
          <BotAvatar :avatar-style="b.avatar_style" :state="b.state" size="md" />
          <div class="flex-1 min-w-0">
            <div class="flex items-center justify-between gap-1.5">
              <span class="text-[13px] font-semibold text-slate-100 truncate">{{ b.name }}</span>
              <span
                v-if="b.unread_count > 0"
                class="px-1.5 py-0.5 rounded-full bg-[#f2b880] text-[#070a13] font-mono text-[10px] font-bold leading-none shrink-0"
              >
                {{ b.unread_count > 99 ? '99+' : b.unread_count }}
              </span>
              <span
                v-else-if="b.state !== 'idle'"
                class="inline-flex items-center gap-1 font-mono text-[10px] text-slate-300 shrink-0"
              >
                <span :class="['w-1.5 h-1.5 rounded-full', stateDotClass(b.state)]" aria-hidden="true" />
                <span>{{ botStateLabel(b.state) }}</span>
              </span>
            </div>
            <div class="text-[11px] text-slate-400/90 truncate mt-0.5">
              {{ b.last_summary || b.role_title }}
            </div>
          </div>
        </button>
      </div>

      <!-- Minimal 3-Tool Footer -->
      <div class="p-2.5 border-t border-[#fff8ee]/8 grid grid-cols-3 gap-1 pb-[max(0.625rem,env(safe-area-inset-bottom,0.625rem))]">
        <button
          type="button"
          @click="store.state.showMemoriesModal = true; emit('closeMobile')"
          class="min-h-[44px] sm:min-h-[36px] flex items-center justify-center gap-1.5 rounded-xl text-xs text-slate-400 hover:text-[#f2b880] hover:bg-white/[0.04] cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 2a4 4 0 0 0-4 4v1a3 3 0 0 0-3 3v1a3 3 0 0 0 2 2.83V17a3 3 0 0 0 3 3h4a3 3 0 0 0 3-3v-3.17A3 3 0 0 0 19 11v-1a3 3 0 0 0-3-3V6a4 4 0 0 0-4-4z" /><line x1="12" y1="7" x2="12" y2="15" /></svg>
          <span>记忆</span>
        </button>
        <button
          type="button"
          @click="store.state.showLogsModal = true; emit('closeMobile')"
          class="min-h-[44px] sm:min-h-[36px] flex items-center justify-center gap-1.5 rounded-xl text-xs text-slate-400 hover:text-[#f2b880] hover:bg-white/[0.04] cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12" /></svg>
          <span>流水</span>
        </button>
        <button
          type="button"
          @click="store.state.showSettingsModal = true; emit('closeMobile')"
          class="min-h-[44px] sm:min-h-[36px] flex items-center justify-center gap-1.5 rounded-xl text-xs text-slate-400 hover:text-[#f2b880] hover:bg-white/[0.04] cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>
          <span>设置</span>
        </button>
      </div>
    </div>
  </aside>
</template>
