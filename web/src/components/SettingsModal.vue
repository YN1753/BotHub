<script setup lang="ts">
import { ref, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import type { ModelPoolEntry } from '../types/bothub'
import BaseModal from './BaseModal.vue'
import CustomSelect from './CustomSelect.vue'
import SegmentedTabs from './SegmentedTabs.vue'
import ToggleSwitch from './ToggleSwitch.vue'

const store = useAppStore()
const activeTab = ref<'ai' | 'push' | 'integrations' | 'security'>('ai')

const SETTINGS_TABS = [
  { id: 'ai', label: 'AI 大脑设置' },
  { id: 'push', label: '原生 APP 通知' },
  { id: 'integrations', label: '巡检与探针凭据' },
  { id: 'security', label: '安全与数据灾备' },
]

// Config fields
const deepseekKey = ref('')
const deepseekBaseUrl = ref('https://api.deepseek.com/v1')
const aiModel = ref('deepseek-flash')
const activePersona = ref('grok')
const poolForLight = ref(true)
const poolFallback = ref(true)
const poolEntries = ref<ModelPoolEntry[]>([])
const expandedPoolId = ref('')
const probingId = ref('')
const probeResults = ref<Record<string, { ok: boolean; msg: string }>>({})

const lightBaseUrl = ref('')
const lightApiKey = ref('')
const lightModel = ref('')

const nezhaUrl = ref('https://dash.starryheavens.uk')
const nezhaDbPath = ref('/opt/nezha/dashboard/data/sqlite.db')
const appBearerToken = ref('')
const cfApiToken = ref('')
const biliSessdata = ref('')

const newPassword = ref('')
const saving = ref(false)
const testingPush = ref(false)

const PERSONA_OPTIONS = [
  { value: 'grok', label: 'Grok 锐评官' },
  { value: 'butler', label: '长庚执更管家' },
  { value: 'architect', label: '系统架构师' },
]

watch(
  () => store.state.showSettingsModal,
  (open) => {
    if (!open) return
    const c = store.state.configs || {}
    deepseekKey.value = c['deepseek_api_key'] || ''
    deepseekBaseUrl.value = c['deepseek_base_url'] || 'https://api.deepseek.com/v1'
    const rawModel = c['ai_model'] || 'deepseek-flash'
    aiModel.value =
      rawModel === 'deepseek-chat' || rawModel === 'deepseek-reasoner' || rawModel === 'deepseek-v4-pro'
        ? 'deepseek-flash'
        : rawModel
    activePersona.value = c['active_persona'] || 'grok'
    poolForLight.value = c['pool_for_light'] !== 'false'
    poolFallback.value = c['pool_fallback'] !== 'false'
    poolEntries.value = (store.state.modelPool || []).map((x) => ({ ...x }))
    probeResults.value = {}
    expandedPoolId.value = poolEntries.value[0]?.id || ''

    lightBaseUrl.value = c['light_base_url'] || ''
    lightApiKey.value = c['light_api_key'] || ''
    lightModel.value = c['light_model'] || ''

    nezhaUrl.value = c['nezha_url'] || 'https://dash.starryheavens.uk'
    nezhaDbPath.value = c['nezha_db_path'] || '/opt/nezha/dashboard/data/sqlite.db'
    appBearerToken.value = c['app_bearer_token'] || ''
    cfApiToken.value = c['cf_api_token'] || ''
    biliSessdata.value = c['bili_sessdata'] || ''
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
      deepseek_base_url: deepseekBaseUrl.value.trim() || 'https://api.deepseek.com/v1',
      ai_model: aiModel.value.trim() || 'deepseek-flash',
      active_persona: activePersona.value,
      pool_for_light: poolForLight.value ? 'true' : 'false',
      pool_fallback: poolFallback.value ? 'true' : 'false',
      light_base_url: lightBaseUrl.value.trim(),
      light_api_key: lightApiKey.value.trim(),
      light_model: lightModel.value.trim(),
      nezha_url: nezhaUrl.value.trim(),
      nezha_db_path: nezhaDbPath.value.trim(),
      app_bearer_token: appBearerToken.value.trim(),
      cf_api_token: cfApiToken.value.trim(),
      bili_sessdata: biliSessdata.value.trim(),
    })
    if (poolEntries.value.length > 0) {
      const normalized = poolEntries.value.map((e, i) => ({
        ...e,
        priority: (i + 1) * 10,
      }))
      await BotHubApi.saveModelPool(normalized)
    }
    store.notify('AI 引擎、免费模型池与凭据已安全保存')
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    saving.value = false
  }
}

