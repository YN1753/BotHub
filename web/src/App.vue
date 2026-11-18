<script setup lang="ts">
import { ref, reactive, watch, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore } from './stores/useAppStore'
import { BotHubApi } from './api/client'
import { lazyOverlayChunk } from './utils/lazyComponent'
import { resolveTabSlideTransition, type DirectionalTransitionName } from './utils/motion'
import StarfieldBg from './components/StarfieldBg.vue'
import LoginGate from './components/LoginGate.vue'
import Sidebar from './components/Sidebar.vue'
import BotWorkspace from './components/BotWorkspace.vue'
import MobileImRoster from './components/MobileImRoster.vue'
import MobileBottomDock, { type MobileImTab } from './components/MobileBottomDock.vue'

// On-demand route/modal code-splitting to shrink cold-boot bundle substantially
const OpsWorkspace = lazyOverlayChunk(() => import('./components/OpsWorkspace.vue'))
const CreateBotModal = lazyOverlayChunk(() => import('./components/CreateBotModal.vue'))
const InboxModal = lazyOverlayChunk(() => import('./components/InboxModal.vue'))
const WorkhubModal = lazyOverlayChunk(() => import('./components/WorkhubModal.vue'))
const MemoriesModal = lazyOverlayChunk(() => import('./components/MemoriesModal.vue'))
const SkillsModal = lazyOverlayChunk(() => import('./components/SkillsModal.vue'))
const LogsModal = lazyOverlayChunk(() => import('./components/LogsModal.vue'))
const SettingsModal = lazyOverlayChunk(() => import('./components/SettingsModal.vue'))
const AssetModal = lazyOverlayChunk(() => import('./components/AssetModal.vue'))
const TaskModal = lazyOverlayChunk(() => import('./components/TaskModal.vue'))
const QuickSwitcherModal = lazyOverlayChunk(() => import('./components/QuickSwitcherModal.vue'))
const ForwardModal = lazyOverlayChunk(() => import('./components/ForwardModal.vue'))

const store = useAppStore()
const mobileSidebarOpen = ref(false)
const desktopCollapsed = ref(localStorage.getItem('bothub_sidebar_collapsed') === '1')
const isOnline = ref(typeof navigator !== 'undefined' ? navigator.onLine : true)
const isAppShell = ref(
  typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('app_shell') === '1'
)
const mobileImTab = ref<MobileImTab>('chats')
const mobileInChatRoom = ref(isAppShell.value)
const workspaceTransition = ref<DirectionalTransitionName>('vesper-slide-forward')

function handleSelectMobileImTab(tab: MobileImTab) {
  mobileInChatRoom.value = false
  if (tab === 'stash') {
    store.state.showWorkhubModal = true
    return
  }
  store.state.showWorkhubModal = false
  store.state.showInboxModal = false
  mobileImTab.value = tab
  store.state.workspace = tab === 'ops' ? 'ops' : 'bots'
}

watch(
  () => store.state.workspace,
  (next, prev) => {
    workspaceTransition.value = resolveTabSlideTransition(next, prev || 'bots', ['bots', 'ops'])
  }
)

// Once opened, keep modal components mounted so closing <dialog> transitions stay smooth
const mountedFlags = reactive({
  ops: false,
  createBot: false,
  inbox: false,
  workhub: false,
  memories: false,
  skills: false,
  logs: false,
  settings: false,
  asset: false,
  task: false,
  quickSwitcher: false,
  forward: false,
})

watch(() => store.state.workspace, (v) => { if (v === 'ops') mountedFlags.ops = true })
watch(() => store.state.showCreateBotModal, (v) => { if (v) mountedFlags.createBot = true })
watch(() => store.state.showInboxModal, (v) => { if (v) mountedFlags.inbox = true })
watch(() => store.state.showWorkhubModal, (v) => { if (v) mountedFlags.workhub = true })
watch(() => store.state.showMemoriesModal, (v) => { if (v) mountedFlags.memories = true })
watch(() => store.state.showSkillsModal, (v) => { if (v) mountedFlags.skills = true })
watch(() => store.state.showLogsModal, (v) => { if (v) mountedFlags.logs = true })
watch(() => store.state.showSettingsModal, (v) => { if (v) mountedFlags.settings = true })
watch(() => store.state.editingAsset, (v) => { if (v !== undefined) mountedFlags.asset = true })
watch(() => store.state.editingTask, (v) => { if (v !== undefined) mountedFlags.task = true })
watch(() => store.state.showQuickSwitcher, (v) => { if (v) mountedFlags.quickSwitcher = true })
watch(() => store.state.forwardingMessage, (v) => { if (v) mountedFlags.forward = true })

