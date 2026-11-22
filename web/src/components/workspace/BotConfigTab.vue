<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import { resolveScopedMemories } from '../../utils/botLook'
import { parseJsonStringArray } from '../../utils/format'
import type { AgentMemory, BotMode, MemoryPolicy } from '../../types/bothub'
import BotAvatar from '../BotAvatar.vue'
import BotAvatarPicker from '../BotAvatarPicker.vue'
import CustomSelect from '../CustomSelect.vue'
import SegmentedTabs from '../SegmentedTabs.vue'
import ToggleSwitch from '../ToggleSwitch.vue'
import ModelRoutePreview from '../ui/ModelRoutePreview.vue'

const store = useAppStore()
const bot = store.activeBot

const cfgName = ref('')
const cfgRole = ref('')
const cfgPrompt = ref('')
const cfgModel = ref('auto:free')
const cfgAllowPaidFallback = ref(false)
const cfgAvatar = ref('butler:gold')
const cfgThinking = ref(true)
const cfgBotMode = ref<BotMode>('hybrid')
const cfgMemoryPolicy = ref<MemoryPolicy>('standard')
const cfgTraits = ref('')
const cfgSkillIds = ref<string[]>([])
const cfgMembers = ref<string[]>([])
const cfgGroupMode = ref('smart')
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
const availableSkills = computed(() => (store.state.skills || []).filter((s) => s.enabled !== false))

const hasProfileChanges = computed(() => {
  const b = bot.value
  if (!b) return false
  const normModel = !b.model || b.model === 'free-pool' ? 'auto:free' : b.model
  const normMode = (b.bot_mode as BotMode) || 'hybrid'
  const normPolicy = (b.memory_policy as MemoryPolicy) || 'standard'
  const parseSorted = (raw?: string) => parseJsonStringArray(raw).toSorted().join(',')
  return (
    cfgName.value.trim() !== (b.name || '') ||
    cfgRole.value.trim() !== (b.role_title || '') ||
    cfgPrompt.value.trim() !== (b.system_prompt || '').trim() ||
    cfgModel.value !== normModel ||
    cfgAllowPaidFallback.value !== Boolean(b.allow_paid_fallback) ||
    cfgAvatar.value !== (b.avatar_style || 'cyan') ||
    cfgThinking.value !== (b.enable_thinking !== false) ||
    cfgBotMode.value !== normMode ||
    cfgMemoryPolicy.value !== normPolicy ||
    cfgTraits.value.trim() !== (b.traits || '').trim() ||
    cfgGroupMode.value !== (b.group_mode || 'smart') ||
    cfgSkillIds.value.toSorted().join(',') !== parseSorted(b.skill_ids) ||
    cfgMembers.value.toSorted().join(',') !== parseSorted(b.member_ids)
  )
})

const scopedMemories = computed(() =>
  resolveScopedMemories(bot.value, store.state.botMemories || [], store.state.memories)
)

