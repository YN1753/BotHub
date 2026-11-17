<script setup lang="ts">
import { ref, computed, nextTick, useId, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { botStateLabel } from '../../utils/format'
import { closePopoverWithTransition } from '../../utils/topLayer'
import type { Bot } from '../../types/bothub'
import BotAvatar from '../BotAvatar.vue'
import BotGroupAvatar from '../BotGroupAvatar.vue'
import SegmentedTabs from '../SegmentedTabs.vue'

defineProps<{
  bot: Bot | null
  isGroupBot: boolean
  groupMembers: Bot[]
  activeSpeakerId?: string
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
  { id: 'stream', label: '对话' },
  { id: 'routines', label: '例程' },
  { id: 'config', label: '设定' },
  { id: 'activity', label: '脉搏' },
]

const topicMenuOpen = ref(false)
const topicWrapRef = ref<HTMLElement | null>(null)
const summarizing = ref(false)

const moreBtnRef = ref<HTMLButtonElement | null>(null)
const morePopoverRef = ref<HTMLElement | null>(null)
const moreFallbackOpen = ref(false)
const hasNativePopover =
  typeof HTMLElement !== 'undefined' && Object.hasOwn(HTMLElement.prototype, 'popover')

const activeThreadTitle = computed(() => {
  if (!store.state.activeThreadId) return '主时间线'
  const found = store.state.botThreads.find((t) => t.id === store.state.activeThreadId)
  return found?.title || '独立话题'
})

async function handleCreateNewTopic() {
  topicMenuOpen.value = false
  await store.createThread('')
}

async function handlePickTopic(id: string) {
  topicMenuOpen.value = false
  await store.switchThread(id)
}

async function handleQuickSummarize() {
  if (summarizing.value) return
  summarizing.value = true
  topicMenuOpen.value = false
  try {
    await store.summarizeThread(store.state.activeThreadId || 'main', true)
  } finally {
    summarizing.value = false
  }
}

function positionFallbackPopover() {
  const btn = moreBtnRef.value
  const pop = morePopoverRef.value
  if (!btn || !pop) return
  pop.removeAttribute('data-closing')
  const rect = btn.getBoundingClientRect()
  const spaceBelow = window.innerHeight - rect.bottom
  pop.setAttribute('data-origin', spaceBelow < 180 ? 'bottom' : 'top')
  if (window.CSS?.supports?.('position-area', 'block-end')) return
  const panelWidth = Math.max(pop.offsetWidth, 184)
  const left = Math.max(8, Math.min(rect.right - panelWidth, window.innerWidth - panelWidth - 8))
  const top = spaceBelow < 180 ? Math.max(8, rect.top - 180) : rect.bottom + 8
  pop.style.top = `${top}px`
  pop.style.left = `${left}px`
}

function onMoreBeforeToggle(e: Event) {
  const toggleEvt = e as ToggleEvent
  if (toggleEvt.newState === 'open') {
    morePopoverRef.value?.removeAttribute('data-closing')
    nextTick(() => positionFallbackPopover())
  }
}

function handleMoreButtonClick() {
  if (!hasNativePopover) {
    moreFallbackOpen.value = !moreFallbackOpen.value
  }
}

async function closeMoreMenu() {
  if (hasNativePopover && morePopoverRef.value) {
    await closePopoverWithTransition(morePopoverRef.value)
  }
  moreFallbackOpen.value = false
}

function handleOutsideClick(e: MouseEvent) {
  const t = e.target as Node
  if (topicMenuOpen.value && topicWrapRef.value && !topicWrapRef.value.contains(t)) {
    topicMenuOpen.value = false
  }
  if (!hasNativePopover && moreFallbackOpen.value) {
    if (!moreBtnRef.value?.contains(t) && !morePopoverRef.value?.contains(t)) {
      moreFallbackOpen.value = false
    }
  }
}

onMounted(() => {
  document.addEventListener('click', handleOutsideClick)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleOutsideClick)
})
</script>

