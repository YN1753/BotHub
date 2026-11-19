<script setup lang="ts">
import { ref } from 'vue'

withDefaults(
  defineProps<{
    modelValue: string
    id?: string
    name?: string
    placeholder?: string
    autocomplete?: string
    enterkeyhint?: 'enter' | 'done' | 'go' | 'next' | 'previous' | 'search' | 'send'
    minlength?: number
    required?: boolean
    disabled?: boolean
    size?: 'sm' | 'md'
    toggleLabel?: string
  }>(),
  {
    autocomplete: 'off',
    size: 'md',
    toggleLabel: '密钥',
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const visible = ref(false)
</script>

<template>
  <div class="relative flex-1 min-w-0">
    <input
      :id="id"
      :name="name"
      :value="modelValue"
      :type="visible ? 'text' : 'password'"
      :placeholder="placeholder"
      :autocomplete="autocomplete"
      :enterkeyhint="enterkeyhint"
      :minlength="minlength"
      :required="required"
      :disabled="disabled"
      autocorrect="off"
      autocapitalize="none"
      spellcheck="false"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      :class="[
        'w-full border border-line bg-canvas font-mono text-fg placeholder:text-fg-muted focus:border-accent',
        size === 'sm'
          ? 'min-h-[38px] rounded-xl pl-3 pr-12 py-1.5 text-xs'
          : 'min-h-[44px] sm:min-h-[40px] rounded-xl pl-3 pr-14 py-2 text-sm',
      ]"
    />
    <button
      type="button"
      :aria-pressed="visible"
      :aria-label="visible ? `隐藏${toggleLabel}` : `显示${toggleLabel}`"
      @click="visible = !visible"
      :class="[
        'absolute top-1/2 -translate-y-1/2 rounded font-mono text-fg-muted hover:text-accent-fg cursor-pointer',
        size === 'sm' ? 'right-2 px-1.5 py-0.5 text-xs' : 'right-2.5 px-2 py-0.5 text-xs',
      ]"
    >
      {{ visible ? '隐藏' : '显示' }}
    </button>
  </div>
</template>
