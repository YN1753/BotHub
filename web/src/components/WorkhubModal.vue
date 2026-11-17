<script setup lang="ts">
import { ref, computed, watch, nextTick, defineAsyncComponent } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { handleMarkdownCodeCopy, renderSafeMarkdown } from '../utils/markdown'
import { formatTime, triggerBlobDownload } from '../utils/format'
import type { TodoItem, WorkNote, WorkspaceBriefing } from '../types/bothub'
import BaseModal from './BaseModal.vue'
import CustomSelect from './CustomSelect.vue'
import SegmentedTabs from './SegmentedTabs.vue'

const WorkhubBriefingTab = defineAsyncComponent(() => import('./workhub/WorkhubBriefingTab.vue'))

const store = useAppStore()

const TAB_ITEMS = [
  { id: 'todos', label: '待办与行动清单' },
  { id: 'notes', label: '工作文档与交付区' },
  { id: 'briefing', label: '今日简报' },
]

const TODO_FILTERS = [
  { id: 'open', label: '未完成' },
  { id: 'all', label: '全部' },
  { id: 'done', label: '已完成' },
]

const PRIORITY_OPTIONS = [
  { value: 'high', label: '高优先' },
  { value: 'normal', label: '普通' },
  { value: 'low', label: '低优先' },
]

const activeTab = computed({
  get: () => store.state.workhubTab,
  set: (v: 'todos' | 'notes' | 'briefing') => {
    store.state.workhubTab = v
  },
})

// Todos state
const todoFilter = ref<'open' | 'all' | 'done'>('open')
const filterTodoBotId = ref('')
const todoSearch = ref('')
const urgentOnly = ref(false)
const dueTodayOnly = ref(false)
const newTodoTitle = ref('')
const newTodoPriority = ref<'high' | 'normal' | 'low'>('normal')
const newTodoDueAt = ref('')
const newTodoBotId = ref('')
const savingTodo = ref(false)
const todoInputRef = ref<HTMLInputElement | null>(null)
const noteTitleRef = ref<HTMLInputElement | null>(null)

// Inline Edit Todo state
const editingTodoId = ref<string | null>(null)
const editTodoTitle = ref('')
const editTodoNotes = ref('')
const editTodoPriority = ref<'high' | 'normal' | 'low'>('normal')
const editTodoDueAt = ref('')
const editTodoBotId = ref('')

// Notes state
const noteSearch = ref('')
const selectedNoteId = ref('')
const editingNote = ref(false)
const draftNoteId = ref('')
const draftNoteTitle = ref('')
const draftNoteCategory = ref('deliverable')
const draftNoteContent = ref('')
const draftNotePinned = ref(false)
const savingNote = ref(false)

// Briefing state
const briefing = ref<WorkspaceBriefing | null>(null)
const loadingBriefing = ref(false)

const botAssigneeOptions = computed(() => [
  { value: '', label: '未指定执行 Bot' },
  ...store.state.bots.map((b) => ({ value: b.id, label: b.name })),
])

const botFilterOptions = computed(() => [
  { value: '', label: '全部成员待办' },
  ...store.state.bots.map((b) => ({ value: b.id, label: b.name })),
])

const todayPrefix = computed(() => new Date().toISOString().slice(0, 10))
const doneTodoCount = computed(() => store.state.todos.filter((t) => t.status === 'done').length)

function quoteNoteToChat(n: WorkNote) {
  const targetBotId = n.bot_id || store.activeBot.value?.id || store.state.bots[0]?.id || ''
  if (!targetBotId) {
    store.notify('请先创建一个 Bot')
    return
  }
  const snippet = n.content ? `\n\n[参考文档：${n.title}]\n${n.content.slice(0, 600)}` : `\n\n[参考文档：${n.title}]`
  closeModal()
  void store.jumpToBot(targetBotId, snippet.trimStart())
}

