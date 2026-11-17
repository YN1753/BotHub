<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
  label: string
  hint?: string
  description?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()
</script>

<template>
  <button
    type="button"
    role="switch"
    :aria-checked="props.modelValue"
    @click="emit('update:modelValue', !props.modelValue)"
    class="w-full min-h-[44px] flex items-center justify-between gap-3 rounded-xl border border-[#fff8ee]/10 bg-[#070a16]/70 hover:border-[#f2b880]/35 px-3.5 py-2.5 text-left cursor-pointer"
  >
    <div class="min-w-0">
      <div class="text-xs font-medium text-slate-100">{{ label }}</div>
      <div v-if="hint || description" class="text-[11px] text-slate-400 mt-0.5 leading-snug">{{ hint || description }}</div>
    </div>

    <span
      :class="[
        'relative inline-flex h-6 w-11 shrink-0 items-center rounded-full border transition-colors duration-200',
        modelValue
          ? 'bg-[#f2b880] border-[#f2b880]'
          : 'bg-[#0b1020] border-[#fff8ee]/16'
      ]"
    >
      <span
        :class="[
          'inline-block h-4 w-4 rounded-full shadow-md transition-transform duration-250 ease-[var(--ease-spring)]',
          modelValue
            ? 'translate-x-[22px] bg-[#070a13]'
            : 'translate-x-[3px] bg-slate-400'
        ]"
      />
    </span>
  </button>
</template>
