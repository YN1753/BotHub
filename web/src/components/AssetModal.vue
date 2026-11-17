<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { generateSlug, parseJsonRecord } from '../utils/format'
import BaseModal from './BaseModal.vue'
import CustomSelect from './CustomSelect.vue'
import ToggleSwitch from './ToggleSwitch.vue'

const store = useAppStore()
const isOpen = computed(() => store.state.editingAsset !== undefined)
const isNew = computed(() => store.state.editingAsset === null)

const id = ref('')
const name = ref('')
const category = ref('compute')
const customCategory = ref('')
const spec = ref('')

// Structured pricing: Currency type + numeric amount + cycle
const currency = ref<'CNY' | 'USD' | 'EUR' | 'HKD' | 'JPY' | 'GBP' | 'FREE'>('CNY')
const amountNum = ref('')
const billingCycle = ref<'年' | '月' | '季' | '一次性'>('年')

const status = ref<'生效中' | '已到期' | '已停用'>('生效中')
const dueMode = ref<'date' | 'permanent'>('date')
const dueYear = ref('2027')
const dueMonth = ref('01')
const dueDay = ref('01')
const autoRenew = ref(false)
const warnDays = ref('[7, 3, 1]')
const extraRows = ref<Array<{ key: string; value: string }>>([])
const saving = ref(false)

const BASE_CATEGORIES = [
  { value: 'compute', label: '算力 / 云主机' },
  { value: 'domain', label: '域名 / DNS' },
  { value: 'life', label: '订阅 / 软件服务' },
  { value: 'other', label: '其他数字资产' },
]

const categoryOptions = computed(() => {
  const known = new Set(BASE_CATEGORIES.map((c) => c.value))
  const extraOpts = store.state.assets
    .map((a) => String(a.category || '').trim())
    .filter((c) => c && !known.has(c))
  const uniqueExtra = Array.from(new Set(extraOpts)).map((c) => ({ value: c, label: c }))
  return [...BASE_CATEGORIES, ...uniqueExtra, { value: '__custom__', label: '+ 自定义分类...' }]
})

const CURRENCY_OPTIONS = [
  { value: 'CNY', label: '人民币 (CNY ¥)' },
  { value: 'USD', label: '美元 (USD $)' },
  { value: 'EUR', label: '欧元 (EUR €)' },
  { value: 'HKD', label: '港元 (HKD HK$)' },
  { value: 'JPY', label: '日元 (JPY ¥)' },
  { value: 'GBP', label: '英镑 (GBP £)' },
  { value: 'FREE', label: '免费 / 赠金' },
]

const CURRENCY_PREFIX: Record<string, string> = {
  CNY: '¥',
  USD: '$',
  EUR: '€',
  HKD: 'HK$',
  JPY: 'JPY ',
  GBP: '£',
  FREE: '免费',
}

const CYCLE_OPTIONS = [
  { value: '年', label: '年付 (/年)' },
  { value: '月', label: '月付 (/月)' },
  { value: '季', label: '季付 (/季)' },
  { value: '一次性', label: '一次性 / 买断' },
]

const STATUS_OPTIONS = [
  { value: '生效中', label: '生效中' },
  { value: '已到期', label: '已到期' },
  { value: '已停用', label: '已停用' },
]

const DUE_MODE_OPTIONS = [
  { value: 'date', label: '指定具体到期日' },
  { value: 'permanent', label: '长期有效 / 免续费' },
]

const WARN_OPTIONS = [
  { value: '[7, 3, 1]', label: '到期前 7天 / 3天 / 1天' },
  { value: '[14, 7, 3, 1]', label: '到期前 14天 / 7天 / 3天 / 1天' },
  { value: '[30, 15, 7, 3, 1]', label: '到期前 30天 / 15天 / 7天 / 3天 / 1天' },
]

const YEAR_OPTIONS = Array.from({ length: 12 }, (_, i) => {
  const y = String(2025 + i)
  return { value: y, label: `${y} 年` }
})
const MONTH_OPTIONS = Array.from({ length: 12 }, (_, i) => {
  const m = String(i + 1).padStart(2, '0')
  return { value: m, label: `${m} 月` }
})
const dayOptions = computed(() => {
  const y = Number(dueYear.value) || 2026
  const m = Number(dueMonth.value) || 1
  const maxDay = new Date(y, m, 0).getDate()
  return Array.from({ length: maxDay }, (_, i) => {
    const d = String(i + 1).padStart(2, '0')
    return { value: d, label: `${d} 日` }
  })
})

