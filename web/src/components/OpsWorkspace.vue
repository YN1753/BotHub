<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import {
  describeCron,
  formatDateOnly,
  inferRecommendedRenewMonths,
  summarizeAssetsMonthlyCost,
  summarizeAssetsYearlyCost,
  triggerBlobDownload,
  csvEscapeCell,
} from '../utils/format'
import { addMonthsPlain, diffPlainDaysFromToday, todayPlainDate } from '../utils/calendar'
import { mergeExtensibleCategories, resolveCategoryMeta } from '../utils/assetCategories'
import { pickDefaultWorkerBot } from '../utils/botLook'
import { applySiblingStaggerFallback, trackSpotlight } from '../utils/topLayer'
import type { Asset } from '../types/bothub'
import SegmentedTabs from './SegmentedTabs.vue'
import BotAvatar from './BotAvatar.vue'
import WorkspaceSidebarControls from './ui/WorkspaceSidebarControls.vue'
import AssetCategoryIcon from './ui/AssetCategoryIcon.vue'

defineProps<{
  desktopCollapsed?: boolean
}>()

const emit = defineEmits<{
  openMobileMenu: []
  toggleDesktopSidebar: []
}>()

const store = useAppStore()
const searchQuery = ref('')
const activeCat = ref('all')
const expiringOnly = ref(false)
const sortByUrgency = ref(false)
const taskSearch = ref('')
const taskStatusFilter = ref<'all' | 'enabled' | 'paused'>('all')
const syncing = ref(false)
const runningTaskId = ref('')
const assetGridRef = ref<HTMLElement | null>(null)
const taskGridRef = ref<HTMLElement | null>(null)
const taskSectionRef = ref<HTMLElement | null>(null)

const categoryTabs = computed(() => {
  const merged = mergeExtensibleCategories(store.state.assetCategories, store.state.assets)
  return [
    { id: 'all', label: '全部' },
    ...merged.map((c) => ({ id: c.id, label: c.label })),
  ]
})

function assetCategoryMeta(cat: string) {
  return resolveCategoryMeta(cat, store.state.assetCategories)
}

function isExpiringSoon(a: Asset): boolean {
  if (a.status === '已停用' || String(a.due_date || '').startsWith('2099')) return false
  return a.days_left <= 7
}

function assetStatusBadgeClass(status?: string): string {
  if (status === '已到期') return 'border-danger-border bg-danger-soft text-danger'
  if (status === '已停用') return 'border-line bg-subtle text-fg-muted'
  return 'border-ok-border bg-ok-soft text-ok'
}

const filteredAssets = computed(() => {
  const list = store.state.assets.filter((a) => {
    if (activeCat.value !== 'all' && a.category !== activeCat.value) return false
    if (expiringOnly.value && !isExpiringSoon(a)) return false
    const q = searchQuery.value.trim().toLowerCase()
    if (!q) return true
    return `${a.name} ${a.spec} ${a.status} ${a.extra_config}`.toLowerCase().includes(q)
  })
  if (!sortByUrgency.value) return list
  return list.toSorted((a, b) => {
    const aPerm = a.status === '长期有效' || String(a.due_date || '').startsWith('2099')
    const bPerm = b.status === '长期有效' || String(b.due_date || '').startsWith('2099')
    if (aPerm !== bPerm) return aPerm ? 1 : -1
    return (a.days_left ?? 9999) - (b.days_left ?? 9999)
  })
})

const activeCostSummary = computed(() => summarizeAssetsMonthlyCost(filteredAssets.value))
const activeYearlyCostSummary = computed(() => summarizeAssetsYearlyCost(filteredAssets.value))

const categoryCostBreakdowns = computed(() => {
  const cats = mergeExtensibleCategories(store.state.assetCategories, store.state.assets)
  return cats
    .map((c) => {
      const subset = store.state.assets.filter((a) => a.category === c.id && a.status !== '已停用')
      const monthly = summarizeAssetsMonthlyCost(subset)
      const yearly = summarizeAssetsYearlyCost(subset)
      return {
        id: c.id,
        label: c.label,
        count: subset.length,
        monthly,
        yearly,
      }
    })
    .filter((x) => x.count > 0 && (x.monthly || x.yearly))
})

