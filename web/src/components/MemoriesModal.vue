<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { formatTime } from '../utils/format'
import { applySiblingStaggerFallback, trackSpotlight } from '../utils/topLayer'
import type { AgentMemory } from '../types/bothub'
import BaseModal from './BaseModal.vue'
import CustomSelect from './CustomSelect.vue'
import SegmentedTabs from './SegmentedTabs.vue'

const store = useAppStore()
const listRef = ref<HTMLElement | null>(null)
const scope = ref('global')
const category = ref('preference')
const factKey = ref('')
const content = ref('')
const importance = ref(3)
const pinned = ref(false)
const searchQuery = ref('')
const filterCat = ref('all')
const filterScope = ref('all')
const pinnedOnly = ref(false)
const coreOnly = ref(false)
const sortBy = ref<'weight' | 'recent' | 'recalled'>('weight')
const pruning = ref(false)
const saving = ref(false)

// Inline edit state
const editingMemId = ref<number | null>(null)
const editContent = ref('')
const editCategory = ref('preference')
const editImportance = ref(3)
const editPinned = ref(false)
const editScope = ref('global')
const selectedIds = ref<number[]>([])
const bulkBusy = ref(false)

const memoryStats = computed(() => {
  const list = filteredMemories.value
  const pinnedCount = list.filter((m) => Boolean(m.pinned)).length
  const totalRecalls = list.reduce((acc, m) => acc + (m.recall_count || 0), 0)
  return {
    total: list.length,
    pinned: pinnedCount,
    recalls: totalRecalls,
  }
})

const CAT_OPTIONS = [
  { value: 'preference', label: '偏好与习惯 (preference)' },
  { value: 'insight', label: '交互洞察与约定 (insight)' },
  { value: 'infra', label: '环境与节点事实 (infra)' },
  { value: 'schedule', label: '日程与约定 (schedule)' },
  { value: 'general', label: '通用背景 (general)' },
]

const IMPORTANCE_OPTIONS = [
  { value: '5', label: '权重 5 (核心必记)' },
  { value: '4', label: '权重 4 (高优先级)' },
  { value: '3', label: '权重 3 (常规事实)' },
  { value: '2', label: '权重 2 (次要背景)' },
  { value: '1', label: '权重 1 (临时印象)' },
]

const scopeOptions = computed(() => [
  { value: 'global', label: '全局共享 (所有 Bot 可见)' },
  ...store.state.bots.map((b) => ({
    value: `bot:${b.id}`,
    label: b.is_group ? `专属群聊: ${b.name}` : `专属 Bot: ${b.name}`,
  })),
])

const filterScopeOptions = computed(() => [
  { value: 'all', label: '全部作用域' },
  ...scopeOptions.value,
])

const FILTER_TABS = [
  { id: 'all', label: '全部' },
  { id: 'preference', label: 'preference' },
  { id: 'insight', label: 'insight' },
  { id: 'infra', label: 'infra' },
  { id: 'general', label: 'general' },
]

const SORT_OPTIONS = [
  { value: 'weight', label: '按权重置顶优先' },
  { value: 'recalled', label: '按高频召回次数' },
  { value: 'recent', label: '按最近更新时间' },
]

const duplicateFactExists = computed(() => {
  const k = factKey.value.trim().toLowerCase()
  if (!k) return false
  return store.state.memories.some(
    (m) => (m.scope || 'global') === scope.value && m.fact_key.toLowerCase() === k
  )
})

