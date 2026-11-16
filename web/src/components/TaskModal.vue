<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { describeCron, generateSlug } from '../utils/format'
import BaseModal from './BaseModal.vue'
import CustomSelect from './CustomSelect.vue'
import ToggleSwitch from './ToggleSwitch.vue'

const store = useAppStore()
const isOpen = computed(() => store.state.editingTask !== undefined)
const isNew = computed(() => {
  const t = store.state.editingTask
  return !t || !(t as any).id
})

const id = ref('')
const name = ref('')
const type = ref('morning')
const botId = ref('butler')
const enabled = ref(true)
const description = ref('')

// Visual Cron Builder
const freq = ref<'daily' | 'weekly' | 'monthly' | 'hourly' | 'custom'>('daily')
const subVal = ref('1')
const hour = ref('08')
const minute = ref('00')
const customCron = ref('0 8 * * *')

const paramsMap = ref<Record<string, string>>({})
const saving = ref(false)

const FREQ_OPTIONS = [
  { value: 'daily', label: '每天固定时间' },
  { value: 'weekly', label: '每周固定时间' },
  { value: 'monthly', label: '每月固定日期' },
  { value: 'hourly', label: '每小时整点' },
  { value: 'custom', label: '自定义 Cron 表达式' },
]
const HOUR_OPTIONS = Array.from({ length: 24 }, (_, i) => {
  const v = String(i).padStart(2, '0')
  return { value: v, label: `${v} 时` }
})
const MIN_OPTIONS = ['00', '15', '30', '45'].map((v) => ({ value: v, label: `${v} 分` }))
const WEEK_OPTIONS = [
  { value: '1', label: '每周一' },
  { value: '2', label: '每周二' },
  { value: '3', label: '每周三' },
  { value: '4', label: '每周四' },
  { value: '5', label: '每周五' },
  { value: '6', label: '每周六' },
  { value: '0', label: '每周日' },
]
const MONTH_DAY_OPTIONS = Array.from({ length: 28 }, (_, i) => {
  const v = String(i + 1)
  return { value: v, label: `每月 ${v} 日` }
})

const templateOptions = computed(() =>
  store.state.templates.map((t) => ({ value: t.type, label: t.name }))
)
const botOptions = computed(() =>
  store.state.bots.map((b) => ({ value: b.id, label: `${b.name} (${b.id})` }))
)
const currentFields = computed(() => {
  const tpl = store.state.templates.find((t) => t.type === type.value)
  return tpl?.fields || []
})

const computedCron = computed(() => {
  if (freq.value === 'custom') return customCron.value.trim() || '0 8 * * *'
  const m = String(parseInt(minute.value, 10) || 0)
  const h = String(parseInt(hour.value, 10) || 0)
  if (freq.value === 'hourly') return `${m} * * * *`
  if (freq.value === 'daily') return `${m} ${h} * * *`
  if (freq.value === 'weekly') return `${m} ${h} * * ${subVal.value}`
  if (freq.value === 'monthly') return `${m} ${h} ${subVal.value} * *`
  return '0 8 * * *'
})

function parseCronExpr(expr: string) {
  const parts = expr.trim().split(/\s+/)
  if (parts.length !== 5) {
    freq.value = 'custom'
    customCron.value = expr
    return
  }
  const [m, h, dom, mon, dow] = parts
  const pad = (x: string) => String(parseInt(x, 10) || 0).padStart(2, '0')
  if (!Number.isNaN(Number(m)) && !Number.isNaN(Number(h)) && dom === '*' && mon === '*' && dow === '*') {
    freq.value = 'daily'
    hour.value = pad(h)
    minute.value = pad(m)
  } else if (!Number.isNaN(Number(m)) && !Number.isNaN(Number(h)) && dom === '*' && mon === '*' && !Number.isNaN(Number(dow))) {
    freq.value = 'weekly'
    subVal.value = dow
    hour.value = pad(h)
    minute.value = pad(m)
  } else if (!Number.isNaN(Number(m)) && !Number.isNaN(Number(h)) && !Number.isNaN(Number(dom)) && mon === '*' && dow === '*') {
    freq.value = 'monthly'
    subVal.value = dom
    hour.value = pad(h)
    minute.value = pad(m)
  } else if (!Number.isNaN(Number(m)) && h === '*' && dom === '*' && mon === '*' && dow === '*') {
    freq.value = 'hourly'
    minute.value = pad(m)
  } else {
    freq.value = 'custom'
    customCron.value = expr
  }
}

