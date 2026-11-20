<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import { useAppStore, type SideInspectorTab } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import { resolveScopedMemories } from '../../utils/botLook'
import { resolveCategoryMeta } from '../../utils/assetCategories'
import { addMonthsPlain, diffPlainDaysFromToday, todayPlainDate } from '../../utils/calendar'
import {
  formatDateOnly,
  formatTime,
  summarizeAssetsMonthlyCost,
  summarizeAssetsYearlyCost,
} from '../../utils/format'
import { handleMarkdownCodeCopy, renderSafeMarkdown } from '../../utils/markdown'
import type { AgentMemory, Asset, StashItem, TodoItem, WorkNote } from '../../types/bothub'
import AssetCategoryIcon from '../ui/AssetCategoryIcon.vue'

const emit = defineEmits<{
  quoteToChat: [text: string]
}>()

const store = useAppStore()

const TABS: Array<{ id: SideInspectorTab; label: string }> = [
  { id: 'todos', label: '待办' },
  { id: 'notes', label: '笔记' },
  { id: 'stash', label: '收藏' },
  { id: 'assets', label: '资产' },
  { id: 'memories', label: '记忆' },
]

const activeTab = computed({
  get: () => store.state.sideInspectorTab,
  set: (v: SideInspectorTab) => store.setSideInspectorTab(v),
})

// ---- Todos State ----
const newTodoTitle = ref('')
const newTodoPriority = ref<'high' | 'normal' | 'low'>('normal')
const todoShowAll = ref(false)
const savingTodo = ref(false)
const busyTodoId = ref('')

const todayPrefix = computed(() => todayPlainDate())
const visibleTodos = computed(() =>
  store.state.todos.filter((t) => (todoShowAll.value ? true : t.status !== 'done'))
)

function isTodoOverdue(t: TodoItem): boolean {
  return Boolean(t.status !== 'done' && t.due_at && t.due_at.slice(0, 10) < todayPrefix.value)
}

async function handleCreateTodo() {
  const title = newTodoTitle.value.trim()
  if (!title || savingTodo.value) return
  savingTodo.value = true
  newTodoTitle.value = ''
  try {
    const res = await BotHubApi.createTodo({
      bot_id: store.activeBot.value?.id || '',
      thread_id: store.state.activeThreadId || '',
      title,
      priority: newTodoPriority.value,
      status: 'pending',
    })
    if (res?.todo) {
      store.state.todos = [res.todo, ...store.state.todos.filter((x) => x.id !== res.todo.id)]
    }
    void store.refreshWorkspaceSlices(['todos'])
    store.notify('待办已添加')
  } catch (err: any) {
    newTodoTitle.value = title
    store.notify(err?.message || '添加待办失败')
  } finally {
    savingTodo.value = false
  }
}

async function toggleTodoStatus(td: TodoItem) {
  if (busyTodoId.value) return
  const prev = td.status
  const next = prev === 'done' ? 'pending' : 'done'
  busyTodoId.value = td.id
  td.status = next
  try {
    await BotHubApi.updateTodo(td.id, { status: next })
    void store.refreshWorkspaceSlices(['todos'])
  } catch (err: any) {
    td.status = prev
    store.notify(err?.message || '更新待办失败')
  } finally {
    busyTodoId.value = ''
  }
}

async function removeTodo(td: TodoItem) {
  if (busyTodoId.value) return
  busyTodoId.value = td.id
  const prev = [...store.state.todos]
  store.state.todos = store.state.todos.filter((x) => x.id !== td.id)
  try {
    await BotHubApi.deleteTodo(td.id)
    void store.refreshWorkspaceSlices(['todos'])
    store.notify('已删除待办')
  } catch (err: any) {
    store.state.todos = prev
    store.notify(err?.message || '删除待办失败')
  } finally {
    busyTodoId.value = ''
  }
}

function quoteTodoToChat(td: TodoItem) {
  const duePart = td.due_at ? `（截止 ${td.due_at}）` : ''
  emit('quoteToChat', `请帮我跟进待办「${td.title}」${duePart}：`)
}

// ---- Notes State ----
const noteSearch = ref('')
const selectedNoteId = ref('')
const noteEditing = ref(false)
const draftNoteId = ref('')
const draftNoteTitle = ref('')
const draftNoteCategory = ref('deliverable')
const draftNoteContent = ref('')
const savingNote = ref(false)
const noteTitleRef = ref<HTMLInputElement | null>(null)

const filteredNotes = computed(() => {
  const q = noteSearch.value.trim().toLowerCase()
  return store.state.notes.filter((n) => {
    if (!q) return true
    return `${n.title} ${n.category} ${n.content}`.toLowerCase().includes(q)
  })
})

const selectedNote = computed<WorkNote | null>(() => {
  if (!selectedNoteId.value) return null
  return store.state.notes.find((n) => n.id === selectedNoteId.value) || null
})

function openNoteViewer(n: WorkNote) {
  selectedNoteId.value = n.id
  noteEditing.value = false
}

function startNewNote() {
  selectedNoteId.value = ''
  draftNoteId.value = ''
  draftNoteTitle.value = ''
  draftNoteCategory.value = 'deliverable'
  draftNoteContent.value = ''
  noteEditing.value = true
  nextTick(() => noteTitleRef.value?.focus())
}