const filteredMemories = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const list = store.state.memories.filter((m) => {
    const mScope = m.scope || 'global'
    if (filterScope.value !== 'all' && mScope !== filterScope.value) return false
    if (filterCat.value !== 'all' && m.category !== filterCat.value) return false
    if (pinnedOnly.value && !m.pinned) return false
    if (coreOnly.value && (m.importance || 3) < 4) return false
    if (!q) return true
    return `${m.fact_key} ${m.content} ${m.category} ${mScope}`.toLowerCase().includes(q)
  })
  return [...list].sort((a, b) => {
    if (sortBy.value === 'recent') {
      return (b.updated_at || '').localeCompare(a.updated_at || '')
    }
    if (sortBy.value === 'recalled') {
      const diff = (b.recall_count || 0) - (a.recall_count || 0)
      if (diff !== 0) return diff
      return (b.importance || 3) - (a.importance || 3)
    }
    const pinDiff = (b.pinned ? 1 : 0) - (a.pinned ? 1 : 0)
    if (pinDiff !== 0) return pinDiff
    const impDiff = (b.importance || 3) - (a.importance || 3)
    if (impDiff !== 0) return impDiff
    return (b.updated_at || '').localeCompare(a.updated_at || '')
  })
})

function scopeLabel(raw?: string): string {
  if (!raw || raw === 'global') return '全局共享'
  if (raw.startsWith('bot:')) {
    const id = raw.slice(4)
    const b = store.state.bots.find((x) => x.id === id)
    return b ? (b.is_group ? `群 · ${b.name}` : `Bot · ${b.name}`) : raw
  }
  return raw
}

function closeModal() {
  store.state.showMemoriesModal = false
  editingMemId.value = null
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
      importance: Number(importance.value) || 3,
      pinned: pinned.value,
    })
    factKey.value = ''
    content.value = ''
    pinned.value = false
    importance.value = 3
    store.notify('长期记忆已持久化写入')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '写入记忆失败')
  } finally {
    saving.value = false
  }
}

function startEdit(m: AgentMemory) {
  editingMemId.value = m.id
  editScope.value = m.scope || 'global'
  editContent.value = m.content
  editCategory.value = m.category || 'general'
  editImportance.value = m.importance || 3
  editPinned.value = Boolean(m.pinned)
}

async function saveEdit(m: AgentMemory) {
  if (!editContent.value.trim()) return
  try {
    await BotHubApi.updateMemory(m.id, {
      scope: editScope.value,
      category: editCategory.value,
      content: editContent.value.trim(),
      importance: Number(editImportance.value) || 3,
      pinned: editPinned.value,
    })
    editingMemId.value = null
    store.notify('记忆条目已更新')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '更新记忆失败')
  }
}

async function toggleScopePromote(m: AgentMemory) {
  const curScope = m.scope || 'global'
  const fallbackBotScope = store.activeBot.value ? `bot:${store.activeBot.value.id}` : ''
  const nextScope = curScope === 'global' ? (fallbackBotScope || 'global') : 'global'
  if (nextScope === curScope) return
  try {
    await BotHubApi.updateMemory(m.id, { scope: nextScope })
    store.notify(nextScope === 'global' ? '已提升为全局共享记忆' : '已收归当前 Bot 私有域')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '迁移作用域失败')
  }
}

function toggleSelectMem(id: number) {
  if (selectedIds.value.includes(id)) {
    selectedIds.value = selectedIds.value.filter((x) => x !== id)
  } else {
    selectedIds.value = [...selectedIds.value, id]
  }
}

function toggleSelectAllFiltered() {
  const visible = filteredMemories.value.map((m) => m.id)
  if (visible.length > 0 && visible.every((id) => selectedIds.value.includes(id))) {
    selectedIds.value = []
  } else {
    selectedIds.value = visible
  }
}

async function runBulkScope(targetScope: string) {
  if (!selectedIds.value.length || bulkBusy.value) return
  bulkBusy.value = true
  try {
    await Promise.all(selectedIds.value.map((id) => BotHubApi.updateMemory(id, { scope: targetScope })))
    selectedIds.value = []
    store.notify(targetScope === 'global' ? '已批量迁入全局共享记忆' : '已批量迁移作用域')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '批量迁移失败')
  } finally {
    bulkBusy.value = false
  }
}

async function runBulkDelete() {
  if (!selectedIds.value.length || bulkBusy.value) return
  if (!window.confirm(`确定删除已选中的 ${selectedIds.value.length} 条记忆吗？`)) return
  bulkBusy.value = true
  try {
    await Promise.all(selectedIds.value.map((id) => BotHubApi.deleteMemory(id)))
    selectedIds.value = []
    store.notify('已批量移除选中记忆')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '批量删除失败')
  } finally {
    bulkBusy.value = false
  }
}

