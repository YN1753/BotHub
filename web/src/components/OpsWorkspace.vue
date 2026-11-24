<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import {
  describeCron,
  formatDateOnly,
  summarizeAssetsMonthlyCost,
  summarizeAssetsYearlyCost,
} from '../utils/format'
import { todayPlainDate } from '../utils/calendar'
import { mergeExtensibleCategories, resolveCategoryMeta } from '../utils/assetCategories'
import { applySiblingStaggerFallback, trackSpotlight } from '../utils/topLayer'
import type { Asset, TodoItem, WorkNote } from '../types/bothub'
import SegmentedTabs from './SegmentedTabs.vue'
import BotAvatar from './BotAvatar.vue'
import WorkspaceSidebarControls from './ui/WorkspaceSidebarControls.vue'
import AssetCategoryIcon from './ui/AssetCategoryIcon.vue'

defineProps<{
  desktopCollapsed?: boolean
}>()

const emit = defineEmits<{
  openMobileMenu: []
  toggleDesktopSidebar: []
}>()

const store = useAppStore()
const activeHubTab = ref<'assets' | 'tasks' | 'journal'>('assets')

// Asset controls
const searchQuery = ref('')
const activeCat = ref('all')
const expiringOnly = ref(false)
const sortByUrgency = ref(false)

// Task controls
const taskSearch = ref('')
const taskStatusFilter = ref<'all' | 'enabled' | 'paused'>('all')
const runningTaskId = ref('')

// Inline Todos & Notes (Journal tab)
const newTodoTitle = ref('')
const newTodoPriority = ref<'normal' | 'high'>('normal')
const savingTodo = ref(false)
const showDoneTodos = ref(false)
const clearingDone = ref(false)
const showNoteEditor = ref(false)
const draftNoteId = ref('')
const draftNoteTitle = ref('')
const draftNoteCategory = ref('deliverable')
const draftNoteContent = ref('')
const draftNotePinned = ref(false)
const savingNote = ref(false)

const assetGridRef = ref<HTMLElement | null>(null)
const taskGridRef = ref<HTMLElement | null>(null)

const todayPrefix = computed(() => todayPlainDate())

const openTodos = computed(() => store.state.todos.filter((t) => t.status !== 'done'))
const doneTodos = computed(() => store.state.todos.filter((t) => t.status === 'done'))
const sortedNotes = computed(() =>
  [...store.state.notes].sort((a, b) => {
    if (Boolean(a.pinned) !== Boolean(b.pinned)) return a.pinned ? -1 : 1
    return String(b.updated_at || '').localeCompare(String(a.updated_at || ''))
  })
)

// Dynamic Today Focus Strip: ONLY visible when there are <=14d expiring assets or today/overdue/high-priority open todos
const focusAssets = computed(() =>
  store.state.assets
    .filter((a) => {
      const isPerm = a.status === '长期有效' || String(a.due_date || '').startsWith('2099') || a.days_left > 3650
      return !isPerm && a.status !== '已停用' && a.status !== '已归档' && a.days_left <= 14
    })
    .toSorted((a, b) => (a.days_left ?? 9999) - (b.days_left ?? 9999))
)

const focusTodos = computed(() =>
  openTodos.value
    .filter((t) => {
      const due = String(t.due_at || '').trim().slice(0, 10)
      const urgentDate = due.length === 10 && due <= todayPrefix.value
      return urgentDate || t.priority === 'high'
    })
    .toSorted((a, b) => {
      const rank = (t: TodoItem) => {
        const due = String(t.due_at || '').trim().slice(0, 10)
        if (due.length === 10 && due < todayPrefix.value) return 0
        if (due.length === 10 && due === todayPrefix.value) return 1
        if (t.priority === 'high') return 2
        return 3
      }
      return rank(a) - rank(b)
    })
)

const hasFocusItems = computed(() => focusAssets.value.length > 0 || focusTodos.value.length > 0)

const hubTabs = computed(() => [
  { id: 'assets', label: store.state.assets.length > 0 ? `资产 ${store.state.assets.length}` : '资产' },
  { id: 'tasks', label: store.state.tasks.length > 0 ? `任务 ${store.state.tasks.length}` : '任务' },
  { id: 'journal', label: openTodos.value.length > 0 ? `待办与手记 ${openTodos.value.length}` : '待办与手记' },
])

const categoryTabs = computed(() => {
  const merged = mergeExtensibleCategories(store.state.assetCategories, store.state.assets)
  return [
    { id: 'all', label: `全部 ${store.state.assets.length}` },
    ...merged.map((c) => {
      const cnt = store.state.assets.filter((a) => a.category === c.id).length
      return { id: c.id, label: cnt > 0 ? `${c.label} ${cnt}` : c.label }
    }),
  ]
})

function assetCategoryMeta(cat: string) {
  return resolveCategoryMeta(cat, store.state.assetCategories)
}

function isExpiringSoon(a: Asset): boolean {
  if (a.status === '已停用' || String(a.due_date || '').startsWith('2099')) return false
  return a.days_left <= 7
}