function offsetDateStr(days: number): string {
  const d = new Date()
  d.setDate(d.getDate() + days)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

const filteredTodos = computed(() =>
  store.state.todos.filter((t) => {
    if (filterTodoBotId.value && (t.bot_id || '') !== filterTodoBotId.value) return false
    if (todoFilter.value === 'open' && t.status === 'done') return false
    if (todoFilter.value === 'done' && t.status !== 'done') return false
    if (urgentOnly.value && t.priority !== 'high' && !isOverdue(t)) return false
    if (dueTodayOnly.value && (!t.due_at || t.due_at.slice(0, 10) !== todayPrefix.value)) return false
    const q = todoSearch.value.trim().toLowerCase()
    if (!q) return true
    return `${t.title} ${t.notes || ''} ${t.due_at || ''}`.toLowerCase().includes(q)
  })
)

const filteredNotes = computed(() => {
  const q = noteSearch.value.trim().toLowerCase()
  return store.state.notes.filter((n) => {
    if (!q) return true
    return `${n.title} ${n.category} ${n.content}`.toLowerCase().includes(q)
  })
})

const selectedNote = computed<WorkNote | null>(() => {
  const list = filteredNotes.value
  if (list.length === 0) return null
  return list.find((n) => n.id === selectedNoteId.value) || list[0]
})

const briefingPriorityTodos = computed(() => {
  if (!briefing.value) return []
  const seen = new Set<string>()
  const merged = [
    ...briefing.value.overdue_todos,
    ...briefing.value.due_today_todos,
    ...briefing.value.high_priority_todos,
  ]
  const out: TodoItem[] = []
  for (const td of merged) {
    if (!seen.has(td.id)) {
      seen.add(td.id)
      out.push(td)
      if (out.length >= 6) break
    }
  }
  return out
})

async function loadBriefing() {
  if (loadingBriefing.value) return
  loadingBriefing.value = true
  try {
    const res = await BotHubApi.getBriefing()
    briefing.value = res.briefing
  } catch (err: any) {
    store.notify(err.message || '加载今日简报失败')
  } finally {
    loadingBriefing.value = false
  }
}

watch(
  () => [store.state.showWorkhubModal, activeTab.value] as const,
  ([open, tab]) => {
    if (!open) return
    if (!newTodoBotId.value && store.activeBot.value) {
      newTodoBotId.value = store.activeBot.value.id
    }
    if (!selectedNoteId.value && store.state.notes.length > 0) {
      selectedNoteId.value = store.state.notes[0].id
    }
    if (tab === 'briefing' || !briefing.value) {
      loadBriefing()
    }
    if (tab === 'todos') {
      nextTick(() => todoInputRef.value?.focus())
    }
  }
)

function botNameById(botId?: string): string {
  if (!botId) return ''
  return store.state.bots.find((b) => b.id === botId)?.name || botId
}

function isOverdue(t: TodoItem): boolean {
  return Boolean(t.status !== 'done' && t.due_at && t.due_at.slice(0, 10) < todayPrefix.value)
}

function formatTodoDueLabel(t: TodoItem): string {
  if (!t.due_at) return ''
  const due = t.due_at.slice(0, 10)
  const diffDays = Math.round(
    (new Date(`${due}T00:00:00`).getTime() - new Date(`${todayPrefix.value}T00:00:00`).getTime()) / 86400000
  )
  if (Number.isNaN(diffDays)) return due
  if (t.status !== 'done') {
    if (diffDays < 0) return `已逾期 ${Math.abs(diffDays)} 天 (${due})`
    if (diffDays === 0) return `今天截止 (${due})`
    if (diffDays === 1) return `明天截止 (${due})`
  }
  return `截止 ${due}`
}

function closeModal() {
  store.state.showWorkhubModal = false
  editingNote.value = false
  editingTodoId.value = null
}

async function handleAddTodo() {
  if (!newTodoTitle.value.trim() || savingTodo.value) return
  savingTodo.value = true
  try {
    await BotHubApi.createTodo({
      title: newTodoTitle.value.trim(),
      priority: newTodoPriority.value,
      due_at: newTodoDueAt.value.trim(),
      bot_id: newTodoBotId.value,
      status: 'pending',
    })
    newTodoTitle.value = ''
    newTodoDueAt.value = ''
    newTodoPriority.value = 'normal'
    await store.loadDashboard(false)
    if (activeTab.value === 'briefing') await loadBriefing()
    store.notify('待办已添加至个人行动清单')
  } catch (err: any) {
    store.notify(err.message || '添加待办失败')
  } finally {
    savingTodo.value = false
  }
}

function startEditTodo(t: TodoItem) {
  editingTodoId.value = t.id
  editTodoTitle.value = t.title
  editTodoNotes.value = t.notes || ''
  editTodoPriority.value = (t.priority as 'high' | 'normal' | 'low') || 'normal'
  editTodoDueAt.value = t.due_at || ''
  editTodoBotId.value = t.bot_id || ''
}

async function saveEditTodo(t: TodoItem) {
  if (!editTodoTitle.value.trim()) return
  try {
    await BotHubApi.updateTodo(t.id, {
      title: editTodoTitle.value.trim(),
      notes: editTodoNotes.value.trim(),
      priority: editTodoPriority.value,
      due_at: editTodoDueAt.value.trim(),
      bot_id: editTodoBotId.value,
    })
    editingTodoId.value = null
    await store.loadDashboard(false)
    store.notify('待办已更新')
  } catch (err: any) {
    store.notify(err.message || '更新待办失败')
  }
}

async function toggleTodoDone(t: TodoItem) {
  const nextStatus = t.status === 'done' ? 'pending' : 'done'
  try {
    await BotHubApi.updateTodo(t.id, { status: nextStatus })
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message || '更新状态失败')
  }
}

