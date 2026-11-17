<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import { groupAvailableTools } from '../../utils/toolMeta'
import type { AgentMemory, BotMode, MemoryPolicy } from '../../types/bothub'
import BotAvatar from '../BotAvatar.vue'
import BotAvatarPicker from '../BotAvatarPicker.vue'
import CustomSelect from '../CustomSelect.vue'
import SegmentedTabs from '../SegmentedTabs.vue'
import ToggleSwitch from '../ToggleSwitch.vue'

const store = useAppStore()
const bot = store.activeBot

const cfgName = ref('')
const cfgRole = ref('')
const cfgPrompt = ref('')
const cfgModel = ref('primary')
const cfgAvatar = ref('butler:gold')
const cfgThinking = ref(true)
const cfgBotMode = ref<BotMode>('hybrid')
const cfgMemoryPolicy = ref<MemoryPolicy>('standard')
const cfgTraits = ref('')
const cfgTools = ref<string[]>([])
const cfgSkillIds = ref<string[]>([])
const cfgMembers = ref<string[]>([])
const cfgGroupMode = ref('smart')
const cfgToolFilter = ref('')
const savingProfile = ref(false)

const newMemKey = ref('')
const newMemContent = ref('')
const newMemCategory = ref('preference')
const newMemImportance = ref(3)
const newMemPinned = ref(false)
const savingMem = ref(false)
const consolidatingMem = ref(false)
const editingMemId = ref<number | null>(null)
const editMemContent = ref('')

const BOT_MODE_TABS = [
  { id: 'companion', label: '陪伴聊天型' },
  { id: 'hybrid', label: '综合协同型' },
  { id: 'worker', label: '干活执行型' },
]

const MEMORY_POLICY_OPTIONS = [
  { value: 'isolated', label: '严格私有 (仅 Bot 自身可见)' },
  { value: 'standard', label: '私有 + 全局共享事实' },
]

const isGroupBot = computed(() => Boolean(bot.value?.is_group))
const availableSpecialistBots = computed(() => store.state.bots.filter((b) => !b.is_group))
const availableTools = computed(() => store.state.availableTools || [])
const availableSkills = computed(() => (store.state.skills || []).filter((s) => s.enabled !== false))
const groupedTools = computed(() => groupAvailableTools(availableTools.value))

