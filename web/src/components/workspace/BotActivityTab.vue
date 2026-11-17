<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { formatTime } from '../../utils/format'
import { applySiblingStaggerFallback, clearSearchOnEscape } from '../../utils/topLayer'

const store = useAppStore()
const bot = store.activeBot
const activityListRef = ref<HTMLElement | null>(null)
const activityFilter = ref<'all' | 'tool' | 'routine' | 'error'>('all')
const activityQuery = ref('')

const botRoutines = computed(() => {
  const bid = bot.value?.id
  if (!bid) return []
  return store.state.tasks.filter((t) => t.bot_id === bid)
})

const activityEntries = computed(() => {
  const cur = bot.value
  if (!cur) return []
  const items: Array<{ title: string; time: string; detail: string; status?: string; durationMs?: number }> = []

  ;(store.state.botExecutions || []).forEach((ex) => {
    const kindLabel =
      ex.exec_type === 'tool'
        ? `工具调用 · ${ex.name}`
        : ex.exec_type === 'skill'
        ? `技能调用 · ${ex.name}`
        : ex.exec_type === 'routine'
        ? '例程任务'
        : ex.exec_type === 'delegate'
        ? `团队协办 · ${ex.name}`
        : `对话响应 · ${ex.name}`
    items.push({
      title: kindLabel,
      time: `${formatTime(ex.created_at)} · ${ex.duration_ms}ms`,
      detail: ex.output_summary || ex.input_summary || ex.name,
      status: ex.status,
      durationMs: ex.duration_ms,
    })
  })

  const boundTaskIDs = new Set<string>()
  botRoutines.value.forEach((t) => {
    boundTaskIDs.add(t.id)
    if (t.last_run) {
      items.push({
        title: `绑定例程 · ${t.name}`,
        time: `${formatTime(t.last_run.started_at)} · ${t.last_run.duration_ms}ms`,
        detail: t.last_run.summary,
        status: t.last_run.status,
      })
    }
  })

  store.state.notifications
    .filter((n) => boundTaskIDs.has(n.source) || n.source === `bot:${cur.id}` || n.source === cur.id)
    .slice(0, 12)
    .forEach((n) => {
      items.push({
        title: `专属推送 · ${n.title}`,
        time: formatTime(n.created_at),
        detail: n.summary,
      })
    })
  return items
})

const filteredActivityEntries = computed(() => {
  const q = activityQuery.value.trim().toLowerCase()
  return activityEntries.value.filter((act) => {
    if (activityFilter.value === 'error' && act.status !== 'error') return false
    if (activityFilter.value === 'tool' && !act.title.includes('工具') && !act.title.includes('技能')) return false
    if (activityFilter.value === 'routine' && !act.title.includes('例程')) return false
    if (!q) return true
    return `${act.title} ${act.detail}`.toLowerCase().includes(q)
  })
})

watch(
  () => [store.state.botView, filteredActivityEntries.value.length] as const,
  ([view]) => {
    if (view === 'activity') {
      nextTick(() => applySiblingStaggerFallback(activityListRef.value))
    }
  }
)

onMounted(() => {
  nextTick(() => applySiblingStaggerFallback(activityListRef.value))
})
</script>

<template>
  <div class="flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
    <div class="max-w-4xl mx-auto flex flex-col gap-3">
      <div class="flex items-center justify-between gap-2 flex-wrap">
        <div>
          <h2 class="text-base sm:text-lg font-semibold text-white">
            {{ bot?.name || '当前 Bot' }} 的专属运行脉搏
          </h2>
          <span class="text-[11px] font-mono text-slate-400">
            仅显示该会话自身的工具/技能调用、绑定例程与专属推送
          </span>
        </div>
        <div class="flex items-center gap-2 flex-wrap">
          <div class="inline-flex rounded-xl border border-[#fff8ee]/10 bg-[#050811]/70 p-0.5 text-[11px]">
            <button
              type="button"
              @click="activityFilter = 'all'"
              :class="['px-2.5 py-1 rounded-lg cursor-pointer transition-colors', activityFilter === 'all' ? 'bg-[#f2b880]/20 text-[#f2b880]' : 'text-slate-400 hover:text-white']"
            >
              全部 ({{ activityEntries.length }})
            </button>
            <button
              type="button"
              @click="activityFilter = 'tool'"
              :class="['px-2.5 py-1 rounded-lg cursor-pointer transition-colors', activityFilter === 'tool' ? 'bg-[#f2b880]/20 text-[#f2b880]' : 'text-slate-400 hover:text-white']"
            >
              工具/技能
            </button>
            <button
              type="button"
              @click="activityFilter = 'routine'"
              :class="['px-2.5 py-1 rounded-lg cursor-pointer transition-colors', activityFilter === 'routine' ? 'bg-cyan-500/20 text-cyan-200' : 'text-slate-400 hover:text-white']"
            >
              例程
            </button>
            <button
              type="button"
              @click="activityFilter = 'error'"
              :class="['px-2.5 py-1 rounded-lg cursor-pointer transition-colors', activityFilter === 'error' ? 'bg-rose-500/20 text-rose-300' : 'text-slate-400 hover:text-white']"
            >
              仅异常
            </button>
          </div>
          <input
            v-if="activityEntries.length > 0"
            v-model="activityQuery"
            type="search"
            aria-label="筛选运行脉搏"
            placeholder="筛选日志..."
            @keydown.escape="clearSearchOnEscape($event, activityQuery, () => { activityQuery = '' })"
            class="w-36 min-h-[30px] rounded-xl border border-[#fff8ee]/10 bg-[#050811] px-2.5 text-[11px] text-white placeholder-slate-500 focus:border-[#f2b880]/55 focus:outline-none"
          />
        </div>
      </div>
      <div v-if="filteredActivityEntries.length === 0" class="vesper-card rounded-2xl p-8 text-center flex flex-col items-center gap-2 text-sm text-slate-400">
        <span>{{ activityEntries.length ? '当前筛选条件下无匹配运行记录' : '暂无该 Bot 的专属运行记录' }}</span>
        <button
          v-if="activityFilter !== 'all' || activityQuery.trim()"
          type="button"
          @click="activityFilter = 'all'; activityQuery = ''"
          class="px-3 py-1 rounded-lg border border-[#fff8ee]/12 text-xs text-slate-300 hover:text-[#f2b880] cursor-pointer"
        >
          重置筛选条件
        </button>
      </div>
      <div ref="activityListRef" class="stagger-list flex flex-col gap-2.5">
        <article
          v-for="(act, idx) in filteredActivityEntries"
          :key="idx"
          class="stagger-item vesper-card rounded-xl px-4 py-3"
        >
          <div class="flex items-center justify-between gap-2 flex-wrap">
            <div class="flex items-center gap-2 min-w-0">
              <span
                v-if="act.status"
                :class="[
                  'px-1.5 py-0.5 rounded font-mono text-[10px] border shrink-0',
                  act.status === 'error'
                    ? 'border-rose-400/40 bg-rose-500/10 text-rose-300'
                    : 'border-emerald-400/40 bg-emerald-500/10 text-emerald-300',
                ]"
              >
                {{ act.status }}
              </span>
              <strong class="text-sm text-white truncate">{{ act.title }}</strong>
            </div>
            <span :class="['font-mono text-[11px] shrink-0', (act.durationMs || 0) >= 3000 ? 'text-amber-300' : 'text-slate-400']">{{ act.time }}</span>
          </div>
          <p class="text-xs text-slate-300 mt-1">{{ act.detail }}</p>
        </article>
      </div>
    </div>
  </div>
</template>
