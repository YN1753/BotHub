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
    return 'border-rose-400/35 bg-rose-400/8 text-rose-200'
  }
  if (preview.value.tone === 'paid') {
    return 'border-[#f2b880]/35 bg-[#f2b880]/8 text-[#f7cba0]'
  }
  return 'border-emerald-400/28 bg-emerald-400/8 text-emerald-200'
})

const dotClasses = computed(() => {
  if (preview.value.tone === 'warn') return 'bg-rose-400'
  if (preview.value.tone === 'paid') return 'bg-[#f2b880]'
  return 'bg-emerald-400'
})
</script>

<template>
  <div :class="['rounded-xl border px-3 py-2 text-[11px] flex flex-col gap-1.5', containerClasses]">
    <div class="flex items-center gap-2 min-w-0">
      <span :class="['w-1.5 h-1.5 rounded-full shrink-0', dotClasses]" aria-hidden="true" />
      <span class="font-medium leading-snug">{{ preview.headline }}</span>
    </div>
    <div v-if="preview.chain.length > 0" class="flex items-center gap-1.5 flex-wrap pl-3.5">
      <template v-for="(step, idx) in preview.chain" :key="step + idx">
        <span
          class="font-mono text-[10.5px] px-2 py-0.5 rounded-md border border-[#fff8ee]/14 bg-[#070a14]/65 text-slate-200"
        >
          {{ step }}
        </span>
        <span v-if="idx < preview.chain.length - 1" class="text-[10px] opacity-60" aria-hidden="true">-></span>
      </template>
    </div>
    <p v-if="preview.note" class="text-[10.5px] opacity-85 pl-3.5 leading-relaxed">
      {{ preview.note }}
    </p>
  </div>
</template>