const filteredGroupedTools = computed(() => {
  const q = cfgToolFilter.value.trim().toLowerCase()
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

const hasProfileChanges = computed(() => {
  const b = bot.value
  if (!b) return false
  const normModel = !b.model || b.model === 'deepseek-flash' ? 'primary' : b.model
  const normMode = (b.bot_mode as BotMode) || 'hybrid'
  const normPolicy = (b.memory_policy as MemoryPolicy) || 'standard'
  const parseSorted = (raw?: string) => {
    try {
      const a = JSON.parse(raw || '[]')
      return Array.isArray(a) ? a.toSorted().join(',') : ''
    } catch {
      return ''
    }
  }
  return (
    cfgName.value.trim() !== (b.name || '') ||
    cfgRole.value.trim() !== (b.role_title || '') ||
    cfgPrompt.value.trim() !== (b.system_prompt || '').trim() ||
    cfgModel.value !== normModel ||
    cfgAvatar.value !== (b.avatar_style || 'cyan') ||
    cfgThinking.value !== (b.enable_thinking !== false) ||
    cfgBotMode.value !== normMode ||
    cfgMemoryPolicy.value !== normPolicy ||
    cfgTraits.value.trim() !== (b.traits || '').trim() ||
    cfgGroupMode.value !== (b.group_mode || 'smart') ||
    cfgTools.value.toSorted().join(',') !== parseSorted(b.allowed_tools) ||
    cfgSkillIds.value.toSorted().join(',') !== parseSorted(b.skill_ids) ||
    cfgMembers.value.toSorted().join(',') !== parseSorted(b.member_ids)
  )
})

const scopedMemories = computed(() => {
  const serverMems = store.state.botMemories || []
  if (serverMems.length > 0) return serverMems
  const cur = bot.value
  if (!cur) return []
  const bScope = `bot:${cur.id}`
  if (cur.memory_policy === 'isolated') {
    return store.state.memories.filter((m) => m.scope === bScope)
  }
  return store.state.memories.filter((m) => !m.scope || m.scope === 'global' || m.scope === bScope)
})

watch(
  bot,
  (b) => {
    if (!b) return
    const normModel = !b.model || b.model === 'deepseek-flash' ? 'primary' : b.model
    cfgName.value = b.name
    cfgRole.value = b.role_title
    cfgPrompt.value = b.system_prompt
    cfgModel.value = normModel
    cfgAvatar.value = b.avatar_style || 'cyan'
    cfgThinking.value = b.enable_thinking !== false
    cfgBotMode.value = (b.bot_mode as BotMode) || 'hybrid'
    cfgMemoryPolicy.value = (b.memory_policy as MemoryPolicy) || 'standard'
    cfgTraits.value = b.traits || ''
    cfgGroupMode.value = b.group_mode || 'smart'
    editingMemId.value = null
    try {
      const arr = JSON.parse(b.allowed_tools || '[]')
      cfgTools.value = Array.isArray(arr) ? arr : []
    } catch {
      cfgTools.value = []
    }
    try {
      const sArr = JSON.parse(b.skill_ids || '[]')
      cfgSkillIds.value = Array.isArray(sArr) ? sArr : []
    } catch {
      cfgSkillIds.value = []
    }
    try {
      const mArr = JSON.parse(b.member_ids || '[]')
      cfgMembers.value = Array.isArray(mArr) ? mArr : []
    } catch {
      cfgMembers.value = []
    }
  },
  { immediate: true }
)

function toggleCfgMember(id: string) {
  if (cfgMembers.value.includes(id)) {
    cfgMembers.value = cfgMembers.value.filter((x) => x !== id)
  } else {
    cfgMembers.value = [...cfgMembers.value, id]
  }
}

async function saveBotProfile() {
  if (!bot.value || savingProfile.value) return
  savingProfile.value = true
  try {
    await BotHubApi.updateBot(bot.value.id, {
      name: cfgName.value.trim(),
      role_title: cfgRole.value.trim(),
      avatar_style: cfgAvatar.value,
      model: cfgModel.value,
      bot_mode: cfgBotMode.value,
      memory_policy: cfgMemoryPolicy.value,
      traits: cfgTraits.value.trim(),
      enable_thinking: cfgThinking.value,
      system_prompt: cfgPrompt.value.trim(),
      allowed_tools: JSON.stringify(cfgTools.value),
      skill_ids: JSON.stringify(cfgSkillIds.value),
      member_ids: JSON.stringify(cfgMembers.value),
      group_mode: cfgGroupMode.value,
      is_pinned: bot.value.is_pinned,
    })
    store.notify('Bot 人设、记忆策略与技能权限已更新')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    savingProfile.value = false
  }
}

async function deleteCustomBot() {
  if (!bot.value) return
  if (!window.confirm(`确定删除 [${bot.value.name}] 吗？其专属聊天流将一并移除。`)) return
  await BotHubApi.deleteBot(bot.value.id)
  store.notify(`已删除「${bot.value.name}」`)
  await store.loadDashboard(false)
  if (store.state.bots.length > 0) {
    await store.selectBot(store.state.bots[0].id)
  }
}

async function addBotMemory() {
  if (!bot.value || !newMemKey.value.trim() || !newMemContent.value.trim() || savingMem.value) return
  savingMem.value = true
  try {
    await BotHubApi.saveMemory({
      scope: `bot:${bot.value.id}`,
      category: newMemCategory.value,
      fact_key: newMemKey.value.trim(),
      content: newMemContent.value.trim(),
      importance: newMemImportance.value,
      pinned: newMemPinned.value,
    })
    newMemKey.value = ''
    newMemContent.value = ''
    newMemPinned.value = false
    store.notify(`已写入「${bot.value.name}」专属记忆`)
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '写入记忆失败')
  } finally {
    savingMem.value = false
  }
}

async function saveInlineMemEdit(mem: AgentMemory) {
  if (!editMemContent.value.trim()) return
  try {
    await BotHubApi.updateMemory(mem.id, { content: editMemContent.value.trim() })
    editingMemId.value = null
    store.notify('记忆条目已更新')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '更新失败')
  }
}

