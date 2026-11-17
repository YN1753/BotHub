<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import type { ModelHealthStats, ModelPoolEntry, UpstreamModelCheck } from '../../types/bothub'
import ToggleSwitch from '../ToggleSwitch.vue'

interface FetchedModelMetaItem {
  usable: UpstreamModelCheck[]
  failed: UpstreamModelCheck[]
  filteredOutCount: number
  unverifiedCount: number
  showFailed: boolean
  query: string
}

const props = defineProps<{
  poolForLight: boolean
  poolFallback: boolean
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
  'update:poolForLight': [value: boolean]
  'update:poolFallback': [value: boolean]
  'update:verifyOnFetch': [value: boolean]
  'update:poolEntries': [value: ModelPoolEntry[]]
  'update:probingAll': [value: boolean]
}>()

const store = useAppStore()
const expandedPoolId = ref(props.poolEntries[0]?.id || '')
const poolKeyVisible = ref<Record<string, boolean>>({})
const probingAll = ref(false)

const poolSummaryChip = computed(() => {
  const total = props.poolEntries.length
  const enabled = props.poolEntries.filter((x) => x.enabled).length
  const ready = props.poolEntries.filter((x) => x.enabled && x.api_key && x.model).length
  return `已启用 ${enabled}/${total} · 就绪 ${ready}`
})

