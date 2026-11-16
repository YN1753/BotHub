<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { trackSpotlight } from '../utils/topLayer'
import BaseModal from './BaseModal.vue'
import BotAvatar from './BotAvatar.vue'
import BotAvatarPicker from './BotAvatarPicker.vue'
import CustomSelect from './CustomSelect.vue'
import SegmentedTabs from './SegmentedTabs.vue'
import ToggleSwitch from './ToggleSwitch.vue'

const store = useAppStore()

const CREATE_TABS = [
  { id: 'bot', label: '创建专属 Bot' },
  { id: 'group', label: '创建多 Bot 群聊' },
]

const mode = computed({
  get: () => store.state.createModalMode,
  set: (v: 'bot' | 'group') => {
    store.state.createModalMode = v
  },
})

const name = ref('')
const roleTitle = ref('')
const avatarStyle = ref('butler:gold')
const model = ref('deepseek-flash')
const enableThinking = ref(true)
const systemPrompt = ref('')
const allowedTools = ref<string[]>([
  'read_url',
  'web_search',
  'query_nezha_nodes',
  'manage_assets',
  'manage_tasks',
  'remember_fact',
  'recall_memory',
])

// Group chat state
const groupName = ref('')
const groupDesc = ref('')
const groupAvatar = ref('swarm:indigo')
const groupMembers = ref<string[]>([])
const saving = ref(false)

const TOOL_CATALOG = [
  { id: 'read_url', label: '网页抓取' },
  { id: 'web_search', label: '开源雷达' },
  { id: 'query_nezha_nodes', label: '哪吒探针' },
  { id: 'manage_assets', label: '资产台账' },
  { id: 'manage_tasks', label: '定时调度' },
  { id: 'remember_fact', label: '写入记忆' },
  { id: 'recall_memory', label: '检索记忆' },
]

const MODEL_OPTIONS = [
  { value: 'deepseek-flash', label: 'DeepSeek Flash (主力)' },
  { value: 'deepseek-v4-pro', label: 'DeepSeek V4 Pro (深度)' },
]

const availableSpecialistBots = computed(() =>
  store.state.bots.filter((b) => !b.is_group && b.id !== 'team_room')
)

watch(
  () => store.state.showCreateBotModal,
  (open) => {
    if (!open) return
    if (groupMembers.value.length === 0) {
      groupMembers.value = availableSpecialistBots.value.slice(0, 3).map((b) => b.id)
    }
  }
)

function toggleGroupMember(id: string) {
  const idx = groupMembers.value.indexOf(id)
  if (idx >= 0) {
    groupMembers.value = groupMembers.value.filter((x) => x !== id)
  } else {
    groupMembers.value = [...groupMembers.value, id]
  }
}

function closeModal() {
  store.state.showCreateBotModal = false
}

