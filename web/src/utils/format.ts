function pad2(n: string | number): string {
  return String(n).padStart(2, '0')
}

export function parseJsonStringArray(raw?: unknown): string[] {
  if (Array.isArray(raw)) return raw.map((x) => String(x))
  if (typeof raw !== 'string' || !raw.trim()) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.map((x) => String(x)) : []
  } catch {
    return []
  }
}

export function parseJsonRecord(raw?: unknown): Record<string, string> {
  if (typeof raw !== 'string' || !raw.trim()) return {}
  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    const out: Record<string, string> = {}
    for (const [k, v] of Object.entries(parsed)) {
      out[k] = String(v ?? '')
    }
    return out
  } catch {
    return {}
  }
}

export function formatLocalDate(date: Date = new Date()): string {
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`
}

export function escapeHtml(s: unknown): string {
  return String(s ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
}

// Neutralize leading spreadsheet formula characters (=, +, -, @, tab, CR) unless it's a pure signed number
export function csvEscapeCell(raw: unknown): string {
  let str = String(raw ?? '')
  if (/^[=+\-@\t\r]/.test(str) && !/^[-+]?\d+(?:\.\d+)?$/.test(str.trim())) {
    str = `'${str}`
  }
  return `"${str.replaceAll('"', '""')}"`
}

export function formatTime(str?: string): string {
  if (!str) return '--'
  const d = new Date(str)
  if (Number.isNaN(d.getTime())) return str
  return `${formatLocalDate(d)} ${pad2(d.getHours())}:${pad2(d.getMinutes())}`
}

export function formatRelativeTime(str?: string): string {
  if (!str) return '--'
  const d = new Date(str)
  if (Number.isNaN(d.getTime())) return str
  const diffSec = Math.floor((Date.now() - d.getTime()) / 1000)
  if (diffSec < 0) return formatTime(str)
  if (diffSec < 60) return '刚刚'
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)} 分钟前`
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} 小时前`
  return formatChatTime(str) || formatTime(str)
}

export function formatDurationMs(ms?: number): string {
  if (ms === undefined || ms === null || Number.isNaN(ms)) return ''
  if (ms < 1000) return `${Math.round(ms)}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.floor(ms / 60_000)}m${Math.round((ms % 60_000) / 1000)}s`
}

