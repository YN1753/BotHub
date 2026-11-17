<script setup lang="ts">
import { ref, nextTick, useId, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { botStateLabel, groupModeLabel } from '../../utils/format'
import type { Bot } from '../../types/bothub'
import BotAvatar from '../BotAvatar.vue'
import BotGroupAvatar from '../BotGroupAvatar.vue'
import SegmentedTabs from '../SegmentedTabs.vue'

defineProps<{
  bot: Bot | null
  isGroupBot: boolean
  groupMembers: Bot[]
  desktopCollapsed?: boolean
}>()

const emit = defineEmits<{
  openMobileMenu: []
  toggleDesktopSidebar: []
  openChatSearch: []
  exportMarkdown: []
  clearTimeline: []
}>()

const store = useAppStore()
const moreMenuId = useId()

const VIEW_TABS = [
  { id: 'stream', label: '对话流' },
  { id: 'routines', label: '例程' },
  { id: 'config', label: '能力与记忆' },
  { id: 'activity', label: '运行脉搏' },
]

const moreBtnRef = ref<HTMLButtonElement | null>(null)
const morePopoverRef = ref<HTMLElement | null>(null)
const moreFallbackOpen = ref(false)
const hasNativePopover =
  typeof HTMLElement !== 'undefined' && Object.hasOwn(HTMLElement.prototype, 'popover')

function botModeLabel(mode?: string): string {
  if (mode === 'companion') return '陪伴'
  if (mode === 'worker') return '执行'
  return '全能'
}

function stateBadgeClass(state?: string): string {
  if (state === 'thinking') return 'border-amber-400/40 bg-amber-400/10 text-amber-300'
  if (state === 'working') return 'border-[#f2b880]/45 bg-[#f2b880]/12 text-[#f2b880]'
  if (state === 'done') return 'border-emerald-400/40 bg-emerald-400/10 text-emerald-300'
  if (state === 'blocked') return 'border-rose-400/45 bg-rose-400/10 text-rose-300'
  return 'border-[#fff8ee]/15 text-slate-400'
}

function positionFallbackPopover() {
  const btn = moreBtnRef.value
  const pop = morePopoverRef.value
  if (!btn || !pop) return
  if (window.CSS?.supports?.('position-area', 'block-end')) return
  const rect = btn.getBoundingClientRect()
  const panelWidth = Math.max(pop.offsetWidth, 184)
  const left = Math.max(8, Math.min(rect.right - panelWidth, window.innerWidth - panelWidth - 8))
  const spaceBelow = window.innerHeight - rect.bottom
  const top = spaceBelow < 180 ? Math.max(8, rect.top - 160) : rect.bottom + 8
  pop.style.top = `${top}px`
  pop.style.left = `${left}px`
}

function onMoreBeforeToggle(e: Event) {
  const toggleEvt = e as ToggleEvent
  if (toggleEvt.newState === 'open') {
    nextTick(() => positionFallbackPopover())
  }
}

function handleMoreButtonClick() {
  if (!hasNativePopover) {
    moreFallbackOpen.value = !moreFallbackOpen.value
  }
}

function closeMoreMenu() {
  if (hasNativePopover && morePopoverRef.value?.matches(':popover-open')) {
    morePopoverRef.value.hidePopover()
  }
  moreFallbackOpen.value = false
}

function handleOutsideFallback(e: MouseEvent) {
  if (hasNativePopover || !moreFallbackOpen.value) return
  const t = e.target as Node
  if (moreBtnRef.value?.contains(t) || morePopoverRef.value?.contains(t)) return
  moreFallbackOpen.value = false
}

onMounted(() => {
  document.addEventListener('click', handleOutsideFallback)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleOutsideFallback)
})
</script>

