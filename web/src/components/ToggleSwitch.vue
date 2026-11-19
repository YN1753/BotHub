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
    class="vesper-card w-full min-h-[44px] flex items-center justify-between gap-3 rounded-xl hover:border-line-strong px-3.5 py-2.5 text-left cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline focus-visible:outline-2 focus-visible:outline-accent"
  >
    <div class="min-w-0">
      <div class="text-xs font-medium text-fg">{{ label }}</div>
      <div v-if="hint || description" class="text-xs text-fg-muted mt-0.5 leading-snug">{{ hint || description }}</div>
    </div>

    <span
      aria-hidden="true"
      :class="[
        'relative inline-flex h-6 w-11 shrink-0 items-center rounded-full border transition-colors duration-150',
        modelValue
          ? 'bg-accent border-transparent'
          : 'bg-canvas border-line-strong'
      ]"
    >
      <span
        :class="[
          'inline-block h-4 w-4 rounded-full transition-transform duration-200 ease-[var(--ease-snappy)]',
          modelValue
            ? 'translate-x-[22px] bg-on-accent'
            : 'translate-x-[3px] bg-fg-muted'
        ]"
      />
    </span>
  </button>
</template>
