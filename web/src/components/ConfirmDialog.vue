<script setup lang="ts">
import { ref, watch, nextTick, useId } from 'vue'
import { useAppStore } from '../stores/useAppStore'

const store = useAppStore()
const dialogRef = ref<HTMLDialogElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)
const cancelBtnRef = ref<HTMLButtonElement | null>(null)
const promptValue = ref('')
const titleId = useId()
const descId = useId()
const supportsClosedBy =
  typeof HTMLDialogElement !== 'undefined' && 'closedBy' in HTMLDialogElement.prototype

let pointerStartedOnBackdrop = false

watch(
  () => store.state.dialogRequest,
  async (req) => {
    const el = dialogRef.value
    if (!req) {
      if (el?.open) el.close('dismiss')
      return
    }
    promptValue.value = req.defaultValue || ''
    await nextTick()
    if (!el) return
    if (!el.open) {
      el.returnValue = ''
      el.showModal()
    }
    await nextTick()
    if (req.kind === 'prompt') {
      inputRef.value?.focus()
      inputRef.value?.select()
    } else if (req.danger) {
      cancelBtnRef.value?.focus()
    }
  },
  { immediate: true }
)

function handleDialogClose() {
  const el = dialogRef.value
  const req = store.state.dialogRequest
  const rv = el?.returnValue || ''
  if (el) el.returnValue = ''
  if (!req) return
  if (rv === 'confirm') {
    store.resolveDialog(req.kind === 'prompt' ? promptValue.value : true)
  } else {
    store.resolveDialog(req.kind === 'prompt' ? null : false)
  }
}

function handleBackdropPointerDown(e: PointerEvent) {
  pointerStartedOnBackdrop = e.target === dialogRef.value
}

function handleBackdropPointerUp(e: PointerEvent) {
  if (supportsClosedBy) return
  const req = store.state.dialogRequest
  if (pointerStartedOnBackdrop && e.target === dialogRef.value && req?.kind !== 'prompt') {
    dialogRef.value?.close('cancel')
  }
  pointerStartedOnBackdrop = false
}

function submitCancel() {
  dialogRef.value?.close('cancel')
}
</script>

<template>
  <dialog
    ref="dialogRef"
    class="vesper-dialog vesper-dialog-sm"
    :closedby="store.state.dialogRequest?.kind === 'prompt' ? 'closerequest' : 'any'"
    :aria-labelledby="titleId"
    :aria-describedby="descId"
    @close="handleDialogClose"
    @pointerdown="handleBackdropPointerDown"
    @pointerup="handleBackdropPointerUp"
  >
    <div
      v-if="store.state.dialogRequest"
      class="flex flex-col"
    >
      <div
        class="flex items-start justify-between gap-3 px-4 sm:px-5 pt-4 pb-3 border-b border-[#fff8ee]/10"
        style="background: radial-gradient(85% 140% at 12% 0%, rgba(var(--accent-rgb), 0.1) 0%, transparent 62%), var(--surface-dialog-header)"
      >
        <div class="min-w-0">
          <h3 :id="titleId" class="text-base font-semibold text-white tracking-tight">
            {{ store.state.dialogRequest.title }}
          </h3>
        </div>
        <button
          type="button"
          @click="submitCancel"
          aria-label="关闭"
          class="min-w-[34px] min-h-[34px] rounded-xl flex items-center justify-center text-slate-400 hover:text-white hover:bg-slate-500/10 transition cursor-pointer shrink-0 -mr-1 -mt-0.5"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>

      <form method="dialog" class="p-4 sm:p-5 flex flex-col gap-4">
        <p :id="descId" class="text-xs sm:text-sm text-slate-300 leading-relaxed whitespace-pre-line">
          {{ store.state.dialogRequest.message }}
        </p>

        <input
          v-if="store.state.dialogRequest.kind === 'prompt'"
          ref="inputRef"
          v-model="promptValue"
          type="text"
          :placeholder="store.state.dialogRequest.placeholder || '请输入内容...'"
          class="w-full px-3.5 py-2.5 rounded-xl border border-[#fff8ee]/15 text-sm text-white placeholder:text-slate-500 focus:outline-none focus:border-[#f2b880]/55"
        />

        <div class="flex items-center justify-end gap-2.5 pt-1">
          <button
            ref="cancelBtnRef"
            type="button"
            :autofocus="Boolean(store.state.dialogRequest.danger)"
            @click="submitCancel"
            class="btn-tactile px-4 min-h-[40px] sm:min-h-[36px] rounded-xl text-xs sm:text-sm font-medium cursor-pointer"
          >
            {{ store.state.dialogRequest.cancelText || '取消' }}
          </button>
          <button
            type="submit"
            value="confirm"
            :class="[
              'px-4 min-h-[40px] sm:min-h-[36px] rounded-xl text-xs sm:text-sm font-semibold cursor-pointer transition',
              store.state.dialogRequest.danger
                ? 'bg-rose-500 hover:bg-rose-400 text-white border border-rose-300/40 shadow-lg shadow-rose-950/35'
                : 'btn-jewel'
            ]"
          >
            {{ store.state.dialogRequest.confirmText || '确认' }}
          </button>
        </div>
      </form>
    </div>
  </dialog>
</template>
