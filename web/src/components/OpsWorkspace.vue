<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { describeCron, formatDateOnly } from '../utils/format'
import { applySiblingStaggerFallback, trackSpotlight } from '../utils/topLayer'
import type { Asset } from '../types/bothub'
import SegmentedTabs from './SegmentedTabs.vue'
import BotAvatar from './BotAvatar.vue'

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
const syncing = ref(false)
const assetGridRef = ref<HTMLElement | null>(null)
const taskGridRef = ref<HTMLElement | null>(null)

const CAT_TABS = [
  { id: 'all', label: '全部' },
  { id: 'compute', label: '算力基建' },
  { id: 'domain', label: '核心域名' },
  { id: 'life', label: '生活订阅' },
]

function isExpiringSoon(a: Asset): boolean {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) return false
  return a.days_left <= 7
}

const filteredAssets = computed(() =>
  store.state.assets.filter((a) => {
    if (activeCat.value !== 'all' && a.category !== activeCat.value) return false
    if (expiringOnly.value && !isExpiringSoon(a)) return false
    const q = searchQuery.value.trim().toLowerCase()
    if (!q) return true
    return `${a.name} ${a.spec} ${a.status} ${a.extra_config}`.toLowerCase().includes(q)
  })
)

function daysText(a: Asset): string {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) return '长期有效'
  if (a.days_left < 0) return `逾期 ${Math.abs(a.days_left)} 天`
  return `剩余 ${a.days_left} 天`
}

function daysBadgeClass(a: Asset): string {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) {
    return 'text-emerald-300'
  }
  if (a.days_left <= 3) return 'text-rose-400 font-semibold'
  if (a.days_left <= 7) return 'text-amber-300 font-semibold'
  return 'text-emerald-300'
}

function cardAccentClass(a: Asset): string {
  if (a.status === '长期有效' || String(a.due_date || '').startsWith('2099')) {
    return 'border-l-[3px] border-l-emerald-400/45'
  }
  if (a.days_left <= 3) return 'border-l-[3px] border-l-rose-400'
  if (a.days_left <= 7) return 'border-l-[3px] border-l-amber-400'
  return 'border-l-[3px] border-l-[#f2b880]/40'
}

function resetAssetFilters() {
  searchQuery.value = ''
  activeCat.value = 'all'
  expiringOnly.value = false
}

onMounted(() => {
  applySiblingStaggerFallback(assetGridRef.value)
  applySiblingStaggerFallback(taskGridRef.value)
})

watch(() => filteredAssets.value.length, () => nextTick(() => applySiblingStaggerFallback(assetGridRef.value)))
watch(() => store.state.tasks.length, () => nextTick(() => applySiblingStaggerFallback(taskGridRef.value)))

async function handleSyncAssets() {
  syncing.value = true
  try {
    const res = await BotHubApi.syncAssets()
    store.notify(res.summary || '资产嗅探同步完成')
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    syncing.value = false
  }
}

function taskBot(botId?: string) {
  const id = botId || 'butler'
  return store.state.bots.find((b) => b.id === id)
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
    .map((cols) => cols.map((c) => `"${String(c ?? '').replaceAll('"', '""')}"`).join(','))
    .join('\n')
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `bothub_assets_${Date.now()}.csv`
  a.click()
  URL.revokeObjectURL(url)
  store.notify(`已导出 ${filteredAssets.value.length} 项数字资产`)
}

async function runTaskNow(id: string) {
  try {
    const res = await BotHubApi.runTask(id)
    store.notify(res.summary || '执行完成')
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message)
  }
}

async function toggleTaskEnabled(id: string, enabled: boolean) {
  try {
    await BotHubApi.updateTask(id, { enabled: !enabled })
    store.notify(!enabled ? '任务已启用' : '任务已暂停')
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message)
  }
}
</script>

