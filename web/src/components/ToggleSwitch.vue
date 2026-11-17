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
    class="vesper-card w-full min-h-[44px] flex items-center justify-between gap-3 rounded-xl hover:border-[#f2b880]/40 px-3.5 py-2.5 text-left cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#f2b880]"
  >
    <div class="min-w-0">
      <div class="text-xs font-medium text-slate-100">{{ label }}</div>
      <div v-if="hint || description" class="text-[11px] text-slate-400 mt-0.5 leading-snug">{{ hint || description }}</div>
    </div>

    <span
      aria-hidden="true"
      :class="[
        'relative inline-flex h-6 w-11 shrink-0 items-center rounded-full border transition-all duration-200',
        modelValue
          ? 'bg-gradient-to-b from-[#ffe2be] to-[#d99252] border-[#ffe7c7]/70 shadow-[0_0_14px_rgba(242,184,128,0.38),inset_0_1px_0_rgba(255,255,255,0.65)]'
          : 'well-recessed'
      ]"
    >
      <span
        :class="[
          'inline-block h-4 w-4 rounded-full transition-transform duration-250 ease-[var(--ease-spring)]',
          modelValue
            ? 'translate-x-[22px] bg-[#080b16] shadow-[0_2px_6px_rgba(0,0,0,0.55),inset_0_1px_0_rgba(255,255,255,0.2)]'
            : 'translate-x-[3px] bg-gradient-to-b from-slate-200 to-slate-400 shadow-[0_2px_5px_rgba(0,0,0,0.6)]'
        ]"
      />
    </span>
  </button>
</template>