async function cycleTodoProgress(t: TodoItem) {
  const next = t.status === 'pending' ? 'doing' : t.status === 'doing' ? 'done' : 'pending'
  try {
    await BotHubApi.updateTodo(t.id, { status: next })
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message || '更新状态失败')
  }
}

async function removeTodo(id: string) {
  try {
    await BotHubApi.deleteTodo(id)
    if (editingTodoId.value === id) editingTodoId.value = null
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message || '删除失败')
  }
}

async function clearDoneTodos() {
  try {
    const res = await BotHubApi.clearDoneTodos()
    await store.loadDashboard(false)
    store.notify(res.removed > 0 ? `已清理 ${res.removed} 项已完成待办` : '暂无已完成待办可清理')
  } catch (err: any) {
    store.notify(err.message || '清理失败')
  }
}

async function dispatchTodoToBot(t: TodoItem) {
  const targetBotId =
    t.bot_id ||
    store.activeBot.value?.id ||
    store.state.bots[0]?.id ||
    ''
  if (!targetBotId) {
    store.notify('请先创建一个可执行 Bot')
    return
  }
  const detail = [
    `请帮我推进这条个人待办（ID: ${t.id}）：`,
    `标题：${t.title}`,
    t.priority ? `优先级：${t.priority}` : '',
    t.due_at ? `截止时间：${t.due_at}` : '',
    t.notes ? `补充说明：${t.notes}` : '',
    '请直接给出可落地步骤、协助调研或起草成果；如果已完成请帮我更新待办状态。',
  ]
    .filter(Boolean)
    .join('\n')
  closeModal()
  await store.jumpToBot(targetBotId, detail)
}

async function planTodayWithBot() {
  const targetBotId =
    store.activeBot.value?.id ||
    store.state.bots.find((b) => !b.is_group && b.bot_mode !== 'companion')?.id ||
    store.state.bots[0]?.id ||
    ''
  if (!targetBotId) {
    store.notify('请先创建一个可执行 Bot')
    return
  }
  const prompt = '请调用 workspace_briefing 汇总我今天的全部待办、临期资产与未读通知，排出高优推进顺序并给出可直接开干的行动方案。'
  closeModal()
  await store.jumpToBot(targetBotId, prompt)
}

function startNewNote() {
  draftNoteId.value = ''
  draftNoteTitle.value = ''
  draftNoteCategory.value = 'deliverable'
  draftNoteContent.value = ''
  draftNotePinned.value = false
  editingNote.value = true
  nextTick(() => noteTitleRef.value?.focus())
}

function startEditNote(n: WorkNote) {
  draftNoteId.value = n.id
  draftNoteTitle.value = n.title
  draftNoteCategory.value = n.category || 'deliverable'
  draftNoteContent.value = n.content
  draftNotePinned.value = Boolean(n.pinned)
  editingNote.value = true
  nextTick(() => noteTitleRef.value?.focus())
}

function handleNoteMarkdownClick(e: MouseEvent) {
  handleMarkdownCodeCopy(e, () => store.notify('代码块已复制'))
}

function openAssetOpsFromBriefing() {
  closeModal()
  store.state.workspace = 'ops'
}

