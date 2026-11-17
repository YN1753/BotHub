<script setup lang="ts">
import { ref, computed, watch, nextTick, useId, onBeforeUnmount } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import { formatDurationMs } from '../../utils/format'
import { buildSlashCommands, filterSlashCommands, type SlashCmdItem } from '../../utils/slashCommands'
import { isImeComposingEnter } from '../../utils/topLayer'
import { pulseBotListening } from '../../utils/botGaze'
import type { ReplyQuoteTarget } from '../../composables/useBotChatStream'
import type { Bot } from '../../types/bothub'
import BotAvatar from '../BotAvatar.vue'
import CustomSelect from '../CustomSelect.vue'
import BotComposerDropdown from './BotComposerDropdown.vue'
import BotRelayStrip, { type RelayStepItem } from './BotRelayStrip.vue'

export type { RelayStepItem, ReplyQuoteTarget }

export interface StarterChipItem {
  id: string
  label: string
  highlight?: boolean
}

const props = defineProps<{
  inputPrompt: string
  selectedModel: string
  enableThinking: boolean
  streamStatus: string
  replyTarget: ReplyQuoteTarget | null
  relaySteps: RelayStepItem[]
  isGroupBot: boolean
  groupMembers: Bot[]
  lastPrompt: string
  centeredStage?: boolean
  starterChips?: StarterChipItem[]
}>()

const emit = defineEmits<{
  'update:inputPrompt': [val: string]
  'update:selectedModel': [val: string]
  'update:enableThinking': [val: boolean]
  'update:replyTarget': [val: ReplyQuoteTarget | null]
  submitPrompt: [overrideText?: string]
  stopStream: []
  clearTimeline: []
  summarizeThread: []
  pickStarter: [toolId: string]
}>()

const store = useAppStore()
const bot = store.activeBot
const composerId = useId()

const textareaRef = ref<HTMLTextAreaElement | null>(null)
const formRef = ref<HTMLFormElement | null>(null)

const mentionOpen = ref(false)
const mentionQuery = ref('')
const mentionAnchorIdx = ref(-1)
const mentionHighlight = ref(0)

const slashOpen = ref(false)
const slashHighlight = ref(0)

const streamElapsedMs = ref(0)
let streamTicker: number | null = null
let lastCompositionEndAt: number | null = null

watch(
  () => store.state.isStreaming,
  (active) => {
    if (streamTicker !== null) {
      window.clearInterval(streamTicker)
      streamTicker = null
    }
    if (active) {
      const started = Date.now()
      streamElapsedMs.value = 0
      streamTicker = window.setInterval(() => {
        streamElapsedMs.value = Date.now() - started
      }, 200)
    }
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  if (streamTicker !== null) {
    window.clearInterval(streamTicker)
    streamTicker = null
  }
})

const promptModel = computed({
  get: () => props.inputPrompt,
  set: (v: string) => emit('update:inputPrompt', v),
})

const modelValueProxy = computed({
  get: () => props.selectedModel,
  set: (v: string) => emit('update:selectedModel', v),
})

const availableSpecialistBots = computed(() =>
  store.state.bots.filter((b) => !b.is_group)
)

const mentionCandidates = computed(() => {
  const q = mentionQuery.value.toLowerCase()
  const memberIds = new Set(props.groupMembers.map((m) => m.id))
  const sorted = availableSpecialistBots.value.toSorted((a, b) => {
    const aIn = props.isGroupBot ? (memberIds.has(a.id) ? 0 : 1) : a.id === bot.value?.id ? 0 : 1
    const bIn = props.isGroupBot ? (memberIds.has(b.id) ? 0 : 1) : b.id === bot.value?.id ? 0 : 1
    return aIn - bIn
  })
  return sorted
    .filter((sp) => !q || `${sp.name} ${sp.id} ${sp.role_title}`.toLowerCase().includes(q))
    .map((sp) => ({
      bot: sp,
      inRoom: props.isGroupBot ? memberIds.has(sp.id) : sp.id === bot.value?.id,
    }))
})

const slashCommands = computed<SlashCmdItem[]>(() =>
  buildSlashCommands(bot.value, store.state.skills || [])
)

