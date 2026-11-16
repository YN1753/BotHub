<script setup lang="ts">
import { ref, onMounted } from 'vue'
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

const store = useAppStore()
const mobileSidebarOpen = ref(false)
const desktopCollapsed = ref(localStorage.getItem('bothub_sidebar_collapsed') === '1')

function toggleDesktopSidebar() {
  desktopCollapsed.value = !desktopCollapsed.value
  localStorage.setItem('bothub_sidebar_collapsed', desktopCollapsed.value ? '1' : '0')
}

onMounted(() => {
  store.checkAuth()
})
</script>

<template>
  <div class="relative min-h-dvh">
    <StarfieldBg />

    <!-- Initial Loading Gate -->
    <div
      v-if="!store.state.authChecked"
      class="relative z-10 min-h-dvh flex items-center justify-center text-xs font-mono text-slate-400"
    >
      正在连接 Vesper BotHub 中枢...
    </div>

    <!-- Auth Gate -->
    <LoginGate v-else-if="!store.state.authenticated" />

    <!-- Main GrokBot Workspace -->
    <div
      v-else
      class="relative z-10 h-dvh w-full flex overflow-hidden bg-[#050811]/70 backdrop-blur-lg"
    >
      <Sidebar
        :mobile-open="mobileSidebarOpen"
        :desktop-collapsed="desktopCollapsed"
        @close-mobile="mobileSidebarOpen = false"
        @toggle-desktop-collapse="toggleDesktopSidebar"
      />

      <main class="flex-1 min-w-0 min-h-0 flex flex-col">
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
      </main>
    </div>

    <!-- Modals -->
    <CreateBotModal />
    <InboxModal />
    <MemoriesModal />
    <LogsModal />
    <SettingsModal />
    <AssetModal />
    <TaskModal />

    <!-- Global Toast -->
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0 translate-y-2"
      enter-to-class="opacity-100 translate-y-0"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="store.state.toastMessage"
        class="fixed bottom-5 right-5 z-[60] max-w-sm rounded-xl border border-[#f2b880]/40 bg-[#090d1a]/95 px-4 py-2.5 text-xs text-slate-100 shadow-2xl backdrop-blur-xl"
      >
        {{ store.state.toastMessage }}
      </div>
    </Transition>
  </div>
</template>