async function handleSaveNote() {
  if (!draftNoteTitle.value.trim() || savingNote.value) return
  savingNote.value = true
  try {
    let saved: WorkNote
    if (draftNoteId.value) {
      const res = await BotHubApi.updateNote(draftNoteId.value, {
        title: draftNoteTitle.value.trim(),
        category: draftNoteCategory.value.trim() || 'deliverable',
        content: draftNoteContent.value,
        pinned: draftNotePinned.value,
      })
      saved = res.note
    } else {
      const res = await BotHubApi.createNote({
        bot_id: store.activeBot.value?.id || '',
        title: draftNoteTitle.value.trim(),
        category: draftNoteCategory.value.trim() || 'deliverable',
        content: draftNoteContent.value,
        pinned: draftNotePinned.value,
      })
      saved = res.note
    }
    editingNote.value = false
    selectedNoteId.value = saved.id
    await store.loadDashboard(false)
    store.notify('工作文档已保存')
  } catch (err: any) {
    store.notify(err.message || '保存文档失败')
  } finally {
    savingNote.value = false
  }
}

async function removeNote(id: string) {
  if (!window.confirm('确定删除这份工作文档吗？')) return
  try {
    await BotHubApi.deleteNote(id)
    if (selectedNoteId.value === id) selectedNoteId.value = ''
    editingNote.value = false
    await store.loadDashboard(false)
    store.notify('工作文档已删除')
  } catch (err: any) {
    store.notify(err.message || '删除文档失败')
  }
}

async function copyNoteMarkdown(n: WorkNote) {
  try {
    await navigator.clipboard.writeText(`# ${n.title}\n\n${n.content}`)
    store.notify('已复制文档 Markdown')
  } catch {
    store.notify('复制失败')
  }
}

function downloadNoteMarkdown(n: WorkNote) {
  const blob = new Blob([`# ${n.title}\n\n${n.content}\n`], { type: 'text/markdown;charset=utf-8' })
  triggerBlobDownload(blob, `${n.title.replace(/[^\w\u4e00-\u9fa5-]+/g, '_')}.md`)
}

async function refineNoteWithBot(n: WorkNote) {
  const targetBotId = n.bot_id || store.activeBot.value?.id || store.state.bots[0]?.id || ''
  if (!targetBotId) {
    store.notify('请先创建一个 Bot')
    return
  }
  const prompt = `请读取工作文档《${n.title}》（id=${n.id}），帮我按当前需求进一步完善结构并直接通过 manage_notes 更新保存：\n\n[当前正文]\n${n.content.slice(0, 1200)}`
  closeModal()
  await store.jumpToBot(targetBotId, prompt)
}
</script>