function startEditNote(n: WorkNote) {
  draftNoteId.value = n.id
  draftNoteTitle.value = n.title
  draftNoteCategory.value = n.category || 'deliverable'
  draftNoteContent.value = n.content
  noteEditing.value = true
  nextTick(() => noteTitleRef.value?.focus())
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
        category: draftNoteCategory.value.trim() || 'deliverable',
        content: draftNoteContent.value,
      })
      saved = res.note
    } else {
      const res = await BotHubApi.createNote({
        bot_id: store.activeBot.value?.id || '',
        title,
        category: draftNoteCategory.value.trim() || 'deliverable',
        content: draftNoteContent.value,
      })
      saved = res.note
    }
    const idx = store.state.notes.findIndex((x) => x.id === saved.id)
    if (idx >= 0) store.state.notes.splice(idx, 1, saved)
    else store.state.notes = [saved, ...store.state.notes]
    selectedNoteId.value = saved.id
    noteEditing.value = false
    void store.refreshWorkspaceSlices(['notes'])
    store.notify('笔记已保存')
  } catch (err: any) {
    store.notify(err?.message || '保存笔记失败')
  } finally {
    savingNote.value = false
  }
}

function quoteNoteToChat(n: WorkNote) {
  const body = n.content.trim().slice(0, 1400)
  emit('quoteToChat', `参考工作文档《${n.title}》：\n\n${body}\n\n请基于以上内容：`)
}

function handleNoteMarkdownClick(e: MouseEvent) {
  handleMarkdownCodeCopy(e, () => store.notify('代码块已复制'))
}

// ---- Stash State ----
const stashCaptureInput = ref('')
const stashShowAll = ref(false)
const capturingStash = ref(false)
const busyStashId = ref('')

const visibleStash = computed(() =>
  store.state.stash.filter((it) => {
    if (it.status === 'archived') return false
    return stashShowAll.value ? true : it.read_count === 0
  })
)

async function handleCaptureStash() {
  const raw = stashCaptureInput.value.trim()
  if (!raw || capturingStash.value) return
  capturingStash.value = true
  stashCaptureInput.value = ''
  try {
    const res = await BotHubApi.createStash({ raw_input: raw })
    await store.refreshWorkspaceSlices(['stash'])
    store.notify(res.deduplicated ? '已合并至收藏匣' : '已秒收至收藏匣')
  } catch (err: any) {
    stashCaptureInput.value = raw
    store.notify(err?.message || '收藏失败')
  } finally {
    capturingStash.value = false
  }
}

async function markStashRead(item: StashItem) {
  if (busyStashId.value) return
  busyStashId.value = item.id
  item.read_count = (item.read_count || 0) + 1
  try {
    await BotHubApi.updateStash(item.id, { mark_read: true })
    void store.refreshWorkspaceSlices(['stash'])
    store.notify('已标为已读')
  } catch (err: any) {
    item.read_count = Math.max(0, (item.read_count || 1) - 1)
    store.notify(err?.message || '更新失败')
  } finally {
    busyStashId.value = ''
  }
}

async function convertStashToTodo(item: StashItem) {
  if (busyStashId.value) return
  busyStashId.value = item.id
  const notes: string[] = []
  if (item.ai_tldr) notes.push(`结论：${item.ai_tldr}`)
  if (item.url) notes.push(`链接：${item.url}`)
  try {
    await BotHubApi.createTodo({
      bot_id: store.activeBot.value?.id || '',
      title: item.title || item.raw_input.slice(0, 60),
      notes: notes.join('\n'),
      priority: 'normal',
      tags: item.tags || '收藏转化',
      status: 'pending',
    })
    item.read_count = (item.read_count || 0) + 1
    await BotHubApi.updateStash(item.id, { mark_read: true }).catch(() => {})
    await store.refreshWorkspaceSlices(['todos', 'stash'])
    store.notify('已转为待办并标为已读')
  } catch (err: any) {
    store.notify(err?.message || '转待办失败')
  } finally {
    busyStashId.value = ''
  }
}

async function convertStashToNote(item: StashItem) {
  if (busyStashId.value) return
  busyStashId.value = item.id
  const lines: string[] = []
  if (item.url) lines.push(`- 来源: ${item.url}`)
  if (item.ai_tldr) lines.push(`\n## 核心提炼\n\n${item.ai_tldr}`)
  if (Array.isArray(item.points) && item.points.length > 0) {
    lines.push('\n## 关键要点\n')
    item.points.forEach((p) => lines.push(`- ${p}`))
  }
  if (item.user_note) lines.push(`\n## 个人批注\n\n${item.user_note}`)
  if (lines.length === 0) lines.push(item.raw_input)
  try {
    await BotHubApi.createNote({
      bot_id: store.activeBot.value?.id || '',
      title: (item.title || item.raw_input).slice(0, 48),
      category: 'research',
      content: lines.join('\n'),
    })
    await store.refreshWorkspaceSlices(['notes'])
    store.notify('已存为工作笔记')
  } catch (err: any) {
    store.notify(err?.message || '存笔记失败')
  } finally {
    busyStashId.value = ''
  }
}