async function toggleBotMemoryPin(mem: AgentMemory) {
  try {
    await BotHubApi.updateMemory(mem.id, { pinned: !mem.pinned })
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '操作失败')
  }
}

async function deleteBotMemory(id: number) {
  try {
    await BotHubApi.deleteMemory(id)
    store.notify('记忆条目已移除')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '移除失败')
  }
}

async function consolidateBotMemories() {
  if (!bot.value || consolidatingMem.value) return
  consolidatingMem.value = true
  try {
    const res = await BotHubApi.consolidateMemories(`bot:${bot.value.id}`)
    store.notify(res.merged > 0 ? `已为当前域合并 ${res.merged} 条冗余记忆` : '当前域记忆无重复冗余')
    if (res.merged > 0) {
      await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
    }
  } catch (err: any) {
    store.notify(err.message || '合并记忆失败')
  } finally {
    consolidatingMem.value = false
  }
}

function openScopedMemoriesManager() {
  if (!bot.value) return
  store.state.memoryModalDefaultScope = `bot:${bot.value.id}`
  store.state.showMemoriesModal = true
}
</script>

<template>
  <div class="flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
    <div class="max-w-4xl mx-auto flex flex-col gap-5">
      <form @submit.prevent="saveBotProfile" class="luxury-card rounded-2xl p-5 flex flex-col gap-4">
        <BotAvatarPicker v-model="cfgAvatar" :preview-name="cfgName || bot?.name" />

        <div class="flex flex-col gap-2 text-xs text-slate-300">
          <span>交流定位 / 角色模式</span>
          <SegmentedTabs v-model="cfgBotMode" :items="BOT_MODE_TABS" variant="subtle" aria-label="交流定位" />
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <label class="flex flex-col gap-1.5 text-xs text-slate-300">
            <span>{{ isGroupBot ? '群聊名称' : 'Bot 名称' }}</span>
            <input v-model="cfgName" type="text" required class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60" />
          </label>
          <label class="flex flex-col gap-1.5 text-xs text-slate-300">
            <span>职责定位 / 关系称谓</span>
            <input v-model="cfgRole" type="text" required class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60" />
          </label>
          <label class="md:col-span-2 flex flex-col gap-1.5 text-xs text-slate-300">
            <span>性格特质与沟通口吻</span>
            <input
              v-model="cfgTraits"
              type="text"
              placeholder="如：说话像真人发消息一样简短有温度 / 严谨务实直击根因"
              class="min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-xs text-white focus:border-[#f2b880]/60"
            />
          </label>
          <div class="flex flex-col gap-1.5 text-xs text-slate-300">
            <span>默认推理模型</span>
            <CustomSelect v-model="cfgModel" :options="store.modelOptions.value" aria-label="默认推理模型" />
          </div>
          <div class="flex flex-col gap-1.5 text-xs text-slate-300">
            <span>记忆隔离策略</span>
            <CustomSelect v-model="cfgMemoryPolicy" :options="MEMORY_POLICY_OPTIONS" aria-label="记忆隔离策略" />
          </div>
          <div class="md:col-span-2 flex items-center justify-between">
            <ToggleSwitch v-model="cfgThinking" label="默认开启深度思考" />
          </div>

          <div v-if="isGroupBot" class="md:col-span-2 flex flex-col gap-2.5 text-xs text-slate-300">
            <div class="flex items-center justify-between gap-2">
              <span>群协同策略</span>
            </div>
            <SegmentedTabs
              v-model="cfgGroupMode"
              :items="[
                { id: 'smart', label: '智能协同' },
                { id: 'sequential', label: '全员接力' },
                { id: 'parallel', label: '全员并发' },
                { id: 'mention_only', label: '仅 @点名' },
              ]"
              variant="subtle"
              aria-label="群协同策略"
            />

            <span>群聊协同成员 ({{ cfgMembers.length }} 位)</span>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <button
                v-for="sp in availableSpecialistBots"
                :key="sp.id"
                type="button"
                @click="toggleCfgMember(sp.id)"
                :aria-pressed="cfgMembers.includes(sp.id)"
                :class="[
                  'group flex items-center gap-2.5 px-3 py-2 rounded-xl border text-left cursor-pointer',
                  cfgMembers.includes(sp.id)
                    ? 'border-[#f2b880]/60 bg-[#f2b880]/12 text-white'
                    : 'border-[#fff8ee]/10 bg-[#070a16]/70 text-slate-400 hover:border-[#fff8ee]/25'
                ]"
              >
                <BotAvatar :avatar-style="sp.avatar_style" :state="sp.state" size="sm" />
                <span class="text-xs font-medium flex-1 truncate">{{ sp.name }}</span>
                <span class="text-[11px] font-mono">{{ cfgMembers.includes(sp.id) ? '在群' : '未选' }}</span>
              </button>
            </div>
          </div>

          <label class="md:col-span-2 flex flex-col gap-1.5 text-xs text-slate-300">
            <div class="flex items-center justify-between gap-2">
              <span>专属人设与行为约束 (System Prompt)</span>
              <span class="font-mono text-[10.5px] text-slate-500">{{ cfgPrompt.length }} 字符</span>
            </div>
            <textarea v-model="cfgPrompt" rows="4" class="auto-size-textarea rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 resize-y" />
          </label>

          <div class="md:col-span-2 flex flex-col gap-2 text-xs text-slate-300">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <span>绑定内置工具集 ({{ cfgTools.length }}/{{ availableTools.length }})</span>
              <div class="flex items-center gap-2 text-[11px] flex-wrap">
                <input
                  v-model="cfgToolFilter"
                  type="search"
                  aria-label="筛选工具"
                  placeholder="筛选工具..."
                  @keydown.escape.stop="cfgToolFilter = ''"
                  class="w-32 min-h-[26px] rounded-lg border border-[#fff8ee]/12 bg-[#060a16] px-2 text-[11px] text-white placeholder-slate-500 focus:border-[#f2b880]/55 focus:outline-none"
                />
                <button
                  type="button"
                  @click="cfgTools = availableTools.map((t) => t.id)"
                  class="text-[#f2b880] hover:underline cursor-pointer"
                >
                  全选
                </button>
                <button
                  type="button"
                  @click="cfgTools = availableTools.filter((t) => ['manage_todos','manage_notes','manage_tasks','workspace_briefing','send_notification','web_search','read_url','calculate_eval','datetime_calc','remember_fact','recall_memory','manage_memory','consult_bot'].includes(t.id)).map((t) => t.id)"
                  class="text-cyan-300 hover:underline cursor-pointer"
                >
                  总助推荐
                </button>
                <button
                  type="button"
                  @click="cfgTools = []"
                  class="text-slate-400 hover:underline cursor-pointer"
                >
                  清空
                </button>
              </div>
            </div>
            <div class="flex flex-col gap-2.5">
              <div v-if="filteredGroupedTools.length === 0" class="text-[11px] text-slate-500 py-2">
                未找到匹配「{{ cfgToolFilter }}」的内置工具
              </div>
              <div v-for="grp in filteredGroupedTools" :key="grp.id" class="flex flex-col gap-1.5">
                <span class="text-[10.5px] font-mono text-slate-500">{{ grp.label }}</span>
                <div class="flex flex-wrap gap-2">
                  <label
                    v-for="tool in grp.tools"
                    :key="tool.id"
                    :title="tool.description"
                    class="min-h-[34px] inline-flex items-center gap-2 px-3 py-1 rounded-xl border border-[#fff8ee]/12 bg-[#060a16]/75 text-xs text-slate-300 cursor-pointer hover:border-[#f2b880]/40 has-[:checked]:border-[#f2b880]/55 has-[:checked]:bg-[#f2b880]/14 has-[:checked]:text-[#f7cba0] transition-colors"
                  >
                    <input v-model="cfgTools" type="checkbox" :value="tool.id" class="accent-[#f2b880]" />
                    <span>{{ tool.shortLabel }}</span>
                  </label>
                </div>
              </div>
            </div>
          </div>

          <div class="md:col-span-2 flex flex-col gap-2 text-xs text-slate-300 pt-2 border-t border-[#fff8ee]/8">
            <div class="flex items-center justify-between">
              <span>挂载自定义技能 ({{ cfgSkillIds.length }}/{{ availableSkills.length }})</span>
              <button
                type="button"
                @click="store.state.showSkillsModal = true"
                class="text-[11px] text-[#f2b880] hover:underline cursor-pointer"
              >
                打开技能工坊
              </button>
            </div>
            <div v-if="availableSkills.length === 0" class="text-[11px] text-slate-500">
              暂无已启用的自定义技能，点击右上「打开技能工坊」创建自己的 Playbook 或 Webhook 工具。
            </div>
            <div v-else class="flex flex-wrap gap-2">
              <label
                v-for="sk in availableSkills"
                :key="sk.id"
                class="min-h-[36px] inline-flex items-center gap-2 px-3 py-1 rounded-xl border border-[#fff8ee]/12 bg-[#060a16]/75 text-xs text-slate-300 cursor-pointer hover:border-cyan-400/40 has-[:checked]:border-cyan-400/55 has-[:checked]:bg-cyan-500/14 has-[:checked]:text-cyan-200 transition-colors"
              >
                <input v-model="cfgSkillIds" type="checkbox" :value="sk.id" class="accent-cyan-400" />
                <span>{{ sk.name }}</span>
                <span class="font-mono text-[10px] opacity-70">{{ sk.kind === 'webhook' ? 'API' : 'Doc' }}</span>
              </label>
            </div>
          </div>
        </div>

        <div
          v-if="hasProfileChanges"
          class="flex items-center justify-between gap-2 rounded-xl border border-[#f2b880]/35 bg-[#f2b880]/10 px-3 py-2 text-xs"
        >
          <span class="text-[#f7cba0]">检测到人设或工具权限已更改，记得点击右侧保存生效</span>
          <span class="font-mono text-[10px] text-[#f2b880]">未保存</span>
        </div>

        <div class="flex items-center justify-between gap-3 flex-wrap pt-3 border-t border-[#fff8ee]/10">
          <button
            v-if="bot"
            type="button"
            @click="deleteCustomBot"
            class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-rose-400/40 bg-rose-500/15 text-rose-300 hover:bg-rose-500/25 text-xs cursor-pointer"
          >
            {{ isGroupBot ? '解散此群聊' : '删除此 Bot' }}
          </button>
          <span v-else />
          <button
            type="submit"
            :disabled="savingProfile"
            class="min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50"
          >
            {{ savingProfile ? '保存中...' : '保存配置' }}
          </button>
        </div>
      </form>

      <div class="vesper-card rounded-2xl p-5 flex flex-col gap-3">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div>
            <h3 class="font-semibold text-sm text-white">
              {{ isGroupBot ? '当前群聊专属记忆' : '当前 Bot 记忆域' }} ({{ scopedMemories.length }})
            </h3>
            <p class="text-[11px] text-slate-400 mt-0.5">
              {{
                cfgMemoryPolicy === 'isolated'
                  ? '当前策略为严格私有：仅读取当前 Bot/群聊的独立记忆，绝不与其他 Bot 混淆。'
                  : '当前策略为私有优先 + 全局共享：当前 Bot 专属记忆权重更高，并保留全局基础设施事实。'
              }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              :disabled="consolidatingMem"
              @click="consolidateBotMemories"
              class="px-3 py-1.5 rounded-xl border border-cyan-400/30 text-xs text-cyan-200 hover:bg-cyan-400/15 cursor-pointer disabled:opacity-50"
            >
              {{ consolidatingMem ? '合并中...' : '合并冗余' }}
            </button>
            <button
              type="button"
              @click="openScopedMemoriesManager"
              class="px-3 py-1.5 rounded-xl border border-[#fff8ee]/12 text-xs text-slate-300 hover:text-[#f2b880] cursor-pointer"
            >
              在记忆库管理
            </button>
          </div>
        </div>

        <form @submit.prevent="addBotMemory" class="grid grid-cols-1 sm:grid-cols-12 gap-2">
          <input
            v-model="newMemKey"
            type="text"
            placeholder="记忆键名 (如 preferred_tone)"
            required
            class="sm:col-span-3 min-h-[36px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-2.5 font-mono text-xs text-white"
          />
          <input
            v-model="newMemContent"
            type="text"
            placeholder="写给它记住的专属事实或偏好..."
            required
            class="sm:col-span-6 min-h-[36px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-2.5 text-xs text-white"
          />
          <label class="sm:col-span-1 inline-flex items-center justify-center gap-1 text-[11px] text-slate-300 cursor-pointer">
            <input v-model="newMemPinned" type="checkbox" class="accent-[#f2b880]" />
            <span>置顶</span>
          </label>
          <button
            type="submit"
            :disabled="savingMem"
            class="sm:col-span-2 min-h-[36px] rounded-xl bg-[#f2b880]/18 border border-[#f2b880]/45 text-[#f7cba0] hover:bg-[#f2b880]/28 text-xs font-medium cursor-pointer disabled:opacity-50"
          >
            {{ savingMem ? '写入中' : '写入专属记忆' }}
          </button>
        </form>

        <div v-if="scopedMemories.length === 0" class="text-xs text-slate-500 py-3">
          暂无关联长期记忆；对话中会自动沉淀偏好与重要背景，也可在上方直接添加。
        </div>
        <div class="flex flex-col gap-2">
          <article
            v-for="mem in scopedMemories"
            :key="mem.id"
            class="rounded-xl border border-[#fff8ee]/10 bg-[#070a16]/75 px-3.5 py-2.5"
          >
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <div class="flex items-center gap-1.5 min-w-0">
                <span
                  v-if="mem.pinned"
                  class="px-1.5 py-0.5 rounded border border-amber-400/50 bg-amber-400/15 font-mono text-[10px] text-amber-200"
                >
                  置顶
                </span>
                <span class="px-1.5 py-0.5 rounded border border-[#fff8ee]/10 font-mono text-[10px] text-slate-400">
                  W{{ mem.importance || 3 }}
                </span>
                <strong class="font-mono text-xs text-[#f2b880]">{{ mem.fact_key }}</strong>
                <span class="font-mono text-[10.5px] text-slate-500">
                  · {{ mem.scope?.startsWith('bot:') ? '本域私有' : '全局共享' }}
                  <template v-if="mem.recall_count"> · 召回 {{ mem.recall_count }} 次</template>
                </span>
              </div>
              <div class="flex items-center gap-1.5">
                <button
                  type="button"
                  @click="toggleBotMemoryPin(mem)"
                  class="px-2 py-0.5 rounded border border-[#fff8ee]/10 text-[10.5px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
                >
                  {{ mem.pinned ? '取消置顶' : '置顶' }}
                </button>
                <button
                  type="button"
                  @click="editingMemId === mem.id ? (editingMemId = null) : (editingMemId = mem.id, editMemContent = mem.content)"
                  class="px-2 py-0.5 rounded border border-[#fff8ee]/10 text-[10.5px] text-slate-300 hover:text-white cursor-pointer"
                >
                  {{ editingMemId === mem.id ? '取消' : '编辑' }}
                </button>
                <button
                  type="button"
                  @click="deleteBotMemory(mem.id)"
                  class="px-2 py-0.5 rounded border border-rose-400/30 text-[10.5px] text-rose-300 hover:bg-rose-500/15 cursor-pointer"
                >
                  移除
                </button>
              </div>
            </div>
            <div v-if="editingMemId === mem.id" class="mt-2 flex gap-2">
              <input
                v-model="editMemContent"
                type="text"
                placeholder="Enter 保存 · Esc 取消"
                @keydown.enter.prevent="saveInlineMemEdit(mem)"
                @keydown.escape.stop="editingMemId = null"
                class="flex-1 min-h-[32px] rounded-lg border border-[#f2b880]/45 bg-[#050811] px-2.5 text-xs text-white"
              />
              <button
                type="button"
                @click="saveInlineMemEdit(mem)"
                class="px-3 py-1 rounded-lg bg-[#f2b880] text-[#070a13] font-semibold text-xs cursor-pointer"
              >
                保存
              </button>
            </div>
            <p v-else class="text-xs text-slate-200 mt-1">{{ mem.content }}</p>
          </article>
        </div>
      </div>
    </div>
  </div>
</template>
