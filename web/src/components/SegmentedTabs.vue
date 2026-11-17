<script setup lang="ts">
import { ref, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'

export interface TabItem {
  id: string
  label: string
}

const props = withDefaults(
  defineProps<{
    modelValue: string
    items: TabItem[]
    ariaLabel?: string
    variant?: 'pill' | 'subtle'
  }>(),
  {
    variant: 'pill',
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const trackRef = ref<HTMLElement | null>(null)
const pillStyle = ref<{ transform: string; width: string; opacity: number }>({
  transform: 'translate3d(0px, 0, 0)',
  width: '0px',
  opacity: 0,
})

let resizeObs: ResizeObserver | null = null

function updateIndicator() {
  const track = trackRef.value
  if (!track) return
  const activeBtn = track.querySelector<HTMLButtonElement>('[data-active="true"]')
  if (!activeBtn) {
    pillStyle.value = { transform: 'translate3d(0px, 0, 0)', width: '0px', opacity: 0 }
    return
  }
  pillStyle.value = {
    transform: `translate3d(${activeBtn.offsetLeft}px, 0, 0)`,
    width: `${activeBtn.offsetWidth}px`,
    opacity: 1,
  }
}

watch(
  () => [props.modelValue, props.items.length],
  () => {
    nextTick(() => {
      updateIndicator()
      const track = trackRef.value
      const activeBtn = track?.querySelector<HTMLButtonElement>('[data-active="true"]')
      activeBtn?.scrollIntoView({ behavior: 'smooth', inline: 'nearest', block: 'nearest' })
    })
  }
)

function handleTabKeydown(e: KeyboardEvent, idx: number) {
  if (!props.items.length) return
  let nextIdx = -1
  if (e.key === 'ArrowRight') nextIdx = (idx + 1) % props.items.length
  else if (e.key === 'ArrowLeft') nextIdx = (idx - 1 + props.items.length) % props.items.length
  else if (e.key === 'Home') nextIdx = 0
  else if (e.key === 'End') nextIdx = props.items.length - 1
  if (nextIdx < 0) return

  e.preventDefault()
  const target = props.items[nextIdx]
  if (!target) return
  emit('update:modelValue', target.id)
  nextTick(() => {
    const btns = trackRef.value?.querySelectorAll<HTMLButtonElement>('[role="tab"]')
    btns?.[nextIdx]?.focus()
  })
}

onMounted(() => {
  nextTick(updateIndicator)
  if (typeof ResizeObserver !== 'undefined' && trackRef.value) {
    resizeObs = new ResizeObserver(updateIndicator)
    resizeObs.observe(trackRef.value)
  }
})

onBeforeUnmount(() => {
  resizeObs?.disconnect()
})
</script>

<template>
  <div
    ref="trackRef"
    role="tablist"
    :aria-label="ariaLabel || '视图切换'"
    :class="[
      'relative inline-flex items-center p-1 rounded-full overflow-x-auto no-scrollbar max-w-full select-none',
      variant === 'pill'
        ? 'bg-[#060913]/92 border border-[#fff8ee]/10'
        : 'bg-[#060913]/65 border border-[#fff8ee]/8'
    ]"
  >
    <!-- Gliding Spring Indicator -->
    <span
      aria-hidden="true"
      :style="{
        width: pillStyle.width,
        opacity: pillStyle.opacity,
        transform: pillStyle.transform,
        transitionTimingFunction: 'var(--ease-spring)',
      }"
      class="pointer-events-none absolute top-1 bottom-1 left-0 rounded-full bg-[#f2b880] shadow-[0_2px_12px_rgba(242,184,128,0.28)] transition-[transform,width,opacity] duration-300 motion-reduce:transition-none"
    />

    <button
      v-for="(item, idx) in items"
      :key="item.id"
      type="button"
      role="tab"
      :tabindex="modelValue === item.id ? 0 : -1"
      :data-active="modelValue === item.id"
      :aria-selected="modelValue === item.id"
      @keydown="handleTabKeydown($event, idx)"
      @click="emit('update:modelValue', item.id)"
      :class="[
        'relative z-10 min-h-[38px] sm:min-h-[30px] px-3.5 rounded-full text-xs whitespace-nowrap shrink-0 cursor-pointer transition-colors duration-200',
        modelValue === item.id
          ? 'text-[#070a13] font-semibold'
          : 'text-slate-300 hover:text-white'
      ]"
    >
      {{ item.label }}
    </button>
  </div>
</template>
