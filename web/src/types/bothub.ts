export type BotState = 'idle' | 'thinking' | 'working' | 'blocked' | 'done'

export type AvatarStyle = 'cyan' | 'indigo' | 'amber' | 'emerald' | 'violet'

export type BotMode = 'worker' | 'companion' | 'hybrid'
export type MemoryPolicy = 'standard' | 'isolated'

export interface Bot {
  id: string
  name: string
  role_title: string
  avatar_style: AvatarStyle | string
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
  group_mode?: 'smart' | 'sequential' | 'mention_only' | string
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
}

export type SkillKind = 'prompt' | 'webhook'

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

export interface ToolTrace {
  id: string
  name: string
  arguments: string
  result: string
}

export interface TokenUsage {
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
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
  model_used?: string
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

export interface Asset {
  id: string
  name: string
  category: 'compute' | 'domain' | 'life' | string
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
  description: string
  cron_expr: string
  enabled: boolean
  params: string
  next_run_at?: string
  is_running?: boolean
  last_run?: ModuleRun
  created_at?: string
  updated_at?: string
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

export interface Persona {
  id: string
  name: string
  description: string
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

export interface UpstreamModelCheck {
  id: string
  usable: boolean
  latency_ms?: number
  reason?: string
}

export interface ModelPoolEntry {
  id: string
  name: string
  provider: string
  base_url: string
  api_key: string
  model: string
  enabled: boolean
  priority: number
  note: string
}

export interface ModelHealthStats {
  target_id: string
  success_count: number
  fail_count: number
  last_latency_ms: number
  last_error?: string
  cooldown_sec: number
}

export interface DashboardPayload {
  version: string
  commit: string
  build_time: string
  uptime_sec: number
  server_now: string
  tasks: Task[]
  bots: Bot[]
  skills?: Skill[]
  model_pool?: ModelPoolEntry[]
  available_tools?: AvailableToolMeta[]
  personas?: Persona[]
  model_health?: Record<string, ModelHealthStats>
  templates: TaskTemplate[]
  agent_memories: AgentMemory[]
  notifications: NotificationItem[]
  configs: Record<string, string>
  assets: Asset[]
  push_logs: PushLog[]
  stats: {
    total_assets: number
    total_pushes: number
    sent_pushes: number
    seen_items: number
    total_memories: number
    total_notifications: number
    unread_notifications: number
  }
}

export interface SelectOption {
  value: string
  label: string
}
