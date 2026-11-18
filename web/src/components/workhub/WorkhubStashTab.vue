<script setup lang="ts">
import { ref, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import { BotHubApi } from '../../api/client'
import { pickDefaultWorkerBot } from '../../utils/botLook'
import { formatTime } from '../../utils/format'
import type { StashItem, StashTagCount } from '../../types/bothub'
import SegmentedTabs from '../SegmentedTabs.vue'

const emit = defineEmits<{
  closeModal: []
}>()

const store = useAppStore()

const STASH_FILTERS = [
  { id: 'unread', label: '未读干货' },
  { id: 'all', label: '全部' },
  { id: 'pinned', label: '置顶' },
  { id: 'note', label: '纯闪念' },
  { id: 'archived', label: '已归档' },
]

type FilterId = 'unread' | 'all' | 'pinned' | 'note' | 'archived'

const activeFilter = ref<FilterId>('unread')
const searchQuery = ref('')
const activeTag = ref('')
const rawInput = ref('')
const userNoteInput = ref('')
const showNoteField = ref(false)
const capturing = ref(false)
const loadingList = ref(false)
const captureInputRef = ref<HTMLTextAreaElement | null>(null)

const remoteItems = ref<StashItem[] | null>(null)
const remoteTags = ref<StashTagCount[]>([])
const expandedIds = ref<Record<string, boolean>>({})

// Inline editing state
const editingId = ref<string | null>(null)
const editTitle = ref('')
const editUserNote = ref('')
const editTags = ref('')
const savingEdit = ref(false)

let searchTimer: number | null = null
let activeAbort: AbortController | null = null

const detectedInputKind = computed(() => {
  const txt = rawInput.value.trim()
  if (!txt) return ''
  const urlMatch = txt.match(/https?:\/\/[^\s<>"'`\]）】》]+/i)
  if (!urlMatch) return '纯文字闪念'
  const lower = urlMatch[0].toLowerCase()
  if (lower.includes('bilibili.com') || lower.includes('b23.tv')) return 'Bilibili 视频'
  if (lower.includes('github.com')) return 'GitHub 仓库'
  return '网页外链'
})

const topTags = computed<StashTagCount[]>(() => {
  if (remoteTags.value.length > 0) return remoteTags.value
  return store.state.stashTags || []
})

const needsRemoteQuery = computed(() => {
  return Boolean(
    searchQuery.value.trim() ||
      activeTag.value ||
      activeFilter.value === 'archived'
  )
})

const displayedItems = computed<StashItem[]>(() => {
  if (remoteItems.value !== null && needsRemoteQuery.value) {
    return remoteItems.value
  }
  const list = store.state.stash || []
  return list.filter((it) => {
    if (activeFilter.value === 'unread') {
      return it.status !== 'archived' && it.read_count === 0
    }
    if (activeFilter.value === 'pinned') {
      return it.status !== 'archived' && it.pinned
    }
    if (activeFilter.value === 'note') {
      return it.status !== 'archived' && it.kind === 'note'
    }
    if (activeFilter.value === 'archived') {
      return it.status === 'archived'
    }
    return it.status !== 'archived'
  })
})

async function fetchFilteredStash() {
  if (activeAbort) {
    activeAbort.abort()
    activeAbort = null
  }
  if (!needsRemoteQuery.value) {
    remoteItems.value = null
    return
  }
  const ctrl = new AbortController()
  activeAbort = ctrl
  loadingList.value = true
  try {
    const res = await BotHubApi.listStash(
      {
        q: searchQuery.value.trim(),
        filter: activeFilter.value,
        tag: activeTag.value,
        limit: 100,
      },
      ctrl.signal
    )
    if (activeAbort === ctrl) {
      remoteItems.value = res.items || []
      if (res.top_tags) remoteTags.value = res.top_tags
    }
  } catch (err: any) {
    if (err?.name !== 'AbortError') {
      store.notify(err.message || '检索收藏匣失败')
    }
  } finally {
    if (activeAbort === ctrl) {
      loadingList.value = false
      activeAbort = null
    }
  }
}

function scheduleSearch(delayMs = 200) {
  if (searchTimer !== null) {
    window.clearTimeout(searchTimer)
  }
  searchTimer = window.setTimeout(() => {
    searchTimer = null
    void fetchFilteredStash()
  }, delayMs)
}

watch([activeFilter, activeTag], () => {
  void fetchFilteredStash()
})

watch(searchQuery, () => {
  scheduleSearch(220)
})

// Re-sync active remote search whenever SSE assistant_update("stash") updates store.state.stash
watch(
  () => store.state.stash,
  () => {
    if (needsRemoteQuery.value) {
      void fetchFilteredStash()
    }
  }
)

onBeforeUnmount(() => {
  if (searchTimer !== null) window.clearTimeout(searchTimer)
  if (activeAbort) activeAbort.abort()
})

function kindBadgeMeta(kind: string): { label: string; cls: string } {
  switch (kind) {
    case 'bilibili':
      return {
        label: 'Bilibili',
        cls: 'border-rose-400/35 bg-rose-500/12 text-rose-200',
      }
    case 'github':
      return {
        label: 'GitHub',
        cls: 'border-indigo-400/35 bg-indigo-500/14 text-indigo-200',
      }
    case 'note':
      return {
        label: '闪念',
        cls: 'border-[#f2b880]/40 bg-[#f2b880]/12 text-[#f7cba0]',
      }
    default:
      return {
        label: 'Web',
        cls: 'border-cyan-400/35 bg-cyan-500/12 text-cyan-200',
      }
  }
}

function parseTagList(tags?: string): string[] {
  if (!tags) return []
  return tags
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean)
}

function toggleExpand(id: string) {
  expandedIds.value[id] = !expandedIds.value[id]
}

function toggleTagFilter(tag: string) {
  activeTag.value = activeTag.value.toLowerCase() === tag.toLowerCase() ? '' : tag
}

function handleSearchEscape(e: KeyboardEvent) {
  if (!searchQuery.value.trim() && !activeTag.value) return
  e.preventDefault()
  e.stopPropagation()
  searchQuery.value = ''
  activeTag.value = ''
}

async function handleCapture() {
  const input = rawInput.value.trim()
  if (!input || capturing.value) return
  capturing.value = true
  try {
    const res = await BotHubApi.createStash({
      raw_input: input,
      user_note: userNoteInput.value.trim(),
    })
    rawInput.value = ''
    userNoteInput.value = ''
    showNoteField.value = false
    if (activeFilter.value === 'archived') {
      activeFilter.value = 'unread'
    }
    await store.refreshWorkspaceSlices(['stash'])
    if (needsRemoteQuery.value) await fetchFilteredStash()
    store.notify(
      res.deduplicated
        ? '该链接已在收藏匣中，已合并最新批注'
        : '已秒收至收藏匣，后台正在异步提炼干货要点'
    )
    nextTick(() => captureInputRef.value?.focus())
  } catch (err: any) {
    store.notify(err.message || '收藏投喂失败')
  } finally {
    capturing.value = false
  }
}

async function pasteFromClipboard() {
  try {
    const text = (await navigator.clipboard?.readText())?.trim() || ''
    if (!text) {
      store.notify('剪贴板为空')
      return
    }
    rawInput.value = text
    nextTick(() => captureInputRef.value?.focus())
  } catch {
    store.notify('浏览器未授权读取剪贴板，请直接 Ctrl/Cmd+V 粘贴')
  }
}

async function openSourceAndMarkRead(item: StashItem) {
  if (item.url) {
    window.open(item.url, '_blank', 'noopener,noreferrer')
  }
  item.read_count = (item.read_count || 0) + 1
  try {
    await BotHubApi.updateStash(item.id, { mark_read: true })
    void store.refreshWorkspaceSlices(['stash'])
    if (needsRemoteQuery.value) void fetchFilteredStash()
  } catch {
    // ignore background counter error
  }
}

async function togglePin(item: StashItem) {
  const prevPin = item.pinned
  const nextPin = !prevPin
  item.pinned = nextPin
  store.notify(nextPin ? '已置顶该条收藏' : '已取消置顶')
  try {
    await BotHubApi.updateStash(item.id, { pinned: nextPin })
    void store.refreshWorkspaceSlices(['stash'])
    if (needsRemoteQuery.value) void fetchFilteredStash()
  } catch (err: any) {
    item.pinned = prevPin
    store.notify(err.message || '置顶操作失败')
  }
}

async function convertToTodo(item: StashItem) {
  const noteParts: string[] = []
  if (item.ai_tldr) noteParts.push(`核心结论：${item.ai_tldr}`)
  if (item.url) noteParts.push(`原链接：${item.url}`)
  if (item.user_note) noteParts.push(`批注：${item.user_note}`)
  try {
    await BotHubApi.createTodo({
      bot_id: store.activeBot.value?.id || '',
      title: item.title || item.raw_input.slice(0, 60),
      notes: noteParts.join('\n'),
      priority: 'normal',
      tags: item.tags || '收藏转化',
      status: 'pending',
    })
    await store.refreshWorkspaceSlices(['todos'])
    store.notify('已一键转化为个人待办事项')
  } catch (err: any) {
    store.notify(err.message || '转化为待办失败')
  }
}

async function sendToBotChat(item: StashItem) {
  const targetBotId = pickDefaultWorkerBot(store.state.bots, store.activeBot.value)
  if (!targetBotId) {
    store.notify('请先创建一个 Bot')
    return
  }
  const lines = [
    `请结合我收藏匣中的这条内容（ID: ${item.id}）继续深入分析或解答：`,
    `标题：${item.title || '未命名'}`,
    item.url ? `链接：${item.url}` : '',
    item.ai_tldr ? `AI 结论：${item.ai_tldr}` : '',
    item.points?.length ? `要点：\n- ${item.points.join('\n- ')}` : '',
    item.user_note ? `我的批注：${item.user_note}` : '',
  ].filter(Boolean)
  emit('closeModal')
  await store.jumpToBot(targetBotId, lines.join('\n'))
}

async function triggerReprocess(item: StashItem) {
  try {
    await BotHubApi.reprocessStash(item.id)
    await store.refreshWorkspaceSlices(['stash'])
    if (needsRemoteQuery.value) await fetchFilteredStash()
    store.notify('已重新排入后台抓取与 AI 提炼队列')
  } catch (err: any) {
    store.notify(err.message || '重提炼触发失败')
  }
}

async function toggleArchive(item: StashItem) {
  const prevStatus = item.status
  const nextStatus = prevStatus === 'archived' ? 'ready' : 'archived'
  item.status = nextStatus
  store.notify(nextStatus === 'archived' ? '已归档收纳' : '已恢复至活跃收藏匣')
  try {
    await BotHubApi.updateStash(item.id, { status: nextStatus })
    void store.refreshWorkspaceSlices(['stash'])
    if (needsRemoteQuery.value) void fetchFilteredStash()
  } catch (err: any) {
    item.status = prevStatus
    store.notify(err.message || '归档状态更新失败')
  }
}

async function removeStash(item: StashItem) {
  const ok = await store.openConfirm({
    title: '删除收藏条目',
    message: `确定彻底删除「${item.title || '该条收藏'}」吗？`,
    confirmText: '彻底删除',
    danger: true,
  })
  if (!ok) return
  const prevStash = [...store.state.stash]
  const prevRemote = remoteItems.value ? [...remoteItems.value] : null
  store.state.stash = store.state.stash.filter((x) => x.id !== item.id)
  if (remoteItems.value) {
    remoteItems.value = remoteItems.value.filter((x) => x.id !== item.id)
  }
  if (editingId.value === item.id) editingId.value = null
  store.notify('已删除收藏条目')
  try {
    await BotHubApi.deleteStash(item.id)
    void store.refreshWorkspaceSlices(['stash'])
    if (needsRemoteQuery.value) void fetchFilteredStash()
  } catch (err: any) {
    store.state.stash = prevStash
    if (prevRemote !== null) remoteItems.value = prevRemote
    store.notify(err.message || '删除收藏失败')
  }
}

function startEdit(item: StashItem) {
  editingId.value = item.id
  editTitle.value = item.title
  editUserNote.value = item.user_note || ''
  editTags.value = item.tags || ''
}

async function saveEdit(item: StashItem) {
  if (savingEdit.value) return
  savingEdit.value = true
  const snapshot = { title: item.title, user_note: item.user_note, tags: item.tags }
  item.title = editTitle.value.trim()
  item.user_note = editUserNote.value.trim()
  item.tags = editTags.value.trim()
  editingId.value = null
  try {
    await BotHubApi.updateStash(item.id, {
      title: item.title,
      user_note: item.user_note,
      tags: item.tags,
    })
    void store.refreshWorkspaceSlices(['stash'])
    if (needsRemoteQuery.value) void fetchFilteredStash()
    store.notify('收藏信息已更新')
  } catch (err: any) {
    Object.assign(item, snapshot)
    store.notify(err.message || '更新失败')
  } finally {
    savingEdit.value = false
  }
}
</script>

<template>
  <div class="flex-1 min-h-0 flex flex-col gap-3">
    <!-- Quick Capture Bar -->
    <form @submit.prevent="handleCapture" class="modal-section flex flex-col gap-2 shrink-0">
      <div class="flex items-start gap-2">
        <div class="flex-1 min-w-0 relative">
          <textarea
            ref="captureInputRef"
            v-model="rawInput"
            rows="1"
            aria-label="极速投喂链接、B站分享文案或纯文字闪念"
            placeholder="粘贴 URL、B 站分享文案（如【UP主】标题 https://b23.tv/...）或随手闪念，回车秒收..."
            @keydown.enter.exact.prevent="handleCapture"
            class="w-full min-h-[38px] max-h-28 [field-sizing:content] resize-none rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-xs text-white placeholder-slate-500 focus:border-[#f2b880]/60 leading-relaxed"
          ></textarea>
        </div>
        <button
          type="button"
          @click="pasteFromClipboard"
          class="min-h-[38px] px-2.5 rounded-xl border border-[#f2b880]/30 bg-[#f2b880]/10 text-[11px] text-[#f7cba0] hover:bg-[#f2b880]/20 cursor-pointer shrink-0"
        >
          粘贴
        </button>
        <button
          type="button"
          :aria-pressed="showNoteField || Boolean(userNoteInput)"
          @click="showNoteField = !showNoteField"
          class="min-h-[38px] px-2.5 rounded-xl border border-[#fff8ee]/12 bg-[#050811]/70 text-[11px] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f7cba0] cursor-pointer shrink-0"
        >
          {{ showNoteField || userNoteInput ? '收起批注' : '+ 批注' }}
        </button>
        <button
          type="submit"
          :disabled="!rawInput.trim() || capturing"
          class="min-h-[38px] px-3.5 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50 shrink-0"
        >
          {{ capturing ? '入库中...' : '秒收提炼' }}
        </button>
      </div>

      <div v-if="showNoteField || userNoteInput" class="flex items-center gap-2">
        <input
          v-model="userNoteInput"
          type="text"
          aria-label="随手批注"
          placeholder="附带一句个人批注或收藏原因（回车一并投喂）..."
          @keydown.enter.exact.prevent="handleCapture"
          class="flex-1 min-h-[34px] rounded-lg border border-[#fff8ee]/10 bg-[#050811] px-3 text-xs text-slate-200 placeholder-slate-500 focus:border-[#f2b880]/55"
        />
      </div>

      <div class="flex items-center justify-between gap-2 flex-wrap text-[11px] text-slate-500">
        <span>支持外链智能去重、B 站/GitHub 专项解析与 3 日后早报防吃灰重温；Shift+Enter 换行记长闪念</span>
        <span
          v-if="detectedInputKind"
          class="px-2 py-0.5 rounded-full font-mono text-[10px] border border-[#f2b880]/35 bg-[#f2b880]/10 text-[#f7cba0]"
        >
          识别为: {{ detectedInputKind }}
        </span>
      </div>
    </form>

    <!-- Search, View Switcher & Dynamic Tag Pills -->
    <div class="flex flex-col gap-2 shrink-0">
      <div class="flex items-center justify-between gap-2 flex-wrap">
        <SegmentedTabs v-model="activeFilter" :items="STASH_FILTERS" variant="subtle" aria-label="收藏匣视图切换" />
        <div class="flex items-center gap-2 flex-1 sm:flex-initial justify-end">
          <input
            v-model="searchQuery"
            type="search"
            aria-label="全文检索收藏与闪念"
            placeholder="搜标题 / 干货结论 / 正文快照..."
            @keydown.escape="handleSearchEscape"
            class="w-full sm:w-56 min-h-[32px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 text-xs text-white placeholder-slate-500 focus:border-[#f2b880]/55"
          />
        </div>
      </div>

      <div v-if="topTags.length > 0" class="flex items-center gap-1.5 flex-wrap">
        <span class="font-mono text-[10.5px] text-slate-500">高频标签:</span>
        <button
          v-for="tg in topTags"
          :key="tg.tag"
          type="button"
          :aria-pressed="activeTag.toLowerCase() === tg.tag.toLowerCase()"
          @click="toggleTagFilter(tg.tag)"
          :class="[
            'px-2 py-0.5 rounded-full font-mono text-[10.5px] border cursor-pointer transition-colors',
            activeTag.toLowerCase() === tg.tag.toLowerCase()
              ? 'border-[#f2b880]/60 bg-[#f2b880]/18 text-[#f7cba0]'
              : 'border-[#fff8ee]/10 bg-[#050811]/60 text-slate-400 hover:border-[#fff8ee]/25 hover:text-slate-200',
          ]"
        >
          {{ tg.tag }} ({{ tg.count }})
        </button>
        <button
          v-if="activeTag || searchQuery"
          type="button"
          @click="activeTag = ''; searchQuery = ''"
          class="px-2 py-0.5 rounded-full text-[10.5px] text-slate-400 hover:text-[#f2b880] cursor-pointer"
        >
          清除筛选
        </button>
      </div>
    </div>

    <!-- Stash Cards List -->
    <div class="flex-1 min-h-0 overflow-y-auto flex flex-col gap-2 pr-1">
      <div
        v-if="loadingList && displayedItems.length === 0"
        class="py-10 text-center text-xs text-slate-400"
      >
        正在执行全文检索...
      </div>

      <div
        v-else-if="displayedItems.length === 0"
        class="py-12 text-center flex flex-col items-center gap-2 text-xs text-slate-500"
      >
        <span>
          {{
            searchQuery || activeTag
              ? '未检索到匹配的收藏或闪念条目'
              : activeFilter === 'unread'
                ? '当前没有待消化的未读干货；上方粘贴链接或手机分享即可秒收提炼'
                : '当前视图暂无条目'
          }}
        </span>
        <button
          v-if="searchQuery || activeTag || activeFilter !== 'all'"
          type="button"
          @click="searchQuery = ''; activeTag = ''; activeFilter = 'all'"
          class="px-3 py-1 rounded-full border border-[#fff8ee]/12 text-slate-300 hover:text-[#f2b880] cursor-pointer"
        >
          查看全部收藏
        </button>
      </div>

      <TransitionGroup v-else name="vesper-list" tag="div" class="flex flex-col gap-2">
        <article
          v-for="item in displayedItems"
          :key="item.id"
          class="luxury-card rounded-xl px-3.5 py-2.5 flex flex-col gap-2"
        >
          <!-- Card Header: Badges + Title + Quick Actions -->
          <div class="flex items-start justify-between gap-3 flex-wrap sm:flex-nowrap">
            <div
              class="min-w-0 flex-1 cursor-pointer"
              role="button"
              tabindex="0"
              :aria-expanded="Boolean(expandedIds[item.id])"
              @click="toggleExpand(item.id)"
              @keydown.enter.prevent="toggleExpand(item.id)"
              @keydown.space.prevent="toggleExpand(item.id)"
            >
              <div class="flex items-center gap-1.5 flex-wrap">
                <span
                  :class="[
                    'px-1.5 py-0.5 rounded font-mono text-[10px] border shrink-0',
                    kindBadgeMeta(item.kind).cls,
                  ]"
                >
                  {{ kindBadgeMeta(item.kind).label }}
                </span>
                <span
                  v-if="item.pinned"
                  class="px-1.5 py-0.5 rounded font-mono text-[10px] border border-[#f2b880]/45 bg-[#f2b880]/15 text-[#f7cba0]"
                >
                  置顶
                </span>
                <span
                  v-if="item.status === 'processing' || item.status === 'pending'"
                  class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded font-mono text-[10px] border border-cyan-400/35 bg-cyan-500/10 text-cyan-200"
                >
                  <span>AI 提炼中</span>
                  <span class="thinking-dots" aria-hidden="true"><span></span><span></span><span></span></span>
                </span>
                <span
                  v-else-if="item.status === 'failed'"
                  class="px-1.5 py-0.5 rounded font-mono text-[10px] border border-amber-400/40 bg-amber-500/12 text-amber-200"
                >
                  待重提炼
                </span>
                <span
                  v-if="item.read_count === 0 && item.status !== 'archived'"
                  class="px-1.5 py-0.5 rounded font-mono text-[10px] border border-emerald-400/30 bg-emerald-500/10 text-emerald-200"
                >
                  未读
                </span>
                <span v-else-if="item.read_count > 0" class="font-mono text-[10px] text-slate-500">
                  已阅 {{ item.read_count }} 次
                </span>
                <span v-if="item.domain" class="font-mono text-[10.5px] text-slate-500 truncate max-w-[180px]">
                  {{ item.domain }}
                </span>
                <span class="font-mono text-[10px] text-slate-500">
                  {{ formatTime(item.created_at) }}
                </span>
              </div>

              <h4 class="text-sm font-medium text-white mt-1 leading-snug break-words">
                {{ item.title || item.raw_input.slice(0, 72) }}
              </h4>

              <p
                v-if="item.ai_tldr"
                class="text-xs text-slate-300 mt-1 leading-relaxed"
              >
                {{ item.ai_tldr }}
              </p>
              <p
                v-else-if="item.status === 'processing'"
                class="text-xs text-slate-400 mt-1"
              >
                正在安全抓取来源快照并提炼核心结论与要点...
              </p>
              <p
                v-else-if="item.error_msg"
                class="text-[11px] text-amber-300/90 mt-1"
              >
                {{ item.error_msg }}
              </p>

              <div v-if="parseTagList(item.tags).length > 0" class="flex items-center gap-1 flex-wrap mt-1.5">
                <span
                  v-for="tg in parseTagList(item.tags)"
                  :key="tg"
                  class="px-1.5 py-0.2 rounded font-mono text-[10px] border border-[#fff8ee]/10 bg-[#050811]/70 text-slate-400"
                >
                  {{ tg }}
                </span>
              </div>
            </div>

            <!-- Action Buttons -->
            <div class="flex items-center gap-1.5 flex-wrap shrink-0">
              <button
                type="button"
                @click="toggleExpand(item.id)"
                class="px-2 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-white cursor-pointer"
              >
                {{ expandedIds[item.id] ? '收起' : '要点' }}
              </button>
              <button
                type="button"
                @click="openSourceAndMarkRead(item)"
                class="px-2.5 py-1 rounded-lg border border-[#f2b880]/40 bg-[#f2b880]/12 text-[11px] text-[#f7cba0] hover:bg-[#f2b880]/22 cursor-pointer"
              >
                {{ item.url ? '打开原链' : '标为已阅' }}
              </button>
              <button
                type="button"
                @click="convertToTodo(item)"
                class="px-2 py-1 rounded-lg border border-cyan-400/30 bg-cyan-500/10 text-[11px] text-cyan-200 hover:bg-cyan-500/20 cursor-pointer"
              >
                转待办
              </button>
              <button
                type="button"
                @click="togglePin(item)"
                class="px-2 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
              >
                {{ item.pinned ? '取消置顶' : '置顶' }}
              </button>
              <button
                type="button"
                @click="triggerReprocess(item)"
                :disabled="item.status === 'processing'"
                class="px-2 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-white cursor-pointer disabled:opacity-40"
              >
                重提炼
              </button>
              <button
                type="button"
                @click="toggleArchive(item)"
                class="px-2 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-400 hover:text-slate-200 cursor-pointer"
              >
                {{ item.status === 'archived' ? '取消归档' : '归档' }}
              </button>
              <button
                type="button"
                @click="removeStash(item)"
                class="px-2 py-1 rounded-lg border border-rose-400/30 text-[11px] text-rose-300 hover:bg-rose-500/15 cursor-pointer"
              >
                删
              </button>
            </div>
          </div>

          <!-- Expanded Detail Drawer: Key Points + Personal Note + Excerpt + Edit -->
          <div
            v-if="expandedIds[item.id]"
            class="pt-2.5 mt-0.5 border-t border-[#fff8ee]/10 flex flex-col gap-2.5 text-xs"
          >
            <div v-if="item.points && item.points.length > 0" class="flex flex-col gap-1">
              <span class="font-mono text-[10.5px] text-[#f2b880]">AI 核心干货要点</span>
              <ul class="list-disc pl-4 space-y-1 text-slate-200 leading-relaxed">
                <li v-for="(pt, idx) in item.points" :key="idx">{{ pt }}</li>
              </ul>
            </div>

            <div
              v-if="item.user_note"
              class="rounded-lg border border-[#f2b880]/25 bg-[#f2b880]/[0.06] px-3 py-2 text-slate-200 whitespace-pre-line"
            >
              <span class="font-mono text-[10px] text-[#f7cba0] block mb-0.5">个人批注</span>
              {{ item.user_note }}
            </div>

            <details v-if="item.content_excerpt" class="vesper-details rounded-lg border border-[#fff8ee]/8 bg-[#050811]/75">
              <summary class="px-3 py-1.5 cursor-pointer flex items-center justify-between text-[11px] text-slate-400 hover:text-slate-200">
                <span>查看抓取正文 / 简介 / 字幕快照 ({{ item.content_excerpt.length }} 字)</span>
                <svg class="details-chevron w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                  <polyline points="9 18 15 12 9 6" />
                </svg>
              </summary>
              <div class="px-3 pb-2.5 pt-1 text-[11.5px] text-slate-300 whitespace-pre-line max-h-56 overflow-y-auto leading-relaxed border-t border-[#fff8ee]/6">
                {{ item.content_excerpt }}
              </div>
            </details>

            <div class="flex items-center justify-between gap-2 flex-wrap">
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  @click="editingId === item.id ? (editingId = null) : startEdit(item)"
                  class="px-2.5 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-white cursor-pointer"
                >
                  {{ editingId === item.id ? '取消编辑' : '编辑标题 / 批注 / 标签' }}
                </button>
                <button
                  type="button"
                  @click="sendToBotChat(item)"
                  class="px-2.5 py-1 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
                >
                  发给 Bot 深聊
                </button>
              </div>
              <a
                v-if="item.url"
                :href="item.url"
                target="_blank"
                rel="noopener noreferrer"
                @click="BotHubApi.updateStash(item.id, { mark_read: true }).catch(() => {})"
                class="font-mono text-[10.5px] text-slate-400 hover:text-[#f2b880] underline truncate max-w-xs"
              >
                {{ item.url }}
              </a>
            </div>

            <!-- Inline Editor -->
            <div v-if="editingId === item.id" class="grid grid-cols-1 sm:grid-cols-12 gap-2 pt-1">
              <input
                v-model="editTitle"
                type="text"
                placeholder="收藏标题"
                @keydown.enter.prevent="saveEdit(item)"
                @keydown.escape.stop="editingId = null"
                class="sm:col-span-7 min-h-[34px] rounded-lg border border-[#f2b880]/40 bg-[#050811] px-2.5 text-xs text-white"
              />
              <input
                v-model="editTags"
                type="text"
                placeholder="标签 (逗号分隔，如 AI,架构)"
                @keydown.enter.prevent="saveEdit(item)"
                @keydown.escape.stop="editingId = null"
                class="sm:col-span-5 min-h-[34px] rounded-lg border border-[#fff8ee]/12 bg-[#050811] px-2.5 text-xs text-white"
              />
              <input
                v-model="editUserNote"
                type="text"
                placeholder="个人批注..."
                @keydown.enter.prevent="saveEdit(item)"
                @keydown.escape.stop="editingId = null"
                class="sm:col-span-10 min-h-[34px] rounded-lg border border-[#fff8ee]/12 bg-[#050811] px-2.5 text-xs text-white"
              />
              <button
                type="button"
                :disabled="savingEdit"
                @click="saveEdit(item)"
                class="sm:col-span-2 min-h-[34px] rounded-lg bg-[#f2b880] text-[#070a13] font-semibold text-xs cursor-pointer"
              >
                保存
              </button>
            </div>
          </div>
        </article>
      </TransitionGroup>
    </div>
  </div>
</template>
