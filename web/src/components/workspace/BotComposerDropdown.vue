<script setup lang="ts">
import type { Bot } from '../../types/bothub'
import BotAvatar from '../BotAvatar.vue'

interface MentionCandidateItem {
  bot: Bot
  inRoom: boolean
}

interface SlashCommandItem {
  cmd: string
  title: string
  desc: string
  prompt?: string
  action?: 'open_drawer' | 'clear_timeline' | 'new_thread' | 'summarize_thread' | 'open_skills' | 'open_workhub'
}

defineProps<{
  isGroupBot: boolean
  groupMemberCount: number
  mentionOpen: boolean
  mentionQuery: string
  mentionCandidates: MentionCandidateItem[]
  mentionHighlight: number
  slashOpen: boolean
  slashCommands: SlashCommandItem[]
  slashHighlight: number
}>()

const emit = defineEmits<{
  pickMentionText: [text: string]
  pickMentionCandidate: [bot: Bot]
  inviteFromMentionAndPick: [bot: Bot]
  pickSlashCommand: [cmd: SlashCommandItem]
}>()
</script>

<template>
  <!-- Interactive @Mention Autocomplete Popover -->
  <Transition
    enter-active-class="transition-[opacity,transform] duration-200 ease-[var(--ease-spring)] motion-reduce:duration-100"
    enter-from-class="opacity-0 translate-y-2 scale-95 motion-reduce:translate-y-0 motion-reduce:scale-100"
    enter-to-class="opacity-100 translate-y-0 scale-100"
    leave-active-class="transition-[opacity,transform] duration-130 ease-[var(--ease-in-snappy)] motion-reduce:duration-75"
    leave-from-class="opacity-100 translate-y-0 scale-100"
    leave-to-class="opacity-0 translate-y-1.5 scale-96 motion-reduce:translate-y-0 motion-reduce:scale-100"
  >
    <div
      v-if="mentionOpen && mentionCandidates.length > 0"
      class="origin-bottom max-w-[820px] w-full mx-auto mb-2 rounded-2xl border border-[#f2b880]/35 bg-[#090e1e]/96 backdrop-blur-2xl shadow-2xl p-1.5 flex flex-col gap-0.5 max-h-60 overflow-y-auto"
    >
      <div class="px-2.5 py-1 flex items-center justify-between text-[10.5px] font-mono text-slate-400">
        <span>选择要 @点名 或临时支援的 Bot</span>
        <span>Enter 确认 · Esc 关闭</span>
      </div>
      <button
        v-if="isGroupBot && (!mentionQuery || '全体成员所有人all'.includes(mentionQuery.toLowerCase()))"
        type="button"
        @click="emit('pickMentionText', '全体成员')"
        class="flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-xl bg-[#f2b880]/10 hover:bg-[#f2b880]/20 text-left cursor-pointer"
      >
        <div class="flex items-center gap-2 min-w-0">
          <span class="w-6 h-6 rounded-full bg-[#f2b880]/25 text-[#f2b880] font-mono text-[11px] font-bold flex items-center justify-center shrink-0">@</span>
          <div class="min-w-0">
            <div class="text-xs font-semibold text-[#f7cba0]">@全体成员</div>
            <div class="text-[10.5px] text-slate-400 truncate">唤起群内全部 {{ groupMemberCount }} 位成员协同回应</div>
          </div>
        </div>
        <span class="px-1.5 py-0.5 rounded font-mono text-[9.5px] bg-[#f2b880]/18 text-[#f2b880] shrink-0">群协同</span>
      </button>
      <div
        v-for="(item, idx) in mentionCandidates"
        :key="item.bot.id"
        :class="[
          'flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-xl transition-colors',
          idx === mentionHighlight ? 'bg-[#f2b880]/16' : 'hover:bg-white/[0.04]'
        ]"
      >
        <button
          type="button"
          @click="emit('pickMentionCandidate', item.bot)"
          class="flex-1 min-w-0 flex items-center gap-2.5 text-left cursor-pointer"
        >
          <BotAvatar :avatar-style="item.bot.avatar_style" :state="item.bot.state" size="sm" :interactive="false" />
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-1.5">
              <span class="text-xs font-semibold text-white truncate">@{{ item.bot.name }}</span>
              <span
                :class="[
                  'px-1.5 py-0.5 rounded font-mono text-[9.5px] leading-none',
                  item.inRoom ? 'bg-[#f2b880]/15 text-[#f2b880]' : 'bg-emerald-400/12 text-emerald-300'
                ]"
              >
                {{ item.inRoom ? (isGroupBot ? '群内成员' : '当前Bot') : '临时召唤外援' }}
              </span>
            </div>
            <div class="text-[10.5px] text-slate-400 truncate">{{ item.bot.role_title }}</div>
          </div>
        </button>
        <button
          v-if="isGroupBot && !item.inRoom"
          type="button"
          @click="emit('inviteFromMentionAndPick', item.bot)"
          class="px-2 py-1 rounded-lg border border-[#f2b880]/40 bg-[#f2b880]/12 text-[10.5px] font-medium text-[#f2b880] hover:bg-[#f2b880]/22 shrink-0 cursor-pointer"
        >
          拉群并@
        </button>
      </div>
    </div>
  </Transition>

  <!-- Interactive /Slash Command Palette -->
  <Transition
    enter-active-class="transition-[opacity,transform] duration-200 ease-[var(--ease-spring)] motion-reduce:duration-100"
    enter-from-class="opacity-0 translate-y-2 scale-95 motion-reduce:translate-y-0 motion-reduce:scale-100"
    enter-to-class="opacity-100 translate-y-0 scale-100"
    leave-active-class="transition-[opacity,transform] duration-130 ease-[var(--ease-in-snappy)] motion-reduce:duration-75"
    leave-from-class="opacity-100 translate-y-0 scale-100"
    leave-to-class="opacity-0 translate-y-1.5 scale-96 motion-reduce:translate-y-0 motion-reduce:scale-100"
  >
    <div
      v-if="slashOpen && slashCommands.length > 0"
      class="origin-bottom max-w-[820px] w-full mx-auto mb-2 rounded-2xl border border-[#f2b880]/35 bg-[#090e1e]/96 backdrop-blur-2xl shadow-2xl p-1.5 flex flex-col gap-0.5"
    >
      <div class="px-2.5 py-1 text-[10.5px] font-mono text-slate-400">快捷斜杠指令 (Enter 执行)</div>
      <button
        v-for="(cmd, idx) in slashCommands"
        :key="cmd.cmd"
        type="button"
        @click="emit('pickSlashCommand', cmd)"
        :class="[
          'w-full flex items-center justify-between gap-3 px-3 py-2 rounded-xl text-left cursor-pointer transition-colors',
          idx === slashHighlight ? 'bg-[#f2b880]/16 text-white' : 'text-slate-300 hover:bg-white/[0.04]'
        ]"
      >
        <div class="flex items-center gap-2.5 min-w-0">
          <span class="font-mono text-xs font-semibold text-[#f2b880] w-16 shrink-0">{{ cmd.cmd }}</span>
          <span class="text-xs font-medium truncate">{{ cmd.title }}</span>
        </div>
        <span class="text-[11px] text-slate-400 truncate">{{ cmd.desc }}</span>
      </button>
    </div>
  </Transition>
</template>
