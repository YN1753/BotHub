import type { ChatMessageUI } from './chatMapper'

export interface AssetCurrencyBreakdown {
  currency: string
  monthlyEstimated: number
  annualizedEstimated: number
  pricedItems: number
}

export interface AssetSummaryCardData {
  kind: 'asset_summary'
  activeCount: number
  inactiveCount: number
  expiring30dCount: number
  byCategory: Record<string, number>
  costByCurrency: AssetCurrencyBreakdown[]
}

export interface AssetCardData {
  kind: 'asset'
  id: string
  name: string
  category: string
  spec: string
  price: string
  status: string
  dueDate: string
  daysLeft: number
  autoRenew: boolean
  isExpiring: boolean
}

export interface NezhaNodeCardData {
  kind: 'nezha_node'
  id: string
  name: string
  status: string
  lastActive: string
  hostSummary: string
  rawHostInfo: string
  source: string
}

export interface TodoCardData {
  kind: 'todo'
  id: string
  title: string
  notes: string
  status: string
  priority: string
  dueAt: string
  tags: string
  overdue: boolean
}

export interface TaskCardData {
  kind: 'task'
  id: string
  name: string
  taskType: string
  botId: string
  cronExpr: string
  runAt: string
  enabled: boolean
  nextRunAt: string
  lastSummary: string
  lastStatus: string
}

export interface BriefingCardData {
  kind: 'briefing'
  today: string
  weekday: string
  openTodoCount: number
  overdueCount: number
  dueTodayCount: number
  highPriorityCount: number
  expiringAssetCount: number
  unreadCount: number
  activeTaskCount: number
  summaryText: string
  focusHints: string[]
}

export type StructuredToolCard =
  | AssetSummaryCardData
  | AssetCardData
  | NezhaNodeCardData
  | TodoCardData
  | TaskCardData
  | BriefingCardData

function tryParseToolObject(raw?: string): Record<string, any> | null {
  if (typeof raw !== 'string') return null
  const trimmed = raw.trim()
  if (!trimmed.startsWith('{')) return null
  try {
    const parsed = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null
    return parsed as Record<string, any>
  } catch {
    return null
  }
}

function asStr(val: unknown, fallback = ''): string {
  if (typeof val === 'string') return val.trim()
  if (typeof val === 'number' || typeof val === 'boolean') return String(val)
  return fallback
}

function asNum(val: unknown, fallback = 0): number {
  const n = Number(val)
  return Number.isFinite(n) ? n : fallback
}

function asBool(val: unknown, fallback = false): boolean {
  if (typeof val === 'boolean') return val
  if (val === 'true' || val === 1) return true
  if (val === 'false' || val === 0) return false
  return fallback
}

function formatByteSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return ''
  const gb = bytes / (1024 * 1024 * 1024)
  if (gb >= 1) {
    return `${gb >= 10 ? Math.round(gb) : gb.toFixed(1).replace(/\.0$/, '')} GB`
  }
  const mb = bytes / (1024 * 1024)
  if (mb >= 1) return `${Math.round(mb)} MB`
  return `${Math.round(bytes / 1024)} KB`
}

