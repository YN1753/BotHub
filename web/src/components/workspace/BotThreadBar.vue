<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import type { Bot } from '../../types/bothub'

const props = defineProps<{
  bot: Bot
}>()

const emit = defineEmits<{
  carrySummary: [summary: string, title: string]
}>()

const store = useAppStore()

const renamingThreadId = ref<string | null>(null)
const renameThreadTitle = ref('')
const renameInputRef = ref<HTMLInputElement | HTMLInputElement[] | null>(null)
const summarizingThread = ref(false)
const summaryExpanded = ref(false)

const activeThreadObj = computed(() =>
  store.state.activeThreadId
    ? store.state.botThreads.find((t) => t.id === store.state.activeThreadId) || null
    : null
)

function startRenameThread(id: string, curTitle: string) {
  renamingThreadId.value = id
  renameThreadTitle.value = curTitle
  nextTick(() => {
    const raw = renameInputRef.value
    const el = Array.isArray(raw) ? raw[0] : raw
    el?.focus()
    el?.select()
  })
}

async function commitRenameThread() {
  if (!renamingThreadId.value) return
  const title = renameThreadTitle.value.trim()
  if (title) {
    await store.renameThread(renamingThreadId.value, title)
  }
  renamingThreadId.value = null
}

async function handleSummarizeThread() {
  if (summarizingThread.value) return
  summarizingThread.value = true
  try {
    await store.summarizeThread(store.state.activeThreadId || 'main', true)
  } finally {
    summarizingThread.value = false
  }
}

async function saveThreadSummaryToNote() {
  const th = activeThreadObj.value
  if (!th?.summary) return
  try {
    await BotHubApi.createNote({
      title: `话题纪要: ${th.title || props.bot.name}`,
      category: 'summary',
      bot_id: props.bot.id,
      content: `# ${th.title || '话题纪要'}\n\n> 来源：${props.bot.name}\n\n${th.summary}\n`,
    })
    const notesRes = await BotHubApi.listNotes('', '')
    store.state.notes = notesRes.notes || []
    store.notify('前情提要已沉淀至工作文档')
  } catch (err: any) {
    store.notify(err.message || '保存文档失败')
  }
}

function carrySummaryToNewThread() {
  const th = activeThreadObj.value
  if (!th?.summary) return
  emit('carrySummary', th.summary, th.title || '延续话题')
}
</script>

