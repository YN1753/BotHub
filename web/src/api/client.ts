import type {
  AgentMemory,
  AgentMessage,
  Asset,
  Bot,
  DashboardPayload,
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

  // Memories
  saveMemory: (payload: Pick<AgentMemory, 'category' | 'fact_key' | 'content'>) =>
    apiRequest('/api/agent/memories', { method: 'POST', body: JSON.stringify(payload) }),
  deleteMemory: (id: number) =>
    apiRequest(`/api/agent/memories/${encodeURIComponent(String(id))}`, { method: 'DELETE' }),

  // Notifications
  markNotificationRead: (id: number) =>
    apiRequest(`/api/v2/notifications/${encodeURIComponent(String(id))}/read`, { method: 'POST' }),
  markAllNotificationsRead: () =>
    apiRequest('/api/v2/notifications/read-all', { method: 'POST' }),
}
