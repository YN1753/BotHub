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

const greetingLine = computed(() => {
  const hr = new Date().getHours()
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
  </div>
</template>
