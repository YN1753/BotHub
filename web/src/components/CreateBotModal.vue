<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { groupModeLabel, parseJsonStringArray } from '../utils/format'
import { clearSearchOnEscape, trackSpotlight } from '../utils/topLayer'
import { groupAvailableTools } from '../utils/toolMeta'
import type { BotMode, MemoryPolicy } from '../types/bothub'
import BaseModal from './BaseModal.vue'
import BotAvatar from './BotAvatar.vue'
import BotAvatarPicker from './BotAvatarPicker.vue'
import CustomSelect from './CustomSelect.vue'
import SegmentedTabs from './SegmentedTabs.vue'
import ToggleSwitch from './ToggleSwitch.vue'
import ModelRoutePreview from './ui/ModelRoutePreview.vue'

const store = useAppStore()

const CREATE_TABS = [
  { id: 'bot', label: '创建专属 Bot' },
  { id: 'group', label: '创建多 Bot 群聊' },
]

const BOT_MODE_TABS = [
  { id: 'companion', label: '陪伴聊天型' },
  { id: 'hybrid', label: '综合协同型' },
  { id: 'worker', label: '干活执行型' },
]

const MEMORY_POLICY_OPTIONS = [
  { value: 'isolated', label: '严格私有记忆 (仅 Bot 自己可见，不串记忆)' },
  { value: 'standard', label: '私有记忆 + 全局共享事实' },
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
const model = ref(store.modelOptions.value[0]?.value || 'auto:free')
const allowPaidFallback = ref(false)
const botMode = ref<BotMode>('hybrid')
const memoryPolicy = ref<MemoryPolicy>('isolated')
const traits = ref('')
const enableThinking = ref(true)
const systemPrompt = ref('')
const allowedTools = ref<string[]>([])
const selectedSkillIds = ref<string[]>([])

// Group chat state
const groupName = ref('')
const groupDesc = ref('')
const groupAvatar = ref('swarm:indigo')
const groupMembers = ref<string[]>([])
const groupMode = ref<'smart' | 'sequential' | 'parallel' | 'mention_only'>('smart')
const groupModel = ref('')
const groupAnnouncement = ref('')
const toolSearch = ref('')
const saving = ref(false)
const botNameInputRef = ref<HTMLInputElement | null>(null)
const groupNameInputRef = ref<HTMLInputElement | null>(null)

function resetCreateForm() {
  name.value = ''
  roleTitle.value = ''
  traits.value = ''
  systemPrompt.value = ''
  selectedSkillIds.value = []
  groupName.value = ''
  groupDesc.value = ''
  groupAnnouncement.value = ''
  toolSearch.value = ''
}

const STARTER_ARCHETYPES: Array<{
  label: string
  name: string
  role: string
  mode: BotMode
  avatar: string
  traits: string
  prompt: string
  tools: string[]
}> = [
  {
    label: '个人总助',
    name: 'Muse 总助',
    role: '全能个人效率与日程参谋',
    mode: 'hybrid',
    avatar: 'butler:gold',
    traits: '简洁高效、主动帮用户拆解待办与时间规划',
    prompt: '你是用户的专属个人总助。收到想法时先查现有待办/文档/收藏匣，能落地就调用工具直接创建待办、收纳闪念链接、写笔记或设定提醒，禁止废话与 Emoji。',
    tools: ['manage_todos', 'manage_notes', 'manage_stash', 'manage_tasks', 'workspace_briefing', 'datetime_calc', 'calculate_eval', 'remember_fact', 'recall_memory', 'manage_memory', 'send_notification'],
  },
  {
    label: '资产管家',
    name: '账单守望',
    role: '周期资产与提醒账单管家',
    mode: 'worker',
    avatar: 'visor:obsidian',
    traits: '严谨客观、先核查资产台账与到期规则再汇报结论',
    prompt: '你是资产与周期提醒管理助手。涉及到期账单、分类汇总与提醒计划时优先调用 manage_assets 与 manage_tasks 获取真实数据，严禁使用 Emoji。',
    tools: ['manage_assets', 'manage_tasks', 'send_notification', 'remember_fact', 'recall_memory', 'datetime_calc', 'calculate_eval'],
  },
  {
    label: '深度研究员',
    name: 'Researcher',
    role: '技术情报与长文精读分析师',
    mode: 'worker',
    avatar: 'radar:indigo',
    traits: '结构清晰、善用网页检索与链接阅读提炼要点',
    prompt: '你是深度技术调研员。回答复杂调研问题前先调用 web_search、read_url 与 manage_stash 获取一手依据与历史收藏，给出对照对比与可执行建议，严禁使用 Emoji。',
    tools: ['web_search', 'read_url', 'manage_stash', 'manage_notes', 'remember_fact', 'recall_memory', 'calculate_eval'],
  },
  {
    label: '轻松伙伴',
    name: '深夜酒馆',
    role: '温柔真诚的日常聊天老友',
    mode: 'companion',
    avatar: 'swarm:violet',
    traits: '像真人发微信一样自然松弛，不列死板条框',
    prompt: '你是用户信任的生活伙伴，陪聊解闷、倾听日常并悄悄记住关键约定。语气真实温暖、简短接梗，严禁列报告式大纲或使用 Emoji。',
    tools: ['remember_fact', 'recall_memory'],
  },
]

function applyArchetype(preset: (typeof STARTER_ARCHETYPES)[number]) {
  name.value = preset.name
  roleTitle.value = preset.role
  avatarStyle.value = preset.avatar
  traits.value = preset.traits
  systemPrompt.value = preset.prompt
  botMode.value = preset.mode
  nextTick(() => {
    allowedTools.value = availableTools.value
      .filter((t) => preset.tools.includes(t.id))
      .map((t) => t.id)
  })
}

const availableTools = computed(() => store.state.availableTools || [])
const groupedTools = computed(() => groupAvailableTools(availableTools.value))
const filteredGroupedTools = computed(() => {
  const q = toolSearch.value.trim().toLowerCase()
  if (!q) return groupedTools.value
  return groupedTools.value
    .map((grp) => ({
      ...grp,
      tools: grp.tools.filter(
        (t) =>
          t.id.toLowerCase().includes(q) ||
          t.shortLabel.toLowerCase().includes(q) ||
          (t.description || '').toLowerCase().includes(q)
      ),
    }))
    .filter((grp) => grp.tools.length > 0)
})
const availableSkills = computed(() => (store.state.skills || []).filter((s) => s.enabled !== false))

const availableSpecialistBots = computed(() =>
  store.state.bots.filter((b) => !b.is_group)
)

function applyBotModeDefaults(nextMode: BotMode) {
  if (nextMode === 'companion') {
    // Companion friend bots should converse naturally without forced ops tools unless explicitly selected
    allowedTools.value = availableTools.value
      .filter((t) => t.id === 'remember_fact' || t.id === 'recall_memory')
      .map((t) => t.id)
    enableThinking.value = false
  } else if (nextMode === 'worker') {
    allowedTools.value = availableTools.value.map((t) => t.id)
    enableThinking.value = true
  } else {
    allowedTools.value = availableTools.value.map((t) => t.id)
    enableThinking.value = true
  }
}

watch(botMode, (next) => {
  applyBotModeDefaults(next)
})

watch(
  () => store.state.showCreateBotModal,
  (open) => {
    if (!open) return
    if (!store.modelOptions.value.some((o) => o.value === model.value)) {
      model.value = store.modelOptions.value[0]?.value || 'auto:free'
    }
    allowPaidFallback.value = false
    if (allowedTools.value.length === 0 && botMode.value !== 'companion' && availableTools.value.length > 0) {
      allowedTools.value = availableTools.value.map((t) => t.id)
    }
    if (groupMembers.value.length === 0) {
      groupMembers.value = availableSpecialistBots.value.slice(0, 3).map((b) => b.id)
    }
    nextTick(() => {
      if (mode.value === 'group') groupNameInputRef.value?.focus()
      else botNameInputRef.value?.focus()
    })
  },
  { immediate: true }
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

function openSkillsWorkshop() {
  closeModal()
  store.state.showSkillsModal = true
}

async function handleSubmit() {
  if (saving.value) return
  if (mode.value === 'group') {
    if (!groupName.value.trim()) {
      store.notify('请输入群聊名称')
      groupNameInputRef.value?.focus()
      return
    }
    if (groupMembers.value.length === 0) {
      store.notify('请至少选择 1 位 Bot 加入群聊')
      return
    }
    saving.value = true
    try {
      const selectedBots = availableSpecialistBots.value.filter((b) =>
        groupMembers.value.includes(b.id)
      )
      const toolSet = new Set<string>()
      const skillSet = new Set<string>()
      let derivedModel = groupModel.value || store.modelOptions.value[0]?.value || 'auto:free'
      let allCompanion = selectedBots.length > 0
      selectedBots.forEach((b) => {
        if (!groupModel.value && b.model && derivedModel === (store.modelOptions.value[0]?.value || 'auto:free')) {
          derivedModel = b.model
        }
        if (b.bot_mode !== 'companion') allCompanion = false
        parseJsonStringArray(b.allowed_tools).forEach((t) => toolSet.add(t))
        parseJsonStringArray(b.skill_ids).forEach((s) => skillSet.add(s))
      })

      const res = await BotHubApi.createBot({
        name: groupName.value.trim(),
        role_title: groupDesc.value.trim() || `多 Bot 协同群 (${groupMembers.value.length} 成员)`,
        avatar_style: groupAvatar.value,
        model: derivedModel,
        allow_paid_fallback: false,
        bot_mode: allCompanion ? 'companion' : 'hybrid',
        memory_policy: 'isolated',
        enable_thinking: !allCompanion,
        is_group: true,
        member_ids: JSON.stringify(groupMembers.value),
        group_mode: groupMode.value,
        announcement: groupAnnouncement.value.trim(),
        allowed_tools: JSON.stringify(Array.from(toolSet)),
        skill_ids: JSON.stringify(Array.from(skillSet)),
        system_prompt: groupDesc.value.trim() || '根据群内成员性格与职责自然交流协同。',
      })
      store.notify('多 Bot 协同群聊创建成功')
      groupName.value = ''
      groupDesc.value = ''
      groupAnnouncement.value = ''
      groupMode.value = 'smart'
      closeModal()
      if (res.bot) {
        store.state.bots = [res.bot, ...store.state.bots.filter((b) => b.id !== res.bot.id)]
        void store.selectBot(res.bot.id)
      }
      void store.refreshWorkspaceSlices(['bots'])
    } catch (err: any) {
      store.notify(err.message)
    } finally {
      saving.value = false
    }
    return
  }

  if (!name.value.trim()) {
    store.notify('请输入 Bot 名称')
    botNameInputRef.value?.focus()
    return
  }
  saving.value = true
  try {
    const res = await BotHubApi.createBot({
      name: name.value.trim(),
      role_title: roleTitle.value.trim() || (botMode.value === 'companion' ? '专属生活伙伴' : '专属智能助理'),
      avatar_style: avatarStyle.value,
      model: model.value,
      allow_paid_fallback: allowPaidFallback.value,
      bot_mode: botMode.value,
      memory_policy: memoryPolicy.value,
      traits: traits.value.trim(),
      enable_thinking: enableThinking.value,
      is_group: false,
      member_ids: '[]',
      allowed_tools: JSON.stringify(allowedTools.value),
      skill_ids: JSON.stringify(selectedSkillIds.value),
      system_prompt: systemPrompt.value.trim(),
    })
    store.notify('专属 Bot 创建并激活成功')
    name.value = ''
    roleTitle.value = ''
    traits.value = ''
    systemPrompt.value = ''
    selectedSkillIds.value = []
    closeModal()
    if (res.bot) {
      store.state.bots = [res.bot, ...store.state.bots.filter((b) => b.id !== res.bot.id)]
      void store.selectBot(res.bot.id)
    }
    void store.refreshWorkspaceSlices(['bots'])
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
    :title="mode === 'group' ? '组建多 Bot 协同群聊' : '创造全新专属 Bot 伙伴'"
    :subtitle="mode === 'group' ? '拉起独立人格成员在同一话题流按需互动与协作' : '赋予独立性格、记忆域、交流定位以及定制技能/工具'"
    max-width="2xl"
    @close="closeModal"
  >
    <template #header-actions>
      <button
        type="button"
        @click="resetCreateForm"
        class="px-2.5 py-1 rounded-lg border border-line text-xs text-fg-muted hover:text-fg cursor-pointer"
      >
        重置表单
      </button>
    </template>
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
        <!-- Mode 1: Single Specialist / Companion Bot -->
        <div v-if="mode === 'bot'" key="bot" class="flex flex-col gap-3.5">
          <div class="flex items-center gap-1.5 flex-wrap text-xs">
            <span class="text-fg-muted">快速人设模板:</span>
            <button
              v-for="arc in STARTER_ARCHETYPES"
              :key="arc.label"
              type="button"
              @click="applyArchetype(arc)"
              class="px-2.5 py-1 rounded-full border border-line bg-canvas text-fg-sub hover:border-accent-border hover:text-accent-fg cursor-pointer"
            >
              {{ arc.label }}
            </button>
          </div>
          <BotAvatarPicker v-model="avatarStyle" :preview-name="name || '新 Bot 伙伴'" />

          <div class="modal-section flex flex-col gap-2">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-medium text-fg">交流定位 / 角色模式</span>
              <span class="text-xs text-fg-muted">
                {{
                  botMode === 'companion'
                    ? '像真人朋友一样自然聊天陪伴，不主动套用死板汇报格式'
                    : botMode === 'worker'
                    ? '结果导向的干活专家，优先核验工具数据再下结论'
                    : '兼顾自然沟通与工具执行'
                }}
              </span>
            </div>
            <SegmentedTabs v-model="botMode" :items="BOT_MODE_TABS" variant="subtle" aria-label="角色模式" />
          </div>

          <div class="modal-section grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label class="flex flex-col gap-1.5 text-xs text-fg-sub">
              <span>Bot 名称</span>
              <input
                ref="botNameInputRef"
                v-model="name"
                type="text"
                placeholder="如：阿彻 / 云账单管家 / 深夜酒馆"
                required
                class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-line bg-canvas px-3 py-2 text-sm text-fg focus:border-accent"
              />
            </label>
            <label class="flex flex-col gap-1.5 text-xs text-fg-sub">
              <span>关系定位 / 职责头衔</span>
              <input
                v-model="roleTitle"
                type="text"
                :placeholder="botMode === 'companion' ? '如：毒舌却温暖的老朋友' : '如：服务器巡检与成本分析专员'"
                required
                class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-line bg-canvas px-3 py-2 text-sm text-fg focus:border-accent"
              />
            </label>
            <label class="sm:col-span-2 flex flex-col gap-1.5 text-xs text-fg-sub">
              <span>性格特质与说话口吻</span>
              <input
                v-model="traits"
                type="text"
                placeholder="如：说话简短直接、爱接梗、像真人发微信，不要每次列一二三点"
                class="min-h-[40px] rounded-xl border border-line bg-canvas px-3 py-2 text-xs text-fg focus:border-accent"
              />
            </label>
            <div class="flex flex-col gap-1.5 text-xs text-fg-sub">
              <span>推理模型 (动态实时读取)</span>
              <CustomSelect v-model="model" :options="store.modelOptions.value" aria-label="默认推理模型" />
            </div>
            <div class="flex flex-col gap-1.5 text-xs text-fg-sub">
              <span>记忆隔离策略</span>
              <CustomSelect v-model="memoryPolicy" :options="MEMORY_POLICY_OPTIONS" aria-label="记忆隔离策略" />
            </div>
            <div class="sm:col-span-2 flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 pt-1">
              <ToggleSwitch v-model="enableThinking" label="默认开启深度思考推理链" />
              <ToggleSwitch
                v-model="allowPaidFallback"
                label="允许付费兜底"
                description="关闭则严格限免费模型池"
              />
            </div>
            <div class="sm:col-span-2">
              <ModelRoutePreview
                :selected-model="model"
                :allow-paid-fallback="allowPaidFallback"
                :need-tools="allowedTools.length > 0"
              />
            </div>
          </div>

          <div class="modal-section flex flex-col gap-3">
            <label class="flex flex-col gap-1.5 text-xs text-fg-sub">
              <span>灵魂背景与行为准则 (System Prompt)</span>
              <textarea
                v-model="systemPrompt"
                rows="3"
                placeholder="描写它的过往经历、与你的关系、遇到不同话题时该如何回应..."
                class="auto-size-textarea w-full rounded-xl border border-line bg-canvas px-3 py-2 text-sm text-fg focus:border-accent resize-y"
              />
            </label>

            <div class="flex flex-col gap-2 text-xs text-fg-sub">
              <div class="flex items-center justify-between gap-2 flex-wrap">
                <span>内置工具白名单 ({{ allowedTools.length }}/{{ availableTools.length }})</span>
                <div class="flex items-center gap-1.5 text-xs flex-wrap">
                  <input
                    v-model="toolSearch"
                    type="search"
                    aria-label="搜索工具"
                    placeholder="筛选工具..."
                    @keydown.escape="clearSearchOnEscape($event, toolSearch, () => { toolSearch = '' })"
                    class="w-28 min-h-[26px] rounded-lg border border-line bg-canvas px-2 text-xs text-fg placeholder:text-fg-muted focus:border-accent focus:outline-none"
                  />
                  <button
                    type="button"
                    @click="allowedTools = availableTools.map((t) => t.id)"
                    class="text-accent-fg hover:underline cursor-pointer"
                  >
                    全选
                  </button>
                  <span class="text-fg-muted">·</span>
                  <button
                    type="button"
                    @click="allowedTools = availableTools.filter((t) => ['manage_bots','manage_todos','manage_notes','manage_stash','manage_tasks','workspace_briefing','send_notification','web_search','read_url','calculate_eval','datetime_calc','remember_fact','recall_memory','manage_memory','consult_bot'].includes(t.id)).map((t) => t.id)"
                    class="text-info hover:underline cursor-pointer"
                  >
                    个人总助推荐
                  </button>
                  <span class="text-fg-muted">·</span>
                  <button
                    type="button"
                    @click="allowedTools = []"
                    class="text-fg-muted hover:underline cursor-pointer"
                  >
                    清空
                  </button>
                </div>
              </div>
              <div class="flex flex-col gap-2">
                <div v-if="filteredGroupedTools.length === 0" class="text-xs text-fg-muted py-2">
                  无匹配工具「{{ toolSearch }}」
                </div>
                <div v-for="grp in filteredGroupedTools" :key="grp.id" class="flex flex-col gap-1">
                  <span class="text-xs font-mono text-fg-muted">{{ grp.label }}</span>
                  <div class="flex flex-wrap gap-1.5">
                    <label
                      v-for="tool in grp.tools"
                      :key="tool.id"
                      :title="tool.description"
                      class="min-h-[32px] inline-flex items-center gap-1.5 px-2.5 py-1 rounded-xl border border-line bg-canvas text-xs text-fg-sub cursor-pointer hover:border-accent-border has-[:checked]:border-accent-border has-[:checked]:bg-accent-soft has-[:checked]:text-accent-fg transition-colors"
                    >
                      <input v-model="allowedTools" type="checkbox" :value="tool.id" class="accent-accent" />
                      <span>{{ tool.shortLabel }}</span>
                    </label>
                  </div>
                </div>
              </div>
            </div>

            <div class="flex flex-col gap-1.5 text-xs text-fg-sub pt-1 border-t border-line">
              <div class="flex items-center justify-between">
                <span>挂载自定义技能 ({{ selectedSkillIds.length }}/{{ availableSkills.length }})</span>
                <button
                  type="button"
                  @click="openSkillsWorkshop"
                  class="text-xs text-accent-fg hover:underline cursor-pointer"
                >
                  + 去技能工坊新建
                </button>
              </div>
              <div v-if="availableSkills.length === 0" class="text-xs text-fg-muted">
                暂无已启用的自定义技能，可随时在「技能工坊」创建后回来绑定。
              </div>
              <div v-else class="flex flex-wrap gap-1.5">
                <label
                  v-for="sk in availableSkills"
                  :key="sk.id"
                  class="min-h-[34px] inline-flex items-center gap-1.5 px-2.5 py-1 rounded-xl border border-line bg-canvas text-xs text-fg-sub cursor-pointer hover:border-info-border has-[:checked]:border-info-border has-[:checked]:bg-info-soft has-[:checked]:text-info transition-colors"
                >
                  <input v-model="selectedSkillIds" type="checkbox" :value="sk.id" class="accent-cyan-400" />
                  <span>{{ sk.name }}</span>
                  <span class="font-mono text-xs opacity-70">{{ sk.kind === 'webhook' ? 'API' : 'Doc' }}</span>
                </label>
              </div>
            </div>
          </div>
        </div>

        <!-- Mode 2: Multi-Bot Group Room -->
        <div v-else key="group" class="flex flex-col gap-3.5">
          <BotAvatarPicker v-model="groupAvatar" :preview-name="groupName || '多 Bot 协同群'" />

          <div class="modal-section grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label class="flex flex-col gap-1.5 text-xs text-fg-sub">
              <span>群聊名称</span>
              <input
                ref="groupNameInputRef"
                v-model="groupName"
                type="text"
                placeholder="如：深夜吐槽茶话会 / 生产故障联合群"
                required
                class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-line bg-canvas px-3 py-2 text-sm text-fg focus:border-accent"
              />
            </label>
            <label class="flex flex-col gap-1.5 text-xs text-fg-sub">
              <span>群聊定位 / 议题说明</span>
              <input
                v-model="groupDesc"
                type="text"
                placeholder="如：轻松聊天互相接梗 / 运维评估"
                class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-line bg-canvas px-3 py-2 text-sm text-fg focus:border-accent"
              />
            </label>
            <div class="sm:col-span-2 flex flex-col gap-1.5 text-xs text-fg-sub">
              <span>默认协同策略</span>
              <SegmentedTabs
                v-model="groupMode"
                :items="[
                  { id: 'smart', label: '智能编排' },
                  { id: 'sequential', label: '轮流发言' },
                  { id: 'parallel', label: '并发思考' },
                  { id: 'mention_only', label: '仅 @点名' },
                ]"
                variant="subtle"
                aria-label="群协同策略"
              />
            </div>
            <div class="sm:col-span-2 flex flex-col gap-1.5 text-xs text-fg-sub">
              <span>群协调主力模型 (可选)</span>
              <CustomSelect
                v-model="groupModel"
                :options="[{ value: '', label: '自动沿用群内成员主力模型' }, ...store.modelOptions.value]"
                aria-label="群协调主力模型"
              />
            </div>
            <label class="sm:col-span-2 flex flex-col gap-1.5 text-xs text-fg-sub">
              <span>初始群公告（可选，置顶显示在对话流顶部）</span>
              <input
                v-model="groupAnnouncement"
                type="text"
                placeholder="如：各自带入本人性格与专长自然接话"
                class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-line bg-canvas px-3 py-2 text-sm text-fg focus:border-accent"
              />
            </label>
          </div>

          <div class="modal-section flex flex-col gap-2.5">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-medium text-fg">选择进群成员（已选 {{ groupMembers.length }} 位）</span>
              <span class="font-mono text-xs text-fg-muted">自动汇集所选成员的性格与可用技能</span>
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
                  'group vesper-card rounded-xl p-2.5 flex items-center gap-2.5 text-left cursor-pointer',
                  groupMembers.includes(b.id)
                    ? 'border-accent-border bg-accent-soft'
                    : 'border-line opacity-80 hover:opacity-100'
                ]"
              >
                <BotAvatar :avatar-style="b.avatar_style" :state="b.state" size="md" />
                <div class="flex-1 min-w-0">
                  <div class="text-xs font-semibold text-fg truncate">{{ b.name }}</div>
                  <div class="text-xs text-fg-muted truncate">{{ b.role_title }}</div>
                </div>
                <span
                  :class="[
                    'w-5 h-5 rounded-full border flex items-center justify-center shrink-0 transition-colors',
                    groupMembers.includes(b.id)
                      ? 'bg-accent border-accent-border text-on-accent'
                      : 'border-line text-transparent'
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

      <div class="flex items-center justify-between gap-2 pt-2 border-t border-line shrink-0 flex-wrap">
        <span class="font-mono text-xs text-fg-muted">
          <template v-if="mode === 'bot'">
            {{ BOT_MODE_TABS.find((t) => t.id === botMode)?.label }} · {{ allowedTools.length }} 工具 · {{ selectedSkillIds.length }} 技能
          </template>
          <template v-else>
            已拉起 {{ groupMembers.length }} 位群成员 · {{ groupModeLabel(groupMode) }}
          </template>
        </span>
        <div class="flex items-center gap-2 ml-auto">
          <button
            type="button"
            @click="closeModal"
            class="min-h-[44px] sm:min-h-[36px] px-4 py-2 rounded-xl border border-line text-xs text-fg-sub hover:bg-hover cursor-pointer"
          >
            取消
          </button>
          <button
            type="submit"
            :disabled="saving"
            class="min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-xl bg-accent hover:bg-accent-hover text-on-accent font-semibold text-xs cursor-pointer disabled:opacity-50"
          >
            {{ saving ? '创建中...' : (mode === 'group' ? '创建协同群聊' : '创建并激活 Bot') }}
          </button>
        </div>
      </div>
    </form>
  </BaseModal>
</template>
