<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import type { ModelHealthStats, ModelPoolEntry, UpstreamModelCheck } from '../../types/bothub'
import SecretInput from '../ui/SecretInput.vue'

interface FetchedModelMetaItem {
  usable: UpstreamModelCheck[]
  failed: UpstreamModelCheck[]
  filteredOutCount: number
  unverifiedCount: number
  showFailed: boolean
  query: string
}

const props = defineProps<{
  tier: 'free' | 'paid'
  verifyOnFetch: boolean
  poolEntries: ModelPoolEntry[]
  probingId: string
  probeResults: Record<string, { ok: boolean; msg: string }>
  fetchingModelsId: string
  fetchedModelMeta: Record<string, FetchedModelMetaItem>
  healthForTarget: (key: string) => ModelHealthStats | undefined
  healthBadgeText: (key: string) => string
  filteredUsableChecks: (targetId: string) => UpstreamModelCheck[]
  probeTarget: (targetId: string, provider: string, baseUrl: string, apiKey: string, modelId: string) => Promise<void>
  fetchModelsForTarget: (
    targetId: string,
    provider: string,
    baseUrl: string,
    apiKey: string,
    entry?: ModelPoolEntry
  ) => Promise<void>
}>()

const emit = defineEmits<{
  'update:poolEntries': [value: ModelPoolEntry[]]
}>()

const store = useAppStore()
const expandedPoolId = ref(props.poolEntries[0]?.id || '')
const probingAll = ref(false)
const customModelInput = ref<Record<string, string>>({})

const isPaidTier = computed(() => props.tier === 'paid')

function selectedModels(entry: ModelPoolEntry): string[] {
  const list: string[] = []
  const seen = new Set<string>()
  for (const m of entry.models || []) {
    const clean = (m || '').trim()
    if (clean && !seen.has(clean)) {
      seen.add(clean)
      list.push(clean)
    }
  }
  const primary = (entry.model || '').trim()
  if (primary && !seen.has(primary)) {
    list.unshift(primary)
  }
  return list
}

function setEntryModels(entry: ModelPoolEntry, nextModels: string[]) {
  const cleaned = Array.from(new Set(nextModels.map((m) => m.trim()).filter(Boolean)))
  entry.models = cleaned
  entry.model = cleaned[0] || ''
}

function toggleEntryModel(entry: ModelPoolEntry, modelId: string) {
  const clean = modelId.trim()
  if (!clean) return
  const cur = selectedModels(entry)
  if (cur.includes(clean)) {
    setEntryModels(
      entry,
      cur.filter((x) => x !== clean)
    )
  } else {
    setEntryModels(entry, [...cur, clean])
  }
}

function addManualModel(entry: ModelPoolEntry) {
  const val = (customModelInput.value[entry.id] || '').trim()
  if (!val) return
  const parts = val
    .split(/[,，\n]/)
    .map((s) => s.trim())
    .filter(Boolean)
  setEntryModels(entry, [...selectedModels(entry), ...parts])
  customModelInput.value[entry.id] = ''
}

function selectAllFetchedUsable(entry: ModelPoolEntry) {
  const candidates = props
    .filteredUsableChecks(entry.id)
    .slice(0, 12)
    .map((c) => c.id)
  if (candidates.length === 0) return
  setEntryModels(entry, [...selectedModels(entry), ...candidates])
}

const poolSummaryChip = computed(() => {
  const total = props.poolEntries.length
  const enabled = props.poolEntries.filter((x) => x.enabled).length
  const readyModels = props.poolEntries
    .filter((x) => x.enabled && x.api_key && selectedModels(x).length > 0)
    .reduce((acc, x) => acc + selectedModels(x).length, 0)
  return `已启用 ${enabled}/${total} 节点 · ${readyModels} 模型就绪`
})

function addEmptyPoolNode() {
  const suffix = Date.now().toString(36).slice(-4)
  const id = `${props.tier}_${suffix}`
  const labelPrefix = isPaidTier.value ? '付费节点' : '免费节点'
  const next: ModelPoolEntry[] = [
    ...props.poolEntries,
    {
      id,
      name: `${labelPrefix} #${props.poolEntries.length + 1}`,
      tier: props.tier,
      provider: isPaidTier.value ? 'deepseek' : 'openai',
      base_url: '',
      api_key: '',
      model: '',
      models: [],
      enabled: true,
      priority: (props.poolEntries.length + 1) * 10,
      note: '',
    },
  ]
  emit('update:poolEntries', next)
  expandedPoolId.value = id
}

