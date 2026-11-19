<script setup lang="ts">
import { useAppStore } from '../../stores/useAppStore'

export interface RelayStepItem {
  id: string
  name: string
  status: 'done' | 'active'
  guest?: boolean
}

defineProps<{
  steps: RelayStepItem[]
}>()

const store = useAppStore()
</script>

<template>
  <Transition
    enter-active-class="transition-[opacity,transform] duration-200 ease-[var(--ease-spring)] motion-reduce:duration-100"
    enter-from-class="opacity-0 translate-y-2 scale-96 motion-reduce:translate-y-0 motion-reduce:scale-100"
    enter-to-class="opacity-100 translate-y-0 scale-100"
    leave-active-class="transition-[opacity,transform] duration-150 ease-in motion-reduce:duration-75"
    leave-from-class="opacity-100 translate-y-0 scale-100"
    leave-to-class="opacity-0 translate-y-1.5 scale-96 motion-reduce:translate-y-0 motion-reduce:scale-100"
  >
    <div
      v-if="steps.length > 0"
      :class="[
        'origin-bottom w-full mx-auto mb-2 px-3 py-1.5 rounded-full border border-accent-border bg-card flex items-center gap-2 overflow-x-auto no-scrollbar text-xs',
        store.state.chatWide ? 'max-w-[1120px]' : 'max-w-[840px]',
      ]"
    >
      <span class="font-mono text-fg-muted shrink-0">协同接力:</span>
      <template v-for="(step, sidx) in steps" :key="step.id + sidx">
        <span v-if="sidx > 0" class="text-fg-muted shrink-0">-&gt;</span>
        <span
          :class="[
            'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full shrink-0 transition-colors duration-200',
            step.status === 'active'
              ? 'bg-accent-soft text-accent-fg font-medium'
              : 'bg-ok-soft text-ok',
          ]"
        >
          <span :class="['w-1.5 h-1.5 rounded-full', step.status === 'active' ? 'bg-accent animate-pulse' : 'bg-ok']" />
          <span>{{ step.name }}</span>
          <span v-if="step.guest" class="text-xs opacity-80">(外援)</span>
        </span>
      </template>
    </div>
  </Transition>
</template>
