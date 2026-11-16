<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore } from './stores/useAppStore'
import StarfieldBg from './components/StarfieldBg.vue'
import LoginGate from './components/LoginGate.vue'
import Sidebar from './components/Sidebar.vue'
import BotWorkspace from './components/BotWorkspace.vue'
import OpsWorkspace from './components/OpsWorkspace.vue'
import CreateBotModal from './components/CreateBotModal.vue'
import InboxModal from './components/InboxModal.vue'
import MemoriesModal from './components/MemoriesModal.vue'
import LogsModal from './components/LogsModal.vue'
import SettingsModal from './components/SettingsModal.vue'
import AssetModal from './components/AssetModal.vue'
import TaskModal from './components/TaskModal.vue'
import QuickSwitcherModal from './components/QuickSwitcherModal.vue'

const store = useAppStore()
const mobileSidebarOpen = ref(false)
const desktopCollapsed = ref(localStorage.getItem('bothub_sidebar_collapsed') === '1')

function toggleDesktopSidebar() {
  desktopCollapsed.value = !desktopCollapsed.value
  localStorage.setItem('bothub_sidebar_collapsed', desktopCollapsed.value ? '1' : '0')
}

function handleGlobalKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    if (store.state.authenticated) {
      store.state.showQuickSwitcher = !store.state.showQuickSwitcher
    }
    return
  }
  if (e.key === 'Escape' && mobileSidebarOpen.value) {
    mobileSidebarOpen.value = false
  }
}

onMounted(() => {
  store.checkAuth()
  window.addEventListener('keydown', handleGlobalKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleGlobalKeydown)
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
    <MemoriesModal />
    <LogsModal />
    <SettingsModal />
    <AssetModal />
    <TaskModal />
    <QuickSwitcherModal />

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
        class="fixed top-[max(0.875rem,env(safe-area-inset-top,0.875rem))] left-1/2 -translate-x-1/2 sm:left-auto sm:translate-x-0 sm:right-5 z-[60] max-w-[min(92vw,24rem)] flex items-center gap-2.5 rounded-xl border border-[#f2b880]/45 bg-[#0a0f1f]/96 px-3.5 py-2.5 text-xs text-slate-100 shadow-2xl backdrop-blur-xl"
      >
        <span class="w-1.5 h-1.5 rounded-full bg-[#f2b880] shrink-0" aria-hidden="true" />
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