function movePoolNode(index: number, delta: number) {
  const nextIdx = index + delta
  if (nextIdx < 0 || nextIdx >= props.poolEntries.length) return
  const arr = [...props.poolEntries]
  const [item] = arr.splice(index, 1)
  arr.splice(nextIdx, 0, item)
  emit('update:poolEntries', arr)
}

function removePoolNode(id: string) {
  emit(
    'update:poolEntries',
    props.poolEntries.filter((x) => x.id !== id)
  )
}

function duplicatePoolNode(entry: ModelPoolEntry) {
  const suffix = Date.now().toString(36).slice(-4)
  const copy: ModelPoolEntry = {
    ...entry,
    id: `${props.tier}_${suffix}`,
    tier: props.tier,
    models: [...selectedModels(entry)],
    name: `${entry.name} (副本)`,
    priority: (props.poolEntries.length + 1) * 10,
  }
  emit('update:poolEntries', [...props.poolEntries, copy])
  expandedPoolId.value = copy.id
}

async function probeAllEnabledPool() {
  if (probingAll.value || props.probingId) return
  const active = props.poolEntries.filter((e) => e.enabled && e.api_key.trim() !== '')
  if (active.length === 0) {
    store.notify(`请至少为一个启用中的${isPaidTier.value ? '付费' : '免费'}节点填入 API Key`)
    return
  }
  probingAll.value = true
  let okCount = 0
  for (const entry of active) {
    const firstModel = selectedModels(entry)[0] || entry.model
    await props.probeTarget(entry.id, entry.provider, entry.base_url, entry.api_key, firstModel)
    if (props.probeResults[entry.id]?.ok) okCount++
  }
  probingAll.value = false
  store.notify(`${isPaidTier.value ? '付费池' : '免费池'}测速完成：${okCount}/${active.length} 节点连通`)
}
</script>