watch([dueYear, dueMonth], ([yStr, mStr]) => {
  const maxDay = new Date(Number(yStr) || 2026, Number(mStr) || 1, 0).getDate()
  if (Number(dueDay.value) > maxDay) {
    dueDay.value = String(maxDay).padStart(2, '0')
  }
})

const pricePreview = computed(() => formatStructuredPrice())

function normalizeStatus(raw?: string): '生效中' | '已到期' | '已停用' {
  const s = String(raw || '').trim()
  if (s === '已到期' || s.toLowerCase() === 'expired') return '已到期'
  if (s === '已停用' || s === '待购入' || s.toLowerCase() === 'paused' || s.toLowerCase() === 'offline') return '已停用'
  return '生效中'
}

function parseStructuredPrice(raw?: string) {
  const txt = String(raw || '').trim()
  let cycle: '年' | '月' | '季' | '一次性' = '年'
  if (txt.includes('月')) cycle = '月'
  else if (txt.includes('季')) cycle = '季'
  else if (txt.includes('一次') || txt.includes('永久')) cycle = '一次性'

  const moneyPart = txt.split('/')[0].trim()
  if (!moneyPart || moneyPart.includes('免费') || moneyPart.includes('0元') || /^free$/i.test(moneyPart)) {
    return {
      cur: moneyPart.includes('免费') || /^free$/i.test(moneyPart) ? ('FREE' as const) : ('CNY' as const),
      num: '',
      cycle,
    }
  }

  let cur: 'CNY' | 'USD' | 'EUR' | 'HKD' | 'JPY' | 'GBP' = 'CNY'
  const upper = moneyPart.toUpperCase()
  if (upper.includes('HK$') || upper.includes('HKD')) cur = 'HKD'
  else if (upper.includes('JPY')) cur = 'JPY'
  else if (upper.includes('GBP') || moneyPart.includes('£')) cur = 'GBP'
  else if (upper.includes('EUR') || moneyPart.includes('€')) cur = 'EUR'
  else if (upper.includes('USD') || moneyPart.includes('$')) cur = 'USD'
  else if (upper.includes('CNY') || moneyPart.includes('¥') || moneyPart.includes('￥') || moneyPart.includes('元')) cur = 'CNY'

  const numMatch = moneyPart.match(/\d+(?:\.\d+)?/)
  return {
    cur,
    num: numMatch ? numMatch[0] : '',
    cycle,
  }
}

function formatStructuredPrice(): string {
  if (currency.value === 'FREE') {
    return billingCycle.value === '一次性' ? '免费 (一次性)' : `免费 / ${billingCycle.value}`
  }
  const cleanNum = String(amountNum.value ?? '').trim()
  if (!cleanNum) {
    return billingCycle.value
  }
  const prefix = CURRENCY_PREFIX[currency.value] || '¥'
  const numText = `${prefix}${cleanNum}`
  if (billingCycle.value === '一次性') {
    return `${numText} (一次性)`
  }
  return `${numText} / ${billingCycle.value}`
}

watch(
  () => store.state.editingAsset,
  (item) => {
    if (item === undefined) return
    if (item === null) {
      id.value = ''
      name.value = ''
      category.value = 'compute'
      customCategory.value = ''
      spec.value = ''
      currency.value = 'CNY'
      amountNum.value = ''
      billingCycle.value = '年'
      status.value = '生效中'
      dueMode.value = 'date'
      dueYear.value = String(new Date().getFullYear() + 1)
      dueMonth.value = '01'
      dueDay.value = '01'
      autoRenew.value = false
      warnDays.value = '[7, 3, 1]'
      extraRows.value = []
      return
    }
    id.value = item.id
    name.value = item.name
    const cat = String(item.category || 'compute').trim()
    category.value = cat
    customCategory.value = ''
    spec.value = item.spec || ''
    status.value = normalizeStatus(item.status)
    autoRenew.value = Boolean(item.auto_renew)
    warnDays.value = item.warn_days || '[7, 3, 1]'

    const parsed = parseStructuredPrice(item.price)
    currency.value = parsed.cur
    amountNum.value = parsed.num
    billingCycle.value = parsed.cycle

    if (!item.due_date || item.due_date.startsWith('2099')) {
      dueMode.value = 'permanent'
    } else {
      dueMode.value = 'date'
      const m = item.due_date.match(/^(\d{4})-(\d{2})-(\d{2})/)
      if (m) {
        dueYear.value = m[1]
        dueMonth.value = m[2]
        dueDay.value = m[3]
      }
    }
    extraRows.value = Object.entries(parseJsonRecord(item.extra_config))
      .filter(([k]) => k.trim() !== '')
      .map(([k, v]) => ({ key: k, value: v }))
  }
)