async function handleSubmit() {
  if (saving.value) return
  if (mode.value === 'group') {
    if (!groupName.value.trim()) {
      store.notify('请输入群聊名称')
      return
    }
    if (groupMembers.value.length === 0) {
      store.notify('请至少选择 1 位 Bot 加入群聊')
      return
    }
    saving.value = true
    try {
      const res = await BotHubApi.createBot({
        name: groupName.value.trim(),
        role_title: groupDesc.value.trim() || `多 Bot 协同群 (${groupMembers.value.length} 成员)`,
        avatar_style: groupAvatar.value,
        model: 'deepseek-flash',
        enable_thinking: true,
        is_group: true,
        member_ids: JSON.stringify(groupMembers.value),
        allowed_tools: '["read_url","web_search","query_nezha_nodes","manage_assets","manage_tasks","remember_fact","recall_memory"]',
        system_prompt: groupDesc.value.trim() || '你是群聊协同协调官，整合各位成员观点并输出精确决策。',
      })
      store.notify('多 Bot 协同群聊创建成功')
      groupName.value = ''
      groupDesc.value = ''
      closeModal()
      await store.loadDashboard(false)
      if (res.bot?.id) {
        await store.selectBot(res.bot.id)
      }
    } catch (err: any) {
      store.notify(err.message)
    } finally {
      saving.value = false
    }
    return
  }

  if (!name.value.trim()) return
  saving.value = true
  try {
    const res = await BotHubApi.createBot({
      name: name.value.trim(),
      role_title: roleTitle.value.trim() || '专属智能助理',
      avatar_style: avatarStyle.value,
      model: model.value,
      enable_thinking: enableThinking.value,
      is_group: false,
      member_ids: '[]',
      allowed_tools: JSON.stringify(allowedTools.value),
      system_prompt: systemPrompt.value.trim(),
    })
    store.notify('专属 Bot 创建并激活成功')
    name.value = ''
    roleTitle.value = ''
    systemPrompt.value = ''
    closeModal()
    await store.loadDashboard(false)
    if (res.bot?.id) {
      await store.selectBot(res.bot.id)
    }
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <BaseModal
    :open="store.state.showCreateBotModal"
    :title="mode === 'group' ? '组建多 Bot 协同群聊' : '招募全新专属 Bot 成员'"
    :subtitle="mode === 'group' ? '编排多位专属 Bot 在同一时间线内按序或 @点名接力协同' : '定制动态机甲形象、思考偏好与最小权限工具白名单'"
    max-width="2xl"
    @close="closeModal"
  >
    <div class="shrink-0">
      <SegmentedTabs v-model="mode" :items="CREATE_TABS" aria-label="创建类型" />
    </div>

    <form @submit.prevent="handleSubmit" class="flex-1 min-h-0 overflow-y-auto pr-1 flex flex-col gap-3.5">
      <Transition
        mode="out-in"
        enter-active-class="transition duration-180 ease-out"
        enter-from-class="opacity-0 translate-y-1"
        enter-to-class="opacity-100 translate-y-0"
        leave-active-class="transition duration-100 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <!-- Mode 1: Single Specialist Bot -->
        <div v-if="mode === 'bot'" key="bot" class="flex flex-col gap-3.5">
          <BotAvatarPicker v-model="avatarStyle" :preview-name="name || '新专属 Bot'" />

          <div class="modal-section grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>Bot 名称</span>
              <input
                v-model="name"
                type="text"
                placeholder="如：云账单精算师"
                required
                class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60"
              />
            </label>
            <label class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>职责定位</span>
              <input
                v-model="roleTitle"
                type="text"
                placeholder="如：账单追踪与资源优化顾问"
                required
                class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60"
              />
            </label>
            <div class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>默认推理模型</span>
              <CustomSelect v-model="model" :options="MODEL_OPTIONS" aria-label="默认推理模型" />
            </div>
            <div class="flex flex-col justify-end">
              <ToggleSwitch v-model="enableThinking" label="默认开启深度思考" />
            </div>
          </div>

          <div class="modal-section flex flex-col gap-3">
            <label class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>专属人设指令 (System Prompt)</span>
              <textarea
                v-model="systemPrompt"
                rows="3"
                placeholder="描述它的思考风格、决策依据与输出偏好..."
                class="auto-size-textarea w-full rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 resize-y"
              />
            </label>

            <div class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>授权工具集 ({{ allowedTools.length }}/{{ TOOL_CATALOG.length }})</span>
              <div class="flex flex-wrap gap-1.5">
                <label
                  v-for="tool in TOOL_CATALOG"
                  :key="tool.id"
                  class="min-h-[38px] sm:min-h-[32px] inline-flex items-center gap-1.5 px-3 py-1 rounded-xl border border-[#fff8ee]/12 bg-[#050811]/80 text-xs text-slate-300 cursor-pointer hover:border-[#f2b880]/40 has-[:checked]:border-[#f2b880]/55 has-[:checked]:bg-[#f2b880]/14 has-[:checked]:text-[#f7cba0] transition-colors"
                >
                  <input v-model="allowedTools" type="checkbox" :value="tool.id" class="accent-[#f2b880]" />
                  <span>{{ tool.label }}</span>
                </label>
              </div>
            </div>
          </div>
        </div>

        <!-- Mode 2: Multi-Bot Group Room -->
        <div v-else key="group" class="flex flex-col gap-3.5">
          <BotAvatarPicker v-model="groupAvatar" :preview-name="groupName || '多 Bot 协同群'" />

          <div class="modal-section grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>群聊名称</span>
              <input
                v-model="groupName"
                type="text"
                placeholder="如：深夜故障联合应急群"
                required
                class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60"
              />
            </label>
            <label class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>群聊定位 / 议题说明</span>
              <input
                v-model="groupDesc"
                type="text"
                placeholder="如：探针异常排查与架构评估"
                class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60"
              />
            </label>
          </div>

          <div class="modal-section flex flex-col gap-2.5">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-medium text-slate-200">选择进群协同 Bot 成员（已选 {{ groupMembers.length }} 位）</span>
              <span class="font-mono text-[10.5px] text-slate-400">支持 @点名 或按序接力协同</span>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <button
                v-for="b in availableSpecialistBots"
                :key="b.id"
                type="button"
                @pointermove="trackSpotlight"
                @click="toggleGroupMember(b.id)"
                :aria-pressed="groupMembers.includes(b.id)"
                :class="[
                  'group luxury-card rounded-xl p-2.5 flex items-center gap-2.5 text-left cursor-pointer',
                  groupMembers.includes(b.id)
                    ? 'border-[#f2b880]/65 bg-[#f2b880]/12'
                    : 'border-[#fff8ee]/10 opacity-80 hover:opacity-100'
                ]"
              >
                <BotAvatar :avatar-style="b.avatar_style" :state="b.state" size="md" />
                <div class="flex-1 min-w-0">
                  <div class="text-xs font-semibold text-white truncate">{{ b.name }}</div>
                  <div class="text-[11px] text-slate-400 truncate">{{ b.role_title }}</div>
                </div>
                <span
                  :class="[
                    'w-5 h-5 rounded-full border flex items-center justify-center shrink-0 transition-colors',
                    groupMembers.includes(b.id)
                      ? 'bg-[#f2b880] border-[#f2b880] text-[#070a13]'
                      : 'border-[#fff8ee]/20 text-transparent'
                  ]"
                >
                  <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2.6" aria-hidden="true">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                </span>
              </button>
            </div>
          </div>
        </div>
      </Transition>

      <div class="flex justify-end gap-2 pt-2 border-t border-[#fff8ee]/10 shrink-0">
        <button
          type="button"
          @click="closeModal"
          class="min-h-[44px] sm:min-h-[36px] px-4 py-2 rounded-xl border border-[#fff8ee]/14 text-xs text-slate-300 hover:bg-white/5 cursor-pointer"
        >
          取消
        </button>
        <button
          type="submit"
          :disabled="saving"
          class="min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50"
        >
          {{ saving ? '创建中...' : (mode === 'group' ? '创建协同群聊' : '创建并激活 Bot') }}
        </button>
      </div>
    </form>
  </BaseModal>
</template>
