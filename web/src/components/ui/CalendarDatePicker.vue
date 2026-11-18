<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount, useId } from 'vue'
import {
  buildCalendarMonthGrid,
  formatPlainDate,
  formatPlainDateTime,
  pad2,
  parsePlainDateParts,
  parsePlainDateTimeParts,
  todayPlainDateParts,
} from '../../utils/calendar'

export interface DatePresetOption {
  label: string
  value: string
}

const props = withDefaults(
  defineProps<{
    modelValue: string
    mode?: 'date' | 'datetime'
    placeholder?: string
    allowClear?: boolean
    compact?: boolean
    disabled?: boolean
    ariaLabel?: string
    presets?: DatePresetOption[]
  }>(),
  {
    mode: 'date',
    placeholder: '选择日期...',
    allowClear: true,
    compact: false,
    disabled: false,
    presets: () => [],
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const uid = useId().replace(/[^a-zA-Z0-9_-]/g, '')
const popoverId = `cal-pop-${uid}`
const anchorName = `--cal-anchor-${uid}`

const WEEKDAY_LABELS = ['一', '二', '三', '四', '五', '六', '日']
const MONTH_LABELS = ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月']
const QUICK_TIMES = ['08:00', '09:00', '12:00', '18:00', '21:00']

const rootEl = ref<HTMLElement | null>(null)
const triggerEl = ref<HTMLButtonElement | null>(null)
const panelEl = ref<HTMLElement | null>(null)

const isOpen = ref(false)
const pickerView = ref<'days' | 'ym'>('days')
const fallbackPosStyle = ref<Record<string, string>>({})

const supportsNativePopover =
  typeof HTMLElement !== 'undefined' && Object.hasOwn(HTMLElement.prototype, 'popover')
const supportsAnchorArea =
  typeof CSS !== 'undefined' && typeof CSS.supports === 'function' && CSS.supports('position-area: block-end')

const initialToday = todayPlainDateParts()
const viewYear = ref(initialToday.year)
const viewMonth = ref(initialToday.month)
const timeHour = ref(9)
const timeMinute = ref(0)

const selectedDateStr = computed(() => {
  if (props.mode === 'datetime') {
    const dt = parsePlainDateTimeParts(props.modelValue)
    return dt ? formatPlainDate(dt.year, dt.month, dt.day) : ''
  }
  const d = parsePlainDateParts(props.modelValue)
  return d ? formatPlainDate(d.year, d.month, d.day) : ''
})

const displayLabel = computed(() => {
  if (!props.modelValue?.trim()) return ''
  if (props.mode === 'datetime') {
    const dt = parsePlainDateTimeParts(props.modelValue)
    return dt ? formatPlainDateTime(dt.year, dt.month, dt.day, dt.hour, dt.minute) : props.modelValue
  }
  const d = parsePlainDateParts(props.modelValue)
  return d ? formatPlainDate(d.year, d.month, d.day) : props.modelValue
})

const monthCells = computed(() => buildCalendarMonthGrid(viewYear.value, viewMonth.value))

const yearChoices = computed(() => {
  const cur = todayPlainDateParts().year
  const start = Math.min(viewYear.value - 2, cur - 2)
  return Array.from({ length: 12 }, (_, idx) => start + idx)
})

function syncViewFromModel() {
  if (props.mode === 'datetime') {
    const dt = parsePlainDateTimeParts(props.modelValue)
    if (dt) {
      viewYear.value = dt.year
      viewMonth.value = dt.month
      timeHour.value = dt.hour
      timeMinute.value = dt.minute
      return
    }
  } else {
    const d = parsePlainDateParts(props.modelValue)
    if (d) {
      viewYear.value = d.year
      viewMonth.value = d.month
      return
    }
  }
  const today = todayPlainDateParts()
  viewYear.value = today.year
  viewMonth.value = today.month
}

function updateFallbackCoords() {
  if (supportsAnchorArea) {
    fallbackPosStyle.value = {}
    return
  }
  const trigger = triggerEl.value
  if (!trigger) return
  const rect = trigger.getBoundingClientRect()
  const panelW = Math.min(312, window.innerWidth - 16)
  const panelH = props.mode === 'datetime' ? 390 : 330
  const left = Math.max(8, Math.min(rect.left, window.innerWidth - panelW - 8))
  const spaceBelow = window.innerHeight - rect.bottom
  const openAbove = spaceBelow < panelH + 12 && rect.top > spaceBelow
  const top = openAbove
    ? Math.max(8, rect.top - panelH - 6)
    : Math.min(window.innerHeight - panelH - 8, rect.bottom + 6)
  fallbackPosStyle.value = {
    position: 'fixed',
    left: `${Math.round(left)}px`,
    top: `${Math.round(top)}px`,
    width: `${panelW}px`,
    margin: '0px',
  }
}

async function openPanel() {
  if (props.disabled) return
  syncViewFromModel()
  pickerView.value = 'days'
  updateFallbackCoords()
  if (supportsNativePopover && panelEl.value) {
    try {
      if (!panelEl.value.matches(':popover-open')) {
        panelEl.value.showPopover()
      }
      isOpen.value = true
      await nextTick()
      updateFallbackCoords()
      return
    } catch {
      // fall back to state toggle
    }
  }
  isOpen.value = true
}

function closePanel() {
  if (supportsNativePopover && panelEl.value) {
    try {
      if (panelEl.value.matches(':popover-open')) {
        panelEl.value.hidePopover()
      }
    } catch {
      // ignore
    }
  }
  isOpen.value = false
}

function togglePanel() {
  if (isOpen.value) closePanel()
  else void openPanel()
}

function stepMonth(delta: number) {
  let m = viewMonth.value + delta
  let y = viewYear.value
  while (m < 1) {
    m += 12
    y -= 1
  }
  while (m > 12) {
    m -= 12
    y += 1
  }
  viewYear.value = y
  viewMonth.value = m
}

function pickDay(dateStr: string) {
  if (props.mode === 'datetime') {
    const parsed = parsePlainDateParts(dateStr)
    if (parsed) {
      emit(
        'update:modelValue',
        formatPlainDateTime(parsed.year, parsed.month, parsed.day, timeHour.value, timeMinute.value)
      )
      viewYear.value = parsed.year
      viewMonth.value = parsed.month
    }
    return
  }
  emit('update:modelValue', dateStr)
  closePanel()
}

function pickToday() {
  const t = todayPlainDateParts()
  const dStr = formatPlainDate(t.year, t.month, t.day)
  viewYear.value = t.year
  viewMonth.value = t.month
  if (props.mode === 'datetime') {
    emit('update:modelValue', formatPlainDateTime(t.year, t.month, t.day, timeHour.value, timeMinute.value))
  } else {
    emit('update:modelValue', dStr)
    closePanel()
  }
}

function applyPreset(val: string) {
  emit('update:modelValue', val)
  if (props.mode === 'datetime') {
    const dt = parsePlainDateTimeParts(val)
    if (dt) {
      viewYear.value = dt.year
      viewMonth.value = dt.month
      timeHour.value = dt.hour
      timeMinute.value = dt.minute
    }
  } else {
    closePanel()
  }
}

function adjustTime(hourDelta: number, minDelta: number) {
  timeHour.value = (timeHour.value + hourDelta + 24) % 24
  timeMinute.value = (timeMinute.value + minDelta + 60) % 60
  const base = parsePlainDateParts(selectedDateStr.value) || {
    year: viewYear.value,
    month: viewMonth.value,
    day: todayPlainDateParts().day,
  }
  emit(
    'update:modelValue',
    formatPlainDateTime(base.year, base.month, base.day, timeHour.value, timeMinute.value)
  )
}

function setQuickClock(hhmm: string) {
  const [h, m] = hhmm.split(':').map(Number)
  timeHour.value = h
  timeMinute.value = m
  const base = parsePlainDateParts(selectedDateStr.value) || todayPlainDateParts()
  emit('update:modelValue', formatPlainDateTime(base.year, base.month, base.day, h, m))
}

function clearValue() {
  emit('update:modelValue', '')
  closePanel()
}

function handleNativeToggle(e: Event) {
  const ev = e as ToggleEvent
  isOpen.value = ev.newState === 'open'
  if (isOpen.value) {
    syncViewFromModel()
    updateFallbackCoords()
  }
}

function handleOutsideClick(e: MouseEvent) {
  if (!isOpen.value || supportsNativePopover) return
  if (rootEl.value && !rootEl.value.contains(e.target as Node)) {
    closePanel()
  }
}

watch(isOpen, (val) => {
  if (val) window.addEventListener('resize', updateFallbackCoords)
  else window.removeEventListener('resize', updateFallbackCoords)
})

onMounted(() => document.addEventListener('click', handleOutsideClick))
onBeforeUnmount(() => {
  document.removeEventListener('click', handleOutsideClick)
  window.removeEventListener('resize', updateFallbackCoords)
})
</script>

<template>
  <div ref="rootEl" class="relative inline-block w-full">
    <button
      ref="triggerEl"
      type="button"
      :disabled="disabled"
      :aria-expanded="isOpen"
      :aria-controls="popoverId"
      :aria-label="ariaLabel || displayLabel || placeholder"
      :style="supportsAnchorArea ? { anchorName } : undefined"
      @click="togglePanel"
      :class="[
        'btn-tactile w-full flex items-center justify-between gap-2 rounded-xl text-left cursor-pointer disabled:opacity-45 disabled:cursor-not-allowed',
        isOpen ? 'border-[#f2b880]/60 text-white' : 'text-slate-100',
        compact ? 'px-2.5 py-1.5 text-xs min-h-[38px] sm:min-h-[36px]' : 'px-3 py-2 text-sm min-h-[44px] sm:min-h-[40px]'
      ]"
    >
      <span class="flex items-center gap-2 min-w-0">
        <svg
          viewBox="0 0 24 24"
          width="14"
          height="14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.9"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
          class="shrink-0 text-[#f2b880]"
        >
          <rect x="3" y="4" width="18" height="18" rx="2.5" />
          <line x1="16" y1="2" x2="16" y2="6" />
          <line x1="8" y1="2" x2="8" y2="6" />
          <line x1="3" y1="10" x2="21" y2="10" />
        </svg>
        <span :class="['truncate font-mono', displayLabel ? '' : 'text-slate-400 font-sans']">
          {{ displayLabel || placeholder }}
        </span>
      </span>
      <span
        v-if="allowClear && displayLabel"
        role="button"
        tabindex="0"
        title="清除日期"
        aria-label="清除日期"
        @click.stop="clearValue"
        @keydown.enter.stop.prevent="clearValue"
        class="shrink-0 px-1.5 py-0.5 rounded-md text-[10.5px] text-slate-400 hover:text-rose-300 hover:bg-rose-500/12"
      >
        清除
      </span>
    </button>

    <div
      :id="popoverId"
      ref="panelEl"
      :popover="supportsNativePopover ? 'auto' : undefined"
      v-show="supportsNativePopover || isOpen"
      :class="[
        'calendar-popover aero-glass rounded-2xl p-3 shadow-2xl',
        supportsAnchorArea ? 'calendar-popover-anchored' : '',
        !supportsNativePopover ? 'fixed z-[120]' : ''
      ]"
      :style="{
        ...(supportsAnchorArea ? { positionAnchor: anchorName } : fallbackPosStyle),
      }"
      @toggle="handleNativeToggle"
      @pointerdown.stop
      @pointerup.stop
      @click.stop
    >
      <!-- Calendar Header -->
      <div class="flex items-center justify-between gap-1 mb-2.5">
        <div class="flex items-center gap-0.5">
          <button
            type="button"
            aria-label="上一年"
            @click="viewYear -= 1"
            class="cal-nav-btn w-7 h-7 rounded-lg inline-flex items-center justify-center text-xs cursor-pointer"
          >
            «
          </button>
          <button
            type="button"
            aria-label="上一月"
            @click="stepMonth(-1)"
            class="cal-nav-btn w-7 h-7 rounded-lg inline-flex items-center justify-center text-xs cursor-pointer"
          >
            ‹
          </button>
        </div>

        <button
          type="button"
          @click="pickerView = pickerView === 'days' ? 'ym' : 'days'"
          class="px-2.5 py-1 rounded-lg font-mono text-xs font-semibold hover:bg-[#f2b880]/15 text-[#f2b880] cursor-pointer transition-colors"
        >
          {{ viewYear }} 年 {{ pad2(viewMonth) }} 月 {{ pickerView === 'ym' ? '▴' : '▾' }}
        </button>

        <div class="flex items-center gap-0.5">
          <button
            type="button"
            aria-label="下一月"
            @click="stepMonth(1)"
            class="cal-nav-btn w-7 h-7 rounded-lg inline-flex items-center justify-center text-xs cursor-pointer"
          >
            ›
          </button>
          <button
            type="button"
            aria-label="下一年"
            @click="viewYear += 1"
            class="cal-nav-btn w-7 h-7 rounded-lg inline-flex items-center justify-center text-xs cursor-pointer"
          >
            »
          </button>
        </div>
      </div>

      <!-- Fast Year & Month Selector -->
      <div v-if="pickerView === 'ym'" class="flex flex-col gap-2.5 py-1">
        <div class="grid grid-cols-4 gap-1.5">
          <button
            v-for="y in yearChoices"
            :key="y"
            type="button"
            @click="viewYear = y"
            :class="[
              'py-1.5 rounded-lg font-mono text-xs cursor-pointer transition-colors',
              viewYear === y ? 'cal-cell-selected font-semibold' : 'cal-nav-btn'
            ]"
          >
            {{ y }}
          </button>
        </div>
        <div class="grid grid-cols-4 gap-1.5 pt-1 border-t border-[#fff8ee]/10">
          <button
            v-for="(mLabel, mIdx) in MONTH_LABELS"
            :key="mLabel"
            type="button"
            @click="viewMonth = mIdx + 1; pickerView = 'days'"
            :class="[
              'py-1.5 rounded-lg text-xs cursor-pointer transition-colors',
              viewMonth === mIdx + 1 ? 'cal-cell-selected font-semibold' : 'cal-nav-btn'
            ]"
          >
            {{ mLabel }}
          </button>
        </div>
      </div>

      <!-- Days Grid -->
      <div v-else class="flex flex-col gap-1">
        <div class="grid grid-cols-7 gap-1 text-center text-[10.5px] font-medium text-slate-400 pb-1">
          <span v-for="w in WEEKDAY_LABELS" :key="w">{{ w }}</span>
        </div>
        <div class="grid grid-cols-7 gap-1">
          <button
            v-for="cell in monthCells"
            :key="cell.dateStr"
            type="button"
            @click="pickDay(cell.dateStr)"
            :class="[
              'cal-day-cell h-8 rounded-lg font-mono text-xs inline-flex items-center justify-center cursor-pointer transition-all',
              cell.dateStr === selectedDateStr
                ? 'cal-cell-selected font-bold'
                : cell.isToday
                  ? 'cal-cell-today font-semibold'
                  : cell.inMonth
                    ? 'cal-cell-inmonth'
                    : 'cal-cell-outmonth'
            ]"
          >
            {{ cell.day }}
          </button>
        </div>
      </div>

      <!-- Time Picker Bar for DateTime mode -->
      <div
        v-if="mode === 'datetime'"
        class="mt-2.5 pt-2.5 border-t border-[#fff8ee]/10 flex flex-col gap-2"
      >
        <div class="flex items-center justify-between gap-2">
          <span class="text-[11px] text-slate-400">触发时刻 (上海时间)</span>
          <div class="flex items-center gap-1 font-mono text-xs">
            <button type="button" @click="adjustTime(-1, 0)" class="cal-nav-btn w-6 h-6 rounded-md cursor-pointer">-</button>
            <span class="px-1.5 font-semibold text-white">{{ pad2(timeHour) }}</span>
            <button type="button" @click="adjustTime(1, 0)" class="cal-nav-btn w-6 h-6 rounded-md cursor-pointer">+</button>
            <span class="text-slate-400">:</span>
            <button type="button" @click="adjustTime(0, -5)" class="cal-nav-btn w-6 h-6 rounded-md cursor-pointer">-</button>
            <span class="px-1.5 font-semibold text-white">{{ pad2(timeMinute) }}</span>
            <button type="button" @click="adjustTime(0, 5)" class="cal-nav-btn w-6 h-6 rounded-md cursor-pointer">+</button>
          </div>
        </div>
        <div class="flex items-center gap-1 flex-wrap">
          <button
            v-for="qt in QUICK_TIMES"
            :key="qt"
            type="button"
            @click="setQuickClock(qt)"
            class="cal-preset-chip px-2 py-0.5 rounded-md font-mono text-[10.5px] cursor-pointer"
          >
            {{ qt }}
          </button>
        </div>
      </div>

      <!-- Footer Quick Actions -->
      <div class="mt-2.5 pt-2 border-t border-[#fff8ee]/10 flex items-center justify-between gap-1.5 flex-wrap">
        <div class="flex items-center gap-1 flex-wrap">
          <button
            type="button"
            @click="pickToday"
            class="cal-preset-chip px-2 py-0.5 rounded-md text-[11px] cursor-pointer"
          >
            今天
          </button>
          <button
            v-for="p in presets"
            :key="p.label"
            type="button"
            @click="applyPreset(p.value)"
            class="cal-preset-chip px-2 py-0.5 rounded-md text-[11px] cursor-pointer"
          >
            {{ p.label }}
          </button>
        </div>
        <button
          type="button"
          @click="closePanel"
          class="px-2.5 py-0.5 rounded-md bg-[#f2b880] text-[#070a13] font-semibold text-[11px] cursor-pointer"
        >
          确定
        </button>
      </div>
    </div>
  </div>
</template>
