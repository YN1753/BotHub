<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { formatTime, formatRelativeTime, triggerBlobDownload } from '../utils/format'
import { copyToClipboard } from '../utils/clipboard'
import { applySiblingStaggerFallback, clearSearchOnEscape, trackSpotlight } from '../utils/topLayer'
import BaseModal from './BaseModal.vue'
import SegmentedTabs from './SegmentedTabs.vue'

const store = useAppStore()
const listRef = ref<HTMLElement | null>(null)
const filterStatus = ref<'all' | 'sent' | 'failed'>('all')
const logQuery = ref('')

const LOG_TABS = [
  { id: 'all', label: '全部流水' },
  { id: 'sent', label: '已投递' },
  { id: 'failed', label: '异常重试' },
]

const sentCount = computed(() => store.state.pushLogs.filter((l) => l.status === 'sent').length)
const failedCount = computed(() => store.state.pushLogs.filter((l) => l.status !== 'sent').length)

const filteredLogs = computed(() => {
  const q = logQuery.value.trim().toLowerCase()
  return store.state.pushLogs.filter((l) => {
    if (filterStatus.value === 'sent' && l.status !== 'sent') return false
    if (filterStatus.value === 'failed' && l.status === 'sent') return false
    if (!q) return true
    return `${l.title} ${l.module_id} ${l.response_msg} ${l.content}`.toLowerCase().includes(q)
  })
})

async function copyPayload(content: string) {
  const ok = await copyToClipboard(content)
  store.notify(ok ? '已复制推送报文内容' : '复制失败')
}

function exportLogsJson() {
  if (!filteredLogs.value.length) {
    store.notify('当前无可导出的流水记录')
    return
  }
  const blob = new Blob([JSON.stringify(filteredLogs.value, null, 2)], { type: 'application/json;charset=utf-8' })
  triggerBlobDownload(blob, `bothub_push_logs_${Date.now()}.json`)
  store.notify(`已导出 ${filteredLogs.value.length} 条流水记录`)
}

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
    <template #header-actions>
      <button
        type="button"
        @click="exportLogsJson"
        class="min-h-[44px] sm:min-h-[34px] px-3 py-1.5 rounded-xl border border-line text-xs text-fg-sub hover:text-accent-fg hover:border-accent-border hover:bg-hover cursor-pointer"
      >
        导出 JSON
      </button>
    </template>
    <!-- Interactive Audit Strip -->
    <div class="modal-section grid grid-cols-3 divide-x divide-line p-0 overflow-hidden shrink-0">
      <button
        type="button"
        :aria-pressed="filterStatus === 'all'"
        @click="filterStatus = 'all'"
        :class="['px-3.5 py-2.5 flex items-center justify-between cursor-pointer hover:bg-hover', filterStatus === 'all' ? 'bg-accent-soft' : '']"
      >
        <span class="text-xs text-fg-muted">总记录</span>
        <span class="font-mono tabular-nums text-sm font-semibold text-fg">{{ store.state.pushLogs.length }}</span>
      </button>
      <button
        type="button"
        :aria-pressed="filterStatus === 'sent'"
        @click="filterStatus = 'sent'"
        :class="['px-3.5 py-2.5 flex items-center justify-between cursor-pointer hover:bg-hover', filterStatus === 'sent' ? 'bg-ok-soft' : '']"
      >
        <span class="text-xs text-fg-muted">已投递</span>
        <span class="font-mono tabular-nums text-sm font-semibold text-ok">{{ sentCount }}</span>
      </button>
      <button
        type="button"
        :aria-pressed="filterStatus === 'failed'"
        @click="filterStatus = 'failed'"
        :class="['px-3.5 py-2.5 flex items-center justify-between cursor-pointer hover:bg-hover', filterStatus === 'failed' ? 'bg-danger-soft' : '']"
      >
        <span class="text-xs text-fg-muted">异常</span>
        <span :class="['font-mono tabular-nums text-sm font-semibold', failedCount > 0 ? 'text-danger' : 'text-fg-muted']">
          {{ failedCount }}
        </span>
      </button>
    </div>

    <div class="shrink-0 flex items-center justify-between gap-2 flex-wrap">
      <SegmentedTabs v-model="filterStatus" :items="LOG_TABS" variant="subtle" aria-label="投递状态筛选" />
      <input
        v-if="store.state.pushLogs.length > 0"
        v-model="logQuery"
        type="search"
        aria-label="筛选推送流水"
        placeholder="搜索标题、模块或报文..."
        @keydown.escape="clearSearchOnEscape($event, logQuery, () => { logQuery = '' })"
        class="w-full sm:w-52 min-h-[34px] rounded-xl border border-line bg-canvas px-2.5 text-xs text-fg placeholder:text-fg-muted focus:border-accent focus:outline-none"
      />
    </div>

    <div ref="listRef" class="stagger-list flex-1 min-h-0 overflow-y-auto flex flex-col gap-2.5 pr-1">
      <div v-if="filteredLogs.length === 0" class="py-12 text-center text-sm text-fg-muted flex flex-col items-center gap-2">
        <span>暂无匹配推送流水记录</span>
        <button
          v-if="filterStatus !== 'all' || logQuery.trim()"
          type="button"
          @click="filterStatus = 'all'; logQuery = ''"
          class="px-3 py-1 rounded-lg border border-line text-xs text-fg-sub hover:border-accent-border hover:text-accent-fg cursor-pointer"
        >
          重置筛选条件
        </button>
      </div>
      <article
        v-for="log in filteredLogs"
        :key="log.id"
        @pointermove="trackSpotlight"
        class="stagger-item vesper-card rounded-xl p-3.5"
      >
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div class="flex items-center gap-2 min-w-0">
            <span
              :class="[
                'inline-flex items-center gap-1.5 font-mono text-xs px-2 py-0.5 rounded-full border shrink-0',
                log.status === 'sent'
                  ? 'border-ok-border bg-ok-soft text-ok'
                  : 'border-danger-border bg-danger-soft text-danger'
              ]"
            >
              <span :class="['w-1.5 h-1.5 rounded-full', log.status === 'sent' ? 'bg-ok' : 'bg-danger']" aria-hidden="true" />
              {{ log.status === 'sent' ? '已投递' : '失败' }}
            </span>
            <strong class="text-sm text-fg truncate">{{ log.title }}</strong>
          </div>
          <span :title="formatTime(log.created_at)" class="font-mono tabular-nums text-xs text-fg-muted shrink-0">{{ formatRelativeTime(log.created_at) }}</span>
        </div>
        <div class="font-mono text-xs text-fg-muted mt-1.5 truncate">模块: {{ log.module_id }} · {{ log.response_msg }}</div>
        <details v-if="log.content" class="vesper-details mt-2">
          <summary class="cursor-pointer inline-flex items-center gap-1 font-mono text-xs text-accent-fg select-none">
            <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="details-chevron shrink-0">
              <polyline points="9 18 15 12 9 6" />
            </svg>
            <span>查看推送报文</span>
          </summary>
          <div class="mt-2 relative">
            <button
              type="button"
              @click="copyPayload(log.content)"
              class="absolute top-2 right-2 px-2 py-0.5 rounded border border-line bg-canvas text-xs text-fg-sub hover:text-accent-fg hover:border-accent-border cursor-pointer"
            >
              复制报文
            </button>
            <pre class="p-2.5 pr-16 rounded-lg bg-canvas border border-line text-xs font-mono text-fg-sub whitespace-pre-wrap break-all">{{ log.content }}</pre>
          </div>
        </details>
      </article>
    </div>
  </BaseModal>
</template>
