<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../stores/useAppStore'

export type MobileImTab = 'chats' | 'contacts' | 'stash' | 'ops'

defineProps<{
  activeTab: MobileImTab
}>()

const emit = defineEmits<{
  selectTab: [tab: MobileImTab]
}>()

const store = useAppStore()

const totalUnreadMessages = computed(() => {
  const botUnread = store.state.bots.reduce((sum, b) => sum + (b.unread_count || 0), 0)
  return botUnread + store.unreadInboxCount.value
})
</script>

<template>
  <nav
    aria-label="移动端主导航"
    class="md:hidden shrink-0 border-t border-[#fff8ee]/12 bg-[#080c1a]/95 backdrop-blur-2xl px-2.5 pt-1.5 pb-[max(0.375rem,env(safe-area-inset-bottom,0.375rem))]"
  >
    <div class="grid grid-cols-4 gap-1">
      <button
        type="button"
        @click="emit('selectTab', 'chats')"
        :class="[
          'relative min-h-[49px] rounded-2xl flex flex-col items-center justify-center gap-0.5 transition-colors cursor-pointer',
          activeTab === 'chats' && !store.state.showWorkhubModal
            ? 'bg-[#f2b880]/15 text-[#ffe2be]'
            : 'text-slate-400 hover:text-slate-200'
        ]"
      >
        <span class="relative inline-flex items-center justify-center w-9 h-5">
          <svg viewBox="0 0 24 24" width="19" height="19" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
          </svg>
          <span
            v-if="totalUnreadMessages > 0"
            class="absolute -top-1 -right-0.5 min-w-[16px] h-[16px] px-1 rounded-full bg-rose-500 text-white font-mono text-[9.5px] font-bold leading-none inline-flex items-center justify-center"
          >
            {{ totalUnreadMessages > 99 ? '99+' : totalUnreadMessages }}
          </span>
        </span>
        <span class="text-[11px] font-medium leading-tight">消息</span>
      </button>

      <button
        type="button"
        @click="emit('selectTab', 'contacts')"
        :class="[
          'relative min-h-[49px] rounded-2xl flex flex-col items-center justify-center gap-0.5 transition-colors cursor-pointer',
          activeTab === 'contacts' && !store.state.showWorkhubModal
            ? 'bg-[#f2b880]/15 text-[#ffe2be]'
            : 'text-slate-400 hover:text-slate-200'
        ]"
      >
        <span class="relative inline-flex items-center justify-center w-9 h-5">
          <svg viewBox="0 0 24 24" width="19" height="19" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true">
            <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
            <circle cx="9" cy="7" r="4" />
            <path d="M22 21v-2a4 4 0 0 0-3-3.87" />
            <path d="M16 3.13a4 4 0 0 1 0 7.75" />
          </svg>
        </span>
        <span class="text-[11px] font-medium leading-tight">好友</span>
      </button>

      <button
        type="button"
        @click="emit('selectTab', 'stash')"
        :class="[
          'relative min-h-[49px] rounded-2xl flex flex-col items-center justify-center gap-0.5 transition-colors cursor-pointer',
          store.state.showWorkhubModal
            ? 'bg-[#f2b880]/15 text-[#ffe2be]'
            : 'text-slate-400 hover:text-slate-200'
        ]"
      >
        <span class="relative inline-flex items-center justify-center w-9 h-5">
          <svg viewBox="0 0 24 24" width="19" height="19" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true">
            <path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z" />
          </svg>
        </span>
        <span class="text-[11px] font-medium leading-tight">收藏</span>
      </button>

      <button
        type="button"
        @click="emit('selectTab', 'ops')"
        :class="[
          'relative min-h-[49px] rounded-2xl flex flex-col items-center justify-center gap-0.5 transition-colors cursor-pointer',
          activeTab === 'ops' && !store.state.showWorkhubModal
            ? 'bg-[#f2b880]/15 text-[#ffe2be]'
            : 'text-slate-400 hover:text-slate-200'
        ]"
      >
        <span class="relative inline-flex items-center justify-center w-9 h-5">
          <svg viewBox="0 0 24 24" width="19" height="19" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true">
            <rect width="20" height="8" x="2" y="2" rx="2" ry="2" />
            <rect width="20" height="8" x="2" y="14" rx="2" ry="2" />
            <line x1="6" x2="6.01" y1="6" y2="6" />
            <line x1="6" x2="6.01" y1="18" y2="18" />
          </svg>
        </span>
        <span class="text-[11px] font-medium leading-tight">中枢</span>
      </button>
    </div>
  </nav>
</template>