function assetStatusBadgeClass(status?: string): string {
  if (status === '已到期') return 'border-danger-border bg-danger-soft text-danger'
  if (status === '已停用') return 'border-line bg-subtle text-fg-muted'
  return 'border-ok-border bg-ok-soft text-ok'
}

const filteredAssets = computed(() => {
  const list = store.state.assets.filter((a) => {
    if (activeCat.value !== 'all' && a.category !== activeCat.value) return false
    if (expiringOnly.value && !isExpiringSoon(a)) return false
    const q = searchQuery.value.trim().toLowerCase()
    if (!q) return true
    return `${a.name} ${a.spec} ${a.status} ${a.extra_config}`.toLowerCase().includes(q)
  })
  if (!sortByUrgency.value) return list
  return list.toSorted((a, b) => {
    const aPerm = a.status === '长期有效' || String(a.due_date || '').startsWith('2099')
    const bPerm = b.status === '长期有效' || String(b.due_date || '').startsWith('2099')
    if (aPerm !== bPerm) return aPerm ? 1 : -1
    return (a.days_left ?? 9999) - (b.days_left ?? 9999)
  })
})

const activeCostSummary = computed(() => summarizeAssetsMonthlyCost(filteredAssets.value, store.state.fxRates))
const activeYearlyCostSummary = computed(() => summarizeAssetsYearlyCost(filteredAssets.value, store.state.fxRates))

function resetAssetFilters() {
  searchQuery.value = ''
  activeCat.value = 'all'
  expiringOnly.value = false
}

function daysText(a: Asset): string {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) return '长期'
  if (a.days_left < 0) return `逾期 ${Math.abs(a.days_left)} 天`
  return `${a.days_left} 天`
}

function daysBadgeClass(a: Asset): string {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) return 'text-ok'
  if (a.days_left <= 3) return 'text-danger font-semibold'
  if (a.days_left <= 7) return 'text-warn font-semibold'
  return 'text-ok'
}

function assetRunwayPercent(a: Asset): number {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) return 100
  if (a.days_left <= 0) return 6
  return Math.max(8, Math.min(100, Math.round((a.days_left / 90) * 100)))
}

function assetRunwayBarClass(a: Asset): string {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) return 'bg-ok'
  if (a.days_left <= 3) return 'bg-danger'
  if (a.days_left <= 7) return 'bg-warn'
  return 'bg-accent'
}

function cardAccentClass(a: Asset): string {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) return 'border-l-[3px] border-l-ok'
  if (a.days_left <= 3) return 'border-l-[3px] border-l-danger'
  if (a.days_left <= 7) return 'border-l-[3px] border-l-warn'
  return 'border-l-[3px] border-l-accent-border'
}

const filteredTasks = computed(() =>
  store.state.tasks.filter((t) => {
    if (taskStatusFilter.value === 'enabled' && !t.enabled) return false
    if (taskStatusFilter.value === 'paused' && t.enabled) return false
    const q = taskSearch.value.trim().toLowerCase()
    if (!q) return true
    return `${t.name} ${t.description} ${t.cron_expr || ''} ${t.bot_id || ''}`.toLowerCase().includes(q)
  })
)

onMounted(() => {
  applySiblingStaggerFallback(assetGridRef.value)
  applySiblingStaggerFallback(taskGridRef.value)
})

watch(() => filteredAssets.value.length, () => nextTick(() => applySiblingStaggerFallback(assetGridRef.value)))
watch(() => filteredTasks.value.length, () => nextTick(() => applySiblingStaggerFallback(taskGridRef.value)))

function taskBot(botId?: string) {
  if (!botId) return undefined
  return store.state.bots.find((b) => b.id === botId)
}

async function runTaskNow(id: string) {
  if (runningTaskId.value) return
  runningTaskId.value = id
  try {
    const res = await BotHubApi.runTask(id)
    store.notify(res.summary || '执行完成')
    void store.refreshWorkspaceSlices(['tasks', 'notifications', 'bots'])
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    runningTaskId.value = ''
  }
}

async function toggleTaskEnabled(id: string, enabled: boolean) {
  const target = store.state.tasks.find((t) => t.id === id)
  const nextEnabled = !enabled
  if (target) target.enabled = nextEnabled
  store.notify(nextEnabled ? '任务已启用' : '任务已暂停')
  try {
    await BotHubApi.updateTask(id, { enabled: nextEnabled })
    void store.refreshWorkspaceSlices(['tasks'])
  } catch (err: any) {
    if (target) target.enabled = enabled
    store.notify(err.message)
  }
}

// Inline Todo actions
async function handleCreateTodo() {
  const title = newTodoTitle.value.trim()
  if (!title || savingTodo.value) return
  savingTodo.value = true
  try {
    const res = await BotHubApi.createTodo({
      title,
      priority: newTodoPriority.value,
      bot_id: store.activeBot.value?.id || '',
    })
    store.state.todos = [res.todo, ...store.state.todos.filter((x) => x.id !== res.todo.id)]
    newTodoTitle.value = ''
    newTodoPriority.value = 'normal'
    store.notify('已添加待办')
    void store.refreshWorkspaceSlices(['todos'])
  } catch (err: any) {
    store.notify(err.message || '添加待办失败')
  } finally {
    savingTodo.value = false
  }
}

