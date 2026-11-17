<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import { describeCron } from '../../utils/format'
import { applySiblingStaggerFallback, trackSpotlight } from '../../utils/topLayer'

const store = useAppStore()
const bot = store.activeBot
const routinesGridRef = ref<HTMLElement | null>(null)
const runningTaskId = ref('')

const botRoutines = computed(() => {
  const bid = bot.value?.id
  if (!bid) return []
  return store.state.tasks.filter((t) => t.bot_id === bid)
})

async function runRoutine(taskId: string) {
  if (runningTaskId.value) return
  runningTaskId.value = taskId
  try {
    const res = await BotHubApi.runTask(taskId)
    store.notify(res.summary || '执行完成')
    await store.loadDashboard(false)
    await store.selectBot(store.state.activeBotId)
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    runningTaskId.value = ''
  }
}

async function toggleRoutineEnabled(taskId: string, enabled: boolean) {
  try {
    await BotHubApi.updateTask(taskId, { enabled: !enabled })
    store.notify(!enabled ? '例程已启用' : '例程已暂停')
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message)
  }
}

watch(
  () => [store.state.botView, botRoutines.value.length] as const,
  ([view]) => {
    if (view === 'routines') {
      nextTick(() => applySiblingStaggerFallback(routinesGridRef.value))
    }
  }
)

onMounted(() => {
  nextTick(() => applySiblingStaggerFallback(routinesGridRef.value))
})
</script>

<template>
  <div class="cq-container flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
    <div class="max-w-4xl mx-auto flex flex-col gap-4">
      <div class="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h2 class="text-base sm:text-lg font-semibold text-white">专属定时例程</h2>
          <p class="text-xs text-slate-400">触发后由当前 Bot 自主执行并将总结汇报至对话流</p>
        </div>
        <button
          type="button"
          @click="store.state.editingTask = { bot_id: bot?.id || '' }"
          class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer"
        >
          + 添加例程
        </button>
      </div>

      <div v-if="botRoutines.length === 0" class="vesper-card rounded-2xl p-8 text-center text-sm text-slate-400">
        当前 Bot 尚未绑定自动化例程，可点击右上角新增
      </div>

      <div ref="routinesGridRef" class="ops-task-grid stagger-list">
        <article
          v-for="t in botRoutines"
          :key="t.id"
          @pointermove="trackSpotlight"
          class="stagger-item luxury-card vesper-card-interactive rounded-2xl p-4 flex flex-col gap-3"
        >
          <div class="flex items-start justify-between gap-2.5">
            <div class="flex items-center gap-2.5 min-w-0">
              <span class="w-8 h-8 rounded-xl border border-[#f2b880]/22 bg-[#f2b880]/10 text-[#f2b880] inline-flex items-center justify-center shrink-0">
                <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                  <circle cx="12" cy="12" r="9" />
                  <polyline points="12 7 12 12 15 14" />
                </svg>
              </span>
              <div class="min-w-0">
                <h3 class="font-semibold text-sm text-white truncate">{{ t.name }}</h3>
                <div class="font-mono text-[11px] text-slate-400 mt-0.5 truncate">
                  {{ t.run_at ? `单次提醒 ${t.run_at}` : describeCron(t.cron_expr) }}
                </div>
              </div>
            </div>
            <button
              type="button"
              @click="toggleRoutineEnabled(t.id, t.enabled)"
              :title="t.enabled ? '点击暂停此例程' : '点击启用此例程'"
              :class="[
                'inline-flex items-center gap-1.5 font-mono text-[11px] shrink-0 px-2.5 py-0.5 rounded-full border cursor-pointer transition-colors',
                t.enabled ? 'border-emerald-400/25 bg-emerald-400/10 text-emerald-300 hover:bg-emerald-400/18' : 'border-[#fff8ee]/10 text-slate-500 hover:text-slate-200'
              ]"
            >
              <span :class="['w-1.5 h-1.5 rounded-full', t.enabled ? 'bg-emerald-400 animate-pulse' : 'bg-slate-600']" aria-hidden="true" />
              {{ t.enabled ? '运行中' : '已暂停' }}
            </button>
          </div>
          <p class="text-xs text-slate-300/90 leading-relaxed">{{ t.description }}</p>
          <div class="font-mono text-[11px] text-slate-400 truncate rounded-lg bg-[#050811]/60 border border-[#fff8ee]/6 px-2.5 py-1.5 flex items-center justify-between gap-2">
            <span class="truncate">最近结果: {{ t.last_run?.summary || '等待首次执行' }}</span>
            <span
              v-if="t.last_run"
              :class="['shrink-0 text-[10px] px-1.5 py-0.5 rounded', t.last_run.status === 'error' ? 'bg-rose-500/15 text-rose-300' : 'bg-emerald-500/12 text-emerald-300']"
            >
              {{ t.last_run.status === 'error' ? '异常' : `${t.last_run.duration_ms}ms` }}
            </span>
          </div>
          <div class="flex items-center gap-2 pt-1">
            <button
              type="button"
              :disabled="runningTaskId === t.id"
              @click="runRoutine(t.id)"
              class="min-h-[44px] sm:min-h-[32px] px-3 py-1.5 rounded-lg bg-[#f2b880]/15 border border-[#f2b880]/40 text-[#f2b880] hover:bg-[#f2b880]/25 text-xs cursor-pointer disabled:opacity-50"
            >
              {{ runningTaskId === t.id ? '执行中...' : '立即执行' }}
            </button>
            <button
              type="button"
              @click="store.state.editingTask = t"
              class="min-h-[44px] sm:min-h-[32px] px-3 py-1.5 rounded-lg border border-[#fff8ee]/12 text-slate-300 hover:bg-white/5 text-xs cursor-pointer"
            >
              配置参数
            </button>
          </div>
        </article>
      </div>
    </div>
  </div>
</template>
