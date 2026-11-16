<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { formatTime } from '../utils/format'
import { applySiblingStaggerFallback } from '../utils/topLayer'
import BaseModal from './BaseModal.vue'

const store = useAppStore()
const listRef = ref<HTMLElement | null>(null)

watch(
  () => [store.state.showLogsModal, store.state.pushLogs.length],
  () => nextTick(() => applySiblingStaggerFallback(listRef.value))
)
</script>

<template>
  <BaseModal
    :open="store.state.showLogsModal"
    title="外部通道推送历史流水"
    subtitle="企业微信与 PushPlus 投递记录 (实时审计)"
    max-width="2xl"
    @close="store.state.showLogsModal = false"
  >
    <div ref="listRef" class="stagger-list flex-1 min-h-0 overflow-y-auto flex flex-col gap-2.5 pr-1">
      <div v-if="store.state.pushLogs.length === 0" class="py-12 text-center text-sm text-slate-500">
        暂无推送流水记录
      </div>
      <article
        v-for="log in store.state.pushLogs"
        :key="log.id"
        class="stagger-item rounded-xl border border-[#fff8ee]/11 bg-[#070a16]/80 p-3.5"
      >
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div class="flex items-center gap-2">
            <span
              :class="[
                'font-mono text-[10.5px] px-2 py-0.5 rounded-md border',
                log.status === 'sent'
                  ? 'border-emerald-400/35 bg-emerald-400/10 text-emerald-300'
                  : 'border-rose-400/40 bg-rose-400/10 text-rose-300'
              ]"
            >
              {{ log.status === 'sent' ? '已投递' : '失败' }}
            </span>
            <strong class="text-sm text-white">{{ log.title }}</strong>
          </div>
          <span class="font-mono text-[11px] text-slate-400">{{ formatTime(log.created_at) }}</span>
        </div>
        <div class="font-mono text-[11px] text-slate-400 mt-1">模块: {{ log.module_id }} · {{ log.response_msg }}</div>
      </article>
    </div>
  </BaseModal>
</template>
