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
const subtitleId = useId()
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
    :aria-describedby="subtitle ? subtitleId : undefined"
    :class="['vesper-dialog', sizeClassMap[maxWidth]]"
    @cancel="handleNativeCancel"
    @close="handleNativeClose"
    @pointerdown="handlePointerDown"
    @pointerup="handlePointerUp"
  >
    <div class="relative flex flex-col min-h-0 max-h-[90dvh] sm:max-h-[86dvh]">
      <div class="sm:hidden w-10 h-1 rounded-full bg-line-strong mx-auto mt-2.5 shrink-0" aria-hidden="true" />

      <div
        class="flex items-start justify-between gap-3 px-4 sm:px-5 pt-4 pb-3.5 bg-card border-b border-line shrink-0"
      >
        <div class="min-w-0">
          <h3 :id="titleId" class="text-base sm:text-lg font-semibold text-fg tracking-tight">
            {{ title }}
          </h3>
          <p v-if="subtitle" :id="subtitleId" class="text-xs text-fg-muted mt-0.5">
            {{ subtitle }}
          </p>
        </div>

        <div class="flex items-center gap-2 shrink-0 flex-wrap">
          <slot name="header-actions" />
          <kbd aria-hidden="true" class="well-recessed hidden sm:inline-flex items-center px-1.5 py-0.5 rounded-lg text-xs font-mono text-fg-muted">Esc</kbd>
          <button
            type="button"
            aria-label="关闭窗口"
            title="关闭 (Esc)"
            @click="requestAnimatedClose"
            class="btn-tactile min-h-[44px] min-w-[44px] sm:min-h-[34px] sm:min-w-[34px] inline-flex items-center justify-center rounded-lg text-fg-sub hover:text-fg focus-visible:outline focus-visible:outline-2 focus-visible:outline-accent cursor-pointer"
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
      </div>

      <div class="flex-1 min-h-0 flex flex-col p-4 sm:p-5 pb-[max(1.1rem,env(safe-area-inset-bottom,1.1rem))] gap-4 overflow-hidden overscroll-contain">
        <slot />
      </div>
    </div>
  </dialog>
</template>