function handleOnlineStatus() {
  isOnline.value = typeof navigator !== 'undefined' ? navigator.onLine : true
  if (!isOnline.value) {
    store.stopSSE()
    return
  }
  if (store.state.authenticated) {
    store.startSSE()
    void store.loadDashboard(false)
  }
}

function toggleDesktopSidebar() {
  desktopCollapsed.value = !desktopCollapsed.value
  localStorage.setItem('bothub_sidebar_collapsed', desktopCollapsed.value ? '1' : '0')
}

function isEditableElement(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable
}

function handleGlobalKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === ',') {
    e.preventDefault()
    if (store.state.authenticated) {
      store.state.showSettingsModal = !store.state.showSettingsModal
    }
    return
  }
  if (e.altKey && !e.metaKey && !e.ctrlKey && !isEditableElement(e.target) && store.state.authenticated && !document.querySelector('dialog[open]')) {
    if (e.key === '1') {
      e.preventDefault()
      store.state.workspace = 'bots'
      return
    }
    if (e.key === '2') {
      e.preventDefault()
      store.state.workspace = 'ops'
      return
    }
  }
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    if (store.state.authenticated) {
      store.state.showQuickSwitcher = !store.state.showQuickSwitcher
    }
    return
  }
  if (e.key === '?' && !e.metaKey && !e.ctrlKey && !e.altKey && !isEditableElement(e.target)) {
    if (store.state.authenticated && !document.querySelector('dialog[open]')) {
      e.preventDefault()
      store.state.showQuickSwitcher = true
      return
    }
  }
  if (e.key === 'Escape' && mobileSidebarOpen.value) {
    mobileSidebarOpen.value = false
  }
}

function handleUnauthorized() {
  if (!store.state.authenticated) return
  store.stopSSE()
  store.state.authenticated = false
  store.notify('会话登录已失效，请重新验证访问密钥')
}

watch(
  () => [
    store.state.authenticated,
    store.unreadInboxCount.value,
    store.state.bots.reduce((sum, b) => sum + (b.unread_count || 0), 0),
    store.activeBot.value?.name || '',
  ] as const,
  ([authed, inboxUnread, botUnread, activeName]) => {
    if (typeof document === 'undefined') return
    const total = inboxUnread + botUnread
    const base = activeName ? `${activeName} · BotHub` : 'BotHub'
    document.title = authed && total > 0 ? `(${total}) ${base}` : base
  },
  { immediate: true }
)

function handleViewportResize() {
  if (typeof window !== 'undefined' && window.innerWidth >= 768 && mobileSidebarOpen.value) {
    mobileSidebarOpen.value = false
  }
}

watch(
  () => store.state.authenticated,
  async (authed) => {
    if (!authed || typeof window === 'undefined') return
    const params = new URLSearchParams(window.location.search)

    const entry = params.get('entry')
    if (entry) {
      params.delete('entry')
      const restEntry = params.toString()
      window.history.replaceState(
        {},
        '',
        window.location.pathname + (restEntry ? `?${restEntry}` : '') + window.location.hash
      )
      switch (entry) {
        case 'ops':
          store.state.workspace = 'ops'
          break
        case 'create_bot':
          store.state.workspace = 'bots'
          store.state.showCreateBotModal = true
          break
        case 'bot_config':
          store.state.workspace = 'bots'
          store.state.botView = 'config'
          break
        case 'create_asset':
          store.state.workspace = 'ops'
          store.state.editingAsset = null
          break
        case 'create_task':
          store.state.workspace = 'ops'
          store.state.editingTask = null
          break
        case 'skills':
          store.state.showSkillsModal = true
          break
        case 'memories':
          store.state.showMemoriesModal = true
          break
        case 'logs':
          store.state.showLogsModal = true
          break
        case 'settings':
          store.state.showSettingsModal = true
          break
        case 'inbox':
          store.state.showInboxModal = true
          break
        case 'stash':
          store.state.showWorkhubModal = true
          break
      }
    }

    if (params.get('auto_pair_app') !== '1') return
    params.delete('auto_pair_app')
    const rest = params.toString()
    const cleanUrl = window.location.pathname + (rest ? `?${rest}` : '') + window.location.hash
    window.history.replaceState({}, '', cleanUrl)
    try {
      const res = await BotHubApi.createAppPairCode()
      store.notify('正在无感回跳激活手机客户端实时通知...')
      window.location.href = res.deep_link
    } catch {
      // Ignore if pair creation fails
    }
  },
  { immediate: true }
)

