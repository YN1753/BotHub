import type { AvailableToolMeta, Skill } from '../types/bothub'

export interface ToolGroup {
  id: string
  label: string
  tools: Array<AvailableToolMeta & { shortLabel: string }>
}

const TOOL_META: Record<string, { group: string; shortLabel: string }> = {
  manage_todos: { group: 'assistant', shortLabel: '个人待办管理' },
  manage_notes: { group: 'assistant', shortLabel: '工作文档交付' },
  manage_stash: { group: 'assistant', shortLabel: '收藏与闪念匣' },
  manage_tasks: { group: 'assistant', shortLabel: '定时与单次提醒' },
  workspace_briefing: { group: 'assistant', shortLabel: '今日工作简报' },
  send_notification: { group: 'assistant', shortLabel: '主动消息推送' },

  web_search: { group: 'research', shortLabel: '联网检索' },
  read_url: { group: 'research', shortLabel: '网页正文抓取' },
  calculate_eval: { group: 'research', shortLabel: '精确数学计算' },
  datetime_calc: { group: 'research', shortLabel: '日期日历推算' },

  remember_fact: { group: 'memory', shortLabel: '写入长期记忆' },
  recall_memory: { group: 'memory', shortLabel: '检索长期记忆' },
  manage_memory: { group: 'memory', shortLabel: '整理与遗忘记忆' },
  manage_bots: { group: 'memory', shortLabel: '创建 Bot 与拉群' },
  manage_models: { group: 'ops', shortLabel: 'AI 模型池巡检调优' },
  manage_skills: { group: 'assistant', shortLabel: '自定义技能工坊' },
  consult_bot: { group: 'memory', shortLabel: '协同队友委派' },

  manage_assets: { group: 'ops', shortLabel: '数字资产台账' },
  query_nezha_nodes: { group: 'ops', shortLabel: '哪吒探针巡检' },
}

export function formatToolDisplayName(name: string, skills: Skill[] = []): string {
  if (name.startsWith('skill_')) {
    const slug = name.slice(6)
    const found = skills.find((s) => s.slug === slug)
    return found ? `${found.name} (${name})` : name
  }
  const meta = TOOL_META[name]
  return meta ? `${meta.shortLabel} (${name})` : name
}

export function formatToolHumanName(name: string, skills: Skill[] = []): string {
  if (name.startsWith('skill_')) {
    const slug = name.slice(6)
    const found = skills.find((s) => s.slug === slug)
    return found?.name || slug
  }
  if (name === 'read_url') return '阅读网页'
  return TOOL_META[name]?.shortLabel || name
}

export interface HumanToolSource {
  title: string
  url: string
  domain: string
}

export interface HumanToolStepSummary {
  title: string
  subtitle?: string
  url?: string
  statusLabel: string
  sources: HumanToolSource[]
  isError: boolean
}

function tryParseJson(raw?: string): Record<string, any> | null {
  if (!raw) return null
  const trimmed = raw.trim()
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return null
  try {
    const parsed = JSON.parse(trimmed)
    return parsed && typeof parsed === 'object' ? parsed : null
  } catch {
    return null
  }
}

function safeHttpUrl(raw: unknown): string {
  if (typeof raw !== 'string') return ''
  const s = raw.trim()
  if (!s) return ''
  try {
    const u = new URL(s)
    if (u.protocol === 'http:' || u.protocol === 'https:') return u.toString()
  } catch {
    // ignore invalid URL
  }
  return ''
}

function formatCompactUrl(rawUrl: string): { display: string; domain: string } {
  try {
    const u = new URL(rawUrl)
    const host = u.hostname.replace(/^www\./, '')
    const path = u.pathname && u.pathname !== '/' ? u.pathname.replace(/\/$/, '') : ''
    const full = `${host}${path}`
    return {
      domain: host,
      display: full.length > 44 ? `${full.slice(0, 41)}...` : full,
    }
  } catch {
    return { domain: rawUrl, display: rawUrl.slice(0, 44) }
  }
}

function clipText(val: unknown, maxLen = 48): string {
  if (typeof val !== 'string') return ''
  const clean = val.replace(/\s+/g, ' ').trim()
  if (!clean) return ''
  return clean.length > maxLen ? `${clean.slice(0, maxLen - 1)}…` : clean
}

const TOOL_ACTION_LABELS: Record<string, string> = {
  list: '查看列表',
  get: '查看详情',
  add: '新增',
  create: '创建',
  update: '更新',
  edit: '编辑',
  delete: '删除',
  remove: '移除',
  search: '检索',
  query: '查询',
  complete: '标记完成',
  done: '标记完成',
  toggle: '切换状态',
  archive: '归档',
  create_bot: '新建 Bot',
  update_bot: '更新 Bot',
  delete_bot: '删除 Bot',
  create_group: '组建群聊',
  update_group: '更新群聊',
  add_group_members: '拉人进群',
  remove_group_members: '移出成员',
  status: '状态巡检',
  clear_cooldown: '解除冷却',
  set_strategy: '切换调度策略',
  toggle_model: '启停模型',
  move_tier: '调整模型池',
  reorder: '调整优先级',
  upsert: '保存配置',
  bind_bot: '挂载到 Bot',
}