watch(
  () => store.state.editingTask,
  (item) => {
    if (item === undefined) return
    if (!item || !(item as any).id) {
      id.value = ''
      name.value = ''
      type.value = 'morning'
      botId.value = (item as any)?.bot_id || 'butler'
      enabled.value = true
      description.value = ''
      freq.value = 'daily'
      hour.value = '08'
      minute.value = '00'
      paramsMap.value = {}
      return
    }
    const t = item as any
    id.value = t.id
    name.value = t.name
    type.value = t.type
    botId.value = t.bot_id || 'butler'
    enabled.value = Boolean(t.enabled)
    description.value = t.description || ''
    parseCronExpr(t.cron_expr || '0 8 * * *')
    try {
      const parsed = JSON.parse(t.params || '{}')
      const m: Record<string, string> = {}
      Object.entries(parsed).forEach(([k, v]) => {
        m[k] = String(v ?? '')
      })
      paramsMap.value = m
    } catch {
      paramsMap.value = {}
    }
  }
)

function closeModal() {
  store.state.editingTask = undefined
}

async function handleSubmit() {
  if (!name.value.trim() || saving.value) return
  saving.value = true
  try {
    const finalID = isNew.value ? id.value.trim() || generateSlug(name.value, type.value) : id.value
    const tpl = store.state.templates.find((t) => t.type === type.value)
    const payload = {
      id: finalID,
      name: name.value.trim(),
      type: type.value,
      bot_id: botId.value,
      cron_expr: computedCron.value,
      enabled: enabled.value,
      description: description.value.trim() || tpl?.description || '',
      params: JSON.stringify(paramsMap.value),
    }
    if (isNew.value) {
      await BotHubApi.createTask(payload)
      store.notify('新例程任务创建成功')
    } else {
      await BotHubApi.updateTask(finalID, payload)
      store.notify('任务配置已更新')
    }
    closeModal()
    await store.loadDashboard(false)
  } catch (err: any) {
    store.notify(err.message)
  } finally {
    saving.value = false
  }
}

async function handleDelete() {
  if (isNew.value || !id.value) return
  if (!window.confirm(`确定删除调度任务 [${name.value}] 吗？`)) return
  await BotHubApi.deleteTask(id.value)
  store.notify('任务已删除')
  closeModal()
  await store.loadDashboard(false)
}
</script>