<template>
  <!-- Floating Dynamic-Island Glass HUD Header (Zero Heavy Full-Width Divider Lines) -->
  <header class="relative z-20 px-3.5 sm:px-6 pt-[max(0.75rem,env(safe-area-inset-top,0.75rem))] pb-1.5 flex items-center justify-between gap-2 shrink-0">
    <!-- Left Floating Identity + Inline Topic Switcher Capsule -->
    <div class="flex items-center gap-2 min-w-0">
      <button
        type="button"
        aria-label="打开菜单"
        class="btn-tactile md:hidden min-h-[40px] min-w-[40px] inline-flex items-center justify-center rounded-full text-slate-200 cursor-pointer shrink-0"
        @click="emit('openMobileMenu')"
      >
        <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
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
        class="btn-tactile relative hidden md:inline-flex items-center justify-center w-9 h-9 rounded-full text-slate-200 cursor-pointer shrink-0"
        @click="emit('toggleDesktopSidebar')"
      >
        <span
          v-if="store.unreadInboxCount.value > 0 || store.pendingTodoCount.value > 0"
          class="absolute -top-0.5 -right-0.5 w-2 h-2 rounded-full bg-[#f2b880] shadow-[0_0_6px_#f2b880]"
          aria-hidden="true"
        />
        <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <rect x="3" y="3" width="18" height="18" rx="2" />
          <line x1="9" y1="3" x2="9" y2="21" />
          <polyline points="13 9 16 12 13 15" />
        </svg>
      </button>

      <div ref="topicWrapRef" class="relative flex items-center min-w-0">
        <div class="well-recessed rounded-full pl-1.5 pr-2 py-1 flex items-center gap-2 min-w-0">
          <BotGroupAvatar
            v-if="isGroupBot"
            :members="groupMembers"
            :fallback-style="bot?.avatar_style"
            :state="bot?.state"
            :active-speaker-id="activeSpeakerId"
            size="sm"
          />
          <BotAvatar v-else :avatar-style="bot?.avatar_style" :state="bot?.state" size="sm" />

          <h1 class="text-xs sm:text-[13.5px] font-semibold text-white truncate max-w-[9rem] sm:max-w-[13rem]">
            {{ bot?.name || 'BotHub' }}
          </h1>

          <span
            v-if="bot?.state && bot.state !== 'idle'"
            class="hidden md:inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-[#f2b880]/15 text-[#ffe2be] font-mono text-[10px] leading-none shrink-0"
          >
            <span class="w-1.5 h-1.5 rounded-full bg-[#f2b880] animate-pulse" aria-hidden="true" />
            <span>{{ botStateLabel(bot?.state) }}</span>
          </span>

          <!-- Inline Topic Pill (Replaces Full-Width Thread Bar Clutter) -->
          <template v-if="bot && store.state.botView === 'stream'">
            <span class="h-3.5 w-px bg-[#fff8ee]/12 shrink-0" aria-hidden="true" />
            <button
              type="button"
              :aria-expanded="topicMenuOpen"
              title="切换或新建独立话题"
              @click.stop="topicMenuOpen = !topicMenuOpen"
              class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] text-slate-300 hover:text-[#ffe2be] hover:bg-white/[0.07] cursor-pointer max-w-[7.5rem] sm:max-w-[9.5rem]"
            >
              <span class="text-[#f2b880]/80 font-mono">#</span>
              <span class="truncate">{{ activeThreadTitle }}</span>
              <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true" :class="['shrink-0 text-slate-400 transition-transform duration-150', topicMenuOpen ? 'rotate-180 text-[#f2b880]' : '']">
                <polyline points="6 9 12 15 18 9" />
              </svg>
            </button>
          </template>
        </div>

        <!-- Inline Topic Switcher Popover -->
        <Transition
          enter-active-class="transition-[opacity,transform] duration-180 ease-[var(--ease-spring)] motion-reduce:duration-100"
          enter-from-class="opacity-0 scale-95 -translate-y-1.5 motion-reduce:scale-100 motion-reduce:translate-y-0"
          enter-to-class="opacity-100 scale-100 translate-y-0"
          leave-active-class="transition-[opacity,transform] duration-120 ease-in motion-reduce:duration-75"
          leave-from-class="opacity-100 scale-100 translate-y-0"
          leave-to-class="opacity-0 scale-95 -translate-y-1 motion-reduce:scale-100 motion-reduce:translate-y-0"
        >
          <div
            v-if="topicMenuOpen && bot"
            class="aero-glass absolute left-0 top-full mt-2 z-50 w-64 rounded-2xl border border-[#fff8ee]/16 p-1.5 origin-top-left flex flex-col gap-0.5"
          >
            <button
              type="button"
              @click="handlePickTopic('')"
              :class="[
                'w-full flex items-center justify-between px-3 py-2 rounded-xl text-xs cursor-pointer text-left',
                !store.state.activeThreadId ? 'bg-[#f2b880]/18 text-[#ffe2be] font-medium' : 'text-slate-200 hover:bg-white/[0.06]'
              ]"
            >
              <span># 主时间线</span>
              <span v-if="!store.state.activeThreadId" class="font-mono text-[10px] text-[#f2b880]">当前</span>
            </button>

            <div v-if="store.state.botThreads.length > 0" class="max-h-48 overflow-y-auto flex flex-col gap-0.5">
              <div
                v-for="th in store.state.botThreads"
                :key="th.id"
                :class="[
                  'flex items-center justify-between gap-1 px-2.5 py-1.5 rounded-xl text-xs',
                  store.state.activeThreadId === th.id ? 'bg-[#f2b880]/18 text-[#ffe2be] font-medium' : 'text-slate-300 hover:bg-white/[0.06]'
                ]"
              >
                <button
                  type="button"
                  @click="handlePickTopic(th.id)"
                  class="flex-1 min-w-0 text-left truncate cursor-pointer"
                >
                  # {{ th.title }}
                  <span v-if="th.msg_count > 0" class="font-mono text-[10px] opacity-60 ml-0.5">({{ th.msg_count }})</span>
                </button>
                <button
                  type="button"
                  title="删除此话题"
                  @click.stop="store.deleteThread(th.id)"
                  class="px-1.5 py-0.5 rounded text-[11px] text-slate-400 hover:text-rose-300 cursor-pointer shrink-0"
                >
                  ×
                </button>
              </div>
            </div>

            <div class="my-1 border-t border-[#fff8ee]/10" />
            <button
              v-if="store.state.timeline.length >= 2"
              type="button"
              :disabled="summarizing"
              @click="handleQuickSummarize"
              class="w-full flex items-center gap-2 px-3 py-1.5 rounded-xl text-xs text-emerald-300 hover:bg-emerald-500/12 cursor-pointer text-left disabled:opacity-50"
            >
              <span>{{ summarizing ? '正在提炼...' : '提炼当前话题存为记忆' }}</span>
            </button>
            <button
              type="button"
              @click="handleCreateNewTopic"
              class="w-full flex items-center gap-2 px-3 py-1.5 rounded-xl text-xs text-[#ffe2be] hover:bg-[#f2b880]/15 cursor-pointer text-left font-medium"
            >
              <span>+ 开启新独立话题</span>
            </button>
          </div>
        </Transition>
      </div>
    </div>

    <!-- Center Floating View Switcher Island (Desktop / Tablet) -->
    <div v-if="bot" class="hidden sm:inline-flex">
      <SegmentedTabs v-model="store.state.botView" :items="VIEW_TABS" aria-label="Bot 子视图" />
    </div>

    <!-- Right Compact Collaboration & More Capsule -->
    <div v-if="bot" class="flex items-center gap-1.5 shrink-0">
      <button
        type="button"
        @click="store.state.showGroupDrawer = true"
        :title="isGroupBot ? '管理群成员与协作策略' : '拉人发起协同群聊'"
        class="btn-tactile-gold min-h-[38px] sm:min-h-[34px] inline-flex items-center gap-1.5 px-3 rounded-full text-xs font-medium cursor-pointer shrink-0"
      >
        <svg viewBox="0 0 24 24" width="13.5" height="13.5" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
          <circle cx="8.5" cy="7" r="4" />
          <line x1="20" y1="8" x2="20" y2="14" />
          <line x1="23" y1="11" x2="17" y2="11" />
        </svg>
        <span class="hidden md:inline">{{ isGroupBot ? `成员 ${groupMembers.length}` : '协同' }}</span>
      </button>

      <div class="relative shrink-0">
        <button
          ref="moreBtnRef"
          type="button"
          :popovertarget="hasNativePopover ? moreMenuId : undefined"
          aria-label="更多操作"
          title="更多操作"
          @click="handleMoreButtonClick"
          class="btn-tactile min-h-[38px] min-w-[38px] sm:min-h-[34px] sm:min-w-[34px] inline-flex items-center justify-center rounded-full text-slate-300 cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="15" height="15" fill="currentColor" aria-hidden="true">
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
            !hasNativePopover ? (moreFallbackOpen ? 'fixed right-4 top-14 flex flex-col gap-0.5 z-50' : 'hidden') : ''
          ]"
        >
          <button
            type="button"
            @click="emit('openChatSearch'); closeMoreMenu()"
            class="min-h-[40px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" /></svg>
            <span>查找聊天记录</span>
          </button>
          <button
            type="button"
            @click="store.state.showLogsModal = true; closeMoreMenu()"
            class="min-h-[40px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12" /></svg>
            <span>系统推送与流水</span>
          </button>
          <button
            type="button"
            @click="store.toggleSound(); closeMoreMenu()"
            class="min-h-[40px] sm:min-h-[34px] w-full flex items-center justify-between gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
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
            @click="store.toggleAmbientMotion(); closeMoreMenu()"
            class="min-h-[40px] sm:min-h-[34px] w-full flex items-center justify-between gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
          >
            <span class="inline-flex items-center gap-2.5">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><circle cx="12" cy="12" r="10" /><polygon v-if="store.state.ambientMotionPaused" points="10 8 16 12 10 16 10 8" /><template v-else><line x1="10" y1="15" x2="10" y2="9" /><line x1="14" y1="15" x2="14" y2="9" /></template></svg>
              <span>{{ store.state.ambientMotionPaused ? '恢复背景动效' : '暂停背景动效' }}</span>
            </span>
            <span class="font-mono text-[10px]" :class="!store.state.ambientMotionPaused ? 'text-[#f2b880]' : 'text-slate-500'">
              {{ store.state.ambientMotionPaused ? '已暂停' : '运行' }}
            </span>
          </button>
          <button
            type="button"
            @click="emit('exportMarkdown'); closeMoreMenu()"
            class="min-h-[40px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-slate-200 hover:bg-[#f2b880]/14 hover:text-[#f2b880] cursor-pointer text-left"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="7 10 12 15 17 10" /><line x1="12" y1="15" x2="12" y2="3" /></svg>
            <span>导出 Markdown</span>
          </button>
          <div class="my-1 border-t border-[#fff8ee]/10" />
          <button
            type="button"
            @click="emit('clearTimeline'); closeMoreMenu()"
            class="min-h-[40px] sm:min-h-[34px] w-full flex items-center gap-2.5 px-3 rounded-lg text-xs text-rose-300 hover:bg-rose-500/15 cursor-pointer text-left"
          >
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /></svg>
            <span>清空对话流</span>
          </button>
        </div>
      </div>
    </div>
  </header>

  <!-- Slim Mobile Subview + Quick Stash/Todo Bar (<640px) -->
  <div
    v-if="bot"
    class="sm:hidden px-3.5 pb-1.5 flex items-center justify-between gap-2 shrink-0"
  >
    <div class="min-w-0 flex-1 overflow-x-auto no-scrollbar">
      <SegmentedTabs v-model="store.state.botView" :items="VIEW_TABS" aria-label="移动端 Bot 子视图" />
    </div>
    <div class="flex items-center gap-1.5 shrink-0">
      <button
        type="button"
        title="收藏与闪念匣"
        aria-label="收藏与闪念匣"
        @click="store.state.workhubTab = 'stash'; store.state.showWorkhubModal = true"
        class="btn-tactile relative min-h-[36px] inline-flex items-center gap-1 px-2.5 rounded-full text-[11px] cursor-pointer"
      >
        <span v-if="store.unreadStashCount.value > 0" class="w-1.5 h-1.5 rounded-full bg-rose-400" aria-hidden="true" />
        <span>闪念</span>
      </button>
      <button
        type="button"
        title="个人行动台"
        aria-label="个人行动台"
        @click="store.state.workhubTab = 'todos'; store.state.showWorkhubModal = true"
        class="btn-tactile relative min-h-[36px] inline-flex items-center gap-1 px-2.5 rounded-full text-[11px] cursor-pointer"
      >
        <span v-if="store.pendingTodoCount.value > 0" class="w-1.5 h-1.5 rounded-full bg-cyan-400" aria-hidden="true" />
        <span>行动</span>
      </button>
    </div>
  </div>
</template>
