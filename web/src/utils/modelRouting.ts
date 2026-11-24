import type {
  ModelProvider,
  PoolStrategy,
  PooledModel,
  ProviderProtocol,
} from '../types/bothub'

export interface PublicProviderPreset {
  key: string
  label: string
  name: string
  protocol: ProviderProtocol
  base_url: string
  hint: string
}

/**
 * Strictly public vendor endpoint presets (zero personal keys or private endpoints).
 * Clicking any chip fills only standard public Base URL + protocol.
 */
export const PUBLIC_PROVIDER_PRESETS: PublicProviderPreset[] = [
  {
    key: 'openrouter',
    label: 'OpenRouter',
    name: 'OpenRouter',
    protocol: 'openai',
    base_url: 'https://openrouter.ai/api/v1',
    hint: 'Claude / GPT / Gemini / DeepSeek 统一聚合',
  },
  {
    key: 'deepseek',
    label: 'DeepSeek',
    name: 'DeepSeek 官方',
    protocol: 'openai',
    base_url: 'https://api.deepseek.com/v1',
    hint: 'DeepSeek 官方直连',
  },
  {
    key: 'gemini',
    label: 'Google AI Studio',
    name: 'Google AI Studio',
    protocol: 'gemini',
    base_url: 'https://generativelanguage.googleapis.com/v1beta/openai',
    hint: 'Gemini 官方直连',
  },
  {
    key: 'siliconflow',
    label: '硅基流动',
    name: '硅基流动',
    protocol: 'openai',
    base_url: 'https://api.siliconflow.cn/v1',
    hint: '国产开源模型按量聚合',
  },
  {
    key: 'groq',
    label: 'Groq',
    name: 'Groq',
    protocol: 'openai',
    base_url: 'https://api.groq.com/openai/v1',
    hint: '超低延迟开源推理',
  },
  {
    key: 'ollama',
    label: 'Ollama 本地',
    name: 'Ollama 本地',
    protocol: 'openai',
    base_url: 'http://127.0.0.1:11434/v1',
    hint: '本机/内网开源模型',
  },
  {
    key: 'custom',
    label: '自定义中转',
    name: '自定义中转渠道',
    protocol: 'openai',
    base_url: '',
    hint: '任意 OpenAI 兼容聚合站',
  },
]

export const POOL_STRATEGY_OPTIONS: Array<{ id: PoolStrategy; label: string; desc: string }> = [
  { id: 'priority', label: '按优先级', desc: '按顺位优先调用主力模型，异常自动无缝切备选' },
  { id: 'round_robin', label: '轮询均摊', desc: '在健康模型间轮流分发，均衡速率与额度' },
  { id: 'lowest_latency', label: '最快延迟', desc: '优先选择近期测速响应最快的健康模型' },
]

export function cleanProviderBaseUrl(raw: string): string {
  let u = (raw || '').trim().replace(/\/+$/, '')
  const suffixes = ['/chat/completions', '/completions', '/models']
  for (const s of suffixes) {
    if (u.toLowerCase().endsWith(s)) {
      u = u.slice(0, u.length - s.length).replace(/\/+$/, '')
    }
  }
  return u
}

export function canonicalModelTargetId(providerId: string, modelId: string): string {
  return `pool:${providerId.trim()}::${modelId.trim()}`
}

export function formatTokenCount(tokens: number): string {
  const n = Number(tokens) || 0
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(2)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

export function formatCostUSD(usd?: number): string {
  const v = Number(usd) || 0
  if (v <= 0) return '$0.00'
  if (v < 0.01) return `$${v.toFixed(4)}`
  return `$${v.toFixed(2)}`
}

export function formatCostCNY(cny?: number): string {
  const v = Number(cny) || 0
  if (v <= 0) return '¥0.00'
  if (v < 0.01) return `¥${v.toFixed(4)}`
  return `¥${v.toFixed(2)}`
}

export function strategyLabel(strategy?: string): string {
  const found = POOL_STRATEGY_OPTIONS.find((s) => s.id === strategy)
  return found ? found.label : '按优先级'
}

export interface RoutePreviewResult {
  tone: 'ok' | 'accent' | 'warn'
  headline: string
  chain: string[]
  note?: string
}

export function buildRoutePreview(params: {
  selectedModel: string
  providers: ModelProvider[]
  pooledModels: PooledModel[]
  strategy?: PoolStrategy
  needTools?: boolean
}): RoutePreviewResult {
  const { selectedModel, providers, pooledModels, strategy = 'priority', needTools = false } = params

  const pMap = new Map<string, ModelProvider>()
  for (const p of providers || []) {
    if (p.enabled && p.api_key && p.base_url) {
      pMap.set(p.id, p)
    }
  }

  const readyList = (pooledModels || [])
    .filter((m) => m.enabled && pMap.has(m.provider_id))
    .sort((a, b) => a.priority - b.priority)

  const fmtModel = (m: PooledModel) => {
    const p = pMap.get(m.provider_id)
    return `${p?.name || m.provider_id} · ${m.model_id}`
  }

  const req = (selectedModel || '').trim()
  const toolsNote = needTools ? ' · 优先已验证 Tools 节点' : ''

  if (!req || req === 'auto') {
    if (readyList.length === 0) {
      return {
        tone: 'warn',
        headline: '模型池暂无可用节点',
        chain: [],
        note: '请先在「系统设置 -> AI 大脑设置」中配置渠道并添加主力模型。',
      }
    }
    const chain = readyList.slice(0, 3).map(fmtModel)
    if (readyList.length > 3) chain.push(`+${readyList.length - 3} 个备选`)
    return {
      tone: 'accent',
      headline: `统一模型池自动路由（${strategyLabel(strategy)}${toolsNote}）`,
      chain,
    }
  }

  if (req.startsWith('pool:')) {
    const rawSpec = req.slice(5)
    const sep = rawSpec.indexOf('::')
    const pid = sep >= 0 ? rawSpec.slice(0, sep) : rawSpec
    const mid = sep >= 0 ? rawSpec.slice(sep + 2) : ''
    const matched = (pooledModels || []).find(
      (m) => m.provider_id === pid && (!mid || m.model_id === mid)
    )
    if (matched) {
      const chain = [fmtModel(matched)]
      if (readyList.length > 1) {
        chain.push(`自动容灾备选 (${readyList.length - 1})`)
      }
      return {
        tone: 'ok',
        headline: '定点锁定模型 · 异常自动切同池备选',
        chain,
      }
    }
  }

  return {
    tone: 'ok',
    headline: `指定模型: ${req}`,
    chain: [req],
  }
}
