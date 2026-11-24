<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { formatTime, formatTimelineDivider } from '../../utils/format'
import { renderSafeMarkdown } from '../../utils/markdown'
import type { ChatMessageUI } from '../../utils/chatMapper'
import BotAvatar from '../BotAvatar.vue'
import BotToolTrail from './BotToolTrail.vue'
import BotInlineCards from './BotInlineCards.vue'

const props = defineProps<{
  message: ChatMessageUI
  showTimestamp: boolean
  deferred: boolean
  focusedMatch: boolean
  isGroupBot: boolean
  activeBotId?: string
  isLast: boolean
  canRetry: boolean
}>()

const emit = defineEmits<{
  quote: [msg: ChatMessageUI]
  editResend: [msg: ChatMessageUI]
  copyMarkdown: [text: string]
  copyPlain: [text: string]
  fork: [serverId: number]
  delete: [msg: ChatMessageUI]
  mention: [tag: string]
  jumpBot: [botId: string]
  continue: []
  retry: []
  retryModel: [modelId: string]
  saveMemory: [msg: ChatMessageUI]
  convertTodo: [msg: ChatMessageUI]
  saveNote: [msg: ChatMessageUI]
  forward: [msg: ChatMessageUI]
}>()

const store = useAppStore()
const expandedUser = ref(false)
const touchActionsOpen = ref(false)
const rowMaxWidthClass = computed(() => (store.state.chatWide ? 'max-w-[1120px]' : 'max-w-[840px]'))

const retryModelChoices = computed(() => {
  const opts = store.state.modelOptions || []
  if (opts.length > 0) {
    return opts.filter((o) => o.available).map((o) => ({
      id: o.id,
      label: o.label,
    }))
  }
  return [{ id: 'auto', label: '自动路由 · 统一模型池' }]
})

function handleTouchMessageTap(e: MouseEvent) {
  const target = e.target as HTMLElement | null
  if (!target) return
  if (target.closest('a, button, summary, details, pre, code')) return
  if (typeof window !== 'undefined' && window.getSelection()?.toString()) return
  touchActionsOpen.value = !touchActionsOpen.value
}

const isLongUserMessage = computed(() => {
  const c = props.message.content
  if (!c) return false
  return c.length > 260 || c.split('\n').length > 6
})

const renderedMarkdownHtml = computed(() =>
  props.message.content ? renderSafeMarkdown(props.message.content) : ''
)

function closeDetailsMenu(e: Event) {
  ;(e.currentTarget as HTMLElement | null)?.closest('details')?.removeAttribute('open')
}
</script>

