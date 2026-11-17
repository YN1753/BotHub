<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import { copyToClipboard } from '../../utils/clipboard'
import { formatLocalDate, formatUptime, triggerBlobDownload } from '../../utils/format'
import {
  checkPasskeyCapabilities,
  registerPasskeyCeremony,
  syncAcceptedPasskeys,
  type PasskeyCredentialItem,
} from '../../utils/passkey'
import SecretInput from '../ui/SecretInput.vue'

const emit = defineEmits<{
  close: []
}>()

const store = useAppStore()
const newPassword = ref('')
const exportingMode = ref<'' | 'full' | 'redacted'>('')
const revokingDevices = ref(false)

const passkeySupported = ref(false)
const passkeyLoading = ref(false)
const passkeyRegistering = ref(false)
const passkeys = ref<PasskeyCredentialItem[]>([])

function formatEpochDate(epochSec?: number): string {
  if (!epochSec || epochSec <= 0) return '尚未在登录时使用'
  try {
    return new Date(epochSec * 1000).toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return ''
  }
}

async function loadPasskeyList() {
  passkeyLoading.value = true
  try {
    const res = await BotHubApi.listPasskeys()
    const creds = (res.credentials || []) as PasskeyCredentialItem[]
    passkeys.value = creds
    await syncAcceptedPasskeys(res.rpId, res.userId, creds)
  } catch {
    // Ignore silently
  } finally {
    passkeyLoading.value = false
  }
}

onMounted(async () => {
  const caps = await checkPasskeyCapabilities()
  passkeySupported.value = caps.platformSupported
  await loadPasskeyList()
})

async function handleCreatePasskey() {
  if (passkeyRegistering.value) return
  passkeyRegistering.value = true
  try {
    const res = await registerPasskeyCeremony({ promotion: false })
    if (res.ok) {
      store.notify('面容 / 指纹通行密钥已创建，手机 APP 与网页端均可直接无感登录')
      await loadPasskeyList()
    } else if (!res.cancelled && res.error) {
      store.notify(res.error)
    }
  } finally {
    passkeyRegistering.value = false
  }
}

async function handleRenamePasskey(item: PasskeyCredentialItem) {
  const next = window.prompt('修改通行密钥备注名称：', item.name)
  if (!next || next.trim() === item.name) return
  try {
    await BotHubApi.renamePasskey(item.id, next.trim())
    store.notify('通行密钥名称已更新')
    await loadPasskeyList()
  } catch (err: any) {
    store.notify(err.message || '重命名失败')
  }
}