onMounted(() => {
  store.checkAuth()
  window.addEventListener('keydown', handleGlobalKeydown)
  window.addEventListener('online', handleOnlineStatus)
  window.addEventListener('offline', handleOnlineStatus)
  window.addEventListener('bothub:unauthorized', handleUnauthorized)
  window.addEventListener('resize', handleViewportResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleGlobalKeydown)
  window.removeEventListener('online', handleOnlineStatus)
  window.removeEventListener('offline', handleOnlineStatus)
  window.removeEventListener('bothub:unauthorized', handleUnauthorized)
  window.removeEventListener('resize', handleViewportResize)
})
</script>

<template>
  <div
    class="relative min-h-dvh"
    :data-ambient-paused="store.state.ambientMotionPaused || undefined"
    :data-app-shell="isAppShell ? '1' : undefined"
  >
    <StarfieldBg />

    <!-- Initial Loading Gate -->
    <div
      v-if="!store.state.authChecked"
      class="relative z-10 min-h-dvh flex items-center justify-center gap-3 text-xs font-mono text-slate-400"
    >
      <span class="thinking-dots inline-flex items-center gap-1" aria-hidden="true">
        <span /><span /><span />
      </span>
      <span>正在连接 BotHub 中枢...</span>
    </div>

    <!-- Auth Gate -->
    <LoginGate v-else-if="!store.state.authenticated" />

    <!-- Main Persistent GrokBot Workspace (Floating Spatial Islands on Desktop, Full-Bleed on Mobile) -->
    <div
      v-else
      :class="[
        'relative z-10 h-dvh w-full flex overflow-hidden md:p-2.5 transition-[gap] duration-250',
        desktopCollapsed ? 'md:gap-0' : 'md:gap-2.5'
      ]"
    >
      <Sidebar
        :mobile-open="mobileSidebarOpen"
        :desktop-collapsed="desktopCollapsed"
        @close-mobile="mobileSidebarOpen = false"
        @toggle-desktop-collapse="toggleDesktopSidebar"
      />

      <main
        :inert="mobileSidebarOpen || undefined"
        class="workspace-island flex-1 min-w-0 min-h-0 flex flex-col overflow-hidden md:rounded-[24px] md:border md:border-[#fff8ee]/12"
      >
        <MobileImRoster
          v-if="!isAppShell && store.state.workspace === 'bots' && !mobileInChatRoom"
          :mode="mobileImTab === 'contacts' ? 'contacts' : 'chats'"
          @enter-chat="mobileInChatRoom = true"
        />

        <Transition :name="workspaceTransition" mode="out-in">
          <BotWorkspace
            v-if="store.state.workspace === 'bots'"
            key="bots"
            :class="!isAppShell && !mobileInChatRoom ? 'hidden md:flex' : ''"
            :desktop-collapsed="desktopCollapsed"
            @open-mobile-menu="isAppShell ? (mobileSidebarOpen = true) : (mobileInChatRoom = false)"
            @toggle-desktop-sidebar="toggleDesktopSidebar"
          />
          <OpsWorkspace
            v-else-if="mountedFlags.ops || store.state.workspace === 'ops'"
            key="ops"
            :desktop-collapsed="desktopCollapsed"
            @open-mobile-menu="mobileSidebarOpen = true"
            @toggle-desktop-sidebar="toggleDesktopSidebar"
          />
        </Transition>

        <MobileBottomDock
          v-if="!isAppShell && !(store.state.workspace === 'bots' && mobileInChatRoom)"
          :active-tab="store.state.workspace === 'ops' ? 'ops' : mobileImTab"
          @select-tab="handleSelectMobileImTab"
        />
      </main>
    </div>

    <!-- Lazy-Loaded Native <dialog> Modals -->
    <CreateBotModal v-if="mountedFlags.createBot || store.state.showCreateBotModal" />
    <InboxModal v-if="mountedFlags.inbox || store.state.showInboxModal" />
    <WorkhubModal v-if="mountedFlags.workhub || store.state.showWorkhubModal" />
    <MemoriesModal v-if="mountedFlags.memories || store.state.showMemoriesModal" />
    <SkillsModal v-if="mountedFlags.skills || store.state.showSkillsModal" />
    <LogsModal v-if="mountedFlags.logs || store.state.showLogsModal" />
    <SettingsModal v-if="mountedFlags.settings || store.state.showSettingsModal" />
    <AssetModal v-if="mountedFlags.asset || store.state.editingAsset !== undefined" />
    <TaskModal v-if="mountedFlags.task || store.state.editingTask !== undefined" />
    <QuickSwitcherModal v-if="mountedFlags.quickSwitcher || store.state.showQuickSwitcher" />
    <ForwardModal v-if="mountedFlags.forward || Boolean(store.state.forwardingMessage)" />

    <!-- Offline Banner -->
    <Transition
      enter-active-class="transition-[opacity,transform] duration-220 ease-[var(--ease-spring)] motion-reduce:duration-100"
      enter-from-class="opacity-0 translate-y-3 scale-95 motion-reduce:translate-y-0 motion-reduce:scale-100"
      enter-to-class="opacity-100 translate-y-0 scale-100"
      leave-active-class="transition-[opacity,transform] duration-150 ease-in motion-reduce:duration-75"
      leave-from-class="opacity-100 translate-y-0 scale-100"
      leave-to-class="opacity-0 translate-y-2 scale-95 motion-reduce:translate-y-0 motion-reduce:scale-100"
    >
      <div
        v-if="!isOnline"
        role="status"
        class="fixed bottom-4 left-1/2 -translate-x-1/2 z-[60] flex items-center gap-2 rounded-full border border-rose-400/45 bg-[#120912]/95 px-4 py-1.5 text-xs text-rose-200 shadow-2xl backdrop-blur-xl"
      >
        <span class="w-2 h-2 rounded-full bg-rose-400 animate-pulse" aria-hidden="true" />
        <span>网络连接已断开，正在等待恢复...</span>
      </div>
    </Transition>

    <!-- Non-intrusive Top Toast Banner -->
    <Transition
      mode="out-in"
      enter-active-class="transition-[opacity,transform] duration-240 ease-[var(--ease-spring)] motion-reduce:duration-100"
      enter-from-class="opacity-0 -translate-y-3 scale-95 motion-reduce:translate-y-0 motion-reduce:scale-100"
      enter-to-class="opacity-100 translate-y-0 scale-100"
      leave-active-class="transition-[opacity,transform] duration-130 ease-in motion-reduce:duration-75"
      leave-from-class="opacity-100 translate-y-0 scale-100"
      leave-to-class="opacity-0 -translate-y-2 scale-96 motion-reduce:translate-y-0 motion-reduce:scale-100"
    >
      <div
        v-if="store.state.toastMessage"
        :key="store.state.toastSeq"
        role="status"
        aria-live="polite"
        @mouseenter="store.pauseToast"
        @mouseleave="store.resumeToast"
        @focusin="store.pauseToast"
        @focusout="store.resumeToast"
        :class="[
          'toast-host overflow-hidden fixed top-[max(0.875rem,env(safe-area-inset-top,0.875rem))] left-1/2 -translate-x-1/2 sm:left-auto sm:translate-x-0 sm:right-5 z-[60] max-w-[min(92vw,24rem)] flex items-center gap-2.5 rounded-xl bg-[#0a0f1f]/96 px-3.5 py-2.5 text-xs text-slate-100 shadow-2xl backdrop-blur-xl border',
          /失败|错误|异常|超时|失效|无法/.test(store.state.toastMessage)
            ? 'border-rose-400/55'
            : 'border-[#f2b880]/45'
        ]"
      >
        <span
          :class="[
            'w-1.5 h-1.5 rounded-full shrink-0',
            /失败|错误|异常|超时|失效|无法/.test(store.state.toastMessage) ? 'bg-rose-400' : 'bg-[#f2b880]'
          ]"
          aria-hidden="true"
        />
        <span class="flex-1 leading-snug">{{ store.state.toastMessage }}</span>
        <button
          type="button"
          aria-label="关闭提示"
          @click="store.state.toastMessage = ''"
          class="p-1 rounded-md text-slate-400 hover:text-white cursor-pointer shrink-0"
        >
          <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
        <span
          aria-hidden="true"
          :class="[
            'toast-progress-bar pointer-events-none absolute inset-x-0 bottom-0 h-[1.5px] opacity-65',
            /失败|错误|异常|超时|失效|无法/.test(store.state.toastMessage) ? 'bg-rose-400' : 'bg-[#f2b880]'
          ]"
        />
      </div>
    </Transition>
  </div>
</template>
