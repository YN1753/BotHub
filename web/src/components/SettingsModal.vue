<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { lazyPanelChunk } from '../utils/lazyComponent'
import { resolveTabSlideTransition, type DirectionalTransitionName } from '../utils/motion'
import { checkPasskeyCapabilities, registerPasskeyCeremony } from '../utils/passkey'
import { useAppTheme } from '../utils/theme'
import type { ModelPoolEntry, UpstreamModelCheck } from '../types/bothub'
import BaseModal from './BaseModal.vue'
import SegmentedTabs from './SegmentedTabs.vue'
import SecretInput from './ui/SecretInput.vue'

const SettingsSecurityTab = lazyPanelChunk(() => import('./settings/SettingsSecurityTab.vue'))
const ModelPoolSection = lazyPanelChunk(() => import('./settings/ModelPoolSection.vue'))

const store = useAppStore()
const { themeId, themeOptions, setAppTheme } = useAppTheme()
const activeTab = ref<'ai' | 'push' | 'integrations' | 'security'>('ai')
const settingsTabTransition = ref<DirectionalTransitionName>('vesper-slide-forward')

watch(activeTab, (next, prev) => {
  settingsTabTransition.value = resolveTabSlideTransition(
    next,
    prev || 'ai',
    ['ai', 'push', 'integrations', 'security']
  )
})

const SETTINGS_TABS = [
  { id: 'ai', label: 'AI 大脑设置' },
  { id: 'push', label: '原生 APP 通知' },
  { id: 'integrations', label: '巡检与探针凭据' },
  { id: 'security', label: '安全与数据灾备' },
]

// Config fields (clean blank defaults, no hardcoded presets)
const deepseekKey = ref('')
const deepseekBaseUrl = ref('')
const aiModel = ref('')
const poolForLight = ref(true)
const poolFallback = ref(true)
const poolEntries = ref<ModelPoolEntry[]>([])
const probingId = ref('')
const probeResults = ref<Record<string, { ok: boolean; msg: string }>>({})
const fetchingModelsId = ref('')
const verifyOnFetch = ref(true)
const fetchedModelMeta = ref<
  Record<
    string,
    {
      usable: UpstreamModelCheck[]
      failed: UpstreamModelCheck[]
      filteredOutCount: number
      unverifiedCount: number
      showFailed: boolean
      query: string
    }
  >
>({})

const lightBaseUrl = ref('')
const lightApiKey = ref('')
const lightModel = ref('')

const nezhaUrl = ref('')
const nezhaDbPath = ref('')
const cfApiToken = ref('')
const biliSessdata = ref('')

const saving = ref(false)
const testingPush = ref(false)
const pairingApp = ref(false)
const registeringPasskey = ref(false)
const passkeyCapable = ref(false)
const passkeyCount = ref(0)
const savedSnapshot = ref('')

function currentConfigSnapshot(): string {
  return JSON.stringify({
    deepseekKey: deepseekKey.value.trim(),
    deepseekBaseUrl: deepseekBaseUrl.value.trim(),
    aiModel: aiModel.value.trim(),
    poolForLight: poolForLight.value,
    poolFallback: poolFallback.value,
    poolEntries: poolEntries.value,
    lightBaseUrl: lightBaseUrl.value.trim(),
    lightApiKey: lightApiKey.value.trim(),
    lightModel: lightModel.value.trim(),
    nezhaUrl: nezhaUrl.value.trim(),
    nezhaDbPath: nezhaDbPath.value.trim(),
    cfApiToken: cfApiToken.value.trim(),
    biliSessdata: biliSessdata.value.trim(),
  })
}

const hasUnsavedChanges = computed(() =>
  Boolean(savedSnapshot.value) && currentConfigSnapshot() !== savedSnapshot.value
)

