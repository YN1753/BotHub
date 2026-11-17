import type { Bot } from '../types/bothub'

export type BotShape = 'butler' | 'sentinel' | 'radar' | 'critic' | 'swarm' | 'visor'
export type BotColor = 'gold' | 'indigo' | 'emerald' | 'amber' | 'violet'

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
  fill: string
  eye: string
  visor: string
}

export const BOT_SHAPES: ShapeOption[] = [
  { id: 'butler', label: '星冕智脑', hint: '冠冕天线' },
  { id: 'sentinel', label: '哨兵雷达', hint: '双耳感测' },
  { id: 'radar', label: '天穹猎手', hint: '弧形天盘' },
  { id: 'critic', label: '锐角极客', hint: '双角锋锐' },
  { id: 'swarm', label: '星云协从', hint: '双星环绕' },
  { id: 'visor', label: '赛博领航', hint: '全景目镜' },
]

export const BOT_COLORS: ColorOption[] = [
  {
    id: 'gold',
    label: '暖琥珀金',
    ring: '#f2b880',
    glow: 'rgba(242, 184, 128, 0.34)',
    fill: 'rgba(242, 184, 128, 0.14)',
    eye: '#fbf3e8',
    visor: 'rgba(242, 184, 128, 0.2)',
  },
  {
    id: 'indigo',
    label: '深空暮靛',
    ring: '#a5b4fc',
    glow: 'rgba(165, 180, 252, 0.32)',
    fill: 'rgba(165, 180, 252, 0.14)',
    eye: '#eef2ff',
    visor: 'rgba(165, 180, 252, 0.2)',
  },
  {
    id: 'emerald',
    label: '星野翠绿',
    ring: '#34d399',
    glow: 'rgba(52, 211, 153, 0.32)',
    fill: 'rgba(52, 211, 153, 0.14)',
    eye: '#d1fae5',
    visor: 'rgba(52, 211, 153, 0.2)',
  },
  {
    id: 'amber',
    label: '炽焰流金',
    ring: '#fbbf24',
    glow: 'rgba(251, 191, 36, 0.34)',
    fill: 'rgba(251, 191, 36, 0.14)',
    eye: '#fef3c7',
    visor: 'rgba(251, 191, 36, 0.2)',
  },
  {
    id: 'violet',
    label: '星云紫微',
    ring: '#d8b4fe',
    glow: 'rgba(216, 180, 254, 0.34)',
    fill: 'rgba(216, 180, 254, 0.14)',
    eye: '#faf5ff',
    visor: 'rgba(216, 180, 254, 0.2)',
  },
]

const LEGACY_MAP: Record<string, { shape: BotShape; color: BotColor }> = {
  cyan: { shape: 'butler', color: 'gold' },
  indigo: { shape: 'sentinel', color: 'indigo' },
  emerald: { shape: 'radar', color: 'emerald' },
  amber: { shape: 'critic', color: 'amber' },
  violet: { shape: 'swarm', color: 'violet' },
}

export function parseAvatarStyle(raw?: string): { shape: BotShape; color: BotColor } {
  const val = (raw || 'cyan').trim().toLowerCase()
  if (val.includes(':')) {
    const [s, c] = val.split(':')
    const shape = BOT_SHAPES.some((x) => x.id === s) ? (s as BotShape) : 'butler'
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
  try {
    const ids = JSON.parse(groupBot.member_ids || '[]')
    if (Array.isArray(ids)) {
      const resolved = ids.map((id: string) => specialistMap.get(id)).filter((x): x is Bot => Boolean(x))
      if (resolved.length > 0) return resolved
    }
  } catch {
    // ignore malformed member_ids JSON
  }
  return Array.from(specialistMap.values()).slice(0, 4)
}
