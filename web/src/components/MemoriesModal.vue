<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { formatTime } from '../utils/format'
import { applySiblingStaggerFallback, trackSpotlight } from '../utils/topLayer'
import BaseModal from './BaseModal.vue'
import CustomSelect from './CustomSelect.vue'
import SegmentedTabs from './SegmentedTabs.vue'

const store = useAppStore()
const listRef = ref<HTMLElement | null>(null)
const scope = ref('global')
const category = ref('infra')
const factKey = ref('')
const content = ref('')
const searchQuery = ref('')
const filterCat = ref('all')
const saving = ref(false)

const CAT_OPTIONS = [
  { value: 'infra', label: '基础设施与节点 (infra)' },
  { value: 'preference', label: '用户偏好与习惯 (preference)' },
  { value: 'schedule', label: '日程与运维约定 (schedule)' },
  { value: 'insight', label: 'Bot 洞察沉淀 (insight)' },
  { value: 'general', label: '通用背景事实 (general)' },
]

const scopeOptions = computed(() => [
  { value: 'global', label: '全局共享记忆 (所有 Bot 可见)' },
  ...store.state.bots
    .filter((b) => !b.is_group && b.id !== 'team_room')
    .map((b) => ({ value: `bot:${b.id}`, label: `仅限专属 Bot: ${b.name}` })),
])

const FILTER_TABS = [
  { id: 'all', label: '全部' },
  { id: 'infra', label: 'infra' },
  { id: 'preference', label: 'preference' },
  { id: 'insight', label: 'insight' },
  { id: 'general', label: 'general' },
]

const filteredMemories = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return store.state.memories.filter((m) => {
    if (filterCat.value !== 'all' && m.category !== filterCat.value) return false
    if (!q) return true
    return `${m.fact_key} ${m.content} ${m.category} ${m.scope || ''}`.toLowerCase().includes(q)
  })
})

function scopeLabel(raw?: string): string {
  if (!raw || raw === 'global') return '全局共享'
  if (raw.startsWith('bot:')) {
    const id = raw.slice(4)
    const b = store.state.bots.find((x) => x.id === id)
    return b ? `专属 · ${b.name}` : raw
  }
  return raw
}

function closeModal() {
  store.state.showMemoriesModal = false
}

async function handleAdd() {
  if (!factKey.value.trim() || !content.value.trim() || saving.value) return
  saving.value = true
  try {
    await BotHubApi.saveMemory({
      scope: scope.value,
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

watch(
  () => [store.state.showMemoriesModal, store.state.memories.length],
  () => nextTick(() => applySiblingStaggerFallback(listRef.value))
)
</script>

<template>
  <BaseModal
    :open="store.state.showMemoriesModal"
    title="全局长期记忆库 (SQLite Memory)"
    subtitle="跨 Bot 与跨会话持久化的基础设施约定与个人偏好"
    max-width="2xl"
    @close="closeModal"
  >
    <form @submit.prevent="handleAdd" class="modal-section grid grid-cols-1 sm:grid-cols-3 gap-3 shrink-0">
      <div class="flex flex-col gap-1 text-xs text-slate-300">
        <span>可见作用域</span>
        <CustomSelect v-model="scope" :options="scopeOptions" aria-label="可见作用域" />
      </div>
      <div class="flex flex-col gap-1 text-xs text-slate-300">
        <span>记忆分类</span>
        <CustomSelect v-model="category" :options="CAT_OPTIONS" aria-label="记忆分类" />
      </div>
      <label class="flex flex-col gap-1 text-xs text-slate-300">
        <span>事实键名 (唯一标识)</span>
        <input
          v-model="factKey"
          type="text"
          placeholder="如 primary_vps"
          required
          class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60"
        />
      </label>
      <div class="sm:col-span-3 flex flex-col sm:flex-row gap-2">
        <input
          v-model="content"
          type="text"
          placeholder="如 主力生产节点位于 CloudCone LA DC2，部署 Vesper 与哪吒面板"
          aria-label="记忆内容"
          required
          class="flex-1 min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60"
        />
        <button
          type="submit"
          :disabled="saving"
          class="min-h-[44px] sm:min-h-[40px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50 shrink-0"
        >
          写入记忆
        </button>
      </div>
    </form>

    <div class="flex flex-wrap items-center justify-between gap-2 shrink-0">
      <input
        v-model="searchQuery"
        type="search"
        placeholder="搜索键名、内容或 Bot 专属域..."
        aria-label="搜索长期记忆"
        class="w-full sm:w-56 min-h-[38px] sm:min-h-[34px] rounded-xl border border-[#fff8ee]/12 bg-[#060914] px-3 py-1.5 text-xs text-white placeholder-slate-500 focus:border-[#f2b880]/60"
      />
      <SegmentedTabs v-model="filterCat" :items="FILTER_TABS" variant="subtle" aria-label="记忆分类筛选" />
    </div>

    <div ref="listRef" class="stagger-list flex-1 min-h-0 overflow-y-auto flex flex-col gap-2.5 pr-1">
      <div v-if="filteredMemories.length === 0" class="py-10 text-center text-sm text-slate-500">
        {{ store.state.memories.length ? '无匹配的记忆条目' : '尚未写入长期记忆条目' }}
      </div>
      <article
        v-for="m in filteredMemories"
        :key="m.id"
        @pointermove="trackSpotlight"
        class="stagger-item luxury-card rounded-xl p-3.5"
      >
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div class="flex items-center gap-2 min-w-0">
            <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-md border border-[#f2b880]/30 bg-[#f2b880]/10 text-[#f2b880] shrink-0">
              {{ m.category }}
            </span>
            <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#fff8ee]/[0.04] text-slate-300 shrink-0">
              {{ scopeLabel(m.scope) }}
            </span>
            <strong class="font-mono text-xs text-white truncate">{{ m.fact_key }}</strong>
          </div>
          <div class="flex items-center gap-2 shrink-0">
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
        <p class="text-xs text-slate-200 mt-1.5 leading-relaxed">{{ m.content }}</p>
      </article>
    </div>
  </BaseModal>
</template>