export function summarizeToolStep(
  tool: {
    name: string
    arguments?: string
    result?: string
    status?: string
    done?: boolean
  },
  skills: Skill[] = []
): HumanToolStepSummary {
  const args = tryParseJson(tool.arguments) || {}
  const res = tryParseJson(tool.result)
  const done = Boolean(tool.done)
  const isError = tool.status === 'error' || Boolean(res && res.ok === false && res.error)
  const errMsg = res && typeof res.error === 'string' ? clipText(res.error, 42) : ''

  if (tool.name === 'web_search') {
    const q = clipText(args.query || res?.query, 44)
    const rawResults = Array.isArray(res?.results) ? res.results : []
    const count = typeof res?.count === 'number' ? res.count : rawResults.length
    const sources: HumanToolSource[] = []
    for (const item of rawResults) {
      if (sources.length >= 3) break
      const url = safeHttpUrl(item?.url)
      if (!url) continue
      const { domain } = formatCompactUrl(url)
      sources.push({
        title: clipText(item?.title || domain, 36) || domain,
        url,
        domain,
      })
    }
    return {
      title: q ? `搜索「${q}」` : '联网搜索',
      statusLabel: !done
        ? '搜索中...'
        : isError
          ? errMsg || '检索异常'
          : count > 0
            ? `找到 ${count} 条结果`
            : '未命中结果',
      sources,
      isError,
    }
  }

  if (tool.name === 'read_url') {
    const url = safeHttpUrl(args.url || res?.url)
    const formatted = url ? formatCompactUrl(url) : null
    const pageTitle = clipText(res?.title, 48)
    return {
      title: formatted ? `阅读网页 · ${formatted.display}` : '阅读网页',
      subtitle: pageTitle && pageTitle !== formatted?.display ? pageTitle : undefined,
      url: url || undefined,
      statusLabel: !done ? '读取中...' : isError ? errMsg || '读取失败' : '已读取',
      sources: [],
      isError,
    }
  }

  if (tool.name === 'manage_bots') {
    const act = typeof args.action === 'string' ? args.action.trim().toLowerCase() : ''
    const actLabel = TOOL_ACTION_LABELS[act] || '管理 Bot 与群聊'
    const targetName = clipText(
      res?.name || res?.group_name || args.name || args.group_name || args.bot_name || args.group_id || args.bot_id,
      36
    )
    const memberNames = Array.isArray(res?.member_names)
      ? res.member_names.map((x: unknown) => String(x)).join('、')
      : ''
    const subtitle = memberNames ? `成员：${clipText(memberNames, 44)}` : targetName || undefined
    return {
      title: targetName ? `${actLabel} · ${targetName}` : actLabel,
      subtitle: memberNames ? subtitle : undefined,
      statusLabel: !done
        ? '处理中...'
        : isError
          ? errMsg || '操作失败'
          : typeof res?.count === 'number'
            ? `共 ${res.count} 个`
            : '已完成',
      sources: [],
      isError,
    }
  }

  if (tool.name === 'consult_bot') {
    if (res && res.parallel_count && Array.isArray(res.results)) {
      const names = res.results
        .filter((x: any) => x?.ok && x?.bot_name)
        .map((x: any) => String(x.bot_name))
      return {
        title: `并发协同 (${res.parallel_count || res.results.length} 位成员)`,
        subtitle: names.length ? names.join('、') : undefined,
        statusLabel: !done ? '协作中...' : isError ? '协作异常' : '已完成',
        sources: [],
        isError,
      }
    }
    const targetName = clipText(res?.bot_name || args.bot_name || args.target_bot, 28)
    const roleTitle = clipText(res?.role_title, 24)
    return {
      title: targetName ? `协同成员 · ${targetName}` : '协同队友委派',
      subtitle: roleTitle || undefined,
      statusLabel: !done ? '协作中...' : isError ? errMsg || '协作异常' : '已完成',
      sources: [],
      isError,
    }
  }

  const humanName = formatToolHumanName(tool.name, skills)
  const actionKey = typeof args.action === 'string' ? args.action.trim().toLowerCase() : ''
  const actionLabel = actionKey ? TOOL_ACTION_LABELS[actionKey] || '' : ''
  const subject = clipText(
    args.title || args.name || args.fact_key || args.query || args.expression || args.target || args.content,
    42
  )
  const count =
    typeof res?.count === 'number'
      ? res.count
      : Array.isArray(res?.items)
        ? res.items.length
        : Array.isArray(res?.nodes)
          ? res.nodes.length
          : Array.isArray(res?.memories)
            ? res.memories.length
            : undefined

  return {
    title: actionLabel ? `${humanName} · ${actionLabel}` : humanName,
    subtitle: subject || undefined,
    statusLabel: !done
      ? '处理中...'
      : isError
        ? errMsg || '异常'
        : count !== undefined
          ? `共 ${count} 项`
          : '已完成',
    sources: [],
    isError,
  }
}

const GROUP_ORDER: Array<{ id: string; label: string }> = [
  { id: 'assistant', label: '个人助手与行动' },
  { id: 'research', label: '检索、阅读与精确计算' },
  { id: 'memory', label: '记忆与多 Bot 协作' },
  { id: 'ops', label: '资产与节点运维' },
  { id: 'extra', label: '其他扩展工具' },
]

export function shortToolLabel(tool: AvailableToolMeta): string {
  return TOOL_META[tool.id]?.shortLabel || tool.id
}

export function groupAvailableTools(tools: AvailableToolMeta[]): ToolGroup[] {
  const buckets = new Map<string, Array<AvailableToolMeta & { shortLabel: string }>>()
  for (const g of GROUP_ORDER) {
    buckets.set(g.id, [])
  }
  for (const tool of tools || []) {
    const meta = TOOL_META[tool.id]
    const gid = meta?.group || 'extra'
    const list = buckets.get(gid) || buckets.get('extra')!
    list.push({
      ...tool,
      shortLabel: meta?.shortLabel || tool.id,
    })
  }
  return GROUP_ORDER.map((g) => ({
    id: g.id,
    label: g.label,
    tools: buckets.get(g.id) || [],
  })).filter((g) => g.tools.length > 0)
}