function addEmptyPoolNode() {
  const suffix = Date.now().toString(36).slice(-4)
  const id = `node_${suffix}`
  const next = [
    ...props.poolEntries,
    {
      id,
      name: `模型节点 #${props.poolEntries.length + 1}`,
      provider: 'openai',
      base_url: '',
      api_key: '',
      model: '',
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
    id: `node_${suffix}`,
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
    store.notify('请至少为一个启用中的免费节点填入 API Key')
    return
  }
  probingAll.value = true
  emit('update:probingAll', true)
  let okCount = 0
  for (const entry of active) {
    await props.probeTarget(entry.id, entry.provider, entry.base_url, entry.api_key, entry.model)
    if (props.probeResults[entry.id]?.ok) okCount++
  }
  probingAll.value = false
  emit('update:probingAll', false)
  store.notify(`全池测速完成：${okCount}/${active.length} 节点连通`)
}
</script>

<template>
  <div class="modal-section flex flex-col gap-3">
    <div class="flex items-center justify-between gap-2 flex-wrap">
      <div>
        <div class="flex items-center gap-2 flex-wrap">
          <span class="text-xs font-semibold text-emerald-300">免费 / 扩展 AI 模型池 (自主添加 · 实时探测可用模型)</span>
          <span
            v-if="poolEntries.length > 0"
            class="font-mono text-[10px] px-1.5 py-0.5 rounded border border-emerald-400/25 bg-emerald-400/10 text-emerald-200"
          >
            {{ poolSummaryChip }}
          </span>
        </div>
        <p class="text-[11px] text-slate-400 mt-0.5">支持接入任意 OpenAI 兼容或 Gemini 接口，自动过滤不可用模型并按优先级故障切换</p>
      </div>
      <div class="flex items-center gap-1.5 shrink-0 flex-wrap">
        <button
          v-if="poolEntries.length > 0"
          type="button"
          :disabled="probingAll"
          @click="probeAllEnabledPool"
          class="min-h-[30px] px-2.5 rounded-lg border border-[#f2b880]/35 bg-[#f2b880]/10 hover:bg-[#f2b880]/20 text-[11px] text-[#f2b880] cursor-pointer disabled:opacity-45"
        >
          {{ probingAll ? '全池测速中...' : '一键全池测速' }}
        </button>
        <button
          type="button"
          @click="addEmptyPoolNode"
          class="min-h-[30px] px-2.5 rounded-lg border border-emerald-400/35 bg-emerald-400/10 hover:bg-emerald-400/18 text-[11px] text-emerald-300 cursor-pointer shrink-0"
        >
          + 添加模型节点
        </button>
      </div>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
      <ToggleSwitch
        :model-value="poolForLight"
        @update:model-value="emit('update:poolForLight', $event)"
        label="后台摘要优先用免费池"
        description="晨报/雷达/网页精简先走免费节点"
      />
      <ToggleSwitch
        :model-value="poolFallback"
        @update:model-value="emit('update:poolFallback', $event)"
        label="主力缺 Key/故障自动回退"
        description="主力异常时自动顺延调用免费池"
      />
      <ToggleSwitch
        :model-value="verifyOnFetch"
        @update:model-value="emit('update:verifyOnFetch', $event)"
        label="拉取时自动实测可用性"
        description="过滤 embedding/tts 并逐一探测 429/404 不可用模型"
      />
    </div>

    <div
      v-if="poolEntries.length === 0"
      class="rounded-xl border border-dashed border-[#fff8ee]/12 bg-[#050811]/55 px-4 py-6 text-center flex flex-col items-center gap-2"
    >
      <p class="text-xs text-slate-300">当前模型池为空。</p>
      <p class="text-[11px] text-slate-500">点击右上角「+ 添加模型节点」，填写 Base URL 与 API Key 即可自动探测并挑选可用模型。</p>
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
                  : entry.api_key && entry.model
                    ? 'bg-emerald-400 shadow-[0_0_8px_#34d399]'
                    : 'bg-amber-400/75'
              ]"
            />
            <span class="text-xs font-semibold text-white truncate">{{ entry.name }}</span>
            <span class="font-mono text-[10px] px-1.5 py-0.5 rounded bg-[#fff8ee]/[0.05] text-slate-400 shrink-0">
              {{ entry.model || '未选择模型' }}
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
              @click="probeTarget(entry.id, entry.provider, entry.base_url, entry.api_key, entry.model)"
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
              title="克隆节点地址与密钥配置新模型"
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
            <div class="relative">
              <input v-model="entry.api_key" :type="poolKeyVisible[entry.id] ? 'text' : 'password'" placeholder="粘贴对应平台的 API Key..." class="w-full min-h-[36px] rounded-lg border border-[#fff8ee]/12 bg-[#070b18] pl-2.5 pr-12 py-1 font-mono text-xs text-white focus:border-[#f2b880]/60" />
              <button type="button" @click="poolKeyVisible[entry.id] = !poolKeyVisible[entry.id]" class="absolute right-2 top-1/2 -translate-y-1/2 px-1.5 py-0.5 rounded text-[10.5px] font-mono text-slate-400 hover:text-[#f2b880] cursor-pointer">
                {{ poolKeyVisible[entry.id] ? '隐藏' : '显示' }}
              </button>
            </div>
          </label>
          <label class="flex flex-col gap-1 text-slate-400">
            <span>显示名称</span>
            <input v-model="entry.name" type="text" class="min-h-[36px] rounded-lg border border-[#fff8ee]/12 bg-[#070b18] px-2.5 py-1 text-xs text-white focus:border-[#f2b880]/60" />
          </label>
          <div class="flex flex-col gap-1 text-slate-400">
            <div class="flex items-center justify-between gap-2">
              <span>上游模型 ID</span>
              <button
                type="button"
                :disabled="fetchingModelsId === entry.id"
                @click="fetchModelsForTarget(entry.id, entry.provider, entry.base_url, entry.api_key, entry)"
                class="text-[10.5px] text-emerald-300 hover:underline cursor-pointer disabled:opacity-45"
              >
                {{ fetchingModelsId === entry.id ? '探测可用模型中...' : verifyOnFetch ? '拉取并筛选可用模型' : '仅拉取清单' }}
              </button>
            </div>
            <input v-model="entry.model" type="text" placeholder="点击上方自动筛选可用模型" class="min-h-[36px] rounded-lg border border-[#fff8ee]/12 bg-[#070b18] px-2.5 py-1 font-mono text-xs text-white focus:border-[#f2b880]/60" />
          </div>
          <div v-if="fetchedModelMeta[entry.id]" class="sm:col-span-2 flex flex-col gap-2 rounded-lg border border-emerald-400/20 bg-[#070b18] p-2.5">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <span class="text-[10.5px] text-slate-300">
                {{ verifyOnFetch ? '实测可用' : '候选模型' }} ({{ filteredUsableChecks(entry.id).length }})
                <template v-if="fetchedModelMeta[entry.id].filteredOutCount > 0">
                  · 已过滤 {{ fetchedModelMeta[entry.id].filteredOutCount }} 个非对话模型
                </template>
              </span>
              <input
                v-if="fetchedModelMeta[entry.id].usable.length > 6"
                v-model="fetchedModelMeta[entry.id].query"
                type="search"
                placeholder="搜索可用模型..."
                class="w-32 bg-[#050811] border border-[#fff8ee]/12 rounded-md px-2 py-0.5 text-[10px] text-white focus:border-emerald-400/50 focus:outline-none"
              />
            </div>
            <div v-if="filteredUsableChecks(entry.id).length > 0" class="flex flex-wrap gap-1.5 max-h-32 overflow-y-auto">
              <button
                v-for="item in filteredUsableChecks(entry.id)"
                :key="item.id"
                type="button"
                @click="entry.model = item.id"
                :class="[
                  'inline-flex items-center gap-1.5 px-2 py-0.5 rounded border font-mono text-[10px] cursor-pointer transition',
                  entry.model === item.id
                    ? 'border-emerald-400 bg-emerald-400/20 text-emerald-200'
                    : 'border-[#fff8ee]/12 text-slate-300 hover:border-emerald-400/45'
                ]"
              >
                <span>{{ item.id }}</span>
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
          <label class="sm:col-span-2 flex flex-col gap-1 text-slate-400">
            <span>OpenAI 兼容 / Google Base URL</span>
            <input v-model="entry.base_url" type="text" placeholder="https://.../v1" class="min-h-[36px] rounded-lg border border-[#fff8ee]/12 bg-[#070b18] px-2.5 py-1 font-mono text-xs text-white focus:border-[#f2b880]/60" />
          </label>
          <p v-if="entry.note" class="sm:col-span-2 text-slate-500">{{ entry.note }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
