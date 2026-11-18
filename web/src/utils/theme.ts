import { ref } from 'vue'

export type BotHubThemeId =
  | 'obsidian'
  | 'ocean'
  | 'emerald'
  | 'amethyst'
  | 'daylight'
  | 'parchment'
  | 'system'

export type ResolvedBotHubThemeId = Exclude<BotHubThemeId, 'system'>

export interface BotHubThemeOption {
  id: BotHubThemeId
  label: string
  desc: string
  swatchAccent: string
  swatchBg: string
  themeColor: string
  isLightPreview: boolean
  glowA: string
  glowB: string
  starRgb: string
}

export const THEME_STORAGE_KEY = 'bothub_theme'

export const BOTHUB_THEME_OPTIONS: BotHubThemeOption[] = [
  {
    id: 'obsidian',
    label: '曜石暖金',
    desc: '深邃夜空配暖香槟金',
    swatchAccent: '#f2b880',
    swatchBg: '#080d1b',
    themeColor: '#080c1c',
    isLightPreview: false,
    glowA: 'rgba(242, 184, 128, 0.08)',
    glowB: 'rgba(99, 102, 241, 0.08)',
    starRgb: '245, 243, 255'
  },
  {
    id: 'ocean',
    label: '深海青岚',
    desc: '沧溟夜海配清澈天青',
    swatchAccent: '#38bdf8',
    swatchBg: '#071624',
    themeColor: '#061320',
    isLightPreview: false,
    glowA: 'rgba(56, 189, 248, 0.09)',
    glowB: 'rgba(37, 99, 235, 0.09)',
    starRgb: '224, 242, 254'
  },
  {
    id: 'emerald',
    label: '松烟翡翠',
    desc: '静谧林夜配润泽翠绿',
    swatchAccent: '#34d399',
    swatchBg: '#071813',
    themeColor: '#061510',
    isLightPreview: false,
    glowA: 'rgba(52, 211, 153, 0.085)',
    glowB: 'rgba(13, 148, 136, 0.085)',
    starRgb: '209, 250, 229'
  },
  {
    id: 'amethyst',
    label: '紫晶星云',
    desc: '暮夜星云配幻紫微光',
    swatchAccent: '#c084fc',
    swatchBg: '#120b24',
    themeColor: '#110b22',
    isLightPreview: false,
    glowA: 'rgba(192, 132, 252, 0.09)',
    glowB: 'rgba(236, 72, 153, 0.07)',
    starRgb: '243, 232, 255'
  },
  {
    id: 'daylight',
    label: '晴雪素白',
    desc: '澄澈晴空白昼与钴蓝',
    swatchAccent: '#2563eb',
    swatchBg: '#f3f6fb',
    themeColor: '#f2f5fa',
    isLightPreview: true,
    glowA: 'rgba(37, 99, 235, 0.06)',
    glowB: 'rgba(99, 102, 241, 0.05)',
    starRgb: '37, 99, 235'
  },
  {
    id: 'parchment',
    label: '宣纸暖茶',
    desc: '温润米白配沉稳琥珀茶',
    swatchAccent: '#b45309',
    swatchBg: '#f6f0e4',
    themeColor: '#f4eee2',
    isLightPreview: true,
    glowA: 'rgba(180, 83, 9, 0.065)',
    glowB: 'rgba(217, 119, 6, 0.055)',
    starRgb: '180, 83, 9'
  },
  {
    id: 'system',
    label: '跟随系统',
    desc: '随系统明暗自动切换',
    swatchAccent: '#94a3b8',
    swatchBg: '#1e293b',
    themeColor: '#080c1c',
    isLightPreview: false,
    glowA: 'rgba(242, 184, 128, 0.08)',
    glowB: 'rgba(99, 102, 241, 0.08)',
    starRgb: '245, 243, 255'
  }
]

const VALID_IDS = new Set<string>(BOTHUB_THEME_OPTIONS.map((t) => t.id))

function normalizeThemeId(raw: string | null | undefined): BotHubThemeId {
  return raw && VALID_IDS.has(raw) ? (raw as BotHubThemeId) : 'obsidian'
}

export function resolveThemeId(themeId: BotHubThemeId): ResolvedBotHubThemeId {
  if (themeId !== 'system') return themeId
  if (typeof window !== 'undefined' && window.matchMedia) {
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'daylight' : 'obsidian'
  }
  return 'obsidian'
}

export function getResolvedThemeMeta(themeId: BotHubThemeId): BotHubThemeOption {
  const resolved = resolveThemeId(themeId)
  return BOTHUB_THEME_OPTIONS.find((item) => item.id === resolved) || BOTHUB_THEME_OPTIONS[0]
}

const currentThemeId = ref<BotHubThemeId>(
  typeof window !== 'undefined'
    ? (() => {
        try {
          return normalizeThemeId(window.localStorage.getItem(THEME_STORAGE_KEY))
        } catch {
          return 'obsidian'
        }
      })()
    : 'obsidian'
)

const resolvedThemeId = ref<ResolvedBotHubThemeId>(resolveThemeId(currentThemeId.value))

export function applyThemeToDom(themeId: BotHubThemeId) {
  if (typeof document === 'undefined') return
  const resolved = resolveThemeId(themeId)
  const meta = getResolvedThemeMeta(themeId)
  resolvedThemeId.value = resolved

  const root = document.documentElement
  root.dataset.theme = resolved
  root.dataset.colorMode = meta.isLightPreview ? 'light' : 'dark'
  root.classList.toggle('dark', !meta.isLightPreview)

  const schemeMeta = document.querySelector('meta[name="color-scheme"]')
  if (schemeMeta) {
    schemeMeta.setAttribute(
      'content',
      themeId === 'system' ? 'light dark' : meta.isLightPreview ? 'light' : 'dark'
    )
  }

  const themeMeta = document.querySelector('meta[name="theme-color"]')
  if (themeMeta) {
    themeMeta.setAttribute('content', meta.themeColor)
  }
}

export function setAppTheme(themeId: BotHubThemeId) {
  const safeId = normalizeThemeId(themeId)
  currentThemeId.value = safeId
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, safeId)
  } catch {}
  applyThemeToDom(safeId)
}

let listenersBound = false
export function initThemeSync() {
  applyThemeToDom(currentThemeId.value)
  if (listenersBound || typeof window === 'undefined') return
  listenersBound = true

  if (window.matchMedia) {
    const mq = window.matchMedia('(prefers-color-scheme: light)')
    const onSystemChange = () => {
      if (currentThemeId.value === 'system') {
        applyThemeToDom('system')
      }
    }
    if (mq.addEventListener) {
      mq.addEventListener('change', onSystemChange)
    }
  }

  window.addEventListener('storage', (ev) => {
    if (ev.key === THEME_STORAGE_KEY) {
      currentThemeId.value = normalizeThemeId(ev.newValue)
      applyThemeToDom(currentThemeId.value)
    }
  })
}

export function useAppTheme() {
  return {
    themeId: currentThemeId,
    resolvedThemeId,
    themeOptions: BOTHUB_THEME_OPTIONS,
    setAppTheme
  }
}
