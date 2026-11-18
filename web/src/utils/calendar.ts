// Location-agnostic calendar date & wall-clock datetime helpers.
// Uses native Temporal.PlainDate / Temporal.PlainDateTime when available, with identical
// integer-tuple calendar math fallback so Safari / Firefox behave with zero timezone drift.

export interface PlainDateParts {
  year: number
  month: number // 1..12
  day: number // 1..31
}

export interface PlainDateTimeParts extends PlainDateParts {
  hour: number // 0..23
  minute: number // 0..59
}

export interface CalendarCell {
  dateStr: string
  year: number
  month: number
  day: number
  inMonth: boolean
  isToday: boolean
}

type TemporalLike = {
  Now?: {
    plainDateISO?: () => { year: number; month: number; day: number }
  }
  PlainDate?: {
    from?: (v: string | { year: number; month: number; day: number }) => {
      year: number
      month: number
      day: number
      add: (d: { days?: number; months?: number }, opts?: { overflow?: 'constrain' | 'reject' }) => {
        year: number
        month: number
        day: number
        toString: () => string
      }
      toString: () => string
    }
  }
}

function getTemporal(): TemporalLike | null {
  if (typeof globalThis === 'undefined') return null
  return ((globalThis as unknown as { Temporal?: TemporalLike }).Temporal ?? null)
}

export function pad2(n: number): string {
  return String(Math.max(0, Math.floor(n))).padStart(2, '0')
}

export function daysInMonth(year: number, month: number): number {
  if (month === 2) {
    const leap = (year % 4 === 0 && year % 100 !== 0) || year % 400 === 0
    return leap ? 29 : 28
  }
  if (month === 4 || month === 6 || month === 9 || month === 11) return 30
  return 31
}

export function formatPlainDate(year: number, month: number, day: number): string {
  return `${String(year).padStart(4, '0')}-${pad2(month)}-${pad2(day)}`
}

export function formatPlainDateTime(
  year: number,
  month: number,
  day: number,
  hour: number,
  minute: number
): string {
  return `${formatPlainDate(year, month, day)} ${pad2(hour)}:${pad2(minute)}`
}

export function todayPlainDateParts(): PlainDateParts {
  const temporal = getTemporal()
  if (temporal?.Now?.plainDateISO) {
    try {
      const pd = temporal.Now.plainDateISO()
      return { year: pd.year, month: pd.month, day: pd.day }
    } catch {
      // fall through
    }
  }
  const now = new Date()
  return {
    year: now.getFullYear(),
    month: now.getMonth() + 1,
    day: now.getDate(),
  }
}

export function todayPlainDate(): string {
  const t = todayPlainDateParts()
  return formatPlainDate(t.year, t.month, t.day)
}

export function parsePlainDateParts(raw?: string): PlainDateParts | null {
  const txt = String(raw || '').trim()
  const m = txt.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (!m) return null
  const year = Number(m[1])
  const month = Number(m[2])
  const day = Number(m[3])
  if (!Number.isInteger(year) || month < 1 || month > 12) return null
  const maxDay = daysInMonth(year, month)
  if (day < 1 || day > maxDay) return null
  return { year, month, day }
}

export function parsePlainDateTimeParts(raw?: string): PlainDateTimeParts | null {
  const txt = String(raw || '').trim()
  const m = txt.match(/^(\d{4})-(\d{2})-(\d{2})(?:[T\s](\d{2}):(\d{2}))?/)
  if (!m) return null
  const base = parsePlainDateParts(`${m[1]}-${m[2]}-${m[3]}`)
  if (!base) return null
  const hour = m[4] !== undefined ? Number(m[4]) : 9
  const minute = m[5] !== undefined ? Number(m[5]) : 0
  if (hour < 0 || hour > 23 || minute < 0 || minute > 59) return null
  return { ...base, hour, minute }
}

