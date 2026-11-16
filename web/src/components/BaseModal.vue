<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount, useId } from 'vue'
import { closeDialogWithTransition, isPointOutsideRect } from '../utils/topLayer'

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    subtitle?: string
    maxWidth?: 'sm' | 'lg' | '2xl' | '3xl'
    closableByBackdrop?: boolean
  }>(),
  {
    maxWidth: '2xl',
    closableByBackdrop: true,
  }
)

const emit = defineEmits<{
  close: []
}>()

const titleId = useId()
const dialogRef = ref<HTMLDialogElement | null>(null)
let pressedOutside = false

const sizeClassMap: Record<string, string> = {
  sm: 'vesper-dialog-sm',
  lg: '',
  '2xl': 'vesper-dialog-2xl',
  '3xl': 'vesper-dialog-3xl',
}

async function requestAnimatedClose() {
  const el = dialogRef.value
  if (!el || !el.open) {
    emit('close')
    return
  }
  await closeDialogWithTransition(el)
  emit('close')
}

function handleNativeCancel(e: Event) {
  if (e.cancelable) {
    e.preventDefault()
    requestAnimatedClose()
  }
}

function handleNativeClose() {
  const el = dialogRef.value
  if (el) {
    el.removeAttribute('data-closing')
    el.returnValue = ''
  }
  if (props.open) {
    emit('close')
  }
}

function handlePointerDown(e: PointerEvent) {
  if (!props.closableByBackdrop || 'closedBy' in HTMLDialogElement.prototype) return
  const el = dialogRef.value
  if (!el) return
  pressedOutside = isPointOutsideRect(e.clientX, e.clientY, el.getBoundingClientRect())
}

function handlePointerUp(e: PointerEvent) {
  if (!props.closableByBackdrop || 'closedBy' in HTMLDialogElement.prototype) return
  const el = dialogRef.value
  if (!el) return
  if (pressedOutside && isPointOutsideRect(e.clientX, e.clientY, el.getBoundingClientRect())) {
    pressedOutside = false
    requestAnimatedClose()
  } else {
    pressedOutside = false
  }
}

watch(
  () => props.open,
  async (val) => {
    const el = dialogRef.value
    if (!el) return
    if (val) {
      el.removeAttribute('data-closing')
      if (!el.open) {
        el.showModal()
      }
    } else if (el.open) {
      await closeDialogWithTransition(el)
    }
  },
  { flush: 'post' }
)

onMounted(() => {
  const el = dialogRef.value
  if (el && props.open && !el.open) {
    el.showModal()
  }
})

onBeforeUnmount(() => {
  const el = dialogRef.value
  if (el && el.open) {
    el.close()
  }
})
</script>

<template>
  <dialog
    ref="dialogRef"
    :closedby="closableByBackdrop ? 'any' : 'closerequest'"
    :aria-labelledby="titleId"
    :class="['vesper-dialog', sizeClassMap[maxWidth]]"
    @cancel="handleNativeCancel"
    @close="handleNativeClose"
    @pointerdown="handlePointerDown"
    @pointerup="handlePointerUp"
  >
    <div class="flex flex-col min-h-0 max-h-[90dvh] sm:max-h-[86dvh] p-4 sm:p-5 pb-[max(1rem,env(safe-area-inset-bottom,1rem))] gap-4">
      <div class="sm:hidden w-10 h-1 rounded-full bg-[#fff8ee]/20 mx-auto -mt-1 shrink-0" aria-hidden="true" />

      <div class="flex items-start justify-between gap-3 shrink-0">
        <div class="min-w-0">
          <h3 :id="titleId" class="text-base sm:text-lg font-semibold text-white">
            {{ title }}
          </h3>
          <p v-if="subtitle" class="text-xs text-slate-400 mt-0.5">
            {{ subtitle }}
          </p>
        </div>

        <div class="flex items-center gap-2 shrink-0 flex-wrap">
          <slot name="header-actions" />
          <button
            type="button"
            aria-label="关闭窗口"
            @click="requestAnimatedClose"
            class="min-h-[44px] min-w-[44px] sm:min-h-[34px] sm:min-w-[34px] inline-flex items-center justify-center rounded-xl border border-[#fff8ee]/14 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 hover:bg-white/5 cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
      </div>

      <slot />
    </div>
  </dialog>
</template>