function exportVisibleMemoriesMarkdown() {
  const list = filteredMemories.value
  if (list.length === 0) {
    store.notify('当前筛选视图暂无记忆可导出')
    return
  }
  const lines: string[] = [
    `# Vesper BotHub 记忆导出 (${filterScope.value === 'all' ? '全部作用域' : scopeLabel(filterScope.value)})`,
    '',
    `共 ${list.length} 条，导出于 ${new Date().toLocaleString()}`,
    '',
  ]
  for (const m of list) {
    lines.push(
      `- **[${m.fact_key}]** (${m.category} · ${scopeLabel(m.scope)} · 权重 W${m.importance || 3}${m.pinned ? ' · 置顶' : ''}): ${m.content}`
    )
  }
  const blob = new Blob([lines.join('\n') + '\n'], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `bothub_memories_${Date.now()}.md`
  a.click()
  URL.revokeObjectURL(url)
  store.notify('已导出记忆清单 Markdown')
}

async function togglePin(m: AgentMemory) {
  try {
    await BotHubApi.updateMemory(m.id, { pinned: !m.pinned })
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '切换置顶失败')
  }
}

async function copyMemoryContent(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    store.notify('已复制记忆内容')
  } catch {
    store.notify('复制失败')
  }
}

async function handleDelete(id: number) {
  try {
    await BotHubApi.deleteMemory(id)
    if (editingMemId.value === id) editingMemId.value = null
    store.notify('记忆条目已移除')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '删除记忆失败')
  }
}

async function handlePruneScope() {
  if (filterScope.value === 'all' || pruning.value) return
  if (!window.confirm(`确定整理「${scopeLabel(filterScope.value)}」作用域记忆，仅保留全部置顶记忆与综合权重最高的 25 条吗？`)) return
  pruning.value = true
  try {
    const res = await BotHubApi.pruneMemories(filterScope.value, 25)
    store.notify(res.removed > 0 ? `已归档清理 ${res.removed} 条冷门记忆` : '当前作用域记忆紧凑健康，无需清理')
    await Promise.all([store.loadDashboard(false), store.refreshActiveBot()])
  } catch (err: any) {
    store.notify(err.message || '整理记忆失败')
  } finally {
    pruning.value = false
  }
}

watch(
  () => store.state.showMemoriesModal,
  (open) => {
    if (open) {
      const defScope =
        store.state.memoryModalDefaultScope ||
        (store.activeBot.value ? `bot:${store.activeBot.value.id}` : 'global')
      scope.value = defScope
      filterScope.value = store.state.memoryModalDefaultScope ? defScope : 'all'
      store.state.memoryModalDefaultScope = ''
    }
  }
)

watch(
  () => [store.state.showMemoriesModal, store.state.memories.length],
  () => nextTick(() => applySiblingStaggerFallback(listRef.value))
)
</script>