watch(
  () => store.state.showSettingsModal,
  (open) => {
    if (!open) return
    const c = store.state.configs || {}
    deepseekKey.value = c['deepseek_api_key'] || ''
    deepseekBaseUrl.value = c['deepseek_base_url'] || ''
    aiModel.value = c['ai_model'] || ''
    poolForLight.value = c['pool_for_light'] !== 'false'
    poolFallback.value = c['pool_fallback'] !== 'false'
    poolEntries.value = (store.state.modelPool || []).map((x) => ({ ...x }))
    probeResults.value = {}
    fetchedModelMeta.value = {}

    lightBaseUrl.value = c['light_base_url'] || ''
    lightApiKey.value = c['light_api_key'] || ''
    lightModel.value = c['light_model'] || ''

    nezhaUrl.value = c['nezha_url'] || ''
    nezhaDbPath.value = c['nezha_db_path'] || ''
    cfApiToken.value = c['cf_api_token'] || ''
    biliSessdata.value = c['bili_sessdata'] || ''
    savedSnapshot.value = currentConfigSnapshot()
    void checkPasskeyCapabilities().then((caps) => {
      passkeyCapable.value = caps.platformSupported
    })
    void BotHubApi.passkeyStatus()
      .then((st) => {
        passkeyCount.value = st.count || 0
      })
      .catch(() => {})
  }
)

function closeModal() {
  store.state.showSettingsModal = false
}

async function saveAllConfig() {
  saving.value = true
  try {
    await BotHubApi.saveConfig({
      deepseek_api_key: deepseekKey.value.trim(),
      deepseek_base_url: deepseekBaseUrl.value.trim(),
      ai_model: aiModel.value.trim(),
      pool_for_light: poolForLight.value ? 'true' : 'false',
      pool_fallback: poolFallback.value ? 'true' : 'false',
      light_base_url: lightBaseUrl.value.trim(),
      light_api_key: lightApiKey.value.trim(),
      light_model: lightModel.value.trim(),
      nezha_url: nezhaUrl.value.trim(),
      nezha_db_path: nezhaDbPath.value.trim(),
      cf_api_token: cfApiToken.value.trim(),
      bili_sessdata: biliSessdata.value.trim(),
    })
    const normalized = poolEntries.value.map((e, i) => ({
      ...e,
      priority: (i + 1) * 10,
    }))
    await BotHubApi.saveModelPool(normalized)
    store.notify('AI 引擎、免费模型池与凭据已安全保存')
    await store.loadDashboard(false)
    poolEntries.value = (store.state.modelPool || []).map((x) => ({ ...x }))
    savedSnapshot.value = currentConfigSnapshot()
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    saving.value = false
  }
}

function healthForTarget(key: string) {
  return store.state.modelHealth?.[key]
}

function healthBadgeText(key: string): string {
  const h = healthForTarget(key)
  if (!h) return ''
  if (h.cooldown_sec > 0) return `冷却中 ${h.cooldown_sec}s`
  if (h.success_count > 0) return `成功 ${h.success_count} · ${h.last_latency_ms}ms`
  if (h.fail_count > 0) return `异常 (${h.fail_count})`
  return ''
}

async function probeTarget(targetId: string, provider: string, baseUrl: string, apiKey: string, modelId: string) {
  if (probingId.value) return
  if (!modelId.trim()) {
    store.notify('请先拉取可用模型并选中或填写模型 ID')
    return
  }
  probingId.value = targetId
  delete probeResults.value[targetId]
  try {
    const res = await BotHubApi.testModelTarget({
      target_id: targetId,
      provider,
      base_url: baseUrl.trim(),
      api_key: apiKey.trim(),
      model: modelId.trim(),
    })
    probeResults.value[targetId] = { ok: true, msg: res.message || `${res.latency_ms}ms OK` }
    if (res.model_health) {
      store.state.modelHealth = res.model_health
    }
  } catch (err: any) {
    probeResults.value[targetId] = { ok: false, msg: err.message || '连接失败' }
  } finally {
    probingId.value = ''
  }
}

