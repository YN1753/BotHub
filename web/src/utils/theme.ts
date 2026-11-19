import { ref } from 'vue'

export type ColorModeId = 'dark' | 'light' | 'system'
export type ResolvedColorMode = 'dark' | 'light'
export type AccentId = 'amber' | 'blue' | 'cyan' | 'emerald' | 'violet' | 'mono'

export interface ColorModeOption {
  id: ColorModeId
  label: string
}

export interface AccentOption {
  id: AccentId
  label: string
  swatchDark: string
  swatchLight: string
}

export const COLOR_MODE_STORAGE_KEY = 'bothub_color_mode'
export const ACCENT_STORAGE_KEY = 'bothub_accent'
export const LEGACY_THEME_STORAGE_KEY = 'bothub_theme'

export const COLOR_MODE_OPTIONS: ColorModeOption[] = [
  { id: 'dark', label: '深色' },
  { id: 'light', label: '浅色' },
  { id: 'system', label: '跟随系统' },
]

export const ACCENT_OPTIONS: AccentOption[] = [
  { id: 'amber', label: '琥珀金', swatchDark: '#e5a45c', swatchLight: '#b45309' },
  { id: 'blue', label: '经典蓝', swatchDark: '#3b82f6', swatchLight: '#2563eb' },
  { id: 'cyan', label: '青岚', swatchDark: '#06b6d4', swatchLight: '#0284c7' },
  { id: 'emerald', label: '翠绿', swatchDark: '#10b981', swatchLight: '#059669' },
  { id: 'violet', label: '紫晶', swatchDark: '#8b5cf6', swatchLight: '#7c3aed' },
  { id: 'mono', label: '曜石黑白', swatchDark: '#f4f5f7', swatchLight: '#111827' },
]

const VALID_MODES = new Set<string>(COLOR_MODE_OPTIONS.map((o) => o.id))
const VALID_ACCENTS = new Set<string>(ACCENT_OPTIONS.map((o) => o.id))

const LEGACY_THEME_MAP: Record<string, { mode: ColorModeId; accent: AccentId }> = {
  obsidian: { mode: 'dark', accent: 'amber' },
  ocean: { mode: 'dark', accent: 'cyan' },
  emerald: { mode: 'dark', accent: 'emerald' },
  amethyst: { mode: 'dark', accent: 'violet' },
  daylight: { mode: 'light', accent: 'blue' },
  parchment: { mode: 'light', accent: 'amber' },
  system: { mode: 'system', accent: 'blue' },
}

export function migrateLegacyTheme(
  legacyTheme: string | null | undefined
): { mode: ColorModeId; accent: AccentId } {
  if (legacyTheme && Object.prototype.hasOwnProperty.call(LEGACY_THEME_MAP, legacyTheme)) {
    return { ...LEGACY_THEME_MAP[legacyTheme] }
  }
  return { mode: 'dark', accent: 'blue' }
}

function normalizeColorMode(raw: string | null | undefined): ColorModeId {
  return raw && VALID_MODES.has(raw) ? (raw as ColorModeId) : 'dark'
}

function normalizeAccent(raw: string | null | undefined): AccentId {
  return raw && VALID_ACCENTS.has(raw) ? (raw as AccentId) : 'blue'
}

export function resolveColorMode(mode: ColorModeId): ResolvedColorMode {
  if (mode !== 'system') return mode
  if (typeof window !== 'undefined' && window.matchMedia) {
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
  }
  return 'dark'
}

function loadInitialThemeState(): { mode: ColorModeId; accent: AccentId } {
  if (typeof window === 'undefined') {
    return { mode: 'dark', accent: 'blue' }
  }
  try {
    const storedMode = window.localStorage.getItem(COLOR_MODE_STORAGE_KEY)
    const storedAccent = window.localStorage.getItem(ACCENT_STORAGE_KEY)
    if (!storedMode && !storedAccent) {
      const legacy = window.localStorage.getItem(LEGACY_THEME_STORAGE_KEY)
      if (legacy && Object.prototype.hasOwnProperty.call(LEGACY_THEME_MAP, legacy)) {
        const migrated = migrateLegacyTheme(legacy)
        window.localStorage.setItem(COLOR_MODE_STORAGE_KEY, migrated.mode)
        window.localStorage.setItem(ACCENT_STORAGE_KEY, migrated.accent)
        return migrated
      }
    }
    return {
      mode: normalizeColorMode(storedMode),
      accent: normalizeAccent(storedAccent),
    }
  } catch {
    return { mode: 'dark', accent: 'blue' }
  }
}

const initial = loadInitialThemeState()
const colorMode = ref<ColorModeId>(initial.mode)
const accentId = ref<AccentId>(initial.accent)
const resolvedColorMode = ref<ResolvedColorMode>(resolveColorMode(initial.mode))

export function applyThemeToDom(mode: ColorModeId, accent: AccentId) {
  const resolved = resolveColorMode(mode)
  resolvedColorMode.value = resolved
  if (typeof document === 'undefined') return

  const root = document.documentElement
  root.dataset.colorMode = resolved
  root.dataset.accent = accent
  root.classList.toggle('dark', resolved === 'dark')

  const schemeMeta = document.querySelector('meta[name="color-scheme"]')
  if (schemeMeta) {
    schemeMeta.setAttribute('content', mode === 'system' ? 'light dark' : resolved)
  }

  const themeMeta = document.querySelector('meta[name="theme-color"]')
  if (themeMeta) {
    themeMeta.setAttribute('content', resolved === 'light' ? '#ffffff' : '#111317')
  }
}

export function setColorMode(mode: ColorModeId) {
  const safeMode = normalizeColorMode(mode)
  colorMode.value = safeMode
  try {
    window.localStorage.setItem(COLOR_MODE_STORAGE_KEY, safeMode)
  } catch {}
  applyThemeToDom(safeMode, accentId.value)
}

export function setAccent(accent: AccentId) {
  const safeAccent = normalizeAccent(accent)
  accentId.value = safeAccent
  try {
    window.localStorage.setItem(ACCENT_STORAGE_KEY, safeAccent)
  } catch {}
  applyThemeToDom(colorMode.value, safeAccent)
}

let listenersBound = false
export function initThemeSync() {
  applyThemeToDom(colorMode.value, accentId.value)
  if (listenersBound || typeof window === 'undefined') return
  listenersBound = true

  if (window.matchMedia) {
    const mq = window.matchMedia('(prefers-color-scheme: light)')
    const onSystemChange = () => {
      if (colorMode.value === 'system') {
        applyThemeToDom('system', accentId.value)
      }
    }
    if (mq.addEventListener) {
      mq.addEventListener('change', onSystemChange)
    }
  }

  window.addEventListener('storage', (ev) => {
    if (ev.key === COLOR_MODE_STORAGE_KEY) {
      colorMode.value = normalizeColorMode(ev.newValue)
      applyThemeToDom(colorMode.value, accentId.value)
    } else if (ev.key === ACCENT_STORAGE_KEY) {
      accentId.value = normalizeAccent(ev.newValue)
      applyThemeToDom(colorMode.value, accentId.value)
    }
  })
}

export function useAppTheme() {
  return {
    colorMode,
    resolvedColorMode,
    accentId,
    colorModeOptions: COLOR_MODE_OPTIONS,
    accentOptions: ACCENT_OPTIONS,
    setColorMode,
    setAccent,
  }
}