<template>
  <div class="modal-section flex flex-col gap-3">
    <div class="flex items-center justify-between gap-2 flex-wrap">
      <div>
        <div class="flex items-center gap-2 flex-wrap">
          <span :class="['text-xs font-semibold', isPaidTier ? 'text-[#f2b880]' : 'text-emerald-300']">
            {{ isPaidTier ? '付费模型池 (Paid Pool)' : '免费模型池 (Free Pool)' }}
          </span>
          <span
            v-if="poolEntries.length > 0"
            :class="[
              'font-mono text-[10px] px-1.5 py-0.5 rounded border',
              isPaidTier
                ? 'border-[#f2b880]/30 bg-[#f2b880]/10 text-[#f2b880]'
                : 'border-emerald-400/25 bg-emerald-400/10 text-emerald-200'
            ]"
          >
            {{ poolSummaryChip }}
          </span>
        </div>
        <p class="text-[11px] text-slate-400 mt-0.5">
          {{
            isPaidTier
              ? '同一个 Base URL + API Key 可同时勾选多个付费/主力模型；仅当会话或任务明确选择，或单任务开启「允许付费兜底」时调用'
              : '同一个 Base URL + API Key 可勾选多个免费额度模型；默认所有对话与后台例程优先且仅在本池内容灾轮询'
          }}
        </p>
      </div>
      <div class="flex items-center gap-1.5 shrink-0 flex-wrap">
        <button
          v-if="poolEntries.length > 0"
          type="button"
          :disabled="probingAll"
          @click="probeAllEnabledPool"
          class="min-h-[30px] px-2.5 rounded-lg border border-[#f2b880]/35 bg-[#f2b880]/10 hover:bg-[#f2b880]/20 text-[11px] text-[#f2b880] cursor-pointer disabled:opacity-45"
        >
          {{ probingAll ? '测速中...' : '一键测速' }}
        </button>
        <button
          type="button"
          @click="addEmptyPoolNode"
          :class="[
            'min-h-[30px] px-2.5 rounded-lg border text-[11px] cursor-pointer shrink-0',
            isPaidTier
              ? 'border-[#f2b880]/45 bg-[#f2b880]/12 hover:bg-[#f2b880]/22 text-[#f2b880]'
              : 'border-emerald-400/35 bg-emerald-400/10 hover:bg-emerald-400/18 text-emerald-300'
          ]"
        >
          {{ isPaidTier ? '+ 添加付费节点' : '+ 添加免费节点' }}
        </button>
      </div>
    </div>

    <div
      v-if="poolEntries.length === 0"
      class="rounded-xl border border-dashed border-[#fff8ee]/12 bg-[#050811]/55 px-4 py-5 text-center flex flex-col items-center gap-1.5"
    >
      <p class="text-xs text-slate-300">{{ isPaidTier ? '付费模型池暂未配置' : '免费模型池暂未配置' }}</p>
      <p class="text-[11px] text-slate-500">点击右上角按钮添加服务商节点，同一组 API Key 可勾选任意多个可用模型</p>
    </div>

    <div v-else class="flex flex-col gap-2">
      <div
        v-for="(entry, idx) in poolEntries"
        :key="entry.id"
        class="rounded-xl border border-[#fff8ee]/10 bg-[#050811]/85 p-3 flex flex-col gap-2"
      >
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <button
            type="button"
            @click="expandedPoolId = expandedPoolId === entry.id ? '' : entry.id"
            class="flex items-center gap-2 min-w-0 flex-1 text-left cursor-pointer"
          >
            <span
              :class="[
                'w-2 h-2 rounded-full shrink-0',
                !entry.enabled
                  ? 'bg-slate-600'
                  : entry.api_key && selectedModels(entry).length > 0
                    ? isPaidTier
                      ? 'bg-[#f2b880] shadow-[0_0_8px_#f2b880]'
                      : 'bg-emerald-400 shadow-[0_0_8px_#34d399]'
                    : 'bg-amber-400/75'
              ]"
            />
            <span class="text-xs font-semibold text-white truncate">{{ entry.name }}</span>
            <span class="font-mono text-[10px] px-1.5 py-0.5 rounded bg-[#fff8ee]/[0.05] text-slate-400 shrink-0">
              {{
                selectedModels(entry).length > 1
                  ? `${selectedModels(entry)[0]} (+${selectedModels(entry).length - 1})`
                  : selectedModels(entry)[0] || '待选择模型'
              }}
            </span>
            <span
              v-if="healthBadgeText('pool:' + entry.id)"
              :class="[
                'hidden sm:inline-flex font-mono text-[9.5px] px-1.5 py-0.5 rounded border shrink-0',
                (healthForTarget('pool:' + entry.id)?.cooldown_sec || 0) > 0
                  ? 'border-amber-400/35 bg-amber-400/10 text-amber-300'
                  : (healthForTarget('pool:' + entry.id)?.success_count || 0) > 0
                    ? 'border-emerald-400/30 bg-emerald-400/10 text-emerald-300'
                    : 'border-rose-400/30 bg-rose-500/10 text-rose-300'
              ]"
            >
              {{ healthBadgeText('pool:' + entry.id) }}
            </span>
          </button>

          <div class="flex items-center gap-1 shrink-0">
            <button
              type="button"
              :disabled="idx === 0"
              @click="movePoolNode(idx, -1)"
              title="提高优先级"
              class="w-6 h-6 rounded border border-[#fff8ee]/10 text-[10px] text-slate-400 hover:text-[#f2b880] disabled:opacity-25 cursor-pointer"
            >
              ↑
            </button>
            <button
              type="button"
              :disabled="idx === poolEntries.length - 1"
              @click="movePoolNode(idx, 1)"
              title="降低优先级"
              class="w-6 h-6 rounded border border-[#fff8ee]/10 text-[10px] text-slate-400 hover:text-[#f2b880] disabled:opacity-25 cursor-pointer"
            >
              ↓
            </button>
            <button
              type="button"
              :disabled="probingId === entry.id"
              @click="probeTarget(entry.id, entry.provider, entry.base_url, entry.api_key, selectedModels(entry)[0] || entry.model)"
              class="px-2 py-0.5 rounded border border-[#f2b880]/35 text-[10.5px] text-[#f2b880] hover:bg-[#f2b880]/15 cursor-pointer disabled:opacity-40"
            >
              {{ probingId === entry.id ? '测速中' : '测速' }}
            </button>
            <button
              type="button"
              @click="entry.enabled = !entry.enabled"
              :class="[
                'px-2 py-0.5 rounded border text-[10.5px] cursor-pointer',
                entry.enabled
                  ? 'border-emerald-400/35 bg-emerald-400/10 text-emerald-300'
                  : 'border-[#fff8ee]/12 text-slate-500'
              ]"
            >
              {{ entry.enabled ? '启用' : '停用' }}
            </button>
            <button
              type="button"
              @click="duplicatePoolNode(entry)"
              title="克隆节点地址与密钥配置"
              class="px-2 py-0.5 rounded border border-[#fff8ee]/12 text-[10.5px] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer"
            >
              克隆
            </button>
            <button
              type="button"
              @click="removePoolNode(entry.id)"
              class="px-1.5 py-0.5 rounded border border-rose-400/30 text-[10.5px] text-rose-300 hover:bg-rose-500/15 cursor-pointer"
            >
              删
            </button>
          </div>
        </div>

        <div
          v-if="probeResults[entry.id]"
          :class="[
            'text-[10.5px] font-mono px-2 py-1 rounded border',
            probeResults[entry.id].ok
              ? 'border-emerald-400/25 bg-emerald-400/10 text-emerald-300'
              : 'border-rose-400/30 bg-rose-500/10 text-rose-300'
          ]"
        >
          {{ probeResults[entry.id].msg }}
        </div>

        <div v-if="expandedPoolId === entry.id" class="pt-2 border-t border-[#fff8ee]/8 grid grid-cols-1 sm:grid-cols-2 gap-2 text-[11px]">
          <label class="sm:col-span-2 flex flex-col gap-1 text-slate-400">
            <span>API Key ({{ entry.api_key ? '已配置，留着 **** 不变或覆盖输入新 Key' : '待填入 API Key' }})</span>
            <SecretInput
              v-model="entry.api_key"
              size="sm"
              placeholder="粘贴对应平台的 API Key..."
              toggle-label="节点 API Key"
            />
          </label>
          <label class="flex flex-col gap-1 text-slate-400">
            <span>节点名称</span>
            <input v-model="entry.name" type="text" class="min-h-[36px] rounded-lg border border-[#fff8ee]/12 bg-[#070b18] px-2.5 py-1 text-xs text-white focus:border-[#f2b880]/60" />
          </label>
          <label class="flex flex-col gap-1 text-slate-400">
            <span>OpenAI 兼容 / Google Base URL</span>
            <input v-model="entry.base_url" type="text" placeholder="https://.../v1" class="min-h-[36px] rounded-lg border border-[#fff8ee]/12 bg-[#070b18] px-2.5 py-1 font-mono text-xs text-white focus:border-[#f2b880]/60" />
          </label>

          <!-- Checked Models under this Provider Endpoint -->
          <div class="sm:col-span-2 flex flex-col gap-1.5 rounded-lg border border-[#fff8ee]/10 bg-[#070b18] p-2.5">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <span class="text-[11px] font-medium text-slate-300">
                已勾选加入当前池的模型 ({{ selectedModels(entry).length }})
              </span>
              <button
                type="button"
                :disabled="fetchingModelsId === entry.id"
                @click="fetchModelsForTarget(entry.id, entry.provider, entry.base_url, entry.api_key, entry)"
                :class="[
                  'text-[10.5px] hover:underline cursor-pointer disabled:opacity-45',
                  isPaidTier ? 'text-[#f2b880]' : 'text-emerald-300'
                ]"
              >
                {{ fetchingModelsId === entry.id ? '探测可用模型中...' : verifyOnFetch ? '拉取并实测可用模型' : '拉取候选清单' }}
              </button>
            </div>

            <div v-if="selectedModels(entry).length > 0" class="flex flex-wrap gap-1.5">
              <button
                v-for="m in selectedModels(entry)"
                :key="m"
                type="button"
                @click="toggleEntryModel(entry, m)"
                title="点击移出已选模型"
                :class="[
                  'inline-flex items-center gap-1 px-2 py-0.5 rounded-md border font-mono text-[10.5px] cursor-pointer',
                  isPaidTier
                    ? 'border-[#f2b880]/45 bg-[#f2b880]/15 text-[#f7cba0]'
                    : 'border-emerald-400/45 bg-emerald-400/15 text-emerald-200'
                ]"
              >
                <span>{{ m }}</span>
                <span class="opacity-70">×</span>
              </button>
            </div>
            <div v-else class="text-[10.5px] text-slate-500">
              尚未勾选模型：点击右上角拉取列表勾选，或在下方直接输入模型 ID 添加。
            </div>

            <div class="flex items-center gap-1.5 pt-0.5">
              <input
                v-model="customModelInput[entry.id]"
                type="text"
                placeholder="手动输入模型 ID (支持逗号分隔多个)..."
                @keydown.enter.prevent="addManualModel(entry)"
                class="flex-1 min-h-[32px] rounded-lg border border-[#fff8ee]/12 bg-[#050811] px-2.5 py-1 font-mono text-[11px] text-white focus:border-[#f2b880]/60"
              />
              <button
                type="button"
                @click="addManualModel(entry)"
                class="min-h-[32px] px-2.5 rounded-lg border border-[#fff8ee]/14 text-[10.5px] text-slate-200 hover:border-[#f2b880]/45 cursor-pointer"
              >
                添加模型
              </button>
            </div>
          </div>

          <div v-if="fetchedModelMeta[entry.id]" class="sm:col-span-2 flex flex-col gap-2 rounded-lg border border-emerald-400/20 bg-[#070b18] p-2.5">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <span class="text-[10.5px] text-slate-300">
                {{ verifyOnFetch ? '实测可用（点击勾选/取消）' : '候选模型（点击勾选/取消）' }} ({{ filteredUsableChecks(entry.id).length }})
                <template v-if="fetchedModelMeta[entry.id].filteredOutCount > 0">
                  · 已过滤 {{ fetchedModelMeta[entry.id].filteredOutCount }} 个非对话模型
                </template>
              </span>
              <div class="flex items-center gap-2">
                <button
                  v-if="filteredUsableChecks(entry.id).length > 1"
                  type="button"
                  @click="selectAllFetchedUsable(entry)"
                  class="text-[10px] text-[#f2b880] hover:underline cursor-pointer"
                >
                  勾选前 {{ Math.min(12, filteredUsableChecks(entry.id).length) }} 个
                </button>
                <input
                  v-if="fetchedModelMeta[entry.id].usable.length > 6"
                  v-model="fetchedModelMeta[entry.id].query"
                  type="search"
                  placeholder="搜索可用模型..."
                  class="w-32 bg-[#050811] border border-[#fff8ee]/12 rounded-md px-2 py-0.5 text-[10px] text-white focus:border-emerald-400/50 focus:outline-none"
                />
              </div>
            </div>
            <div v-if="filteredUsableChecks(entry.id).length > 0" class="flex flex-wrap gap-1.5 max-h-32 overflow-y-auto">
              <button
                v-for="item in filteredUsableChecks(entry.id)"
                :key="item.id"
                type="button"
                @click="toggleEntryModel(entry, item.id)"
                :class="[
                  'inline-flex items-center gap-1.5 px-2 py-0.5 rounded border font-mono text-[10px] cursor-pointer transition',
                  selectedModels(entry).includes(item.id)
                    ? isPaidTier
                      ? 'border-[#f2b880] bg-[#f2b880]/20 text-[#f7cba0]'
                      : 'border-emerald-400 bg-emerald-400/20 text-emerald-200'
                    : 'border-[#fff8ee]/12 text-slate-300 hover:border-emerald-400/45'
                ]"
              >
                <span>{{ selectedModels(entry).includes(item.id) ? '[已选] ' : '' }}{{ item.id }}</span>
                <span v-if="item.latency_ms" class="text-[9px] text-emerald-300/80">{{ item.latency_ms }}ms</span>
              </button>
            </div>
            <div v-if="fetchedModelMeta[entry.id].failed.length > 0" class="pt-1 border-t border-[#fff8ee]/8">
              <button
                type="button"
                @click="fetchedModelMeta[entry.id].showFailed = !fetchedModelMeta[entry.id].showFailed"
                class="text-[10px] text-slate-400 hover:text-slate-200 cursor-pointer"
              >
                {{ fetchedModelMeta[entry.id].showFailed ? '收起不可用模型' : `查看被排除的不可用模型 (${fetchedModelMeta[entry.id].failed.length})` }}
              </button>
              <div v-if="fetchedModelMeta[entry.id].showFailed" class="mt-1.5 flex flex-col gap-1 max-h-24 overflow-y-auto">
                <div v-for="bad in fetchedModelMeta[entry.id].failed" :key="bad.id" class="flex items-center justify-between gap-2 text-[10px] font-mono text-rose-300/80">
                  <span class="truncate">{{ bad.id }}</span>
                  <span class="shrink-0 text-slate-400">{{ bad.reason }}</span>
                </div>
              </div>
            </div>
          </div>
          <p v-if="entry.note" class="sm:col-span-2 text-slate-500">{{ entry.note }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
