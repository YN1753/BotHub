<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import type {
  ModelHealthStats,
  ModelProvider,
  PoolStrategy,
  PooledModel,
} from '../../types/bothub'
import {
  POOL_STRATEGY_OPTIONS,
  PUBLIC_PROVIDER_PRESETS,
  canonicalModelTargetId,
  cleanProviderBaseUrl,
  formatCostCNY,
  formatCostUSD,
  formatTokenCount,
  type PublicProviderPreset,
} from '../../utils/modelRouting'
import SegmentedTabs from '../SegmentedTabs.vue'
import SecretInput from '../ui/SecretInput.vue'

const props = defineProps<{
  providers: ModelProvider[]
  models: PooledModel[]
  strategy: PoolStrategy
}>()

const emit = defineEmits<{
  'update:providers': [value: ModelProvider[]]
  'update:models': [value: PooledModel[]]
  'update:strategy': [value: PoolStrategy]
}>()

const store = useAppStore()

const subTab = ref<'providers' | 'pool'>('providers')
const usageWindow = ref<'7d' | '30d'>('7d')
const expandedProviderId = ref<string>(props.providers[0]?.id || '')
const fetchingProviderId = ref('')
const fetchedByProvider = ref<
  Record<
    string,
    {
      models: string[]
      filteredOutCount: number
      query: string
      filterTag: string
    }
  >
>({})
const manualModelInput = ref<Record<string, string>>({})
const probingKey = ref('')
const probingPool = ref(false)
const probeMsgByKey = ref<Record<string, { ok: boolean; msg: string }>>({})
const clearingCooldown = ref(false)
const showUsageDetail = ref(false)
const verifyOnFetch = ref(true)
const revealedKeyProviderId = ref<string>('')
const editingPriceKey = ref<string>('')

function callerDisplayLabel(botId: string, taskId: string, source: string): string {
  if (taskId) {
    const task = store.state.tasks.find((t) => t.id === taskId)
    return task ? `任务 · ${task.name}` : `任务 · ${taskId}`
  }
  if (botId) {
    const bot = store.state.bots.find((b) => b.id === botId)
    return bot ? bot.name : botId
  }
  return source === 'routine' ? '后台例程' : '系统轻模型'
}

const orderedModels = computed(() =>
  [...props.models].sort((a, b) => a.priority - b.priority)
)

const SUB_TABS = computed(() => [
  { id: 'providers', label: `渠道管理 (${props.providers.length})` },
  { id: 'pool', label: `统一模型池 (${props.models.length})` },
])

const providerMap = computed(() => {
  const m = new Map<string, ModelProvider>()
  for (const p of props.providers) m.set(p.id, p)
  return m
})

const activeUsage = computed(() =>
  usageWindow.value === '30d' ? store.state.modelUsage30d : store.state.modelUsageSummary
)

const successRateLabel = computed(() => {
  const total = activeUsage.value?.total_calls || 0
  if (total <= 0) return '--'
  return `${Math.round(((activeUsage.value?.success_calls || 0) / total) * 100)}%`
})

const cacheHitRateLabel = computed(() => {
  const prompt = activeUsage.value?.total_prompt_tokens || 0
  if (prompt <= 0) return '--'
  const rate = activeUsage.value?.cache_hit_rate ?? 0
  return `${Math.round(rate * 100)}%`
})

function providerAssignedCount(providerId: string): number {
  let count = 0
  for (const m of props.models) {
    if (m.provider_id === providerId) count++
  }
  return count
}

function isModelInPool(providerId: string, modelId: string): boolean {
  return props.models.some((m) => m.provider_id === providerId && m.model_id === modelId)
}

function addProviderFromPreset(preset: PublicProviderPreset) {
  const suffix = Date.now().toString(36).slice(-4)
  const id = `${preset.key}_${suffix}`
  const next: ModelProvider[] = [
    ...props.providers,
    {
      id,
      name: preset.name,
      protocol: preset.protocol,
      base_url: preset.base_url,
      api_key: '',
      enabled: true,
      note: '',
    },
  ]
  emit('update:providers', next)
  expandedProviderId.value = id
  subTab.value = 'providers'
}

function updateProviderField(id: string, patch: Partial<ModelProvider>) {
  const next = props.providers.map((p) => (p.id === id ? { ...p, ...patch } : p))
  emit('update:providers', next)
}

function splitApiKeys(raw: string): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const part of raw.split(/[\r\n,;]+/)) {
    const k = part.trim()
    if (k && !seen.has(k)) {
      seen.add(k)
      out.push(k)
    }
  }
  return out
}

function providerEffectiveKeys(provider: ModelProvider): string[] {
  if (Array.isArray(provider.api_keys) && provider.api_keys.length > 0) {
    return splitApiKeys(provider.api_keys.join('\n'))
  }
  return splitApiKeys(provider.api_key || '')
}

function providerKeysText(provider: ModelProvider): string {
  return providerEffectiveKeys(provider).join('\n')
}

function updateProviderKeys(provider: ModelProvider, rawText: string) {
  const keys = splitApiKeys(rawText)
  updateProviderField(provider.id, {
    api_key: keys[0] || '',
    api_keys: keys,
  })
}

function normalizeProviderUrl(provider: ModelProvider) {
  const cleaned = cleanProviderBaseUrl(provider.base_url)
  if (cleaned !== provider.base_url) {
    updateProviderField(provider.id, { base_url: cleaned })
  }
}

async function removeProvider(provider: ModelProvider) {
  const count = providerAssignedCount(provider.id)
  if (count > 0) {
    const ok = await store.openConfirm({
      title: '删除模型渠道',
      message: `渠道「${provider.name}」当前关联了 ${count} 个模型池条目，删除后将一并移除。确定删除？`,
      confirmText: '确认删除',
      danger: true,
    })
    if (!ok) return
  }
  emit(
    'update:providers',
    props.providers.filter((p) => p.id !== provider.id)
  )
  emit(
    'update:models',
    props.models.filter((m) => m.provider_id !== provider.id)
  )
}

