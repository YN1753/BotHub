<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import { pickDefaultWorkerBot } from '../../utils/botLook'
import { copyToClipboard } from '../../utils/clipboard'
import { handleMarkdownCodeCopy, renderSafeMarkdown } from '../../utils/markdown'
import { formatTime, triggerBlobDownload } from '../../utils/format'
import { clearSearchOnEscape } from '../../utils/topLayer'
import type { WorkNote } from '../../types/bothub'

const emit = defineEmits<{
  closeModal: []
}>()

const store = useAppStore()
const noteSearch = ref('')
const selectedNoteId = ref('')
const editingNote = ref(false)
const draftNoteId = ref('')
const draftNoteTitle = ref('')
const draftNoteCategory = ref('deliverable')
const draftNoteContent = ref('')
const draftNotePinned = ref(false)
const savingNote = ref(false)
const noteTitleRef = ref<HTMLInputElement | null>(null)

const noteCategoryChoices = computed(() => {
  const base = ['general', 'deliverable', 'plan', 'research', 'note']
  const fromNotes = store.state.notes.map((n) => String(n.category || '').trim()).filter(Boolean)
  return Array.from(new Set([...base, ...fromNotes]))
})

watch(
  () => store.state.pendingNoteId,
  (targetId) => {
    if (!targetId) return
    noteSearch.value = ''
    selectedNoteId.value = targetId
    editingNote.value = false
    store.state.pendingNoteId = ''
  },
  { immediate: true }
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

function botNameById(botId?: string): string {
  if (!botId) return ''
  return store.state.bots.find((b) => b.id === botId)?.name || botId
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
    const existingIdx = store.state.notes.findIndex((x) => x.id === saved.id)
    if (existingIdx >= 0) {
      store.state.notes.splice(existingIdx, 1, saved)
    } else {
      store.state.notes = [saved, ...store.state.notes]
    }
    void store.refreshWorkspaceSlices(['notes'])
    store.notify('工作文档已保存')
  } catch (err: any) {
    store.notify(err.message || '保存文档失败')
  } finally {
    savingNote.value = false
  }
}

async function removeNote(id: string) {
  const ok = await store.openConfirm({
    title: '删除工作文档',
    message: '确定删除这份工作文档吗？删除后将无法恢复。',
    confirmText: '删除文档',
    danger: true,
  })
  if (!ok) return
  const prevNotes = [...store.state.notes]
  store.state.notes = store.state.notes.filter((x) => x.id !== id)
  if (selectedNoteId.value === id) selectedNoteId.value = ''
  editingNote.value = false
  store.notify('工作文档已删除')
  try {
    await BotHubApi.deleteNote(id)
    void store.refreshWorkspaceSlices(['notes'])
  } catch (err: any) {
    store.state.notes = prevNotes
    store.notify(err.message || '删除文档失败')
  }
}

async function copyNoteMarkdown(n: WorkNote) {
  const ok = await copyToClipboard(`# ${n.title}\n\n${n.content}`)
  store.notify(ok ? '已复制文档 Markdown' : '复制失败')
}

function downloadNoteMarkdown(n: WorkNote) {
  const blob = new Blob([`# ${n.title}\n\n${n.content}\n`], { type: 'text/markdown;charset=utf-8' })
  triggerBlobDownload(blob, `${n.title.replace(/[^\w\u4e00-\u9fa5-]+/g, '_')}.md`)
}

function quoteNoteToChat(n: WorkNote) {
  const targetBotId = pickDefaultWorkerBot(store.state.bots, store.activeBot.value, n.bot_id)
  if (!targetBotId) {
    store.notify('请先创建一个 Bot')
    return
  }
  const snippet = n.content ? `\n\n[参考文档：${n.title}]\n${n.content.slice(0, 600)}` : `\n\n[参考文档：${n.title}]`
  emit('closeModal')
  void store.jumpToBot(targetBotId, snippet.trimStart())
}

