<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { formatTime } from '../utils/format'
import { renderSafeMarkdown } from '../utils/markdown'
import { applySiblingStaggerFallback, trackSpotlight } from '../utils/topLayer'
import BaseModal from './BaseModal.vue'
import SegmentedTabs from './SegmentedTabs.vue'

const store = useAppStore()
const activeFilter = ref<'all' | 'morning' | 'radar' | 'asset' | 'alert'>('all')
const listRef = ref<HTMLElement | null>(null)

const FILTER_TABS = [
  { id: 'all', label: '全部' },
  { id: 'morning', label: 'AI 晨报' },
  { id: 'radar', label: '前沿雷达' },
  { id: 'asset', label: '资产守望' },
  { id: 'alert', label: '探针告警' },
]
const desktopNotify = ref(
  localStorage.getItem('bothub_desktop_notify') === '1' &&
    typeof Notification !== 'undefined' &&
    Notification.permission === 'granted'
)

const filtered = computed(() =>
  store.state.notifications.filter((n) => {
    if (activeFilter.value === 'all') return true
    const s = n.source.toLowerCase()
    if (activeFilter.value === 'morning') return s.includes('morning')
    if (activeFilter.value === 'radar') return s.includes('radar') || s.includes('bili')
    if (activeFilter.value === 'asset') return s.includes('asset')
    if (activeFilter.value === 'alert') return s.includes('nezha') || s.includes('alert') || s.includes('webhook')
    return true
  })
)

function closeModal() {
  store.state.showInboxModal = false
}

async function askBotAnalyze(n: { id: number; source: string; title: string; summary: string }) {
  await markRead(n.id)
  const s = n.source.toLowerCase()
  let targetBotId = 'team_room'
  if (s.includes('nezha') || s.includes('alert') || s.includes('webhook')) targetBotId = 'sentinel'
  else if (s.includes('radar') || s.includes('bili')) targetBotId = 'radar'
  else if (s.includes('morning') || s.includes('asset')) targetBotId = 'butler'
  await store.jumpToBot(targetBotId, `请深度分析这条系统情报并给出建议：\n标题：${n.title}\n摘要：${n.summary}`)
}

async function markRead(id: number) {
  const item = store.state.notifications.find((n) => n.id === id)
  if (!item || item.read_at) return
  await BotHubApi.markNotificationRead(id).catch(() => {})
  item.read_at = new Date().toISOString()
}

async function markAllRead() {
  await BotHubApi.markAllNotificationsRead()
  const now = new Date().toISOString()
  store.state.notifications.forEach((n) => {
    if (!n.read_at) n.read_at = now
  })
  store.notify('全部情报已标记为已读')
}

async function toggleDesktopNotify() {
  if (typeof Notification === 'undefined') {
    store.notify('当前浏览器不支持桌面通知 API')
    return
  }
  if (desktopNotify.value) {
    localStorage.setItem('bothub_desktop_notify', '0')
    desktopNotify.value = false
    store.notify('已关闭桌面告警弹窗')
    return
  }
  const perm = Notification.permission === 'granted' ? 'granted' : await Notification.requestPermission()
  if (perm === 'granted') {
    localStorage.setItem('bothub_desktop_notify', '1')
    desktopNotify.value = true
    store.notify('已开启后台高优告警桌面弹窗')
  } else {
    store.notify('浏览器未授予通知权限')
  }
}

watch(
  () => [store.state.showInboxModal, filtered.value.length],
  () => nextTick(() => applySiblingStaggerFallback(listRef.value))
)
</script>

<template>
  <BaseModal
    :open="store.state.showInboxModal"
    title="原生情报收件箱 (Inbox)"
    subtitle="实时接收 AI 晨报、前沿雷达、资产到期提醒与哪吒探针告警"
    max-width="3xl"
    @close="closeModal"
  >
    <template #header-actions>
      <button
        type="button"
        @click="toggleDesktopNotify"
        class="min-h-[44px] sm:min-h-[34px] px-3 py-1.5 rounded-xl border border-[#fff8ee]/14 text-xs text-slate-300 hover:bg-white/5 cursor-pointer"
      >
        桌面弹窗: {{ desktopNotify ? '开' : '关' }}
      </button>
      <button
        type="button"
        @click="markAllRead"
        class="min-h-[44px] sm:min-h-[34px] px-3 py-1.5 rounded-xl border border-[#fff8ee]/14 text-xs text-slate-300 hover:text-[#f2b880] hover:border-[#f2b880]/40 hover:bg-white/5 cursor-pointer"
      >
        全部已读
      </button>
    </template>

    <div class="shrink-0">
      <SegmentedTabs v-model="activeFilter" :items="FILTER_TABS" variant="subtle" aria-label="情报分类筛选" />
    </div>

    <div ref="listRef" class="stagger-list flex-1 min-h-0 overflow-y-auto flex flex-col gap-2.5 pr-1">
      <div v-if="filtered.length === 0" class="py-12 text-center text-sm text-slate-500">
        当前分类下暂无情报消息
      </div>
      <article
        v-for="n in filtered"
        :key="n.id"
        @pointermove="trackSpotlight"
        :class="[
          'stagger-item luxury-card rounded-xl p-3.5',
          !n.read_at ? 'border-l-[3px] border-l-[#f2b880]' : ''
        ]"
      >
        <div class="flex items-start justify-between gap-2 flex-wrap">
          <div class="flex items-center gap-2 flex-wrap min-w-0">
            <span
              v-if="!n.read_at"
              class="w-2 h-2 rounded-full bg-[#f2b880] shadow-[0_0_8px_#f2b880] shrink-0"
              title="未读"
            />
            <span class="font-mono text-[10.5px] px-2 py-0.5 rounded-md border border-[#f2b880]/28 bg-[#f2b880]/10 text-[#f2b880] shrink-0">
              {{ n.source }}
            </span>
            <strong class="text-sm text-white truncate">{{ n.title }}</strong>
          </div>
          <span class="font-mono tabular-nums text-[11px] text-slate-400 shrink-0">{{ formatTime(n.created_at) }}</span>
        </div>
        <p class="text-xs text-slate-400 mt-1.5 leading-relaxed">{{ n.summary }}</p>
        <div class="mt-2 flex items-center justify-between gap-2 flex-wrap">
          <details class="vesper-details flex-1 min-w-0" @toggle="(e: any) => e.target?.open && markRead(n.id)">
            <summary class="cursor-pointer inline-flex items-center gap-1 font-mono text-[11px] text-[#f2b880] select-none">
              <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="details-chevron shrink-0">
                <polyline points="9 18 15 12 9 6" />
              </svg>
              <span>展开全文</span>
            </summary>
            <div class="mt-2 pt-2 border-t border-[#fff8ee]/10" v-html="renderSafeMarkdown(n.markdown || n.summary)" />
          </details>
          <button
            type="button"
            @click="askBotAnalyze(n)"
            class="min-h-[32px] sm:min-h-[26px] px-2.5 py-0.5 rounded-lg border border-[#f2b880]/30 bg-[#f2b880]/10 hover:bg-[#f2b880]/20 text-[11px] text-[#f2b880] cursor-pointer shrink-0 self-start"
          >
            让 Bot 分析 ->
          </button>
        </div>
      </article>
    </div>
  </BaseModal>
</template>