<template>
  <!-- Minimal Bot Top Header -->
  <header class="px-3.5 sm:px-6 pt-[max(0.75rem,env(safe-area-inset-top,0.75rem))] pb-3 border-b border-[#fff8ee]/8 bg-[#080c18]/50 backdrop-blur-md flex flex-wrap items-center justify-between gap-2.5">
    <div class="flex items-center gap-2.5 min-w-0">
      <button
        type="button"
        aria-label="打开菜单"
        class="md:hidden min-h-[44px] min-w-[44px] inline-flex items-center justify-center rounded-xl border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
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
        class="relative hidden md:inline-flex items-center justify-center w-9 h-9 rounded-xl border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
        @click="emit('toggleDesktopSidebar')"
      >
        <span
          v-if="store.unreadInboxCount.value > 0 || store.pendingTodoCount.value > 0"
          class="absolute -top-0.5 -right-0.5 w-2 h-2 rounded-full bg-[#f2b880] shadow-[0_0_6px_#f2b880]"
          aria-hidden="true"
        />
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <rect x="3" y="3" width="18" height="18" rx="2" />
          <line x1="9" y1="3" x2="9" y2="21" />
          <polyline points="13 9 16 12 13 15" />
        </svg>
      </button>
      <BotGroupAvatar
        v-if="isGroupBot"
        :members="groupMembers"
        :fallback-style="bot?.avatar_style"
        :state="bot?.state"
        size="md"
      />
      <BotAvatar v-else :avatar-style="bot?.avatar_style" :state="bot?.state" size="md" />
      <div class="min-w-0">
        <div class="flex items-center gap-2">
          <h1 class="text-sm md:text-base font-semibold text-white truncate">{{ bot?.name || 'BotHub 工作台' }}</h1>
          <span
            v-if="isGroupBot"
            class="hidden sm:inline-flex font-mono text-[10px] px-2 py-0.5 rounded-full border border-[#fff8ee]/14 bg-[#fff8ee]/[0.04] text-slate-300 leading-none shrink-0"
          >
            {{ groupModeLabel(bot?.group_mode) }}
          </span>
          <span
            v-else-if="bot"
            :class="[
              'hidden sm:inline-flex font-mono text-[10px] px-2 py-0.5 rounded-full border leading-none shrink-0',
              bot.bot_mode === 'companion'
                ? 'border-emerald-400/35 bg-emerald-500/10 text-emerald-200'
                : bot.bot_mode === 'worker'
                ? 'border-cyan-400/35 bg-cyan-500/10 text-cyan-200'
                : 'border-[#fff8ee]/14 bg-[#fff8ee]/[0.04] text-slate-300',
            ]"
          >
            {{ botModeLabel(bot.bot_mode) }}
          </span>
          <span
            v-if="bot?.state && bot.state !== 'idle'"
            :class="['font-mono text-[10px] px-2 py-0.5 rounded-full border leading-none shrink-0', stateBadgeClass(bot?.state)]"
          >
            {{ botStateLabel(bot?.state) }}
          </span>
        </div>
        <p class="hidden sm:block text-xs text-slate-400/90 truncate">{{ bot?.role_title }}</p>
      </div>
    </div>

    <div v-if="bot" class="flex items-center gap-2 max-w-full ml-auto">
      <!-- Gliding Physics Segmented Control (Desktop/Tablet) -->
      <div class="hidden sm:inline-flex">
        <SegmentedTabs v-model="store.state.botView" :items="VIEW_TABS" aria-label="Bot 子视图" />
      </div>

      <!-- Quick Personal Assistant Workhub Pill -->
      <button
        type="button"
        @click="store.state.showWorkhubModal = true"
        title="打开个人行动台（待办与工作文档）"
        class="hidden md:inline-flex min-h-[36px] items-center gap-1.5 px-2.5 rounded-full border border-[#fff8ee]/12 bg-[#060913]/85 hover:border-cyan-400/45 hover:text-cyan-200 text-xs text-slate-300 cursor-pointer shrink-0"
      >
        <span>行动台</span>
        <span
          v-if="store.pendingTodoCount.value > 0"
          class="px-1.5 py-0.5 rounded-full bg-cyan-400/20 text-cyan-200 font-mono text-[10px] leading-none"
        >
          {{ store.pendingTodoCount.value }}
        </span>
      </button>

      <!-- QQ/WeChat Member / Invite Pill -->
      <button
        type="button"
        @click="store.state.showGroupDrawer = true"
        :title="isGroupBot ? '查看群成员与拉人管理' : '临时拉人发起协同群聊'"
        class="min-h-[44px] sm:min-h-[36px] inline-flex items-center gap-1.5 px-3 rounded-full border border-[#f2b880]/30 bg-[#f2b880]/10 hover:bg-[#f2b880]/18 hover:border-[#f2b880]/55 text-xs font-medium text-[#f7cba0] cursor-pointer shrink-0"
      >
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
          <circle cx="8.5" cy="7" r="4" />
          <line x1="20" y1="8" x2="20" y2="14" />
          <line x1="23" y1="11" x2="17" y2="11" />
        </svg>
        <span class="hidden sm:inline">{{ isGroupBot ? `群成员 (${groupMembers.length})` : '拉人建群' }}</span>
      </button>

      <!-- Native Top-Layer Popover + Anchor Positioned More Actions -->
      <div class="relative shrink-0">
        <button
          ref="moreBtnRef"
          type="button"
          :popovertarget="hasNativePopover ? moreMenuId : undefined"
          aria-label="更多操作"
          title="更多操作"
          @click="handleMoreButtonClick"
          class="min-h-[44px] min-w-[44px] sm:min-h-[36px] sm:min-w-[36px] inline-flex items-center justify-center rounded-full border border-[#fff8ee]/10 bg-[#060913]/85 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor" aria-hidden="true">
            <circle cx="5" cy="12" r="1.75" />
            <circle cx="12" cy="12" r="1.75" />
            <circle cx="19" cy="12" r="1.75" />
          </svg>
        </button>

        <div
          ref="morePopoverRef"
          :id="moreMenuId"
          :popover="hasNativePopover ? 'auto' : undefined"
          @beforetoggle="onMoreBeforeToggle"
          :class="[
            'vesper-popover anchored-end',
            !hasNativePopover ? (moreFallbackOpen ? 'fixed right-4 top-16 flex flex-col gap-0.5 z-50' : 'hidden') : ''
          ]"
        >
          <button
            type="button"
            @click="emit('openChatSearch'); closeMoreMenu()"
            class="min-h-[44px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" /></svg>
            <span>查找聊天记录</span>
          </button>
          <button
            type="button"
            @click="store.toggleSound(); closeMoreMenu()"
            class="min-h-[44px] sm:min-h-[34px] w-full flex items-center justify-between gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
          >
            <span class="inline-flex items-center gap-2.5">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" /><path d="M15.54 8.46a5 5 0 0 1 0 7.07" /></svg>
              <span>回复提示音</span>
            </span>
            <span class="font-mono text-[10px]" :class="store.state.soundEnabled ? 'text-[#f2b880]' : 'text-slate-500'">
              {{ store.state.soundEnabled ? '开' : '关' }}
            </span>
          </button>
          <button
            type="button"
            @click="store.loadDashboard(false); closeMoreMenu()"
            class="min-h-[44px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><polyline points="23 4 23 10 17 10" /><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" /></svg>
            <span>刷新状态</span>
          </button>
          <button
            type="button"
            @click="emit('exportMarkdown'); closeMoreMenu()"
            class="min-h-[44px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="7 10 12 15 17 10" /><line x1="12" y1="15" x2="12" y2="3" /></svg>
            <span>导出 Markdown</span>
          </button>
          <div class="my-1 border-t border-[#fff8ee]/10" />
          <button
            type="button"
            @click="emit('clearTimeline'); closeMoreMenu()"
            class="min-h-[44px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-rose-300 hover:bg-rose-500/15 cursor-pointer text-left"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /></svg>
            <span>清空对话流</span>
          </button>
        </div>
      </div>
    </div>
  </header>

  <!-- Dedicated Mobile View Switcher + Workhub Strip (<640px) -->
  <div
    v-if="bot"
    class="sm:hidden px-3 py-1.5 border-b border-[#fff8ee]/8 bg-[#060913]/80 flex items-center justify-between gap-2 shrink-0"
  >
    <div class="min-w-0 flex-1 overflow-x-auto no-scrollbar">
      <SegmentedTabs v-model="store.state.botView" :items="VIEW_TABS" aria-label="移动端 Bot 子视图" />
    </div>
    <button
      type="button"
      @click="store.state.showWorkhubModal = true"
      class="min-h-[34px] inline-flex items-center gap-1 px-2.5 rounded-full border border-[#fff8ee]/12 bg-[#0a0f1e] text-[11px] text-slate-300 shrink-0 cursor-pointer"
    >
      <span>行动台</span>
      <span
        v-if="store.pendingTodoCount.value > 0"
        class="px-1.5 py-0.5 rounded-full bg-cyan-400/20 text-cyan-200 font-mono text-[9.5px] leading-none"
      >
        {{ store.pendingTodoCount.value }}
      </span>
    </button>
  </div>
</template>
