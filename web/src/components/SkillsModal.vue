<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { parseJsonRecord, parseJsonStringArray } from '../utils/format'
import { clearSearchOnEscape } from '../utils/topLayer'
import type { Skill, SkillParamField } from '../types/bothub'
import BaseModal from './BaseModal.vue'
import SegmentedTabs from './SegmentedTabs.vue'
import SkillWebhookBuilder, { type HeaderRowItem } from './skills/SkillWebhookBuilder.vue'
import SkillTestBench from './skills/SkillTestBench.vue'

const store = useAppStore()

const KIND_TABS = [
  { id: 'prompt', label: '行为规范 / Playbook' },
  { id: 'webhook', label: 'API / Webhook 工具' },
]

const editingId = ref<string | null>(null)
const name = ref('')
const slug = ref('')
const kind = ref<'prompt' | 'webhook'>('prompt')
const description = ref('')
const instruction = ref('')
const httpMethod = ref('GET')
const httpUrl = ref('')
const headerRows = ref<HeaderRowItem[]>([])
const paramRows = ref<SkillParamField[]>([])
const enabled = ref(true)
const saving = ref(false)

const testSkillTarget = ref<Skill | null>(null)

function applyWebhookTemplate(mode: 'get' | 'post') {
  kind.value = 'webhook'
  if (mode === 'get') {
    httpMethod.value = 'GET'
    httpUrl.value = 'https://api.example.com/v1/lookup?q={{query}}'
    instruction.value = '按关键词查询外部 REST 接口并返回结果'
    paramRows.value = [{ key: 'query', description: '查询关键词', type: 'string', required: true }]
    return
  }
  httpMethod.value = 'POST'
  httpUrl.value = 'https://api.example.com/v1/webhook'
  instruction.value = '{"title": "{{title}}", "content": "{{content}}"}'
  headerRows.value = [{ key: 'Content-Type', value: 'application/json' }]
  paramRows.value = [
    { key: 'title', description: '标题或事件名', type: 'string', required: true },
    { key: 'content', description: '正文负载内容', type: 'string', required: false },
  ]
}

const skills = computed(() => store.state.skills || [])
const skillSearch = ref('')

const filteredSkills = computed(() => {
  const q = skillSearch.value.trim().toLowerCase()
  if (!q) return skills.value
  return skills.value.filter(
    (sk) =>
      sk.name.toLowerCase().includes(q) ||
      sk.slug.toLowerCase().includes(q) ||
      (sk.description || '').toLowerCase().includes(q) ||
      (sk.http_url || '').toLowerCase().includes(q)
  )
})

function mountedBotCount(skillId: string): number {
  return store.state.bots.filter((b) => parseJsonStringArray(b.skill_ids).includes(skillId)).length
}

function resetForm() {
  editingId.value = null
  name.value = ''
  slug.value = ''
  kind.value = 'prompt'
  description.value = ''
  instruction.value = ''
  httpMethod.value = 'GET'
  httpUrl.value = ''
  headerRows.value = []
  paramRows.value = []
  enabled.value = true
}

function loadSkillToEdit(sk: Skill) {
  editingId.value = sk.id
  name.value = sk.name
  slug.value = sk.slug
  kind.value = sk.kind === 'webhook' ? 'webhook' : 'prompt'
  description.value = sk.description || ''
  instruction.value = sk.instruction || ''
  httpMethod.value = sk.http_method || 'GET'
  httpUrl.value = sk.http_url || ''
  enabled.value = sk.enabled !== false

  headerRows.value = Object.entries(parseJsonRecord(sk.http_headers)).map(([k, v]) => ({
    key: k,
    value: v,
  }))

  try {
    const pArr = JSON.parse(sk.params_schema || '[]')
    paramRows.value = Array.isArray(pArr)
      ? pArr.map((f: any) => ({
          key: String(f.key || ''),
          type: String(f.type || 'string'),
          description: String(f.description || ''),
          required: Boolean(f.required),
          default: String(f.default || ''),
        }))
      : []
  } catch {
    paramRows.value = []
  }
}

