import type {
  AgentMemory,
  AgentMessage,
  Asset,
  AssetCategoryOption,
  Bot,
  BotExecutionLog,
  BotThread,
  DashboardPayload,
  ModelHealthStats,
  ModelOptionDTO,
  ModelProvider,
  ModelUsageSummary,
  PoolStrategies,
  PooledModel,
  Skill,
  StashItem,
  StashTagCount,
  Task,
  TodoItem,
  WorkNote,
  WorkspaceBriefing,
} from '../types/bothub'

async function executeRequest<T = any>(
  path: string,
  options: RequestInit & { timeoutMs?: number } = {}
): Promise<{ status: number; data: T }> {
  const { timeoutMs = 45_000, ...fetchOpts } = options
  let signal: AbortSignal | undefined = fetchOpts.signal ?? undefined
  if (timeoutMs > 0 && typeof AbortSignal !== 'undefined' && typeof AbortSignal.timeout === 'function') {
    const deadlineSignal = AbortSignal.timeout(timeoutMs)
    signal =
      signal && typeof AbortSignal.any === 'function'
        ? AbortSignal.any([signal, deadlineSignal])
        : signal || deadlineSignal
  }
  const hasBody = fetchOpts.body !== undefined && fetchOpts.body !== null
  const headers: Record<string, string> = {
    ...(hasBody ? { 'Content-Type': 'application/json' } : {}),
    ...((fetchOpts.headers as Record<string, string> | undefined) || {}),
  }
  try {
    const res = await fetch(path, {
      credentials: 'same-origin',
      ...fetchOpts,
      signal,
      headers,
    })
    const rawText = await res.text().catch(() => '')
    let data: any = {}
    if (rawText) {
      try {
        data = JSON.parse(rawText)
      } catch {
        // Not JSON (e.g. reverse proxy 502/504 HTML error page)
      }
    }
    if (res.status === 401 && !path.startsWith('/api/auth/') && typeof window !== 'undefined') {
      window.dispatchEvent(new CustomEvent('bothub:unauthorized'))
    }
    if (!res.ok) {
      let msg = data?.error
      if (!msg && rawText) {
        const titleMatch = rawText.match(/<title[^>]*>([^<]+)<\/title>/i)
        const snippet = titleMatch ? titleMatch[1].trim() : rawText.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim().slice(0, 120)
        if (snippet) msg = `${res.status} ${snippet}`
      }
      const err: any = new Error(msg || `请求失败 (${res.status})`)
      err.status = res.status
      throw err
    }
    return { status: res.status, data: data as T }
  } catch (err: any) {
    if (err?.name === 'TimeoutError') {
      throw new Error('请求超时，请检查网络连接或服务端响应状态', { cause: err })
    }
    throw err
  }
}

export async function apiRequest<T = any>(
  path: string,
  options: RequestInit & { timeoutMs?: number } = {}
): Promise<T> {
  const method = (options.method || 'GET').toUpperCase()
  const isIdempotentGet = method === 'GET' || method === 'HEAD'
  try {
    const res = await executeRequest<T>(path, options)
    return res.data
  } catch (err: any) {
    const status = Number(err?.status || 0)
    const transient = !status || status === 502 || status === 503 || status === 504
    if (isIdempotentGet && transient && !options.signal?.aborted) {
      await new Promise((r) => setTimeout(r, 350))
      const retry = await executeRequest<T>(path, options)
      return retry.data
    }
    throw err
  }
}

