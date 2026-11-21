<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { formatRelativeTime, formatTime } from '../utils/format'
import { handleMarkdownCodeCopy, renderSafeMarkdown } from '../utils/markdown'
import { applySiblingStaggerFallback, clearSearchOnEscape, trackSpotlight } from '../utils/topLayer'
import BaseModal from './BaseModal.vue'
import SegmentedTabs from './SegmentedTabs.vue'

const store = useAppStore()
const activeFilter = ref<'all' | 'morning' | 'radar' | 'asset' | 'alert'>('all')
const onlyUnread = ref(false)
const inboxQuery = ref('')
const listRef = ref<HTMLElement | null>(null)
const rerollOpenId = ref<number | null>(null)
const rerollFeedback = ref('')
const rerollingId = ref<number | null>(null)
const savedTodoIds = ref<Record<number, boolean>>({})
const savedNoteIds = ref<Record<number, boolean>>({})

const FILTER_TABS = [
  { id: 'all', label: '全部' },
  { id: 'morning', label: '晨报' },
  { id: 'radar', label: '雷达' },
  { id: 'asset', label: '资产' },
  { id: 'alert', label: '告警' },
]
const desktopNotify = ref(
  localStorage.getItem('bothub_desktop_notify') === '1' &&
    typeof Notification !== 'undefined' &&
    Notification.permission === 'granted'
)

const filtered = computed(() =>
  store.state.notifications.filter((n) => {
    if (onlyUnread.value && n.read_at) return false
    const s = n.source.toLowerCase()
    if (activeFilter.value === 'morning' && !s.includes('morning')) return false
    if (activeFilter.value === 'radar' && !s.includes('radar') && !s.includes('bili')) return false
    if (activeFilter.value === 'asset' && !s.includes('asset')) return false
    if (activeFilter.value === 'alert' && !s.includes('nezha') && !s.includes('alert') && !s.includes('webhook')) return false
    const q = inboxQuery.value.trim().toLowerCase()
    if (!q) return true
    return `${n.title} ${n.summary} ${n.source} ${n.markdown || ''}`.toLowerCase().includes(q)
  })
)

function closeModal() {
  store.state.showInboxModal = false
}

async function askBotAnalyze(n: { id: number; source: string; title: string; summary: string; markdown: string }) {
  await markRead(n.id)
  if (store.state.bots.length === 0) {
    store.state.showInboxModal = false
    store.state.createModalMode = 'bot'
    store.state.showCreateBotModal = true
    store.notify('当前还没有 Bot，请先创建一个助手')
    return
  }
  const s = n.source.toLowerCase()
  const boundTask = store.state.tasks.find((t) => t.id === n.source && t.bot_id)
  const titlePrefixMatch = n.title.match(/^\[([^\]]+)\]/)
  const botFromTitle = titlePrefixMatch
    ? store.state.bots.find((b) => b.name === titlePrefixMatch[1].trim() || b.id === titlePrefixMatch[1].trim())
    : undefined
  const findByTool = (toolId: string) =>
    store.state.bots.find((b) => !b.is_group && (b.allowed_tools || '').includes(toolId))

  let picked =
    botFromTitle ||
    (boundTask?.bot_id && store.state.bots.find((b) => b.id === boundTask.bot_id)) ||
    store.state.bots.find((b) => b.id === 'bot_butler') ||
    store.state.bots.find((b) => !b.is_group && (b.bot_mode === 'worker' || b.bot_mode === 'hybrid')) ||
    store.state.bots[0]

  if (!boundTask?.bot_id && !botFromTitle) {
    if (s.includes('nezha') || s.includes('alert') || s.includes('webhook')) {
      picked = findByTool('query_nezha_nodes') || picked
    } else if (s.includes('radar') || s.includes('bili')) {
      picked = findByTool('web_search') || picked
    } else if (s.includes('morning') || s.includes('asset')) {
      picked = findByTool('manage_assets') || picked
    }
  }
  await store.quoteNotificationToBot(picked.id, n)
}

function toggleRerollBox(id: number) {
  if (rerollingId.value) return
  if (rerollOpenId.value === id) {
    rerollOpenId.value = null
    rerollFeedback.value = ''
  } else {
    rerollOpenId.value = id
    rerollFeedback.value = ''
  }
}

