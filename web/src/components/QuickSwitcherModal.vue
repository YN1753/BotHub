<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import BaseModal from './BaseModal.vue'
import BotAvatar from './BotAvatar.vue'
import BotGroupAvatar from './BotGroupAvatar.vue'
import type { Bot } from '../types/bothub'

const store = useAppStore()
const query = ref('')
const highlighted = ref(0)
const inputRef = ref<HTMLInputElement | null>(null)

const specialistMap = computed(() => {
  const m = new Map<string, Bot>()
  store.state.bots.forEach((b) => {
    if (!b.is_group) m.set(b.id, b)
  })
  return m
})

function resolveGroupMembers(b: Bot): Bot[] {
  try {
    const ids = JSON.parse(b.member_ids || '[]')
    if (Array.isArray(ids)) {
      const resolved = ids.map((id: string) => specialistMap.value.get(id)).filter((x): x is Bot => Boolean(x))
      if (resolved.length > 0) return resolved
    }
  } catch {
    // ignore
  }
  return Array.from(specialistMap.value.values()).slice(0, 4)
}

interface SearchEntry {
  key: string
  category: '会话' | '话题' | '技能' | '待办' | '文档' | '快捷操作' | '资产' | '例程' | '记忆'
  title: string
  subtitle: string
  bot?: Bot
  run: () => void | Promise<void>
}

const entries = computed<SearchEntry[]>(() => {
  const q = query.value.trim().toLowerCase()
  const list: SearchEntry[] = []

  store.state.bots.forEach((b) => {
    list.push({
      key: `bot_${b.id}`,
      category: '会话',
      title: b.name,
      subtitle: `${b.is_group ? '多 Bot 群聊 · ' : ''}${b.role_title}`,
      bot: b,
      run: () => store.jumpToBot(b.id),
    })
  })

  const quickActions: SearchEntry[] = [
    {
      key: 'act_group',
      category: '快捷操作',
      title: '组建多 Bot 协同群聊',
      subtitle: '创建新的 Bot 讨论群或配置群协同策略',
      run: () => {
        store.state.createModalMode = 'group'
        store.state.showCreateBotModal = true
      },
    },
    {
      key: 'act_drawer',
      category: '快捷操作',
      title: '拉人进群 / 会话成员管理',
      subtitle: '向当前群聊邀请或移除 Bot，或在私聊一键拉起群聊',
      run: () => {
        store.state.workspace = 'bots'
        store.state.showGroupDrawer = true
      },
    },
    {
      key: 'act_new_bot',
      category: '快捷操作',
      title: '创造全新专属 Bot 伙伴',
      subtitle: '自由配置独立性格、记忆域、自定义技能与工具集',
      run: () => {
        store.state.createModalMode = 'bot'
        store.state.showCreateBotModal = true
      },
    },
    {
      key: 'act_skills',
      category: '快捷操作',
      title: '打开自定义技能工坊',
      subtitle: '创建行为规范 Playbook 或测试 Webhook API 技能工具',
      run: () => {
        store.state.showSkillsModal = true
      },
    },
    {
      key: 'act_ops',
      category: '快捷操作',
      title: '切换到全局资产与调度台',
      subtitle: '查看 VPS、域名续费预警与后台定时任务',
      run: () => {
        store.state.workspace = 'ops'
      },
    },
    {
      key: 'act_workhub',
      category: '快捷操作',
      title: '打开个人行动台 (待办 / 文档)',
      subtitle: `待办 ${store.pendingTodoCount.value} 项 · 文档 ${store.state.notes.length} 份`,
      run: () => {
        store.state.workhubTab = 'todos'
        store.state.showWorkhubModal = true
      },
    },
    {
      key: 'act_inbox',
      category: '快捷操作',
      title: '打开原生情报收件箱',
      subtitle: `当前未读 ${store.unreadInboxCount.value} 条情报`,
      run: () => {
        store.state.showInboxModal = true
      },
    },
    {
      key: 'act_settings',
      category: '快捷操作',
      title: '系统设置与模型配置',
      subtitle: '配置 DeepSeek API、原生 APP Token 与安全口令',
      run: () => {
        store.state.showSettingsModal = true
      },
    },
  ]
  list.push(...quickActions)

  store.state.botThreads.forEach((th) => {
    list.push({
      key: `th_${th.id}`,
      category: '话题',
      title: th.title,
      subtitle: `当前会话话题 · ${th.msg_count || 0} 条消息${th.summary ? ' · ' + th.summary : ''}`,
      run: () => store.switchThread(th.id),
    })
  })

  ;(store.state.skills || []).forEach((sk) => {
    list.push({
      key: `sk_${sk.id}`,
      category: '技能',
      title: sk.name,
      subtitle: `${sk.kind === 'webhook' ? `API tool: skill_${sk.slug}` : 'Playbook 行为规范'} · ${sk.description || '点击管理'}`,
      run: () => {
        store.state.showSkillsModal = true
      },
    })
  })

  ;(store.state.todos || [])
    .filter((t) => t.status !== 'done')
    .slice(0, 15)
    .forEach((td) => {
      list.push({
        key: `td_${td.id}`,
        category: '待办',
        title: td.title,
        subtitle: `${td.priority === 'high' ? '高优先' : '待办'}${td.due_at ? ' · 截止 ' + td.due_at : ''}`,
        run: () => {
          store.state.workhubTab = 'todos'
          store.state.showWorkhubModal = true
        },
      })
    })

  ;(store.state.notes || []).slice(0, 15).forEach((nt) => {
    list.push({
      key: `nt_${nt.id}`,
      category: '文档',
      title: nt.title,
      subtitle: `${nt.category || 'deliverable'} · 点击在个人行动台查看`,
      run: () => {
        store.state.workhubTab = 'notes'
        store.state.showWorkhubModal = true
      },
    })
  })

  store.state.assets.forEach((a) => {
    list.push({
      key: `asset_${a.id}`,
      category: '资产',
      title: a.name,
      subtitle: `${a.spec || a.category} · ${a.status}`,
      run: () => {
        store.state.workspace = 'ops'
        store.state.editingAsset = a
      },
    })
  })

  store.state.tasks.forEach((t) => {
    list.push({
      key: `task_${t.id}`,
      category: '例程',
      title: t.name,
      subtitle: `${t.description} · 立即触发执行`,
      run: async () => {
        const res = await BotHubApi.runTask(t.id)
        store.notify(res.summary || '任务执行完成')
        await store.loadDashboard(false)
      },
    })
  })

  store.state.memories.forEach((m) => {
    list.push({
      key: `mem_${m.id}`,
      category: '记忆',
      title: m.fact_key,
      subtitle: `${m.category} · ${m.content}`,
      run: () => {
        store.state.showMemoriesModal = true
      },
    })
  })

  if (!q) return list.slice(0, 12)
  return list
    .filter((item) => `${item.title} ${item.subtitle} ${item.category}`.toLowerCase().includes(q))
    .slice(0, 14)
})

