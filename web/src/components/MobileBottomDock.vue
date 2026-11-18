<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../stores/useAppStore'

defineProps<{
  mobileMenuOpen: boolean
}>()

const emit = defineEmits<{
  toggleMobileMenu: []
}>()

const store = useAppStore()

const botUnreadTotal = computed(() =>
  store.state.bots.reduce((sum, b) => sum + (b.unread_count || 0), 0)
)

const inboxUnread = computed(() => store.unreadInboxCount.value)

function switchWorkspace(target: 'bots' | 'ops') {
  store.state.showInboxModal = false
  store.state.showWorkhubModal = false
  store.state.workspace = target
}

function openInbox() {
  store.state.showWorkhubModal = false
  store.state.showInboxModal = true
}

function openStash() {
  store.state.showInboxModal = false
  store.state.showWorkhubModal = true
}
</script>

<template>
  <nav
    aria-label="移动端主导航"
    class="md:hidden shrink-0 border-t border-[#fff8ee]/12 bg-[#080c1a]/94 backdrop-blur-2xl px-2.5 pt-1.5 pb-[max(0.375rem,env(safe-area-inset-bottom,0.375rem))]"
  >
    <div class="grid grid-cols-5 gap-1">
      <button
        type="button"
        @click="switchWorkspace('bots')"
        :class="[
          'relative min-h-[48px] rounded-2xl flex flex-col items-center justify-center gap-0.5 transition-colors cursor-pointer',
          store.state.workspace === 'bots' && !store.state.showInboxModal && !store.state.showWorkhubModal && !mobileMenuOpen
            ? 'bg-[#f2b880]/15 text-[#ffe2be]'
            : 'text-slate-400 hover:text-slate-200'
        ]"
      >
        <span class="relative inline-flex items-center justify-center w-8 h-5">
          <svg viewBox="0 0 24 24" width="19" height="19" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
          </svg>
          <span
            v-if="botUnreadTotal > 0"
            class="absolute -top-1 -right-0.5 min-w-[15px] h-[15px] px-1 rounded-full bg-[#f2b880] text-[#090c16] font-mono text-[9.5px] font-bold leading-none inline-flex items-center justify-center"
          >
            {{ botUnreadTotal > 99 ? '99+' : botUnreadTotal }}
          </span>
        </span>
        <span class="text-[11px] font-medium leading-tight">对话</span>
      </button>

      <button
        type="button"
        @click="openInbox"
        :class="[
          'relative min-h-[48px] rounded-2xl flex flex-col items-center justify-center gap-0.5 transition-colors cursor-pointer',
          store.state.showInboxModal
            ? 'bg-[#f2b880]/15 text-[#ffe2be]'
            : 'text-slate-400 hover:text-slate-200'
        ]"
      >
        <span class="relative inline-flex items-center justify-center w-8 h-5">
          <svg viewBox="0 0 24 24" width="19" height="19" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true">
            <polyline points="22 12 16 12 14 15 10 15 8 12 2 12" />
            <path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z" />
          </svg>
          <span
            v-if="inboxUnread > 0"
            class="absolute -top-1 -right-0.5 min-w-[15px] h-[15px] px-1 rounded-full bg-[#f2b880] text-[#090c16] font-mono text-[9.5px] font-bold leading-none inline-flex items-center justify-center"
          >
            {{ inboxUnread > 99 ? '99+' : inboxUnread }}
          </span>
        </span>
        <span class="text-[11px] font-medium leading-tight">情报箱</span>
      </button>

      <button
        type="button"
        @click="openStash"
        :class="[
          'relative min-h-[48px] rounded-2xl flex flex-col items-center justify-center gap-0.5 transition-colors cursor-pointer',
          store.state.showWorkhubModal
            ? 'bg-[#f2b880]/15 text-[#ffe2be]'
            : 'text-slate-400 hover:text-slate-200'
        ]"
      >
        <span class="relative inline-flex items-center justify-center w-8 h-5">
          <svg viewBox="0 0 24 24" width="19" height="19" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true">
            <path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z" />
          </svg>
        </span>
        <span class="text-[11px] font-medium leading-tight">收藏匣</span>
      </button>

      <button
        type="button"
        @click="switchWorkspace('ops')"
        :class="[
          'relative min-h-[48px] rounded-2xl flex flex-col items-center justify-center gap-0.5 transition-colors cursor-pointer',
          store.state.workspace === 'ops' && !store.state.showInboxModal && !store.state.showWorkhubModal && !mobileMenuOpen
            ? 'bg-[#f2b880]/15 text-[#ffe2be]'
            : 'text-slate-400 hover:text-slate-200'
        ]"
      >
        <span class="relative inline-flex items-center justify-center w-8 h-5">
          <svg viewBox="0 0 24 24" width="19" height="19" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true">
            <rect width="20" height="8" x="2" y="2" rx="2" ry="2" />
            <rect width="20" height="8" x="2" y="14" rx="2" ry="2" />
            <line x1="6" x2="6.01" y1="6" y2="6" />
            <line x1="6" x2="6.01" y1="18" y2="18" />
          </svg>
        </span>
        <span class="text-[11px] font-medium leading-tight">中枢</span>
      </button>

      <button
        type="button"
        @click="emit('toggleMobileMenu')"
        :class="[
          'relative min-h-[48px] rounded-2xl flex flex-col items-center justify-center gap-0.5 transition-colors cursor-pointer',
          mobileMenuOpen
            ? 'bg-[#f2b880]/15 text-[#ffe2be]'
            : 'text-slate-400 hover:text-slate-200'
        ]"
      >
        <span class="relative inline-flex items-center justify-center w-8 h-5">
          <svg viewBox="0 0 24 24" width="19" height="19" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true">
            <rect x="3" y="3" width="7" height="7" rx="1.5" />
            <rect x="14" y="3" width="7" height="7" rx="1.5" />
            <rect x="14" y="14" width="7" height="7" rx="1.5" />
            <rect x="3" y="14" width="7" height="7" rx="1.5" />
          </svg>
        </span>
        <span class="text-[11px] font-medium leading-tight">工坊</span>
      </button>
    </div>
  </nav>
</template>