function quoteStashToChat(item: StashItem) {
  const parts = [`参考收藏「${item.title || item.raw_input.slice(0, 48)}」`]
  if (item.url) parts.push(`链接：${item.url}`)
  if (item.ai_tldr) parts.push(`摘要：${item.ai_tldr}`)
  emit('quoteToChat', `${parts.join('\n')}\n请帮我进一步梳理：`)
}

// ---- Assets State ----
const assetExpiringOnly = ref(false)
const renewingAssetId = ref('')

const visibleAssets = computed(() => {
  const list = store.state.assets.filter((a) => {
    if (!assetExpiringOnly.value) return true
    if (a.status === '已停用' || String(a.due_date || '').startsWith('2099')) return false
    return typeof a.days_left === 'number' && a.days_left <= 7
  })
  return list.toSorted((a, b) => (a.days_left ?? 9999) - (b.days_left ?? 9999))
})

const monthlyAssetCost = computed(() => summarizeAssetsMonthlyCost(visibleAssets.value))
const yearlyAssetCost = computed(() => summarizeAssetsYearlyCost(visibleAssets.value))

async function quickRenewAsset(a: Asset, months: 1 | 3 | 12) {
  if (renewingAssetId.value) return
  const curDate = (a.due_date || '').slice(0, 10)
  const curDiff = diffPlainDaysFromToday(curDate)
  const baseDate =
    /^\d{4}-\d{2}-\d{2}$/.test(curDate) &&
    !curDate.startsWith('2099') &&
    curDiff !== null &&
    curDiff >= -30
      ? curDate
      : todayPlainDate()
  const nextDue = addMonthsPlain(baseDate, months)
  const nextDays = diffPlainDaysFromToday(nextDue) ?? a.days_left
  renewingAssetId.value = a.id
  try {
    await BotHubApi.updateAsset(a.id, {
      ...a,
      due_date: nextDue,
      days_left: nextDays,
      status: '生效中',
    })
    const stepLabel = months === 12 ? '1 年' : months === 3 ? '1 季' : '1 月'
    store.notify(`已将「${a.name}」顺延 ${stepLabel} 至 ${nextDue}`)
    await store.refreshWorkspaceSlices(['assets'])
  } catch (err: any) {
    store.notify(err?.message || '顺延到期日失败')
  } finally {
    renewingAssetId.value = ''
  }
}

// ---- Memories State ----
const memFactKey = ref('')
const memContent = ref('')
const memScopeMode = ref<'bot' | 'global'>('bot')
const savingMemory = ref(false)
const busyMemId = ref<number | null>(null)

const scopedMemories = computed(() =>
  resolveScopedMemories(store.activeBot.value, store.state.botMemories || [], store.state.memories)
)

async function handleAddMemory() {
  const key = memFactKey.value.trim()
  const body = memContent.value.trim()
  if (!key || !body || savingMemory.value) return
  savingMemory.value = true
  const botId = store.activeBot.value?.id
  const targetScope = memScopeMode.value === 'bot' && botId ? `bot:${botId}` : 'global'
  try {
    await BotHubApi.saveMemory({
      scope: targetScope,
      category: 'insight',
      fact_key: key,
      content: body,
      importance: 3,
      pinned: false,
    })
    memFactKey.value = ''
    memContent.value = ''
    await Promise.all([store.refreshWorkspaceSlices(['memories']), store.refreshActiveBot()])
    store.notify('长期记忆已写入')
  } catch (err: any) {
    store.notify(err?.message || '写入记忆失败')
  } finally {
    savingMemory.value = false
  }
}

async function toggleMemoryPin(m: AgentMemory) {
  if (busyMemId.value !== null) return
  const nextPin = !m.pinned
  busyMemId.value = m.id
  m.pinned = nextPin
  try {
    await BotHubApi.updateMemory(m.id, { pinned: nextPin })
    void store.refreshWorkspaceSlices(['memories'])
    store.notify(nextPin ? '已置顶该条记忆' : '已取消记忆置顶')
  } catch (err: any) {
    m.pinned = !nextPin
    store.notify(err?.message || '更新记忆失败')
  } finally {
    busyMemId.value = null
  }
}

async function removeMemory(m: AgentMemory) {
  if (busyMemId.value !== null) return
  busyMemId.value = m.id
  const prevGlobal = [...store.state.memories]
  const prevBot = [...store.state.botMemories]
  store.state.memories = store.state.memories.filter((x) => x.id !== m.id)
  store.state.botMemories = store.state.botMemories.filter((x) => x.id !== m.id)
  try {
    await BotHubApi.deleteMemory(m.id)
    void store.refreshWorkspaceSlices(['memories'])
    store.notify('已移除记忆')
  } catch (err: any) {
    store.state.memories = prevGlobal
    store.state.botMemories = prevBot
    store.notify(err?.message || '删除记忆失败')
  } finally {
    busyMemId.value = null
  }
}

function openFullView() {
  if (activeTab.value === 'assets') {
    store.state.workspace = 'ops'
    return
  }
  if (activeTab.value === 'memories') {
    store.state.memoryModalDefaultScope = store.activeBot.value
      ? `bot:${store.activeBot.value.id}`
      : 'global'
    store.state.showMemoriesModal = true
    return
  }
  store.state.workhubTab = activeTab.value
  store.state.showWorkhubModal = true
}