async function toggleTodoDone(todo: TodoItem) {
  const prevStatus = todo.status
  const nextStatus = prevStatus === 'done' ? 'todo' : 'done'
  todo.status = nextStatus
  try {
    await BotHubApi.updateTodo(todo.id, { status: nextStatus })
    void store.refreshWorkspaceSlices(['todos'])
  } catch (err: any) {
    todo.status = prevStatus
    store.notify(err.message || '更新待办失败')
  }
}

async function deleteTodoItem(todo: TodoItem) {
  const prev = [...store.state.todos]
  store.state.todos = store.state.todos.filter((x) => x.id !== todo.id)
  try {
    await BotHubApi.deleteTodo(todo.id)
    store.notify('已删除待办')
    void store.refreshWorkspaceSlices(['todos'])
  } catch (err: any) {
    store.state.todos = prev
    store.notify(err.message || '删除待办失败')
  }
}

async function handleClearDoneTodos() {
  if (clearingDone.value || doneTodos.value.length === 0) return
  clearingDone.value = true
  try {
    const res = await BotHubApi.clearDoneTodos()
    store.state.todos = store.state.todos.filter((t) => t.status !== 'done')
    showDoneTodos.value = false
    store.notify(res.removed > 0 ? `已清理 ${res.removed} 项已完成待办` : '已清空已完成待办')
    void store.refreshWorkspaceSlices(['todos'])
  } catch (err: any) {
    store.notify(err.message || '清理失败')
  } finally {
    clearingDone.value = false
  }
}

function todoFocusBadge(t: TodoItem): { text: string; cls: string } {
  const due = String(t.due_at || '').trim().slice(0, 10)
  if (due.length === 10 && due < todayPrefix.value) {
    return { text: '已逾期', cls: 'border-danger-border bg-danger-soft text-danger' }
  }
  if (due.length === 10 && due === todayPrefix.value) {
    return { text: '今日', cls: 'border-warn-border bg-warn-soft text-warn' }
  }
  return { text: '高优', cls: 'border-danger-border bg-danger-soft text-danger' }
}

// Inline Note actions
function openNewNoteEditor() {
  draftNoteId.value = ''
  draftNoteTitle.value = ''
  draftNoteCategory.value = 'deliverable'
  draftNoteContent.value = ''
  draftNotePinned.value = false
  showNoteEditor.value = true
}

function openEditNoteEditor(note: WorkNote) {
  draftNoteId.value = note.id
  draftNoteTitle.value = note.title
  draftNoteCategory.value = note.category || 'deliverable'
  draftNoteContent.value = note.content || ''
  draftNotePinned.value = Boolean(note.pinned)
  showNoteEditor.value = true
}

async function handleSaveNote() {
  const title = draftNoteTitle.value.trim()
  if (!title || savingNote.value) return
  savingNote.value = true
  try {
    let saved: WorkNote
    if (draftNoteId.value) {
      const res = await BotHubApi.updateNote(draftNoteId.value, {
        title,
        category: draftNoteCategory.value,
        content: draftNoteContent.value,
        pinned: draftNotePinned.value,
      })
      saved = res.note
    } else {
      const res = await BotHubApi.createNote({
        bot_id: store.activeBot.value?.id || '',
        title,
        category: draftNoteCategory.value,
        content: draftNoteContent.value,
        pinned: draftNotePinned.value,
      })
      saved = res.note
    }
    const idx = store.state.notes.findIndex((n) => n.id === saved.id)
    if (idx >= 0) store.state.notes.splice(idx, 1, saved)
    else store.state.notes = [saved, ...store.state.notes]
    showNoteEditor.value = false
    store.notify('手记已保存')
    void store.refreshWorkspaceSlices(['notes'])
  } catch (err: any) {
    store.notify(err.message || '保存手记失败')
  } finally {
    savingNote.value = false
  }
}

async function handleDeleteNote(note: WorkNote) {
  const prev = [...store.state.notes]
  store.state.notes = store.state.notes.filter((n) => n.id !== note.id)
  if (draftNoteId.value === note.id) showNoteEditor.value = false
  try {
    await BotHubApi.deleteNote(note.id)
    store.notify('手记已删除')
    void store.refreshWorkspaceSlices(['notes'])
  } catch (err: any) {
    store.state.notes = prev
    store.notify(err.message || '删除手记失败')
  }
}

function noteCategoryLabel(cat: string): string {
  const k = String(cat || '').toLowerCase()
  if (k === 'sop') return '运维手册'
  if (k === 'memo' || k === 'note') return '随手备忘'
  return '工作产出'
}
</script>

