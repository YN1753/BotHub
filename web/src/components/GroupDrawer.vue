<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { trackSpotlight } from '../utils/topLayer'
import type { Bot } from '../types/bothub'
import BotAvatar from './BotAvatar.vue'
import BotGroupAvatar from './BotGroupAvatar.vue'
import SegmentedTabs from './SegmentedTabs.vue'
import ToggleSwitch from './ToggleSwitch.vue'

const store = useAppStore()
const bot = store.activeBot

const isGroup = computed(() => Boolean(bot.value?.is_group))

const specialistBots = computed(() =>
  store.state.bots.filter((b) => !b.is_group)
)

const currentMemberIds = computed<string[]>(() => {
  if (!bot.value) return []
  try {
    const arr = JSON.parse(bot.value.member_ids || '[]')
    return Array.isArray(arr) ? arr : []
  } catch {
    return []
  }
})

const currentMembers = computed<Bot[]>(() => {
  const map = new Map(specialistBots.value.map((b) => [b.id, b]))
  return currentMemberIds.value.map((id) => map.get(id)).filter((x): x is Bot => Boolean(x))
})

const inviteCandidates = computed<Bot[]>(() =>
  specialistBots.value.filter((b) => !currentMemberIds.value.includes(b.id))
)

// Group edit state
const showInviteList = ref(false)
const editName = ref('')
const editAnnouncement = ref('')
const editMode = ref('smart')
const savingGroup = ref(false)
const busyMemberId = ref('')

// Adhoc group creation state (from 1v1 DM)
const adhocSelectedIds = ref<string[]>([])
const adhocGroupName = ref('')
const adhocCarryContext = ref(true)
const creatingAdhoc = ref(false)

const MODE_TABS = [
  { id: 'smart', label: '智能协同' },
  { id: 'sequential', label: '全员接力' },
  { id: 'mention_only', label: '仅 @点名' },
]

const modeHint = computed(() => {
  if (editMode.value === 'sequential') return '未 @点名时，群内全部成员按序接力发言并补充结论'
  if (editMode.value === 'mention_only') return '仅响应被 @点名 的成员；未指定 @ 时由首位协调官简答'
  return '根据提问关键词智能匹配 1~2 位最对口专家协同作答，高效零刷屏'
})

watch(
  () => [store.state.showGroupDrawer, bot.value?.id] as const,
  ([open]) => {
    if (!open || !bot.value) return
    showInviteList.value = false
    editName.value = bot.value.name || ''
    editAnnouncement.value = bot.value.announcement || ''
    editMode.value = bot.value.group_mode || 'smart'
    adhocSelectedIds.value = []
    adhocGroupName.value = ''
    adhocCarryContext.value = true
  }
)

function closeDrawer() {
  store.state.showGroupDrawer = false
}

function handleDrawerKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && store.state.showGroupDrawer) {
    e.preventDefault()
    closeDrawer()
  }
}