function toggleModelInPool(providerId: string, modelId: string) {
  const cleanId = modelId.trim()
  if (!cleanId) return
  if (isModelInPool(providerId, cleanId)) {
    emit(
      'update:models',
      props.models.filter((m) => !(m.provider_id === providerId && m.model_id === cleanId))
    )
    return
  }
  const nextModel: PooledModel = {
    provider_id: providerId,
    model_id: cleanId,
    enabled: true,
    priority: (props.models.length + 1) * 10,
  }
  emit('update:models', [...props.models, nextModel])
}

function addManualModels(providerId: string) {
  const raw = (manualModelInput.value[providerId] || '').trim()
  if (!raw) return
  const parts = raw
    .split(/[,，\n]/)
    .map((s) => s.trim())
    .filter(Boolean)
  if (parts.length === 0) return

  const next = [...props.models]
  let seq = next.length
  for (const mid of parts) {
    const idx = next.findIndex((m) => m.provider_id === providerId && m.model_id === mid)
    if (idx >= 0) {
      next[idx] = { ...next[idx], enabled: true }
    } else {
      seq += 1
      next.push({
        provider_id: providerId,
        model_id: mid,
        enabled: true,
        priority: seq * 10,
      })
    }
  }
  emit('update:models', next)
  manualModelInput.value[providerId] = ''
  store.notify(`已将 ${parts.length} 个模型加入统一模型池`)
}

async function fetchFullModels(provider: ModelProvider) {
  if (fetchingProviderId.value) return
  const baseUrl = cleanProviderBaseUrl(provider.base_url)
  const keys = providerEffectiveKeys(provider)
  if (!baseUrl || keys.length === 0) {
    store.notify('请先填写 Base URL 与至少一个 API Key 再拉取模型列表')
    return
  }
  if (baseUrl !== provider.base_url) {
    updateProviderField(provider.id, { base_url: baseUrl })
  }
  fetchingProviderId.value = provider.id
  try {
    const res = await BotHubApi.fetchUpstreamModels({
      provider_id: provider.id,
      protocol: provider.protocol,
      base_url: baseUrl,
      api_key: keys[0],
      api_keys: keys,
      verify: verifyOnFetch.value,
    })
    const list = res.models || []
    fetchedByProvider.value[provider.id] = {
      models: list,
      filteredOutCount: (res.filtered_out_count || 0) + (res.failed_verify_count || 0),
      query: '',
      filterTag: '',
    }
    if (list.length > 0) {
      const parts: string[] = []
      if ((res.filtered_out_count || 0) > 0) parts.push(`过滤 ${res.filtered_out_count} 个非对话模型`)
      if ((res.failed_verify_count || 0) > 0) parts.push(`拦截 ${res.failed_verify_count} 个不可用模型`)
      const skipNote = parts.length > 0 ? `（已${parts.join('、')}）` : ''
      store.notify(`${verifyOnFetch.value ? '实测拉取' : '拉取'}完成：共 ${list.length} 个可用对话模型${skipNote}`)
    } else {
      store.notify('上游接口未返回可用对话模型')
    }
  } catch (err: any) {
    store.notify(`拉取模型失败: ${err.message}`)
  } finally {
    fetchingProviderId.value = ''
  }
}

function vendorTagsForProvider(providerId: string): string[] {
  const meta = fetchedByProvider.value[providerId]
  if (!meta) return []
  const counts = new Map<string, number>()
  for (const id of meta.models) {
    const slash = id.indexOf('/')
    if (slash > 0) {
      const prefix = id.slice(0, slash)
      counts.set(prefix, (counts.get(prefix) || 0) + 1)
    }
  }
  return Array.from(counts.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, 6)
    .map(([k]) => k)
}

function filteredFetchedModels(providerId: string): string[] {
  const meta = fetchedByProvider.value[providerId]
  if (!meta) return []
  const q = meta.query.trim().toLowerCase()
  const tag = meta.filterTag
  return meta.models.filter((id) => {
    const lower = id.toLowerCase()
    if (tag && !lower.startsWith(`${tag.toLowerCase()}/`)) return false
    if (q && !lower.includes(q)) return false
    return true
  })
}

function batchAddFiltered(providerId: string, maxCount = 12) {
  const list = filteredFetchedModels(providerId).slice(0, maxCount)
  if (list.length === 0) return
  const next = [...props.models]
  let seq = next.length
  let added = 0
  for (const mid of list) {
    const exists = next.some((m) => m.provider_id === providerId && m.model_id === mid)
    if (!exists) {
      seq += 1
      next.push({
        provider_id: providerId,
        model_id: mid,
        enabled: true,
        priority: seq * 10,
      })
      added += 1
    }
  }
  if (added > 0) {
    emit('update:models', next)
    store.notify(`已批量添加 ${added} 个模型至统一模型池`)
  } else {
    store.notify('当前筛出的模型均已在池中')
  }
}

function updatePooledModel(m: PooledModel, patch: Partial<PooledModel>) {
  emit(
    'update:models',
    props.models.map((x) =>
      x.provider_id === m.provider_id && x.model_id === m.model_id
        ? { ...x, ...patch }
        : x
    )
  )
}

function movePooledModel(index: number, delta: number) {
  const list = [...orderedModels.value]
  const nextIdx = index + delta
  if (nextIdx < 0 || nextIdx >= list.length) return
  const [item] = list.splice(index, 1)
  list.splice(nextIdx, 0, item)
  const renumbered = list.map((m, i) => ({ ...m, priority: (i + 1) * 10 }))
  emit('update:models', renumbered)
}

function removePooledModel(m: PooledModel) {
  emit(
    'update:models',
    props.models.filter((x) => !(x.provider_id === m.provider_id && x.model_id === m.model_id))
  )
}

function healthForModel(m: PooledModel): ModelHealthStats | undefined {
  const key = canonicalModelTargetId(m.provider_id, m.model_id)
  return store.state.modelHealth?.[key]
}