<template>
  <BaseModal
    :open="store.state.showWorkhubModal"
    title="个人行动台 · 待办、文档与今日简报"
    subtitle="助手帮你拆解落地的待办事项、Markdown 交付文档与今日全局聚焦面板"
    max-width="3xl"
    @close="closeModal"
  >
    <div class="flex items-center justify-between gap-2 flex-wrap shrink-0">
      <SegmentedTabs v-model="activeTab" :items="TAB_ITEMS" aria-label="行动台视图切换" />
      <span class="font-mono text-[11px] text-slate-400">
        待办 {{ store.pendingTodoCount.value }} 项 · 文档 {{ store.state.notes.length }} 份
      </span>
    </div>

    <!-- Tab 1: Personal Todos -->
    <div v-if="activeTab === 'todos'" class="flex-1 min-h-0 flex flex-col gap-3">
      <form @submit.prevent="handleAddTodo" class="modal-section flex flex-col gap-2 shrink-0">
        <div class="grid grid-cols-1 sm:grid-cols-12 gap-2">
          <input
            ref="todoInputRef"
            v-model="newTodoTitle"
            type="text"
            required
            placeholder="记录要办的事，或直接让对话里的助手用 manage_todos 帮你记..."
            class="sm:col-span-5 min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 text-xs text-white focus:border-[#f2b880]/60"
          />
          <div class="sm:col-span-2">
            <CustomSelect v-model="newTodoPriority" :options="PRIORITY_OPTIONS" compact aria-label="优先级" />
          </div>
          <input
            v-model="newTodoDueAt"
            type="text"
            placeholder="截止 YYYY-MM-DD"
            class="sm:col-span-2 min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-2.5 font-mono text-xs text-white focus:border-[#f2b880]/60"
          />
          <div class="sm:col-span-2">
            <CustomSelect v-model="newTodoBotId" :options="botAssigneeOptions" compact aria-label="指派 Bot" />
          </div>
          <button
            type="submit"
            :disabled="savingTodo"
            class="sm:col-span-1 min-h-[38px] rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50"
          >
            新增
          </button>
        </div>
        <div class="flex items-center gap-1.5 flex-wrap text-[11px] text-slate-400">
          <span class="font-mono text-[10.5px] text-slate-500">快捷截止:</span>
          <button
            type="button"
            @click="newTodoDueAt = offsetDateStr(0)"
            class="px-2 py-0.5 rounded-md border border-[#fff8ee]/10 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer"
          >
            今天
          </button>
          <button
            type="button"
            @click="newTodoDueAt = offsetDateStr(1)"
            class="px-2 py-0.5 rounded-md border border-[#fff8ee]/10 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer"
          >
            明天
          </button>
          <button
            type="button"
            @click="newTodoDueAt = offsetDateStr(3)"
            class="px-2 py-0.5 rounded-md border border-[#fff8ee]/10 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer"
          >
            3天后
          </button>
          <button
            type="button"
            @click="newTodoDueAt = offsetDateStr(7)"
            class="px-2 py-0.5 rounded-md border border-[#fff8ee]/10 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer"
          >
            一周内
          </button>
        </div>
      </form>

      <div class="flex items-center justify-between gap-2 flex-wrap shrink-0">
        <div class="flex items-center gap-2 flex-wrap">
          <SegmentedTabs v-model="todoFilter" :items="TODO_FILTERS" variant="subtle" aria-label="待办状态筛选" />
          <div class="w-36">
            <CustomSelect v-model="filterTodoBotId" :options="botFilterOptions" compact aria-label="按负责 Bot 筛选" />
          </div>
          <button
            type="button"
            :aria-pressed="urgentOnly"
            @click="urgentOnly = !urgentOnly"
            :class="[
              'min-h-[32px] px-2.5 rounded-xl border text-[11px] cursor-pointer transition-colors',
              urgentOnly
                ? 'border-rose-400/50 bg-rose-500/15 text-rose-200'
                : 'border-[#fff8ee]/10 text-slate-400 hover:text-white'
            ]"
          >
            仅高优/逾期
          </button>
          <button
            type="button"
            :aria-pressed="dueTodayOnly"
            @click="dueTodayOnly = !dueTodayOnly"
            :class="[
              'min-h-[32px] px-2.5 rounded-xl border text-[11px] cursor-pointer transition-colors',
              dueTodayOnly
                ? 'border-[#f2b880]/55 bg-[#f2b880]/15 text-[#f2b880]'
                : 'border-[#fff8ee]/10 text-slate-400 hover:text-white'
            ]"
          >
            今日截止
          </button>
          <input
            v-model="todoSearch"
            type="search"
            aria-label="筛选待办"
            placeholder="筛选待办..."
            @keydown.escape.stop="todoSearch = ''"
            class="w-32 sm:w-40 min-h-[32px] rounded-xl border border-[#fff8ee]/10 bg-[#050811] px-2.5 text-[11px] text-white placeholder-slate-500 focus:border-[#f2b880]/55"
          />
        </div>
        <button
          type="button"
          :disabled="doneTodoCount === 0"
          @click="clearDoneTodos"
          class="px-3 py-1 rounded-lg border border-[#fff8ee]/10 text-[11px] text-slate-400 hover:text-rose-300 hover:border-rose-400/35 cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
        >
          清理已完成 ({{ doneTodoCount }})
        </button>
      </div>

      <div class="flex-1 min-h-0 overflow-y-auto flex flex-col gap-2 pr-1">
        <div v-if="filteredTodos.length === 0" class="py-12 text-center flex flex-col items-center gap-2 text-xs text-slate-500">
          <span>{{ store.state.todos.length ? '当前筛选条件无匹配待办' : '当前暂无待办；在任意支持工具的助手对话里说「帮我记一下待办」，也会自动出现在这里。' }}</span>
          <button
            v-if="todoSearch || urgentOnly || dueTodayOnly || filterTodoBotId"
            type="button"
            @click="todoSearch = ''; urgentOnly = false; dueTodayOnly = false; filterTodoBotId = ''"
            class="px-3 py-1 rounded-full border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] cursor-pointer"
          >
            重置待办筛选
          </button>
        </div>
        <article
          v-for="t in filteredTodos"
          :key="t.id"
          class="luxury-card rounded-xl px-3.5 py-2.5 flex flex-col gap-2"
        >
          <div class="flex items-start justify-between gap-3 flex-wrap sm:flex-nowrap">
            <div class="flex items-start gap-2.5 min-w-0 flex-1">
              <input
                type="checkbox"
                :checked="t.status === 'done'"
                @change="toggleTodoDone(t)"
                :aria-label="`完成 ${t.title}`"
                class="mt-1 accent-[#f2b880] cursor-pointer shrink-0"
              />
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-1.5 flex-wrap">
                  <span
                    :class="[
                      'text-sm font-medium',
                      t.status === 'done' ? 'line-through text-slate-500' : 'text-white',
                    ]"
                  >
                    {{ t.title }}
                  </span>
                  <span
                    v-if="t.priority === 'high'"
                    class="px-1.5 py-0.5 rounded font-mono text-[10px] border border-rose-400/45 bg-rose-500/12 text-rose-300"
                  >
                    高优
                  </span>
                  <span
                    v-if="t.status === 'doing'"
                    class="px-1.5 py-0.5 rounded font-mono text-[10px] border border-[#f2b880]/45 bg-[#f2b880]/12 text-[#f7cba0]"
                  >
                    进行中
                  </span>
                  <span
                    v-if="t.due_at"
                    :class="[
                      'px-1.5 py-0.5 rounded font-mono text-[10px] border',
                      isOverdue(t)
                        ? 'border-rose-400/45 bg-rose-500/15 text-rose-300'
                        : 'border-[#fff8ee]/12 text-slate-400',
                    ]"
                  >
                    {{ formatTodoDueLabel(t) }}
                  </span>
                  <span
                    v-if="t.bot_id"
                    class="px-1.5 py-0.5 rounded font-mono text-[10px] bg-cyan-500/10 text-cyan-200 border border-cyan-400/25"
                  >
                    {{ botNameById(t.bot_id) }}
                  </span>
                </div>
                <p v-if="t.notes" class="text-xs text-slate-400 mt-1 whitespace-pre-line">{{ t.notes }}</p>
              </div>
            </div>
            <div class="flex items-center gap-1.5 flex-wrap shrink-0">
              <button
                type="button"
                @click="cycleTodoProgress(t)"
                class="px-2 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
              >
                {{ t.status === 'pending' ? '推进中' : t.status === 'doing' ? '标完成' : '重开' }}
              </button>
              <button
                type="button"
                @click="editingTodoId === t.id ? (editingTodoId = null) : startEditTodo(t)"
                class="px-2 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-white cursor-pointer"
              >
                {{ editingTodoId === t.id ? '取消' : '编辑' }}
              </button>
              <button
                v-if="t.status !== 'done'"
                type="button"
                @click="dispatchTodoToBot(t)"
                class="px-2.5 py-1 rounded-lg border border-[#f2b880]/40 bg-[#f2b880]/12 text-[11px] text-[#f7cba0] hover:bg-[#f2b880]/22 cursor-pointer"
              >
                派 Bot 推进
              </button>
              <button
                type="button"
                @click="removeTodo(t.id)"
                class="px-2 py-1 rounded-lg border border-rose-400/30 text-[11px] text-rose-300 hover:bg-rose-500/15 cursor-pointer"
              >
                删
              </button>
            </div>
          </div>

          <div v-if="editingTodoId === t.id" class="pt-2 border-t border-[#fff8ee]/8 grid grid-cols-1 sm:grid-cols-12 gap-2">
            <input
              v-model="editTodoTitle"
              type="text"
              placeholder="待办标题"
              @keydown.enter.prevent="saveEditTodo(t)"
              @keydown.escape.stop="editingTodoId = null"
              class="sm:col-span-6 min-h-[34px] rounded-lg border border-[#f2b880]/40 bg-[#050811] px-2.5 text-xs text-white"
            />
            <div class="sm:col-span-2">
              <CustomSelect v-model="editTodoPriority" :options="PRIORITY_OPTIONS" compact aria-label="编辑优先级" />
            </div>
            <input
              v-model="editTodoDueAt"
              type="text"
              placeholder="截止 YYYY-MM-DD"
              @keydown.enter.prevent="saveEditTodo(t)"
              @keydown.escape.stop="editingTodoId = null"
              class="sm:col-span-2 min-h-[34px] rounded-lg border border-[#fff8ee]/12 bg-[#050811] px-2 font-mono text-xs text-white"
            />
            <div class="sm:col-span-2">
              <CustomSelect v-model="editTodoBotId" :options="botAssigneeOptions" compact aria-label="编辑指派 Bot" />
            </div>
            <input
              v-model="editTodoNotes"
              type="text"
              placeholder="补充步骤或备注 (Enter 保存 · Esc 取消)..."
              @keydown.enter.prevent="saveEditTodo(t)"
              @keydown.escape.stop="editingTodoId = null"
              class="sm:col-span-10 min-h-[34px] rounded-lg border border-[#fff8ee]/12 bg-[#050811] px-2.5 text-xs text-white"
            />
            <button
              type="button"
              @click="saveEditTodo(t)"
              class="sm:col-span-2 min-h-[34px] rounded-lg bg-[#f2b880] text-[#070a13] font-semibold text-xs cursor-pointer"
            >
              保存修改
            </button>
          </div>
        </article>
      </div>
    </div>

    <!-- Tab 2: Work Notes / Deliverables -->
    <div v-else-if="activeTab === 'notes'" class="flex-1 min-h-0 grid grid-cols-1 md:grid-cols-12 gap-3">
      <div class="md:col-span-4 min-h-0 flex flex-col gap-2 md:border-r md:border-[#fff8ee]/8 md:pr-2">
        <div class="flex items-center gap-2 shrink-0">
          <input
            v-model="noteSearch"
            type="search"
            placeholder="搜索文档..."
            class="flex-1 min-w-0 min-h-[34px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-2.5 text-xs text-white"
          />
          <button
            type="button"
            @click="startNewNote"
            class="min-h-[34px] px-3 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer shrink-0"
          >
            + 新文档
          </button>
        </div>

        <div class="flex-1 min-h-0 overflow-y-auto flex flex-col gap-1.5">
          <div v-if="filteredNotes.length === 0" class="py-8 text-center text-xs text-slate-500">
            暂无工作文档
          </div>
          <button
            v-for="n in filteredNotes"
            :key="n.id"
            type="button"
            @click="selectedNoteId = n.id; editingNote = false"
            :class="[
              'w-full text-left px-3 py-2 rounded-xl border transition-colors cursor-pointer',
              selectedNote?.id === n.id && !editingNote
                ? 'border-[#f2b880]/50 bg-[#f2b880]/12'
                : 'border-[#fff8ee]/8 bg-[#070b18]/60 hover:border-[#fff8ee]/20',
            ]"
          >
            <div class="flex items-center justify-between gap-1.5">
              <strong class="text-xs text-white truncate">{{ n.title }}</strong>
              <span v-if="n.pinned" class="font-mono text-[9.5px] text-amber-300 shrink-0">置顶</span>
            </div>
            <div class="flex items-center justify-between gap-2 mt-0.5 text-[10.5px] font-mono text-slate-400">
              <span>{{ n.category }}</span>
              <span>{{ formatTime(n.updated_at) }}</span>
            </div>
          </button>
        </div>
      </div>

      <div class="md:col-span-8 min-h-0 flex flex-col">
        <form v-if="editingNote" @submit.prevent="handleSaveNote" class="flex-1 min-h-0 flex flex-col gap-2.5">
          <div class="grid grid-cols-1 sm:grid-cols-12 gap-2 shrink-0">
            <input
              ref="noteTitleRef"
              v-model="draftNoteTitle"
              type="text"
              required
              placeholder="文档标题"
              class="sm:col-span-7 min-h-[36px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 text-xs text-white"
            />
            <input
              v-model="draftNoteCategory"
              type="text"
              placeholder="分类 (deliverable/plan)"
              class="sm:col-span-3 min-h-[36px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-2.5 font-mono text-xs text-white"
            />
            <label class="sm:col-span-2 inline-flex items-center justify-center gap-1.5 text-xs text-slate-300 cursor-pointer">
              <input v-model="draftNotePinned" type="checkbox" class="accent-[#f2b880]" />
              <span>置顶</span>
            </label>
          </div>
          <textarea
            v-model="draftNoteContent"
            placeholder="编写或粘贴 Markdown 文档正文（Cmd/Ctrl + Enter 快速保存）..."
            @keydown.meta.enter.prevent="handleSaveNote"
            @keydown.ctrl.enter.prevent="handleSaveNote"
            class="flex-1 min-h-[220px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] p-3 font-mono text-xs text-slate-100 resize-none"
          />
          <div class="flex items-center justify-between gap-2 shrink-0">
            <span class="font-mono text-[10.5px] text-slate-500">
              {{ draftNoteContent.length }} 字符 · {{ draftNoteContent ? draftNoteContent.split('\n').length : 0 }} 行
            </span>
            <div class="flex items-center gap-2">
              <button
                type="button"
                @click="editingNote = false"
                class="px-3.5 py-1.5 rounded-xl border border-[#fff8ee]/12 text-xs text-slate-300 hover:text-white cursor-pointer"
              >
                取消
              </button>
              <button
                type="submit"
                :disabled="savingNote"
                class="px-4 py-1.5 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50"
              >
                {{ savingNote ? '保存中...' : '保存文档' }}
              </button>
            </div>
          </div>
        </form>

        <div v-else-if="selectedNote" class="flex-1 min-h-0 flex flex-col gap-2.5">
          <div class="flex items-center justify-between gap-2 flex-wrap pb-2 border-b border-[#fff8ee]/8 shrink-0">
            <div class="min-w-0">
              <h3 class="text-sm font-semibold text-white truncate">{{ selectedNote.title }}</h3>
              <div class="text-[11px] font-mono text-slate-400 mt-0.5">
                {{ selectedNote.category }}
                <template v-if="selectedNote.bot_id"> · 来自 {{ botNameById(selectedNote.bot_id) }}</template>
                · 更新于 {{ formatTime(selectedNote.updated_at) }}
              </div>
            </div>
            <div class="flex items-center gap-1.5 flex-wrap shrink-0">
              <button
                type="button"
                @click="quoteNoteToChat(selectedNote)"
                class="px-2.5 py-1 rounded-lg border border-cyan-400/35 text-[11px] text-cyan-200 hover:bg-cyan-500/15 cursor-pointer"
              >
                引用进对话
              </button>
              <button
                type="button"
                @click="refineNoteWithBot(selectedNote)"
                class="px-2.5 py-1 rounded-lg border border-[#f2b880]/40 bg-[#f2b880]/12 text-[11px] text-[#f7cba0] hover:bg-[#f2b880]/22 cursor-pointer"
              >
                派 Bot 润色
              </button>
              <button
                type="button"
                @click="startEditNote(selectedNote)"
                class="px-2.5 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-white cursor-pointer"
              >
                编辑
              </button>
              <button
                type="button"
                @click="copyNoteMarkdown(selectedNote)"
                class="px-2.5 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
              >
                复制
              </button>
              <button
                type="button"
                @click="downloadNoteMarkdown(selectedNote)"
                class="px-2.5 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
              >
                下载 .md
              </button>
              <button
                type="button"
                @click="removeNote(selectedNote.id)"
                class="px-2.5 py-1 rounded-lg border border-rose-400/35 text-[11px] text-rose-300 hover:bg-rose-500/15 cursor-pointer"
              >
                删除
              </button>
            </div>
          </div>
          <div @click="handleNoteMarkdownClick" class="flex-1 min-h-0 overflow-y-auto pr-1 text-xs leading-relaxed">
            <div v-if="selectedNote.content" v-html="renderSafeMarkdown(selectedNote.content)" />
            <div v-else class="text-slate-500 py-8 text-center">文档正文为空，可点击「编辑」写入内容</div>
          </div>
        </div>

        <div v-else class="flex-1 flex flex-col items-center justify-center text-center px-6 text-xs text-slate-500 gap-2">
          <span>这里统一汇总你与各专属助手产出的报告、计划、邮件草稿与 Markdown 文档。</span>
          <button
            type="button"
            @click="startNewNote"
            class="px-3.5 py-1.5 rounded-xl border border-[#f2b880]/40 text-[#f2b880] hover:bg-[#f2b880]/12 cursor-pointer"
          >
            + 创建第一份工作文档
          </button>
        </div>
      </div>
    </div>

    <!-- Tab 3: Live Workspace Daily Briefing -->
    <WorkhubBriefingTab
      v-else
      :briefing="briefing"
      :loading-briefing="loadingBriefing"
      :priority-todos="briefingPriorityTodos"
      @refresh="loadBriefing"
      @plan-today="planTodayWithBot"
      @go-todos="activeTab = 'todos'"
      @go-notes="activeTab = 'notes'"
      @open-ops="openAssetOpsFromBriefing"
    />
  </BaseModal>
</template>
