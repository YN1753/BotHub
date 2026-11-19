<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import type { ChatMessageUI } from '../../utils/chatMapper'
import {
  extractStructuredToolCards,
  type AssetCardData,
  type AssetSummaryCardData,
  type BriefingCardData,
  type NezhaNodeCardData,
  type TaskCardData,
  type TodoCardData,
} from '../../utils/toolCards'
import { resolveCategoryMeta } from '../../utils/assetCategories'
import { describeCron, formatDateOnly, formatRelativeTime, formatTime } from '../../utils/format'
import type { Asset } from '../../types/bothub'
import AssetCategoryIcon from '../ui/AssetCategoryIcon.vue'

const props = defineProps<{
  tools: ChatMessageUI['tools']
}>()

const store = useAppStore()
const todoOverrides = reactive<Record<string, string>>({})
const togglingTodoId = ref('')

const cards = computed(() => extractStructuredToolCards(props.tools))

const summaryCards = computed(() =>
  cards.value.filter(
    (c): c is BriefingCardData | AssetSummaryCardData =>
      c.kind === 'briefing' || c.kind === 'asset_summary'
  )
)

const gridCards = computed(() =>
  cards.value.filter(
    (c): c is AssetCardData | NezhaNodeCardData | TodoCardData | TaskCardData =>
      c.kind !== 'briefing' && c.kind !== 'asset_summary'
  )
)

function assetCategoryMeta(cat: string) {
  return resolveCategoryMeta(cat, store.state.assetCategories)
}

function assetStatusClass(status: string): string {
  if (status === '已到期') return 'border-rose-400/35 bg-rose-500/12 text-rose-300'
  if (status === '已停用') return 'border-slate-500/30 bg-slate-500/10 text-slate-400'
  return 'border-emerald-400/30 bg-emerald-400/10 text-emerald-300'
}

function isPermanentAsset(card: AssetCardData): boolean {
  return card.status === '长期有效' || card.dueDate.startsWith('2099')
}

function runwayColorClass(card: AssetCardData): string {
  if (isPermanentAsset(card)) return 'bg-emerald-400/75'
  if (card.status === '已停用') return 'bg-slate-500/50'
  if (card.daysLeft <= 3) return 'bg-rose-400'
  if (card.daysLeft <= 7) return 'bg-amber-400'
  return 'bg-emerald-400'
}

function runwayPercent(card: AssetCardData): number {
  if (isPermanentAsset(card)) return 100
  if (card.status === '已停用') return 18
  if (card.daysLeft <= 0) return 8
  return Math.max(10, Math.min(100, Math.round((card.daysLeft / 30) * 100)))
}

function assetDaysLabel(card: AssetCardData): string {
  if (isPermanentAsset(card)) return '长期有效'
  if (card.daysLeft < 0) return `逾期 ${Math.abs(card.daysLeft)} 天`
  if (card.daysLeft === 0) return '今日到期'
  return `剩余 ${card.daysLeft} 天`
}

function assetDaysTextClass(card: AssetCardData): string {
  if (isPermanentAsset(card)) return 'text-emerald-300'
  if (card.daysLeft <= 3) return 'text-rose-400 font-semibold'
  if (card.daysLeft <= 7) return 'text-amber-300 font-semibold'
  return 'text-emerald-300'
}

function formatCurrencySymbol(currency: string): string {
  const c = currency.toUpperCase()
  if (c === 'USD') return '$'
  if (c === 'CNY' || c === 'RMB') return '¥'
  if (c === 'EUR') return '€'
  if (c === 'GBP') return '£'
  if (c === 'HKD') return 'HK$'
  return `${currency} `
}

function formatAmount(n: number): string {
  if (!Number.isFinite(n)) return '0'
  return n.toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1')
}

function openAssetEditor(card: AssetCardData) {
  const matched = store.state.assets.find((a) => a.id === card.id)
  if (matched) {
    store.state.editingAsset = matched
    return
  }
  const synthetic: Asset = {
    id: card.id,
    name: card.name,
    category: card.category || 'general',
    spec: card.spec,
    price: card.price,
    status: card.status || '生效中',
    due_date: card.dueDate,
    days_left: card.daysLeft,
    auto_renew: card.autoRenew,
    warn_days: '[7, 3, 1]',
    extra_config: '{}',
  }
  store.state.editingAsset = synthetic
}

function effectiveTodoStatus(card: TodoCardData): string {
  if (todoOverrides[card.id]) return todoOverrides[card.id]
  const live = store.state.todos.find((t) => t.id === card.id)
  return live ? live.status : card.status
}

