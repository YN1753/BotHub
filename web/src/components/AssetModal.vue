<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { generateSlug } from '../utils/format'
import BaseModal from './BaseModal.vue'
import CustomSelect from './CustomSelect.vue'
import ToggleSwitch from './ToggleSwitch.vue'

const store = useAppStore()
const isOpen = computed(() => store.state.editingAsset !== undefined)
const isNew = computed(() => store.state.editingAsset === null)

const id = ref('')
const name = ref('')
const category = ref('compute')
const spec = ref('')
const amount = ref('')
const billingCycle = ref('年')
const status = ref('运行中')
const dueMode = ref<'date' | 'permanent'>('date')
const dueYear = ref('2027')
const dueMonth = ref('01')
const dueDay = ref('01')
const autoRenew = ref(false)
const warnDays = ref('[7, 3, 1]')
const extraValues = ref<Record<string, string>>({})
const saving = ref(false)

const CATEGORY_OPTIONS = [
  { value: 'compute', label: '算力节点 / VPS' },
  { value: 'domain', label: '核心域名资产' },
  { value: 'life', label: '生活会员订阅' },
]
const CYCLE_OPTIONS = [
  { value: '年', label: '年付 (/年)' },
  { value: '月', label: '月付 (/月)' },
  { value: '季', label: '季付 (/季)' },
  { value: '一次性', label: '永久 / 一次性' },
]
const STATUS_OPTIONS = [
  { value: '运行中', label: '运行中' },
  { value: '会员有效', label: '会员有效' },
  { value: '活动体验中', label: '活动体验中' },
  { value: '待购入', label: '待购入' },
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
const DAY_OPTIONS = Array.from({ length: 31 }, (_, i) => {
  const d = String(i + 1).padStart(2, '0')
  return { value: d, label: `${d} 日` }
})

const currentTemplateFields = computed(() => {
  const tpl = store.state.assetTemplates.find((t) => t.category === category.value)
  return tpl?.fields || []
})

watch(
  () => store.state.editingAsset,
  (item) => {
    if (item === undefined) return
    if (item === null) {
      id.value = ''
      name.value = ''
      category.value = 'compute'
      spec.value = ''
      amount.value = ''
      billingCycle.value = '年'
      status.value = '运行中'
      dueMode.value = 'date'
      dueYear.value = '2027'
      dueMonth.value = '01'
      dueDay.value = '01'
      autoRenew.value = false
      warnDays.value = '[7, 3, 1]'
      extraValues.value = {}
      return
    }
    id.value = item.id
    name.value = item.name
    category.value = item.category || 'compute'
    spec.value = item.spec || ''
    status.value = item.status || '运行中'
    autoRenew.value = Boolean(item.auto_renew)
    warnDays.value = item.warn_days || '[7, 3, 1]'

    const pParts = String(item.price || '').split('/')
    amount.value = (pParts[0] || '').trim()
    billingCycle.value = pParts[1]?.trim() || (String(item.price || '').includes('月') ? '月' : '年')

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
    try {
      const parsed = JSON.parse(item.extra_config || '{}')
      const map: Record<string, string> = {}
      Object.entries(parsed).forEach(([k, v]) => {
        map[k] = String(v ?? '')
      })
      extraValues.value = map
    } catch {
      extraValues.value = {}
    }
  }
)

function closeModal() {
  store.state.editingAsset = undefined
}

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
    const finalID = isNew.value ? id.value.trim() || generateSlug(name.value, category.value) : id.value
    const formattedPrice =
      amount.value.trim() === ''
        ? billingCycle.value
        : billingCycle.value === '一次性'
          ? `${amount.value.trim()} (一次性)`
          : `${amount.value.trim()} / ${billingCycle.value}`
    const dueDate =
      dueMode.value === 'permanent'
        ? '2099-12-31T23:59:59+08:00'
        : `${dueYear.value}-${dueMonth.value}-${dueDay.value}T23:59:59+08:00`

    const payload = {
      id: finalID,
      name: name.value.trim(),
      category: category.value,
      spec: spec.value.trim(),
      price: formattedPrice,
      status: status.value,
      due_date: dueDate,
      auto_renew: autoRenew.value,
      warn_days: warnDays.value,
      extra_config: JSON.stringify(extraValues.value),
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
  if (!window.confirm(`确定下架并删除资产 [${name.value}] 吗？`)) return
  await BotHubApi.deleteAsset(id.value)
  store.notify('资产已下架删除')
  closeModal()
  await store.loadDashboard(false)
}
</script>

<template>
  <BaseModal
    :open="isOpen"
    :title="isNew ? '登记新数字资产' : '编辑数字资产配置'"
    subtitle="维护算力节点、核心域名与订阅周期的续费及预警策略"
    max-width="2xl"
    @close="closeModal"
  >
    <form @submit.prevent="handleSubmit" class="flex-1 min-h-0 overflow-y-auto pr-1 flex flex-col gap-3.5">
      <!-- Section 1: Basic Info & Billing -->
      <div class="modal-section flex flex-col gap-3">
        <div class="text-xs font-semibold text-[#f2b880]">基础台账与费用周期</div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>资产类别</span>
            <CustomSelect v-model="category" :options="CATEGORY_OPTIONS" aria-label="资产类别" />
          </div>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>资产名称</span>
            <input v-model="name" type="text" required placeholder="如：CloudCone LA" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>配置规格 / 描述</span>
            <input v-model="spec" type="text" placeholder="如：2C 2G · 110G SSD" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60" />
          </label>
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>当前状态</span>
            <CustomSelect v-model="status" :options="STATUS_OPTIONS" aria-label="当前状态" />
          </div>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>费用金额</span>
            <input v-model="amount" type="text" placeholder="如：$21.66 或 148元" class="min-h-[44px] sm:min-h-[40px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60" />
          </label>
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>付款周期</span>
            <CustomSelect v-model="billingCycle" :options="CYCLE_OPTIONS" aria-label="付款周期" />
          </div>
        </div>
      </div>

      <!-- Section 2: Due Date & Alert Rules -->
      <div class="modal-section flex flex-col gap-3">
        <div class="text-xs font-semibold text-[#f2b880]">到期守望与续费方式</div>
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
                <CustomSelect v-model="dueDay" :options="DAY_OPTIONS" aria-label="到期日期" />
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
              label="已启用自动续费扣款"
              hint="开启后临期提醒将附带自动扣款核对提示"
            />
          </div>
        </div>
      </div>

      <!-- Section 3: Template Specific Probe Extra -->
      <div v-if="currentTemplateFields.length > 0" class="modal-section flex flex-col gap-3">
        <div class="text-xs font-semibold text-[#f2b880]">分类专有嗅探参数</div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <label
            v-for="f in currentTemplateFields"
            :key="f.key"
            class="flex flex-col gap-1 text-xs text-slate-300"
          >
            <span>{{ f.label }}</span>
            <input
              v-model="extraValues[f.key]"
              type="text"
              :placeholder="f.placeholder || ''"
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
          下架并删除
        </button>
        <span v-else />
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
