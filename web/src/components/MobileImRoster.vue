<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { botStateDotClass, formatChatTime } from '../utils/format'
import { resolveGroupMemberBots } from '../utils/botLook'
import type { Bot } from '../types/bothub'
import BotAvatar from './BotAvatar.vue'
import BotGroupAvatar from './BotGroupAvatar.vue'
import VesperBrandIcon from './ui/VesperBrandIcon.vue'

defineProps<{
  mode?: 'chats' | 'contacts'
}>()

const emit = defineEmits<{
  enterChat: [botId: string]
}>()

const store = useAppStore()
const searchQuery = ref('')
const plusOpen = ref(false)

const inboxUnread = computed(() => store.unreadInboxCount.value)
const botUnreadTotal = computed(() =>
  store.state.bots.reduce((sum, b) => sum + (b.unread_count || 0), 0)
)
const totalUnread = computed(() => inboxUnread.value + botUnreadTotal.value)
const latestNotif = computed(() => store.state.notifications[0] || null)

function isGroup(b: Bot): boolean {
  return Boolean(b.is_group)
}

function resolveGroupMembers(b: Bot): Bot[] {
  return resolveGroupMemberBots(b, store.state.bots)
}

function botDraftPreview(b: Bot): string {
  const activeKey = b.active_thread_id ? `${b.id}::${b.active_thread_id}` : b.id
  if (store.state.draftsByBot[activeKey]) return store.state.draftsByBot[activeKey]
  if (store.state.draftsByBot[b.id]) return store.state.draftsByBot[b.id]
  return ''
}

const sortedBots = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return store.state.bots
    .filter((b) => {
      if (!q) return true
      return `${b.name} ${b.role_title} ${b.traits || ''} ${b.last_summary}`.toLowerCase().includes(q)
    })
    .toSorted((a, b) => {
      if (Boolean(a.is_pinned) !== Boolean(b.is_pinned)) return a.is_pinned ? -1 : 1
      const au = (a.unread_count || 0) > 0
      const bu = (b.unread_count || 0) > 0
      if (au !== bu) return au ? -1 : 1
      return String(b.updated_at || '').localeCompare(String(a.updated_at || ''))
    })
})

const groupList = computed(() => sortedBots.value.filter((b) => isGroup(b)))
const friendList = computed(() => sortedBots.value.filter((b) => !isGroup(b)))

function handleSelectConversation(botId: string) {
  if (!store.state.isStreaming) {
    // Do NOT await network round-trip before entering chat room: timeline swaps in 0ms from cache!
    void store.selectBot(botId)
  }
  emit('enterChat', botId)
}

function openCreateBot() {
  plusOpen.value = false
  store.state.createModalMode = 'bot'
  store.state.showCreateBotModal = true
}

function openCreateGroup() {
  plusOpen.value = false
  store.state.createModalMode = 'group'
  store.state.showCreateBotModal = true
}
</script>