function todoPriorityLabel(prio: string): { label: string; cls: string } {
  if (prio === 'high') {
    return { label: '高优先', cls: 'border-rose-400/35 bg-rose-500/12 text-rose-300' }
  }
  if (prio === 'low') {
    return { label: '低优先', cls: 'border-slate-500/30 bg-slate-500/10 text-slate-400' }
  }
  return { label: '普通', cls: 'border-[#fff8ee]/12 bg-[#fff8ee]/[0.04] text-slate-300' }
}

async function toggleTodoDone(card: TodoCardData) {
  if (togglingTodoId.value) return
  const prevStatus = effectiveTodoStatus(card)
  const nextStatus = prevStatus === 'done' ? 'pending' : 'done'
  togglingTodoId.value = card.id
  todoOverrides[card.id] = nextStatus
  const live = store.state.todos.find((t) => t.id === card.id)
  if (live) live.status = nextStatus

  try {
    await BotHubApi.updateTodo(card.id, { status: nextStatus })
    await store.refreshWorkspaceSlices(['todos'])
    store.notify(nextStatus === 'done' ? '已完成该项待办' : '已重新打开待办')
  } catch (err: any) {
    todoOverrides[card.id] = prevStatus
    if (live) live.status = prevStatus
    store.notify(err?.message || '更新待办状态失败')
  } finally {
    togglingTodoId.value = ''
  }
}

function resolveBotName(botId: string): string {
  if (!botId) return ''
  return store.state.bots.find((b) => b.id === botId)?.name || botId
}

function openTaskEditor(card: TaskCardData) {
  const matched = store.state.tasks.find((t) => t.id === card.id)
  if (matched) {
    store.state.editingTask = matched
  }
}
</script>

