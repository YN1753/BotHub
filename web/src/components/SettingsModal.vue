<script setup lang="ts">
import { ref, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import CustomSelect from './CustomSelect.vue'

const store = useAppStore()
const activeTab = ref<'ai' | 'push' | 'integrations' | 'security'>('ai')

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
  <div
    v-if="store.state.showSettingsModal"
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/65 backdrop-blur-sm"
    @click.self="store.state.showSettingsModal = false"
  >
    <div class="aero-glass w-full max-w-2xl max-h-[88vh] rounded-2xl p-5 flex flex-col gap-4 shadow-2xl">
      <div class="flex items-center justify-between">
        <h3 class="text-lg font-semibold text-white">系统设置与全局凭据</h3>
        <button
          type="button"
          @click="store.state.showSettingsModal = false"
          class="px-2.5 py-1 rounded-lg border border-slate-400/20 text-xs text-slate-300 hover:bg-white/5 cursor-pointer"
        >
          关闭
        </button>
      </div>

      <div class="flex flex-wrap gap-1.5 border-b border-white/10 pb-2.5">
        <button
          v-for="tab in [
            { id: 'ai', label: 'AI 大脑设置' },
            { id: 'push', label: '消息推送通道' },
            { id: 'integrations', label: '巡检与探针凭据' },
            { id: 'security', label: '安全与数据灾备' },
          ]"
          :key="tab.id"
          type="button"
          @click="activeTab = tab.id as any"
          :class="[
            'px-3 py-1.5 rounded-lg text-xs border transition cursor-pointer',
            activeTab === tab.id
              ? 'border-sky-400/55 bg-sky-400/15 text-sky-300 font-medium'
              : 'border-slate-400/20 text-slate-300 hover:bg-white/5'
          ]"
        >
          {{ tab.label }}
        </button>
      </div>

      <div class="flex-1 min-h-0 overflow-y-auto pr-1">
        <!-- Tab: AI -->
        <form v-if="activeTab === 'ai'" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>DeepSeek API Key</span>
            <input v-model="deepseekKey" type="password" placeholder="sk-..." class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white focus:border-sky-400/60 focus:outline-none" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>API 接口地址 (Base URL)</span>
            <input v-model="deepseekBaseUrl" type="text" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white focus:border-sky-400/60 focus:outline-none" />
          </label>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="flex flex-col gap-1 text-xs text-slate-300">
              <span>默认主力模型</span>
              <CustomSelect v-model="aiModel" :options="AI_MODEL_OPTIONS" />
            </div>
            <div class="flex flex-col gap-1 text-xs text-slate-300">
              <span>默认人设</span>
              <CustomSelect v-model="activePersona" :options="PERSONA_OPTIONS" />
            </div>
          </div>

          <div class="pt-3 mt-1 border-t border-white/10">
            <h4 class="text-sm font-semibold text-white">第二套轻量省钱模型配置 (可选)</h4>
            <p class="text-[11px] text-slate-400 mt-0.5">用于 read_url 摘要、后台记忆提炼及晨报/雷达批处理；留空自动回退主模型</p>
          </div>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>轻量模型 Base URL</span>
            <input v-model="lightBaseUrl" type="text" placeholder="如 https://api.siliconflow.cn/v1" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white focus:border-sky-400/60 focus:outline-none" />
          </label>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>轻量模型 API Key</span>
              <input v-model="lightApiKey" type="password" placeholder="留空则回退主 Key" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white focus:border-sky-400/60 focus:outline-none" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>轻量模型 ID</span>
              <input v-model="lightModel" type="text" placeholder="如 deepseek-flash" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white focus:border-sky-400/60 focus:outline-none" />
            </label>
          </div>

          <div class="flex justify-end pt-2">
            <button type="submit" :disabled="saving" class="px-4 py-2 rounded-xl bg-sky-500 hover:bg-sky-400 text-slate-950 font-semibold text-xs cursor-pointer">
              保存 AI 大脑设置
            </button>
          </div>
        </form>

        <!-- Tab: Push -->
        <form v-if="activeTab === 'push'" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>主消息推送通道</span>
            <CustomSelect v-model="pushChannel" :options="CHANNEL_OPTIONS" />
          </div>
          <template v-if="pushChannel === 'wecom'">
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <label class="flex flex-col gap-1 text-xs text-slate-300">
                <span>企业微信 CorpID</span>
                <input v-model="wecomCorpId" type="text" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white" />
              </label>
              <label class="flex flex-col gap-1 text-xs text-slate-300">
                <span>应用 AgentID</span>
                <input v-model="wecomAgentId" type="text" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white" />
              </label>
              <label class="flex flex-col gap-1 text-xs text-slate-300">
                <span>应用 Secret</span>
                <input v-model="wecomSecret" type="password" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white" />
              </label>
              <label class="flex flex-col gap-1 text-xs text-slate-300">
                <span>接收成员 (ToUser)</span>
                <input v-model="wecomToUser" type="text" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white" />
              </label>
            </div>
          </template>
          <template v-else>
            <label class="flex flex-col gap-1 text-xs text-slate-300">
              <span>PushPlus Token</span>
              <input v-model="pushplusToken" type="password" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white" />
            </label>
          </template>

          <div class="flex items-center justify-end gap-2 pt-2">
            <button type="button" @click="handleTestPush" class="px-3.5 py-2 rounded-xl border border-slate-400/25 text-xs text-slate-200 hover:bg-white/5 cursor-pointer">
              发送测试消息
            </button>
            <button type="submit" :disabled="saving" class="px-4 py-2 rounded-xl bg-sky-500 hover:bg-sky-400 text-slate-950 font-semibold text-xs cursor-pointer">
              保存通道设置
            </button>
          </div>
        </form>

        <!-- Tab: Integrations -->
        <form v-if="activeTab === 'integrations'" @submit.prevent="saveAllConfig" class="flex flex-col gap-3.5">
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>哪吒监控面板地址</span>
            <input v-model="nezhaUrl" type="text" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>哪吒 SQLite 本机路径 (只读直连)</span>
            <input v-model="nezhaDbPath" type="text" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>Android 原生客户端令牌 (App Bearer Token)</span>
            <input v-model="appBearerToken" type="password" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>Cloudflare API Token</span>
            <input v-model="cfApiToken" type="password" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>哔哩哔哩 SESSDATA</span>
            <input v-model="biliSessdata" type="password" class="rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white" />
          </label>
          <div class="flex justify-end pt-2">
            <button type="submit" :disabled="saving" class="px-4 py-2 rounded-xl bg-sky-500 hover:bg-sky-400 text-slate-950 font-semibold text-xs cursor-pointer">
              保存巡检凭据
            </button>
          </div>
        </form>

        <!-- Tab: Security & Backup -->
        <div v-if="activeTab === 'security'" class="flex flex-col gap-5">
          <form @submit.prevent="handleUpdatePassword" class="flex flex-col gap-2">
            <h4 class="text-sm font-semibold text-white">修改控制台访问密钥</h4>
            <div class="flex gap-2">
              <input v-model="newPassword" type="password" minlength="6" placeholder="输入新密钥 (至少6位)" required class="flex-1 rounded-xl border border-slate-400/20 bg-[#080d1c] px-3 py-2 font-mono text-sm text-white" />
              <button type="submit" class="px-4 py-2 rounded-xl border border-slate-400/25 text-xs text-slate-200 hover:bg-white/5 cursor-pointer">
                更新密钥
              </button>
            </div>
          </form>

          <div class="pt-4 border-t border-white/10 flex flex-col gap-2.5">
            <h4 class="text-sm font-semibold text-white">系统数据备份与令牌轮换</h4>
            <div class="flex flex-wrap gap-2.5">
              <button type="button" @click="handleExportBackup" class="px-3.5 py-2 rounded-xl border border-slate-400/25 text-xs text-slate-200 hover:bg-white/5 cursor-pointer">
                导出配置备份 (JSON)
              </button>
              <label class="px-3.5 py-2 rounded-xl border border-slate-400/25 text-xs text-slate-200 hover:bg-white/5 cursor-pointer inline-flex items-center">
                导入恢复备份
                <input type="file" accept=".json" class="hidden" @change="handleImportBackup" />
              </label>
              <button type="button" @click="handleRotateToken" class="px-3.5 py-2 rounded-xl border border-rose-400/40 bg-rose-500/15 text-xs text-rose-300 hover:bg-rose-500/25 cursor-pointer">
                重置移动端 App Token
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
