import type { AvailableToolMeta, Skill } from '../types/bothub'

export interface ToolGroup {
  id: string
  label: string
  tools: Array<AvailableToolMeta & { shortLabel: string }>
}

const TOOL_META: Record<string, { group: string; shortLabel: string }> = {
  manage_todos: { group: 'assistant', shortLabel: '个人待办管理' },
  manage_notes: { group: 'assistant', shortLabel: '工作文档交付' },
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
