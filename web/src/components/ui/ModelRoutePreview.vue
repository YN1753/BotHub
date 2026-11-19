<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { buildRoutePreview } from '../../utils/modelRouting'

const props = withDefaults(
  defineProps<{
    selectedModel?: string
    allowPaidFallback?: boolean
    maxPaidCallsPerDay?: number
    isTask?: boolean
    needTools?: boolean
  }>(),
  {
    selectedModel: '',
    allowPaidFallback: false,
    maxPaidCallsPerDay: 0,
    isTask: false,
    needTools: false,
  }
)

const store = useAppStore()

const preview = computed(() =>
  buildRoutePreview({
    selectedModel: props.selectedModel || '',
    allowPaidFallback: Boolean(props.allowPaidFallback),
    maxPaidCallsPerDay: Number(props.maxPaidCallsPerDay) || 0,
    providers: store.state.modelProviders || [],
    pooledModels: store.state.pooledModels || [],
    strategies: store.state.poolStrategies,
    isTask: props.isTask,
    needTools: props.needTools,
  })
)

const containerClasses = computed(() => {
  if (preview.value.tone === 'warn') {
    return 'border-danger-border bg-danger-soft text-danger'
  }
  if (preview.value.tone === 'paid') {
    return 'border-accent-border bg-accent-soft text-accent-fg'
  }
  return 'border-ok-border bg-ok-soft text-ok'
})

const dotClasses = computed(() => {
  if (preview.value.tone === 'warn') return 'bg-danger'
  if (preview.value.tone === 'paid') return 'bg-accent'
  return 'bg-ok'
})
</script>

<template>
  <div :class="['rounded-xl border px-3 py-2 text-xs flex flex-col gap-1.5', containerClasses]">
    <div class="flex items-center gap-2 min-w-0">
      <span :class="['w-1.5 h-1.5 rounded-full shrink-0', dotClasses]" aria-hidden="true" />
      <span class="font-medium leading-snug">{{ preview.headline }}</span>
    </div>
    <div v-if="preview.chain.length > 0" class="flex items-center gap-1.5 flex-wrap pl-3.5">
      <template v-for="(step, idx) in preview.chain" :key="step + idx">
        <span
          class="font-mono text-xs px-2 py-0.5 rounded-lg border border-line bg-canvas text-fg"
        >
          {{ step }}
        </span>
        <span v-if="idx < preview.chain.length - 1" class="text-xs opacity-60" aria-hidden="true">-></span>
      </template>
    </div>
    <p v-if="preview.note" class="text-xs opacity-85 pl-3.5 leading-relaxed">
      {{ preview.note }}
    </p>
  </div>
</template>
