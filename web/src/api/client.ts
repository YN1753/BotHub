import type {
  AgentMemory,
  AgentMessage,
  Asset,
  Bot,
  DashboardPayload,
  ModelPoolEntry,
  Task,
} from '../types/bothub'

export async function apiRequest<T = any>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: 'same-origin',
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    },
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(data.error || `请求失败 (${res.status})`)
  }
  return data as T
}

export const BotHubApi = {
  authStatus: () => apiRequest<{ authenticated: boolean }>('/api/auth/status'),
  login: (password: string) =>
    apiRequest('/api/auth/login', { method: 'POST', body: JSON.stringify({ password }) }),
  logout: () => apiRequest('/api/auth/logout', { method: 'POST' }),
  updatePassword: (new_password: string) =>
    apiRequest('/api/auth/password', { method: 'POST', body: JSON.stringify({ new_password }) }),
  dashboard: () => apiRequest<DashboardPayload>('/api/dashboard'),
  saveConfig: (payload: Record<string, string>) =>
    apiRequest('/api/config', { method: 'POST', body: JSON.stringify(payload) }),
  saveModelPool: (entries: ModelPoolEntry[]) =>
    apiRequest<{ ok: boolean; model_pool: ModelPoolEntry[] }>('/api/v2/model-pool', {
      method: 'POST',
      body: JSON.stringify({ entries }),
    }),
  testModelTarget: (payload: {
    target_id: string
    provider: string
    base_url: string
    api_key: string
    model: string
  }) =>
    apiRequest<{
      ok: boolean
      latency_ms: number
      reply: string
      model_health?: Record<string, any>
      message: string
    }>('/api/v2/model-pool/test', { method: 'POST', body: JSON.stringify(payload) }),
  testPush: () => apiRequest<{ ok: boolean; message: string }>('/api/test-push', { method: 'POST' }),
  rotateAppToken: () =>
    apiRequest<{ ok: boolean; token: string; message: string }>('/api/v2/auth/token/rotate', {
      method: 'POST',
    }),
  importBackup: (payload: unknown) =>
    apiRequest('/api/system/import', { method: 'POST', body: JSON.stringify(payload) }),

  // Assets
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
  getBotMessages: (id: string) =>
    apiRequest<{ ok: boolean; bot: Bot; messages: AgentMessage[] }>(
      `/api/v2/bots/${encodeURIComponent(id)}/messages`
    ),
  clearBotMessages: (id: string) =>
    apiRequest(`/api/v2/bots/${encodeURIComponent(id)}/messages`, { method: 'DELETE' }),
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
  saveMemory: (payload: Pick<AgentMemory, 'category' | 'fact_key' | 'content'> & { scope?: string }) =>
    apiRequest('/api/agent/memories', { method: 'POST', body: JSON.stringify(payload) }),
  deleteMemory: (id: number) =>
    apiRequest(`/api/agent/memories/${encodeURIComponent(String(id))}`, { method: 'DELETE' }),

  // Notifications
  markNotificationRead: (id: number) =>
    apiRequest(`/api/v2/notifications/${encodeURIComponent(String(id))}/read`, { method: 'POST' }),
  markAllNotificationsRead: () =>
    apiRequest('/api/v2/notifications/read-all', { method: 'POST' }),
}