function cloneSkillToForm(sk: Skill) {
  loadSkillToEdit(sk)
  editingId.value = null
  name.value = `${sk.name} (副本)`
  slug.value = `${sk.slug}_copy`
}

function buildPayload(): Partial<Skill> {
  const headersObj: Record<string, string> = {}
  for (const h of headerRows.value) {
    const k = h.key.trim()
    if (k) headersObj[k] = h.value.trim()
  }
  const cleanParams = paramRows.value
    .map((p) => ({
      key: p.key.trim(),
      type: p.type || 'string',
      description: p.description.trim(),
      required: Boolean(p.required),
      default: (p.default || '').trim(),
    }))
    .filter((p) => p.key !== '')

  return {
    name: name.value.trim(),
    slug: slug.value.trim(),
    kind: kind.value,
    description: description.value.trim(),
    instruction: instruction.value.trim(),
    http_method: httpMethod.value,
    http_url: httpUrl.value.trim(),
    http_headers: JSON.stringify(headersObj),
    params_schema: JSON.stringify(cleanParams),
    enabled: enabled.value,
  }
}

async function handleSave() {
  if (!name.value.trim() || saving.value) return
  if (kind.value === 'webhook' && !httpUrl.value.trim()) {
    store.notify('Webhook 外部工具必须填写请求 URL')
    return
  }
  saving.value = true
  try {
    const payload = buildPayload()
    if (editingId.value) {
      await BotHubApi.updateSkill(editingId.value, payload)
      store.notify(`已更新技能「${payload.name}」`)
    } else {
      await BotHubApi.createSkill(payload)
      store.notify(`已创建技能「${payload.name}」`)
    }
    resetForm()
    await store.refreshWorkspaceSlices(['skills'])
  } catch (err: any) {
    store.notify(err.message || '保存技能失败')
  } finally {
    saving.value = false
  }
}

async function toggleSkillEnabled(sk: Skill) {
  try {
    await BotHubApi.updateSkill(sk.id, { enabled: !sk.enabled })
    await store.refreshWorkspaceSlices(['skills'])
    store.notify(!sk.enabled ? `已启用技能「${sk.name}」` : `已停用技能「${sk.name}」`)
  } catch (err: any) {
    store.notify(err.message || '切换技能状态失败')
  }
}

async function handleDelete(sk: Skill) {
  try {
    await BotHubApi.deleteSkill(sk.id)
    if (editingId.value === sk.id) resetForm()
    if (testSkillTarget.value?.id === sk.id) {
      testSkillTarget.value = null
    }
    await store.refreshWorkspaceSlices(['skills'])
    store.notify(`已删除技能「${sk.name}」`)
  } catch (err: any) {
    store.notify(err.message || '删除技能失败')
  }
}

function closeModal() {
  store.state.showSkillsModal = false
}

watch(
  () => store.state.showSkillsModal,
  (open) => {
    if (!open) {
      testSkillTarget.value = null
    }
  }
)
</script>