onMounted(() => window.addEventListener('keydown', handleDrawerKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', handleDrawerKeydown))

async function inviteMember(memberBot: Bot) {
  if (!bot.value || busyMemberId.value) return
  busyMemberId.value = memberBot.id
  try {
    const res = await BotHubApi.updateGroupMembers(bot.value.id, 'add', [memberBot.id])
    if (res.bot) {
      bot.value.member_ids = res.bot.member_ids
      bot.value.last_summary = res.bot.last_summary
    }
    if (res.system_message && !store.state.timeline.some((m) => m.serverId === res.system_message!.id)) {
      store.state.timeline.push({
        localId: `srv_${res.system_message.id}`,
        serverId: res.system_message.id,
        role: 'system',
        senderBotId: bot.value.id,
        senderName: '系统动态',
        avatarStyle: bot.value.avatar_style,
        model: bot.value.model,
        msgKind: 'system',
        createdAt: res.system_message.created_at,
        content: res.system_message.content,
        reasoning: '',
        tools: [],
        statusText: '',
      })
    }
    store.notify(`已邀请「${memberBot.name}」加入群聊`)
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    busyMemberId.value = ''
  }
}

async function removeMember(memberBot: Bot) {
  if (!bot.value || busyMemberId.value) return
  if (currentMembers.value.length <= 1) {
    store.notify('群聊至少需要保留 1 位协同成员')
    return
  }
  busyMemberId.value = memberBot.id
  try {
    const res = await BotHubApi.updateGroupMembers(bot.value.id, 'remove', [memberBot.id])
    if (res.bot) {
      bot.value.member_ids = res.bot.member_ids
      bot.value.last_summary = res.bot.last_summary
    }
    if (res.system_message && !store.state.timeline.some((m) => m.serverId === res.system_message!.id)) {
      store.state.timeline.push({
        localId: `srv_${res.system_message.id}`,
        serverId: res.system_message.id,
        role: 'system',
        senderBotId: bot.value.id,
        senderName: '系统动态',
        avatarStyle: bot.value.avatar_style,
        model: bot.value.model,
        msgKind: 'system',
        createdAt: res.system_message.created_at,
        content: res.system_message.content,
        reasoning: '',
        tools: [],
        statusText: '',
      })
    }
    store.notify(`已将「${memberBot.name}」移出群聊`)
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    busyMemberId.value = ''
  }
}

async function saveGroupMeta() {
  if (!bot.value || savingGroup.value) return
  savingGroup.value = true
  try {
    const res = await BotHubApi.updateBot(bot.value.id, {
      name: editName.value.trim() || bot.value.name,
      announcement: editAnnouncement.value.trim(),
      group_mode: editMode.value,
    })
    if (res.bot) {
      bot.value.name = res.bot.name
      bot.value.announcement = res.bot.announcement
      bot.value.group_mode = res.bot.group_mode
    }
    store.notify('群聊设置与公告已保存')
    await store.loadDashboard(false)
    await store.selectBot(bot.value.id)
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    savingGroup.value = false
  }
}

function toggleAdhocBot(id: string) {
  if (adhocSelectedIds.value.includes(id)) {
    adhocSelectedIds.value = adhocSelectedIds.value.filter((x) => x !== id)
  } else {
    adhocSelectedIds.value = [...adhocSelectedIds.value, id]
  }
}

async function launchAdhocGroup() {
  if (!bot.value || creatingAdhoc.value || adhocSelectedIds.value.length === 0) return
  creatingAdhoc.value = true
  try {
    const res = await BotHubApi.createAdhocGroup({
      source_bot_id: bot.value.id,
      member_ids: adhocSelectedIds.value,
      name: adhocGroupName.value.trim(),
      group_mode: 'smart',
      carry_context: adhocCarryContext.value,
    })
    store.notify(`已拉起群聊「${res.bot.name}」`)
    closeDrawer()
    await store.loadDashboard(false)
    if (res.bot?.id) {
      await store.selectBot(res.bot.id)
    }
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    creatingAdhoc.value = false
  }
}

async function handleClearTimeline() {
  if (!bot.value || store.state.isStreaming) return
  if (!window.confirm(`确定清空 [${bot.value.name}] 的聊天记录吗？`)) return
  await store.clearActiveBotTimeline()
  closeDrawer()
}

async function openScopedMemory() {
  if (!bot.value) return
  store.state.memoryModalDefaultScope = `bot:${bot.value.id}`
  closeDrawer()
  store.state.showMemoriesModal = true
}

async function handleCreateFreshThread() {
  closeDrawer()
  await store.createThread('')
}

async function handleDisbandGroup() {
  if (!bot.value) return
  if (!window.confirm(`确定解散群聊 [${bot.value.name}] 吗？群消息记录将被清除。`)) return
  await BotHubApi.deleteBot(bot.value.id)
  store.notify('群聊已解散')
  closeDrawer()
  await store.loadDashboard(false)
  if (store.state.bots.length > 0) {
    await store.selectBot(store.state.bots[0].id)
  }
}
</script>

<template>
  <Transition
    enter-active-class="transition duration-200 ease-out"
    enter-from-class="opacity-0"
    enter-to-class="opacity-100"
    leave-active-class="transition duration-150 ease-in"
    leave-from-class="opacity-100"
    leave-to-class="opacity-0"
  >
    <div
      v-if="store.state.showGroupDrawer"
      class="fixed inset-0 z-40 bg-black/55 backdrop-blur-xs"
      aria-hidden="true"
      @click="closeDrawer"
    />
  </Transition>

  <Transition
    enter-active-class="transition duration-240 ease-[var(--ease-snappy)]"
    enter-from-class="translate-x-full opacity-0"
    enter-to-class="translate-x-0 opacity-100"
    leave-active-class="transition duration-180 ease-in"
    leave-from-class="translate-x-0 opacity-100"
    leave-to-class="translate-x-full opacity-0"
  >
    <aside
      v-if="store.state.showGroupDrawer && bot"
      role="dialog"
      aria-modal="true"
      aria-label="群聊与会话详情"
      class="fixed inset-y-0 right-0 z-50 w-[min(24rem,94vw)] bg-[#080c18]/96 backdrop-blur-2xl border-l border-[#fff8ee]/12 shadow-[-20px_0_60px_rgba(0,0,0,0.65)] flex flex-col overflow-hidden"
    >
      <div class="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-[#f2b880]/45 to-transparent" aria-hidden="true" />

      <!-- Drawer Header -->
      <div class="px-4 py-3.5 pt-[max(0.875rem,env(safe-area-inset-top,0.875rem))] border-b border-[#fff8ee]/10 flex items-center justify-between gap-2 shrink-0">
        <div class="flex items-center gap-2.5 min-w-0">
          <BotGroupAvatar
            v-if="isGroup"
            :members="currentMembers"
            :fallback-style="bot.avatar_style"
            :state="bot.state"
            size="sm"
          />
          <BotAvatar v-else :avatar-style="bot.avatar_style" :state="bot.state" size="sm" />
          <div class="min-w-0">
            <h2 class="text-sm font-semibold text-white truncate">
              {{ isGroup ? `群聊详情 (${currentMembers.length})` : '会话与拉群管理' }}
            </h2>
            <p class="text-[11px] text-slate-400 truncate">{{ bot.name }}</p>
          </div>
        </div>
        <button
          type="button"
          aria-label="关闭抽屉"
          @click="closeDrawer"
          class="min-h-[40px] min-w-[40px] sm:min-h-[32px] sm:min-w-[32px] inline-flex items-center justify-center rounded-full border border-[#fff8ee]/12 text-slate-400 hover:text-white hover:border-[#f2b880]/40 cursor-pointer"
        >
          <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>

      <!-- Scrollable Body -->
      <div class="flex-1 min-h-0 overflow-y-auto p-4 flex flex-col gap-3.5 pb-[max(1rem,env(safe-area-inset-bottom,1rem))]">
        <!-- GROUP VIEW -->
        <template v-if="isGroup">
          <!-- QQ/WeChat Member Mosaic Grid + Live Invite/Remove -->
          <div class="modal-section flex flex-col gap-3">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-medium text-slate-200">群聊成员 ({{ currentMembers.length }})</span>
              <button
                v-if="inviteCandidates.length > 0"
                type="button"
                @click="showInviteList = !showInviteList"
                class="text-[11px] font-medium text-[#f2b880] hover:text-[#f7cba0] cursor-pointer"
              >
                {{ showInviteList ? '收起候选' : `+ 临时拉人 (${inviteCandidates.length})` }}
              </button>
            </div>

            <div class="grid grid-cols-4 gap-2">
              <div
                v-for="m in currentMembers"
                :key="m.id"
                class="group relative flex flex-col items-center gap-1 p-1.5 rounded-xl hover:bg-white/[0.04] transition-colors"
              >
                <button
                  v-if="currentMembers.length > 1"
                  type="button"
                  :disabled="busyMemberId === m.id"
                  title="移出群聊"
                  @click="removeMember(m)"
                  class="opacity-0 group-hover:opacity-100 focus:opacity-100 absolute -top-1 -right-1 w-5 h-5 rounded-full bg-[#090d1a] border border-rose-400/50 text-rose-300 flex items-center justify-center hover:bg-rose-500/25 cursor-pointer z-10 transition-opacity"
                >
                  <svg viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2.6" aria-hidden="true">
                    <line x1="18" y1="6" x2="6" y2="18" />
                    <line x1="6" y1="6" x2="18" y2="18" />
                  </svg>
                </button>
                <button
                  type="button"
                  @click="store.jumpToBot(m.id)"
                  :title="`点击进入 ${m.name} 私聊`"
                  class="flex flex-col items-center gap-1 cursor-pointer w-full"
                >
                  <BotAvatar :avatar-style="m.avatar_style" :state="m.state" size="md" :interactive="false" />
                  <span class="text-[11px] text-slate-200 truncate w-full text-center">{{ m.name }}</span>
                </button>
              </div>

              <!-- '+' Invite Tile -->
              <button
                v-if="inviteCandidates.length > 0"
                type="button"
                @click="showInviteList = !showInviteList"
                class="flex flex-col items-center justify-center gap-1 p-1.5 rounded-xl border border-dashed border-[#fff8ee]/18 hover:border-[#f2b880]/55 text-slate-400 hover:text-[#f2b880] cursor-pointer"
              >
                <span class="w-9 h-9 rounded-full border border-[#fff8ee]/14 bg-[#fff8ee]/[0.03] flex items-center justify-center">
                  <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                    <line x1="12" y1="5" x2="12" y2="19" />
                    <line x1="5" y1="12" x2="19" y2="12" />
                  </svg>
                </span>
                <span class="text-[11px]">拉人</span>
              </button>
            </div>

            <!-- Expandable Invite Candidate List -->
            <div v-if="showInviteList && inviteCandidates.length > 0" class="pt-2.5 border-t border-[#fff8ee]/10 flex flex-col gap-1.5">
              <span class="font-mono text-[10.5px] text-slate-400">选择要临时拉入本群的 Bot：</span>
              <button
                v-for="cand in inviteCandidates"
                :key="cand.id"
                type="button"
                @pointermove="trackSpotlight"
                :disabled="busyMemberId === cand.id"
                @click="inviteMember(cand)"
                class="luxury-card rounded-xl px-2.5 py-2 flex items-center gap-2.5 text-left cursor-pointer"
              >
                <BotAvatar :avatar-style="cand.avatar_style" :state="cand.state" size="sm" :interactive="false" />
                <div class="flex-1 min-w-0">
                  <div class="text-xs font-medium text-white truncate">{{ cand.name }}</div>
                  <div class="text-[10.5px] text-slate-400 truncate">{{ cand.role_title }}</div>
                </div>
                <span class="px-2 py-0.5 rounded-md border border-[#f2b880]/40 bg-[#f2b880]/12 text-[11px] font-medium text-[#f2b880] shrink-0">
                  {{ busyMemberId === cand.id ? '拉入中' : '+ 拉入' }}
                </span>
              </button>
            </div>
          </div>

          <!-- Group Name, Mode & Announcement -->
          <form @submit.prevent="saveGroupMeta" class="modal-section flex flex-col gap-3">
            <label class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>群聊名称</span>
              <input
                v-model="editName"
                type="text"
                class="min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-1.5 text-xs text-white focus:border-[#f2b880]/60"
              />
            </label>

            <div class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>群协同策略</span>
              <SegmentedTabs v-model="editMode" :items="MODE_TABS" variant="subtle" aria-label="群协同策略" />
              <p class="text-[11px] text-slate-400 leading-relaxed">{{ modeHint }}</p>
            </div>

            <label class="flex flex-col gap-1.5 text-xs text-slate-300">
              <div class="flex items-center justify-between">
                <span>群公告（将置顶提示在聊天气泡上方）</span>
                <span class="font-mono text-[10px] text-slate-500">{{ editAnnouncement.length }} / 240</span>
              </div>
              <textarea
                v-model="editAnnouncement"
                rows="2"
                maxlength="240"
                placeholder="填写群内协作规范、值守说明或本群聚焦目标..."
                class="auto-size-textarea rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-xs text-white focus:border-[#f2b880]/60 resize-y"
              />
            </label>

            <button
              type="submit"
              :disabled="savingGroup"
              class="min-h-[38px] rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50"
            >
              {{ savingGroup ? '保存中...' : '保存群设置' }}
            </button>
          </form>

          <!-- Pin, Topic, Memory & Dangerous Actions -->
          <div class="modal-section flex flex-col gap-2.5">
            <ToggleSwitch
              :model-value="bot.is_pinned"
              label="置顶此群聊"
              description="固定显示在左侧会话列表最上方"
              @update:model-value="store.togglePinBot(bot)"
            />
            <div class="grid grid-cols-2 gap-2 pt-1">
              <button
                type="button"
                @click="handleCreateFreshThread"
                class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 text-xs text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
              >
                开启新话题
              </button>
              <button
                type="button"
                @click="openScopedMemory"
                class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 text-xs text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
              >
                本群专属记忆
              </button>
              <button
                type="button"
                @click="handleClearTimeline"
                class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 text-xs text-slate-300 hover:bg-white/5 cursor-pointer"
              >
                清空聊天记录
              </button>
              <button
                type="button"
                @click="handleDisbandGroup"
                class="min-h-[38px] rounded-xl border border-rose-400/35 bg-rose-500/12 text-xs text-rose-300 hover:bg-rose-500/22 cursor-pointer"
              >
                解散群聊
              </button>
            </div>
          </div>
        </template>

        <!-- 1v1 DM VIEW: WeChat-style "临时拉人发起群聊" -->
        <template v-else>
          <div class="modal-section flex items-center gap-3">
            <BotAvatar :avatar-style="bot.avatar_style" :state="bot.state" size="lg" />
            <div class="min-w-0 flex-1">
              <div class="text-sm font-semibold text-white truncate">{{ bot.name }}</div>
              <div class="text-xs text-slate-400 truncate">{{ bot.role_title }}</div>
            </div>
          </div>

          <div class="modal-section flex flex-col gap-3">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-semibold text-[#f7cba0]">临时拉人发起协同群聊</span>
              <span class="font-mono text-[10.5px] text-slate-400">已拉 {{ adhocSelectedIds.length }} 位外援</span>
            </div>
            <p class="text-[11.5px] text-slate-400 leading-relaxed">
              无需中断思路：保留「{{ bot.name }}」，勾选其他专家即刻组建多人协同群。
            </p>

            <div class="flex flex-col gap-1.5">
              <button
                v-for="cand in specialistBots.filter((x) => x.id !== bot!.id)"
                :key="cand.id"
                type="button"
                @pointermove="trackSpotlight"
                @click="toggleAdhocBot(cand.id)"
                :class="[
                  'luxury-card rounded-xl px-2.5 py-2 flex items-center gap-2.5 text-left cursor-pointer',
                  adhocSelectedIds.includes(cand.id)
                    ? 'border-[#f2b880]/60 bg-[#f2b880]/10'
                    : 'border-[#fff8ee]/10 opacity-85 hover:opacity-100'
                ]"
              >
                <BotAvatar :avatar-style="cand.avatar_style" :state="cand.state" size="sm" :interactive="false" />
                <div class="flex-1 min-w-0">
                  <div class="text-xs font-medium text-white truncate">{{ cand.name }}</div>
                  <div class="text-[10.5px] text-slate-400 truncate">{{ cand.role_title }}</div>
                </div>
                <span
                  :class="[
                    'w-5 h-5 rounded-full border flex items-center justify-center shrink-0 transition-colors',
                    adhocSelectedIds.includes(cand.id)
                      ? 'bg-[#f2b880] border-[#f2b880] text-[#070a13]'
                      : 'border-[#fff8ee]/20 text-transparent'
                  ]"
                >
                  <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2.6" aria-hidden="true">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                </span>
              </button>
            </div>

            <input
              v-model="adhocGroupName"
              type="text"
              placeholder="群名称（留空自动生成，如 XX × XX 协同群）"
              class="min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-1.5 text-xs text-white focus:border-[#f2b880]/60"
            />

            <ToggleSwitch
              v-model="adhocCarryContext"
              label="带入当前最近对话上下文"
              description="自动提取最近几轮要点作为新群聊系统开场背景"
            />

            <button
              type="button"
              :disabled="adhocSelectedIds.length === 0 || creatingAdhoc"
              @click="launchAdhocGroup"
              class="min-h-[40px] rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-40"
            >
              {{ creatingAdhoc ? '建群中...' : `立即拉起 ${adhocSelectedIds.length + 1} 人协同群` }}
            </button>
          </div>

          <div class="modal-section flex flex-col gap-2.5">
            <ToggleSwitch
              :model-value="bot.is_pinned"
              label="置顶此 Bot 对话"
              @update:model-value="store.togglePinBot(bot)"
            />
            <div class="grid grid-cols-2 gap-2">
              <button
                type="button"
                @click="store.state.botView = 'config'; closeDrawer()"
                class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 text-xs text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
              >
                形象与人设配置
              </button>
              <button
                type="button"
                @click="openScopedMemory"
                class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 text-xs text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
              >
                专属记忆域
              </button>
              <button
                type="button"
                @click="handleCreateFreshThread"
                class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 text-xs text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
              >
                开启新话题
              </button>
              <button
                type="button"
                @click="handleClearTimeline"
                class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 text-xs text-slate-300 hover:bg-white/5 cursor-pointer"
              >
                清空聊天记录
              </button>
            </div>
          </div>
        </template>
      </div>
    </aside>
  </Transition>
</template>
