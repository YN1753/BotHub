<script setup lang="ts">
import { ref } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import BaseModal from './BaseModal.vue'
import CustomSelect from './CustomSelect.vue'

const store = useAppStore()
const name = ref('')
const roleTitle = ref('')
const avatarStyle = ref('cyan')
const model = ref('deepseek-flash')
const systemPrompt = ref('')
const saving = ref(false)

const AVATAR_OPTIONS = [
  { value: 'cyan', label: '暮光暖金 (Vesper Gold)' },
  { value: 'indigo', label: '深空靛蓝 (Deep Indigo)' },
  { value: 'amber', label: '炽焰琥珀 (Warm Amber)' },
  { value: 'emerald', label: '星野翠绿 (Emerald)' },
  { value: 'violet', label: '星云暮紫 (Nebula Violet)' },
]

const MODEL_OPTIONS = [
  { value: 'deepseek-flash', label: 'DeepSeek Flash (主力)' },
  { value: 'deepseek-v4-pro', label: 'DeepSeek V4 Pro (深度)' },
]

function closeModal() {
  store.state.showCreateBotModal = false
}

async function handleSubmit() {
  if (!name.value.trim() || saving.value) return
  saving.value = true
  try {
    const res = await BotHubApi.createBot({
      name: name.value.trim(),
      role_title: roleTitle.value.trim() || '专属智能助理',
      avatar_style: avatarStyle.value,
      model: model.value,
      enable_thinking: true,
      allowed_tools: '["read_url","web_search","query_nezha_nodes","manage_assets","manage_tasks","remember_fact","recall_memory"]',
      system_prompt: systemPrompt.value.trim(),
    })
    store.notify('专属 Bot 创建并激活成功')
    name.value = ''
    roleTitle.value = ''
    systemPrompt.value = ''
    closeModal()
    await store.loadDashboard(false)
    if (res.bot?.id) {
      await store.selectBot(res.bot.id)
    }
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <BaseModal
    :open="store.state.showCreateBotModal"
    title="招募全新专属 Bot 成员"
    subtitle="为其分配独立身份、模型偏好与最小权限工具集"
    max-width="sm"
    @close="closeModal"
  >
    <form @submit.prevent="handleSubmit" class="flex-1 min-h-0 overflow-y-auto pr-1 flex flex-col gap-3.5">
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
        <label class="flex flex-col gap-1.5 text-xs text-slate-300">
          <span>Bot 名称</span>
          <input
            v-model="name"
            type="text"
            autofocus
            placeholder="如：财务核对专员"
            required
            class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none"
          />
        </label>
        <label class="flex flex-col gap-1.5 text-xs text-slate-300">
          <span>职责定位</span>
          <input
            v-model="roleTitle"
            type="text"
            placeholder="如：云账单追踪与续费评估"
            required
            class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none"
          />
        </label>
        <div class="flex flex-col gap-1.5 text-xs text-slate-300">
          <span>头像主题色</span>
          <CustomSelect v-model="avatarStyle" :options="AVATAR_OPTIONS" aria-label="头像主题色" />
        </div>
        <div class="flex flex-col gap-1.5 text-xs text-slate-300">
          <span>默认推理模型</span>
          <CustomSelect v-model="model" :options="MODEL_OPTIONS" aria-label="默认推理模型" />
        </div>
      </div>

      <label class="flex flex-col gap-1.5 text-xs text-slate-300">
        <span>专属指令 (System Prompt)</span>
        <textarea
          v-model="systemPrompt"
          rows="3"
          placeholder="描述它的工作风格、输出格式与关注重点..."
          class="auto-size-textarea w-full rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none resize-y"
        />
      </label>

      <div class="flex justify-end gap-2 pt-2 border-t border-[#fff8ee]/10">
        <button
          type="button"
          @click="closeModal"
          class="min-h-[44px] sm:min-h-[36px] px-4 py-2 rounded-xl border border-[#fff8ee]/14 text-xs text-slate-300 hover:bg-white/5 cursor-pointer"
        >
          取消
        </button>
        <button
          type="submit"
          :disabled="saving"
          class="min-h-[44px] sm:min-h-[36px] px-4 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50"
        >
          {{ saving ? '创建中...' : '创建并激活 Bot' }}
        </button>
      </div>
    </form>
  </BaseModal>
</template>