<template>
  <BaseModal
    :open="isOpen"
    :title="isNew ? '创建自动化例程任务' : '编辑例程任务'"
    subtitle="绑定专属汇报 Bot 与可执行模板的定时调度规则"
    max-width="2xl"
    @close="closeModal"
  >
    <form @submit.prevent="handleSubmit" class="flex-1 min-h-0 overflow-y-auto pr-1 flex flex-col gap-3.5">
      <!-- Section 1: Basic Info -->
      <div class="modal-section flex flex-col gap-3">
        <div class="text-xs font-semibold text-[#f2b880]">任务身份与归属 Bot</div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>执行模板类型</span>
            <CustomSelect v-model="type" :options="templateOptions" aria-label="执行模板类型" />
          </div>
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>归属汇报 Bot</span>
            <CustomSelect v-model="botId" :options="botOptions" aria-label="归属汇报 Bot" />
          </div>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>任务名称</span>
            <input v-model="name" type="text" required placeholder="如：每日晨间简报" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60" />
          </label>
          <div class="flex flex-col justify-end">
            <ToggleSwitch v-model="enabled" label="启用后台自动调度" />
          </div>
        </div>
      </div>

      <!-- Section 2: Visual Cron Schedule -->
      <div class="modal-section flex flex-col gap-3">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <span class="text-xs font-semibold text-[#f2b880]">触发频率与时间窗</span>
          <div class="flex items-center gap-1.5 flex-wrap">
            <span
              v-if="(store.state.editingTask as any)?.next_run_at"
              class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-emerald-400/25 bg-emerald-400/10 text-emerald-300"
            >
              下次: {{ (store.state.editingTask as any).next_run_at }}
            </span>
            <span class="inline-flex items-center gap-1.5 font-mono text-[11px] px-2.5 py-0.5 rounded-full border border-[#f2b880]/30 bg-[#f2b880]/10 text-[#f7cba0]">
              {{ describeCron(computedCron) }} ({{ computedCron }})
            </span>
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>执行频率</span>
            <CustomSelect v-model="freq" :options="FREQ_OPTIONS" aria-label="执行频率" />
          </div>
          <div v-if="freq === 'weekly'" class="flex flex-col gap-1 text-xs text-slate-300">
            <span>星期选择</span>
            <CustomSelect v-model="subVal" :options="WEEK_OPTIONS" aria-label="星期选择" />
          </div>
          <div v-if="freq === 'monthly'" class="flex flex-col gap-1 text-xs text-slate-300">
            <span>日期选择</span>
            <CustomSelect v-model="subVal" :options="MONTH_DAY_OPTIONS" aria-label="日期选择" />
          </div>
          <div v-if="freq !== 'custom'" class="grid grid-cols-2 gap-2">
            <div v-if="freq !== 'hourly'" class="flex flex-col gap-1 text-xs text-slate-300">
              <span>小时</span>
              <CustomSelect v-model="hour" :options="HOUR_OPTIONS" aria-label="小时" />
            </div>
            <div class="flex flex-col gap-1 text-xs text-slate-300">
              <span>分钟</span>
              <CustomSelect v-model="minute" :options="MIN_OPTIONS" aria-label="分钟" />
            </div>
          </div>
          <label v-if="freq === 'custom'" class="sm:col-span-2 flex flex-col gap-1 text-xs text-slate-300">
            <span>Cron 表达式</span>
            <input v-model="customCron" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
          </label>
        </div>
      </div>

      <!-- Section 3: Module Params -->
      <div v-if="currentFields.length > 0" class="modal-section flex flex-col gap-3">
        <div class="text-xs font-semibold text-[#f2b880]">模板运行参数</div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <label
            v-for="f in currentFields"
            :key="f.key"
            class="flex flex-col gap-1 text-xs text-slate-300"
          >
            <span>{{ f.label }}</span>
            <input
              v-model="paramsMap[f.key]"
              type="text"
              :placeholder="f.placeholder || f.default_value || ''"
              class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60"
            />
          </label>
        </div>
      </div>

      <div class="flex items-center justify-between pt-2 border-t border-[#fff8ee]/10">
        <button
          v-if="!isNew"
          type="button"
          @click="handleDelete"
          class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-rose-400/40 bg-rose-500/15 text-xs text-rose-300 hover:bg-rose-500/25 cursor-pointer"
        >
          删除该任务
        </button>
        <span v-else />
        <div class="flex gap-2">
          <button type="button" @click="closeModal" class="min-h-[44px] sm:min-h-[34px] px-4 py-2 rounded-xl border border-[#fff8ee]/14 text-xs text-slate-300 hover:bg-white/5 cursor-pointer">
            取消
          </button>
          <button type="submit" :disabled="saving" class="min-h-[44px] sm:min-h-[34px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50">
            保存任务
          </button>
        </div>
      </div>
    </form>
  </BaseModal>
</template>