export function formatUptime(sec?: number): string {
  const s = Math.max(0, Math.floor(sec || 0))
  if (s < 60) return `${s}s`
  const days = Math.floor(s / 86400)
  const hours = Math.floor((s % 86400) / 3600)
  const mins = Math.floor((s % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

export function truncateText(raw?: string, maxLen = 60): string {
  const clean = String(raw || '').trim()
  if (!clean || maxLen <= 0 || clean.length <= maxLen) return clean
  return `${clean.slice(0, maxLen)}...`
}

export function formatDateOnly(str?: string): string {
  if (!str || str.startsWith('2099')) return '长期有效'
  const s = str.trim()
  const m = s.match(/^(\d{4}-\d{2}-\d{2})/)
  if (m) return m[1]
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  return formatLocalDate(d)
}

export function generateSlug(text: string, prefix = 'item'): string {
  const clean = text
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '_')
    .replace(/^_+|_+$/g, '')
  return clean || `${prefix}_${Date.now().toString(36).slice(-4)}`
}

// Append anchor to document.body & defer revokeObjectURL so Safari downloads trigger reliably
export function triggerBlobDownload(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.style.display = 'none'
  document.body.appendChild(a)
  a.click()
  a.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

export function describeCron(expr?: string): string {
  const trimmed = String(expr || '').trim()
  if (!trimmed) return '未指定调度策略'
  const parts = trimmed.split(/\s+/)
  if (parts.length !== 5) return `表达式: ${trimmed}`

  const [min, hour, dom, mon, dow] = parts

  if (min === '0' && hour === '*' && dom === '*' && mon === '*' && dow === '*') {
    return '每小时整点执行一次'
  }
  if (min.startsWith('*/') && hour === '*' && dom === '*' && mon === '*' && dow === '*') {
    return `每隔 ${min.slice(2)} 分钟轮询一次`
  }
  if (min === '0' && hour.startsWith('*/') && dom === '*' && mon === '*' && dow === '*') {
    return `每隔 ${hour.slice(2)} 小时整点执行`
  }
  if (!Number.isNaN(Number(min)) && !Number.isNaN(Number(hour)) && dom === '*' && mon === '*' && dow === '*') {
    const h = parseInt(hour, 10)
    const period = h < 6 ? '凌晨' : h < 12 ? '早间' : h < 14 ? '午间' : h < 18 ? '下午' : '晚间'
    return `每天 ${period} ${pad2(hour)}:${pad2(min)} 执行`
  }
  if (!Number.isNaN(Number(min)) && !Number.isNaN(Number(hour)) && dom === '*' && mon === '*' && (dow === '1-5' || dow === '1,2,3,4,5')) {
    return `工作日 (周一至五) ${pad2(hour)}:${pad2(min)} 执行`
  }
  if (!Number.isNaN(Number(min)) && !Number.isNaN(Number(hour)) && dom === '*' && mon === '*' && (dow === '6,0' || dow === '0,6' || dow === '6,7')) {
    return `周末 (周六日) ${pad2(hour)}:${pad2(min)} 执行`
  }
  const weekMap: Record<string, string> = { '0': '周日', '1': '周一', '2': '周二', '3': '周三', '4': '周四', '5': '周五', '6': '周六', '7': '周日' }
  if (!Number.isNaN(Number(min)) && !Number.isNaN(Number(hour)) && dom === '*' && mon === '*' && weekMap[dow]) {
    return `每${weekMap[dow]} ${pad2(hour)}:${pad2(min)} 执行`
  }
  if (!Number.isNaN(Number(min)) && !Number.isNaN(Number(hour)) && !Number.isNaN(Number(dom)) && mon === '*' && dow === '*') {
    return `每月 ${pad2(dom)} 日 ${pad2(hour)}:${pad2(min)} 执行`
  }
  if (!Number.isNaN(Number(min)) && hour.includes(',') && dom === '*' && mon === '*' && dow === '*') {
    return `每天 ${hour.split(',').map((h) => `${pad2(h)}:${pad2(min)}`).join(' 与 ')} 各执行一次`
  }
  return `定时 Cron: ${trimmed}`
}

export function botStateLabel(state?: string): string {
  const map: Record<string, string> = {
    idle: '待命',
    thinking: '思考中',
    working: '执行中',
    done: '新汇报',
    blocked: '告警',
  }
  return map[String(state || 'idle')] || '待命'
}

export function botStateDotClass(state?: string): string {
  if (state === 'thinking') return 'bg-warn animate-pulse'
  if (state === 'working') return 'bg-accent animate-pulse'
  if (state === 'done') return 'bg-ok'
  if (state === 'blocked') return 'bg-danger'
  return 'bg-fg-muted'
}

export const DEFAULT_FX_RATES_CNY: Record<string, number> = {
  CNY: 1,
  RMB: 1,
  USD: 7.24,
  EUR: 7.82,
  HKD: 0.93,
  GBP: 9.18,
  JPY: 0.0475,
  TWD: 0.224,
  SGD: 5.38,
  CAD: 5.16,
  AUD: 4.7,
  KRW: 0.0052,
}

export function detectPriceCurrencyCode(raw: string): string {
  const s = String(raw || '').trim()
  const upper = s.toUpperCase()
  if (upper.includes('HK$') || upper.includes('HKD') || s.includes('港币') || s.includes('港元')) return 'HKD'
  if (upper.includes('NT$') || upper.includes('TWD') || s.includes('台币')) return 'TWD'
  if (upper.includes('S$') || upper.includes('SGD') || s.includes('新币')) return 'SGD'
  if (upper.includes('C$') || upper.includes('CAD') || s.includes('加币')) return 'CAD'
  if (upper.includes('A$') || upper.includes('AUD') || s.includes('澳币')) return 'AUD'
  if (upper.includes('JPY') || s.includes('日元') || s.includes('円')) return 'JPY'
  if (s.includes('€') || upper.includes('EUR') || s.includes('欧元')) return 'EUR'
  if (s.includes('£') || upper.includes('GBP') || s.includes('英镑')) return 'GBP'
  if (s.includes('$') || upper.includes('USD') || s.includes('美元') || s.includes('刀')) return 'USD'
  return 'CNY'
}

function resolveCNYRate(code: string, fxRates?: Record<string, number>): number {
  const upper = String(code || 'CNY').toUpperCase()
  const custom = fxRates?.[upper]
  if (typeof custom === 'number' && Number.isFinite(custom) && custom > 0) return custom
  return DEFAULT_FX_RATES_CNY[upper] ?? 1
}

export function summarizeAssetsMonthlyCost(
  assets: Array<{ status?: string; price?: string }>,
  fxRates?: Record<string, number>
): string {
  let totalCNY = 0
  let count = 0
  for (const a of assets) {
    if (a.status === '已停用' || a.status === '已归档') continue
    const raw = String(a.price || '').trim()
    if (!raw || raw.includes('免费') || raw.includes('一次') || raw.includes('永久')) continue
    const m = raw.match(/(\d+(?:\.\d+)?)/)
    if (!m) continue
    const num = Number(m[1])
    if (!Number.isFinite(num) || num <= 0) continue
    const code = detectPriceCurrencyCode(raw)
    const cnyAmt = num * resolveCNYRate(code, fxRates)
    let monthly = cnyAmt
    const lower = raw.toLowerCase()
    if (raw.includes('年') || lower.includes('/yr') || lower.includes('year') || lower.includes('/y')) {
      monthly = cnyAmt / 12
    } else if (raw.includes('季') || lower.includes('/qtr') || lower.includes('quarter')) {
      monthly = cnyAmt / 3
    } else if (raw.includes('周') || lower.includes('/wk') || lower.includes('week')) {
      monthly = cnyAmt * 4.33
    } else if (raw.includes('天') || raw.includes('日') || lower.includes('/day') || lower.includes('/d')) {
      monthly = cnyAmt * 30
    }
    totalCNY += monthly
    count++
  }
  if (count === 0 || totalCNY <= 0) return ''
  return `¥${totalCNY.toFixed(1).replace(/\.0$/, '')}/月`
}

export function summarizeAssetsYearlyCost(
  assets: Array<{ status?: string; price?: string }>,
  fxRates?: Record<string, number>
): string {
  let totalCNY = 0
  let count = 0
  for (const a of assets) {
    if (a.status === '已停用' || a.status === '已归档') continue
    const raw = String(a.price || '').trim()
    if (!raw || raw.includes('免费') || raw.includes('一次') || raw.includes('永久')) continue
    const m = raw.match(/(\d+(?:\.\d+)?)/)
    if (!m) continue
    const num = Number(m[1])
    if (!Number.isFinite(num) || num <= 0) continue
    const code = detectPriceCurrencyCode(raw)
    const cnyAmt = num * resolveCNYRate(code, fxRates)
    let yearly = cnyAmt * 12
    const lower = raw.toLowerCase()
    if (raw.includes('年') || lower.includes('/yr') || lower.includes('year') || lower.includes('/y')) {
      yearly = cnyAmt
    } else if (raw.includes('季') || lower.includes('/qtr') || lower.includes('quarter')) {
      yearly = cnyAmt * 4
    }
    totalCNY += yearly
    count++
  }
  if (count === 0 || totalCNY <= 0) return ''
  return `¥${Math.round(totalCNY)}/年`
}

export function inferRecommendedRenewMonths(price?: string): 1 | 3 | 12 {
  const raw = String(price || '').toLowerCase()
  if (raw.includes('月') || raw.includes('/mo') || raw.includes('month')) return 1
  if (raw.includes('季') || raw.includes('/qtr') || raw.includes('quarter')) return 3
  return 12
}

export function groupModeLabel(mode?: string): string {
  if (mode === 'sequential') return '全员接力'
  if (mode === 'parallel') return '全员并发'
  if (mode === 'mention_only') return '仅 @点名'
  return '智能协同'
}

function computeLocalDayDiff(
  str?: string,
  now: Date = new Date()
): { d: Date; hm: string; diffDays: number } | null {
  if (!str) return null
  const d = new Date(str)
  if (Number.isNaN(d.getTime())) return null
  const hm = `${pad2(d.getHours())}:${pad2(d.getMinutes())}`
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const targetDay = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
  const diffDays = Math.round((today - targetDay) / 86_400_000)
  return { d, hm, diffDays }
}

export function formatChatTime(str?: string): string {
  const now = new Date()
  const info = computeLocalDayDiff(str, now)
  if (!info) return ''
  const { d, hm, diffDays } = info
  if (diffDays === 0) return hm
  if (diffDays === 1) return `昨天 ${hm}`
  if (d.getFullYear() === now.getFullYear()) return `${d.getMonth() + 1}/${d.getDate()}`
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()}`
}

export function formatTimelineDivider(str?: string): string {
  const now = new Date()
  const info = computeLocalDayDiff(str, now)
  if (!info) return ''
  const { d, hm, diffDays } = info
  if (diffDays === 0) return `今天 ${hm}`
  if (diffDays === 1) return `昨天 ${hm}`
  if (d.getFullYear() === now.getFullYear()) return `${d.getMonth() + 1}月${d.getDate()}日 ${hm}`
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日 ${hm}`
}

export function scoreOmnisearchMatch(
  title: string,
  subtitle: string,
  category: string,
  body: string,
  rawQuery: string
): number {
  const q = rawQuery.trim().toLowerCase()
  if (!q) return 1
  const t = String(title || '').toLowerCase()
  const sub = String(subtitle || '').toLowerCase()
  const cat = String(category || '').toLowerCase()
  const b = String(body || '').toLowerCase()

  if (t === q) return 120
  if (t.startsWith(q)) return 95
  if (t.includes(q)) return 75
  if (cat === q || cat.includes(q)) return 55
  if (sub.includes(q)) return 40
  if (b.includes(q)) return 25
  return 0
}
