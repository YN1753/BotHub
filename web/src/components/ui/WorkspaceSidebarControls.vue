<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../../stores/useAppStore'

const props = withDefaults(
  defineProps<{
    desktopCollapsed?: boolean
    shape?: 'pill' | 'rounded'
  }>(),
  {
    shape: 'rounded',
  }
)

const emit = defineEmits<{
  openMobileMenu: []
  toggleDesktopSidebar: []
}>()

const store = useAppStore()
const otherUnreadCount = computed(() => {
  const botUnread = store.state.bots
    .filter((b) => b.id !== store.state.activeBotId)
    .reduce((s, b) => s + (b.unread_count || 0), 0)
  return botUnread + store.unreadInboxCount.value
})
const hasUnreadBadge = computed(
  () => store.unreadInboxCount.value > 0 || store.pendingTodoCount.value > 0
)
</script>

<template>
  <!-- Mobile: In Chat Room ("pill") renders WeChat-style "< 消息" back button; in Ops renders menu button -->
  <button
    v-if="props.shape === 'pill'"
    type="button"
    aria-label="返回消息列表"
    class="btn-tactile md:hidden min-h-[38px] px-2.5 rounded-full inline-flex items-center gap-1 text-xs font-medium text-fg cursor-pointer shrink-0"
    @click="emit('openMobileMenu')"
  >
    <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
      <polyline points="15 18 9 12 15 6" />
    </svg>
    <span>{{ otherUnreadCount > 0 ? `消息(${otherUnreadCount})` : '消息' }}</span>
  </button>
  <button
    v-else
    type="button"
    aria-label="打开菜单"
    class="btn-tactile md:hidden min-h-[42px] min-w-[42px] rounded-xl inline-flex items-center justify-center text-fg cursor-pointer shrink-0"
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
    :class="[
      'btn-tactile relative hidden md:inline-flex items-center justify-center w-9 h-9 text-fg cursor-pointer shrink-0',
      shape === 'pill' ? 'rounded-full' : 'rounded-xl',
    ]"
    @click="emit('toggleDesktopSidebar')"
  >
    <span
      v-if="hasUnreadBadge"
      class="absolute -top-0.5 -right-0.5 w-2 h-2 rounded-full bg-accent shadow-[0_0_6px_#f2b880]"
      aria-hidden="true"
    />
    <svg viewBox="0 0 24 24" :width="shape === 'pill' ? 15 : 16" :height="shape === 'pill' ? 15 : 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <rect x="3" y="3" width="18" height="18" rx="2" />
      <line x1="9" y1="3" x2="9" y2="21" />
      <polyline points="13 9 16 12 13 15" />
    </svg>
  </button>
</template>
