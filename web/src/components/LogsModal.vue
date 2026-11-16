<script setup lang="ts">
import { useAppStore } from '../stores/useAppStore'
import { formatTime } from '../utils/format'

const store = useAppStore()
</script>

<template>
  <div
    v-if="store.state.showLogsModal"
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/65 backdrop-blur-sm"
    @click.self="store.state.showLogsModal = false"
  >
    <div class="aero-glass w-full max-w-2xl max-h-[85vh] rounded-2xl p-5 flex flex-col gap-4 shadow-2xl">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-lg font-semibold text-white">外部通道推送历史流水</h3>
          <p class="text-xs text-slate-400">企业微信与 PushPlus 投递记录 (实时审计)</p>
        </div>
        <button
          type="button"
          @click="store.state.showLogsModal = false"
          class="px-2.5 py-1 rounded-lg border border-slate-400/20 text-xs text-slate-300 hover:bg-white/5 cursor-pointer"
        >
          关闭
        </button>
      </div>

      <div class="flex-1 min-h-0 overflow-y-auto flex flex-col gap-2.5 pr-1">
        <div v-if="store.state.pushLogs.length === 0" class="py-10 text-center text-sm text-slate-500">
          暂无推送流水记录
        </div>
        <article
          v-for="log in store.state.pushLogs"
          :key="log.id"
          class="rounded-xl border border-slate-400/15 bg-[#080d1c]/80 p-3.5"
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
    </div>
  </div>
</template>
