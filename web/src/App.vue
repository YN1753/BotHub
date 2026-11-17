<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore } from './stores/useAppStore'
import StarfieldBg from './components/StarfieldBg.vue'
import LoginGate from './components/LoginGate.vue'
import Sidebar from './components/Sidebar.vue'
import BotWorkspace from './components/BotWorkspace.vue'
import OpsWorkspace from './components/OpsWorkspace.vue'
import CreateBotModal from './components/CreateBotModal.vue'
import InboxModal from './components/InboxModal.vue'
import WorkhubModal from './components/WorkhubModal.vue'
import MemoriesModal from './components/MemoriesModal.vue'
import SkillsModal from './components/SkillsModal.vue'
import LogsModal from './components/LogsModal.vue'
import SettingsModal from './components/SettingsModal.vue'
import AssetModal from './components/AssetModal.vue'
import TaskModal from './components/TaskModal.vue'
import QuickSwitcherModal from './components/QuickSwitcherModal.vue'
import ForwardModal from './components/ForwardModal.vue'

const store = useAppStore()
const mobileSidebarOpen = ref(false)
const desktopCollapsed = ref(localStorage.getItem('bothub_sidebar_collapsed') === '1')
const isOnline = ref(typeof navigator !== 'undefined' ? navigator.onLine : true)

function handleOnlineStatus() {
  isOnline.value = typeof navigator !== 'undefined' ? navigator.onLine : true
  if (isOnline.value && store.state.authenticated) {
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
  () => [store.state.authenticated, store.unreadInboxCount.value] as const,
  ([authed, unread]) => {
    if (typeof document === 'undefined') return
    document.title = authed && unread > 0 ? `(${unread}) Vesper BotHub` : 'Vesper BotHub'
  },
  { immediate: true }
)

onMounted(() => {
  store.checkAuth()
  window.addEventListener('keydown', handleGlobalKeydown)
  window.addEventListener('online', handleOnlineStatus)
  window.addEventListener('offline', handleOnlineStatus)
  window.addEventListener('bothub:unauthorized', handleUnauthorized)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleGlobalKeydown)
  window.removeEventListener('online', handleOnlineStatus)
  window.removeEventListener('offline', handleOnlineStatus)
  window.removeEventListener('bothub:unauthorized', handleUnauthorized)
})
</script>

<template>
  <div class="relative min-h-dvh">
    <StarfieldBg />

    <!-- Initial Loading Gate -->
    <div
      v-if="!store.state.authChecked"
      class="relative z-10 min-h-dvh flex items-center justify-center gap-3 text-xs font-mono text-slate-400"
    >
      <span class="thinking-dots inline-flex items-center gap-1" aria-hidden="true">
        <span /><span /><span />
      </span>
      <span>正在连接 Vesper BotHub 中枢...</span>
    </div>

    <!-- Auth Gate -->
    <LoginGate v-else-if="!store.state.authenticated" />

    <!-- Main Persistent GrokBot Workspace -->
    <div
      v-else
      class="relative z-10 h-dvh w-full flex overflow-hidden bg-[#050811]/66 backdrop-blur-lg"
    >
      <Sidebar
        :mobile-open="mobileSidebarOpen"
        :desktop-collapsed="desktopCollapsed"
        @close-mobile="mobileSidebarOpen = false"
        @toggle-desktop-collapse="toggleDesktopSidebar"
      />

      <main
        :inert="mobileSidebarOpen || undefined"
        class="flex-1 min-w-0 min-h-0 flex flex-col"
      >
        <Transition
          mode="out-in"
          enter-active-class="transition duration-200 ease-out"
          enter-from-class="opacity-0 translate-y-1.5"
          enter-to-class="opacity-100 translate-y-0"
          leave-active-class="transition duration-120 ease-in"
          leave-from-class="opacity-100"
          leave-to-class="opacity-0"
        >
          <BotWorkspace
            v-if="store.state.workspace === 'bots'"
            :desktop-collapsed="desktopCollapsed"
            @open-mobile-menu="mobileSidebarOpen = true"
            @toggle-desktop-sidebar="toggleDesktopSidebar"
          />
          <OpsWorkspace
            v-else
            :desktop-collapsed="desktopCollapsed"
            @open-mobile-menu="mobileSidebarOpen = true"
            @toggle-desktop-sidebar="toggleDesktopSidebar"
          />
        </Transition>
      </main>
    </div>

    <!-- Native <dialog> Modals -->
    <CreateBotModal />
    <InboxModal />
    <WorkhubModal />
    <MemoriesModal />
    <SkillsModal />
    <LogsModal />
    <SettingsModal />
    <AssetModal />
    <TaskModal />
    <QuickSwitcherModal />
    <ForwardModal />

    <!-- Offline Banner -->
    <div
      v-if="!isOnline"
      role="status"
      class="fixed bottom-4 left-1/2 -translate-x-1/2 z-[60] flex items-center gap-2 rounded-full border border-rose-400/45 bg-[#120912]/95 px-4 py-1.5 text-xs text-rose-200 shadow-2xl backdrop-blur-xl"
    >
      <span class="w-2 h-2 rounded-full bg-rose-400 animate-pulse" aria-hidden="true" />
      <span>网络连接已断开，正在等待恢复...</span>
    </div>

    <!-- Non-intrusive Top Toast Banner -->
    <Transition
      enter-active-class="transition duration-250 ease-[var(--ease-spring)]"
      enter-from-class="opacity-0 -translate-y-3 scale-95"
      enter-to-class="opacity-100 translate-y-0 scale-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100 translate-y-0"
      leave-to-class="opacity-0 -translate-y-2 scale-95"
    >
      <div
        v-if="store.state.toastMessage"
        role="status"
        aria-live="polite"
        :class="[
          'fixed top-[max(0.875rem,env(safe-area-inset-top,0.875rem))] left-1/2 -translate-x-1/2 sm:left-auto sm:translate-x-0 sm:right-5 z-[60] max-w-[min(92vw,24rem)] flex items-center gap-2.5 rounded-xl bg-[#0a0f1f]/96 px-3.5 py-2.5 text-xs text-slate-100 shadow-2xl backdrop-blur-xl border',
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
      </div>
    </Transition>
  </div>
</template>