<template>
  <!-- Per-Bot / Per-Group Scoped Topic Threads Bar -->
  <div
    class="chip-rail px-4 md:px-6 py-1.5 border-b border-[#fff8ee]/8 bg-[#050811]/75 flex items-center gap-1.5 overflow-x-auto shrink-0 no-scrollbar"
  >
    <span class="text-[10.5px] font-mono text-slate-500 shrink-0 mr-1">话题</span>
    <button
      type="button"
      @click="store.switchThread('')"
      :class="[
        'snap-chip px-2.5 py-1 rounded-lg text-[11px] whitespace-nowrap cursor-pointer transition-colors border',
        !store.state.activeThreadId
          ? 'border-[#f2b880]/55 bg-[#f2b880]/15 text-[#f7cba0] font-medium'
          : 'border-[#fff8ee]/10 bg-transparent text-slate-400 hover:text-slate-200',
      ]"
    >
      主时间线
    </button>

    <div
      v-for="th in store.state.botThreads"
      :key="th.id"
      class="snap-chip inline-flex items-center gap-1 shrink-0"
    >
      <input
        v-if="renamingThreadId === th.id"
        ref="renameInputRef"
        v-model="renameThreadTitle"
        type="text"
        aria-label="重命名话题"
        @keydown.enter.prevent="commitRenameThread"
        @keydown.escape.stop="renamingThreadId = null"
        @blur="commitRenameThread"
        class="w-32 px-2 py-0.5 rounded-lg border border-[#f2b880]/60 bg-[#070a16] text-[11px] text-white"
      />
      <div
        v-else
        :class="[
          'inline-flex items-center gap-1 rounded-lg border transition-colors',
          store.state.activeThreadId === th.id
            ? 'border-[#f2b880]/55 bg-[#f2b880]/15 text-[#f7cba0]'
            : 'border-[#fff8ee]/10 bg-transparent text-slate-400 hover:text-slate-200',
        ]"
      >
        <button
          type="button"
          :title="th.summary ? `${th.summary}（双击重命名）` : '点击切换话题 · 双击重命名'"
          @click="store.switchThread(th.id)"
          @dblclick="startRenameThread(th.id, th.title)"
          class="px-2.5 py-1 text-[11px] whitespace-nowrap cursor-pointer font-medium"
        >
          {{ th.title }}
          <span v-if="th.msg_count > 0" class="font-mono text-[10px] opacity-65 ml-0.5">
            ({{ th.msg_count }})
          </span>
        </button>
        <button
          v-if="store.state.activeThreadId === th.id"
          type="button"
          title="重命名话题"
          @click.stop="startRenameThread(th.id, th.title)"
          class="pr-1 text-[10px] text-slate-400 hover:text-[#f2b880] cursor-pointer"
        >
          改
        </button>
        <button
          v-if="store.state.activeThreadId === th.id"
          type="button"
          title="删除此话题"
          @click.stop="store.deleteThread(th.id)"
          class="pr-2 text-[10px] text-slate-400 hover:text-rose-300 cursor-pointer"
        >
          ×
        </button>
      </div>
    </div>

    <div class="ml-auto flex items-center gap-1.5 shrink-0">
      <button
        v-if="store.state.timeline.length >= 2"
        type="button"
        :disabled="summarizingThread"
        @click="handleSummarizeThread"
        title="提炼当前话题核心结论并存为专属记忆"
        class="px-2.5 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:border-emerald-400/45 hover:text-emerald-300 cursor-pointer whitespace-nowrap disabled:opacity-50"
      >
        {{ summarizingThread ? '提炼中...' : '总结存记忆' }}
      </button>
      <button
        type="button"
        @click="store.createThread('')"
        class="px-2.5 py-1 rounded-lg border border-dashed border-[#fff8ee]/16 text-[11px] text-slate-300 hover:border-[#f2b880]/50 hover:text-[#f2b880] cursor-pointer whitespace-nowrap"
      >
        + 新话题
      </button>
    </div>
  </div>

  <!-- Active Thread Summary Pill Banner -->
  <Transition
    enter-active-class="transition-[opacity,transform] duration-180 ease-[var(--ease-snappy)] motion-reduce:duration-100"
    enter-from-class="opacity-0 -translate-y-1.5 motion-reduce:translate-y-0"
    enter-to-class="opacity-100 translate-y-0"
    leave-active-class="transition-[opacity,transform] duration-120 ease-in motion-reduce:duration-75"
    leave-from-class="opacity-100 translate-y-0"
    leave-to-class="opacity-0 -translate-y-1 motion-reduce:translate-y-0"
  >
    <div
      v-if="activeThreadObj?.summary"
      class="px-4 md:px-6 py-1.5 border-b border-emerald-400/15 bg-emerald-500/[0.06] flex items-start justify-between gap-2 text-[11px] shrink-0"
    >
      <button
        type="button"
        @click="summaryExpanded = !summaryExpanded"
        class="flex items-start gap-1.5 min-w-0 flex-1 text-left cursor-pointer group"
      >
        <span class="px-1.5 py-0.5 rounded bg-emerald-400/15 text-emerald-300 font-mono text-[10px] shrink-0">
          前情提要
        </span>
        <span
          :class="[
            'text-slate-300 group-hover:text-white transition-colors',
            summaryExpanded ? 'whitespace-pre-wrap' : 'truncate',
          ]"
        >
          {{ activeThreadObj.summary }}
        </span>
      </button>
      <div class="flex items-center gap-1.5 shrink-0">
        <button
          type="button"
          @click="saveThreadSummaryToNote"
          class="px-2 py-0.5 rounded border border-[#fff8ee]/12 text-[10.5px] text-slate-300 hover:text-white hover:border-[#fff8ee]/30 cursor-pointer"
        >
          存成文档
        </button>
        <button
          type="button"
          @click="carrySummaryToNewThread"
          class="px-2 py-0.5 rounded border border-emerald-400/30 text-[10.5px] text-emerald-300 hover:bg-emerald-400/15 cursor-pointer"
        >
          带摘要续开
        </button>
      </div>
    </div>
  </Transition>
</template>
