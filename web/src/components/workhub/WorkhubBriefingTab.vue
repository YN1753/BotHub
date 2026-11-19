<script setup lang="ts">
import { formatTime } from '../../utils/format'
import type { TodoItem, WorkspaceBriefing } from '../../types/bothub'

defineProps<{
  briefing: WorkspaceBriefing | null
  loadingBriefing: boolean
  priorityTodos: TodoItem[]
}>()

const emit = defineEmits<{
  refresh: []
  planToday: []
  goTodos: []
  goNotes: []
  openOps: []
}>()
</script>

<template>
  <div class="flex-1 min-h-0 flex flex-col gap-3 overflow-y-auto pr-1">
    <div class="modal-section flex items-center justify-between gap-3 flex-wrap shrink-0">
      <div>
        <div class="text-sm font-semibold text-fg">
          {{ briefing ? `${briefing.today} (${briefing.weekday})` : '今日全景工作简报' }}
        </div>
        <div class="text-xs font-mono text-fg-muted mt-0.5">
          动态聚合当前待办、14天内到期资产、启用定时任务、未读预警与最近工作文档
        </div>
      </div>
      <div class="flex items-center gap-2">
        <button
          type="button"
          :disabled="loadingBriefing"
          @click="emit('refresh')"
          class="px-3 py-1.5 rounded-xl border border-line text-xs text-fg-sub hover:text-fg cursor-pointer disabled:opacity-50"
        >
          {{ loadingBriefing ? '刷新中...' : '刷新简报' }}
        </button>
        <button
          type="button"
          @click="emit('planToday')"
          class="px-3.5 py-1.5 rounded-xl bg-accent hover:bg-accent-hover text-on-accent font-semibold text-xs cursor-pointer"
        >
          让助手生成今日行动规划
        </button>
      </div>
    </div>

    <div v-if="briefing" class="grid grid-cols-2 sm:grid-cols-4 gap-2.5 shrink-0">
      <button
        type="button"
        @click="emit('goTodos')"
        class="vesper-card rounded-xl p-3 text-left cursor-pointer hover:border-accent-border"
      >
        <div class="text-xs font-mono text-fg-muted">未完待办</div>
        <div class="text-xl font-semibold text-fg mt-0.5">{{ briefing.open_todo_count }}</div>
        <div class="text-xs font-mono mt-0.5" :class="briefing.overdue_todos.length ? 'text-danger' : 'text-ok'">
          逾期 {{ briefing.overdue_todos.length }} · 今日 {{ briefing.due_today_todos.length }}
        </div>
      </button>
      <button
        type="button"
        @click="emit('openOps')"
        class="vesper-card rounded-xl p-3 text-left cursor-pointer hover:border-accent-border"
      >
        <div class="text-xs font-mono text-fg-muted">14天内到期资产</div>
        <div class="text-xl font-semibold mt-0.5" :class="briefing.expiring_assets.length ? 'text-warn' : 'text-fg'">
          {{ briefing.expiring_assets.length }}
        </div>
        <div class="text-xs font-mono text-fg-muted mt-0.5">点击前往资产守望</div>
      </button>
      <button
        type="button"
        @click="emit('openOps')"
        class="vesper-card rounded-xl p-3 text-left cursor-pointer hover:border-accent-border"
      >
        <div class="text-xs font-mono text-fg-muted">运行中例程</div>
        <div class="text-xl font-semibold text-info mt-0.5">{{ briefing.enabled_tasks.length }}</div>
        <div class="text-xs font-mono text-fg-muted mt-0.5">自动值守任务</div>
      </button>
      <button
        type="button"
        @click="emit('goNotes')"
        class="vesper-card rounded-xl p-3 text-left cursor-pointer hover:border-accent-border"
      >
        <div class="text-xs font-mono text-fg-muted">未读情报与告警</div>
        <div class="text-xl font-semibold mt-0.5" :class="briefing.unread_count ? 'text-accent-fg' : 'text-fg'">
          {{ briefing.unread_count }}
        </div>
        <div class="text-xs font-mono text-fg-muted mt-0.5">近况文档 {{ briefing.recent_notes.length }} 份 -></div>
      </button>
    </div>

    <div v-if="briefing?.focus_hints?.length" class="rounded-xl border border-accent-border bg-accent-soft px-3.5 py-2.5 flex flex-col gap-1 shrink-0">
      <div class="text-xs font-mono text-accent-fg">今日建议聚焦</div>
      <ul class="text-xs text-fg flex flex-col gap-0.5 list-disc list-inside">
        <li v-for="(hint, hi) in briefing.focus_hints" :key="hi">{{ hint }}</li>
      </ul>
    </div>

    <div v-if="briefing" class="grid grid-cols-1 md:grid-cols-2 gap-3">
      <div class="vesper-card rounded-xl p-3.5 flex flex-col gap-2">
        <div class="text-xs font-semibold text-accent-fg">今日优先关注待办</div>
        <div v-if="!priorityTodos.length" class="text-xs text-fg-muted">
          当前无逾期或紧急待办
        </div>
        <div
          v-for="td in priorityTodos"
          :key="td.id"
          class="flex items-center justify-between gap-2 text-xs py-1 border-b border-line last:border-0"
        >
          <span class="text-fg truncate">{{ td.title }}</span>
          <span class="font-mono text-xs text-fg-muted shrink-0">{{ td.due_at || td.priority }}</span>
        </div>
      </div>

      <div class="vesper-card rounded-xl p-3.5 flex flex-col gap-2">
        <div class="text-xs font-semibold text-accent-fg">资产续费与未读动态</div>
        <div v-if="!briefing.expiring_assets.length && !briefing.unread_alerts.length" class="text-xs text-fg-muted">
          资产与收件箱均处于健康状态
        </div>
        <div
          v-for="a in briefing.expiring_assets.slice(0, 4)"
          :key="a.id"
          class="flex items-center justify-between gap-2 text-xs py-1 border-b border-line"
        >
          <span class="text-warn truncate">资产: {{ a.name }}</span>
          <span class="font-mono text-xs text-warn shrink-0">剩余 {{ a.days_left }} 天</span>
        </div>
        <div
          v-for="al in briefing.unread_alerts.slice(0, 4)"
          :key="al.id"
          class="flex items-center justify-between gap-2 text-xs py-1 border-b border-line last:border-0"
        >
          <span class="text-fg truncate">{{ al.title }}</span>
          <span class="font-mono text-xs text-fg-muted shrink-0">{{ formatTime(al.created_at) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
