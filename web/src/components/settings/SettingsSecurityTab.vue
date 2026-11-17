<script setup lang="ts">
import { ref } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import { formatUptime, triggerBlobDownload } from '../../utils/format'

const emit = defineEmits<{
  close: []
}>()

const store = useAppStore()
const newPassword = ref('')
const showNewPassword = ref(false)
const exportingMode = ref<'' | 'full' | 'redacted'>('')
const revokingDevices = ref(false)

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
    await store.refreshWorkspaceSlices(['configs'])
  } catch (err: any) {
    store.notify(err.message)
  }
}

async function handleExportBackup(maskSecrets = false) {
  if (exportingMode.value) return
  exportingMode.value = maskSecrets ? 'redacted' : 'full'
  try {
    const url = maskSecrets ? '/api/system/export?mask_secrets=1' : '/api/system/export'
    const res = await fetch(url, { credentials: 'same-origin' })
    if (!res.ok) {
      throw new Error(`导出备份失败 (${res.status})`)
    }
    const blob = await res.blob()
    const dateTag = new Date().toISOString().slice(0, 10)
    const filename = maskSecrets
      ? `vesper_bothub_backup_redacted_${dateTag}.json`
      : `vesper_bothub_backup_${dateTag}.json`
    triggerBlobDownload(blob, filename)
    store.notify(maskSecrets ? '已导出脱敏备份（已隐藏 API Key 与密码）' : '已导出完整灾备配置 JSON')
  } catch (err: any) {
    store.notify(err.message || '导出备份失败')
  } finally {
    exportingMode.value = ''
  }
}

async function handleRevokeOtherDevices() {
  if (revokingDevices.value) return
  if (!window.confirm('确定吊销除当前浏览器外所有已登录设备（含勾选“记住此设备”的 30 天会话）吗？')) return
  revokingDevices.value = true
  try {
    await store.logoutAllOtherDevices()
  } catch (err: any) {
    store.notify(err.message || '吊销会话失败')
  } finally {
    revokingDevices.value = false
  }
}

async function copyBuildDiagnostic() {
  const info = `${store.state.version} (${store.state.commit}) · uptime ${formatUptime(store.state.uptimeSec)}`
  try {
    await navigator.clipboard.writeText(info)
    store.notify('已复制版本诊断信息')
  } catch {
    store.notify('复制失败')
  }
}

async function handleLogout() {
  if (!window.confirm('确定退出当前 Vesper BotHub 控制台会话吗？')) return
  emit('close')
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
    const summaryParts = [
      `${Array.isArray(parsed.assets) ? parsed.assets.length : 0} 项资产`,
      `${Array.isArray(parsed.tasks) ? parsed.tasks.length : 0} 条任务`,
      `${Array.isArray(parsed.bots) ? parsed.bots.length : 0} 个 Bot`,
      `${Array.isArray(parsed.skills) ? parsed.skills.length : 0} 项技能`,
      `${Array.isArray(parsed.todos) ? parsed.todos.length : 0} 条待办`,
      `${Array.isArray(parsed.notes) ? parsed.notes.length : 0} 篇文档`,
      `${Array.isArray(parsed.agent_memories) ? parsed.agent_memories.length : 0} 条记忆`,
      `${parsed.configs && typeof parsed.configs === 'object' ? Object.keys(parsed.configs).length : 0} 项配置`,
    ].join(' · ')
    if (!window.confirm(`确定导入备份并覆盖恢复当前工作台数据吗？\n\n备份内容：${summaryParts}`)) return
    const res = await BotHubApi.importBackup(parsed)
    store.notify(res.message || '备份导入恢复成功')
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify('导入失败: ' + err.message)
  } finally {
    input.value = ''
  }
}
</script>

