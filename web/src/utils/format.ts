function pad2(n: string | number): string {
  return String(n).padStart(2, '0')
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
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ${pad2(d.getHours())}:${pad2(d.getMinutes())}`
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

export function formatDateOnly(str?: string): string {
  if (!str || str.startsWith('2099')) return '长期有效'
  const s = str.trim()
  const m = s.match(/^(\d{4}-\d{2}-\d{2})/)
  if (m) return m[1]
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
}

export function generateSlug(text: string, prefix = 'item'): string {
  const clean = text
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '_')
    .replace(/^_+|_+$/g, '')
  return clean || `${prefix}_${Date.now().toString(36).slice(-4)}`
}

export function shortVersion(ver: string): string {
  return String(ver || '').replace(/-[0-9a-f]{12,40}$/i, '')
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
  const pad = (n: string | number) => String(n).padStart(2, '0')

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
    return `每天 ${period} ${pad(hour)}:${pad(min)} 执行`
  }
  if (!Number.isNaN(Number(min)) && !Number.isNaN(Number(hour)) && dom === '*' && mon === '*' && (dow === '1-5' || dow === '1,2,3,4,5')) {
    return `工作日 (周一至五) ${pad(hour)}:${pad(min)} 执行`
  }
  if (!Number.isNaN(Number(min)) && !Number.isNaN(Number(hour)) && dom === '*' && mon === '*' && (dow === '6,0' || dow === '0,6' || dow === '6,7')) {
    return `周末 (周六日) ${pad(hour)}:${pad(min)} 执行`
  }
  const weekMap: Record<string, string> = { '0': '周日', '1': '周一', '2': '周二', '3': '周三', '4': '周四', '5': '周五', '6': '周六', '7': '周日' }
  if (!Number.isNaN(Number(min)) && !Number.isNaN(Number(hour)) && dom === '*' && mon === '*' && weekMap[dow]) {
    return `每${weekMap[dow]} ${pad(hour)}:${pad(min)} 执行`
  }
  if (!Number.isNaN(Number(min)) && !Number.isNaN(Number(hour)) && !Number.isNaN(Number(dom)) && mon === '*' && dow === '*') {
    return `每月 ${pad(dom)} 日 ${pad(hour)}:${pad(min)} 执行`
  }
  if (!Number.isNaN(Number(min)) && hour.includes(',') && dom === '*' && mon === '*' && dow === '*') {
    return `每天 ${hour.split(',').map((h) => `${pad(h)}:${pad(min)}`).join(' 与 ')} 各执行一次`
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

export function summarizeAssetsMonthlyCost(
  assets: Array<{ status?: string; price?: string }>
): string {
  const totals = new Map<string, number>()
  for (const a of assets) {
    if (a.status === '已停用') continue
    const raw = String(a.price || '').trim()
    if (!raw || raw.includes('免费')) continue
    const m = raw.match(/(\d+(?:\.\d+)?)/)
    if (!m) continue
    const num = Number(m[1])
    if (!Number.isFinite(num) || num <= 0) continue
    const upper = raw.toUpperCase()
    const sym =
      upper.includes('HK$') || upper.includes('HKD')
        ? 'HK$'
        : upper.includes('$') || upper.includes('USD')
        ? '$'
        : upper.includes('€') || upper.includes('EUR')
        ? '€'
        : upper.includes('£') || upper.includes('GBP')
        ? '£'
        : '¥'
    let monthly = num
    const lower = raw.toLowerCase()
    if (raw.includes('年') || lower.includes('/yr') || lower.includes('year') || lower.includes('/y')) {
      monthly = num / 12
    } else if (raw.includes('季') || lower.includes('/qtr') || lower.includes('quarter')) {
      monthly = num / 3
    } else if (raw.includes('周') || lower.includes('/wk') || lower.includes('week')) {
      monthly = num * 4.33
    } else if (raw.includes('天') || raw.includes('日') || lower.includes('/day') || lower.includes('/d')) {
      monthly = num * 30
    }
    totals.set(sym, (totals.get(sym) || 0) + monthly)
  }
  if (totals.size === 0) return ''
  return (
    Array.from(totals.entries())
      .map(([sym, val]) => `${sym}${val.toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1')}/月`)
      .join(' + ')
  )
}

export function groupModeLabel(mode?: string): string {
  if (mode === 'sequential') return '全员接力'
  if (mode === 'parallel') return '全员并发'
  if (mode === 'mention_only') return '仅 @点名'
  return '智能协同'
}

export function formatChatTime(str?: string): string {
  if (!str) return ''
  const d = new Date(str)
  if (Number.isNaN(d.getTime())) return ''
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`

  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const targetDay = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
  const diffDays = Math.round((today - targetDay) / 86400000)

  if (diffDays === 0) return hm
  if (diffDays === 1) return `昨天 ${hm}`
  if (d.getFullYear() === now.getFullYear()) return `${d.getMonth() + 1}/${d.getDate()}`
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()}`
}

export function formatTimelineDivider(str?: string): string {
  if (!str) return ''
  const d = new Date(str)
  if (Number.isNaN(d.getTime())) return ''
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`

  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const targetDay = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
  const diffDays = Math.round((today - targetDay) / 86400000)

  if (diffDays === 0) return `今天 ${hm}`
  if (diffDays === 1) return `昨天 ${hm}`
  if (d.getFullYear() === now.getFullYear()) return `${d.getMonth() + 1}月${d.getDate()}日 ${hm}`
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日 ${hm}`
}
