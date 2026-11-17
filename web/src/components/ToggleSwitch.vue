<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    modelValue: boolean
    label: string
    hint?: string
    description?: string
    disabled?: boolean
  }>(),
  {
    disabled: false,
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

function toggle() {
  if (props.disabled) return
  emit('update:modelValue', !props.modelValue)
}
</script>

<template>
  <button
    type="button"
    role="switch"
    :disabled="props.disabled"
    :aria-checked="props.modelValue"
    @click="toggle"
    class="w-full min-h-[44px] flex items-center justify-between gap-3 rounded-xl border border-[#fff8ee]/10 bg-[#070a16]/70 hover:border-[#f2b880]/35 px-3.5 py-2.5 text-left cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#f2b880]"
  >
    <div class="min-w-0">
      <div class="text-xs font-medium text-slate-100">{{ label }}</div>
      <div v-if="hint || description" class="text-[11px] text-slate-400 mt-0.5 leading-snug">{{ hint || description }}</div>
    </div>

    <span
      aria-hidden="true"
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
