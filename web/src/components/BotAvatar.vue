<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    avatarStyle?: string
    state?: string
    size?: 'sm' | 'md' | 'lg'
  }>(),
  {
    avatarStyle: 'cyan',
    state: 'idle',
    size: 'md',
  }
)

const palette = computed(() => {
  const map: Record<string, { ring: string; fill: string; eye: string }> = {
    cyan: { ring: '#f2b880', fill: 'rgba(242, 184, 128, 0.15)', eye: '#fbf3e8' },
    indigo: { ring: '#a5b4fc', fill: 'rgba(165, 180, 252, 0.16)', eye: '#eef2ff' },
    amber: { ring: '#fbbf24', fill: 'rgba(251, 191, 36, 0.16)', eye: '#fef3c7' },
    emerald: { ring: '#34d399', fill: 'rgba(52, 211, 153, 0.16)', eye: '#d1fae5' },
    violet: { ring: '#d8b4fe', fill: 'rgba(216, 180, 254, 0.16)', eye: '#faf5ff' },
  }
  return map[props.avatarStyle] || map.cyan
})

const ringColor = computed(() => {
  if (props.state === 'blocked') return '#ef4444'
  if (props.state === 'done') return '#34d399'
  if (props.state === 'thinking') return '#fbbf24'
  return palette.value.ring
})

const sizeClass = computed(() => {
  if (props.size === 'sm') return 'w-7 h-7'
  if (props.size === 'lg') return 'w-11 h-11'
  return 'w-9 h-9'
})
</script>

<template>
  <div :class="['shrink-0 inline-flex items-center justify-center', sizeClass, `bot-avatar-${state}`]">
    <svg viewBox="0 0 44 44" fill="none" class="w-full h-full block">
      <circle
        cx="22"
        cy="22"
        r="19"
        :fill="palette.fill"
        :stroke="ringColor"
        stroke-width="2"
        class="bot-ring"
      />
      <g class="bot-eyes">
        <circle cx="16.5" cy="21" r="2.6" :fill="palette.eye" />
        <circle cx="27.5" cy="21" r="2.6" :fill="palette.eye" />
        <path
          d="M16 28.5 Q22 31.5 28 28.5"
          :stroke="ringColor"
          stroke-width="1.8"
          stroke-linecap="round"
          opacity="0.85"
        />
      </g>
    </svg>
  </div>
</template>