<template>
  <div v-if="cards.length > 0" class="cq-inline-cards flex flex-col gap-2 my-0.5">
    <!-- Top Summary / Briefing Strips -->
    <template v-for="sum in summaryCards" :key="sum.kind">
      <div
        v-if="sum.kind === 'briefing'"
        class="vesper-card rounded-2xl px-3.5 py-2.5 flex flex-col gap-2"
      >
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div class="flex items-center gap-2 min-w-0">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" class="text-[#f2b880] shrink-0" aria-hidden="true">
              <rect x="3" y="4" width="18" height="18" rx="2" />
              <line x1="16" y1="2" x2="16" y2="6" />
              <line x1="8" y1="2" x2="8" y2="6" />
              <line x1="3" y1="10" x2="21" y2="10" />
            </svg>
            <span class="text-xs font-semibold text-white">工作简报</span>
            <span v-if="sum.today" class="font-mono text-[10.5px] text-slate-400">
              {{ sum.today }}<template v-if="sum.weekday"> · {{ sum.weekday }}</template>
            </span>
          </div>
          <div class="flex flex-wrap items-center gap-1.5">
            <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-cyan-400/30 bg-cyan-500/10 text-cyan-200">
              待办 {{ sum.openTodoCount }}
            </span>
            <span
              v-if="sum.overdueCount > 0"
              class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-rose-400/35 bg-rose-500/12 text-rose-300"
            >
              逾期 {{ sum.overdueCount }}
            </span>
            <span
              :class="[
                'font-mono text-[10.5px] px-2 py-0.5 rounded-full border',
                sum.expiringAssetCount > 0
                  ? 'border-amber-400/35 bg-amber-500/12 text-amber-200'
                  : 'border-[#fff8ee]/12 bg-[#fff8ee]/[0.04] text-slate-300',
              ]"
            >
              临期 {{ sum.expiringAssetCount }}
            </span>
            <span
              v-if="sum.unreadCount > 0"
              class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-[#f2b880]/35 bg-[#f2b880]/12 text-[#ffe2be]"
            >
              未读 {{ sum.unreadCount }}
            </span>
            <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-emerald-400/30 bg-emerald-500/10 text-emerald-300">
              例程 {{ sum.activeTaskCount }}
            </span>
          </div>
        </div>
        <div v-if="sum.summaryText || sum.focusHints.length > 0" class="text-[11.5px] text-slate-300 leading-snug">
          {{ sum.summaryText || sum.focusHints[0] }}
        </div>
      </div>

      <div
        v-else-if="sum.kind === 'asset_summary'"
        class="vesper-card rounded-2xl px-3.5 py-2.5 flex items-center justify-between gap-2.5 flex-wrap"
      >
        <div class="flex items-center gap-2 flex-wrap">
          <span class="inline-flex items-center gap-1.5 text-xs font-semibold text-white">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" class="text-[#f2b880]" aria-hidden="true">
              <ellipse cx="12" cy="5" rx="9" ry="3" />
              <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5" />
              <path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3" />
            </svg>
            <span>资产概览</span>
          </span>
          <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-emerald-400/30 bg-emerald-500/10 text-emerald-300">
            活跃资产 {{ sum.activeCount }}
          </span>
          <span
            v-if="sum.expiring30dCount > 0"
            class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-amber-400/35 bg-amber-500/12 text-amber-200"
          >
            临期 {{ sum.expiring30dCount }}
          </span>
          <span
            v-if="sum.inactiveCount > 0"
            class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-slate-500/30 bg-slate-500/10 text-slate-400"
          >
            停用 {{ sum.inactiveCount }}
          </span>
        </div>

        <div v-if="sum.costByCurrency.length > 0" class="flex flex-wrap items-center gap-1.5">
          <span class="font-mono text-[10px] text-slate-400">成本汇总</span>
          <span
            v-for="c in sum.costByCurrency"
            :key="c.currency"
            class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-[#f2b880]/30 bg-[#f2b880]/10 text-[#ffe2be]"
          >
            {{ formatCurrencySymbol(c.currency) }}{{ formatAmount(c.monthlyEstimated) }}/月 · {{ formatCurrencySymbol(c.currency) }}{{ formatAmount(c.annualizedEstimated) }}/年
          </span>
        </div>
      </div>
    </template>

    <!-- Bento Grid of Individual Structured Cards -->
    <div v-if="gridCards.length > 0" class="inline-bento-grid">
      <template v-for="card in gridCards" :key="`${card.kind}:${card.id}`">
        <!-- Asset Card -->
        <button
          v-if="card.kind === 'asset'"
          type="button"
          @click="openAssetEditor(card)"
          class="vesper-card vesper-card-interactive text-left rounded-2xl p-3 flex flex-col gap-2 cursor-pointer"
        >
          <div class="flex items-start justify-between gap-2">
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-1.5 mb-1">
                <span class="inline-flex items-center gap-1 font-mono text-[10px] px-1.5 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#fff8ee]/[0.04] text-slate-300">
                  <AssetCategoryIcon :icon="assetCategoryMeta(card.category).icon" class="w-3 h-3 text-[#f2b880]" />
                  <span>{{ assetCategoryMeta(card.category).label }}</span>
                </span>
                <span
                  v-if="card.autoRenew"
                  class="font-mono text-[9.5px] px-1.5 py-0.5 rounded-md border border-cyan-400/30 bg-cyan-500/10 text-cyan-200"
                >
                  自动续费
                </span>
              </div>
              <div class="text-xs font-semibold text-white truncate">{{ card.name }}</div>
            </div>
            <span
              :class="[
                'font-mono text-[10px] px-2 py-0.5 rounded-full border shrink-0',
                assetStatusClass(card.status),
              ]"
            >
              {{ card.status }}
            </span>
          </div>

          <div v-if="card.spec || card.price" class="flex items-center justify-between gap-2 text-[11px]">
            <span class="text-slate-400 truncate">{{ card.spec || '无规格备注' }}</span>
            <span v-if="card.price" class="font-mono text-[10.5px] px-1.5 py-0.5 rounded bg-[#f2b880]/12 text-[#ffe2be] shrink-0">
              {{ card.price }}
            </span>
          </div>

          <div class="flex flex-col gap-1 pt-0.5">
            <div class="flex items-center justify-between gap-2 text-[10.5px] font-mono">
              <span class="text-slate-400">{{ formatDateOnly(card.dueDate) }}</span>
              <span :class="assetDaysTextClass(card)">{{ assetDaysLabel(card) }}</span>
            </div>
            <div class="h-1 w-full rounded-full bg-[#fff8ee]/[0.07] overflow-hidden">
              <div
                :class="['h-full rounded-full transition-all', runwayColorClass(card)]"
                :style="{ width: `${runwayPercent(card)}%` }"
              />
            </div>
          </div>
        </button>

        <!-- Nezha Node Card -->
        <div
          v-else-if="card.kind === 'nezha_node'"
          class="vesper-card rounded-2xl p-3 flex flex-col gap-2"
        >
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2 min-w-0">
              <span
                :class="[
                  'w-2 h-2 rounded-full shrink-0',
                  card.status === 'online'
                    ? 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.65)]'
                    : 'bg-rose-400',
                ]"
                aria-hidden="true"
              />
              <span class="text-xs font-semibold text-white truncate">{{ card.name }}</span>
            </div>
            <span
              :class="[
                'font-mono text-[10px] px-2 py-0.5 rounded-full border shrink-0',
                card.status === 'online'
                  ? 'border-emerald-400/30 bg-emerald-500/10 text-emerald-300'
                  : 'border-rose-400/35 bg-rose-500/12 text-rose-300',
              ]"
            >
              {{ card.status === 'online' ? '在线' : card.status || '离线' }}
            </span>
          </div>

          <div class="text-[11px] font-mono text-slate-300 leading-snug line-clamp-2">
            {{ card.hostSummary || '探针节点已连接' }}
          </div>

          <div class="flex items-center justify-between gap-2 text-[10px] font-mono text-slate-400 pt-0.5">
            <span class="truncate">{{ card.source || 'nezha' }}</span>
            <span v-if="card.lastActive" :title="formatTime(card.lastActive)" class="shrink-0">
              活跃 {{ formatRelativeTime(card.lastActive) }}
            </span>
          </div>
        </div>

        <!-- Todo Card -->
        <div
          v-else-if="card.kind === 'todo'"
          :class="[
            'vesper-card rounded-2xl p-3 flex items-start gap-2.5',
            effectiveTodoStatus(card) === 'done' ? 'opacity-70' : '',
          ]"
        >
          <button
            type="button"
            :disabled="togglingTodoId === card.id"
            :aria-label="effectiveTodoStatus(card) === 'done' ? '标为未完成' : '标为已完成'"
            @click="toggleTodoDone(card)"
            :class="[
              'mt-0.5 w-5 h-5 rounded-md border flex items-center justify-center shrink-0 cursor-pointer transition-colors',
              effectiveTodoStatus(card) === 'done'
                ? 'border-emerald-400/50 bg-emerald-500/20 text-emerald-300'
                : 'border-[#fff8ee]/20 bg-[#fff8ee]/[0.04] text-transparent hover:border-[#f2b880]/60',
            ]"
          >
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2.6" aria-hidden="true">
              <polyline points="20 6 9 17 4 12" />
            </svg>
          </button>

          <div class="min-w-0 flex-1 flex flex-col gap-1.5">
            <div
              :class="[
                'text-xs font-medium leading-snug break-words',
                effectiveTodoStatus(card) === 'done' ? 'line-through text-slate-400' : 'text-white',
              ]"
            >
              {{ card.title }}
            </div>
            <div class="flex flex-wrap items-center gap-1.5">
              <span
                :class="[
                  'font-mono text-[10px] px-1.5 py-0.5 rounded border',
                  todoPriorityLabel(card.priority).cls,
                ]"
              >
                {{ todoPriorityLabel(card.priority).label }}
              </span>
              <span
                v-if="card.dueAt"
                :class="[
                  'font-mono text-[10px] px-1.5 py-0.5 rounded border',
                  card.overdue && effectiveTodoStatus(card) !== 'done'
                    ? 'border-rose-400/35 bg-rose-500/12 text-rose-300'
                    : 'border-[#fff8ee]/12 bg-[#fff8ee]/[0.04] text-slate-400',
                ]"
              >
                {{ card.overdue && effectiveTodoStatus(card) !== 'done' ? `逾期 ${card.dueAt}` : card.dueAt }}
              </span>
              <span
                v-if="effectiveTodoStatus(card) === 'done'"
                class="font-mono text-[10px] text-emerald-300"
              >
                已完成
              </span>
            </div>
          </div>
        </div>

        <!-- Task Card -->
        <div
          v-else-if="card.kind === 'task'"
          @click="openTaskEditor(card)"
          :class="[
            'vesper-card rounded-2xl p-3 flex flex-col gap-1.5',
            store.state.tasks.some((t) => t.id === card.id) ? 'vesper-card-interactive cursor-pointer' : '',
          ]"
        >
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2 min-w-0">
              <span
                :class="[
                  'w-2 h-2 rounded-full shrink-0',
                  card.enabled ? 'bg-emerald-400' : 'bg-slate-500',
                ]"
                aria-hidden="true"
              />
              <span class="text-xs font-semibold text-white truncate">{{ card.name }}</span>
            </div>
            <span
              :class="[
                'font-mono text-[10px] px-2 py-0.5 rounded-full border shrink-0',
                card.enabled
                  ? 'border-emerald-400/30 bg-emerald-500/10 text-emerald-300'
                  : 'border-slate-500/30 bg-slate-500/10 text-slate-400',
              ]"
            >
              {{ card.enabled ? '运行中' : '已暂停' }}
            </span>
          </div>

          <div class="flex flex-wrap items-center gap-1.5 text-[10.5px]">
            <span class="font-mono px-1.5 py-0.5 rounded border border-[#fff8ee]/12 bg-[#fff8ee]/[0.04] text-slate-300">
              {{ card.runAt ? `单次 ${card.runAt}` : describeCron(card.cronExpr) }}
            </span>
            <span
              v-if="card.botId"
              class="font-mono px-1.5 py-0.5 rounded border border-[#f2b880]/25 bg-[#f2b880]/10 text-[#ffe2be]"
            >
              {{ resolveBotName(card.botId) }}
            </span>
          </div>

          <div v-if="card.lastSummary" class="text-[11px] text-slate-400 line-clamp-2 leading-snug">
            {{ card.lastSummary }}
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
