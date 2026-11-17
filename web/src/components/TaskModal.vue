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
const botId = ref('')
const enabled = ref(true)
const description = ref('')

// Visual Cron & One-Shot Builder
const freq = ref<'daily' | 'weekly' | 'monthly' | 'hourly' | 'once' | 'custom'>('daily')
const subVal = ref('1')
const hour = ref('08')
const minute = ref('00')
const customCron = ref('0 8 * * *')
const runAt = ref('')

const paramsMap = ref<Record<string, string>>({})
const saving = ref(false)
const forceCreateCopy = ref(false)

const isCustomCronValid = computed(() => {
  if (freq.value !== 'custom') return true
  const parts = customCron.value.trim().split(/\s+/)
  return parts.length === 5 || parts.length === 6
})

function formatQuickRunAt(offsetMinutes: number, fixedHour?: number): string {
  const d = new Date(Date.now() + offsetMinutes * 60_000)
  if (fixedHour !== undefined) {
    d.setDate(d.getDate() + 1)
    d.setHours(fixedHour, 0, 0, 0)
  }
  const y = d.getFullYear()
  const mo = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `${y}-${mo}-${day} ${hh}:${mm}`
}

function duplicateAsNewTask() {
  forceCreateCopy.value = true
  id.value = ''
  name.value = `${name.value.trim()} (副本)`
  handleSubmit()
}