export const BotHubApi = {
  authStatus: () =>
    apiRequest<{
      authenticated: boolean
      oidc_enabled?: boolean
      oidc_provider_name?: string
    }>('/api/auth/status'),
  passkeyStatus: () =>
    apiRequest<{ has_passkeys: boolean; count: number; rp_id: string }>('/api/auth/passkey/status'),
  listPasskeys: () =>
    apiRequest<{ rpId: string; userId: string; credentials: Array<any> }>('/api/auth/passkeys'),
  renamePasskey: (id: string, name: string) =>
    apiRequest<{ ok: boolean; name: string }>(`/api/auth/passkeys/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify({ name }),
    }),
  deletePasskey: (id: string) =>
    apiRequest<{ ok: boolean }>(`/api/auth/passkeys/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  createAppPairCode: () =>
    apiRequest<{ ok: boolean; code: string; expires_in: number; deep_link: string }>(
      '/api/v2/auth/pair/create',
      { method: 'POST' }
    ),
  login: (password: string, remember_device = false) =>
    apiRequest('/api/auth/login', { method: 'POST', body: JSON.stringify({ password, remember_device }) }),
  logout: () => apiRequest('/api/auth/logout', { method: 'POST' }),
  logoutAllDevices: (keep_current = true) =>
    apiRequest<{ ok: boolean; keep_current: boolean; message: string }>('/api/auth/logout-all', {
      method: 'POST',
      body: JSON.stringify({ keep_current }),
    }),
  updatePassword: (new_password: string) =>
    apiRequest('/api/auth/password', { method: 'POST', body: JSON.stringify({ new_password }) }),
  dashboard: () => apiRequest<DashboardPayload>('/api/dashboard'),
  dashboardSlice: (kinds: string[]) =>
    apiRequest<Partial<DashboardPayload> & { ok: boolean }>(
      `/api/v2/dashboard/slice?kinds=${encodeURIComponent(kinds.join(','))}`
    ),
  saveConfig: (payload: Record<string, string>) =>
    apiRequest('/api/config', { method: 'POST', body: JSON.stringify(payload) }),
  getModelConfig: () =>
    apiRequest<{
      ok: boolean
      providers: ModelProvider[]
      models: PooledModel[]
      strategies: PoolStrategies
      model_options?: ModelOptionDTO[]
      model_health?: Record<string, ModelHealthStats>
      model_usage_summary?: ModelUsageSummary
    }>('/api/v2/model-config'),
  saveModelConfig: (payload: {
    providers: ModelProvider[]
    models: PooledModel[]
    strategies: PoolStrategies
  }) =>
    apiRequest<{
      ok: boolean
      providers: ModelProvider[]
      models: PooledModel[]
      strategies: PoolStrategies
      model_options?: ModelOptionDTO[]
      model_health?: Record<string, ModelHealthStats>
      model_usage_summary?: ModelUsageSummary
    }>('/api/v2/model-config', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  clearModelCooldown: (target_id = '') =>
    apiRequest<{
      ok: boolean
      model_health?: Record<string, ModelHealthStats>
      model_options?: ModelOptionDTO[]
    }>('/api/v2/model-pool/cooldown/clear', {
      method: 'POST',
      body: JSON.stringify({ target_id }),
    }),
  testModelTarget: (payload: {
    provider_id?: string
    entry_id?: string
    target_id?: string
    protocol?: string
    provider?: string
    base_url: string
    api_key: string
    model: string
    probe_tools?: boolean
  }) =>
    apiRequest<{
      ok: boolean
      target_id?: string
      latency_ms: number
      preview?: string
      error?: string
      supports_tools?: boolean
      supports_thinking?: boolean
      pooled_models?: PooledModel[]
      model_health?: Record<string, ModelHealthStats>
      model_options?: ModelOptionDTO[]
    }>('/api/v2/model-pool/test', { method: 'POST', timeoutMs: 90_000, body: JSON.stringify(payload) }),
  testModelBatch: (payload: {
    tier?: 'free' | 'paid' | ''
    disable_failed?: boolean
    probe_tools?: boolean
  }) =>
    apiRequest<{
      ok: boolean
      results: Array<{
        target_id: string
        provider_id: string
        model_id: string
        tier: string
        ok: boolean
        latency_ms: number
        error?: string
        supports_tools?: boolean
        supports_thinking?: boolean
      }>
      pooled_models?: PooledModel[]
      model_health?: Record<string, ModelHealthStats>
      model_options?: ModelOptionDTO[]
    }>('/api/v2/model-pool/test-batch', { method: 'POST', timeoutMs: 120_000, body: JSON.stringify(payload) }),
  fetchUpstreamModels: (payload: {
    provider_id?: string
    entry_id?: string
    target_id?: string
    protocol?: string
    provider?: string
    base_url: string
    api_key: string
    api_keys?: string[]
    verify?: boolean
  }) =>
    apiRequest<{
      ok: boolean
      models: string[]
      count: number
      checked_models?: Array<{
        id: string
        usable: boolean
        latency_ms?: number
        reason?: string
      }>
      filtered_out_count?: number
      failed_verify_count?: number
      unverified_count?: number
    }>('/api/v2/model-pool/models', { method: 'POST', timeoutMs: 120_000, body: JSON.stringify(payload) }),
  testPush: () => apiRequest<{ ok: boolean; message: string }>('/api/test-push', { method: 'POST' }),
  rotateAppToken: () =>
    apiRequest<{ ok: boolean; token: string; message: string }>('/api/v2/auth/token/rotate', {
      method: 'POST',
    }),
  importBackup: (payload: unknown) =>
    apiRequest('/api/system/import', { method: 'POST', body: JSON.stringify(payload) }),

  // Assets & Extensible Categories
  saveAssetCategories: (categories: AssetCategoryOption[]) =>
    apiRequest<{ ok: boolean; categories: AssetCategoryOption[] }>('/api/v2/asset-categories', {
      method: 'PUT',
      body: JSON.stringify({ categories }),
    }),
  createAsset: (payload: Partial<Asset>) =>
    apiRequest('/api/assets', { method: 'POST', body: JSON.stringify(payload) }),
  updateAsset: (id: string, payload: Partial<Asset>) =>
    apiRequest(`/api/assets/${encodeURIComponent(id)}`, { method: 'POST', body: JSON.stringify(payload) }),
  deleteAsset: (id: string) =>
    apiRequest(`/api/assets/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  syncAssets: () =>
    apiRequest<{ summary: string }>('/api/modules/asset_watcher/run', { method: 'POST' }),

  // Tasks / Routines
  createTask: (payload: Partial<Task>) =>
    apiRequest('/api/tasks', { method: 'POST', body: JSON.stringify(payload) }),
  updateTask: (id: string, payload: Partial<Task>) =>
    apiRequest(`/api/tasks/${encodeURIComponent(id)}`, { method: 'POST', body: JSON.stringify(payload) }),
  deleteTask: (id: string) =>
    apiRequest(`/api/tasks/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  runTask: (id: string) =>
    apiRequest<{ summary: string }>(`/api/tasks/${encodeURIComponent(id)}/run`, { method: 'POST' }),

  // Persistent Bots
  createBot: (payload: Partial<Bot>) =>
    apiRequest<{ ok: boolean; bot: Bot }>('/api/v2/bots', { method: 'POST', body: JSON.stringify(payload) }),
  updateBot: (id: string, payload: Partial<Bot>) =>
    apiRequest<{ ok: boolean; bot: Bot }>(`/api/v2/bots/${encodeURIComponent(id)}`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  deleteBot: (id: string) =>
    apiRequest(`/api/v2/bots/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  getBotMessages: (id: string, threadId?: string) => {
    const qs = threadId ? `?thread_id=${encodeURIComponent(threadId)}` : ''
    return apiRequest<{
      ok: boolean
      bot: Bot
      thread_id: string
      messages: AgentMessage[]
      threads: BotThread[]
      memories: AgentMemory[]
      executions: BotExecutionLog[]
    }>(`/api/v2/bots/${encodeURIComponent(id)}/messages${qs}`)
  },
  clearBotMessages: (id: string, threadId?: string) => {
    const qs = threadId ? `?thread_id=${encodeURIComponent(threadId)}` : ''
    return apiRequest(`/api/v2/bots/${encodeURIComponent(id)}/messages${qs}`, { method: 'DELETE' })
  },
  createBotThread: (botId: string, title?: string) =>
    apiRequest<{ ok: boolean; thread: BotThread; threads: BotThread[] }>(
      `/api/v2/bots/${encodeURIComponent(botId)}/threads`,
      { method: 'POST', body: JSON.stringify({ title: title || '' }) }
    ),
  forkBotThread: (
    botId: string,
    payload: { source_thread_id?: string; up_to_msg_id?: number; title?: string }
  ) =>
    apiRequest<{ ok: boolean; thread: BotThread; threads: BotThread[]; messages: AgentMessage[] }>(
      `/api/v2/bots/${encodeURIComponent(botId)}/threads/fork`,
      { method: 'POST', body: JSON.stringify(payload) }
    ),
  updateBotThread: (botId: string, threadId: string, payload: { title?: string; archived?: boolean }) =>
    apiRequest<{ ok: boolean; thread: BotThread; threads: BotThread[] }>(
      `/api/v2/bots/${encodeURIComponent(botId)}/threads/${encodeURIComponent(threadId)}`,
      { method: 'POST', body: JSON.stringify(payload) }
    ),
  deleteBotThread: (botId: string, threadId: string) =>
    apiRequest<{ ok: boolean; threads: BotThread[] }>(
      `/api/v2/bots/${encodeURIComponent(botId)}/threads/${encodeURIComponent(threadId)}`,
      { method: 'DELETE' }
    ),
  summarizeBotThread: (botId: string, threadId: string, saveToMemory = true) =>
    apiRequest<{
      ok: boolean
      summary: string
      saved_fact_key?: string
      threads: BotThread[]
      memories: AgentMemory[]
    }>(
      `/api/v2/bots/${encodeURIComponent(botId)}/threads/${encodeURIComponent(threadId || 'main')}/summarize`,
      { method: 'POST', body: JSON.stringify({ save_to_memory: saveToMemory }) }
    ),
  deleteBotMessage: (botId: string, msgId: number) =>
    apiRequest(`/api/v2/bots/${encodeURIComponent(botId)}/messages/${encodeURIComponent(String(msgId))}`, {
      method: 'DELETE',
    }),
  updateGroupMembers: (id: string, action: 'add' | 'remove' | 'set', bot_ids: string[]) =>
    apiRequest<{ ok: boolean; bot: Bot; system_message?: AgentMessage }>(
      `/api/v2/bots/${encodeURIComponent(id)}/members`,
      { method: 'POST', body: JSON.stringify({ action, bot_ids }) }
    ),
  createAdhocGroup: (payload: {
    source_bot_id?: string
    member_ids: string[]
    name?: string
    group_mode?: string
    carry_context?: boolean
  }) => apiRequest<{ ok: boolean; bot: Bot }>('/api/v2/bots/adhoc-group', { method: 'POST', body: JSON.stringify(payload) }),
  forwardBotMessage: (
    targetBotId: string,
    payload: { from_name: string; content: string; note?: string }
  ) =>
    apiRequest<{ ok: boolean; message: AgentMessage }>(`/api/v2/bots/${encodeURIComponent(targetBotId)}/forward`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),

  // Memories
  listMemories: (scope?: string, q?: string) => {
    const params = new URLSearchParams()
    if (scope) params.set('scope', scope)
    if (q) params.set('q', q)
    const qs = params.toString() ? `?${params.toString()}` : ''
    return apiRequest<{ ok: boolean; memories: AgentMemory[] }>(`/api/agent/memories${qs}`)
  },
  saveMemory: (
    payload: Pick<AgentMemory, 'category' | 'fact_key' | 'content'> & {
      scope?: string
      importance?: number
      pinned?: boolean
    }
  ) => apiRequest('/api/agent/memories', { method: 'POST', body: JSON.stringify(payload) }),
  updateMemory: (
    id: number,
    payload: Partial<Pick<AgentMemory, 'scope' | 'category' | 'fact_key' | 'content' | 'importance' | 'pinned'>>
  ) =>
    apiRequest<{ ok: boolean; memory: AgentMemory }>(
      `/api/agent/memories/${encodeURIComponent(String(id))}`,
      { method: 'POST', body: JSON.stringify(payload) }
    ),
  deleteMemory: (id: number) =>
    apiRequest(`/api/agent/memories/${encodeURIComponent(String(id))}`, { method: 'DELETE' }),
  pruneMemories: (scope: string, maxKeep = 25) =>
    apiRequest<{ ok: boolean; scope: string; removed: number }>('/api/agent/memories/prune', {
      method: 'POST',
      body: JSON.stringify({ scope, max_keep: maxKeep }),
    }),
  consolidateMemories: (scope = 'all') =>
    apiRequest<{ ok: boolean; scope: string; merged: number }>('/api/agent/memories/consolidate', {
      method: 'POST',
      body: JSON.stringify({ scope }),
    }),

  // Custom Skills
  createSkill: (payload: Partial<Skill>) =>
    apiRequest<{ ok: boolean; skill: Skill }>('/api/v2/skills', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  updateSkill: (id: string, payload: Partial<Skill>) =>
    apiRequest<{ ok: boolean; skill: Skill }>(`/api/v2/skills/${encodeURIComponent(id)}`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  deleteSkill: (id: string) =>
    apiRequest(`/api/v2/skills/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  testSkill: (id: string, args: Record<string, any>) =>
    apiRequest<{ ok: boolean; status_code: number; latency_ms: number; result: any }>(
      `/api/v2/skills/${encodeURIComponent(id)}/test`,
      { method: 'POST', body: JSON.stringify({ args }) }
    ),

  // Notifications
  markNotificationRead: (id: number) =>
    apiRequest(`/api/v2/notifications/${encodeURIComponent(String(id))}/read`, { method: 'POST' }),
  deleteNotification: (id: number) =>
    apiRequest<{ ok: boolean; unread_count: number }>(`/api/v2/notifications/${encodeURIComponent(String(id))}`, {
      method: 'DELETE',
    }),
  markAllNotificationsRead: () =>
    apiRequest('/api/v2/notifications/read-all', { method: 'POST' }),
  clearReadNotifications: () =>
    apiRequest<{ ok: boolean; deleted: number; unread_count: number }>('/api/v2/notifications/clear-read', {
      method: 'POST',
    }),

  // Personal Assistant Workbench (Briefing, Todos & Notes)
  getBriefing: () => apiRequest<{ ok: boolean; briefing: WorkspaceBriefing }>('/api/v2/briefing'),
  createTodo: (payload: Partial<TodoItem>) =>
    apiRequest<{ ok: boolean; todo: TodoItem }>('/api/v2/todos', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  updateTodo: (id: string, payload: Partial<TodoItem>) =>
    apiRequest<{ ok: boolean; todo: TodoItem }>(`/api/v2/todos/${encodeURIComponent(id)}`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  deleteTodo: (id: string) =>
    apiRequest(`/api/v2/todos/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  clearDoneTodos: () =>
    apiRequest<{ ok: boolean; removed: number }>('/api/v2/todos/clear-done', { method: 'POST' }),

  listNotes: (q = '', botId = '') => {
    const params = new URLSearchParams()
    if (q) params.set('q', q)
    if (botId) params.set('bot_id', botId)
    const qs = params.toString() ? `?${params.toString()}` : ''
    return apiRequest<{ ok: boolean; notes: WorkNote[] }>(`/api/v2/notes${qs}`)
  },
  createNote: (payload: Partial<WorkNote>) =>
    apiRequest<{ ok: boolean; note: WorkNote }>('/api/v2/notes', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  updateNote: (id: string, payload: Partial<WorkNote>) =>
    apiRequest<{ ok: boolean; note: WorkNote }>(`/api/v2/notes/${encodeURIComponent(id)}`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  deleteNote: (id: string) =>
    apiRequest(`/api/v2/notes/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  // Smart Stash / Capture
  listStash: (
    params: { q?: string; filter?: string; tag?: string; kind?: string; limit?: number } = {},
    signal?: AbortSignal
  ) => {
    const qs = new URLSearchParams()
    if (params.q) qs.set('q', params.q)
    if (params.filter) qs.set('filter', params.filter)
    if (params.tag) qs.set('tag', params.tag)
    if (params.kind) qs.set('kind', params.kind)
    if (params.limit) qs.set('limit', String(params.limit))
    const queryStr = qs.toString() ? `?${qs.toString()}` : ''
    return apiRequest<{ ok: boolean; items: StashItem[]; top_tags: StashTagCount[] }>(
      `/api/v2/stash${queryStr}`,
      { signal }
    )
  },
  createStash: (payload: {
    raw_input: string
    user_note?: string
    tags?: string
    force_reprocess?: boolean
  }) =>
    apiRequest<{ ok: boolean; deduplicated: boolean; item: StashItem }>('/api/v2/stash', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  updateStash: (
    id: string,
    payload: {
      title?: string
      user_note?: string
      tags?: string
      pinned?: boolean
      status?: string
      mark_read?: boolean
      mark_unread?: boolean
    }
  ) =>
    apiRequest<{ ok: boolean; item: StashItem }>(`/api/v2/stash/${encodeURIComponent(id)}`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  markAllStashRead: () =>
    apiRequest<{ ok: boolean; updated: number }>('/api/v2/stash/mark-all-read', {
      method: 'POST',
    }),
  reprocessStash: (id: string) =>
    apiRequest<{ ok: boolean; item: StashItem }>(`/api/v2/stash/${encodeURIComponent(id)}/reprocess`, {
      method: 'POST',
    }),
  deleteStash: (id: string) =>
    apiRequest<{ ok: boolean }>(`/api/v2/stash/${encodeURIComponent(id)}`, { method: 'DELETE' }),
}