export function parseNezhaHostInfo(rawHostInfo: unknown): string {
  let obj: Record<string, any> | null = null
  if (rawHostInfo && typeof rawHostInfo === 'object' && !Array.isArray(rawHostInfo)) {
    obj = rawHostInfo as Record<string, any>
  } else if (typeof rawHostInfo === 'string') {
    const trimmed = rawHostInfo.trim()
    if (!trimmed) return ''
    if (trimmed.startsWith('{')) {
      try {
        const parsed = JSON.parse(trimmed)
        if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
          obj = parsed as Record<string, any>
        }
      } catch {
        // Fallback to plain text cleanup
      }
    }
    if (!obj) {
      const compact = trimmed.replace(/\s+/g, ' ')
      return compact.length > 84 ? `${compact.slice(0, 81)}...` : compact
    }
  }
  if (!obj) return ''

  const parts: string[] = []

  // OS / Platform
  const platform = asStr(obj.Platform || obj.platform || obj.OS || obj.os)
  const platformVer = asStr(obj.PlatformVersion || obj.platform_version)
  if (platform) {
    parts.push(platformVer && !platform.includes(platformVer) ? `${platform} ${platformVer}` : platform)
  }

  // CPU
  const cpuRaw = obj.CPU ?? obj.cpu
  if (Array.isArray(cpuRaw) && cpuRaw.length > 0) {
    const firstCpu = asStr(cpuRaw[0]).replace(/\s+/g, ' ')
    if (firstCpu) {
      const coreLabel = cpuRaw.length > 1 ? `${cpuRaw.length}C · ` : ''
      const clippedCpu = firstCpu.length > 28 ? `${firstCpu.slice(0, 25)}...` : firstCpu
      parts.push(`${coreLabel}${clippedCpu}`)
    }
  } else if (typeof cpuRaw === 'string' && cpuRaw.trim()) {
    parts.push(cpuRaw.trim().slice(0, 32))
  } else if (typeof cpuRaw === 'number' && cpuRaw > 0) {
    parts.push(`${cpuRaw}C`)
  }

  // Memory
  const memTotal = obj.MemTotal ?? obj.mem_total
  if (typeof memTotal === 'number' && memTotal > 0) {
    const label = formatByteSize(memTotal)
    if (label) parts.push(`RAM ${label}`)
  } else if (typeof obj.Mem === 'string' && obj.Mem.trim()) {
    parts.push(`RAM ${obj.Mem.trim()}`)
  } else if (typeof obj.mem === 'string' && obj.mem.trim()) {
    parts.push(`RAM ${obj.mem.trim()}`)
  }

  // Disk
  const diskTotal = obj.DiskTotal ?? obj.disk_total
  if (typeof diskTotal === 'number' && diskTotal > 0) {
    const label = formatByteSize(diskTotal)
    if (label) parts.push(`Disk ${label}`)
  }

  // Arch / Country
  const arch = asStr(obj.Arch || obj.arch)
  const country = asStr(obj.CountryCode || obj.country_code).toUpperCase()
  if (arch) parts.push(arch)
  if (country) parts.push(country)

  return parts.join(' · ')
}

function normalizeAssetCard(raw: Record<string, any>, isExpiringHint = false): AssetCardData | null {
  const id = asStr(raw.id)
  const name = asStr(raw.name) || id
  if (!id && !name) return null
  const dueDate = asStr(raw.due_date)
  const status = asStr(raw.status, '生效中')
  const daysLeft = asNum(raw.days_left, 0)
  const isPermanent = status === '长期有效' || dueDate.startsWith('2099')
  const isExpiring =
    isExpiringHint ||
    (!isPermanent && status !== '已停用' && Number.isFinite(Number(raw.days_left)) && daysLeft <= 7)

  return {
    kind: 'asset',
    id: id || name,
    name,
    category: asStr(raw.category, 'general'),
    spec: asStr(raw.spec),
    price: asStr(raw.price),
    status,
    dueDate,
    daysLeft,
    autoRenew: asBool(raw.auto_renew, false),
    isExpiring,
  }
}

function normalizeTodoCard(raw: Record<string, any>, todayHint = '', overdueHint = false): TodoCardData | null {
  const id = asStr(raw.id)
  const title = asStr(raw.title)
  if (!id && !title) return null
  const status = asStr(raw.status, 'pending')
  const dueAt = asStr(raw.due_at)
  const dueDay = dueAt.slice(0, 10)
  const overdue =
    overdueHint ||
    Boolean(status !== 'done' && todayHint && dueDay && /^\d{4}-\d{2}-\d{2}$/.test(dueDay) && dueDay < todayHint)

  return {
    kind: 'todo',
    id: id || title,
    title: title || id,
    notes: asStr(raw.notes),
    status,
    priority: asStr(raw.priority, 'normal'),
    dueAt,
    tags: asStr(raw.tags),
    overdue,
  }
}

function normalizeTaskCard(raw: Record<string, any>): TaskCardData | null {
  const id = asStr(raw.id)
  const name = asStr(raw.name) || id
  if (!id && !name) return null
  const lastRun = raw.last_run && typeof raw.last_run === 'object' ? raw.last_run : null
  return {
    kind: 'task',
    id: id || name,
    name,
    taskType: asStr(raw.type, 'bot_prompt'),
    botId: asStr(raw.bot_id),
    cronExpr: asStr(raw.cron_expr),
    runAt: asStr(raw.run_at),
    enabled: asBool(raw.enabled, true),
    nextRunAt: asStr(raw.next_run_at),
    lastSummary: asStr(lastRun?.summary || raw.last_summary || raw.description),
    lastStatus: asStr(lastRun?.status),
  }
}