<template>
  <div class="flex flex-col gap-3.5">
    <form @submit.prevent="handleUpdatePassword" class="modal-section flex flex-col gap-2.5">
      <label for="settings-new-password" class="text-xs font-semibold text-[#f2b880]">访问密钥管理</label>
      <div class="flex flex-col sm:flex-row gap-2">
        <div class="relative flex-1 flex items-center">
          <input
            id="settings-new-password"
            v-model="newPassword"
            name="new-password"
            :type="showNewPassword ? 'text' : 'password'"
            autocomplete="new-password"
            enterkeyhint="done"
            minlength="6"
            placeholder="输入新密钥 (至少6位)"
            required
            class="w-full min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] pl-3 pr-14 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60"
          />
          <button
            type="button"
            @click="showNewPassword = !showNewPassword"
            :aria-label="showNewPassword ? '隐藏新密钥' : '显示新密钥'"
            class="absolute right-2 px-2 py-1 rounded-lg text-[11px] text-slate-400 hover:text-slate-200 cursor-pointer"
          >
            {{ showNewPassword ? '隐藏' : '显示' }}
          </button>
        </div>
        <button type="submit" class="min-h-[44px] sm:min-h-[40px] px-4 py-2 rounded-xl border border-[#fff8ee]/16 text-xs text-slate-200 hover:bg-white/5 cursor-pointer shrink-0">
          更新密钥
        </button>
      </div>
    </form>

    <div class="modal-section flex flex-col gap-3">
      <div>
        <div class="text-xs font-semibold text-[#f2b880]">系统数据灾备与令牌轮换</div>
        <p class="text-[11px] text-slate-400 mt-0.5">支持含密钥全量灾备导出，或自动遮蔽 API Key 与密码的脱敏分享导出（导入时自动保留现有密钥）</p>
      </div>
      <div class="flex flex-wrap gap-2.5">
        <button
          type="button"
          :disabled="Boolean(exportingMode)"
          @click="handleExportBackup(false)"
          class="min-h-[44px] sm:min-h-[36px] px-3.5 py-2 rounded-xl border border-[#fff8ee]/16 text-xs text-slate-200 hover:bg-white/5 cursor-pointer disabled:opacity-50"
        >
          {{ exportingMode === 'full' ? '正在导出...' : '全量导出备份 (JSON)' }}
        </button>
        <button
          type="button"
          :disabled="Boolean(exportingMode)"
          @click="handleExportBackup(true)"
          class="min-h-[44px] sm:min-h-[36px] px-3.5 py-2 rounded-xl border border-emerald-400/35 bg-emerald-500/10 text-xs text-emerald-200 hover:bg-emerald-500/20 cursor-pointer disabled:opacity-50"
        >
          {{ exportingMode === 'redacted' ? '正在导出...' : '脱敏导出备份 (隐藏密钥)' }}
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

    <div class="modal-section flex items-center justify-between gap-3 flex-wrap">
      <div>
        <h4 class="text-xs font-semibold text-white">运行版本与构建信息</h4>
        <p class="font-mono text-[11px] text-slate-400 mt-0.5">
          {{ store.state.version }}
          <template v-if="store.state.commit"> · {{ store.state.commit }}</template>
          <template v-if="store.state.buildTime"> · {{ store.state.buildTime }}</template>
          <template v-if="store.state.uptimeSec > 0"> · 已运行 {{ formatUptime(store.state.uptimeSec) }}</template>
        </p>
      </div>
      <div class="flex items-center gap-2">
        <button
          type="button"
          @click="copyBuildDiagnostic"
          class="px-2.5 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer"
        >
          复制诊断
        </button>
        <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-[#fff8ee]/12 text-slate-400">运行中</span>
      </div>
    </div>

    <div class="modal-section flex items-center justify-between gap-3 flex-wrap">
      <div>
        <h4 class="text-xs font-semibold text-white">多端会话管理与退出登录</h4>
        <p class="text-[11px] text-slate-400 mt-0.5">可一键轮换会话签名密钥使所有其他设备（含 30 天免输设备）立即下线，或退出本机登录</p>
      </div>
      <div class="flex items-center gap-2 flex-wrap">
        <button
          type="button"
          :disabled="revokingDevices"
          @click="handleRevokeOtherDevices"
          class="min-h-[44px] sm:min-h-[36px] px-3.5 py-2 rounded-xl border border-amber-400/40 bg-amber-500/12 text-xs font-medium text-amber-200 hover:bg-amber-500/22 cursor-pointer disabled:opacity-50"
        >
          {{ revokingDevices ? '正在吊销...' : '吊销其他已登录设备' }}
        </button>
        <button
          type="button"
          @click="handleLogout"
          class="min-h-[44px] sm:min-h-[36px] px-4 py-2 rounded-xl border border-rose-400/45 bg-rose-500/15 text-xs font-medium text-rose-300 hover:bg-rose-500/25 cursor-pointer"
        >
          退出本机登录
        </button>
      </div>
    </div>
  </div>
</template>
