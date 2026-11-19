<script setup lang="ts">
import { ref, computed, nextTick, watch, onMounted, onBeforeUnmount, useId } from 'vue'
import type { SelectOption } from '../types/bothub'

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: SelectOption[]
    placeholder?: string
    compact?: boolean
    disabled?: boolean
    ariaLabel?: string
  }>(),
  {
    placeholder: '请选择...',
    compact: false,
    disabled: false,
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const listboxId = useId()
const open = ref(false)
const openAbove = ref(false)
const highlightedIndex = ref(0)
const rootEl = ref<HTMLElement | null>(null)
const triggerEl = ref<HTMLButtonElement | null>(null)
const listboxEl = ref<HTMLElement | null>(null)

async function scrollHighlightedIntoView() {
  await nextTick()
  const box = listboxEl.value
  if (!box) return
  const target = box.querySelector<HTMLElement>(`[data-option-idx="${highlightedIndex.value}"]`)
  target?.scrollIntoView({ block: 'nearest' })
}

const currentLabel = computed(() => {
  const found = props.options.find((o) => o.value === props.modelValue)
  return found ? found.label : props.placeholder
})

function syncHighlightToValue() {
  const idx = props.options.findIndex((o) => o.value === props.modelValue)
  highlightedIndex.value = idx >= 0 ? idx : 0
}

function measureDirection() {
  const rect = triggerEl.value?.getBoundingClientRect()
  if (!rect) return
  const spaceBelow = window.innerHeight - rect.bottom
  const spaceAbove = rect.top
  openAbove.value = spaceBelow < 240 && spaceAbove > spaceBelow
}

async function toggleOpen() {
  if (props.disabled) return
  if (open.value) {
    open.value = false
    return
  }
  syncHighlightToValue()
  measureDirection()
  open.value = true
  await scrollHighlightedIntoView()
}

function selectOption(val: string) {
  emit('update:modelValue', val)
  open.value = false
  triggerEl.value?.focus()
}

function handleTriggerKeydown(e: KeyboardEvent) {
  if (props.disabled) return
  if (e.key === 'Tab' && open.value) {
    open.value = false
    return
  }
  if (e.key === 'Escape' && open.value) {
    e.preventDefault()
    e.stopPropagation()
    open.value = false
    return
  }
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    if (!open.value) {
      syncHighlightToValue()
      measureDirection()
      open.value = true
      void scrollHighlightedIntoView()
      return
    }
    const len = props.options.length
    if (!len) return
    highlightedIndex.value =
      e.key === 'ArrowDown'
        ? (highlightedIndex.value + 1) % len
        : (highlightedIndex.value - 1 + len) % len
    void scrollHighlightedIntoView()
    return
  }
  if (e.key === 'Home' && open.value) {
    e.preventDefault()
    highlightedIndex.value = 0
    void scrollHighlightedIntoView()
    return
  }
  if (e.key === 'End' && open.value) {
    e.preventDefault()
    highlightedIndex.value = Math.max(0, props.options.length - 1)
    void scrollHighlightedIntoView()
    return
  }
  if ((e.key === 'Enter' || e.key === ' ') && open.value) {
    e.preventDefault()
    const opt = props.options[highlightedIndex.value]
    if (opt) selectOption(opt.value)
    return
  }
  if (open.value && e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
    const ch = e.key.toLowerCase()
    const start = (highlightedIndex.value + 1) % Math.max(1, props.options.length)
    for (let step = 0; step < props.options.length; step++) {
      const idx = (start + step) % props.options.length
      if (props.options[idx].label.toLowerCase().includes(ch)) {
        e.preventDefault()
        highlightedIndex.value = idx
        void scrollHighlightedIntoView()
        return
      }
    }
  }
}

watch(open, (val) => {
  if (val) window.addEventListener('resize', measureDirection)
  else window.removeEventListener('resize', measureDirection)
})

function handleOutside(e: MouseEvent) {
  if (rootEl.value && !rootEl.value.contains(e.target as Node)) {
    open.value = false
  }
}

