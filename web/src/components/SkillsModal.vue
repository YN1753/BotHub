<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import type { Skill, SkillParamField } from '../types/bothub'
import BaseModal from './BaseModal.vue'
import CustomSelect from './CustomSelect.vue'
import SegmentedTabs from './SegmentedTabs.vue'

const store = useAppStore()

const KIND_TABS = [
  { id: 'prompt', label: '行为规范 / Playbook' },
  { id: 'webhook', label: 'API / Webhook 工具' },
]

const METHOD_OPTIONS = [
  { value: 'GET', label: 'GET' },
  { value: 'POST', label: 'POST' },
  { value: 'PUT', label: 'PUT' },
  { value: 'PATCH', label: 'PATCH' },
  { value: 'DELETE', label: 'DELETE' },
]

const PARAM_TYPE_OPTIONS = [
  { value: 'string', label: '文本 (string)' },
  { value: 'integer', label: '整数 (integer)' },
  { value: 'boolean', label: '布尔 (boolean)' },
]

const editingId = ref<string | null>(null)
const name = ref('')
const slug = ref('')
const kind = ref<'prompt' | 'webhook'>('prompt')
const description = ref('')
const instruction = ref('')
const httpMethod = ref('GET')
const httpUrl = ref('')
const headerRows = ref<Array<{ key: string; value: string }>>([])
const paramRows = ref<SkillParamField[]>([])
const enabled = ref(true)
const saving = ref(false)

// Live test state for webhook skills
const testSkillTarget = ref<Skill | null>(null)
const testArgs = ref<Record<string, string>>({})
const testing = ref(false)
const testResult = ref<{ statusCode?: number; latencyMs?: number; output: string; error?: string } | null>(null)

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
  return store.state.bots.filter((b) => Array.isArray(b.skill_ids) && b.skill_ids.includes(skillId)).length
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

  try {
    const hObj = JSON.parse(sk.http_headers || '{}')
    headerRows.value =
      hObj && typeof hObj === 'object'
        ? Object.entries(hObj).map(([k, v]) => ({ key: k, value: String(v ?? '') }))
        : []
  } catch {
    headerRows.value = []
  }

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

function addHeaderRow() {
  headerRows.value.push({ key: '', value: '' })
}

function removeHeaderRow(idx: number) {
  headerRows.value.splice(idx, 1)
}

function addParamRow() {
  paramRows.value.push({ key: '', type: 'string', description: '', required: false, default: '' })
}

function removeParamRow(idx: number) {
  paramRows.value.splice(idx, 1)
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
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message || '保存技能失败')
  } finally {
    saving.value = false
  }
}

async function toggleSkillEnabled(sk: Skill) {
  try {
    await BotHubApi.updateSkill(sk.id, { enabled: !sk.enabled })
    await store.loadDashboard(false)
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
      testResult.value = null
    }
    await store.loadDashboard(false)
    store.notify(`已删除技能「${sk.name}」`)
  } catch (err: any) {
    store.notify(err.message || '删除技能失败')
  }
}

function openTestPanel(sk: Skill) {
  testSkillTarget.value = sk
  testResult.value = null
  const initArgs: Record<string, string> = {}
  try {
    const arr = JSON.parse(sk.params_schema || '[]')
    if (Array.isArray(arr)) {
      arr.forEach((f: any) => {
        if (f?.key) initArgs[String(f.key)] = String(f.default || '')
      })
    }
  } catch {
    // ignore
  }
  testArgs.value = initArgs
}

