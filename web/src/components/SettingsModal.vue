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
  { id: 'push', label: '消息推送通道' },
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

const pushChannel = ref('wecom')
const wecomCorpId = ref('')
const wecomAgentId = ref('')
const wecomSecret = ref('')
const wecomToUser = ref('@all')
const pushplusToken = ref('')

const nezhaUrl = ref('https://dash.starryheavens.uk')
const nezhaDbPath = ref('/opt/nezha/dashboard/data/sqlite.db')
const appBearerToken = ref('')
const cfApiToken = ref('')
const biliSessdata = ref('')

const newPassword = ref('')
const saving = ref(false)

const AI_MODEL_OPTIONS = [
  { value: 'deepseek-flash', label: 'DeepSeek Flash (默认主力)' },
  { value: 'deepseek-v4-pro', label: 'DeepSeek V4 Pro (深度主力)' },
]
const PERSONA_OPTIONS = [
  { value: 'grok', label: 'Grok 锐评官' },
  { value: 'butler', label: '长庚执更管家' },
  { value: 'architect', label: '系统架构师' },
]
const CHANNEL_OPTIONS = [
  { value: 'wecom', label: '企业微信 (长庚执更主通道)' },
  { value: 'pushplus', label: 'PushPlus 备用通道' },
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

    pushChannel.value = c['push_channel'] || 'wecom'
    wecomCorpId.value = c['wecom_corpid'] || ''
    wecomAgentId.value = c['wecom_agentid'] || ''
    wecomSecret.value = c['wecom_secret'] || ''
    wecomToUser.value = c['wecom_touser'] || '@all'
    pushplusToken.value = c['pushplus_token'] || ''

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
      push_channel: pushChannel.value,
      wecom_corpid: wecomCorpId.value.trim(),
      wecom_agentid: wecomAgentId.value.trim(),
      wecom_secret: wecomSecret.value.trim(),
      wecom_touser: wecomToUser.value.trim() || '@all',
      pushplus_token: pushplusToken.value.trim(),
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
  try {
    const res = await BotHubApi.testPush()
    store.notify(res.message || '测试消息已推送')
  } catch (err: any) {
    store.notify(err.message)
  }
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
        <label class="flex flex-col gap-1 text-xs text-slate-300">
          <span>DeepSeek API Key</span>
          <input v-model="deepseekKey" type="password" placeholder="sk-..." class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
        </label>
        <label class="flex flex-col gap-1 text-xs text-slate-300">
          <span>API 接口地址 (Base URL)</span>
          <input v-model="deepseekBaseUrl" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
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

        <div class="pt-3 mt-1 border-t border-[#fff8ee]/10">
          <h4 class="text-sm font-semibold text-white">第二套轻量省钱模型配置 (可选)</h4>
          <p class="text-[11px] text-slate-400 mt-0.5">用于 read_url 摘要、后台记忆提炼及晨报/雷达批处理；留空自动回退主模型</p>
        </div>
        <label class="flex flex-col gap-1 text-xs text-slate-300">
          <span>轻量模型 Base URL</span>
          <input v-model="lightBaseUrl" type="text" placeholder="如 https://api.siliconflow.cn/v1" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
        </label>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>轻量模型 API Key</span>
            <input v-model="lightApiKey" type="password" placeholder="留空则回退主 Key" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>轻量模型 ID</span>
            <input v-model="lightModel" type="text" placeholder="如 deepseek-flash" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
          </label>
        </div>

        <div class="flex justify-end pt-2">
          <button type="submit" :disabled="saving" class="min-h-[44px] sm:min-h-[36px] px-4 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer">
            保存 AI 大脑设置
          </button>
        </div>
      </form>

        <!-- Tab: Push -->
        <form v-else-if="activeTab === 'push'" key="push" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
        <div class="flex flex-col gap-1 text-xs text-slate-300">
          <span>主消息推送通道</span>
          <CustomSelect v-model="pushChannel" :options="CHANNEL_OPTIONS" aria-label="主消息推送通道" />
        </div>
        <template v-if="pushChannel === 'wecom'">
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>企业微信 CorpID</span>
              <input v-model="wecomCorpId" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>应用 AgentID</span>
              <input v-model="wecomAgentId" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>应用 Secret</span>
              <input v-model="wecomSecret" type="password" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>接收成员 (ToUser)</span>
              <input v-model="wecomToUser" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
            </label>
          </div>
        </template>
        <template v-else>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>PushPlus Token</span>
            <input v-model="pushplusToken" type="password" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
          </label>
        </template>

        <div class="flex items-center justify-end gap-2 pt-2">
          <button type="button" @click="handleTestPush" class="min-h-[44px] sm:min-h-[36px] px-3.5 py-2 rounded-xl border border-[#fff8ee]/16 text-xs text-slate-200 hover:bg-white/5 cursor-pointer">
            发送测试消息
          </button>
          <button type="submit" :disabled="saving" class="min-h-[44px] sm:min-h-[36px] px-4 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer">
            保存通道设置
          </button>
        </div>
      </form>

        <!-- Tab: Integrations -->
        <form v-else-if="activeTab === 'integrations'" key="integrations" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
        <label class="flex flex-col gap-1 text-xs text-slate-300">
          <span>哪吒监控面板地址</span>
          <input v-model="nezhaUrl" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
        </label>
        <label class="flex flex-col gap-1 text-xs text-slate-300">
          <span>哪吒 SQLite 本机路径 (只读直连)</span>
          <input v-model="nezhaDbPath" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
        </label>
        <label class="flex flex-col gap-1 text-xs text-slate-300">
          <span>Android 原生客户端令牌 (App Bearer Token)</span>
          <input v-model="appBearerToken" type="password" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
        </label>
        <label class="flex flex-col gap-1 text-xs text-slate-300">
          <span>Cloudflare API Token</span>
          <input v-model="cfApiToken" type="password" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
        </label>
        <label class="flex flex-col gap-1 text-xs text-slate-300">
          <span>哔哩哔哩 SESSDATA</span>
          <input v-model="biliSessdata" type="password" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
        </label>
        <div class="flex justify-end pt-2">
          <button type="submit" :disabled="saving" class="min-h-[44px] sm:min-h-[36px] px-4 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer">
            保存巡检凭据
          </button>
        </div>
      </form>

        <!-- Tab: Security & Backup -->
        <div v-else key="security" class="flex flex-col gap-5">
        <form @submit.prevent="handleUpdatePassword" class="flex flex-col gap-2">
          <h4 class="text-sm font-semibold text-white">修改控制台访问密钥</h4>
          <div class="flex flex-col sm:flex-row gap-2">
            <input v-model="newPassword" type="password" minlength="6" placeholder="输入新密钥 (至少6位)" aria-label="输入新密钥" required class="flex-1 min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
            <button type="submit" class="min-h-[44px] sm:min-h-[40px] px-4 py-2 rounded-xl border border-[#fff8ee]/16 text-xs text-slate-200 hover:bg-white/5 cursor-pointer">
              更新密钥
            </button>
          </div>
        </form>

        <div class="pt-4 border-t border-[#fff8ee]/10 flex flex-col gap-2.5">
          <h4 class="text-sm font-semibold text-white">系统数据备份与令牌轮换</h4>
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

        <div class="pt-4 border-t border-[#fff8ee]/10 flex items-center justify-between gap-3">
          <div>
            <h4 class="text-sm font-semibold text-white">当前会话安全</h4>
            <p class="text-[11px] text-slate-400">退出后将清除本机控制台登录 Cookie</p>
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