const renewingAssetId = ref('')

async function quickRenewAsset(a: Asset, months: 1 | 3 | 12) {
  if (renewingAssetId.value) return
  const curDate = (a.due_date || '').slice(0, 10)
  const curDiff = diffPlainDaysFromToday(curDate)
  const baseDate =
    /^\d{4}-\d{2}-\d{2}$/.test(curDate) &&
    !curDate.startsWith('2099') &&
    curDiff !== null &&
    curDiff >= -30
      ? curDate
      : todayPlainDate()
  const nextDue = addMonthsPlain(baseDate, months)
  const nextDays = diffPlainDaysFromToday(nextDue) ?? a.days_left
  renewingAssetId.value = a.id
  try {
    await BotHubApi.updateAsset(a.id, {
      ...a,
      due_date: nextDue,
      days_left: nextDays,
      status: '生效中',
    })
    const stepLabel = months === 12 ? '1 年' : months === 3 ? '1 季度' : '1 个月'
    store.notify(`已将「${a.name}」顺延 ${stepLabel} 至 ${nextDue}`)
    await store.refreshWorkspaceSlices(['assets'])
  } catch (err: any) {
    store.notify(err.message || '顺延到期日失败')
  } finally {
    renewingAssetId.value = ''
  }
}

const healthMetrics = computed(() => {
  const total = store.state.assets.length
  if (!total) {
    return { healthyPct: 100, okStop: '100%', warnStop: '100%' }
  }
  let expired = 0
  let warn = 0
  for (const a of store.state.assets) {
    if (a.status === '已停用' || String(a.due_date || '').startsWith('2099')) continue
    if (a.status === '已到期' || a.days_left < 0) expired++
    else if (a.days_left <= 7) warn++
  }
  const healthy = Math.max(0, total - expired - warn)
  const okPct = Math.round((healthy / total) * 100)
  const warnPct = Math.round(((healthy + warn) / total) * 100)
  return {
    healthyPct: okPct,
    okStop: `${okPct}%`,
    warnStop: `${warnPct}%`,
  }
})

function resetAssetFilters() {
  searchQuery.value = ''
  activeCat.value = 'all'
  expiringOnly.value = false
}

function daysText(a: Asset): string {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) return '长期有效'
  if (a.days_left < 0) return `逾期 ${Math.abs(a.days_left)} 天`
  return `剩余 ${a.days_left} 天`
}

function daysBadgeClass(a: Asset): string {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) {
    return 'text-ok'
  }
  if (a.days_left <= 3) return 'text-danger font-semibold'
  if (a.days_left <= 7) return 'text-warn font-semibold'
  return 'text-ok'
}

function assetRunwayPercent(a: Asset): number {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) return 100
  if (a.days_left <= 0) return 6
  return Math.max(8, Math.min(100, Math.round((a.days_left / 90) * 100)))
}

function assetRunwayBarClass(a: Asset): string {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) {
    return 'bg-ok'
  }
  if (a.days_left <= 3) return 'bg-danger'
  if (a.days_left <= 7) return 'bg-warn'
  return 'bg-accent'
}

function cardAccentClass(a: Asset): string {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) {
    return 'border-l-[3px] border-l-ok'
  }
  if (a.days_left <= 3) return 'border-l-[3px] border-l-danger'
  if (a.days_left <= 7) return 'border-l-[3px] border-l-warn'
  return 'border-l-[3px] border-l-accent-border'
}

const filteredTasks = computed(() =>
  store.state.tasks.filter((t) => {
    if (taskStatusFilter.value === 'enabled' && !t.enabled) return false
    if (taskStatusFilter.value === 'paused' && t.enabled) return false
    const q = taskSearch.value.trim().toLowerCase()
    if (!q) return true
    return `${t.name} ${t.description} ${t.cron_expr || ''} ${t.bot_id || ''}`.toLowerCase().includes(q)
  })
)