function formatTestOutput(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

async function copyTestOutput(raw: string) {
  try {
    await navigator.clipboard.writeText(formatTestOutput(raw))
    store.notify('已复制响应内容')
  } catch {
    store.notify('复制失败')
  }
}

async function runLiveTest() {
  if (!testSkillTarget.value || testing.value) return
  testing.value = true
  testResult.value = null
  try {
    const typedArgs: Record<string, any> = {}
    let schemaList: SkillParamField[] = []
    try {
      schemaList = JSON.parse(testSkillTarget.value.params_schema || '[]')
    } catch {
      schemaList = []
    }
    const typeMap = new Map(schemaList.map((f) => [f.key, f.type]))
    for (const [k, rawVal] of Object.entries(testArgs.value)) {
      const ftype = typeMap.get(k) || 'string'
      if (ftype === 'integer') {
        typedArgs[k] = Number.parseInt(rawVal || '0', 10) || 0
      } else if (ftype === 'boolean') {
        typedArgs[k] = rawVal.trim().toLowerCase() === 'true' || rawVal.trim() === '1'
      } else {
        typedArgs[k] = rawVal
      }
    }
    const res = await BotHubApi.testSkill(testSkillTarget.value.id, typedArgs)
    testResult.value = {
      statusCode: res.status_code,
      latencyMs: res.latency_ms,
      output: typeof res.result === 'string' ? res.result : JSON.stringify(res.result, null, 2),
    }
  } catch (err: any) {
    testResult.value = {
      output: '',
      error: err.message || '调用测试失败',
    }
  } finally {
    testing.value = false
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
      testResult.value = null
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
        class="lg:col-span-6 rounded-xl border border-[#fff8ee]/10 bg-[#060914] p-4 flex flex-col gap-3 h-fit"
      >
        <div class="flex items-center justify-between gap-2">
          <h4 class="text-xs font-semibold text-white">
            {{ editingId ? '编辑自定义技能' : '新建自定义技能' }}
          </h4>
          <button
            v-if="editingId"
            type="button"
            @click="resetForm"
            class="text-[11px] text-[#f2b880] hover:underline cursor-pointer"
          >
            取消编辑
          </button>
        </div>

        <SegmentedTabs v-model="kind" :items="KIND_TABS" aria-label="技能类型" />

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>技能名称</span>
            <input
              v-model="name"
              type="text"
              placeholder="如 天气查询 / 晨间复盘流程"
              required
              class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-1.5 text-xs text-white focus:border-[#f2b880]/60"
            />
          </label>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>函数标识符 Slug (可选)</span>
            <input
              v-model="slug"
              type="text"
              placeholder="自动生成如 check_weather"
              class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-1.5 font-mono text-xs text-white focus:border-[#f2b880]/60"
            />
          </label>
        </div>

        <label class="flex flex-col gap-1 text-xs text-slate-300">
          <span>功能简述 (Bot 靠它判断何时触发)</span>
          <input
            v-model="description"
            type="text"
            placeholder="简洁说明这个技能用来干什么、什么时候该调用"
            class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-1.5 text-xs text-white focus:border-[#f2b880]/60"
          />
        </label>

        <!-- Prompt Playbook Content -->
        <label v-if="kind === 'prompt'" class="flex flex-col gap-1 text-xs text-slate-300">
          <span>指令正文 / 行动准则</span>
          <textarea
            v-model="instruction"
            rows="4"
            placeholder="写入具体的工作流步骤、说话风格、输出要求或判断原则；挂载后将注入该 Bot 的能力上下文。"
            class="rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-xs text-white focus:border-[#f2b880]/60 leading-relaxed"
          />
        </label>

        <!-- Webhook / API Tool Builder -->
        <template v-else>
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
            <div class="flex flex-col gap-1 text-xs text-slate-300">
              <span>请求方法</span>
              <CustomSelect v-model="httpMethod" :options="METHOD_OPTIONS" aria-label="请求方法" />
            </div>
            <label class="sm:col-span-2 flex flex-col gap-1 text-xs text-slate-300">
              <span>Endpoint URL (支持 <code class="text-[#f2b880]">&#123;&#123;key&#125;&#125;</code> 参数插值)</span>
              <input
                v-model="httpUrl"
                type="url"
                placeholder="https://api.example.com/v1/item?q={{query}}"
                required
                class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-1.5 font-mono text-xs text-white focus:border-[#f2b880]/60"
              />
              <div v-if="paramRows.some((p) => p.key.trim())" class="flex items-center gap-1 flex-wrap text-[10.5px]">
                <span class="text-slate-500">插入变量:</span>
                <button
                  v-for="pk in paramRows.map((p) => p.key.trim()).filter(Boolean)"
                  :key="pk"
                  type="button"
                  @click="httpUrl = `${httpUrl}{{${pk}}}`"
                  class="px-1.5 py-0.5 rounded border border-[#fff8ee]/12 bg-[#050811] font-mono text-[#f2b880] hover:border-[#f2b880]/45 cursor-pointer"
                >
                  &#123;&#123;{{ pk }}&#125;&#125;
                </button>
              </div>
            </label>
          </div>

          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span v-if="httpMethod === 'GET'">补充调用说明 (可选)</span>
            <span v-else>Body 请求模板 (支持 <code class="text-[#f2b880]">&#123;&#123;key&#125;&#125;</code> 插值或留空自动传 JSON)</span>
            <textarea
              v-model="instruction"
              rows="3"
              :placeholder="
                httpMethod === 'GET'
                  ? '给模型的参数提示，例如：city 建议传英文城市名'
                  : '{&quot;target&quot;: &quot;{{target}}&quot;, &quot;note&quot;: &quot;{{note}}&quot;}'
              "
              class="rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-[11.5px] text-white focus:border-[#f2b880]/60"
            />
          </label>

          <!-- Dynamic Parameter Schema -->
          <div class="rounded-xl border border-[#fff8ee]/10 bg-[#050811]/70 p-2.5 flex flex-col gap-2">
            <div class="flex items-center justify-between">
              <span class="text-[11px] font-medium text-slate-300">输入参数定义 (编译为 Function Calling Schema)</span>
              <button
                type="button"
                @click="addParamRow"
                class="text-[11px] text-[#f2b880] hover:underline cursor-pointer"
              >
                + 添加参数
              </button>
            </div>
            <div v-if="paramRows.length === 0" class="text-[11px] text-slate-500">
              无入参；若 URL/Body 中包含插值变量，请在这里声明参数名。
            </div>
            <div
              v-for="(p, idx) in paramRows"
              :key="idx"
              class="grid grid-cols-1 sm:grid-cols-12 gap-1.5 items-center"
            >
              <input
                v-model="p.key"
                type="text"
                placeholder="参数名 key"
                class="sm:col-span-3 min-h-[32px] rounded-lg border border-[#fff8ee]/10 bg-[#070a13] px-2 font-mono text-[11px] text-white"
              />
              <div class="sm:col-span-2">
                <CustomSelect v-model="p.type" :options="PARAM_TYPE_OPTIONS" aria-label="参数类型" />
              </div>
              <input
                v-model="p.description"
                type="text"
                placeholder="参数说明"
                class="sm:col-span-3 min-h-[32px] rounded-lg border border-[#fff8ee]/10 bg-[#070a13] px-2 text-[11px] text-white"
              />
              <input
                v-model="p.default"
                type="text"
                placeholder="默认值(可选)"
                class="sm:col-span-2 min-h-[32px] rounded-lg border border-[#fff8ee]/10 bg-[#070a13] px-2 font-mono text-[11px] text-white"
              />
              <div class="sm:col-span-2 flex items-center justify-between gap-1">
                <label class="inline-flex items-center gap-1 text-[10.5px] text-slate-400 cursor-pointer">
                  <input v-model="p.required" type="checkbox" class="accent-[#f2b880]" />
                  <span>必填</span>
                </label>
                <button
                  type="button"
                  @click="removeParamRow(idx)"
                  class="text-[11px] text-rose-400 hover:text-rose-300 cursor-pointer"
                >
                  删
                </button>
              </div>
            </div>
            <div v-if="paramRows.length > 0" class="hidden sm:grid sm:grid-cols-12 gap-1.5 text-[10px] text-slate-500 px-0.5">
              <span class="col-span-3">参数键名</span>
              <span class="col-span-2">类型</span>
              <span class="col-span-3">语义描述</span>
              <span class="col-span-2">默认值</span>
              <span class="col-span-2">操作</span>
            </div>
          </div>

          <!-- Custom Headers -->
          <div class="rounded-xl border border-[#fff8ee]/10 bg-[#050811]/70 p-2.5 flex flex-col gap-2">
            <div class="flex items-center justify-between">
              <span class="text-[11px] font-medium text-slate-300">请求头 Headers (可选)</span>
              <button
                type="button"
                @click="addHeaderRow"
                class="text-[11px] text-[#f2b880] hover:underline cursor-pointer"
              >
                + 添加请求头
              </button>
            </div>
            <div
              v-for="(h, idx) in headerRows"
              :key="idx"
              class="grid grid-cols-12 gap-1.5 items-center"
            >
              <input
                v-model="h.key"
                type="text"
                placeholder="Header 名 (如 Authorization)"
                class="col-span-5 min-h-[32px] rounded-lg border border-[#fff8ee]/10 bg-[#070a13] px-2 font-mono text-[11px] text-white"
              />
              <input
                v-model="h.value"
                type="text"
                placeholder="Header 值 (如 Bearer xxx)"
                class="col-span-6 min-h-[32px] rounded-lg border border-[#fff8ee]/10 bg-[#070a13] px-2 font-mono text-[11px] text-white"
              />
              <button
                type="button"
                @click="removeHeaderRow(idx)"
                class="col-span-1 text-[11px] text-rose-400 hover:text-rose-300 cursor-pointer text-right"
              >
                删
              </button>
            </div>
          </div>
        </template>

        <div class="flex items-center justify-between pt-1">
          <label class="inline-flex items-center gap-2 text-xs text-slate-300 cursor-pointer">
            <input v-model="enabled" type="checkbox" class="accent-[#f2b880]" />
            <span>启用此技能</span>
          </label>
          <button
            type="submit"
            :disabled="saving"
            class="min-h-[38px] px-4 py-1.5 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50"
          >
            {{ saving ? '保存中...' : editingId ? '保存修改' : '创建技能' }}
          </button>
        </div>
      </form>

      <!-- Right: Skill Library & Live Test -->
      <div class="lg:col-span-6 flex flex-col gap-3">
        <div class="rounded-xl border border-[#fff8ee]/10 bg-[#060914] p-4 flex flex-col gap-2.5">
          <div class="flex items-center justify-between gap-2 flex-wrap">
            <h4 class="text-xs font-semibold text-white">已创建的技能 ({{ filteredSkills.length }}<span v-if="skillSearch.trim()"> / {{ skills.length }}</span>)</h4>
            <span class="text-[11px] text-slate-500">Webhook 技能自动编译为 skill_ 工具</span>
          </div>

          <input
            v-if="skills.length > 0"
            v-model="skillSearch"
            type="search"
            aria-label="筛选技能"
            placeholder="搜索技能名称、slug 或描述..."
            @keydown.escape.stop="skillSearch = ''"
            class="w-full min-h-[34px] rounded-lg border border-[#fff8ee]/10 bg-[#050811] px-2.5 text-[11px] text-white placeholder-slate-500 focus:border-[#f2b880]/55 focus:outline-none"
          />

          <div v-if="skills.length === 0" class="py-8 text-center text-xs text-slate-500 leading-relaxed">
            当前没有任何技能。你可以在左侧创建一个习惯指令 Playbook，或接入外部 REST API Webhook 成为 Bot 的专属实时工具。
          </div>
          <div v-else-if="filteredSkills.length === 0" class="py-6 text-center text-xs text-slate-500">
            没有匹配「{{ skillSearch }}」的技能
          </div>

          <article
            v-for="sk in filteredSkills"
            :key="sk.id"
            class="rounded-xl border border-[#fff8ee]/10 bg-[#050811] p-3 flex flex-col gap-2"
          >
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <div class="flex items-center gap-2 flex-wrap">
                  <strong class="text-xs font-semibold text-white">{{ sk.name }}</strong>
                  <span
                    :class="[
                      'px-1.5 py-0.5 rounded font-mono text-[10px] border',
                      sk.kind === 'webhook'
                        ? 'border-cyan-400/35 bg-cyan-500/10 text-cyan-300'
                        : 'border-[#f2b880]/35 bg-[#f2b880]/10 text-[#f2b880]',
                    ]"
                  >
                    {{ sk.kind === 'webhook' ? `${sk.http_method || 'GET'} skill_${sk.slug}` : 'Playbook' }}
                  </span>
                  <span
                    v-if="mountedBotCount(sk.id) > 0"
                    class="px-1.5 py-0.5 rounded border border-emerald-400/25 bg-emerald-500/10 text-[10px] text-emerald-300"
                  >
                    {{ mountedBotCount(sk.id) }} 个 Bot 已挂载
                  </span>
                  <span v-if="!sk.enabled" class="text-[10px] text-slate-500">已停用</span>
                </div>
                <p v-if="sk.description" class="text-[11px] text-slate-400 mt-1 leading-relaxed">
                  {{ sk.description }}
                </p>
                <div v-if="sk.kind === 'webhook'" class="text-[10.5px] font-mono text-slate-500 mt-1 truncate">
                  {{ sk.http_url }}
                </div>
                <p v-else-if="sk.instruction" class="text-[11px] text-slate-500 mt-1 line-clamp-2">
                  {{ sk.instruction }}
                </p>
              </div>
            </div>

            <div class="flex items-center justify-end gap-1.5 flex-wrap pt-1 border-t border-[#fff8ee]/6">
              <button
                v-if="sk.kind === 'webhook'"
                type="button"
                @click="openTestPanel(sk)"
                class="px-2.5 py-1 rounded-lg border border-cyan-400/35 text-[11px] text-cyan-300 hover:bg-cyan-500/15 cursor-pointer"
              >
                实时调试
              </button>
              <button
                type="button"
                @click="toggleSkillEnabled(sk)"
                class="px-2.5 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-white cursor-pointer"
              >
                {{ sk.enabled ? '停用' : '启用' }}
              </button>
              <button
                type="button"
                @click="loadSkillToEdit(sk)"
                class="px-2.5 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
              >
                编辑
              </button>
              <button
                type="button"
                @click="handleDelete(sk)"
                class="px-2.5 py-1 rounded-lg border border-rose-400/30 text-[11px] text-rose-300 hover:bg-rose-500/15 cursor-pointer"
              >
                删除
              </button>
            </div>
          </article>
        </div>

        <!-- Webhook Live Test Box -->
        <div
          v-if="testSkillTarget"
          class="rounded-xl border border-cyan-400/30 bg-[#060914] p-4 flex flex-col gap-2.5"
        >
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-semibold text-cyan-200">
              实时调试工具: skill_{{ testSkillTarget.slug }}
            </h4>
            <button
              type="button"
              @click="testSkillTarget = null"
              class="text-[11px] text-slate-400 hover:text-white cursor-pointer"
            >
              收起
            </button>
          </div>
          <div v-if="Object.keys(testArgs).length > 0" class="flex flex-col gap-2">
            <label
              v-for="k in Object.keys(testArgs)"
              :key="k"
              class="flex flex-col gap-1 text-[11px] text-slate-300"
            >
              <span class="font-mono text-cyan-300">{{ k }}</span>
              <input
                v-model="testArgs[k]"
                type="text"
                placeholder="输入测试参数值"
                class="min-h-[34px] rounded-lg border border-[#fff8ee]/12 bg-[#050811] px-2.5 text-xs text-white"
              />
            </label>
          </div>
          <button
            type="button"
            :disabled="testing"
            @click="runLiveTest"
            class="min-h-[36px] px-3 py-1.5 rounded-xl bg-cyan-500/20 border border-cyan-400/40 text-xs font-medium text-cyan-200 hover:bg-cyan-500/30 cursor-pointer disabled:opacity-50"
          >
            {{ testing ? '正在请求远端接口...' : '发送测试请求' }}
          </button>
          <div v-if="testResult" class="rounded-lg bg-[#050811] border border-[#fff8ee]/10 p-2.5 text-[11px] font-mono">
            <div v-if="testResult.error" class="text-rose-300">{{ testResult.error }}</div>
            <template v-else>
              <div class="flex items-center justify-between gap-2 mb-1">
                <span class="text-emerald-300">
                  HTTP {{ testResult.statusCode || 200 }} · {{ testResult.latencyMs }}ms
                </span>
                <button
                  type="button"
                  @click="copyTestOutput(testResult.output)"
                  class="px-2 py-0.5 rounded border border-[#fff8ee]/12 text-[10px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
                >
                  复制响应
                </button>
              </div>
              <pre class="whitespace-pre-wrap break-all text-slate-300 max-h-48 overflow-y-auto">{{ formatTestOutput(testResult.output) }}</pre>
            </template>
          </div>
        </div>
      </div>
    </div>
  </BaseModal>
</template>
