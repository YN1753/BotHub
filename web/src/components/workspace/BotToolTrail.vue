<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore, type ChatMessageUI } from '../../stores/useAppStore'
import { formatDurationMs } from '../../utils/format'
import { formatToolHumanName, summarizeToolStep } from '../../utils/toolMeta'

const props = defineProps<{
  tools: ChatMessageUI['tools']
}>()

const store = useAppStore()

const totalDurationMs = computed(() =>
  props.tools.reduce((acc, tc) => acc + (typeof tc.durationMs === 'number' ? tc.durationMs : 0), 0)
)

const isPending = computed(() => props.tools.some((t) => !t.done))
const errorCount = computed(() => props.tools.filter((t) => t.status === 'error').length)

const trailSummary = computed(() => {
  const uniq: string[] = []
  for (const tc of props.tools) {
    const label = formatToolHumanName(tc.name, store.state.skills || [])
    if (!uniq.includes(label)) uniq.push(label)
    if (uniq.length >= 3) break
  }
  return uniq.join(' · ') + (props.tools.length > 3 ? ' 等' : '')
})

const summarizedSteps = computed(() =>
  props.tools.map((tc) => ({
    id: tc.id,
    durationMs: tc.durationMs,
    ...summarizeToolStep(tc, store.state.skills || []),
  }))
)
</script>

<template>
  <details
    v-if="tools.length"
    :class="[
      'vesper-details well-recessed w-fit max-w-full rounded-full px-3 py-1 text-xs [&[open]]:basis-full [&[open]]:w-full [&[open]]:rounded-2xl [&[open]]:px-3.5 [&[open]]:py-2 transition-[border-radius] duration-150',
      errorCount > 0 ? 'border-rose-400/35' : '',
    ]"
  >
    <summary class="cursor-pointer flex items-center gap-1.5 text-slate-300 select-none min-w-0">
      <span
        :class="[
          'w-1.5 h-1.5 rounded-full shrink-0',
          isPending
            ? 'bg-[#f2b880] animate-pulse'
            : errorCount > 0
              ? 'bg-rose-400'
              : 'bg-emerald-400',
        ]"
        aria-hidden="true"
      />
      <svg viewBox="0 0 24 24" width="12.5" height="12.5" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="details-chevron text-[#f2b880] shrink-0">
        <polyline points="9 18 15 12 9 6" />
      </svg>
      <span class="truncate">
        {{ isPending ? '正在协同' : '查阅步骤' }} · {{ trailSummary }}
      </span>
      <span class="font-mono text-[10.5px] text-slate-400 shrink-0">
        {{ tools.length }} 步<template v-if="totalDurationMs > 0"> · {{ formatDurationMs(totalDurationMs) }}</template>
      </span>
      <span
        :class="[
          'px-1.5 py-0.5 rounded-full text-[10px] border leading-none shrink-0',
          isPending
            ? 'border-[#f2b880]/40 bg-[#f2b880]/15 text-[#ffe2be]'
            : errorCount > 0
              ? 'border-rose-400/40 bg-rose-500/15 text-rose-200'
              : 'border-emerald-400/35 bg-emerald-500/12 text-emerald-300'
        ]"
      >
        {{ isPending ? '进行中' : errorCount > 0 ? `${errorCount} 受限` : '完成' }}
      </span>
    </summary>

    <div class="mt-2 pt-2 border-t border-[#fff8ee]/8 flex flex-col gap-1.5">
      <div
        v-for="(step, idx) in summarizedSteps"
        :key="step.id"
        class="rounded-xl border border-[#fff8ee]/10 bg-[#fff8ee]/[0.025] px-3 py-2 flex flex-col gap-1"
      >
        <div class="flex items-center justify-between gap-2">
          <div class="min-w-0 flex items-center gap-1.5">
            <span class="text-[10.5px] font-mono text-slate-500 shrink-0">{{ idx + 1 }}.</span>
            <a
              v-if="step.url"
              :href="step.url"
              target="_blank"
              rel="noopener noreferrer"
              class="text-xs font-medium text-[#f2b880] hover:underline truncate"
            >
              {{ step.title }}
            </a>
            <span v-else class="text-xs font-medium text-slate-200 truncate">{{ step.title }}</span>
          </div>
          <div class="flex items-center gap-2 shrink-0 text-[10.5px]">
            <span v-if="step.durationMs !== undefined" class="text-slate-500">{{ formatDurationMs(step.durationMs) }}</span>
            <span :class="step.isError ? 'text-rose-400' : isPending && step.statusLabel.endsWith('...') ? 'text-[#f2b880]' : 'text-emerald-400'">
              {{ step.statusLabel }}
            </span>
          </div>
        </div>

        <div v-if="step.subtitle" class="text-[11px] text-slate-400 pl-4 truncate">
          {{ step.subtitle }}
        </div>

        <div v-if="step.sources.length > 0" class="flex flex-wrap gap-1.5 pl-4 pt-0.5">
          <a
            v-for="src in step.sources"
            :key="src.url"
            :href="src.url"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1 max-w-full px-2 py-0.5 rounded-full border border-[#fff8ee]/10 bg-[#fff8ee]/[0.03] hover:border-[#f2b880]/45 text-[10.5px] text-slate-300 hover:text-[#ffe2be] transition"
          >
            <span class="truncate max-w-[14rem]">{{ src.title }}</span>
            <span class="text-slate-500 shrink-0">· {{ src.domain }}</span>
          </a>
        </div>
      </div>
    </div>
  </details>
</template>