<template>
  <BaseModal
    :open="store.state.showMemoriesModal"
    title="分域长期记忆库 (Scoped Memory)"
    subtitle="每个 Bot 与群聊均保有严格隔离的记忆域，支持权重排序、置顶必记与召回统计"
    max-width="3xl"
    @close="closeModal"
  >
    <form @submit.prevent="handleAdd" class="modal-section grid grid-cols-1 sm:grid-cols-4 gap-3 shrink-0">
      <div class="flex flex-col gap-1 text-xs text-slate-300">
        <span>可见作用域</span>
        <CustomSelect v-model="scope" :options="scopeOptions" aria-label="可见作用域" />
      </div>
      <div class="flex flex-col gap-1 text-xs text-slate-300">
        <span>记忆分类</span>
        <CustomSelect v-model="category" :options="CAT_OPTIONS" aria-label="记忆分类" />
      </div>
      <div class="flex flex-col gap-1 text-xs text-slate-300">
        <span>重要权重</span>
        <CustomSelect
          :model-value="String(importance)"
          :options="IMPORTANCE_OPTIONS"
          aria-label="重要权重"
          @update:model-value="importance = Number($event) || 3"
        />
      </div>
      <label class="flex flex-col gap-1 text-xs text-slate-300">
        <span>事实键名 (同域唯一)</span>
        <input
          v-model="factKey"
          type="text"
          placeholder="如 user_nickname"
          required
          class="min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-xs text-white focus:border-[#f2b880]/60"
        />
      </label>
      <div class="sm:col-span-4 flex flex-col gap-1.5">
        <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-2">
          <input
            v-model="content"
            type="text"
            placeholder="写一条给这个 Bot 或全局记住的事实、性格偏好、重要约定..."
            aria-label="记忆内容"
            required
            class="flex-1 min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-xs text-white focus:border-[#f2b880]/60"
          />
          <label class="inline-flex items-center gap-1.5 text-xs text-slate-300 px-2 cursor-pointer shrink-0">
            <input v-model="pinned" type="checkbox" class="accent-[#f2b880]" />
            <span>置顶核心记忆</span>
          </label>
          <button
            type="submit"
            :disabled="saving"
            class="min-h-[40px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50 shrink-0"
          >
            {{ duplicateFactExists ? '覆盖更新记忆' : '写入记忆' }}
          </button>
        </div>
        <span v-if="duplicateFactExists" class="text-[11px] font-mono text-amber-300/90">
          提示：同作用域内已存在键名「{{ factKey.trim() }}」，提交后将覆写原有内容
        </span>
      </div>
    </form>

    <div class="flex flex-wrap items-center justify-between gap-2 shrink-0">
      <div class="flex flex-wrap items-center gap-2 flex-1 min-w-0">
        <input
          v-model="searchQuery"
          type="search"
          placeholder="搜索键名、内容或 Bot 专属域..."
          aria-label="搜索长期记忆"
          class="w-full sm:w-52 min-h-[34px] rounded-xl border border-[#fff8ee]/12 bg-[#060914] px-3 py-1.5 text-xs text-white placeholder-slate-500 focus:border-[#f2b880]/60"
        />
        <div class="w-44">
          <CustomSelect v-model="filterScope" :options="filterScopeOptions" aria-label="按作用域筛选" />
        </div>
        <button
          v-if="store.activeBot.value"
          type="button"
          @click="filterScope = filterScope === `bot:${store.activeBot.value.id}` ? 'all' : `bot:${store.activeBot.value.id}`"
          :class="[
            'min-h-[34px] px-2.5 rounded-xl border text-[11px] cursor-pointer transition-colors',
            filterScope === `bot:${store.activeBot.value.id}`
              ? 'border-cyan-400/55 bg-cyan-500/15 text-cyan-200'
              : 'border-[#fff8ee]/12 text-slate-400 hover:text-white'
          ]"
        >
          只看当前Bot
        </button>
        <div class="w-36">
          <CustomSelect v-model="sortBy" :options="SORT_OPTIONS" aria-label="排序方式" />
        </div>
        <button
          type="button"
          :aria-pressed="pinnedOnly"
          @click="pinnedOnly = !pinnedOnly"
          :class="[
            'min-h-[34px] px-2.5 rounded-xl border text-[11px] cursor-pointer transition-colors',
            pinnedOnly
              ? 'border-amber-400/55 bg-amber-400/15 text-amber-200'
              : 'border-[#fff8ee]/12 text-slate-400 hover:text-white'
          ]"
        >
          仅置顶
        </button>
        <button
          type="button"
          :aria-pressed="coreOnly"
          @click="coreOnly = !coreOnly"
          :class="[
            'min-h-[34px] px-2.5 rounded-xl border text-[11px] cursor-pointer transition-colors',
            coreOnly
              ? 'border-[#f2b880]/55 bg-[#f2b880]/15 text-[#f2b880]'
              : 'border-[#fff8ee]/12 text-slate-400 hover:text-white'
          ]"
        >
          核心 W4+
        </button>
        <button
          v-if="filterScope !== 'all' && filteredMemories.length > 25"
          type="button"
          :disabled="pruning"
          @click="handlePruneScope"
          class="min-h-[34px] px-3 rounded-xl border border-[#f2b880]/35 text-[11px] text-[#f2b880] hover:bg-[#f2b880]/15 cursor-pointer disabled:opacity-50"
        >
          {{ pruning ? '整理中...' : '清理冷门记忆' }}
        </button>
      </div>
      <SegmentedTabs v-model="filterCat" :items="FILTER_TABS" variant="subtle" aria-label="记忆分类筛选" />
    </div>

    <div class="flex items-center justify-between gap-2 flex-wrap px-1 text-[11px] font-mono text-slate-400 shrink-0">
      <div class="flex items-center gap-2.5 flex-wrap">
        <span>当前视图 {{ memoryStats.total }} 条 · 置顶 {{ memoryStats.pinned }} 条 · 累计召回 {{ memoryStats.recalls }} 次</span>
        <button
          v-if="filteredMemories.length > 0"
          type="button"
          @click="toggleSelectAllFiltered"
          class="text-cyan-300 hover:underline cursor-pointer"
        >
          {{ filteredMemories.every((m) => selectedIds.includes(m.id)) ? '取消全选' : '全选本页' }}
        </button>
      </div>
      <div class="flex items-center gap-2">
        <template v-if="selectedIds.length > 0">
          <button
            type="button"
            :disabled="bulkBusy"
            @click="runBulkScope('global')"
            class="px-2 py-0.5 rounded border border-cyan-400/40 text-cyan-200 hover:bg-cyan-500/12 cursor-pointer disabled:opacity-50"
          >
            批量升全局 ({{ selectedIds.length }})
          </button>
          <button
            v-if="store.activeBot.value"
            type="button"
            :disabled="bulkBusy"
            @click="runBulkScope(`bot:${store.activeBot.value.id}`)"
            class="px-2 py-0.5 rounded border border-[#f2b880]/40 text-[#f2b880] hover:bg-[#f2b880]/12 cursor-pointer disabled:opacity-50"
          >
            批量收归当前Bot
          </button>
          <button
            type="button"
            :disabled="bulkBusy"
            @click="runBulkDelete"
            class="px-2 py-0.5 rounded border border-rose-400/40 text-rose-300 hover:bg-rose-500/12 cursor-pointer disabled:opacity-50"
          >
            批量移除
          </button>
        </template>
        <button
          type="button"
          @click="exportVisibleMemoriesMarkdown"
          class="text-slate-300 hover:text-[#f2b880] cursor-pointer"
        >
          导出 .md
        </button>
      </div>
    </div>

    <div ref="listRef" class="stagger-list flex-1 min-h-0 overflow-y-auto flex flex-col gap-2.5 pr-1">
      <div v-if="filteredMemories.length === 0" class="py-10 text-center flex flex-col items-center gap-2 text-sm text-slate-500">
        <span>{{ store.state.memories.length ? '当前筛选下无记忆条目' : '尚未写入长期记忆条目' }}</span>
        <button
          v-if="searchQuery || filterCat !== 'all' || filterScope !== 'all' || pinnedOnly || coreOnly"
          type="button"
          @click="searchQuery = ''; filterCat = 'all'; filterScope = 'all'; pinnedOnly = false; coreOnly = false"
          class="px-3 py-1 rounded-full border border-[#fff8ee]/12 text-xs text-slate-300 hover:text-[#f2b880] cursor-pointer"
        >
          重置记忆筛选
        </button>
      </div>
      <article
        v-for="m in filteredMemories"
        :key="m.id"
        @pointermove="trackSpotlight"
        class="stagger-item luxury-card rounded-xl p-3.5"
      >
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div class="flex items-center gap-1.5 min-w-0 flex-wrap">
            <input
              type="checkbox"
              :checked="selectedIds.includes(m.id)"
              @change="toggleSelectMem(m.id)"
              :aria-label="`选择记忆 ${m.fact_key}`"
              class="accent-[#f2b880] cursor-pointer shrink-0"
            />
            <span
              v-if="m.pinned"
              class="font-mono text-[10px] px-1.5 py-0.5 rounded border border-amber-400/50 bg-amber-400/15 text-amber-200 shrink-0"
            >
              置顶
            </span>
            <span class="font-mono text-[10px] px-1.5 py-0.5 rounded border border-[#fff8ee]/12 bg-[#fff8ee]/[0.04] text-slate-300 shrink-0">
              W{{ m.importance || 3 }}
            </span>
            <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-md border border-[#f2b880]/30 bg-[#f2b880]/10 text-[#f2b880] shrink-0">
              {{ m.category }}
            </span>
            <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-md border border-cyan-400/25 bg-cyan-500/10 text-cyan-200 shrink-0">
              {{ scopeLabel(m.scope) }}
            </span>
            <strong class="font-mono text-xs text-white truncate">{{ m.fact_key }}</strong>
          </div>
          <div class="flex items-center gap-1.5 shrink-0">
            <span v-if="m.recall_count" class="font-mono text-[10.5px] text-emerald-300/80">
              召回 {{ m.recall_count }} 次
            </span>
            <span class="font-mono text-[10.5px] text-slate-500">{{ formatTime(m.updated_at) }}</span>
            <button
              type="button"
              v-if="m.scope !== 'global' || store.activeBot.value"
              @click="toggleScopePromote(m)"
              class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 text-[11px] text-cyan-200/85 hover:text-cyan-200 hover:border-cyan-400/40 cursor-pointer"
            >
              {{ (m.scope || 'global') === 'global' ? '收归私有' : '升为全局' }}
            </button>
            <button
              type="button"
              @click="togglePin(m)"
              class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
            >
              {{ m.pinned ? '取消置顶' : '置顶' }}
            </button>
            <button
              type="button"
              @click="copyMemoryContent(m.content)"
              class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
            >
              复制
            </button>
            <button
              type="button"
              @click="editingMemId === m.id ? (editingMemId = null) : startEdit(m)"
              class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-white cursor-pointer"
            >
              {{ editingMemId === m.id ? '取消' : '编辑' }}
            </button>
            <button
              type="button"
              @click="handleDelete(m.id)"
              class="px-2 py-0.5 rounded-md border border-rose-400/35 text-[11px] text-rose-300 hover:bg-rose-500/15 cursor-pointer"
            >
              移除
            </button>
          </div>
        </div>

        <div v-if="editingMemId === m.id" class="mt-2.5 flex flex-col gap-2 pt-2 border-t border-[#fff8ee]/8">
          <div class="grid grid-cols-1 sm:grid-cols-4 gap-2">
            <CustomSelect v-model="editScope" :options="scopeOptions" aria-label="编辑记忆作用域" />
            <CustomSelect v-model="editCategory" :options="CAT_OPTIONS" aria-label="编辑记忆分类" />
            <CustomSelect
              :model-value="String(editImportance)"
              :options="IMPORTANCE_OPTIONS"
              aria-label="编辑记忆权重"
              @update:model-value="editImportance = Number($event) || 3"
            />
            <label class="inline-flex items-center gap-1.5 text-xs text-slate-300 px-2 cursor-pointer">
              <input v-model="editPinned" type="checkbox" class="accent-[#f2b880]" />
              <span>置顶核心记忆</span>
            </label>
          </div>
          <div class="flex gap-2">
            <input
              v-model="editContent"
              type="text"
              placeholder="Enter 保存 · Esc 取消"
              @keydown.enter.prevent="saveEdit(m)"
              @keydown.escape.stop="editingMemId = null"
              class="flex-1 min-h-[34px] rounded-lg border border-[#f2b880]/40 bg-[#050811] px-2.5 text-xs text-white"
            />
            <button
              type="button"
              @click="saveEdit(m)"
              class="px-3 py-1 rounded-lg bg-[#f2b880] text-[#070a13] font-semibold text-xs cursor-pointer"
            >
              保存
            </button>
          </div>
        </div>
        <p v-else class="text-xs text-slate-200 mt-1.5 leading-relaxed">{{ m.content }}</p>
      </article>
    </div>
  </BaseModal>
</template>
