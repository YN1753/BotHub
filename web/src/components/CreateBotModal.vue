<script setup lang="ts">
import { ref } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
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
    store.state.showCreateBotModal = false
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
  <div
    v-if="store.state.showCreateBotModal"
    class="fixed inset-0 z-50 flex items-end sm:items-center justify-center sm:p-4 bg-black/70 backdrop-blur-sm"
    @click.self="store.state.showCreateBotModal = false"
  >
    <div class="aero-glass w-full sm:max-w-lg max-h-[90dvh] sm:max-h-[86vh] rounded-t-2xl rounded-b-none sm:rounded-2xl p-4 sm:p-6 pb-[max(1rem,env(safe-area-inset-bottom))] flex flex-col gap-4 shadow-2xl">
      <div class="sm:hidden w-10 h-1 rounded-full bg-[#fff8ee]/20 mx-auto -mt-1 shrink-0" />
      <div class="flex items-center justify-between">
        <h3 class="text-lg font-semibold text-white">招募全新专属 Bot 成员</h3>
        <button
          type="button"
          @click="store.state.showCreateBotModal = false"
          class="min-h-[40px] sm:min-h-[32px] px-3 py-1 rounded-lg border border-[#fff8ee]/14 text-xs text-slate-300 hover:bg-white/5 cursor-pointer"
        >
          关闭
        </button>
      </div>

      <form @submit.prevent="handleSubmit" class="flex-1 min-h-0 overflow-y-auto pr-1 flex flex-col gap-3.5">
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
          <label class="flex flex-col gap-1.5 text-xs text-slate-300">
            <span>Bot 名称</span>
            <input
              v-model="name"
              type="text"
              placeholder="如：财务核对专员"
              required
              class="min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none"
            />
          </label>
          <label class="flex flex-col gap-1.5 text-xs text-slate-300">
            <span>职责定位</span>
            <input
              v-model="roleTitle"
              type="text"
              placeholder="如：云账单追踪与续费评估"
              required
              class="min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none"
            />
          </label>
          <div class="flex flex-col gap-1.5 text-xs text-slate-300">
            <span>头像主题色</span>
            <CustomSelect v-model="avatarStyle" :options="AVATAR_OPTIONS" />
          </div>
          <div class="flex flex-col gap-1.5 text-xs text-slate-300">
            <span>默认推理模型</span>
            <CustomSelect v-model="model" :options="MODEL_OPTIONS" />
          </div>
        </div>

        <label class="flex flex-col gap-1.5 text-xs text-slate-300">
          <span>专属指令 (System Prompt)</span>
          <textarea
            v-model="systemPrompt"
            rows="3"
            placeholder="描述它的工作风格、输出格式与关注重点..."
            class="rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none"
          />
        </label>

        <div class="flex justify-end gap-2 pt-2">
          <button
            type="button"
            @click="store.state.showCreateBotModal = false"
            class="min-h-[40px] sm:min-h-[34px] px-4 py-2 rounded-xl border border-[#fff8ee]/14 text-xs text-slate-300 hover:bg-white/5 cursor-pointer"
          >
            取消
          </button>
          <button
            type="submit"
            :disabled="saving"
            class="min-h-[40px] sm:min-h-[34px] px-4 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50 transition"
          >
            {{ saving ? '创建中...' : '创建并激活 Bot' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
