import type { Bot, Skill } from '../types/bothub'
import { parseJsonStringArray } from './format.ts'

export type SlashCommandAction =
  | 'open_drawer'
  | 'clear_timeline'
  | 'new_thread'
  | 'summarize_thread'
  | 'open_skills'
  | 'open_workhub'

export interface SlashCmdItem {
  cmd: string
  title: string
  desc: string
  prompt?: string
  action?: SlashCommandAction
}

interface SlashCommandSpec extends SlashCmdItem {
  anyOfTools?: readonly string[]
}

export const BUILTIN_SLASH_SPECS: readonly SlashCommandSpec[] = [
  {
    cmd: '/assets',
    title: '资产全面审计',
    desc: '盘点 VPS、域名与订阅到期风险',
    prompt: '请调用工具盘点当前全部数字资产与临期风险并给出续费建议。',
    anyOfTools: ['manage_assets'],
  },
  {
    cmd: '/nezha',
    title: '探针节点巡检',
    desc: '查看哪吒节点在线率与负载情况',
    prompt: '请调用 query_nezha_nodes 检查哪吒监控面板节点的实时在线与健康状态。',
    anyOfTools: ['query_nezha_nodes'],
  },
  {
    cmd: '/search',
    title: '全网检索',
    desc: '联网搜索最新资讯与公开资料',
    prompt: '请帮我联网搜索：',
    anyOfTools: ['web_search'],
  },
  {
    cmd: '/tasks',
    title: '调度队列核查',
    desc: '检查各自动化例程状态与下次触发时间',
    prompt: '请核查当前所有自动化调度任务的运行状态与上次执行摘要。',
    anyOfTools: ['manage_tasks'],
  },
  {
    cmd: '/remind',
    title: '创建定时/单次提醒',
    desc: '设定一次性定时提醒或周期性自动汇报任务',
    prompt: '请帮我设定一个定时提醒：',
    anyOfTools: ['manage_tasks'],
  },
  {
    cmd: '/todo',
    title: '记录或核查个人待办',
    desc: '添加行动项、查看未完成或勾选完结事项',
    prompt: '请帮我整理或记录以下待办事项：',
    anyOfTools: ['manage_todos'],
  },
  {
    cmd: '/plan',
    title: '目标拆解与批量排期',
    desc: '把复杂目标一键拆解成多条带截止日的待办子任务',
    prompt: '请调用 manage_todos(action="batch_add") 将以下目标拆解为可执行的分步行动清单并设定优先级：',
    anyOfTools: ['manage_todos'],
  },
  {
    cmd: '/note',
    title: '整理产出工作文档',
    desc: '将方案、总结或草稿沉淀到工作文档交付区',
    prompt: '请将以下内容整理为结构化 Markdown 工作文档并保存：',
    anyOfTools: ['manage_notes'],
  },
  {
    cmd: '/stash',
    title: '检索或收纳收藏与闪念',
    desc: '全文搜索之前收藏的文章/B站视频，或秒收新链接与灵感',
    prompt: '请调用 manage_stash 检索或收纳以下内容：',
    anyOfTools: ['manage_stash'],
  },
  {
    cmd: '/consult',
    title: '协调团队成员协办',
    desc: '调用其他专精 Bot 协助完成子任务并汇总',
    prompt: '请协调工作区里最合适的专业成员共同处理：',
    anyOfTools: ['consult_bot'],
  },
  {
    cmd: '/briefing',
    title: '今日行动全景简报',
    desc: '汇总逾期/今日待办、临期资产与未读预警并排期',
    prompt: '请调用 workspace_briefing 为我梳理今日重点待办、日程与风险，并给出行动建议。',
    anyOfTools: ['workspace_briefing'],
  },
  {
    cmd: '/calc',
    title: '精确表达式计算',
    desc: '使用高精度计算引擎核算费用、比例或复利',
    prompt: '请调用 calculate_eval 精确计算：',
    anyOfTools: ['calculate_eval'],
  },
  {
    cmd: '/memories',
    title: '回忆关键信息',
    desc: '检索与当前会话相关的历史重要记忆',
    prompt: '请先检索我们的历史记忆，梳理目前你记住的关键事实与约定。',
    anyOfTools: ['manage_memory', 'recall_memory'],
  },
  {
    cmd: '/consolidate',
    title: '清理并合并重复记忆',
    desc: '梳理当前作用域的冗余记忆条目并合并去重',
    prompt: '请先调用 manage_memory(action="consolidate") 合并当前作用域内的重复冗余记忆，再简报整理结果。',
    anyOfTools: ['manage_memory'],
  },
  { cmd: '/workhub', title: '打开个人行动台', desc: '管理待办清单与 Bot 产出的 Markdown 文档', action: 'open_workhub' },
  { cmd: '/thread', title: '新建独立话题', desc: '在当前会话内开启一条干净的新话题线', action: 'new_thread' },
  { cmd: '/summarize', title: '总结话题并存记忆', desc: '提炼当前话题核心结论并沉淀到专属记忆库', action: 'summarize_thread' },
  { cmd: '/skills', title: '打开自定义技能工坊', desc: '管理 Playbook 与 Webhook 外部技能工具', action: 'open_skills' },
  { cmd: '/invite', title: '拉人进群 / 群设置', desc: '打开右侧群成员与协同策略抽屉', action: 'open_drawer' },
  { cmd: '/clear', title: '清空当前对话', desc: '清空当前话题聊天时间线', action: 'clear_timeline' },
]

export function buildSlashCommands(bot: Bot | null | undefined, skills: Skill[] = []): SlashCmdItem[] {
  const allowedTools = new Set(parseJsonStringArray(bot?.allowed_tools))
  const boundSkills = new Set(parseJsonStringArray(bot?.skill_ids))
  const items: SlashCmdItem[] = []

  for (const sk of skills) {
    if (!sk.enabled || !boundSkills.has(sk.id)) continue
    items.push({
      cmd: `/${sk.slug}`,
      title: `技能 · ${sk.name}`,
      desc: sk.description || (sk.kind === 'webhook' ? `调用外部技能 skill_${sk.slug}` : '按专属技能规范响应'),
      prompt: sk.kind === 'webhook' ? `请使用 skill_${sk.slug} 工具协助处理当前问题：` : `请遵循「${sk.name}」技能规范回答：`,
    })
  }

  for (const spec of BUILTIN_SLASH_SPECS) {
    if (spec.anyOfTools && !spec.anyOfTools.some((t) => allowedTools.has(t))) {
      continue
    }
    const { anyOfTools: _unused, ...item } = spec
    items.push(item)
  }

  return items
}

export function filterSlashCommands(commands: SlashCmdItem[], rawInput: string): SlashCmdItem[] {
  const val = rawInput.trim().toLowerCase()
  if (!val.startsWith('/')) return []
  return commands.filter(
    (c) => c.cmd.toLowerCase().includes(val) || c.title.toLowerCase().includes(val.slice(1))
  )
}
