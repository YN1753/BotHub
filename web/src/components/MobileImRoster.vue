<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { botStateDotClass, formatChatTime } from '../utils/format'
import { resolveGroupMemberBots } from '../utils/botLook'
import type { Bot } from '../types/bothub'
import BotAvatar from './BotAvatar.vue'
import BotGroupAvatar from './BotGroupAvatar.vue'
import VesperBrandIcon from './ui/VesperBrandIcon.vue'

const props = defineProps<{
  mode?: 'chats' | 'contacts'
}>()

const emit = defineEmits<{
  enterChat: [botId: string]
}>()

const store = useAppStore()
const searchQuery = ref('')
const plusOpen = ref(false)
const rosterScope = ref<'all' | 'groups' | 'companion' | 'worker'>('all')

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
      if (props.mode !== 'contacts') {
        if (rosterScope.value === 'groups' && !isGroup(b)) return false
        if (rosterScope.value === 'companion' && (isGroup(b) || b.bot_mode !== 'companion')) return false
        if (rosterScope.value === 'worker' && (isGroup(b) || b.bot_mode === 'companion')) return false
      }
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

async function handleSelectConversation(botId: string) {
  if (!store.state.isStreaming) {
    await store.selectBot(botId)
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
    <!-- WeChat / QQ Mobile IM Header -->
    <header class="relative z-20 px-4 pt-[max(0.75rem,env(safe-area-inset-top,0.75rem))] pb-2.5 border-b border-[#fff8ee]/10 bg-[#070b16]/90 backdrop-blur-xl flex items-center justify-between gap-3 shrink-0">
      <div class="flex items-center gap-2.5 min-w-0">
        <VesperBrandIcon :size="28" />
        <div class="min-w-0">
          <h1 class="text-[16px] font-semibold text-[#fff8ee] leading-tight truncate">
            <template v-if="mode === 'contacts'">通讯录</template>
            <template v-else>{{ totalUnread > 0 ? `消息 (${totalUnread})` : '消息' }}</template>
          </h1>
          <p class="text-[11px] text-slate-400 leading-tight truncate">
            <template v-if="mode === 'contacts'">
              {{ friendList.length }} 位 AI 好友 · {{ groupList.length }} 个群聊
            </template>
            <template v-else>AI 好友与协同群聊实时在线</template>
          </p>
        </div>
      </div>

      <div class="relative flex items-center gap-2 shrink-0">
        <button
          type="button"
          aria-label="添加好友或发起群聊"
          @click="plusOpen = !plusOpen"
          class="btn-tactile w-9 h-9 rounded-full inline-flex items-center justify-center text-slate-200 cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <line x1="12" y1="5" x2="12" y2="19" />
            <line x1="5" y1="12" x2="19" y2="12" />
          </svg>
        </button>

        <button
          type="button"
          aria-label="系统设置"
          @click="store.state.showSettingsModal = true"
          class="btn-tactile w-9 h-9 rounded-full inline-flex items-center justify-center text-slate-300 cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true">
            <circle cx="12" cy="12" r="3" />
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
          </svg>
        </button>

        <!-- WeChat "+" Floating Menu -->
        <div
          v-if="plusOpen"
          class="fixed inset-0 z-30"
          @click="plusOpen = false"
        />
        <div
          v-if="plusOpen"
          class="absolute right-0 top-11 z-40 w-44 rounded-2xl border border-[#fff8ee]/16 bg-[#0d1428]/98 p-1.5 shadow-2xl backdrop-blur-2xl flex flex-col gap-0.5"
        >
          <button
            type="button"
            @click="openCreateGroup"
            class="w-full px-3 py-2.5 rounded-xl text-left text-xs text-slate-100 hover:bg-white/8 cursor-pointer"
          >
            发起协作群聊
          </button>
          <button
            type="button"
            @click="openCreateBot"
            class="w-full px-3 py-2.5 rounded-xl text-left text-xs text-slate-100 hover:bg-white/8 cursor-pointer"
          >
            添加 AI 好友
          </button>
          <button
            type="button"
            @click="plusOpen = false; store.state.showWorkhubModal = true"
            class="w-full px-3 py-2.5 rounded-xl text-left text-xs text-slate-100 hover:bg-white/8 cursor-pointer"
          >
            秒收链接 / 闪念
          </button>
          <button
            type="button"
            @click="plusOpen = false; store.state.showSkillsModal = true"
            class="w-full px-3 py-2.5 rounded-xl text-left text-xs text-slate-100 hover:bg-white/8 cursor-pointer"
          >
            技能工坊
          </button>
        </div>
      </div>
    </header>

    <!-- Search Bar + Optional Scope Filter -->
    <div class="px-3.5 pt-2.5 pb-1.5 shrink-0 flex flex-col gap-2">
      <div class="flex items-center gap-2 rounded-xl border border-[#fff8ee]/12 bg-[#0a0f1f]/90 px-3 py-2">
        <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" class="text-slate-500 shrink-0" aria-hidden="true">
          <circle cx="11" cy="11" r="8" />
          <line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
        <input
          v-model="searchQuery"
          type="search"
          placeholder="搜索 AI 好友、群聊或近期消息..."
          class="flex-1 min-w-0 bg-transparent text-[13px] text-white placeholder-slate-500 focus:outline-none"
        />
      </div>

      <div v-if="mode !== 'contacts'" class="flex items-center gap-1.5 overflow-x-auto no-scrollbar">
        <button
          v-for="tab in [
            { id: 'all', label: '全部' },
            { id: 'groups', label: '群聊' },
            { id: 'companion', label: '伴聊' },
            { id: 'worker', label: '助理' }
          ]"
          :key="tab.id"
          type="button"
          @click="rosterScope = tab.id as any"
          :class="[
            'px-3 py-1 rounded-full text-[11.5px] font-medium border transition-colors cursor-pointer shrink-0',
            rosterScope === tab.id
              ? 'bg-[#f2b880]/16 border-[#f2b880]/55 text-[#ffe2be]'
              : 'bg-[#0b1022]/80 border-[#fff8ee]/10 text-slate-400'
          ]"
        >
          {{ tab.label }}
        </button>
      </div>
    </div>

    <!-- Mode A: Contacts Directory ("好友 / 通讯录") -->
    <div v-if="mode === 'contacts'" class="flex-1 min-h-0 overflow-y-auto pb-4">
      <div class="mx-3.5 mt-1 mb-3 rounded-2xl border border-[#fff8ee]/12 bg-[#0c1224]/90 divide-y divide-[#fff8ee]/8">
        <button
          type="button"
          @click="openCreateBot"
          class="w-full px-3.5 py-3 flex items-center justify-between gap-3 text-left cursor-pointer"
        >
          <div>
            <div class="text-[14px] font-medium text-[#fff8ee]">新的 AI 好友</div>
            <div class="text-[11.5px] text-slate-400">定制专属性格、口吻与绑定技能</div>
          </div>
          <span class="text-xs text-[#ffe2be]">+ 添加</span>
        </button>
        <button
          type="button"
          @click="openCreateGroup"
          class="w-full px-3.5 py-3 flex items-center justify-between gap-3 text-left cursor-pointer"
        >
          <div>
            <div class="text-[14px] font-medium text-[#fff8ee]">发起协作群聊</div>
            <div class="text-[11.5px] text-slate-400">拉多位 AI 好友进群分工讨论</div>
          </div>
          <span class="text-xs text-indigo-300">+ 建群</span>
        </button>
        <button
          type="button"
          @click="store.state.showMemoriesModal = true"
          class="w-full px-3.5 py-3 flex items-center justify-between gap-3 text-left cursor-pointer"
        >
          <div>
            <div class="text-[14px] font-medium text-[#fff8ee]">长期记忆与技能库</div>
            <div class="text-[11.5px] text-slate-400">管理跨会话记忆与自定义工具</div>
          </div>
          <span class="text-xs text-slate-400">&gt;</span>
        </button>
      </div>

      <template v-if="groupList.length > 0">
        <div class="px-4 py-1.5 text-[11.5px] font-semibold text-slate-400">
          我的群聊 ({{ groupList.length }})
        </div>
        <div class="divide-y divide-[#fff8ee]/8">
          <button
            v-for="g in groupList"
            :key="g.id"
            type="button"
            @click="handleSelectConversation(g.id)"
            class="w-full px-4 py-2.5 flex items-center gap-3 text-left hover:bg-white/[0.03] cursor-pointer"
          >
            <div class="w-11 h-11 rounded-xl border border-[#fff8ee]/14 bg-[#101830] flex items-center justify-center shrink-0">
              <BotGroupAvatar :members="resolveGroupMembers(g)" :fallback-style="g.avatar_style" :state="g.state" size="sm" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-[14.5px] font-medium text-[#fff8ee] truncate">{{ g.name }}</div>
              <div class="text-[12px] text-slate-400 truncate">{{ resolveGroupMembers(g).length }} 位成员 · {{ g.role_title }}</div>
            </div>
          </button>
        </div>
      </template>

      <div class="px-4 pt-3 pb-1.5 text-[11.5px] font-semibold text-slate-400">
        AI 好友 ({{ friendList.length }})
      </div>
      <div class="divide-y divide-[#fff8ee]/8">
        <button
          v-for="f in friendList"
          :key="f.id"
          type="button"
          @click="handleSelectConversation(f.id)"
          class="w-full px-4 py-2.5 flex items-center gap-3 text-left hover:bg-white/[0.03] cursor-pointer"
        >
          <div class="relative w-11 h-11 rounded-xl border border-[#fff8ee]/14 bg-[#0e152b] flex items-center justify-center shrink-0">
            <BotAvatar :avatar-style="f.avatar_style" :state="f.state" size="sm" :seed-key="f.id" />
            <span :class="['absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full ring-2 ring-[#050811]', botStateDotClass(f.state)]" />
          </div>
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-1.5">
              <span class="text-[14.5px] font-medium text-[#fff8ee] truncate">{{ f.name }}</span>
              <span class="px-1.5 py-0.5 rounded text-[10px] border border-[#fff8ee]/12 text-slate-300 shrink-0">
                {{ f.bot_mode === 'companion' ? '伴聊' : '助理' }}
              </span>
            </div>
            <div class="text-[12px] text-slate-400 truncate">{{ f.traits || f.role_title || '随时在线' }}</div>
          </div>
        </button>
      </div>
    </div>

    <!-- Mode B: WeChat / QQ IM Conversation List ("消息") -->
    <div v-else class="flex-1 min-h-0 overflow-y-auto pb-3">
      <!-- Official Service Notification Channel -->
      <button
        v-if="!searchQuery.trim()"
        type="button"
        @click="store.state.showInboxModal = true"
        class="w-full px-4 py-3 flex items-center gap-3.5 text-left bg-[#0d1428]/45 border-b border-[#fff8ee]/8 hover:bg-white/[0.04] cursor-pointer"
      >
        <div class="relative w-12 h-12 rounded-[14px] border border-[#f2b880]/35 bg-[#101830] flex items-center justify-center shrink-0">
          <VesperBrandIcon :size="32" />
          <span
            v-if="inboxUnread > 0"
            class="absolute -top-1 -right-1 min-w-[17px] h-[17px] px-1 rounded-full bg-rose-500 text-white font-mono text-[10px] font-bold leading-none inline-flex items-center justify-center"
          >
            {{ inboxUnread > 99 ? '99+' : inboxUnread }}
          </span>
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-1.5 min-w-0">
              <span class="text-[15px] font-semibold text-[#fff8ee] truncate">服务通知</span>
              <span class="px-1.5 py-0.5 rounded text-[9.5px] bg-indigo-400/15 border border-indigo-300/30 text-indigo-200 shrink-0">订阅号</span>
            </div>
            <span v-if="latestNotif" class="font-mono text-[11px] text-slate-500 shrink-0">
              {{ formatChatTime(latestNotif.created_at) }}
            </span>
          </div>
          <div class="mt-0.5 text-[12.5px] text-slate-400 truncate">
            <span v-if="inboxUnread > 0" class="text-[#f2b880] font-medium">[{{ inboxUnread }}条未读] </span>
            <span>{{ latestNotif ? latestNotif.title : '晨间简报 · 技术雷达 · 节点告警实时推送' }}</span>
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
          'w-full px-4 py-3 flex items-center gap-3.5 text-left border-b border-[#fff8ee]/7 transition-colors cursor-pointer',
          b.is_pinned ? 'bg-[#0e152a]/55' : 'hover:bg-white/[0.03]'
        ]"
      >
        <div class="relative w-12 h-12 rounded-[14px] border border-[#fff8ee]/14 bg-[#0f162c] flex items-center justify-center shrink-0">
          <BotGroupAvatar
            v-if="isGroup(b)"
            :members="resolveGroupMembers(b)"
            :fallback-style="b.avatar_style"
            :state="b.state"
            size="md"
          />
          <template v-else>
            <BotAvatar :avatar-style="b.avatar_style" :state="b.state" size="md" :seed-key="b.id" />
            <span :class="['absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full ring-2 ring-[#050811]', botStateDotClass(b.state)]" />
          </template>

          <span
            v-if="(b.unread_count || 0) > 0"
            class="absolute -top-1 -right-1 min-w-[17px] h-[17px] px-1 rounded-full bg-rose-500 text-white font-mono text-[10px] font-bold leading-none inline-flex items-center justify-center"
          >
            {{ b.unread_count > 99 ? '99+' : b.unread_count }}
          </span>
        </div>

        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-1.5 min-w-0">
              <span class="text-[15px] font-semibold text-[#fff8ee] truncate">{{ b.name }}</span>
              <span
                :class="[
                  'px-1.5 py-0.5 rounded text-[9.5px] border shrink-0',
                  isGroup(b)
                    ? 'bg-[#f2b880]/15 border-[#f2b880]/40 text-[#ffe2be]'
                    : 'bg-white/5 border-[#fff8ee]/14 text-slate-400'
                ]"
              >
                {{ isGroup(b) ? '群聊' : b.bot_mode === 'companion' ? '伴聊' : '助理' }}
              </span>
              <span v-if="b.is_pinned" class="text-[9.5px] text-[#f2b880] shrink-0">置顶</span>
            </div>
            <span class="font-mono text-[11px] text-slate-500 shrink-0">
              {{ formatChatTime(b.updated_at) }}
            </span>
          </div>

          <div class="mt-0.5 text-[12.5px] text-slate-400 truncate">
            <template v-if="botDraftPreview(b)">
              <span class="text-rose-400 font-medium">[草稿] </span>
              <span>{{ botDraftPreview(b) }}</span>
            </template>
            <template v-else>
              <span v-if="(b.unread_count || 0) > 0" class="text-[#f2b880] font-medium">[{{ b.unread_count }}条] </span>
              <span>{{ b.last_summary || b.traits || b.role_title || '点击开始对话' }}</span>
            </template>
          </div>
        </div>
      </button>
    </div>
  </section>
</template>
