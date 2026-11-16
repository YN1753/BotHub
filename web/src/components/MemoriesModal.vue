<script setup lang="ts">
import { ref } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { formatTime } from '../utils/format'
import CustomSelect from './CustomSelect.vue'

const store = useAppStore()
const category = ref('infra')
const factKey = ref('')
const content = ref('')
const saving = ref(false)

const CAT_OPTIONS = [
  { value: 'infra', label: '基础设施与节点 (infra)' },
  { value: 'preference', label: '用户偏好与习惯 (preference)' },
  { value: 'schedule', label: '日程与运维约定 (schedule)' },
  { value: 'general', label: '通用背景事实 (general)' },
]

async function handleAdd() {
  if (!factKey.value.trim() || !content.value.trim() || saving.value) return
  saving.value = true
  try {
    await BotHubApi.saveMemory({
      category: category.value,
      fact_key: factKey.value.trim(),
      content: content.value.trim(),
    })
    factKey.value = ''
    content.value = ''
    store.notify('长期记忆已持久化写入')
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    saving.value = false
  }
}

async function handleDelete(id: number) {
  await BotHubApi.deleteMemory(id)
  store.notify('记忆条目已移除')
  await store.loadDashboard(false)
}
</script>

<template>
  <div
    v-if="store.state.showMemoriesModal"
    class="fixed inset-0 z-50 flex items-end sm:items-center justify-center sm:p-4 bg-black/70 backdrop-blur-sm"
    @click.self="store.state.showMemoriesModal = false"
  >
    <div class="aero-glass w-full sm:max-w-2xl max-h-[90dvh] sm:max-h-[85vh] rounded-t-2xl rounded-b-none sm:rounded-2xl p-4 sm:p-5 pb-[max(1rem,env(safe-area-inset-bottom))] flex flex-col gap-4 shadow-2xl">
      <div class="sm:hidden w-10 h-1 rounded-full bg-[#fff8ee]/20 mx-auto -mt-1 shrink-0" />
      <div class="flex items-center justify-between gap-2">
        <div>
          <h3 class="text-lg font-semibold text-white">全局长期记忆库 (SQLite Memory)</h3>
          <p class="text-xs text-slate-400">跨 Bot 与跨会话持久化的基础设施约定与个人偏好</p>
        </div>
        <button
          type="button"
          @click="store.state.showMemoriesModal = false"
          class="min-h-[40px] sm:min-h-[32px] px-3 py-1 rounded-lg border border-[#fff8ee]/14 text-xs text-slate-300 hover:bg-white/5 cursor-pointer shrink-0"
        >
          关闭
        </button>
      </div>

      <form @submit.prevent="handleAdd" class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div class="flex flex-col gap-1 text-xs text-slate-300">
          <span>记忆分类</span>
          <CustomSelect v-model="category" :options="CAT_OPTIONS" />
        </div>
        <label class="flex flex-col gap-1 text-xs text-slate-300">
          <span>事实键名 (唯一标识)</span>
          <input
            v-model="factKey"
            type="text"
            placeholder="如 primary_vps"
            required
            class="min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none"
          />
        </label>
        <div class="sm:col-span-2 flex flex-col sm:flex-row gap-2">
          <input
            v-model="content"
            type="text"
            placeholder="如 主力生产节点位于 CloudCone LA DC2，部署 Vesper 与哪吒面板"
            required
            class="flex-1 min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none"
          />
          <button
            type="submit"
            :disabled="saving"
            class="min-h-[40px] sm:min-h-[36px] px-4 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50 transition"
          >
            写入记忆
          </button>
        </div>
      </form>

      <div class="flex-1 min-h-0 overflow-y-auto flex flex-col gap-2 pr-1">
        <div v-if="store.state.memories.length === 0" class="py-8 text-center text-sm text-slate-500">
          尚未写入长期记忆条目
        </div>
        <article
          v-for="m in store.state.memories"
          :key="m.id"
          class="rounded-xl border border-[#fff8ee]/11 bg-[#070a16]/80 p-3"
        >
          <div class="flex items-center justify-between gap-2 flex-wrap">
            <div class="flex items-center gap-2">
              <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-md border border-[#f2b880]/30 bg-[#f2b880]/10 text-[#f2b880]">
                {{ m.category }}
              </span>
              <strong class="font-mono text-xs text-white">{{ m.fact_key }}</strong>
            </div>
            <div class="flex items-center gap-2">
              <span class="font-mono text-[11px] text-slate-500">{{ formatTime(m.updated_at) }}</span>
              <button
                type="button"
                @click="handleDelete(m.id)"
                class="min-h-[36px] sm:min-h-[26px] px-2.5 py-0.5 rounded-md border border-rose-400/35 text-[11px] text-rose-300 hover:bg-rose-500/15 cursor-pointer"
              >
                移除
              </button>
            </div>
          </div>
          <p class="text-xs text-slate-200 mt-1.5">{{ m.content }}</p>
        </article>
      </div>
    </div>
  </div>
</template>
