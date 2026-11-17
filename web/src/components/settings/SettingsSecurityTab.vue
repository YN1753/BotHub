<script setup lang="ts">
import { ref } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'

const emit = defineEmits<{
  close: []
}>()

const store = useAppStore()
const newPassword = ref('')

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
      `${Array.isArray(parsed.agent_memories) ? parsed.agent_memories.length : 0} 条记忆`,
      `${parsed.configs && typeof parsed.configs === 'object' ? Object.keys(parsed.configs).length : 0} 项配置`,
    ].join(' · ')
    if (!window.confirm(`确定导入备份并覆盖恢复当前工作台数据吗？\n\n备份内容：${summaryParts}`)) return
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
  <div class="flex flex-col gap-3.5">
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
        <h4 class="text-xs font-semibold text-white">运行版本与构建信息</h4>
        <p class="font-mono text-[11px] text-slate-400 mt-0.5">
          {{ store.state.version }}
          <template v-if="store.state.commit"> · {{ store.state.commit }}</template>
          <template v-if="store.state.buildTime"> · {{ store.state.buildTime }}</template>
        </p>
      </div>
      <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-[#fff8ee]/12 text-slate-400">运行中</span>
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
</template>