async function fetchModelsForTarget(
  targetId: string,
  provider: string,
  baseUrl: string,
  apiKey: string,
  entry?: ModelPoolEntry
) {
  if (fetchingModelsId.value) return
  if (!baseUrl.trim()) {
    store.notify('请先填写 Base URL 再拉取实时模型列表')
    return
  }
  fetchingModelsId.value = targetId
  try {
    const res = await BotHubApi.fetchUpstreamModels({
      target_id: targetId,
      provider,
      base_url: baseUrl.trim(),
      api_key: apiKey.trim(),
      verify: verifyOnFetch.value,
    })
    const checked = res.checked_models || (res.models || []).map((id) => ({ id, usable: true }))
    const usable = checked.filter((c) => c.usable)
    const failed = checked.filter((c) => !c.usable)
    fetchedModelMeta.value[targetId] = {
      usable,
      failed,
      filteredOutCount: res.filtered_out_count || 0,
      unverifiedCount: res.unverified_count || 0,
      showFailed: usable.length === 0 && failed.length > 0,
      query: '',
    }
    if (usable.length > 0) {
      if (targetId === 'primary' && !aiModel.value.trim()) {
        aiModel.value = usable[0].id
      } else if (entry && !entry.model.trim()) {
        entry.model = usable[0].id
      }
      const skipInfo =
        failed.length > 0 || (res.filtered_out_count || 0) > 0
          ? `（已自动剔除 ${failed.length + (res.filtered_out_count || 0)} 个不可用/非对话模型）`
          : ''
      store.notify(`已筛选出 ${usable.length} 个实测可用模型${skipInfo}`)
    } else if (failed.length > 0) {
      store.notify(`已拉取列表但实测均不可用 (${failed.length} 个)，可展开下方查看原因`)
    } else {
      store.notify('上游接口未返回可用对话模型')
    }
  } catch (err: any) {
    store.notify(`拉取模型失败: ${err.message}`)
  } finally {
    fetchingModelsId.value = ''
  }
}

function filteredUsableChecks(targetId: string): UpstreamModelCheck[] {
  const meta = fetchedModelMeta.value[targetId]
  if (!meta) return []
  const q = meta.query.trim().toLowerCase()
  if (!q) return meta.usable
  return meta.usable.filter((c) => c.id.toLowerCase().includes(q))
}

async function handleTestPush() {
  if (testingPush.value) return
  testingPush.value = true
  try {
    const res = await BotHubApi.testPush()
    store.notify(res.message || '测试消息已推送')
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    testingPush.value = false
  }
}

async function handleOneTapPairApp() {
  if (pairingApp.value) return
  pairingApp.value = true
  try {
    const res = await BotHubApi.createAppPairCode()
    store.notify('正在唤醒手机客户端并自动完成免密绑定...')
    window.location.href = res.deep_link
  } catch (err: any) {
    store.notify(err.message || '发起一键绑定失败')
  } finally {
    pairingApp.value = false
  }
}

async function handleQuickCreatePasskey() {
  if (registeringPasskey.value) return
  registeringPasskey.value = true
  try {
    const res = await registerPasskeyCeremony({ promotion: false })
    if (res.ok) {
      passkeyCount.value += 1
      store.notify('已绑定面容 / 指纹通行密钥，手机与网页端均可直接刷脸或指纹连接')
    } else if (!res.cancelled && res.error) {
      store.notify(res.error)
    }
  } finally {
    registeringPasskey.value = false
  }
}
</script>

