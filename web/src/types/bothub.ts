export type BotState = 'idle' | 'thinking' | 'working' | 'blocked' | 'done'

export type BotMode = 'worker' | 'companion' | 'hybrid'
export type MemoryPolicy = 'standard' | 'isolated'

export interface Bot {
  id: string
  name: string
  role_title: string
  avatar_style: string
  system_prompt: string
  model: string
  enable_thinking: boolean
  allowed_tools: string
  bot_mode?: BotMode | string
  memory_policy?: MemoryPolicy | string
  skill_ids?: string
  traits?: string
  active_thread_id?: string
  state: BotState | string
  unread_count: number
  is_pinned: boolean
  is_builtin: boolean
  is_group?: boolean
  member_ids?: string
  group_mode?: 'smart' | 'sequential' | 'parallel' | 'mention_only' | string
  announcement?: string
  sort_order: number
  last_summary: string
  created_at: string
  updated_at: string
}

export interface SkillParamField {
  key: string
  type: 'string' | 'integer' | 'boolean' | string
  description: string
  required?: boolean
  default?: string
}

export interface MCPToolMeta {
  name: string
  description: string
  input_schema?: Record<string, any>
}

export type SkillKind = 'builtin_mcp' | 'mcp' | 'webhook' | 'prompt'

export interface Skill {
  id: string
  slug: string
  name: string
  description: string
  kind: SkillKind | string
  instruction: string
  http_method: string
  http_url: string
  http_headers: string
  params_schema: string
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export interface BotThread {
  id: string
  bot_id: string
  title: string
  summary?: string
  rolling_summary?: string
  summary_up_to_id?: number
  msg_count: number
  archived: boolean
  created_at: string
  updated_at: string
}

export interface BotExecutionLog {
  id: number
  bot_id: string
  thread_id?: string
  exec_type: string
  name: string
  input_summary?: string
  output_summary?: string
  status: string
  duration_ms: number
  created_at: string
}

export interface AgentMessage {
  id: number
  session_id: string
  bot_id?: string
  thread_id?: string
  sender_bot_id?: string
  msg_kind?: 'chat' | 'routine' | 'system' | string
  role: 'user' | 'assistant' | 'tool' | 'system' | string
  content: string
  reasoning_content?: string
  tool_calls?: string
  tool_call_id?: string
  reply_to_id?: number
  reply_preview?: string
  attachments?: string
  model_used?: string
  model_target_id?: string
  created_at: string
}

export interface AgentMemory {
  id: number
  scope?: string
  category: string
  fact_key: string
  content: string
  importance?: number
  pinned?: boolean
  recall_count?: number
  last_recalled_at?: string
  created_at: string
  updated_at: string
}

export interface AssetCategoryOption {
  id: string
  label: string
  icon?: string
}

export interface Asset {
  id: string
  name: string
  category: string
  spec: string
  price: string
  status: string
  due_date: string
  days_left: number
  auto_renew: boolean
  warn_days: string
  last_checked_at?: string
  extra_config: string
}

export interface ModuleRun {
  id: number
  module_id: string
  module_name: string
  trigger_type: string
  status: string
  summary: string
  items_pushed: number
  duration_ms: number
  started_at: string
  finished_at: string
}

export interface Task {
  id: string
  name: string
  type: string
  bot_id?: string
  model?: string
  description: string
  cron_expr: string
  run_at?: string
  enabled: boolean
  params: string
  next_run_at?: string
  is_running?: boolean
  last_run?: ModuleRun
  created_at?: string
  updated_at?: string
}

export interface TodoItem {
  id: string
  bot_id?: string
  thread_id?: string
  title: string
  notes?: string
  status: 'pending' | 'doing' | 'done' | string
  priority: 'high' | 'normal' | 'low' | string
  due_at?: string
  tags?: string
  created_at: string
  updated_at: string
  completed_at?: string
}

export interface WorkNote {
  id: string
  bot_id?: string
  title: string
  content: string
  category: string
  pinned: boolean
  created_at: string
  updated_at: string
}

export type StashKind = 'link' | 'bilibili' | 'github' | 'xiaohongshu' | 'douyin' | 'wechat' | 'note' | string
export type StashStatus = 'pending' | 'processing' | 'ready' | 'failed' | 'archived' | string

export interface StashItem {
  id: string
  kind: StashKind
  url: string
  domain: string
  title: string
  raw_input: string
  user_note: string
  ai_tldr: string
  ai_points: string
  points: string[]
  content_excerpt: string
  tags: string
  status: StashStatus
  error_msg?: string
  pinned: boolean
  read_count: number
  last_reviewed_at?: string
  created_at: string
  updated_at: string
}

export interface StashTagCount {
  tag: string
  count: number
}

export interface WorkspaceBriefing {
  generated_at: string
  today: string
  weekday: string
  open_todo_count: number
  overdue_todos: TodoItem[]
  due_today_todos: TodoItem[]
  high_priority_todos: TodoItem[]
  expiring_assets: Asset[]
  enabled_tasks: Task[]
  unread_alerts: NotificationItem[]
  unread_count: number
  recent_notes: WorkNote[]
  focus_hints?: string[]
}

export interface TaskTemplateField {
  key: string
  label: string
  type: string
  default_val?: string
  default_value?: string
  placeholder?: string
  hint?: string
  options?: Array<string | { value: string; label: string }>
}

export interface TaskTemplate {
  type: string
  name: string
  description: string
  default_cron?: string
  params_schema?: TaskTemplateField[]
  fields?: TaskTemplateField[]
}

export interface NotificationItem {
  id: number
  source: string
  title: string
  summary: string
  markdown: string
  priority: 'high' | 'normal' | 'low' | string
  created_at: string
  read_at: string
}

export interface PushLog {
  id: number
  module_id: string
  title: string
  content: string
  status: string
  response_msg: string
  created_at: string
}

export interface AvailableToolMeta {
  id: string
  description: string
}

export type ProviderProtocol = 'openai' | 'gemini'
export type PoolStrategy = 'priority' | 'round_robin' | 'lowest_latency'

export interface ModelProvider {
  id: string
  name: string
  protocol: ProviderProtocol | string
  base_url: string
  api_key: string
  api_keys?: string[]
  enabled: boolean
  note?: string
  alias_ids?: string[]
}

export interface PooledModel {
  provider_id: string
  model_id: string
  display_name?: string
  enabled: boolean
  priority: number
  is_light?: boolean
  input_price_per_m?: number
  output_price_per_m?: number
  supports_tools?: boolean
  supports_thinking?: boolean
  last_latency_ms?: number
  last_verified_at?: string
}

export interface ModelUsageModelStat {
  target_id: string
  model_id: string
  calls: number
  success_calls?: number
  prompt_tokens?: number
  cached_tokens?: number
  completion_tokens?: number
  tokens: number
  cost_usd?: number
  cost_cny?: number
  avg_latency_ms: number
}

export interface ModelUsageCallerStat {
  bot_id: string
  task_id: string
  source: string
  calls: number
  cached_tokens?: number
  tokens: number
  cost_usd?: number
  cost_cny?: number
}

export interface ModelUsageSummary {
  days: number
  total_calls: number
  success_calls: number
  total_prompt_tokens?: number
  total_cached_tokens?: number
  total_completion_tokens?: number
  total_tokens?: number
  cache_hit_rate?: number
  total_cost_usd?: number
  total_cost_cny?: number
  usd_to_cny?: number
  top_models?: ModelUsageModelStat[]
  top_callers?: ModelUsageCallerStat[]
}

export interface ModelOptionDTO {
  id: string
  provider_id?: string
  label: string
  provider: string
  model: string
  available: boolean
  is_auto?: boolean
  is_light?: boolean
  supports_tools?: boolean
  supports_thinking?: boolean
  last_latency_ms?: number
}

export interface ModelHealthStats {
  target_id: string
  status?: string
  success_count: number
  fail_count: number
  consecutive_fails?: number
  last_latency_ms: number
  last_error?: string
  cooldown_sec: number
}

export interface FXRateSnapshot {
  base: string
  rates: Record<string, number>
  updated_at?: string
  source?: string
}

export interface DashboardPayload {
  version: string
  commit: string
  build_time: string
  uptime_sec: number
  server_now?: string
  tasks: Task[]
  bots: Bot[]
  skills?: Skill[]
  todos?: TodoItem[]
  notes?: WorkNote[]
  stash?: StashItem[]
  stash_tags?: StashTagCount[]
  model_providers?: ModelProvider[]
  pooled_models?: PooledModel[]
  pool_strategy?: PoolStrategy
  model_usage_summary?: ModelUsageSummary
  model_usage_30d?: ModelUsageSummary
  model_options?: ModelOptionDTO[]
  revisions?: Record<string, number>
  available_tools?: AvailableToolMeta[]
  model_health?: Record<string, ModelHealthStats>
  templates: TaskTemplate[]
  agent_memories: AgentMemory[]
  notifications: NotificationItem[]
  configs: Record<string, string>
  assets: Asset[]
  asset_categories?: AssetCategoryOption[]
  fx_rates?: FXRateSnapshot
  push_logs: PushLog[]
  stats: {
    sent_pushes: number
  }
}

export interface SelectOption {
  value: string
  label: string
}
