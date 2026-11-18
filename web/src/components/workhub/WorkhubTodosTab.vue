<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import { addDaysPlain, diffPlainDaysFromToday, todayPlainDate } from '../../utils/calendar'
import { pickDefaultWorkerBot } from '../../utils/botLook'
import { clearSearchOnEscape } from '../../utils/topLayer'
import type { TodoItem } from '../../types/bothub'
import CustomSelect from '../CustomSelect.vue'
import SegmentedTabs from '../SegmentedTabs.vue'
import CalendarDatePicker from '../ui/CalendarDatePicker.vue'

const emit = defineEmits<{
  closeModal: []
}>()

const store = useAppStore()

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

const editingTodoId = ref<string | null>(null)
const editTodoTitle = ref('')
const editTodoNotes = ref('')
const editTodoPriority = ref<'high' | 'normal' | 'low'>('normal')
const editTodoDueAt = ref('')
const editTodoBotId = ref('')

const botAssigneeOptions = computed(() => [
  { value: '', label: '未指定执行 Bot' },
  ...store.state.bots.map((b) => ({ value: b.id, label: b.name })),
])

const botFilterOptions = computed(() => [
  { value: '', label: '全部成员待办' },
  ...store.state.bots.map((b) => ({ value: b.id, label: b.name })),
])

const todayPrefix = computed(() => todayPlainDate())
const doneTodoCount = computed(() => store.state.todos.filter((t) => t.status === 'done').length)

function offsetDateStr(days: number): string {
  return addDaysPlain(todayPlainDate(), days)
}

const todoDatePresets = computed(() => [
  { label: '明天', value: offsetDateStr(1) },
  { label: '3天后', value: offsetDateStr(3) },
  { label: '一周内', value: offsetDateStr(7) },
])