async function submitReroll(n: { id: number; title: string }, customFeedback?: string) {
  if (rerollingId.value) return
  rerollingId.value = n.id
  const fb = (customFeedback ?? rerollFeedback.value).trim()
  try {
    const res = await BotHubApi.rerollNotification(n.id, fb)
    if (res?.notification) {
      const updated = res.notification
      store.state.notifications = store.state.notifications.map((item) =>
        item.id === n.id
          ? {
              ...item,
              title: updated.title || item.title,
              summary: updated.summary || item.summary,
              markdown: updated.markdown_body || updated.summary || item.markdown,
              created_at: updated.created_at || item.created_at,
              read_at: updated.read_at || '',
            }
          : item
      )
    }
    rerollOpenId.value = null
    rerollFeedback.value = ''
    void store.refreshWorkspaceSlices(['memories'])
    store.notify(res?.memory_saved ? '已换全新方向重推，并将你的口味偏好写入长期记忆' : '已换全新方向原位重推')
  } catch (err: any) {
    store.notify(err.message || '重推失败，请稍后重试')
  } finally {
    rerollingId.value = null
  }
}

async function convertInboxToTodo(n: { id: number; title: string; summary: string; markdown: string; priority: string }) {
  if (savedTodoIds.value[n.id]) return
  void markRead(n.id)
  try {
    const res = await BotHubApi.createTodo({
      title: n.title,
      notes: (n.summary || n.markdown || '').slice(0, 260),
      priority: n.priority === 'high' ? 'high' : 'normal',
      status: 'pending',
    })
    if (res?.todo) {
      store.state.todos = [res.todo, ...store.state.todos.filter((x) => x.id !== res.todo.id)]
    }
    savedTodoIds.value[n.id] = true
    void store.refreshWorkspaceSlices(['todos'])
    store.notify('已转入个人待办清单')
  } catch (err: any) {
    store.notify(err.message || '转待办失败')
  }
}

async function saveInboxToNote(n: { id: number; source: string; title: string; summary: string; markdown: string }) {
  if (savedNoteIds.value[n.id]) return
  void markRead(n.id)
  try {
    const res = await BotHubApi.createNote({
      title: n.title,
      category: n.source || 'inbox',
      content: n.markdown || n.summary || n.title,
    })
    if (res?.note) {
      store.state.notes = [res.note, ...store.state.notes.filter((x) => x.id !== res.note.id)]
    }
    savedNoteIds.value[n.id] = true
    void store.refreshWorkspaceSlices(['notes'])
    store.notify(`已归档至工作文档《${n.title}》`)
  } catch (err: any) {
    store.notify(err.message || '存文档失败')
  }
}

async function markRead(id: number) {
  const item = store.state.notifications.find((n) => n.id === id)
  if (!item || item.read_at) return
  item.read_at = new Date().toISOString()
  await BotHubApi.markNotificationRead(id).catch(() => {})
}

async function markAllRead() {
  const now = new Date().toISOString()
  store.state.notifications.forEach((n) => {
    if (!n.read_at) n.read_at = now
  })
  store.notify('全部情报已标记为已读')
  await BotHubApi.markAllNotificationsRead().catch(() => {})
}

async function deleteItem(id: number) {
  const prev = [...store.state.notifications]
  store.state.notifications = prev.filter((n) => n.id !== id)
  try {
    await BotHubApi.deleteNotification(id)
    store.notify('已删除该条通知')
  } catch {
    store.state.notifications = prev
    store.notify('删除通知失败')
  }
}

