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
  <!-- Auto-Hiding Multi-Topic Chip Rail (Zero Height When Only Main Timeline Exists!) -->
  <div
    v-if="store.state.botThreads.length > 0"
    class="chip-rail px-4 md:px-6 pb-1 flex items-center gap-1.5 overflow-x-auto shrink-0 no-scrollbar"
  >
    <button
      type="button"
      @click="store.switchThread('')"
      :class="[
        'snap-chip px-2.5 py-0.5 rounded-full text-[11px] whitespace-nowrap cursor-pointer transition-all',
        !store.state.activeThreadId
          ? 'btn-tactile-gold font-medium'
          : 'text-slate-400 hover:text-slate-200 hover:bg-white/[0.05]',
      ]"
    >
      # 主时间线
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
        class="plain-inline-input w-32 bg-transparent px-2.5 py-0.5 rounded-full border border-[#f2b880]/60 text-[11px] text-white focus:outline-none"
      />
      <button
        v-else
        type="button"
        :title="th.summary ? `${th.summary}（双击重命名）` : '点击切换 · 双击重命名'"
        @click="store.switchThread(th.id)"
        @dblclick="startRenameThread(th.id, th.title)"
        :class="[
          'px-2.5 py-0.5 rounded-full text-[11px] whitespace-nowrap cursor-pointer transition-all',
          store.state.activeThreadId === th.id
            ? 'btn-tactile-gold font-medium'
            : 'text-slate-400 hover:text-slate-200 hover:bg-white/[0.05]',
        ]"
      >
        # {{ th.title }}
      </button>
    </div>
  </div>

  <!-- Contextual Active Thread Summary Capsule -->
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
      class="mx-4 md:mx-6 mb-1.5 px-3.5 py-1.5 rounded-2xl border border-emerald-400/20 bg-emerald-500/[0.06] flex items-start justify-between gap-2 text-[11px] shrink-0"
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
          class="px-2 py-0.5 rounded-full text-[10.5px] text-slate-300 hover:text-white hover:bg-white/[0.08] cursor-pointer"
        >
          存文档
        </button>
        <button
          type="button"
          @click="carrySummaryToNewThread"
          class="px-2 py-0.5 rounded-full text-[10.5px] text-emerald-300 hover:bg-emerald-400/15 cursor-pointer"
        >
          续开
        </button>
      </div>
    </div>
  </Transition>
</template>