<template>
  <section class="flex-1 min-h-0 flex flex-col overflow-hidden">
    <header
      class="px-4 md:px-6 pt-[max(0.75rem,env(safe-area-inset-top,0.75rem))] pb-3 bg-sidebar border-b border-line flex items-center justify-between gap-3"
    >
      <div class="flex items-center gap-3 min-w-0">
        <WorkspaceSidebarControls
          :desktop-collapsed="desktopCollapsed"
          @open-mobile-menu="emit('openMobileMenu')"
          @toggle-desktop-sidebar="emit('toggleDesktopSidebar')"
        />
        <div class="min-w-0">
          <h1 class="text-base md:text-lg font-semibold text-fg truncate">中枢工作台</h1>
        </div>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <button
          type="button"
          @click="store.state.showLogsModal = true"
          title="查看系统推送与执行流水"
          class="btn-tactile min-h-[36px] sm:min-h-[32px] inline-flex items-center gap-1.5 px-3 rounded-full text-xs cursor-pointer"
        >
          <span class="w-1.5 h-1.5 rounded-full bg-info" aria-hidden="true" />
          <span>流水 {{ store.state.stats.sent_pushes }}</span>
        </button>
        <button
          type="button"
          @click="store.loadDashboard(false)"
          class="btn-tactile min-h-[36px] sm:min-h-[32px] px-3 rounded-full text-xs cursor-pointer"
        >
          刷新
        </button>
      </div>
    </header>

    <div class="cq-container flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
      <div class="max-w-5xl mx-auto flex flex-col gap-4">
        <!-- Dynamic Today Focus Strip: ONLY rendered when there are urgent assets (<=14d) or today/overdue/high todos -->
        <div
          v-if="hasFocusItems"
          class="vesper-card rounded-xl px-3.5 py-2.5 border-warn-border bg-warn-soft/20 flex flex-col gap-2"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="text-xs font-semibold text-warn">今日焦点</span>
            <span class="font-mono text-xs text-fg-muted">
              待处理 {{ focusAssets.length + focusTodos.length }} 项
            </span>
          </div>
          <div class="flex flex-col gap-1.5">
            <div
              v-for="a in focusAssets.slice(0, 4)"
              :key="`focus_asset_${a.id}`"
              @click="store.state.editingAsset = a"
              class="flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-lg bg-canvas/80 border border-line/70 hover:border-warn-border transition cursor-pointer"
            >
              <strong class="text-xs font-semibold text-fg truncate">{{ a.name }}</strong>
              <div class="flex items-center gap-2 shrink-0 text-xs">
                <span v-if="a.price" class="font-mono text-fg-muted shrink-0">{{ a.price }}</span>
                <span :class="['font-mono shrink-0', daysBadgeClass(a)]">{{ daysText(a) }}</span>
              </div>
            </div>
            <div
              v-for="td in focusTodos.slice(0, 4)"
              :key="`focus_todo_${td.id}`"
              class="flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-lg bg-canvas/80 border border-line/70"
            >
              <label class="flex items-center gap-2 min-w-0 flex-1 cursor-pointer">
                <input
                  type="checkbox"
                  :checked="td.status === 'done'"
                  @change="toggleTodoDone(td)"
                  class="w-3.5 h-3.5 accent-accent rounded cursor-pointer shrink-0"
                />
                <span class="text-xs font-medium text-fg truncate">{{ td.title }}</span>
              </label>
              <span :class="['font-mono text-[11px] px-1.5 py-0.5 rounded-md border shrink-0', todoFocusBadge(td).cls]">
                {{ todoFocusBadge(td).text }}
              </span>
            </div>
          </div>
        </div>

        <!-- 3-Section Clean Segmented Switcher -->
        <div class="flex items-center justify-between gap-3 flex-wrap">
          <SegmentedTabs v-model="activeHubTab" :items="hubTabs" aria-label="中枢板块切换" />
          <div v-if="activeHubTab === 'assets'" class="flex items-center gap-2 shrink-0">
            <button
              type="button"
              @click="store.state.editingAsset = null"
              class="btn-tactile-gold min-h-[34px] px-3.5 rounded-full text-xs font-medium cursor-pointer"
            >
              + 添加
            </button>
          </div>
          <div v-else-if="activeHubTab === 'tasks'" class="flex items-center gap-2 shrink-0">
            <button
              type="button"
              @click="store.state.editingTask = null"
              class="btn-tactile-gold min-h-[34px] px-3.5 rounded-full text-xs font-medium cursor-pointer"
            >
              + 新建任务
            </button>
          </div>
          <div v-else class="flex items-center gap-2 shrink-0">
            <button
              type="button"
              @click="openNewNoteEditor"
              class="btn-tactile-gold min-h-[34px] px-3.5 rounded-full text-xs font-medium cursor-pointer"
            >
              + 新建手记
            </button>
          </div>
        </div>

        <!-- TAB 1: Assets -->
        <div v-if="activeHubTab === 'assets'" class="flex flex-col gap-3.5">
          <!-- Single Compact Summary Bar (Unified RMB) -->
          <div class="vesper-card rounded-xl px-4 py-3 flex items-center justify-between gap-3 flex-wrap">
            <div class="flex items-baseline gap-2.5 flex-wrap min-w-0">
              <span class="font-mono tabular-nums text-base sm:text-lg font-bold text-accent-fg">
                月均 {{ activeCostSummary || '¥0/月' }}
              </span>
              <span v-if="activeYearlyCostSummary" class="font-mono tabular-nums text-xs text-fg-muted">
                年化 {{ activeYearlyCostSummary }}
              </span>
              <span class="text-xs text-fg-muted">· 显示 {{ filteredAssets.length }} 项</span>
              <button
                v-if="store.expiringAssetCount.value > 0"
                type="button"
                @click="expiringOnly = !expiringOnly"
                :class="[
                  'font-mono text-xs px-2 py-0.5 rounded-full border cursor-pointer transition',
                  expiringOnly ? 'border-warn bg-warn-soft text-warn font-semibold' : 'border-warn-border bg-warn-soft/60 text-warn'
                ]"
              >
                {{ store.expiringAssetCount.value }} 项临期
              </button>
            </div>
            <div class="flex items-center gap-1.5 shrink-0">
              <button
                type="button"
                :aria-pressed="sortByUrgency"
                @click="sortByUrgency = !sortByUrgency"
                :class="[
                  'min-h-[30px] px-2.5 rounded-full text-xs cursor-pointer',
                  sortByUrgency ? 'btn-tactile-gold' : 'btn-tactile text-fg-muted'
                ]"
              >
                按紧迫度
              </button>
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <input
              v-model="searchQuery"
              type="search"
              aria-label="搜索资产"
              placeholder="搜索资产名称、规格..."
              @keydown.escape="searchQuery = ''"
              class="w-full sm:w-48 min-h-[34px] rounded-full border border-line bg-canvas px-3.5 py-1 text-xs text-fg placeholder:text-fg-muted focus:border-accent focus:outline-none"
            />
            <SegmentedTabs v-model="activeCat" :items="categoryTabs" variant="subtle" aria-label="资产类别筛选" />
          </div>

          <div v-if="filteredAssets.length === 0" class="vesper-card rounded-xl p-8 text-center flex flex-col items-center gap-2.5 text-sm text-fg-muted">
            <span>{{ store.state.assets.length ? '当前筛选条件下无匹配资产' : '暂无追踪中的资产' }}</span>
            <button
              v-if="store.state.assets.length > 0"
              type="button"
              @click="resetAssetFilters"
              class="btn-tactile min-h-[32px] px-3.5 py-1 rounded-full text-xs cursor-pointer"
            >
              重置筛选条件
            </button>
          </div>

          <div ref="assetGridRef" class="ops-asset-grid stagger-list">
            <button
              v-for="a in filteredAssets"
              :key="a.id"
              type="button"
              @pointermove="trackSpotlight"
              @click="store.state.editingAsset = a"
              :class="[
                'group stagger-item vesper-card vesper-card-interactive rounded-xl p-3.5 flex flex-col gap-2.5 cursor-pointer text-left',
                cardAccentClass(a)
              ]"
            >
              <div class="flex items-start justify-between gap-2 w-full">
                <div class="flex items-center gap-2.5 min-w-0">
                  <span
                    :title="assetCategoryMeta(a.category).label"
                    class="w-7 h-7 rounded-lg border border-accent-border bg-accent-soft text-accent-fg inline-flex items-center justify-center shrink-0"
                  >
                    <AssetCategoryIcon :icon="assetCategoryMeta(a.category).icon" :size="14" />
                  </span>
                  <div class="min-w-0">
                    <div class="flex items-center gap-1.5 min-w-0">
                      <strong class="block text-sm font-semibold text-fg group-hover:text-accent-fg transition-colors truncate">{{ a.name }}</strong>
                      <span
                        v-if="a.status === '已到期' || a.status === '已停用'"
                        :class="['font-mono text-[11px] px-1.5 py-0.5 rounded-md border leading-none shrink-0', assetStatusBadgeClass(a.status)]"
                      >
                        {{ a.status }}
                      </span>
                    </div>
                    <span class="block text-xs text-fg-muted truncate">{{ a.spec || assetCategoryMeta(a.category).label }}</span>
                  </div>
                </div>
                <span class="font-mono tabular-nums text-xs font-semibold px-2 py-0.5 rounded-lg border border-accent-border bg-accent-soft text-accent-fg shrink-0">
                  {{ a.price || '--' }}
                </span>
              </div>

              <div class="flex flex-col gap-1.5 pt-2 border-t border-line w-full">
                <div class="well-recessed h-1 w-full rounded-full overflow-hidden" aria-hidden="true">
                  <div
                    :style="{ width: `${assetRunwayPercent(a)}%` }"
                    :class="['h-full rounded-full transition-[width] duration-300', assetRunwayBarClass(a)]"
                  />
                </div>
                <div class="flex items-center justify-between text-xs w-full gap-2">
                  <span class="font-mono tabular-nums text-fg-muted">
                    {{ formatDateOnly(a.due_date) }}<template v-if="a.auto_renew"> · 自动续费</template>
                  </span>
                  <span :class="['font-mono tabular-nums shrink-0', daysBadgeClass(a)]">{{ daysText(a) }}</span>
                </div>
              </div>
            </button>
          </div>
        </div>

        <!-- TAB 2: Tasks -->
        <div v-else-if="activeHubTab === 'tasks'" class="flex flex-col gap-3.5 pb-4">
          <div class="flex flex-wrap items-center justify-between gap-2.5">
            <div class="flex items-center gap-2 flex-wrap">
              <input
                v-if="store.state.tasks.length > 0"
                v-model="taskSearch"
                type="search"
                aria-label="搜索调度任务"
                placeholder="搜索任务名称、Cron 或关联 Bot..."
                @keydown.escape="taskSearch = ''"
                class="w-full sm:w-64 min-h-[34px] rounded-full border border-line bg-canvas px-3.5 py-1 text-xs text-fg placeholder:text-fg-muted focus:border-accent focus:outline-none"
              />
              <div class="well-recessed inline-flex rounded-lg p-0.5 text-xs">
                <button
                  type="button"
                  @click="taskStatusFilter = 'all'"
                  :class="['px-2.5 py-1 rounded-lg cursor-pointer transition-all', taskStatusFilter === 'all' ? 'btn-tactile-gold' : 'text-fg-muted hover:text-fg']"
                >
                  全部
                </button>
                <button
                  type="button"
                  @click="taskStatusFilter = 'enabled'"
                  :class="['px-2.5 py-1 rounded-lg cursor-pointer transition-all', taskStatusFilter === 'enabled' ? 'bg-ok-soft text-ok border border-ok-border' : 'text-fg-muted hover:text-fg']"
                >
                  已启用
                </button>
                <button
                  type="button"
                  @click="taskStatusFilter = 'paused'"
                  :class="['px-2.5 py-1 rounded-lg cursor-pointer transition-all', taskStatusFilter === 'paused' ? 'bg-hover text-fg' : 'text-fg-muted hover:text-fg']"
                >
                  已暂停
                </button>
              </div>
            </div>
            <span class="font-mono text-xs text-fg-muted">
              已启用 {{ store.state.tasks.filter((t) => t.enabled).length }} / {{ store.state.tasks.length }}
            </span>
          </div>

          <div v-if="filteredTasks.length === 0" class="vesper-card rounded-xl p-6 text-center flex flex-col items-center gap-2 text-xs text-fg-muted">
            <span>{{ store.state.tasks.length ? '当前筛选条件下无匹配调度任务' : '暂无自动化调度任务' }}</span>
            <button
              v-if="taskSearch.trim() || taskStatusFilter !== 'all'"
              type="button"
              @click="taskSearch = ''; taskStatusFilter = 'all'"
              class="btn-tactile px-3 py-1 rounded-full text-xs cursor-pointer"
            >
              重置任务筛选
            </button>
          </div>

          <div ref="taskGridRef" class="ops-task-grid stagger-list">
            <article
              v-for="t in filteredTasks"
              :key="t.id"
              @pointermove="trackSpotlight"
              class="stagger-item vesper-card vesper-card-interactive rounded-xl p-3.5 flex flex-col gap-2.5"
            >
              <div class="flex items-start justify-between gap-2.5">
                <div class="min-w-0">
                  <strong class="block text-sm font-semibold text-fg truncate">{{ t.name }}</strong>
                  <div class="font-mono text-xs text-fg-muted mt-0.5 truncate">
                    {{ t.run_at ? `单次提醒 ${t.run_at}` : describeCron(t.cron_expr) }}
                  </div>
                </div>
                <button
                  type="button"
                  @click="toggleTaskEnabled(t.id, t.enabled)"
                  :class="[
                    'inline-flex items-center gap-1.5 font-mono text-xs min-h-[28px] px-2.5 py-0.5 rounded-full border cursor-pointer shrink-0',
                    t.enabled ? 'border-ok-border bg-ok-soft text-ok' : 'border-line text-fg-muted'
                  ]"
                >
                  <span :class="['w-1.5 h-1.5 rounded-full', t.enabled ? 'bg-ok animate-pulse' : 'bg-fg-muted']" aria-hidden="true" />
                  {{ t.enabled ? '已启用' : '已暂停' }}
                </button>
              </div>
              <div class="flex items-center justify-between gap-2 pt-1 border-t border-line flex-wrap">
                <div class="flex items-center gap-1.5">
                  <button
                    type="button"
                    :disabled="runningTaskId === t.id"
                    @click="runTaskNow(t.id)"
                    class="btn-tactile-gold min-h-[30px] px-2.5 py-1 rounded-lg text-xs cursor-pointer disabled:opacity-60"
                  >
                    {{ runningTaskId === t.id ? '执行中...' : '立即执行' }}
                  </button>
                  <button
                    type="button"
                    @click="store.state.editingTask = t"
                    class="btn-tactile min-h-[30px] px-2.5 py-1 rounded-lg text-xs cursor-pointer"
                  >
                    编辑
                  </button>
                </div>
                <button
                  v-if="t.bot_id && taskBot(t.bot_id)"
                  type="button"
                  @click="store.jumpToBot(t.bot_id)"
                  class="btn-tactile min-h-[28px] inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs cursor-pointer"
                >
                  <BotAvatar :avatar-style="taskBot(t.bot_id)?.avatar_style" :state="taskBot(t.bot_id)?.state" size="xs" :interactive="false" />
                  <span>{{ taskBot(t.bot_id)?.name }}</span>
                </button>
              </div>
            </article>
          </div>
        </div>

        <!-- TAB 3: Combined Todos + Work Notes -->
        <div v-else class="flex flex-col gap-5 pb-6">
          <!-- Section 1: Open Todos -->
          <div class="flex flex-col gap-2.5">
            <div class="flex items-center justify-between gap-2">
              <h2 class="text-sm font-semibold text-fg">
                待办清单
                <span v-if="openTodos.length > 0" class="ml-1 font-mono text-xs text-fg-muted">{{ openTodos.length }} 项进行中</span>
              </h2>
            </div>

            <form @submit.prevent="handleCreateTodo" class="flex items-center gap-2">
              <input
                v-model="newTodoTitle"
                type="text"
                placeholder="输入待办事项，回车直接创建..."
                class="flex-1 min-h-[36px] rounded-lg border border-line bg-canvas px-3 py-1.5 text-xs text-fg placeholder:text-fg-muted focus:border-accent focus:outline-none"
              />
              <button
                type="button"
                @click="newTodoPriority = newTodoPriority === 'high' ? 'normal' : 'high'"
                :class="[
                  'min-h-[36px] px-3 rounded-lg text-xs border cursor-pointer transition shrink-0',
                  newTodoPriority === 'high' ? 'border-danger-border bg-danger-soft text-danger font-semibold' : 'border-line bg-canvas text-fg-muted'
                ]"
              >
                {{ newTodoPriority === 'high' ? '高优' : '普通' }}
              </button>
              <button
                type="submit"
                :disabled="!newTodoTitle.trim() || savingTodo"
                class="btn-tactile-gold min-h-[36px] px-3.5 rounded-lg text-xs font-medium cursor-pointer disabled:opacity-50 shrink-0"
              >
                添加
              </button>
            </form>

            <div v-if="openTodos.length === 0 && doneTodos.length === 0" class="vesper-card rounded-xl px-4 py-4 text-xs text-fg-muted">
              暂无待办事项，输入上方即可快速记录，或让 Bot 帮你拆解待办。
            </div>
            <div v-else-if="openTodos.length === 0" class="vesper-card rounded-xl px-4 py-3 text-xs text-fg-muted">
              当前待办已全部完成。
            </div>

            <div v-else class="flex flex-col gap-2">
              <div
                v-for="td in openTodos"
                :key="td.id"
                class="vesper-card rounded-xl px-3.5 py-2.5 flex items-center justify-between gap-3"
              >
                <label class="flex items-start gap-2.5 min-w-0 flex-1 cursor-pointer">
                  <input
                    type="checkbox"
                    :checked="false"
                    @change="toggleTodoDone(td)"
                    class="mt-0.5 w-4 h-4 accent-accent rounded cursor-pointer shrink-0"
                  />
                  <div class="min-w-0">
                    <div class="text-sm font-medium text-fg truncate">{{ td.title }}</div>
                    <div v-if="td.due_at || td.notes" class="text-xs text-fg-muted truncate mt-0.5">
                      <span v-if="td.due_at" class="font-mono">截止 {{ td.due_at.slice(0, 16) }}</span>
                      <span v-if="td.due_at && td.notes"> · </span>
                      <span v-if="td.notes">{{ td.notes }}</span>
                    </div>
                  </div>
                </label>
                <div class="flex items-center gap-2 shrink-0">
                  <span v-if="td.priority === 'high'" class="font-mono text-[11px] px-1.5 py-0.5 rounded-md border border-danger-border bg-danger-soft text-danger">
                    高优
                  </span>
                  <button
                    type="button"
                    @click="deleteTodoItem(td)"
                    class="text-xs text-fg-muted hover:text-danger transition cursor-pointer px-1"
                  >
                    删除
                  </button>
                </div>
              </div>
            </div>

            <!-- Collapsible Done Todos -->
            <div v-if="doneTodos.length > 0" class="flex flex-col gap-2 pt-1">
              <div class="flex items-center justify-between text-xs text-fg-muted">
                <button
                  type="button"
                  @click="showDoneTodos = !showDoneTodos"
                  class="hover:text-fg transition cursor-pointer"
                >
                  {{ showDoneTodos ? `收起已完成 (${doneTodos.length})` : `展开已完成 (${doneTodos.length})` }}
                </button>
                <button
                  type="button"
                  :disabled="clearingDone"
                  @click="handleClearDoneTodos"
                  class="hover:text-danger transition cursor-pointer disabled:opacity-50"
                >
                  清理已完成
                </button>
              </div>
              <div v-if="showDoneTodos" class="flex flex-col gap-1.5">
                <div
                  v-for="td in doneTodos"
                  :key="td.id"
                  class="vesper-card rounded-lg px-3.5 py-2 flex items-center justify-between gap-2 opacity-75"
                >
                  <label class="flex items-center gap-2.5 min-w-0 flex-1 cursor-pointer">
                    <input
                      type="checkbox"
                      checked
                      @change="toggleTodoDone(td)"
                      class="w-3.5 h-3.5 accent-ok rounded cursor-pointer shrink-0"
                    />
                    <span class="text-xs text-fg-muted line-through truncate">{{ td.title }}</span>
                  </label>
                  <button
                    type="button"
                    @click="deleteTodoItem(td)"
                    class="text-xs text-fg-muted hover:text-danger transition cursor-pointer"
                  >
                    删除
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Section 2: Work Notes -->
          <div class="flex flex-col gap-2.5 pt-2 border-t border-line">
            <div class="flex items-center justify-between gap-2">
              <h2 class="text-sm font-semibold text-fg">
                工作手记
                <span v-if="sortedNotes.length > 0" class="ml-1 font-mono text-xs text-fg-muted">{{ sortedNotes.length }} 篇</span>
              </h2>
            </div>

            <!-- Inline Note Editor -->
            <div v-if="showNoteEditor" class="vesper-card rounded-xl p-4 flex flex-col gap-3 border-accent-border">
              <div class="flex items-center gap-2 flex-wrap">
                <input
                  v-model="draftNoteTitle"
                  type="text"
                  placeholder="手记标题..."
                  class="flex-1 min-w-[200px] min-h-[34px] rounded-lg border border-line bg-canvas px-3 py-1 text-sm font-medium text-fg focus:border-accent focus:outline-none"
                />
                <select
                  v-model="draftNoteCategory"
                  class="min-h-[34px] rounded-lg border border-line bg-canvas px-2.5 text-xs text-fg focus:border-accent focus:outline-none"
                >
                  <option value="deliverable">工作产出</option>
                  <option value="memo">随手备忘</option>
                  <option value="sop">运维手册</option>
                </select>
                <label class="inline-flex items-center gap-1.5 text-xs text-fg-muted cursor-pointer px-1">
                  <input v-model="draftNotePinned" type="checkbox" class="accent-accent" />
                  <span>置顶</span>
                </label>
              </div>
              <textarea
                v-model="draftNoteContent"
                rows="6"
                placeholder="支持 Markdown 记录..."
                class="w-full rounded-lg border border-line bg-canvas p-3 text-xs text-fg font-mono leading-relaxed focus:border-accent focus:outline-none resize-y"
              />
              <div class="flex items-center justify-end gap-2">
                <button
                  type="button"
                  @click="showNoteEditor = false"
                  class="btn-tactile min-h-[32px] px-3 rounded-lg text-xs cursor-pointer"
                >
                  取消
                </button>
                <button
                  type="button"
                  :disabled="!draftNoteTitle.trim() || savingNote"
                  @click="handleSaveNote"
                  class="btn-tactile-gold min-h-[32px] px-3.5 rounded-lg text-xs font-medium cursor-pointer disabled:opacity-50"
                >
                  {{ savingNote ? '保存中...' : '保存手记' }}
                </button>
              </div>
            </div>

            <div v-if="sortedNotes.length === 0" class="vesper-card rounded-xl px-4 py-4 text-xs text-fg-muted">
              暂无工作手记，点击右上角「+ 新建手记」或让 Bot 整理产出文档。
            </div>

            <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-2.5">
              <div
                v-for="note in sortedNotes"
                :key="note.id"
                @click="openEditNoteEditor(note)"
                class="vesper-card vesper-card-interactive rounded-xl px-3.5 py-3 flex flex-col gap-1.5 cursor-pointer"
              >
                <div class="flex items-center justify-between gap-2">
                  <strong class="text-sm font-semibold text-fg truncate">{{ note.title }}</strong>
                  <div class="flex items-center gap-1.5 shrink-0">
                    <span v-if="note.pinned" class="font-mono text-[11px] px-1.5 py-0.5 rounded-md border border-accent-border bg-accent-soft text-accent-fg">
                      置顶
                    </span>
                    <span class="font-mono text-[11px] px-1.5 py-0.5 rounded-md border border-line bg-subtle text-fg-muted">
                      {{ noteCategoryLabel(note.category) }}
                    </span>
                    <button
                      type="button"
                      @click.stop="handleDeleteNote(note)"
                      class="text-xs text-fg-muted hover:text-danger transition cursor-pointer pl-1"
                    >
                      删除
                    </button>
                  </div>
                </div>
                <p class="text-xs text-fg-muted line-clamp-2 leading-relaxed">
                  {{ note.content || '点击查看或编辑正文' }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
