<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { pickDefaultWorkerBot } from '../utils/botLook'
import { lazyPanelChunk } from '../utils/lazyComponent'
import { resolveTabSlideTransition, type DirectionalTransitionName } from '../utils/motion'
import type { TodoItem, WorkspaceBriefing } from '../types/bothub'
import BaseModal from './BaseModal.vue'
import SegmentedTabs from './SegmentedTabs.vue'
import WorkhubTodosTab from './workhub/WorkhubTodosTab.vue'

const WorkhubBriefingTab = lazyPanelChunk(() => import('./workhub/WorkhubBriefingTab.vue'))
const WorkhubNotesTab = lazyPanelChunk(() => import('./workhub/WorkhubNotesTab.vue'))
const WorkhubStashTab = lazyPanelChunk(() => import('./workhub/WorkhubStashTab.vue'))

const store = useAppStore()

const TAB_ITEMS = [
  { id: 'todos', label: '待办与行动清单' },
  { id: 'notes', label: '工作文档与交付区' },
  { id: 'stash', label: '收藏与闪念匣' },
  { id: 'briefing', label: '今日简报' },
]

const activeTab = computed({
  get: () => store.state.workhubTab,
  set: (v: 'todos' | 'notes' | 'stash' | 'briefing') => {
    store.state.workhubTab = v
  },
})

const workhubTabTransition = ref<DirectionalTransitionName>('vesper-slide-forward')
watch(activeTab, (next, prev) => {
  workhubTabTransition.value = resolveTabSlideTransition(next, prev || 'todos', ['todos', 'notes', 'stash', 'briefing'])
})

const briefing = ref<WorkspaceBriefing | null>(null)
const loadingBriefing = ref(false)

const briefingPriorityTodos = computed(() => {
  if (!briefing.value) return []
  const seen = new Set<string>()
  const merged = [
    ...briefing.value.overdue_todos,
    ...briefing.value.due_today_todos,
    ...briefing.value.high_priority_todos,
  ]
  const out: TodoItem[] = []
  for (const td of merged) {
    if (!seen.has(td.id)) {
      seen.add(td.id)
      out.push(td)
      if (out.length >= 6) break
    }
  }
  return out
})

async function loadBriefing() {
  if (loadingBriefing.value) return
  loadingBriefing.value = true
  try {
    const res = await BotHubApi.getBriefing()
    briefing.value = res.briefing
  } catch (err: any) {
    store.notify(err.message || '加载今日简报失败')
  } finally {
    loadingBriefing.value = false
  }
}

watch(
  () => [store.state.showWorkhubModal, activeTab.value] as const,
  ([open, tab]) => {
    if (!open) return
    if (tab === 'briefing' || !briefing.value) {
      void loadBriefing()
    }
  }
)

function closeModal() {
  store.state.showWorkhubModal = false
}

async function planTodayWithBot() {
  const targetBotId = pickDefaultWorkerBot(store.state.bots, store.activeBot.value)
  if (!targetBotId) {
    store.notify('请先创建一个可执行 Bot')
    return
  }
  const prompt = '请调用 workspace_briefing 汇总我今天的全部待办、临期资产与未读通知，排出高优推进顺序并给出可直接开干的行动方案。'
  closeModal()
  await store.jumpToBot(targetBotId, prompt)
}

function openAssetOpsFromBriefing() {
  closeModal()
  store.state.workspace = 'ops'
}
</script>

<template>
  <BaseModal
    :open="store.state.showWorkhubModal"
    title="个人行动台 · 待办、文档、收藏匣与今日简报"
    subtitle="助手帮你拆解落地的待办事项、Markdown 交付文档、智能收藏闪念匣与今日全局聚焦面板"
    max-width="3xl"
    @close="closeModal"
  >
    <div class="flex items-center justify-between gap-2 flex-wrap shrink-0">
      <SegmentedTabs v-model="activeTab" :items="TAB_ITEMS" aria-label="行动台视图切换" />
      <span class="font-mono text-[11px] text-slate-400">
        待办 {{ store.pendingTodoCount.value }} 项 · 收藏 {{ store.unreadStashCount.value }} 未读 · 文档 {{ store.state.notes.length }} 份
      </span>
    </div>

    <Transition :name="workhubTabTransition" mode="out-in">
      <WorkhubTodosTab v-if="activeTab === 'todos'" key="todos" @close-modal="closeModal" />

      <WorkhubNotesTab v-else-if="activeTab === 'notes'" key="notes" @close-modal="closeModal" />

      <WorkhubStashTab v-else-if="activeTab === 'stash'" key="stash" @close-modal="closeModal" />

      <WorkhubBriefingTab
        v-else
        key="briefing"
        :briefing="briefing"
        :loading-briefing="loadingBriefing"
        :priority-todos="briefingPriorityTodos"
        @refresh="loadBriefing"
        @plan-today="planTodayWithBot"
        @go-todos="activeTab = 'todos'"
        @go-notes="activeTab = 'notes'"
        @open-ops="openAssetOpsFromBriefing"
      />
    </Transition>
  </BaseModal>
</template>