async function clearReadItems() {
  const prev = [...store.state.notifications]
  const readCount = prev.filter((n) => !!n.read_at).length
  if (readCount === 0) {
    store.notify('当前没有可清理的已读情报')
    return
  }
  store.state.notifications = prev.filter((n) => !n.read_at)
  store.notify(`已清理 ${readCount} 条已读情报`)
  try {
    await BotHubApi.clearReadNotifications()
  } catch {
    store.state.notifications = prev
    store.notify('清理失败')
  }
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
    title="服务通知"
    subtitle="统一接收定时任务汇报、资产到期预警与探针告警"
    max-width="3xl"
    @close="closeModal"
  >
    <template #header-actions>
      <button
        type="button"
        @click="toggleDesktopNotify"
        class="min-h-[44px] sm:min-h-[34px] px-3 py-1.5 rounded-xl border border-line text-xs text-fg-sub hover:bg-hover cursor-pointer"
      >
        桌面弹窗: {{ desktopNotify ? '开' : '关' }}
      </button>
      <button
        type="button"
        @click="markAllRead"
        class="min-h-[44px] sm:min-h-[34px] px-3 py-1.5 rounded-xl border border-line text-xs text-fg-sub hover:text-accent-fg hover:border-accent-border hover:bg-hover cursor-pointer"
      >
        全部已读
      </button>
      <button
        type="button"
        @click="clearReadItems"
        class="min-h-[44px] sm:min-h-[34px] px-3 py-1.5 rounded-xl border border-line text-xs text-fg-muted hover:text-danger hover:border-danger-border hover:bg-hover cursor-pointer"
      >
        清理已读
      </button>
    </template>

    <div class="shrink-0 flex items-center justify-between gap-2 flex-wrap">
      <SegmentedTabs v-model="activeFilter" :items="FILTER_TABS" variant="subtle" aria-label="情报分类筛选" />
      <div class="flex items-center gap-2 flex-wrap">
        <input
          v-model="inboxQuery"
          type="search"
          aria-label="搜索收件箱情报"
          placeholder="搜索情报关键词..."
          @keydown.escape="clearSearchOnEscape($event, inboxQuery, () => { inboxQuery = '' })"
          class="w-36 sm:w-44 min-h-[34px] rounded-xl border border-line bg-canvas px-2.5 text-xs text-fg placeholder:text-fg-muted focus:border-accent"
        />
        <button
          type="button"
          :aria-pressed="onlyUnread"
          @click="onlyUnread = !onlyUnread"
          :class="[
            'min-h-[34px] px-3 rounded-xl border text-xs cursor-pointer transition-colors',
            onlyUnread
              ? 'border-accent-border bg-accent-soft text-accent-fg'
              : 'border-line text-fg-muted hover:text-fg'
          ]"
        >
          仅看未读 ({{ store.unreadInboxCount.value }})
        </button>
      </div>
    </div>

    <div ref="listRef" class="stagger-list flex-1 min-h-0 overflow-y-auto flex flex-col gap-2.5 pr-1">
      <div v-if="filtered.length === 0" class="py-12 text-center text-sm text-fg-muted">
        {{ onlyUnread ? '当前分类下的情报已全部阅读' : '当前分类下暂无情报消息' }}
      </div>
      <article
        v-for="n in filtered"
        :key="n.id"
        @pointermove="trackSpotlight"
        :class="[
          'stagger-item vesper-card rounded-xl p-3.5',
          n.priority === 'high' ? 'border-l-[3px] border-l-rose-400' : !n.read_at ? 'border-l-[3px] border-l-accent' : ''
        ]"
      >
        <div class="flex items-start justify-between gap-2 flex-wrap">
          <div class="flex items-center gap-2 flex-wrap min-w-0">
            <span
              v-if="!n.read_at"
              class="w-2 h-2 rounded-full bg-accent shadow-[0_0_8px_#f2b880] shrink-0"
              title="未读"
            />
            <span
              v-if="n.priority === 'high'"
              class="font-mono text-xs px-1.5 py-0.5 rounded border border-danger-border bg-danger-soft text-danger shrink-0"
            >
              高优
            </span>
            <span class="font-mono text-xs px-2 py-0.5 rounded-lg border border-accent-border bg-accent-soft text-accent-fg shrink-0">
              {{ n.source }}
            </span>
            <strong class="text-sm text-fg truncate">{{ n.title }}</strong>
          </div>
          <span :title="formatTime(n.created_at)" class="font-mono tabular-nums text-xs text-fg-muted shrink-0">
            {{ formatRelativeTime(n.created_at) }}
          </span>
        </div>
        <p class="text-xs text-fg-muted mt-1.5 leading-relaxed">{{ n.summary }}</p>
        <div class="mt-2 flex items-center justify-between gap-2 flex-wrap">
          <details class="vesper-details flex-1 min-w-0" @toggle="(e: any) => e.target?.open && markRead(n.id)">
            <summary class="cursor-pointer inline-flex items-center gap-1 font-mono text-xs text-accent-fg select-none">
              <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" class="details-chevron shrink-0">
                <polyline points="9 18 15 12 9 6" />
              </svg>
              <span>展开全文</span>
            </summary>
            <div @click="handleMarkdownCodeCopy($event)" class="mt-2 pt-2 border-t border-line" v-html="renderSafeMarkdown(n.markdown || n.summary)" />
          </details>
          <div class="flex items-center gap-1.5 flex-wrap shrink-0 self-start">
            <button
              v-if="!n.read_at"
              type="button"
              @click="markRead(n.id)"
              class="min-h-[32px] sm:min-h-[26px] px-2 py-0.5 rounded-lg border border-line hover:border-ok-border text-xs text-fg-sub hover:text-ok cursor-pointer"
            >
              标已读
            </button>
            <button
              type="button"
              :disabled="savedTodoIds[n.id]"
              @click="convertInboxToTodo(n)"
              :class="[
                'min-h-[32px] sm:min-h-[26px] px-2 py-0.5 rounded-lg border text-xs cursor-pointer',
                savedTodoIds[n.id]
                  ? 'border-ok-border bg-ok-soft text-ok cursor-default'
                  : 'border-info-border bg-info-soft text-info',
              ]"
            >
              {{ savedTodoIds[n.id] ? '已建待办' : '转为待办' }}
            </button>
            <button
              type="button"
              :disabled="savedNoteIds[n.id]"
              @click="saveInboxToNote(n)"
              :class="[
                'min-h-[32px] sm:min-h-[26px] px-2 py-0.5 rounded-lg border text-xs cursor-pointer',
                savedNoteIds[n.id]
                  ? 'border-ok-border bg-ok-soft text-ok cursor-default'
                  : 'border-line hover:border-accent-border text-fg-sub hover:text-accent-fg',
              ]"
            >
              {{ savedNoteIds[n.id] ? '已存笔记' : '存为笔记' }}
            </button>
            <button
              type="button"
              :disabled="rerollingId !== null"
              @click="toggleRerollBox(n.id)"
              class="min-h-[32px] sm:min-h-[26px] px-2 py-0.5 rounded-lg border border-line hover:border-amber-400/60 text-xs text-fg-sub hover:text-amber-500 cursor-pointer disabled:opacity-50"
            >
              {{ rerollingId === n.id ? '重推中...' : '换一条重推' }}
            </button>
            <button
              type="button"
              @click="askBotAnalyze(n)"
              class="min-h-[32px] sm:min-h-[26px] px-2.5 py-0.5 rounded-lg border border-accent-border bg-accent-soft hover:bg-accent-soft text-xs text-accent-fg cursor-pointer"
            >
              找 TA 追问
            </button>
            <button
              type="button"
              @click="deleteItem(n.id)"
              class="min-h-[32px] sm:min-h-[26px] px-2 py-0.5 rounded-lg border border-line hover:border-danger-border text-xs text-fg-muted hover:text-danger cursor-pointer"
            >
              删除
            </button>
          </div>
        </div>
        <div
          v-if="rerollOpenId === n.id"
          class="mt-2.5 pt-2.5 border-t border-line flex flex-col sm:flex-row items-stretch sm:items-center gap-2"
        >
          <input
            v-model="rerollFeedback"
            type="text"
            :disabled="rerollingId === n.id"
            placeholder="可选：告诉 TA 哪里不满意或想看什么（留空直接换方向重推并记入避雷偏好）"
            @keydown.enter.prevent="submitReroll(n)"
            class="flex-1 min-h-[34px] rounded-xl border border-line bg-canvas px-3 text-xs text-fg placeholder:text-fg-muted focus:border-accent"
          />
          <div class="flex items-center gap-1.5 shrink-0">
            <button
              type="button"
              :disabled="rerollingId !== null"
              @click="submitReroll(n)"
              class="min-h-[34px] px-3 rounded-xl border border-accent-border bg-accent-soft text-xs font-medium text-accent-fg hover:brightness-105 cursor-pointer disabled:opacity-50"
            >
              {{ rerollingId === n.id ? '正在重新生成...' : '立即原位重推' }}
            </button>
            <button
              type="button"
              :disabled="rerollingId !== null"
              @click="toggleRerollBox(n.id)"
              class="min-h-[34px] px-2.5 rounded-xl border border-line text-xs text-fg-muted hover:text-fg cursor-pointer"
            >
              取消
            </button>
          </div>
        </div>
      </article>
    </div>
  </BaseModal>
</template>
