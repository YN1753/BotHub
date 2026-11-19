<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { resolveGroupMemberBots } from '../utils/botLook'
import { clearSearchOnEscape, trackSpotlight } from '../utils/topLayer'
import type { Bot } from '../types/bothub'
import BaseModal from './BaseModal.vue'
import BotAvatar from './BotAvatar.vue'
import BotGroupAvatar from './BotGroupAvatar.vue'

const store = useAppStore()
const selectedId = ref('')
const note = ref('')
const targetQuery = ref('')
const sending = ref(false)

const targets = computed(() =>
  store.state.bots.filter((b) => b.id !== store.state.activeBotId)
)

const filteredTargets = computed(() => {
  const q = targetQuery.value.trim().toLowerCase()
  if (!q) return targets.value
  return targets.value.filter(
    (b) => b.name.toLowerCase().includes(q) || (b.role_title || '').toLowerCase().includes(q)
  )
})

function resolveGroupMembers(b: Bot): Bot[] {
  return resolveGroupMemberBots(b, store.state.bots)
}

watch(
  () => store.state.forwardingMessage,
  (val) => {
    if (!val) return
    note.value = ''
    targetQuery.value = ''
    selectedId.value = targets.value[0]?.id || ''
  }
)

function closeModal() {
  store.state.forwardingMessage = null
}

async function submitForward(jumpAfter: boolean) {
  const payload = store.state.forwardingMessage
  if (!payload || !selectedId.value || sending.value) return
  sending.value = true
  try {
    await BotHubApi.forwardBotMessage(selectedId.value, {
      from_name: payload.fromName,
      content: payload.content,
      note: note.value.trim(),
    })
    const targetId = selectedId.value
    const targetName = store.state.bots.find((b) => b.id === targetId)?.name || targetId
    closeModal()
    store.notify(`已转发至「${targetName}」`)
    void store.refreshWorkspaceSlices(['bots'])
    if (jumpAfter) {
      await store.jumpToBot(targetId)
    }
  } catch (err: any) {
    store.notify(err.message || '转发失败')
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <BaseModal
    :open="Boolean(store.state.forwardingMessage)"
    title="转发消息到其他会话"
    subtitle="像 QQ/微信一样跨频道传递参考上下文，可附带一句话说明"
    max-width="lg"
    @close="closeModal"
  >
    <div v-if="store.state.forwardingMessage" class="flex flex-col gap-3 min-h-0">
      <div class="modal-section text-xs text-fg-sub">
        <div class="font-mono text-xs text-accent-fg mb-1">
          来自「{{ store.state.forwardingMessage.fromName }}」的消息摘要：
        </div>
        <p class="line-clamp-3 text-fg-sub leading-relaxed whitespace-pre-wrap">
          {{ store.state.forwardingMessage.content }}
        </p>
      </div>

      <div class="flex flex-col gap-1.5 min-h-0">
        <div class="flex items-center justify-between gap-2">
          <span class="text-xs font-medium text-fg-sub">选择接收会话</span>
          <input
            v-if="targets.length > 4"
            v-model="targetQuery"
            type="search"
            aria-label="筛选转发目标"
            placeholder="搜索目标会话..."
            @keydown.escape="clearSearchOnEscape($event, targetQuery, () => { targetQuery = '' })"
            class="w-44 min-h-[30px] rounded-lg border border-line bg-canvas px-2.5 text-xs text-fg placeholder:text-fg-muted focus:border-accent focus:outline-none"
          />
        </div>
        <div v-if="filteredTargets.length === 0" class="py-6 text-center text-xs text-fg-muted">
          没有匹配的目标会话
        </div>
        <div v-else class="max-h-52 overflow-y-auto grid grid-cols-1 sm:grid-cols-2 gap-1.5 pr-0.5">
          <button
            v-for="b in filteredTargets"
            :key="b.id"
            type="button"
            @pointermove="trackSpotlight"
            @click="selectedId = b.id"
            :class="[
              'vesper-card rounded-xl px-2.5 py-2 flex items-center gap-2.5 text-left cursor-pointer',
              selectedId === b.id
                ? 'border-accent-border bg-accent-soft'
                : 'border-line opacity-85 hover:opacity-100'
            ]"
          >
            <BotGroupAvatar
              v-if="b.is_group"
              :members="resolveGroupMembers(b)"
              :fallback-style="b.avatar_style"
              :state="b.state"
              size="sm"
            />
            <BotAvatar v-else :avatar-style="b.avatar_style" :state="b.state" size="sm" :interactive="false" />
            <div class="flex-1 min-w-0">
              <div class="text-xs font-semibold text-fg truncate">{{ b.name }}</div>
              <div class="text-xs text-fg-muted truncate">{{ b.role_title }}</div>
            </div>
            <span
              :class="[
                'w-4 h-4 rounded-full border flex items-center justify-center shrink-0 transition-colors',
                selectedId === b.id
                  ? 'bg-accent border-accent-border text-on-accent'
                  : 'border-line text-transparent'
              ]"
            >
              <svg viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2.8" aria-hidden="true">
                <polyline points="20 6 9 17 4 12" />
              </svg>
            </span>
          </button>
        </div>
      </div>

      <div class="flex flex-col gap-1.5 text-xs text-fg-sub">
        <div class="flex items-center justify-between">
          <span>附言说明（可选）</span>
          <span class="text-xs font-mono text-fg-muted">{{ note.length }}/500</span>
        </div>
        <input
          v-model="note"
          type="text"
          maxlength="500"
          aria-label="转发附言说明"
          placeholder="如：请参考这条记录排查关联节点... (Cmd/Ctrl + Enter 发送)"
          @keydown.meta.enter.prevent="selectedId && !sending && submitForward(true)"
          @keydown.ctrl.enter.prevent="selectedId && !sending && submitForward(true)"
          class="min-h-[40px] rounded-xl border border-line bg-canvas px-3 py-1.5 text-xs text-fg focus:border-accent"
        />
        <div class="flex items-center gap-1.5 flex-wrap text-xs">
          <button
            v-for="tag in ['请协助跟进', '供参考上下文', '排查关联异常']"
            :key="tag"
            type="button"
            :aria-pressed="note === tag"
            @click="note = note === tag ? '' : tag"
            :class="[
              'px-2 py-0.5 rounded-lg border cursor-pointer transition-colors',
              note === tag
                ? 'border-accent-border bg-accent-soft text-accent-fg'
                : 'border-line bg-canvas text-fg-muted hover:border-accent-border hover:text-accent-fg'
            ]"
          >
            {{ tag }}
          </button>
        </div>
      </div>

      <div class="flex items-center justify-end gap-2 pt-2 border-t border-line">
        <button
          type="button"
          @click="closeModal"
          class="min-h-[38px] px-3.5 rounded-xl border border-line text-xs text-fg-sub hover:bg-hover cursor-pointer"
        >
          取消
        </button>
        <button
          type="button"
          :disabled="!selectedId || sending"
          @click="submitForward(false)"
          class="min-h-[38px] px-4 rounded-xl border border-accent-border bg-accent-soft text-xs font-medium text-accent-fg hover:bg-accent-soft cursor-pointer disabled:opacity-40"
        >
          仅转发
        </button>
        <button
          type="button"
          :disabled="!selectedId || sending"
          @click="submitForward(true)"
          class="min-h-[38px] px-4 rounded-xl bg-accent hover:bg-accent-hover text-xs font-semibold text-on-accent cursor-pointer disabled:opacity-40"
        >
          {{ sending ? '转发中...' : '转发并前往' }}
        </button>
      </div>
    </div>
  </BaseModal>
</template>