function normalizeNezhaNode(raw: Record<string, any>): NezhaNodeCardData | null {
  const id = asStr(raw.id)
  const name = asStr(raw.name) || id
  if (!id && !name) return null
  const rawHost =
    typeof raw.host_info === 'string'
      ? raw.host_info
      : raw.host_info && typeof raw.host_info === 'object'
        ? JSON.stringify(raw.host_info)
        : ''
  return {
    kind: 'nezha_node',
    id: id || name,
    name,
    status: asStr(raw.status, 'online').toLowerCase(),
    lastActive: asStr(raw.last_active),
    hostSummary: parseNezhaHostInfo(raw.host_info),
    rawHostInfo: rawHost,
    source: asStr(raw.source, 'nezha'),
  }
}

/**
 * Extracts structured, deduplicated visualization cards from completed tool outputs
 * in a single assistant turn. Malformed or partial streaming JSON returns cleanly without throwing.
 */
export function extractStructuredToolCards(tools?: ChatMessageUI['tools']): StructuredToolCard[] {
  if (!Array.isArray(tools) || tools.length === 0) return []

  const cards: StructuredToolCard[] = []
  const indexByKey = new Map<string, number>()

  const upsertCard = (key: string, card: StructuredToolCard) => {
    const existingIdx = indexByKey.get(key)
    if (existingIdx !== undefined) {
      const prev = cards[existingIdx]
      if (prev.kind === 'asset' && card.kind === 'asset') {
        cards[existingIdx] = {
          ...prev,
          ...card,
          isExpiring: prev.isExpiring || card.isExpiring,
        }
      } else if (prev.kind === 'todo' && card.kind === 'todo') {
        cards[existingIdx] = {
          ...prev,
          ...card,
          overdue: prev.overdue || card.overdue,
        }
      } else {
        cards[existingIdx] = card
      }
      return
    }
    indexByKey.set(key, cards.length)
    cards.push(card)
  }

  for (const tool of tools) {
    if (!tool || tool.status === 'error') continue
    const res = tryParseToolObject(tool.result)
    if (!res) continue
    if (res.ok === false && res.error) continue

    switch (tool.name) {
      case 'manage_assets': {
        if (typeof res.active_count === 'number' || Array.isArray(res.cost_by_currency)) {
          const byCat: Record<string, number> = {}
          if (res.by_category && typeof res.by_category === 'object' && !Array.isArray(res.by_category)) {
            for (const [k, v] of Object.entries(res.by_category)) {
              byCat[k] = asNum(v, 0)
            }
          }
          const costList: AssetCurrencyBreakdown[] = Array.isArray(res.cost_by_currency)
            ? res.cost_by_currency.map((c: any) => ({
                currency: asStr(c?.currency, 'USD'),
                monthlyEstimated: asNum(c?.monthly_estimated, 0),
                annualizedEstimated: asNum(c?.annualized_estimated, 0),
                pricedItems: asNum(c?.priced_items, 0),
              }))
            : []
          upsertCard('summary:asset_summary', {
            kind: 'asset_summary',
            activeCount: asNum(res.active_count, 0),
            inactiveCount: asNum(res.inactive_count, 0),
            expiring30dCount: asNum(res.expiring_30d_count, 0),
            byCategory: byCat,
            costByCurrency: costList,
          })
        }

        if (Array.isArray(res.assets)) {
          for (const raw of res.assets) {
            if (!raw || typeof raw !== 'object') continue
            const card = normalizeAssetCard(raw, false)
            if (card) upsertCard(`asset:${card.id}`, card)
          }
        }

        if (Array.isArray(res.expiring)) {
          for (const raw of res.expiring) {
            if (!raw || typeof raw !== 'object') continue
            const card = normalizeAssetCard(raw, true)
            if (card) upsertCard(`asset:${card.id}`, card)
          }
        }

        if (res.asset && typeof res.asset === 'object') {
          const card = normalizeAssetCard(res.asset, false)
          if (card) upsertCard(`asset:${card.id}`, card)
        }
        break
      }

      case 'query_nezha_nodes': {
        if (Array.isArray(res.nezha_nodes)) {
          for (const raw of res.nezha_nodes) {
            if (!raw || typeof raw !== 'object') continue
            const card = normalizeNezhaNode({ ...raw, source: raw.source || res.source })
            if (card) upsertCard(`nezha:${card.id}`, card)
          }
        }
        if (Array.isArray(res.compute_assets)) {
          for (const raw of res.compute_assets) {
            if (!raw || typeof raw !== 'object') continue
            const card = normalizeAssetCard(raw, false)
            if (card) upsertCard(`asset:${card.id}`, card)
          }
        }
        break
      }

      case 'manage_todos': {
        if (Array.isArray(res.todos)) {
          for (const raw of res.todos) {
            if (!raw || typeof raw !== 'object') continue
            const card = normalizeTodoCard(raw)
            if (card) upsertCard(`todo:${card.id}`, card)
          }
        }
        if (res.todo && typeof res.todo === 'object') {
          const card = normalizeTodoCard(res.todo)
          if (card) upsertCard(`todo:${card.id}`, card)
        }
        break
      }

      case 'manage_tasks': {
        if (Array.isArray(res.tasks)) {
          for (const raw of res.tasks) {
            if (!raw || typeof raw !== 'object') continue
            const card = normalizeTaskCard(raw)
            if (card) upsertCard(`task:${card.id}`, card)
          }
        }
        if (res.task && typeof res.task === 'object') {
          const card = normalizeTaskCard(res.task)
          if (card) upsertCard(`task:${card.id}`, card)
        }
        break
      }

      case 'workspace_briefing': {
        const today = asStr(res.today)
        const overdueList = Array.isArray(res.overdue_todos) ? res.overdue_todos : []
        const dueTodayList = Array.isArray(res.due_today_todos) ? res.due_today_todos : []
        const highList = Array.isArray(res.high_priority_todos) ? res.high_priority_todos : []
        const expiringAssets = Array.isArray(res.expiring_assets) ? res.expiring_assets : []
        const unreadNotifs = Array.isArray(res.unread_notifications)
          ? res.unread_notifications
          : Array.isArray(res.unread_alerts)
            ? res.unread_alerts
            : []
        const activeTasks = Array.isArray(res.active_tasks)
          ? res.active_tasks
          : Array.isArray(res.enabled_tasks)
            ? res.enabled_tasks
            : []
        const focusHints = Array.isArray(res.focus_hints)
          ? res.focus_hints.map((h: unknown) => asStr(h)).filter(Boolean)
          : []
        const openTodoCount =
          typeof res.open_todo_count === 'number'
            ? res.open_todo_count
            : new Set(
                [...overdueList, ...dueTodayList, ...highList]
                  .map((t: any) => asStr(t?.id || t?.title))
                  .filter(Boolean)
              ).size

        upsertCard('summary:briefing', {
          kind: 'briefing',
          today,
          weekday: asStr(res.weekday),
          openTodoCount,
          overdueCount: overdueList.length,
          dueTodayCount: dueTodayList.length,
          highPriorityCount: highList.length,
          expiringAssetCount: expiringAssets.length,
          unreadCount: typeof res.unread_count === 'number' ? res.unread_count : unreadNotifs.length,
          activeTaskCount: activeTasks.length,
          summaryText: asStr(res.summary_text),
          focusHints,
        })

        for (const raw of overdueList) {
          if (!raw || typeof raw !== 'object') continue
          const card = normalizeTodoCard(raw, today, true)
          if (card) upsertCard(`todo:${card.id}`, card)
        }
        for (const raw of dueTodayList) {
          if (!raw || typeof raw !== 'object') continue
          const card = normalizeTodoCard(raw, today, false)
          if (card) upsertCard(`todo:${card.id}`, card)
        }
        for (const raw of highList) {
          if (!raw || typeof raw !== 'object') continue
          const card = normalizeTodoCard(raw, today, false)
          if (card) upsertCard(`todo:${card.id}`, card)
        }
        for (const raw of expiringAssets) {
          if (!raw || typeof raw !== 'object') continue
          const card = normalizeAssetCard(raw, true)
          if (card) upsertCard(`asset:${card.id}`, card)
        }
        break
      }

      default:
        break
    }
  }

  return cards
}