async function refineNoteWithBot(n: WorkNote) {
  const targetBotId = pickDefaultWorkerBot(store.state.bots, store.activeBot.value, n.bot_id)
  if (!targetBotId) {
    store.notify('请先创建一个 Bot')
    return
  }
  const prompt = `请读取工作文档《${n.title}》（id=${n.id}），帮我按当前需求进一步完善结构并直接通过 manage_notes 更新保存：\n\n[当前正文]\n${n.content.slice(0, 1200)}`
  emit('closeModal')
  await store.jumpToBot(targetBotId, prompt)
}
</script>

<template>
  <div class="flex-1 min-h-0 grid grid-cols-1 md:grid-cols-12 gap-3">
    <div class="md:col-span-4 min-h-0 flex flex-col gap-2 md:border-r md:border-line md:pr-2">
      <div class="flex items-center gap-2 shrink-0">
        <input
          v-model="noteSearch"
          type="search"
          aria-label="搜索文档"
          placeholder="搜索文档..."
          @keydown.escape="clearSearchOnEscape($event, noteSearch, () => { noteSearch = '' })"
          class="flex-1 min-w-0 min-h-[34px] rounded-xl border border-line bg-canvas px-2.5 text-xs text-fg"
        />
        <button
          type="button"
          @click="startNewNote"
          class="min-h-[34px] px-3 rounded-xl bg-accent hover:bg-accent-hover text-on-accent font-semibold text-xs cursor-pointer shrink-0"
        >
          + 新文档
        </button>
      </div>

      <div class="flex-1 min-h-0 overflow-y-auto flex flex-col gap-1.5">
        <div v-if="filteredNotes.length === 0" class="py-8 text-center text-xs text-fg-muted">
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
              ? 'border-accent-border bg-accent-soft'
              : 'border-line bg-canvas hover:border-line-strong',
          ]"
        >
          <div class="flex items-center justify-between gap-1.5">
            <strong class="text-xs text-fg truncate">{{ n.title }}</strong>
            <span v-if="n.pinned" class="font-mono text-xs text-warn shrink-0">置顶</span>
          </div>
          <div class="flex items-center justify-between gap-2 mt-0.5 text-xs font-mono text-fg-muted">
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
            class="sm:col-span-7 min-h-[36px] rounded-xl border border-line bg-canvas px-3 text-xs text-fg"
          />
          <input
            v-model="draftNoteCategory"
            type="text"
            list="workhub-note-categories"
            placeholder="输入或选择文档分类..."
            class="sm:col-span-3 min-h-[36px] rounded-xl border border-line bg-canvas px-2.5 font-mono text-xs text-fg"
          />
          <datalist id="workhub-note-categories">
            <option v-for="cat in noteCategoryChoices" :key="cat" :value="cat" />
          </datalist>
          <label class="sm:col-span-2 inline-flex items-center justify-center gap-1.5 text-xs text-fg-sub cursor-pointer">
            <input v-model="draftNotePinned" type="checkbox" class="accent-accent" />
            <span>置顶</span>
          </label>
        </div>
        <div class="flex items-center gap-1.5 flex-wrap text-xs shrink-0">
          <span class="text-fg-muted">分类快选:</span>
          <button
            v-for="cat in noteCategoryChoices"
            :key="cat"
            type="button"
            @click="draftNoteCategory = cat"
            :class="[
              'px-2 py-0.5 rounded-lg border font-mono cursor-pointer transition-colors',
              draftNoteCategory === cat
                ? 'border-accent-border bg-accent-soft text-accent-fg'
                : 'border-line text-fg-muted hover:text-fg'
            ]"
          >
            {{ cat }}
          </button>
        </div>
        <textarea
          v-model="draftNoteContent"
          placeholder="编写或粘贴 Markdown 文档正文（Cmd/Ctrl + Enter 快速保存）..."
          @keydown.meta.enter.prevent="handleSaveNote"
          @keydown.ctrl.enter.prevent="handleSaveNote"
          class="flex-1 min-h-[220px] rounded-xl border border-line bg-canvas p-3 font-mono text-xs text-fg resize-none"
        />
        <div class="flex items-center justify-between gap-2 shrink-0">
          <span class="font-mono text-xs text-fg-muted">
            {{ draftNoteContent.length }} 字符 · {{ draftNoteContent ? draftNoteContent.split('\n').length : 0 }} 行
          </span>
          <div class="flex items-center gap-2">
            <button
              type="button"
              @click="editingNote = false"
              class="px-3.5 py-1.5 rounded-xl border border-line text-xs text-fg-sub hover:text-fg cursor-pointer"
            >
              取消
            </button>
            <button
              type="submit"
              :disabled="savingNote"
              class="px-4 py-1.5 rounded-xl bg-accent hover:bg-accent-hover text-on-accent font-semibold text-xs cursor-pointer disabled:opacity-50"
            >
              {{ savingNote ? '保存中...' : '保存文档' }}
            </button>
          </div>
        </div>
      </form>

      <div v-else-if="selectedNote" class="flex-1 min-h-0 flex flex-col gap-2.5">
        <div class="flex items-center justify-between gap-2 flex-wrap pb-2 border-b border-line shrink-0">
          <div class="min-w-0">
            <h3 class="text-sm font-semibold text-fg truncate">{{ selectedNote.title }}</h3>
            <div class="text-xs font-mono text-fg-muted mt-0.5">
              {{ selectedNote.category }}
              <template v-if="selectedNote.bot_id"> · 来自 {{ botNameById(selectedNote.bot_id) }}</template>
              · 更新于 {{ formatTime(selectedNote.updated_at) }}
            </div>
          </div>
          <div class="flex items-center gap-1.5 flex-wrap shrink-0">
            <button
              type="button"
              @click="quoteNoteToChat(selectedNote)"
              class="px-2.5 py-1 rounded-lg border border-info-border text-xs text-info hover:bg-info-soft cursor-pointer"
            >
              引用进对话
            </button>
            <button
              type="button"
              @click="refineNoteWithBot(selectedNote)"
              class="px-2.5 py-1 rounded-lg border border-accent-border bg-accent-soft text-xs text-accent-fg hover:bg-accent-soft cursor-pointer"
            >
              派 Bot 润色
            </button>
            <button
              type="button"
              @click="startEditNote(selectedNote)"
              class="px-2.5 py-1 rounded-lg border border-line text-xs text-fg-sub hover:text-fg cursor-pointer"
            >
              编辑
            </button>
            <button
              type="button"
              @click="copyNoteMarkdown(selectedNote)"
              class="px-2.5 py-1 rounded-lg border border-line text-xs text-fg-sub hover:text-accent-fg cursor-pointer"
            >
              复制
            </button>
            <button
              type="button"
              @click="downloadNoteMarkdown(selectedNote)"
              class="px-2.5 py-1 rounded-lg border border-line text-xs text-fg-sub hover:text-accent-fg cursor-pointer"
            >
              下载 .md
            </button>
            <button
              type="button"
              @click="removeNote(selectedNote.id)"
              class="px-2.5 py-1 rounded-lg border border-danger-border text-xs text-danger hover:bg-danger-soft cursor-pointer"
            >
              删除
            </button>
          </div>
        </div>
        <div @click="handleNoteMarkdownClick" class="flex-1 min-h-0 overflow-y-auto pr-1 text-xs leading-relaxed">
          <div v-if="selectedNote.content" v-html="renderSafeMarkdown(selectedNote.content)" />
          <div v-else class="text-fg-muted py-8 text-center">文档正文为空，可点击「编辑」写入内容</div>
        </div>
      </div>

      <div v-else class="flex-1 flex flex-col items-center justify-center text-center px-6 text-xs text-fg-muted gap-2">
        <span>这里统一汇总你与各专属助手产出的报告、计划、邮件草稿与 Markdown 文档。</span>
        <button
          type="button"
          @click="startNewNote"
          class="px-3.5 py-1.5 rounded-xl border border-accent-border text-accent-fg hover:bg-accent-soft cursor-pointer"
        >
          + 创建第一份工作文档
        </button>
      </div>
    </div>
  </div>
</template>