const probingAll = ref(false)

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
  if (probingId.value && !probingAll.value) return
  probingId.value = targetId
  delete probeResults.value[targetId]
  try {
    const res = await BotHubApi.testModelTarget({
      target_id: targetId,
      provider,
      base_url: baseUrl.trim(),
      api_key: apiKey.trim(),
      model: modelId.trim() || 'deepseek-flash',
    })
    probeResults.value[targetId] = { ok: true, msg: res.message || `${res.latency_ms}ms OK` }
    if (res.model_health) {
      store.state.modelHealth = res.model_health
    }
  } catch (err: any) {
    probeResults.value[targetId] = { ok: false, msg: err.message || '连接失败' }
  } finally {
    if (!probingAll.value) probingId.value = ''
  }
}

async function probeAllEnabledPool() {
  if (probingAll.value || probingId.value) return
  const active = poolEntries.value.filter((e) => e.enabled && e.api_key.trim() !== '')
  if (active.length === 0) {
    store.notify('请至少为一个启用中的免费节点填入 API Key')
    return
  }
  probingAll.value = true
  let okCount = 0
  for (const entry of active) {
    await probeTarget(entry.id, entry.provider, entry.base_url, entry.api_key, entry.model)
    if (probeResults.value[entry.id]?.ok) okCount++
  }
  probingId.value = ''
  probingAll.value = false
  store.notify(`全池测速完成：${okCount}/${active.length} 节点连通`)
}

function addCustomPoolNode() {
  const id = `custom_${Date.now().toString(36).slice(-4)}`
  poolEntries.value.push({
    id,
    name: '自定义中转 / 免费模型节点',
    provider: 'custom',
    base_url: 'https://api.siliconflow.cn/v1',
    api_key: '',
    model: 'Qwen/Qwen2.5-7B-Instruct',
    enabled: true,
    priority: (poolEntries.value.length + 1) * 10,
    note: '任意 OpenAI 兼容免费或公益中转接口',
  })
  expandedPoolId.value = id
}

function movePoolNode(index: number, delta: number) {
  const next = index + delta
  if (next < 0 || next >= poolEntries.value.length) return
  const arr = [...poolEntries.value]
  const [item] = arr.splice(index, 1)
  arr.splice(next, 0, item)
  poolEntries.value = arr
}

function removePoolNode(id: string) {
  poolEntries.value = poolEntries.value.filter((x) => x.id !== id)
}

function useOfficialFlashPreset() {
  deepseekBaseUrl.value = 'https://api.deepseek.com/v1'
  aiModel.value = 'deepseek-flash'
}

function useRelayPreset() {
  if (!deepseekBaseUrl.value || deepseekBaseUrl.value.includes('api.deepseek.com')) {
    deepseekBaseUrl.value = 'https://your-relay.example.com/v1'
  }
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

function copyNativeAppProfile() {
  const origin = typeof window !== 'undefined' ? window.location.origin : 'https://bot.starryheavens.uk'
  const profile = JSON.stringify(
    {
      endpoint: origin,
      sse_events: `${origin}/api/v2/events`,
      inbox_api: `${origin}/api/v2/notifications`,
      auth_header: 'Authorization: Bearer <YOUR_APP_BEARER_TOKEN>',
    },
    null,
    2
  )
  navigator.clipboard?.writeText(profile).then(() => {
    store.notify('已复制原生 APP 接入端点 JSON')
  })
}

async function handleUpdatePassword() {
  if (newPassword.value.trim().length < 6) {
    store.notify('新密钥至少需要 6 位')
    return
  }
  try {
    await BotHubApi.updatePassword(newPassword.value.trim())
    newPassword.value = ''
    store.notify('控制台访问密钥修改成功')
  } catch (err: any) {
    store.notify(err.message)
  }
}

async function handleRotateToken() {
  if (!window.confirm('确定重置移动端 App Bearer Token 吗？重置后手机端需重新连接。')) return
  try {
    const res = await BotHubApi.rotateAppToken()
    if (res.token) {
      await navigator.clipboard?.writeText(res.token).catch(() => {})
    }
    store.notify(res.message || '已生成全新 App Bearer Token 并复制到剪贴板')
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message)
  }
}

function handleExportBackup() {
  window.open('/api/system/export', '_blank')
}

async function handleLogout() {
  if (!window.confirm('确定退出当前 Vesper BotHub 控制台会话吗？')) return
  closeModal()
  await store.logout()
}