function focusEnabledTasks() {
  taskStatusFilter.value = taskStatusFilter.value === 'enabled' ? 'all' : 'enabled'
  taskSectionRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

onMounted(() => {
  applySiblingStaggerFallback(assetGridRef.value)
  applySiblingStaggerFallback(taskGridRef.value)
})

watch(() => filteredAssets.value.length, () => nextTick(() => applySiblingStaggerFallback(assetGridRef.value)))
watch(() => filteredTasks.value.length, () => nextTick(() => applySiblingStaggerFallback(taskGridRef.value)))

async function handleSyncAssets() {
  syncing.value = true
  try {
    const res = await BotHubApi.syncAssets()
    store.notify(res.summary || '资产嗅探同步完成')
    void store.refreshWorkspaceSlices(['assets'])
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    syncing.value = false
  }
}

async function auditAssetsWithAssistant() {
  const targetBotId = pickDefaultWorkerBot(store.state.bots, store.activeBot.value)
  if (!targetBotId) {
    store.notify('请先创建一个可执行 Bot')
    return
  }
  await store.jumpToBot(
    targetBotId,
    '请调用 manage_assets(action="summary") 与 manage_assets(action="expiring", within_days=30) 为我全面核查月/年费预算与 30 天内到期续费风险，需要续费的项目请顺便整理为待办事项。'
  )
}

function taskBot(botId?: string) {
  if (!botId) return undefined
  return store.state.bots.find((b) => b.id === botId)
}

function exportAssetsCsv() {
  if (!filteredAssets.value.length) {
    store.notify('当前筛选结果为空，无法导出')
    return
  }
  const header = ['ID', '名称', '类别', '规格', '费用', '状态', '到期日期', '剩余天数', '自动续费']
  const rows = filteredAssets.value.map((a) => [
    a.id,
    a.name,
    a.category,
    a.spec,
    a.price,
    a.status,
    formatDateOnly(a.due_date),
    String(a.days_left),
    a.auto_renew ? '是' : '否',
  ])
  const csv = [header, ...rows]
    .map((cols) => cols.map((c) => csvEscapeCell(c)).join(','))
    .join('\n')
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' })
  triggerBlobDownload(blob, `bothub_assets_${Date.now()}.csv`)
  store.notify(`已导出 ${filteredAssets.value.length} 项数字资产`)
}

async function runTaskNow(id: string) {
  if (runningTaskId.value) return
  runningTaskId.value = id
  try {
    const res = await BotHubApi.runTask(id)
    store.notify(res.summary || '执行完成')
    void store.refreshWorkspaceSlices(['tasks', 'notifications', 'bots'])
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    runningTaskId.value = ''
  }
}

async function toggleTaskEnabled(id: string, enabled: boolean) {
  const target = store.state.tasks.find((t) => t.id === id)
  const nextEnabled = !enabled
  if (target) target.enabled = nextEnabled
  store.notify(nextEnabled ? '任务已启用' : '任务已暂停')
  try {
    await BotHubApi.updateTask(id, { enabled: nextEnabled })
    void store.refreshWorkspaceSlices(['tasks'])
  } catch (err: any) {
    if (target) target.enabled = enabled
    store.notify(err.message)
  }
}
</script>

<template>
  <section class="flex-1 min-h-0 flex flex-col overflow-hidden">
    <header
      class="px-4 md:px-6 pt-[max(0.75rem,env(safe-area-inset-top,0.75rem))] pb-3 bg-sidebar border-b border-line flex items-center justify-between gap-3"
    >
      <div class="flex items-center gap-3 min-w-0">
        <WorkspaceSidebarControls
          :desktop-collapsed="desktopCollapsed"
          @open-mobile-menu="emit('openMobileMenu')"
          @toggle-desktop-sidebar="emit('toggleDesktopSidebar')"
        />
        <div class="min-w-0">
          <h1 class="text-base md:text-lg font-semibold text-fg truncate">
            <span class="sm:hidden">资产与调度</span>
            <span class="hidden sm:inline">全局资产守望与调度引擎</span>
          </h1>
          <p class="hidden sm:block text-xs text-fg-muted truncate">VPS / 域名 / 订阅到期台账与后台自动化调度</p>
        </div>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <button
          type="button"
          @click="store.state.showLogsModal = true"
          title="查看系统推送与执行流水"
          class="btn-tactile min-h-[38px] sm:min-h-[34px] inline-flex items-center gap-1.5 px-3 rounded-full text-xs cursor-pointer"
        >
          <span class="w-1.5 h-1.5 rounded-full bg-info" aria-hidden="true" />
          <span>流水 {{ store.state.stats.sent_pushes }}</span>
        </button>
        <button
          type="button"
          @click="store.loadDashboard(false)"
          class="btn-tactile min-h-[38px] sm:min-h-[34px] px-3 rounded-full text-xs cursor-pointer"
        >
          刷新
        </button>
      </div>
    </header>

    <div class="cq-container flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
      <div class="max-w-6xl mx-auto flex flex-col gap-6">
        <!-- Asymmetric Bento Cockpit: 2-Column Compact Mobile Grid -> 4-Column Desktop Grid -->
        <div class="grid grid-cols-2 lg:grid-cols-4 gap-2.5 sm:gap-3">
          <div
            @pointermove="trackSpotlight"
            class="vesper-card col-span-2 rounded-xl p-3.5 sm:p-4 flex items-center justify-between gap-3 sm:gap-4 flex-wrap sm:flex-nowrap"
          >
            <div class="flex items-center gap-3.5 min-w-0">
              <div
                class="relative w-14 h-14 shrink-0 flex items-center justify-center"
                role="img"
                :aria-label="`资产健康率 ${healthMetrics.healthyPct}%`"
              >
                <div
                  class="health-donut-ring absolute inset-0"
                  :style="{ '--ok-stop': healthMetrics.okStop, '--warn-stop': healthMetrics.warnStop }"
                  aria-hidden="true"
                />
                <span class="relative z-10 font-mono text-xs font-bold text-fg tabular-nums">{{ healthMetrics.healthyPct }}%</span>
              </div>
              <div class="min-w-0">
                <div class="text-xs text-fg-muted">
                  全局追踪 {{ store.state.assets.length }} 项 · 月均 / 年化折算
                </div>
                <div class="font-mono tabular-nums text-lg sm:text-xl font-bold text-accent-fg truncate mt-0.5">
                  {{ activeCostSummary || '¥0 / 月' }}
                </div>
                <div v-if="activeYearlyCostSummary" class="font-mono text-xs text-fg-muted truncate">
                  年化: {{ activeYearlyCostSummary }}
                </div>
              </div>
            </div>

            <div class="flex items-center gap-2 shrink-0 ml-auto">
              <button
                v-if="store.state.bots.length > 0"
                type="button"
                @click="auditAssetsWithAssistant"
                class="btn-tactile min-h-[36px] px-3 rounded-lg text-info text-xs cursor-pointer"
              >
                AI 审计
              </button>
              <button
                type="button"
                :disabled="syncing"
                @click="handleSyncAssets"
                class="btn-jewel min-h-[36px] px-3.5 rounded-lg text-xs cursor-pointer disabled:opacity-50"
              >
                {{ syncing ? '同步中...' : '嗅探同步' }}
              </button>
            </div>
          </div>

          <button
            type="button"
            :aria-pressed="expiringOnly"
            @pointermove="trackSpotlight"
            @click="expiringOnly = !expiringOnly"
            :class="[
              'vesper-card vesper-card-interactive rounded-xl p-4 flex items-center justify-between gap-3 cursor-pointer text-left',
              expiringOnly ? 'border-warn-border ring-1 ring-warn-border' : ''
            ]"
            title="点击切换仅看 7 天内临期资产"
          >
            <div class="min-w-0">
              <div class="text-xs text-fg-muted truncate">
                {{ expiringOnly ? '筛选中 · 7天临期' : '7天内临期预警' }}
              </div>
              <div :class="['font-mono tabular-nums text-2xl font-bold mt-0.5', store.expiringAssetCount.value > 0 ? 'text-warn' : 'text-ok']">
                {{ store.expiringAssetCount.value }}
              </div>
            </div>
            <span
              :class="[
                'w-10 h-10 rounded-lg border inline-flex items-center justify-center shrink-0',
                store.expiringAssetCount.value > 0
                  ? 'bg-warn-soft border-warn-border text-warn'
                  : 'bg-ok-soft border-ok-border text-ok'
              ]"
              aria-hidden="true"
            >
              <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9"><circle cx="12" cy="12" r="9" /><line x1="12" y1="8" x2="12" y2="12" /><line x1="12" y1="16" x2="12.01" y2="16" /></svg>
            </span>
          </button>

          <button
            type="button"
            :aria-pressed="taskStatusFilter === 'enabled'"
            @pointermove="trackSpotlight"
            @click="focusEnabledTasks"
            title="点击定位并筛选已启用调度任务"
            :class="[
              'vesper-card vesper-card-interactive rounded-xl p-4 flex items-center justify-between gap-3 cursor-pointer text-left',
              taskStatusFilter === 'enabled' ? 'border-accent-border ring-1 ring-accent-border' : ''
            ]"
          >
            <div class="min-w-0">
              <div class="text-xs text-fg-muted truncate">
                {{ taskStatusFilter === 'enabled' ? '筛选中 · 活跃例程' : '活跃自动化例程' }}
              </div>
              <div class="font-mono tabular-nums text-2xl font-bold text-fg mt-0.5">
                {{ store.state.tasks.filter((t) => t.enabled).length }}
              </div>
            </div>
            <span class="w-10 h-10 rounded-lg bg-accent-soft border border-accent-border text-accent-fg inline-flex items-center justify-center shrink-0" aria-hidden="true">
              <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.9"><circle cx="12" cy="12" r="9" /><polyline points="12 7 12 12 15 14" /></svg>
            </span>
          </button>
        </div>

        <!-- Asset Watcher Section -->
        <div class="flex flex-col gap-3.5">
          <!-- Per-Category Original-Currency Cost Breakdown Pills -->
          <div
            v-if="categoryCostBreakdowns.length > 0"
            class="flex items-center gap-2 flex-wrap"
          >
            <button
              v-for="cb in categoryCostBreakdowns"
              :key="cb.id"
              type="button"
              @click="activeCat = activeCat === cb.id ? 'all' : cb.id"
              :class="[
                'rounded-lg border px-3 py-1.5 text-left transition cursor-pointer flex items-center gap-2',
                activeCat === cb.id
                  ? 'border-accent-border bg-accent-soft text-fg'
                  : 'border-line bg-card text-fg-sub hover:border-line-strong',
              ]"
            >
              <span class="text-xs font-medium">{{ cb.label }} ({{ cb.count }})</span>
              <span class="font-mono text-xs text-accent-fg">{{ cb.monthly }}</span>
              <span v-if="cb.yearly" class="font-mono text-xs text-fg-muted">· {{ cb.yearly }}</span>
            </button>
          </div>
          <div class="flex flex-wrap items-center justify-between gap-2.5">
            <div class="flex items-center gap-2 flex-wrap flex-1 min-w-0">
              <input
                v-model="searchQuery"
                type="search"
                aria-label="搜索资产"
                placeholder="搜索资产名称、规格或服务商..."
                @keydown.escape="searchQuery = ''"
                class="w-full sm:w-52 min-h-[36px] rounded-full border border-line bg-canvas px-3.5 py-1.5 text-xs text-fg placeholder:text-fg-muted focus:border-accent focus:outline-none"
              />
              <SegmentedTabs v-model="activeCat" :items="categoryTabs" variant="subtle" aria-label="资产类别筛选" />
              <button
                type="button"
                :aria-pressed="sortByUrgency"
                @click="sortByUrgency = !sortByUrgency"
                :class="[
                  'min-h-[34px] px-3 rounded-full text-xs cursor-pointer',
                  sortByUrgency ? 'btn-tactile-gold' : 'btn-tactile text-fg-muted'
                ]"
              >
                按紧迫度
              </button>
            </div>

            <div class="flex items-center gap-1.5 shrink-0">
              <button
                type="button"
                @click="exportAssetsCsv"
                title="导出当前筛选列表为 CSV"
                class="btn-tactile min-h-[34px] px-3 rounded-full text-xs cursor-pointer"
              >
                导出
              </button>
              <button
                type="button"
                @click="store.state.editingAsset = null"
                class="btn-tactile-gold min-h-[34px] px-3.5 rounded-full text-xs font-medium cursor-pointer"
              >
                + 新增资产
              </button>
            </div>
          </div>

          <div v-if="filteredAssets.length === 0" class="vesper-card rounded-xl p-8 text-center flex flex-col items-center gap-2.5 text-sm text-fg-muted">
            <span>{{ store.state.assets.length ? '当前筛选条件下无匹配资产' : '暂无追踪中的资产' }}</span>
            <button
              v-if="store.state.assets.length > 0"
              type="button"
              @click="resetAssetFilters"
              class="btn-tactile min-h-[40px] sm:min-h-[32px] px-3.5 py-1 rounded-full text-xs cursor-pointer"
            >
              重置筛选条件
            </button>
          </div>

          <!-- Asset Cards -->
          <div ref="assetGridRef" class="ops-asset-grid stagger-list">
            <button
              v-for="a in filteredAssets"
              :key="a.id"
              type="button"
              @pointermove="trackSpotlight"
              @click="store.state.editingAsset = a"
              :class="[
                'group stagger-item vesper-card vesper-card-interactive rounded-xl p-4 flex flex-col gap-3 cursor-pointer text-left',
                cardAccentClass(a)
              ]"
            >
              <div class="flex items-start justify-between gap-2.5 w-full">
                <div class="flex items-center gap-2.5 min-w-0">
                  <span
                    :title="assetCategoryMeta(a.category).label"
                    class="w-8 h-8 rounded-lg border border-accent-border bg-accent-soft text-accent-fg inline-flex items-center justify-center shrink-0"
                  >
                    <AssetCategoryIcon :icon="assetCategoryMeta(a.category).icon" :size="15" />
                  </span>
                  <div class="min-w-0">
                    <div class="flex items-center gap-1.5 min-w-0">
                      <strong class="block text-sm font-semibold text-fg group-hover:text-accent-fg transition-colors truncate">{{ a.name }}</strong>
                      <span :class="['font-mono text-xs px-1.5 py-0.5 rounded-lg border leading-none shrink-0', assetStatusBadgeClass(a.status)]">
                        {{ a.status || '生效中' }}
                      </span>
                    </div>
                    <span class="block text-xs text-fg-muted truncate">{{ a.spec }}</span>
                  </div>
                </div>
                <span class="font-mono tabular-nums text-xs font-semibold px-2 py-0.5 rounded-lg border border-accent-border bg-accent-soft text-accent-fg shrink-0">
                  {{ a.price || '--' }}
                </span>
              </div>

              <div class="flex flex-col gap-2 pt-2.5 border-t border-line w-full">
                <div class="well-recessed h-1.5 w-full rounded-full overflow-hidden p-[1px]" aria-hidden="true">
                  <div
                    :style="{ width: `${assetRunwayPercent(a)}%` }"
                    :class="['h-full rounded-full transition-[width] duration-300', assetRunwayBarClass(a)]"
                  />
                </div>
                <div class="flex items-center justify-between text-xs w-full gap-2 flex-wrap">
                  <span class="font-mono tabular-nums text-fg-muted">
                    {{ formatDateOnly(a.due_date) }} · {{ a.auto_renew ? '自动续费' : '手动续费' }}
                  </span>
                  <div class="flex items-center gap-1.5 ml-auto">
                    <template v-if="a.status !== '长期有效' && !String(a.due_date || '').startsWith('2099')">
                      <button
                        v-for="step in ([1, 3, 12] as const)"
                        :key="step"
                        type="button"
                        :disabled="renewingAssetId === a.id"
                        :title="`按原到期日顺延 +${step === 12 ? '1年' : step === 3 ? '1季' : '1月'}`"
                        @click.stop="quickRenewAsset(a, step)"
                        :class="[
                          'font-mono text-xs px-1.5 py-0.5 rounded-lg border transition cursor-pointer',
                          inferRecommendedRenewMonths(a.price) === step
                            ? 'border-accent-border bg-accent-soft text-accent-fg hover:border-accent'
                            : 'border-line text-fg-muted hover:border-line-strong hover:text-fg',
                        ]"
                      >
                        +{{ step === 12 ? '1年' : step === 3 ? '1季' : '1月' }}
                      </button>
                    </template>
                    <span :class="['font-mono tabular-nums shrink-0', daysBadgeClass(a)]">{{ daysText(a) }}</span>
                  </div>
                </div>
              </div>
            </button>
          </div>
        </div>

        <!-- Tasks Engine Section -->
        <div ref="taskSectionRef" class="flex flex-col gap-3.5 pb-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h2 class="text-base font-semibold text-fg">
              自动化情报与调度任务
              <span class="ml-1.5 text-xs font-mono text-fg-muted">({{ filteredTasks.length }})</span>
            </h2>
            <div class="flex items-center gap-2 flex-wrap">
              <div class="well-recessed inline-flex rounded-lg p-0.5 text-xs">
                <button
                  type="button"
                  @click="taskStatusFilter = 'all'"
                  :class="['px-2.5 py-1 rounded-lg cursor-pointer transition-all', taskStatusFilter === 'all' ? 'btn-tactile-gold' : 'text-fg-muted hover:text-fg']"
                >
                  全部
                </button>
                <button
                  type="button"
                  @click="taskStatusFilter = 'enabled'"
                  :class="['px-2.5 py-1 rounded-lg cursor-pointer transition-all', taskStatusFilter === 'enabled' ? 'bg-ok-soft text-ok border border-ok-border' : 'text-fg-muted hover:text-fg']"
                >
                  已启用
                </button>
                <button
                  type="button"
                  @click="taskStatusFilter = 'paused'"
                  :class="['px-2.5 py-1 rounded-lg cursor-pointer transition-all', taskStatusFilter === 'paused' ? 'bg-hover text-fg' : 'text-fg-muted hover:text-fg']"
                >
                  已暂停
                </button>
              </div>
              <button
                type="button"
                @click="store.state.editingTask = null"
                class="btn-tactile min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-lg text-xs cursor-pointer"
              >
                + 创建任务
              </button>
            </div>
          </div>

          <input
            v-if="store.state.tasks.length > 0"
            v-model="taskSearch"
            type="search"
            aria-label="搜索调度任务"
            placeholder="搜索任务名称、Cron 或关联 Bot..."
            @keydown.escape="taskSearch = ''"
            class="w-full sm:w-72 min-h-[44px] sm:min-h-[36px] rounded-lg border border-line bg-canvas px-3 py-2 text-xs text-fg placeholder:text-fg-muted focus:border-accent focus:outline-none"
          />

          <div v-if="filteredTasks.length === 0" class="vesper-card rounded-xl p-6 text-center flex flex-col items-center gap-2 text-xs text-fg-muted">
            <span>{{ store.state.tasks.length ? '当前筛选条件下无匹配调度任务' : '暂无自动化调度任务' }}</span>
            <button
              v-if="taskSearch.trim() || taskStatusFilter !== 'all'"
              type="button"
              @click="taskSearch = ''; taskStatusFilter = 'all'"
              class="btn-tactile px-3 py-1 rounded-full text-xs cursor-pointer"
            >
              重置任务筛选
            </button>
          </div>

          <div ref="taskGridRef" class="ops-task-grid stagger-list">
            <article
              v-for="t in filteredTasks"
              :key="t.id"
              @pointermove="trackSpotlight"
              class="stagger-item vesper-card vesper-card-interactive rounded-xl p-4 flex flex-col gap-3"
            >
              <div class="flex items-start justify-between gap-2.5">
                <div class="flex items-center gap-2.5 min-w-0">
                  <span class="w-8 h-8 rounded-lg border border-accent-border bg-accent-soft text-accent-fg inline-flex items-center justify-center shrink-0">
                    <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                      <circle cx="12" cy="12" r="9" />
                      <polyline points="12 7 12 12 15 14" />
                    </svg>
                  </span>
                  <div class="min-w-0">
                    <strong class="block text-sm font-semibold text-fg truncate">{{ t.name }}</strong>
                    <div class="font-mono text-xs text-fg-muted mt-0.5 truncate">
                      {{ t.run_at ? `单次提醒 ${t.run_at}` : describeCron(t.cron_expr) }} · {{ t.bot_id || '全局后台调度' }}
                    </div>
                  </div>
                </div>
                <button
                  type="button"
                  @click="toggleTaskEnabled(t.id, t.enabled)"
                  :class="[
                    'inline-flex items-center gap-1.5 font-mono text-xs min-h-[40px] sm:min-h-[28px] px-2.5 py-1 rounded-full border cursor-pointer shrink-0',
                    t.enabled ? 'border-ok-border bg-ok-soft text-ok' : 'border-line text-fg-muted'
                  ]"
                >
                  <span :class="['w-1.5 h-1.5 rounded-full', t.enabled ? 'bg-ok animate-pulse' : 'bg-fg-muted']" aria-hidden="true" />
                  {{ t.enabled ? '已启用' : '已暂停' }}
                </button>
              </div>
              <p class="text-xs text-fg-sub leading-relaxed">{{ t.description }}</p>
              <div class="well-recessed font-mono text-xs text-fg-sub truncate rounded-lg px-3 py-1.5">
                最近摘要: {{ t.last_run?.summary || '等待执行' }}
              </div>
              <div class="flex items-center justify-between gap-2 pt-0.5 flex-wrap">
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    :disabled="runningTaskId === t.id"
                    @click="runTaskNow(t.id)"
                    class="btn-tactile-gold min-h-[44px] sm:min-h-[32px] px-3 py-1.5 rounded-lg text-xs cursor-pointer disabled:opacity-60"
                  >
                    {{ runningTaskId === t.id ? '执行中...' : '立即执行' }}
                  </button>
                  <button
                    type="button"
                    @click="store.state.editingTask = t"
                    class="btn-tactile min-h-[44px] sm:min-h-[32px] px-3 py-1.5 rounded-lg text-xs cursor-pointer"
                  >
                    编辑配置
                  </button>
                </div>
                <button
                  v-if="t.bot_id && taskBot(t.bot_id)"
                  type="button"
                  @click="store.jumpToBot(t.bot_id)"
                  class="btn-tactile min-h-[36px] sm:min-h-[28px] inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs cursor-pointer"
                >
                  <BotAvatar :avatar-style="taskBot(t.bot_id)?.avatar_style" :state="taskBot(t.bot_id)?.state" size="xs" :interactive="false" />
                  <span>{{ taskBot(t.bot_id)?.name }}</span>
                </button>
                <span v-else class="font-mono text-xs text-fg-muted">未绑定 Bot</span>
              </div>
            </article>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
