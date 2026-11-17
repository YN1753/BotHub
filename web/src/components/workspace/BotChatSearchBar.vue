<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'

const props = defineProps<{
  open: boolean
  query: string
  matchCount: number
  cursor: number
}>()

const emit = defineEmits<{
  'update:query': [val: string]
  jump: [delta: number]
  close: []
}>()

const inputRef = ref<HTMLInputElement | null>(null)

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      nextTick(() => {
        inputRef.value?.focus()
        inputRef.value?.select()
      })
    }
  }
)
</script>

<template>
  <Transition
    enter-active-class="transition-[opacity,transform] duration-180 ease-[var(--ease-snappy)] motion-reduce:duration-100"
    enter-from-class="opacity-0 -translate-y-2 motion-reduce:translate-y-0"
    enter-to-class="opacity-100 translate-y-0"
    leave-active-class="transition-[opacity,transform] duration-130 ease-[var(--ease-in-snappy)] motion-reduce:duration-75"
    leave-from-class="opacity-100 translate-y-0"
    leave-to-class="opacity-0 -translate-y-1.5 motion-reduce:translate-y-0"
  >
    <div
      v-if="open"
      class="px-4 md:px-6 py-2 border-b border-[#fff8ee]/10 bg-[#070b17]/95 backdrop-blur-lg flex items-center gap-2 shrink-0"
    >
      <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="text-[#f2b880] shrink-0">
        <circle cx="11" cy="11" r="8" />
        <line x1="21" y1="21" x2="16.65" y2="16.65" />
      </svg>
      <input
        ref="inputRef"
        :value="query"
        type="search"
        aria-label="在当前会话中搜索聊天记录"
        placeholder="在当前会话中搜索聊天记录..."
        @input="emit('update:query', ($event.target as HTMLInputElement).value)"
        @keydown.enter.exact.prevent="emit('jump', 1)"
        @keydown.shift.enter.prevent="emit('jump', -1)"
        @keydown.escape="emit('close')"
        class="flex-1 min-w-0 bg-transparent text-xs text-white placeholder-slate-500 py-1 focus:outline-none"
      />
      <span v-if="query.trim()" class="font-mono text-[11px] text-slate-400 shrink-0">
        {{ matchCount ? `${cursor + 1}/${matchCount}` : '0/0' }}
      </span>
      <button
        type="button"
        :disabled="!matchCount"
        @click="emit('jump', -1)"
        aria-label="上一条匹配"
        class="w-7 h-7 rounded-lg border border-[#fff8ee]/10 text-slate-300 hover:text-[#f2b880] disabled:opacity-30 inline-flex items-center justify-center cursor-pointer"
      >
        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.2"><polyline points="18 15 12 9 6 15" /></svg>
      </button>
      <button
        type="button"
        :disabled="!matchCount"
        @click="emit('jump', 1)"
        aria-label="下一条匹配"
        class="w-7 h-7 rounded-lg border border-[#fff8ee]/10 text-slate-300 hover:text-[#f2b880] disabled:opacity-30 inline-flex items-center justify-center cursor-pointer"
      >
        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.2"><polyline points="6 9 12 15 18 9" /></svg>
      </button>
      <button
        type="button"
        @click="emit('close')"
        aria-label="关闭查找"
        class="w-7 h-7 rounded-lg text-slate-400 hover:text-white inline-flex items-center justify-center cursor-pointer"
      >
        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.2"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
      </button>
    </div>
  </Transition>
</template>