<template>
  <section class="flex-1 min-h-0 flex flex-col overflow-hidden">
    <header class="px-4 md:px-6 pt-[max(0.75rem,env(safe-area-inset-top,0.75rem))] pb-3 border-b border-[#fff8ee]/8 bg-[#080c18]/50 backdrop-blur-md flex items-center justify-between gap-3">
      <div class="flex items-center gap-3 min-w-0">
        <button
          type="button"
          aria-label="打开菜单"
          class="md:hidden min-h-[44px] min-w-[44px] inline-flex items-center justify-center rounded-xl border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
          @click="emit('openMobileMenu')"
        >
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <line x1="3" y1="6" x2="21" y2="6" />
            <line x1="3" y1="12" x2="21" y2="12" />
            <line x1="3" y1="18" x2="21" y2="18" />
          </svg>
        </button>
        <button
          v-if="desktopCollapsed"
          type="button"
          aria-label="展开侧边栏"
          title="展开侧边栏"
          class="hidden md:inline-flex items-center justify-center w-9 h-9 rounded-xl border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer"
          @click="emit('toggleDesktopSidebar')"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <rect x="3" y="3" width="18" height="18" rx="2" />
            <line x1="9" y1="3" x2="9" y2="21" />
            <polyline points="13 9 16 12 13 15" />
          </svg>
        </button>
        <div class="min-w-0">
          <h1 class="text-base md:text-lg font-semibold text-white truncate">全局资产守望与调度引擎</h1>
          <p class="text-xs text-slate-400/90 truncate">VPS / 域名 / 订阅到期台账与后台自动化调度</p>
        </div>
      </div>
      <button
        type="button"
        @click="store.loadDashboard(false)"
        class="min-h-[44px] sm:min-h-[34px] px-3.5 py-1.5 rounded-xl text-xs border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer shrink-0"
      >
        刷新数据
      </button>
    </header>

    <!-- Hairline 52px Interactive Telemetry Bar (clean, zero card bloat) -->
    <div class="border-b border-[#fff8ee]/8 bg-[#070a15]/60 grid grid-cols-2 lg:grid-cols-4 divide-y lg:divide-y-0 lg:divide-x divide-[#fff8ee]/8 shrink-0">
      <div class="min-h-[52px] px-4 md:px-6 py-2 flex items-center justify-between gap-2">
        <div class="flex items-center gap-2 min-w-0">
          <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 shrink-0" aria-hidden="true" />
          <span class="text-xs text-slate-400 truncate">在线追踪资产</span>
        </div>
        <span class="font-mono text-base font-bold text-[#f2b880]">{{ store.state.assets.length }}</span>
      </div>

      <button
        type="button"
        :aria-pressed="expiringOnly"
        @click="expiringOnly = !expiringOnly"
        :class="[
          'min-h-[52px] px-4 md:px-6 py-2 flex items-center justify-between gap-2 hover:bg-white/[0.03] cursor-pointer text-left transition-colors',
          expiringOnly ? 'bg-amber-400/12' : ''
        ]"
        title="点击切换仅看 7 天内临期资产"
      >
        <div class="flex items-center gap-2 min-w-0">
          <span
            :class="[
              'w-1.5 h-1.5 rounded-full shrink-0',
              store.expiringAssetCount.value > 0 ? 'bg-amber-400 animate-pulse' : 'bg-emerald-400'
            ]"
            aria-hidden="true"
          />
          <span class="text-xs text-slate-400 truncate">
            {{ expiringOnly ? '筛选中 · 临期预警' : '7天内临期预警' }}
          </span>
        </div>
        <span :class="['font-mono text-base font-bold', store.expiringAssetCount.value > 0 ? 'text-amber-300' : 'text-emerald-300']">
          {{ store.expiringAssetCount.value }}
        </span>
      </button>

      <div class="min-h-[52px] px-4 md:px-6 py-2 flex items-center justify-between gap-2">
        <div class="flex items-center gap-2 min-w-0">
          <span class="w-1.5 h-1.5 rounded-full bg-[#f2b880] shrink-0" aria-hidden="true" />
          <span class="text-xs text-slate-400 truncate">启用调度例程</span>
        </div>
        <span class="font-mono text-base font-bold text-white">
          {{ store.state.tasks.filter((t) => t.enabled).length }}
        </span>
      </div>

      <button
        type="button"
        @click="store.state.showLogsModal = true"
        class="min-h-[52px] px-4 md:px-6 py-2 flex items-center justify-between gap-2 hover:bg-white/[0.03] cursor-pointer text-left"
      >
        <div class="flex items-center gap-2 min-w-0">
          <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 shrink-0" aria-hidden="true" />
          <span class="text-xs text-slate-400 truncate">原生 APP 通知</span>
        </div>
        <span class="font-mono text-xs font-semibold text-emerald-300 shrink-0">
          成功 {{ store.state.stats.sent_pushes }}
        </span>
      </button>
    </div>

    <div class="cq-container flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
      <div class="max-w-6xl mx-auto flex flex-col gap-7">
        <!-- Asset Watcher Section -->
        <div class="flex flex-col gap-3.5">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h2 class="text-base font-semibold text-white">数字资产与会员守望</h2>
            <div class="flex items-center gap-2 flex-wrap">
              <button
                type="button"
                @click="exportAssetsCsv"
                class="min-h-[44px] sm:min-h-[34px] px-3 py-2 rounded-xl border border-[#fff8ee]/12 text-slate-300 hover:border-[#f2b880]/40 hover:text-[#f2b880] text-xs cursor-pointer"
              >
                导出 CSV
              </button>
              <button
                type="button"
                @click="store.state.editingAsset = null"
                class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-[#fff8ee]/12 text-slate-200 hover:border-[#f2b880]/40 hover:text-[#f2b880] text-xs cursor-pointer"
              >
                + 新增资产
              </button>
              <button
                type="button"
                :disabled="syncing"
                @click="handleSyncAssets"
                class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50"
              >
                {{ syncing ? '嗅探同步中...' : '立即嗅探同步' }}
              </button>
            </div>
          </div>

          <div class="flex flex-wrap items-center justify-between gap-3">
            <input
              v-model="searchQuery"
              type="search"
              aria-label="搜索资产"
              placeholder="搜索资产名称、规格或服务商..."
              @keydown.escape="searchQuery = ''"
              class="w-full sm:w-64 min-h-[44px] sm:min-h-[36px] rounded-xl border border-[#fff8ee]/12 bg-[#070a16]/85 px-3 py-2 text-xs text-white placeholder-slate-500 focus:border-[#f2b880]/60 focus:outline-none"
            />
            <SegmentedTabs v-model="activeCat" :items="CAT_TABS" variant="subtle" aria-label="资产类别筛选" />
          </div>

          <div v-if="filteredAssets.length === 0" class="vesper-card rounded-2xl p-8 text-center flex flex-col items-center gap-2.5 text-sm text-slate-400">
            <span>{{ store.state.assets.length ? '当前筛选条件下无匹配资产' : '暂无追踪中的资产' }}</span>
            <button
              v-if="store.state.assets.length > 0"
              type="button"
              @click="resetAssetFilters"
              class="min-h-[40px] sm:min-h-[32px] px-3.5 py-1 rounded-full border border-[#fff8ee]/14 text-xs text-slate-300 hover:text-[#f2b880] cursor-pointer"
            >
              重置筛选条件
            </button>
          </div>

          <!-- Flagship Luxury Bento Asset Cards -->
          <div ref="assetGridRef" class="ops-asset-grid stagger-list">
            <button
              v-for="a in filteredAssets"
              :key="a.id"
              type="button"
              @pointermove="trackSpotlight"
              @click="store.state.editingAsset = a"
              :class="[
                'group stagger-item luxury-card vesper-card-interactive rounded-2xl p-4 flex flex-col gap-3 cursor-pointer text-left',
                cardAccentClass(a)
              ]"
            >
              <div class="flex items-start justify-between gap-2.5 w-full">
                <div class="flex items-center gap-2.5 min-w-0">
                  <span class="w-8 h-8 rounded-xl border border-[#f2b880]/22 bg-[#f2b880]/10 text-[#f2b880] inline-flex items-center justify-center shrink-0">
                    <svg v-if="a.category === 'compute'" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                      <rect x="2" y="3" width="20" height="8" rx="2" />
                      <rect x="2" y="13" width="20" height="8" rx="2" />
                      <line x1="6" y1="7" x2="6.01" y2="7" />
                      <line x1="6" y1="17" x2="6.01" y2="17" />
                    </svg>
                    <svg v-else-if="a.category === 'domain'" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                      <circle cx="12" cy="12" r="9" />
                      <line x1="3" y1="12" x2="21" y2="12" />
                      <path d="M12 3a14 14 0 0 1 0 18 14 14 0 0 1 0-18z" />
                    </svg>
                    <svg v-else viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                      <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
                    </svg>
                  </span>
                  <div class="min-w-0">
                    <strong class="block text-sm font-semibold text-white group-hover:text-[#f7cba0] transition-colors truncate">{{ a.name }}</strong>
                    <span class="block text-[11.5px] text-slate-400 truncate">{{ a.spec }}</span>
                  </div>
                </div>
                <span class="font-mono tabular-nums text-xs font-semibold px-2 py-0.5 rounded-lg border border-[#f2b880]/22 bg-[#f2b880]/8 text-[#f2b880] shrink-0">
                  {{ a.price || '--' }}
                </span>
              </div>

              <div class="flex items-center justify-between text-[11px] pt-2.5 border-t border-[#fff8ee]/7 w-full">
                <span class="font-mono tabular-nums text-slate-500">
                  {{ formatDateOnly(a.due_date) }} · {{ a.auto_renew ? '自动续费' : '手动续费' }}
                </span>
                <span :class="['font-mono tabular-nums shrink-0', daysBadgeClass(a)]">{{ daysText(a) }}</span>
              </div>
            </button>
          </div>
        </div>

        <!-- Tasks Engine Section -->
        <div class="flex flex-col gap-3.5 pb-4">
          <div class="flex items-center justify-between gap-3">
            <h2 class="text-base font-semibold text-white">自动化情报与调度任务</h2>
            <button
              type="button"
              @click="store.state.editingTask = null"
              class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-[#fff8ee]/12 text-slate-200 hover:border-[#f2b880]/40 hover:text-[#f2b880] text-xs cursor-pointer"
            >
              + 创建任务
            </button>
          </div>

          <div ref="taskGridRef" class="ops-task-grid stagger-list">
            <article
              v-for="t in store.state.tasks"
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
                    <strong class="block text-sm font-semibold text-white truncate">{{ t.name }}</strong>
                    <div class="font-mono text-[11px] text-slate-400 mt-0.5 truncate">
                      {{ describeCron(t.cron_expr) }} · {{ t.bot_id || 'butler' }}
                    </div>
                  </div>
                </div>
                <button
                  type="button"
                  @click="toggleTaskEnabled(t.id, t.enabled)"
                  :class="[
                    'inline-flex items-center gap-1.5 font-mono text-[11px] min-h-[40px] sm:min-h-[28px] px-2.5 py-1 rounded-full border cursor-pointer shrink-0',
                    t.enabled ? 'border-emerald-400/25 bg-emerald-400/10 text-emerald-300' : 'border-[#fff8ee]/10 text-slate-400'
                  ]"
                >
                  <span :class="['w-1.5 h-1.5 rounded-full', t.enabled ? 'bg-emerald-400 animate-pulse' : 'bg-slate-600']" aria-hidden="true" />
                  {{ t.enabled ? '已启用' : '已暂停' }}
                </button>
              </div>
              <p class="text-xs text-slate-300/90 leading-relaxed">{{ t.description }}</p>
              <div class="font-mono text-[11px] text-slate-400 truncate rounded-lg bg-[#050811]/60 border border-[#fff8ee]/6 px-2.5 py-1.5">
                最近摘要: {{ t.last_run?.summary || '等待执行' }}
              </div>
              <div class="flex items-center justify-between gap-2 pt-0.5 flex-wrap">
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    @click="runTaskNow(t.id)"
                    class="min-h-[44px] sm:min-h-[32px] px-3 py-1.5 rounded-lg bg-[#f2b880]/15 border border-[#f2b880]/40 text-[#f2b880] hover:bg-[#f2b880]/25 text-xs cursor-pointer"
                  >
                    立即执行
                  </button>
                  <button
                    type="button"
                    @click="store.state.editingTask = t"
                    class="min-h-[44px] sm:min-h-[32px] px-3 py-1.5 rounded-lg border border-[#fff8ee]/12 text-slate-300 hover:bg-white/5 text-xs cursor-pointer"
                  >
                    编辑配置
                  </button>
                </div>
                <button
                  type="button"
                  @click="store.jumpToBot(t.bot_id || 'butler')"
                  class="min-h-[36px] sm:min-h-[28px] inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border border-[#fff8ee]/10 bg-[#050811]/70 hover:border-[#f2b880]/40 text-[11px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
                >
                  <BotAvatar :avatar-style="taskBot(t.bot_id)?.avatar_style" :state="taskBot(t.bot_id)?.state" size="xs" :interactive="false" />
                  <span>{{ taskBot(t.bot_id)?.name || (t.bot_id || 'butler') }}</span>
                </button>
              </div>
            </article>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
