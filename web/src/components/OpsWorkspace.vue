<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { describeCron, formatDateOnly } from '../utils/format'
import type { Asset } from '../types/bothub'

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
const syncing = ref(false)

const filteredAssets = computed(() =>
  store.state.assets.filter((a) => {
    if (activeCat.value !== 'all' && a.category !== activeCat.value) return false
    const q = searchQuery.value.trim().toLowerCase()
    if (!q) return true
    return `${a.name} ${a.spec} ${a.status} ${a.extra_config}`.toLowerCase().includes(q)
  })
)

function categoryName(cat: string): string {
  if (cat === 'compute') return '算力基建'
  if (cat === 'domain') return '核心域名'
  return '生活订阅'
}

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
    <header class="px-4 md:px-6 py-3 border-b border-[#fff8ee]/10 bg-[#080c18]/55 flex items-center justify-between gap-3">
      <div class="flex items-center gap-3 min-w-0">
        <button
          type="button"
          aria-label="打开菜单"
          class="md:hidden min-h-[40px] min-w-[40px] inline-flex items-center justify-center rounded-xl border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer transition"
          @click="emit('openMobileMenu')"
        >
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="6" x2="21" y2="6" /><line x1="3" y1="12" x2="21" y2="12" /><line x1="3" y1="18" x2="21" y2="18" /></svg>
        </button>
        <button
          v-if="desktopCollapsed"
          type="button"
          aria-label="展开侧边栏"
          title="展开侧边栏"
          class="hidden md:inline-flex items-center justify-center w-9 h-9 rounded-xl border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer transition"
          @click="emit('toggleDesktopSidebar')"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="3" width="18" height="18" rx="2" />
            <line x1="9" y1="3" x2="9" y2="21" />
            <polyline points="13 9 16 12 13 15" />
          </svg>
        </button>
        <div class="min-w-0">
          <h2 class="text-base md:text-lg font-semibold text-white truncate">全局资产守望与调度引擎</h2>
          <p class="text-xs text-slate-400 truncate">VPS / 域名 / 订阅到期台账与后台自动化调度</p>
        </div>
      </div>
      <button
        type="button"
        @click="store.loadDashboard(false)"
        class="min-h-[40px] sm:min-h-[34px] px-3 py-1.5 rounded-lg text-xs border border-[#fff8ee]/14 text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 cursor-pointer shrink-0 transition"
      >
        刷新数据
      </button>
    </header>

    <div class="flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
      <div class="max-w-6xl mx-auto flex flex-col gap-6">
        <!-- Compact 52px Telemetry Overview Strip -->
        <div class="aero-glass rounded-xl grid grid-cols-2 lg:grid-cols-4 divide-y lg:divide-y-0 lg:divide-x divide-[#fff8ee]/10">
          <div class="min-h-[52px] px-4 py-2 flex items-center justify-between gap-2">
            <div class="flex items-center gap-2 min-w-0">
              <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 shrink-0" />
              <span class="text-xs text-slate-400 truncate">在线追踪资产</span>
            </div>
            <span class="font-mono text-base font-bold text-[#f2b880]">{{ store.state.assets.length }}</span>
          </div>

          <div class="min-h-[52px] px-4 py-2 flex items-center justify-between gap-2">
            <div class="flex items-center gap-2 min-w-0">
              <span
                :class="[
                  'w-1.5 h-1.5 rounded-full shrink-0',
                  store.expiringAssetCount.value > 0 ? 'bg-amber-400' : 'bg-emerald-400'
                ]"
              />
              <span class="text-xs text-slate-400 truncate">7天内临期预警</span>
            </div>
            <span :class="['font-mono text-base font-bold', store.expiringAssetCount.value > 0 ? 'text-amber-300' : 'text-emerald-300']">
              {{ store.expiringAssetCount.value }}
            </span>
          </div>

          <div class="min-h-[52px] px-4 py-2 flex items-center justify-between gap-2">
            <div class="flex items-center gap-2 min-w-0">
              <span class="w-1.5 h-1.5 rounded-full bg-[#f2b880] shrink-0" />
              <span class="text-xs text-slate-400 truncate">启用调度例程</span>
            </div>
            <span class="font-mono text-base font-bold text-white">
              {{ store.state.tasks.filter((t) => t.enabled).length }}
            </span>
          </div>

          <button
            type="button"
            @click="store.state.showLogsModal = true"
            class="min-h-[52px] px-4 py-2 flex items-center justify-between gap-2 hover:bg-white/[0.03] transition cursor-pointer text-left"
          >
            <div class="flex items-center gap-2 min-w-0">
              <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 shrink-0" />
              <span class="text-xs text-slate-400 truncate">
                通道 · {{ store.state.configs['push_channel'] === 'pushplus' ? 'PushPlus' : '企业微信' }}
              </span>
            </div>
            <span class="font-mono text-xs font-semibold text-emerald-300 shrink-0">
              成功 {{ store.state.stats.sent_pushes }}
            </span>
          </button>
        </div>

        <!-- Asset Watcher Section -->
        <div class="flex flex-col gap-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h3 class="text-base font-semibold text-white">数字资产与会员守望</h3>
            <div class="flex items-center gap-2">
              <button
                type="button"
                @click="store.state.editingAsset = null"
                class="min-h-[40px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-[#fff8ee]/14 text-slate-200 hover:border-[#f2b880]/40 hover:text-[#f2b880] text-xs cursor-pointer transition"
              >
                + 新增资产
              </button>
              <button
                type="button"
                :disabled="syncing"
                @click="handleSyncAssets"
                class="min-h-[40px] sm:min-h-[34px] px-3.5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50 transition"
              >
                {{ syncing ? '嗅探同步中...' : '立即嗅探同步' }}
              </button>
            </div>
          </div>

          <div class="flex flex-wrap items-center justify-between gap-3">
            <input
              v-model="searchQuery"
              type="search"
              placeholder="搜索资产名称、规格或服务商..."
              class="w-full sm:w-64 min-h-[40px] sm:min-h-[36px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16]/85 px-3 py-2 text-xs text-white placeholder-slate-500 focus:border-[#f2b880]/60 focus:outline-none"
            />
            <div class="flex items-center gap-1.5 flex-wrap">
              <button
                v-for="cat in [
                  { id: 'all', label: '全部' },
                  { id: 'compute', label: '算力基建' },
                  { id: 'domain', label: '核心域名' },
                  { id: 'life', label: '生活订阅' },
                ]"
                :key="cat.id"
                type="button"
                @click="activeCat = cat.id"
                :class="[
                  'min-h-[40px] sm:min-h-[32px] px-3 py-1 rounded-full text-xs border transition cursor-pointer',
                  activeCat === cat.id
                    ? 'border-[#f2b880]/55 bg-[#f2b880]/15 text-[#f2b880] font-medium'
                    : 'border-[#fff8ee]/14 text-slate-300 hover:bg-white/5'
                ]"
              >
                {{ cat.label }}
              </button>
            </div>
          </div>

          <div v-if="filteredAssets.length === 0" class="aero-glass rounded-2xl p-8 text-center text-sm text-slate-400">
            {{ store.state.assets.length ? '当前筛选条件下无匹配资产' : '暂无追踪中的资产' }}
          </div>

          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3.5">
            <article
              v-for="a in filteredAssets"
              :key="a.id"
              @click="store.state.editingAsset = a"
              class="aero-glass rounded-2xl p-4 flex flex-col gap-2 cursor-pointer hover:border-[#f2b880]/45 transition"
            >
              <div class="flex items-center justify-between gap-2">
                <strong class="text-sm font-semibold text-white truncate">{{ a.name }}</strong>
                <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-[#fff8ee]/14 text-slate-300">
                  {{ categoryName(a.category) }}
                </span>
              </div>
              <div class="text-xs text-slate-400 truncate">{{ a.spec }}</div>
              <div class="flex items-center justify-between text-xs pt-2 border-t border-[#fff8ee]/8">
                <span class="font-mono text-slate-300">{{ a.price || '--' }}</span>
                <span :class="['font-mono', daysBadgeClass(a)]">{{ daysText(a) }}</span>
              </div>
              <div class="font-mono text-[11px] text-slate-500">
                到期: {{ formatDateOnly(a.due_date) }} · {{ a.auto_renew ? '自动续费' : '手动续费' }}
              </div>
            </article>
          </div>
        </div>

        <!-- Tasks Engine Section -->
        <div class="flex flex-col gap-3">
          <div class="flex items-center justify-between gap-3">
            <h3 class="text-base font-semibold text-white">自动化情报与调度任务</h3>
            <button
              type="button"
              @click="store.state.editingTask = null"
              class="min-h-[40px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-[#fff8ee]/14 text-slate-200 hover:border-[#f2b880]/40 hover:text-[#f2b880] text-xs cursor-pointer transition"
            >
              + 创建任务
            </button>
          </div>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-3.5">
            <article
              v-for="t in store.state.tasks"
              :key="t.id"
              class="aero-glass rounded-2xl p-4 flex flex-col gap-2.5"
            >
              <div class="flex items-start justify-between gap-2">
                <div>
                  <strong class="text-sm font-semibold text-white">{{ t.name }}</strong>
                  <div class="font-mono text-[11px] text-slate-400 mt-0.5">
                    {{ describeCron(t.cron_expr) }} · 归属 Bot: {{ t.bot_id || 'butler' }}
                  </div>
                </div>
                <button
                  type="button"
                  @click="toggleTaskEnabled(t.id, t.enabled)"
                  :class="[
                    'font-mono text-[11px] min-h-[36px] sm:min-h-[28px] px-2.5 py-1 rounded-full border cursor-pointer transition',
                    t.enabled ? 'border-emerald-400/35 bg-emerald-400/10 text-emerald-300' : 'border-[#fff8ee]/15 text-slate-400'
                  ]"
                >
                  {{ t.enabled ? '已启用' : '已暂停' }}
                </button>
              </div>
              <p class="text-xs text-slate-300">{{ t.description }}</p>
              <div class="font-mono text-[11px] text-slate-400 truncate">
                最近摘要: {{ t.last_run?.summary || '等待执行' }}
              </div>
              <div class="flex items-center gap-2 pt-1">
                <button
                  type="button"
                  @click="runTaskNow(t.id)"
                  class="min-h-[40px] sm:min-h-[32px] px-3 py-1.5 rounded-lg bg-[#f2b880]/15 border border-[#f2b880]/40 text-[#f2b880] hover:bg-[#f2b880]/25 text-xs cursor-pointer transition"
                >
                  立即执行
                </button>
                <button
                  type="button"
                  @click="store.state.editingTask = t"
                  class="min-h-[40px] sm:min-h-[32px] px-3 py-1.5 rounded-lg border border-[#fff8ee]/14 text-slate-300 hover:bg-white/5 text-xs cursor-pointer transition"
                >
                  编辑配置
                </button>
              </div>
            </article>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