const hasCooling = computed(() =>
  orderedModels.value.some((m) => (healthForModel(m)?.cooldown_sec || 0) > 0)
)

const hasFailedProbe = computed(() =>
  orderedModels.value.some((m) => {
    const key = canonicalModelTargetId(m.provider_id, m.model_id)
    return m.enabled && probeMsgByKey.value[key]?.ok === false
  })
)

async function clearCooldown(targetId = '') {
  if (clearingCooldown.value) return
  clearingCooldown.value = true
  try {
    const res = await BotHubApi.clearModelCooldown(targetId)
    if (res.model_health) store.state.modelHealth = res.model_health
    if (res.model_options) store.state.modelOptions = res.model_options
    store.notify(targetId ? '已解除该模型冷却状态' : '已解除全部模型冷却状态')
  } catch (err: any) {
    store.notify(err.message || '解除冷却失败')
  } finally {
    clearingCooldown.value = false
  }
}

async function probeSingleModel(m: PooledModel) {
  const p = providerMap.value.get(m.provider_id)
  if (!p) {
    store.notify('未找到所属渠道')
    return
  }
  const key = canonicalModelTargetId(m.provider_id, m.model_id)
  probingKey.value = key
  delete probeMsgByKey.value[key]
  try {
    const res = await BotHubApi.testModelTarget({
      provider_id: p.id,
      protocol: p.protocol,
      base_url: cleanProviderBaseUrl(p.base_url),
      api_key: p.api_key.trim(),
      model: m.model_id,
      probe_tools: true,
    })
    if (res.model_health) store.state.modelHealth = res.model_health
    if (res.model_options) store.state.modelOptions = res.model_options
    if (!res.ok) {
      probeMsgByKey.value[key] = { ok: false, msg: res.error || '探测失败' }
      return
    }
    const caps: string[] = []
    if (res.supports_tools === true) caps.push('Tools 可用')
    else if (res.supports_tools === false) caps.push('无 Tools')
    if (res.supports_thinking === true) caps.push('Thinking')
    const capSuffix = caps.length > 0 ? ` · ${caps.join(' / ')}` : ''
    probeMsgByKey.value[key] = { ok: true, msg: `${res.latency_ms}ms OK${capSuffix}` }

    updatePooledModel(m, {
      supports_tools: res.supports_tools !== undefined ? res.supports_tools : m.supports_tools,
      supports_thinking:
        res.supports_thinking !== undefined ? res.supports_thinking : m.supports_thinking,
      last_latency_ms: res.latency_ms || m.last_latency_ms,
    })
  } catch (err: any) {
    probeMsgByKey.value[key] = { ok: false, msg: err.message || '连接失败' }
  } finally {
    probingKey.value = ''
  }
}

async function probeEntirePool() {
  if (probingPool.value) return
  const list = orderedModels.value.filter((m) => {
    const p = providerMap.value.get(m.provider_id)
    return m.enabled && p?.enabled && Boolean(p.api_key)
  })
  if (list.length === 0) {
    store.notify('当前模型池没有可测速的已启用模型')
    return
  }
  const ok = await store.openConfirm({
    title: '模型池连通性测速确认',
    message: `即将对模型池中 ${list.length} 个已启用模型并发发送极短探测请求（每模型约几十 Tokens），用于探测延迟、Tools 与 Thinking 能力。确定继续？`,
    confirmText: '开始测速',
    danger: false,
  })
  if (!ok) return
  probingPool.value = true
  try {
    const res = await BotHubApi.testModelBatch({ probe_tools: true })
    if (res.model_health) store.state.modelHealth = res.model_health
    if (res.model_options) store.state.modelOptions = res.model_options
    let okCount = 0
    let failCount = 0
    const nextModels = [...props.models]
    for (const r of res.results || []) {
      if (r.ok) {
        okCount += 1
        const caps: string[] = []
        if (r.supports_tools === true) caps.push('Tools 可用')
        else if (r.supports_tools === false) caps.push('无 Tools')
        if (r.supports_thinking === true) caps.push('Thinking')
        probeMsgByKey.value[r.target_id] = {
          ok: true,
          msg: `${r.latency_ms}ms OK${caps.length ? ' · ' + caps.join(' / ') : ''}`,
        }
        const idx = nextModels.findIndex(
          (x) => x.provider_id === r.provider_id && x.model_id === r.model_id
        )
        if (idx >= 0) {
          nextModels[idx] = {
            ...nextModels[idx],
            supports_tools: r.supports_tools ?? nextModels[idx].supports_tools,
            supports_thinking: r.supports_thinking ?? nextModels[idx].supports_thinking,
            last_latency_ms: r.latency_ms || nextModels[idx].last_latency_ms,
          }
        }
      } else {
        failCount += 1
        probeMsgByKey.value[r.target_id] = { ok: false, msg: r.error || '探测失败' }
      }
    }
    emit('update:models', nextModels)
    store.notify(`模型池测速完成：${okCount} 个可用${failCount > 0 ? `，${failCount} 个异常` : ''}`)
  } catch (err: any) {
    store.notify(err.message || '批量测速失败')
  } finally {
    probingPool.value = false
  }
}

