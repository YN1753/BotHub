<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import type { SelectOption } from '../types/bothub'

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: SelectOption[]
    placeholder?: string
    compact?: boolean
  }>(),
  {
    placeholder: '请选择...',
    compact: false,
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const open = ref(false)
const rootEl = ref<HTMLElement | null>(null)

const currentLabel = computed(() => {
  const found = props.options.find((o) => o.value === props.modelValue)
  return found ? found.label : props.placeholder
})

function selectOption(val: string) {
  emit('update:modelValue', val)
  open.value = false
}

function handleOutside(e: MouseEvent) {
  if (rootEl.value && !rootEl.value.contains(e.target as Node)) {
    open.value = false
  }
}

onMounted(() => document.addEventListener('click', handleOutside))
onBeforeUnmount(() => document.removeEventListener('click', handleOutside))
</script>

<template>
  <div ref="rootEl" class="relative inline-block w-full">
    <button
      type="button"
      @click="open = !open"
      :class="[
        'w-full flex items-center justify-between gap-2 rounded-xl border transition-colors text-left cursor-pointer',
        'bg-[#0a1020]/80 border-slate-400/20 hover:border-sky-400/45 text-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400/60',
        compact ? 'px-3 py-1.5 text-xs min-h-[34px]' : 'px-3.5 py-2 text-sm min-h-[40px]'
      ]"
    >
      <span class="truncate">{{ currentLabel }}</span>
      <svg
        viewBox="0 0 24 24"
        width="14"
        height="14"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        :class="['shrink-0 text-slate-400 transition-transform duration-150', open ? 'rotate-180' : '']"
      >
        <polyline points="6 9 12 15 18 9"></polyline>
      </svg>
    </button>

    <div
      v-if="open"
      class="absolute z-50 mt-1.5 w-full min-w-[170px] max-h-60 overflow-y-auto rounded-xl border border-slate-400/25 bg-[#0a1020]/95 backdrop-blur-xl shadow-2xl p-1"
    >
      <button
        v-for="opt in options"
        :key="opt.value"
        type="button"
        @click="selectOption(opt.value)"
        :class="[
          'w-full text-left px-3 py-2 rounded-lg text-xs sm:text-sm transition-colors cursor-pointer',
          opt.value === modelValue
            ? 'bg-sky-500/20 text-sky-300 font-medium'
            : 'text-slate-300 hover:bg-white/5 hover:text-white'
        ]"
      >
        {{ opt.label }}
      </button>
    </div>
  </div>
</template>