function fullViewLabel(): string {
  if (activeTab.value === 'assets') return '资产台账'
  if (activeTab.value === 'memories') return '记忆全景'
  return '全屏工作台'
}
</script>

<template>
  <aside
    aria-label="桌面协同副屏"
    class="hidden lg:flex w-[376px] xl:w-[400px] shrink-0 border-l border-line flex-col min-h-0 bg-subtle"
  >
    <!-- Inspector Top Header + Full-Screen Link -->
    <div class="px-3.5 pt-2.5 pb-2 border-b border-line flex flex-col gap-2 shrink-0">
      <div class="flex items-center justify-between gap-2">
        <div class="flex items-center gap-1.5 min-w-0">
          <svg viewBox="0 0 24 24" width="13.5" height="13.5" fill="none" stroke="currentColor" stroke-width="2" class="text-accent-fg shrink-0" aria-hidden="true">
            <rect x="3" y="3" width="18" height="18" rx="2" />
            <line x1="15" y1="3" x2="15" y2="21" />
          </svg>
          <span class="text-xs font-semibold text-fg truncate">协同副屏</span>
        </div>
        <div class="flex items-center gap-1.5 shrink-0">
          <button
            type="button"
            @click="openFullView"
            class="px-2 py-1 rounded-lg border border-line hover:border-accent-border text-xs text-fg-sub hover:text-accent-fg cursor-pointer inline-flex items-center gap-1"
          >
            <span>{{ fullViewLabel() }}</span>
            <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <polyline points="15 3 21 3 21 9" />
              <line x1="10" y1="14" x2="21" y2="3" />
            </svg>
          </button>
          <button
            type="button"
            aria-label="收起协同副屏"
            title="收起协同副屏"
            @click="store.toggleSideInspector()"
            class="w-6 h-6 rounded-lg inline-flex items-center justify-center text-fg-muted hover:text-fg hover:bg-hover cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
      </div>

      <!-- 5 Segmented Tabs -->
      <div role="tablist" aria-label="协同副屏面板" class="well-recessed rounded-full p-0.5 grid grid-cols-5 gap-0.5">
        <button
          v-for="tab in TABS"
          :key="tab.id"
          type="button"
          role="tab"
          :aria-selected="activeTab === tab.id"
          :data-active="activeTab === tab.id"
          @click="activeTab = tab.id"
          :class="[
            'relative py-1 rounded-full text-xs font-medium cursor-pointer transition-colors flex items-center justify-center gap-1',
            activeTab === tab.id
              ? 'bg-accent text-on-accent shadow-xs'
              : 'text-fg-sub hover:text-fg',
          ]"
        >
          <span>{{ tab.label }}</span>
          <span
            v-if="tab.id === 'todos' && store.pendingTodoCount.value > 0 && activeTab !== 'todos'"
            class="w-1.5 h-1.5 rounded-full bg-info"
            aria-hidden="true"
          />
          <span
            v-else-if="tab.id === 'stash' && store.unreadStashCount.value > 0 && activeTab !== 'stash'"
            class="w-1.5 h-1.5 rounded-full bg-danger"
            aria-hidden="true"
          />
          <span
            v-else-if="tab.id === 'assets' && store.expiringAssetCount.value > 0 && activeTab !== 'assets'"
            class="w-1.5 h-1.5 rounded-full bg-warn"
            aria-hidden="true"
          />
        </button>
      </div>
    </div>

    <!-- Inspector Body -->
    <div class="flex-1 min-h-0 overflow-y-auto px-3.5 py-3 flex flex-col gap-2.5">
      <!-- TAB 1: TODOS -->
      <template v-if="activeTab === 'todos'">
        <form @submit.prevent="handleCreateTodo" class="flex flex-col gap-1.5">
          <div class="flex items-center gap-1.5">
            <input
              v-model="newTodoTitle"
              type="text"
              placeholder="快速记待办，回车添加..."
              class="flex-1 min-w-0 rounded-xl px-2.5 py-1.5 text-xs border border-line focus:border-accent"
            />
            <button
              type="submit"
              :disabled="!newTodoTitle.trim() || savingTodo"
              class="btn-jewel px-2.5 py-1.5 rounded-xl text-xs shrink-0 cursor-pointer disabled:opacity-40"
            >
              添加
            </button>
          </div>
          <div class="flex items-center justify-between gap-2 text-xs">
            <div class="flex items-center gap-1">
              <button
                v-for="p in (['high', 'normal', 'low'] as const)"
                :key="p"
                type="button"
                @click="newTodoPriority = p"
                :class="[
                  'px-2 py-0.5 rounded-full border cursor-pointer',
                  newTodoPriority === p
                    ? 'border-accent-border bg-accent-soft text-accent-fg'
                    : 'border-line text-fg-muted hover:text-fg',
                ]"
              >
                {{ p === 'high' ? '高优' : p === 'low' ? '低优' : '普通' }}
              </button>
            </div>
            <button
              type="button"
              @click="todoShowAll = !todoShowAll"
              class="font-mono text-fg-muted hover:text-accent-fg cursor-pointer"
            >
              {{ todoShowAll ? '仅未完成' : `全部 (${store.state.todos.length})` }}
            </button>
          </div>
        </form>

        <div v-if="visibleTodos.length === 0" class="py-10 text-center text-xs text-fg-muted">
          暂无待办事项
        </div>

        <div v-else class="flex flex-col gap-1.5">
          <div
            v-for="td in visibleTodos"
            :key="td.id"
            :class="[
              'vesper-card rounded-xl p-2.5 flex items-start gap-2',
              td.status === 'done' ? 'opacity-65' : '',
            ]"
          >
            <button
              type="button"
              :aria-label="td.status === 'done' ? '标为未完成' : '标为已完成'"
              @click="toggleTodoStatus(td)"
              :class="[
                'mt-0.5 w-4.5 h-4.5 rounded border flex items-center justify-center shrink-0 cursor-pointer transition-colors',
                td.status === 'done'
                  ? 'border-ok-border bg-ok-soft text-ok'
                  : 'border-line bg-subtle text-transparent hover:border-accent-border',
              ]"
            >
              <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2.6" aria-hidden="true">
                <polyline points="20 6 9 17 4 12" />
              </svg>
            </button>

            <div class="min-w-0 flex-1 flex flex-col gap-1">
              <div
                :class="[
                  'text-xs leading-snug break-words',
                  td.status === 'done' ? 'line-through text-fg-muted' : 'text-fg',
                ]"
              >
                {{ td.title }}
              </div>
              <div class="flex flex-wrap items-center gap-1 text-xs font-mono">
                <span
                  :class="[
                    'px-1.5 py-0.5 rounded border',
                    td.priority === 'high'
                      ? 'border-danger-border bg-danger-soft text-danger'
                      : td.priority === 'low'
                        ? 'border-line text-fg-muted'
                        : 'border-line text-fg-sub',
                  ]"
                >
                  {{ td.priority === 'high' ? '高' : td.priority === 'low' ? '低' : '普通' }}
                </span>
                <span
                  v-if="td.due_at"
                  :class="[
                    'px-1.5 py-0.5 rounded border',
                    isTodoOverdue(td)
                      ? 'border-danger-border bg-danger-soft text-danger'
                      : 'border-line text-fg-muted',
                  ]"
                >
                  {{ td.due_at }}
                </span>
              </div>
            </div>

            <div class="flex items-center gap-0.5 shrink-0">
              <button
                type="button"
                title="引用到对话"
                aria-label="引用到对话"
                @click="quoteTodoToChat(td)"
                class="w-6 h-6 rounded-lg inline-flex items-center justify-center text-fg-muted hover:text-accent-fg hover:bg-hover cursor-pointer"
              >
                <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                  <polyline points="15 10 20 15 15 20" />
                  <path d="M4 4v7a4 4 0 0 0 4 4h12" />
                </svg>
              </button>
              <button
                type="button"
                title="删除待办"
                aria-label="删除待办"
                @click="removeTodo(td)"
                class="w-6 h-6 rounded-lg inline-flex items-center justify-center text-fg-muted hover:text-danger hover:bg-danger-soft cursor-pointer"
              >
                <svg viewBox="0 0 24 24" width="11.5" height="11.5" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </svg>
              </button>
            </div>
          </div>
        </div>
      </template>

      <!-- TAB 2: NOTES -->
      <template v-else-if="activeTab === 'notes'">
        <!-- Inline Note Editor -->
        <form v-if="noteEditing" @submit.prevent="handleSaveNote" class="flex flex-col gap-2">
          <div class="flex items-center justify-between gap-2">
            <span class="text-xs font-semibold text-fg">{{ draftNoteId ? '编辑笔记' : '新建工作笔记' }}</span>
            <button
              type="button"
              @click="noteEditing = false"
              class="text-xs text-fg-muted hover:text-fg cursor-pointer"
            >
              取消
            </button>
          </div>
          <input
            ref="noteTitleRef"
            v-model="draftNoteTitle"
            type="text"
            placeholder="文档标题..."
            class="rounded-xl px-2.5 py-1.5 text-xs border border-line focus:border-accent"
          />
          <input
            v-model="draftNoteCategory"
            type="text"
            placeholder="分类 (如 deliverable / plan / research)"
            class="rounded-xl px-2.5 py-1.5 text-xs font-mono border border-line focus:border-accent"
          />
          <textarea
            v-model="draftNoteContent"
            rows="10"
            placeholder="支持 Markdown 正文..."
            class="rounded-xl p-2.5 text-xs border border-line focus:border-accent leading-relaxed resize-y"
          />
          <div class="flex items-center justify-end gap-2">
            <button
              type="submit"
              :disabled="!draftNoteTitle.trim() || savingNote"
              class="btn-jewel px-3 py-1.5 rounded-xl text-xs cursor-pointer disabled:opacity-40"
            >
              {{ savingNote ? '保存中...' : '保存笔记' }}
            </button>
          </div>
        </form>

        <!-- Inline Note Viewer -->
        <div v-else-if="selectedNote" class="flex flex-col gap-2.5">
          <div class="flex items-center justify-between gap-2">
            <button
              type="button"
              @click="selectedNoteId = ''"
              class="inline-flex items-center gap-1 text-xs text-fg-sub hover:text-accent-fg cursor-pointer"
            >
              <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <polyline points="15 18 9 12 15 6" />
              </svg>
              <span>返回列表</span>
            </button>
            <div class="flex items-center gap-1.5">
              <button
                type="button"
                @click="quoteNoteToChat(selectedNote)"
                class="px-2 py-1 rounded-lg border border-accent-border bg-accent-soft text-xs text-accent-fg hover:bg-accent-soft cursor-pointer"
              >
                引用正文到对话
              </button>
              <button
                type="button"
                @click="startEditNote(selectedNote)"
                class="px-2 py-1 rounded-lg border border-line text-xs text-fg-sub hover:text-fg cursor-pointer"
              >
                编辑
              </button>
            </div>
          </div>

          <div class="vesper-card rounded-xl p-3 flex flex-col gap-2">
            <div class="flex items-center justify-between gap-2">
              <h3 class="text-xs font-semibold text-fg leading-snug">{{ selectedNote.title }}</h3>
              <span class="font-mono text-xs px-1.5 py-0.5 rounded bg-subtle text-fg-sub shrink-0">
                {{ selectedNote.category || 'note' }}
              </span>
            </div>
            <div
              @click="handleNoteMarkdownClick"
              class="text-xs leading-relaxed"
              v-html="renderSafeMarkdown(selectedNote.content || '（暂无正文）')"
            />
          </div>
        </div>

        <!-- Notes List -->
        <template v-else>
          <div class="flex items-center gap-1.5">
            <input
              v-model="noteSearch"
              type="search"
              placeholder="搜索工作笔记..."
              class="flex-1 min-w-0 rounded-xl px-2.5 py-1.5 text-xs border border-line focus:border-accent"
            />
            <button
              type="button"
              @click="startNewNote"
              class="btn-tactile-gold px-2.5 py-1.5 rounded-xl text-xs font-medium shrink-0 cursor-pointer"
            >
              + 新建
            </button>
          </div>

          <div v-if="filteredNotes.length === 0" class="py-10 text-center text-xs text-fg-muted">
            暂无匹配的工作笔记
          </div>

          <div v-else class="flex flex-col gap-1.5">
            <div
              v-for="n in filteredNotes"
              :key="n.id"
              class="vesper-card vesper-card-interactive rounded-xl p-2.5 flex flex-col gap-1.5"
            >
              <div class="flex items-start justify-between gap-2">
                <button
                  type="button"
                  @click="openNoteViewer(n)"
                  class="min-w-0 flex-1 text-left cursor-pointer"
                >
                  <div class="text-xs font-medium text-fg truncate hover:text-accent-fg">
                    {{ n.title }}
                  </div>
                </button>
                <span class="font-mono text-xs px-1.5 py-0.5 rounded border border-line text-fg-muted shrink-0">
                  {{ n.category || 'note' }}
                </span>
              </div>
              <div class="flex items-center justify-between gap-2 text-xs text-fg-muted">
                <span class="font-mono truncate">{{ formatTime(n.updated_at) }}</span>
                <button
                  type="button"
                  @click="quoteNoteToChat(n)"
                  class="text-accent-fg hover:underline cursor-pointer shrink-0"
                >
                  引用正文到对话
                </button>
              </div>
            </div>
          </div>
        </template>
      </template>

      <!-- TAB 3: STASH -->
      <template v-else-if="activeTab === 'stash'">
        <form @submit.prevent="handleCaptureStash" class="flex flex-col gap-1.5">
          <div class="flex items-center gap-1.5">
            <input
              v-model="stashCaptureInput"
              type="text"
              placeholder="粘贴链接或输入闪念，回车秒收..."
              class="flex-1 min-w-0 rounded-xl px-2.5 py-1.5 text-xs border border-line focus:border-accent"
            />
            <button
              type="submit"
              :disabled="!stashCaptureInput.trim() || capturingStash"
              class="btn-jewel px-2.5 py-1.5 rounded-xl text-xs shrink-0 cursor-pointer disabled:opacity-40"
            >
              收藏
            </button>
          </div>
          <div class="flex items-center justify-between text-xs">
            <span class="font-mono text-fg-muted">未读 {{ store.unreadStashCount.value }}</span>
            <button
              type="button"
              @click="stashShowAll = !stashShowAll"
              class="font-mono text-fg-muted hover:text-accent-fg cursor-pointer"
            >
              {{ stashShowAll ? '只看未读' : '显示全部' }}
            </button>
          </div>
        </form>

        <div v-if="visibleStash.length === 0" class="py-10 text-center text-xs text-fg-muted">
          {{ stashShowAll ? '收藏匣暂无内容' : '当前无未读干货' }}
        </div>

        <div v-else class="flex flex-col gap-2">
          <div
            v-for="item in visibleStash"
            :key="item.id"
            class="vesper-card rounded-xl p-2.5 flex flex-col gap-1.5"
          >
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-1.5 mb-0.5">
                  <span class="font-mono text-xs px-1.5 py-0.5 rounded border border-info-border bg-info-soft text-info">
                    {{ item.kind === 'note' ? '闪念' : item.kind === 'github' ? 'GitHub' : item.kind === 'bilibili' ? 'Bilibili' : item.kind === 'xiaohongshu' ? '小红书' : item.kind === 'douyin' ? '抖音' : item.kind === 'wechat' ? '公众号' : 'Web' }}
                  </span>
                  <span v-if="item.read_count === 0" class="w-1.5 h-1.5 rounded-full bg-danger shrink-0" aria-hidden="true" />
                  <span v-if="item.domain" class="font-mono text-xs text-fg-muted truncate">{{ item.domain }}</span>
                </div>
                <a
                  v-if="item.url"
                  :href="item.url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-xs font-medium text-fg hover:text-accent-fg hover:underline line-clamp-2"
                >
                  {{ item.title || item.raw_input }}
                </a>
                <div v-else class="text-xs font-medium text-fg line-clamp-2">
                  {{ item.title || item.raw_input }}
                </div>
              </div>
            </div>

            <div v-if="item.ai_tldr" class="text-xs text-fg-sub line-clamp-2 leading-snug">
              {{ item.ai_tldr }}
            </div>

            <div class="flex flex-wrap items-center gap-1 pt-0.5">
              <button
                v-if="item.read_count === 0"
                type="button"
                @click="markStashRead(item)"
                class="px-2 py-0.5 rounded-lg border border-line text-xs text-fg-sub hover:text-fg hover:bg-hover cursor-pointer"
              >
                标为已读
              </button>
              <button
                type="button"
                @click="convertStashToTodo(item)"
                class="px-2 py-0.5 rounded-lg border border-info-border bg-info-soft text-xs text-info hover:bg-info-soft cursor-pointer"
              >
                转待办
              </button>
              <button
                type="button"
                @click="convertStashToNote(item)"
                class="px-2 py-0.5 rounded-lg border border-ok-border bg-ok-soft text-xs text-ok hover:bg-ok-soft cursor-pointer"
              >
                存笔记
              </button>
              <button
                type="button"
                @click="quoteStashToChat(item)"
                class="px-2 py-0.5 rounded-lg border border-accent-border bg-accent-soft text-xs text-accent-fg hover:bg-accent-soft cursor-pointer"
              >
                引用进对话
              </button>
            </div>
          </div>
        </div>
      </template>

      <!-- TAB 4: ASSETS -->
      <template v-else-if="activeTab === 'assets'">
        <div class="vesper-card rounded-xl p-2.5 flex flex-col gap-1.5">
          <div class="flex items-center justify-between gap-2 text-xs">
            <span class="text-fg-sub font-medium">费用预估 ({{ visibleAssets.length }} 项)</span>
            <div class="flex items-center gap-1.5">
              <button
                type="button"
                @click="assetExpiringOnly = !assetExpiringOnly"
                :class="[
                  'px-2 py-0.5 rounded-full border text-xs cursor-pointer',
                  assetExpiringOnly
                    ? 'border-warn-border bg-warn-soft text-warn'
                    : 'border-line text-fg-muted hover:text-fg',
                ]"
              >
                {{ assetExpiringOnly ? '仅临期' : '全部' }}
              </button>
              <button
                type="button"
                @click="store.state.editingAsset = null"
                class="px-2 py-0.5 rounded-full border border-accent-border bg-accent-soft text-xs text-accent-fg cursor-pointer"
              >
                + 新增
              </button>
            </div>
          </div>
          <div v-if="monthlyAssetCost || yearlyAssetCost" class="font-mono text-xs text-accent-fg">
            <span v-if="monthlyAssetCost">月均 {{ monthlyAssetCost }}</span>
            <span v-if="monthlyAssetCost && yearlyAssetCost" class="mx-1.5 text-fg-muted">|</span>
            <span v-if="yearlyAssetCost">年化 {{ yearlyAssetCost }}</span>
          </div>
        </div>

        <div v-if="visibleAssets.length === 0" class="py-10 text-center text-xs text-fg-muted">
          {{ assetExpiringOnly ? '暂无 7 天内临期资产' : '暂无数字资产' }}
        </div>

        <div v-else class="flex flex-col gap-1.5">
          <div
            v-for="a in visibleAssets"
            :key="a.id"
            class="vesper-card rounded-xl p-2.5 flex flex-col gap-1.5"
          >
            <button
              type="button"
              @click="store.state.editingAsset = a"
              class="w-full text-left flex items-start justify-between gap-2 cursor-pointer"
            >
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-1.5">
                  <AssetCategoryIcon
                    :icon="resolveCategoryMeta(a.category, store.state.assetCategories).icon"
                    class="w-3.5 h-3.5 text-accent-fg shrink-0"
                  />
                  <span class="text-xs font-medium text-fg truncate hover:text-accent-fg">{{ a.name }}</span>
                </div>
                <div class="mt-0.5 flex items-center gap-2 text-xs font-mono text-fg-muted">
                  <span>{{ formatDateOnly(a.due_date) }}</span>
                  <span v-if="a.price" class="text-accent-fg">{{ a.price }}</span>
                </div>
              </div>
              <span
                :class="[
                  'font-mono text-xs shrink-0',
                  a.status === '长期有效' || String(a.due_date || '').startsWith('2099')
                    ? 'text-ok'
                    : a.days_left <= 3
                      ? 'text-danger font-semibold'
                      : a.days_left <= 7
                        ? 'text-warn font-semibold'
                        : 'text-ok',
                ]"
              >
                {{
                  a.status === '长期有效' || String(a.due_date || '').startsWith('2099')
                    ? '长期'
                    : a.days_left < 0
                      ? `逾期${Math.abs(a.days_left)}天`
                      : `剩${a.days_left}天`
                }}
              </span>
            </button>

            <div
              v-if="!(a.status === '长期有效' || String(a.due_date || '').startsWith('2099'))"
              class="flex items-center justify-between gap-1.5 pt-0.5"
            >
              <span class="text-xs text-fg-muted truncate">{{ a.spec || a.status }}</span>
              <div class="flex items-center gap-1 shrink-0">
                <button
                  type="button"
                  :disabled="renewingAssetId === a.id"
                  @click="quickRenewAsset(a, 1)"
                  class="px-1.5 py-0.5 rounded border border-line hover:border-accent-border text-xs font-mono text-fg-sub hover:text-accent-fg cursor-pointer"
                >
                  +1月
                </button>
                <button
                  type="button"
                  :disabled="renewingAssetId === a.id"
                  @click="quickRenewAsset(a, 3)"
                  class="px-1.5 py-0.5 rounded border border-line hover:border-accent-border text-xs font-mono text-fg-sub hover:text-accent-fg cursor-pointer"
                >
                  +1季
                </button>
                <button
                  type="button"
                  :disabled="renewingAssetId === a.id"
                  @click="quickRenewAsset(a, 12)"
                  class="px-1.5 py-0.5 rounded border border-line hover:border-accent-border text-xs font-mono text-fg-sub hover:text-accent-fg cursor-pointer"
                >
                  +1年
                </button>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- TAB 5: MEMORIES -->
      <template v-else-if="activeTab === 'memories'">
        <form @submit.prevent="handleAddMemory" class="flex flex-col gap-1.5">
          <div class="flex items-center gap-1.5">
            <input
              v-model="memFactKey"
              type="text"
              placeholder="记忆键 (如 preferred_lang)"
              class="flex-1 min-w-0 rounded-xl px-2.5 py-1.5 text-xs border border-line focus:border-accent"
            />
            <button
              type="button"
              @click="memScopeMode = memScopeMode === 'bot' ? 'global' : 'bot'"
              class="px-2 py-1.5 rounded-xl border border-line text-xs font-mono text-fg-sub hover:text-accent-fg shrink-0 cursor-pointer"
            >
              {{ memScopeMode === 'bot' ? '当前Bot' : '全局' }}
            </button>
          </div>
          <div class="flex items-center gap-1.5">
            <input
              v-model="memContent"
              type="text"
              placeholder="记下一条偏好、事实或约定..."
              class="flex-1 min-w-0 rounded-xl px-2.5 py-1.5 text-xs border border-line focus:border-accent"
            />
            <button
              type="submit"
              :disabled="!memFactKey.trim() || !memContent.trim() || savingMemory"
              class="btn-jewel px-2.5 py-1.5 rounded-xl text-xs shrink-0 cursor-pointer disabled:opacity-40"
            >
              存入
            </button>
          </div>
        </form>

        <div v-if="scopedMemories.length === 0" class="py-10 text-center text-xs text-fg-muted">
          当前作用域暂无长期记忆
        </div>

        <div v-else class="flex flex-col gap-1.5">
          <div
            v-for="m in scopedMemories"
            :key="m.id"
            class="vesper-card rounded-xl p-2.5 flex flex-col gap-1"
          >
            <div class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-1.5 min-w-0">
                <span
                  :class="[
                    'font-mono text-xs px-1.5 py-0.5 rounded border shrink-0',
                    !m.scope || m.scope === 'global'
                      ? 'border-info-border bg-info-soft text-info'
                      : 'border-accent-border bg-accent-soft text-accent-fg',
                  ]"
                >
                  {{ !m.scope || m.scope === 'global' ? '全局' : '专属' }}
                </span>
                <span class="font-mono text-xs font-medium text-fg truncate">{{ m.fact_key }}</span>
              </div>
              <div class="flex items-center gap-0.5 shrink-0">
                <button
                  type="button"
                  :title="m.pinned ? '取消置顶' : '置顶记忆'"
                  @click="toggleMemoryPin(m)"
                  :class="[
                    'px-1.5 py-0.5 rounded text-xs font-mono cursor-pointer',
                    m.pinned
                      ? 'bg-accent-soft text-accent-fg'
                      : 'text-fg-muted hover:text-fg',
                  ]"
                >
                  {{ m.pinned ? '已置顶' : '置顶' }}
                </button>
                <button
                  type="button"
                  title="删除记忆"
                  aria-label="删除记忆"
                  @click="removeMemory(m)"
                  class="w-5 h-5 rounded inline-flex items-center justify-center text-fg-muted hover:text-danger cursor-pointer"
                >
                  <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                    <line x1="18" y1="6" x2="6" y2="18" />
                    <line x1="6" y1="6" x2="18" y2="18" />
                  </svg>
                </button>
              </div>
            </div>
            <div class="text-xs text-fg-sub leading-snug break-words">
              {{ m.content }}
            </div>
          </div>
        </div>
      </template>
    </div>
  </aside>
</template>