watch(
  bot,
  (b) => {
    if (!b) return
    const normModel = !b.model || b.model === 'free-pool' ? 'auto:free' : b.model
    cfgName.value = b.name
    cfgRole.value = b.role_title
    cfgPrompt.value = b.system_prompt
    cfgModel.value = normModel
    cfgAllowPaidFallback.value = Boolean(b.allow_paid_fallback)
    cfgAvatar.value = b.avatar_style || 'cyan'
    cfgThinking.value = b.enable_thinking !== false
    cfgBotMode.value = (b.bot_mode as BotMode) || 'hybrid'
    cfgMemoryPolicy.value = (b.memory_policy as MemoryPolicy) || 'standard'
    cfgTraits.value = b.traits || ''
    cfgGroupMode.value = b.group_mode || 'smart'
    editingMemId.value = null
    cfgSkillIds.value = parseJsonStringArray(b.skill_ids)
    cfgMembers.value = parseJsonStringArray(b.member_ids)
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
      allow_paid_fallback: cfgAllowPaidFallback.value,
      bot_mode: cfgBotMode.value,
      memory_policy: cfgMemoryPolicy.value,
      traits: cfgTraits.value.trim(),
      enable_thinking: cfgThinking.value,
      system_prompt: cfgPrompt.value.trim(),
      allowed_tools: '[]',
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
  const ok = await store.openConfirm({
    title: '删除 Bot',
    message: `确定删除 [${bot.value.name}] 吗？其专属聊天流将一并移除。`,
    confirmText: '确认删除',
    danger: true,
  })
  if (!ok) return
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
      <form @submit.prevent="saveBotProfile" class="vesper-card rounded-xl p-5 flex flex-col gap-4">
        <BotAvatarPicker v-model="cfgAvatar" :preview-name="cfgName || bot?.name" />

        <div class="flex flex-col gap-2 text-xs text-fg-sub">
          <span>交流定位 / 角色模式</span>
          <SegmentedTabs v-model="cfgBotMode" :items="BOT_MODE_TABS" variant="subtle" aria-label="交流定位" />
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <label class="flex flex-col gap-1.5 text-xs text-fg-sub">
            <span>{{ isGroupBot ? '群聊名称' : 'Bot 名称' }}</span>
            <input v-model="cfgName" type="text" required class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-line bg-canvas px-3 py-2 text-sm text-fg focus:border-accent" />
          </label>
          <label class="flex flex-col gap-1.5 text-xs text-fg-sub">
            <span>职责定位 / 关系称谓</span>
            <input v-model="cfgRole" type="text" required class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-line bg-canvas px-3 py-2 text-sm text-fg focus:border-accent" />
          </label>
          <label class="md:col-span-2 flex flex-col gap-1.5 text-xs text-fg-sub">
            <span>性格特质与沟通口吻</span>
            <input
              v-model="cfgTraits"
              type="text"
              placeholder="如：说话像真人发消息一样简短有温度 / 严谨务实直击根因"
              class="min-h-[40px] rounded-xl border border-line bg-canvas px-3 py-2 text-xs text-fg focus:border-accent"
            />
          </label>
          <div class="flex flex-col gap-1.5 text-xs text-fg-sub">
            <span>默认推理模型</span>
            <CustomSelect v-model="cfgModel" :options="store.modelOptions.value" aria-label="默认推理模型" />
          </div>
          <div class="flex flex-col gap-1.5 text-xs text-fg-sub">
            <span>记忆隔离策略</span>
            <CustomSelect v-model="cfgMemoryPolicy" :options="MEMORY_POLICY_OPTIONS" aria-label="记忆隔离策略" />
          </div>
          <div class="md:col-span-2 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <ToggleSwitch v-model="cfgThinking" label="默认开启深度思考" />
            <ToggleSwitch
              v-model="cfgAllowPaidFallback"
              label="允许付费兜底"
              description="开启后，免费池不可用时可回退至付费池"
            />
          </div>
          <div class="md:col-span-2">
            <ModelRoutePreview
              :selected-model="cfgModel"
              :allow-paid-fallback="cfgAllowPaidFallback"
              :need-tools="cfgBotMode !== 'companion' || cfgSkillIds.length > 0"
            />
          </div>

          <div v-if="isGroupBot" class="md:col-span-2 flex flex-col gap-2.5 text-xs text-fg-sub">
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
                    ? 'border-accent-border bg-accent-soft text-fg'
                    : 'border-line bg-canvas text-fg-muted hover:border-line-strong'
                ]"
              >
                <BotAvatar :avatar-style="sp.avatar_style" :state="sp.state" size="sm" />
                <span class="text-xs font-medium flex-1 truncate">{{ sp.name }}</span>
                <span class="text-xs font-mono">{{ cfgMembers.includes(sp.id) ? '在群' : '未选' }}</span>
              </button>
            </div>
          </div>

          <label class="md:col-span-2 flex flex-col gap-1.5 text-xs text-fg-sub">
            <div class="flex items-center justify-between gap-2">
              <span>专属人设与行为约束 (System Prompt)</span>
              <span class="font-mono text-xs text-fg-muted">{{ cfgPrompt.length }} 字符</span>
            </div>
            <textarea v-model="cfgPrompt" rows="4" class="auto-size-textarea rounded-xl border border-line bg-canvas px-3 py-2 text-sm text-fg focus:border-accent resize-y" />
          </label>

          <div class="md:col-span-2 rounded-xl border border-line bg-canvas/70 px-3.5 py-2.5 flex flex-col gap-1 text-xs">
            <div class="flex items-center justify-between gap-2">
              <span class="font-medium text-fg">Agent 原生认知底座 (Native Core)</span>
              <span class="font-mono text-[11px] px-2 py-0.5 rounded-full border border-ok-border bg-ok-soft text-ok">
                已全员永久内置
              </span>
            </div>
            <p class="text-fg-muted leading-relaxed">
              实时多源联网搜索 (web_search)、网页正文读取 (read_url)、长期记忆、精确时间与数学计算、多 Bot 协同及 MCP 动态发现能力已对所有模式 Bot 默认内置，无需手动勾选且永不被裁剪。
            </p>
          </div>

          <div class="md:col-span-2 flex flex-col gap-2 text-xs text-fg-sub pt-2 border-t border-line">
            <div class="flex items-center justify-between">
              <span>已挂载 MCP 服务与扩展 ({{ cfgSkillIds.length }}/{{ availableSkills.length }} · 聊到相关功能时自动发现并永久挂载)</span>
              <button
                type="button"
                @click="store.state.showSkillsModal = true"
                class="text-xs text-accent-fg hover:underline cursor-pointer"
              >
                打开 MCP 中心
              </button>
            </div>
            <div v-if="availableSkills.length === 0" class="text-xs text-fg-muted">
              暂无已启用的 MCP 服务，点击右上「打开 MCP 中心」接入标准 MCP Server 或启用内置业务 MCP。
            </div>
            <div v-else class="flex flex-wrap gap-2">
              <label
                v-for="sk in availableSkills"
                :key="sk.id"
                class="min-h-[36px] inline-flex items-center gap-2 px-3 py-1 rounded-xl border border-line bg-canvas text-xs text-fg-sub cursor-pointer hover:border-info-border has-[:checked]:border-info-border has-[:checked]:bg-info-soft has-[:checked]:text-info transition-colors"
              >
                <input v-model="cfgSkillIds" type="checkbox" :value="sk.id" class="accent-cyan-400" />
                <span>{{ sk.name }}</span>
                <span class="font-mono text-xs opacity-70">{{ sk.kind === 'builtin_mcp' || sk.kind === 'mcp' ? 'MCP' : sk.kind === 'webhook' ? 'API' : 'Doc' }}</span>
              </label>
            </div>
          </div>
        </div>

        <div
          v-if="hasProfileChanges"
          class="flex items-center justify-between gap-2 rounded-xl border border-accent-border bg-accent-soft px-3 py-2 text-xs"
        >
          <span class="text-accent-fg">检测到人设或工具权限已更改，记得点击右侧保存生效</span>
          <span class="font-mono text-xs text-accent-fg">未保存</span>
        </div>

        <div class="flex items-center justify-between gap-3 flex-wrap pt-3 border-t border-line">
          <button
            v-if="bot"
            type="button"
            @click="deleteCustomBot"
            class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-danger-border bg-danger-soft text-danger hover:bg-danger-soft text-xs cursor-pointer"
          >
            {{ isGroupBot ? '解散此群聊' : '删除此 Bot' }}
          </button>
          <span v-else />
          <button
            type="submit"
            :disabled="savingProfile"
            class="min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-xl bg-accent hover:bg-accent-hover text-on-accent font-semibold text-xs cursor-pointer disabled:opacity-50"
          >
            {{ savingProfile ? '保存中...' : '保存配置' }}
          </button>
        </div>
      </form>

      <div class="vesper-card rounded-xl p-5 flex flex-col gap-3">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div>
            <h3 class="font-semibold text-sm text-fg">
              {{ isGroupBot ? '当前群聊专属记忆' : '当前 Bot 记忆域' }} ({{ scopedMemories.length }})
            </h3>
            <p class="text-xs text-fg-muted mt-0.5">
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
              class="px-3 py-1.5 rounded-xl border border-info-border text-xs text-info hover:bg-info-soft cursor-pointer disabled:opacity-50"
            >
              {{ consolidatingMem ? '合并中...' : '合并冗余' }}
            </button>
            <button
              type="button"
              @click="openScopedMemoriesManager"
              class="px-3 py-1.5 rounded-xl border border-line text-xs text-fg-sub hover:text-accent-fg cursor-pointer"
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
            class="sm:col-span-3 min-h-[36px] rounded-xl border border-line bg-canvas px-2.5 font-mono text-xs text-fg"
          />
          <input
            v-model="newMemContent"
            type="text"
            placeholder="写给它记住的专属事实或偏好..."
            required
            class="sm:col-span-6 min-h-[36px] rounded-xl border border-line bg-canvas px-2.5 text-xs text-fg"
          />
          <label class="sm:col-span-1 inline-flex items-center justify-center gap-1 text-xs text-fg-sub cursor-pointer">
            <input v-model="newMemPinned" type="checkbox" class="accent-accent" />
            <span>置顶</span>
          </label>
          <button
            type="submit"
            :disabled="savingMem"
            class="sm:col-span-2 min-h-[36px] rounded-xl bg-accent-soft border border-accent-border text-accent-fg hover:bg-accent-soft text-xs font-medium cursor-pointer disabled:opacity-50"
          >
            {{ savingMem ? '写入中' : '写入专属记忆' }}
          </button>
        </form>

        <div v-if="scopedMemories.length === 0" class="text-xs text-fg-muted py-3">
          暂无关联长期记忆；对话中会自动沉淀偏好与重要背景，也可在上方直接添加。
        </div>
        <div class="flex flex-col gap-2">
          <article
            v-for="mem in scopedMemories"
            :key="mem.id"
            class="rounded-xl border border-line bg-canvas px-3.5 py-2.5"
          >
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <div class="flex items-center gap-1.5 min-w-0">
                <span
                  v-if="mem.pinned"
                  class="px-1.5 py-0.5 rounded border border-warn-border bg-warn-soft font-mono text-xs text-warn"
                >
                  置顶
                </span>
                <span class="px-1.5 py-0.5 rounded border border-line font-mono text-xs text-fg-muted">
                  W{{ mem.importance || 3 }}
                </span>
                <strong class="font-mono text-xs text-accent-fg">{{ mem.fact_key }}</strong>
                <span class="font-mono text-xs text-fg-muted">
                  · {{ mem.scope?.startsWith('bot:') ? '本域私有' : '全局共享' }}
                  <template v-if="mem.recall_count"> · 召回 {{ mem.recall_count }} 次</template>
                </span>
              </div>
              <div class="flex items-center gap-1.5">
                <button
                  type="button"
                  @click="toggleBotMemoryPin(mem)"
                  class="px-2 py-0.5 rounded border border-line text-xs text-fg-sub hover:text-accent-fg cursor-pointer"
                >
                  {{ mem.pinned ? '取消置顶' : '置顶' }}
                </button>
                <button
                  type="button"
                  @click="editingMemId === mem.id ? (editingMemId = null) : (editingMemId = mem.id, editMemContent = mem.content)"
                  class="px-2 py-0.5 rounded border border-line text-xs text-fg-sub hover:text-fg cursor-pointer"
                >
                  {{ editingMemId === mem.id ? '取消' : '编辑' }}
                </button>
                <button
                  type="button"
                  @click="deleteBotMemory(mem.id)"
                  class="px-2 py-0.5 rounded border border-danger-border text-xs text-danger hover:bg-danger-soft cursor-pointer"
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
                class="flex-1 min-h-[32px] rounded-lg border border-accent-border bg-canvas px-2.5 text-xs text-fg"
              />
              <button
                type="button"
                @click="saveInlineMemEdit(mem)"
                class="px-3 py-1 rounded-lg bg-accent text-on-accent font-semibold text-xs cursor-pointer"
              >
                保存
              </button>
            </div>
            <p v-else class="text-xs text-fg mt-1">{{ mem.content }}</p>
          </article>
        </div>
      </div>
    </div>
  </div>
</template>
