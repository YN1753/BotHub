import type { Asset, AssetCategoryOption } from '../types/bothub'

export type AssetCategoryIconId =
  | 'tag'
  | 'server'
  | 'globe'
  | 'sparkles'
  | 'key'
  | 'shield'
  | 'package'
  | 'wallet'

export const ASSET_CATEGORY_ICONS: Array<{ id: AssetCategoryIconId; label: string }> = [
  { id: 'tag', label: '通用标签' },
  { id: 'server', label: '算力节点' },
  { id: 'globe', label: '域名网络' },
  { id: 'sparkles', label: '会员订阅' },
  { id: 'key', label: '授权凭证' },
  { id: 'shield', label: '安全保险' },
  { id: 'package', label: '硬件设备' },
  { id: 'wallet', label: '账单合约' },
]

// Purely optional 1-click starter ideas for a blank workspace (never auto-inserted into user data)
export const SUGGESTED_ASSET_CATEGORIES: AssetCategoryOption[] = [
  { id: 'general', label: '通用资产', icon: 'tag' },
  { id: 'compute', label: '算力服务', icon: 'server' },
  { id: 'domain', label: '域名证书', icon: 'globe' },
  { id: 'subscription', label: '会员订阅', icon: 'sparkles' },
  { id: 'license', label: '软件授权', icon: 'key' },
  { id: 'hardware', label: '硬件设备', icon: 'package' },
  { id: 'account', label: '账号凭证', icon: 'shield' },
  { id: 'finance', label: '账单合约', icon: 'wallet' },
]

const LEGACY_DISPLAY_META: Record<string, { label: string; icon: AssetCategoryIconId }> = {
  general: { label: '通用资产', icon: 'tag' },
  other: { label: '其他资产', icon: 'tag' },
  compute: { label: '算力节点', icon: 'server' },
  domain: { label: '域名资产', icon: 'globe' },
  life: { label: '订阅服务', icon: 'sparkles' },
  subscription: { label: '会员订阅', icon: 'sparkles' },
  license: { label: '软件授权', icon: 'key' },
  hardware: { label: '硬件设备', icon: 'package' },
  account: { label: '账号凭证', icon: 'shield' },
  finance: { label: '账单合约', icon: 'wallet' },
}

export function normalizeCategoryId(raw: string): string {
  return String(raw || '').trim().replace(/\s+/g, '_')
}

export function resolveCategoryMeta(
  catId: string,
  savedCategories: AssetCategoryOption[] = []
): { id: string; label: string; icon: AssetCategoryIconId } {
  const id = String(catId || '').trim() || 'general'
  const lower = id.toLowerCase()
  const saved = savedCategories.find((c) => c.id.toLowerCase() === lower)
  if (saved) {
    return {
      id: saved.id,
      label: saved.label || saved.id,
      icon: (saved.icon as AssetCategoryIconId) || 'tag',
    }
  }
  const fallback = LEGACY_DISPLAY_META[lower]
  if (fallback) {
    return { id, label: fallback.label, icon: fallback.icon }
  }
  return { id, label: id, icon: 'tag' }
}

export function mergeExtensibleCategories(
  savedCategories: AssetCategoryOption[],
  assets: Asset[]
): AssetCategoryOption[] {
  const result: AssetCategoryOption[] = []
  const seen = new Set<string>()

  const pushUnique = (item: AssetCategoryOption) => {
    const id = String(item.id || '').trim()
    if (!id) return
    const key = id.toLowerCase()
    if (seen.has(key)) return
    seen.add(key)
    result.push({
      id,
      label: String(item.label || '').trim() || id,
      icon: item.icon || LEGACY_DISPLAY_META[key]?.icon || 'tag',
    })
  }

  for (const cat of savedCategories) {
    pushUnique(cat)
  }
  for (const asset of assets) {
    const rawCat = String(asset.category || '').trim()
    if (!rawCat) continue
    const meta = resolveCategoryMeta(rawCat, savedCategories)
    pushUnique({ id: meta.id, label: meta.label, icon: meta.icon })
  }
  return result
}
