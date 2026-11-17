<script setup lang="ts">
import { computed } from 'vue'
import type { Bot } from '../types/bothub'
import BotAvatar from './BotAvatar.vue'

const props = withDefaults(
  defineProps<{
    members: Bot[]
    fallbackStyle?: string
    state?: string
    size?: 'sm' | 'md' | 'lg' | 'xl'
  }>(),
  {
    fallbackStyle: 'swarm:indigo',
    state: 'idle',
    size: 'md',
  }
)

const displayMembers = computed(() => props.members.slice(0, 4))

const boxClass = computed(() => {
  if (props.size === 'sm') return 'w-7 h-7 rounded-lg p-[2px] gap-[1px]'
  if (props.size === 'lg') return 'w-11 h-11 rounded-xl p-[3px] gap-[2px]'
  if (props.size === 'xl') return 'w-20 h-20 rounded-2xl p-1.5 gap-1'
  return 'w-9 h-9 rounded-xl p-[2.5px] gap-[1.5px]'
})

const isBusy = computed(() => props.state === 'working' || props.state === 'thinking')
</script>

<template>
  <BotAvatar
    v-if="displayMembers.length <= 1"
    :avatar-style="displayMembers[0]?.avatar_style || fallbackStyle"
    :state="state"
    :size="size"
  />
  <div
    v-else
    :class="[
      'shrink-0 relative grid grid-cols-2 place-items-center bg-[#080d1c] border overflow-hidden select-none transition-all duration-200',
      boxClass,
      isBusy
        ? 'border-[#f2b880]/70 shadow-[0_0_16px_rgba(242,184,128,0.28)]'
        : state === 'done'
          ? 'border-emerald-400/55 shadow-[0_0_10px_rgba(52,211,153,0.18)]'
          : 'border-[#fff8ee]/16 shadow-[inset_0_1px_0_rgba(255,248,238,0.08)]'
    ]"
  >
    <div
      v-for="m in displayMembers"
      :key="m.id"
      :class="[
        'w-full h-full flex items-center justify-center rounded-[4px] bg-[#0e1427]/90 overflow-hidden transition-colors',
        m.state === 'working' || m.state === 'thinking' ? 'ring-1 ring-[#f2b880]/75 bg-[#f2b880]/10' : ''
      ]"
    >
      <BotAvatar
        :avatar-style="m.avatar_style"
        :state="m.state"
        :size="size === 'xl' ? 'sm' : 'xs'"
        :interactive="false"
      />
    </div>
    <span
      v-if="members.length > 4"
      class="absolute bottom-0.5 right-0.5 px-1 rounded bg-[#070a13]/90 border border-[#fff8ee]/15 font-mono text-[8px] leading-tight text-[#f2b880]"
    >
      +{{ members.length - 4 }}
    </span>
  </div>
</template>
