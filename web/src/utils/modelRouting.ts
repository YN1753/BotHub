import type {
  ModelProvider,
  PoolStrategies,
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
    hint: '免费 :free 与主流付费聚合',
  },
  {
    key: 'siliconflow',
    label: '硅基流动',
    name: '硅基流动',
    protocol: 'openai',
    base_url: 'https://api.siliconflow.cn/v1',
    hint: '国产开源模型免费/按量',
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
    hint: 'Gemini 免费额度与直连',
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
  { id: 'priority', label: '按优先级', desc: '按顺序优先调用，故障自动切换下一顺位' },
  { id: 'round_robin', label: '轮询均摊', desc: '在健康节点间轮流分发，缓解免费接口限频' },
  { id: 'lowest_latency', label: '最快延迟', desc: '优先选择近期测速/响应最快的健康模型' },
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

export function strategyLabel(strategy?: string): string {
  const found = POOL_STRATEGY_OPTIONS.find((s) => s.id === strategy)
  return found ? found.label : '按优先级'
}

export interface RoutePreviewResult {
  tone: 'free' | 'paid' | 'warn'
  headline: string
  chain: string[]
  note?: string
}

export function buildRoutePreview(params: {
  selectedModel: string
  allowPaidFallback: boolean
  maxPaidCallsPerDay?: number
  providers: ModelProvider[]
  pooledModels: PooledModel[]
  strategies?: PoolStrategies
  isTask?: boolean
  needTools?: boolean
}): RoutePreviewResult {
  const {
    selectedModel,
    allowPaidFallback,
    maxPaidCallsPerDay = 0,
    providers,
    pooledModels,
    strategies,
    isTask = false,
    needTools = false,
  } = params

  const pMap = new Map<string, ModelProvider>()
  for (const p of providers || []) {
    if (p.enabled && p.api_key && p.base_url) {
      pMap.set(p.id, p)
    }
  }

  const readyByTier = (tier: 'free' | 'paid') =>
    (pooledModels || [])
      .filter((m) => m.enabled && m.tier === tier && pMap.has(m.provider_id))
      .sort((a, b) => a.priority - b.priority)

  const freeList = readyByTier('free')
  const paidList = readyByTier('paid')
  const fmtModel = (m: PooledModel) => {
    const p = pMap.get(m.provider_id)
    return `${p?.name || m.provider_id} · ${m.model_id}`
  }

  const req = (selectedModel || '').trim()
  const quotaNote =
    allowPaidFallback && maxPaidCallsPerDay > 0 ? ` · 付费上限 ${maxPaidCallsPerDay} 次/日` : ''
  const toolsNote = needTools ? ' · 优先已验证 Tools 节点' : ''

  if (req === 'auto:free' || req === 'free-pool') {
    const chain = freeList.slice(0, 3).map(fmtModel)
    if (freeList.length > 3) chain.push(`+${freeList.length - 3} 个免费备选`)

    if (freeList.length === 0) {
      if (allowPaidFallback && paidList.length > 0) {
        return {
          tone: 'paid',
          headline: `免费池为空 -> 将直接回退付费池（${strategyLabel(strategies?.paid)}${quotaNote}）`,
          chain: paidList.slice(0, 2).map(fmtModel),
        }
      }
      return {
        tone: 'warn',
        headline: '免费模型池暂无可用节点，执行时将被拦截',
        chain: [],
        note: '请先在「系统设置 -> AI 大脑设置」中向免费池添加模型，或单独开启「允许付费兜底」。',
      }
    }

    if (allowPaidFallback && paidList.length > 0) {
      chain.push(`兜底: ${fmtModel(paidList[0])}`)
      return {
        tone: 'free',
        headline: `免费池优先（${strategyLabel(strategies?.free)}${toolsNote}），异常时允许付费兜底${quotaNote}`,
        chain,
      }
    }

    return {
      tone: 'free',
      headline: `仅走免费模型池（${strategyLabel(strategies?.free)}${toolsNote}）· 严禁触发付费调用`,
      chain,
    }
  }

  if (!req || req === 'auto:paid' || req === 'paid-pool') {
    if (isTask && !allowPaidFallback) {
      if (!req && freeList.length > 0) {
        return {
          tone: 'free',
          headline: `未开启付费调用 -> 仅走免费模型池（${strategyLabel(strategies?.free)}${toolsNote}）`,
          chain: freeList.slice(0, 3).map(fmtModel),
        }
      }
      return {
        tone: 'warn',
        headline: '冲突提醒：已选付费池，但未开启「允许该任务调用付费模型」',
        chain: [],
        note: '运行时会被防付费护栏拦截；如需执行请开启右侧开关。',
      }
    }
    if (paidList.length === 0) {
      if (freeList.length > 0) {
        return {
          tone: 'free',
          headline: `付费池为空 -> 自动回退免费模型池（${strategyLabel(strategies?.free)}${toolsNote}）`,
          chain: freeList.slice(0, 3).map(fmtModel),
        }
      }
      return {
        tone: 'warn',
        headline: '付费模型池暂无可用节点',
        chain: [],
        note: '请先在「系统设置 -> AI 大脑设置 -> 付费模型池」添加模型。',
      }
    }
    const chain = paidList.slice(0, 3).map(fmtModel)
    if (paidList.length > 3) chain.push(`+${paidList.length - 3} 个付费备选`)
    return {
      tone: 'paid',
      headline: `付费主力池优先（${strategyLabel(strategies?.paid)}${toolsNote}${quotaNote}）`,
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
      const tier = matched.tier
      if (tier === 'paid' && isTask && !allowPaidFallback) {
        return {
          tone: 'warn',
          headline: `冲突提醒：已指定付费模型 ${matched.model_id}，但未开启付费调用开关`,
          chain: [],
          note: '定时触发时将被防付费护栏拦截。',
        }
      }
      const chain = [fmtModel(matched)]
      if (tier === 'free') {
        if (freeList.length > 1) chain.push(`同池容灾 (${freeList.length - 1})`)
        if (allowPaidFallback && paidList.length > 0) chain.push(`付费兜底 (${paidList.length})`)
      } else if (paidList.length > 1) {
        chain.push(`同池容灾 (${paidList.length - 1})`)
      }
      return {
        tone: tier === 'paid' ? 'paid' : 'free',
        headline:
          tier === 'paid'
            ? `定点使用付费模型${quotaNote}`
            : allowPaidFallback
            ? `定点使用免费模型，故障时可切换免费池/付费池${quotaNote}`
            : '定点使用免费模型 · 严禁触发付费调用',
        chain,
      }
    }
  }

  return {
    tone: 'free',
    headline: `指定模型: ${req}`,
    chain: [req],
  }
}
