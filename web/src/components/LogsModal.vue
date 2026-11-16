<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { formatTime } from '../utils/format'
import { applySiblingStaggerFallback, trackSpotlight } from '../utils/topLayer'
import BaseModal from './BaseModal.vue'

const store = useAppStore()
const listRef = ref<HTMLElement | null>(null)

const sentCount = computed(() => store.state.pushLogs.filter((l) => l.status === 'sent').length)
const failedCount = computed(() => store.state.pushLogs.filter((l) => l.status !== 'sent').length)

watch(
  () => [store.state.showLogsModal, store.state.pushLogs.length],
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

    <div ref="listRef" class="stagger-list flex-1 min-h-0 overflow-y-auto flex flex-col gap-2.5 pr-1">
      <div v-if="store.state.pushLogs.length === 0" class="py-12 text-center text-sm text-slate-500">
        暂无推送流水记录
      </div>
      <article
        v-for="log in store.state.pushLogs"
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
      </article>
    </div>
  </BaseModal>
</template>
