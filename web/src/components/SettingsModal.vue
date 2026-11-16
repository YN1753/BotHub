<script setup lang="ts">
import { ref, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import BaseModal from './BaseModal.vue'
import CustomSelect from './CustomSelect.vue'
import SegmentedTabs from './SegmentedTabs.vue'

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

const AI_MODEL_OPTIONS = [
  { value: 'deepseek-flash', label: 'DeepSeek Flash (默认主力)' },
  { value: 'deepseek-v4-pro', label: 'DeepSeek V4 Pro (深度主力)' },
]
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
    aiModel.value = c['ai_model'] || 'deepseek-flash'
    activePersona.value = c['active_persona'] || 'grok'
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
      ai_model: aiModel.value,
      active_persona: activePersona.value,
      light_base_url: lightBaseUrl.value.trim(),
      light_api_key: lightApiKey.value.trim(),
      light_model: lightModel.value.trim(),
      nezha_url: nezhaUrl.value.trim(),
      nezha_db_path: nezhaDbPath.value.trim(),
      app_bearer_token: appBearerToken.value.trim(),
      cf_api_token: cfApiToken.value.trim(),
      bili_sessdata: biliSessdata.value.trim(),
    })
    store.notify('全局系统凭据已安全保存')
    await store.loadDashboard(false)
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
        <!-- Tab: AI -->
        <form v-if="activeTab === 'ai'" key="ai" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
          <div class="modal-section flex flex-col gap-3">
            <div class="text-xs font-semibold text-[#f2b880]">主推理大脑 (Primary Engine)</div>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>DeepSeek API Key</span>
              <input v-model="deepseekKey" type="password" placeholder="sk-..." class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>API 接口地址 (Base URL)</span>
              <input v-model="deepseekBaseUrl" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            </label>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div class="flex flex-col gap-1 text-xs text-slate-300">
                <span>默认主力模型</span>
                <CustomSelect v-model="aiModel" :options="AI_MODEL_OPTIONS" aria-label="默认主力模型" />
              </div>
              <div class="flex flex-col gap-1 text-xs text-slate-300">
                <span>默认人设</span>
                <CustomSelect v-model="activePersona" :options="PERSONA_OPTIONS" aria-label="默认人设" />
              </div>
            </div>
          </div>

          <div class="modal-section flex flex-col gap-3">
            <div>
              <div class="text-xs font-semibold text-[#f2b880]">第二套轻量省钱模型 (可选)</div>
              <p class="text-[11px] text-slate-400 mt-0.5">用于网页摘要、记忆提炼及晨报批处理；留空自动回退主模型</p>
            </div>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>轻量模型 Base URL</span>
              <input v-model="lightBaseUrl" type="text" placeholder="如 https://api.siliconflow.cn/v1" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            </label>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <label class="flex flex-col gap-1 text-xs text-slate-300">
                <span>轻量模型 API Key</span>
                <input v-model="lightApiKey" type="password" placeholder="留空则回退主 Key" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
              </label>
              <label class="flex flex-col gap-1 text-xs text-slate-300">
                <span>轻量模型 ID</span>
                <input v-model="lightModel" type="text" placeholder="如 deepseek-flash" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
              </label>
            </div>
          </div>

          <div class="flex justify-end pt-1">
            <button type="submit" :disabled="saving" class="min-h-[44px] sm:min-h-[36px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer">
              保存 AI 大脑设置
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
