import type { AgentMemory, Bot } from '../types/bothub'
import { botStateDotClass, parseJsonStringArray } from './format.ts'

export { botStateDotClass }

export type BotShape = 'butler' | 'sentinel' | 'radar' | 'critic' | 'swarm' | 'visor'
export type BotColor = 'gold' | 'indigo' | 'emerald' | 'amber' | 'violet' | 'cyan' | 'rose' | 'obsidian'

export interface ShapeOption {
  id: BotShape
  label: string
  hint: string
}

export interface ColorOption {
  id: BotColor
  label: string
  ring: string
  glow: string
  shellHi: string
  shellMid: string
  shellDeep: string
  aurora: string
  eyeInk: string
  blush: string
  shadow: string
}

export const BOT_SHAPES: ShapeOption[] = [
  { id: 'butler', label: '云团灵眸', hint: '圆润糯团' },
  { id: 'sentinel', label: '猫耳灵团', hint: '一体灵耳' },
  { id: 'radar', label: '流光星滴', hint: '水滴一体' },
  { id: 'critic', label: '灵角小魔', hint: '一体双角' },
  { id: 'swarm', label: '绵绵星云', hint: '云朵轮廓' },
  { id: 'visor', label: '方糖果冻', hint: '超椭一体' },
]

export const BOT_COLORS: ColorOption[] = [
  {
    id: 'gold',
    label: '奶白暖杏',
    ring: '#f2b880',
    glow: 'rgba(242, 184, 128, 0.44)',
    shellHi: '#ffffff',
    shellMid: '#f6ede2',
    shellDeep: '#d8c2aa',
    aurora: '#ffb87a',
    eyeInk: '#131826',
    blush: '#ff8570',
    shadow: 'rgba(4, 7, 16, 0.58)',
  },
  {
    id: 'indigo',
    label: '冰透星蓝',
    ring: '#a5b4fc',
    glow: 'rgba(147, 197, 253, 0.44)',
    shellHi: '#ffffff',
    shellMid: '#e6eeff',
    shellDeep: '#b2c5f2',
    aurora: '#7dd3fc',
    eyeInk: '#10172b',
    blush: '#fb7185',
    shadow: 'rgba(4, 7, 16, 0.58)',
  },
  {
    id: 'emerald',
    label: '薄荷青玉',
    ring: '#34d399',
    glow: 'rgba(52, 211, 153, 0.42)',
    shellHi: '#ffffff',
    shellMid: '#e2faef',
    shellDeep: '#9edbc3',
    aurora: '#34d399',
    eyeInk: '#0f201d',
    blush: '#fb923c',
    shadow: 'rgba(4, 7, 16, 0.58)',
  },
  {
    id: 'amber',
    label: '蜜桃晨曦',
    ring: '#fbbf24',
    glow: 'rgba(251, 191, 36, 0.44)',
    shellHi: '#fffefb',
    shellMid: '#ffefd6',
    shellDeep: '#efc48e',
    aurora: '#fb923c',
    eyeInk: '#1f1512',
    blush: '#f43f5e',
    shadow: 'rgba(4, 7, 16, 0.58)',
  },
  {
    id: 'violet',
    label: '雾粉星紫',
    ring: '#d8b4fe',
    glow: 'rgba(216, 180, 254, 0.44)',
    shellHi: '#ffffff',
    shellMid: '#f2e7ff',
    shellDeep: '#c9b1ee',
    aurora: '#f472b6',
    eyeInk: '#1a1228',
    blush: '#fb7185',
    shadow: 'rgba(4, 7, 16, 0.58)',
  },
  {
    id: 'cyan',
    label: '天青星岚',
    ring: '#38bdf8',
    glow: 'rgba(56, 189, 248, 0.44)',
    shellHi: '#ffffff',
    shellMid: '#e0f7fe',
    shellDeep: '#9cd8f2',
    aurora: '#22d3ee',
    eyeInk: '#0e1d2a',
    blush: '#fb7185',
    shadow: 'rgba(4, 7, 16, 0.58)',
  },
  {
    id: 'rose',
    label: '绯红莓果',
    ring: '#fb7185',
    glow: 'rgba(251, 113, 133, 0.44)',
    shellHi: '#ffffff',
    shellMid: '#ffe7ec',
    shellDeep: '#f3afc0',
    aurora: '#f472b6',
    eyeInk: '#261018',
    blush: '#fb923c',
    shadow: 'rgba(4, 7, 16, 0.58)',
  },
  {
    id: 'obsidian',
    label: '暗夜流光',
    ring: '#38bdf8',
    glow: 'rgba(56, 189, 248, 0.45)',
    shellHi: '#374669',
    shellMid: '#182036',
    shellDeep: '#090d1a',
    aurora: '#38bdf8',
    eyeInk: '#f0f9ff',
    blush: '#f472b6',
    shadow: 'rgba(2, 4, 10, 0.72)',
  },
]