<template>
  <section class="md:hidden flex-1 min-h-0 flex flex-col overflow-hidden">
    <!-- Compact Single-Row WeChat / QQ Mobile IM Header -->
    <header class="relative z-20 px-4 pt-[max(0.65rem,env(safe-area-inset-top,0.65rem))] pb-2 border-b border-line bg-sidebar flex items-center justify-between gap-3 shrink-0">
      <h1 class="text-lg font-bold text-fg leading-tight truncate">
        <template v-if="mode === 'contacts'">好友</template>
        <template v-else>{{ totalUnread > 0 ? `消息 (${totalUnread})` : '消息' }}</template>
      </h1>

      <div class="relative flex items-center gap-2 shrink-0">
        <button
          type="button"
          aria-label="添加好友或发起群聊"
          @click="plusOpen = !plusOpen"
          class="btn-tactile w-9 h-9 rounded-full inline-flex items-center justify-center text-fg cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <line x1="12" y1="5" x2="12" y2="19" />
            <line x1="5" y1="12" x2="19" y2="12" />
          </svg>
        </button>

        <button
          type="button"
          aria-label="设置"
          @click="store.state.showSettingsModal = true"
          class="btn-tactile w-9 h-9 rounded-full inline-flex items-center justify-center text-fg-sub cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true">
            <circle cx="12" cy="12" r="3" />
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06-.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
          </svg>
        </button>

        <div
          v-if="plusOpen"
          class="fixed inset-0 z-30"
          @click="plusOpen = false"
        />
        <div
          v-if="plusOpen"
          class="aero-glass absolute right-0 top-11 z-40 w-40 rounded-lg p-1.5 flex flex-col gap-0.5"
        >
          <button
            type="button"
            @click="openCreateGroup"
            class="w-full px-3 py-2.5 rounded-xl text-left text-xs text-fg hover:bg-hover cursor-pointer"
          >
            发起群聊
          </button>
          <button
            type="button"
            @click="openCreateBot"
            class="w-full px-3 py-2.5 rounded-xl text-left text-xs text-fg hover:bg-hover cursor-pointer"
          >
            添加 AI 好友
          </button>
          <button
            type="button"
            @click="plusOpen = false; store.state.showWorkhubModal = true"
            class="w-full px-3 py-2.5 rounded-xl text-left text-xs text-fg hover:bg-hover cursor-pointer"
          >
            快速收藏
          </button>
        </div>
      </div>
    </header>

    <!-- Recessed Search Bar -->
    <div class="px-3.5 py-2 shrink-0">
      <div class="flex items-center gap-2 rounded-full border border-line bg-canvas px-3.5 py-2">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" class="text-fg-muted shrink-0" aria-hidden="true">
          <circle cx="11" cy="11" r="8" />
          <line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
        <input
          v-model="searchQuery"
          type="search"
          :placeholder="mode === 'contacts' ? '搜索好友或群聊' : '搜索'"
          class="flex-1 min-w-0 bg-transparent text-sm text-fg placeholder:text-fg-muted focus:outline-none"
        />
      </div>
    </div>

    <!-- Mode A: Contacts Directory ("好友") -->
    <div v-if="mode === 'contacts'" class="flex-1 min-h-0 overflow-y-auto pb-4">
      <div
        v-if="!searchQuery.trim()"
        class="mx-3.5 mt-0.5 mb-3 rounded-xl border border-line bg-canvas divide-y divide-line"
      >
        <button
          type="button"
          @click="openCreateBot"
          class="w-full px-3.5 py-3 flex items-center justify-between gap-3 text-left cursor-pointer"
        >
          <span class="text-base font-medium text-fg">添加 AI 好友</span>
          <span class="text-xs text-fg-muted">&gt;</span>
        </button>
        <button
          type="button"
          @click="openCreateGroup"
          class="w-full px-3.5 py-3 flex items-center justify-between gap-3 text-left cursor-pointer"
        >
          <span class="text-base font-medium text-fg">发起群聊</span>
          <span class="text-xs text-fg-muted">&gt;</span>
        </button>
        <button
          type="button"
          @click="store.state.showInboxModal = true"
          class="w-full px-3.5 py-3 flex items-center justify-between gap-3 text-left cursor-pointer"
        >
          <span class="text-base font-medium text-fg">服务通知</span>
          <span class="inline-flex items-center gap-1.5">
            <span
              v-if="inboxUnread > 0"
              class="min-w-[17px] h-[17px] px-1 rounded-full bg-danger text-white font-mono text-xs font-bold leading-none inline-flex items-center justify-center"
            >
              {{ inboxUnread > 99 ? '99+' : inboxUnread }}
            </span>
            <span class="text-xs text-fg-muted">&gt;</span>
          </span>
        </button>
      </div>

      <template v-if="groupList.length > 0">
        <div class="px-4 py-1.5 text-xs font-semibold text-fg-muted">
          群聊 ({{ groupList.length }})
        </div>
        <div class="divide-y divide-line">
          <button
            v-for="g in groupList"
            :key="g.id"
            type="button"
            @click="handleSelectConversation(g.id)"
            class="w-full px-4 py-2.5 flex items-center gap-3 text-left hover:bg-hover cursor-pointer"
          >
            <div class="w-11 h-11 flex items-center justify-center shrink-0">
              <BotGroupAvatar :members="resolveGroupMembers(g)" :fallback-style="g.avatar_style" :state="g.state" size="md" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-base font-semibold text-fg truncate">{{ g.name }}</div>
              <div class="text-xs text-fg-muted truncate">{{ resolveGroupMembers(g).length }}人</div>
            </div>
          </button>
        </div>
      </template>

      <div class="px-4 pt-3 pb-1.5 text-xs font-semibold text-fg-muted">
        好友 ({{ friendList.length }})
      </div>
      <div class="divide-y divide-line">
        <button
          v-for="f in friendList"
          :key="f.id"
          type="button"
          @click="handleSelectConversation(f.id)"
          class="w-full px-4 py-2.5 flex items-center gap-3 text-left hover:bg-hover cursor-pointer"
        >
          <div class="relative w-11 h-11 flex items-center justify-center shrink-0">
            <BotAvatar :avatar-style="f.avatar_style" :state="f.state" size="md" :seed-key="f.id" />
            <span :class="['absolute bottom-0 right-0 w-2.5 h-2.5 rounded-full ring-2 ring-sidebar', botStateDotClass(f.state)]" />
          </div>
          <div class="flex-1 min-w-0">
            <div class="text-base font-semibold text-fg truncate">{{ f.name }}</div>
            <div class="text-xs text-fg-muted truncate">{{ f.traits || f.role_title || '在线' }}</div>
          </div>
        </button>
      </div>
    </div>

    <!-- Mode B: WeChat / QQ IM Conversation List ("消息") -->
    <div v-else class="flex-1 min-h-0 overflow-y-auto pb-3">
      <!-- Service Notification Channel -->
      <button
        v-if="!searchQuery.trim()"
        type="button"
        @click="store.state.showInboxModal = true"
        class="w-full px-4 py-3 flex items-center gap-3.5 text-left border-b border-line hover:bg-hover cursor-pointer"
      >
        <div class="relative w-12 h-12 flex items-center justify-center shrink-0">
          <VesperBrandIcon class="w-10 h-10" />
          <span
            v-if="inboxUnread > 0"
            class="absolute -top-0.5 -right-0.5 min-w-[17px] h-[17px] px-1 rounded-full bg-danger text-white font-mono text-xs font-bold leading-none inline-flex items-center justify-center"
          >
            {{ inboxUnread > 99 ? '99+' : inboxUnread }}
          </span>
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between gap-2">
            <span class="text-base font-semibold text-fg truncate">服务通知</span>
            <span v-if="latestNotif" class="font-mono text-xs text-fg-muted shrink-0">
              {{ formatChatTime(latestNotif.created_at) }}
            </span>
          </div>
          <div class="mt-0.5 text-sm text-fg-muted truncate">
            <span v-if="inboxUnread > 0" class="text-accent-fg font-medium">[{{ inboxUnread }}条] </span>
            <span>{{ latestNotif ? latestNotif.title : '暂无新通知' }}</span>
          </div>
        </div>
      </button>

      <!-- AI Friend & Group Conversation Rows -->
      <button
        v-for="b in sortedBots"
        :key="b.id"
        type="button"
        @click="handleSelectConversation(b.id)"
        :class="[
          'w-full px-4 py-3 flex items-center gap-3.5 text-left border-b border-line transition-colors cursor-pointer',
          b.is_pinned ? 'bg-canvas' : 'hover:bg-hover'
        ]"
      >
        <div class="relative w-12 h-12 flex items-center justify-center shrink-0">
          <BotGroupAvatar
            v-if="isGroup(b)"
            :members="resolveGroupMembers(b)"
            :fallback-style="b.avatar_style"
            :state="b.state"
            size="md"
          />
          <template v-else>
            <BotAvatar :avatar-style="b.avatar_style" :state="b.state" size="md" :seed-key="b.id" />
            <span :class="['absolute bottom-0 right-0 w-2.5 h-2.5 rounded-full ring-2 ring-sidebar', botStateDotClass(b.state)]" />
          </template>

          <span
            v-if="(b.unread_count || 0) > 0"
            class="absolute -top-1 -right-1 min-w-[17px] h-[17px] px-1 rounded-full bg-danger text-white font-mono text-xs font-bold leading-none inline-flex items-center justify-center"
          >
            {{ b.unread_count > 99 ? '99+' : b.unread_count }}
          </span>
        </div>

        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-1.5 min-w-0">
              <span class="text-base font-semibold text-fg truncate">{{ b.name }}</span>
              <span
                v-if="isGroup(b)"
                class="px-1.5 py-0.5 rounded-lg text-xs font-medium bg-accent-soft border border-accent-border text-accent-fg shrink-0 whitespace-nowrap"
              >
                群聊
              </span>
              <svg
                v-if="b.is_pinned"
                viewBox="0 0 24 24"
                width="11"
                height="11"
                fill="currentColor"
                class="text-accent-fg shrink-0"
                aria-label="已置顶"
              >
                <path d="M16 12V4h1V2H7v2h1v8l-2 2v2h5.2v6h1.6v-6H18v-2l-2-2z" />
              </svg>
            </div>
            <span class="font-mono text-xs text-fg-muted shrink-0 whitespace-nowrap">
              {{ formatChatTime(b.updated_at) }}
            </span>
          </div>

          <div class="mt-0.5 text-sm text-fg-muted truncate">
            <template v-if="botDraftPreview(b)">
              <span class="text-danger font-semibold">[草稿] </span>
              <span class="text-fg">{{ botDraftPreview(b) }}</span>
            </template>
            <template v-else>
              <span v-if="(b.unread_count || 0) > 0" class="text-accent-fg font-medium">[{{ b.unread_count }}条] </span>
              <span>{{ b.last_summary || b.traits || b.role_title || '点击开始对话' }}</span>
            </template>
          </div>
        </div>
      </button>
    </div>
  </section>
</template>
