<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { generateSlug } from '../utils/format'
import CustomSelect from './CustomSelect.vue'

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
const autoRenew = ref('false')
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
const RENEW_OPTIONS = [
  { value: 'false', label: '手动续费' },
  { value: 'true', label: '自动续费扣款' },
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
      autoRenew.value = 'false'
      warnDays.value = '[7, 3, 1]'
      extraValues.value = {}
      return
    }
    id.value = item.id
    name.value = item.name
    category.value = item.category || 'compute'
    spec.value = item.spec || ''
    status.value = item.status || '运行中'
    autoRenew.value = item.auto_renew ? 'true' : 'false'
    warnDays.value = item.warn_days || '[7, 3, 1]'

    // Parse combined price like "$21.66 / 年"
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
      auto_renew: autoRenew.value === 'true',
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
  <div
    v-if="isOpen"
    class="fixed inset-0 z-50 flex items-end sm:items-center justify-center sm:p-4 bg-black/70 backdrop-blur-sm"
    @click.self="closeModal"
  >
    <div class="aero-glass w-full sm:max-w-2xl max-h-[90dvh] sm:max-h-[88vh] rounded-t-2xl rounded-b-none sm:rounded-2xl p-4 sm:p-5 pb-[max(1rem,env(safe-area-inset-bottom))] flex flex-col gap-4 shadow-2xl">
      <div class="sm:hidden w-10 h-1 rounded-full bg-[#fff8ee]/20 mx-auto -mt-1 shrink-0" />
      <div class="flex items-center justify-between gap-2">
        <h3 class="text-lg font-semibold text-white">{{ isNew ? '登记新数字资产' : '编辑数字资产配置' }}</h3>
        <button type="button" @click="closeModal" class="min-h-[40px] sm:min-h-[32px] px-3 py-1 rounded-lg border border-[#fff8ee]/14 text-xs text-slate-300 hover:bg-white/5 cursor-pointer">
          关闭
        </button>
      </div>

      <form @submit.prevent="handleSubmit" class="flex-1 min-h-0 overflow-y-auto pr-1 flex flex-col gap-3.5">
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>资产类别</span>
            <CustomSelect v-model="category" :options="CATEGORY_OPTIONS" />
          </div>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>资产名称</span>
            <input v-model="name" type="text" required placeholder="如：CloudCone LA" class="min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>配置规格 / 描述</span>
            <input v-model="spec" type="text" placeholder="如：2C 2G · 110G SSD" class="min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
          </label>
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>当前状态</span>
            <CustomSelect v-model="status" :options="STATUS_OPTIONS" />
          </div>
          <label class="flex flex-col gap-1 text-xs text-slate-300">
            <span>费用金额</span>
            <input v-model="amount" type="text" placeholder="如：$21.66 或 148元" class="min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 text-sm text-white focus:border-[#f2b880]/60 focus:outline-none" />
          </label>
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>付款周期</span>
            <CustomSelect v-model="billingCycle" :options="CYCLE_OPTIONS" />
          </div>
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>到期类型</span>
            <CustomSelect v-model="dueMode" :options="DUE_MODE_OPTIONS" />
          </div>
          <div class="flex flex-col gap-1 text-xs text-slate-300">
            <span>续费方式</span>
            <CustomSelect v-model="autoRenew" :options="RENEW_OPTIONS" />
          </div>

          <div v-if="dueMode === 'date'" class="sm:col-span-2 grid grid-cols-3 gap-2.5">
            <div class="flex flex-col gap-1 text-xs text-slate-300">
              <span>到期年份</span>
              <CustomSelect v-model="dueYear" :options="YEAR_OPTIONS" />
            </div>
            <div class="flex flex-col gap-1 text-xs text-slate-300">
              <span>到期月份</span>
              <CustomSelect v-model="dueMonth" :options="MONTH_OPTIONS" />
            </div>
            <div class="flex flex-col gap-1 text-xs text-slate-300">
              <span>到期日期</span>
              <CustomSelect v-model="dueDay" :options="DAY_OPTIONS" />
            </div>
          </div>

          <div class="sm:col-span-2 flex flex-col gap-1 text-xs text-slate-300">
            <span>临期预警梯度</span>
            <CustomSelect v-model="warnDays" :options="WARN_OPTIONS" />
          </div>

          <template v-if="currentTemplateFields.length > 0">
            <div class="sm:col-span-2 pt-2 border-t border-[#fff8ee]/10 text-xs font-semibold text-slate-200">
              分类专有嗅探参数
            </div>
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
                class="min-h-[40px] rounded-xl border border-[#fff8ee]/14 bg-[#070a16] px-3 py-2 font-mono text-sm text-white focus:border-[#f2b880]/60 focus:outline-none"
              />
            </label>
          </template>
        </div>

        <div class="flex items-center justify-between pt-3 border-t border-[#fff8ee]/10">
          <button
            v-if="!isNew"
            type="button"
            @click="handleDelete"
            class="min-h-[40px] sm:min-h-[34px] px-3.5 py-2 rounded-xl border border-rose-400/40 bg-rose-500/15 text-xs text-rose-300 cursor-pointer"
          >
            下架并删除
          </button>
          <span v-else />
          <div class="flex gap-2">
            <button type="button" @click="closeModal" class="min-h-[40px] sm:min-h-[34px] px-4 py-2 rounded-xl border border-[#fff8ee]/14 text-xs text-slate-300 hover:bg-white/5 cursor-pointer">
              取消
            </button>
            <button type="submit" :disabled="saving" class="min-h-[40px] sm:min-h-[34px] px-4 py-2 rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-50 transition">
              保存资产
            </button>
          </div>
        </div>
      </form>
    </div>
  </div>
</template>