const FREQ_OPTIONS = [
  { value: 'daily', label: '每天固定时间' },
  { value: 'weekly', label: '每周固定时间' },
  { value: 'monthly', label: '每月固定日期' },
  { value: 'hourly', label: '每小时整点' },
  { value: 'once', label: '一次性定时提醒 (到点执行)' },
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
const botOptions = computed(() => [
  { value: '', label: '通用后台调度（不绑定特定 Bot）' },
  ...store.state.bots.map((b) => ({ value: b.id, label: `${b.name} (${b.id})` })),
])
const currentFields = computed(() => {
  const tpl = store.state.templates.find((t) => t.type === type.value)
  return tpl?.params_schema || tpl?.fields || []
})

const computedCron = computed(() => {
  if (freq.value === 'once') return ''
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
      type.value = store.state.templates[0]?.type || 'bot_prompt'
      botId.value = (item as any)?.bot_id || store.state.activeBotId || ''
      enabled.value = true
      description.value = ''
      freq.value = 'daily'
      hour.value = '09'
      minute.value = '00'
      runAt.value = ''
      const initParams: Record<string, string> = {}
      const tpl = store.state.templates.find((t) => t.type === type.value)
      ;(tpl?.params_schema || tpl?.fields || []).forEach((f) => {
        if (f.default_val !== undefined) initParams[f.key] = String(f.default_val)
      })
      paramsMap.value = initParams
      return
    }
    const t = item as any
    id.value = t.id
    name.value = t.name
    type.value = t.type
    botId.value = t.bot_id || ''
    enabled.value = Boolean(t.enabled)
    description.value = t.description || ''
    if (t.run_at) {
      freq.value = 'once'
      runAt.value = t.run_at
    } else {
      runAt.value = ''
      parseCronExpr(t.cron_expr || '0 8 * * *')
    }
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
  if (freq.value === 'once' && !runAt.value.trim()) {
    store.notify('请填写一次性触发时间 (YYYY-MM-DD HH:MM)')
    return
  }
  if (!isCustomCronValid.value) {
    store.notify('自定义 Cron 表达式需为 5 段（或 6 段）空格分隔格式')
    return
  }
  const createMode = isNew.value || forceCreateCopy.value
  forceCreateCopy.value = false
  saving.value = true
  try {
    const finalID = createMode ? id.value.trim() || generateSlug(name.value, type.value) : id.value
    const tpl = store.state.templates.find((t) => t.type === type.value)
    const payload = {
      id: finalID,
      name: name.value.trim(),
      type: type.value,
      bot_id: botId.value,
      cron_expr: freq.value === 'once' ? '' : computedCron.value,
      run_at: freq.value === 'once' ? runAt.value.trim() : '',
      enabled: enabled.value,
      description: description.value.trim() || tpl?.description || '',
      params: JSON.stringify(paramsMap.value),
    }
    if (createMode) {
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
            <button type="button" @click="parseCronExpr('30 8 * * *')" class="px-2 py-0.5 rounded-md border border-[#fff8ee]/10 text-[10.5px] text-slate-400 hover:border-[#f2b880]/40 hover:text-[#f2b880] cursor-pointer">每日 08:30</button>
            <button type="button" @click="parseCronExpr('0 9 * * 1-5')" class="px-2 py-0.5 rounded-md border border-[#fff8ee]/10 text-[10.5px] text-slate-400 hover:border-[#f2b880]/40 hover:text-[#f2b880] cursor-pointer">工作日 09:00</button>
            <button type="button" @click="parseCronExpr('0 * * * *')" class="px-2 py-0.5 rounded-md border border-[#fff8ee]/10 text-[10.5px] text-slate-400 hover:border-[#f2b880]/40 hover:text-[#f2b880] cursor-pointer">每小时</button>
            <span
              v-if="(store.state.editingTask as any)?.next_run_at"
              class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-emerald-400/25 bg-emerald-400/10 text-emerald-300"
            >
              下次: {{ (store.state.editingTask as any).next_run_at }}
            </span>
            <span class="inline-flex items-center gap-1.5 font-mono text-[11px] px-2.5 py-0.5 rounded-full border border-[#f2b880]/30 bg-[#f2b880]/10 text-[#f7cba0]">
              {{ freq === 'once' ? `单次提醒: ${runAt || '待设时间'}` : `${describeCron(computedCron)} (${computedCron})` }}
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
          <div v-if="freq !== 'custom' && freq !== 'once'" class="grid grid-cols-2 gap-2">
            <div v-if="freq !== 'hourly'" class="flex flex-col gap-1 text-xs text-slate-300">
              <span>小时</span>
              <CustomSelect v-model="hour" :options="HOUR_OPTIONS" aria-label="小时" />
            </div>
            <div class="flex flex-col gap-1 text-xs text-slate-300">
              <span>分钟</span>
              <CustomSelect v-model="minute" :options="MIN_OPTIONS" aria-label="分钟" />
            </div>
          </div>
          <div v-if="freq === 'once'" class="sm:col-span-2 flex flex-col gap-1.5 text-xs text-slate-300">
            <span>单次触发时间 (上海时间 YYYY-MM-DD HH:MM)</span>
            <input v-model="runAt" type="text" placeholder="2026-11-18 09:30" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            <div class="flex items-center gap-1.5 flex-wrap text-[11px]">
              <span class="text-slate-500">快捷设定:</span>
              <button type="button" @click="runAt = formatQuickRunAt(30)" class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer">30 分钟后</button>
              <button type="button" @click="runAt = formatQuickRunAt(120)" class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer">2 小时后</button>
              <button type="button" @click="runAt = formatQuickRunAt(0, 9)" class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer">明早 09:00</button>
            </div>
          </div>
          <div v-if="freq === 'custom'" class="sm:col-span-2 flex flex-col gap-1.5 text-xs text-slate-300">
            <div class="flex items-center justify-between">
              <span>Cron 表达式</span>
              <span :class="['font-mono text-[10.5px]', isCustomCronValid ? 'text-emerald-300' : 'text-rose-400']">
                {{ isCustomCronValid ? '格式有效 (分 时 日 月 周)' : '需由 5 段空格分隔' }}
              </span>
            </div>
            <input v-model="customCron" type="text" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60" />
            <div class="flex items-center gap-1.5 flex-wrap text-[11px]">
              <span class="text-slate-500">常用表达式:</span>
              <button type="button" @click="customCron = '*/30 * * * *'" class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] font-mono text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer">每 30 分钟</button>
              <button type="button" @click="customCron = '0 9 * * 1-5'" class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] font-mono text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer">工作日 09:00</button>
              <button type="button" @click="customCron = '0 9,18 * * *'" class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] font-mono text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer">早晚 09/18 点</button>
            </div>
          </div>
        </div>
      </div>

      <!-- Section 3: Module Params -->
      <div v-if="currentFields.length > 0" class="modal-section flex flex-col gap-3">
        <div class="text-xs font-semibold text-[#f2b880]">执行引擎参数</div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <label
            v-for="f in currentFields"
            :key="f.key"
            class="flex flex-col gap-1 text-xs text-slate-300"
          >
            <span>{{ f.label }}</span>
            <input
              v-model="paramsMap[f.key]"
              :type="f.type === 'number' ? 'number' : 'text'"
              :placeholder="f.placeholder || f.default_val || f.default_value || ''"
              class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60"
            />
            <span v-if="f.hint" class="text-[10.5px] text-slate-500">{{ f.hint }}</span>
          </label>
        </div>
      </div>

      <div class="flex items-center justify-between gap-2 flex-wrap pt-2 border-t border-[#fff8ee]/10">
        <div v-if="!isNew" class="flex items-center gap-2">
          <button
            type="button"
            @click="handleDelete"
            class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-rose-400/40 bg-rose-500/15 text-xs text-rose-300 hover:bg-rose-500/25 cursor-pointer"
          >
            删除该任务
          </button>
          <button
            type="button"
            :disabled="saving"
            @click="duplicateAsNewTask"
            class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-[#fff8ee]/14 text-xs text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer disabled:opacity-50"
          >
            另存为新任务
          </button>
        </div>
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