const filteredSlashCommands = computed(() =>
  filterSlashCommands(slashCommands.value, promptModel.value)
)

function focus() {
  textareaRef.value?.focus()
}

function closePopovers() {
  mentionOpen.value = false
  slashOpen.value = false
}

defineExpose({ focus, closePopovers })

watch(promptModel, (val) => {
  if (!val) {
    mentionOpen.value = false
    slashOpen.value = false
  } else {
    pulseBotListening()
  }
})

function handleCompositionEnd(e: CompositionEvent) {
  lastCompositionEndAt = e.timeStamp
}

function syncMentionAndSlash() {
  const el = textareaRef.value
  const val = promptModel.value
  const pos = el?.selectionStart ?? val.length

  if (val.startsWith('/') && !val.includes('\n') && !val.includes(' ')) {
    slashOpen.value = filteredSlashCommands.value.length > 0
    slashHighlight.value = 0
  } else {
    slashOpen.value = false
  }

  const beforeCaret = val.slice(0, pos)
  const match = beforeCaret.match(/(?:^|\s)@([^\s@]*)$/)
  if (match) {
    mentionQuery.value = match[1] || ''
    mentionAnchorIdx.value = beforeCaret.lastIndexOf('@')
    mentionHighlight.value = 0
    mentionOpen.value = mentionCandidates.value.length > 0
  } else {
    mentionOpen.value = false
  }
}

function appendMention(tag: string) {
  const prefix = promptModel.value && !promptModel.value.endsWith(' ') ? ' ' : ''
  promptModel.value = `${promptModel.value}${prefix}${tag}`
  nextTick(() => focus())
}

function pickMentionText(mentionName: string) {
  const val = promptModel.value
  const pos = textareaRef.value?.selectionStart ?? val.length
  const anchor = mentionAnchorIdx.value >= 0 ? mentionAnchorIdx.value : val.lastIndexOf('@', pos)
  if (anchor >= 0) {
    const before = val.slice(0, anchor)
    const after = val.slice(pos)
    promptModel.value = `${before}@${mentionName} ${after}`
  } else {
    appendMention(`@${mentionName} `)
  }
  mentionOpen.value = false
  nextTick(() => focus())
}

function pickMentionCandidate(sp: Bot) {
  pickMentionText(sp.name)
}

async function inviteFromMentionAndPick(sp: Bot) {
  if (props.isGroupBot && bot.value) {
    try {
      await BotHubApi.updateGroupMembers(bot.value.id, 'add', [sp.id])
      await store.refreshWorkspaceSlices(['bots'])
      store.notify(`已将「${sp.name}」正式拉入当前群聊`)
    } catch {
      // fallback to temp mention
    }
  }
  pickMentionCandidate(sp)
}

function pickSlashCommand(item: SlashCmdItem) {
  slashOpen.value = false
  if (item.action === 'open_drawer') {
    promptModel.value = ''
    store.state.showGroupDrawer = true
    return
  }
  if (item.action === 'clear_timeline') {
    promptModel.value = ''
    emit('clearTimeline')
    return
  }
  if (item.action === 'new_thread') {
    promptModel.value = ''
    void store.createThread('')
    return
  }
  if (item.action === 'summarize_thread') {
    promptModel.value = ''
    emit('summarizeThread')
    return
  }
  if (item.action === 'open_skills') {
    promptModel.value = ''
    store.state.showSkillsModal = true
    return
  }
  if (item.action === 'open_workhub') {
    promptModel.value = ''
    store.state.showWorkhubModal = true
    return
  }
  if (item.prompt) {
    if (item.prompt.endsWith('：')) {
      promptModel.value = item.prompt
      nextTick(() => focus())
    } else {
      promptModel.value = ''
      emit('submitPrompt', item.prompt)
    }
  }
}

