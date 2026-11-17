<script setup lang="ts">
export interface RelayStepItem {
  id: string
  name: string
  status: 'done' | 'active'
  guest?: boolean
}

defineProps<{
  steps: RelayStepItem[]
}>()
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
      class="origin-bottom max-w-[820px] w-full mx-auto mb-2 px-3 py-1.5 rounded-full border border-[#f2b880]/25 bg-[#080d1c]/90 backdrop-blur-md flex items-center gap-2 overflow-x-auto no-scrollbar text-[11px]"
    >
      <span class="font-mono text-slate-400 shrink-0">协同接力:</span>
      <template v-for="(step, sidx) in steps" :key="step.id + sidx">
        <span v-if="sidx > 0" class="text-slate-600 shrink-0">-&gt;</span>
        <span
          :class="[
            'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full shrink-0 transition-colors duration-200',
            step.status === 'active'
              ? 'bg-[#f2b880]/15 text-[#f2b880] font-medium'
              : 'bg-emerald-400/10 text-emerald-300',
          ]"
        >
          <span :class="['w-1.5 h-1.5 rounded-full', step.status === 'active' ? 'bg-[#f2b880] animate-pulse' : 'bg-emerald-400']" />
          <span>{{ step.name }}</span>
          <span v-if="step.guest" class="text-[9.5px] opacity-80">(外援)</span>
        </span>
      </template>
    </div>
  </Transition>
</template>