function disableFailedModels() {
  let disabledCount = 0
  const next = props.models.map((m) => {
    if (!m.enabled) return m
    const key = canonicalModelTargetId(m.provider_id, m.model_id)
    if (probeMsgByKey.value[key]?.ok === false) {
      disabledCount += 1
      return { ...m, enabled: false }
    }
    return m
  })
  if (disabledCount > 0) {
    emit('update:models', next)
    store.notify(`已停用 ${disabledCount} 个测速异常的模型，请点击「保存」生效`)
  }
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <!-- Unified Cost & Prompt Cache Telemetry Dashboard -->
    <div class="modal-section flex flex-col gap-2.5">
      <div class="flex items-center justify-between gap-2 flex-wrap">
        <div>
          <div class="text-xs font-semibold text-fg">
            统一模型池 · 费用估算与 Prompt 缓存看板
          </div>
          <p class="text-xs text-fg-muted mt-0.5">
            已开启两区 Prompt Caching 缓存加速；所有 Bot 与例程共享统一主力模型池，故障无缝容灾。
          </p>
        </div>
        <div class="flex items-center gap-1.5 shrink-0">
          <div class="inline-flex rounded-lg border border-line bg-canvas p-0.5 text-xs">
            <button
              type="button"
              @click="usageWindow = '7d'"
              :class="[
                'px-2 py-0.5 rounded-md cursor-pointer transition',
                usageWindow === '7d' ? 'bg-accent-soft text-accent-fg font-semibold' : 'text-fg-muted hover:text-fg',
              ]"
            >
              近 7 天
            </button>
            <button
              type="button"
              @click="usageWindow = '30d'"
              :class="[
                'px-2 py-0.5 rounded-md cursor-pointer transition',
                usageWindow === '30d' ? 'bg-accent-soft text-accent-fg font-semibold' : 'text-fg-muted hover:text-fg',
              ]"
            >
              近 30 天
            </button>
          </div>
          <span class="font-mono text-xs px-2.5 py-0.5 rounded-full border border-line text-fg-sub">
            成功率 {{ successRateLabel }}
          </span>
        </div>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
        <div class="rounded-xl border border-accent-border bg-accent-soft px-3 py-2">
          <div class="text-xs text-accent-fg">预估消费 ({{ usageWindow === '30d' ? '30日' : '7日' }})</div>
          <div class="font-mono text-sm font-semibold text-fg mt-0.5">
            {{ formatCostCNY(activeUsage?.total_cost_cny) }}
            <span class="text-xs font-normal text-fg-muted">({{ formatCostUSD(activeUsage?.total_cost_usd) }})</span>
          </div>
          <div class="font-mono text-xs text-fg-muted">
            汇率 1 USD ≈ ¥{{ (activeUsage?.usd_to_cny || 7.25).toFixed(2) }}
          </div>
        </div>

        <div class="rounded-xl border border-ok-border bg-ok-soft px-3 py-2">
          <div class="text-xs text-ok">Prompt 缓存命中率</div>
          <div class="font-mono text-sm font-semibold text-fg mt-0.5">
            {{ cacheHitRateLabel }}
          </div>
          <div class="font-mono text-xs text-fg-muted">
            命中 {{ formatTokenCount(activeUsage?.total_cached_tokens || 0) }} / {{ formatTokenCount(activeUsage?.total_prompt_tokens || 0) }} 输入
          </div>
        </div>

        <div class="rounded-xl border border-line bg-canvas px-3 py-2">
          <div class="text-xs text-fg-sub">总调用与用量</div>
          <div class="font-mono text-sm font-semibold text-fg mt-0.5">
            {{ activeUsage?.total_calls || 0 }} <span class="text-xs font-normal text-fg-muted">次</span>
          </div>
          <div class="font-mono text-xs text-fg-muted">
            共 {{ formatTokenCount(activeUsage?.total_tokens || 0) }} Tokens
          </div>
        </div>
      </div>

      <div
        v-if="(activeUsage?.top_models?.length || 0) > 0 || (activeUsage?.top_callers?.length || 0) > 0"
        class="pt-1 border-t border-line flex flex-col gap-2"
      >
        <button
          type="button"
          @click="showUsageDetail = !showUsageDetail"
          class="self-start text-xs text-accent-fg hover:underline cursor-pointer"
        >
          {{ showUsageDetail ? '收起费用与调用明细 ▲' : `展开近 ${usageWindow === '30d' ? '30' : '7'} 天按模型 / 按 Bot 消耗排行 ▼` }}
        </button>

        <div v-if="showUsageDetail" class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
          <div class="rounded-xl border border-line bg-canvas p-2.5 flex flex-col gap-1.5">
            <div class="text-xs font-semibold text-fg">按模型花费排行</div>
            <div
              v-for="row in (activeUsage?.top_models || []).slice(0, 8)"
              :key="row.target_id || row.model_id"
              class="flex items-center justify-between gap-2 text-xs"
            >
              <span class="font-mono text-fg-sub truncate flex-1">{{ row.model_id }}</span>
              <span class="font-mono text-fg-sub shrink-0">
                {{ row.calls }}次 · {{ formatTokenCount(row.tokens) }}
                <span class="text-accent-fg ml-1">{{ formatCostCNY(row.cost_cny) }}</span>
              </span>
            </div>
          </div>

          <div class="rounded-xl border border-line bg-canvas p-2.5 flex flex-col gap-1.5">
            <div class="text-xs font-semibold text-fg">按 Bot / 任务花费排行</div>
            <div
              v-for="(c, idx) in (activeUsage?.top_callers || []).slice(0, 8)"
              :key="`${c.bot_id}_${c.task_id}_${c.source}_${idx}`"
              class="flex items-center justify-between gap-2 text-xs"
            >
              <span class="text-fg-sub truncate flex-1">{{ callerDisplayLabel(c.bot_id, c.task_id, c.source) }}</span>
              <span class="font-mono text-fg-sub shrink-0">
                {{ c.calls }}次 · {{ formatTokenCount(c.tokens) }}
                <span class="text-accent-fg ml-1">{{ formatCostCNY(c.cost_cny) }}</span>
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 2-Tab Switcher: Providers / Unified Model Pool -->
    <SegmentedTabs v-model="subTab" :items="SUB_TABS" variant="subtle" aria-label="AI 模型配置分层视图" />

    <!-- TAB 1: Providers -->
    <div v-if="subTab === 'providers'" class="flex flex-col gap-3">
      <div class="modal-section flex flex-col gap-2">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <span class="text-xs font-semibold text-accent-fg">快速添加模型渠道</span>
          <span class="text-xs text-fg-muted">填入标准接口地址与 API Key，拉取后勾选加入统一模型池</span>
        </div>
        <div class="flex items-center gap-1.5 flex-wrap">
          <button
            v-for="preset in PUBLIC_PROVIDER_PRESETS"
            :key="preset.key"
            type="button"
            :title="preset.hint"
            @click="addProviderFromPreset(preset)"
            class="min-h-[30px] px-2.5 py-1 rounded-lg border border-line hover:border-accent-border hover:bg-accent-soft text-xs text-fg hover:text-accent-fg transition cursor-pointer"
          >
            + {{ preset.label }}
          </button>
        </div>
      </div>

      <div
        v-if="providers.length === 0"
        class="rounded-xl border border-dashed border-line bg-canvas px-4 py-7 text-center flex flex-col items-center gap-2"
      >
        <div class="text-xs font-semibold text-fg">尚未添加任何模型渠道</div>
        <p class="text-xs text-fg-muted max-w-md leading-relaxed">
          点击上方常用渠道模板或「自定义中转」填入 Base URL 与 API Key，即可拉取模型列表并一键加入统一模型池。
        </p>
      </div>

      <div
        v-for="provider in providers"
        :key="provider.id"
        :class="[
          'rounded-xl border transition-colors',
          provider.enabled ? 'border-line bg-canvas' : 'border-line bg-canvas opacity-75',
        ]"
      >
        <div class="flex items-center justify-between gap-2 px-3.5 py-2.5 flex-wrap">
          <button
            type="button"
            @click="expandedProviderId = expandedProviderId === provider.id ? '' : provider.id"
            class="flex items-center gap-2 min-w-0 flex-1 text-left cursor-pointer"
          >
            <span
              :class="[
                'w-2 h-2 rounded-full shrink-0',
                provider.enabled && providerEffectiveKeys(provider).length > 0 ? 'bg-ok' : 'bg-fg-muted',
              ]"
              aria-hidden="true"
            />
            <span class="text-xs font-semibold text-fg truncate">{{ provider.name || provider.id }}</span>
            <span class="font-mono text-xs px-1.5 py-0.5 rounded-lg border border-line text-fg-sub shrink-0">
              {{ provider.protocol === 'gemini' ? 'Gemini' : 'OpenAI 兼容' }}
            </span>
            <span
              v-if="providerEffectiveKeys(provider).length > 1"
              class="font-mono text-xs px-1.5 py-0.5 rounded-lg border border-line text-fg-sub shrink-0"
            >
              {{ providerEffectiveKeys(provider).length }} 密钥
            </span>
            <span
              v-if="providerAssignedCount(provider.id) > 0"
              class="font-mono text-xs px-1.5 py-0.5 rounded-lg border border-accent-border bg-accent-soft text-accent-fg shrink-0"
            >
              已入池 {{ providerAssignedCount(provider.id) }}
            </span>
          </button>

          <div class="flex items-center gap-1.5 shrink-0">
            <button
              type="button"
              role="switch"
              :aria-checked="provider.enabled"
              aria-label="启用该渠道"
              @click="updateProviderField(provider.id, { enabled: !provider.enabled })"
              :class="[
                'relative inline-flex h-5 w-9 shrink-0 items-center rounded-full border transition-colors cursor-pointer',
                provider.enabled ? 'border-accent-border bg-accent-soft' : 'border-line bg-canvas',
              ]"
            >
              <span
                :class="[
                  'inline-block h-3.5 w-3.5 rounded-full transition-transform',
                  provider.enabled ? 'translate-x-4 bg-accent' : 'translate-x-0.5 bg-fg-muted',
                ]"
              />
            </button>
            <button
              type="button"
              @click="expandedProviderId = expandedProviderId === provider.id ? '' : provider.id"
              class="min-h-[28px] px-2.5 rounded-lg border border-line text-xs text-fg-sub hover:border-accent-border hover:text-accent-fg whitespace-nowrap cursor-pointer"
            >
              {{ expandedProviderId === provider.id ? '收起' : '配置模型' }}
            </button>
            <button
              type="button"
              @click="removeProvider(provider)"
              class="min-h-[28px] px-2.5 rounded-lg border border-danger-border text-xs text-danger hover:bg-danger-soft whitespace-nowrap cursor-pointer"
            >
              删除
            </button>
          </div>
        </div>

        <div
          v-if="expandedProviderId === provider.id"
          class="border-t border-line px-3.5 py-3 flex flex-col gap-3"
        >
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-2.5">
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>渠道名称</span>
              <input
                :value="provider.name"
                @input="updateProviderField(provider.id, { name: ($event.target as HTMLInputElement).value })"
                type="text"
                placeholder="如：OpenRouter / DeepSeek"
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-2.5 py-1.5 text-xs text-fg focus:border-accent"
              />
            </label>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>接口协议</span>
              <select
                :value="provider.protocol"
                @change="updateProviderField(provider.id, { protocol: ($event.target as HTMLSelectElement).value })"
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-2.5 py-1.5 text-xs text-fg focus:border-accent"
              >
                <option value="openai">OpenAI 兼容 (/v1)</option>
                <option value="gemini">Google Gemini (AI Studio)</option>
              </select>
            </label>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>备注 (可选)</span>
              <input
                :value="provider.note || ''"
                @input="updateProviderField(provider.id, { note: ($event.target as HTMLInputElement).value })"
                type="text"
                placeholder="如：主力按量账号"
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-2.5 py-1.5 text-xs text-fg focus:border-accent"
              />
            </label>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>Base URL（自动清洗末尾 /chat/completions）</span>
              <input
                :value="provider.base_url"
                @input="updateProviderField(provider.id, { base_url: ($event.target as HTMLInputElement).value })"
                @blur="normalizeProviderUrl(provider)"
                type="text"
                placeholder="https://api.example.com/v1"
                class="min-h-[38px] rounded-xl border border-line bg-canvas px-2.5 py-1.5 font-mono text-xs text-fg focus:border-accent"
              />
            </label>
            <div class="flex flex-col gap-1 text-xs text-fg-sub">
              <div class="flex items-center justify-between gap-2">
                <span>API Keys（支持多行自动容灾切换）</span>
                <button
                  type="button"
                  @click="revealedKeyProviderId = revealedKeyProviderId === provider.id ? '' : provider.id"
                  class="text-[11px] text-accent-fg hover:underline cursor-pointer"
                >
                  {{ revealedKeyProviderId === provider.id ? '隐藏明文' : '多行编辑' }}
                </button>
              </div>
              <textarea
                v-if="revealedKeyProviderId === provider.id || providerEffectiveKeys(provider).length > 1"
                :value="providerKeysText(provider)"
                @change="updateProviderKeys(provider, ($event.target as HTMLTextAreaElement).value)"
                rows="3"
                spellcheck="false"
                placeholder="每行填入一个 Key"
                class="rounded-xl border border-line bg-canvas px-2.5 py-1.5 font-mono text-xs text-fg focus:border-accent resize-y"
              />
              <SecretInput
                v-else
                :model-value="provider.api_key"
                @update:model-value="updateProviderKeys(provider, $event)"
                :toggle-label="`${provider.name} API Key`"
                placeholder="sk-... / AIza..."
              />
            </div>
          </div>

          <!-- Model Discovery & Add to Pool -->
          <div class="rounded-xl border border-line bg-canvas p-2.5 flex flex-col gap-2.5">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <div class="flex items-center gap-3 flex-wrap">
                <span class="text-xs font-medium text-fg">发现并加入模型池</span>
                <label class="inline-flex items-center gap-1.5 text-[11px] text-fg-sub cursor-pointer select-none">
                  <input v-model="verifyOnFetch" type="checkbox" class="rounded border-line accent-accent" />
                  <span>实测过滤不可用模型</span>
                </label>
              </div>
              <button
                type="button"
                :disabled="fetchingProviderId === provider.id"
                @click="fetchFullModels(provider)"
                class="min-h-[30px] px-3 rounded-lg border border-accent-border bg-accent-soft text-xs font-medium text-accent-fg cursor-pointer disabled:opacity-50"
              >
                {{
                  fetchingProviderId === provider.id
                    ? '正在拉取列表...'
                    : fetchedByProvider[provider.id]
                    ? `刷新模型列表 (${fetchedByProvider[provider.id].models.length})`
                    : '拉取可用模型列表'
                }}
              </button>
            </div>

            <div class="flex items-center gap-1.5 flex-wrap">
              <input
                v-model="manualModelInput[provider.id]"
                type="text"
                placeholder="手动输入模型 ID（支持逗号分隔），如 anthropic/claude-sonnet-4, deepseek-chat"
                @keydown.enter.prevent="addManualModels(provider.id)"
                class="flex-1 min-w-[200px] min-h-[32px] rounded-lg border border-line bg-canvas px-2.5 text-xs font-mono text-fg placeholder:text-fg-muted focus:border-accent"
              />
              <button
                type="button"
                @click="addManualModels(provider.id)"
                class="min-h-[32px] px-3 rounded-lg border border-accent-border bg-accent-soft text-xs text-accent-fg font-medium cursor-pointer"
              >
                + 加入模型池
              </button>
            </div>

            <div v-if="fetchedByProvider[provider.id]" class="flex flex-col gap-2 pt-1 border-t border-line">
              <div class="flex items-center gap-2 flex-wrap">
                <input
                  v-model="fetchedByProvider[provider.id].query"
                  type="search"
                  placeholder="搜索模型名，如 claude / gpt / gemini / deepseek..."
                  class="flex-1 min-w-[180px] min-h-[30px] rounded-lg border border-line bg-canvas px-2.5 text-xs text-fg placeholder:text-fg-muted focus:border-accent"
                />
                <span class="font-mono text-xs text-fg-muted">
                  匹配 {{ filteredFetchedModels(provider.id).length }} / {{ fetchedByProvider[provider.id].models.length }}
                </span>
                <button
                  type="button"
                  @click="batchAddFiltered(provider.id, 12)"
                  class="min-h-[26px] px-2.5 rounded-lg border border-accent-border bg-accent-soft text-xs text-accent-fg cursor-pointer"
                >
                  前 12 个一键入池
                </button>
              </div>

              <div v-if="vendorTagsForProvider(provider.id).length > 0" class="flex items-center gap-1 flex-wrap">
                <button
                  type="button"
                  @click="fetchedByProvider[provider.id].filterTag = ''"
                  :class="[
                    'px-2 py-0.5 rounded-lg text-xs border cursor-pointer',
                    !fetchedByProvider[provider.id].filterTag
                      ? 'border-accent-border bg-accent-soft text-accent-fg'
                      : 'border-line text-fg-muted hover:text-fg',
                  ]"
                >
                  全部
                </button>
                <button
                  v-for="tag in vendorTagsForProvider(provider.id)"
                  :key="tag"
                  type="button"
                  @click="
                    fetchedByProvider[provider.id].filterTag =
                      fetchedByProvider[provider.id].filterTag === tag ? '' : tag
                  "
                  :class="[
                    'px-2 py-0.5 rounded-lg font-mono text-xs border cursor-pointer',
                    fetchedByProvider[provider.id].filterTag === tag
                      ? 'border-accent-border bg-accent-soft text-accent-fg'
                      : 'border-line text-fg-muted hover:text-fg',
                  ]"
                >
                  {{ tag }}
                </button>
              </div>

              <div class="max-h-56 overflow-y-auto pr-1 flex flex-col gap-1">
                <div
                  v-for="mid in filteredFetchedModels(provider.id).slice(0, 150)"
                  :key="mid"
                  class="flex items-center justify-between gap-2 rounded-lg border border-line bg-canvas px-2.5 py-1.5"
                >
                  <span class="font-mono text-xs text-fg truncate flex-1" :title="mid">{{ mid }}</span>
                  <button
                    type="button"
                    @click="toggleModelInPool(provider.id, mid)"
                    :class="[
                      'px-2.5 py-0.5 rounded-lg text-xs border transition cursor-pointer shrink-0',
                      isModelInPool(provider.id, mid)
                        ? 'border-accent-border bg-accent-soft text-accent-fg font-semibold'
                        : 'border-line text-fg-muted hover:border-accent-border hover:text-accent-fg',
                    ]"
                  >
                    {{ isModelInPool(provider.id, mid) ? '已在池中' : '+ 加入池' }}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- TAB 2: Unified Model Pool -->
    <div v-else class="flex flex-col gap-3">
      <div class="modal-section flex flex-col gap-2.5">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div>
            <div class="text-xs font-semibold text-fg">统一模型池 · 调度策略与健康探测</div>
            <p class="text-xs text-fg-muted mt-0.5">
              排在前面的主力模型优先响应对话；可给便宜快速的模型点亮「后台轻量」，专供记忆整合与对话压缩使用。
            </p>
          </div>
          <div class="flex items-center gap-1.5 shrink-0 flex-wrap">
            <button
              v-if="hasFailedProbe"
              type="button"
              @click="disableFailedModels"
              class="min-h-[30px] px-2.5 rounded-lg border border-danger-border bg-danger-soft text-xs text-danger cursor-pointer"
            >
              停用异常模型
            </button>
            <button
              v-if="hasCooling"
              type="button"
              :disabled="clearingCooldown"
              @click="clearCooldown('')"
              class="min-h-[30px] px-2.5 rounded-lg border border-warn-border bg-warn-soft text-xs text-warn cursor-pointer disabled:opacity-50"
            >
              全部解封冷却
            </button>
            <button
              type="button"
              :disabled="probingPool || orderedModels.length === 0"
              @click="probeEntirePool"
              class="min-h-[30px] px-3 rounded-lg border border-accent-border bg-accent-soft text-xs font-medium text-accent-fg cursor-pointer disabled:opacity-45"
            >
              {{ probingPool ? '正在测速...' : '一键测速本池' }}
            </button>
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-3 gap-1.5">
          <button
            v-for="opt in POOL_STRATEGY_OPTIONS"
            :key="opt.id"
            type="button"
            @click="emit('update:strategy', opt.id)"
            :class="[
              'rounded-xl border px-2.5 py-2 text-left transition cursor-pointer',
              strategy === opt.id
                ? 'border-accent-border bg-accent-soft text-fg'
                : 'border-line bg-canvas text-fg-muted hover:border-line-strong',
            ]"
          >
            <div class="text-xs font-semibold">{{ opt.label }}</div>
            <div class="text-xs opacity-80 mt-0.5 leading-snug">{{ opt.desc }}</div>
          </button>
        </div>
      </div>

      <div
        v-if="orderedModels.length === 0"
        class="rounded-xl border border-dashed border-line bg-canvas px-4 py-7 text-center flex flex-col items-center gap-2.5"
      >
        <div class="text-xs font-semibold text-fg">统一模型池暂无模型</div>
        <p class="text-xs text-fg-muted max-w-md leading-relaxed">
          前往「渠道管理」拉取模型列表或手动输入模型 ID，点击「+ 加入池」即可启用。
        </p>
        <button
          type="button"
          @click="subTab = 'providers'"
          class="min-h-[32px] px-3.5 rounded-xl border border-accent-border bg-accent-soft text-xs text-accent-fg cursor-pointer"
        >
          去渠道管理添加模型
        </button>
      </div>

      <div class="flex flex-col gap-2">
        <div
          v-for="(m, idx) in orderedModels"
          :key="canonicalModelTargetId(m.provider_id, m.model_id)"
          :class="[
            'rounded-xl border px-3 py-2.5 flex flex-col gap-2 transition-colors',
            m.enabled && providerMap.get(m.provider_id)?.enabled
              ? 'border-line bg-canvas'
              : 'border-line bg-canvas opacity-65',
          ]"
        >
          <div class="flex items-start justify-between gap-2 flex-wrap">
            <div class="min-w-0 flex-1 flex flex-col gap-1">
              <div class="flex items-center gap-1.5 flex-wrap">
                <span class="font-mono text-xs text-fg-muted">#{{ idx + 1 }}</span>
                <span class="text-xs px-1.5 py-0.5 rounded-lg border border-line bg-card text-fg">
                  {{ providerMap.get(m.provider_id)?.name || m.provider_id }}
                </span>
                <span class="font-mono text-xs font-semibold text-fg break-all">{{ m.model_id }}</span>
                <span
                  v-if="m.is_light"
                  class="text-[11px] px-1.5 py-0.5 rounded-lg border border-info-border bg-info-soft text-info"
                >
                  后台轻量
                </span>
              </div>

              <div class="flex items-center gap-1.5 flex-wrap text-xs">
                <span
                  v-if="m.supports_tools === true"
                  class="px-1.5 py-0.5 rounded-lg border border-ok-border bg-ok-soft text-ok"
                >
                  Tools 可用
                </span>
                <span
                  v-else-if="m.supports_tools === false"
                  class="px-1.5 py-0.5 rounded-lg border border-line bg-subtle text-fg-muted"
                >
                  不支持 Tools
                </span>
                <span
                  v-if="m.supports_thinking === true"
                  class="px-1.5 py-0.5 rounded-lg border border-info-border bg-info-soft text-info"
                >
                  Thinking
                </span>
                <span
                  v-if="(m.input_price_per_m || 0) > 0 || (m.output_price_per_m || 0) > 0"
                  class="font-mono px-1.5 py-0.5 rounded-lg border border-line text-fg-muted"
                >
                  ${{ m.input_price_per_m || 0 }}/${{ m.output_price_per_m || 0 }} /M
                </span>

                <template v-if="healthForModel(m)">
                  <span
                    v-if="(healthForModel(m)?.cooldown_sec || 0) > 0"
                    class="font-mono px-1.5 py-0.5 rounded-lg border border-warn-border bg-warn-soft text-warn"
                  >
                    冷却中 {{ healthForModel(m)?.cooldown_sec }}s
                  </span>
                  <span
                    v-else-if="(healthForModel(m)?.last_latency_ms || 0) > 0"
                    class="font-mono px-1.5 py-0.5 rounded-lg border border-ok-border bg-ok-soft text-ok"
                  >
                    正常 · {{ healthForModel(m)?.last_latency_ms }}ms
                  </span>
                </template>
                <span
                  v-else-if="m.last_latency_ms"
                  class="font-mono px-1.5 py-0.5 rounded-lg border border-line text-fg-sub"
                >
                  上次 {{ m.last_latency_ms }}ms
                </span>

                <span
                  v-if="probeMsgByKey[canonicalModelTargetId(m.provider_id, m.model_id)]"
                  :class="[
                    'font-mono text-xs',
                    probeMsgByKey[canonicalModelTargetId(m.provider_id, m.model_id)].ok
                      ? 'text-ok'
                      : 'text-danger',
                  ]"
                >
                  {{ probeMsgByKey[canonicalModelTargetId(m.provider_id, m.model_id)].msg }}
                </span>
              </div>
            </div>

            <div class="flex items-center gap-1 flex-wrap shrink-0">
              <button
                type="button"
                @click="updatePooledModel(m, { is_light: !m.is_light })"
                :title="m.is_light ? '取消后台轻量标记' : '标记为后台轻量模型（记忆/摘要/压缩优先使用）'"
                :class="[
                  'min-h-[28px] px-2 rounded-lg border text-xs cursor-pointer',
                  m.is_light
                    ? 'border-info-border bg-info-soft text-info font-medium'
                    : 'border-line text-fg-sub hover:border-info-border hover:text-info',
                ]"
              >
                轻量
              </button>
              <button
                type="button"
                @click="
                  editingPriceKey =
                    editingPriceKey === canonicalModelTargetId(m.provider_id, m.model_id)
                      ? ''
                      : canonicalModelTargetId(m.provider_id, m.model_id)
                "
                class="min-h-[28px] px-2 rounded-lg border border-line text-xs text-fg-sub hover:border-accent-border hover:text-accent-fg cursor-pointer"
              >
                单价
              </button>
              <button
                type="button"
                :disabled="probingKey === canonicalModelTargetId(m.provider_id, m.model_id)"
                @click="probeSingleModel(m)"
                class="min-h-[28px] px-2 rounded-lg border border-line hover:border-accent-border text-xs text-fg hover:text-accent-fg cursor-pointer disabled:opacity-50"
              >
                {{ probingKey === canonicalModelTargetId(m.provider_id, m.model_id) ? '测速中...' : '测速' }}
              </button>
              <button
                v-if="(healthForModel(m)?.cooldown_sec || 0) > 0"
                type="button"
                @click="clearCooldown(canonicalModelTargetId(m.provider_id, m.model_id))"
                class="min-h-[28px] px-2 rounded-lg border border-warn-border bg-warn-soft text-xs text-warn cursor-pointer"
              >
                解封
              </button>
              <button
                type="button"
                :disabled="idx === 0"
                @click="movePooledModel(idx, -1)"
                aria-label="上移优先级"
                class="min-h-[28px] px-1.5 rounded-lg border border-line text-xs text-fg-sub hover:border-accent-border disabled:opacity-30 cursor-pointer"
              >
                ↑
              </button>
              <button
                type="button"
                :disabled="idx === orderedModels.length - 1"
                @click="movePooledModel(idx, 1)"
                aria-label="下移优先级"
                class="min-h-[28px] px-1.5 rounded-lg border border-line text-xs text-fg-sub hover:border-accent-border disabled:opacity-30 cursor-pointer"
              >
                ↓
              </button>
              <button
                type="button"
                role="switch"
                :aria-checked="m.enabled"
                aria-label="启停该模型"
                @click="updatePooledModel(m, { enabled: !m.enabled })"
                :class="[
                  'relative inline-flex h-5 w-9 shrink-0 items-center rounded-full border transition-colors cursor-pointer mx-0.5',
                  m.enabled ? 'border-accent-border bg-accent-soft' : 'border-line bg-canvas',
                ]"
              >
                <span
                  :class="[
                    'inline-block h-3.5 w-3.5 rounded-full transition-transform',
                    m.enabled ? 'translate-x-4 bg-accent' : 'translate-x-0.5 bg-fg-muted',
                  ]"
                />
              </button>
              <button
                type="button"
                @click="removePooledModel(m)"
                aria-label="移出模型池"
                class="min-h-[28px] px-2 rounded-lg border border-danger-border text-xs text-danger hover:bg-danger-soft whitespace-nowrap cursor-pointer"
              >
                移除
              </button>
            </div>
          </div>

          <!-- Optional Per-Model Custom Price Override (USD per 1M tokens) -->
          <div
            v-if="editingPriceKey === canonicalModelTargetId(m.provider_id, m.model_id)"
            class="pt-2 border-t border-line grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs text-fg-sub"
          >
            <label class="flex items-center gap-2">
              <span class="shrink-0">输入单价 ($/1M)</span>
              <input
                type="number"
                step="0.01"
                min="0"
                :value="m.input_price_per_m || 0"
                @input="updatePooledModel(m, { input_price_per_m: Math.max(0, Number(($event.target as HTMLInputElement).value) || 0) })"
                placeholder="0 表示用内置价目表"
                class="flex-1 min-h-[30px] rounded-lg border border-line bg-card px-2 font-mono text-xs text-fg"
              />
            </label>
            <label class="flex items-center gap-2">
              <span class="shrink-0">输出单价 ($/1M)</span>
              <input
                type="number"
                step="0.01"
                min="0"
                :value="m.output_price_per_m || 0"
                @input="updatePooledModel(m, { output_price_per_m: Math.max(0, Number(($event.target as HTMLInputElement).value) || 0) })"
                placeholder="0 表示用内置价目表"
                class="flex-1 min-h-[30px] rounded-lg border border-line bg-card px-2 font-mono text-xs text-fg"
              />
            </label>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