export function addDaysPlain(dateStr: string, days: number): string {
  const parsed = parsePlainDateParts(dateStr) || todayPlainDateParts()
  const temporal = getTemporal()
  if (temporal?.PlainDate?.from) {
    try {
      return temporal.PlainDate.from(parsed).add({ days }).toString()
    } catch {
      // fall through
    }
  }
  // Howard Hinnant civil-from-days / local UTC tuple free of local DST gaps
  const d = new Date(Date.UTC(parsed.year, parsed.month - 1, parsed.day + days))
  return formatPlainDate(d.getUTCFullYear(), d.getUTCMonth() + 1, d.getUTCDate())
}

export function addMonthsPlain(dateStr: string, months: number): string {
  const parsed = parsePlainDateParts(dateStr) || todayPlainDateParts()
  const temporal = getTemporal()
  if (temporal?.PlainDate?.from) {
    try {
      return temporal.PlainDate.from(parsed).add({ months }, { overflow: 'constrain' }).toString()
    } catch {
      // fall through
    }
  }
  const zeroBasedMonth = parsed.month - 1 + months
  const year = parsed.year + Math.floor(zeroBasedMonth / 12)
  const month = (((zeroBasedMonth % 12) + 12) % 12) + 1
  const day = Math.min(parsed.day, daysInMonth(year, month))
  return formatPlainDate(year, month, day)
}

export function addMinutesPlainDateTime(addMinutes: number, presetHour?: number): string {
  const now = new Date()
  if (presetHour !== undefined) {
    const baseStr = formatPlainDate(now.getFullYear(), now.getMonth() + 1, now.getDate())
    const nextDay = addDaysPlain(baseStr, 1)
    return `${nextDay} ${pad2(presetHour)}:00`
  }
  const future = new Date(now.getTime() + addMinutes * 60_000)
  return formatPlainDateTime(
    future.getFullYear(),
    future.getMonth() + 1,
    future.getDate(),
    future.getHours(),
    future.getMinutes()
  )
}

export function diffPlainDaysFromToday(dateStr: string): number | null {
  const target = parsePlainDateParts(dateStr)
  if (!target) return null
  const today = todayPlainDateParts()
  const targetUtc = Date.UTC(target.year, target.month - 1, target.day)
  const todayUtc = Date.UTC(today.year, today.month - 1, today.day)
  return Math.round((targetUtc - todayUtc) / 86_400_000)
}

// ISO weekday index: Monday = 0 .. Sunday = 6
function weekdayMondayFirst(year: number, month: number, day: number): number {
  const jsDay = new Date(Date.UTC(year, month - 1, day)).getUTCDay() // 0=Sun..6=Sat
  return (jsDay + 6) % 7
}

export function buildCalendarMonthGrid(viewYear: number, viewMonth: number): CalendarCell[] {
  const today = todayPlainDate()
  const leadingCount = weekdayMondayFirst(viewYear, viewMonth, 1)
  const currentMonthDays = daysInMonth(viewYear, viewMonth)

  const prevMonth = viewMonth === 1 ? 12 : viewMonth - 1
  const prevYear = viewMonth === 1 ? viewYear - 1 : viewYear
  const prevMonthDays = daysInMonth(prevYear, prevMonth)

  const nextMonth = viewMonth === 12 ? 1 : viewMonth + 1
  const nextYear = viewMonth === 12 ? viewYear + 1 : viewYear

  const cells: CalendarCell[] = []
  for (let i = leadingCount - 1; i >= 0; i--) {
    const day = prevMonthDays - i
    const dateStr = formatPlainDate(prevYear, prevMonth, day)
    cells.push({
      dateStr,
      year: prevYear,
      month: prevMonth,
      day,
      inMonth: false,
      isToday: dateStr === today,
    })
  }
  for (let day = 1; day <= currentMonthDays; day++) {
    const dateStr = formatPlainDate(viewYear, viewMonth, day)
    cells.push({
      dateStr,
      year: viewYear,
      month: viewMonth,
      day,
      inMonth: true,
      isToday: dateStr === today,
    })
  }
  let trailDay = 1
  while (cells.length < 42) {
    const dateStr = formatPlainDate(nextYear, nextMonth, trailDay)
    cells.push({
      dateStr,
      year: nextYear,
      month: nextMonth,
      day: trailDay,
      inMonth: false,
      isToday: dateStr === today,
    })
    trailDay++
  }
  return cells
}