async function handleDeletePasskey(item: PasskeyCredentialItem) {
  if (!window.confirm(`确定移除通行密钥「${item.name}」吗？`)) return
  try {
    await BotHubApi.deletePasskey(item.id)
    store.notify('已移除通行密钥并同步更新设备密码库')
    await loadPasskeyList()
  } catch (err: any) {
    store.notify(err.message || '移除失败')
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
  if (!window.confirm('确定轮换底层推送凭证吗？轮换后已连接手机需重新刷脸或一键唤醒绑定。')) return
  try {
    const res = await BotHubApi.rotateAppToken()
    store.notify(res.message || '已轮换底层推送凭证，旧设备连接已断开')
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
    const dateTag = formatLocalDate(new Date())
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
  const ok = await copyToClipboard(info)
  store.notify(ok ? '已复制版本诊断信息' : '复制失败')
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
    <!-- Passkey Biometric Management -->
    <div class="modal-section flex flex-col gap-3">
      <div class="flex items-start justify-between gap-3 flex-wrap">
        <div>
          <div class="text-xs font-semibold text-[#f2b880]">面容 / 指纹通行密钥 (Passkey)</div>
          <p class="text-[11px] text-slate-400 mt-0.5">
            绑定后在 Android 原生 APP 或浏览器均可优先无感刷脸、按指纹免密码登录与激活通知。
          </p>
        </div>
        <button
          v-if="passkeySupported"
          type="button"
          :disabled="passkeyRegistering"
          @click="handleCreatePasskey"
          class="btn-jewel min-h-[36px] px-3.5 py-1.5 rounded-xl text-xs font-semibold cursor-pointer shrink-0 disabled:opacity-50"
        >
          {{ passkeyRegistering ? '正在验证生物特征...' : '+ 注册面容 / 指纹密钥' }}
        </button>
      </div>

      <div v-if="passkeyLoading" class="text-xs text-slate-400 py-1">正在读取已注册密钥...</div>
      <div v-else-if="passkeys.length === 0" class="rounded-xl border border-[#fff8ee]/10 bg-[#050811]/60 px-3 py-2.5 text-xs text-slate-400">
        尚未注册通行密钥。点击右上方按钮即可使用当前设备的面容、触控 ID 或指纹一键绑定。
      </div>
      <div v-else class="flex flex-col gap-2">
        <div
          v-for="pk in passkeys"
          :key="pk.id"
          class="flex items-center justify-between gap-3 rounded-xl border border-[#fff8ee]/12 bg-[#050811]/75 px-3 py-2.5"
        >
          <div class="flex items-center gap-2.5 min-w-0">
            <span class="w-8 h-8 rounded-xl bg-[#f2b880]/15 border border-[#f2b880]/30 inline-flex items-center justify-center text-[#f2b880] shrink-0" aria-hidden="true">
              <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M12 10a2 2 0 0 0-2 2c0 1.02-.1 2.51-.26 4" />
                <path d="M14 13.12c0 2.38 0 6.38-1 8.88" />
                <path d="M2 12a10 10 0 0 1 18-6" />
                <path d="M5 19.5C5.5 18 6 15 6 12a6 6 0 0 1 .34-2" />
                <path d="M9 6.8a6 6 0 0 1 9 5.2v2" />
              </svg>
            </span>
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-xs font-medium text-white truncate">{{ pk.name }}</span>
                <span
                  v-if="pk.credentialBackedUp"
                  class="font-mono text-[9.5px] px-1.5 py-0.5 rounded border border-emerald-400/30 bg-emerald-500/10 text-emerald-300 shrink-0"
                >
                  跨端同步
                </span>
              </div>
              <div class="text-[10.5px] text-slate-400 font-mono truncate mt-0.5">
                注册于 {{ formatEpochDate(pk.registeredAt) }} · 最近使用 {{ formatEpochDate(pk.lastUsedAt) }}
              </div>
            </div>
          </div>
          <div class="flex items-center gap-1.5 shrink-0">
            <button
              type="button"
              @click="handleRenamePasskey(pk)"
              class="px-2.5 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
            >
              改名
            </button>
            <button
              type="button"
              @click="handleDeletePasskey(pk)"
              class="px-2.5 py-1 rounded-lg border border-rose-400/35 bg-rose-500/10 text-[11px] text-rose-300 hover:bg-rose-500/20 cursor-pointer"
            >
              移除
            </button>
          </div>
        </div>
      </div>
    </div>

    <form @submit.prevent="handleUpdatePassword" class="modal-section flex flex-col gap-2.5">
      <label for="settings-new-password" class="text-xs font-semibold text-[#f2b880]">兜底访问密码管理</label>
      <div class="flex flex-col sm:flex-row gap-2">
        <SecretInput
          id="settings-new-password"
          v-model="newPassword"
          name="new-password"
          autocomplete="new-password"
          enterkeyhint="done"
          :minlength="6"
          placeholder="输入新密钥 (至少6位)"
          toggle-label="新密钥"
          required
        />
        <button type="submit" class="min-h-[44px] sm:min-h-[40px] px-4 py-2 rounded-xl border border-[#fff8ee]/16 text-xs text-slate-200 hover:bg-white/5 cursor-pointer shrink-0">
          更新密码
        </button>
      </div>
    </form>

    <div class="modal-section flex flex-col gap-3">
      <div>
        <div class="text-xs font-semibold text-[#f2b880]">系统数据灾备与设备解绑</div>
        <p class="text-[11px] text-slate-400 mt-0.5">支持含密钥全量灾备导出，或自动遮蔽 API Key 与密码的脱敏分享导出</p>
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
          解绑所有已连接手机客户端
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
        <p class="text-[11px] text-slate-400 mt-0.5">可一键吊销其他浏览器登录会话，或退出本机登录</p>
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
