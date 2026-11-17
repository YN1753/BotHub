<script setup lang="ts">
import { useAppStore, type ChatMessageUI } from '../../stores/useAppStore'
import { formatDurationMs } from '../../utils/format'
import { formatToolDisplayName } from '../../utils/toolMeta'

defineProps<{
  tools: ChatMessageUI['tools']
}>()

const emit = defineEmits<{
  copy: [text: string]
}>()

const store = useAppStore()

function formatToolLabel(name: string): string {
  return formatToolDisplayName(name, store.state.skills || [])
}

function sumToolDurationMs(tools: ChatMessageUI['tools']): number {
  return tools.reduce((acc, tc) => acc + (typeof tc.durationMs === 'number' ? tc.durationMs : 0), 0)
}

function toolTrailSummary(tools: ChatMessageUI['tools']): string {
  const uniq: string[] = []
  for (const tc of tools) {
    const label = formatToolLabel(tc.name).split(' (')[0]
    if (!uniq.includes(label)) uniq.push(label)
    if (uniq.length >= 3) break
  }
  const tail = tools.length > 3 ? '...' : ''
  return uniq.join(' -> ') + tail
}

function formatToolPayload(raw?: string): string {
  if (!raw) return ''
  const trimmed = raw.trim()
  if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2)
    } catch {
      return trimmed
    }
  }
  return trimmed
}

function parseConsultSummary(raw?: string): { label: string; tools: string[] } | null {
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw)
    if (parsed && parsed.ok && parsed.bot_name) {
      return {
        label: `委派协办成员: ${parsed.bot_name}${parsed.role_title ? ` (${parsed.role_title})` : ''}`,
        tools: Array.isArray(parsed.tools_used) ? parsed.tools_used.map(String) : [],
      }
    }
    if (parsed && parsed.parallel_count && Array.isArray(parsed.results)) {
      const names = parsed.results.filter((x: any) => x?.ok && x?.bot_name).map((x: any) => String(x.bot_name))
      return {
        label: `并发协办 (${parsed.parallel_count || parsed.results.length} 位成员): ${names.join('、')}`,
        tools: [],
      }
    }
  } catch {
    // ignore
  }
  return null
}
</script>

<template>
  <details
    v-if="tools.length"
    :class="[
      'vesper-details well-recessed rounded-2xl px-3.5 py-2.5 font-mono text-[11.5px]',
      tools.some((t) => t.status === 'error')
        ? 'border-rose-400/35'
        : '',
    ]"
  >
    <summary class="cursor-pointer flex items-center justify-between gap-2 text-slate-200 select-none">
      <span class="inline-flex items-center gap-2 min-w-0">
        <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="details-chevron text-[#f2b880] shrink-0">
          <polyline points="9 18 15 12 9 6" />
        </svg>
        <span class="truncate">
          工具轨迹 · {{ toolTrailSummary(tools) }} ({{ tools.length }} 步<template v-if="sumToolDurationMs(tools) > 0"> · {{ formatDurationMs(sumToolDurationMs(tools)) }}</template>)
        </span>
      </span>
      <span
        :class="[
          'px-2 py-0.5 rounded-full text-[10px] border shrink-0',
          tools.some((t) => !t.done)
            ? 'border-[#f2b880]/40 bg-[#f2b880]/15 text-[#ffe2be]'
            : tools.some((t) => t.status === 'error')
            ? 'border-rose-400/40 bg-rose-500/15 text-rose-200'
            : 'border-emerald-400/35 bg-emerald-500/12 text-emerald-300'
        ]"
      >
        {{
          tools.some((t) => !t.done)
            ? '执行中'
            : tools.some((t) => t.status === 'error')
            ? `存在异常 (${tools.filter((t) => t.status === 'error').length})`
            : '全部完成'
        }}
      </span>
    </summary>
    <div class="mt-2.5 pt-2.5 border-t border-[#fff8ee]/8 flex flex-col gap-2">
      <div
        v-for="(tc, idx) in tools"
        :key="tc.id"
        class="rounded-xl border border-[#fff8ee]/10 bg-white/[0.025] shadow-[inset_0_1px_0_rgba(255,248,238,0.06)] px-3 py-2"
      >
        <div class="flex items-center justify-between gap-2 text-[11px]">
          <span class="text-[#f2b880] font-semibold truncate">#{{ idx + 1 }} {{ formatToolLabel(tc.name) }}</span>
          <div class="flex items-center gap-2 shrink-0">
            <span v-if="tc.durationMs !== undefined" class="text-[10px] text-slate-400">{{ formatDurationMs(tc.durationMs) }}</span>
            <button
              v-if="tc.result || tc.arguments"
              type="button"
              @click="emit('copy', tc.result || tc.arguments || '')"
              class="text-[10px] text-slate-500 hover:text-[#f2b880] cursor-pointer"
            >
              复制
            </button>
            <span
              :class="
                !tc.done
                  ? 'text-amber-300'
                  : tc.status === 'error'
                  ? 'text-rose-400'
                  : 'text-emerald-400'
              "
            >
              {{ !tc.done ? '执行中' : tc.status === 'error' ? '异常' : '已完成' }}
            </span>
          </div>
        </div>
        <div v-if="tc.arguments && tc.arguments !== '{}'" class="mt-1 text-slate-400 whitespace-pre-wrap break-all max-h-32 overflow-y-auto">入参: {{ formatToolPayload(tc.arguments) }}</div>
        <div
          v-if="tc.name === 'consult_bot' && parseConsultSummary(tc.result)"
          class="mt-1.5 px-2 py-1 rounded border border-cyan-400/25 bg-cyan-500/8 text-[10.5px] text-cyan-200"
        >
          {{ parseConsultSummary(tc.result)?.label }}
          <template v-if="parseConsultSummary(tc.result)?.tools.length"> · 使用工具: {{ parseConsultSummary(tc.result)?.tools.join(', ') }}</template>
        </div>
        <div v-if="tc.result" class="mt-1 text-slate-300 whitespace-pre-wrap break-all max-h-48 overflow-y-auto border-t border-[#fff8ee]/6 pt-1">出参: {{ formatToolPayload(tc.result) }}</div>
      </div>
    </div>
  </details>
</template>