async function saveLongDraftAsNote() {
  const text = promptModel.value.trim()
  if (!text || !bot.value) return
  try {
    const firstLine = text.split('\n').find((l) => l.trim()) || '草稿资料'
    const title = firstLine.replace(/^#+\s*/, '').slice(0, 32)
    const res = await BotHubApi.createNote({
      title: `参考资料: ${title}`,
      category: 'draft',
      bot_id: bot.value.id,
      content: text,
    })
    const notesRes = await BotHubApi.listNotes('', '')
    store.state.notes = notesRes.notes || []
    promptModel.value = `请使用 manage_notes(action="get", id="${res.note.id}") 读取我刚存入工作文档《${res.note.title}》的完整正文，帮我提炼要点并处理：`
    nextTick(() => focus())
    store.notify(`已将 ${text.length} 字存入工作文档`)
  } catch (err: any) {
    store.notify(err.message || '保存文档失败')
  }
}

function clearDraftPrompt() {
  promptModel.value = ''
  if (store.state.activeBotId) {
    store.setBotDraft(store.state.activeBotId, '', store.state.activeThreadId)
  }
  nextTick(() => focus())
}

function handleComposerKeydown(e: KeyboardEvent) {
  if (mentionOpen.value && mentionCandidates.value.length > 0) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      mentionHighlight.value = (mentionHighlight.value + 1) % mentionCandidates.value.length
      return
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      mentionHighlight.value = (mentionHighlight.value - 1 + mentionCandidates.value.length) % mentionCandidates.value.length
      return
    }
    if ((e.key === 'Enter' || e.key === 'Tab') && !isImeComposingEnter(e, lastCompositionEndAt)) {
      e.preventDefault()
      const picked = mentionCandidates.value[mentionHighlight.value]
      if (picked) pickMentionCandidate(picked.bot)
      return
    }
    if (e.key === 'Escape') {
      e.preventDefault()
      mentionOpen.value = false
      return
    }
  }

  if (slashOpen.value && filteredSlashCommands.value.length > 0) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      slashHighlight.value = (slashHighlight.value + 1) % filteredSlashCommands.value.length
      return
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      slashHighlight.value = (slashHighlight.value - 1 + filteredSlashCommands.value.length) % filteredSlashCommands.value.length
      return
    }
    if ((e.key === 'Enter' || e.key === 'Tab') && !isImeComposingEnter(e, lastCompositionEndAt)) {
      e.preventDefault()
      const picked = filteredSlashCommands.value[slashHighlight.value]
      if (picked) pickSlashCommand(picked)
      return
    }
    if (e.key === 'Escape') {
      e.preventDefault()
      slashOpen.value = false
      return
    }
  }

  if (e.key === 'Escape' && props.replyTarget) {
    e.preventDefault()
    e.stopPropagation()
    emit('update:replyTarget', null)
    return
  }

  if (e.key === 'ArrowUp' && !promptModel.value) {
    const fallbackPrompt =
      props.lastPrompt ||
      store.state.timeline.findLast((m) => m.role === 'user')?.content ||
      ''
    if (fallbackPrompt) {
      e.preventDefault()
      promptModel.value = fallbackPrompt
      return
    }
  }

  if ((e.key === 'Enter' && !e.shiftKey) || ((e.metaKey || e.ctrlKey) && e.key === 'Enter')) {
    e.preventDefault()
    if (isImeComposingEnter(e, lastCompositionEndAt)) return
    formRef.value?.requestSubmit()
  }
}
</script>