onMounted(() => document.addEventListener('click', handleOutside))
onBeforeUnmount(() => {
  document.removeEventListener('click', handleOutside)
  window.removeEventListener('resize', measureDirection)
})
</script>

<template>
  <div ref="rootEl" class="relative inline-block w-full">
    <button
      ref="triggerEl"
      type="button"
      role="combobox"
      :disabled="disabled"
      :aria-expanded="open"
      :aria-controls="listboxId"
      :aria-activedescendant="open && options.length ? `${listboxId}-opt-${highlightedIndex}` : undefined"
      aria-haspopup="listbox"
      :aria-label="ariaLabel || currentLabel"
      @click="toggleOpen"
      @keydown="handleTriggerKeydown"
      :class="[
        'btn-tactile w-full flex items-center justify-between gap-2 rounded-xl text-left cursor-pointer disabled:opacity-45 disabled:cursor-not-allowed',
        open ? 'border-accent-border text-fg' : 'text-fg',
        compact ? 'px-3 py-1.5 text-xs min-h-[44px] sm:min-h-[34px]' : 'px-3.5 py-2 text-sm min-h-[44px]'
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
        aria-hidden="true"
        :class="['shrink-0 text-fg-muted transition-transform duration-150', open ? 'rotate-180 text-accent-fg' : '']"
      >
        <polyline points="6 9 12 15 18 9" />
      </svg>
    </button>

    <Transition
      enter-active-class="transition-[opacity,transform] duration-180 ease-[var(--ease-spring)] motion-reduce:duration-100"
      :enter-from-class="openAbove ? 'opacity-0 scale-95 translate-y-1.5 motion-reduce:scale-100 motion-reduce:translate-y-0' : 'opacity-0 scale-95 -translate-y-1.5 motion-reduce:scale-100 motion-reduce:translate-y-0'"
      enter-to-class="opacity-100 scale-100 translate-y-0"
      leave-active-class="transition-[opacity,transform] duration-130 ease-[var(--ease-in-snappy)] motion-reduce:duration-75"
      leave-from-class="opacity-100 scale-100 translate-y-0"
      :leave-to-class="openAbove ? 'opacity-0 scale-95 translate-y-1 motion-reduce:scale-100 motion-reduce:translate-y-0' : 'opacity-0 scale-95 -translate-y-1 motion-reduce:scale-100 motion-reduce:translate-y-0'"
    >
      <div
        v-if="open"
        :id="listboxId"
        ref="listboxEl"
        role="listbox"
        :class="[
          'aero-glass absolute left-0 z-50 w-full min-w-[180px] max-h-60 overflow-y-auto rounded-xl border border-line p-1.5',
          openAbove ? 'bottom-full mb-1.5 origin-bottom-left' : 'top-full mt-1.5 origin-top-left'
        ]"
      >
        <div v-if="options.length === 0" class="px-3 py-2 text-xs text-fg-muted text-center">
          暂无可选项
        </div>
        <button
          v-for="(opt, idx) in options"
          :id="`${listboxId}-opt-${idx}`"
          :key="opt.value"
          :data-option-idx="idx"
          type="button"
          role="option"
          :aria-selected="opt.value === modelValue"
          @mouseenter="highlightedIndex = idx"
          @click="selectOption(opt.value)"
          :class="[
            'w-full text-left px-3 min-h-[44px] sm:min-h-[34px] rounded-lg text-xs sm:text-sm transition-colors cursor-pointer flex items-center justify-between gap-2',
            opt.value === modelValue
              ? 'bg-accent-soft text-accent-fg font-medium'
              : idx === highlightedIndex
                ? 'bg-subtle text-fg'
                : 'text-fg-sub hover:bg-hover hover:text-fg'
          ]"
        >
          <span class="truncate">{{ opt.label }}</span>
          <svg
            v-if="opt.value === modelValue"
            viewBox="0 0 24 24"
            width="14"
            height="14"
            fill="none"
            stroke="currentColor"
            stroke-width="2.4"
            aria-hidden="true"
            class="shrink-0 text-accent-fg"
          >
            <polyline points="20 6 9 17 4 12" />
          </svg>
        </button>
      </div>
    </Transition>
  </div>
</template>
