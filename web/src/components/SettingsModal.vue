<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { lazyPanelChunk } from '../utils/lazyComponent'
import { resolveTabSlideTransition, type DirectionalTransitionName } from '../utils/motion'
import { checkPasskeyCapabilities, registerPasskeyCeremony } from '../utils/passkey'
import { useAppTheme } from '../utils/theme'
import type { ModelProvider, PoolStrategies, PooledModel } from '../types/bothub'
import BaseModal from './BaseModal.vue'
import SegmentedTabs from './SegmentedTabs.vue'
import SecretInput from './ui/SecretInput.vue'

const SettingsSecurityTab = lazyPanelChunk(() => import('./settings/SettingsSecurityTab.vue'))
const ModelConfigPanel = lazyPanelChunk(() => import('./settings/ModelConfigPanel.vue'))

const store = useAppStore()
const {
  colorMode,
  resolvedColorMode,
  accentId,
  colorModeOptions,
  accentOptions,
  setColorMode,
  setAccent,
} = useAppTheme()
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

// Decoupled Two-Layer Model Config (Providers + Free/Paid Pooled Models + Intra-Pool Strategies)
const draftProviders = ref<ModelProvider[]>([])
const draftModels = ref<PooledModel[]>([])
const draftStrategies = ref<PoolStrategies>({ free: 'priority', paid: 'priority' })

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

function hydrateModelConfigDrafts() {
  draftProviders.value = (store.state.modelProviders || []).map((p) => ({
    ...p,
    alias_ids: p.alias_ids ? [...p.alias_ids] : undefined,
  }))
  draftModels.value = (store.state.pooledModels || []).map((m) => ({ ...m }))
  draftStrategies.value = {
    free: store.state.poolStrategies?.free || 'priority',
    paid: store.state.poolStrategies?.paid || 'priority',
  }
}