<template>
  <div class="relative px-3 sm:px-6 pt-1 pb-[max(0.75rem,env(safe-area-inset-bottom,0.75rem))] sm:pb-5">
    <BotRelayStrip :steps="relaySteps" />

    <BotComposerDropdown
      :is-group-bot="isGroupBot"
      :group-member-count="groupMembers.length"
      :mention-open="mentionOpen"
      :mention-query="mentionQuery"
      :mention-candidates="mentionCandidates"
      :mention-highlight="mentionHighlight"
      :slash-open="slashOpen"
      :slash-commands="filteredSlashCommands"
      :slash-highlight="slashHighlight"
      @pick-mention-text="pickMentionText"
      @pick-mention-candidate="pickMentionCandidate"
      @invite-from-mention-and-pick="inviteFromMentionAndPick"
      @pick-slash-command="pickSlashCommand"
    />

    <div
      v-if="isGroupBot && groupMembers.length > 0 && !centeredStage"
      class="max-w-[780px] w-full mx-auto mb-2 flex items-center gap-1.5 overflow-x-auto no-scrollbar"
    >
      <button
        v-for="member in groupMembers"
        :key="member.id"
        type="button"
        @click="appendMention(`@${member.name} `)"
        class="btn-tactile min-h-[30px] inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11.5px] shrink-0 cursor-pointer"
      >
        <BotAvatar :avatar-style="member.avatar_style" :state="member.state" size="xs" :interactive="false" />
        <span>@{{ member.name }}</span>
      </button>
    </div>

    <form
      ref="formRef"
      @submit.prevent="emit('submitPrompt')"
      :class="[
        'composer-dock max-w-[780px] w-full mx-auto rounded-[24px] px-4 sm:px-4.5 pt-3.5 pb-2.5 flex flex-col gap-2',
        store.state.isStreaming ? 'is-streaming' : '',
      ]"
    >
      <Transition
        enter-active-class="transition-[opacity,transform] duration-180 ease-[var(--ease-spring)] motion-reduce:duration-100"
        enter-from-class="opacity-0 -translate-y-1.5 scale-98 motion-reduce:translate-y-0 motion-reduce:scale-100"
        enter-to-class="opacity-100 translate-y-0 scale-100"
        leave-active-class="transition-[opacity,transform] duration-120 ease-in motion-reduce:duration-75"
        leave-from-class="opacity-100 translate-y-0 scale-100"
        leave-to-class="opacity-0 -translate-y-1 scale-98 motion-reduce:translate-y-0 motion-reduce:scale-100"
      >
        <div
          v-if="replyTarget"
          class="flex items-center justify-between gap-2 px-3 py-1.5 rounded-xl border border-[#f2b880]/25 bg-[#f2b880]/[0.06] text-xs"
        >
          <div class="flex items-center gap-2 min-w-0">
            <span class="w-0.5 h-4 rounded-full bg-[#f2b880] shrink-0" />
            <span class="text-slate-300 truncate">{{ replyTarget.preview }}</span>
          </div>
          <button
            type="button"
            aria-label="取消引用"
            @click="emit('update:replyTarget', null)"
            class="w-5 h-5 rounded-full inline-flex items-center justify-center text-slate-400 hover:text-white cursor-pointer shrink-0"
          >
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
      </Transition>

      <label :for="composerId" class="sr-only">向当前专属 Bot 下达指令</label>
      <div class="composer-grow-wrap" :data-replicated-value="promptModel">
        <textarea
          :id="composerId"
          ref="textareaRef"
          v-model="promptModel"
          rows="1"
          placeholder="输入指令，@ 召唤任何 Bot，/ 呼出快捷指令（Enter 发送）..."
          @input="syncMentionAndSlash"
          @compositionend="handleCompositionEnd"
          @keydown="handleComposerKeydown"
          class="auto-size-textarea composer-textarea w-full bg-transparent border-0 text-sm text-slate-100 placeholder-slate-500 resize-none focus:outline-none min-h-[24px] max-h-[160px] overflow-y-auto leading-relaxed"
        />
      </div>

      <div class="flex items-center justify-between gap-2 pt-1.5">
        <div class="flex items-center gap-1.5 min-w-0 flex-1 flex-wrap sm:flex-nowrap">
          <div class="min-w-[132px] max-w-[196px] shrink-0">
            <CustomSelect v-model="modelValueProxy" :options="store.modelOptions.value" compact aria-label="选择推理模型" />
          </div>
          <button
            type="button"
            :aria-pressed="enableThinking"
            @click="emit('update:enableThinking', !enableThinking)"
            :title="selectedModel.startsWith('pool:') || selectedModel === 'free-pool' ? '深度思考开关主要作用于 DeepSeek Flash / 支持 thinking 的中转站' : '开启或关闭深度推理思考链'"
            :class="[
              'min-h-[40px] sm:min-h-[34px] inline-flex items-center gap-1.5 px-3 rounded-xl text-xs cursor-pointer whitespace-nowrap shrink-0 focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#f2b880]',
              enableThinking ? 'btn-tactile-gold' : 'btn-tactile text-slate-400',
            ]"
          >
            <span
              :class="[
                'w-1.5 h-1.5 rounded-full transition-colors',
                enableThinking ? 'bg-[#f2b880] shadow-[0_0_8px_#f2b880]' : 'bg-slate-500',
              ]"
              aria-hidden="true"
            />
            <span>思考</span>
          </button>
          <span
            v-if="store.state.isStreaming"
            class="hidden sm:inline-flex items-center gap-1.5 font-mono text-[11px] text-[#ffe2be] truncate pl-1"
            aria-live="polite"
          >
            <span class="w-1.5 h-1.5 rounded-full bg-[#f2b880] animate-pulse shrink-0" aria-hidden="true" />
            <span class="truncate">
              {{ streamStatus }}<template v-if="streamElapsedMs >= 400"> · {{ formatDurationMs(streamElapsedMs) }}</template>
            </span>
          </span>
        </div>

        <div class="flex items-center gap-1.5 shrink-0">
          <button
            v-if="promptModel.length >= 1600 && !store.state.isStreaming"
            type="button"
            @click="saveLongDraftAsNote"
            title="把粘贴的长文档自动存进 Workhub 后按 ID 引用，对话更干净"
            class="hidden sm:inline-flex px-2.5 py-1 rounded-full border border-cyan-400/35 bg-cyan-500/10 text-[10.5px] text-cyan-200 hover:bg-cyan-500/20 cursor-pointer"
          >
            长文转文档 ({{ promptModel.length }}字)
          </button>
          <span
            v-else-if="promptModel.length > 120"
            class="hidden sm:inline font-mono text-[10.5px] text-slate-500 tabular-nums"
          >
            {{ promptModel.length }} 字
          </span>
          <button
            v-if="promptModel.trim().length > 20 && !store.state.isStreaming"
            type="button"
            @click="clearDraftPrompt"
            title="清空当前输入框草稿"
            class="inline-flex items-center min-h-[34px] px-2.5 py-1 rounded-full text-[11px] text-slate-400 hover:text-slate-200 hover:bg-white/[0.05] cursor-pointer"
          >
            清空
          </button>
          <button
            v-if="store.state.isStreaming"
            type="button"
            @click="emit('stopStream')"
            class="min-h-[38px] sm:min-h-[34px] px-3.5 rounded-full text-xs bg-rose-500/20 border border-rose-400/45 text-rose-300 hover:bg-rose-500/30 cursor-pointer"
          >
            停止
          </button>
          <button
            type="submit"
            :disabled="store.state.isStreaming || !promptModel.trim()"
            aria-label="发送指令"
            title="发送 (Enter)"
            class="btn-jewel w-10 h-10 sm:w-9 sm:h-9 rounded-full hover:scale-105 flex items-center justify-center focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#f2b880] cursor-pointer disabled:opacity-35 disabled:hover:scale-100 disabled:cursor-not-allowed"
          >
            <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2.5" aria-hidden="true">
              <line x1="12" y1="19" x2="12" y2="5" />
              <polyline points="5 12 12 5 19 12" />
            </svg>
          </button>
        </div>
      </div>
    </form>

    <div
      v-if="centeredStage && starterChips && starterChips.length > 0"
      class="max-w-[760px] w-full mx-auto mt-3.5 flex flex-wrap items-center justify-center gap-2"
    >
      <button
        v-for="chip in starterChips"
        :key="chip.id"
        type="button"
        @click="emit('pickStarter', chip.id)"
        :class="[
          'min-h-[34px] inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-full text-xs cursor-pointer',
          chip.highlight ? 'btn-tactile-gold font-medium' : 'btn-tactile text-slate-300',
        ]"
      >
        <span
          :class="[
            'w-1.5 h-1.5 rounded-full shrink-0',
            chip.highlight ? 'bg-[#f2b880] shadow-[0_0_6px_#f2b880]' : 'bg-slate-400/80',
          ]"
          aria-hidden="true"
        />
        <span>{{ chip.label }}</span>
      </button>
    </div>
  </div>
</template>