<template>
  <BaseModal
    :open="store.state.showSettingsModal"
    title="系统设置与全局凭据"
    subtitle="集中管理 AI 大脑、消息通道、巡检密钥与会话安全"
    max-width="2xl"
    @close="closeModal"
  >
    <div class="border-b border-[#fff8ee]/10 pb-3 shrink-0 flex flex-col gap-2.5">
      <div class="flex items-center justify-between gap-2 flex-wrap">
        <span class="text-[11px] font-medium text-slate-400 shrink-0">界面主题 · 弹窗统一换肤</span>
        <div class="flex items-center gap-1.5 flex-wrap" role="radiogroup" aria-label="界面外观主题">
          <button
            v-for="opt in themeOptions"
            :key="opt.id"
            type="button"
            role="radio"
            :aria-checked="themeId === opt.id"
            :title="opt.desc"
            @click="setAppTheme(opt.id)"
            :class="[
              'inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] font-medium border transition cursor-pointer',
              themeId === opt.id
                ? 'border-[#f2b880]/55 bg-[#f2b880]/15 text-[#f2b880]'
                : 'border-[#fff8ee]/12 text-slate-300 hover:border-[#f2b880]/35'
            ]"
          >
            <span
              class="w-3 h-3 rounded-full border border-white/25 inline-flex items-center justify-center shrink-0"
              :style="{ backgroundColor: opt.swatchBg }"
              aria-hidden="true"
            >
              <span class="w-1.5 h-1.5 rounded-full" :style="{ backgroundColor: opt.swatchAccent }" />
            </span>
            <span>{{ opt.label }}</span>
          </button>
        </div>
      </div>
      <SegmentedTabs v-model="activeTab" :items="SETTINGS_TABS" aria-label="系统设置分类" />
    </div>

    <div class="flex-1 min-h-0 overflow-y-auto overflow-x-hidden pr-1">
      <Transition :name="settingsTabTransition" mode="out-in">
        <!-- Tab: AI & Free Model Pool -->
        <form v-if="activeTab === 'ai'" key="ai" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
          <!-- Primary: DeepSeek Flash + Relay Station Support -->
          <div class="modal-section flex flex-col gap-3">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <div>
                <div class="text-xs font-semibold text-[#f2b880]">主力引擎 (DeepSeek / 自定义中转站)</div>
                <p class="text-[11px] text-slate-400 mt-0.5">支持 DeepSeek 官方或任何 OpenAI 兼容中转站；点击下方实时拉取并自动验证真实可用的对话模型</p>
              </div>
              <button
                type="button"
                :disabled="probingId === 'primary'"
                @click="probeTarget('primary', 'deepseek', deepseekBaseUrl, deepseekKey, aiModel)"
                class="min-h-[32px] px-3 rounded-lg border border-[#f2b880]/40 bg-[#f2b880]/12 hover:bg-[#f2b880]/20 text-[11px] font-medium text-[#f2b880] cursor-pointer disabled:opacity-45 shrink-0"
              >
                {{ probingId === 'primary' ? '测速中...' : '测试主力连通' }}
              </button>
            </div>

            <div
              v-if="probeResults.primary"
              :class="[
                'text-[11px] font-mono px-2.5 py-1.5 rounded-lg border',
                probeResults.primary.ok
                  ? 'border-emerald-400/30 bg-emerald-400/10 text-emerald-300'
                  : 'border-rose-400/35 bg-rose-500/10 text-rose-300'
              ]"
            >
              {{ probeResults.primary.msg }}
            </div>

            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <div class="flex items-center justify-between gap-2">
                <span>主力 API Key (官方或中转站令牌)</span>
                <span
                  :class="[
                    'font-mono text-[10px] px-1.5 py-0.5 rounded border',
                    deepseekKey.trim()
                      ? 'border-emerald-400/35 bg-emerald-500/10 text-emerald-300'
                      : 'border-amber-400/35 bg-amber-500/10 text-amber-300'
                  ]"
                >
                  {{ deepseekKey.trim() ? 'Key 已填入' : '未配置主力 Key' }}
                </span>
              </div>
              <SecretInput
                v-model="deepseekKey"
                placeholder="输入 API Key"
                toggle-label="主力 API Key"
              />
            </label>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <label class="flex flex-col gap-1 text-xs text-slate-300">
                <div class="flex items-center justify-between gap-2">
                  <span>API 地址 (OpenAI 兼容 Base URL)</span>
                  <button
                    v-if="/\/chat\/completions\/?$/i.test(deepseekBaseUrl.trim())"
                    type="button"
                    @click="deepseekBaseUrl = deepseekBaseUrl.trim().replace(/\/chat\/completions\/?$/i, '')"
                    class="text-[10.5px] text-amber-300 hover:underline cursor-pointer"
                  >
                    去掉多余 /chat/completions
                  </button>
                </div>
                <input v-model="deepseekBaseUrl" type="text" placeholder="填写 Base URL，如 https://.../v1" class="min-h-[42px] sm:min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-xs text-white focus:border-[#f2b880]/60" />
              </label>
              <div class="flex flex-col gap-1 text-xs text-slate-300">
                <div class="flex items-center justify-between gap-2">
                  <span>主力模型 ID (实时拉取或手填)</span>
                  <button
                    type="button"
                    :disabled="fetchingModelsId === 'primary'"
                    @click="fetchModelsForTarget('primary', 'deepseek', deepseekBaseUrl, deepseekKey)"
                    class="text-[10.5px] text-[#f2b880] hover:underline cursor-pointer disabled:opacity-45"
                  >
                    {{ fetchingModelsId === 'primary' ? '探测可用模型中...' : verifyOnFetch ? '拉取并筛选可用模型' : '仅拉取清单' }}
                  </button>
                </div>
                <input v-model="aiModel" type="text" placeholder="点击上方自动探测可用模型或手动输入" class="min-h-[42px] sm:min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-xs text-white focus:border-[#f2b880]/60" />
              </div>
            </div>
            <div v-if="fetchedModelMeta.primary" class="flex flex-col gap-2 rounded-xl border border-[#f2b880]/20 bg-[#050811]/70 p-2.5">
              <div class="flex items-center justify-between gap-2 flex-wrap">
                <span class="text-[10.5px] text-slate-300">
                  {{ verifyOnFetch ? '实测可用模型' : '对话模型候选' }} ({{ filteredUsableChecks('primary').length }})
                  <template v-if="fetchedModelMeta.primary.filteredOutCount > 0">
                    · 已过滤 {{ fetchedModelMeta.primary.filteredOutCount }} 个非对话模型
                  </template>
                </span>
                <input
                  v-if="fetchedModelMeta.primary.usable.length > 6"
                  v-model="fetchedModelMeta.primary.query"
                  type="search"
                  placeholder="搜索模型..."
                  class="w-32 bg-[#070b18] border border-[#fff8ee]/12 rounded-md px-2 py-0.5 text-[10.5px] text-white focus:border-[#f2b880]/50 focus:outline-none"
                />
              </div>
              <div v-if="filteredUsableChecks('primary').length > 0" class="flex flex-wrap gap-1.5 max-h-32 overflow-y-auto">
                <button
                  v-for="item in filteredUsableChecks('primary')"
                  :key="item.id"
                  type="button"
                  @click="aiModel = item.id"
                  :class="[
                    'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md border font-mono text-[10.5px] cursor-pointer transition',
                    aiModel === item.id
                      ? 'border-[#f2b880] bg-[#f2b880]/20 text-[#f2b880]'
                      : 'border-[#fff8ee]/12 bg-[#070b18] text-slate-300 hover:border-[#f2b880]/45'
                  ]"
                >
                  <span>{{ item.id }}</span>
                  <span v-if="item.latency_ms" class="text-[9.5px] text-emerald-300/80">{{ item.latency_ms }}ms</span>
                </button>
              </div>
              <div v-if="fetchedModelMeta.primary.failed.length > 0" class="pt-1 border-t border-[#fff8ee]/8">
                <button
                  type="button"
                  @click="fetchedModelMeta.primary.showFailed = !fetchedModelMeta.primary.showFailed"
                  class="text-[10px] text-slate-400 hover:text-slate-200 cursor-pointer"
                >
                  {{ fetchedModelMeta.primary.showFailed ? '收起不可用模型' : `查看被排除的不可用模型 (${fetchedModelMeta.primary.failed.length})` }}
                </button>
                <div v-if="fetchedModelMeta.primary.showFailed" class="mt-1.5 flex flex-col gap-1 max-h-24 overflow-y-auto">
                  <div v-for="bad in fetchedModelMeta.primary.failed" :key="bad.id" class="flex items-center justify-between gap-2 text-[10px] font-mono text-rose-300/80">
                    <span class="truncate">{{ bad.id }}</span>
                    <span class="shrink-0 text-slate-400">{{ bad.reason }}</span>
                  </div>
                </div>
              </div>
            </div>
            <div class="pt-3 border-t border-[#fff8ee]/8 flex flex-col gap-2.5">
              <div class="flex items-center justify-between gap-2 flex-wrap">
                <div class="flex items-center gap-2">
                  <span class="text-[11px] font-medium text-slate-300">后台记忆提炼与轻量任务模型</span>
                  <span class="font-mono text-[9.5px] px-1.5 py-0.5 rounded border border-[#fff8ee]/12 text-slate-400">
                    {{ lightModel.trim() || lightBaseUrl.trim() ? '独立节点覆盖' : '自动复用免费池/主力' }}
                  </span>
                </div>
                <button
                  type="button"
                  :disabled="fetchingModelsId === 'light'"
                  @click="fetchModelsForTarget('light', 'openai', lightBaseUrl.trim() || deepseekBaseUrl.trim(), lightApiKey.trim() || deepseekKey.trim())"
                  class="text-[10.5px] text-[#f2b880] hover:underline cursor-pointer disabled:opacity-45 shrink-0"
                >
                  {{ fetchingModelsId === 'light' ? '探测可用模型中...' : '拉取轻量可用模型' }}
                </button>
              </div>
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-2.5">
                <label class="flex flex-col gap-1 text-xs text-slate-300">
                  <span>轻量 Base URL</span>
                  <input v-model="lightBaseUrl" type="text" placeholder="留空复用主力地址" class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-1.5 font-mono text-xs text-white focus:border-[#f2b880]/60" />
                </label>
                <label class="flex flex-col gap-1 text-xs text-slate-300">
                  <span>轻量 API Key</span>
                  <SecretInput
                    v-model="lightApiKey"
                    size="sm"
                    placeholder="留空自动走免费池"
                    toggle-label="轻量 API Key"
                  />
                </label>
                <label class="flex flex-col gap-1 text-xs text-slate-300">
                  <span>轻量模型 ID</span>
                  <input v-model="lightModel" type="text" placeholder="拉取或输入模型 ID" class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-1.5 font-mono text-xs text-white focus:border-[#f2b880]/60" />
                </label>
              </div>
              <div v-if="fetchedModelMeta.light && filteredUsableChecks('light').length > 0" class="flex flex-wrap gap-1.5 max-h-24 overflow-y-auto p-2 rounded-xl border border-[#f2b880]/20 bg-[#050811]/70">
                <button
                  v-for="item in filteredUsableChecks('light')"
                  :key="item.id"
                  type="button"
                  @click="lightModel = item.id"
                  :class="[
                    'inline-flex items-center gap-1 px-2 py-0.5 rounded-md border font-mono text-[10.5px] cursor-pointer',
                    lightModel === item.id
                      ? 'border-[#f2b880] bg-[#f2b880]/20 text-[#f2b880]'
                      : 'border-[#fff8ee]/12 bg-[#070b18] text-slate-300 hover:border-[#f2b880]/45'
                  ]"
                >
                  <span>{{ item.id }}</span>
                </button>
              </div>
            </div>
          </div>

          <!-- Free AI Model Pool -->
          <ModelPoolSection
            v-model:pool-for-light="poolForLight"
            v-model:pool-fallback="poolFallback"
            v-model:verify-on-fetch="verifyOnFetch"
            v-model:pool-entries="poolEntries"
            :probing-id="probingId"
            :probe-results="probeResults"
            :fetching-models-id="fetchingModelsId"
            :fetched-model-meta="fetchedModelMeta"
            :health-for-target="healthForTarget"
            :health-badge-text="healthBadgeText"
            :filtered-usable-checks="filteredUsableChecks"
            :probe-target="probeTarget"
            :fetch-models-for-target="fetchModelsForTarget"
          />

          <div class="flex justify-end pt-1">
            <button type="submit" :disabled="saving" class="min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer">
              {{ saving ? '保存中...' : '保存 AI 引擎与免费模型池' }}
            </button>
          </div>
        </form>

        <!-- Tab: Native App Push (Zero Manual Configuration) -->
        <div v-else-if="activeTab === 'push'" key="push" class="flex flex-col gap-3.5">
          <div class="luxury-card rounded-2xl p-4 border-[#f2b880]/28">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <div class="flex items-center gap-2">
                <span :class="['w-2 h-2 rounded-full shrink-0', store.state.sseConnected ? 'bg-emerald-400 animate-pulse' : 'bg-amber-400']" aria-hidden="true" />
                <strong class="text-sm font-semibold text-white">手机原生通知（零配置自动就绪）</strong>
              </div>
              <span :class="['font-mono text-[10.5px] px-2 py-0.5 rounded-full border', store.state.sseConnected ? 'border-emerald-400/30 bg-emerald-400/10 text-emerald-300' : 'border-amber-400/30 bg-amber-400/10 text-amber-300']">
                {{ store.state.sseConnected ? 'SSE 实时通道在线' : 'SSE 等待重连' }}
              </span>
            </div>
            <p class="text-xs text-slate-300/90 mt-1.5 leading-relaxed">
              服务端已自动维护加密推送凭证，无需手动填写任何 Token。定时晨报、技术雷达、资产到期提醒与哪吒探针告警均自动直推手机锁屏。
            </p>
          </div>

          <div class="modal-section flex flex-col gap-3">
            <div>
              <div class="text-xs font-semibold text-[#f2b880]">无感面容 / 指纹激活手机客户端</div>
              <p class="text-[11px] text-slate-400 mt-0.5">
                方式一：注册面容/指纹通行密钥后，手机安装 APP 打开即可无感刷脸（或按指纹）直连；方式二：在手机浏览器直接点下方「一键免密唤醒绑定本机 APP」，无需输入网址或密码。
              </p>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
              <button
                type="button"
                :disabled="pairingApp"
                @click="handleOneTapPairApp"
                class="btn-jewel min-h-[44px] px-4 py-2.5 rounded-xl text-xs font-semibold cursor-pointer disabled:opacity-50"
              >
                {{ pairingApp ? '正在唤醒客户端...' : '一键免密唤醒绑定本机 APP' }}
              </button>

              <button
                v-if="passkeyCapable"
                type="button"
                :disabled="registeringPasskey"
                @click="handleQuickCreatePasskey"
                class="min-h-[44px] px-4 py-2.5 rounded-xl border border-[#f2b880]/45 bg-[#f2b880]/12 hover:bg-[#f2b880]/22 text-xs font-medium text-[#f2b880] cursor-pointer disabled:opacity-50"
              >
                {{
                  registeringPasskey
                    ? '正在调起生物识别...'
                    : passkeyCount > 0
                    ? `新增面容/指纹密钥 (已绑 ${passkeyCount} 个)`
                    : '绑定面容 / 指纹通行密钥'
                }}
              </button>
            </div>
          </div>

          <div class="flex items-center justify-between gap-2 flex-wrap pt-1">
            <a
              href="https://github.com/YN1753/BotHub/releases/latest/download/vesper-bothub-android.apk"
              target="_blank"
              rel="noopener noreferrer"
              class="min-h-[44px] sm:min-h-[36px] px-3.5 py-2 rounded-xl border border-[#fff8ee]/16 text-xs text-slate-200 hover:text-[#f2b880] hover:border-[#f2b880]/40 inline-flex items-center justify-center"
            >
              下载最新 Android 客户端 APK
            </a>
            <button
              type="button"
              :disabled="testingPush"
              @click="handleTestPush"
              class="min-h-[44px] sm:min-h-[36px] px-4 py-2 rounded-xl border border-[#f2b880]/40 bg-[#f2b880]/12 hover:bg-[#f2b880]/20 text-xs font-medium text-[#f2b880] cursor-pointer disabled:opacity-50"
            >
              {{ testingPush ? '正在推送测试...' : '发送测试通知到手机' }}
            </button>
          </div>
        </div>

        <!-- Tab: Integrations -->
        <form v-else-if="activeTab === 'integrations'" key="integrations" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
          <div class="modal-section flex flex-col gap-3">
            <div class="text-xs font-semibold text-[#f2b880]">服务器探针直连 (哪吒监控)</div>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>哪吒监控面板地址</span>
              <input v-model="nezhaUrl" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>哪吒 SQLite 本机路径 (只读直连，可选)</span>
              <input v-model="nezhaDbPath" type="text" placeholder="留空仅走网页/资产查询，填实际 .db 路径可直连探针" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            </label>
          </div>

          <div class="modal-section flex flex-col gap-3">
            <div class="text-xs font-semibold text-[#f2b880]">域名与生态扩展凭据</div>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>Cloudflare API Token</span>
              <SecretInput v-model="cfApiToken" toggle-label="Cloudflare API Token" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>哔哩哔哩 SESSDATA</span>
              <SecretInput v-model="biliSessdata" toggle-label="哔哩哔哩 SESSDATA" />
            </label>
          </div>

          <div class="flex justify-end pt-1">
            <button type="submit" :disabled="saving" class="min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer">
              保存巡检凭据
            </button>
          </div>
        </form>

        <!-- Tab: Security & Backup -->
        <SettingsSecurityTab v-else key="security" @close="closeModal" />
      </Transition>
    </div>

    <div
      v-if="hasUnsavedChanges && activeTab !== 'security'"
      class="shrink-0 flex items-center justify-between gap-3 rounded-xl border border-[#f2b880]/40 bg-[#f2b880]/10 px-3.5 py-2.5 text-xs"
    >
      <div class="flex items-center gap-2 min-w-0">
        <span class="w-2 h-2 rounded-full bg-[#f2b880] animate-pulse shrink-0" aria-hidden="true" />
        <span class="text-[#f7cba0] truncate">检测到配置改动，记得保存后生效</span>
      </div>
      <button
        type="button"
        :disabled="saving"
        @click="saveAllConfig"
        class="min-h-[34px] px-4 rounded-lg bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer shrink-0 disabled:opacity-50"
      >
        {{ saving ? '保存中...' : '立即保存全部改动' }}
      </button>
    </div>
  </BaseModal>
</template>