<template>
  <BaseModal
    :open="store.state.showSkillsModal"
    title="自定义技能与工具工坊 (Skills)"
    subtitle="创建行为规范 Playbook 或实时 REST API 工具，并按需挂载给任意 Bot"
    max-width="3xl"
    @close="closeModal"
  >
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-4 min-h-0 flex-1 overflow-y-auto pr-0.5">
      <!-- Left: Create / Edit Form -->
      <form
        @submit.prevent="handleSave"
        class="lg:col-span-6 rounded-xl border border-line bg-canvas p-4 flex flex-col gap-3 h-fit"
      >
        <div class="flex items-center justify-between gap-2">
          <h4 class="text-xs font-semibold text-fg">
            {{ editingId ? '编辑自定义技能' : '新建自定义技能' }}
          </h4>
          <button
            v-if="editingId"
            type="button"
            @click="resetForm"
            class="text-xs text-accent-fg hover:underline cursor-pointer"
          >
            取消编辑
          </button>
        </div>

        <SegmentedTabs v-model="kind" :items="KIND_TABS" aria-label="技能类型" />

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
          <label class="flex flex-col gap-1 text-xs text-fg-sub">
            <span>技能名称</span>
            <input
              v-model="name"
              type="text"
              placeholder="如 天气查询 / 晨间复盘流程"
              required
              class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 text-xs text-fg focus:border-accent"
            />
          </label>
          <label class="flex flex-col gap-1 text-xs text-fg-sub">
            <span>函数标识符 Slug (可选)</span>
            <input
              v-model="slug"
              type="text"
              placeholder="自动生成如 check_weather"
              class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 font-mono text-xs text-fg focus:border-accent"
            />
          </label>
        </div>

        <label class="flex flex-col gap-1 text-xs text-fg-sub">
          <span>功能简述 (Bot 靠它判断何时触发)</span>
          <input
            v-model="description"
            type="text"
            placeholder="简洁说明这个技能用来干什么、什么时候该调用"
            class="min-h-[38px] rounded-xl border border-line bg-canvas px-3 py-1.5 text-xs text-fg focus:border-accent"
          />
        </label>

        <!-- Prompt Playbook Content -->
        <label v-if="kind === 'prompt'" class="flex flex-col gap-1 text-xs text-fg-sub">
          <span>指令正文 / 行动准则</span>
          <textarea
            v-model="instruction"
            rows="4"
            placeholder="写入具体的工作流步骤、说话风格、输出要求或判断原则；挂载后将注入该 Bot 的能力上下文。"
            class="auto-size-textarea rounded-xl border border-line bg-canvas px-3 py-2 text-xs text-fg focus:border-accent leading-relaxed"
          />
        </label>

        <!-- Webhook / API Tool Builder -->
        <SkillWebhookBuilder
          v-else
          v-model:http-method="httpMethod"
          v-model:http-url="httpUrl"
          v-model:instruction="instruction"
          v-model:param-rows="paramRows"
          v-model:header-rows="headerRows"
          @apply-template="applyWebhookTemplate"
        />

        <div class="flex items-center justify-between pt-1">
          <label class="inline-flex items-center gap-2 text-xs text-fg-sub cursor-pointer">
            <input v-model="enabled" type="checkbox" class="accent-accent" />
            <span>启用此技能</span>
          </label>
          <button
            type="submit"
            :disabled="saving"
            class="min-h-[38px] px-4 py-1.5 rounded-xl bg-accent hover:bg-accent-hover text-on-accent font-semibold text-xs cursor-pointer disabled:opacity-50"
          >
            {{ saving ? '保存中...' : editingId ? '保存修改' : '创建技能' }}
          </button>
        </div>
      </form>

      <!-- Right: Skill Library & Live Test -->
      <div class="lg:col-span-6 flex flex-col gap-3">
        <div class="rounded-xl border border-line bg-canvas p-4 flex flex-col gap-2.5">
          <div class="flex items-center justify-between gap-2 flex-wrap">
            <h4 class="text-xs font-semibold text-fg">已创建的技能 ({{ filteredSkills.length }}<span v-if="skillSearch.trim()"> / {{ skills.length }}</span>)</h4>
            <span class="text-xs text-fg-muted">Webhook 技能自动编译为 skill_ 工具</span>
          </div>

          <input
            v-if="skills.length > 0"
            v-model="skillSearch"
            type="search"
            aria-label="筛选技能"
            placeholder="搜索技能名称、slug 或描述..."
            @keydown.escape="clearSearchOnEscape($event, skillSearch, () => { skillSearch = '' })"
            class="w-full min-h-[34px] rounded-lg border border-line bg-canvas px-2.5 text-xs text-fg placeholder:text-fg-muted focus:border-accent focus:outline-none"
          />

          <div v-if="skills.length === 0" class="py-8 text-center text-xs text-fg-muted leading-relaxed">
            当前没有任何技能。你可以在左侧创建一个习惯指令 Playbook，或接入外部 REST API Webhook 成为 Bot 的专属实时工具。
          </div>
          <div v-else-if="filteredSkills.length === 0" class="py-6 text-center text-xs text-fg-muted">
            没有匹配「{{ skillSearch }}」的技能
          </div>

          <article
            v-for="sk in filteredSkills"
            :key="sk.id"
            class="rounded-xl border border-line bg-canvas p-3 flex flex-col gap-2"
          >
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <div class="flex items-center gap-2 flex-wrap">
                  <strong class="text-xs font-semibold text-fg">{{ sk.name }}</strong>
                  <span
                    :class="[
                      'px-1.5 py-0.5 rounded font-mono text-xs border',
                      sk.kind === 'webhook'
                        ? 'border-info-border bg-info-soft text-info'
                        : 'border-accent-border bg-accent-soft text-accent-fg',
                    ]"
                  >
                    {{ sk.kind === 'webhook' ? `${sk.http_method || 'GET'} skill_${sk.slug}` : 'Playbook' }}
                  </span>
                  <span
                    v-if="mountedBotCount(sk.id) > 0"
                    class="px-1.5 py-0.5 rounded border border-ok-border bg-ok-soft text-xs text-ok"
                  >
                    {{ mountedBotCount(sk.id) }} 个 Bot 已挂载
                  </span>
                  <span v-if="!sk.enabled" class="text-xs text-fg-muted">已停用</span>
                </div>
                <p v-if="sk.description" class="text-xs text-fg-muted mt-1 leading-relaxed">
                  {{ sk.description }}
                </p>
                <div v-if="sk.kind === 'webhook'" class="text-xs font-mono text-fg-muted mt-1 truncate">
                  {{ sk.http_url }}
                </div>
                <p v-else-if="sk.instruction" class="text-xs text-fg-muted mt-1 line-clamp-2">
                  {{ sk.instruction }}
                </p>
              </div>
            </div>

            <div class="flex items-center justify-end gap-1.5 flex-wrap pt-1 border-t border-line">
              <button
                v-if="sk.kind === 'webhook'"
                type="button"
                @click="testSkillTarget = sk"
                class="px-2.5 py-1 rounded-lg border border-info-border text-xs text-info hover:bg-info-soft cursor-pointer"
              >
                实时调试
              </button>
              <button
                type="button"
                @click="toggleSkillEnabled(sk)"
                class="px-2.5 py-1 rounded-lg border border-line text-xs text-fg-sub hover:text-fg cursor-pointer"
              >
                {{ sk.enabled ? '停用' : '启用' }}
              </button>
              <button
                type="button"
                @click="cloneSkillToForm(sk)"
                class="px-2.5 py-1 rounded-lg border border-line text-xs text-fg-sub hover:text-info cursor-pointer"
              >
                复用新建
              </button>
              <button
                type="button"
                @click="loadSkillToEdit(sk)"
                class="px-2.5 py-1 rounded-lg border border-line text-xs text-fg-sub hover:text-accent-fg cursor-pointer"
              >
                编辑
              </button>
              <button
                type="button"
                @click="handleDelete(sk)"
                class="px-2.5 py-1 rounded-lg border border-danger-border text-xs text-danger hover:bg-danger-soft cursor-pointer"
              >
                删除
              </button>
            </div>
          </article>
        </div>

        <!-- Webhook Live Test Box -->
        <SkillTestBench
          v-if="testSkillTarget"
          :skill="testSkillTarget"
          @close="testSkillTarget = null"
        />
      </div>
    </div>
  </BaseModal>
</template>