async function handleImportBackup(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try {
    const text = await file.text()
    const parsed = JSON.parse(text)
    if (!parsed || (!parsed.assets && !parsed.tasks && !parsed.configs)) {
      throw new Error('备份文件格式不合法')
    }
    if (!window.confirm('确定导入备份并恢复覆盖当前资产、任务与配置吗？')) return
    await BotHubApi.importBackup(parsed)
    store.notify('备份导入恢复成功')
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify('导入失败: ' + err.message)
  } finally {
    input.value = ''
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
    <div class="border-b border-[#fff8ee]/10 pb-3 shrink-0">
      <SegmentedTabs v-model="activeTab" :items="SETTINGS_TABS" aria-label="系统设置分类" />
    </div>

    <div class="flex-1 min-h-0 overflow-y-auto pr-1">
      <Transition
        mode="out-in"
        enter-active-class="transition duration-180 ease-out"
        enter-from-class="opacity-0 translate-y-1"
        enter-to-class="opacity-100 translate-y-0"
        leave-active-class="transition duration-100 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <!-- Tab: AI & Free Model Pool -->
        <form v-if="activeTab === 'ai'" key="ai" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
          <!-- Primary: DeepSeek Flash + Relay Station Support -->
          <div class="modal-section flex flex-col gap-3">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <div>
                <div class="text-xs font-semibold text-[#f2b880]">主力引擎 (DeepSeek Flash / 中转站)</div>
                <p class="text-[11px] text-slate-400 mt-0.5">主力固定采用轻量高性价比 DeepSeek Flash；支持填入任何 OpenAI 兼容中转站地址与模型别名</p>
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

            <div class="flex items-center gap-1.5 flex-wrap">
              <span class="text-[11px] text-slate-400">接口快速预设:</span>
              <button
                type="button"
                @click="useOfficialFlashPreset"
                class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] text-[11px] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer"
              >
                DeepSeek 官方 (Flash)
              </button>
              <button
                type="button"
                @click="useRelayPreset"
                class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] text-[11px] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer"
              >
                中转站 /v1 地址
              </button>
            </div>

            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>主力 API Key (官方或中转站令牌)</span>
              <input v-model="deepseekKey" type="password" placeholder="sk-..." class="min-h-[42px] sm:min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            </label>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <label class="flex flex-col gap-1 text-xs text-slate-300">
                <span>API 地址 (支持中转站 Base URL)</span>
                <input v-model="deepseekBaseUrl" type="text" placeholder="https://api.deepseek.com/v1" class="min-h-[42px] sm:min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-xs text-white focus:border-[#f2b880]/60" />
              </label>
              <label class="flex flex-col gap-1 text-xs text-slate-300">
                <span>主力模型 ID (默认 Flash / 可填中转别名)</span>
                <input v-model="aiModel" type="text" placeholder="deepseek-flash" class="min-h-[42px] sm:min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-xs text-white focus:border-[#f2b880]/60" />
              </label>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div class="flex flex-col gap-1 text-xs text-slate-300">
                <span>默认人设</span>
                <CustomSelect v-model="activePersona" :options="PERSONA_OPTIONS" aria-label="默认人设" />
              </div>
              <div class="flex flex-col gap-1 text-xs text-slate-300">
                <span>额外轻量 Key (可选覆盖)</span>
                <input v-model="lightApiKey" type="password" placeholder="留空自动走下方免费池" class="min-h-[42px] sm:min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-xs text-white focus:border-[#f2b880]/60" />
              </div>
            </div>
          </div>

          <!-- Free AI Model Pool -->
          <div class="modal-section flex flex-col gap-3">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <div>
                <div class="text-xs font-semibold text-emerald-300">免费 AI 模型池 (Google AI Studio / 硅基流动 / 智谱等)</div>
                <p class="text-[11px] text-slate-400 mt-0.5">按优先级自动轮询容灾：遇到限流或主力未填 Key 时无缝切换下一个可用免费节点</p>
              </div>
              <div class="flex items-center gap-1.5 shrink-0">
                <button
                  type="button"
                  :disabled="probingAll"
                  @click="probeAllEnabledPool"
                  class="min-h-[30px] px-2.5 rounded-lg border border-[#f2b880]/35 bg-[#f2b880]/10 hover:bg-[#f2b880]/20 text-[11px] text-[#f2b880] cursor-pointer disabled:opacity-45"
                >
                  {{ probingAll ? '全池测速中...' : '一键全池测速' }}
                </button>
                <button
                  type="button"
                  @click="addCustomPoolNode"
                  class="min-h-[30px] px-2.5 rounded-lg border border-emerald-400/35 bg-emerald-400/10 hover:bg-emerald-400/18 text-[11px] text-emerald-300 cursor-pointer shrink-0"
                >
                  + 自定义节点
                </button>
              </div>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <ToggleSwitch
                v-model="poolForLight"
                label="后台摘要优先用免费池"
                description="晨报/雷达/网页精简先走免费节点"
              />
              <ToggleSwitch
                v-model="poolFallback"
                label="主力缺 Key/故障自动回退"
                description="主力异常时自动顺延调用免费池"
              />
            </div>

            <div class="flex flex-col gap-2">
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
                          : entry.api_key
                            ? 'bg-emerald-400 shadow-[0_0_8px_#34d399]'
                            : 'bg-amber-400/75'
                      ]"
                    />
                    <span class="text-xs font-semibold text-white truncate">{{ entry.name }}</span>
                    <span class="font-mono text-[10px] px-1.5 py-0.5 rounded bg-[#fff8ee]/[0.05] text-slate-400 shrink-0">
                      {{ entry.model }}
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
                      v-if="entry.provider === 'custom'"
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
                    <span>API Key ({{ entry.api_key ? '已配置，留着 **** 不变或覆盖输入新 Key' : '待填入免费 Key' }})</span>
                    <input v-model="entry.api_key" type="password" placeholder="粘贴对应平台的免费 API Key..." class="min-h-[36px] rounded-lg border border-[#fff8ee]/12 bg-[#070b18] px-2.5 py-1 font-mono text-xs text-white focus:border-[#f2b880]/60" />
                  </label>
                  <label class="flex flex-col gap-1 text-slate-400">
                    <span>显示名称</span>
                    <input v-model="entry.name" type="text" class="min-h-[36px] rounded-lg border border-[#fff8ee]/12 bg-[#070b18] px-2.5 py-1 text-xs text-white focus:border-[#f2b880]/60" />
                  </label>
                  <label class="flex flex-col gap-1 text-slate-400">
                    <span>上游模型 ID</span>
                    <input v-model="entry.model" type="text" class="min-h-[36px] rounded-lg border border-[#fff8ee]/12 bg-[#070b18] px-2.5 py-1 font-mono text-xs text-white focus:border-[#f2b880]/60" />
                  </label>
                  <label class="sm:col-span-2 flex flex-col gap-1 text-slate-400">
                    <span>OpenAI 兼容 Base URL</span>
                    <input v-model="entry.base_url" type="text" class="min-h-[36px] rounded-lg border border-[#fff8ee]/12 bg-[#070b18] px-2.5 py-1 font-mono text-xs text-white focus:border-[#f2b880]/60" />
                  </label>
                  <p v-if="entry.note" class="sm:col-span-2 text-slate-500">{{ entry.note }}</p>
                </div>
              </div>
            </div>
          </div>

          <div class="flex justify-end pt-1">
            <button type="submit" :disabled="saving" class="min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer">
              {{ saving ? '保存中...' : '保存 AI 引擎与免费模型池' }}
            </button>
          </div>
        </form>

        <!-- Tab: Native App Push -->
        <form v-else-if="activeTab === 'push'" key="push" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
          <div class="luxury-card rounded-2xl p-4 border-[#f2b880]/28">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <div class="flex items-center gap-2">
                <span :class="['w-2 h-2 rounded-full shrink-0', store.state.sseConnected ? 'bg-emerald-400 animate-pulse' : 'bg-amber-400']" aria-hidden="true" />
                <strong class="text-sm font-semibold text-white">原生 APP 通知通道（独家启用）</strong>
              </div>
              <span :class="['font-mono text-[10.5px] px-2 py-0.5 rounded-full border', store.state.sseConnected ? 'border-emerald-400/30 bg-emerald-400/10 text-emerald-300' : 'border-amber-400/30 bg-amber-400/10 text-amber-300']">
                {{ store.state.sseConnected ? 'SSE 实时通道已连接' : 'SSE 等待重连' }}
              </span>
            </div>
            <p class="text-xs text-slate-300/90 mt-1.5 leading-relaxed">
              所有定时晨报、前沿雷达、资产到期提醒与哪吒探针告警均直接入库至原生收件箱，并通过 <code class="font-mono text-[#f2b880]">/api/v2/events</code> SSE 长连接实时投递至移动端 APP 与控制台。
            </p>
          </div>

          <div class="modal-section flex flex-col gap-2">
            <label class="flex flex-col gap-1.5 text-xs text-slate-300">
              <span>Android / 原生 APP 接入令牌 (App Bearer Token)</span>
              <input v-model="appBearerToken" type="password" placeholder="bh_..." class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            </label>
            <p class="text-[11px] text-slate-400">客户端请求头携带 Authorization: Bearer 令牌即可订阅实时通知流；可在「安全与数据灾备」中轮换。</p>
          </div>

          <div class="flex items-center justify-between gap-2 flex-wrap pt-1">
            <button type="button" @click="copyNativeAppProfile" class="min-h-[44px] sm:min-h-[36px] px-3.5 py-2 rounded-xl border border-[#fff8ee]/16 text-xs text-slate-200 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer">
              复制移动端接入配置
            </button>
            <div class="flex items-center gap-2">
              <button type="button" :disabled="testingPush" @click="handleTestPush" class="min-h-[44px] sm:min-h-[36px] px-3.5 py-2 rounded-xl border border-[#fff8ee]/16 text-xs text-slate-200 hover:bg-white/5 cursor-pointer disabled:opacity-50">
                {{ testingPush ? '发送自检中...' : '发送测试通知到原生 APP' }}
              </button>
              <button type="submit" :disabled="saving" class="min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer">
                保存 APP 通道配置
              </button>
            </div>
          </div>
        </form>

        <!-- Tab: Integrations -->
        <form v-else-if="activeTab === 'integrations'" key="integrations" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
          <div class="modal-section flex flex-col gap-3">
            <div class="text-xs font-semibold text-[#f2b880]">服务器探针直连 (哪吒监控)</div>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>哪吒监控面板地址</span>
              <input v-model="nezhaUrl" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>哪吒 SQLite 本机路径 (只读直连)</span>
              <input v-model="nezhaDbPath" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            </label>
          </div>

          <div class="modal-section flex flex-col gap-3">
            <div class="text-xs font-semibold text-[#f2b880]">域名与生态扩展凭据</div>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>Cloudflare API Token</span>
              <input v-model="cfApiToken" type="password" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>哔哩哔哩 SESSDATA</span>
              <input v-model="biliSessdata" type="password" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            </label>
          </div>

          <div class="flex justify-end pt-1">
            <button type="submit" :disabled="saving" class="min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer">
              保存巡检凭据
            </button>
          </div>
        </form>

        <!-- Tab: Security & Backup -->
        <div v-else key="security" class="flex flex-col gap-3.5">
          <form @submit.prevent="handleUpdatePassword" class="modal-section flex flex-col gap-2.5">
            <div class="text-xs font-semibold text-[#f2b880]">访问密钥管理</div>
            <div class="flex flex-col sm:flex-row gap-2">
              <input v-model="newPassword" type="password" minlength="6" placeholder="输入新密钥 (至少6位)" aria-label="输入新密钥" required class="flex-1 min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
              <button type="submit" class="min-h-[44px] sm:min-h-[40px] px-4 py-2 rounded-xl border border-[#fff8ee]/16 text-xs text-slate-200 hover:bg-white/5 cursor-pointer shrink-0">
                更新密钥
              </button>
            </div>
          </form>

          <div class="modal-section flex flex-col gap-3">
            <div class="text-xs font-semibold text-[#f2b880]">系统数据灾备与令牌轮换</div>
            <div class="flex flex-wrap gap-2.5">
              <button type="button" @click="handleExportBackup" class="min-h-[44px] sm:min-h-[36px] px-3.5 py-2 rounded-xl border border-[#fff8ee]/16 text-xs text-slate-200 hover:bg-white/5 cursor-pointer">
                导出配置备份 (JSON)
              </button>
              <label class="min-h-[44px] sm:min-h-[36px] px-3.5 py-2 rounded-xl border border-[#fff8ee]/16 text-xs text-slate-200 hover:bg-white/5 cursor-pointer inline-flex items-center">
                导入恢复备份
                <input type="file" accept=".json" class="hidden" @change="handleImportBackup" />
              </label>
              <button type="button" @click="handleRotateToken" class="min-h-[44px] sm:min-h-[36px] px-3.5 py-2 rounded-xl border border-rose-400/40 bg-rose-500/15 text-xs text-rose-300 hover:bg-rose-500/25 cursor-pointer">
                重置移动端 App Token
              </button>
            </div>
          </div>

          <div class="modal-section flex items-center justify-between gap-3">
            <div>
              <h4 class="text-xs font-semibold text-white">退出当前控制台登录</h4>
              <p class="text-[11px] text-slate-400 mt-0.5">清除本机 Cookie 鉴权状态，需重新验证密钥</p>
            </div>
            <button
              type="button"
              @click="handleLogout"
              class="min-h-[44px] sm:min-h-[36px] px-4 py-2 rounded-xl border border-rose-400/45 bg-rose-500/15 text-xs font-medium text-rose-300 hover:bg-rose-500/25 cursor-pointer shrink-0"
            >
              退出登录
            </button>
          </div>
        </div>
      </Transition>
    </div>
  </BaseModal>
</template>
