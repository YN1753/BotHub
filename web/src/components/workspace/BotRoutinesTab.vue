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
    void store.refreshWorkspaceSlices(['tasks', 'bots', 'notifications'])
    void store.selectBot(store.state.activeBotId)
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    runningTaskId.value = ''
  }
}

async function toggleRoutineEnabled(taskId: string, enabled: boolean) {
  const target = store.state.tasks.find((t) => t.id === taskId)
  const nextEnabled = !enabled
  if (target) target.enabled = nextEnabled
  store.notify(nextEnabled ? '例程已启用' : '例程已暂停')
  try {
    await BotHubApi.updateTask(taskId, { enabled: nextEnabled })
    void store.refreshWorkspaceSlices(['tasks'])
  } catch (err: any) {
    if (target) target.enabled = enabled
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
          <h2 class="text-base sm:text-lg font-semibold text-fg">专属定时例程</h2>
          <p class="text-xs text-fg-muted">触发后由当前 Bot 自主执行并将总结汇报至对话流</p>
        </div>
        <button
          type="button"
          @click="store.state.editingTask = { bot_id: bot?.id || '' }"
          class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl bg-accent hover:bg-accent-hover text-on-accent font-semibold text-xs cursor-pointer"
        >
          + 添加例程
        </button>
      </div>

      <div v-if="botRoutines.length === 0" class="vesper-card rounded-xl p-8 text-center text-sm text-fg-muted">
        当前 Bot 尚未绑定自动化例程，可点击右上角新增
      </div>

      <div ref="routinesGridRef" class="ops-task-grid stagger-list">
        <article
          v-for="t in botRoutines"
          :key="t.id"
          @pointermove="trackSpotlight"
          class="stagger-item vesper-card vesper-card-interactive rounded-xl p-4 flex flex-col gap-3"
        >
          <div class="flex items-start justify-between gap-2.5">
            <div class="flex items-center gap-2.5 min-w-0">
              <span class="w-8 h-8 rounded-xl border border-accent-border bg-accent-soft text-accent-fg inline-flex items-center justify-center shrink-0">
                <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                  <circle cx="12" cy="12" r="9" />
                  <polyline points="12 7 12 12 15 14" />
                </svg>
              </span>
              <div class="min-w-0">
                <h3 class="font-semibold text-sm text-fg truncate">{{ t.name }}</h3>
                <div class="font-mono text-xs text-fg-muted mt-0.5 truncate">
                  {{ t.run_at ? `单次提醒 ${t.run_at}` : describeCron(t.cron_expr) }}
                </div>
              </div>
            </div>
            <button
              type="button"
              @click="toggleRoutineEnabled(t.id, t.enabled)"
              :title="t.enabled ? '点击暂停此例程' : '点击启用此例程'"
              :class="[
                'inline-flex items-center gap-1.5 font-mono text-xs shrink-0 px-2.5 py-0.5 rounded-full border cursor-pointer transition-colors',
                t.enabled ? 'border-ok-border bg-ok-soft text-ok hover:bg-ok-soft' : 'border-line text-fg-muted hover:text-fg'
              ]"
            >
              <span :class="['w-1.5 h-1.5 rounded-full', t.enabled ? 'bg-ok animate-pulse' : 'bg-fg-muted']" aria-hidden="true" />
              {{ t.enabled ? '运行中' : '已暂停' }}
            </button>
          </div>
          <p class="text-xs text-fg-sub leading-relaxed">{{ t.description }}</p>
          <div class="font-mono text-xs text-fg-muted truncate rounded-lg bg-canvas border border-line px-2.5 py-1.5 flex items-center justify-between gap-2">
            <span class="truncate">最近结果: {{ t.last_run?.summary || '等待首次执行' }}</span>
            <span
              v-if="t.last_run"
              :class="['shrink-0 text-xs px-1.5 py-0.5 rounded', t.last_run.status === 'error' ? 'bg-danger-soft text-danger' : 'bg-ok-soft text-ok']"
            >
              {{ t.last_run.status === 'error' ? '异常' : `${t.last_run.duration_ms}ms` }}
            </span>
          </div>
          <div class="flex items-center gap-2 pt-1">
            <button
              type="button"
              :disabled="runningTaskId === t.id"
              @click="runRoutine(t.id)"
              class="min-h-[44px] sm:min-h-[32px] px-3 py-1.5 rounded-lg bg-accent-soft border border-accent-border text-accent-fg hover:bg-accent-soft text-xs cursor-pointer disabled:opacity-50"
            >
              {{ runningTaskId === t.id ? '执行中...' : '立即执行' }}
            </button>
            <button
              type="button"
              @click="store.state.editingTask = t"
              class="min-h-[44px] sm:min-h-[32px] px-3 py-1.5 rounded-lg border border-line text-fg-sub hover:bg-hover text-xs cursor-pointer"
            >
              配置参数
            </button>
          </div>
        </article>
      </div>
    </div>
  </div>
</template>
