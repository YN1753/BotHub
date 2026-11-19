<script setup lang="ts">
import { computed } from 'vue'
import type { Bot } from '../../types/bothub'
import BotAvatar from '../BotAvatar.vue'
import BotGroupAvatar from '../BotGroupAvatar.vue'

const props = defineProps<{
  bot: Bot | null
  isGroupBot: boolean
  groupMembers: Bot[]
}>()

const emit = defineEmits<{
  pickPrompt: [prompt: string, prefillOnly?: boolean]
}>()

const isConcierge = computed(
  () => props.bot?.id === 'concierge' || props.bot?.name === '大管家'
)

const conciergeCards = [
  {
    title: '创建专属 Bot',
    desc: '自然语言描述定位，自动配好人设、工具与免费池模型',
    prompt: '请调用 manage_bots(action="create_bot") 帮我创建一个专属 Bot：',
    prefill: true,
  },
  {
    title: '组建多 Bot 协同群',
    desc: '把多位专员拉入讨论群，支持智能接力或全员并发',
    prompt: '请调用 manage_bots(action="create_group") 帮我拉一个多 Bot 协同群聊：',
    prefill: true,
  },
  {
    title: '巡检模型池与 7 日用量',
    desc: '查看免费/付费池各节点健康度、Token 消耗或解封冷却',
    prompt: '请调用 manage_models(action="status") 为我巡检当前免费池与付费池各模型健康状态及近 7 日调用量。',
    prefill: false,
  },
  {
    title: '今日工作简报与临期资产',
    desc: '汇总待办清单、日程例程与 30 天内即将到期资产',
    prompt: '请调用 workspace_briefing 与 manage_assets(action="expiring", within_days=30) 为我梳理今日重点待办与近期到期资产。',
    prefill: false,
  },
]

const greetingLine = computed(() => {
  const hr = new Date().getHours()
  if (isConcierge.value) {
    if (hr < 12) return '早安，需要新建 Bot、拉群协同还是巡检系统'
    if (hr < 18) return '下午好，大管家随时待命统筹全局'
    return '晚上好，有什么想法交给我落地为 Bot、提醒或清单'
  }
  const isCompanion = props.bot?.bot_mode === 'companion'
  if (hr < 5) return isCompanion ? '夜深了，想随便聊聊还是早点休息' : '夜深了，有什么交给我替你收尾'
  if (hr < 12) return isCompanion ? '早安，今天心情怎么样' : '早安，今天我们先推进哪件事'
  if (hr < 18) return isCompanion ? '下午好，忙里偷闲聊两句吧' : '下午好，随时准备协同执行'
  return isCompanion ? '晚上好，今天过得还好吗' : '晚上好，需要梳理待办还是检索资料'
})
</script>

<template>
  <div class="w-full max-w-[760px] mx-auto text-center px-4 pt-2 pb-4 flex flex-col items-center gap-3">
    <!-- Living Unboxed GrokBot Companion / Spirit Squad -->
    <div class="starter-stage-halo relative inline-flex items-center justify-center">
      <BotGroupAvatar
        v-if="isGroupBot"
        :members="groupMembers"
        :fallback-style="bot?.avatar_style"
        :state="bot?.state || 'idle'"
        size="xl"
      />
      <BotAvatar
        v-else
        :avatar-style="bot?.avatar_style"
        :state="bot?.state || 'idle'"
        size="xl"
        focusable
      />
    </div>

    <div class="flex flex-col items-center gap-1">
      <h2 class="text-xl sm:text-[26px] font-semibold text-white tracking-tight leading-snug">
        {{ greetingLine }}
      </h2>
      <p class="text-xs sm:text-[13px] text-slate-400 max-w-md">
        <span class="text-[#ffe2be] font-medium">{{ bot?.name }}</span>
        <template v-if="bot?.role_title"> · {{ bot.role_title }}</template>
        <template v-if="isGroupBot && groupMembers.length > 1"> · 点击任意糯团切至 C 位</template>
      </p>
    </div>

    <!-- Dedicated Zero-Friction Concierge Command Cards -->
    <div
      v-if="isConcierge"
      class="w-full max-w-[680px] grid grid-cols-1 sm:grid-cols-2 gap-2.5 pt-2 text-left"
    >
      <button
        v-for="card in conciergeCards"
        :key="card.title"
        type="button"
        @click="emit('pickPrompt', card.prompt, card.prefill)"
        class="group rounded-2xl border border-[#fff8ee]/12 bg-[#070b16]/75 hover:border-[#f2b880]/50 hover:bg-[#f2b880]/[0.07] px-3.5 py-2.5 flex flex-col gap-1 transition cursor-pointer text-left"
      >
        <div class="flex items-center justify-between gap-2">
          <span class="text-xs font-semibold text-white group-hover:text-[#ffe2be] transition-colors">
            {{ card.title }}
          </span>
          <span class="font-mono text-[10px] text-[#f2b880] opacity-75 group-hover:opacity-100">
            {{ card.prefill ? '填入指令 ->' : '一键执行 ->' }}
          </span>
        </div>
        <span class="text-[11px] text-slate-400 leading-snug">{{ card.desc }}</span>
      </button>
    </div>
  </div>
</template>
