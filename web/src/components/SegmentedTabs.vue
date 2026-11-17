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
const indicatorAnimated = ref(false)
const pillStyle = ref<{ transform: string; width: string; opacity: number }>({
  transform: 'translate3d(0px, 0, 0)',
  width: '0px',
  opacity: 0,
})

let resizeObs: ResizeObserver | null = null
let intersectObs: IntersectionObserver | null = null
let enableAnimRaf = 0

function updateIndicator(allowAnimate = true) {
  const track = trackRef.value
  if (!track) return
  const activeBtn = track.querySelector<HTMLButtonElement>('[data-active="true"]')
  if (!activeBtn || activeBtn.offsetWidth === 0) {
    indicatorAnimated.value = false
    pillStyle.value = { transform: 'translate3d(0px, 0, 0)', width: '0px', opacity: 0 }
    return
  }

  const wasReady = indicatorAnimated.value
  if (!allowAnimate || !wasReady) {
    indicatorAnimated.value = false
  }

  pillStyle.value = {
    transform: `translate3d(${activeBtn.offsetLeft}px, 0, 0)`,
    width: `${activeBtn.offsetWidth}px`,
    opacity: 1,
  }

  if (!wasReady || !allowAnimate) {
    window.cancelAnimationFrame(enableAnimRaf)
    enableAnimRaf = window.requestAnimationFrame(() => {
      indicatorAnimated.value = true
    })
  }
}

watch(
  () => [props.modelValue, props.items.length],
  () => {
    nextTick(() => {
      updateIndicator(true)
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
  nextTick(() => updateIndicator(false))
  if (typeof ResizeObserver !== 'undefined' && trackRef.value) {
    resizeObs = new ResizeObserver(() => updateIndicator(false))
    resizeObs.observe(trackRef.value)
  }
  if (typeof IntersectionObserver !== 'undefined' && trackRef.value) {
    intersectObs = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting)) {
        updateIndicator(false)
      }
    })
    intersectObs.observe(trackRef.value)
  }
})

onBeforeUnmount(() => {
  window.cancelAnimationFrame(enableAnimRaf)
  resizeObs?.disconnect()
  intersectObs?.disconnect()
})
</script>

<template>
  <div
    ref="trackRef"
    role="tablist"
    :aria-label="ariaLabel || '视图切换'"
    :class="[
      'chip-rail relative inline-flex items-center p-1 rounded-full overflow-x-auto no-scrollbar max-w-full select-none [contain:layout_style]',
      variant === 'pill'
        ? 'bg-[#060913]/92 border border-[#fff8ee]/10'
        : 'bg-[#060913]/65 border border-[#fff8ee]/8'
    ]"
  >
    <!-- Gliding Spring Indicator (snaps instantly on dialog mount, glides with spring on user switch) -->
    <span
      aria-hidden="true"
      :style="{
        width: pillStyle.width,
        opacity: pillStyle.opacity,
        transform: pillStyle.transform,
        transitionTimingFunction: indicatorAnimated ? 'var(--ease-spring)' : 'none',
      }"
      :class="[
        'pointer-events-none absolute top-1 bottom-1 left-0 rounded-full bg-[#f2b880] shadow-[0_2px_12px_rgba(242,184,128,0.28)] motion-reduce:transition-none',
        indicatorAnimated ? 'transition-[transform,width,opacity] duration-300' : 'transition-none'
      ]"
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
        'snap-chip relative z-10 min-h-[38px] sm:min-h-[30px] px-3.5 rounded-full text-xs whitespace-nowrap shrink-0 cursor-pointer transition-colors duration-200',
        modelValue === item.id
          ? 'text-[#070a13] font-semibold'
          : 'text-slate-300 hover:text-white'
      ]"
    >
      {{ item.label }}
    </button>
  </div>
</template>
