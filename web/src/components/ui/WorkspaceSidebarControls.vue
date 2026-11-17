<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../../stores/useAppStore'

withDefaults(
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
const hasUnreadBadge = computed(
  () => store.unreadInboxCount.value > 0 || store.pendingTodoCount.value > 0
)
</script>

<template>
  <button
    type="button"
    aria-label="打开菜单"
    :class="[
      'btn-tactile md:hidden inline-flex items-center justify-center text-slate-200 cursor-pointer shrink-0',
      shape === 'pill' ? 'min-h-[40px] min-w-[40px] rounded-full' : 'min-h-[44px] min-w-[44px] rounded-xl',
    ]"
    @click="emit('openMobileMenu')"
  >
    <svg viewBox="0 0 24 24" :width="shape === 'pill' ? 17 : 18" :height="shape === 'pill' ? 17 : 18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
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
      'btn-tactile relative hidden md:inline-flex items-center justify-center w-9 h-9 text-slate-200 cursor-pointer shrink-0',
      shape === 'pill' ? 'rounded-full' : 'rounded-xl',
    ]"
    @click="emit('toggleDesktopSidebar')"
  >
    <span
      v-if="hasUnreadBadge"
      class="absolute -top-0.5 -right-0.5 w-2 h-2 rounded-full bg-[#f2b880] shadow-[0_0_6px_#f2b880]"
      aria-hidden="true"
    />
    <svg viewBox="0 0 24 24" :width="shape === 'pill' ? 15 : 16" :height="shape === 'pill' ? 15 : 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <rect x="3" y="3" width="18" height="18" rx="2" />
      <line x1="9" y1="3" x2="9" y2="21" />
      <polyline points="13 9 16 12 13 15" />
    </svg>
  </button>
</template>
