<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { shortToolLabel } from '../../utils/toolMeta'
import { applySiblingStaggerFallback } from '../../utils/topLayer'
import type { Bot } from '../../types/bothub'
import BotAvatar from '../BotAvatar.vue'
import BotGroupAvatar from '../BotGroupAvatar.vue'

const props = defineProps<{
  bot: Bot | null
  isGroupBot: boolean
  groupMembers: Bot[]
}>()

const emit = defineEmits<{
  starter: [toolId: string]
}>()

const store = useAppStore()
const starterGridRef = ref<HTMLElement | null>(null)
const availableTools = computed(() => store.state.availableTools || [])

const botToolBadges = computed(() => {
  if (!props.bot) return []
  let allowed: string[] = []
  try {
    const parsed = JSON.parse(props.bot.allowed_tools || '[]')
    if (Array.isArray(parsed)) allowed = parsed
  } catch {
    allowed = []
  }
  return allowed.map((id) => {
    const found = availableTools.value.find((t) => t.id === id)
    return {
      id,
      desc: found?.description || id,
    }
  })
})

watch(
  () => [props.bot?.id, botToolBadges.value.length] as const,
  () => nextTick(() => applySiblingStaggerFallback(starterGridRef.value))
)

onMounted(() => {
  nextTick(() => applySiblingStaggerFallback(starterGridRef.value))
})
</script>

<template>
  <div class="m-auto max-w-[780px] w-full text-center py-6 px-2 flex flex-col items-center gap-3.5">
    <div class="relative inline-flex items-center justify-center mb-1">
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
      />
    </div>
    <div class="flex flex-col items-center gap-1.5">
      <h2 class="text-xl sm:text-2xl font-semibold text-white tracking-tight">{{ bot?.name }}</h2>
      <p class="text-xs sm:text-sm text-slate-400 max-w-md">
        {{ bot?.role_title }} · 输入消息开始对话，键入 <code class="font-mono text-[#f2b880]">@</code> 协同成员或 <code class="font-mono text-[#f2b880]">/</code> 触发指令
      </p>
      <div
        v-if="isGroupBot && groupMembers.length > 0"
        class="flex flex-wrap items-center justify-center gap-1.5 mt-1"
      >
        <span
          v-for="m in groupMembers.slice(0, 5)"
          :key="m.id"
          class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full border border-[#fff8ee]/10 bg-[#070b18]/75 text-[10.5px] text-slate-300"
        >
          <span class="w-1.5 h-1.5 rounded-full bg-cyan-300/75" aria-hidden="true" />
          {{ m.name }}
        </span>
        <span
          v-if="groupMembers.length > 5"
          class="text-[10.5px] font-mono text-slate-500"
        >
          +{{ groupMembers.length - 5 }}
        </span>
      </div>
    </div>

    <div
      v-if="botToolBadges.length > 0"
      ref="starterGridRef"
      class="stagger-list mt-2 flex flex-wrap items-center justify-center gap-2 max-w-2xl"
    >
      <button
        v-for="tool in botToolBadges"
        :key="tool.id"
        type="button"
        :title="`${tool.desc}（点击立即触发或填入指令）`"
        @click="emit('starter', tool.id)"
        class="stagger-item min-h-[34px] inline-flex items-center gap-1.5 px-3 py-1 rounded-full border border-[#fff8ee]/12 bg-[#070b18]/80 hover:border-[#f2b880]/55 hover:text-[#f7cba0] focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#f2b880] text-[11px] text-slate-300 cursor-pointer transition-colors"
      >
        <span class="w-1.5 h-1.5 rounded-full bg-[#f2b880]/75" aria-hidden="true" />
        <span>{{ shortToolLabel({ id: tool.id, description: tool.desc }) }}</span>
      </button>
    </div>

    <div
      v-if="bot?.bot_mode !== 'companion' && (store.pendingTodoCount.value > 0 || store.expiringAssetCount.value > 0)"
      class="mt-2 px-4 py-2.5 rounded-2xl border border-[#f2b880]/25 bg-[#f2b880]/[0.06] flex items-center justify-center gap-3 flex-wrap text-xs"
    >
      <span class="text-slate-300">
        当前工作区共有
        <strong v-if="store.pendingTodoCount.value > 0" class="text-cyan-200 font-mono"> {{ store.pendingTodoCount.value }} </strong>
        <template v-if="store.pendingTodoCount.value > 0">项未完待办</template>
        <template v-if="store.pendingTodoCount.value > 0 && store.expiringAssetCount.value > 0"> · </template>
        <strong v-if="store.expiringAssetCount.value > 0" class="text-amber-300 font-mono">{{ store.expiringAssetCount.value }} </strong>
        <template v-if="store.expiringAssetCount.value > 0">项 7 日内到期资产</template>
      </span>
      <button
        type="button"
        @click="emit('starter', 'workspace_briefing')"
        class="px-3 py-1 rounded-lg bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-[11px] cursor-pointer"
      >
        立即梳理今日安排
      </button>
    </div>
  </div>
</template>