function currentConfigSnapshot(): string {
  return JSON.stringify({
    providers: draftProviders.value,
    models: draftModels.value,
    strategies: draftStrategies.value,
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
  async (open) => {
    if (!open) return
    const c = store.state.configs || {}
    hydrateModelConfigDrafts()

    nezhaUrl.value = c['nezha_url'] || ''
    nezhaDbPath.value = c['nezha_db_path'] || ''
    cfApiToken.value = c['cf_api_token'] || ''
    biliSessdata.value = c['bili_sessdata'] || ''
    savedSnapshot.value = currentConfigSnapshot()

    // Refresh latest model telemetry & 7d usage asynchronously without blocking modal open
    void BotHubApi.getModelConfig()
      .then((res) => {
        store.state.modelProviders = res.providers || []
        store.state.pooledModels = res.models || []
        if (res.strategies) store.state.poolStrategies = res.strategies
        if (res.model_health) store.state.modelHealth = res.model_health
        if (res.model_usage_summary) store.state.modelUsageSummary = res.model_usage_summary
        if (!hasUnsavedChanges.value) {
          hydrateModelConfigDrafts()
          savedSnapshot.value = currentConfigSnapshot()
        }
      })
      .catch(() => {})

    void checkPasskeyCapabilities().then((caps) => {
      passkeyCapable.value = caps.platformSupported
    })
    void BotHubApi.passkeyStatus()
      .then((st) => {
        passkeyCount.value = st.count || 0
      })
      .catch(() => {})
  },
  { immediate: true }
)

function closeModal() {
  store.state.showSettingsModal = false
}

async function saveAllConfig() {
  saving.value = true
  try {
    const cfgPayload: Record<string, string> = {
      nezha_url: nezhaUrl.value.trim(),
      nezha_db_path: nezhaDbPath.value.trim(),
      cf_api_token: cfApiToken.value.trim(),
      bili_sessdata: biliSessdata.value.trim(),
    }
    const hasPaidModel = draftModels.value.some((m) => m.tier === 'paid')
    if (!hasPaidModel && (store.state.configs?.['deepseek_api_key'] || '').trim()) {
      cfgPayload.deepseek_api_key = ''
      cfgPayload.deepseek_base_url = ''
      cfgPayload.ai_model = ''
    }

    await BotHubApi.saveConfig(cfgPayload)
    const modelRes = await BotHubApi.saveModelConfig({
      providers: draftProviders.value,
      models: draftModels.value,
      strategies: draftStrategies.value,
    })
    store.state.modelProviders = modelRes.providers || []
    store.state.pooledModels = modelRes.models || []
    if (modelRes.strategies) store.state.poolStrategies = modelRes.strategies
    if (modelRes.model_options) store.state.modelOptions = modelRes.model_options
    if (modelRes.model_health) store.state.modelHealth = modelRes.model_health
    if (modelRes.model_usage_summary) store.state.modelUsageSummary = modelRes.model_usage_summary

    store.notify('模型渠道、双池策略与系统凭据已保存')
    await store.loadDashboard(false)
    hydrateModelConfigDrafts()
    savedSnapshot.value = currentConfigSnapshot()
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    saving.value = false
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
    <div class="border-b border-line pb-3 shrink-0 flex flex-col gap-2.5">
      <div class="flex flex-col gap-2">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <span class="text-xs font-medium text-fg-muted shrink-0">外观模式</span>
          <div class="flex items-center gap-1.5 flex-wrap" role="radiogroup" aria-label="外观模式">
            <button
              v-for="opt in colorModeOptions"
              :key="opt.id"
              type="button"
              role="radio"
              :aria-checked="colorMode === opt.id"
              @click="setColorMode(opt.id)"
              :class="[
                'inline-flex items-center px-3 py-1 rounded-full text-xs font-medium border transition cursor-pointer',
                colorMode === opt.id
                  ? 'border-accent-border bg-accent-soft text-accent-fg'
                  : 'border-line bg-card text-fg-sub hover:border-line-strong hover:text-fg'
              ]"
            >
              {{ opt.label }}
            </button>
          </div>
        </div>

        <div class="flex items-center justify-between gap-2 flex-wrap">
          <span class="text-xs font-medium text-fg-muted shrink-0">强调色</span>
          <div class="flex items-center gap-1.5 flex-wrap" role="radiogroup" aria-label="强调色">
            <button
              v-for="opt in accentOptions"
              :key="opt.id"
              type="button"
              role="radio"
              :aria-checked="accentId === opt.id"
              @click="setAccent(opt.id)"
              :class="[
                'inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border transition cursor-pointer',
                accentId === opt.id
                  ? 'border-accent-border bg-accent-soft text-accent-fg'
                  : 'border-line bg-card text-fg-sub hover:border-line-strong hover:text-fg'
              ]"
            >
              <span
                class="w-2.5 h-2.5 rounded-full border border-line-strong shrink-0"
                :style="{ backgroundColor: resolvedColorMode === 'light' ? opt.swatchLight : opt.swatchDark }"
                aria-hidden="true"
              />
              <span>{{ opt.label }}</span>
            </button>
          </div>
        </div>
      </div>
      <SegmentedTabs v-model="activeTab" :items="SETTINGS_TABS" aria-label="系统设置分类" />
    </div>

    <div class="flex-1 min-h-0 overflow-y-auto overflow-x-hidden pr-1">
      <Transition :name="settingsTabTransition" mode="out-in">
        <!-- Tab: Decoupled Model Providers & Free/Paid Pools -->
        <form v-if="activeTab === 'ai'" key="ai" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
          <ModelConfigPanel
            v-model:providers="draftProviders"
            v-model:models="draftModels"
            v-model:strategies="draftStrategies"
          />

          <div class="flex justify-end pt-1">
            <button type="submit" :disabled="saving" class="btn-jewel min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-lg text-xs cursor-pointer disabled:opacity-50">
              {{ saving ? '保存中...' : '保存模型配置与系统凭据' }}
            </button>
          </div>
        </form>

        <!-- Tab: Native App Push (Zero Manual Configuration) -->
        <div v-else-if="activeTab === 'push'" key="push" class="flex flex-col gap-3.5">
          <div class="vesper-card rounded-xl p-4 border-line">
            <div class="flex items-center justify-between gap-2 flex-wrap">
              <div class="flex items-center gap-2">
                <span :class="['w-2 h-2 rounded-full shrink-0', store.state.sseConnected ? 'bg-ok animate-pulse' : 'bg-warn']" aria-hidden="true" />
                <strong class="text-sm font-semibold text-fg">手机原生通知（零配置自动就绪）</strong>
              </div>
              <span :class="['font-mono text-xs px-2 py-0.5 rounded-full border', store.state.sseConnected ? 'border-ok-border bg-ok-soft text-ok' : 'border-warn-border bg-warn-soft text-warn']">
                {{ store.state.sseConnected ? 'SSE 实时通道在线' : 'SSE 等待重连' }}
              </span>
            </div>
            <p class="text-xs text-fg-sub mt-1.5 leading-relaxed">
              服务端已自动维护加密推送凭证，无需手动填写任何 Token。定时晨报、技术雷达、资产到期提醒与哪吒探针告警均自动直推手机锁屏。
            </p>
          </div>

          <div class="modal-section flex flex-col gap-3">
            <div>
              <div class="text-xs font-semibold text-accent-fg">无感面容 / 指纹激活手机客户端</div>
              <p class="text-xs text-fg-muted mt-0.5">
                方式一：注册面容/指纹通行密钥后，手机安装 APP 打开即可无感刷脸（或按指纹）直连；方式二：在手机浏览器直接点下方「一键免密唤醒绑定本机 APP」，无需输入网址或密码。
              </p>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
              <button
                type="button"
                :disabled="pairingApp"
                @click="handleOneTapPairApp"
                class="btn-jewel min-h-[44px] px-4 py-2.5 rounded-lg text-xs cursor-pointer disabled:opacity-50"
              >
                {{ pairingApp ? '正在唤醒客户端...' : '一键免密唤醒绑定本机 APP' }}
              </button>

              <button
                v-if="passkeyCapable"
                type="button"
                :disabled="registeringPasskey"
                @click="handleQuickCreatePasskey"
                class="btn-tactile-gold min-h-[44px] px-4 py-2.5 rounded-lg text-xs font-medium cursor-pointer disabled:opacity-50"
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
              class="btn-tactile min-h-[44px] sm:min-h-[36px] px-3.5 py-2 rounded-lg text-xs inline-flex items-center justify-center"
            >
              下载最新 Android 客户端 APK
            </a>
            <button
              type="button"
              :disabled="testingPush"
              @click="handleTestPush"
              class="btn-tactile-gold min-h-[44px] sm:min-h-[36px] px-4 py-2 rounded-lg text-xs font-medium cursor-pointer disabled:opacity-50"
            >
              {{ testingPush ? '正在推送测试...' : '发送测试通知到手机' }}
            </button>
          </div>
        </div>

        <!-- Tab: Integrations -->
        <form v-else-if="activeTab === 'integrations'" key="integrations" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
          <div class="modal-section flex flex-col gap-3">
            <div class="text-xs font-semibold text-accent-fg">服务器探针直连 (哪吒监控)</div>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>哪吒监控面板地址</span>
              <input v-model="nezhaUrl" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-lg border border-line bg-canvas px-3 py-2 font-mono text-sm text-fg focus:border-accent" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>哪吒 SQLite 本机路径 (只读直连，可选)</span>
              <input v-model="nezhaDbPath" type="text" placeholder="留空仅走网页/资产查询，填实际 .db 路径可直连探针" class="min-h-[44px] sm:min-h-[40px] rounded-lg border border-line bg-canvas px-3 py-2 font-mono text-sm text-fg placeholder:text-fg-muted focus:border-accent" />
            </label>
          </div>

          <div class="modal-section flex flex-col gap-3">
            <div class="text-xs font-semibold text-accent-fg">域名与生态扩展凭据</div>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>Cloudflare API Token</span>
              <SecretInput v-model="cfApiToken" toggle-label="Cloudflare API Token" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-fg-sub">
              <span>哔哩哔哩 SESSDATA</span>
              <SecretInput v-model="biliSessdata" toggle-label="哔哩哔哩 SESSDATA" />
            </label>
          </div>

          <div class="flex justify-end pt-1">
            <button type="submit" :disabled="saving" class="btn-jewel min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-lg text-xs cursor-pointer disabled:opacity-50">
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
      class="shrink-0 flex items-center justify-between gap-3 rounded-xl border border-accent-border bg-accent-soft px-3.5 py-2.5 text-xs"
    >
      <div class="flex items-center gap-2 min-w-0">
        <span class="w-2 h-2 rounded-full bg-accent animate-pulse shrink-0" aria-hidden="true" />
        <span class="text-accent-fg truncate">检测到配置改动，记得保存后生效</span>
      </div>
      <button
        type="button"
        :disabled="saving"
        @click="saveAllConfig"
        class="btn-jewel min-h-[34px] px-4 rounded-lg text-xs cursor-pointer shrink-0 disabled:opacity-50"
      >
        {{ saving ? '保存中...' : '立即保存全部改动' }}
      </button>
    </div>
  </BaseModal>
</template>
