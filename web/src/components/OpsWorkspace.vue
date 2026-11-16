<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { describeCron, formatDateOnly } from '../utils/format'
import type { Asset } from '../types/bothub'

const emit = defineEmits<{
  openMobileMenu: []
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
    <header class="px-4 md:px-6 py-3 border-b border-slate-400/15 bg-[#080c18]/45 flex items-center justify-between gap-3">
      <div class="flex items-center gap-3">
        <button
          type="button"
          class="md:hidden p-2 rounded-xl border border-slate-400/20 text-slate-300 hover:text-white cursor-pointer"
          @click="emit('openMobileMenu')"
        >
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="6" x2="21" y2="6" /><line x1="3" y1="12" x2="21" y2="12" /><line x1="3" y1="18" x2="21" y2="18" /></svg>
        </button>
        <div>
          <h2 class="text-base md:text-lg font-semibold text-white">全局资产守望与调度引擎</h2>
          <p class="text-xs text-slate-400">VPS / 域名 / 订阅到期台账与后台自动化调度</p>
        </div>
      </div>
      <button
        type="button"
        @click="store.loadDashboard(false)"
        class="px-3 py-1.5 rounded-lg text-xs border border-slate-400/20 text-slate-300 hover:bg-white/5 cursor-pointer"
      >
        刷新数据
      </button>
    </header>

    <div class="flex-1 min-h-0 overflow-y-auto p-4 md:p-6">
      <div class="max-w-6xl mx-auto flex flex-col gap-6">
        <!-- 4 Metrics Cards -->
        <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
          <article class="aero-glass rounded-2xl p-4">
            <span class="text-xs text-slate-400">追踪中的数字资产</span>
            <div class="font-mono text-2xl font-bold text-sky-300 mt-1">{{ store.state.assets.length }}</div>
          </article>
          <article class="aero-glass rounded-2xl p-4">
            <span class="text-xs text-slate-400">7天内临期预警</span>
            <div class="font-mono text-2xl font-bold text-amber-300 mt-1">{{ store.expiringAssetCount.value }}</div>
          </article>
          <article class="aero-glass rounded-2xl p-4">
            <span class="text-xs text-slate-400">常态活跃调度任务</span>
            <div class="font-mono text-2xl font-bold text-white mt-1">{{ store.state.tasks.filter((t) => t.enabled).length }}</div>
          </article>
          <article
            @click="store.state.showLogsModal = true"
            class="aero-glass rounded-2xl p-4 cursor-pointer hover:border-sky-400/40 transition"
          >
            <span class="text-xs text-slate-400">累计推送成功流水</span>
            <div class="font-mono text-2xl font-bold text-emerald-300 mt-1">{{ store.state.stats.sent_pushes }}</div>
          </article>
        </div>

        <!-- Asset Watcher Section -->
        <div class="flex flex-col gap-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h3 class="text-base font-semibold text-white">数字资产与会员守望</h3>
            <div class="flex items-center gap-2">
              <button
                type="button"
                @click="store.state.editingAsset = null"
                class="px-3.5 py-2 rounded-xl border border-slate-400/20 text-slate-200 hover:bg-white/5 text-xs cursor-pointer"
              >
                + 新增资产
              </button>
              <button
                type="button"
                :disabled="syncing"
                @click="handleSyncAssets"
                class="px-3.5 py-2 rounded-xl bg-sky-500 hover:bg-sky-400 text-slate-950 font-semibold text-xs cursor-pointer disabled:opacity-50"
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
              class="w-full sm:w-64 rounded-xl border border-slate-400/20 bg-[#080d1c]/85 px-3 py-2 text-xs text-white placeholder-slate-500 focus:border-sky-400/50 focus:outline-none"
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
                  'px-3 py-1 rounded-full text-xs border transition cursor-pointer',
                  activeCat === cat.id
                    ? 'border-sky-400/55 bg-sky-400/15 text-sky-300'
                    : 'border-slate-400/20 text-slate-300 hover:bg-white/5'
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
              class="aero-glass rounded-2xl p-4 flex flex-col gap-2 cursor-pointer hover:border-sky-400/45 transition"
            >
              <div class="flex items-center justify-between gap-2">
                <strong class="text-sm font-semibold text-white truncate">{{ a.name }}</strong>
                <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-slate-400/20 text-slate-300">
                  {{ categoryName(a.category) }}
                </span>
              </div>
              <div class="text-xs text-slate-400 truncate">{{ a.spec }}</div>
              <div class="flex items-center justify-between text-xs pt-2 border-t border-white/5">
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
              class="px-3.5 py-2 rounded-xl border border-slate-400/20 text-slate-200 hover:bg-white/5 text-xs cursor-pointer"
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
                    'font-mono text-[11px] px-2.5 py-1 rounded-full border cursor-pointer',
                    t.enabled ? 'border-emerald-400/35 bg-emerald-400/10 text-emerald-300' : 'border-slate-500/30 text-slate-400'
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
                  class="px-3 py-1.5 rounded-lg bg-sky-500/20 border border-sky-400/40 text-sky-300 hover:bg-sky-500/30 text-xs cursor-pointer"
                >
                  立即执行
                </button>
                <button
                  type="button"
                  @click="store.state.editingTask = t"
                  class="px-3 py-1.5 rounded-lg border border-slate-400/20 text-slate-300 hover:bg-white/5 text-xs cursor-pointer"
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