function isOverdue(t: TodoItem): boolean {
  return Boolean(t.status !== 'done' && t.due_at && t.due_at.slice(0, 10) < todayPrefix.value)
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

function botNameById(botId?: string): string {
  if (!botId) return ''
  return store.state.bots.find((b) => b.id === botId)?.name || botId
}

function formatTodoDueLabel(t: TodoItem): string {
  if (!t.due_at) return ''
  const due = t.due_at.slice(0, 10)
  const diffDays = diffPlainDaysFromToday(due)
  if (diffDays === null) return due
  if (t.status !== 'done') {
    if (diffDays < 0) return `已逾期 ${Math.abs(diffDays)} 天 (${due})`
    if (diffDays === 0) return `今天截止 (${due})`
    if (diffDays === 1) return `明天截止 (${due})`
  }
  return `截止 ${due}`
}

function syncDefaultBotAndFocus() {
  if (!newTodoBotId.value && store.activeBot.value) {
    newTodoBotId.value = store.activeBot.value.id
  }
  nextTick(() => todoInputRef.value?.focus())
}

onMounted(() => {
  syncDefaultBotAndFocus()
})

watch(
  () => store.state.showWorkhubModal,
  (open) => {
    if (open) {
      syncDefaultBotAndFocus()
    } else {
      editingTodoId.value = null
    }
  }
)

async function handleAddTodo() {
  const cleanTitle = newTodoTitle.value.trim()
  if (!cleanTitle || savingTodo.value) return
  const prio = newTodoPriority.value
  const due = newTodoDueAt.value.trim()
  const botId = newTodoBotId.value
  savingTodo.value = true
  newTodoTitle.value = ''
  newTodoDueAt.value = ''
  newTodoPriority.value = 'normal'
  try {
    const res = await BotHubApi.createTodo({
      title: cleanTitle,
      priority: prio,
      due_at: due,
      bot_id: botId,
      status: 'pending',
    })
    if (res?.todo) {
      store.state.todos = [res.todo, ...store.state.todos.filter((x) => x.id !== res.todo.id)]
    }
    void store.refreshWorkspaceSlices(['todos'])
    store.notify('待办已添加至个人行动清单')
  } catch (err: any) {
    newTodoTitle.value = cleanTitle
    newTodoDueAt.value = due
    newTodoPriority.value = prio
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
  const cleanTitle = editTodoTitle.value.trim()
  if (!cleanTitle) return
  const snapshot = { ...t }
  t.title = cleanTitle
  t.notes = editTodoNotes.value.trim()
  t.priority = editTodoPriority.value
  t.due_at = editTodoDueAt.value.trim()
  t.bot_id = editTodoBotId.value
  editingTodoId.value = null
  try {
    await BotHubApi.updateTodo(t.id, {
      title: t.title,
      notes: t.notes,
      priority: t.priority,
      due_at: t.due_at,
      bot_id: t.bot_id,
    })
    void store.refreshWorkspaceSlices(['todos'])
    store.notify('待办已更新')
  } catch (err: any) {
    Object.assign(t, snapshot)
    store.notify(err.message || '更新待办失败')
  }
}

async function toggleTodoDone(t: TodoItem) {
  const prevStatus = t.status
  const nextStatus = prevStatus === 'done' ? 'pending' : 'done'
  t.status = nextStatus
  try {
    await BotHubApi.updateTodo(t.id, { status: nextStatus })
    void store.refreshWorkspaceSlices(['todos'])
  } catch (err: any) {
    t.status = prevStatus
    store.notify(err.message || '更新状态失败')
  }
}

async function cycleTodoProgress(t: TodoItem) {
  const prevStatus = t.status
  const next = prevStatus === 'pending' ? 'doing' : prevStatus === 'doing' ? 'done' : 'pending'
  t.status = next
  try {
    await BotHubApi.updateTodo(t.id, { status: next })
    void store.refreshWorkspaceSlices(['todos'])
  } catch (err: any) {
    t.status = prevStatus
    store.notify(err.message || '更新状态失败')
  }
}

async function removeTodo(id: string) {
  const prevList = [...store.state.todos]
  store.state.todos = store.state.todos.filter((t) => t.id !== id)
  if (editingTodoId.value === id) editingTodoId.value = null
  try {
    await BotHubApi.deleteTodo(id)
    void store.refreshWorkspaceSlices(['todos'])
  } catch (err: any) {
    store.state.todos = prevList
    store.notify(err.message || '删除失败')
  }
}

async function clearDoneTodos() {
  const prevList = [...store.state.todos]
  const doneCount = prevList.filter((t) => t.status === 'done').length
  if (doneCount === 0) {
    store.notify('暂无已完成待办可清理')
    return
  }
  store.state.todos = prevList.filter((t) => t.status !== 'done')
  try {
    const res = await BotHubApi.clearDoneTodos()
    void store.refreshWorkspaceSlices(['todos'])
    store.notify(`已清理 ${res.removed || doneCount} 项已完成待办`)
  } catch (err: any) {
    store.state.todos = prevList
    store.notify(err.message || '清理失败')
  }
}

async function dispatchTodoToBot(t: TodoItem) {
  const targetBotId = pickDefaultWorkerBot(store.state.bots, store.activeBot.value, t.bot_id)
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
  emit('closeModal')
  await store.jumpToBot(targetBotId, detail)
}
</script>

<template>
  <div class="flex-1 min-h-0 flex flex-col gap-3">
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
        <div class="sm:col-span-2">
          <CalendarDatePicker
            v-model="newTodoDueAt"
            compact
            placeholder="截止日期"
            :presets="todoDatePresets"
            aria-label="选择新建待办截止日期"
          />
        </div>
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
              : 'border-[#fff8ee]/10 text-slate-400 hover:text-white',
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
              : 'border-[#fff8ee]/10 text-slate-400 hover:text-white',
          ]"
        >
          今日截止
        </button>
        <input
          v-model="todoSearch"
          type="search"
          aria-label="筛选待办"
          placeholder="筛选待办..."
          @keydown.escape="clearSearchOnEscape($event, todoSearch, () => { todoSearch = '' })"
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
      <TransitionGroup v-else name="vesper-list" tag="div" class="flex flex-col gap-2">
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
            <div class="sm:col-span-2">
              <CalendarDatePicker
                v-model="editTodoDueAt"
                compact
                placeholder="截止日期"
                :presets="todoDatePresets"
                aria-label="修改待办截止日期"
              />
            </div>
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
      </TransitionGroup>
    </div>
  </div>
</template>