<template>
  <!-- 5-Minute Time Cluster Divider -->
  <div v-if="showTimestamp" class="flex justify-center py-0.5 select-none">
    <span class="px-2.5 py-0.5 rounded-full bg-subtle text-xs font-mono text-fg-muted">
      {{ formatTimelineDivider(message.createdAt) }}
    </span>
  </div>

  <!-- Centered System Dynamic Pill -->
  <div
    v-if="message.role === 'system' || message.msgKind === 'system'"
    :class="[rowMaxWidthClass, 'w-full mx-auto flex justify-center my-0.5', message.isLiveNew ? 'msg-enter' : '']"
  >
    <div class="px-3.5 py-1 rounded-full border border-line bg-subtle text-xs text-fg-muted text-center whitespace-pre-line leading-relaxed">
      {{ message.content }}
    </div>
  </div>

  <!-- User Message Pill -->
  <div
    v-else-if="message.role === 'user'"
    :data-msg-id="message.localId"
    :data-touch-open="touchActionsOpen ? 'true' : undefined"
    @click="handleTouchMessageTap"
    :class="[
      'reveal-host w-full mx-auto flex flex-col items-end gap-1 rounded-xl transition-shadow',
      rowMaxWidthClass,
      message.isLiveNew ? 'msg-enter-user' : '',
      deferred ? 'msg-deferred-user' : '',
      focusedMatch ? 'ring-1 ring-accent p-1' : '',
    ]"
  >
    <div
      :title="formatTime(message.createdAt)"
      class="msg-bubble-user max-w-[85%] sm:max-w-[72%] rounded-xl px-4 py-2.5 text-sm text-on-accent leading-relaxed whitespace-pre-wrap"
    >
      <div
        v-if="message.replyPreview"
        class="mb-2 pl-2.5 border-l-2 border-on-accent/60 text-xs text-on-accent/90 bg-black/15 rounded-r-lg py-1 pr-2 line-clamp-2"
      >
        {{ message.replyPreview }}
      </div>
      <div v-if="message.attachments && message.attachments.length > 0" class="mb-2 flex flex-wrap gap-1.5">
        <a
          v-for="(imgUrl, imgIdx) in message.attachments"
          :key="imgUrl + imgIdx"
          :href="imgUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="block rounded-lg overflow-hidden border border-white/20 bg-black/10"
        >
          <img
            :src="imgUrl"
            alt="对话附带图片"
            loading="lazy"
            class="max-h-52 max-w-[240px] object-cover block"
          />
        </a>
      </div>
      <div :class="isLongUserMessage && !expandedUser ? 'line-clamp-5' : ''">
        {{ message.content }}
      </div>
      <button
        v-if="isLongUserMessage"
        type="button"
        @click="expandedUser = !expandedUser"
        class="mt-1.5 font-mono text-xs text-on-accent/90 hover:text-on-accent hover:underline cursor-pointer"
      >
        {{ expandedUser ? '收起长消息' : '展开全文' }}
      </button>
    </div>
    <div class="reveal-actions msg-action-bar inline-flex items-center gap-0.5 rounded-full px-1.5 py-0.5">
      <button
        type="button"
        @click="emit('quote', message)"
        class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs text-fg-muted hover:text-fg hover:bg-hover cursor-pointer"
      >
        <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><polyline points="9 14 4 9 9 4" /><path d="M20 20v-7a4 4 0 0 0-4-4H4" /></svg>
        <span>引用</span>
      </button>
      <button
        type="button"
        @click="emit('editResend', message)"
        class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs text-fg-muted hover:text-fg hover:bg-hover cursor-pointer"
      >
        <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 20h9" /><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" /></svg>
        <span>重编</span>
      </button>
      <button
        type="button"
        @click="emit('copyMarkdown', message.content)"
        class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs text-fg-muted hover:text-fg hover:bg-hover cursor-pointer"
      >
        <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="9" y="9" width="13" height="13" rx="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></svg>
        <span>复制</span>
      </button>
      <button
        v-if="message.serverId"
        type="button"
        @click="emit('fork', message.serverId)"
        class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs text-fg-muted hover:text-ok hover:bg-hover cursor-pointer"
      >
        <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="18" cy="18" r="3" /><circle cx="6" cy="6" r="3" /><path d="M6 21V9a9 9 0 0 0 9 9" /></svg>
        <span>分支</span>
      </button>
      <button
        v-if="message.serverId"
        type="button"
        @click="emit('delete', message)"
        class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs text-fg-muted hover:text-danger hover:bg-danger-soft cursor-pointer"
      >
        <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6" /></svg>
        <span>删除</span>
      </button>
    </div>
  </div>

  <!-- Open Editorial Left-Avatar Bot Message -->
  <article
    v-else
    :data-msg-id="message.localId"
    :data-is-last="isLast ? 'true' : undefined"
    :data-touch-open="touchActionsOpen ? 'true' : undefined"
    @click="handleTouchMessageTap"
    :class="[
      'reveal-host w-full mx-auto flex items-start gap-3 sm:gap-3.5 rounded-xl transition-shadow',
      rowMaxWidthClass,
      message.isLiveNew ? 'msg-enter-bot' : '',
      deferred ? 'msg-deferred-bot' : '',
      focusedMatch ? 'ring-1 ring-accent bg-accent-soft p-2' : '',
    ]"
  >
    <div class="flex flex-col items-center self-stretch shrink-0 pt-0.5">
      <button
        type="button"
        @click="emit('mention', `@${message.senderName} `)"
        :title="`点击 @${message.senderName}`"
        class="cursor-pointer rounded-full"
      >
        <BotAvatar
          :avatar-style="message.avatarStyle"
          :state="message.streaming ? 'working' : message.msgKind === 'routine' ? 'done' : 'idle'"
          size="sm"
        />
      </button>
      <span
        v-if="message.reasoning || message.tools.length || message.streaming"
        aria-hidden="true"
        class="w-px flex-1 mt-2 bg-line"
      />
    </div>

    <div
      :class="[
        'flex-1 min-w-0 flex flex-col gap-2',
        message.msgKind === 'routine' ? 'vesper-card rounded-xl border-accent-border p-4' : 'pt-0.5',
      ]"
    >
      <div class="flex items-center justify-between gap-2 text-xs text-fg-muted">
        <div class="flex items-center gap-2 min-w-0">
          <button
            type="button"
            @click="emit('mention', `@${message.senderName} `)"
            class="text-fg hover:text-accent-fg hover:underline font-medium truncate cursor-pointer"
          >
            {{ message.senderName }}
          </button>
          <span
            v-if="message.roleTitle && isGroupBot"
            class="hidden sm:inline-flex font-mono text-xs text-fg-muted truncate max-w-[160px]"
          >
            {{ message.roleTitle }}
          </span>
          <span
            v-if="message.isTempGuest"
            class="font-mono text-xs px-1.5 py-0.5 rounded-full border border-ok-border bg-ok-soft text-ok shrink-0"
          >
            外援
          </span>
          <span
            v-if="message.msgKind === 'routine'"
            class="font-mono text-xs px-2 py-0.5 rounded-full bg-accent-soft text-accent-fg shrink-0"
          >
            自动汇报
          </span>
        </div>
        <div class="flex items-center gap-1.5 shrink-0">
          <span
            v-if="message.model && !message.streaming"
            :class="[isLast ? 'hidden sm:inline-flex' : 'reveal-actions hidden sm:inline-flex', 'font-mono text-xs text-fg-muted']"
          >
            {{ message.model }}
          </span>
          <button
            v-if="isGroupBot && message.senderBotId && message.senderBotId !== activeBotId"
            type="button"
            @click="emit('jumpBot', message.senderBotId)"
            class="reveal-actions font-mono text-xs px-2 py-0.5 rounded-full border border-line text-fg-muted hover:text-fg cursor-pointer"
          >
            私聊
          </button>
          <span :class="['font-mono text-xs', message.errorText ? 'text-danger' : 'text-fg-muted']">
            {{ message.statusText }}
          </span>
        </div>
      </div>

      <!-- Calm Thinking Waveform -->
      <div
        v-if="message.streaming && !message.content"
        class="py-1 flex items-center gap-2 text-xs text-fg-muted"
        aria-live="polite"
      >
        <span class="thinking-dots inline-flex items-center gap-1" aria-hidden="true">
          <span /><span /><span />
        </span>
        <span>{{ message.reasoning ? '正在深度思考' : message.tools.length ? '正在检索与整理结果' : '正在构思' }}</span>
      </div>

      <!-- Compact Pill Thinking + Tool Trail Row -->
      <div v-if="message.reasoning || message.tools.length" class="flex flex-wrap items-start gap-2">
        <details
          v-if="message.reasoning"
          class="vesper-details well-recessed w-fit max-w-full rounded-full px-3 py-1 text-xs text-fg-muted [&[open]]:basis-full [&[open]]:w-full [&[open]]:rounded-xl [&[open]]:px-3.5 [&[open]]:py-2 transition-[border-radius] duration-150"
        >
          <summary class="cursor-pointer flex items-center gap-1.5 text-accent-fg select-none">
            <span
              :class="[
                'w-1.5 h-1.5 rounded-full shrink-0',
                message.streaming && !message.content ? 'bg-accent animate-pulse' : 'bg-accent/75',
              ]"
              aria-hidden="true"
            />
            <svg viewBox="0 0 24 24" width="12.5" height="12.5" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="details-chevron shrink-0">
              <polyline points="9 18 15 12 9 6" />
            </svg>
            <span>思考过程</span>
            <span class="font-mono text-xs text-fg-muted">({{ message.reasoning.length }} 字)</span>
          </summary>
          <div class="mt-2 pt-2 border-t border-line whitespace-pre-wrap leading-relaxed text-fg-sub max-h-60 overflow-y-auto">
            {{ message.reasoning.replace(/<\/?think>/g, '').trim() }}
          </div>
        </details>

        <BotToolTrail :tools="message.tools" />
      </div>

      <!-- Inline Structured Data Bento Cards -->
      <BotInlineCards v-if="message.tools.length" :tools="message.tools" />

      <!-- Open Editorial Markdown Body -->
      <div
        v-if="renderedMarkdownHtml"
        :class="message.streaming ? 'md-streaming' : ''"
        v-html="renderedMarkdownHtml"
      />
      <div v-else-if="message.errorText" class="flex items-center justify-between gap-2 flex-wrap text-sm text-danger">
        <span>[调用失败] {{ message.errorText }}</span>
        <button
          v-if="canRetry"
          type="button"
          @click="emit('retry')"
          class="px-2.5 py-1 rounded-lg border border-danger-border bg-danger-soft hover:opacity-90 text-xs text-danger cursor-pointer shrink-0"
        >
          立即重试
        </button>
      </div>

      <!-- Refined Micro-Action Pill + Progressive 'More' Menu -->
      <div v-if="!message.streaming && message.content" class="reveal-actions pt-0.5">
        <div class="msg-action-bar inline-flex flex-wrap items-center gap-0.5 rounded-full px-1.5 py-0.5">
          <button
            type="button"
            @click="emit('copyMarkdown', message.content)"
            class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs text-fg-sub hover:text-fg hover:bg-hover cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="9" y="9" width="13" height="13" rx="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></svg>
            <span>复制</span>
          </button>

          <button
            type="button"
            @click="emit('quote', message)"
            class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs text-fg-sub hover:text-fg hover:bg-hover cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><polyline points="9 14 4 9 9 4" /><path d="M20 20v-7a4 4 0 0 0-4-4H4" /></svg>
            <span>引用</span>
          </button>

          <button
            v-if="isLast"
            type="button"
            @click="emit('continue')"
            class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs text-accent-fg hover:bg-accent-soft cursor-pointer font-medium"
          >
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><polyline points="15 10 20 15 15 20" /><path d="M4 4v7a4 4 0 0 0 4 4h12" /></svg>
            <span>追问</span>
          </button>
          <button
            v-else-if="canRetry"
            type="button"
            @click="emit('retry')"
            class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs text-fg-sub hover:text-fg hover:bg-hover cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><polyline points="23 4 23 10 17 10" /><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" /></svg>
            <span>重试</span>
          </button>

          <details v-if="canRetry" class="msg-more-menu relative">
            <summary class="px-2.5 py-0.5 rounded-full text-xs text-fg-sub hover:text-fg hover:bg-hover cursor-pointer select-none inline-flex items-center gap-1">
              <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><polygon points="12 2 2 7 12 12 22 7 12 2" /><polyline points="2 17 12 22 22 17" /><polyline points="2 12 12 17 22 12" /></svg>
              <span>换模型</span>
              <svg viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true"><polyline points="6 9 12 15 18 9" /></svg>
            </summary>
            <div class="aero-glass absolute right-0 sm:left-0 sm:right-auto bottom-full mb-1.5 z-30 min-w-[14rem] max-w-[19rem] max-h-60 overflow-y-auto rounded-lg border border-line-strong p-1 flex flex-col gap-0.5">
              <div class="px-2 py-1 text-xs text-fg-muted border-b border-line">
                切换模型原位重答
              </div>
              <button
                v-for="choice in retryModelChoices"
                :key="choice.id"
                type="button"
                @click="emit('retryModel', choice.id); closeDetailsMenu($event)"
                class="px-2.5 py-1.5 rounded-lg text-xs text-left truncate cursor-pointer text-fg hover:bg-accent-soft hover:text-accent-fg"
              >
                {{ choice.label }}
              </button>
            </div>
          </details>

          <details class="msg-more-menu relative">
            <summary class="px-2.5 py-0.5 rounded-full text-xs text-fg-muted hover:text-fg hover:bg-hover cursor-pointer select-none inline-flex items-center gap-1">
              <svg viewBox="0 0 24 24" width="12" height="12" fill="currentColor" aria-hidden="true"><circle cx="5" cy="12" r="1.75" /><circle cx="12" cy="12" r="1.75" /><circle cx="19" cy="12" r="1.75" /></svg>
              <span>更多</span>
              <svg viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true"><polyline points="6 9 12 15 18 9" /></svg>
            </summary>
            <div class="aero-glass absolute right-0 sm:left-0 sm:right-auto bottom-full mb-1.5 z-30 min-w-[8.5rem] rounded-lg border border-line-strong p-1 flex flex-col gap-0.5">
              <button type="button" @click="emit('saveMemory', message); closeDetailsMenu($event)" class="px-2.5 py-1.5 rounded-lg text-xs text-fg hover:bg-ok-soft hover:text-ok text-left cursor-pointer">存为长期记忆</button>
              <button type="button" @click="emit('convertTodo', message); closeDetailsMenu($event)" class="px-2.5 py-1.5 rounded-lg text-xs text-fg hover:bg-info-soft hover:text-info text-left cursor-pointer">转为个人待办</button>
              <button type="button" @click="emit('saveNote', message); closeDetailsMenu($event)" class="px-2.5 py-1.5 rounded-lg text-xs text-fg hover:bg-accent-soft hover:text-accent-fg text-left cursor-pointer">存为工作文档</button>
              <button type="button" @click="emit('forward', message); closeDetailsMenu($event)" class="px-2.5 py-1.5 rounded-lg text-xs text-fg hover:bg-hover text-left cursor-pointer">转发给其他 Bot</button>
              <button type="button" @click="emit('copyPlain', message.content); closeDetailsMenu($event)" class="px-2.5 py-1.5 rounded-lg text-xs text-fg-sub hover:bg-hover text-left cursor-pointer">复制纯文本</button>
              <button v-if="message.serverId" type="button" @click="emit('fork', message.serverId); closeDetailsMenu($event)" class="px-2.5 py-1.5 rounded-lg text-xs text-fg-sub hover:bg-ok-soft hover:text-ok text-left cursor-pointer">从此分支新话题</button>
              <button v-if="message.serverId" type="button" @click="emit('delete', message); closeDetailsMenu($event)" class="px-2.5 py-1.5 rounded-lg text-xs text-danger hover:bg-danger-soft text-left cursor-pointer">删除此消息</button>
            </div>
          </details>
        </div>
      </div>
    </div>
  </article>
</template>
