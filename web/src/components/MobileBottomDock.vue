<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../stores/useAppStore'

export type MobileImTab = 'chats' | 'contacts' | 'stash' | 'ops'

const props = defineProps<{
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

const unreadStash = computed(() => store.unreadStashCount.value)

function isSelected(tab: MobileImTab): boolean {
  if (tab === 'stash') return store.state.showWorkhubModal
  return !store.state.showWorkhubModal && props.activeTab === tab
}
</script>

<template>
  <nav
    aria-label="移动端主导航"
    class="md:hidden shrink-0 border-t border-line bg-sidebar px-2 pt-1.5 pb-[max(0.375rem,env(safe-area-inset-bottom,0.375rem))]"
  >
    <div class="grid grid-cols-4 gap-1">
      <button
        type="button"
        :aria-current="isSelected('chats') ? 'page' : undefined"
        @click="emit('selectTab', 'chats')"
        :class="[
          'relative min-h-[50px] flex flex-col items-center justify-center gap-0.5 transition-transform duration-150 active:scale-92 cursor-pointer',
          isSelected('chats') ? 'text-accent-fg' : 'text-fg-muted hover:text-fg'
        ]"
      >
        <span :class="['relative inline-flex items-center justify-center w-9 h-6 transition-transform duration-150', isSelected('chats') ? '-translate-y-0.5 scale-105' : '']">
          <svg viewBox="0 0 24 24" width="21" height="21" :fill="isSelected('chats') ? 'currentColor' : 'none'" stroke="currentColor" :stroke-width="isSelected('chats') ? '1.2' : '1.85'" aria-hidden="true">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
          </svg>
          <span
            v-if="totalUnreadMessages > 0"
            class="absolute -top-1 -right-1 min-w-[16px] h-[16px] px-1 rounded-full bg-danger text-white font-mono text-xs font-bold leading-none inline-flex items-center justify-center"
          >
            {{ totalUnreadMessages > 99 ? '99+' : totalUnreadMessages }}
          </span>
        </span>
        <span :class="['text-xs leading-tight whitespace-nowrap', isSelected('chats') ? 'font-semibold' : 'font-medium']">消息</span>
        <span v-if="isSelected('chats')" class="w-1 h-1 rounded-full bg-accent" aria-hidden="true" />
      </button>

      <button
        type="button"
        :aria-current="isSelected('contacts') ? 'page' : undefined"
        @click="emit('selectTab', 'contacts')"
        :class="[
          'relative min-h-[50px] flex flex-col items-center justify-center gap-0.5 transition-transform duration-150 active:scale-92 cursor-pointer',
          isSelected('contacts') ? 'text-accent-fg' : 'text-fg-muted hover:text-fg'
        ]"
      >
        <span :class="['relative inline-flex items-center justify-center w-9 h-6 transition-transform duration-150', isSelected('contacts') ? '-translate-y-0.5 scale-105' : '']">
          <svg viewBox="0 0 24 24" width="21" height="21" :fill="isSelected('contacts') ? 'currentColor' : 'none'" stroke="currentColor" :stroke-width="isSelected('contacts') ? '1.3' : '1.85'" aria-hidden="true">
            <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
            <circle cx="9" cy="7" r="4" />
            <path d="M22 21v-2a4 4 0 0 0-3-3.87" fill="none" />
            <path d="M16 3.13a4 4 0 0 1 0 7.75" fill="none" />
          </svg>
        </span>
        <span :class="['text-xs leading-tight whitespace-nowrap', isSelected('contacts') ? 'font-semibold' : 'font-medium']">好友</span>
        <span v-if="isSelected('contacts')" class="w-1 h-1 rounded-full bg-accent" aria-hidden="true" />
      </button>

      <button
        type="button"
        :aria-current="isSelected('stash') ? 'page' : undefined"
        @click="emit('selectTab', 'stash')"
        :class="[
          'relative min-h-[50px] flex flex-col items-center justify-center gap-0.5 transition-transform duration-150 active:scale-92 cursor-pointer',
          isSelected('stash') ? 'text-accent-fg' : 'text-fg-muted hover:text-fg'
        ]"
      >
        <span :class="['relative inline-flex items-center justify-center w-9 h-6 transition-transform duration-150', isSelected('stash') ? '-translate-y-0.5 scale-105' : '']">
          <svg viewBox="0 0 24 24" width="21" height="21" :fill="isSelected('stash') ? 'currentColor' : 'none'" stroke="currentColor" :stroke-width="isSelected('stash') ? '1.2' : '1.85'" aria-hidden="true">
            <path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z" />
          </svg>
          <span
            v-if="unreadStash > 0"
            class="absolute -top-1 -right-1 min-w-[16px] h-[16px] px-1 rounded-full bg-danger text-white font-mono text-xs font-bold leading-none inline-flex items-center justify-center"
          >
            {{ unreadStash > 99 ? '99+' : unreadStash }}
          </span>
        </span>
        <span :class="['text-xs leading-tight whitespace-nowrap', isSelected('stash') ? 'font-semibold' : 'font-medium']">收藏</span>
        <span v-if="isSelected('stash')" class="w-1 h-1 rounded-full bg-accent" aria-hidden="true" />
      </button>

      <button
        type="button"
        :aria-current="isSelected('ops') ? 'page' : undefined"
        @click="emit('selectTab', 'ops')"
        :class="[
          'relative min-h-[50px] flex flex-col items-center justify-center gap-0.5 transition-transform duration-150 active:scale-92 cursor-pointer',
          isSelected('ops') ? 'text-accent-fg' : 'text-fg-muted hover:text-fg'
        ]"
      >
        <span :class="['relative inline-flex items-center justify-center w-9 h-6 transition-transform duration-150', isSelected('ops') ? '-translate-y-0.5 scale-105' : '']">
          <svg viewBox="0 0 24 24" width="21" height="21" :fill="isSelected('ops') ? 'currentColor' : 'none'" stroke="currentColor" :stroke-width="isSelected('ops') ? '1.2' : '1.85'" aria-hidden="true">
            <rect width="8" height="8" x="3" y="3" rx="2" />
            <rect width="8" height="8" x="13" y="3" rx="2" />
            <rect width="8" height="8" x="3" y="13" rx="2" />
            <rect width="8" height="8" x="13" y="13" rx="2" />
          </svg>
        </span>
        <span :class="['text-xs leading-tight whitespace-nowrap', isSelected('ops') ? 'font-semibold' : 'font-medium']">中枢</span>
        <span v-if="isSelected('ops')" class="w-1 h-1 rounded-full bg-accent" aria-hidden="true" />
      </button>
    </div>
  </nav>
</template>
