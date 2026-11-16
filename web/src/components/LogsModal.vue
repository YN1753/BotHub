<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { formatTime } from '../utils/format'
import { applySiblingStaggerFallback, trackSpotlight } from '../utils/topLayer'
import BaseModal from './BaseModal.vue'
import SegmentedTabs from './SegmentedTabs.vue'

const store = useAppStore()
const listRef = ref<HTMLElement | null>(null)
const filterStatus = ref<'all' | 'sent' | 'failed'>('all')

const LOG_TABS = [
  { id: 'all', label: '全部流水' },
  { id: 'sent', label: '已投递' },
  { id: 'failed', label: '异常重试' },
]

const sentCount = computed(() => store.state.pushLogs.filter((l) => l.status === 'sent').length)
const failedCount = computed(() => store.state.pushLogs.filter((l) => l.status !== 'sent').length)

const filteredLogs = computed(() =>
  store.state.pushLogs.filter((l) => {
    if (filterStatus.value === 'sent') return l.status === 'sent'
    if (filterStatus.value === 'failed') return l.status !== 'sent'
    return true
  })
)

watch(
  () => [store.state.showLogsModal, filteredLogs.value.length],
  () => nextTick(() => applySiblingStaggerFallback(listRef.value))
)
</script>

<template>
  <BaseModal
    :open="store.state.showLogsModal"
    title="原生 APP 通知推送流水"
    subtitle="原生收件箱入库与 SSE 实时推送审计记录"
    max-width="2xl"
    @close="store.state.showLogsModal = false"
  >
    <!-- Compact Audit Strip -->
    <div class="modal-section grid grid-cols-3 divide-x divide-[#fff8ee]/10 p-0 overflow-hidden shrink-0">
      <div class="px-3.5 py-2.5 flex items-center justify-between">
        <span class="text-xs text-slate-400">总记录</span>
        <span class="font-mono tabular-nums text-sm font-semibold text-white">{{ store.state.pushLogs.length }}</span>
      </div>
      <div class="px-3.5 py-2.5 flex items-center justify-between">
        <span class="text-xs text-slate-400">已投递</span>
        <span class="font-mono tabular-nums text-sm font-semibold text-emerald-300">{{ sentCount }}</span>
      </div>
      <div class="px-3.5 py-2.5 flex items-center justify-between">
        <span class="text-xs text-slate-400">异常</span>
        <span :class="['font-mono tabular-nums text-sm font-semibold', failedCount > 0 ? 'text-rose-400' : 'text-slate-500']">
          {{ failedCount }}
        </span>
      </div>
    </div>

    <div class="shrink-0">
      <SegmentedTabs v-model="filterStatus" :items="LOG_TABS" variant="subtle" aria-label="投递状态筛选" />
    </div>

    <div ref="listRef" class="stagger-list flex-1 min-h-0 overflow-y-auto flex flex-col gap-2.5 pr-1">
      <div v-if="filteredLogs.length === 0" class="py-12 text-center text-sm text-slate-500">
        暂无匹配推送流水记录
      </div>
      <article
        v-for="log in filteredLogs"
        :key="log.id"
        @pointermove="trackSpotlight"
        class="stagger-item luxury-card rounded-xl p-3.5"
      >
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div class="flex items-center gap-2 min-w-0">
            <span
              :class="[
                'inline-flex items-center gap-1.5 font-mono text-[10.5px] px-2 py-0.5 rounded-full border shrink-0',
                log.status === 'sent'
                  ? 'border-emerald-400/30 bg-emerald-400/10 text-emerald-300'
                  : 'border-rose-400/40 bg-rose-400/10 text-rose-300'
              ]"
            >
              <span :class="['w-1.5 h-1.5 rounded-full', log.status === 'sent' ? 'bg-emerald-400' : 'bg-rose-400']" aria-hidden="true" />
              {{ log.status === 'sent' ? '已投递' : '失败' }}
            </span>
            <strong class="text-sm text-white truncate">{{ log.title }}</strong>
          </div>
          <span class="font-mono tabular-nums text-[11px] text-slate-400 shrink-0">{{ formatTime(log.created_at) }}</span>
        </div>
        <div class="font-mono text-[11px] text-slate-400 mt-1.5 truncate">模块: {{ log.module_id }} · {{ log.response_msg }}</div>
        <details v-if="log.content" class="vesper-details mt-2">
          <summary class="cursor-pointer inline-flex items-center gap-1 font-mono text-[10.5px] text-[#f2b880] select-none">
            <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="details-chevron shrink-0">
              <polyline points="9 18 15 12 9 6" />
            </svg>
            <span>查看推送报文</span>
          </summary>
          <pre class="mt-2 p-2.5 rounded-lg bg-[#050811]/85 border border-[#fff8ee]/8 text-[11px] font-mono text-slate-300 whitespace-pre-wrap break-all">{{ log.content }}</pre>
        </details>
      </article>
    </div>
  </BaseModal>
</template>