const LEGACY_MAP: Record<string, { shape: BotShape; color: BotColor }> = {
  gold: { shape: 'butler', color: 'gold' },
  indigo: { shape: 'sentinel', color: 'indigo' },
  emerald: { shape: 'radar', color: 'emerald' },
  amber: { shape: 'critic', color: 'amber' },
  violet: { shape: 'swarm', color: 'violet' },
  cyan: { shape: 'visor', color: 'cyan' },
  rose: { shape: 'critic', color: 'rose' },
  obsidian: { shape: 'visor', color: 'obsidian' },
}

export function parseAvatarStyle(raw?: string): { shape: BotShape; color: BotColor } {
  const val = (raw || 'butler:gold').trim().toLowerCase()
  if (val.includes(':')) {
    const [rawShape, c] = val.split(':')
    const normalizedShape = rawShape === 'scholar' ? 'visor' : rawShape
    const shape = BOT_SHAPES.some((x) => x.id === normalizedShape) ? (normalizedShape as BotShape) : 'butler'
    const color = BOT_COLORS.some((x) => x.id === c) ? (c as BotColor) : 'gold'
    return { shape, color }
  }
  return LEGACY_MAP[val] || { shape: 'butler', color: 'gold' }
}

export function composeAvatarStyle(shape: BotShape, color: BotColor): string {
  return `${shape}:${color}`
}

export function getColorPalette(color: BotColor): ColorOption {
  return BOT_COLORS.find((c) => c.id === color) || BOT_COLORS[0]
}

export function resolveGroupMemberBots(groupBot: Bot, allBots: Bot[]): Bot[] {
  const specialistMap = new Map<string, Bot>()
  for (const b of allBots) {
    if (!b.is_group) specialistMap.set(b.id, b)
  }
  const ids = parseJsonStringArray(groupBot.member_ids)
  const resolved = ids.map((id) => specialistMap.get(id)).filter((x): x is Bot => Boolean(x))
  if (resolved.length > 0) return resolved
  return Array.from(specialistMap.values()).slice(0, 4)
}

export function pickDefaultWorkerBot(
  bots: Bot[],
  activeBot?: Bot | null,
  preferredBotId?: string
): string {
  if (preferredBotId && bots.some((b) => b.id === preferredBotId)) {
    return preferredBotId
  }
  if (activeBot?.id) return activeBot.id
  return (
    bots.find((b) => !b.is_group && b.bot_mode !== 'companion')?.id ||
    bots[0]?.id ||
    ''
  )
}

export function resolveScopedMemories(
  bot: Bot | null | undefined,
  botMemories: AgentMemory[],
  allMemories: AgentMemory[]
): AgentMemory[] {
  if (botMemories.length > 0) return botMemories
  if (!bot) return []
  const bScope = `bot:${bot.id}`
  if (bot.memory_policy === 'isolated') {
    return allMemories.filter((m) => m.scope === bScope)
  }
  return allMemories.filter((m) => !m.scope || m.scope === 'global' || m.scope === bScope)
}