function closeModal() {
  store.state.editingAsset = undefined
}

function addExtraRow(presetKey = '', presetVal = '') {
  extraRows.value.push({ key: presetKey, value: presetVal })
}

function removeExtraRow(idx: number) {
  extraRows.value.splice(idx, 1)
}

const previewDaysLeft = computed(() => {
  if (dueMode.value === 'permanent') return null
  const target = new Date(`${dueYear.value}-${dueMonth.value}-${dueDay.value}T23:59:59`)
  if (Number.isNaN(target.getTime())) return null
  return Math.ceil((target.getTime() - Date.now()) / 86_400_000)
})

function extendDueDate(months: number) {
  dueMode.value = 'date'
  const base = new Date(`${dueYear.value}-${dueMonth.value}-${dueDay.value}T00:00:00`)
  const target = Number.isNaN(base.getTime()) ? new Date() : base
  target.setMonth(target.getMonth() + months)
  dueYear.value = String(target.getFullYear())
  dueMonth.value = String(target.getMonth() + 1).padStart(2, '0')
  dueDay.value = String(target.getDate()).padStart(2, '0')
}

async function handleSubmit() {
  if (!name.value.trim() || saving.value) return
  saving.value = true
  try {
    const finalCat =
      category.value === '__custom__'
        ? customCategory.value.trim() || 'other'
        : category.value || 'compute'
    const finalID = isNew.value ? id.value.trim() || generateSlug(name.value, finalCat) : id.value
    const dueDate =
      dueMode.value === 'permanent'
        ? '2099-12-31T23:59:59+08:00'
        : `${dueYear.value}-${dueMonth.value}-${dueDay.value}T23:59:59+08:00`

    const extraObj: Record<string, string> = {}
    for (const row of extraRows.value) {
      const k = row.key.trim()
      if (k) {
        extraObj[k] = row.value.trim()
      }
    }

    const payload = {
      id: finalID,
      name: name.value.trim(),
      category: finalCat,
      spec: spec.value.trim(),
      price: formatStructuredPrice(),
      status: status.value,
      due_date: dueDate,
      auto_renew: autoRenew.value,
      warn_days: warnDays.value,
      extra_config: JSON.stringify(extraObj),
    }
    if (isNew.value) {
      await BotHubApi.createAsset(payload)
      store.notify('新资产创建成功')
    } else {
      await BotHubApi.updateAsset(finalID, payload)
      store.notify('资产配置已更新')
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
  if (!window.confirm(`确定删除资产 [${name.value}] 吗？`)) return
  await BotHubApi.deleteAsset(id.value)
  store.notify('资产已删除')
  closeModal()
  await store.loadDashboard(false)
}
</script>

<template>
  <BaseModal
    :open="isOpen"
    :title="isNew ? '登记数字资产' : '编辑数字资产'"
    subtitle="结构化管理资产周期、币种成本、状态与可扩展属性"
    max-width="2xl"
    @close="closeModal"
  >
    <form @submit.prevent="handleSubmit" class="flex-1 min-h-0 overflow-y-auto pr-1 flex flex-col gap-3.5">
      <!-- Section 1: Basic Info & Structured Billing -->
      <div class="modal-section flex flex-col gap-3">
        <div class="text-xs font-semibold text-[#f2b880]">基础信息与结构化费用</div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>资产类别</span>
            <CustomSelect v-model="category" :options="categoryOptions" aria-label="资产类别" />
          </div>
          <label v-if="category === '__custom__'" class="flex flex-col gap-1 text-xs text-slate-300">
            <span>自定义分类名称</span>
            <input
              v-model="customCategory"
              type="text"
              required
              placeholder="如：license / telecom / storage"
              class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60"
            />
          </label>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>资产名称</span>
            <input v-model="name" type="text" required placeholder="如：Tokyo VPS / example.com" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>规格与备注描述</span>
            <input v-model="spec" type="text" placeholder="如：2C 2G · 40G NVMe" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60" />
          </label>
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>生命周期状态</span>
            <CustomSelect v-model="status" :options="STATUS_OPTIONS" aria-label="生命周期状态" />
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-1">
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>币种 / 费用类型</span>
            <CustomSelect v-model="currency" :options="CURRENCY_OPTIONS" aria-label="币种类型" />
          </div>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>金额数值 ({{ currency === 'FREE' ? '免填' : '纯数字' }})</span>
            <input
              v-model="amountNum"
              type="number"
              min="0"
              step="0.01"
              :disabled="currency === 'FREE'"
              :placeholder="currency === 'FREE' ? '0 (免费)' : '如 21.66'"
              class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 disabled:opacity-45"
            />
          </label>
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>计费周期</span>
            <CustomSelect v-model="billingCycle" :options="CYCLE_OPTIONS" aria-label="计费周期" />
          </div>
        </div>
        <div class="flex items-center justify-between text-[11px] font-mono text-slate-400 px-0.5">
          <span>账单展示预览: <strong class="text-[#f2b880]">{{ pricePreview }}</strong></span>
        </div>
      </div>

      <!-- Section 2: Due Date & Alert Rules -->
      <div class="modal-section flex flex-col gap-3">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div class="text-xs font-semibold text-[#f2b880]">到期守望与续费规则</div>
          <span
            v-if="dueMode === 'permanent'"
            class="font-mono text-[10.5px] px-2 py-0.5 rounded-full border border-emerald-400/25 bg-emerald-400/10 text-emerald-300"
          >
            长期有效
          </span>
          <span
            v-else-if="previewDaysLeft !== null"
            :class="[
              'font-mono text-[10.5px] px-2 py-0.5 rounded-full border',
              previewDaysLeft <= 0
                ? 'border-rose-400/40 bg-rose-500/15 text-rose-300'
                : previewDaysLeft <= 14
                  ? 'border-amber-400/35 bg-amber-400/10 text-amber-300'
                  : 'border-emerald-400/25 bg-emerald-400/10 text-emerald-300',
            ]"
          >
            {{ previewDaysLeft <= 0 ? `已逾期 ${Math.abs(previewDaysLeft)} 天` : `预计距今剩余 ${previewDaysLeft} 天` }}
          </span>
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>到期类型</span>
            <CustomSelect v-model="dueMode" :options="DUE_MODE_OPTIONS" aria-label="到期类型" />
          </div>
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>临期预警梯度</span>
            <CustomSelect v-model="warnDays" :options="WARN_OPTIONS" aria-label="临期预警梯度" />
          </div>

          <div v-if="dueMode === 'date'" class="sm:col-span-2 flex flex-col gap-2">
            <div class="grid grid-cols-3 gap-2.5">
              <div class="flex flex-col gap-1 text-xs text-slate-300">
                <span>到期年份</span>
                <CustomSelect v-model="dueYear" :options="YEAR_OPTIONS" aria-label="到期年份" />
              </div>
              <div class="flex flex-col gap-1 text-xs text-slate-300">
                <span>到期月份</span>
                <CustomSelect v-model="dueMonth" :options="MONTH_OPTIONS" aria-label="到期月份" />
              </div>
              <div class="flex flex-col gap-1 text-xs text-slate-300">
                <span>到期日期</span>
                <CustomSelect v-model="dueDay" :options="dayOptions" aria-label="到期日期" />
              </div>
            </div>
            <div class="flex items-center gap-1.5 flex-wrap">
              <span class="text-[11px] text-slate-500">一键顺延:</span>
              <button type="button" @click="extendDueDate(1)" class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] text-[11px] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer">+1 个月</button>
              <button type="button" @click="extendDueDate(3)" class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] text-[11px] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer">+1 季度</button>
              <button type="button" @click="extendDueDate(12)" class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] text-[11px] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer">+1 年</button>
            </div>
          </div>

          <div class="sm:col-span-2">
            <ToggleSwitch
              v-model="autoRenew"
              label="已开启自动续费扣款"
              hint="开启后临期预警将提示核对自动扣款授权"
            />
          </div>
        </div>
      </div>

      <!-- Section 3: Extensible Custom Key-Value Properties -->
      <div class="modal-section flex flex-col gap-3">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div>
            <div class="text-xs font-semibold text-[#f2b880]">可扩展自定义属性 (Key / Value)</div>
            <p class="text-[11px] text-slate-400 mt-0.5">按需记录任意元信息（如 registrar、region、account_id、anchor_day、sync_adapter）</p>
          </div>
          <button
            type="button"
            @click="addExtraRow()"
            class="min-h-[30px] px-2.5 rounded-lg border border-[#f2b880]/35 bg-[#f2b880]/10 hover:bg-[#f2b880]/20 text-[11px] text-[#f2b880] cursor-pointer"
          >
            + 添加属性
          </button>
        </div>

        <div v-if="extraRows.length === 0" class="flex flex-col gap-2 py-1">
          <span class="text-[11px] text-slate-500">暂无扩展属性，点击右上角「+ 添加属性」或点击下方常用字段名快速添加：</span>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="k in ['host', 'region', 'registrar', 'panel_url', 'account']"
              :key="k"
              type="button"
              @click="addExtraRow(k, '')"
              class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] font-mono text-[10.5px] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer"
            >
              + {{ k }}
            </button>
          </div>
        </div>

        <div v-else class="flex flex-col gap-2">
          <div
            v-for="(row, idx) in extraRows"
            :key="idx"
            class="grid grid-cols-[1fr_1.3fr_auto] gap-2 items-center"
          >
            <input
              v-model="row.key"
              type="text"
              placeholder="键名 (如 region)"
              class="min-h-[36px] rounded-lg border border-[#fff8ee]/12 bg-[#050811] px-2.5 py-1 font-mono text-xs text-white focus:border-[#f2b880]/60"
            />
            <div class="flex items-center gap-1.5 min-w-0">
              <input
                v-model="row.value"
                type="text"
                placeholder="属性值"
                class="flex-1 min-w-0 min-h-[36px] rounded-lg border border-[#fff8ee]/12 bg-[#050811] px-2.5 py-1 font-mono text-xs text-white focus:border-[#f2b880]/60"
              />
              <a
                v-if="/^https?:\/\//i.test(row.value.trim())"
                :href="row.value.trim()"
                target="_blank"
                rel="noopener noreferrer"
                title="在新窗口打开此链接"
                class="px-2 py-1.5 rounded-lg border border-cyan-400/35 text-[10.5px] text-cyan-200 hover:bg-cyan-500/15 shrink-0"
              >
                打开
              </a>
            </div>
            <button
              type="button"
              @click="removeExtraRow(idx)"
              aria-label="移除该属性"
              class="w-8 h-8 rounded-lg border border-rose-400/25 text-xs text-rose-300 hover:bg-rose-500/15 cursor-pointer"
            >
              删
            </button>
          </div>
        </div>
      </div>

      <div class="flex items-center justify-between pt-2 border-t border-[#fff8ee]/10">
        <button
          v-if="!isNew"
          type="button"
          @click="handleDelete"
          class="min-h-[44px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-rose-400/40 bg-rose-500/15 text-xs text-rose-300 hover:bg-rose-500/25 cursor-pointer"
        >
          删除资产
        </button>
        <span v-else class="font-mono text-[11px] text-slate-400">
          {{ dueMode === 'permanent' ? '长期有效 · 免提醒' : `到期日 ${dueYear}-${dueMonth}-${dueDay}` }}
        </span>
        <div class="flex gap-2">
          <button type="button" @click="closeModal" class="min-h-[44px] sm:min-h-[34px] px-4 py-2 rounded-xl border border-[#fff8ee]/14 text-xs text-slate-300 hover:bg-white/5 cursor-pointer">
            取消
          </button>
          <button type="submit" :disabled="saving" class="min-h-[44px] sm:min-h-[34px] px-5 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50">
            保存资产
          </button>
        </div>
      </div>
    </form>
  </BaseModal>
</template>