watch(
  () => store.state.showQuickSwitcher,
  (open) => {
    if (!open) return
    query.value = ''
    highlighted.value = 0
    nextTick(() => inputRef.value?.focus())
  }
)

watch(
  () => entries.value.length,
  (len) => {
    if (highlighted.value >= len) highlighted.value = Math.max(0, len - 1)
  }
)

function closeModal() {
  store.state.showQuickSwitcher = false
}

async function executeEntry(entry?: SearchEntry) {
  const target = entry || entries.value[highlighted.value]
  if (!target) return
  closeModal()
  await target.run()
}

function handleKeydown(e: KeyboardEvent) {
  const len = entries.value.length
  if (!len) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    highlighted.value = (highlighted.value + 1) % len
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    highlighted.value = (highlighted.value - 1 + len) % len
  } else if (e.key === 'Enter') {
    e.preventDefault()
    executeEntry()
  }
}
</script>

<template>
  <BaseModal
    :open="store.state.showQuickSwitcher"
    title="全局快速跳转 (Cmd/Ctrl + K)"
    subtitle="搜索专属 Bot、协同群聊、数字资产、定时例程或快捷动作"
    max-width="lg"
    @close="closeModal"
  >
    <div class="flex flex-col gap-3 min-h-0">
      <div class="relative">
        <svg
          viewBox="0 0 24 24"
          width="15"
          height="15"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          aria-hidden="true"
          class="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none"
        >
          <circle cx="11" cy="11" r="8" />
          <line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
        <input
          ref="inputRef"
          v-model="query"
          type="search"
          placeholder="输入 Bot 名称、资产关键词或操作指令..."
          @keydown="handleKeydown"
          class="w-full min-h-[44px] rounded-xl border border-[#f2b880]/30 bg-[#050811] pl-10 pr-3 py-2 text-sm text-white placeholder-slate-500 focus:border-[#f2b880]"
        />
      </div>

      <div class="flex-1 min-h-0 max-h-[52dvh] overflow-y-auto flex flex-col gap-1 pr-0.5">
        <div v-if="entries.length === 0" class="py-8 text-center text-xs text-slate-500">
          未找到匹配结果
        </div>
        <button
          v-for="(item, idx) in entries"
          :key="item.key"
          type="button"
          @mouseenter="highlighted = idx"
          @click="executeEntry(item)"
          :class="[
            'w-full min-h-[48px] flex items-center justify-between gap-3 px-3 py-2 rounded-xl border text-left cursor-pointer transition-colors',
            idx === highlighted
              ? 'border-[#f2b880]/50 bg-[#f2b880]/12 text-white'
              : 'border-transparent hover:bg-white/[0.04] text-slate-200'
          ]"
        >
          <div class="flex items-center gap-2.5 min-w-0 flex-1">
            <template v-if="item.bot">
              <BotGroupAvatar
                v-if="item.bot.is_group"
                :members="resolveGroupMembers(item.bot)"
                :fallback-style="item.bot.avatar_style"
                :state="item.bot.state"
                size="sm"
              />
              <BotAvatar v-else :avatar-style="item.bot.avatar_style" :state="item.bot.state" size="sm" :interactive="false" />
            </template>
            <span
              v-else
              class="w-7 h-7 rounded-lg border border-[#fff8ee]/12 bg-[#fff8ee]/[0.04] text-[#f2b880] inline-flex items-center justify-center shrink-0 font-mono text-[10px]"
            >
              {{ item.category.slice(0, 1) }}
            </span>
            <div class="min-w-0 flex-1">
              <div class="text-xs font-semibold truncate">{{ item.title }}</div>
              <div class="text-[11px] text-slate-400 truncate">{{ item.subtitle }}</div>
            </div>
          </div>
          <span class="font-mono text-[10px] px-2 py-0.5 rounded-md border border-[#fff8ee]/10 bg-[#050811]/70 text-slate-400 shrink-0">
            {{ item.category }}
          </span>
        </button>
      </div>
    </div>
  </BaseModal>
</template>
