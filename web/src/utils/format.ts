export function escapeHtml(s: unknown): string {
  return String(s ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
}

export function formatTime(str?: string): string {
  if (!str) return '--'
  const d = new Date(str)
  if (Number.isNaN(d.getTime())) return str
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function formatDateOnly(str?: string): string {
  if (!str || str.startsWith('2099')) return '长期有效'
  const s = str.trim()
  const m = s.match(/^(\d{4}-\d{2}-\d{2})/)
  if (m) return m[1]
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
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

export function groupModeLabel(mode?: string): string {
  if (mode === 'sequential') return '全员接力'
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
