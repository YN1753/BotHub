import type { AgentMessage, Bot } from '../types/bothub'

export interface ChatMessageUI {
  localId: string
  serverId?: number
  role: 'user' | 'assistant' | 'system'
  senderBotId: string
  senderName: string
  roleTitle?: string
  avatarStyle: string
  model: string
  modelTargetId?: string
  modelTier?: string
  msgKind: 'chat' | 'routine' | 'system'
  createdAt: string
  content: string
  reasoning: string
  tools: Array<{
    id: string
    name: string
    arguments: string
    result?: string
    durationMs?: number
    status?: string
    done: boolean
  }>
  replyToId?: number
  replyPreview?: string
  isTempGuest?: boolean
  statusText: string
  errorText?: string
  streaming?: boolean
  isLiveNew?: boolean
}

export interface MapMessagesOptions {
  bots?: Bot[]
  defaultModel?: string
}

export function mapServerMessage(
  m: AgentMessage,
  fallbackBot?: Bot,
  options: MapMessagesOptions = {}
): ChatMessageUI {
  const sender =
    options.bots?.find((b) => b.id === (m.sender_bot_id || m.bot_id)) || fallbackBot
  let parsedTools: ChatMessageUI['tools'] = []
  try {
    const raw = JSON.parse(m.tool_calls || '[]')
    if (Array.isArray(raw)) {
      parsedTools = raw.map((tc: any, idx: number) => ({
        id: tc.id || `tool_${idx}`,
        name: tc.function?.name || 'tool',
        arguments: tc.function?.arguments || '{}',
        done: true,
      }))
    }
  } catch {
    parsedTools = []
  }
  const kind =
    m.msg_kind === 'system' || m.role === 'system'
      ? 'system'
      : m.msg_kind === 'routine'
        ? 'routine'
        : 'chat'
  const role: ChatMessageUI['role'] =
    kind === 'system' ? 'system' : m.role === 'user' ? 'user' : 'assistant'

  return {
    localId: `srv_${m.id}`,
    serverId: m.id,
    role,
    senderBotId: m.sender_bot_id || m.bot_id || sender?.id || '',
    senderName: sender?.name || 'BotHub',
    roleTitle: sender?.role_title || '',
    avatarStyle: sender?.avatar_style || 'cyan',
    model: m.model_used || sender?.model || options.defaultModel || '',
    modelTargetId: m.model_target_id || undefined,
    modelTier: m.model_tier || undefined,
    msgKind: kind,
    createdAt: m.created_at,
    content: m.content || '',
    reasoning: m.reasoning_content || '',
    tools: parsedTools,
    replyToId: m.reply_to_id || undefined,
    replyPreview: m.reply_preview || undefined,
    statusText: kind === 'routine' ? '例程主动汇报' : '已完成',
  }
}

// Reconstructs full assistant turn by joining tool_calls with their corresponding role="tool" outputs
// and merging multi-round tool turns from the same sender into a single clean timeline bubble.
export function mapServerMessages(
  msgs: AgentMessage[],
  fallbackBot?: Bot,
  options: MapMessagesOptions = {}
): ChatMessageUI[] {
  const toolResultByCallID = new Map<string, string>()
  for (const m of msgs) {
    if (m.role === 'tool' && m.tool_call_id) {
      toolResultByCallID.set(m.tool_call_id, m.content || '')
    }
  }

  const out: ChatMessageUI[] = []
  for (const m of msgs) {
    if (m.role !== 'user' && m.role !== 'assistant' && m.role !== 'system') {
      continue
    }
    const mapped = mapServerMessage(m, fallbackBot, options)
    if (mapped.tools.length > 0) {
      for (const tc of mapped.tools) {
        if (toolResultByCallID.has(tc.id)) {
          tc.result = toolResultByCallID.get(tc.id)
        }
      }
    }

    const prev = out.length > 0 ? out[out.length - 1] : null
    if (
      prev &&
      prev.role === 'assistant' &&
      mapped.role === 'assistant' &&
      prev.senderBotId === mapped.senderBotId &&
      prev.tools.length > 0
    ) {
      prev.serverId = mapped.serverId
      prev.localId = mapped.localId
      if (mapped.reasoning && !prev.reasoning.includes(mapped.reasoning)) {
        prev.reasoning = prev.reasoning ? `${prev.reasoning}\n${mapped.reasoning}` : mapped.reasoning
      }
      if (mapped.tools.length > 0) {
        prev.tools.push(...mapped.tools)
      }
      if (mapped.content) {
        prev.content = mapped.content
      }
      if (mapped.model) {
        prev.model = mapped.model
      }
      if (mapped.modelTargetId) {
        prev.modelTargetId = mapped.modelTargetId
      }
      if (mapped.modelTier) {
        prev.modelTier = mapped.modelTier
      }
      prev.createdAt = mapped.createdAt
      continue
    }
    out.push(mapped)
  }
  return out
}
