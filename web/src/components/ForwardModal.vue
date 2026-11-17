<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { resolveGroupMemberBots } from '../utils/botLook'
import { trackSpotlight } from '../utils/topLayer'
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
    const targetName = store.state.bots.find((b) => b.id === selectedId.value)?.name || selectedId.value
    closeModal()
    store.notify(`已转发至「${targetName}」`)
    await store.loadDashboard(false)
    if (jumpAfter) {
      await store.jumpToBot(selectedId.value)
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
      <div class="modal-section text-xs text-slate-300">
        <div class="font-mono text-[10.5px] text-[#f2b880] mb-1">
          来自「{{ store.state.forwardingMessage.fromName }}」的消息摘要：
        </div>
        <p class="line-clamp-3 text-slate-300/90 leading-relaxed whitespace-pre-wrap">
          {{ store.state.forwardingMessage.content }}
        </p>
      </div>

      <div class="flex flex-col gap-1.5 min-h-0">
        <div class="flex items-center justify-between gap-2">
          <span class="text-xs font-medium text-slate-300">选择接收会话</span>
          <input
            v-if="targets.length > 4"
            v-model="targetQuery"
            type="search"
            aria-label="筛选转发目标"
            placeholder="搜索目标会话..."
            @keydown.escape.stop="targetQuery = ''"
            class="w-44 min-h-[30px] rounded-lg border border-[#fff8ee]/12 bg-[#050811] px-2.5 text-[11px] text-white placeholder-slate-500 focus:border-[#f2b880]/60 focus:outline-none"
          />
        </div>
        <div v-if="filteredTargets.length === 0" class="py-6 text-center text-xs text-slate-500">
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
              'luxury-card rounded-xl px-2.5 py-2 flex items-center gap-2.5 text-left cursor-pointer',
              selectedId === b.id
                ? 'border-[#f2b880]/65 bg-[#f2b880]/12'
                : 'border-[#fff8ee]/10 opacity-85 hover:opacity-100'
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
              <div class="text-xs font-semibold text-white truncate">{{ b.name }}</div>
              <div class="text-[10.5px] text-slate-400 truncate">{{ b.role_title }}</div>
            </div>
            <span
              :class="[
                'w-4 h-4 rounded-full border flex items-center justify-center shrink-0 transition-colors',
                selectedId === b.id
                  ? 'bg-[#f2b880] border-[#f2b880] text-[#070a13]'
                  : 'border-[#fff8ee]/20 text-transparent'
              ]"
            >
              <svg viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2.8" aria-hidden="true">
                <polyline points="20 6 9 17 4 12" />
              </svg>
            </span>
          </button>
        </div>
      </div>

      <div class="flex flex-col gap-1.5 text-xs text-slate-300">
        <span>附言说明（可选）</span>
        <input
          v-model="note"
          type="text"
          aria-label="转发附言说明"
          placeholder="如：请参考这条记录排查关联节点..."
          class="min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-1.5 text-xs text-white focus:border-[#f2b880]/60"
        />
        <div class="flex items-center gap-1.5 flex-wrap text-[11px]">
          <button
            v-for="tag in ['请协助跟进', '供参考上下文', '排查关联异常']"
            :key="tag"
            type="button"
            @click="note = tag"
            class="px-2 py-0.5 rounded-md border border-[#fff8ee]/10 bg-[#050811] text-slate-400 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer"
          >
            {{ tag }}
          </button>
        </div>
      </div>

      <div class="flex items-center justify-end gap-2 pt-2 border-t border-[#fff8ee]/10">
        <button
          type="button"
          @click="closeModal"
          class="min-h-[38px] px-3.5 rounded-xl border border-[#fff8ee]/14 text-xs text-slate-300 hover:bg-white/5 cursor-pointer"
        >
          取消
        </button>
        <button
          type="button"
          :disabled="!selectedId || sending"
          @click="submitForward(false)"
          class="min-h-[38px] px-4 rounded-xl border border-[#f2b880]/40 bg-[#f2b880]/12 text-xs font-medium text-[#f2b880] hover:bg-[#f2b880]/20 cursor-pointer disabled:opacity-40"
        >
          仅转发
        </button>
        <button
          type="button"
          :disabled="!selectedId || sending"
          @click="submitForward(true)"
          class="min-h-[38px] px-4 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-xs font-semibold text-[#070a13] cursor-pointer disabled:opacity-40"
        >
          {{ sending ? '转发中...' : '转发并前往' }}
        </button>
      </div>
    </div>
  </BaseModal>
</template>
