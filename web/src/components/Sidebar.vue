<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { botStateLabel, shortVersion } from '../utils/format'
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

const filteredBots = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return store.state.bots
  return store.state.bots.filter((b) =>
    `${b.name} ${b.role_title} ${b.last_summary}`.toLowerCase().includes(q)
  )
})

function stateBadgeClass(state: string): string {
  if (state === 'thinking') return 'border-amber-400/40 bg-amber-400/10 text-amber-300'
  if (state === 'working') return 'border-[#f2b880]/45 bg-[#f2b880]/12 text-[#f2b880]'
  if (state === 'done') return 'border-emerald-400/40 bg-emerald-400/10 text-emerald-300'
  if (state === 'blocked') return 'border-rose-400/45 bg-rose-400/10 text-rose-300'
  return 'border-[#fff8ee]/15 text-slate-400'
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
</script>

<template>
  <!-- Mobile Backdrop -->
  <div
    v-if="mobileOpen"
    class="fixed inset-0 z-30 bg-black/60 backdrop-blur-xs md:hidden"
    @click="emit('closeMobile')"
  />

  <aside
    :class="[
      'fixed md:static inset-y-0 left-0 z-40 shrink-0 flex flex-col min-h-0 bg-[#060913]/92 backdrop-blur-xl transition-all duration-200 ease-out overflow-hidden',
      mobileOpen ? 'translate-x-0 w-72 border-r border-[#fff8ee]/11' : '-translate-x-full w-72 md:translate-x-0',
      desktopCollapsed ? 'md:w-0 md:border-r-0 md:pointer-events-none' : 'md:w-72 md:border-r md:border-[#fff8ee]/11'
    ]"
  >
    <!-- Top Section -->
    <div class="p-3.5 pb-2.5 flex flex-col gap-3">
      <div class="flex items-center justify-between px-1">
        <div class="flex items-center gap-2">
          <span class="w-2 h-2 rounded-full bg-emerald-400 shadow-[0_0_8px_#34d399]" />
          <strong class="text-base font-semibold tracking-tight text-white whitespace-nowrap">Vesper BotHub</strong>
        </div>
        <div class="flex items-center gap-1.5">
          <span class="font-mono text-[11px] text-slate-400">{{ shortVersion(store.state.version) }}</span>
          <button
            type="button"
            aria-label="收起侧边栏"
            title="收起侧边栏"
            class="hidden md:inline-flex items-center justify-center w-8 h-8 rounded-lg text-slate-400 hover:text-[#f2b880] hover:bg-white/8 transition cursor-pointer"
            @click="emit('toggleDesktopCollapse')"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="3" width="18" height="18" rx="2" />
              <line x1="9" y1="3" x2="9" y2="21" />
              <polyline points="15 9 12 12 15 15" />
            </svg>
          </button>
          <button
            type="button"
            aria-label="关闭侧边栏"
            class="md:hidden min-h-[40px] min-w-[40px] inline-flex items-center justify-center rounded-lg text-slate-400 hover:text-white hover:bg-white/10"
            @click="emit('closeMobile')"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
          </button>
        </div>
      </div>

      <button
        type="button"
        @click="store.state.showCreateBotModal = true"
        class="w-full min-h-[40px] flex items-center justify-center gap-2 rounded-full border border-[#fff8ee]/14 bg-[#fff8ee]/[0.04] hover:bg-[#f2b880]/15 hover:border-[#f2b880]/45 text-sm font-medium text-slate-100 hover:text-[#f2b880] transition cursor-pointer"
      >
        <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
        <span>新建专属 Bot</span>
      </button>

      <!-- Workspace & Inbox Navigation -->
      <nav class="flex flex-col gap-1">
        <button
          type="button"
          @click="switchWorkspace('bots')"
          :class="[
            'w-full min-h-[40px] flex items-center gap-2.5 px-3 py-2 rounded-xl text-sm transition cursor-pointer text-left',
            store.state.workspace === 'bots'
              ? 'bg-[#f2b880]/14 text-[#f2b880] font-medium border border-[#f2b880]/25'
              : 'border border-transparent text-slate-300 hover:bg-white/5 hover:text-white'
          ]"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M23 21v-2a4 4 0 0 0-3-3.87" /><path d="M16 3.13a4 4 0 0 1 0 7.75" /></svg>
          <span>专属 Bot 团队</span>
        </button>

        <button
          type="button"
          @click="switchWorkspace('ops')"
          :class="[
            'w-full min-h-[40px] flex items-center gap-2.5 px-3 py-2 rounded-xl text-sm transition cursor-pointer text-left',
            store.state.workspace === 'ops'
              ? 'bg-[#f2b880]/14 text-[#f2b880] font-medium border border-[#f2b880]/25'
              : 'border border-transparent text-slate-300 hover:bg-white/5 hover:text-white'
          ]"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="2" y="3" width="20" height="8" rx="2" /><rect x="2" y="13" width="20" height="8" rx="2" /><line x1="6" y1="7" x2="6.01" y2="7" /><line x1="6" y1="17" x2="6.01" y2="17" /></svg>
          <span>全局资产与调度</span>
        </button>

        <button
          type="button"
          @click="store.state.showInboxModal = true; emit('closeMobile')"
          class="w-full min-h-[40px] flex items-center gap-2.5 px-3 py-2 rounded-xl border border-transparent text-sm text-slate-300 hover:bg-white/5 hover:text-white transition cursor-pointer text-left"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" /><path d="M13.73 21a2 2 0 0 1-3.46 0" /></svg>
          <span class="flex-1">情报收件箱</span>
          <span
            v-if="store.unreadInboxCount.value > 0"
            class="px-1.5 py-0.5 min-w-[20px] text-center rounded-full bg-[#f2b880] text-[#070a13] font-mono text-[11px] font-bold leading-none"
          >
            {{ store.unreadInboxCount.value > 99 ? '99+' : store.unreadInboxCount.value }}
          </span>
        </button>
      </nav>

      <!-- Roster Header -->
      <div class="pt-2 border-t border-[#fff8ee]/10 flex items-center justify-between gap-2">
        <span class="font-mono text-[11px] text-slate-400">BOT 名录</span>
        <input
          v-model="searchQuery"
          type="search"
          placeholder="筛选..."
          class="w-28 bg-transparent border-b border-[#fff8ee]/15 focus:border-[#f2b880] text-xs text-slate-100 placeholder-slate-500 px-1 py-0.5 focus:outline-none"
        />
      </div>
    </div>

    <!-- Scrollable Persistent Bot Roster -->
    <div class="flex-1 min-h-0 overflow-y-auto px-2.5 pb-2 flex flex-col gap-1">
      <div
        v-if="filteredBots.length === 0"
        class="py-8 text-center text-xs text-slate-500"
      >
        无匹配的专属 Bot
      </div>

      <button
        v-for="b in filteredBots"
        :key="b.id"
        type="button"
        @click="handlePickBot(b.id)"
        :class="[
          'relative w-full min-h-[52px] flex items-center gap-2.5 px-2.5 py-2.5 rounded-xl border text-left transition cursor-pointer',
          store.state.activeBotId === b.id && store.state.workspace === 'bots'
            ? 'bg-[#f2b880]/12 border-[#f2b880]/28 border-l-[3px] border-l-[#f2b880]'
            : 'border-transparent hover:bg-white/5'
        ]"
      >
        <BotAvatar :avatar-style="b.avatar_style" :state="b.state" size="md" />
        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between gap-1.5">
            <span class="text-[13px] font-semibold text-slate-100 truncate">{{ b.name }}</span>
            <span
              v-if="b.unread_count > 0"
              class="px-1.5 py-0.5 rounded-full bg-[#f2b880] text-[#070a13] font-mono text-[10px] font-bold leading-none"
            >
              {{ b.unread_count > 99 ? '99+' : b.unread_count }}
            </span>
            <span
              v-else
              :class="['font-mono text-[10px] px-1.5 py-0.5 rounded-full border leading-none', stateBadgeClass(b.state)]"
            >
              {{ botStateLabel(b.state) }}
            </span>
          </div>
          <div class="text-[11px] text-slate-400 truncate mt-0.5">
            {{ b.last_summary || b.role_title }}
          </div>
        </div>
      </button>
    </div>

    <!-- Sidebar Footer Utilities (3 safe items; Logout moved to SettingsModal) -->
    <div class="p-2.5 border-t border-[#fff8ee]/10 grid grid-cols-3 gap-1.5 pb-[max(0.625rem,env(safe-area-inset-bottom))]">
      <button
        type="button"
        @click="store.state.showMemoriesModal = true; emit('closeMobile')"
        class="min-h-[40px] flex items-center justify-center gap-1.5 rounded-lg text-xs text-slate-300 hover:text-[#f2b880] hover:bg-white/5 transition cursor-pointer"
      >
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2a4 4 0 0 0-4 4v1a3 3 0 0 0-3 3v1a3 3 0 0 0 2 2.83V17a3 3 0 0 0 3 3h4a3 3 0 0 0 3-3v-3.17A3 3 0 0 0 19 11v-1a3 3 0 0 0-3-3V6a4 4 0 0 0-4-4z" /><line x1="12" y1="7" x2="12" y2="15" /></svg>
        <span>记忆</span>
      </button>
      <button
        type="button"
        @click="store.state.showLogsModal = true; emit('closeMobile')"
        class="min-h-[40px] flex items-center justify-center gap-1.5 rounded-lg text-xs text-slate-300 hover:text-[#f2b880] hover:bg-white/5 transition cursor-pointer"
      >
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12" /></svg>
        <span>流水</span>
      </button>
      <button
        type="button"
        @click="store.state.showSettingsModal = true; emit('closeMobile')"
        class="min-h-[40px] flex items-center justify-center gap-1.5 rounded-lg text-xs text-slate-300 hover:text-[#f2b880] hover:bg-white/5 transition cursor-pointer"
      >
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>
        <span>设置</span>
      </button>
    </div>
  </aside>
</template>
