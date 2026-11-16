<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { describeCron, generateSlug } from '../utils/format'
import BaseModal from './BaseModal.vue'
import CustomSelect from './CustomSelect.vue'

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
const enabled = ref('true')
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
const ENABLED_OPTIONS = [
  { value: 'true', label: '启用调度' },
  { value: 'false', label: '暂停调度' },
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
      enabled.value = 'true'
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
    enabled.value = t.enabled ? 'true' : 'false'
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
      enabled: enabled.value === 'true',
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
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
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
          <input v-model="name" type="text" autofocus required placeholder="如：每日晨间简报" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
        </label>
        <div class="flex flex-col gap-1 text-xs text-slate-300">
          <span>启用状态</span>
          <CustomSelect v-model="enabled" :options="ENABLED_OPTIONS" aria-label="启用状态" />
        </div>
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
          <input v-model="customCron" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
        </label>
        <div class="sm:col-span-2 font-mono text-[11px] text-[#f2b880]">
          调度预览: {{ describeCron(computedCron) }} ({{ computedCron }})
        </div>

        <template v-if="currentFields.length > 0">
          <div class="sm:col-span-2 pt-2 border-t border-[#fff8ee]/10 text-xs font-semibold text-slate-200">
            模板运行参数
          </div>
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
              class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none"
            />
          </label>
        </template>
      </div>

      <div class="flex items-center justify-between pt-3 border-t border-[#fff8ee]/10">
        <button
          v-if="!isNew"
          type="button"
          @click="handleDelete"
          class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-rose-400/40 bg-rose-500/15 text-xs text-rose-300 cursor-pointer"
        >
          删除该任务
        </button>
        <span v-else />
        <div class="flex gap-2">
          <button type="button" @click="closeModal" class="min-h-[44px] sm:min-h-[34px] px-4 py-2 rounded-xl border border-[#fff8ee]/14 text-xs text-slate-300 hover:bg-white/5 cursor-pointer">
            取消
          </button>
          <button type="submit" :disabled="saving" class="min-h-[44px] sm:min-h-[34px] px-4 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50">
            保存任务
          </button>
        </div>
      </div>
    </form>
  </BaseModal>
</template>
